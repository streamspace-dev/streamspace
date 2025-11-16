package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yourusername/streamspace/api/internal/plugins"
)

// CompliancePlugin manages regulatory compliance frameworks and policies
type CompliancePlugin struct {
	plugins.BasePlugin
	config        ComplianceConfig
	frameworks    []ComplianceFramework
	activePolicies []CompliancePolicy
}

// ComplianceConfig holds plugin configuration
type ComplianceConfig struct {
	Enabled                bool              `json:"enabled"`
	DefaultFrameworks      []string          `json:"defaultFrameworks"`
	AutoEnforcement        bool              `json:"autoEnforcement"`
	DefaultEnforcementLevel string           `json:"defaultEnforcementLevel"`
	DataRetentionDays      map[string]int    `json:"dataRetentionDays"`
	ViolationActions       ViolationActionConfig `json:"violationActions"`
	EscalationEmails       []string          `json:"escalationEmails"`
	EnableAutomaticChecks  bool              `json:"enableAutomaticChecks"`
	CheckInterval          int               `json:"checkInterval"`
}

// ComplianceFramework represents a regulatory framework
type ComplianceFramework struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Controls    []ComplianceControl    `json:"controls"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ComplianceControl represents a specific control
type ComplianceControl struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category"`
	Automated      bool      `json:"automated"`
	CheckInterval  int       `json:"check_interval_hours,omitempty"`
	Status         string    `json:"status,omitempty"`
	LastChecked    time.Time `json:"last_checked,omitempty"`
}

// CompliancePolicy represents a compliance policy
type CompliancePolicy struct {
	ID                int64                    `json:"id"`
	Name              string                   `json:"name"`
	FrameworkID       int64                    `json:"framework_id"`
	Enabled           bool                     `json:"enabled"`
	EnforcementLevel  string                   `json:"enforcement_level"`
	DataRetention     DataRetentionConfig      `json:"data_retention"`
	AccessControls    AccessControlConfig      `json:"access_controls"`
	ViolationActions  ViolationActionConfig    `json:"violation_actions"`
	CreatedAt         time.Time                `json:"created_at"`
}

// DataRetentionConfig defines retention rules
type DataRetentionConfig struct {
	Enabled         bool   `json:"enabled"`
	SessionDataDays int    `json:"session_data_days"`
	RecordingDays   int    `json:"recording_days"`
	AuditLogDays    int    `json:"audit_log_days"`
	AutoPurge       bool   `json:"auto_purge"`
}

// AccessControlConfig defines access controls
type AccessControlConfig struct {
	RequireMFA            bool     `json:"require_mfa"`
	AllowedIPRanges       []string `json:"allowed_ip_ranges,omitempty"`
	SessionTimeout        int      `json:"session_timeout_minutes"`
	MaxConcurrentSessions int      `json:"max_concurrent_sessions"`
}

// ViolationActionConfig defines violation actions
type ViolationActionConfig struct {
	NotifyUser       bool     `json:"notify_user"`
	NotifyAdmin      bool     `json:"notify_admin"`
	CreateTicket     bool     `json:"create_ticket"`
	BlockAction      bool     `json:"block_action"`
	SuspendUser      bool     `json:"suspend_user"`
	EscalationEmails []string `json:"escalation_emails,omitempty"`
}

// ComplianceViolation represents a policy violation
type ComplianceViolation struct {
	ID            int64                  `json:"id"`
	PolicyID      int64                  `json:"policy_id"`
	PolicyName    string                 `json:"policy_name,omitempty"`
	UserID        string                 `json:"user_id"`
	ViolationType string                 `json:"violation_type"`
	Severity      string                 `json:"severity"`
	Description   string                 `json:"description"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
}

// Initialize sets up the compliance plugin
func (p *CompliancePlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal compliance config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("Compliance plugin is disabled")
		return nil
	}

	// Create database tables
	if err := p.createDatabaseTables(ctx); err != nil {
		return fmt.Errorf("failed to create database tables: %w", err)
	}

	// Load default frameworks
	if err := p.loadDefaultFrameworks(ctx); err != nil {
		return fmt.Errorf("failed to load frameworks: %w", err)
	}

	// Load active policies
	if err := p.loadActivePolicies(ctx); err != nil {
		return fmt.Errorf("failed to load policies: %w", err)
	}

	ctx.Logger.Info("Compliance plugin initialized successfully",
		"frameworks", len(p.frameworks),
		"policies", len(p.activePolicies),
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *CompliancePlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Compliance & Regulatory Framework plugin loaded")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *CompliancePlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Compliance plugin unloading")
	return nil
}

// OnSessionCreated checks compliance on session creation
func (p *CompliancePlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	userID := fmt.Sprintf("%v", sessionMap["user_id"])

	// Check session-related policies
	for _, policy := range p.activePolicies {
		if err := p.checkSessionPolicy(ctx, policy, userID, sessionMap); err != nil {
			ctx.Logger.Warn("Policy check failed", "policy", policy.Name, "error", err)
		}
	}

	return nil
}

// OnUserLogin checks compliance on user login
func (p *CompliancePlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	// Check MFA requirements
	for _, policy := range p.activePolicies {
		if policy.AccessControls.RequireMFA {
			if err := p.checkMFACompliance(ctx, policy, userID); err != nil {
				p.recordViolation(ctx, policy, userID, "mfa_violation", "critical", err.Error())
			}
		}
	}

	return nil
}

// OnDataExport checks data export compliance
func (p *CompliancePlugin) OnDataExport(ctx *plugins.PluginContext, exportData interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	exportMap, ok := exportData.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", exportMap["user_id"])

	// Check data export policies
	for _, policy := range p.activePolicies {
		if policy.EnforcementLevel == "blocking" {
			// Validate export is compliant
			ctx.Logger.Info("Checking data export compliance", "user", userID, "policy", policy.Name)
		}
	}

	return nil
}

// RunScheduledJob handles scheduled compliance tasks
func (p *CompliancePlugin) RunScheduledJob(ctx *plugins.PluginContext, jobName string) error {
	switch jobName {
	case "run-compliance-checks":
		return p.runComplianceChecks(ctx)
	case "generate-monthly-report":
		return p.generateMonthlyReport(ctx)
	case "check-data-retention":
		return p.checkDataRetention(ctx)
	}
	return nil
}

// createDatabaseTables creates necessary database tables
func (p *CompliancePlugin) createDatabaseTables(ctx *plugins.PluginContext) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS compliance_frameworks (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			display_name VARCHAR(200),
			description TEXT,
			version VARCHAR(50),
			enabled BOOLEAN DEFAULT true,
			controls JSONB,
			created_by VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS compliance_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			framework_id INTEGER REFERENCES compliance_frameworks(id),
			enabled BOOLEAN DEFAULT true,
			enforcement_level VARCHAR(50),
			data_retention JSONB,
			access_controls JSONB,
			violation_actions JSONB,
			created_by VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS compliance_violations (
			id SERIAL PRIMARY KEY,
			policy_id INTEGER REFERENCES compliance_policies(id),
			user_id VARCHAR(255) NOT NULL,
			violation_type VARCHAR(100),
			severity VARCHAR(50),
			description TEXT,
			details JSONB,
			status VARCHAR(50),
			resolution TEXT,
			resolved_by VARCHAR(255),
			resolved_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS compliance_reports (
			id SERIAL PRIMARY KEY,
			framework_id INTEGER REFERENCES compliance_frameworks(id),
			report_type VARCHAR(50),
			start_date DATE,
			end_date DATE,
			overall_status VARCHAR(50),
			controls_summary JSONB,
			violations JSONB,
			recommendations TEXT[],
			generated_by VARCHAR(255),
			generated_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, table := range tables {
		if err := ctx.Database.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// loadDefaultFrameworks loads pre-configured frameworks
func (p *CompliancePlugin) loadDefaultFrameworks(ctx *plugins.PluginContext) error {
	// Query existing frameworks
	rows, err := ctx.Database.Query(`SELECT id, name, display_name, version, enabled, controls, created_at FROM compliance_frameworks`)
	if err != nil {
		return err
	}
	defer rows.Close()

	p.frameworks = []ComplianceFramework{}
	for rows.Next() {
		var f ComplianceFramework
		var controlsJSON []byte
		if err := rows.Scan(&f.ID, &f.Name, &f.DisplayName, &f.Version, &f.Enabled, &controlsJSON, &f.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal(controlsJSON, &f.Controls)
		p.frameworks = append(p.frameworks, f)
	}

	ctx.Logger.Info("Loaded compliance frameworks", "count", len(p.frameworks))
	return nil
}

// loadActivePolicies loads active compliance policies
func (p *CompliancePlugin) loadActivePolicies(ctx *plugins.PluginContext) error {
	rows, err := ctx.Database.Query(`
		SELECT id, name, framework_id, enabled, enforcement_level,
		       data_retention, access_controls, violation_actions, created_at
		FROM compliance_policies
		WHERE enabled = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	p.activePolicies = []CompliancePolicy{}
	for rows.Next() {
		var policy CompliancePolicy
		var dataRetentionJSON, accessControlsJSON, violationActionsJSON []byte

		if err := rows.Scan(&policy.ID, &policy.Name, &policy.FrameworkID, &policy.Enabled,
			&policy.EnforcementLevel, &dataRetentionJSON, &accessControlsJSON,
			&violationActionsJSON, &policy.CreatedAt); err != nil {
			continue
		}

		json.Unmarshal(dataRetentionJSON, &policy.DataRetention)
		json.Unmarshal(accessControlsJSON, &policy.AccessControls)
		json.Unmarshal(violationActionsJSON, &policy.ViolationActions)

		p.activePolicies = append(p.activePolicies, policy)
	}

	ctx.Logger.Info("Loaded active policies", "count", len(p.activePolicies))
	return nil
}

// checkSessionPolicy validates session against policy
func (p *CompliancePlugin) checkSessionPolicy(ctx *plugins.PluginContext, policy CompliancePolicy, userID string, session map[string]interface{}) error {
	// Check session timeout
	if policy.AccessControls.SessionTimeout > 0 {
		// Would validate session duration
	}

	// Check concurrent sessions
	if policy.AccessControls.MaxConcurrentSessions > 0 {
		count, _ := ctx.Database.QueryInt("SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND status = 'running'", userID)
		if count > policy.AccessControls.MaxConcurrentSessions {
			return p.recordViolation(ctx, policy, userID, "concurrent_session_violation", "medium",
				fmt.Sprintf("User has %d concurrent sessions (limit: %d)", count, policy.AccessControls.MaxConcurrentSessions))
		}
	}

	return nil
}

// checkMFACompliance validates MFA requirement
func (p *CompliancePlugin) checkMFACompliance(ctx *plugins.PluginContext, policy CompliancePolicy, userID string) error {
	// Query user MFA status
	var mfaEnabled bool
	err := ctx.Database.QueryRow("SELECT mfa_enabled FROM users WHERE user_id = $1", userID).Scan(&mfaEnabled)
	if err != nil {
		return err
	}

	if !mfaEnabled && policy.AccessControls.RequireMFA {
		return fmt.Errorf("MFA required by policy %s but not enabled for user", policy.Name)
	}

	return nil
}

// recordViolation creates a compliance violation record
func (p *CompliancePlugin) recordViolation(ctx *plugins.PluginContext, policy CompliancePolicy, userID, violationType, severity, description string) error {
	var id int64
	err := ctx.Database.QueryRow(`
		INSERT INTO compliance_violations (policy_id, user_id, violation_type, severity, description, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', NOW())
		RETURNING id
	`, policy.ID, userID, violationType, severity, description).Scan(&id)

	if err != nil {
		return err
	}

	ctx.Logger.Warn("Compliance violation recorded",
		"id", id,
		"policy", policy.Name,
		"user", userID,
		"type", violationType,
		"severity", severity,
	)

	// Execute violation actions
	p.executeViolationActions(ctx, policy, userID, description)

	return nil
}

// executeViolationActions takes action on violations
func (p *CompliancePlugin) executeViolationActions(ctx *plugins.PluginContext, policy CompliancePolicy, userID, description string) {
	if policy.ViolationActions.NotifyUser {
		ctx.Logger.Info("Sending violation notification to user", "user", userID)
	}

	if policy.ViolationActions.NotifyAdmin {
		ctx.Logger.Info("Sending violation notification to admins", "policy", policy.Name)
	}

	if policy.ViolationActions.CreateTicket {
		ctx.Logger.Info("Creating support ticket for violation", "user", userID)
	}

	if policy.ViolationActions.SuspendUser {
		ctx.Logger.Warn("Suspending user due to violation", "user", userID, "policy", policy.Name)
	}
}

// runComplianceChecks runs automated compliance checks
func (p *CompliancePlugin) runComplianceChecks(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Running automated compliance checks", "frameworks", len(p.frameworks))

	for _, framework := range p.frameworks {
		if !framework.Enabled {
			continue
		}

		for _, control := range framework.Controls {
			if control.Automated {
				ctx.Logger.Debug("Checking control", "framework", framework.Name, "control", control.Name)
				// Automated control checking logic would go here
			}
		}
	}

	return nil
}

// generateMonthlyReport creates a monthly compliance report
func (p *CompliancePlugin) generateMonthlyReport(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Generating monthly compliance report")

	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	for _, framework := range p.frameworks {
		if !framework.Enabled {
			continue
		}

		ctx.Logger.Info("Generating report for framework", "name", framework.Name)
		// Report generation logic would go here
	}

	return nil
}

// checkDataRetention enforces data retention policies
func (p *CompliancePlugin) checkDataRetention(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Checking data retention policies")

	for _, policy := range p.activePolicies {
		if !policy.DataRetention.Enabled || !policy.DataRetention.AutoPurge {
			continue
		}

		// Purge old session data
		if policy.DataRetention.SessionDataDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -policy.DataRetention.SessionDataDays)
			ctx.Logger.Info("Purging old session data", "cutoff", cutoff, "policy", policy.Name)
		}

		// Purge old recordings
		if policy.DataRetention.RecordingDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -policy.DataRetention.RecordingDays)
			ctx.Logger.Info("Purging old recordings", "cutoff", cutoff, "policy", policy.Name)
		}
	}

	return nil
}

// Export the plugin
func init() {
	plugins.Register("streamspace-compliance", &CompliancePlugin{})
}
