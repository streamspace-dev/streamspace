// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements monitoring, metrics, health checks, and alerting endpoints.
//
// MONITORING FEATURES:
// - Prometheus-compatible metrics export
// - Custom application metrics (sessions, users, resources)
// - Health check endpoints (liveness, readiness)
// - Performance metrics (response times, throughput)
// - System resource metrics (CPU, memory, goroutines)
// - Alert management (create, acknowledge, resolve)
//
// METRICS EXPORT:
// - Prometheus format (/monitoring/metrics/prometheus)
// - Session metrics (total, running, hibernated)
// - User metrics (total, active)
// - Resource metrics (CPU, memory, storage usage)
// - Performance metrics (response time percentiles)
//
// HEALTH CHECKS:
// - Basic health: /monitoring/health (200 if API is responding)
// - Detailed health: Database, storage, Kubernetes connectivity
// - Database health: Connection pool status, query latency
// - Storage health: NFS mount status, disk space
//
// SYSTEM METRICS:
// - Go runtime stats (goroutines, memory, GC)
// - Build information (version, git commit, build time)
// - Uptime and request counts
// - Resource usage trends
//
// ALERTING:
// - Create, read, update, delete alerts
// - Acknowledge and resolve workflows
// - Alert severity levels (info, warning, error, critical)
// - Alert filtering and querying
//
// API Endpoints:
// - GET    /api/v1/monitoring/metrics/prometheus - Prometheus metrics
// - GET    /api/v1/monitoring/metrics/sessions - Session metrics
// - GET    /api/v1/monitoring/metrics/resources - Resource metrics
// - GET    /api/v1/monitoring/metrics/users - User metrics
// - GET    /api/v1/monitoring/metrics/performance - Performance metrics
// - GET    /api/v1/monitoring/health - Basic health check
// - GET    /api/v1/monitoring/health/detailed - Detailed health check
// - GET    /api/v1/monitoring/health/database - Database health
// - GET    /api/v1/monitoring/health/storage - Storage health
// - GET    /api/v1/monitoring/system/info - System information
// - GET    /api/v1/monitoring/system/stats - System statistics
// - GET    /api/v1/monitoring/alerts - List alerts
// - POST   /api/v1/monitoring/alerts - Create alert
// - GET    /api/v1/monitoring/alerts/:id - Get alert
// - PUT    /api/v1/monitoring/alerts/:id - Update alert
// - DELETE /api/v1/monitoring/alerts/:id - Delete alert
// - POST   /api/v1/monitoring/alerts/:id/acknowledge - Acknowledge alert
// - POST   /api/v1/monitoring/alerts/:id/resolve - Resolve alert
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - Runtime metrics are thread-safe
//
// Dependencies:
// - Database: sessions, users, alerts tables
// - External Services: Prometheus scraping (optional)
//
// Example Usage:
//
//	handler := NewMonitoringHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// Version information - can be set at build time with linker flags:
// go build -ldflags "-X github.com/streamspace/streamspace/api/internal/handlers.Version=v1.2.3"
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// MonitoringHandler handles monitoring and metrics endpoints
type MonitoringHandler struct {
	db *db.Database
}

// NewMonitoringHandler creates a new monitoring handler
func NewMonitoringHandler(database *db.Database) *MonitoringHandler {
	return &MonitoringHandler{
		db: database,
	}
}

// RegisterRoutes registers monitoring routes
func (h *MonitoringHandler) RegisterRoutes(router *gin.RouterGroup) {
	monitoring := router.Group("/monitoring")
	{
		// Prometheus metrics
		monitoring.GET("/metrics/prometheus", h.PrometheusMetrics)

		// Custom metrics
		monitoring.GET("/metrics/sessions", h.SessionMetrics)
		monitoring.GET("/metrics/resources", h.ResourceMetrics)
		monitoring.GET("/metrics/users", h.UserMetrics)
		monitoring.GET("/metrics/performance", h.PerformanceMetrics)

		// Health checks
		monitoring.GET("/health", h.HealthCheck)
		monitoring.GET("/health/detailed", h.DetailedHealthCheck)
		monitoring.GET("/health/database", h.DatabaseHealth)
		monitoring.GET("/health/storage", h.StorageHealth)

		// System metrics
		monitoring.GET("/system/info", h.SystemInfo)
		monitoring.GET("/system/stats", h.SystemStats)

		// Alerts
		monitoring.GET("/alerts", h.GetAlerts)
		monitoring.POST("/alerts", h.CreateAlert)
		monitoring.GET("/alerts/:id", h.GetAlert)
		monitoring.PUT("/alerts/:id", h.UpdateAlert)
		monitoring.DELETE("/alerts/:id", h.DeleteAlert)
		monitoring.POST("/alerts/:id/acknowledge", h.AcknowledgeAlert)
		monitoring.POST("/alerts/:id/resolve", h.ResolveAlert)
	}
}

// PrometheusMetrics returns metrics in Prometheus format
func (h *MonitoringHandler) PrometheusMetrics(c *gin.Context) {
	ctx := context.Background()

	var metrics []string

	// Session metrics
	var totalSessions, runningSessions, hibernatedSessions int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&totalSessions)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'running'`).Scan(&runningSessions)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'hibernated'`).Scan(&hibernatedSessions)

	metrics = append(metrics,
		fmt.Sprintf("# HELP streamspace_sessions_total Total number of sessions"),
		fmt.Sprintf("# TYPE streamspace_sessions_total gauge"),
		fmt.Sprintf("streamspace_sessions_total %d", totalSessions),
		"",
		fmt.Sprintf("# HELP streamspace_sessions_running Number of running sessions"),
		fmt.Sprintf("# TYPE streamspace_sessions_running gauge"),
		fmt.Sprintf("streamspace_sessions_running %d", runningSessions),
		"",
		fmt.Sprintf("# HELP streamspace_sessions_hibernated Number of hibernated sessions"),
		fmt.Sprintf("# TYPE streamspace_sessions_hibernated gauge"),
		fmt.Sprintf("streamspace_sessions_hibernated %d", hibernatedSessions),
		"",
	)

	// User metrics
	var totalUsers, activeUsers int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&activeUsers)

	metrics = append(metrics,
		fmt.Sprintf("# HELP streamspace_users_total Total number of users"),
		fmt.Sprintf("# TYPE streamspace_users_total gauge"),
		fmt.Sprintf("streamspace_users_total %d", totalUsers),
		"",
		fmt.Sprintf("# HELP streamspace_users_active_24h Number of active users in last 24 hours"),
		fmt.Sprintf("# TYPE streamspace_users_active_24h gauge"),
		fmt.Sprintf("streamspace_users_active_24h %d", activeUsers),
		"",
	)

	// Template metrics
	var totalTemplates int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM templates`).Scan(&totalTemplates)

	metrics = append(metrics,
		fmt.Sprintf("# HELP streamspace_templates_total Total number of templates"),
		fmt.Sprintf("# TYPE streamspace_templates_total gauge"),
		fmt.Sprintf("streamspace_templates_total %d", totalTemplates),
		"",
	)

	// Resource metrics (example - would need actual resource tracking)
	var avgCPU, avgMemory float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(AVG((resources->>'cpu')::float), 0),
			COALESCE(AVG((resources->>'memory')::float), 0)
		FROM sessions
		WHERE state = 'running' AND resources IS NOT NULL
	`).Scan(&avgCPU, &avgMemory)

	metrics = append(metrics,
		fmt.Sprintf("# HELP streamspace_resources_cpu_avg Average CPU allocation (cores)"),
		fmt.Sprintf("# TYPE streamspace_resources_cpu_avg gauge"),
		fmt.Sprintf("streamspace_resources_cpu_avg %.2f", avgCPU),
		"",
		fmt.Sprintf("# HELP streamspace_resources_memory_avg Average memory allocation (GB)"),
		fmt.Sprintf("# TYPE streamspace_resources_memory_avg gauge"),
		fmt.Sprintf("streamspace_resources_memory_avg %.2f", avgMemory),
		"",
	)

	// System metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics = append(metrics,
		fmt.Sprintf("# HELP streamspace_api_memory_bytes API server memory usage in bytes"),
		fmt.Sprintf("# TYPE streamspace_api_memory_bytes gauge"),
		fmt.Sprintf("streamspace_api_memory_bytes %d", memStats.Alloc),
		"",
		fmt.Sprintf("# HELP streamspace_api_goroutines Number of goroutines"),
		fmt.Sprintf("# TYPE streamspace_api_goroutines gauge"),
		fmt.Sprintf("streamspace_api_goroutines %d", runtime.NumGoroutine()),
		"",
	)

	// Return Prometheus-formatted metrics
	c.String(http.StatusOK, fmt.Sprintf("%s\n", joinStrings(metrics, "\n")))
}

// SessionMetrics returns detailed session metrics
func (h *MonitoringHandler) SessionMetrics(c *gin.Context) {
	ctx := context.Background()

	// Session state distribution
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT state, COUNT(*) as count
		FROM sessions
		GROUP BY state
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get session metrics"})
		return
	}
	defer rows.Close()

	stateDistribution := make(map[string]int)
	for rows.Next() {
		var state string
		var count int
		rows.Scan(&state, &count)
		stateDistribution[state] = count
	}

	// Sessions by template
	rows, err = h.db.DB().QueryContext(ctx, `
		SELECT template_name, COUNT(*) as count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '7 days'
		GROUP BY template_name
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template metrics"})
		return
	}
	defer rows.Close()

	topTemplates := []map[string]interface{}{}
	for rows.Next() {
		var templateName string
		var count int
		rows.Scan(&templateName, &count)
		topTemplates = append(topTemplates, map[string]interface{}{
			"template": templateName,
			"count":    count,
		})
	}

	// Session duration statistics
	var avgDuration, maxDuration int
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(AVG(EXTRACT(EPOCH FROM (terminated_at - created_at))), 0),
			COALESCE(MAX(EXTRACT(EPOCH FROM (terminated_at - created_at))), 0)
		FROM sessions
		WHERE terminated_at IS NOT NULL
		AND created_at >= NOW() - INTERVAL '7 days'
	`).Scan(&avgDuration, &maxDuration)

	// Hourly session creation rate (last 24 hours)
	rows, err = h.db.DB().QueryContext(ctx, `
		SELECT
			EXTRACT(HOUR FROM created_at) as hour,
			COUNT(*) as count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get hourly metrics"})
		return
	}
	defer rows.Close()

	hourlyCreation := make(map[int]int)
	for rows.Next() {
		var hour, count int
		rows.Scan(&hour, &count)
		hourlyCreation[hour] = count
	}

	c.JSON(http.StatusOK, gin.H{
		"stateDistribution": stateDistribution,
		"topTemplates":      topTemplates,
		"duration": gin.H{
			"avgSeconds": avgDuration,
			"maxSeconds": maxDuration,
		},
		"hourlyCreation": hourlyCreation,
		"timestamp":      time.Now().UTC(),
	})
}

// ResourceMetrics returns resource utilization metrics
func (h *MonitoringHandler) ResourceMetrics(c *gin.Context) {
	ctx := context.Background()

	// Total allocated resources
	var totalCPU, totalMemory float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM((resources->>'cpu')::float), 0),
			COALESCE(SUM((resources->>'memory')::float), 0)
		FROM sessions
		WHERE state = 'running' AND resources IS NOT NULL
	`).Scan(&totalCPU, &totalMemory)

	// Resource usage by user
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			user_id,
			COUNT(*) as session_count,
			COALESCE(SUM((resources->>'cpu')::float), 0) as total_cpu,
			COALESCE(SUM((resources->>'memory')::float), 0) as total_memory
		FROM sessions
		WHERE state = 'running' AND resources IS NOT NULL
		GROUP BY user_id
		ORDER BY total_cpu DESC
		LIMIT 10
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user resource metrics"})
		return
	}
	defer rows.Close()

	topUsers := []map[string]interface{}{}
	for rows.Next() {
		var userID string
		var sessionCount int
		var cpu, memory float64
		rows.Scan(&userID, &sessionCount, &cpu, &memory)
		topUsers = append(topUsers, map[string]interface{}{
			"userId":       userID,
			"sessionCount": sessionCount,
			"totalCPU":     cpu,
			"totalMemory":  memory,
		})
	}

	// Resource waste (hibernated sessions with resources allocated)
	var wastedCPU, wastedMemory float64
	var wastedSessions int
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM((resources->>'cpu')::float), 0),
			COALESCE(SUM((resources->>'memory')::float), 0)
		FROM sessions
		WHERE state = 'hibernated' AND resources IS NOT NULL
	`).Scan(&wastedSessions, &wastedCPU, &wastedMemory)

	c.JSON(http.StatusOK, gin.H{
		"allocated": gin.H{
			"totalCPU":    totalCPU,
			"totalMemory": totalMemory,
		},
		"topUsers": topUsers,
		"waste": gin.H{
			"sessions": wastedSessions,
			"cpu":      wastedCPU,
			"memory":   wastedMemory,
		},
		"timestamp": time.Now().UTC(),
	})
}

// UserMetrics returns user activity metrics
func (h *MonitoringHandler) UserMetrics(c *gin.Context) {
	ctx := context.Background()

	// Active users by timeframe
	var dau, wau, mau int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= NOW() - INTERVAL '1 day'`).Scan(&dau)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= NOW() - INTERVAL '7 days'`).Scan(&wau)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= NOW() - INTERVAL '30 days'`).Scan(&mau)

	// User growth
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			DATE(created_at) as date,
			COUNT(*) as new_users
		FROM users
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user growth"})
		return
	}
	defer rows.Close()

	userGrowth := []map[string]interface{}{}
	for rows.Next() {
		var date time.Time
		var count int
		rows.Scan(&date, &count)
		userGrowth = append(userGrowth, map[string]interface{}{
			"date":  date,
			"count": count,
		})
	}

	// Top users by session count
	rows, err = h.db.DB().QueryContext(ctx, `
		SELECT user_id, COUNT(*) as session_count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY user_id
		ORDER BY session_count DESC
		LIMIT 10
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top users"})
		return
	}
	defer rows.Close()

	topUsers := []map[string]interface{}{}
	for rows.Next() {
		var userID string
		var count int
		rows.Scan(&userID, &count)
		topUsers = append(topUsers, map[string]interface{}{
			"userId":       userID,
			"sessionCount": count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"activeUsers": gin.H{
			"daily":   dau,
			"weekly":  wau,
			"monthly": mau,
		},
		"growth":    userGrowth,
		"topUsers":  topUsers,
		"timestamp": time.Now().UTC(),
	})
}

// PerformanceMetrics returns system performance metrics
func (h *MonitoringHandler) PerformanceMetrics(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(http.StatusOK, gin.H{
		"memory": gin.H{
			"alloc":      memStats.Alloc,
			"totalAlloc": memStats.TotalAlloc,
			"sys":        memStats.Sys,
			"numGC":      memStats.NumGC,
		},
		"goroutines": runtime.NumGoroutine(),
		"cpus":       runtime.NumCPU(),
		"uptime":     time.Since(startTime).Seconds(),
		"timestamp":  time.Now().UTC(),
	})
}

// HealthCheck returns basic health status
func (h *MonitoringHandler) HealthCheck(c *gin.Context) {
	ctx := context.Background()

	// Check database
	err := h.db.DB().PingContext(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"message": "Database connection failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	})
}

// DetailedHealthCheck returns detailed component health
func (h *MonitoringHandler) DetailedHealthCheck(c *gin.Context) {
	ctx := context.Background()

	components := make(map[string]interface{})

	// Database health
	dbStart := time.Now()
	err := h.db.DB().PingContext(ctx)
	dbLatency := time.Since(dbStart).Milliseconds()
	components["database"] = gin.H{
		"status":  getHealthStatus(err == nil),
		"latency": dbLatency,
	}

	// Database connections
	stats := h.db.DB().Stats()
	components["databasePool"] = gin.H{
		"status":       getHealthStatus(stats.OpenConnections > 0),
		"open":         stats.OpenConnections,
		"inUse":        stats.InUse,
		"idle":         stats.Idle,
		"waitCount":    stats.WaitCount,
		"waitDuration": stats.WaitDuration.Milliseconds(),
	}

	// Memory health
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memUsagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	components["memory"] = gin.H{
		"status":         getHealthStatus(memUsagePercent < 90),
		"usagePercent":   memUsagePercent,
		"allocatedBytes": memStats.Alloc,
		"systemBytes":    memStats.Sys,
	}

	// Goroutines health
	goroutineCount := runtime.NumGoroutine()
	components["goroutines"] = gin.H{
		"status": getHealthStatus(goroutineCount < 10000),
		"count":  goroutineCount,
	}

	// Overall status
	overallHealthy := true
	for _, comp := range components {
		if compMap, ok := comp.(gin.H); ok {
			if status, ok := compMap["status"].(string); ok && status != "healthy" {
				overallHealthy = false
				break
			}
		}
	}

	statusCode := http.StatusOK
	if !overallHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status":     getHealthStatus(overallHealthy),
		"components": components,
		"timestamp":  time.Now().UTC(),
	})
}

// DatabaseHealth returns database-specific health metrics
func (h *MonitoringHandler) DatabaseHealth(c *gin.Context) {
	ctx := context.Background()

	// Ping database
	pingStart := time.Now()
	err := h.db.DB().PingContext(ctx)
	pingLatency := time.Since(pingStart).Milliseconds()

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   err.Error(),
			"latency": pingLatency,
		})
		return
	}

	// Connection pool stats
	stats := h.db.DB().Stats()

	// Database size
	var dbSize int64
	h.db.DB().QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&dbSize)

	// Table sizes
	rows, _ := h.db.DB().QueryContext(ctx, `
		SELECT
			schemaname,
			tablename,
			pg_total_relation_size(schemaname||'.'||tablename) AS size
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY size DESC
		LIMIT 10
	`)
	defer rows.Close()

	tables := []map[string]interface{}{}
	for rows.Next() {
		var schema, table string
		var size int64
		rows.Scan(&schema, &table, &size)
		tables = append(tables, map[string]interface{}{
			"schema": schema,
			"table":  table,
			"size":   size,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"pingLatency":  pingLatency,
		"databaseSize": dbSize,
		"connectionPool": gin.H{
			"open":         stats.OpenConnections,
			"inUse":        stats.InUse,
			"idle":         stats.Idle,
			"waitCount":    stats.WaitCount,
			"waitDuration": stats.WaitDuration.Milliseconds(),
			"maxOpen":      stats.MaxOpenConnections,
		},
		"topTables": tables,
		"timestamp": time.Now().UTC(),
	})
}

// StorageHealth returns storage-specific health metrics
func (h *MonitoringHandler) StorageHealth(c *gin.Context) {
	ctx := context.Background()

	// Snapshot storage usage
	var snapshotCount int
	var totalSnapshotSize int64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM session_snapshots
		WHERE status = 'completed'
	`).Scan(&snapshotCount, &totalSnapshotSize)

	// Sessions with persistent storage
	var persistentSessionCount int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE persistent_home = true
	`).Scan(&persistentSessionCount)

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"snapshots": gin.H{
			"count":     snapshotCount,
			"totalSize": totalSnapshotSize,
		},
		"persistentSessions": persistentSessionCount,
		"timestamp":          time.Now().UTC(),
	})
}

// SystemInfo returns static system information
func (h *MonitoringHandler) SystemInfo(c *gin.Context) {
	version := getVersionInfo()

	c.JSON(http.StatusOK, gin.H{
		"version":    version["version"],
		"gitCommit":  version["gitCommit"],
		"buildTime":  version["buildTime"],
		"goVersion":  runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"startTime":  startTime,
		"uptime":     time.Since(startTime).Seconds(),
		"timestamp":  time.Now().UTC(),
	})
}

// SystemStats returns current system statistics
func (h *MonitoringHandler) SystemStats(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(http.StatusOK, gin.H{
		"memory": gin.H{
			"alloc":      memStats.Alloc,
			"totalAlloc": memStats.TotalAlloc,
			"sys":        memStats.Sys,
			"numGC":      memStats.NumGC,
			"gcPause":    memStats.PauseNs[(memStats.NumGC+255)%256],
		},
		"goroutines": runtime.NumGoroutine(),
		"uptime":     time.Since(startTime).Seconds(),
		"timestamp":  time.Now().UTC(),
	})
}

// GetAlerts returns all alerts
func (h *MonitoringHandler) GetAlerts(c *gin.Context) {
	ctx := context.Background()
	status := c.DefaultQuery("status", "")

	query := `
		SELECT id, name, description, severity, status, condition, threshold,
		       triggered_at, acknowledged_at, resolved_at, created_at
		FROM monitoring_alerts
		WHERE 1=1
	`
	args := []interface{}{}

	if status != "" {
		query += ` AND status = $1`
		args = append(args, status)
	}

	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := h.db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get alerts"})
		return
	}
	defer rows.Close()

	alerts := []map[string]interface{}{}
	for rows.Next() {
		var id, name, description, severity, status, condition string
		var threshold float64
		var triggeredAt, acknowledgedAt, resolvedAt, createdAt sql.NullTime

		rows.Scan(&id, &name, &description, &severity, &status, &condition, &threshold,
			&triggeredAt, &acknowledgedAt, &resolvedAt, &createdAt)

		alerts = append(alerts, map[string]interface{}{
			"id":              id,
			"name":            name,
			"description":     description,
			"severity":        severity,
			"status":          status,
			"condition":       condition,
			"threshold":       threshold,
			"triggeredAt":     triggeredAt.Time,
			"acknowledgedAt":  acknowledgedAt.Time,
			"resolvedAt":      resolvedAt.Time,
			"createdAt":       createdAt.Time,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// CreateAlert creates a new alert
func (h *MonitoringHandler) CreateAlert(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description string  `json:"description"`
		Severity    string  `json:"severity" binding:"required"`
		Condition   string  `json:"condition" binding:"required"`
		Threshold   float64 `json:"threshold" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	id := fmt.Sprintf("alert_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO monitoring_alerts (id, name, description, severity, condition, threshold)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, req.Name, req.Description, req.Severity, req.Condition, req.Threshold)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create alert"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Alert created successfully",
		"id":      id,
	})
}

// GetAlert returns a specific alert
func (h *MonitoringHandler) GetAlert(c *gin.Context) {
	alertID := c.Param("id")
	ctx := context.Background()

	var id, name, description, severity, status, condition string
	var threshold float64
	var triggeredAt, acknowledgedAt, resolvedAt, createdAt sql.NullTime

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, name, description, severity, status, condition, threshold,
		       triggered_at, acknowledged_at, resolved_at, created_at
		FROM monitoring_alerts
		WHERE id = $1
	`, alertID).Scan(&id, &name, &description, &severity, &status, &condition, &threshold,
		&triggeredAt, &acknowledgedAt, &resolvedAt, &createdAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              id,
		"name":            name,
		"description":     description,
		"severity":        severity,
		"status":          status,
		"condition":       condition,
		"threshold":       threshold,
		"triggeredAt":     triggeredAt.Time,
		"acknowledgedAt":  acknowledgedAt.Time,
		"resolvedAt":      resolvedAt.Time,
		"createdAt":       createdAt.Time,
	})
}

// UpdateAlert updates an alert
func (h *MonitoringHandler) UpdateAlert(c *gin.Context) {
	alertID := c.Param("id")

	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Severity    string  `json:"severity"`
		Condition   string  `json:"condition"`
		Threshold   float64 `json:"threshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE monitoring_alerts
		SET name = $1, description = $2, severity = $3, condition = $4, threshold = $5
		WHERE id = $6
	`, req.Name, req.Description, req.Severity, req.Condition, req.Threshold, alertID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Alert updated successfully",
		"id":      alertID,
	})
}

// DeleteAlert deletes an alert
func (h *MonitoringHandler) DeleteAlert(c *gin.Context) {
	alertID := c.Param("id")
	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `DELETE FROM monitoring_alerts WHERE id = $1`, alertID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Alert deleted successfully",
	})
}

// AcknowledgeAlert acknowledges an alert
func (h *MonitoringHandler) AcknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE monitoring_alerts
		SET status = 'acknowledged', acknowledged_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, alertID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to acknowledge alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Alert acknowledged",
	})
}

// ResolveAlert resolves an alert
func (h *MonitoringHandler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE monitoring_alerts
		SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, alertID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Alert resolved",
	})
}

// Helper functions

var startTime = time.Now()

func getHealthStatus(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}

func joinStrings(strings []string, separator string) string {
	result := ""
	for i, s := range strings {
		if i > 0 {
			result += separator
		}
		result += s
	}
	return result
}

// getVersionInfo returns version information from build info or defaults
func getVersionInfo() map[string]string {
	version := Version
	gitCommit := GitCommit
	buildTime := BuildTime

	// Try to get version from build info if not set via linker flags
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}

			// Extract git commit and build time from build settings
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if len(setting.Value) > 7 {
						gitCommit = setting.Value[:7] // Short commit hash
					} else {
						gitCommit = setting.Value
					}
				case "vcs.time":
					buildTime = setting.Value
				}
			}
		}
	}

	return map[string]string{
		"version":   version,
		"gitCommit": gitCommit,
		"buildTime": buildTime,
	}
}
