package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	dbSSLMode := getEnv("DB_SSL_MODE", "disable") // SECURITY: Should be "require" in production

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
	router.Use(middleware.RequestSizeLimit(10 * 1024 * 1024))

	// SECURITY: Add rate limiting to prevent DoS attacks
	// Layer 1: IP-based rate limiting (100 req/sec per IP with burst of 200)
	rateLimiter := middleware.NewRateLimiter(100, 200)
	router.Use(rateLimiter.Middleware())

	// Layer 2: Per-user rate limiting (1000 req/hour per authenticated user)
	// Prevents abuse from compromised tokens
	userRateLimiter := middleware.NewUserRateLimiter(1000, 50)
	router.Use(userRateLimiter.Middleware())

	// SECURITY: Add audit logging for all requests
	auditLogger := middleware.NewAuditLogger(database, false) // Don't log request bodies by default
	router.Use(auditLogger.Middleware())

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
	jwtManager := auth.NewJWTManager(jwtConfig)

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
	apiHandler := api.NewHandler(database, k8sClient, connTracker, syncService, wsManager, quotaEnforcer)
	userHandler := handlers.NewUserHandler(userDB)
	groupHandler := handlers.NewGroupHandler(groupDB, userDB)
	authHandler := auth.NewAuthHandler(userDB, jwtManager, samlAuth)
	activityHandler := handlers.NewActivityHandler(k8sClient, activityTracker)
	catalogHandler := handlers.NewCatalogHandler(database)
	sharingHandler := handlers.NewSharingHandler(database)
	pluginHandler := handlers.NewPluginHandler(database)
	auditLogHandler := handlers.NewAuditLogHandler(database)
	dashboardHandler := handlers.NewDashboardHandler(database, k8sClient)
	sessionActivityHandler := handlers.NewSessionActivityHandler(database)
	apiKeyHandler := handlers.NewAPIKeyHandler(database)
	teamHandler := handlers.NewTeamHandler(database)
	analyticsHandler := handlers.NewAnalyticsHandler(database)
	preferencesHandler := handlers.NewPreferencesHandler(database)
	notificationsHandler := handlers.NewNotificationsHandler(database)
	searchHandler := handlers.NewSearchHandler(database)
	snapshotsHandler := handlers.NewSnapshotsHandler(database)
	sessionTemplatesHandler := handlers.NewSessionTemplatesHandler(database)
	batchHandler := handlers.NewBatchHandler(database)
	monitoringHandler := handlers.NewMonitoringHandler(database)
	quotasHandler := handlers.NewQuotasHandler(database)
	websocketHandler := handlers.NewWebSocketHandler(database)
	billingHandler := handlers.NewBillingHandler(database)

	// SECURITY: Initialize webhook authentication
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("WARNING: WEBHOOK_SECRET not set. Webhook authentication will be disabled.")
		log.Println("         Generate a secret with: openssl rand -hex 32")
	}

	// SECURITY: Initialize CSRF protection
	csrfProtection := middleware.NewCSRFProtection(24 * time.Hour)

	// SECURITY: Create stricter rate limiter for auth endpoints
	authRateLimiter := middleware.NewRateLimiter(5, 10) // 5 req/sec with burst of 10

	// Setup routes
	setupRoutes(router, apiHandler, userHandler, groupHandler, authHandler, activityHandler, catalogHandler, sharingHandler, pluginHandler, auditLogHandler, dashboardHandler, sessionActivityHandler, apiKeyHandler, teamHandler, analyticsHandler, preferencesHandler, notificationsHandler, searchHandler, snapshotsHandler, sessionTemplatesHandler, batchHandler, monitoringHandler, quotasHandler, websocketHandler, billingHandler, jwtManager, userDB, redisCache, webhookSecret, csrfProtection, authRateLimiter)

	// Create HTTP server with security timeouts
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,

		// SECURITY: Prevent slow loris attacks and resource exhaustion
		ReadTimeout:       15 * time.Second, // Time to read request headers + body
		ReadHeaderTimeout: 5 * time.Second,  // Time to read request headers only
		WriteTimeout:      30 * time.Second, // Time to write response
		IdleTimeout:       120 * time.Second, // Keep-alive timeout

		// SECURITY: Limit header size to prevent memory exhaustion
		MaxHeaderBytes: 1 << 20, // 1 MB
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

	log.Println("Graceful shutdown completed")
}

func setupRoutes(router *gin.Engine, h *api.Handler, userHandler *handlers.UserHandler, groupHandler *handlers.GroupHandler, authHandler *auth.AuthHandler, activityHandler *handlers.ActivityHandler, catalogHandler *handlers.CatalogHandler, sharingHandler *handlers.SharingHandler, pluginHandler *handlers.PluginHandler, auditLogHandler *handlers.AuditLogHandler, dashboardHandler *handlers.DashboardHandler, sessionActivityHandler *handlers.SessionActivityHandler, apiKeyHandler *handlers.APIKeyHandler, teamHandler *handlers.TeamHandler, analyticsHandler *handlers.AnalyticsHandler, preferencesHandler *handlers.PreferencesHandler, notificationsHandler *handlers.NotificationsHandler, searchHandler *handlers.SearchHandler, snapshotsHandler *handlers.SnapshotsHandler, sessionTemplatesHandler *handlers.SessionTemplatesHandler, batchHandler *handlers.BatchHandler, monitoringHandler *handlers.MonitoringHandler, quotasHandler *handlers.QuotasHandler, websocketHandler *handlers.WebSocketHandler, billingHandler *handlers.BillingHandler, jwtManager *auth.JWTManager, userDB *db.UserDB, redisCache *cache.Cache, webhookSecret string, csrfProtection *middleware.CSRFProtection, authRateLimiter *middleware.RateLimiter) {
	// SECURITY: Create authentication middleware
	authMiddleware := auth.Middleware(jwtManager, userDB)
	adminMiddleware := auth.RequireRole("admin")
	operatorMiddleware := auth.RequireAnyRole("admin", "operator")

	// SECURITY: Create webhook authentication middleware
	var webhookAuth *middleware.WebhookAuth
	if webhookSecret != "" {
		webhookAuth = middleware.NewWebhookAuth(webhookSecret)
	}

	// Health check (public - no auth required)
	router.GET("/health", h.Health)
	router.GET("/version", h.Version)

	// SECURITY: CSRF token endpoint (public - issues CSRF tokens)
	router.GET("/api/v1/csrf-token", csrfProtection.IssueTokenHandler())

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (public - no auth required, but rate limited)
		authGroup := v1.Group("/auth")
		authGroup.Use(authRateLimiter.Middleware()) // SECURITY: Brute force protection
		{
			authHandler.RegisterRoutes(authGroup)
		}

		// PROTECTED ROUTES - Require authentication
		protected := v1.Group("")
		protected.Use(authMiddleware)
		protected.Use(csrfProtection.Middleware()) // SECURITY: CSRF protection for all state-changing operations
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
				sessions.POST("/:id/heartbeat", h.SessionHeartbeat)

				// Session recording endpoints (nested under sessions)
				sessions.POST("/:sessionId/recordings/start", h.StartSessionRecording)
				sessions.POST("/recordings/:recordingId/stop", h.StopSessionRecording)
			}

			// Session Recordings (recording management and playback)
			recordings := protected.Group("/recordings")
			{
				recordings.GET("", h.ListSessionRecordings)
				recordings.GET("/:recordingId", h.GetSessionRecording)
				recordings.GET("/:recordingId/stream", h.StreamRecording)
				recordings.GET("/:recordingId/download", h.DownloadRecording)
				recordings.DELETE("/:recordingId", h.DeleteRecording)
				recordings.GET("/stats", h.GetRecordingStats)

				// Recording policies (admin/operator only)
				recordingPolicies := recordings.Group("/policies")
				recordingPolicies.Use(operatorMiddleware)
				{
					recordingPolicies.GET("", h.ListRecordingPolicies)
					recordingPolicies.POST("", h.CreateRecordingPolicy)
					recordingPolicies.PATCH("/:policyId", h.UpdateRecordingPolicy)
					recordingPolicies.DELETE("/:policyId", h.DeleteRecordingPolicy)
				}

				// Cleanup expired recordings (admin only)
				recordings.POST("/cleanup", operatorMiddleware, h.CleanupExpiredRecordings)
			}

			// Data Loss Prevention (DLP) - Admin/Operator only
			dlp := protected.Group("/dlp")
			dlp.Use(operatorMiddleware)
			{
				// DLP Policies
				dlp.GET("/policies", h.ListDLPPolicies)
				dlp.POST("/policies", h.CreateDLPPolicy)
				dlp.GET("/policies/:policyId", h.GetDLPPolicy)
				dlp.PATCH("/policies/:policyId", h.UpdateDLPPolicy)
				dlp.DELETE("/policies/:policyId", h.DeleteDLPPolicy)

				// DLP Violations
				dlp.POST("/violations", h.LogDLPViolation)
				dlp.GET("/violations", h.ListDLPViolations)
				dlp.POST("/violations/:violationId/resolve", h.ResolveDLPViolation)

				// DLP Statistics
				dlp.GET("/stats", h.GetDLPStats)
			}

			// Workflow Automation - Operator/Admin only
			workflows := protected.Group("/workflows")
			workflows.Use(operatorMiddleware)
			{
				workflows.GET("", h.ListWorkflows)
				workflows.POST("", h.CreateWorkflow)
				workflows.GET("/:workflowId", h.GetWorkflow)
				workflows.PATCH("/:workflowId", h.UpdateWorkflow)
				workflows.DELETE("/:workflowId", h.DeleteWorkflow)
				workflows.POST("/:workflowId/execute", h.ExecuteWorkflow)

				// Workflow Executions
				workflows.GET("/executions", h.ListWorkflowExecutions)
				workflows.GET("/executions/:executionId", h.GetWorkflowExecution)
				workflows.POST("/executions/:executionId/cancel", h.CancelWorkflowExecution)

				// Workflow Statistics
				workflows.GET("/stats", h.GetWorkflowStats)
			}

			// In-Browser Console & File Manager
			console := protected.Group("/console")
			{
				// Console sessions (terminal and file manager)
				console.POST("/sessions/:sessionId", h.CreateConsoleSession)
				console.GET("/sessions/:sessionId", h.ListConsoleSessions)
				console.POST("/:consoleId/disconnect", h.DisconnectConsoleSession)

				// File Manager operations
				console.GET("/files/:sessionId", h.ListFiles)
				console.GET("/files/:sessionId/content", h.GetFileContent)
				console.POST("/files/:sessionId/upload", h.UploadFile)
				console.GET("/files/:sessionId/download", h.DownloadFile)
				console.POST("/files/:sessionId/directory", h.CreateDirectory)
				console.DELETE("/files/:sessionId", h.DeleteFile)
				console.PATCH("/files/:sessionId/rename", h.RenameFile)

				// File operation history
				console.GET("/files/:sessionId/history", h.GetFileOperationHistory)
			}

			// Multi-Monitor Support
			monitors := protected.Group("/monitors")
			{
				monitors.GET("/sessions/:sessionId", h.GetMonitorConfiguration)
				monitors.POST("/sessions/:sessionId", h.CreateMonitorConfiguration)
				monitors.GET("/sessions/:sessionId/list", h.ListMonitorConfigurations)
				monitors.PATCH("/configurations/:configId", h.UpdateMonitorConfiguration)
				monitors.POST("/configurations/:configId/activate", h.ActivateMonitorConfiguration)
				monitors.DELETE("/configurations/:configId", h.DeleteMonitorConfiguration)
				monitors.GET("/sessions/:sessionId/streams", h.GetMonitorStreams)

				// Preset configurations
				monitors.POST("/sessions/:sessionId/presets/:preset", h.CreatePresetConfiguration)
			}

			// Real-time Collaboration
			collaboration := protected.Group("/collaboration")
			{
				// Collaboration session management
				collaboration.POST("/sessions/:sessionId", h.CreateCollaborationSession)
				collaboration.POST("/:collabId/join", h.JoinCollaborationSession)
				collaboration.POST("/:collabId/leave", h.LeaveCollaborationSession)

				// Participant management
				collaboration.GET("/:collabId/participants", h.GetCollaborationParticipants)
				collaboration.PATCH("/:collabId/participants/:userId", h.UpdateParticipantRole)

				// Chat operations
				collaboration.POST("/:collabId/chat", h.SendChatMessage)
				collaboration.GET("/:collabId/chat", h.GetChatHistory)

				// Annotation operations
				collaboration.POST("/:collabId/annotations", h.CreateAnnotation)
				collaboration.GET("/:collabId/annotations", h.GetAnnotations)
				collaboration.DELETE("/:collabId/annotations/:annotationId", h.DeleteAnnotation)
				collaboration.DELETE("/:collabId/annotations", h.ClearAllAnnotations)

				// Statistics
				collaboration.GET("/:collabId/stats", h.GetCollaborationStats)
			}

		// Integration Hub & Webhooks - Operator/Admin only
		integrations := protected.Group("/integrations")
		integrations.Use(operatorMiddleware)
		{
			// Webhooks
			integrations.GET("/webhooks", h.ListWebhooks)
			integrations.POST("/webhooks", h.CreateWebhook)
			integrations.PATCH("/webhooks/:webhookId", h.UpdateWebhook)
			integrations.DELETE("/webhooks/:webhookId", h.DeleteWebhook)
			integrations.POST("/webhooks/:webhookId/test", h.TestWebhook)
			integrations.GET("/webhooks/:webhookId/deliveries", h.GetWebhookDeliveries)
			integrations.POST("/webhooks/:webhookId/retry/:deliveryId", h.RetryWebhookDelivery)

			// External Integrations
			integrations.GET("/external", h.ListIntegrations)
			integrations.POST("/external", h.CreateIntegration)
			integrations.PATCH("/external/:integrationId", h.UpdateIntegration)
			integrations.DELETE("/external/:integrationId", h.DeleteIntegration)
			integrations.POST("/external/:integrationId/test", h.TestIntegration)

			// Available events
			integrations.GET("/events", h.GetAvailableEvents)
		}

		// Security - MFA, IP Whitelisting, Zero Trust
		security := protected.Group("/security")
		{
			// Multi-Factor Authentication (all users)
			security.POST("/mfa/setup", h.SetupMFA)
			security.POST("/mfa/:mfaId/verify-setup", h.VerifyMFASetup)
			security.POST("/mfa/verify", h.VerifyMFA)
			security.GET("/mfa/methods", h.ListMFAMethods)
			security.DELETE("/mfa/:mfaId", h.DisableMFA)
			security.POST("/mfa/backup-codes", h.GenerateBackupCodes)

			// IP Whitelisting (users can manage their own, admins can manage all)
			security.POST("/ip-whitelist", h.CreateIPWhitelist)
			security.GET("/ip-whitelist", h.ListIPWhitelist)
			security.DELETE("/ip-whitelist/:entryId", h.DeleteIPWhitelist)
			security.GET("/ip-whitelist/check", h.CheckIPAccess)

			// Zero Trust / Session Verification
			security.POST("/sessions/:sessionId/verify", h.VerifySession)
			security.POST("/device-posture", h.CheckDevicePosture)
			security.GET("/alerts", h.GetSecurityAlerts)
		}

		// Session Scheduling & Calendar Integration
		scheduling := protected.Group("/scheduling")
		{
			// Scheduled sessions
			scheduling.GET("/sessions", h.ListScheduledSessions)
			scheduling.POST("/sessions", h.CreateScheduledSession)
			scheduling.GET("/sessions/:scheduleId", h.GetScheduledSession)
			scheduling.PATCH("/sessions/:scheduleId", h.UpdateScheduledSession)
			scheduling.DELETE("/sessions/:scheduleId", h.DeleteScheduledSession)
			scheduling.POST("/sessions/:scheduleId/enable", h.EnableScheduledSession)
			scheduling.POST("/sessions/:scheduleId/disable", h.DisableScheduledSession)

			// Calendar integrations
			scheduling.POST("/calendar/connect", h.ConnectCalendar)
			scheduling.GET("/calendar/oauth/callback", h.CalendarOAuthCallback)
			scheduling.GET("/calendar/integrations", h.ListCalendarIntegrations)
			scheduling.DELETE("/calendar/integrations/:integrationId", h.DisconnectCalendar)
			scheduling.POST("/calendar/integrations/:integrationId/sync", h.SyncCalendar)
			scheduling.GET("/calendar/export.ics", h.ExportICalendar)
		}

		// Load Balancing & Auto-scaling - Admin/Operator only
		scaling := protected.Group("/scaling")
		scaling.Use(operatorMiddleware)
		{
			// Load balancing policies
			scaling.GET("/load-balancing/policies", h.ListLoadBalancingPolicies)
			scaling.POST("/load-balancing/policies", h.CreateLoadBalancingPolicy)
			scaling.GET("/load-balancing/nodes", h.GetNodeStatus)
			scaling.POST("/load-balancing/select-node", h.SelectNode)

			// Auto-scaling policies
			scaling.GET("/autoscaling/policies", h.ListAutoScalingPolicies)
			scaling.POST("/autoscaling/policies", h.CreateAutoScalingPolicy)
			scaling.POST("/autoscaling/policies/:policyId/trigger", h.TriggerScaling)
			scaling.GET("/autoscaling/history", h.GetScalingHistory)
		}

		// Compliance & Governance - Admin only
		compliance := protected.Group("/compliance")
		compliance.Use(adminMiddleware)
		{
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

			// Reports & Dashboard
			compliance.POST("/reports/generate", h.GenerateComplianceReport)
			compliance.GET("/dashboard", h.GetComplianceDashboard)
		}

			// Templates (read: all users, write: operators/admins)
			templates := protected.Group("/templates")
			{
				// Cache template lists for 5 minutes (rarely changing)
				templates.GET("", cache.CacheMiddleware(redisCache, 5*time.Minute), h.ListTemplates)

				// User favorites (all authenticated users) - MUST be before /:id routes
				templates.GET("/favorites", cache.CacheMiddleware(redisCache, 30*time.Second), h.ListUserFavoriteTemplates)

				// Template details and favorite operations
				templates.GET("/:id", cache.CacheMiddleware(redisCache, 5*time.Minute), h.GetTemplate)
				templates.POST("/:id/favorite", cache.InvalidateCacheMiddleware(redisCache, cache.UserFavoritesPattern()), h.AddTemplateFavorite)
				templates.DELETE("/:id/favorite", cache.InvalidateCacheMiddleware(redisCache, cache.UserFavoritesPattern()), h.RemoveTemplateFavorite)
				templates.GET("/:id/favorite", cache.CacheMiddleware(redisCache, 30*time.Second), h.CheckTemplateFavorite)

				// Write operations require operator role
				templatesWrite := templates.Group("")
				templatesWrite.Use(operatorMiddleware)
				{
					templatesWrite.POST("", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.CreateTemplate)
					templatesWrite.PATCH("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.UpdateTemplate)
					templatesWrite.DELETE("/:id", cache.InvalidateCacheMiddleware(redisCache, cache.TemplatePattern()), h.DeleteTemplate)

					// Template Versioning (operator only)
					templatesWrite.POST("/:templateId/versions", h.CreateTemplateVersion)
					templatesWrite.GET("/:templateId/versions", h.ListTemplateVersions)
					templatesWrite.GET("/versions/:versionId", h.GetTemplateVersion)
					templatesWrite.POST("/versions/:versionId/publish", h.PublishTemplateVersion)
					templatesWrite.POST("/versions/:versionId/deprecate", h.DeprecateTemplateVersion)
					templatesWrite.POST("/versions/:versionId/set-default", h.SetDefaultTemplateVersion)
					templatesWrite.POST("/versions/:versionId/clone", h.CloneTemplateVersion)

					// Template Testing (operator only)
					templatesWrite.POST("/versions/:versionId/tests", h.CreateTemplateTest)
					templatesWrite.GET("/versions/:versionId/tests", h.ListTemplateTests)
					templatesWrite.PATCH("/tests/:testId", h.UpdateTemplateTestStatus)

					// Template Inheritance
					templatesWrite.GET("/:templateId/inheritance", h.GetTemplateInheritance)
				}
			}

			// Catalog (read: all users, write: operators/admins)
			catalog := protected.Group("/catalog")
			{
				// Cache catalog data for 10 minutes (changes on sync)
				catalog.GET("/repositories", cache.CacheMiddleware(redisCache, 10*time.Minute), h.ListRepositories)
				catalog.GET("/templates", cache.CacheMiddleware(redisCache, 10*time.Minute), h.BrowseCatalog)

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

			// Team-based RBAC - using dedicated handler
			teamHandler.RegisterRoutes(protected)

			// Analytics - using dedicated handler (operators and admins)
			analyticsProtected := protected.Group("")
			analyticsProtected.Use(operatorMiddleware)
			{
				analyticsHandler.RegisterRoutes(analyticsProtected)
			}

			// Audit logs (admins only for viewing, operators can view their own)
			audit := protected.Group("/audit")
			{
				// Admin can view all audit logs with advanced filtering
				audit.GET("/logs", adminMiddleware, cache.CacheMiddleware(redisCache, 30*time.Second), auditLogHandler.ListAuditLogs)
				audit.GET("/stats", adminMiddleware, cache.CacheMiddleware(redisCache, 1*time.Minute), auditLogHandler.GetAuditLogStats)

				// Users can view their own audit logs
				audit.GET("/users/:userId/logs", auditLogHandler.GetUserAuditLogs)
			}

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
			sessionActivity := protected.Group("/sessions/:sessionId/activity")
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

			// Session snapshots and restore - using dedicated handler (all authenticated users)
			snapshotsHandler.RegisterRoutes(protected)

			// Session templates and presets - using dedicated handler (all authenticated users)
			sessionTemplatesHandler.RegisterRoutes(protected)

			// Batch operations for sessions - using dedicated handler (all authenticated users)
			batchHandler.RegisterRoutes(protected)

			// Advanced monitoring and metrics - using dedicated handler (operators/admins only)
			monitoringHandler.RegisterRoutes(protected.Group("", operatorMiddleware))

			// Resource quotas and limits enforcement - using dedicated handler (operators/admins only)
			quotasHandler.RegisterRoutes(protected.Group("", operatorMiddleware))

			// Cost management and billing - using dedicated handler (all authenticated users)
			billingHandler.RegisterRoutes(protected)

			// Metrics (operators/admins only)
			protected.GET("/metrics", operatorMiddleware, h.GetMetrics)
		}
	}

	// WebSocket endpoints (require authentication)
	ws := router.Group("/api/v1/ws")
	ws.Use(authMiddleware)
	{
		ws.GET("/sessions", h.SessionsWebSocket)
		ws.GET("/cluster", operatorMiddleware, h.ClusterWebSocket)
		ws.GET("/logs/:namespace/:pod", operatorMiddleware, h.LogsWebSocket)
		ws.GET("/enterprise", handlers.HandleEnterpriseWebSocket) // Real-time enterprise features
	}

	// Real-time updates via WebSocket - using dedicated handler (all authenticated users)
	websocketHandler.RegisterRoutes(router.Group("/api/v1", authMiddleware))

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
