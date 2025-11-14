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
	"github.com/streamspace/streamspace/api/internal/activity"
	"github.com/streamspace/streamspace/api/internal/api"
	"github.com/streamspace/streamspace/api/internal/auth"
	"github.com/streamspace/streamspace/api/internal/cache"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/handlers"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/middleware"
	"github.com/streamspace/streamspace/api/internal/quota"
	"github.com/streamspace/streamspace/api/internal/sync"
	"github.com/streamspace/streamspace/api/internal/tracker"
	"github.com/streamspace/streamspace/api/internal/websocket"
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

	// Initialize Redis cache (optional)
	log.Println("Initializing Redis cache...")
	cacheEnabled := getEnv("CACHE_ENABLED", "false") == "true"
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisCache, err := cache.NewCache(cache.Config{
		Host:     redisHost,
		Port:     redisPort,
		Password: redisPassword,
		DB:       0,
		Enabled:  cacheEnabled,
	})
	if err != nil {
		log.Printf("Failed to initialize Redis cache (continuing without caching): %v", err)
		// Create disabled cache instance
		redisCache, _ = cache.NewCache(cache.Config{Enabled: false})
	} else if cacheEnabled {
		log.Println("Redis cache enabled and connected")
	} else {
		log.Println("Redis cache disabled")
	}
	defer redisCache.Close()

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

	// Initialize WebSocket manager
	log.Println("Initializing WebSocket manager...")
	wsManager := websocket.NewManager(database, k8sClient)
	wsManager.Start()

	// Initialize activity tracker
	log.Println("Initializing activity tracker...")
	activityTracker := activity.NewTracker(k8sClient)

	// Start idle session monitor (check every 1 minute)
	idleCheckInterval := getEnv("IDLE_CHECK_INTERVAL", "1m")
	idleInterval, err := time.ParseDuration(idleCheckInterval)
	if err != nil {
		log.Printf("Invalid IDLE_CHECK_INTERVAL, using default 1m: %v", err)
		idleInterval = 1 * time.Minute
	}

	idleCtx, cancelIdle := context.WithCancel(context.Background())
	defer cancelIdle()

	go activityTracker.StartIdleMonitor(idleCtx, "streamspace", idleInterval)

	// Create Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// Add gzip compression (exclude WebSocket endpoints)
	router.Use(middleware.GzipWithExclusions(
		middleware.BestSpeed, // Use best speed for balance of compression vs CPU
		[]string{"/api/v1/ws/"}, // Exclude WebSocket paths
	))

	// Add cache control headers to all responses
	router.Use(cache.CacheControl(5 * time.Minute))

	// Initialize database repositories
	userDB := db.NewUserDB(database.DB())
	groupDB := db.NewGroupDB(database.DB())

	// Initialize quota enforcer
	quotaEnforcer := quota.NewEnforcer(userDB, groupDB)

	// Initialize JWT manager for authentication
	jwtConfig := &auth.JWTConfig{
		SecretKey:     getEnv("JWT_SECRET", "streamspace-secret-change-in-production"),
		Issuer:        "streamspace-api",
		TokenDuration: 24 * time.Hour,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Initialize API handlers
	apiHandler := api.NewHandler(database, k8sClient, connTracker, syncService, wsManager, quotaEnforcer)
	userHandler := handlers.NewUserHandler(userDB)
	groupHandler := handlers.NewGroupHandler(groupDB, userDB)
	authHandler := auth.NewAuthHandler(userDB, jwtManager)
	activityHandler := handlers.NewActivityHandler(k8sClient, activityTracker)
	catalogHandler := handlers.NewCatalogHandler(database)
	sharingHandler := handlers.NewSharingHandler(database)

	// Setup routes
	setupRoutes(router, apiHandler, userHandler, groupHandler, authHandler, activityHandler, catalogHandler, sharingHandler, jwtManager, userDB, redisCache)

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

func setupRoutes(router *gin.Engine, h *api.Handler, userHandler *handlers.UserHandler, groupHandler *handlers.GroupHandler, authHandler *auth.AuthHandler, activityHandler *handlers.ActivityHandler, catalogHandler *handlers.CatalogHandler, sharingHandler *handlers.SharingHandler, jwtManager *auth.JWTManager, userDB *db.UserDB, redisCache *cache.Cache) {
	// Health check (public)
	router.GET("/health", h.Health)
	router.GET("/version", h.Version)

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (public)
		authHandler.RegisterRoutes(v1)
		// Sessions
		sessions := v1.Group("/sessions")
		{
			// Cache session lists for 30 seconds (frequently changing)
			sessions.GET("", cache.CacheMiddleware(redisCache, 30*time.Second), h.ListSessions)
			sessions.POST("", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.CreateSession)
			sessions.GET("/by-tags", cache.CacheMiddleware(redisCache, 30*time.Second), h.ListSessionsByTags)
			sessions.GET("/:id", cache.CacheMiddleware(redisCache, 30*time.Second), h.GetSession)
			sessions.PATCH("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.UpdateSession)
			sessions.DELETE("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.DeleteSession)
			sessions.PATCH("/:id/tags", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.UpdateSessionTags)
			sessions.GET("/:id/connect", h.ConnectSession)
			sessions.POST("/:id/disconnect", h.DisconnectSession)
			sessions.POST("/:id/heartbeat", h.SessionHeartbeat)
		}

		// Templates
		templates := v1.Group("/templates")
		{
			// Cache template lists for 5 minutes (rarely changing)
			templates.GET("", cache.CacheMiddleware(redisCache, 5*time.Minute), h.ListTemplates)
			templates.POST("", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.CreateTemplate)
			templates.GET("/:id", cache.CacheMiddleware(redisCache, 5*time.Minute), h.GetTemplate)
			templates.PATCH("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.UpdateTemplate)
			templates.DELETE("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.DeleteTemplate)
		}

		// Catalog
		catalog := v1.Group("/catalog")
		{
			// Cache catalog data for 10 minutes (changes on sync)
			catalog.GET("/repositories", cache.CacheMiddleware(redisCache, 10*time.Minute), h.ListRepositories)
			catalog.POST("/repositories", h.AddRepository)
			catalog.DELETE("/repositories/:id", h.RemoveRepository)
			catalog.POST("/sync", h.SyncCatalog)
			catalog.GET("/templates", cache.CacheMiddleware(redisCache, 10*time.Minute), h.BrowseCatalog)
			catalog.POST("/install", h.InstallTemplate)
		}

		// Cluster management
		cluster := v1.Group("/cluster")
		{
			// Cache cluster data for 1 minute (can change frequently)
			cluster.GET("/nodes", cache.CacheMiddleware(redisCache, 1*time.Minute), h.ListNodes)
			cluster.GET("/pods", cache.CacheMiddleware(redisCache, 30*time.Second), h.ListPods)
			cluster.GET("/deployments", cache.CacheMiddleware(redisCache, 30*time.Second), h.ListDeployments)
			cluster.GET("/services", cache.CacheMiddleware(redisCache, 1*time.Minute), h.ListServices)
			cluster.GET("/namespaces", cache.CacheMiddleware(redisCache, 2*time.Minute), h.ListNamespaces)
			cluster.POST("/resources", h.CreateResource)
			cluster.PATCH("/resources", h.UpdateResource)
			cluster.DELETE("/resources", h.DeleteResource)
			cluster.GET("/pods/:namespace/:name/logs", h.GetPodLogs)
		}

		// Configuration
		config := v1.Group("/config")
		{
			// Cache configuration for 5 minutes (rarely changes)
			config.GET("", cache.CacheMiddleware(redisCache, 5*time.Minute), h.GetConfig)
			config.PATCH("", cache.InvalidateCacheMiddleware(redisCache, cache.ConfigKey("*")), h.UpdateConfig)
		}

		// User management - using dedicated handler
		userHandler.RegisterRoutes(v1)

		// Group management - using dedicated handler
		groupHandler.RegisterRoutes(v1)

		// Activity tracking - using dedicated handler
		activityHandler.RegisterRoutes(v1)

		// Enhanced catalog - using dedicated handler
		catalogHandler.RegisterRoutes(v1)

		// Session sharing and collaboration - using dedicated handler
		sharingHandler.RegisterRoutes(v1)

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
