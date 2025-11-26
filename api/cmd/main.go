package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/streamspace-dev/streamspace/api/internal/activity"
	"github.com/streamspace-dev/streamspace/api/internal/api"
	"github.com/streamspace-dev/streamspace/api/internal/auth"
	"github.com/streamspace-dev/streamspace/api/internal/cache"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/events"
	"github.com/streamspace-dev/streamspace/api/internal/handlers"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
	"github.com/streamspace-dev/streamspace/api/internal/middleware"
	"github.com/streamspace-dev/streamspace/api/internal/quota"
	"github.com/streamspace-dev/streamspace/api/internal/services"
	"github.com/streamspace-dev/streamspace/api/internal/sync"
	"github.com/streamspace-dev/streamspace/api/internal/tracker"
	internalWebsocket "github.com/streamspace-dev/streamspace/api/internal/websocket"
)

func main() {
	// Configuration from environment
	port := getEnv("API_PORT", "8000")
	tlsCertFile := os.Getenv("TLS_CERT_FILE")       // Path to TLS certificate file (PEM format)
	tlsKeyFile := os.Getenv("TLS_KEY_FILE")         // Path to TLS private key file (PEM format)
	agentCACertFile := os.Getenv("AGENT_CA_CERT_FILE") // Path to CA cert for validating agent client certs (enables mTLS)
	requireClientCert := getEnv("REQUIRE_CLIENT_CERT", "false") == "true" // Require client cert (only with mTLS)
	rateLimitEnabled := getEnv("RATE_LIMIT_ENABLED", "true") == "true" // Enable rate limiting (default: true)
	rateLimitRPM := getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 60) // Requests per minute (default: 60)
	// rateLimitBurst := getEnvInt("RATE_LIMIT_BURST", 10) // Burst capacity (default: 10) - reserved for future use
	auditLogEnabled := getEnv("AUDIT_LOG_ENABLED", "true") == "true" // Enable audit logging (default: true)
	auditLogBodies := getEnv("AUDIT_LOG_BODIES", "false") == "true" // Log request bodies (default: false for privacy)
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "streamspace")
	dbPassword := getEnv("DB_PASSWORD", "streamspace")
	dbName := getEnv("DB_NAME", "streamspace")
	dbSSLMode := getEnv("DB_SSL_MODE", "disable") // SECURITY: Should be "require" in production
	pluginDir := getEnv("PLUGIN_DIR", "./plugins")

	log.Println("Starting StreamSpace API Server...")

	// Initialize database
	log.Println("Connecting to database...")
	database, err := db.NewDatabase(db.Config{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
		SSLMode:  dbSSLMode,
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

	// Initialize stub event publisher (NATS removed - WebSocket used instead)
	log.Println("Initializing event publisher (stub - agents use WebSocket)...")
	eventPublisher, err := events.NewPublisher(events.Config{})
	if err != nil {
		log.Fatalf("Failed to initialize event publisher: %v", err)
	}
	defer eventPublisher.Close()

	// Get platform from environment (for multi-platform support)
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		platform = events.PlatformKubernetes // Default platform
	}

	// Initialize connection tracker
	log.Println("Starting connection tracker...")
	connTracker := tracker.NewConnectionTracker(database, k8sClient, eventPublisher, platform)
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
	wsManager := internalWebsocket.NewManager(database, k8sClient)
	wsManager.Start()

	// Initialize Redis client for AgentHub multi-pod support (optional)
	// This is separate from the cache Redis client and enables agent state sharing across API replicas
	var agentHubRedis *redis.Client
	agentHubRedisEnabled := getEnv("AGENTHUB_REDIS_ENABLED", "false") == "true"

	if agentHubRedisEnabled {
		log.Println("Initializing Redis for AgentHub multi-pod support...")
		agentHubRedisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)

		agentHubRedis = redis.NewClient(&redis.Options{
			Addr:     agentHubRedisAddr,
			Password: redisPassword,
			DB:       1, // Use DB 1 for AgentHub (DB 0 is for cache)
		})

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := agentHubRedis.Ping(ctx).Err(); err != nil {
			log.Printf("WARNING: Failed to connect to Redis for AgentHub (continuing in single-pod mode): %v", err)
			agentHubRedis.Close()
			agentHubRedis = nil
		} else {
			log.Println("AgentHub Redis connected - multi-pod support enabled")
		}
	} else {
		log.Println("AgentHub Redis disabled (single-pod mode) - set AGENTHUB_REDIS_ENABLED=true for multi-pod support")
	}

	// Initialize Agent Hub for v2.0 multi-platform architecture
	log.Println("Initializing Agent Hub...")
	var agentHub *internalWebsocket.AgentHub
	if agentHubRedis != nil {
		agentHub = internalWebsocket.NewAgentHubWithRedis(database, agentHubRedis)
		log.Println("AgentHub initialized with Redis (multi-pod mode)")
	} else {
		agentHub = internalWebsocket.NewAgentHub(database)
		log.Println("AgentHub initialized without Redis (single-pod mode)")
	}
	go agentHub.Run()

	// Initialize Command Dispatcher for agent commands
	log.Println("Initializing Command Dispatcher...")
	commandDispatcher := services.NewCommandDispatcher(database, agentHub)
	go commandDispatcher.Start()

	// Queue any pending commands on startup
	if err := commandDispatcher.DispatchPendingCommands(); err != nil {
		log.Printf("Warning: Failed to dispatch pending commands: %v", err)
	}

	// Initialize activity tracker
	log.Println("Initializing activity tracker...")
	activityTracker := activity.NewTracker(k8sClient, eventPublisher, platform)

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

	// Add request ID middleware for distributed tracing
	router.Use(middleware.RequestID())

	// Add recovery middleware (must be early in chain)
	router.Use(gin.Recovery())

	// Add structured logging with request IDs
	loggerConfig := middleware.DefaultStructuredLoggerConfig()
	router.Use(middleware.StructuredLoggerWithConfigFunc(loggerConfig))

	// SECURITY: Add request timeout to prevent slow loris attacks
	timeoutConfig := middleware.DefaultTimeoutConfig()
	router.Use(middleware.Timeout(timeoutConfig))

	// SECURITY: Restrict HTTP methods to prevent abuse
	router.Use(middleware.AllowedHTTPMethods())

	router.Use(corsMiddleware())

	// SECURITY: Add security headers (HSTS, CSP, X-Frame-Options, etc.)
	router.Use(middleware.SecurityHeaders())

	// SECURITY: Add input validation and sanitization
	inputValidator := middleware.NewInputValidator()
	router.Use(inputValidator.Middleware())
	router.Use(inputValidator.SanitizeJSONMiddleware())

	// SECURITY: Add request size limits to prevent large payload attacks
	// Maximum 10MB for general requests
	router.Use(middleware.RequestSizeLimiter(10 * 1024 * 1024))

	// SECURITY: Add audit logging for all requests (configurable)
	if auditLogEnabled {
		auditLogger := middleware.NewAuditLogger(database, auditLogBodies)
		router.Use(auditLogger.Middleware())
		log.Printf("Audit logging ENABLED (bodies: %v)", auditLogBodies)
	} else {
		log.Println("Audit logging DISABLED (not recommended for production)")
	}

	// Add gzip compression (exclude WebSocket, auth, and metrics endpoints)
	router.Use(middleware.GzipWithExclusions(
		middleware.BestSpeed, // Use best speed for balance of compression vs CPU
		[]string{
			"/api/v1/ws/",     // Exclude WebSocket paths
			"/api/v1/auth/",   // Exclude auth endpoints (setup, login, etc.)
			"/api/v1/metrics", // Exclude metrics (browser handles decompression inconsistently)
		},
	))

	// Add cache control headers to all responses
	router.Use(cache.CacheControl(5 * time.Minute))

	// Initialize database repositories
	userDB := db.NewUserDB(database.DB())
	groupDB := db.NewGroupDB(database.DB())

	// Initialize quota enforcer
	quotaEnforcer := quota.NewEnforcer(userDB, groupDB)

	// Initialize JWT manager for authentication
	// SECURITY: JWT_SECRET must be set in production - no fallback allowed
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("SECURITY ERROR: JWT_SECRET environment variable must be set. Generate with: openssl rand -base64 32")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("SECURITY ERROR: JWT_SECRET must be at least 32 characters long for security")
	}

	jwtConfig := &auth.JWTConfig{
		SecretKey:     jwtSecret,
		Issuer:        "streamspace-api",
		TokenDuration: 24 * time.Hour,
	}
	// Use session-aware JWT manager for server-side session tracking
	// This enables proper logout, session invalidation, and forced re-login on restart
	jwtManager := auth.NewJWTManagerWithSessions(jwtConfig, redisCache)

	// Clear all sessions on startup to force users to re-login
	// This is a security feature that ensures tokens from previous server runs are invalid
	if redisCache.IsEnabled() {
		log.Println("Clearing existing sessions (forcing re-login)...")
		clearCtx, clearCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := jwtManager.ClearAllSessions(clearCtx); err != nil {
			log.Printf("Warning: Failed to clear sessions: %v", err)
		} else {
			log.Println("Sessions cleared - users will need to re-login")
		}
		clearCancel()
	}

	// Initialize SAML authentication (optional)
	var samlAuth *auth.SAMLAuthenticator
	samlEnabled := os.Getenv("SAML_ENABLED")
	if samlEnabled == "true" {
		log.Println("SAML authentication is enabled")
		// NOTE: SAML configuration would be loaded from environment or config file
		// For now, we set samlAuth to nil since full SAML setup requires certificates
		// Users can enable SAML by setting SAML_ENABLED=true and providing:
		// - SAML_ENTITY_ID, SAML_METADATA_URL, SAML_CERT_PATH, SAML_KEY_PATH
		log.Println("WARNING: SAML is enabled but configuration is incomplete. SAML endpoints will return 503.")
		samlAuth = nil
	} else {
		log.Println("SAML authentication is disabled (set SAML_ENABLED=true to enable)")
		samlAuth = nil
	}

	// Initialize API handlers
	// v2.0-beta: agentHub enables multi-agent routing, k8sClient is OPTIONAL (last parameter) - can be nil for standalone API
	apiHandler := api.NewHandler(database, eventPublisher, commandDispatcher, connTracker, syncService, wsManager, quotaEnforcer, platform, agentHub, k8sClient)
	userHandler := handlers.NewUserHandler(userDB, groupDB)
	groupHandler := handlers.NewGroupHandler(groupDB, userDB)
	authHandler := auth.NewAuthHandler(userDB, jwtManager, samlAuth)
	activityHandler := handlers.NewActivityHandler(k8sClient, activityTracker)
	catalogHandler := handlers.NewCatalogHandler(database)
	sharingHandler := handlers.NewSharingHandler(database)
	pluginHandler := handlers.NewPluginHandler(database, pluginDir)
	dashboardHandler := handlers.NewDashboardHandler(database, k8sClient)
	sessionActivityHandler := handlers.NewSessionActivityHandler(database)
	apiKeyHandler := handlers.NewAPIKeyHandler(database)
	teamHandler := handlers.NewTeamHandler(database)
	// NOTE: Analytics is now handled by the streamspace-analytics-advanced plugin
	preferencesHandler := handlers.NewPreferencesHandler(database)
	notificationsHandler := handlers.NewNotificationsHandler(database)
	searchHandler := handlers.NewSearchHandler(database)
	// NOTE: Session snapshots now handled by streamspace-snapshots plugin
	sessionTemplatesHandler := handlers.NewSessionTemplatesHandler(database, k8sClient, eventPublisher, platform)
	batchHandler := handlers.NewBatchHandler(database)
	monitoringHandler := handlers.NewMonitoringHandler(database)
	quotasHandler := handlers.NewQuotasHandler(database)
	nodeHandler := handlers.NewNodeHandler(database, k8sClient, eventPublisher, platform)
	// NOTE: WebSocket routes now use wsManager directly (see ws.GET routes below)
	consoleHandler := handlers.NewConsoleHandler(database)
	collaborationHandler := handlers.NewCollaborationHandler(database)
	integrationsHandler := handlers.NewIntegrationsHandler(database)
	loadBalancingHandler := handlers.NewLoadBalancingHandler(database)
	schedulingHandler := handlers.NewSchedulingHandler(database)
	securityHandler := handlers.NewSecurityHandler(database)
	templateVersioningHandler := handlers.NewTemplateVersioningHandler(database)
	setupHandler := handlers.NewSetupHandler(database)
	applicationHandler := handlers.NewApplicationHandler(database, eventPublisher, k8sClient, platform)
	// NOTE: Billing is now handled by the streamspace-billing plugin
	auditHandler := handlers.NewAuditHandler(database)
	configurationHandler := handlers.NewConfigurationHandler(database)
	licenseHandler := handlers.NewLicenseHandler(database)
	recordingHandler := handlers.NewRecordingHandler(database)
	agentHandler := handlers.NewAgentHandler(database, agentHub, commandDispatcher)
	agentWebSocketHandler := handlers.NewAgentWebSocketHandler(agentHub, database)
	vncProxyHandler := handlers.NewVNCProxyHandler(database, agentHub)

	// SECURITY: Initialize webhook authentication
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("WARNING: WEBHOOK_SECRET not set. Webhook authentication will be disabled.")
		log.Println("         Generate a secret with: openssl rand -hex 32")
	}

	// Setup routes
	setupRoutes(router, apiHandler, userHandler, groupHandler, authHandler, activityHandler, catalogHandler, sharingHandler, pluginHandler, dashboardHandler, sessionActivityHandler, apiKeyHandler, teamHandler, preferencesHandler, notificationsHandler, searchHandler, sessionTemplatesHandler, batchHandler, monitoringHandler, quotasHandler, nodeHandler, wsManager, consoleHandler, collaborationHandler, integrationsHandler, loadBalancingHandler, schedulingHandler, securityHandler, templateVersioningHandler, setupHandler, applicationHandler, auditHandler, configurationHandler, licenseHandler, recordingHandler, agentHandler, agentWebSocketHandler, vncProxyHandler, jwtManager, userDB, database, redisCache, webhookSecret, rateLimitEnabled, rateLimitRPM)

	// SECURITY: Configure mTLS for agent authentication (optional)
	var tlsConfig *tls.Config
	if agentCACertFile != "" {
		log.Println("Configuring mTLS (Mutual TLS) for agent authentication...")

		// Load CA certificate
		caCert, err := ioutil.ReadFile(agentCACertFile)
		if err != nil {
			log.Fatalf("Failed to read agent CA certificate: %v", err)
		}

		// Create CA certificate pool
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			log.Fatalf("Failed to parse agent CA certificate")
		}

		// Configure TLS with client certificate validation
		tlsConfig = &tls.Config{
			ClientCAs: caCertPool,
			ClientAuth: tls.VerifyClientCertIfGiven, // Default: optional client cert
			MinVersion: tls.VersionTLS12, // Enforce TLS 1.2+
		}

		// If REQUIRE_CLIENT_CERT is true, make client certs mandatory
		if requireClientCert {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			log.Println("mTLS: Client certificates REQUIRED")
		} else {
			log.Println("mTLS: Client certificates OPTIONAL (fallback to API keys)")
		}

		log.Printf("mTLS: Loaded CA certificate from %s", agentCACertFile)
	}

	// Create HTTP server with security timeouts
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,

		// SECURITY: Prevent slow loris attacks and resource exhaustion
		ReadTimeout:       15 * time.Second,  // Time to read request headers + body
		ReadHeaderTimeout: 5 * time.Second,   // Time to read request headers only
		WriteTimeout:      30 * time.Second,  // Time to write response
		IdleTimeout:       120 * time.Second, // Keep-alive timeout

		// SECURITY: Limit header size to prevent memory exhaustion
		MaxHeaderBytes: 1 << 20, // 1 MB

		// SECURITY: TLS configuration (includes mTLS if configured)
		TLSConfig: tlsConfig,
	}

	// Start server in goroutine
	go func() {
		// Check if TLS is configured
		if tlsCertFile != "" && tlsKeyFile != "" {
			if agentCACertFile != "" {
				log.Printf("API Server listening on port %s (HTTPS/TLS + mTLS enabled)", port)
			} else {
				log.Printf("API Server listening on port %s (HTTPS/TLS enabled)", port)
			}
			log.Printf("TLS Certificate: %s", tlsCertFile)
			log.Printf("TLS Key: %s", tlsKeyFile)
			if err := srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start HTTPS server: %v", err)
			}
		} else {
			log.Printf("API Server listening on port %s (HTTP - TLS not configured)", port)
			log.Println("WARNING: Running without TLS/HTTPS. This is insecure for production!")
			log.Println("         Set TLS_CERT_FILE and TLS_KEY_FILE environment variables to enable HTTPS")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start HTTP server: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("Received shutdown signal: %v", sig)
	log.Println("Starting graceful shutdown...")

	// Create shutdown context with timeout
	shutdownTimeout := 30 * time.Second
	if timeoutEnv := os.Getenv("SHUTDOWN_TIMEOUT"); timeoutEnv != "" {
		if duration, err := time.ParseDuration(timeoutEnv); err == nil {
			shutdownTimeout = duration
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server (stops accepting new connections)
	log.Println("Shutting down HTTP server...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	} else {
		log.Println("HTTP server stopped gracefully")
	}

	// Close WebSocket connections
	log.Println("Closing WebSocket connections...")
	if wsManager != nil {
		wsManager.CloseAll()
	}

	// Close database connections
	log.Println("Closing database connections...")
	if database != nil {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			log.Println("Database connections closed")
		}
	}

	// Close Redis cache
	log.Println("Closing Redis cache...")
	if redisCache != nil {
		if err := redisCache.Close(); err != nil {
			log.Printf("Error closing Redis cache: %v", err)
		} else {
			log.Println("Redis cache closed")
		}
	}

	// Close AgentHub Redis client
	if agentHubRedis != nil {
		log.Println("Closing AgentHub Redis client...")
		if err := agentHubRedis.Close(); err != nil {
			log.Printf("Error closing AgentHub Redis: %v", err)
		} else {
			log.Println("AgentHub Redis closed")
		}
	}

	log.Println("Graceful shutdown completed")
}

func setupRoutes(router *gin.Engine, h *api.Handler, userHandler *handlers.UserHandler, groupHandler *handlers.GroupHandler, authHandler *auth.AuthHandler, activityHandler *handlers.ActivityHandler, catalogHandler *handlers.CatalogHandler, sharingHandler *handlers.SharingHandler, pluginHandler *handlers.PluginHandler, dashboardHandler *handlers.DashboardHandler, sessionActivityHandler *handlers.SessionActivityHandler, apiKeyHandler *handlers.APIKeyHandler, teamHandler *handlers.TeamHandler, preferencesHandler *handlers.PreferencesHandler, notificationsHandler *handlers.NotificationsHandler, searchHandler *handlers.SearchHandler, sessionTemplatesHandler *handlers.SessionTemplatesHandler, batchHandler *handlers.BatchHandler, monitoringHandler *handlers.MonitoringHandler, quotasHandler *handlers.QuotasHandler, nodeHandler *handlers.NodeHandler, wsManager *internalWebsocket.Manager, consoleHandler *handlers.ConsoleHandler, collaborationHandler *handlers.CollaborationHandler, integrationsHandler *handlers.IntegrationsHandler, loadBalancingHandler *handlers.LoadBalancingHandler, schedulingHandler *handlers.SchedulingHandler, securityHandler *handlers.SecurityHandler, templateVersioningHandler *handlers.TemplateVersioningHandler, setupHandler *handlers.SetupHandler, applicationHandler *handlers.ApplicationHandler, auditHandler *handlers.AuditHandler, configurationHandler *handlers.ConfigurationHandler, licenseHandler *handlers.LicenseHandler, recordingHandler *handlers.RecordingHandler, agentHandler *handlers.AgentHandler, agentWebSocketHandler *handlers.AgentWebSocketHandler, vncProxyHandler *handlers.VNCProxyHandler, jwtManager *auth.JWTManager, userDB *db.UserDB, database *db.Database, redisCache *cache.Cache, webhookSecret string, rateLimitEnabled bool, rateLimitRPM int) {
	// SECURITY: Create authentication middleware
	authMiddleware := auth.Middleware(jwtManager, userDB)
	adminMiddleware := auth.RequireRole("admin")
	operatorMiddleware := auth.RequireAnyRole("admin", "operator")

	// SECURITY: Create agent API key authentication middleware
	agentAuth := middleware.NewAgentAuth(database)

	// SECURITY: Get global rate limiter for agent endpoints
	globalRateLimiter := middleware.GetRateLimiter()
	if rateLimitEnabled {
		log.Printf("Rate limiting ENABLED: %d requests/min", rateLimitRPM)
	} else {
		log.Println("Rate limiting DISABLED (not recommended for production)")
	}

	// SECURITY: Create webhook authentication middleware
	var webhookAuth *middleware.WebhookAuth
	if webhookSecret != "" {
		webhookAuth = middleware.NewWebhookAuth(webhookSecret)
	}

	// WebSocket upgrader for real-time connections
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for development (should be restricted in production)
			return true
		},
	}

	// Health check (public - no auth required)
	router.GET("/health", h.Health)
	router.GET("/version", h.Version)

	// API Documentation (public - no auth required)
	// Serves OpenAPI spec and Swagger UI at /api/docs
	docsHandler := handlers.NewDocsHandler()
	apiDocs := router.Group("/api")
	docsHandler.RegisterRoutes(apiDocs)

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (public - no auth required, but rate limited)
		authGroup := v1.Group("/auth")
		{
			authHandler.RegisterRoutes(authGroup)
			setupHandler.RegisterRoutes(authGroup)
		}

		// PROTECTED ROUTES - Require authentication
		protected := v1.Group("")
		protected.Use(authMiddleware)
		protected.Use(middleware.CSRFProtection()) // SECURITY: CSRF protection for all state-changing operations
		{
			// Sessions (authenticated users only)
			sessions := protected.Group("/sessions")
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

				// Session lifecycle management (v2.0-beta)
				sessions.PUT("/:id/hibernate", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.HibernateSession)
				sessions.PUT("/:id/wake", cache.InvalidateCacheMiddleware(redisCache, cache.SessionPattern()), h.WakeSession)

				// NOTE: Session heartbeat is registered by ActivityHandler.RegisterRoutes()
				// NOTE: Session recording is now handled by the streamspace-recording plugin
				// Install it via: Admin → Plugins → streamspace-recording

			}

			// VNC Proxy (v2.0 multi-platform architecture - authenticated users)
			// Provides VNC WebSocket connections from UI to session desktops via agents
			vncProxyHandler.RegisterRoutes(protected)

			// VNC Viewer (noVNC static HTML page)
			// Serves the noVNC client that connects to the Control Plane VNC proxy
			protected.GET("/vnc-viewer/:sessionId", func(c *gin.Context) {
				c.File("./api/static/vnc-viewer.html")
			})

			// NOTE: Data Loss Prevention (DLP) is now handled by the streamspace-dlp plugin
			// Install it via: Admin → Plugins → streamspace-dlp

			// NOTE: Workflow Automation is now handled by the streamspace-workflows plugin
			// Install it via: Admin → Plugins → streamspace-workflows

			// In-Browser Console & File Manager
			console := protected.Group("/console")
			{
				// Console sessions (terminal and file manager)
				console.POST("/sessions/:sessionId", consoleHandler.CreateConsoleSession)
				console.GET("/sessions/:sessionId", consoleHandler.ListConsoleSessions)
				console.POST("/:consoleId/disconnect", consoleHandler.DisconnectConsoleSession)

				// File Manager operations
				console.GET("/files/:sessionId", consoleHandler.ListFiles)
				console.GET("/files/:sessionId/content", consoleHandler.GetFileContent)
				console.POST("/files/:sessionId/upload", consoleHandler.UploadFile)
				console.GET("/files/:sessionId/download", consoleHandler.DownloadFile)
				console.POST("/files/:sessionId/directory", consoleHandler.CreateDirectory)
				console.DELETE("/files/:sessionId", consoleHandler.DeleteFile)
				console.PATCH("/files/:sessionId/rename", consoleHandler.RenameFile)

				// File operation history
				console.GET("/files/:sessionId/history", consoleHandler.GetFileOperationHistory)
			}

			// NOTE: Multi-Monitor Support is not yet implemented
			// Will be added in a future release or via plugin
			// monitors := protected.Group("/monitors")
			// {
			//	monitors.GET("/sessions/:sessionId", h.GetMonitorConfiguration)
			//	monitors.POST("/sessions/:sessionId", h.CreateMonitorConfiguration)
			//	monitors.GET("/sessions/:sessionId/list", h.ListMonitorConfigurations)
			//	monitors.PATCH("/configurations/:configId", h.UpdateMonitorConfiguration)
			//	monitors.POST("/configurations/:configId/activate", h.ActivateMonitorConfiguration)
			//	monitors.DELETE("/configurations/:configId", h.DeleteMonitorConfiguration)
			//	monitors.GET("/sessions/:sessionId/streams", h.GetMonitorStreams)
			//	monitors.POST("/sessions/:sessionId/presets/:preset", h.CreatePresetConfiguration)
			// }

			// Real-time Collaboration
			collaboration := protected.Group("/collaboration")
			{
				// Collaboration session management
				collaboration.POST("/sessions/:sessionId", collaborationHandler.CreateCollaborationSession)
				collaboration.POST("/:collabId/join", collaborationHandler.JoinCollaborationSession)
				collaboration.POST("/:collabId/leave", collaborationHandler.LeaveCollaborationSession)

				// Participant management
				collaboration.GET("/:collabId/participants", collaborationHandler.GetCollaborationParticipants)
				collaboration.PATCH("/:collabId/participants/:userId", collaborationHandler.UpdateParticipantRole)

				// Chat operations
				collaboration.POST("/:collabId/chat", collaborationHandler.SendChatMessage)
				collaboration.GET("/:collabId/chat", collaborationHandler.GetChatHistory)

				// Annotation operations
				collaboration.POST("/:collabId/annotations", collaborationHandler.CreateAnnotation)
				collaboration.GET("/:collabId/annotations", collaborationHandler.GetAnnotations)
				collaboration.DELETE("/:collabId/annotations/:annotationId", collaborationHandler.DeleteAnnotation)
				collaboration.DELETE("/:collabId/annotations", collaborationHandler.ClearAllAnnotations)

				// Statistics
				collaboration.GET("/:collabId/stats", collaborationHandler.GetCollaborationStats)
			}

			// Integration Hub & Webhooks - Operator/Admin only
			integrations := protected.Group("/integrations")
			integrations.Use(operatorMiddleware)
			{
				// Webhooks
				integrations.GET("/webhooks", integrationsHandler.ListWebhooks)
				integrations.POST("/webhooks", integrationsHandler.CreateWebhook)
				integrations.PATCH("/webhooks/:webhookId", integrationsHandler.UpdateWebhook)
				integrations.DELETE("/webhooks/:webhookId", integrationsHandler.DeleteWebhook)
				integrations.POST("/webhooks/:webhookId/test", integrationsHandler.TestWebhook)
				integrations.GET("/webhooks/:webhookId/deliveries", integrationsHandler.GetWebhookDeliveries)
				// NOTE: Webhook retry not yet implemented
				// integrations.POST("/webhooks/:webhookId/retry/:deliveryId", h.RetryWebhookDelivery)

				// External Integrations
				integrations.GET("/external", integrationsHandler.ListIntegrations)
				integrations.POST("/external", integrationsHandler.CreateIntegration)
				// NOTE: Update and delete integrations not yet implemented
				// integrations.PATCH("/external/:integrationId", h.UpdateIntegration)
				// integrations.DELETE("/external/:integrationId", h.DeleteIntegration)
				integrations.POST("/external/:integrationId/test", integrationsHandler.TestIntegration)

				// Available events
				integrations.GET("/events", integrationsHandler.GetAvailableEvents)
			}

			// Security - MFA, IP Whitelisting, Zero Trust
			security := protected.Group("/security")
			{
				// Multi-Factor Authentication (all users)
				security.POST("/mfa/setup", securityHandler.SetupMFA)
				security.POST("/mfa/:mfaId/verify-setup", securityHandler.VerifyMFASetup)
				security.POST("/mfa/verify", securityHandler.VerifyMFA)
				security.GET("/mfa/methods", securityHandler.ListMFAMethods)
				security.DELETE("/mfa/:mfaId", securityHandler.DisableMFA)
				security.POST("/mfa/backup-codes", securityHandler.GenerateBackupCodes)

				// IP Whitelisting (users can manage their own, admins can manage all)
				security.POST("/ip-whitelist", securityHandler.CreateIPWhitelist)
				security.GET("/ip-whitelist", securityHandler.ListIPWhitelist)
				security.DELETE("/ip-whitelist/:entryId", securityHandler.DeleteIPWhitelist)
				security.GET("/ip-whitelist/check", securityHandler.CheckIPAccess)

				// Zero Trust / Session Verification
				security.POST("/sessions/:sessionId/verify", securityHandler.VerifySession)
				security.POST("/device-posture", securityHandler.CheckDevicePosture)
				security.GET("/alerts", securityHandler.GetSecurityAlerts)
			}

			// Session Scheduling & Calendar Integration
			scheduling := protected.Group("/scheduling")
			{
				// Scheduled sessions
				scheduling.GET("/sessions", schedulingHandler.ListScheduledSessions)
				scheduling.POST("/sessions", schedulingHandler.CreateScheduledSession)
				scheduling.GET("/sessions/:scheduleId", schedulingHandler.GetScheduledSession)
				scheduling.PATCH("/sessions/:scheduleId", schedulingHandler.UpdateScheduledSession)
				scheduling.DELETE("/sessions/:scheduleId", schedulingHandler.DeleteScheduledSession)
				scheduling.POST("/sessions/:scheduleId/enable", schedulingHandler.EnableScheduledSession)
				scheduling.POST("/sessions/:scheduleId/disable", schedulingHandler.DisableScheduledSession)

				// Calendar integrations
				scheduling.POST("/calendar/connect", schedulingHandler.ConnectCalendar)
				scheduling.GET("/calendar/oauth/callback", schedulingHandler.CalendarOAuthCallback)
				scheduling.GET("/calendar/integrations", schedulingHandler.ListCalendarIntegrations)
				scheduling.DELETE("/calendar/integrations/:integrationId", schedulingHandler.DisconnectCalendar)
				scheduling.POST("/calendar/integrations/:integrationId/sync", schedulingHandler.SyncCalendar)
				scheduling.GET("/calendar/export.ics", schedulingHandler.ExportICalendar)
			}

			// Load Balancing & Auto-scaling - Admin/Operator only
			scaling := protected.Group("/scaling")
			scaling.Use(operatorMiddleware)
			{
				// Load balancing policies
				scaling.GET("/load-balancing/policies", loadBalancingHandler.ListLoadBalancingPolicies)
				scaling.POST("/load-balancing/policies", loadBalancingHandler.CreateLoadBalancingPolicy)
				scaling.GET("/load-balancing/nodes", loadBalancingHandler.GetNodeStatus)
				scaling.POST("/load-balancing/select-node", loadBalancingHandler.SelectNode)

				// Auto-scaling policies
				scaling.GET("/autoscaling/policies", loadBalancingHandler.ListAutoScalingPolicies)
				scaling.POST("/autoscaling/policies", loadBalancingHandler.CreateAutoScalingPolicy)
				scaling.POST("/autoscaling/policies/:policyId/trigger", loadBalancingHandler.TriggerScaling)
				scaling.GET("/autoscaling/history", loadBalancingHandler.GetScalingHistory)
			}

			// Compliance & Governance - Admin only
			// NOTE: These are STUB endpoints that return empty data when the compliance plugin
			// is not installed. Install streamspace-compliance plugin for full functionality.
			compliance := protected.Group("/compliance")
			compliance.Use(adminMiddleware)
			{
				// Dashboard
				compliance.GET("/dashboard", h.GetComplianceDashboard)

				// Frameworks
				compliance.GET("/frameworks", h.ListComplianceFrameworks)
				compliance.POST("/frameworks", h.CreateComplianceFramework)

				// Policies
				compliance.GET("/policies", h.ListCompliancePolicies)
				compliance.POST("/policies", h.CreateCompliancePolicy)

				// Violations
				compliance.GET("/violations", h.ListViolations)
				compliance.POST("/violations", h.RecordViolation)
				compliance.POST("/violations/:violationId/resolve", h.ResolveViolation)
			}
			// Templates (read: all users, write: operators/admins)
			templates := protected.Group("/templates")
			{
				// Read-only template endpoints (all authenticated users)
				templates.GET("", cache.CacheMiddleware(redisCache, 5*time.Minute), h.ListTemplates)
				templates.GET("/:id", cache.CacheMiddleware(redisCache, 5*time.Minute), h.GetTemplate)

				// Write operations require operator or admin role
				templatesWrite := templates.Group("")
				templatesWrite.Use(operatorMiddleware)
				{
					templatesWrite.POST("", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.CreateTemplate)
					templatesWrite.PATCH("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.UpdateTemplate)
					templatesWrite.DELETE("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.DeleteTemplate)

					// Template Versioning (operator only)
					templatesWrite.POST("/:id/versions", templateVersioningHandler.CreateTemplateVersion)
					templatesWrite.GET("/:id/versions", templateVersioningHandler.ListTemplateVersions)
					templatesWrite.GET("/:id/versions/:versionId", templateVersioningHandler.GetTemplateVersion)
					templatesWrite.POST("/:id/versions/:versionId/publish", templateVersioningHandler.PublishTemplateVersion)
					templatesWrite.POST("/:id/versions/:versionId/deprecate", templateVersioningHandler.DeprecateTemplateVersion)
					templatesWrite.POST("/:id/versions/:versionId/set-default", templateVersioningHandler.SetDefaultTemplateVersion)
					templatesWrite.POST("/:id/versions/:versionId/clone", templateVersioningHandler.CloneTemplateVersion)

					// Template Testing (operator only)
					templatesWrite.POST("/:id/versions/:versionId/tests", templateVersioningHandler.CreateTemplateTest)
					templatesWrite.GET("/:id/versions/:versionId/tests", templateVersioningHandler.ListTemplateTests)
					templatesWrite.PATCH("/:id/versions/:versionId/tests/:testId", templateVersioningHandler.UpdateTemplateTestStatus)

					// Template Inheritance
					templatesWrite.GET("/:id/inheritance", templateVersioningHandler.GetTemplateInheritance)
				}
			}

			// Catalog repositories (read: all users, write: operators/admins)
			// NOTE: Template catalog routes are handled by CatalogHandler.RegisterRoutes()
			catalog := protected.Group("/catalog")
			{
				// Repository management
				catalog.GET("/repositories", cache.CacheMiddleware(redisCache, 10*time.Minute), h.ListRepositories)

				// Write operations require operator role
				catalogWrite := catalog.Group("")
				catalogWrite.Use(operatorMiddleware)
				{
					catalogWrite.POST("/repositories", h.AddRepository)
					catalogWrite.DELETE("/repositories/:id", h.RemoveRepository)
					catalogWrite.POST("/sync", h.SyncCatalog)
					catalogWrite.POST("/install", h.InstallTemplate)
				}
			}

			// Cluster management (operators/admins only)
			cluster := protected.Group("/cluster")
			cluster.Use(operatorMiddleware)
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

			// Configuration (admins only)
			config := protected.Group("/config")
			config.Use(adminMiddleware)
			{
				// Cache configuration for 5 minutes (rarely changes)
				config.GET("", cache.CacheMiddleware(redisCache, 5*time.Minute), h.GetConfig)
				config.PATCH("", cache.InvalidateCacheMiddleware(redisCache, cache.ConfigKey("*")), h.UpdateConfig)
			}

			// User management - using dedicated handler (with auth applied in handler)
			userHandler.RegisterRoutes(protected)

			// Group management - using dedicated handler (with auth applied in handler)
			groupHandler.RegisterRoutes(protected)

			// Activity tracking - using dedicated handler
			activityHandler.RegisterRoutes(protected)

			// Enhanced catalog - using dedicated handler
			catalogHandler.RegisterRoutes(protected)

			// Session sharing and collaboration - using dedicated handler
			sharingHandler.RegisterRoutes(protected)

			// Plugin system - using dedicated handler
			pluginHandler.RegisterRoutes(protected)

			// Installed applications management - using dedicated handler (admin only for management)
			applicationHandler.RegisterRoutes(protected)

			// Team-based RBAC - using dedicated handler
			teamHandler.RegisterRoutes(protected)

			// NOTE: Analytics & Reporting is now handled by the streamspace-analytics-advanced plugin
			// Install it via: Admin → Plugins → streamspace-analytics-advanced

			// NOTE: Audit logs are now handled by the streamspace-audit plugin
			// Install it via: Admin → Plugins → streamspace-audit
			// audit := protected.Group("/audit")
			// {
			//	// Admin can view all audit logs with advanced filtering
			//	audit.GET("/logs", adminMiddleware, cache.CacheMiddleware(redisCache, 30*time.Second), auditLogHandler.ListAuditLogs)
			//	audit.GET("/stats", adminMiddleware, cache.CacheMiddleware(redisCache, 1*time.Minute), auditLogHandler.GetAuditLogStats)
			//
			//	// Users can view their own audit logs
			//	audit.GET("/users/:userId/logs", auditLogHandler.GetUserAuditLogs)
			// }

			// Dashboard and resource usage (operators and admins can view platform stats)
			dashboard := protected.Group("/dashboard")
			{
				// Personal dashboard (all users)
				dashboard.GET("/me", cache.CacheMiddleware(redisCache, 30*time.Second), dashboardHandler.GetUserDashboard)

				// Platform-wide stats (operators/admins only)
				dashboard.GET("/platform", operatorMiddleware, cache.CacheMiddleware(redisCache, 1*time.Minute), dashboardHandler.GetPlatformStats)
				dashboard.GET("/resources", operatorMiddleware, cache.CacheMiddleware(redisCache, 1*time.Minute), dashboardHandler.GetResourceUsage)
				dashboard.GET("/users", operatorMiddleware, cache.CacheMiddleware(redisCache, 2*time.Minute), dashboardHandler.GetUserUsageStats)
				dashboard.GET("/templates", operatorMiddleware, cache.CacheMiddleware(redisCache, 5*time.Minute), dashboardHandler.GetTemplateUsageStats)
				dashboard.GET("/timeline", operatorMiddleware, cache.CacheMiddleware(redisCache, 5*time.Minute), dashboardHandler.GetActivityTimeline)
			}

			// Session activity recording and queries
			sessionActivity := protected.Group("/sessions/:id/activity")
			{
				// Log new activity event (for internal API use)
				sessionActivity.POST("/log", sessionActivityHandler.LogActivityEvent)

				// Get session activity log
				sessionActivity.GET("", cache.CacheMiddleware(redisCache, 30*time.Second), sessionActivityHandler.GetSessionActivity)

				// Get session timeline (chronological view)
				sessionActivity.GET("/timeline", cache.CacheMiddleware(redisCache, 1*time.Minute), sessionActivityHandler.GetSessionTimeline)
			}

			// Activity statistics and user activity (admins/operators)
			activity := protected.Group("/activity")
			{
				// Activity statistics (admins/operators only)
				activity.GET("/stats", operatorMiddleware, cache.CacheMiddleware(redisCache, 2*time.Minute), sessionActivityHandler.GetActivityStats)

				// User activity across all sessions (users can view their own)
				activity.GET("/users/:userId", sessionActivityHandler.GetUserSessionActivity)
			}

			// API Key management (users can manage their own keys)
			apiKeys := protected.Group("/api-keys")
			{
				// Create new API key (returns full key only once)
				apiKeys.POST("", apiKeyHandler.CreateAPIKey)

				// List user's API keys (does not return full keys)
				apiKeys.GET("", cache.CacheMiddleware(redisCache, 1*time.Minute), apiKeyHandler.ListAPIKeys)

				// Revoke an API key (soft delete - sets is_active to false)
				apiKeys.POST("/:id/revoke", apiKeyHandler.RevokeAPIKey)

				// Permanently delete an API key
				apiKeys.DELETE("/:id", apiKeyHandler.DeleteAPIKey)

				// Get usage statistics for an API key
				apiKeys.GET("/:id/usage", cache.CacheMiddleware(redisCache, 30*time.Second), apiKeyHandler.GetAPIKeyUsage)
			}

			// User preferences and settings - using dedicated handler (all authenticated users)
			preferencesHandler.RegisterRoutes(protected)

			// Notifications system - using dedicated handler (all authenticated users)
			notificationsHandler.RegisterRoutes(protected)

			// Advanced search and filtering - using dedicated handler (all authenticated users)
			searchHandler.RegisterRoutes(protected)

			// NOTE: Session snapshots are now handled by the streamspace-snapshots plugin
			// Install it via: Admin → Plugins → streamspace-snapshots

			// Session templates and presets - using dedicated handler (all authenticated users)
			sessionTemplatesHandler.RegisterRoutes(protected)

			// Batch operations for sessions - using dedicated handler (all authenticated users)
			batchHandler.RegisterRoutes(protected)

			// Advanced monitoring and metrics - using dedicated handler (operators/admins only)
			monitoringHandler.RegisterRoutes(protected.Group("", operatorMiddleware))

			// Resource quotas and limits enforcement - using dedicated handler (operators/admins only)
			quotasHandler.RegisterRoutes(protected.Group("", operatorMiddleware))

			// Node Management (admin only)
			admin := protected.Group("/admin")
			admin.Use(adminMiddleware)
			{
				admin.GET("/nodes", nodeHandler.ListNodes)
				admin.GET("/nodes/stats", nodeHandler.GetClusterStats)
				admin.GET("/nodes/:name", nodeHandler.GetNode)
				admin.PUT("/nodes/:name/labels", nodeHandler.AddNodeLabel)
				admin.DELETE("/nodes/:name/labels/:key", nodeHandler.RemoveNodeLabel)
				admin.POST("/nodes/:name/taints", nodeHandler.AddNodeTaint)
				admin.DELETE("/nodes/:name/taints/:key", nodeHandler.RemoveNodeTaint)
				admin.POST("/nodes/:name/cordon", nodeHandler.CordonNode)
				admin.POST("/nodes/:name/uncordon", nodeHandler.UncordonNode)
				admin.POST("/nodes/:name/drain", nodeHandler.DrainNode)

				// Audit logs (admin only)
				auditHandler.RegisterRoutes(admin)

				// System configuration (admin only)
				configurationHandler.RegisterRoutes(admin)

				// License management (admin only)
				licenseHandler.RegisterRoutes(admin)

				// API keys management (admin only - system-wide view)
				admin.GET("/apikeys", apiKeyHandler.ListAllAPIKeys)

				// Session recordings management (admin only)
				recordingHandler.RegisterRoutes(admin)

				// v2.0 Agent management (admin only - multi-platform architecture)
				agentHandler.RegisterAdminRoutes(admin)

			}

			// NOTE: Billing is now handled by the streamspace-billing plugin
			// Install it via: Admin → Plugins → streamspace-billing

			// Metrics (operators/admins only)
			protected.GET("/metrics", operatorMiddleware, h.GetMetrics)
		}

	// v2.0 Agent self-service routes (require mTLS OR API key authentication, not JWT)
	// These routes are for agents to register themselves and send heartbeats
	// Authentication: mTLS (if configured) or API key fallback
	// Rate limited to prevent brute-force attacks
	agentRoutes := v1.Group("/agents")
	agentRoutes.Use(agentRateLimit(globalRateLimiter, rateLimitEnabled, rateLimitRPM))       // Apply rate limiting first
	agentRoutes.Use(agentAuth.RequireAuth())   // Then authentication
	{
		agentHandler.RegisterRoutes(agentRoutes)
	}

	// v2.0 Agent WebSocket connections (require mTLS OR API key authentication, not JWT)
	// Agents connect here to receive commands and send status updates
	// Authentication: mTLS (if configured) or API key fallback
	// Rate limited to prevent connection flooding
	agentWSRoutes := v1.Group("")
	agentWSRoutes.Use(agentRateLimit(globalRateLimiter, rateLimitEnabled, rateLimitRPM))     // Apply rate limiting first
	agentWSRoutes.Use(agentAuth.RequireAuth()) // Then authentication
	{
		agentWebSocketHandler.RegisterRoutes(agentWSRoutes)
	}
	}

	// WebSocket endpoints (require authentication)
	ws := router.Group("/api/v1/ws")
	ws.Use(authMiddleware)
	{
		// Session updates WebSocket - connects to wsManager for real-time session broadcasts
		ws.GET("/sessions", func(c *gin.Context) {
			// Get user ID from auth middleware
			userID, exists := c.Get("userID")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
				return
			}

			userIDStr, ok := userID.(string)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}

			// Upgrade HTTP connection to WebSocket
			conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// Delegate to wsManager which broadcasts sessions every 3 seconds
			wsManager.HandleSessionsWebSocket(conn, userIDStr, "")
		})

		// Metrics WebSocket - connects to wsManager for real-time metrics broadcasts
		ws.GET("/cluster", operatorMiddleware, func(c *gin.Context) {
			// Upgrade HTTP connection to WebSocket
			conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// Delegate to wsManager which broadcasts metrics every 5 seconds
			wsManager.HandleMetricsWebSocket(conn)
		})

		ws.GET("/logs/:namespace/:pod", operatorMiddleware, h.LogsWebSocket)
		ws.GET("/enterprise", handlers.HandleEnterpriseWebSocket) // Real-time enterprise features
	}

	// Webhook endpoints (HMAC signature validation required)
	webhooks := router.Group("/webhooks")
	{
		if webhookAuth != nil {
			// SECURITY: Require webhook signature validation
			webhooks.POST("/repository/sync", webhookAuth.Middleware(), h.WebhookRepositorySync)
		} else {
			// WARNING: Running without webhook authentication
			log.Println("WARNING: Webhook endpoints running without authentication")
			webhooks.POST("/repository/sync", h.WebhookRepositorySync)
		}
	}
}

// corsMiddleware configures Cross-Origin Resource Sharing (CORS) for the API.
// This middleware enables the web UI to communicate with the API backend when they
// are served from different origins (domains/ports).
//
// SECURITY FEATURES:
// - Origin validation: Only explicitly allowed origins can access the API
// - Credential support: Allows cookies and authorization headers in CORS requests
// - WebSocket support: Includes headers required for WebSocket upgrade handshake
//
// WEBSOCKET HEADERS:
// The following headers are essential for WebSocket connections to work:
// - Upgrade: Indicates protocol upgrade from HTTP to WebSocket
// - Connection: Specifies the connection should be upgraded
// - Sec-WebSocket-Key: Client-generated key for handshake
// - Sec-WebSocket-Version: WebSocket protocol version
// - Sec-WebSocket-Extensions: Optional WebSocket extensions
// - Sec-WebSocket-Protocol: Sub-protocol negotiation
//
// CONFIGURATION:
// Set CORS_ALLOWED_ORIGINS environment variable with comma-separated list of origins:
// Example: CORS_ALLOWED_ORIGINS="https://app.streamspace.io,https://admin.streamspace.io"
//
// DEVELOPMENT:
// If not configured, defaults to localhost:3000,8000 for local development
func corsMiddleware() gin.HandlerFunc {
	// SECURITY: Get allowed origins from environment
	allowedOriginsEnv := getEnv("CORS_ALLOWED_ORIGINS", "")
	var allowedOrigins []string

	if allowedOriginsEnv != "" {
		// Parse comma-separated list of origins
		for _, origin := range strings.Split(allowedOriginsEnv, ",") {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(origin))
		}
	}

	// If no origins specified, use localhost only for development
	if len(allowedOrigins) == 0 {
		log.Println("WARNING: No CORS_ALLOWED_ORIGINS set, defaulting to localhost only")
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:8000"}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Allow standard HTTP headers plus WebSocket upgrade headers
		// WebSocket headers (Upgrade, Connection, Sec-WebSocket-*) are required for
		// real-time features like session updates and VNC connections
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Upgrade, Connection, Sec-WebSocket-Key, Sec-WebSocket-Version, Sec-WebSocket-Extensions, Sec-WebSocket-Protocol")
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// agentRateLimit returns a middleware that rate limits agent requests.
func agentRateLimit(limiter *middleware.RateLimiter, enabled bool, maxRequests int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		// Use client IP as rate limit key
		key := "agent:" + c.ClientIP()
		window := 1 * time.Minute

		// Check rate limit
		if !limiter.CheckLimit(key, maxRequests, window) {
			log.Printf("[RateLimit] Rate limit exceeded for IP %s (max %d req/min)", c.ClientIP(), maxRequests)

			// Set audit metadata for rate limiting event
			c.Set("audit_metadata", map[string]interface{}{
				"rate_limit_exceeded": true,
				"max_requests":        maxRequests,
				"window_seconds":      60,
			})

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "Rate limit exceeded",
				"details":    "Too many requests. Please try again later.",
				"retryAfter": 60, // seconds
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
