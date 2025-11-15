package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DLPPolicy represents a Data Loss Prevention policy
type DLPPolicy struct {
	ID                     int64                  `json:"id"`
	Name                   string                 `json:"name"`
	Description            string                 `json:"description"`
	Enabled                bool                   `json:"enabled"`
	Priority               int                    `json:"priority"`

	// Clipboard controls
	ClipboardEnabled       bool                   `json:"clipboard_enabled"`
	ClipboardDirection     string                 `json:"clipboard_direction"` // "disabled", "to_session", "from_session", "bidirectional"
	ClipboardMaxSize       int                    `json:"clipboard_max_size"`  // in bytes
	ClipboardContentFilter []string               `json:"clipboard_content_filter"` // regex patterns to block

	// File transfer controls
	FileTransferEnabled    bool                   `json:"file_transfer_enabled"`
	FileUploadEnabled      bool                   `json:"file_upload_enabled"`
	FileDownloadEnabled    bool                   `json:"file_download_enabled"`
	FileMaxSize            int64                  `json:"file_max_size"` // in bytes
	FileTypeWhitelist      []string               `json:"file_type_whitelist"` // allowed extensions
	FileTypeBlacklist      []string               `json:"file_type_blacklist"` // blocked extensions
	ScanFilesForMalware    bool                   `json:"scan_files_for_malware"`

	// Screen capture and printing
	ScreenCaptureEnabled   bool                   `json:"screen_capture_enabled"`
	PrintingEnabled        bool                   `json:"printing_enabled"`
	WatermarkEnabled       bool                   `json:"watermark_enabled"`
	WatermarkText          string                 `json:"watermark_text"`
	WatermarkOpacity       float64                `json:"watermark_opacity"` // 0.0 to 1.0
	WatermarkPosition      string                 `json:"watermark_position"` // "top-left", "top-right", "bottom-left", "bottom-right", "center"

	// USB and peripheral devices
	USBDevicesEnabled      bool                   `json:"usb_devices_enabled"`
	AudioEnabled           bool                   `json:"audio_enabled"`
	MicrophoneEnabled      bool                   `json:"microphone_enabled"`
	WebcamEnabled          bool                   `json:"webcam_enabled"`

	// Network controls
	NetworkAccessEnabled   bool                   `json:"network_access_enabled"`
	AllowedDomains         []string               `json:"allowed_domains"`
	BlockedDomains         []string               `json:"blocked_domains"`
	AllowedIPRanges        []string               `json:"allowed_ip_ranges"`
	BlockedIPRanges        []string               `json:"blocked_ip_ranges"`

	// Session controls
	IdleTimeout            int                    `json:"idle_timeout"` // in minutes
	MaxSessionDuration     int                    `json:"max_session_duration"` // in minutes
	RequireReason          bool                   `json:"require_reason"`
	RequireApproval        bool                   `json:"require_approval"`

	// Monitoring and logging
	LogAllActivity         bool                   `json:"log_all_activity"`
	AlertOnViolation       bool                   `json:"alert_on_violation"`
	BlockOnViolation       bool                   `json:"block_on_violation"`
	NotifyUser             bool                   `json:"notify_user"`
	NotifyAdmin            bool                   `json:"notify_admin"`

	// Application scope
	ApplyToUsers           []string               `json:"apply_to_users"`
	ApplyToTeams           []string               `json:"apply_to_teams"`
	ApplyToTemplates       []string               `json:"apply_to_templates"`
	ApplyToSessions        []string               `json:"apply_to_sessions"`

	// Metadata
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy              string                 `json:"created_by"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

// DLPViolation represents a DLP policy violation event
type DLPViolation struct {
	ID            int64                  `json:"id"`
	PolicyID      int64                  `json:"policy_id"`
	PolicyName    string                 `json:"policy_name"`
	SessionID     string                 `json:"session_id"`
	UserID        string                 `json:"user_id"`
	ViolationType string                 `json:"violation_type"` // "clipboard", "file_transfer", "screen_capture", etc.
	Severity      string                 `json:"severity"`       // "low", "medium", "high", "critical"
	Description   string                 `json:"description"`
	Details       map[string]interface{} `json:"details"`
	Action        string                 `json:"action"`        // "blocked", "allowed_with_warning", "logged_only"
	Resolved      bool                   `json:"resolved"`
	ResolvedBy    string                 `json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time             `json:"resolved_at,omitempty"`
	OccurredAt    time.Time              `json:"occurred_at"`
	CreatedAt     time.Time              `json:"created_at"`
}

// DLPStats represents DLP statistics
type DLPStats struct {
	TotalPolicies      int64                   `json:"total_policies"`
	ActivePolicies     int64                   `json:"active_policies"`
	TotalViolations    int64                   `json:"total_violations"`
	ViolationsToday    int64                   `json:"violations_today"`
	ViolationsThisWeek int64                   `json:"violations_this_week"`
	ViolationsBySeverity map[string]int64      `json:"violations_by_severity"`
	ViolationsByType   map[string]int64        `json:"violations_by_type"`
	TopViolators       []map[string]interface{} `json:"top_violators"`
	RecentViolations   []DLPViolation          `json:"recent_violations"`
}

// CreateDLPPolicy creates a new DLP policy
func (h *Handler) CreateDLPPolicy(c *gin.Context) {
	var policy DLPPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	policy.CreatedBy = userID

	err := h.DB.QueryRow(`
		INSERT INTO dlp_policies (
			name, description, enabled, priority,
			clipboard_enabled, clipboard_direction, clipboard_max_size, clipboard_content_filter,
			file_transfer_enabled, file_upload_enabled, file_download_enabled, file_max_size,
			file_type_whitelist, file_type_blacklist, scan_files_for_malware,
			screen_capture_enabled, printing_enabled, watermark_enabled, watermark_text,
			watermark_opacity, watermark_position,
			usb_devices_enabled, audio_enabled, microphone_enabled, webcam_enabled,
			network_access_enabled, allowed_domains, blocked_domains, allowed_ip_ranges, blocked_ip_ranges,
			idle_timeout, max_session_duration, require_reason, require_approval,
			log_all_activity, alert_on_violation, block_on_violation, notify_user, notify_admin,
			apply_to_users, apply_to_teams, apply_to_templates, apply_to_sessions,
			metadata, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45
		) RETURNING id
	`, policy.Name, policy.Description, policy.Enabled, policy.Priority,
		policy.ClipboardEnabled, policy.ClipboardDirection, policy.ClipboardMaxSize, toJSONB(policy.ClipboardContentFilter),
		policy.FileTransferEnabled, policy.FileUploadEnabled, policy.FileDownloadEnabled, policy.FileMaxSize,
		toJSONB(policy.FileTypeWhitelist), toJSONB(policy.FileTypeBlacklist), policy.ScanFilesForMalware,
		policy.ScreenCaptureEnabled, policy.PrintingEnabled, policy.WatermarkEnabled, policy.WatermarkText,
		policy.WatermarkOpacity, policy.WatermarkPosition,
		policy.USBDevicesEnabled, policy.AudioEnabled, policy.MicrophoneEnabled, policy.WebcamEnabled,
		policy.NetworkAccessEnabled, toJSONB(policy.AllowedDomains), toJSONB(policy.BlockedDomains),
		toJSONB(policy.AllowedIPRanges), toJSONB(policy.BlockedIPRanges),
		policy.IdleTimeout, policy.MaxSessionDuration, policy.RequireReason, policy.RequireApproval,
		policy.LogAllActivity, policy.AlertOnViolation, policy.BlockOnViolation, policy.NotifyUser, policy.NotifyAdmin,
		toJSONB(policy.ApplyToUsers), toJSONB(policy.ApplyToTeams), toJSONB(policy.ApplyToTemplates), toJSONB(policy.ApplyToSessions),
		toJSONB(policy.Metadata), userID).Scan(&policy.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create DLP policy"})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// ListDLPPolicies lists all DLP policies
func (h *Handler) ListDLPPolicies(c *gin.Context) {
	enabled := c.Query("enabled")
	sessionID := c.Query("session_id")

	query := `
		SELECT id, name, description, enabled, priority,
		       clipboard_enabled, clipboard_direction, clipboard_max_size, clipboard_content_filter,
		       file_transfer_enabled, file_upload_enabled, file_download_enabled, file_max_size,
		       file_type_whitelist, file_type_blacklist, scan_files_for_malware,
		       screen_capture_enabled, printing_enabled, watermark_enabled, watermark_text,
		       watermark_opacity, watermark_position,
		       usb_devices_enabled, audio_enabled, microphone_enabled, webcam_enabled,
		       network_access_enabled, allowed_domains, blocked_domains, allowed_ip_ranges, blocked_ip_ranges,
		       idle_timeout, max_session_duration, require_reason, require_approval,
		       log_all_activity, alert_on_violation, block_on_violation, notify_user, notify_admin,
		       apply_to_users, apply_to_teams, apply_to_templates, apply_to_sessions,
		       metadata, created_by, created_at, updated_at
		FROM dlp_policies WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if enabled != "" {
		query += fmt.Sprintf(" AND enabled = $%d", argCount)
		args = append(args, enabled == "true")
		argCount++
	}

	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve policies"})
		return
	}
	defer rows.Close()

	policies := []DLPPolicy{}
	for rows.Next() {
		var p DLPPolicy
		var clipboardFilter, fileWhitelist, fileBlacklist, allowedDomains, blockedDomains,
			allowedIPs, blockedIPs, applyUsers, applyTeams, applyTemplates, applySessions, metadata sql.NullString

		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &p.Priority,
			&p.ClipboardEnabled, &p.ClipboardDirection, &p.ClipboardMaxSize, &clipboardFilter,
			&p.FileTransferEnabled, &p.FileUploadEnabled, &p.FileDownloadEnabled, &p.FileMaxSize,
			&fileWhitelist, &fileBlacklist, &p.ScanFilesForMalware,
			&p.ScreenCaptureEnabled, &p.PrintingEnabled, &p.WatermarkEnabled, &p.WatermarkText,
			&p.WatermarkOpacity, &p.WatermarkPosition,
			&p.USBDevicesEnabled, &p.AudioEnabled, &p.MicrophoneEnabled, &p.WebcamEnabled,
			&p.NetworkAccessEnabled, &allowedDomains, &blockedDomains, &allowedIPs, &blockedIPs,
			&p.IdleTimeout, &p.MaxSessionDuration, &p.RequireReason, &p.RequireApproval,
			&p.LogAllActivity, &p.AlertOnViolation, &p.BlockOnViolation, &p.NotifyUser, &p.NotifyAdmin,
			&applyUsers, &applyTeams, &applyTemplates, &applySessions,
			&metadata, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)

		if err == nil {
			// Parse JSONB arrays
			if clipboardFilter.Valid && clipboardFilter.String != "" {
				json.Unmarshal([]byte(clipboardFilter.String), &p.ClipboardContentFilter)
			}
			if fileWhitelist.Valid && fileWhitelist.String != "" {
				json.Unmarshal([]byte(fileWhitelist.String), &p.FileTypeWhitelist)
			}
			if fileBlacklist.Valid && fileBlacklist.String != "" {
				json.Unmarshal([]byte(fileBlacklist.String), &p.FileTypeBlacklist)
			}
			if allowedDomains.Valid && allowedDomains.String != "" {
				json.Unmarshal([]byte(allowedDomains.String), &p.AllowedDomains)
			}
			if blockedDomains.Valid && blockedDomains.String != "" {
				json.Unmarshal([]byte(blockedDomains.String), &p.BlockedDomains)
			}
			if allowedIPs.Valid && allowedIPs.String != "" {
				json.Unmarshal([]byte(allowedIPs.String), &p.AllowedIPRanges)
			}
			if blockedIPs.Valid && blockedIPs.String != "" {
				json.Unmarshal([]byte(blockedIPs.String), &p.BlockedIPRanges)
			}
			if applyUsers.Valid && applyUsers.String != "" {
				json.Unmarshal([]byte(applyUsers.String), &p.ApplyToUsers)
			}
			if applyTeams.Valid && applyTeams.String != "" {
				json.Unmarshal([]byte(applyTeams.String), &p.ApplyToTeams)
			}
			if applyTemplates.Valid && applyTemplates.String != "" {
				json.Unmarshal([]byte(applyTemplates.String), &p.ApplyToTemplates)
			}
			if applySessions.Valid && applySessions.String != "" {
				json.Unmarshal([]byte(applySessions.String), &p.ApplyToSessions)
			}
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &p.Metadata)
			}

			// Filter by session if requested
			if sessionID != "" {
				if h.policyAppliesToSession(p, sessionID) {
					policies = append(policies, p)
				}
			} else {
				policies = append(policies, p)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// GetDLPPolicy retrieves a specific DLP policy
func (h *Handler) GetDLPPolicy(c *gin.Context) {
	policyID, err := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy ID"})
		return
	}

	var p DLPPolicy
	var clipboardFilter, fileWhitelist, fileBlacklist, allowedDomains, blockedDomains,
		allowedIPs, blockedIPs, applyUsers, applyTeams, applyTemplates, applySessions, metadata sql.NullString

	err = h.DB.QueryRow(`
		SELECT id, name, description, enabled, priority,
		       clipboard_enabled, clipboard_direction, clipboard_max_size, clipboard_content_filter,
		       file_transfer_enabled, file_upload_enabled, file_download_enabled, file_max_size,
		       file_type_whitelist, file_type_blacklist, scan_files_for_malware,
		       screen_capture_enabled, printing_enabled, watermark_enabled, watermark_text,
		       watermark_opacity, watermark_position,
		       usb_devices_enabled, audio_enabled, microphone_enabled, webcam_enabled,
		       network_access_enabled, allowed_domains, blocked_domains, allowed_ip_ranges, blocked_ip_ranges,
		       idle_timeout, max_session_duration, require_reason, require_approval,
		       log_all_activity, alert_on_violation, block_on_violation, notify_user, notify_admin,
		       apply_to_users, apply_to_teams, apply_to_templates, apply_to_sessions,
		       metadata, created_by, created_at, updated_at
		FROM dlp_policies WHERE id = $1
	`, policyID).Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &p.Priority,
		&p.ClipboardEnabled, &p.ClipboardDirection, &p.ClipboardMaxSize, &clipboardFilter,
		&p.FileTransferEnabled, &p.FileUploadEnabled, &p.FileDownloadEnabled, &p.FileMaxSize,
		&fileWhitelist, &fileBlacklist, &p.ScanFilesForMalware,
		&p.ScreenCaptureEnabled, &p.PrintingEnabled, &p.WatermarkEnabled, &p.WatermarkText,
		&p.WatermarkOpacity, &p.WatermarkPosition,
		&p.USBDevicesEnabled, &p.AudioEnabled, &p.MicrophoneEnabled, &p.WebcamEnabled,
		&p.NetworkAccessEnabled, &allowedDomains, &blockedDomains, &allowedIPs, &blockedIPs,
		&p.IdleTimeout, &p.MaxSessionDuration, &p.RequireReason, &p.RequireApproval,
		&p.LogAllActivity, &p.AlertOnViolation, &p.BlockOnViolation, &p.NotifyUser, &p.NotifyAdmin,
		&applyUsers, &applyTeams, &applyTemplates, &applySessions,
		&metadata, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve policy"})
		return
	}

	// Parse JSONB fields (same as ListDLPPolicies)
	if clipboardFilter.Valid && clipboardFilter.String != "" {
		json.Unmarshal([]byte(clipboardFilter.String), &p.ClipboardContentFilter)
	}
	// ... parse other JSONB fields ...

	c.JSON(http.StatusOK, p)
}

// UpdateDLPPolicy updates an existing DLP policy
func (h *Handler) UpdateDLPPolicy(c *gin.Context) {
	policyID, err := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy ID"})
		return
	}

	var policy DLPPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE dlp_policies SET
			name = $1, description = $2, enabled = $3, priority = $4,
			clipboard_enabled = $5, clipboard_direction = $6, clipboard_max_size = $7, clipboard_content_filter = $8,
			file_transfer_enabled = $9, file_upload_enabled = $10, file_download_enabled = $11, file_max_size = $12,
			file_type_whitelist = $13, file_type_blacklist = $14, scan_files_for_malware = $15,
			screen_capture_enabled = $16, printing_enabled = $17, watermark_enabled = $18, watermark_text = $19,
			watermark_opacity = $20, watermark_position = $21,
			usb_devices_enabled = $22, audio_enabled = $23, microphone_enabled = $24, webcam_enabled = $25,
			network_access_enabled = $26, allowed_domains = $27, blocked_domains = $28, allowed_ip_ranges = $29, blocked_ip_ranges = $30,
			idle_timeout = $31, max_session_duration = $32, require_reason = $33, require_approval = $34,
			log_all_activity = $35, alert_on_violation = $36, block_on_violation = $37, notify_user = $38, notify_admin = $39,
			apply_to_users = $40, apply_to_teams = $41, apply_to_templates = $42, apply_to_sessions = $43,
			metadata = $44, updated_at = $45
		WHERE id = $46
	`, policy.Name, policy.Description, policy.Enabled, policy.Priority,
		policy.ClipboardEnabled, policy.ClipboardDirection, policy.ClipboardMaxSize, toJSONB(policy.ClipboardContentFilter),
		policy.FileTransferEnabled, policy.FileUploadEnabled, policy.FileDownloadEnabled, policy.FileMaxSize,
		toJSONB(policy.FileTypeWhitelist), toJSONB(policy.FileTypeBlacklist), policy.ScanFilesForMalware,
		policy.ScreenCaptureEnabled, policy.PrintingEnabled, policy.WatermarkEnabled, policy.WatermarkText,
		policy.WatermarkOpacity, policy.WatermarkPosition,
		policy.USBDevicesEnabled, policy.AudioEnabled, policy.MicrophoneEnabled, policy.WebcamEnabled,
		policy.NetworkAccessEnabled, toJSONB(policy.AllowedDomains), toJSONB(policy.BlockedDomains),
		toJSONB(policy.AllowedIPRanges), toJSONB(policy.BlockedIPRanges),
		policy.IdleTimeout, policy.MaxSessionDuration, policy.RequireReason, policy.RequireApproval,
		policy.LogAllActivity, policy.AlertOnViolation, policy.BlockOnViolation, policy.NotifyUser, policy.NotifyAdmin,
		toJSONB(policy.ApplyToUsers), toJSONB(policy.ApplyToTeams), toJSONB(policy.ApplyToTemplates), toJSONB(policy.ApplyToSessions),
		toJSONB(policy.Metadata), time.Now(), policyID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update policy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy updated successfully"})
}

// DeleteDLPPolicy deletes a DLP policy
func (h *Handler) DeleteDLPPolicy(c *gin.Context) {
	policyID, err := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy ID"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM dlp_policies WHERE id = $1", policyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete policy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy deleted successfully"})
}

// LogDLPViolation logs a DLP policy violation
func (h *Handler) LogDLPViolation(c *gin.Context) {
	var violation DLPViolation
	if err := c.ShouldBindJSON(&violation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.DB.QueryRow(`
		INSERT INTO dlp_violations (
			policy_id, policy_name, session_id, user_id, violation_type,
			severity, description, details, action
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, violation.PolicyID, violation.PolicyName, violation.SessionID, violation.UserID,
		violation.ViolationType, violation.Severity, violation.Description,
		toJSONB(violation.Details), violation.Action).Scan(&violation.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log violation"})
		return
	}

	// Send notifications if configured
	var notifyUser, notifyAdmin bool
	h.DB.QueryRow(`SELECT notify_user, notify_admin FROM dlp_policies WHERE id = $1`,
		violation.PolicyID).Scan(&notifyUser, &notifyAdmin)

	if notifyUser || notifyAdmin {
		h.sendDLPViolationNotifications(violation, notifyUser, notifyAdmin)
	}

	c.JSON(http.StatusCreated, violation)
}

// sendDLPViolationNotifications sends notifications for DLP violations
func (h *Handler) sendDLPViolationNotifications(violation DLPViolation, notifyUser bool, notifyAdmin bool) {
	// Notify user if configured
	if notifyUser {
		notification := Notification{
			UserID:   violation.UserID,
			Type:     "dlp_violation",
			Title:    "Data Loss Prevention Alert",
			Message:  fmt.Sprintf("DLP policy '%s' violation: %s", violation.PolicyName, violation.Description),
			Severity: violation.Severity,
			Data: map[string]interface{}{
				"policy_name":    violation.PolicyName,
				"violation_type": violation.ViolationType,
				"session_id":     violation.SessionID,
				"action":         violation.Action,
			},
		}

		if err := h.SendNotification(&notification); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to send DLP notification to user %s: %v\n", violation.UserID, err)
		}
	}

	// Notify admins if configured
	if notifyAdmin {
		// Query all admin users
		rows, err := h.DB.Query(`SELECT user_id FROM users WHERE role = 'admin'`)
		if err != nil {
			fmt.Printf("Failed to query admin users for DLP notification: %v\n", err)
			return
		}
		defer rows.Close()

		var adminIDs []string
		for rows.Next() {
			var adminID string
			if err := rows.Scan(&adminID); err == nil {
				adminIDs = append(adminIDs, adminID)
			}
		}

		// Send notification to each admin
		for _, adminID := range adminIDs {
			notification := Notification{
				UserID:   adminID,
				Type:     "admin_dlp_alert",
				Title:    "DLP Violation Alert",
				Message:  fmt.Sprintf("User %s violated DLP policy '%s': %s (Action: %s)", violation.UserID, violation.PolicyName, violation.Description, violation.Action),
				Severity: violation.Severity,
				Data: map[string]interface{}{
					"user_id":        violation.UserID,
					"policy_name":    violation.PolicyName,
					"violation_type": violation.ViolationType,
					"session_id":     violation.SessionID,
					"action":         violation.Action,
					"severity":       violation.Severity,
				},
			}

			if err := h.SendNotification(&notification); err != nil {
				fmt.Printf("Failed to send DLP notification to admin %s: %v\n", adminID, err)
			}
		}
	}
}

// ListDLPViolations lists DLP violations with filtering
func (h *Handler) ListDLPViolations(c *gin.Context) {
	userID := c.GetString("user_id")
	isAdmin := c.GetBool("is_admin")

	sessionID := c.Query("session_id")
	violationType := c.Query("type")
	severity := c.Query("severity")
	resolved := c.Query("resolved")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	query := `
		SELECT id, policy_id, policy_name, session_id, user_id, violation_type,
		       severity, description, details, action, resolved, resolved_by,
		       resolved_at, occurred_at, created_at
		FROM dlp_violations WHERE 1=1
	`
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

	if violationType != "" {
		query += fmt.Sprintf(" AND violation_type = $%d", argCount)
		args = append(args, violationType)
		argCount++
	}

	if severity != "" {
		query += fmt.Sprintf(" AND severity = $%d", argCount)
		args = append(args, severity)
		argCount++
	}

	if resolved != "" {
		query += fmt.Sprintf(" AND resolved = $%d", argCount)
		args = append(args, resolved == "true")
		argCount++
	}

	// Count total
	countQuery := strings.Replace(query, "SELECT id, policy_id, policy_name, session_id, user_id, violation_type, severity, description, details, action, resolved, resolved_by, resolved_at, occurred_at, created_at", "SELECT COUNT(*)", 1)
	var total int
	h.DB.QueryRow(countQuery, args...).Scan(&total)

	// Add pagination
	query += fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve violations"})
		return
	}
	defer rows.Close()

	violations := []DLPViolation{}
	for rows.Next() {
		var v DLPViolation
		var details sql.NullString
		err := rows.Scan(&v.ID, &v.PolicyID, &v.PolicyName, &v.SessionID, &v.UserID,
			&v.ViolationType, &v.Severity, &v.Description, &details, &v.Action,
			&v.Resolved, &v.ResolvedBy, &v.ResolvedAt, &v.OccurredAt, &v.CreatedAt)
		if err == nil {
			if details.Valid && details.String != "" {
				json.Unmarshal([]byte(details.String), &v.Details)
			}
			violations = append(violations, v)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"violations":  violations,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// ResolveDLPViolation marks a violation as resolved
func (h *Handler) ResolveDLPViolation(c *gin.Context) {
	violationID, err := strconv.ParseInt(c.Param("violationId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid violation ID"})
		return
	}

	userID := c.GetString("user_id")

	_, err = h.DB.Exec(`
		UPDATE dlp_violations
		SET resolved = true, resolved_by = $1, resolved_at = $2
		WHERE id = $3
	`, userID, time.Now(), violationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve violation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "violation resolved successfully"})
}

// GetDLPStats returns DLP statistics
func (h *Handler) GetDLPStats(c *gin.Context) {
	userID := c.GetString("user_id")
	isAdmin := c.GetBool("is_admin")

	var stats DLPStats

	// Total and active policies
	h.DB.QueryRow("SELECT COUNT(*) FROM dlp_policies").Scan(&stats.TotalPolicies)
	h.DB.QueryRow("SELECT COUNT(*) FROM dlp_policies WHERE enabled = true").Scan(&stats.ActivePolicies)

	// Violation counts
	var violationQuery string
	if isAdmin {
		violationQuery = `
			SELECT COUNT(*) as total,
			       COUNT(*) FILTER (WHERE occurred_at > CURRENT_DATE) as today,
			       COUNT(*) FILTER (WHERE occurred_at > CURRENT_DATE - INTERVAL '7 days') as week
			FROM dlp_violations
		`
	} else {
		violationQuery = fmt.Sprintf(`
			SELECT COUNT(*) as total,
			       COUNT(*) FILTER (WHERE occurred_at > CURRENT_DATE) as today,
			       COUNT(*) FILTER (WHERE occurred_at > CURRENT_DATE - INTERVAL '7 days') as week
			FROM dlp_violations WHERE user_id = '%s'
		`, userID)
	}

	h.DB.QueryRow(violationQuery).Scan(&stats.TotalViolations, &stats.ViolationsToday, &stats.ViolationsThisWeek)

	// Violations by severity
	stats.ViolationsBySeverity = make(map[string]int64)
	severityQuery := violationQuery
	if isAdmin {
		severityQuery = "SELECT severity, COUNT(*) FROM dlp_violations GROUP BY severity"
	} else {
		severityQuery = fmt.Sprintf("SELECT severity, COUNT(*) FROM dlp_violations WHERE user_id = '%s' GROUP BY severity", userID)
	}
	rows, _ := h.DB.Query(severityQuery)
	defer rows.Close()
	for rows.Next() {
		var severity string
		var count int64
		rows.Scan(&severity, &count)
		stats.ViolationsBySeverity[severity] = count
	}

	// Violations by type
	stats.ViolationsByType = make(map[string]int64)
	typeQuery := violationQuery
	if isAdmin {
		typeQuery = "SELECT violation_type, COUNT(*) FROM dlp_violations GROUP BY violation_type"
	} else {
		typeQuery = fmt.Sprintf("SELECT violation_type, COUNT(*) FROM dlp_violations WHERE user_id = '%s' GROUP BY violation_type", userID)
	}
	rows2, _ := h.DB.Query(typeQuery)
	defer rows2.Close()
	for rows2.Next() {
		var vtype string
		var count int64
		rows2.Scan(&vtype, &count)
		stats.ViolationsByType[vtype] = count
	}

	// Top violators (admin only)
	if isAdmin {
		rows3, _ := h.DB.Query(`
			SELECT user_id, COUNT(*) as count
			FROM dlp_violations
			GROUP BY user_id
			ORDER BY count DESC
			LIMIT 10
		`)
		defer rows3.Close()
		for rows3.Next() {
			var uid string
			var count int64
			rows3.Scan(&uid, &count)
			stats.TopViolators = append(stats.TopViolators, map[string]interface{}{
				"user_id": uid,
				"count":   count,
			})
		}
	}

	c.JSON(http.StatusOK, stats)
}

// Helper functions

func (h *Handler) policyAppliesToSession(policy DLPPolicy, sessionID string) bool {
	// Check if policy applies to this session
	if len(policy.ApplyToSessions) == 0 {
		return true // Apply to all if no specific sessions
	}

	for _, sid := range policy.ApplyToSessions {
		if sid == sessionID {
			return true
		}
	}

	// Check if session's user or team is in policy scope
	var userID, teamID string
	h.DB.QueryRow("SELECT user_id, team_id FROM sessions WHERE id = $1", sessionID).Scan(&userID, &teamID)

	for _, uid := range policy.ApplyToUsers {
		if uid == userID {
			return true
		}
	}

	for _, tid := range policy.ApplyToTeams {
		if tid == teamID {
			return true
		}
	}

	return false
}
