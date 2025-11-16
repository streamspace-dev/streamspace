package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/yourusername/streamspace/api/internal/plugins"
)

// HoneycombPlugin sends high-cardinality observability events to Honeycomb
type HoneycombPlugin struct {
	plugins.BasePlugin
	config       HoneycombConfig
	httpClient   *http.Client
	eventBuffer  []HoneycombEvent
	bufferMutex  sync.Mutex
	sessionStart map[string]time.Time
	sessionMutex sync.Mutex
}

// HoneycombConfig holds Honeycomb configuration
type HoneycombConfig struct {
	Enabled        bool              `json:"enabled"`
	APIKey         string            `json:"apiKey"`
	Dataset        string            `json:"dataset"`
	APIHost        string            `json:"apiHost"`
	SampleRate     int               `json:"sampleRate"`
	SendFrequency  int               `json:"sendFrequency"`
	MaxBatchSize   int               `json:"maxBatchSize"`
	TrackSessions  bool              `json:"trackSessions"`
	TrackResources bool              `json:"trackResources"`
	TrackUsers     bool              `json:"trackUsers"`
	EnableTracing  bool              `json:"enableTracing"`
	CustomFields   map[string]string `json:"customFields"`
}

// HoneycombEvent represents a single event sent to Honeycomb
type HoneycombEvent struct {
	Timestamp  time.Time              `json:"time"`
	Data       map[string]interface{} `json:"data"`
	SampleRate int                    `json:"samplerate,omitempty"`
}

// HoneycombBatch represents a batch of events
type HoneycombBatch []HoneycombEvent

// Initialize sets up the Honeycomb plugin
func (p *HoneycombPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal Honeycomb config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("Honeycomb integration is disabled")
		return nil
	}

	if p.config.APIKey == "" {
		return fmt.Errorf("Honeycomb API key is required")
	}

	if p.config.Dataset == "" {
		return fmt.Errorf("Honeycomb dataset is required")
	}

	if p.config.APIHost == "" {
		p.config.APIHost = "https://api.honeycomb.io"
	}

	if p.config.SampleRate < 1 {
		p.config.SampleRate = 1
	}

	// Initialize HTTP client
	p.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize buffers
	p.eventBuffer = []HoneycombEvent{}
	p.sessionStart = make(map[string]time.Time)

	ctx.Logger.Info("Honeycomb plugin initialized successfully",
		"dataset", p.config.Dataset,
		"api_host", p.config.APIHost,
		"sample_rate", p.config.SampleRate,
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *HoneycombPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Honeycomb observability plugin loaded")

	return p.sendEvent("plugin.loaded", map[string]interface{}{
		"plugin_name":    "streamspace-honeycomb",
		"plugin_version": "1.0.0",
		"status":         "active",
	})
}

// OnUnload is called when the plugin is unloaded
func (p *HoneycombPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Honeycomb observability plugin unloading")

	// Flush any remaining events
	if err := p.flushEvents(ctx); err != nil {
		ctx.Logger.Error("Failed to flush events on unload", "error", err)
	}

	return p.sendEvent("plugin.unloaded", map[string]interface{}{
		"plugin_name": "streamspace-honeycomb",
		"status":      "inactive",
	})
}

// OnSessionCreated tracks session creation
func (p *HoneycombPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackSessions {
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

	// Send event
	return p.sendEvent("session.created", map[string]interface{}{
		"session_id":   sessionID,
		"user_id":      userID,
		"template":     templateName,
		"event_type":   "session_lifecycle",
		"duration_ms":  0,
	})
}

// OnSessionTerminated tracks session termination
func (p *HoneycombPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackSessions {
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
	durationMs := int64(0)
	if exists {
		durationMs = time.Since(startTime).Milliseconds()
		delete(p.sessionStart, sessionID)
	}
	p.sessionMutex.Unlock()

	// Send event
	return p.sendEvent("session.terminated", map[string]interface{}{
		"session_id":   sessionID,
		"user_id":      userID,
		"template":     templateName,
		"event_type":   "session_lifecycle",
		"duration_ms":  durationMs,
		"duration_sec": float64(durationMs) / 1000.0,
	})
}

// OnSessionHeartbeat tracks session resource usage
func (p *HoneycombPlugin) OnSessionHeartbeat(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackResources {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return nil
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	data := map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"template":   templateName,
		"event_type": "resource_usage",
	}

	// Add resource metrics
	if cpuUsage, ok := sessionMap["cpu_usage"].(float64); ok {
		data["cpu_usage_percent"] = cpuUsage * 100
	}

	if memoryUsage, ok := sessionMap["memory_usage"].(float64); ok {
		data["memory_bytes"] = memoryUsage
		data["memory_mb"] = memoryUsage / (1024 * 1024)
	}

	if storageUsage, ok := sessionMap["storage_usage"].(float64); ok {
		data["storage_bytes"] = storageUsage
		data["storage_mb"] = storageUsage / (1024 * 1024)
	}

	return p.sendEvent("session.heartbeat", data)
}

// OnUserCreated tracks user creation
func (p *HoneycombPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUsers {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user format")
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	return p.sendEvent("user.created", map[string]interface{}{
		"user_id":    userID,
		"event_type": "user_lifecycle",
	})
}

// OnUserLogin tracks user login
func (p *HoneycombPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUsers {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	return p.sendEvent("user.login", map[string]interface{}{
		"user_id":    userID,
		"event_type": "user_activity",
	})
}

// OnUserLogout tracks user logout
func (p *HoneycombPlugin) OnUserLogout(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUsers {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	return p.sendEvent("user.logout", map[string]interface{}{
		"user_id":    userID,
		"event_type": "user_activity",
	})
}

// RunScheduledJob handles the scheduled event flush
func (p *HoneycombPlugin) RunScheduledJob(ctx *plugins.PluginContext, jobName string) error {
	if jobName == "flush-events" {
		return p.flushEvents(ctx)
	}
	return nil
}

// sendEvent adds an event to the buffer
func (p *HoneycombPlugin) sendEvent(name string, data map[string]interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	// Merge with custom fields
	eventData := make(map[string]interface{})
	for k, v := range p.config.CustomFields {
		eventData[k] = v
	}
	for k, v := range data {
		eventData[k] = v
	}

	// Add event name
	eventData["name"] = name

	p.bufferMutex.Lock()
	defer p.bufferMutex.Unlock()

	event := HoneycombEvent{
		Timestamp:  time.Now(),
		Data:       eventData,
		SampleRate: p.config.SampleRate,
	}

	p.eventBuffer = append(p.eventBuffer, event)

	// Auto-flush if batch size reached
	if len(p.eventBuffer) >= p.config.MaxBatchSize {
		go p.flushEvents(nil)
	}

	return nil
}

// flushEvents sends buffered events to Honeycomb
func (p *HoneycombPlugin) flushEvents(ctx *plugins.PluginContext) error {
	if !p.config.Enabled {
		return nil
	}

	p.bufferMutex.Lock()
	if len(p.eventBuffer) == 0 {
		p.bufferMutex.Unlock()
		return nil
	}

	// Get events and clear buffer
	events := make([]HoneycombEvent, len(p.eventBuffer))
	copy(events, p.eventBuffer)
	p.eventBuffer = []HoneycombEvent{}
	p.bufferMutex.Unlock()

	// Send to Honeycomb
	payloadBytes, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	url := fmt.Sprintf("%s/1/batch/%s", p.config.APIHost, p.config.Dataset)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Honeycomb-Team", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Honeycomb API returned status %d: %s", resp.StatusCode, string(body))
	}

	if ctx != nil {
		ctx.Logger.Info("Sent events to Honeycomb", "count", len(events))
	}

	return nil
}

// Export the plugin
func init() {
	plugins.Register("streamspace-honeycomb", &HoneycombPlugin{})
}
