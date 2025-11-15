package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// AnalyticsHandler handles advanced analytics and reporting
type AnalyticsHandler struct {
	db *db.Database
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(database *db.Database) *AnalyticsHandler {
	return &AnalyticsHandler{
		db: database,
	}
}

// RegisterRoutes registers analytics routes
func (h *AnalyticsHandler) RegisterRoutes(router *gin.RouterGroup) {
	analytics := router.Group("/analytics")
	{
		// Usage analytics
		analytics.GET("/usage/trends", h.GetUsageTrends)
		analytics.GET("/usage/by-template", h.GetUsageByTemplate)
		analytics.GET("/usage/by-user", h.GetUsageByUser)
		analytics.GET("/usage/by-team", h.GetUsageByTeam)

		// Session analytics
		analytics.GET("/sessions/duration", h.GetSessionDurationAnalytics)
		analytics.GET("/sessions/lifecycle", h.GetSessionLifecycleAnalytics)
		analytics.GET("/sessions/peak-times", h.GetPeakUsageTimes)

		// User engagement
		analytics.GET("/engagement/active-users", h.GetActiveUsersAnalytics)
		analytics.GET("/engagement/retention", h.GetUserRetention)
		analytics.GET("/engagement/frequency", h.GetUsageFrequency)

		// Resource analytics
		analytics.GET("/resources/utilization", h.GetResourceUtilization)
		analytics.GET("/resources/trends", h.GetResourceTrends)
		analytics.GET("/resources/waste", h.GetResourceWaste)

		// Cost analytics
		analytics.GET("/cost/estimate", h.GetCostEstimate)
		analytics.GET("/cost/by-team", h.GetCostByTeam)
		analytics.GET("/cost/by-template", h.GetCostByTemplate)

		// Summary reports
		analytics.GET("/reports/daily", h.GetDailyReport)
		analytics.GET("/reports/weekly", h.GetWeeklyReport)
		analytics.GET("/reports/monthly", h.GetMonthlyReport)
	}
}

// GetUsageTrends returns time-series usage data
func (h *AnalyticsHandler) GetUsageTrends(c *gin.Context) {
	ctx := context.Background()

	// Get time range from query (default: last 30 days)
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
		if days > 365 {
			days = 365 // Max 1 year
		}
	}

	// Get daily session counts
	query := fmt.Sprintf(`
		SELECT
			DATE(created_at) as date,
			COUNT(*) as total_sessions,
			COUNT(*) FILTER (WHERE state = 'running') as running_sessions,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT team_id) FILTER (WHERE team_id IS NOT NULL) as teams_active
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '%d days'
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`, days)

	rows, err := h.db.DB().QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	trends := []map[string]interface{}{}
	for rows.Next() {
		var date time.Time
		var totalSessions, runningSessions, uniqueUsers, teamsActive int

		if err := rows.Scan(&date, &totalSessions, &runningSessions, &uniqueUsers, &teamsActive); err != nil {
			continue
		}

		trends = append(trends, map[string]interface{}{
			"date":            date.Format("2006-01-02"),
			"totalSessions":   totalSessions,
			"runningSessions": runningSessions,
			"uniqueUsers":     uniqueUsers,
			"teamsActive":     teamsActive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"period": fmt.Sprintf("%d days", days),
	})
}

// GetUsageByTemplate returns session counts per template
func (h *AnalyticsHandler) GetUsageByTemplate(c *gin.Context) {
	ctx := context.Background()

	// Get time range
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
	}

	query := fmt.Sprintf(`
		SELECT
			template_name,
			COUNT(*) as session_count,
			COUNT(DISTINCT user_id) as unique_users,
			AVG(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at))) as avg_duration_seconds
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '%d days'
		GROUP BY template_name
		ORDER BY session_count DESC
		LIMIT 50
	`, days)

	rows, err := h.db.DB().QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	templates := []map[string]interface{}{}
	for rows.Next() {
		var templateName string
		var sessionCount, uniqueUsers int
		var avgDuration sql.NullFloat64

		if err := rows.Scan(&templateName, &sessionCount, &uniqueUsers, &avgDuration); err != nil {
			continue
		}

		templates = append(templates, map[string]interface{}{
			"templateName":        templateName,
			"sessionCount":        sessionCount,
			"uniqueUsers":         uniqueUsers,
			"avgDurationSeconds":  avgDuration.Float64,
			"avgDurationMinutes":  avgDuration.Float64 / 60,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetSessionDurationAnalytics returns session duration statistics
func (h *AnalyticsHandler) GetSessionDurationAnalytics(c *gin.Context) {
	ctx := context.Background()

	// Duration buckets in minutes
	query := `
		WITH session_durations AS (
			SELECT
				EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 60 as duration_minutes
			FROM sessions
			WHERE created_at >= NOW() - INTERVAL '30 days'
		)
		SELECT
			CASE
				WHEN duration_minutes < 5 THEN '0-5 min'
				WHEN duration_minutes < 15 THEN '5-15 min'
				WHEN duration_minutes < 30 THEN '15-30 min'
				WHEN duration_minutes < 60 THEN '30-60 min'
				WHEN duration_minutes < 120 THEN '1-2 hours'
				WHEN duration_minutes < 240 THEN '2-4 hours'
				WHEN duration_minutes < 480 THEN '4-8 hours'
				ELSE '8+ hours'
			END as duration_bucket,
			COUNT(*) as session_count
		FROM session_durations
		GROUP BY duration_bucket
		ORDER BY
			CASE duration_bucket
				WHEN '0-5 min' THEN 1
				WHEN '5-15 min' THEN 2
				WHEN '15-30 min' THEN 3
				WHEN '30-60 min' THEN 4
				WHEN '1-2 hours' THEN 5
				WHEN '2-4 hours' THEN 6
				WHEN '4-8 hours' THEN 7
				WHEN '8+ hours' THEN 8
			END
	`

	rows, err := h.db.DB().QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	buckets := []map[string]interface{}{}
	totalSessions := 0
	for rows.Next() {
		var bucket string
		var count int

		if err := rows.Scan(&bucket, &count); err != nil {
			continue
		}

		buckets = append(buckets, map[string]interface{}{
			"bucket": bucket,
			"count":  count,
		})
		totalSessions += count
	}

	// Calculate percentages
	for _, bucket := range buckets {
		count := bucket["count"].(int)
		bucket["percentage"] = float64(count) / float64(totalSessions) * 100
	}

	// Get average, median, and percentiles
	var avgDuration, medianDuration, p90Duration, p95Duration sql.NullFloat64
	h.db.DB().QueryRowContext(ctx, `
		WITH session_durations AS (
			SELECT
				EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 60 as duration_minutes
			FROM sessions
			WHERE created_at >= NOW() - INTERVAL '30 days'
		)
		SELECT
			AVG(duration_minutes) as avg,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_minutes) as median,
			PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY duration_minutes) as p90,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_minutes) as p95
		FROM session_durations
	`).Scan(&avgDuration, &medianDuration, &p90Duration, &p95Duration)

	c.JSON(http.StatusOK, gin.H{
		"buckets": buckets,
		"statistics": gin.H{
			"avgMinutes":    avgDuration.Float64,
			"medianMinutes": medianDuration.Float64,
			"p90Minutes":    p90Duration.Float64,
			"p95Minutes":    p95Duration.Float64,
		},
		"totalSessions": totalSessions,
	})
}

// GetActiveUsersAnalytics returns active user statistics
func (h *AnalyticsHandler) GetActiveUsersAnalytics(c *gin.Context) {
	ctx := context.Background()

	// Daily Active Users (DAU), Weekly Active Users (WAU), Monthly Active Users (MAU)
	var dau, wau, mau int

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '1 day'
	`).Scan(&dau)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '7 days'
	`).Scan(&wau)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&mau)

	// Engagement ratios
	var dauWauRatio, dauMauRatio float64
	if wau > 0 {
		dauWauRatio = float64(dau) / float64(wau)
	}
	if mau > 0 {
		dauMauRatio = float64(dau) / float64(mau)
	}

	// Get power users (created 10+ sessions in last 30 days)
	var powerUsers int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT user_id, COUNT(*) as session_count
			FROM sessions
			WHERE created_at >= NOW() - INTERVAL '30 days'
			GROUP BY user_id
			HAVING COUNT(*) >= 10
		) power_users
	`).Scan(&powerUsers)

	c.JSON(http.StatusOK, gin.H{
		"activeUsers": gin.H{
			"daily":   dau,
			"weekly":  wau,
			"monthly": mau,
		},
		"engagement": gin.H{
			"dauWauRatio": dauWauRatio,
			"dauMauRatio": dauMauRatio,
			"powerUsers":  powerUsers,
		},
		"timestamp": time.Now(),
	})
}

// GetPeakUsageTimes returns peak usage analysis by hour and day
func (h *AnalyticsHandler) GetPeakUsageTimes(c *gin.Context) {
	ctx := context.Background()

	// Sessions by hour of day
	hourlyQuery := `
		SELECT
			EXTRACT(HOUR FROM created_at) as hour,
			COUNT(*) as session_count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour
	`

	rows, err := h.db.DB().QueryContext(ctx, hourlyQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	hourlyData := []map[string]interface{}{}
	for rows.Next() {
		var hour int
		var count int
		if err := rows.Scan(&hour, &count); err == nil {
			hourlyData = append(hourlyData, map[string]interface{}{
				"hour":  hour,
				"count": count,
			})
		}
	}

	// Sessions by day of week
	weekdayQuery := `
		SELECT
			EXTRACT(DOW FROM created_at) as day_of_week,
			COUNT(*) as session_count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY EXTRACT(DOW FROM created_at)
		ORDER BY day_of_week
	`

	rows2, err := h.db.DB().QueryContext(ctx, weekdayQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows2.Close()

	weekdayData := []map[string]interface{}{}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	for rows2.Next() {
		var dow int
		var count int
		if err := rows2.Scan(&dow, &count); err == nil {
			weekdayData = append(weekdayData, map[string]interface{}{
				"dayOfWeek": dow,
				"dayName":   dayNames[dow],
				"count":     count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"hourly":  hourlyData,
		"weekday": weekdayData,
	})
}

// GetCostEstimate returns estimated cost based on resource usage
func (h *AnalyticsHandler) GetCostEstimate(c *gin.Context) {
	ctx := context.Background()

	// Cost model (configurable via environment variables)
	// Default: $0.01 per CPU hour, $0.005 per GB memory hour
	cpuCostPerHour := 0.01
	memCostPerHour := 0.005

	// Get total resource usage (simplified - assumes 1 CPU, 2GB per session)
	var totalSessionHours float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 3600), 0)
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&totalSessionHours)

	// Estimate costs
	estimatedCPUCost := totalSessionHours * cpuCostPerHour
	estimatedMemCost := totalSessionHours * 2 * memCostPerHour // 2GB per session
	totalEstimatedCost := estimatedCPUCost + estimatedMemCost

	// Get cost per user (top 10)
	userCosts := []map[string]interface{}{}
	userQuery := `
		SELECT
			user_id,
			SUM(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 3600) as total_hours
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY user_id
		ORDER BY total_hours DESC
		LIMIT 10
	`

	rows, err := h.db.DB().QueryContext(ctx, userQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var userID string
			var hours float64
			if err := rows.Scan(&userID, &hours); err == nil {
				cost := hours * (cpuCostPerHour + 2*memCostPerHour)
				userCosts = append(userCosts, map[string]interface{}{
					"userId":        userID,
					"hours":         hours,
					"estimatedCost": cost,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"period": "30 days",
		"totalCost": gin.H{
			"cpu":    estimatedCPUCost,
			"memory": estimatedMemCost,
			"total":  totalEstimatedCost,
		},
		"totalSessionHours": totalSessionHours,
		"costModel": gin.H{
			"cpuCostPerHour": cpuCostPerHour,
			"memCostPerHour": memCostPerHour,
		},
		"topUserCosts": userCosts,
		"note":         "Costs are estimates based on session duration and resource allocation",
	})
}

// GetResourceWaste identifies idle or underutilized resources
func (h *AnalyticsHandler) GetResourceWaste(c *gin.Context) {
	ctx := context.Background()

	// Find sessions with very short duration (< 5 minutes) - potential waste
	var shortSessions int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '7 days'
		  AND EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) < 300
	`).Scan(&shortSessions)

	// Find sessions idle for more than 30 minutes with no activity
	var longIdleSessions int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'running'
		  AND last_connection IS NOT NULL
		  AND NOW() - last_connection > INTERVAL '30 minutes'
	`).Scan(&longIdleSessions)

	// Sessions that should be hibernated
	var shouldBeHibernated int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'running'
		  AND active_connections = 0
		  AND created_at < NOW() - INTERVAL '1 hour'
	`).Scan(&shouldBeHibernated)

	c.JSON(http.StatusOK, gin.H{
		"waste": gin.H{
			"shortSessions":      shortSessions,
			"longIdleSessions":   longIdleSessions,
			"shouldBeHibernated": shouldBeHibernated,
		},
		"recommendations": []string{
			fmt.Sprintf("Consider auto-hibernation after 30 minutes of inactivity (%d sessions affected)", longIdleSessions),
			fmt.Sprintf("Review short sessions to identify configuration issues (%d sessions)", shortSessions),
			fmt.Sprintf("Enable aggressive hibernation to save resources (%d sessions ready)", shouldBeHibernated),
		},
	})
}

// GetDailyReport returns a comprehensive daily summary
func (h *AnalyticsHandler) GetDailyReport(c *gin.Context) {
	ctx := context.Background()

	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Get daily statistics
	var totalSessions, uniqueUsers, totalConnections int
	var avgDuration sql.NullFloat64

	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(DISTINCT user_id),
			AVG(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 60)
		FROM sessions
		WHERE DATE(created_at) = $1
	`, date).Scan(&totalSessions, &uniqueUsers, &avgDuration)

	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM connections
		WHERE DATE(connected_at) = $1
	`, date).Scan(&totalConnections)

	// Top templates for the day
	topTemplates := []map[string]interface{}{}
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT template_name, COUNT(*) as count
		FROM sessions
		WHERE DATE(created_at) = $1
		GROUP BY template_name
		ORDER BY count DESC
		LIMIT 5
	`, date)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var count int
			if err := rows.Scan(&name, &count); err == nil {
				topTemplates = append(topTemplates, map[string]interface{}{
					"template": name,
					"count":    count,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"date": date,
		"summary": gin.H{
			"totalSessions":      totalSessions,
			"uniqueUsers":        uniqueUsers,
			"totalConnections":   totalConnections,
			"avgDurationMinutes": avgDuration.Float64,
		},
		"topTemplates": topTemplates,
	})
}
