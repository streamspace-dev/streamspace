package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionRecording represents a recorded session
type SessionRecording struct {
	ID            int64                  `json:"id"`
	SessionID     string                 `json:"session_id"`
	UserID        string                 `json:"user_id"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      int                    `json:"duration"` // in seconds
	FileSize      int64                  `json:"file_size"`
	FilePath      string                 `json:"file_path"`
	FileHash      string                 `json:"file_hash"` // SHA-256 for integrity
	Format        string                 `json:"format"`    // "webm", "mp4", "vnc"
	Status        string                 `json:"status"`    // "recording", "completed", "failed", "processing"
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	RetentionDays int                    `json:"retention_days"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
	IsAutomatic   bool                   `json:"is_automatic"`
	Reason        string                 `json:"reason,omitempty"` // "compliance", "training", "support", "user_request"
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// RecordingPlayback represents playback session information
type RecordingPlayback struct {
	RecordingID int64     `json:"recording_id"`
	UserID      string    `json:"user_id"`
	StartedAt   time.Time `json:"started_at"`
	Position    int       `json:"position"` // Current playback position in seconds
	Speed       float64   `json:"speed"`    // Playback speed multiplier
}

// RecordingPolicy represents recording retention and automation policies
type RecordingPolicy struct {
	ID                 int64                  `json:"id"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	AutoRecord         bool                   `json:"auto_record"`
	RecordingFormat    string                 `json:"recording_format"` // "webm", "mp4", "vnc"
	RetentionDays      int                    `json:"retention_days"`
	ApplyToUsers       []string               `json:"apply_to_users"`
	ApplyToTeams       []string               `json:"apply_to_teams"`
	ApplyToTemplates   []string               `json:"apply_to_templates"`
	RequireReason      bool                   `json:"require_reason"`
	AllowUserPlayback  bool                   `json:"allow_user_playback"`
	AllowUserDownload  bool                   `json:"allow_user_download"`
	RequireApproval    bool                   `json:"require_approval"`
	NotifyOnRecording  bool                   `json:"notify_on_recording"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Enabled            bool                   `json:"enabled"`
	Priority           int                    `json:"priority"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// RecordingStats represents recording statistics
type RecordingStats struct {
	TotalRecordings   int64   `json:"total_recordings"`
	ActiveRecordings  int64   `json:"active_recordings"`
	TotalDuration     int64   `json:"total_duration_seconds"`
	TotalSize         int64   `json:"total_size_bytes"`
	AverageDuration   float64 `json:"average_duration_seconds"`
	RecordingsByUser  map[string]int64 `json:"recordings_by_user"`
	RecordingsByMonth map[string]int64 `json:"recordings_by_month"`
	StorageUsed       int64   `json:"storage_used_bytes"`
	StorageLimit      int64   `json:"storage_limit_bytes"`
}

// StartSessionRecording starts recording a session
func (h *Handler) StartSessionRecording(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	var req struct {
		Format        string                 `json:"format" binding:"required,oneof=webm mp4 vnc"`
		Reason        string                 `json:"reason"`
		RetentionDays int                    `json:"retention_days"`
		Metadata      map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user has permission to record this session
	if !h.canRecordSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	// Check if session is already being recorded
	var existingID int64
	err := h.DB.QueryRow(`
		SELECT id FROM session_recordings
		WHERE session_id = $1 AND status = 'recording'
	`, sessionID).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "session already being recorded", "recording_id": existingID})
		return
	}

	// Apply default retention if not specified
	retentionDays := req.RetentionDays
	if retentionDays == 0 {
		retentionDays = h.getDefaultRetentionDays(sessionID)
	}

	// Calculate expiration
	expiresAt := time.Now().Add(time.Duration(retentionDays) * 24 * time.Hour)

	// Create recording record
	var recordingID int64
	err = h.DB.QueryRow(`
		INSERT INTO session_recordings (
			session_id, user_id, start_time, format, status,
			retention_days, expires_at, is_automatic, reason, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, sessionID, userID, time.Now(), req.Format, "recording",
		retentionDays, expiresAt, false, req.Reason, toJSONB(req.Metadata)).Scan(&recordingID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start recording"})
		return
	}

	// Trigger actual VNC recording via WebSocket proxy using database polling
	_, err = h.DB.Exec(`
		INSERT INTO session_recording_controls (session_id, recording_id, action, format, status, created_at)
		VALUES ($1, $2, 'start', $3, 'pending', NOW())
		ON CONFLICT (session_id)
		DO UPDATE SET
			recording_id = EXCLUDED.recording_id,
			action = 'start',
			format = EXCLUDED.format,
			status = 'pending',
			created_at = NOW()
	`, sessionID, recordingID, req.Format)

	if err != nil {
		// Log error but don't fail - recording record is created
		fmt.Printf("Failed to create recording control signal: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"recording_id": recordingID,
		"status":       "recording",
		"started_at":   time.Now(),
		"message":      "recording started successfully",
	})
}

// StopSessionRecording stops an active recording
func (h *Handler) StopSessionRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("recordingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recording ID"})
		return
	}

	userID := c.GetString("user_id")

	// Get recording info
	var recording SessionRecording
	err = h.DB.QueryRow(`
		SELECT id, session_id, user_id, start_time, status, file_path
		FROM session_recordings WHERE id = $1
	`, recordingID).Scan(&recording.ID, &recording.SessionID, &recording.UserID,
		&recording.StartTime, &recording.Status, &recording.FilePath)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	// Check permissions
	if !h.canManageRecording(userID, &recording) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	if recording.Status != "recording" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recording is not active"})
		return
	}

	// Stop recording and calculate metadata
	endTime := time.Now()
	duration := int(endTime.Sub(recording.StartTime).Seconds())

	// Signal WebSocket proxy to stop recording
	_, err = h.DB.Exec(`
		INSERT INTO session_recording_controls (session_id, recording_id, action, status, created_at)
		VALUES ($1, $2, 'stop', 'pending', NOW())
		ON CONFLICT (session_id)
		DO UPDATE SET
			recording_id = EXCLUDED.recording_id,
			action = 'stop',
			status = 'pending',
			created_at = NOW()
	`, recording.SessionID, recordingID)

	if err != nil {
		fmt.Printf("Failed to signal recording stop: %v\n", err)
	}

	// Get file info if recording file exists
	filePath := fmt.Sprintf("/var/streamspace/recordings/%d.webm", recordingID)
	fileSize := int64(0)
	fileHash := ""

	// If file exists, calculate actual size and hash
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, _ := os.Stat(filePath)
		fileSize = fileInfo.Size()
		fileHash = h.calculateFileHash(filePath)
	}

	// Update recording record
	_, err = h.DB.Exec(`
		UPDATE session_recordings
		SET end_time = $1, duration = $2, file_size = $3,
		    file_path = $4, file_hash = $5, status = $6, updated_at = $7
		WHERE id = $8
	`, endTime, duration, fileSize, filePath, fileHash, "completed", time.Now(), recordingID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stop recording"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recording_id": recordingID,
		"status":       "completed",
		"duration":     duration,
		"file_size":    fileSize,
		"message":      "recording stopped successfully",
	})
}

// GetSessionRecording retrieves a recording by ID
func (h *Handler) GetSessionRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("recordingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recording ID"})
		return
	}

	userID := c.GetString("user_id")

	var recording SessionRecording
	var metadata sql.NullString
	err = h.DB.QueryRow(`
		SELECT id, session_id, user_id, start_time, end_time, duration,
		       file_size, file_path, file_hash, format, status, metadata,
		       retention_days, expires_at, is_automatic, reason,
		       created_at, updated_at
		FROM session_recordings WHERE id = $1
	`, recordingID).Scan(&recording.ID, &recording.SessionID, &recording.UserID,
		&recording.StartTime, &recording.EndTime, &recording.Duration,
		&recording.FileSize, &recording.FilePath, &recording.FileHash,
		&recording.Format, &recording.Status, &metadata,
		&recording.RetentionDays, &recording.ExpiresAt, &recording.IsAutomatic,
		&recording.Reason, &recording.CreatedAt, &recording.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve recording"})
		return
	}

	// Parse metadata
	if metadata.Valid && metadata.String != "" {
		json.Unmarshal([]byte(metadata.String), &recording.Metadata)
	}

	// Check permissions
	if !h.canViewRecording(userID, &recording) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	c.JSON(http.StatusOK, recording)
}

// ListSessionRecordings lists recordings with filtering
func (h *Handler) ListSessionRecordings(c *gin.Context) {
	userID := c.GetString("user_id")
	isAdmin := c.GetBool("is_admin")

	// Parse query parameters
	sessionID := c.Query("session_id")
	status := c.Query("status")
	format := c.Query("format")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	// Build query
	query := `SELECT id, session_id, user_id, start_time, end_time, duration,
	                 file_size, file_path, file_hash, format, status, metadata,
	                 retention_days, expires_at, is_automatic, reason,
	                 created_at, updated_at
	          FROM session_recordings WHERE 1=1`
	args := []interface{}{}
	argCount := 1

	// Filter by user if not admin
	if !isAdmin {
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}

	if sessionID != "" {
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if format != "" {
		query += fmt.Sprintf(" AND format = $%d", argCount)
		args = append(args, format)
		argCount++
	}

	if startDate != "" {
		query += fmt.Sprintf(" AND start_time >= $%d", argCount)
		args = append(args, startDate)
		argCount++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND start_time <= $%d", argCount)
		args = append(args, endDate)
		argCount++
	}

	// Count total
	countQuery := strings.Replace(query, "SELECT id, session_id, user_id, start_time, end_time, duration, file_size, file_path, file_hash, format, status, metadata, retention_days, expires_at, is_automatic, reason, created_at, updated_at", "SELECT COUNT(*)", 1)
	var total int
	h.DB.QueryRow(countQuery, args...).Scan(&total)

	// Add pagination
	query += fmt.Sprintf(" ORDER BY start_time DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve recordings"})
		return
	}
	defer rows.Close()

	recordings := []SessionRecording{}
	for rows.Next() {
		var r SessionRecording
		var metadata sql.NullString
		err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.StartTime,
			&r.EndTime, &r.Duration, &r.FileSize, &r.FilePath,
			&r.FileHash, &r.Format, &r.Status, &metadata,
			&r.RetentionDays, &r.ExpiresAt, &r.IsAutomatic,
			&r.Reason, &r.CreatedAt, &r.UpdatedAt)
		if err == nil {
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &r.Metadata)
			}
			recordings = append(recordings, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"recordings":  recordings,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// DownloadRecording downloads a recording file
func (h *Handler) DownloadRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("recordingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recording ID"})
		return
	}

	userID := c.GetString("user_id")

	// Get recording info
	var recording SessionRecording
	err = h.DB.QueryRow(`
		SELECT id, session_id, user_id, file_path, format, status
		FROM session_recordings WHERE id = $1
	`, recordingID).Scan(&recording.ID, &recording.SessionID,
		&recording.UserID, &recording.FilePath, &recording.Format, &recording.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	// Check permissions
	if !h.canDownloadRecording(userID, &recording) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	if recording.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recording not completed"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(recording.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording file not found"})
		return
	}

	// Log download access
	h.logRecordingAccess(recordingID, userID, "download")

	// Serve file
	filename := fmt.Sprintf("recording-%d.%s", recordingID, recording.Format)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.File(recording.FilePath)
}

// StreamRecording streams a recording for playback
func (h *Handler) StreamRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("recordingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recording ID"})
		return
	}

	userID := c.GetString("user_id")

	// Get recording info
	var recording SessionRecording
	err = h.DB.QueryRow(`
		SELECT id, session_id, user_id, file_path, format, status
		FROM session_recordings WHERE id = $1
	`, recordingID).Scan(&recording.ID, &recording.SessionID,
		&recording.UserID, &recording.FilePath, &recording.Format, &recording.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	// Check permissions
	if !h.canViewRecording(userID, &recording) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	if recording.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recording not completed"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(recording.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording file not found"})
		return
	}

	// Log playback access
	h.logRecordingAccess(recordingID, userID, "playback")

	// Stream file with range support for seeking
	c.Header("Content-Type", h.getContentType(recording.Format))
	c.Header("Accept-Ranges", "bytes")
	http.ServeFile(c.Writer, c.Request, recording.FilePath)
}

// DeleteRecording deletes a recording
func (h *Handler) DeleteRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("recordingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recording ID"})
		return
	}

	userID := c.GetString("user_id")

	// Get recording info
	var recording SessionRecording
	err = h.DB.QueryRow(`
		SELECT id, session_id, user_id, file_path, status
		FROM session_recordings WHERE id = $1
	`, recordingID).Scan(&recording.ID, &recording.SessionID,
		&recording.UserID, &recording.FilePath, &recording.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	// Check permissions
	if !h.canDeleteRecording(userID, &recording) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	// Delete file if exists
	if _, err := os.Stat(recording.FilePath); err == nil {
		os.Remove(recording.FilePath)
	}

	// Delete database record
	_, err = h.DB.Exec("DELETE FROM session_recordings WHERE id = $1", recordingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete recording"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "recording deleted successfully"})
}

// GetRecordingStats returns recording statistics
func (h *Handler) GetRecordingStats(c *gin.Context) {
	userID := c.GetString("user_id")
	isAdmin := c.GetBool("is_admin")

	var stats RecordingStats
	var query string

	if isAdmin {
		query = `SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'recording') as active,
			COALESCE(SUM(duration), 0) as total_duration,
			COALESCE(SUM(file_size), 0) as total_size,
			COALESCE(AVG(duration), 0) as avg_duration
		FROM session_recordings`
	} else {
		query = fmt.Sprintf(`SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'recording') as active,
			COALESCE(SUM(duration), 0) as total_duration,
			COALESCE(SUM(file_size), 0) as total_size,
			COALESCE(AVG(duration), 0) as avg_duration
		FROM session_recordings WHERE user_id = '%s'`, userID)
	}

	err := h.DB.QueryRow(query).Scan(&stats.TotalRecordings, &stats.ActiveRecordings,
		&stats.TotalDuration, &stats.TotalSize, &stats.AverageDuration)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	// Get recordings by user (admin only)
	stats.RecordingsByUser = make(map[string]int64)
	if isAdmin {
		rows, _ := h.DB.Query(`
			SELECT user_id, COUNT(*)
			FROM session_recordings
			GROUP BY user_id
		`)
		defer rows.Close()

		for rows.Next() {
			var uid string
			var count int64
			rows.Scan(&uid, &count)
			stats.RecordingsByUser[uid] = count
		}
	}

	// Get recordings by month
	stats.RecordingsByMonth = make(map[string]int64)
	monthQuery := query
	if isAdmin {
		monthQuery = `
			SELECT TO_CHAR(start_time, 'YYYY-MM') as month, COUNT(*)
			FROM session_recordings
			GROUP BY month
			ORDER BY month DESC
			LIMIT 12
		`
	} else {
		monthQuery = fmt.Sprintf(`
			SELECT TO_CHAR(start_time, 'YYYY-MM') as month, COUNT(*)
			FROM session_recordings
			WHERE user_id = '%s'
			GROUP BY month
			ORDER BY month DESC
			LIMIT 12
		`, userID)
	}

	rows, _ := h.DB.Query(monthQuery)
	defer rows.Close()

	for rows.Next() {
		var month string
		var count int64
		rows.Scan(&month, &count)
		stats.RecordingsByMonth[month] = count
	}

	c.JSON(http.StatusOK, stats)
}

// Helper functions

func (h *Handler) canRecordSession(userID, sessionID string) bool {
	// Check if user owns the session or is admin
	var ownerID string
	err := h.DB.QueryRow("SELECT user_id FROM sessions WHERE id = $1", sessionID).Scan(&ownerID)
	if err != nil {
		return false
	}

	// Owner can record, or check admin status
	if ownerID == userID {
		return true
	}

	var isAdmin bool
	h.DB.QueryRow("SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
	return isAdmin
}

func (h *Handler) canManageRecording(userID string, recording *SessionRecording) bool {
	// Owner can manage, or check admin status
	if recording.UserID == userID {
		return true
	}

	var isAdmin bool
	h.DB.QueryRow("SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
	return isAdmin
}

func (h *Handler) canViewRecording(userID string, recording *SessionRecording) bool {
	return h.canManageRecording(userID, recording)
}

func (h *Handler) canDownloadRecording(userID string, recording *SessionRecording) bool {
	// Check if policy allows user download
	return h.canManageRecording(userID, recording)
}

func (h *Handler) canDeleteRecording(userID string, recording *SessionRecording) bool {
	return h.canManageRecording(userID, recording)
}

func (h *Handler) getDefaultRetentionDays(sessionID string) int {
	// Get from policy or return default
	return 30 // Default 30 days
}

func (h *Handler) calculateFileHash(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}

	return hex.EncodeToString(hash.Sum(nil))
}

func (h *Handler) getContentType(format string) string {
	switch format {
	case "webm":
		return "video/webm"
	case "mp4":
		return "video/mp4"
	case "vnc":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func (h *Handler) logRecordingAccess(recordingID int64, userID, action string) {
	h.DB.Exec(`
		INSERT INTO recording_access_log (recording_id, user_id, action, accessed_at)
		VALUES ($1, $2, $3, $4)
	`, recordingID, userID, action, time.Now())
}

// Recording Policies

// CreateRecordingPolicy creates a new recording policy
func (h *Handler) CreateRecordingPolicy(c *gin.Context) {
	var policy RecordingPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.DB.QueryRow(`
		INSERT INTO recording_policies (
			name, description, auto_record, recording_format, retention_days,
			apply_to_users, apply_to_teams, apply_to_templates,
			require_reason, allow_user_playback, allow_user_download,
			require_approval, notify_on_recording, metadata, enabled, priority
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`, policy.Name, policy.Description, policy.AutoRecord, policy.RecordingFormat,
		policy.RetentionDays, toJSONB(policy.ApplyToUsers), toJSONB(policy.ApplyToTeams),
		toJSONB(policy.ApplyToTemplates), policy.RequireReason, policy.AllowUserPlayback,
		policy.AllowUserDownload, policy.RequireApproval, policy.NotifyOnRecording,
		toJSONB(policy.Metadata), policy.Enabled, policy.Priority).Scan(&policy.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create policy"})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// ListRecordingPolicies lists all recording policies
func (h *Handler) ListRecordingPolicies(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, name, description, auto_record, recording_format, retention_days,
		       apply_to_users, apply_to_teams, apply_to_templates,
		       require_reason, allow_user_playback, allow_user_download,
		       require_approval, notify_on_recording, metadata, enabled, priority,
		       created_at, updated_at
		FROM recording_policies
		ORDER BY priority DESC, created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve policies"})
		return
	}
	defer rows.Close()

	policies := []RecordingPolicy{}
	for rows.Next() {
		var p RecordingPolicy
		var users, teams, templates, metadata sql.NullString
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.AutoRecord,
			&p.RecordingFormat, &p.RetentionDays, &users, &teams, &templates,
			&p.RequireReason, &p.AllowUserPlayback, &p.AllowUserDownload,
			&p.RequireApproval, &p.NotifyOnRecording, &metadata, &p.Enabled,
			&p.Priority, &p.CreatedAt, &p.UpdatedAt)
		if err == nil {
			if users.Valid && users.String != "" {
				json.Unmarshal([]byte(users.String), &p.ApplyToUsers)
			}
			if teams.Valid && teams.String != "" {
				json.Unmarshal([]byte(teams.String), &p.ApplyToTeams)
			}
			if templates.Valid && templates.String != "" {
				json.Unmarshal([]byte(templates.String), &p.ApplyToTemplates)
			}
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &p.Metadata)
			}
			policies = append(policies, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// UpdateRecordingPolicy updates a recording policy
func (h *Handler) UpdateRecordingPolicy(c *gin.Context) {
	policyID, err := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy ID"})
		return
	}

	var policy RecordingPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE recording_policies SET
			name = $1, description = $2, auto_record = $3, recording_format = $4,
			retention_days = $5, apply_to_users = $6, apply_to_teams = $7,
			apply_to_templates = $8, require_reason = $9, allow_user_playback = $10,
			allow_user_download = $11, require_approval = $12, notify_on_recording = $13,
			metadata = $14, enabled = $15, priority = $16, updated_at = $17
		WHERE id = $18
	`, policy.Name, policy.Description, policy.AutoRecord, policy.RecordingFormat,
		policy.RetentionDays, toJSONB(policy.ApplyToUsers), toJSONB(policy.ApplyToTeams),
		toJSONB(policy.ApplyToTemplates), policy.RequireReason, policy.AllowUserPlayback,
		policy.AllowUserDownload, policy.RequireApproval, policy.NotifyOnRecording,
		toJSONB(policy.Metadata), policy.Enabled, policy.Priority, time.Now(), policyID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update policy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy updated successfully"})
}

// DeleteRecordingPolicy deletes a recording policy
func (h *Handler) DeleteRecordingPolicy(c *gin.Context) {
	policyID, err := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy ID"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM recording_policies WHERE id = $1", policyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete policy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy deleted successfully"})
}

// CleanupExpiredRecordings removes expired recordings
func (h *Handler) CleanupExpiredRecordings(c *gin.Context) {
	// Find expired recordings
	rows, err := h.DB.Query(`
		SELECT id, file_path
		FROM session_recordings
		WHERE expires_at < $1 AND status = 'completed'
	`, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find expired recordings"})
		return
	}
	defer rows.Close()

	deleted := 0
	for rows.Next() {
		var id int64
		var filePath string
		rows.Scan(&id, &filePath)

		// Delete file
		if _, err := os.Stat(filePath); err == nil {
			os.Remove(filePath)
		}

		// Delete record
		h.DB.Exec("DELETE FROM session_recordings WHERE id = $1", id)
		deleted++
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "cleanup completed",
		"deleted": deleted,
	})
}
