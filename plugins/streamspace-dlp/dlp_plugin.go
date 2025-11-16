package main

import (
	"encoding/json"
	"fmt"
	"time"
	"github.com/yourusername/streamspace/api/internal/plugins"
)

type DLPPlugin struct {
	plugins.BasePlugin
	config      DLPConfig
	policies    []DLPPolicy
}

type DLPConfig struct {
	Enabled                bool                   `json:"enabled"`
	DefaultPolicy          string                 `json:"defaultPolicy"`
	ClipboardControl       map[string]interface{} `json:"clipboardControl"`
	FileTransferControl    map[string]interface{} `json:"fileTransferControl"`
	ScreenCaptureControl   map[string]interface{} `json:"screenCaptureControl"`
	DeviceControl          map[string]interface{} `json:"deviceControl"`
	NetworkControl         map[string]interface{} `json:"networkControl"`
	ViolationActions       map[string]interface{} `json:"violationActions"`
}

type DLPPolicy struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Enabled              bool      `json:"enabled"`
	ClipboardDirection   string    `json:"clipboard_direction"`
	FileTransferEnabled  bool      `json:"file_transfer_enabled"`
	ScreenCaptureEnabled bool      `json:"screen_capture_enabled"`
	BlockOnViolation     bool      `json:"block_on_violation"`
	CreatedAt            time.Time `json:"created_at"`
}

type DLPViolation struct {
	ID            int64                  `json:"id"`
	PolicyID      int64                  `json:"policy_id"`
	UserID        string                 `json:"user_id"`
	ViolationType string                 `json:"violation_type"`
	Severity      string                 `json:"severity"`
	Description   string                 `json:"description"`
	Action        string                 `json:"action"`
	OccurredAt    time.Time              `json:"occurred_at"`
}

func (p *DLPPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("DLP plugin is disabled")
		return nil
	}

	p.createDatabaseTables(ctx)
	p.loadPolicies(ctx)

	ctx.Logger.Info("DLP plugin initialized", "policies", len(p.policies))
	return nil
}

func (p *DLPPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("DLP plugin loaded")
	return nil
}

func (p *DLPPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, _ := session.(map[string]interface{})
	userID := fmt.Sprintf("%v", sessionMap["user_id"])

	for _, policy := range p.policies {
		if policy.Enabled {
			p.enforcePolicy(ctx, policy, userID)
		}
	}
	return nil
}

func (p *DLPPlugin) createDatabaseTables(ctx *plugins.PluginContext) error {
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS dlp_policies (
		id SERIAL PRIMARY KEY, name VARCHAR(200), enabled BOOLEAN,
		clipboard_direction VARCHAR(50), file_transfer_enabled BOOLEAN,
		screen_capture_enabled BOOLEAN, block_on_violation BOOLEAN,
		created_at TIMESTAMP DEFAULT NOW()
	)`)
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS dlp_violations (
		id SERIAL PRIMARY KEY, policy_id INTEGER, user_id VARCHAR(255),
		violation_type VARCHAR(100), severity VARCHAR(50), description TEXT,
		action VARCHAR(50), occurred_at TIMESTAMP DEFAULT NOW()
	)`)
	return nil
}

func (p *DLPPlugin) loadPolicies(ctx *plugins.PluginContext) error {
	rows, _ := ctx.Database.Query(`SELECT id, name, enabled, clipboard_direction, 
		file_transfer_enabled, screen_capture_enabled, block_on_violation, created_at 
		FROM dlp_policies WHERE enabled = true`)
	defer rows.Close()

	for rows.Next() {
		var policy DLPPolicy
		rows.Scan(&policy.ID, &policy.Name, &policy.Enabled, &policy.ClipboardDirection,
			&policy.FileTransferEnabled, &policy.ScreenCaptureEnabled,
			&policy.BlockOnViolation, &policy.CreatedAt)
		p.policies = append(p.policies, policy)
	}
	return nil
}

func (p *DLPPlugin) enforcePolicy(ctx *plugins.PluginContext, policy DLPPolicy, userID string) {
	ctx.Logger.Debug("Enforcing DLP policy", "policy", policy.Name, "user", userID)
}

func init() {
	plugins.Register("streamspace-dlp", &DLPPlugin{})
}
