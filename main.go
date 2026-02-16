package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
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

	authed := api.Group("")
	authed.Use(middleware.Auth(store, logger))
	authed.Use(middleware.RateLimiter(300, time.Minute))

	meHandler := &routes.MeHandler{Store: store}
	meHandler.Register(authed)
	paymentsHandler.RegisterAuthed(authed)

	admin := api.Group("/admin")
	admin.Use(middleware.Auth(store, logger))
	admin.Use(middleware.RequireAdminRole())

	adminPosts := &routes.AdminPostsHandler{Store: store}
	adminPosts.Register(admin)

	adminCourses := &routes.AdminCoursesHandler{Store: store}
	adminCourses.Register(admin)

	adminProblems := &routes.AdminProblemsHandler{Store: store}
	adminProblems.Register(admin)

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

	startBackgroundJobs(store, funnelService, logger)

	port := getEnv("APP_PORT", "8080")
	logger.Printf("info [main]: starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		logger.Fatalf("error [main.Run]: %v", err)
	}
}

func startBackgroundJobs(store *stores.Store, funnelService *services.FunnelService, logger *log.Logger) {
	go func() {
		interval := 60 * time.Minute
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			now := time.Now().UTC()
			if err := funnelService.RecalculateAll(now); err != nil {
				logger.Printf("error [funnel]: %v", err)
			}
			<-ticker.C
		}
	}()

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			now := time.Now().UTC()
			if err := store.ExpireEntitlements(now); err != nil {
				logger.Printf("error [entitlements]: %v", err)
			}
			<-ticker.C
		}
	}()

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			now := time.Now().UTC()
			if err := runPostAnalyticsRollup(store, now); err != nil {
				logger.Printf("error [analytics]: %v", err)
			}
			<-ticker.C
		}
	}()

	go func() {
		interval := 24 * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			now := time.Now().UTC()
			if err := runToolAnalyticsRollup(store, now); err != nil {
				logger.Printf("error [tool_analytics]: %v", err)
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
