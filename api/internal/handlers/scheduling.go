package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	conflicts, err := h.checkSchedulingConflicts(userID, req.Schedule, req.Timezone, req.TerminateAfter)
	if err == nil && len(conflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "scheduling conflict detected",
			"conflicts": conflicts,
			"message": "This schedule conflicts with existing scheduled sessions. Please choose a different time.",
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

// ============================================================================
// CALENDAR INTEGRATION
// TODO(plugin-migration): Extract calendar functions to streamspace-calendar plugin
// Functions to extract: ConnectCalendar, CalendarOAuthCallback, ListCalendarIntegrations,
// DisconnectCalendar, SyncCalendar, ExportICalendar, and related Google/Outlook helpers
// ============================================================================

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

	// Implement OAuth token exchange based on provider
	switch provider {
	case "google":
		accessToken, refreshToken, email, expiry, err = h.exchangeGoogleOAuthToken(code)
	case "outlook":
		accessToken, refreshToken, email, expiry, err = h.exchangeOutlookOAuthToken(code)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("token exchange failed: %v", err)})
		return
	}

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

	// Implement calendar sync based on provider
	eventsCreated, err := h.syncScheduledSessionsToCalendar(userID, &ci)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("sync failed: %v", err)})
		return
	}

	// Update last synced timestamp
	h.DB.Exec(`
		UPDATE calendar_integrations
		SET last_synced_at = NOW()
		WHERE id = $1
	`, integrationID)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Calendar sync completed",
		"synced_at":      time.Now(),
		"events_created": eventsCreated,
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
func (h *Handler) checkSchedulingConflicts(userID string, schedule ScheduleConfig, timezone string, terminateAfterMinutes int) ([]int64, error) {
	// Calculate the proposed schedule's next run time
	proposedStart, err := h.calculateNextRun(&schedule, timezone)
	if err != nil {
		return nil, err
	}

	// Default session duration is 8 hours if not specified (480 minutes)
	defaultDuration := 8 * time.Hour

	// Use the provided terminate_after or default
	proposedDuration := defaultDuration
	if terminateAfterMinutes > 0 {
		proposedDuration = time.Duration(terminateAfterMinutes) * time.Minute
	}

	// Get all enabled scheduled sessions for this user
	query := `
		SELECT id, schedule, timezone, terminate_after, next_run_at
		FROM scheduled_sessions
		WHERE user_id = $1 AND enabled = true
	`

	rows, err := h.DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query schedules: %w", err)
	}
	defer rows.Close()

	var conflicts []int64

	for rows.Next() {
		var scheduleID int64
		var existingScheduleJSON, existingTimezone string
		var terminateAfter sql.NullInt64
		var nextRunAt time.Time

		err := rows.Scan(&scheduleID, &existingScheduleJSON, &existingTimezone, &terminateAfter, &nextRunAt)
		if err != nil {
			continue
		}

		// Calculate the duration of the existing schedule
		existingDuration := defaultDuration
		if terminateAfter.Valid && terminateAfter.Int64 > 0 {
			existingDuration = time.Duration(terminateAfter.Int64) * time.Minute
		}

		// Check if the time windows overlap
		// Proposed: [proposedStart, proposedStart + proposedDuration]
		// Existing: [nextRunAt, nextRunAt + existingDuration]
		proposedEnd := proposedStart.Add(proposedDuration)
		existingEnd := nextRunAt.Add(existingDuration)

		// Check for overlap: ranges overlap if one starts before the other ends
		if proposedStart.Before(existingEnd) && proposedEnd.After(nextRunAt) {
			conflicts = append(conflicts, scheduleID)
		}
	}

	return conflicts, nil
}

// Get Google Calendar OAuth URL
func (h *Handler) getGoogleCalendarAuthURL(userID string) string {
	// OAuth2 configuration for Google Calendar
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	if clientID == "" {
		clientID = "placeholder-client-id.apps.googleusercontent.com"
	}

	redirectURI := os.Getenv("GOOGLE_OAUTH_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:3000/api/scheduling/calendar/oauth/callback"
	}

	// Google Calendar OAuth scopes
	scopes := "https://www.googleapis.com/auth/calendar.events"

	// Build OAuth URL with proper parameters
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", scopes)
	params.Add("state", userID) // Pass user ID in state for callback
	params.Add("access_type", "offline") // Request refresh token
	params.Add("prompt", "consent") // Force consent screen to ensure refresh token

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// Get Outlook Calendar OAuth URL
func (h *Handler) getOutlookCalendarAuthURL(userID string) string {
	// OAuth2 configuration for Microsoft Outlook
	clientID := os.Getenv("MICROSOFT_OAUTH_CLIENT_ID")
	if clientID == "" {
		clientID = "placeholder-client-id"
	}

	redirectURI := os.Getenv("MICROSOFT_OAUTH_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:3000/api/scheduling/calendar/oauth/callback"
	}

	// Microsoft Calendar OAuth scopes
	scopes := "Calendars.ReadWrite offline_access"

	// Build OAuth URL with proper parameters
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", scopes)
	params.Add("state", userID) // Pass user ID in state for callback

	return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?" + params.Encode()
}

// exchangeGoogleOAuthToken exchanges authorization code for access/refresh tokens
func (h *Handler) exchangeGoogleOAuthToken(code string) (accessToken, refreshToken, email string, expiry time.Time, err error) {
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	redirectURI := os.Getenv("GOOGLE_OAUTH_REDIRECT_URI")

	if clientID == "" || clientSecret == "" {
		return "", "", "", time.Time{}, fmt.Errorf("Google OAuth not configured - set GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET")
	}

	if redirectURI == "" {
		redirectURI = "http://localhost:3000/api/scheduling/calendar/oauth/callback"
	}

	// Build token request payload
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	// Make HTTP POST request to Google OAuth2 token endpoint
	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return "", "", "", time.Time{}, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Get user email from Google userinfo endpoint
	email, err = h.getGoogleUserEmail(tokenResponse.AccessToken)
	if err != nil {
		// If we can't get email, use a placeholder but continue
		email = "unknown@gmail.com"
	}

	// Calculate token expiry time
	expiry = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return tokenResponse.AccessToken, tokenResponse.RefreshToken, email, expiry, nil
}

// getGoogleUserEmail fetches the user's email from Google userinfo API
func (h *Handler) getGoogleUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo request failed with status %d", resp.StatusCode)
	}

	var userInfo struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}

	return userInfo.Email, nil
}

// exchangeOutlookOAuthToken exchanges authorization code for access/refresh tokens
func (h *Handler) exchangeOutlookOAuthToken(code string) (accessToken, refreshToken, email string, expiry time.Time, err error) {
	clientID := os.Getenv("MICROSOFT_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("MICROSOFT_OAUTH_CLIENT_SECRET")
	redirectURI := os.Getenv("MICROSOFT_OAUTH_REDIRECT_URI")

	if clientID == "" || clientSecret == "" {
		return "", "", "", time.Time{}, fmt.Errorf("Microsoft OAuth not configured - set MICROSOFT_OAUTH_CLIENT_ID and MICROSOFT_OAUTH_CLIENT_SECRET")
	}

	if redirectURI == "" {
		redirectURI = "http://localhost:3000/api/scheduling/calendar/oauth/callback"
	}

	// Build token request payload
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")
	data.Set("scope", "Calendars.ReadWrite offline_access User.Read")

	// Make HTTP POST request to Microsoft OAuth2 token endpoint
	resp, err := http.Post(
		"https://login.microsoftonline.com/common/oauth2/v2.0/token",
		"application/x-www-form-urlencoded",
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return "", "", "", time.Time{}, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Get user email from Microsoft Graph API
	email, err = h.getMicrosoftUserEmail(tokenResponse.AccessToken)
	if err != nil {
		// If we can't get email, use a placeholder but continue
		email = "unknown@outlook.com"
	}

	// Calculate token expiry time
	expiry = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return tokenResponse.AccessToken, tokenResponse.RefreshToken, email, expiry, nil
}

// getMicrosoftUserEmail fetches the user's email from Microsoft Graph API
func (h *Handler) getMicrosoftUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user info request failed with status %d", resp.StatusCode)
	}

	var userInfo struct {
		Mail                string `json:"mail"`
		UserPrincipalName   string `json:"userPrincipalName"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}

	// Use mail if available, otherwise use userPrincipalName
	if userInfo.Mail != "" {
		return userInfo.Mail, nil
	}
	return userInfo.UserPrincipalName, nil
}

// syncScheduledSessionsToCalendar syncs user's scheduled sessions to their calendar
func (h *Handler) syncScheduledSessionsToCalendar(userID string, ci *CalendarIntegration) (int, error) {
	// Fetch enabled scheduled sessions for the user
	rows, err := h.DB.Query(`
		SELECT id, name, template_id, schedule, timezone, next_run_at, terminate_after
		FROM scheduled_sessions
		WHERE user_id = $1 AND enabled = true
	`, userID)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch scheduled sessions: %w", err)
	}
	defer rows.Close()

	eventsCreated := 0

	for rows.Next() {
		var id int64
		var name, templateID, scheduleJSON, timezone string
		var nextRunAt time.Time
		var terminateAfter sql.NullInt64

		err := rows.Scan(&id, &name, &templateID, &scheduleJSON, &timezone, &nextRunAt, &terminateAfter)
		if err != nil {
			continue
		}

		// Calculate event duration
		duration := 480 // Default 8 hours in minutes
		if terminateAfter.Valid && terminateAfter.Int64 > 0 {
			duration = int(terminateAfter.Int64)
		}

		// Create calendar event based on provider
		var eventID string
		switch ci.Provider {
		case "google":
			eventID, err = h.createGoogleCalendarEvent(ci, name, templateID, nextRunAt, duration)
		case "outlook":
			eventID, err = h.createOutlookCalendarEvent(ci, name, templateID, nextRunAt, duration)
		default:
			continue
		}

		if err != nil {
			fmt.Printf("Failed to create calendar event for schedule %d: %v\n", id, err)
			continue
		}

		// Store the event ID for future updates/deletion
		_, err = h.DB.Exec(`
			UPDATE scheduled_sessions
			SET calendar_event_id = $1
			WHERE id = $2
		`, eventID, id)

		if err == nil {
			eventsCreated++
		}
	}

	return eventsCreated, nil
}

// createGoogleCalendarEvent creates an event in Google Calendar
func (h *Handler) createGoogleCalendarEvent(ci *CalendarIntegration, title, description string, startTime time.Time, durationMinutes int) (string, error) {
	if ci.AccessToken == "" {
		return "", fmt.Errorf("no access token available")
	}

	// Calculate end time
	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Build event payload for Google Calendar API
	eventPayload := map[string]interface{}{
		"summary":     title,
		"description": fmt.Sprintf("StreamSpace Session: %s\n\n%s", title, description),
		"start": map[string]string{
			"dateTime": startTime.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"end": map[string]string{
			"dateTime": endTime.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"reminders": map[string]interface{}{
			"useDefault": false,
			"overrides": []map[string]interface{}{
				{
					"method":  "popup",
					"minutes": 15,
				},
			},
		},
	}

	// Encode JSON payload
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode event payload: %w", err)
	}

	// Determine calendar ID (use "primary" if not specified)
	calendarID := "primary"
	if ci.CalendarID != "" {
		calendarID = ci.CalendarID
	}

	// Create HTTP request
	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", calendarID)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ci.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make API request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create calendar event: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("calendar event creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get event ID
	var eventResponse struct {
		ID          string `json:"id"`
		HtmlLink    string `json:"htmlLink"`
		Status      string `json:"status"`
	}

	if err := json.Unmarshal(body, &eventResponse); err != nil {
		return "", fmt.Errorf("failed to parse event response: %w", err)
	}

	return eventResponse.ID, nil
}

// createOutlookCalendarEvent creates an event in Outlook Calendar
func (h *Handler) createOutlookCalendarEvent(ci *CalendarIntegration, title, description string, startTime time.Time, durationMinutes int) (string, error) {
	if ci.AccessToken == "" {
		return "", fmt.Errorf("no access token available")
	}

	// Calculate end time
	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Build event payload for Microsoft Graph API
	eventPayload := map[string]interface{}{
		"subject": title,
		"body": map[string]string{
			"contentType": "text",
			"content":     fmt.Sprintf("StreamSpace Session: %s\n\n%s", title, description),
		},
		"start": map[string]string{
			"dateTime": startTime.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"end": map[string]string{
			"dateTime": endTime.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"isReminderOn": true,
		"reminderMinutesBeforeStart": 15,
	}

	// Encode JSON payload
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode event payload: %w", err)
	}

	// Create HTTP request to Microsoft Graph API
	apiURL := "https://graph.microsoft.com/v1.0/me/events"
	if ci.CalendarID != "" {
		apiURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/calendars/%s/events", ci.CalendarID)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ci.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make API request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create calendar event: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("calendar event creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get event ID
	var eventResponse struct {
		ID              string `json:"id"`
		WebLink         string `json:"webLink"`
		ICalUId         string `json:"iCalUId"`
	}

	if err := json.Unmarshal(body, &eventResponse); err != nil {
		return "", fmt.Errorf("failed to parse event response: %w", err)
	}

	return eventResponse.ID, nil
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
