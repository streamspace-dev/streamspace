// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements dashboard statistics and resource usage queries.
//
// DASHBOARD FEATURES:
// - Platform-wide statistics (users, sessions, templates, connections)
// - Resource usage tracking (CPU, memory, storage quotas)
// - Recent activity metrics (last 24 hours)
// - Real-time data from Kubernetes and database
//
// PLATFORM STATISTICS:
// - Total and active user counts
// - Session counts by state (running, hibernated, total)
// - Template count from Kubernetes CRDs
// - Active connection count
// - 24-hour activity metrics
//
// RESOURCE USAGE:
// - Quota utilization per user or platform-wide
// - CPU usage (millicores)
// - Memory usage (bytes/GB)
// - Storage usage (bytes/GB)
// - Session count against quotas
//
// DATA SOURCES:
// - Database: User, session, connection tables
// - Kubernetes: Template CRDs, resource quotas
// - Hybrid queries for comprehensive metrics
//
// API Endpoints:
// - GET /api/v1/dashboard/platform-stats - Overall platform statistics
// - GET /api/v1/dashboard/resource-usage - Resource usage metrics
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - Kubernetes client operations are thread-safe
//
// Dependencies:
// - Database: users, sessions, connections tables
// - Kubernetes: Template CRDs, namespace resources
// - External Services: None
//
// Example Usage:
//
//	handler := NewDashboardHandler(database, k8sClient)
//	// Routes registered in main API router
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
)

// DashboardHandler handles dashboard and resource usage queries
type DashboardHandler struct {
	db        *db.Database
	k8sClient *k8s.Client
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(database *db.Database, k8sClient *k8s.Client) *DashboardHandler {
	return &DashboardHandler{
		db:        database,
		k8sClient: k8sClient,
	}
}

// GetPlatformStats returns overall platform statistics
func (h *DashboardHandler) GetPlatformStats(c *gin.Context) {
	ctx := context.Background()

	// Get user stats
	var totalUsers, activeUsers int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE active = true`).Scan(&activeUsers)

	// Get session stats
	var totalSessions, runningSessions, hibernatedSessions int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&totalSessions)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'running'`).Scan(&runningSessions)
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'hibernated'`).Scan(&hibernatedSessions)

	// Get template count from Kubernetes
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "streamspace"
	}
	templates, _ := h.k8sClient.ListTemplates(ctx, namespace)
	totalTemplates := len(templates)

	// Get connection stats
	var activeConnections int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM connections`).Scan(&activeConnections)

	// Get recent activity (last 24 hours)
	var sessionsCreated24h, connectionsLast24h int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&sessionsCreated24h)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM connections
		WHERE connected_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&connectionsLast24h)

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":  totalUsers,
			"active": activeUsers,
		},
		"sessions": gin.H{
			"total":      totalSessions,
			"running":    runningSessions,
			"hibernated": hibernatedSessions,
		},
		"templates": gin.H{
			"total": totalTemplates,
		},
		"connections": gin.H{
			"active": activeConnections,
		},
		"activity24h": gin.H{
			"sessionsCreated": sessionsCreated24h,
			"connections":     connectionsLast24h,
		},
		"timestamp": time.Now(),
	})
}

// GetResourceUsage returns resource usage statistics
func (h *DashboardHandler) GetResourceUsage(c *gin.Context) {
	ctx := context.Background()

	// Get quota usage from database
	type QuotaUsage struct {
		UsedSessions int    `json:"usedSessions"`
		UsedCPU      string `json:"usedCpu"`
		UsedMemory   string `json:"usedMemory"`
		UsedStorage  string `json:"usedStorage"`
		MaxSessions  int    `json:"maxSessions"`
		MaxCPU       string `json:"maxCpu"`
		MaxMemory    string `json:"maxMemory"`
		MaxStorage   string `json:"maxStorage"`
	}

	// Aggregate all user quotas
	var aggregateUsage QuotaUsage
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(used_sessions), 0) as used_sessions,
			COALESCE(SUM(max_sessions), 0) as max_sessions
		FROM user_quotas
	`).Scan(&aggregateUsage.UsedSessions, &aggregateUsage.MaxSessions)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get resource usage"})
		return
	}

	// Get top resource consumers
	type TopConsumer struct {
		UserID      string `json:"userId"`
		Sessions    int    `json:"sessions"`
		CPUUsage    string `json:"cpuUsage"`
		MemoryUsage string `json:"memoryUsage"`
	}

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT user_id, used_sessions, used_cpu, used_memory
		FROM user_quotas
		WHERE used_sessions > 0
		ORDER BY used_sessions DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()

		topConsumers := []TopConsumer{}
		for rows.Next() {
			var consumer TopConsumer
			if err := rows.Scan(&consumer.UserID, &consumer.Sessions, &consumer.CPUUsage, &consumer.MemoryUsage); err == nil {
				topConsumers = append(topConsumers, consumer)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"aggregate": aggregateUsage,
			"topConsumers": topConsumers,
			"timestamp": time.Now(),
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// GetUserUsageStats returns per-user usage statistics
func (h *DashboardHandler) GetUserUsageStats(c *gin.Context) {
	ctx := context.Background()

	// Pagination
	limit := 50
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	// Get user usage data
	query := `
		SELECT
			u.id,
			u.username,
			u.email,
			COALESCE(uq.used_sessions, 0) as used_sessions,
			COALESCE(uq.max_sessions, 0) as max_sessions,
			COALESCE(uq.used_cpu, '0') as used_cpu,
			COALESCE(uq.used_memory, '0') as used_memory,
			COALESCE(uq.used_storage, '0') as used_storage,
			u.last_login
		FROM users u
		LEFT JOIN user_quotas uq ON u.id = uq.user_id
		WHERE u.active = true
		ORDER BY uq.used_sessions DESC NULLS LAST
		LIMIT $1 OFFSET $2
	`

	rows, err := h.db.DB().QueryContext(ctx, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type UserUsage struct {
		UserID       string     `json:"userId"`
		Username     string     `json:"username"`
		Email        string     `json:"email"`
		UsedSessions int        `json:"usedSessions"`
		MaxSessions  int        `json:"maxSessions"`
		UsedCPU      string     `json:"usedCpu"`
		UsedMemory   string     `json:"usedMemory"`
		UsedStorage  string     `json:"usedStorage"`
		LastLogin    *time.Time `json:"lastLogin,omitempty"`
	}

	users := []UserUsage{}
	for rows.Next() {
		var user UserUsage
		if err := rows.Scan(
			&user.UserID,
			&user.Username,
			&user.Email,
			&user.UsedSessions,
			&user.MaxSessions,
			&user.UsedCPU,
			&user.UsedMemory,
			&user.UsedStorage,
			&user.LastLogin,
		); err == nil {
			users = append(users, user)
		}
	}

	// Get total count
	var total int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE active = true`).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetTemplateUsageStats returns per-template usage statistics
func (h *DashboardHandler) GetTemplateUsageStats(c *gin.Context) {
	ctx := context.Background()

	// Get session count by template
	query := `
		SELECT template_name, COUNT(*) as session_count
		FROM sessions
		GROUP BY template_name
		ORDER BY session_count DESC
		LIMIT 20
	`

	rows, err := h.db.DB().QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TemplateUsage struct {
		TemplateName string `json:"templateName"`
		SessionCount int    `json:"sessionCount"`
	}

	templates := []TemplateUsage{}
	for rows.Next() {
		var tmpl TemplateUsage
		if err := rows.Scan(&tmpl.TemplateName, &tmpl.SessionCount); err == nil {
			templates = append(templates, tmpl)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"timestamp": time.Now(),
	})
}

// GetActivityTimeline returns activity timeline data for charts
func (h *DashboardHandler) GetActivityTimeline(c *gin.Context) {
	ctx := context.Background()

	// Get time range from query (default: last 7 days)
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
		if days > 90 {
			days = 90 // Max 90 days
		}
	}

	// Get session creation timeline
	sessionTimelineQuery := fmt.Sprintf(`
		SELECT
			DATE(created_at) as date,
			COUNT(*) as count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '%d days'
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`, days)

	rows, err := h.db.DB().QueryContext(ctx, sessionTimelineQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TimelinePoint struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}

	sessionTimeline := []TimelinePoint{}
	for rows.Next() {
		var point TimelinePoint
		var date time.Time
		if err := rows.Scan(&date, &point.Count); err == nil {
			point.Date = date.Format("2006-01-02")
			sessionTimeline = append(sessionTimeline, point)
		}
	}

	// Get connection timeline
	connectionTimelineQuery := fmt.Sprintf(`
		SELECT
			DATE(connected_at) as date,
			COUNT(*) as count
		FROM connections
		WHERE connected_at >= NOW() - INTERVAL '%d days'
		GROUP BY DATE(connected_at)
		ORDER BY date DESC
	`, days)

	rows2, err := h.db.DB().QueryContext(ctx, connectionTimelineQuery)
	if err == nil {
		defer rows2.Close()

		connectionTimeline := []TimelinePoint{}
		for rows2.Next() {
			var point TimelinePoint
			var date time.Time
			if err := rows2.Scan(&date, &point.Count); err == nil {
				point.Date = date.Format("2006-01-02")
				connectionTimeline = append(connectionTimeline, point)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"sessions":    sessionTimeline,
			"connections": connectionTimeline,
			"days":        days,
			"timestamp":   time.Now(),
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// GetUserDashboard returns personalized dashboard for the current user
func (h *DashboardHandler) GetUserDashboard(c *gin.Context) {
	ctx := context.Background()

	// Get user ID from context
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

	// Get user's sessions
	var totalSessions, runningSessions, hibernatedSessions int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = $1
	`, userIDStr).Scan(&totalSessions)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND state = 'running'
	`, userIDStr).Scan(&runningSessions)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND state = 'hibernated'
	`, userIDStr).Scan(&hibernatedSessions)

	// Get user's quota
	type UserQuota struct {
		UsedSessions int    `json:"usedSessions"`
		MaxSessions  int    `json:"maxSessions"`
		UsedCPU      string `json:"usedCpu"`
		MaxCPU       string `json:"maxCpu"`
		UsedMemory   string `json:"usedMemory"`
		MaxMemory    string `json:"maxMemory"`
		UsedStorage  string `json:"usedStorage"`
		MaxStorage   string `json:"maxStorage"`
	}

	var quota UserQuota
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT used_sessions, max_sessions, used_cpu, max_cpu,
		       used_memory, max_memory, used_storage, max_storage
		FROM user_quotas
		WHERE user_id = $1
	`, userIDStr).Scan(
		&quota.UsedSessions, &quota.MaxSessions,
		&quota.UsedCPU, &quota.MaxCPU,
		&quota.UsedMemory, &quota.MaxMemory,
		&quota.UsedStorage, &quota.MaxStorage,
	)

	if err != nil {
		// No quota found, use defaults
		quota = UserQuota{
			UsedSessions: totalSessions,
			MaxSessions:  5,
			UsedCPU:      "0",
			MaxCPU:       "4000m",
			UsedMemory:   "0",
			MaxMemory:    "16Gi",
			UsedStorage:  "0",
			MaxStorage:   "100Gi",
		}
	}

	// Get user's recent activity
	var recentConnections int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM connections
		WHERE user_id = $1 AND connected_at >= NOW() - INTERVAL '24 hours'
	`, userIDStr).Scan(&recentConnections)

	c.JSON(http.StatusOK, gin.H{
		"sessions": gin.H{
			"total":      totalSessions,
			"running":    runningSessions,
			"hibernated": hibernatedSessions,
		},
		"quota": quota,
		"recentActivity": gin.H{
			"connections24h": recentConnections,
		},
		"timestamp": time.Now(),
	})
}
