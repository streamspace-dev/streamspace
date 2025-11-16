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

// NewRelicPlugin sends metrics, events, and traces to New Relic
type NewRelicPlugin struct {
	plugins.BasePlugin
	config        NewRelicConfig
	httpClient    *http.Client
	metricsBuffer []NewRelicMetric
	eventsBuffer  []NewRelicEvent
	bufferMutex   sync.Mutex
	sessionStart  map[string]time.Time
	sessionMutex  sync.Mutex
}

// NewRelicConfig holds New Relic configuration
type NewRelicConfig struct {
	Enabled              bool              `json:"enabled"`
	LicenseKey           string            `json:"licenseKey"`
	AccountID            string            `json:"accountId"`
	Region               string            `json:"region"`
	AppName              string            `json:"appName"`
	EnableMetrics        bool              `json:"enableMetrics"`
	EnableEvents         bool              `json:"enableEvents"`
	EnableTraces         bool              `json:"enableTraces"`
	EnableLogs           bool              `json:"enableLogs"`
	MetricsInterval      int               `json:"metricsInterval"`
	TrackSessionMetrics  bool              `json:"trackSessionMetrics"`
	TrackResourceMetrics bool              `json:"trackResourceMetrics"`
	TrackUserMetrics     bool              `json:"trackUserMetrics"`
	CustomAttributes     map[string]string `json:"customAttributes"`
}

// NewRelicMetric represents a New Relic metric
type NewRelicMetric struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Value      interface{}            `json:"value"`
	Timestamp  int64                  `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}

// NewRelicEvent represents a New Relic custom event
type NewRelicEvent struct {
	EventType  string                 `json:"eventType"`
	Timestamp  int64                  `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Initialize sets up the plugin
func (p *NewRelicPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal New Relic config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("New Relic integration is disabled")
		return nil
	}

	if p.config.LicenseKey == "" {
		return fmt.Errorf("New Relic license key is required")
	}

	if p.config.Region == "" {
		p.config.Region = "US"
	}

	if p.config.AppName == "" {
		p.config.AppName = "StreamSpace"
	}

	// Initialize HTTP client
	p.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize buffers
	p.sessionStart = make(map[string]time.Time)
	p.metricsBuffer = []NewRelicMetric{}
	p.eventsBuffer = []NewRelicEvent{}

	ctx.Logger.Info("New Relic plugin initialized successfully",
		"region", p.config.Region,
		"app_name", p.config.AppName,
		"metrics_enabled", p.config.EnableMetrics,
		"events_enabled", p.config.EnableEvents,
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *NewRelicPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("New Relic monitoring plugin loaded")
	return p.sendEvent(ctx, "PluginLoaded", map[string]interface{}{
		"pluginName":    "streamspace-newrelic",
		"pluginVersion": "1.0.0",
		"status":        "active",
	})
}

// OnUnload is called when the plugin is unloaded
func (p *NewRelicPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("New Relic monitoring plugin unloading")

	// Flush any remaining metrics and events
	if err := p.flushMetrics(ctx); err != nil {
		ctx.Logger.Error("Failed to flush metrics on unload", "error", err)
	}
	if err := p.flushEvents(ctx); err != nil {
		ctx.Logger.Error("Failed to flush events on unload", "error", err)
	}

	return p.sendEvent(ctx, "PluginUnloaded", map[string]interface{}{
		"pluginName": "streamspace-newrelic",
		"status":     "inactive",
	})
}

// OnSessionCreated tracks session creation
func (p *NewRelicPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackSessionMetrics {
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

	// Add metrics
	attrs := p.getBaseAttributes()
	attrs["userId"] = userID
	attrs["template"] = templateName
	attrs["sessionId"] = sessionID

	p.addMetric("streamspace.session.created", "count", 1, attrs)
	p.addMetric("streamspace.session.active", "gauge", 1, attrs)

	// Add event
	return p.sendEvent(ctx, "SessionCreated", map[string]interface{}{
		"sessionId": sessionID,
		"userId":    userID,
		"template":  templateName,
	})
}

// OnSessionTerminated tracks session termination
func (p *NewRelicPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackSessionMetrics {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	// Calculate session duration
	p.sessionMutex.Lock()
	startTime, exists := p.sessionStart[sessionID]
	duration := 0.0
	if exists {
		duration = time.Since(startTime).Seconds()
		delete(p.sessionStart, sessionID)
	}
	p.sessionMutex.Unlock()

	// Add metrics
	attrs := p.getBaseAttributes()
	attrs["userId"] = userID
	attrs["template"] = templateName
	attrs["sessionId"] = sessionID

	p.addMetric("streamspace.session.terminated", "count", 1, attrs)
	p.addMetric("streamspace.session.duration", "gauge", duration, attrs)

	// Add event
	return p.sendEvent(ctx, "SessionTerminated", map[string]interface{}{
		"sessionId": sessionID,
		"userId":    userID,
		"template":  templateName,
		"duration":  duration,
	})
}

// OnSessionHeartbeat tracks session resource usage
func (p *NewRelicPlugin) OnSessionHeartbeat(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled || !p.config.TrackResourceMetrics {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return nil
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	attrs := p.getBaseAttributes()
	attrs["sessionId"] = sessionID
	attrs["userId"] = userID
	attrs["template"] = templateName

	// Track resource usage
	if cpuUsage, ok := sessionMap["cpu_usage"].(float64); ok {
		p.addMetric("streamspace.session.cpu", "gauge", cpuUsage*100, attrs)
	}

	if memoryUsage, ok := sessionMap["memory_usage"].(float64); ok {
		p.addMetric("streamspace.session.memory", "gauge", memoryUsage, attrs)
	}

	if storageUsage, ok := sessionMap["storage_usage"].(float64); ok {
		p.addMetric("streamspace.session.storage", "gauge", storageUsage, attrs)
	}

	return nil
}

// OnUserCreated tracks user creation
func (p *NewRelicPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user format")
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	attrs := p.getBaseAttributes()
	attrs["userId"] = userID

	p.addMetric("streamspace.user.created", "count", 1, attrs)

	return p.sendEvent(ctx, "UserCreated", map[string]interface{}{
		"userId": userID,
	})
}

// OnUserLogin tracks user login
func (p *NewRelicPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	attrs := p.getBaseAttributes()
	attrs["userId"] = userID

	p.addMetric("streamspace.user.login", "count", 1, attrs)

	return nil
}

// OnUserLogout tracks user logout
func (p *NewRelicPlugin) OnUserLogout(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	attrs := p.getBaseAttributes()
	attrs["userId"] = userID

	p.addMetric("streamspace.user.logout", "count", 1, attrs)

	return nil
}

// RunScheduledJob handles the scheduled metrics/events flush
func (p *NewRelicPlugin) RunScheduledJob(ctx *plugins.PluginContext, jobName string) error {
	if jobName == "send-metrics" {
		if err := p.flushMetrics(ctx); err != nil {
			ctx.Logger.Error("Failed to flush metrics", "error", err)
		}
		if err := p.flushEvents(ctx); err != nil {
			ctx.Logger.Error("Failed to flush events", "error", err)
		}
	}
	return nil
}

// getBaseAttributes returns base attributes with custom attributes
func (p *NewRelicPlugin) getBaseAttributes() map[string]interface{} {
	attrs := make(map[string]interface{})
	for k, v := range p.config.CustomAttributes {
		attrs[k] = v
	}
	attrs["appName"] = p.config.AppName
	return attrs
}

// addMetric adds a metric to the buffer
func (p *NewRelicPlugin) addMetric(name, metricType string, value interface{}, attributes map[string]interface{}) {
	p.bufferMutex.Lock()
	defer p.bufferMutex.Unlock()

	metric := NewRelicMetric{
		Name:       name,
		Type:       metricType,
		Value:      value,
		Timestamp:  time.Now().Unix(),
		Attributes: attributes,
	}

	p.metricsBuffer = append(p.metricsBuffer, metric)
}

// sendEvent adds an event to the buffer
func (p *NewRelicPlugin) sendEvent(ctx *plugins.PluginContext, eventType string, attributes map[string]interface{}) error {
	if !p.config.Enabled || !p.config.EnableEvents {
		return nil
	}

	// Merge with base attributes
	allAttrs := p.getBaseAttributes()
	for k, v := range attributes {
		allAttrs[k] = v
	}

	p.bufferMutex.Lock()
	defer p.bufferMutex.Unlock()

	event := NewRelicEvent{
		EventType:  eventType,
		Timestamp:  time.Now().Unix(),
		Attributes: allAttrs,
	}

	p.eventsBuffer = append(p.eventsBuffer, event)
	return nil
}

// flushMetrics sends buffered metrics to New Relic
func (p *NewRelicPlugin) flushMetrics(ctx *plugins.PluginContext) error {
	if !p.config.Enabled || !p.config.EnableMetrics {
		return nil
	}

	p.bufferMutex.Lock()
	if len(p.metricsBuffer) == 0 {
		p.bufferMutex.Unlock()
		return nil
	}

	metrics := make([]NewRelicMetric, len(p.metricsBuffer))
	copy(metrics, p.metricsBuffer)
	p.metricsBuffer = []NewRelicMetric{}
	p.bufferMutex.Unlock()

	// Send to New Relic
	payload := []map[string]interface{}{}
	for _, m := range metrics {
		payload = append(payload, map[string]interface{}{
			"name":       m.Name,
			"type":       m.Type,
			"value":      m.Value,
			"timestamp":  m.Timestamp,
			"attributes": m.Attributes,
		})
	}

	return p.sendToNewRelic(ctx, "metrics", payload)
}

// flushEvents sends buffered events to New Relic
func (p *NewRelicPlugin) flushEvents(ctx *plugins.PluginContext) error {
	if !p.config.Enabled || !p.config.EnableEvents {
		return nil
	}

	p.bufferMutex.Lock()
	if len(p.eventsBuffer) == 0 {
		p.bufferMutex.Unlock()
		return nil
	}

	events := make([]NewRelicEvent, len(p.eventsBuffer))
	copy(events, p.eventsBuffer)
	p.eventsBuffer = []NewRelicEvent{}
	p.bufferMutex.Unlock()

	// Convert to payload format
	payload := []map[string]interface{}{}
	for _, e := range events {
		eventMap := map[string]interface{}{
			"eventType": e.EventType,
			"timestamp": e.Timestamp,
		}
		for k, v := range e.Attributes {
			eventMap[k] = v
		}
		payload = append(payload, eventMap)
	}

	return p.sendToNewRelic(ctx, "events", payload)
}

// sendToNewRelic sends data to New Relic Insights API
func (p *NewRelicPlugin) sendToNewRelic(ctx *plugins.PluginContext, dataType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build URL based on region and data type
	baseURL := "https://insights-collector.newrelic.com"
	if p.config.Region == "EU" {
		baseURL = "https://insights-collector.eu01.nr-data.net"
	}

	endpoint := "v1/accounts/" + p.config.AccountID
	if dataType == "events" {
		endpoint += "/events"
	} else {
		endpoint += "/metrics"
	}

	url := baseURL + "/" + endpoint

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.config.LicenseKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send to New Relic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("New Relic API returned status %d: %s", resp.StatusCode, string(body))
	}

	ctx.Logger.Info("Sent data to New Relic", "type", dataType, "count", len(payload.([]map[string]interface{})))
	return nil
}

// Export the plugin
func init() {
	plugins.Register("streamspace-newrelic", &NewRelicPlugin{})
}
