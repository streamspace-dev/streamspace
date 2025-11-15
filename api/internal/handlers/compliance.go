package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// COMPLIANCE FRAMEWORKS
// ============================================================================

// ComplianceFramework represents a regulatory compliance framework
type ComplianceFramework struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"` // "GDPR", "HIPAA", "SOC2", "ISO27001", "Custom"
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Controls    []ComplianceControl    `json:"controls"`
	Requirements map[string]interface{} `json:"requirements,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// ComplianceControl represents a specific control within a framework
type ComplianceControl struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Category       string                 `json:"category"` // "access_control", "data_protection", "audit", etc.
	Automated      bool                   `json:"automated"` // Can be checked automatically
	CheckInterval  int                    `json:"check_interval_hours,omitempty"`
	Status         string                 `json:"status,omitempty"` // "compliant", "non_compliant", "unknown"
	LastChecked    time.Time              `json:"last_checked,omitempty"`
	Evidence       []string               `json:"evidence,omitempty"`
	Configuration  map[string]interface{} `json:"configuration,omitempty"`
}

// CompliancePolicy represents an organizational compliance policy
type CompliancePolicy struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	FrameworkID       int64                  `json:"framework_id"`
	FrameworkName     string                 `json:"framework_name,omitempty"`
	AppliesTo         PolicyScope            `json:"applies_to"`
	Enabled           bool                   `json:"enabled"`
	EnforcementLevel  string                 `json:"enforcement_level"` // "advisory", "warning", "blocking"
	DataRetention     DataRetentionConfig    `json:"data_retention"`
	DataClassification DataClassificationConfig `json:"data_classification"`
	AccessControls    AccessControlConfig    `json:"access_controls"`
	AuditRequirements AuditRequirementsConfig `json:"audit_requirements"`
	ViolationActions  ViolationActionConfig  `json:"violation_actions"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy         string                 `json:"created_by"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PolicyScope defines who a policy applies to
type PolicyScope struct {
	UserIDs      []string `json:"user_ids,omitempty"`
	TeamIDs      []string `json:"team_ids,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	AllUsers     bool     `json:"all_users"`
}

// DataRetentionConfig defines data retention rules
type DataRetentionConfig struct {
	Enabled           bool   `json:"enabled"`
	SessionDataDays   int    `json:"session_data_days"`    // Retain session metadata
	RecordingDays     int    `json:"recording_days"`       // Retain session recordings
	AuditLogDays      int    `json:"audit_log_days"`       // Retain audit logs
	BackupDays        int    `json:"backup_days"`          // Retain backups
	AutoPurge         bool   `json:"auto_purge"`           // Automatically delete after retention
	PurgeSchedule     string `json:"purge_schedule,omitempty"` // Cron expression for purge job
}

// DataClassificationConfig defines data classification levels
type DataClassificationConfig struct {
	Enabled            bool     `json:"enabled"`
	Levels             []string `json:"levels"` // ["public", "internal", "confidential", "restricted"]
	DefaultLevel       string   `json:"default_level"`
	RequireLabeling    bool     `json:"require_labeling"`
	RestrictedPatterns []string `json:"restricted_patterns,omitempty"` // Regex patterns for sensitive data
}

// AccessControlConfig defines access control requirements
type AccessControlConfig struct {
	RequireMFA          bool     `json:"require_mfa"`
	AllowedIPRanges     []string `json:"allowed_ip_ranges,omitempty"`
	RequireApproval     bool     `json:"require_approval"`
	SessionTimeout      int      `json:"session_timeout_minutes"`
	MaxConcurrentSessions int    `json:"max_concurrent_sessions"`
}

// AuditRequirementsConfig defines audit logging requirements
type AuditRequirementsConfig struct {
	LogAllAccess       bool     `json:"log_all_access"`
	LogDataExport      bool     `json:"log_data_export"`
	LogPolicyChanges   bool     `json:"log_policy_changes"`
	LogAuthEvents      bool     `json:"log_auth_events"`
	AlertOnSuspicious  bool     `json:"alert_on_suspicious"`
	RequireJustification bool   `json:"require_justification"` // Require reason for access
}

// ViolationActionConfig defines actions on policy violations
type ViolationActionConfig struct {
	NotifyUser      bool     `json:"notify_user"`
	NotifyAdmin     bool     `json:"notify_admin"`
	CreateTicket    bool     `json:"create_ticket"`
	BlockAction     bool     `json:"block_action"`
	SuspendUser     bool     `json:"suspend_user"`
	EscalationEmails []string `json:"escalation_emails,omitempty"`
}

// ComplianceViolation represents a policy violation
type ComplianceViolation struct {
	ID           int64                  `json:"id"`
	PolicyID     int64                  `json:"policy_id"`
	PolicyName   string                 `json:"policy_name,omitempty"`
	UserID       string                 `json:"user_id"`
	ViolationType string                `json:"violation_type"`
	Severity     string                 `json:"severity"` // "low", "medium", "high", "critical"
	Description  string                 `json:"description"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Status       string                 `json:"status"` // "open", "acknowledged", "remediated", "closed"
	Resolution   string                 `json:"resolution,omitempty"`
	ResolvedBy   string                 `json:"resolved_by,omitempty"`
	ResolvedAt   time.Time              `json:"resolved_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// ComplianceReport represents a compliance status report
type ComplianceReport struct {
	ID              int64                  `json:"id"`
	FrameworkID     int64                  `json:"framework_id,omitempty"`
	FrameworkName   string                 `json:"framework_name,omitempty"`
	ReportType      string                 `json:"report_type"` // "summary", "detailed", "attestation"
	ReportPeriod    ReportPeriod           `json:"report_period"`
	OverallStatus   string                 `json:"overall_status"` // "compliant", "non_compliant", "partial"
	ControlsSummary ControlsSummary        `json:"controls_summary"`
	Violations      []ComplianceViolation  `json:"violations,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	GeneratedBy     string                 `json:"generated_by"`
	GeneratedAt     time.Time              `json:"generated_at"`
}

// ReportPeriod defines the time period for a report
type ReportPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// ControlsSummary summarizes compliance control status
type ControlsSummary struct {
	Total         int `json:"total"`
	Compliant     int `json:"compliant"`
	NonCompliant  int `json:"non_compliant"`
	Unknown       int `json:"unknown"`
	ComplianceRate float64 `json:"compliance_rate"` // Percentage
}

// CreateComplianceFramework creates a new compliance framework
func (h *Handler) CreateComplianceFramework(c *gin.Context) {
	createdBy := c.GetString("user_id")
	role := c.GetString("role")

	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can create compliance frameworks"})
		return
	}

	var req ComplianceFramework
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.CreatedBy = createdBy
	req.Enabled = true

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO compliance_frameworks
		(name, display_name, description, version, enabled, controls, requirements, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, req.Name, req.DisplayName, req.Description, req.Version, req.Enabled,
		req.Controls, req.Requirements, req.Metadata, createdBy).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create framework"})
		return
	}

	req.ID = id

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Compliance framework created",
		"framework": req,
	})
}

// ListComplianceFrameworks lists all compliance frameworks
func (h *Handler) ListComplianceFrameworks(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, name, display_name, description, version, enabled, controls,
		       requirements, metadata, created_by, created_at, updated_at
		FROM compliance_frameworks
		ORDER BY name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	frameworks := []ComplianceFramework{}
	for rows.Next() {
		var f ComplianceFramework
		err := rows.Scan(&f.ID, &f.Name, &f.DisplayName, &f.Description, &f.Version,
			&f.Enabled, &f.Controls, &f.Requirements, &f.Metadata, &f.CreatedBy,
			&f.CreatedAt, &f.UpdatedAt)
		if err != nil {
			continue
		}
		frameworks = append(frameworks, f)
	}

	c.JSON(http.StatusOK, gin.H{"frameworks": frameworks})
}

// CreateCompliancePolicy creates a new compliance policy
func (h *Handler) CreateCompliancePolicy(c *gin.Context) {
	createdBy := c.GetString("user_id")
	role := c.GetString("role")

	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can create compliance policies"})
		return
	}

	var req CompliancePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.CreatedBy = createdBy
	req.Enabled = true

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO compliance_policies
		(name, framework_id, applies_to, enabled, enforcement_level, data_retention,
		 data_classification, access_controls, audit_requirements, violation_actions,
		 metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`, req.Name, req.FrameworkID, req.AppliesTo, req.Enabled, req.EnforcementLevel,
		req.DataRetention, req.DataClassification, req.AccessControls, req.AuditRequirements,
		req.ViolationActions, req.Metadata, createdBy).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create policy"})
		return
	}

	req.ID = id

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Compliance policy created",
		"policy":  req,
	})
}

// ListCompliancePolicies lists all compliance policies
func (h *Handler) ListCompliancePolicies(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT p.id, p.name, p.framework_id, f.display_name, p.applies_to, p.enabled,
		       p.enforcement_level, p.data_retention, p.data_classification,
		       p.access_controls, p.audit_requirements, p.violation_actions,
		       p.metadata, p.created_by, p.created_at, p.updated_at
		FROM compliance_policies p
		LEFT JOIN compliance_frameworks f ON p.framework_id = f.id
		ORDER BY p.created_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	policies := []CompliancePolicy{}
	for rows.Next() {
		var p CompliancePolicy
		var frameworkName sql.NullString
		err := rows.Scan(&p.ID, &p.Name, &p.FrameworkID, &frameworkName, &p.AppliesTo,
			&p.Enabled, &p.EnforcementLevel, &p.DataRetention, &p.DataClassification,
			&p.AccessControls, &p.AuditRequirements, &p.ViolationActions, &p.Metadata,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		if frameworkName.Valid {
			p.FrameworkName = frameworkName.String
		}
		policies = append(policies, p)
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// RecordViolation records a compliance policy violation
func (h *Handler) RecordViolation(c *gin.Context) {
	var req ComplianceViolation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Status = "open"

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO compliance_violations
		(policy_id, user_id, violation_type, severity, description, details, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, req.PolicyID, req.UserID, req.ViolationType, req.Severity, req.Description,
		req.Details, req.Status).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record violation"})
		return
	}

	// Get policy name and take violation actions
	var policyName string
	var actions ViolationActionConfig
	h.DB.QueryRow(`SELECT name, violation_actions FROM compliance_policies WHERE id = $1`,
		req.PolicyID).Scan(&policyName, &actions)

	// Execute violation actions
	h.executeViolationActions(id, req, policyName, actions)

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Compliance violation recorded",
		"violation": req,
	})
}

// executeViolationActions executes all configured actions for a compliance violation
func (h *Handler) executeViolationActions(violationID int64, violation ComplianceViolation, policyName string, actions ViolationActionConfig) {
	// Notify user if configured
	if actions.NotifyUser {
		h.notifyViolationToUser(violation, policyName)
	}

	// Notify admins if configured
	if actions.NotifyAdmin {
		h.notifyViolationToAdmins(violation, policyName)
	}

	// Create support ticket if configured
	if actions.CreateTicket {
		h.createViolationTicket(violationID, violation, policyName)
	}

	// Send escalation emails if configured
	if len(actions.EscalationEmails) > 0 {
		h.sendViolationEscalationEmails(violation, policyName, actions.EscalationEmails)
	}

	// Block action if configured (update user session or permissions)
	if actions.BlockAction {
		h.blockUserAction(violation)
	}

	// Suspend user if configured
	if actions.SuspendUser {
		h.suspendUserForViolation(violation, policyName)
	}
}

// notifyViolationToUser sends a notification to the user about their violation
func (h *Handler) notifyViolationToUser(violation ComplianceViolation, policyName string) {
	notification := Notification{
		UserID:   violation.UserID,
		Type:     "compliance_violation",
		Title:    "Compliance Policy Violation",
		Message:  fmt.Sprintf("You have violated the policy '%s': %s", policyName, violation.Description),
		Severity: violation.Severity,
		Data: map[string]interface{}{
			"policy_name":    policyName,
			"violation_type": violation.ViolationType,
			"severity":       violation.Severity,
		},
	}

	// Send notification (errors logged but not propagated)
	if err := h.SendNotification(&notification); err != nil {
		log.Printf("Failed to send violation notification to user %s: %v", violation.UserID, err)
	}
}

// notifyViolationToAdmins sends a notification to all admins about the violation
func (h *Handler) notifyViolationToAdmins(violation ComplianceViolation, policyName string) {
	// Query all admin users
	rows, err := h.DB.Query(`SELECT user_id FROM users WHERE role = 'admin'`)
	if err != nil {
		log.Printf("Failed to query admin users: %v", err)
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
			Type:     "admin_compliance_alert",
			Title:    "Compliance Violation Alert",
			Message:  fmt.Sprintf("User %s violated policy '%s': %s", violation.UserID, policyName, violation.Description),
			Severity: violation.Severity,
			Data: map[string]interface{}{
				"user_id":        violation.UserID,
				"policy_name":    policyName,
				"violation_type": violation.ViolationType,
				"severity":       violation.Severity,
			},
		}

		if err := h.SendNotification(&notification); err != nil {
			log.Printf("Failed to send violation notification to admin %s: %v", adminID, err)
		}
	}
}

// createViolationTicket creates a support ticket for the violation
func (h *Handler) createViolationTicket(violationID int64, violation ComplianceViolation, policyName string) {
	title := fmt.Sprintf("Compliance Violation: %s", policyName)
	description := fmt.Sprintf(
		"User: %s\nPolicy: %s\nViolation Type: %s\nSeverity: %s\nDescription: %s",
		violation.UserID, policyName, violation.ViolationType, violation.Severity, violation.Description,
	)

	_, err := h.DB.Exec(`
		INSERT INTO support_tickets (user_id, title, description, priority, status, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'open', $5, NOW())
	`, violation.UserID, title, description, violation.Severity, map[string]interface{}{
		"violation_id":   violationID,
		"policy_name":    policyName,
		"violation_type": violation.ViolationType,
		"auto_created":   true,
	})

	if err != nil {
		log.Printf("Failed to create support ticket for violation %d: %v", violationID, err)
	}
}

// sendViolationEscalationEmails sends escalation emails to specified addresses
func (h *Handler) sendViolationEscalationEmails(violation ComplianceViolation, policyName string, emails []string) {
	subject := fmt.Sprintf("ESCALATION: Compliance Violation - %s", policyName)
	body := fmt.Sprintf(`
A compliance policy violation requires escalation:

User: %s
Policy: %s
Violation Type: %s
Severity: %s
Description: %s

Time: %s

Please review and take appropriate action.
`,
		violation.UserID, policyName, violation.ViolationType,
		violation.Severity, violation.Description, time.Now().Format(time.RFC3339),
	)

	for _, email := range emails {
		// Log the escalation email (actual email sending would require SMTP configuration)
		log.Printf("Escalation email for violation would be sent to %s: %s", email, subject)

		// Store escalation record
		_, err := h.DB.Exec(`
			INSERT INTO compliance_escalations (violation_id, escalated_to, escalation_type, created_at)
			VALUES ($1, $2, 'email', NOW())
		`, violation.ID, email)

		if err != nil {
			log.Printf("Failed to record escalation to %s: %v", email, err)
		}
	}
}

// blockUserAction blocks the user's current action or session
func (h *Handler) blockUserAction(violation ComplianceViolation) {
	// Update user's active sessions to mark them as blocked
	_, err := h.DB.Exec(`
		UPDATE sessions
		SET metadata = jsonb_set(
			COALESCE(metadata, '{}'::jsonb),
			'{blocked_by_compliance}',
			'true'::jsonb
		)
		WHERE user_id = $1 AND status = 'running'
	`, violation.UserID)

	if err != nil {
		log.Printf("Failed to block user action for %s: %v", violation.UserID, err)
	}
}

// suspendUserForViolation suspends the user account due to the violation
func (h *Handler) suspendUserForViolation(violation ComplianceViolation, policyName string) {
	_, err := h.DB.Exec(`
		UPDATE users
		SET status = 'suspended',
		    metadata = jsonb_set(
		        COALESCE(metadata, '{}'::jsonb),
		        '{suspension_reason}',
		        to_jsonb($1::text)
		    )
		WHERE user_id = $2
	`, fmt.Sprintf("Suspended due to %s violation: %s", policyName, violation.Description), violation.UserID)

	if err != nil {
		log.Printf("Failed to suspend user %s: %v", violation.UserID, err)
		return
	}

	// Terminate all active sessions
	_, err = h.DB.Exec(`
		UPDATE sessions
		SET status = 'terminated',
		    terminated_at = NOW(),
		    metadata = jsonb_set(
		        COALESCE(metadata, '{}'::jsonb),
		        '{termination_reason}',
		        '"compliance_violation"'::jsonb
		    )
		WHERE user_id = $1 AND status IN ('running', 'hibernated')
	`, violation.UserID)

	if err != nil {
		log.Printf("Failed to terminate sessions for suspended user %s: %v", violation.UserID, err)
	}
}

// ListViolations lists compliance violations
func (h *Handler) ListViolations(c *gin.Context) {
	userID := c.Query("user_id")
	policyID := c.Query("policy_id")
	status := c.Query("status")
	severity := c.Query("severity")

	query := `
		SELECT v.id, v.policy_id, p.name, v.user_id, v.violation_type, v.severity,
		       v.description, v.details, v.status, v.resolution, v.resolved_by,
		       v.resolved_at, v.created_at
		FROM compliance_violations v
		LEFT JOIN compliance_policies p ON v.policy_id = p.id
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if userID != "" {
		query += fmt.Sprintf(" AND v.user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}
	if policyID != "" {
		query += fmt.Sprintf(" AND v.policy_id = $%d", argCount)
		args = append(args, policyID)
		argCount++
	}
	if status != "" {
		query += fmt.Sprintf(" AND v.status = $%d", argCount)
		args = append(args, status)
		argCount++
	}
	if severity != "" {
		query += fmt.Sprintf(" AND v.severity = $%d", argCount)
		args = append(args, severity)
		argCount++
	}

	query += " ORDER BY v.created_at DESC LIMIT 100"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	violations := []ComplianceViolation{}
	for rows.Next() {
		var v ComplianceViolation
		var policyName, resolution, resolvedBy sql.NullString
		var resolvedAt sql.NullTime

		err := rows.Scan(&v.ID, &v.PolicyID, &policyName, &v.UserID, &v.ViolationType,
			&v.Severity, &v.Description, &v.Details, &v.Status, &resolution, &resolvedBy,
			&resolvedAt, &v.CreatedAt)

		if err != nil {
			continue
		}

		if policyName.Valid {
			v.PolicyName = policyName.String
		}
		if resolution.Valid {
			v.Resolution = resolution.String
		}
		if resolvedBy.Valid {
			v.ResolvedBy = resolvedBy.String
		}
		if resolvedAt.Valid {
			v.ResolvedAt = resolvedAt.Time
		}

		violations = append(violations, v)
	}

	c.JSON(http.StatusOK, gin.H{"violations": violations})
}

// ResolveViolation resolves a compliance violation
func (h *Handler) ResolveViolation(c *gin.Context) {
	violationID := c.Param("violationId")
	resolvedBy := c.GetString("user_id")

	var req struct {
		Resolution string `json:"resolution" binding:"required"`
		Status     string `json:"status" binding:"required,oneof=acknowledged remediated closed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.DB.Exec(`
		UPDATE compliance_violations
		SET status = $1, resolution = $2, resolved_by = $3, resolved_at = NOW()
		WHERE id = $4
	`, req.Status, req.Resolution, resolvedBy, violationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve violation"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "violation not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Violation resolved"})
}

// GenerateComplianceReport generates a compliance report
func (h *Handler) GenerateComplianceReport(c *gin.Context) {
	generatedBy := c.GetString("user_id")
	role := c.GetString("role")

	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can generate compliance reports"})
		return
	}

	var req struct {
		FrameworkID int64     `json:"framework_id,omitempty"`
		ReportType  string    `json:"report_type" binding:"required,oneof=summary detailed attestation"`
		StartDate   time.Time `json:"start_date" binding:"required"`
		EndDate     time.Time `json:"end_date" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get framework details
	var frameworkName string
	var controls []ComplianceControl
	if req.FrameworkID > 0 {
		h.DB.QueryRow(`SELECT display_name, controls FROM compliance_frameworks WHERE id = $1`,
			req.FrameworkID).Scan(&frameworkName, &controls)
	}

	// Get violations in period
	rows, err := h.DB.Query(`
		SELECT id, policy_id, user_id, violation_type, severity, description, details,
		       status, resolution, resolved_by, resolved_at, created_at
		FROM compliance_violations
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY severity DESC, created_at DESC
	`, req.StartDate, req.EndDate)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query violations"})
		return
	}
	defer rows.Close()

	violations := []ComplianceViolation{}
	for rows.Next() {
		var v ComplianceViolation
		var resolution, resolvedBy sql.NullString
		var resolvedAt sql.NullTime

		rows.Scan(&v.ID, &v.PolicyID, &v.UserID, &v.ViolationType, &v.Severity,
			&v.Description, &v.Details, &v.Status, &resolution, &resolvedBy,
			&resolvedAt, &v.CreatedAt)

		if resolution.Valid {
			v.Resolution = resolution.String
		}
		if resolvedBy.Valid {
			v.ResolvedBy = resolvedBy.String
		}
		if resolvedAt.Valid {
			v.ResolvedAt = resolvedAt.Time
		}

		violations = append(violations, v)
	}

	// Calculate compliance rate
	totalControls := len(controls)
	compliantControls := 0
	for _, ctrl := range controls {
		if ctrl.Status == "compliant" {
			compliantControls++
		}
	}

	complianceRate := 0.0
	if totalControls > 0 {
		complianceRate = (float64(compliantControls) / float64(totalControls)) * 100
	}

	overallStatus := "compliant"
	if complianceRate < 100 {
		overallStatus = "partial"
	}
	if complianceRate < 70 {
		overallStatus = "non_compliant"
	}

	report := ComplianceReport{
		FrameworkID:   req.FrameworkID,
		FrameworkName: frameworkName,
		ReportType:    req.ReportType,
		ReportPeriod: ReportPeriod{
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
		},
		OverallStatus: overallStatus,
		ControlsSummary: ControlsSummary{
			Total:         totalControls,
			Compliant:     compliantControls,
			NonCompliant:  totalControls - compliantControls,
			Unknown:       0,
			ComplianceRate: complianceRate,
		},
		Violations:  violations,
		GeneratedBy: generatedBy,
		GeneratedAt: time.Now(),
	}

	// Save report
	var reportID int64
	err = h.DB.QueryRow(`
		INSERT INTO compliance_reports
		(framework_id, report_type, start_date, end_date, overall_status, controls_summary,
		 violations, recommendations, generated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, req.FrameworkID, req.ReportType, req.StartDate, req.EndDate, overallStatus,
		report.ControlsSummary, violations, report.Recommendations, generatedBy).Scan(&reportID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save report"})
		return
	}

	report.ID = reportID

	c.JSON(http.StatusOK, report)
}

// GetComplianceDashboard gets compliance dashboard metrics
func (h *Handler) GetComplianceDashboard(c *gin.Context) {
	// Get total policies
	var totalPolicies, activePolicies int
	h.DB.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE enabled = true) FROM compliance_policies`).Scan(&totalPolicies, &activePolicies)

	// Get violations by severity
	rows, _ := h.DB.Query(`
		SELECT severity, COUNT(*) FROM compliance_violations
		WHERE status = 'open'
		GROUP BY severity
	`)
	defer rows.Close()

	violationsBySeverity := make(map[string]int)
	totalOpenViolations := 0
	for rows.Next() {
		var severity string
		var count int
		rows.Scan(&severity, &count)
		violationsBySeverity[severity] = count
		totalOpenViolations += count
	}

	// Get recent violations
	violationRows, _ := h.DB.Query(`
		SELECT id, policy_id, user_id, violation_type, severity, description, created_at
		FROM compliance_violations
		WHERE status = 'open'
		ORDER BY created_at DESC
		LIMIT 10
	`)
	defer violationRows.Close()

	recentViolations := []ComplianceViolation{}
	for violationRows.Next() {
		var v ComplianceViolation
		violationRows.Scan(&v.ID, &v.PolicyID, &v.UserID, &v.ViolationType, &v.Severity, &v.Description, &v.CreatedAt)
		recentViolations = append(recentViolations, v)
	}

	c.JSON(http.StatusOK, gin.H{
		"total_policies":        totalPolicies,
		"active_policies":       activePolicies,
		"total_open_violations": totalOpenViolations,
		"violations_by_severity": violationsBySeverity,
		"recent_violations":     recentViolations,
	})
}
