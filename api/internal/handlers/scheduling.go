package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// ============================================================================
// SESSION SCHEDULING
// ============================================================================

// ScheduledSession represents a scheduled workspace session
type ScheduledSession struct {
	ID               int64           `json:"id"`
	UserID           string          `json:"user_id"`
	TemplateID       string          `json:"template_id"`
	Name             string          `json:"name"`
	Description      string          `json:"description,omitempty"`
	Timezone         string          `json:"timezone"`
	Schedule         ScheduleConfig  `json:"schedule"`
	Resources        ResourceConfig  `json:"resources"`
	AutoTerminate    bool            `json:"auto_terminate"`
	TerminateAfter   int             `json:"terminate_after_minutes,omitempty"` // Minutes after start
	PreWarm          bool            `json:"pre_warm"`                           // Start before scheduled time
	PreWarmMinutes   int             `json:"pre_warm_minutes,omitempty"`
	PostCleanup      bool            `json:"post_cleanup"`                       // Cleanup after termination
	Enabled          bool            `json:"enabled"`
	NextRunAt        time.Time       `json:"next_run_at,omitempty"`
	LastRunAt        time.Time       `json:"last_run_at,omitempty"`
	LastSessionID    string          `json:"last_session_id,omitempty"`
	LastRunStatus    string          `json:"last_run_status,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// ScheduleConfig defines when a session should run
type ScheduleConfig struct {
	Type          string    `json:"type"` // "once", "daily", "weekly", "monthly", "cron"
	StartTime     time.Time `json:"start_time,omitempty"`
	CronExpr      string    `json:"cron_expr,omitempty"` // For cron type
	DaysOfWeek    []int     `json:"days_of_week,omitempty"` // 0=Sunday, 1=Monday, etc.
	DayOfMonth    int       `json:"day_of_month,omitempty"` // 1-31
	TimeOfDay     string    `json:"time_of_day,omitempty"`  // HH:MM format
	EndDate       time.Time `json:"end_date,omitempty"`     // When to stop recurring
	Exceptions    []string  `json:"exceptions,omitempty"`   // Dates to skip (YYYY-MM-DD)
}

// ResourceConfig for scheduled sessions
type ResourceConfig struct {
	Memory    string `json:"memory"`
	CPU       string `json:"cpu"`
	Storage   string `json:"storage,omitempty"`
	GPUCount  int    `json:"gpu_count,omitempty"`
}

// CreateScheduledSession creates a new scheduled session
func (h *Handler) CreateScheduledSession(c *gin.Context) {
	userID := c.GetString("user_id")

	var req ScheduledSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.UserID = userID
	req.Enabled = true

	// Validate schedule
	if err := h.validateSchedule(&req.Schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate next run time
	nextRun, err := h.calculateNextRun(&req.Schedule, req.Timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timezone or schedule"})
		return
	}
	req.NextRunAt = nextRun

	// Check for scheduling conflicts
	conflicts, err := h.checkSchedulingConflicts(userID, req.Schedule, req.Timezone)
	if err == nil && len(conflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "scheduling conflict detected",
			"conflicts": conflicts,
		})
		return
	}

	// Insert scheduled session
	var id int64
	err = h.DB.QueryRow(`
		INSERT INTO scheduled_sessions
		(user_id, template_id, name, description, timezone, schedule, resources,
		 auto_terminate, terminate_after, pre_warm, pre_warm_minutes, post_cleanup,
		 enabled, next_run_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`, userID, req.TemplateID, req.Name, req.Description, req.Timezone,
		req.Schedule, req.Resources, req.AutoTerminate, req.TerminateAfter,
		req.PreWarm, req.PreWarmMinutes, req.PostCleanup, req.Enabled,
		req.NextRunAt, req.Metadata).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create scheduled session"})
		return
	}

	req.ID = id

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Scheduled session created",
		"next_run_at": nextRun,
		"schedule": req,
	})
}

// ListScheduledSessions lists all scheduled sessions for a user
func (h *Handler) ListScheduledSessions(c *gin.Context) {
	userID := c.GetString("user_id")
	role := c.GetString("role")

	// Admins can see all, users only their own
	query := `
		SELECT id, user_id, template_id, name, description, timezone, schedule,
		       resources, auto_terminate, terminate_after, pre_warm, pre_warm_minutes,
		       post_cleanup, enabled, next_run_at, last_run_at, last_session_id,
		       last_run_status, metadata, created_at, updated_at
		FROM scheduled_sessions
		WHERE user_id = $1 OR $2 = 'admin'
		ORDER BY next_run_at ASC
	`

	rows, err := h.DB.Query(query, userID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	schedules := []ScheduledSession{}
	for rows.Next() {
		var s ScheduledSession
		var lastRun, nextRun sql.NullTime
		var lastSessionID, lastStatus sql.NullString

		err := rows.Scan(&s.ID, &s.UserID, &s.TemplateID, &s.Name, &s.Description,
			&s.Timezone, &s.Schedule, &s.Resources, &s.AutoTerminate, &s.TerminateAfter,
			&s.PreWarm, &s.PreWarmMinutes, &s.PostCleanup, &s.Enabled,
			&nextRun, &lastRun, &lastSessionID, &lastStatus, &s.Metadata,
			&s.CreatedAt, &s.UpdatedAt)

		if err != nil {
			continue
		}

		if nextRun.Valid {
			s.NextRunAt = nextRun.Time
		}
		if lastRun.Valid {
			s.LastRunAt = lastRun.Time
		}
		if lastSessionID.Valid {
			s.LastSessionID = lastSessionID.String
		}
		if lastStatus.Valid {
			s.LastRunStatus = lastStatus.String
		}

		schedules = append(schedules, s)
	}

	c.JSON(http.StatusOK, gin.H{
		"schedules": schedules,
		"count":     len(schedules),
	})
}

// GetScheduledSession gets details of a scheduled session
func (h *Handler) GetScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	var s ScheduledSession
	var lastRun, nextRun sql.NullTime
	var lastSessionID, lastStatus sql.NullString

	err := h.DB.QueryRow(`
		SELECT id, user_id, template_id, name, description, timezone, schedule,
		       resources, auto_terminate, terminate_after, pre_warm, pre_warm_minutes,
		       post_cleanup, enabled, next_run_at, last_run_at, last_session_id,
		       last_run_status, metadata, created_at, updated_at
		FROM scheduled_sessions
		WHERE id = $1 AND (user_id = $2 OR $3 = 'admin')
	`, scheduleID, userID, role).Scan(&s.ID, &s.UserID, &s.TemplateID, &s.Name,
		&s.Description, &s.Timezone, &s.Schedule, &s.Resources, &s.AutoTerminate,
		&s.TerminateAfter, &s.PreWarm, &s.PreWarmMinutes, &s.PostCleanup, &s.Enabled,
		&nextRun, &lastRun, &lastSessionID, &lastStatus, &s.Metadata,
		&s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "scheduled session not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if nextRun.Valid {
		s.NextRunAt = nextRun.Time
	}
	if lastRun.Valid {
		s.LastRunAt = lastRun.Time
	}
	if lastSessionID.Valid {
		s.LastSessionID = lastSessionID.String
	}
	if lastStatus.Valid {
		s.LastRunStatus = lastStatus.String
	}

	c.JSON(http.StatusOK, s)
}

// UpdateScheduledSession updates a scheduled session
func (h *Handler) UpdateScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	var req ScheduledSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM scheduled_sessions WHERE id = $1`, scheduleID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "scheduled session not found"})
		return
	}
	if ownerID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Validate and recalculate next run time if schedule changed
	if req.Schedule.Type != "" {
		if err := h.validateSchedule(&req.Schedule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		nextRun, err := h.calculateNextRun(&req.Schedule, req.Timezone)
		if err == nil {
			req.NextRunAt = nextRun
		}
	}

	_, err = h.DB.Exec(`
		UPDATE scheduled_sessions
		SET name = COALESCE(NULLIF($1, ''), name),
		    description = $2,
		    schedule = COALESCE($3, schedule),
		    resources = COALESCE($4, resources),
		    auto_terminate = COALESCE($5, auto_terminate),
		    terminate_after = COALESCE($6, terminate_after),
		    pre_warm = COALESCE($7, pre_warm),
		    pre_warm_minutes = COALESCE($8, pre_warm_minutes),
		    next_run_at = COALESCE($9, next_run_at),
		    updated_at = NOW()
		WHERE id = $10
	`, req.Name, req.Description, req.Schedule, req.Resources, req.AutoTerminate,
		req.TerminateAfter, req.PreWarm, req.PreWarmMinutes, req.NextRunAt, scheduleID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update scheduled session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scheduled session updated"})
}

// DeleteScheduledSession deletes a scheduled session
func (h *Handler) DeleteScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	// Check ownership
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM scheduled_sessions WHERE id = $1`, scheduleID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "scheduled session not found"})
		return
	}
	if ownerID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	_, err = h.DB.Exec(`DELETE FROM scheduled_sessions WHERE id = $1`, scheduleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scheduled session deleted"})
}

// EnableScheduledSession enables a schedule
func (h *Handler) EnableScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")

	_, err := h.DB.Exec(`
		UPDATE scheduled_sessions SET enabled = true, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, scheduleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule enabled"})
}

// DisableScheduledSession disables a schedule
func (h *Handler) DisableScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")

	_, err := h.DB.Exec(`
		UPDATE scheduled_sessions SET enabled = false, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, scheduleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule disabled"})
}

// ============================================================================
// CALENDAR INTEGRATION
// ============================================================================

// CalendarIntegration represents a calendar connection
type CalendarIntegration struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	Provider      string    `json:"provider"` // "google", "outlook", "ical"
	AccountEmail  string    `json:"account_email"`
	AccessToken   string    `json:"access_token,omitempty"`   // Not exposed in API
	RefreshToken  string    `json:"refresh_token,omitempty"`  // Not exposed in API
	TokenExpiry   time.Time `json:"token_expiry,omitempty"`
	CalendarID    string    `json:"calendar_id,omitempty"`
	Enabled       bool      `json:"enabled"`
	SyncEnabled   bool      `json:"sync_enabled"`
	AutoCreate    bool      `json:"auto_create_events"`       // Auto-create calendar events
	AutoUpdate    bool      `json:"auto_update_events"`       // Sync updates
	LastSyncedAt  time.Time `json:"last_synced_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CalendarEvent represents a calendar event for a session
type CalendarEvent struct {
	ID              int64     `json:"id"`
	ScheduleID      int64     `json:"schedule_id"`
	UserID          string    `json:"user_id"`
	Provider        string    `json:"provider"`
	ExternalEventID string    `json:"external_event_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Location        string    `json:"location,omitempty"` // Session URL
	Attendees       []string  `json:"attendees,omitempty"`
	Status          string    `json:"status"` // "pending", "created", "updated", "cancelled"
	CreatedAt       time.Time `json:"created_at"`
}

// ConnectCalendar initiates calendar OAuth flow
func (h *Handler) ConnectCalendar(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Provider string `json:"provider" binding:"required,oneof=google outlook ical"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate OAuth URL
	var authURL string
	switch req.Provider {
	case "google":
		authURL = h.getGoogleCalendarAuthURL(userID)
	case "outlook":
		authURL = h.getOutlookCalendarAuthURL(userID)
	case "ical":
		// iCal doesn't need OAuth, just URL
		authURL = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"provider": req.Provider,
		"auth_url": authURL,
		"message":  "Complete OAuth flow in browser",
	})
}

// CalendarOAuthCallback handles OAuth callback
func (h *Handler) CalendarOAuthCallback(c *gin.Context) {
	provider := c.Query("provider")
	code := c.Query("code")
	state := c.Query("state") // Contains userID

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no authorization code"})
		return
	}

	// Exchange code for tokens (implementation depends on provider)
	var accessToken, refreshToken, email string
	var expiry time.Time

	// TODO: Implement actual OAuth token exchange
	// For Google: use golang.org/x/oauth2/google
	// For Outlook: use Microsoft Graph SDK

	// Store integration
	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO calendar_integrations
		(user_id, provider, account_email, access_token, refresh_token, token_expiry, enabled, sync_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, true, true)
		RETURNING id
	`, state, provider, email, accessToken, refreshToken, expiry).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save integration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Calendar connected successfully",
	})
}

// ListCalendarIntegrations lists user's calendar integrations
func (h *Handler) ListCalendarIntegrations(c *gin.Context) {
	userID := c.GetString("user_id")

	rows, err := h.DB.Query(`
		SELECT id, provider, account_email, calendar_id, enabled, sync_enabled,
		       auto_create_events, auto_update_events, last_synced_at, created_at
		FROM calendar_integrations
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	integrations := []CalendarIntegration{}
	for rows.Next() {
		var ci CalendarIntegration
		var lastSynced sql.NullTime
		var calendarID sql.NullString

		err := rows.Scan(&ci.ID, &ci.Provider, &ci.AccountEmail, &calendarID,
			&ci.Enabled, &ci.SyncEnabled, &ci.AutoCreate, &ci.AutoUpdate,
			&lastSynced, &ci.CreatedAt)

		if err != nil {
			continue
		}

		ci.UserID = userID
		if lastSynced.Valid {
			ci.LastSyncedAt = lastSynced.Time
		}
		if calendarID.Valid {
			ci.CalendarID = calendarID.String
		}

		integrations = append(integrations, ci)
	}

	c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

// DisconnectCalendar removes a calendar integration
func (h *Handler) DisconnectCalendar(c *gin.Context) {
	integrationID := c.Param("integrationId")
	userID := c.GetString("user_id")

	result, err := h.DB.Exec(`
		DELETE FROM calendar_integrations
		WHERE id = $1 AND user_id = $2
	`, integrationID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Calendar disconnected"})
}

// SyncCalendar manually triggers calendar sync
func (h *Handler) SyncCalendar(c *gin.Context) {
	integrationID := c.Param("integrationId")
	userID := c.GetString("user_id")

	// Get integration details
	var ci CalendarIntegration
	err := h.DB.QueryRow(`
		SELECT id, provider, access_token, refresh_token, calendar_id
		FROM calendar_integrations
		WHERE id = $1 AND user_id = $2
	`, integrationID, userID).Scan(&ci.ID, &ci.Provider, &ci.AccessToken,
		&ci.RefreshToken, &ci.CalendarID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}

	// TODO: Implement actual calendar sync based on provider
	// - Fetch scheduled sessions for user
	// - Create/update calendar events
	// - Store event IDs for future updates

	// Update last synced timestamp
	h.DB.Exec(`
		UPDATE calendar_integrations
		SET last_synced_at = NOW()
		WHERE id = $1
	`, integrationID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Calendar sync completed",
		"synced_at":   time.Now(),
		"events_created": 0, // TODO: Return actual count
	})
}

// ExportICalendar exports scheduled sessions as iCal format
func (h *Handler) ExportICalendar(c *gin.Context) {
	userID := c.GetString("user_id")

	// Get all enabled scheduled sessions
	rows, err := h.DB.Query(`
		SELECT id, name, description, schedule, timezone, template_id
		FROM scheduled_sessions
		WHERE user_id = $1 AND enabled = true
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	// Build iCal file
	ical := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//StreamSpace//Scheduled Sessions//EN\r\n"

	for rows.Next() {
		var id int64
		var name, description, timezone, templateID string
		var schedule ScheduleConfig
		rows.Scan(&id, &name, &description, &schedule, &timezone, &templateID)

		// Create VEVENT for each occurrence (simplified)
		ical += "BEGIN:VEVENT\r\n"
		ical += fmt.Sprintf("UID:streamspace-%d@streamspace.local\r\n", id)
		ical += fmt.Sprintf("SUMMARY:%s\r\n", name)
		ical += fmt.Sprintf("DESCRIPTION:%s\r\n", description)
		ical += "END:VEVENT\r\n"
	}

	ical += "END:VCALENDAR\r\n"

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=streamspace-schedule.ics")
	c.String(http.StatusOK, ical)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Validate schedule configuration
func (h *Handler) validateSchedule(schedule *ScheduleConfig) error {
	switch schedule.Type {
	case "once":
		if schedule.StartTime.IsZero() {
			return fmt.Errorf("start_time required for one-time schedule")
		}
	case "daily":
		if schedule.TimeOfDay == "" {
			return fmt.Errorf("time_of_day required for daily schedule")
		}
	case "weekly":
		if schedule.TimeOfDay == "" || len(schedule.DaysOfWeek) == 0 {
			return fmt.Errorf("time_of_day and days_of_week required for weekly schedule")
		}
	case "monthly":
		if schedule.TimeOfDay == "" || schedule.DayOfMonth == 0 {
			return fmt.Errorf("time_of_day and day_of_month required for monthly schedule")
		}
	case "cron":
		if schedule.CronExpr == "" {
			return fmt.Errorf("cron_expr required for cron schedule")
		}
		// Validate cron expression
		if _, err := cron.ParseStandard(schedule.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %v", err)
		}
	default:
		return fmt.Errorf("invalid schedule type: %s", schedule.Type)
	}
	return nil
}

// Calculate next run time for a schedule
func (h *Handler) calculateNextRun(schedule *ScheduleConfig, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	switch schedule.Type {
	case "once":
		return schedule.StartTime, nil

	case "daily":
		// Parse time
		t, _ := time.Parse("15:04", schedule.TimeOfDay)
		next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next, nil

	case "weekly":
		// Find next matching day of week
		t, _ := time.Parse("15:04", schedule.TimeOfDay)
		for i := 0; i < 7; i++ {
			next := now.AddDate(0, 0, i)
			if containsInt(schedule.DaysOfWeek, int(next.Weekday())) {
				nextTime := time.Date(next.Year(), next.Month(), next.Day(), t.Hour(), t.Minute(), 0, 0, loc)
				if nextTime.After(now) {
					return nextTime, nil
				}
			}
		}

	case "monthly":
		t, _ := time.Parse("15:04", schedule.TimeOfDay)
		next := time.Date(now.Year(), now.Month(), schedule.DayOfMonth, t.Hour(), t.Minute(), 0, 0, loc)
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next, nil

	case "cron":
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err := parser.Parse(schedule.CronExpr)
		if err != nil {
			return time.Time{}, err
		}
		return sched.Next(now), nil
	}

	return time.Time{}, fmt.Errorf("could not calculate next run time")
}

// Check for scheduling conflicts
func (h *Handler) checkSchedulingConflicts(userID string, schedule ScheduleConfig, timezone string) ([]int64, error) {
	// TODO: Implement conflict detection
	// Check if user has overlapping schedules
	return []int64{}, nil
}

// Get Google Calendar OAuth URL
func (h *Handler) getGoogleCalendarAuthURL(userID string) string {
	// TODO: Implement Google OAuth URL generation
	return "https://accounts.google.com/o/oauth2/auth?..."
}

// Get Outlook Calendar OAuth URL
func (h *Handler) getOutlookCalendarAuthURL(userID string) string {
	// TODO: Implement Microsoft OAuth URL generation
	return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?..."
}

// Helper: check if int slice contains value
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
