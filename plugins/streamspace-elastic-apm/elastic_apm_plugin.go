package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/streamspace/api/internal/plugins"
	"go.elastic.co/apm/v2"
)

// ElasticAPMPlugin integrates with Elastic APM for performance monitoring
type ElasticAPMPlugin struct {
	plugins.BasePlugin
	config       ElasticAPMConfig
	tracer       *apm.Tracer
	sessionStart map[string]time.Time
	sessionMutex sync.Mutex
}

// ElasticAPMConfig holds Elastic APM configuration
type ElasticAPMConfig struct {
	Enabled               bool              `json:"enabled"`
	ServerURL             string            `json:"serverUrl"`
	SecretToken           string            `json:"secretToken"`
	APIKey                string            `json:"apiKey"`
	ServiceName           string            `json:"serviceName"`
	ServiceVersion        string            `json:"serviceVersion"`
	Environment           string            `json:"environment"`
	TransactionSampleRate float64           `json:"transactionSampleRate"`
	CaptureBody           string            `json:"captureBody"`
	CaptureHeaders        bool              `json:"captureHeaders"`
	StackTraceLimit       int               `json:"stackTraceLimit"`
	TransactionMaxSpans   int               `json:"transactionMaxSpans"`
	GlobalLabels          map[string]string `json:"globalLabels"`
}

// Initialize sets up the Elastic APM plugin
func (p *ElasticAPMPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal Elastic APM config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("Elastic APM integration is disabled")
		return nil
	}

	if p.config.ServerURL == "" {
		return fmt.Errorf("Elastic APM server URL is required")
	}

	if p.config.ServiceName == "" {
		p.config.ServiceName = "streamspace"
	}

	// Initialize APM tracer
	p.tracer, err = apm.NewTracer(p.config.ServiceName, p.config.ServiceVersion)
	if err != nil {
		return fmt.Errorf("failed to create APM tracer: %w", err)
	}

	// Configure tracer (these would normally be set via environment variables)
	// Note: The actual Elastic APM Go agent primarily uses environment variables
	// This is a simplified example

	p.sessionStart = make(map[string]time.Time)

	ctx.Logger.Info("Elastic APM plugin initialized successfully",
		"service_name", p.config.ServiceName,
		"service_version", p.config.ServiceVersion,
		"environment", p.config.Environment,
		"sample_rate", p.config.TransactionSampleRate,
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *ElasticAPMPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Elastic APM plugin loaded")

	// Send custom event
	tx := p.tracer.StartTransaction("plugin.loaded", "plugin-lifecycle")
	defer tx.End()

	tx.Context.SetLabel("plugin", "streamspace-elastic-apm")
	tx.Context.SetLabel("version", "1.0.0")

	for k, v := range p.config.GlobalLabels {
		tx.Context.SetLabel(k, v)
	}

	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *ElasticAPMPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Elastic APM plugin unloading")

	// Flush and close tracer
	if p.tracer != nil {
		p.tracer.Flush(nil)
		p.tracer.Close()
	}

	return nil
}

// OnSessionCreated tracks session creation
func (p *ElasticAPMPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	// Track session start time
	p.sessionMutex.Lock()
	p.sessionStart[sessionID] = time.Now()
	p.sessionMutex.Unlock()

	// Create transaction
	tx := p.tracer.StartTransaction("session.created", "session-lifecycle")
	defer tx.End()

	tx.Context.SetLabel("session_id", sessionID)
	tx.Context.SetLabel("user_id", userID)
	tx.Context.SetLabel("template", templateName)

	for k, v := range p.config.GlobalLabels {
		tx.Context.SetLabel(k, v)
	}

	// Add custom context
	tx.Context.SetCustom("session", map[string]interface{}{
		"id":       sessionID,
		"user_id":  userID,
		"template": templateName,
	})

	return nil
}

// OnSessionTerminated tracks session termination
func (p *ElasticAPMPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	// Calculate duration
	p.sessionMutex.Lock()
	startTime, exists := p.sessionStart[sessionID]
	duration := time.Duration(0)
	if exists {
		duration = time.Since(startTime)
		delete(p.sessionStart, sessionID)
	}
	p.sessionMutex.Unlock()

	// Create transaction
	tx := p.tracer.StartTransaction("session.terminated", "session-lifecycle")
	defer tx.End()

	tx.Context.SetLabel("session_id", sessionID)
	tx.Context.SetLabel("user_id", userID)
	tx.Context.SetLabel("template", templateName)
	tx.Context.SetLabel("duration_seconds", fmt.Sprintf("%.2f", duration.Seconds()))

	for k, v := range p.config.GlobalLabels {
		tx.Context.SetLabel(k, v)
	}

	// Add custom metrics
	tx.Context.SetCustom("session", map[string]interface{}{
		"id":       sessionID,
		"user_id":  userID,
		"template": templateName,
		"duration": duration.Seconds(),
	})

	return nil
}

// OnSessionHeartbeat tracks session resource usage
func (p *ElasticAPMPlugin) OnSessionHeartbeat(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return nil
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])

	// Create a short transaction for heartbeat
	tx := p.tracer.StartTransaction("session.heartbeat", "session-monitor")
	defer tx.End()

	tx.Context.SetLabel("session_id", sessionID)

	// Add resource usage metrics
	if cpuUsage, ok := sessionMap["cpu_usage"].(float64); ok {
		tx.Context.SetLabel("cpu_usage", fmt.Sprintf("%.2f", cpuUsage*100))
	}

	if memoryUsage, ok := sessionMap["memory_usage"].(float64); ok {
		tx.Context.SetLabel("memory_mb", fmt.Sprintf("%.2f", memoryUsage/(1024*1024)))
	}

	return nil
}

// OnUserCreated tracks user creation
func (p *ElasticAPMPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user format")
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	// Create transaction
	tx := p.tracer.StartTransaction("user.created", "user-lifecycle")
	defer tx.End()

	tx.Context.SetLabel("user_id", userID)

	for k, v := range p.config.GlobalLabels {
		tx.Context.SetLabel(k, v)
	}

	return nil
}

// StartTransaction is a helper to start custom transactions
func (p *ElasticAPMPlugin) StartTransaction(name string, txType string) *apm.Transaction {
	if !p.config.Enabled {
		return nil
	}

	return p.tracer.StartTransaction(name, txType)
}

// RecordError records an error in APM
func (p *ElasticAPMPlugin) RecordError(err error, context map[string]interface{}) {
	if !p.config.Enabled {
		return
	}

	// Send error to APM
	apm.CaptureError(nil, err).Send()
}

// Export the plugin
func init() {
	plugins.Register("streamspace-elastic-apm", &ElasticAPMPlugin{})
}
