package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/routes"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

func main() {
	loadEnv()
	logger := initLogger()

	plans, err := payments.LoadFromEnv()
	if err != nil {
		logger.Fatalf("error [plans]: %v", err)
	}

	dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
	if dsn == "" {
		logger.Fatalf("error [main]: DB_DSN missing")
	}
	store, err := stores.NewStore(dsn)
	if err != nil {
		logger.Fatalf("error [main]: db init failed: %v", err)
	}

	if err := store.EnsureDefaultTools(stores.DefaultTools()); err != nil {
		logger.Fatalf("error [tools]: %v", err)
	}

	geminiClient := services.NewGeminiClient(services.GeminiClientInput{
		APIKey:  strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		Model:   getEnv("GEMINI_MODEL", ""),
		BaseURL: getEnv("GEMINI_API_BASE", ""),
	})

	siteName := getEnv("SITE_NAME", "explore")
	siteURL := getEnv("SITE_URL", "https://explore.mcpx.in")
	primaryColor := getEnv("PRIMARY_COLOR", "#38bdf8")
	_, _ = store.EnsureSiteSettings(stores.SiteSettingsInput{
		SiteName:     siteName,
		SiteURL:      siteURL,
		PrimaryColor: primaryColor,
	})

	contentService := &services.ContentService{Store: store}
	funnelService := &services.FunnelService{Store: store, Logger: logger}
	analysisService := &services.AIAnalysisService{Store: store, AI: geminiClient, Logger: logger}
	judgeService := &services.JudgeService{
		Store:  store,
		Runner: buildJudgeRunner(logger),
		Logger: logger,
	}

	appMode := resolveAppMode(logger)

	if appModeRunsBackgroundJobs(appMode) {
		startBackgroundJobs(backgroundJobsInput{
			Store:         store,
			FunnelService: funnelService,
			JudgeService:  judgeService,
			Logger:        logger,
		})
	}

	if !appModeServesHTTP(appMode) {
		logger.Printf("info [main]: worker mode active")
		waitForShutdownSignal(logger)
		return
	}

	templates, err := routes.LoadTemplates()
	if err != nil {
		logger.Fatalf("error [templates]: %v", err)
	}

	if isProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.LoggerWithWriter(logger.Writer()), gin.Recovery())

	seoHandler := &routes.SEOHandler{Store: store}
	seoHandler.Register(r)

	pageHandler := &routes.PageHandler{Store: store, Content: contentService, Templates: templates}
	pageHandler.Register(r)

	routes.RegisterConfig(r, store)
	routes.RegisterHealth(r)

	r.Static("/assets", "./web/assets")
	r.GET("/privacy", func(c *gin.Context) { c.File("./web/privacy.html") })
	r.GET("/terms", func(c *gin.Context) { c.File("./web/terms.html") })
	r.GET("/cancellation-refunds", func(c *gin.Context) { c.File("./web/cancellation-refunds.html") })
	r.GET("/shipping", func(c *gin.Context) { c.File("./web/shipping.html") })
	r.GET("/contact", func(c *gin.Context) { c.File("./web/contact.html") })

	api := r.Group("/api")
	api.Use(middleware.RateLimiter(1000, time.Minute))
	api.Use(middleware.OptionalAuth(store, logger))

	routes.RegisterAuth(api, store, logger.Printf)

	postsHandler := &routes.PostsHandler{Service: contentService}
	postsHandler.Register(api)

	coursesHandler := &routes.CoursesHandler{Service: contentService}
	coursesHandler.Register(api)

	problemsHandler := &routes.ProblemsHandler{Store: store}
	problemsHandler.Register(api)

	eventsHandler := &routes.EventsHandler{Store: store}
	eventsHandler.Register(api)

	promosHandler := &routes.PromosHandler{Store: store}
	promosHandler.Register(api)

	toolsHandler := &routes.ToolsHandler{
		Store:   store,
		Service: &services.ToolService{Store: store},
		AI:      geminiClient,
	}
	toolsHandler.Register(api)

	paymentsHandler := &routes.PaymentsHandler{Store: store, Plans: plans, Logger: logger}
	paymentsHandler.RegisterPublic(api)
	desktopHandler := &routes.DesktopHandler{Store: store, Logger: logger}
	desktopAPIGlobalLimiter := middleware.GlobalRateLimiter(100, time.Second)
	desktopPublic := api.Group("")
	desktopPublic.Use(desktopAPIGlobalLimiter)
	desktopHandler.RegisterPublic(desktopPublic)

	authed := api.Group("")
	authed.Use(middleware.Auth(store, logger))
	authed.Use(middleware.RateLimiter(300, time.Minute))

	meHandler := &routes.MeHandler{Store: store}
	meHandler.Register(authed)
	paymentsHandler.RegisterAuthed(authed)
	desktopAuthed := authed.Group("")
	desktopAuthed.Use(desktopAPIGlobalLimiter)
	desktopHandler.RegisterAuthed(desktopAuthed)

	submissionsHandler := &routes.SubmissionsHandler{Store: store}
	submissionsHandler.Register(authed)

	analysisHandler := &routes.AIAnalysisHandler{Service: analysisService}
	analysisHandler.Register(authed)

	admin := api.Group("/admin")
	admin.Use(middleware.Auth(store, logger))
	admin.Use(middleware.RequireAdminRole())

	adminPosts := &routes.AdminPostsHandler{Store: store}
	adminPosts.Register(admin)

	adminCourses := &routes.AdminCoursesHandler{Store: store}
	adminCourses.Register(admin)

	adminProblems := &routes.AdminProblemsHandler{Store: store}
	adminProblems.Register(admin)

	adminProblemLists := &routes.AdminProblemListsHandler{Store: store}
	adminProblemLists.Register(admin)

	adminProblemDatasets := &routes.AdminProblemDatasetsHandler{Store: store}
	adminProblemDatasets.Register(admin)

	adminProblemSolutions := &routes.AdminProblemSolutionsHandler{Store: store}
	adminProblemSolutions.Register(admin)

	adminFunnel := &routes.AdminFunnelHandler{Store: store}
	adminFunnel.Register(admin)

	adminPromos := &routes.AdminPromosHandler{Store: store}
	adminPromos.Register(admin)

	adminAnalytics := &routes.AdminAnalyticsHandler{Store: store}
	adminAnalytics.Register(admin)

	adminPostAnalytics := &routes.AdminPostAnalyticsHandler{Store: store}
	adminPostAnalytics.Register(admin)

	adminTools := &routes.AdminToolsHandler{Store: store}
	adminTools.Register(admin)

	adminUsers := &routes.AdminUsersHandler{Store: store}
	adminUsers.Register(admin)

	adminSettings := &routes.AdminSettingsHandler{Store: store}
	adminSettings.Register(admin)

	port := getEnv("APP_PORT", "8080")
	logger.Printf("info [main]: starting server mode=%s on :%s", appMode, port)
	if err := r.Run(":" + port); err != nil {
		logger.Fatalf("error [main.Run]: %v", err)
	}
}

type backgroundJobsInput struct {
	Store         *stores.Store
	FunnelService *services.FunnelService
	JudgeService  *services.JudgeService
	Logger        *log.Logger
}

func startBackgroundJobs(input backgroundJobsInput) {
	startFunnelRecalcJob(funnelJobInput{Service: input.FunnelService, Logger: input.Logger})
	startEntitlementExpiryJob(entitlementJobInput{Store: input.Store, Logger: input.Logger})
	startPostAnalyticsJob(analyticsJobInput{Store: input.Store, Logger: input.Logger})
	startToolAnalyticsJob(analyticsJobInput{Store: input.Store, Logger: input.Logger})
	startJudgeWorker(judgeWorkerInput{
		Service:     input.JudgeService,
		Logger:      input.Logger,
		Interval:    judgePollInterval(),
		Concurrency: judgeWorkerConcurrency(),
	})
}

type funnelJobInput struct {
	Service *services.FunnelService
	Logger  *log.Logger
}

func startFunnelRecalcJob(input funnelJobInput) {
	if input.Service == nil || input.Logger == nil {
		return
	}

	go func() {
		interval := 60 * time.Minute
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			now := time.Now().UTC()
			if err := input.Service.RecalculateAll(now); err != nil {
				input.Logger.Printf("error [funnel]: %v", err)
			}
			<-ticker.C
		}
	}()
}

type entitlementJobInput struct {
	Store  *stores.Store
	Logger *log.Logger
}

func startEntitlementExpiryJob(input entitlementJobInput) {
	if input.Store == nil || input.Logger == nil {
		return
	}

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			now := time.Now().UTC()
			if err := input.Store.ExpireEntitlements(now); err != nil {
				input.Logger.Printf("error [entitlements]: %v", err)
			}
			<-ticker.C
		}
	}()
}

type analyticsJobInput struct {
	Store  *stores.Store
	Logger *log.Logger
}

func startPostAnalyticsJob(input analyticsJobInput) {
	if input.Store == nil || input.Logger == nil {
		return
	}

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			now := time.Now().UTC()
			if err := runPostAnalyticsRollup(input.Store, now); err != nil {
				input.Logger.Printf("error [analytics]: %v", err)
			}
			<-ticker.C
		}
	}()
}

func startToolAnalyticsJob(input analyticsJobInput) {
	if input.Store == nil || input.Logger == nil {
		return
	}

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			now := time.Now().UTC()
			if err := runToolAnalyticsRollup(input.Store, now); err != nil {
				input.Logger.Printf("error [tool_analytics]: %v", err)
			}
			<-ticker.C
		}
	}()
}

type judgeWorkerInput struct {
	Service     *services.JudgeService
	Logger      *log.Logger
	Interval    time.Duration
	Concurrency int
}

func startJudgeWorker(input judgeWorkerInput) {
	if input.Service == nil || input.Logger == nil {
		return
	}

	workerCount := normalizeJudgeWorkerConcurrency(input.Concurrency)
	interval := normalizeJudgePollInterval(input.Interval)

	input.Logger.Printf("info [judge]: starting worker loops=%d", workerCount)

	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		runJudgeWorkerLoop(judgeWorkerLoopInput{
			Service:  input.Service,
			Logger:   input.Logger,
			Interval: interval,
		})
	}
}

type judgeWorkerLoopInput struct {
	Service  *services.JudgeService
	Logger   *log.Logger
	Interval time.Duration
}

func runJudgeWorkerLoop(input judgeWorkerLoopInput) {
	go func() {
		ticker := time.NewTicker(input.Interval)
		defer ticker.Stop()

		for {
			processed, err := input.Service.RunOnce()
			if err != nil {
				input.Logger.Printf("error [judge]: %v", err)
				<-ticker.C
				continue
			}

			if processed {
				continue
			}

			<-ticker.C
		}
	}()
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env not found, using system env")
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func judgePollInterval() time.Duration {
	raw := strings.TrimSpace(getEnv("JUDGE_POLL_INTERVAL_MS", "1000"))
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return 2 * time.Second
	}
	if ms < 200 {
		ms = 200
	}
	return time.Duration(ms) * time.Millisecond
}

func judgeWorkerConcurrency() int {
	raw := strings.TrimSpace(getEnv("JUDGE_WORKER_CONCURRENCY", "1"))
	count, err := strconv.Atoi(raw)
	if err != nil || count <= 0 {
		return 1
	}
	return count
}

func normalizeJudgeWorkerConcurrency(count int) int {
	if count <= 0 {
		return 1
	}
	if count > 16 {
		return 16
	}
	return count
}

func normalizeJudgePollInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 2 * time.Second
	}
	return interval
}

func resolveAppMode(logger *log.Logger) string {
	rawMode := strings.ToLower(strings.TrimSpace(getEnv("APP_MODE", "all")))

	switch rawMode {
	case "", "all":
		return "all"
	case "web", "worker":
		return rawMode
	default:
		if logger != nil {
			logger.Printf("warn [main]: invalid APP_MODE=%q, defaulting to all", rawMode)
		}
		return "all"
	}
}

func appModeRunsBackgroundJobs(appMode string) bool {
	return appMode == "all" || appMode == "worker"
}

func appModeServesHTTP(appMode string) bool {
	return appMode == "all" || appMode == "web"
}

func waitForShutdownSignal(logger *log.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if logger != nil {
		logger.Printf("info [main]: waiting for shutdown signal")
	}

	<-ctx.Done()

	if logger != nil {
		logger.Printf("info [main]: shutdown signal received")
	}
}

func buildJudgeRunner(logger *log.Logger) services.Runner {
	runnerType := strings.ToLower(strings.TrimSpace(getEnv("JUDGE_RUNNER", "docker")))
	workDir := getEnv("JUDGE_WORKDIR", "")

	if runnerType == "k8s" {
		runner := &services.K8sJobRunner{
			Namespace:      resolveJudgeNamespace(),
			Image:          getEnv("JUDGE_K8S_IMAGE", ""),
			ServiceAccount: getEnv("JUDGE_K8S_SERVICE_ACCOUNT", ""),
			CPULimit:       normalizeCPUQuantity(getEnv("JUDGE_K8S_CPU", "1")),
			PollInterval:   parseDurationMs(getEnv("JUDGE_K8S_POLL_INTERVAL_MS", "350")),
			JobTTLSeconds:  int32(parsePositiveInt(getEnv("JUDGE_K8S_JOB_TTL_SECONDS", "120"), 120)),
			KubectlBin:     getEnv("JUDGE_K8S_KUBECTL_BIN", "kubectl"),
			KubeconfigPath: getEnv("JUDGE_K8S_KUBECONFIG", ""),
		}
		if logger != nil {
			logger.Printf("info [judge]: runner=k8s")
		}
		return runner
	}

	if runnerType == "" || runnerType == "docker" {
		runner := &services.DockerRunner{
			WorkDir:      workDir,
			DockerBin:    getEnv("JUDGE_DOCKER_BIN", "docker"),
			Image:        getEnv("JUDGE_DOCKER_IMAGE", ""),
			GoImage:      getEnv("JUDGE_DOCKER_IMAGE_GO", ""),
			CImage:       getEnv("JUDGE_DOCKER_IMAGE_C", ""),
			CppImage:     getEnv("JUDGE_DOCKER_IMAGE_CPP", ""),
			JavaImage:    getEnv("JUDGE_DOCKER_IMAGE_JAVA", ""),
			CPULimit:     normalizeCPUQuantity(getEnv("JUDGE_DOCKER_CPUS", "1")),
			TmpfsSizeMb:  parsePositiveInt(getEnv("JUDGE_DOCKER_TMPFS_MB", "64"), 64),
			PidsLimit:    parsePositiveInt(getEnv("JUDGE_DOCKER_PIDS_LIMIT", "128"), 128),
			CompileLimit: parseDurationMs(getEnv("JUDGE_DOCKER_COMPILE_TIMEOUT_MS", "0")),
			CompileMemMb: parsePositiveInt(getEnv("JUDGE_DOCKER_COMPILE_MEMORY_MB", "512"), 512),
		}
		if logger != nil {
			logger.Printf("info [judge]: runner=docker")
		}
		return runner
	}

	if logger != nil {
		logger.Printf("warn [judge]: runner=local (sandbox disabled)")
	}
	return &services.LocalRunner{WorkDir: workDir}
}

func normalizeCPUQuantity(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "1"
	}

	if strings.HasSuffix(trimmed, "m") {
		number := strings.TrimSuffix(trimmed, "m")
		milliValue, err := strconv.Atoi(number)
		if err != nil || milliValue <= 0 {
			return "1"
		}
		return strconv.Itoa(milliValue) + "m"
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || value <= 0 {
		return "1"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func resolveJudgeNamespace() string {
	namespace := strings.TrimSpace(getEnv("JUDGE_K8S_NAMESPACE", ""))
	if namespace != "" {
		return namespace
	}

	return strings.TrimSpace(getEnv("POD_NAMESPACE", ""))
}

func parsePositiveInt(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseDurationMs(raw string) time.Duration {
	ms := parsePositiveInt(raw, 0)
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func isProduction() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return v == "prod" || v == "production"
}

func runPostAnalyticsRollup(store *stores.Store, now time.Time) error {
	cfg, err := store.GetFunnelConfig()
	if err != nil {
		return err
	}
	retention := analyticsRetentionDays(cfg.ScoringWindowDays)
	return store.RunPostAnalyticsRollup(now, retention)
}

func runToolAnalyticsRollup(store *stores.Store, now time.Time) error {
	cfg, err := store.GetFunnelConfig()
	if err != nil {
		return err
	}
	retention := analyticsRetentionDays(cfg.ScoringWindowDays)
	return store.RunToolAnalyticsRollup(now, retention)
}

func analyticsRetentionDays(scoringWindow int) int {
	if scoringWindow <= 0 {
		scoringWindow = 14
	}
	if scoringWindow < 14 {
		scoringWindow = 14
	}
	return scoringWindow + 7
}

func initLogger() *log.Logger {
	path := getEnv("LOG_FILE", "logs/app.log")
	file := mustOpenLog(path)
	mw := io.MultiWriter(os.Stdout, file)
	logger := log.New(mw, "", log.LstdFlags|log.Lshortfile)
	logger.Printf("info [logger]: writing logs to %s", path)
	return logger
}

func mustOpenLog(path string) *os.File {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("failed to create log dir: %v", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	return f
}
