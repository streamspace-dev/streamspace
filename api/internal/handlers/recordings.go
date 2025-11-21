package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
)

// RecordingHandler handles session recording management
type RecordingHandler struct {
	database *db.Database
}

// NewRecordingHandler creates a new recording handler
func NewRecordingHandler(database *db.Database) *RecordingHandler {
	return &RecordingHandler{
		database: database,
	}
}

// Recording represents a session recording
type Recording struct {
	ID              int64      `json:"id"`
	SessionID       string     `json:"session_id"`
	RecordingType   string     `json:"recording_type"`
	StoragePath     string     `json:"storage_path"`
	FileSizeBytes   int64      `json:"file_size_bytes"`
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       *time.Time `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	Status          string     `json:"status"`
	ErrorMessage    *string    `json:"error_message"`
	CreatedBy       *string    `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Computed fields
	SessionName       string  `json:"session_name,omitempty"`
	UserName          string  `json:"user_name,omitempty"`
	FileSizeMB        float64 `json:"file_size_mb,omitempty"`
	DurationFormatted string  `json:"duration_formatted,omitempty"`
}

// RecordingPolicy represents a recording policy
type RecordingPolicy struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	Description       *string                `json:"description"`
	AutoRecord        bool                   `json:"auto_record"`
	RecordingFormat   string                 `json:"recording_format"`
	RetentionDays     int                    `json:"retention_days"`
	ApplyToUsers      map[string]interface{} `json:"apply_to_users"`
	ApplyToTeams      map[string]interface{} `json:"apply_to_teams"`
	ApplyToTemplates  map[string]interface{} `json:"apply_to_templates"`
	RequireReason     bool                   `json:"require_reason"`
	AllowUserPlayback bool                   `json:"allow_user_playback"`
	AllowUserDownload bool                   `json:"allow_user_download"`
	RequireApproval   bool                   `json:"require_approval"`
	NotifyOnRecording bool                   `json:"notify_on_recording"`
	Metadata          map[string]interface{} `json:"metadata"`
	Enabled           bool                   `json:"enabled"`
	Priority          int                    `json:"priority"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// AccessLog represents a recording access log entry
type AccessLog struct {
	ID          int64     `json:"id"`
	RecordingID int64     `json:"recording_id"`
	UserID      *string   `json:"user_id"`
	Action      string    `json:"action"`
	AccessedAt  time.Time `json:"accessed_at"`
	IPAddress   *string   `json:"ip_address"`
	UserAgent   *string   `json:"user_agent"`

	// Computed fields
	UserName string `json:"user_name,omitempty"`
}

// RegisterRoutes registers recording routes
func (h *RecordingHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Recording management
	router.GET("/recordings", h.ListRecordings)
	router.GET("/recordings/:id", h.GetRecording)
	router.GET("/recordings/:id/download", h.DownloadRecording)
	router.DELETE("/recordings/:id", h.DeleteRecording)
	router.GET("/recordings/:id/access-log", h.GetRecordingAccessLog)
	router.POST("/recordings/:id/start", h.StartRecording)
	router.POST("/recordings/:id/stop", h.StopRecording)

	// Recording policy management
	router.GET("/recording-policies", h.ListPolicies)
	router.GET("/recording-policies/:id", h.GetPolicy)
	router.POST("/recording-policies", h.CreatePolicy)
	router.PUT("/recording-policies/:id", h.UpdatePolicy)
	router.DELETE("/recording-policies/:id", h.DeletePolicy)
}

// ListRecordings lists all recordings with filtering
func (h *RecordingHandler) ListRecordings(c *gin.Context) {
	ctx := context.Background()

	// Parse query parameters
	sessionID := c.Query("session_id")
	status := c.Query("status")
	createdBy := c.Query("created_by")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "50")

	// Convert pagination params
	pageInt, _ := strconv.Atoi(page)
	pageSizeInt, _ := strconv.Atoi(pageSize)
	if pageInt < 1 {
		pageInt = 1
	}
	if pageSizeInt < 1 || pageSizeInt > 100 {
		pageSizeInt = 50
	}
	offset := (pageInt - 1) * pageSizeInt

	// Build query
	query := `
		SELECT
			r.id, r.session_id, r.recording_type, r.storage_path,
			r.file_size_bytes, r.duration_seconds, r.started_at, r.ended_at,
			r.status, r.error_message, r.created_by, r.created_at, r.updated_at,
			s.name as session_name,
			u.username as user_name
		FROM session_recordings r
		LEFT JOIN sessions s ON r.session_id = s.id
		LEFT JOIN users u ON r.created_by = u.id
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if sessionID != "" {
		query += fmt.Sprintf(" AND r.session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if status != "" {
		query += fmt.Sprintf(" AND r.status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if createdBy != "" {
		query += fmt.Sprintf(" AND r.created_by = $%d", argCount)
		args = append(args, createdBy)
		argCount++
	}

	if startDate != "" {
		query += fmt.Sprintf(" AND r.created_at >= $%d", argCount)
		args = append(args, startDate)
		argCount++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND r.created_at <= $%d", argCount)
		args = append(args, endDate)
		argCount++
	}

	if search != "" {
		query += fmt.Sprintf(" AND (s.name ILIKE $%d OR u.username ILIKE $%d)", argCount, argCount)
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern)
		argCount++
	}

	// Add sorting
	allowedSorts := map[string]bool{
		"created_at": true, "started_at": true, "ended_at": true,
		"file_size_bytes": true, "duration_seconds": true, "status": true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	query += fmt.Sprintf(" ORDER BY r.%s %s", sortBy, strings.ToUpper(sortOrder))

	// Add pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, pageSizeInt, offset)

	// Execute query
	rows, err := h.database.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	recordings := []Recording{}
	for rows.Next() {
		var r Recording
		err := rows.Scan(
			&r.ID, &r.SessionID, &r.RecordingType, &r.StoragePath,
			&r.FileSizeBytes, &r.DurationSeconds, &r.StartedAt, &r.EndedAt,
			&r.Status, &r.ErrorMessage, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
			&r.SessionName, &r.UserName,
		)
		if err != nil {
			continue
		}

		// Calculate derived fields
		r.FileSizeMB = float64(r.FileSizeBytes) / (1024 * 1024)
		r.DurationFormatted = formatDuration(r.DurationSeconds)

		recordings = append(recordings, r)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM session_recordings r WHERE 1=1`
	if sessionID != "" {
		countQuery += " AND r.session_id = ?"
	}
	if status != "" {
		countQuery += " AND r.status = ?"
	}
	if createdBy != "" {
		countQuery += " AND r.created_by = ?"
	}

	var total int
	countArgs := []interface{}{}
	if sessionID != "" {
		countArgs = append(countArgs, sessionID)
	}
	if status != "" {
		countArgs = append(countArgs, status)
	}
	if createdBy != "" {
		countArgs = append(countArgs, createdBy)
	}

	// Fix count query to use PostgreSQL syntax
	countQueryPg := strings.ReplaceAll(countQuery, "?", "$")
	for i := range countArgs {
		countQueryPg = strings.Replace(countQueryPg, "$", fmt.Sprintf("$%d", i+1), 1)
	}

	h.database.DB().QueryRowContext(ctx, countQueryPg, countArgs...).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"recordings": recordings,
		"pagination": gin.H{
			"page":        pageInt,
			"page_size":   pageSizeInt,
			"total":       total,
			"total_pages": (total + pageSizeInt - 1) / pageSizeInt,
		},
	})
}

// GetRecording gets a specific recording
func (h *RecordingHandler) GetRecording(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	query := `
		SELECT
			r.id, r.session_id, r.recording_type, r.storage_path,
			r.file_size_bytes, r.duration_seconds, r.started_at, r.ended_at,
			r.status, r.error_message, r.created_by, r.created_at, r.updated_at,
			s.name as session_name,
			u.username as user_name
		FROM session_recordings r
		LEFT JOIN sessions s ON r.session_id = s.id
		LEFT JOIN users u ON r.created_by = u.id
		WHERE r.id = $1
	`

	var r Recording
	err := h.database.DB().QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.SessionID, &r.RecordingType, &r.StoragePath,
		&r.FileSizeBytes, &r.DurationSeconds, &r.StartedAt, &r.EndedAt,
		&r.Status, &r.ErrorMessage, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
		&r.SessionName, &r.UserName,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Recording not found",
			Message: fmt.Sprintf("No recording with ID %s", id),
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Calculate derived fields
	r.FileSizeMB = float64(r.FileSizeBytes) / (1024 * 1024)
	r.DurationFormatted = formatDuration(r.DurationSeconds)

	c.JSON(http.StatusOK, r)
}

// DownloadRecording downloads a recording file
func (h *RecordingHandler) DownloadRecording(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	// Get recording details
	var storagePath string
	var sessionID string
	err := h.database.DB().QueryRowContext(ctx,
		"SELECT storage_path, session_id FROM session_recordings WHERE id = $1",
		id,
	).Scan(&storagePath, &sessionID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Recording not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Recording file not found",
			Message: "The recording file does not exist on disk",
		})
		return
	}

	// Log access
	userID := c.GetString("user_id")
	h.logAccess(ctx, id, userID, "download", c.ClientIP(), c.Request.UserAgent())

	// Serve file
	filename := filepath.Base(storagePath)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.File(storagePath)
}

// DeleteRecording deletes a recording
func (h *RecordingHandler) DeleteRecording(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	// Get storage path before deleting
	var storagePath string
	err := h.database.DB().QueryRowContext(ctx,
		"SELECT storage_path FROM session_recordings WHERE id = $1",
		id,
	).Scan(&storagePath)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Recording not found",
		})
		return
	}

	// Delete from database
	_, err = h.database.DB().ExecContext(ctx,
		"DELETE FROM session_recordings WHERE id = $1",
		id,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete recording",
			Message: err.Error(),
		})
		return
	}

	// Delete file from disk
	if storagePath != "" {
		os.Remove(storagePath) // Ignore errors, file might not exist
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Recording deleted successfully",
	})
}

// GetRecordingAccessLog gets access log for a recording
func (h *RecordingHandler) GetRecordingAccessLog(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	query := `
		SELECT
			l.id, l.recording_id, l.user_id, l.action,
			l.accessed_at, l.ip_address, l.user_agent,
			u.username as user_name
		FROM recording_access_log l
		LEFT JOIN users u ON l.user_id = u.id
		WHERE l.recording_id = $1
		ORDER BY l.accessed_at DESC
		LIMIT 100
	`

	rows, err := h.database.DB().QueryContext(ctx, query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	logs := []AccessLog{}
	for rows.Next() {
		var log AccessLog
		err := rows.Scan(
			&log.ID, &log.RecordingID, &log.UserID, &log.Action,
			&log.AccessedAt, &log.IPAddress, &log.UserAgent,
			&log.UserName,
		)
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}

	c.JSON(http.StatusOK, gin.H{
		"access_log": logs,
	})
}

// StartRecording starts a recording for a session
func (h *RecordingHandler) StartRecording(c *gin.Context) {
	// This would integrate with recording service/plugin
	c.JSON(http.StatusNotImplemented, ErrorResponse{
		Error:   "Not implemented",
		Message: "Recording start is handled by the recording service",
	})
}

// StopRecording stops a recording
func (h *RecordingHandler) StopRecording(c *gin.Context) {
	// This would integrate with recording service/plugin
	c.JSON(http.StatusNotImplemented, ErrorResponse{
		Error:   "Not implemented",
		Message: "Recording stop is handled by the recording service",
	})
}

// ListPolicies lists all recording policies
func (h *RecordingHandler) ListPolicies(c *gin.Context) {
	ctx := context.Background()

	enabled := c.Query("enabled")

	query := `
		SELECT
			id, name, description, auto_record, recording_format, retention_days,
			apply_to_users, apply_to_teams, apply_to_templates,
			require_reason, allow_user_playback, allow_user_download,
			require_approval, notify_on_recording, metadata,
			enabled, priority, created_at, updated_at
		FROM recording_policies
		WHERE 1=1
	`

	args := []interface{}{}
	if enabled != "" {
		query += " AND enabled = $1"
		args = append(args, enabled == "true")
	}

	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := h.database.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	policies := []RecordingPolicy{}
	for rows.Next() {
		var p RecordingPolicy
		var applyToUsers, applyToTeams, applyToTemplates, metadata []byte

		err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.AutoRecord, &p.RecordingFormat, &p.RetentionDays,
			&applyToUsers, &applyToTeams, &applyToTemplates,
			&p.RequireReason, &p.AllowUserPlayback, &p.AllowUserDownload,
			&p.RequireApproval, &p.NotifyOnRecording, &metadata,
			&p.Enabled, &p.Priority, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			continue
		}

		// Parse JSONB fields
		if len(applyToUsers) > 0 {
			json.Unmarshal(applyToUsers, &p.ApplyToUsers)
		}
		if len(applyToTeams) > 0 {
			json.Unmarshal(applyToTeams, &p.ApplyToTeams)
		}
		if len(applyToTemplates) > 0 {
			json.Unmarshal(applyToTemplates, &p.ApplyToTemplates)
		}
		if len(metadata) > 0 {
			json.Unmarshal(metadata, &p.Metadata)
		}

		policies = append(policies, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
	})
}

// GetPolicy gets a specific recording policy
func (h *RecordingHandler) GetPolicy(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	query := `
		SELECT
			id, name, description, auto_record, recording_format, retention_days,
			apply_to_users, apply_to_teams, apply_to_templates,
			require_reason, allow_user_playback, allow_user_download,
			require_approval, notify_on_recording, metadata,
			enabled, priority, created_at, updated_at
		FROM recording_policies
		WHERE id = $1
	`

	var p RecordingPolicy
	var applyToUsers, applyToTeams, applyToTemplates, metadata []byte

	err := h.database.DB().QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.AutoRecord, &p.RecordingFormat, &p.RetentionDays,
		&applyToUsers, &applyToTeams, &applyToTemplates,
		&p.RequireReason, &p.AllowUserPlayback, &p.AllowUserDownload,
		&p.RequireApproval, &p.NotifyOnRecording, &metadata,
		&p.Enabled, &p.Priority, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Policy not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Parse JSONB fields
	if len(applyToUsers) > 0 {
		json.Unmarshal(applyToUsers, &p.ApplyToUsers)
	}
	if len(applyToTeams) > 0 {
		json.Unmarshal(applyToTeams, &p.ApplyToTeams)
	}
	if len(applyToTemplates) > 0 {
		json.Unmarshal(applyToTemplates, &p.ApplyToTemplates)
	}
	if len(metadata) > 0 {
		json.Unmarshal(metadata, &p.Metadata)
	}

	c.JSON(http.StatusOK, p)
}

// CreatePolicyRequest represents a policy creation request
type CreatePolicyRequest struct {
	Name              string                 `json:"name" binding:"required"`
	Description       *string                `json:"description"`
	AutoRecord        bool                   `json:"auto_record"`
	RecordingFormat   string                 `json:"recording_format"`
	RetentionDays     int                    `json:"retention_days"`
	ApplyToUsers      map[string]interface{} `json:"apply_to_users"`
	ApplyToTeams      map[string]interface{} `json:"apply_to_teams"`
	ApplyToTemplates  map[string]interface{} `json:"apply_to_templates"`
	RequireReason     bool                   `json:"require_reason"`
	AllowUserPlayback bool                   `json:"allow_user_playback"`
	AllowUserDownload bool                   `json:"allow_user_download"`
	RequireApproval   bool                   `json:"require_approval"`
	NotifyOnRecording bool                   `json:"notify_on_recording"`
	Metadata          map[string]interface{} `json:"metadata"`
	Enabled           bool                   `json:"enabled"`
	Priority          int                    `json:"priority"`
}

// CreatePolicy creates a new recording policy
func (h *RecordingHandler) CreatePolicy(c *gin.Context) {
	ctx := context.Background()

	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert maps to JSON
	applyToUsers, _ := json.Marshal(req.ApplyToUsers)
	applyToTeams, _ := json.Marshal(req.ApplyToTeams)
	applyToTemplates, _ := json.Marshal(req.ApplyToTemplates)
	metadata, _ := json.Marshal(req.Metadata)

	query := `
		INSERT INTO recording_policies (
			name, description, auto_record, recording_format, retention_days,
			apply_to_users, apply_to_teams, apply_to_templates,
			require_reason, allow_user_playback, allow_user_download,
			require_approval, notify_on_recording, metadata,
			enabled, priority
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`

	var id int64
	var createdAt, updatedAt time.Time
	err := h.database.DB().QueryRowContext(ctx, query,
		req.Name, req.Description, req.AutoRecord, req.RecordingFormat, req.RetentionDays,
		applyToUsers, applyToTeams, applyToTemplates,
		req.RequireReason, req.AllowUserPlayback, req.AllowUserDownload,
		req.RequireApproval, req.NotifyOnRecording, metadata,
		req.Enabled, req.Priority,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create policy",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Policy created successfully",
		"id":         id,
		"created_at": createdAt,
		"updated_at": updatedAt,
	})
}

// UpdatePolicy updates a recording policy
func (h *RecordingHandler) UpdatePolicy(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert maps to JSON
	applyToUsers, _ := json.Marshal(req.ApplyToUsers)
	applyToTeams, _ := json.Marshal(req.ApplyToTeams)
	applyToTemplates, _ := json.Marshal(req.ApplyToTemplates)
	metadata, _ := json.Marshal(req.Metadata)

	query := `
		UPDATE recording_policies SET
			name = $1, description = $2, auto_record = $3, recording_format = $4, retention_days = $5,
			apply_to_users = $6, apply_to_teams = $7, apply_to_templates = $8,
			require_reason = $9, allow_user_playback = $10, allow_user_download = $11,
			require_approval = $12, notify_on_recording = $13, metadata = $14,
			enabled = $15, priority = $16, updated_at = CURRENT_TIMESTAMP
		WHERE id = $17
	`

	result, err := h.database.DB().ExecContext(ctx, query,
		req.Name, req.Description, req.AutoRecord, req.RecordingFormat, req.RetentionDays,
		applyToUsers, applyToTeams, applyToTemplates,
		req.RequireReason, req.AllowUserPlayback, req.AllowUserDownload,
		req.RequireApproval, req.NotifyOnRecording, metadata,
		req.Enabled, req.Priority, id,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update policy",
			Message: err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Policy updated successfully",
	})
}

// DeletePolicy deletes a recording policy
func (h *RecordingHandler) DeletePolicy(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	result, err := h.database.DB().ExecContext(ctx,
		"DELETE FROM recording_policies WHERE id = $1",
		id,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete policy",
			Message: err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Policy deleted successfully",
	})
}

// Helper functions

// logAccess logs recording access
func (h *RecordingHandler) logAccess(ctx context.Context, recordingID, userID, action, ipAddress, userAgent string) {
	query := `
		INSERT INTO recording_access_log (recording_id, user_id, action, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
	`
	h.database.DB().ExecContext(ctx, query, recordingID, userID, action, ipAddress, userAgent)
}

// formatDuration formats duration in seconds to human-readable format
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, remainingMinutes, remainingSeconds)
}
