package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/api"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/sync"
	"github.com/streamspace/streamspace/api/internal/tracker"
)

func main() {
	// Configuration from environment
	port := getEnv("API_PORT", "8000")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "streamspace")
	dbPassword := getEnv("DB_PASSWORD", "streamspace")
	dbName := getEnv("DB_NAME", "streamspace")

	log.Println("Starting StreamSpace API Server...")

	// Initialize database
	log.Println("Connecting to database...")
	database, err := db.NewDatabase(db.Config{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	log.Println("Running database migrations...")
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Kubernetes client
	log.Println("Initializing Kubernetes client...")
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Initialize connection tracker
	log.Println("Starting connection tracker...")
	connTracker := tracker.NewConnectionTracker(database, k8sClient)
	go connTracker.Start()
	defer connTracker.Stop()

	// Initialize sync service
	log.Println("Initializing repository sync service...")
	syncService, err := sync.NewSyncService(database)
	if err != nil {
		log.Fatalf("Failed to initialize sync service: %v", err)
	}

	// Start scheduled sync (every 1 hour by default)
	syncInterval := getEnv("SYNC_INTERVAL", "1h")
	interval, err := time.ParseDuration(syncInterval)
	if err != nil {
		log.Printf("Invalid SYNC_INTERVAL, using default 1h: %v", err)
		interval = 1 * time.Hour
	}

	ctx, cancelSync := context.WithCancel(context.Background())
	defer cancelSync()

	go syncService.StartScheduledSync(ctx, interval)

	// Create Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// Initialize API handlers
	apiHandler := api.NewHandler(database, k8sClient, connTracker, syncService)

	// Setup routes
	setupRoutes(router, apiHandler)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("API Server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func setupRoutes(router *gin.Engine, h *api.Handler) {
	// Health check
	router.GET("/health", h.Health)
	router.GET("/version", h.Version)

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Sessions
		sessions := v1.Group("/sessions")
		{
			sessions.GET("", h.ListSessions)
			sessions.POST("", h.CreateSession)
			sessions.GET("/:id", h.GetSession)
			sessions.PATCH("/:id", h.UpdateSession)
			sessions.DELETE("/:id", h.DeleteSession)
			sessions.GET("/:id/connect", h.ConnectSession)
			sessions.POST("/:id/disconnect", h.DisconnectSession)
			sessions.POST("/:id/heartbeat", h.SessionHeartbeat)
		}

		// Templates
		templates := v1.Group("/templates")
		{
			templates.GET("", h.ListTemplates)
			templates.POST("", h.CreateTemplate)
			templates.GET("/:id", h.GetTemplate)
			templates.PATCH("/:id", h.UpdateTemplate)
			templates.DELETE("/:id", h.DeleteTemplate)
		}

		// Catalog
		catalog := v1.Group("/catalog")
		{
			catalog.GET("/repositories", h.ListRepositories)
			catalog.POST("/repositories", h.AddRepository)
			catalog.DELETE("/repositories/:id", h.RemoveRepository)
			catalog.POST("/sync", h.SyncCatalog)
			catalog.GET("/templates", h.BrowseCatalog)
			catalog.POST("/install", h.InstallTemplate)
		}

		// Cluster management
		cluster := v1.Group("/cluster")
		{
			cluster.GET("/nodes", h.ListNodes)
			cluster.GET("/pods", h.ListPods)
			cluster.GET("/deployments", h.ListDeployments)
			cluster.GET("/services", h.ListServices)
			cluster.GET("/namespaces", h.ListNamespaces)
			cluster.POST("/resources", h.CreateResource)
			cluster.PATCH("/resources", h.UpdateResource)
			cluster.DELETE("/resources", h.DeleteResource)
			cluster.GET("/pods/:namespace/:name/logs", h.GetPodLogs)
		}

		// Configuration
		config := v1.Group("/config")
		{
			config.GET("", h.GetConfig)
			config.PATCH("", h.UpdateConfig)
		}

		// Users (if not using OIDC only)
		users := v1.Group("/users")
		{
			users.GET("", h.ListUsers)
			users.POST("", h.CreateUser)
			users.GET("/me", h.GetCurrentUser)
			users.GET("/:id", h.GetUser)
			users.PATCH("/:id", h.UpdateUser)
			users.GET("/:id/sessions", h.GetUserSessions)
		}

		// Metrics
		v1.GET("/metrics", h.GetMetrics)
	}

	// WebSocket endpoints
	ws := router.Group("/api/v1/ws")
	{
		ws.GET("/sessions", h.SessionsWebSocket)
		ws.GET("/cluster", h.ClusterWebSocket)
		ws.GET("/logs/:namespace/:pod", h.LogsWebSocket)
	}

	// Webhook endpoints (no auth required)
	webhooks := router.Group("/webhooks")
	{
		webhooks.POST("/repository/sync", h.WebhookRepositorySync)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
