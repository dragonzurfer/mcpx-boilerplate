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

	appKey := resolveAppKey()
	billingService := &services.BillingService{
		Store: store,
		Plans: plans,
	}
	if isProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.LoggerWithWriter(logger.Writer()), gin.Recovery())

	ipFilter := middleware.NewIPFilter(store, appKey)
	r.Use(ipFilter.Handler())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	routes.RegisterConfig(r, plans)

	api := r.Group("/api")
	api.Use(middleware.OptionalAPIKey())
	api.Use(middleware.RateLimiter(2000, time.Minute))

	routes.RegisterAuth(api, store, logger.Printf)

	billingHandler := &routes.BillingHandler{
		Store:      store,
		Logger:     logger,
		Plans:      plans,
		Service:    billingService,
		ProjectKey: appKey,
	}
	billingHandler.RegisterPublic(api)

	authed := api.Group("")
	authed.Use(middleware.Auth(store, logger))
	authed.Use(middleware.RateLimiter(300, time.Minute))
	billingHandler.RegisterAuthed(authed)

	metered := authed.Group("")
	metered.Use(middleware.Metered(billingService, appKey, middleware.MeterConfig{Metric: "api_calls", Cost: 1, RequireSubject: true}))
	routes.RegisterExample(metered)

	admin := api.Group("/admin")
	admin.Use(middleware.RequireAdminToken())
	adminHandler := &routes.AdminHandler{Store: store, Logger: logger, ProjectKey: appKey}
	adminHandler.Register(admin)

	r.Static("/assets", "./web/assets")
	r.GET("/privacy", func(c *gin.Context) { c.File("./web/privacy.html") })
	r.GET("/terms", func(c *gin.Context) { c.File("./web/terms.html") })
	r.GET("/cancellation-refunds", func(c *gin.Context) { c.File("./web/cancellation-refunds.html") })
	r.GET("/shipping", func(c *gin.Context) { c.File("./web/shipping.html") })
	r.GET("/contact", func(c *gin.Context) { c.File("./web/contact.html") })
	r.GET("/", func(c *gin.Context) { c.File("./web/index.html") })
	r.NoRoute(func(c *gin.Context) { c.File("./web/index.html") })

	port := getEnv("APP_PORT", "8080")
	logger.Printf("info [main]: starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		logger.Fatalf("error [main.Run]: %v", err)
	}
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

func resolveAppKey() string {
	if v := strings.TrimSpace(os.Getenv("APP_KEY")); v != "" {
		return v
	}
	name := strings.ToLower(strings.TrimSpace(os.Getenv("APP_NAME")))
	name = strings.ReplaceAll(name, " ", "-")
	if name != "" {
		return name
	}
	return "mcpx-app"
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
