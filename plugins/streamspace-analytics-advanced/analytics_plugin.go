package main

import ("context"; "database/sql"; "encoding/json"; "fmt"; "time"; "github.com/yourusername/streamspace/api/internal/plugins")

type AnalyticsPlugin struct {
	plugins.BasePlugin
	config AnalyticsConfig
}

type AnalyticsConfig struct {
	Enabled        bool        `json:"enabled"`
	CostModel      CostModel   `json:"costModel"`
	RetentionDays  int         `json:"retentionDays"`
	ReportSchedule ReportSchedule `json:"reportSchedule"`
	Thresholds     Thresholds  `json:"thresholds"`
}

type CostModel struct {
	CPUCostPerHour       float64 `json:"cpuCostPerHour"`
	MemCostPerGBHour     float64 `json:"memCostPerGBHour"`
	StorageCostPerGBMonth float64 `json:"storageCostPerGBMonth"`
}

type ReportSchedule struct {
	DailyEnabled     bool     `json:"dailyEnabled"`
	WeeklyEnabled    bool     `json:"weeklyEnabled"`
	MonthlyEnabled   bool     `json:"monthlyEnabled"`
	EmailRecipients  []string `json:"emailRecipients"`
}

type Thresholds struct {
	ShortSessionMinutes  int `json:"shortSessionMinutes"`
	IdleTimeoutMinutes   int `json:"idleTimeoutMinutes"`
}

func (p *AnalyticsPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("Analytics plugin is disabled")
		return nil
	}

	p.createDatabaseTables(ctx)
	ctx.Logger.Info("Analytics plugin initialized", "retention", p.config.RetentionDays)
	return nil
}

func (p *AnalyticsPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Advanced Analytics plugin loaded")
	return nil
}

func (p *AnalyticsPlugin) RunScheduledJob(ctx *plugins.PluginContext, jobName string) error {
	switch jobName {
	case "generate-daily-report":
		return p.generateDailyReport(ctx)
	case "cleanup-old-analytics":
		return p.cleanupOldAnalytics(ctx)
	}
	return nil
}

func (p *AnalyticsPlugin) createDatabaseTables(ctx *plugins.PluginContext) error {
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS analytics_cache (
		id SERIAL PRIMARY KEY, cache_key VARCHAR(255) UNIQUE,
		data JSONB, expires_at TIMESTAMP, created_at TIMESTAMP DEFAULT NOW()
	)`)
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS analytics_reports (
		id SERIAL PRIMARY KEY, report_type VARCHAR(100), report_date DATE,
		data JSONB, generated_at TIMESTAMP DEFAULT NOW()
	)`)
	return nil
}

// GetUsageTrends returns time-series usage data
func (p *AnalyticsPlugin) GetUsageTrends(ctx *plugins.PluginContext, days int) (map[string]interface{}, error) {
	if days > 365 {
		days = 365
	}

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

	rows, err := ctx.Database.Query(query)
	if err != nil {
		return nil, err
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

	return map[string]interface{}{
		"trends": trends,
		"period": fmt.Sprintf("%d days", days),
	}, nil
}

// GetUsageByTemplate returns session counts per template
func (p *AnalyticsPlugin) GetUsageByTemplate(ctx *plugins.PluginContext, days int) (map[string]interface{}, error) {
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

	rows, err := ctx.Database.Query(query)
	if err != nil {
		return nil, err
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

	return map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	}, nil
}

// GetSessionDurationAnalytics returns session duration statistics
func (p *AnalyticsPlugin) GetSessionDurationAnalytics(ctx *plugins.PluginContext) (map[string]interface{}, error) {
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

	rows, err := ctx.Database.Query(query)
	if err != nil {
		return nil, err
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
	ctx.Database.QueryRow(`
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

	return map[string]interface{}{
		"buckets": buckets,
		"statistics": map[string]interface{}{
			"avgMinutes":    avgDuration.Float64,
			"medianMinutes": medianDuration.Float64,
			"p90Minutes":    p90Duration.Float64,
			"p95Minutes":    p95Duration.Float64,
		},
		"totalSessions": totalSessions,
	}, nil
}

// GetActiveUsersAnalytics returns active user statistics
func (p *AnalyticsPlugin) GetActiveUsersAnalytics(ctx *plugins.PluginContext) (map[string]interface{}, error) {
	var dau, wau, mau int

	ctx.Database.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '1 day'
	`).Scan(&dau)

	ctx.Database.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '7 days'
	`).Scan(&wau)

	ctx.Database.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&mau)

	var dauWauRatio, dauMauRatio float64
	if wau > 0 {
		dauWauRatio = float64(dau) / float64(wau)
	}
	if mau > 0 {
		dauMauRatio = float64(dau) / float64(mau)
	}

	var powerUsers int
	ctx.Database.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT user_id, COUNT(*) as session_count
			FROM sessions
			WHERE created_at >= NOW() - INTERVAL '30 days'
			GROUP BY user_id
			HAVING COUNT(*) >= 10
		) power_users
	`).Scan(&powerUsers)

	return map[string]interface{}{
		"activeUsers": map[string]interface{}{
			"daily":   dau,
			"weekly":  wau,
			"monthly": mau,
		},
		"engagement": map[string]interface{}{
			"dauWauRatio": dauWauRatio,
			"dauMauRatio": dauMauRatio,
			"powerUsers":  powerUsers,
		},
		"timestamp": time.Now(),
	}, nil
}

// GetPeakUsageTimes returns peak usage analysis
func (p *AnalyticsPlugin) GetPeakUsageTimes(ctx *plugins.PluginContext) (map[string]interface{}, error) {
	hourlyQuery := `
		SELECT
			EXTRACT(HOUR FROM created_at) as hour,
			COUNT(*) as session_count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour
	`

	rows, err := ctx.Database.Query(hourlyQuery)
	if err != nil {
		return nil, err
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

	weekdayQuery := `
		SELECT
			EXTRACT(DOW FROM created_at) as day_of_week,
			COUNT(*) as session_count
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY EXTRACT(DOW FROM created_at)
		ORDER BY day_of_week
	`

	rows2, err := ctx.Database.Query(weekdayQuery)
	if err != nil {
		return nil, err
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

	return map[string]interface{}{
		"hourly":  hourlyData,
		"weekday": weekdayData,
	}, nil
}

// GetCostEstimate returns estimated cost based on resource usage
func (p *AnalyticsPlugin) GetCostEstimate(ctx *plugins.PluginContext) (map[string]interface{}, error) {
	cpuCostPerHour := p.config.CostModel.CPUCostPerHour
	memCostPerHour := p.config.CostModel.MemCostPerGBHour

	var totalSessionHours float64
	ctx.Database.QueryRow(`
		SELECT
			COALESCE(SUM(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 3600), 0)
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&totalSessionHours)

	estimatedCPUCost := totalSessionHours * cpuCostPerHour
	estimatedMemCost := totalSessionHours * 2 * memCostPerHour
	totalEstimatedCost := estimatedCPUCost + estimatedMemCost

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

	rows, err := ctx.Database.Query(userQuery)
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

	return map[string]interface{}{
		"period": "30 days",
		"totalCost": map[string]interface{}{
			"cpu":    estimatedCPUCost,
			"memory": estimatedMemCost,
			"total":  totalEstimatedCost,
		},
		"totalSessionHours": totalSessionHours,
		"costModel": map[string]interface{}{
			"cpuCostPerHour": cpuCostPerHour,
			"memCostPerHour": memCostPerHour,
		},
		"topUserCosts": userCosts,
		"note":         "Costs are estimates based on session duration and resource allocation",
	}, nil
}

// GetResourceWaste identifies idle or underutilized resources
func (p *AnalyticsPlugin) GetResourceWaste(ctx *plugins.PluginContext) (map[string]interface{}, error) {
	shortSessionThreshold := p.config.Thresholds.ShortSessionMinutes * 60
	idleTimeout := p.config.Thresholds.IdleTimeoutMinutes

	var shortSessions int
	ctx.Database.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*)
		FROM sessions
		WHERE created_at >= NOW() - INTERVAL '7 days'
		  AND EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) < %d
	`, shortSessionThreshold)).Scan(&shortSessions)

	var longIdleSessions int
	ctx.Database.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'running'
		  AND last_connection IS NOT NULL
		  AND NOW() - last_connection > INTERVAL '%d minutes'
	`, idleTimeout)).Scan(&longIdleSessions)

	var shouldBeHibernated int
	ctx.Database.QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'running'
		  AND active_connections = 0
		  AND created_at < NOW() - INTERVAL '1 hour'
	`).Scan(&shouldBeHibernated)

	return map[string]interface{}{
		"waste": map[string]interface{}{
			"shortSessions":      shortSessions,
			"longIdleSessions":   longIdleSessions,
			"shouldBeHibernated": shouldBeHibernated,
		},
		"recommendations": []string{
			fmt.Sprintf("Consider auto-hibernation after %d minutes of inactivity (%d sessions affected)", idleTimeout, longIdleSessions),
			fmt.Sprintf("Review short sessions to identify configuration issues (%d sessions)", shortSessions),
			fmt.Sprintf("Enable aggressive hibernation to save resources (%d sessions ready)", shouldBeHibernated),
		},
	}, nil
}

// GetDailyReport returns a comprehensive daily summary
func (p *AnalyticsPlugin) GetDailyReport(ctx *plugins.PluginContext, date string) (map[string]interface{}, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var totalSessions, uniqueUsers, totalConnections int
	var avgDuration sql.NullFloat64

	ctx.Database.QueryRow(`
		SELECT
			COUNT(*),
			COUNT(DISTINCT user_id),
			AVG(EXTRACT(EPOCH FROM (COALESCE(last_disconnect, NOW()) - created_at)) / 60)
		FROM sessions
		WHERE DATE(created_at) = $1
	`, date).Scan(&totalSessions, &uniqueUsers, &avgDuration)

	ctx.Database.QueryRow(`
		SELECT COUNT(*)
		FROM connections
		WHERE DATE(connected_at) = $1
	`, date).Scan(&totalConnections)

	topTemplates := []map[string]interface{}{}
	rows, err := ctx.Database.Query(`
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

	return map[string]interface{}{
		"date": date,
		"summary": map[string]interface{}{
			"totalSessions":      totalSessions,
			"uniqueUsers":        uniqueUsers,
			"totalConnections":   totalConnections,
			"avgDurationMinutes": avgDuration.Float64,
		},
		"topTemplates": topTemplates,
	}, nil
}

func (p *AnalyticsPlugin) generateDailyReport(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Generating daily analytics report")

	if !p.config.ReportSchedule.DailyEnabled {
		ctx.Logger.Info("Daily reports disabled")
		return nil
	}

	date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	report, err := p.GetDailyReport(ctx, date)
	if err != nil {
		return err
	}

	reportJSON, _ := json.Marshal(report)
	_, err = ctx.Database.Exec(`
		INSERT INTO analytics_reports (report_type, report_date, data)
		VALUES ($1, $2, $3)
	`, "daily", date, reportJSON)

	return err
}

func (p *AnalyticsPlugin) cleanupOldAnalytics(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Cleaning up old analytics data", "retention", p.config.RetentionDays)

	ctx.Database.Exec(`
		DELETE FROM analytics_cache
		WHERE expires_at < NOW()
	`)

	ctx.Database.Exec(fmt.Sprintf(`
		DELETE FROM analytics_reports
		WHERE generated_at < NOW() - INTERVAL '%d days'
	`, p.config.RetentionDays))

	return nil
}

func init() {
	plugins.Register("streamspace-analytics-advanced", &AnalyticsPlugin{})
}
