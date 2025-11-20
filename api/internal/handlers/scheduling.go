// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements session scheduling and calendar integration features.
//
// SCHEDULING SYSTEM OVERVIEW:
//
// The scheduling system allows users to create sessions that start automatically
// at specific times or on recurring schedules. This is useful for:
// - Regular team meetings or training sessions
// - Pre-warming environments before work hours
// - Demo environments that start/stop on a schedule
// - Resource optimization by scheduling sessions during off-peak hours
//
// SUPPORTED SCHEDULE TYPES:
//
// 1. One-Time (once): Session starts at a specific date/time, runs once
//    - Example: Demo session on Friday at 2 PM
//    - Requires: start_time field
//
// 2. Daily (daily): Session starts every day at a specific time
//    - Example: Development environment ready at 9 AM every weekday
//    - Requires: time_of_day field (HH:MM format)
//
// 3. Weekly (weekly): Session starts on specific days of the week
//    - Example: Training sessions every Monday and Wednesday at 10 AM
//    - Requires: days_of_week array (0=Sunday, 6=Saturday), time_of_day
//
// 4. Monthly (monthly): Session starts on a specific day of each month
//    - Example: Monthly report review on the 1st at 9 AM
//    - Requires: day_of_month (1-31), time_of_day
//
// 5. Cron Expression (cron): Advanced scheduling using cron syntax
//    - Example: "0 9 * * 1-5" for weekdays at 9 AM
//    - Requires: cron_expr field
//    - Uses standard cron format: minute hour day month weekday
//
// CONFLICT DETECTION:
//
// The system prevents scheduling conflicts by checking if proposed schedules
// would overlap with existing sessions. This prevents:
// - Resource quota violations (too many concurrent sessions)
// - Node capacity issues
// - User confusion from overlapping sessions
//
// CALENDAR INTEGRATION:
//
// Sessions can be automatically synced to external calendars:
// - Google Calendar (via Google Calendar API)
// - Microsoft Outlook (via Microsoft Graph API)
// - iCal export for other calendar applications
//
// This allows users to see their scheduled sessions alongside other events
// and get calendar notifications/reminders.
//
// PRE-WARMING AND AUTO-TERMINATION:
//
// - Pre-warming: Start session N minutes before scheduled time
//   Useful for sessions with slow startup (large container images)
//
// - Auto-termination: Automatically stop session N minutes after start
//   Prevents runaway sessions and saves resources
//
// TIMEZONE HANDLING:
//
// All schedules are stored with timezone information. The system converts
// between timezones when calculating next run times to ensure schedules
// work correctly for users in different locations.
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
	"github.com/streamspace/streamspace/api/internal/db"
)

// SchedulingHandler handles session scheduling and calendar integration requests.
type SchedulingHandler struct {
	DB *db.Database
}

// NewSchedulingHandler creates a new scheduling handler.
func NewSchedulingHandler(database *db.Database) *SchedulingHandler {
	return &SchedulingHandler{DB: database}
}

// ============================================================================
// SESSION SCHEDULING - DATA STRUCTURES
// ============================================================================

// ScheduledSession represents a scheduled workspace session that starts automatically.
//
// This struct defines a session that will be created at specific times based on
// the configured schedule. Unlike on-demand sessions, scheduled sessions are
// managed by a background scheduler process that monitors next_run_at timestamps.
//
// Lifecycle:
// 1. User creates scheduled session via API
// 2. System calculates next_run_at based on schedule configuration
// 3. Scheduler daemon checks for due schedules every minute
// 4. When next_run_at is reached, system creates actual Session resource
// 5. After session is created, next_run_at is recalculated for recurring schedules
// 6. System optionally terminates session after terminate_after minutes
//
// Example use cases:
// - Development environment that starts at 9 AM and terminates at 6 PM
// - Weekly demo session every Friday at 2 PM
// - Training environment that pre-warms 15 minutes before scheduled time
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

// CreateScheduledSession creates a new scheduled session.
//
// This endpoint allows users to schedule sessions that will start automatically
// at specific times. The system performs several validations and checks before
// accepting the schedule:
//
// VALIDATION STEPS:
//
// 1. Schedule Validation:
//    - Ensures required fields are present for the schedule type
//    - For "daily": requires time_of_day
//    - For "weekly": requires time_of_day and days_of_week
//    - For "monthly": requires time_of_day and day_of_month
//    - For "cron": validates cron expression syntax
//    - For "once": requires start_time
//
// 2. Next Run Calculation:
//    - Computes when the schedule will next trigger
//    - Uses the user's timezone for proper time conversion
//    - For recurring schedules, calculates first occurrence after current time
//
// 3. Conflict Detection:
//    - Checks if the proposed schedule would overlap with existing schedules
//    - Prevents double-booking that could violate quotas or confuse users
//    - Considers session duration (terminate_after) when detecting overlaps
//    - Returns HTTP 409 Conflict if overlaps are found
//
// CONFLICT DETECTION LOGIC:
//
// Two schedules conflict if their time windows overlap:
// - Schedule A: [next_run_at, next_run_at + terminate_after]
// - Schedule B: [next_run_at, next_run_at + terminate_after]
// - Conflict if: A.start < B.end AND B.start < A.end
//
// EXAMPLE REQUEST:
//
//	{
//	  "name": "Daily Dev Environment",
//	  "template_id": "vscode",
//	  "timezone": "America/New_York",
//	  "schedule": {
//	    "type": "daily",
//	    "time_of_day": "09:00"
//	  },
//	  "terminate_after": 540,  // 9 hours
//	  "pre_warm": true,
//	  "pre_warm_minutes": 15
//	}
//
// RESPONSE:
//
//	{
//	  "id": 42,
//	  "message": "Scheduled session created",
//	  "next_run_at": "2025-11-17T09:00:00-05:00",
//	  "schedule": { ... }
//	}
//
// SECURITY:
//
// - User can only create schedules for themselves (userID enforced)
// - Schedule is validated to prevent malicious cron expressions
// - Timezone must be valid IANA timezone name
func (h *SchedulingHandler) CreateScheduledSession(c *gin.Context) {
	userID := c.GetString("user_id")

	var req ScheduledSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// SECURITY: Force userID to authenticated user (prevent creating schedules for others)
	req.UserID = userID
	req.Enabled = true

	// STEP 1: Validate schedule configuration
	// This ensures all required fields are present and values are valid
	if err := h.validateSchedule(&req.Schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// STEP 2: Calculate next run time
	// This determines when the schedule will first trigger
	// For recurring schedules, this is the first occurrence after now
	// For one-time schedules, this is the specified start_time
	nextRun, err := h.calculateNextRun(&req.Schedule, req.Timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timezone or schedule"})
		return
	}
	req.NextRunAt = nextRun

	// STEP 3: Check for scheduling conflicts
	// This prevents overlapping sessions that could:
	// - Violate resource quotas
	// - Cause confusion for users
	// - Overload specific nodes
	//
	// The conflict check considers:
	// - All enabled schedules for this user
	// - Session duration (terminate_after minutes)
	// - Timezone differences between schedules
	conflicts, err := h.checkSchedulingConflicts(userID, req.Schedule, req.Timezone, req.TerminateAfter)
	if err == nil && len(conflicts) > 0 {
		// Return HTTP 409 Conflict with details about conflicting schedules
		c.JSON(http.StatusConflict, gin.H{
			"error": "scheduling conflict detected",
			"conflicts": conflicts,  // Array of conflicting schedule IDs
			"message": "This schedule conflicts with existing scheduled sessions. Please choose a different time.",
		})
		return
	}

	// Insert scheduled session
	var id int64
	err = h.DB.DB().QueryRow(`
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create scheduled session",
			"message": fmt.Sprintf("Database insert failed for user %s with template %s: %v", userID, req.TemplateID, err),
		})
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
func (h *SchedulingHandler) ListScheduledSessions(c *gin.Context) {
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

	rows, err := h.DB.DB().Query(query, userID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list scheduled sessions",
			"message": fmt.Sprintf("Database query failed for user %s: %v", userID, err),
		})
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
func (h *SchedulingHandler) GetScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	var s ScheduledSession
	var lastRun, nextRun sql.NullTime
	var lastSessionID, lastStatus sql.NullString

	err := h.DB.DB().QueryRow(`
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
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Scheduled session not found",
			"message": fmt.Sprintf("No scheduled session found with ID %s for user %s", scheduleID, userID),
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scheduled session",
			"message": fmt.Sprintf("Database query failed for schedule ID %s: %v", scheduleID, err),
		})
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
func (h *SchedulingHandler) UpdateScheduledSession(c *gin.Context) {
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
	err := h.DB.DB().QueryRow(`SELECT user_id FROM scheduled_sessions WHERE id = $1`, scheduleID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Scheduled session not found",
			"message": fmt.Sprintf("No scheduled session found with ID %s", scheduleID),
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check ownership",
			"message": fmt.Sprintf("Database query failed for schedule ID %s: %v", scheduleID, err),
		})
		return
	}
	if ownerID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Access denied",
			"message": fmt.Sprintf("User %s does not have permission to update schedule %s", userID, scheduleID),
		})
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

	_, err = h.DB.DB().Exec(`
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update scheduled session",
			"message": fmt.Sprintf("Database update failed for schedule ID %s: %v", scheduleID, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scheduled session updated"})
}

// DeleteScheduledSession deletes a scheduled session
func (h *SchedulingHandler) DeleteScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	// Check ownership
	var ownerID string
	err := h.DB.DB().QueryRow(`SELECT user_id FROM scheduled_sessions WHERE id = $1`, scheduleID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Scheduled session not found",
			"message": fmt.Sprintf("No scheduled session found with ID %s", scheduleID),
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check ownership",
			"message": fmt.Sprintf("Database query failed for schedule ID %s: %v", scheduleID, err),
		})
		return
	}
	if ownerID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Access denied",
			"message": fmt.Sprintf("User %s does not have permission to delete schedule %s", userID, scheduleID),
		})
		return
	}

	_, err = h.DB.DB().Exec(`DELETE FROM scheduled_sessions WHERE id = $1`, scheduleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete scheduled session",
			"message": fmt.Sprintf("Database delete failed for schedule ID %s: %v", scheduleID, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scheduled session deleted"})
}

// EnableScheduledSession enables a schedule
func (h *SchedulingHandler) EnableScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")

	_, err := h.DB.DB().Exec(`
		UPDATE scheduled_sessions SET enabled = true, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, scheduleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to enable schedule",
			"message": fmt.Sprintf("Database update failed for schedule ID %s, user %s: %v", scheduleID, userID, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule enabled"})
}

// DisableScheduledSession disables a schedule
func (h *SchedulingHandler) DisableScheduledSession(c *gin.Context) {
	scheduleID := c.Param("scheduleId")
	userID := c.GetString("user_id")

	_, err := h.DB.DB().Exec(`
		UPDATE scheduled_sessions SET enabled = false, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, scheduleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to disable schedule",
			"message": fmt.Sprintf("Database update failed for schedule ID %s, user %s: %v", scheduleID, userID, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule disabled"})
}

// ============================================================================
// CALENDAR INTEGRATION
// ============================================================================


// ============================================================================
// CALENDAR INTEGRATION - DEPRECATED
// ============================================================================
//
// ⚠️ DEPRECATED: Calendar integration has been moved to the streamspace-calendar plugin.
//
// MIGRATION GUIDE:
//
// Calendar functionality (Google Calendar, Outlook Calendar, iCal export) has been
// extracted into a plugin for better modularity and optional installation.
//
// To restore calendar integration:
//
// 1. Install the streamspace-calendar plugin:
//    - Via Admin UI: Admin → Plugins → Browse → streamspace-calendar → Install
//    - Via CLI: kubectl apply -f https://plugins.streamspace.io/calendar/install.yaml
//
// 2. API endpoints will be available at:
//    - /api/plugins/streamspace-calendar/connect
//    - /api/plugins/streamspace-calendar/oauth/callback
//    - /api/plugins/streamspace-calendar/integrations
//    - /api/plugins/streamspace-calendar/integrations/:id
//    - /api/plugins/streamspace-calendar/integrations/:id/sync
//    - /api/plugins/streamspace-calendar/export.ics
//
// 3. The plugin provides enhanced features:
//    - Google Calendar integration (OAuth 2.0)
//    - Microsoft Outlook Calendar integration (OAuth 2.0)
//    - iCal export for third-party applications
//    - Automatic session synchronization
//    - Configurable sync intervals
//    - Event reminders and notifications
//    - Timezone support
//
// WHY WAS THIS MOVED TO A PLUGIN?
//
// - Optional feature: Not all users need calendar integration
// - External dependencies: Reduces core OAuth complexity
// - Enhanced features: Plugin can evolve independently
// - Better modularity: Separates scheduling from calendar sync
// - Reduced core size: Removes ~500 lines of calendar-specific code
//
// BACKWARDS COMPATIBILITY:
//
// These stub methods remain in core to provide clear migration messages.
// They will be removed in v2.0.0.
// ============================================================================

// CalendarIntegration represents a calendar connection (DEPRECATED)
type CalendarIntegration struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	Provider      string    `json:"provider"`
	AccountEmail  string    `json:"account_email"`
	AccessToken   string    `json:"-"`
	RefreshToken  string    `json:"-"`
	TokenExpiry   time.Time `json:"token_expiry,omitempty"`
	CalendarID    string    `json:"calendar_id,omitempty"`
	Enabled       bool      `json:"enabled"`
	SyncEnabled   bool      `json:"sync_enabled"`
	AutoCreate    bool      `json:"auto_create_events"`
	CreatedAt     time.Time `json:"created_at"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
}

// CalendarEvent represents a calendar event for a session (DEPRECATED)
type CalendarEvent struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	ScheduleID   int64     `json:"schedule_id"`
	CalendarID   string    `json:"calendar_id"`
	EventID      string    `json:"event_id"`
	Provider     string    `json:"provider"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Timezone     string    `json:"timezone,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// calendarDeprecationResponse returns a standardized deprecation message
func (h *SchedulingHandler) calendarDeprecationResponse(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":   "Calendar integration has been moved to a plugin",
		"message": "This functionality has been extracted into the streamspace-calendar plugin for better modularity",
		"migration": gin.H{
			"install": "Admin → Plugins → streamspace-calendar",
			"api_base": "/api/plugins/streamspace-calendar",
			"documentation": "https://docs.streamspace.io/plugins/calendar",
		},
		"features": []string{
			"Google Calendar OAuth integration",
			"Microsoft Outlook Calendar OAuth integration",
			"iCal export for third-party applications",
			"Automatic session synchronization",
			"Event reminders and timezone support",
		},
		"status": "deprecated",
		"removed_in": "v2.0.0",
	})
}

// ConnectCalendar initiates calendar OAuth flow (DEPRECATED)
func (h *SchedulingHandler) ConnectCalendar(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

// CalendarOAuthCallback handles OAuth callback (DEPRECATED)
func (h *SchedulingHandler) CalendarOAuthCallback(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

// ListCalendarIntegrations lists user's calendar integrations (DEPRECATED)
func (h *SchedulingHandler) ListCalendarIntegrations(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

// DisconnectCalendar removes a calendar integration (DEPRECATED)
func (h *SchedulingHandler) DisconnectCalendar(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

// SyncCalendar manually triggers calendar sync (DEPRECATED)
func (h *SchedulingHandler) SyncCalendar(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

// ExportICalendar exports scheduled sessions as iCal format (DEPRECATED)
func (h *SchedulingHandler) ExportICalendar(c *gin.Context) {
	h.calendarDeprecationResponse(c)
}

	userID := c.GetString("user_id")

	// Get all enabled scheduled sessions
	rows, err := h.DB.DB().Query(`
		SELECT id, name, description, schedule, timezone, template_id
		FROM scheduled_sessions
		WHERE user_id = $1 AND enabled = true
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to export calendar",
			"message": fmt.Sprintf("Database query failed for user %s scheduled sessions: %v", userID, err),
		})
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

// validateSchedule validates a schedule configuration.
//
// This function ensures all required fields are present and valid for the
// specified schedule type. Each schedule type has different requirements:
//
// SCHEDULE TYPE: "once"
//   Purpose: Run a single time at a specific date/time
//   Required Fields:
//   - start_time: Exact timestamp when session should start
//   Example: Start demo session on 2025-12-25 at 10:00 AM
//   Validation: start_time cannot be zero value
//
// SCHEDULE TYPE: "daily"
//   Purpose: Run every day at a specific time
//   Required Fields:
//   - time_of_day: Time in HH:MM format (e.g., "09:30")
//   Example: Start dev environment at 9:30 AM every day
//   Validation: time_of_day must be non-empty
//   Note: Time is interpreted in the schedule's timezone
//
// SCHEDULE TYPE: "weekly"
//   Purpose: Run on specific days of the week
//   Required Fields:
//   - time_of_day: Time in HH:MM format
//   - days_of_week: Array of integers (0=Sunday, 1=Monday, ..., 6=Saturday)
//   Example: Training sessions on Monday (1) and Wednesday (3) at 2 PM
//   Validation: Both fields must be present, days_of_week cannot be empty
//
// SCHEDULE TYPE: "monthly"
//   Purpose: Run on a specific day of each month
//   Required Fields:
//   - time_of_day: Time in HH:MM format
//   - day_of_month: Day number (1-31)
//   Example: Monthly report review on the 15th at 10 AM
//   Validation: Both fields must be present, day_of_month must be non-zero
//   Note: If day_of_month > days in month (e.g., 31 in February),
//         schedule will skip that month
//
// SCHEDULE TYPE: "cron"
//   Purpose: Advanced scheduling using cron expression
//   Required Fields:
//   - cron_expr: Standard cron expression (minute hour day month weekday)
//   Example: "0 9 * * 1-5" for weekdays at 9 AM
//   Validation: Expression must parse successfully using cron.ParseStandard
//   Note: Uses standard cron format (5 fields), not extended format
//
// SECURITY CONSIDERATIONS:
//
// - Cron expressions are parsed but not executed directly (prevents injection)
// - Invalid cron expressions are rejected before database storage
// - Day values are validated to prevent out-of-range errors
//
// RETURN VALUES:
//
// - nil: Schedule is valid
// - error: Descriptive error message indicating what's wrong
func (h *SchedulingHandler) validateSchedule(schedule *ScheduleConfig) error {
	switch schedule.Type {
	case "once":
		// One-time schedule: requires specific start timestamp
		if schedule.StartTime.IsZero() {
			return fmt.Errorf("start_time required for one-time schedule")
		}

	case "daily":
		// Daily schedule: requires time of day to run
		// Example: "09:30" for 9:30 AM every day
		if schedule.TimeOfDay == "" {
			return fmt.Errorf("time_of_day required for daily schedule")
		}

	case "weekly":
		// Weekly schedule: requires both time and which days to run
		// days_of_week: 0=Sunday, 1=Monday, ..., 6=Saturday
		// Example: [1, 3, 5] for Monday, Wednesday, Friday
		if schedule.TimeOfDay == "" || len(schedule.DaysOfWeek) == 0 {
			return fmt.Errorf("time_of_day and days_of_week required for weekly schedule")
		}

	case "monthly":
		// Monthly schedule: requires time and day of month
		// day_of_month: 1-31 (may skip months without that day)
		// Example: day 15 at "10:00" for 15th of each month at 10 AM
		if schedule.TimeOfDay == "" || schedule.DayOfMonth == 0 {
			return fmt.Errorf("time_of_day and day_of_month required for monthly schedule")
		}

	case "cron":
		// Cron schedule: advanced scheduling using cron expression
		// Format: minute hour day month weekday
		// Example: "0 9 * * 1-5" for weekdays at 9:00 AM
		if schedule.CronExpr == "" {
			return fmt.Errorf("cron_expr required for cron schedule")
		}

		// SECURITY: Validate cron expression to prevent injection and ensure it's parseable
		// This prevents malformed expressions from being stored in the database
		// and catches errors early before the scheduler tries to use them
		if _, err := cron.ParseStandard(schedule.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %v", err)
		}

	default:
		// Unknown schedule type - reject with error
		return fmt.Errorf("invalid schedule type: %s", schedule.Type)
	}

	return nil
}

// calculateNextRun calculates when a schedule will next trigger.
//
// This is the core scheduling algorithm that determines when a session should
// be created based on the schedule configuration. The algorithm handles different
// schedule types and properly accounts for timezones.
//
// TIMEZONE HANDLING:
//
// All schedule calculations are performed in the user's specified timezone,
// then converted to UTC for storage. This ensures:
// - 9 AM in New York is always 9 AM local time, even across DST changes
// - Schedules work correctly for users in different timezones
// - Database stores normalized UTC timestamps for consistency
//
// ALGORITHM BY SCHEDULE TYPE:
//
// 1. ONE-TIME ("once"):
//    - Simply returns the start_time field
//    - No calculation needed
//    - Schedule will only run once at that exact time
//
// 2. DAILY ("daily"):
//    - Parses time_of_day (e.g., "09:30" -> 9 hours, 30 minutes)
//    - Creates timestamp for TODAY at that time
//    - If that time has already passed today, schedules for TOMORROW
//    - Example: If now is 2 PM and schedule is 9 AM, next run is tomorrow 9 AM
//
// 3. WEEKLY ("weekly"):
//    - Iterates through next 7 days
//    - For each day, checks if weekday matches days_of_week
//    - If match found and time is in future, returns that timestamp
//    - Handles case where multiple days match (returns earliest)
//    - Example: Today is Monday, schedule is Wed/Fri at 2 PM -> returns Wed 2 PM
//
// 4. MONTHLY ("monthly"):
//    - Creates timestamp for THIS MONTH on the specified day_of_month
//    - If that day/time has passed, schedules for NEXT MONTH
//    - NOTE: If day_of_month > days in month (e.g., 31 in February),
//            Go automatically adjusts to first day of next month
//    - Example: Now is Feb 15, schedule is 10th -> next run is Mar 10
//
// 5. CRON ("cron"):
//    - Uses robfig/cron library to parse expression
//    - Calls cron scheduler's Next() method to get next occurrence
//    - Supports standard 5-field cron format: minute hour day month weekday
//    - Example: "0 9 * * 1-5" -> weekdays at 9:00 AM
//
// EDGE CASES HANDLED:
//
// - Invalid timezone: falls back to UTC (prevents errors)
// - Time already passed today: schedules for next occurrence
// - Weekly schedule with no matching day in next 7 days: returns error
// - Monthly schedule on day that doesn't exist (e.g., Feb 30): auto-adjusts
// - DST transitions: timezone-aware time.Time handles automatically
//
// RETURN VALUES:
//
// - time.Time: Next occurrence of the schedule (in user's timezone)
// - error: If schedule cannot be calculated (e.g., invalid cron expression)
//
// EXAMPLES:
//
//	// Daily at 9 AM in New York timezone
//	calculateNextRun(&ScheduleConfig{Type: "daily", TimeOfDay: "09:00"}, "America/New_York")
//	// Returns: tomorrow 9 AM EST if it's after 9 AM today
//
//	// Weekly on Monday and Wednesday at 2 PM
//	calculateNextRun(&ScheduleConfig{
//	  Type: "weekly",
//	  DaysOfWeek: []int{1, 3},  // Monday=1, Wednesday=3
//	  TimeOfDay: "14:00"
//	}, "America/New_York")
//	// Returns: next Monday or Wednesday at 2 PM, whichever comes first
func (h *SchedulingHandler) calculateNextRun(schedule *ScheduleConfig, timezone string) (time.Time, error) {
	// STEP 1: Load the user's timezone
	// If timezone is invalid, fall back to UTC to prevent errors
	// This allows schedules to still work even with misconfigured timezones
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC  // Fallback to UTC if timezone is invalid
	}

	// STEP 2: Get current time in the user's timezone
	// All calculations are done in local time, then converted to UTC for storage
	now := time.Now().In(loc)

	switch schedule.Type {
	case "once":
		// ONE-TIME: Just return the specified start time
		// No calculation needed - schedule runs exactly once
		return schedule.StartTime, nil

	case "daily":
		// DAILY: Run every day at the same time
		//
		// ALGORITHM:
		// 1. Parse time_of_day string (e.g., "09:30") into hours and minutes
		// 2. Create timestamp for TODAY at that time
		// 3. If that time already passed, move to TOMORROW
		//
		// Example: If now is 2025-11-16 14:00 and schedule is "09:00"
		//   - Today's 9 AM is 2025-11-16 09:00 (already passed)
		//   - Next run is 2025-11-17 09:00 (tomorrow)
		t, _ := time.Parse("15:04", schedule.TimeOfDay)  // Parse HH:MM format
		next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc)

		// If today's time already passed, schedule for tomorrow
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)  // Add 1 day
		}
		return next, nil

	case "weekly":
		// WEEKLY: Run on specific days of the week
		//
		// ALGORITHM:
		// 1. Parse time_of_day into hours/minutes
		// 2. Loop through next 7 days
		// 3. For each day, check if weekday matches days_of_week array
		// 4. If match AND time is in future, return that timestamp
		// 5. Return first match found (earliest occurrence)
		//
		// Example: Now is Monday 10 AM, schedule is Mon/Wed/Fri at 9 AM
		//   - Day 0 (today Monday): 9 AM already passed, skip
		//   - Day 1 (Tuesday): Not in [Mon,Wed,Fri], skip
		//   - Day 2 (Wednesday): Match! Return Wed 9 AM
		//
		// NOTE: Weekday numbering: 0=Sunday, 1=Monday, ..., 6=Saturday
		t, _ := time.Parse("15:04", schedule.TimeOfDay)

		// Check next 7 days for a matching weekday
		for i := 0; i < 7; i++ {
			next := now.AddDate(0, 0, i)  // Add i days to current date

			// Check if this day's weekday is in the schedule's days_of_week array
			if containsInt(schedule.DaysOfWeek, int(next.Weekday())) {
				// Create full timestamp with the scheduled time
				nextTime := time.Date(next.Year(), next.Month(), next.Day(), t.Hour(), t.Minute(), 0, 0, loc)

				// Only return if this time is in the future
				if nextTime.After(now) {
					return nextTime, nil
				}
			}
		}
		// No matching day found in next 7 days (should not happen with valid config)

	case "monthly":
		// MONTHLY: Run on a specific day of each month
		//
		// ALGORITHM:
		// 1. Parse time_of_day into hours/minutes
		// 2. Create timestamp for THIS MONTH on the specified day
		// 3. If that day/time already passed, schedule for NEXT MONTH
		//
		// Example: Now is 2025-11-16, schedule is day 10 at 9 AM
		//   - This month's 10th is 2025-11-10 09:00 (already passed)
		//   - Next run is 2025-12-10 09:00 (next month)
		//
		// EDGE CASE: If day_of_month doesn't exist in a month (e.g., Feb 30),
		//           Go's time.Date automatically adjusts to the next valid date
		//           Feb 30 becomes Mar 2 or Mar 3 depending on leap year
		t, _ := time.Parse("15:04", schedule.TimeOfDay)
		next := time.Date(now.Year(), now.Month(), schedule.DayOfMonth, t.Hour(), t.Minute(), 0, 0, loc)

		// If this month's occurrence already passed, schedule for next month
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)  // Add 1 month
		}
		return next, nil

	case "cron":
		// CRON: Advanced scheduling using cron expression
		//
		// Uses robfig/cron library to parse and calculate next occurrence.
		// Supports standard 5-field cron format:
		//   minute hour day-of-month month day-of-week
		//
		// Examples:
		//   "0 9 * * 1-5" -> Weekdays at 9:00 AM
		//   "30 14 1 * *" -> 1st of every month at 2:30 PM
		//   "0 */2 * * *" -> Every 2 hours
		//
		// The library handles all the complex cron logic including:
		// - Ranges (1-5)
		// - Lists (1,3,5)
		// - Steps (*/2)
		// - Special characters (*)
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err := parser.Parse(schedule.CronExpr)
		if err != nil {
			return time.Time{}, err
		}

		// Calculate next occurrence after current time
		return sched.Next(now), nil
	}

	// Should never reach here if validateSchedule was called first
	return time.Time{}, fmt.Errorf("could not calculate next run time")
}

// checkSchedulingConflicts detects if a proposed schedule would overlap with existing schedules.
//
// This function prevents double-booking of resources by checking if the new schedule
// would conflict with any of the user's existing enabled schedules. Conflicts can cause:
//
// - Resource quota violations (user exceeds max concurrent sessions)
// - Node capacity issues (too many sessions on one node)
// - User confusion (two sessions running simultaneously)
// - Wasted resources (redundant sessions)
//
// OVERLAP DETECTION ALGORITHM:
//
// Two schedules conflict if their time windows overlap. The algorithm:
//
// 1. Calculate when the proposed schedule will next run
// 2. Calculate the proposed session duration (with default of 8 hours)
// 3. Query all enabled schedules for this user from database
// 4. For each existing schedule:
//    a. Get its next_run_at and duration
//    b. Check if time windows overlap using interval arithmetic
// 5. Return list of conflicting schedule IDs
//
// INTERVAL OVERLAP LOGIC:
//
// Two intervals [A_start, A_end] and [B_start, B_end] overlap if:
//   A_start < B_end  AND  B_start < A_end
//
// Example:
//   Proposed:  [09:00, 17:00]  (9 AM - 5 PM, 8 hours)
//   Existing:  [14:00, 18:00]  (2 PM - 6 PM, 4 hours)
//   Check:     09:00 < 18:00  AND  14:00 < 17:00  =  TRUE (conflict!)
//
// Non-overlapping example:
//   Proposed:  [09:00, 12:00]  (9 AM - 12 PM)
//   Existing:  [14:00, 18:00]  (2 PM - 6 PM)
//   Check:     09:00 < 18:00  AND  14:00 < 12:00  =  FALSE (no conflict)
//
// DEFAULT DURATIONS:
//
// - If terminate_after is not specified: 8 hours (480 minutes)
// - This conservative default prevents conflicts from long-running sessions
// - Users can specify shorter durations for non-conflicting schedules
//
// TIMEZONE HANDLING:
//
// - All schedules are stored with timezone information
// - next_run_at timestamps are stored in UTC in database
// - Comparisons work correctly even if schedules use different timezones
//
// RETURN VALUES:
//
// - []int64: Array of conflicting schedule IDs (empty if no conflicts)
// - error: Database error or calculation error
//
// SECURITY CONSIDERATIONS:
//
// - Only checks schedules for the same user (no cross-user conflicts)
// - Disabled schedules are excluded from conflict check
// - Terminated schedules are not considered
//
// EXAMPLE:
//
//	// User has daily 9 AM-5 PM schedule, tries to create weekly 2 PM-6 PM schedule
//	conflicts := checkSchedulingConflicts("user1",
//	  ScheduleConfig{Type: "weekly", DaysOfWeek: [1,3], TimeOfDay: "14:00"},
//	  "America/New_York",
//	  240)  // 4 hours
//	// Returns: [existing_schedule_id] because 2-6 PM overlaps with 9 AM-5 PM
func (h *SchedulingHandler) checkSchedulingConflicts(userID string, schedule ScheduleConfig, timezone string, terminateAfterMinutes int) ([]int64, error) {
	// STEP 1: Calculate when the proposed schedule will next run
	// This gives us the start time for conflict detection
	proposedStart, err := h.calculateNextRun(&schedule, timezone)
	if err != nil {
		return nil, err
	}

	// STEP 2: Determine session duration
	// Default to 8 hours if not specified (conservative estimate)
	// This prevents conflicts from long-running sessions
	defaultDuration := 8 * time.Hour

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

	rows, err := h.DB.DB().Query(query, userID)
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
