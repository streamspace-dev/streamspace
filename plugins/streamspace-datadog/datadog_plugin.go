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

// DatadogPlugin sends metrics, traces, and logs to Datadog
type DatadogPlugin struct {
	plugins.BasePlugin
	config        DatadogConfig
	httpClient    *http.Client
	metricsBuffer []DatadogMetric
	metricsMutex  sync.Mutex
	sessionStart  map[string]time.Time
	sessionMutex  sync.Mutex
}

// DatadogConfig holds Datadog configuration
type DatadogConfig struct {
	Enabled              bool     `json:"enabled"`
	APIKey               string   `json:"apiKey"`
	AppKey               string   `json:"appKey"`
	Site                 string   `json:"site"`
	EnableMetrics        bool     `json:"enableMetrics"`
	EnableTraces         bool     `json:"enableTraces"`
	EnableLogs           bool     `json:"enableLogs"`
	GlobalTags           []string `json:"globalTags"`
	MetricsInterval      int      `json:"metricsInterval"`
	TrackSessionMetrics  bool     `json:"trackSessionMetrics"`
	TrackResourceMetrics bool     `json:"trackResourceMetrics"`
	TrackUserMetrics     bool     `json:"trackUserMetrics"`
}

// DatadogMetric represents a Datadog metric
type DatadogMetric struct {
	Metric string   `json:"metric"`
	Points [][]int64 `json:"points"`
	Type   string   `json:"type"`
	Tags   []string `json:"tags,omitempty"`
}

// DatadogMetricSeries is the payload sent to Datadog
type DatadogMetricSeries struct {
	Series []DatadogMetric `json:"series"`
}

// DatadogEvent represents a Datadog event
type DatadogEvent struct {
	Title          string   `json:"title"`
	Text           string   `json:"text"`
	Priority       string   `json:"priority,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	AlertType      string   `json:"alert_type,omitempty"`
	AggregationKey string   `json:"aggregation_key,omitempty"`
}

// Initialize sets up the plugin
func (p *DatadogPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal Datadog config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("Datadog integration is disabled")
		return nil
	}

	if p.config.APIKey == "" {
		return fmt.Errorf("Datadog API key is required")
	}

	if p.config.Site == "" {
		p.config.Site = "datadoghq.com"
	}

	// Initialize HTTP client
	p.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize session tracking
	p.sessionStart = make(map[string]time.Time)
	p.metricsBuffer = []DatadogMetric{}

	ctx.Logger.Info("Datadog plugin initialized successfully",
		"site", p.config.Site,
		"metrics_enabled", p.config.EnableMetrics,
		"traces_enabled", p.config.EnableTraces,
		"logs_enabled", p.config.EnableLogs,
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *DatadogPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Datadog monitoring plugin loaded")
	return p.sendEvent(ctx, "StreamSpace Datadog Plugin Loaded", "Datadog monitoring integration is now active", "info", "normal")
}

// OnUnload is called when the plugin is unloaded
func (p *DatadogPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Datadog monitoring plugin unloading")

	// Flush any remaining metrics
	if err := p.flushMetrics(ctx); err != nil {
		ctx.Logger.Error("Failed to flush metrics on unload", "error", err)
	}

	return p.sendEvent(ctx, "StreamSpace Datadog Plugin Unloaded", "Datadog monitoring integration has been disabled", "info", "normal")
}

// OnSessionCreated tracks session creation
func (p *DatadogPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
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

	// Send session created metric
	tags := append(p.config.GlobalTags,
		fmt.Sprintf("user:%s", userID),
		fmt.Sprintf("template:%s", templateName),
	)

	p.addMetric("streamspace.session.created", 1, "count", tags)
	p.addMetric("streamspace.session.active", 1, "gauge", tags)

	return p.sendEvent(ctx,
		"Session Created",
		fmt.Sprintf("User %s started session %s with template %s", userID, sessionID, templateName),
		"info",
		"low",
	)
}

// OnSessionTerminated tracks session termination
func (p *DatadogPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
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
	if exists {
		duration := time.Since(startTime).Seconds()
		tags := append(p.config.GlobalTags,
			fmt.Sprintf("user:%s", userID),
			fmt.Sprintf("template:%s", templateName),
		)
		p.addMetric("streamspace.session.duration", int64(duration), "gauge", tags)
		delete(p.sessionStart, sessionID)
	}
	p.sessionMutex.Unlock()

	// Send session terminated metric
	tags := append(p.config.GlobalTags,
		fmt.Sprintf("user:%s", userID),
		fmt.Sprintf("template:%s", templateName),
	)

	p.addMetric("streamspace.session.terminated", 1, "count", tags)
	p.addMetric("streamspace.session.active", -1, "gauge", tags)

	return nil
}

// OnSessionHeartbeat tracks session resource usage
func (p *DatadogPlugin) OnSessionHeartbeat(ctx *plugins.PluginContext, session interface{}) error {
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

	tags := append(p.config.GlobalTags,
		fmt.Sprintf("session:%s", sessionID),
		fmt.Sprintf("user:%s", userID),
		fmt.Sprintf("template:%s", templateName),
	)

	// Track resource usage if available
	if cpuUsage, ok := sessionMap["cpu_usage"].(float64); ok {
		p.addMetric("streamspace.session.cpu_usage", int64(cpuUsage*100), "gauge", tags)
	}

	if memoryUsage, ok := sessionMap["memory_usage"].(float64); ok {
		p.addMetric("streamspace.session.memory_usage", int64(memoryUsage), "gauge", tags)
	}

	if storageUsage, ok := sessionMap["storage_usage"].(float64); ok {
		p.addMetric("streamspace.session.storage_usage", int64(storageUsage), "gauge", tags)
	}

	return nil
}

// OnUserCreated tracks user creation
func (p *DatadogPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user format")
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	tags := append(p.config.GlobalTags, fmt.Sprintf("user:%s", userID))

	p.addMetric("streamspace.user.created", 1, "count", tags)
	p.addMetric("streamspace.users.total", 1, "count", p.config.GlobalTags)

	return nil
}

// OnUserLogin tracks user login
func (p *DatadogPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	tags := append(p.config.GlobalTags, fmt.Sprintf("user:%s", userID))

	p.addMetric("streamspace.user.login", 1, "count", tags)

	return nil
}

// OnUserLogout tracks user logout
func (p *DatadogPlugin) OnUserLogout(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled || !p.config.TrackUserMetrics {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return nil
	}

	userID := fmt.Sprintf("%v", userMap["id"])
	tags := append(p.config.GlobalTags, fmt.Sprintf("user:%s", userID))

	p.addMetric("streamspace.user.logout", 1, "count", tags)

	return nil
}

// RunScheduledJob handles the scheduled metrics flush
func (p *DatadogPlugin) RunScheduledJob(ctx *plugins.PluginContext, jobName string) error {
	if jobName == "send-metrics" {
		return p.flushMetrics(ctx)
	}
	return nil
}

// addMetric adds a metric to the buffer
func (p *DatadogPlugin) addMetric(name string, value int64, metricType string, tags []string) {
	p.metricsMutex.Lock()
	defer p.metricsMutex.Unlock()

	now := time.Now().Unix()
	metric := DatadogMetric{
		Metric: name,
		Points: [][]int64{{now, value}},
		Type:   metricType,
		Tags:   tags,
	}

	p.metricsBuffer = append(p.metricsBuffer, metric)
}

// flushMetrics sends buffered metrics to Datadog
func (p *DatadogPlugin) flushMetrics(ctx *plugins.PluginContext) error {
	if !p.config.Enabled || !p.config.EnableMetrics {
		return nil
	}

	p.metricsMutex.Lock()
	if len(p.metricsBuffer) == 0 {
		p.metricsMutex.Unlock()
		return nil
	}

	// Get metrics and clear buffer
	metrics := make([]DatadogMetric, len(p.metricsBuffer))
	copy(metrics, p.metricsBuffer)
	p.metricsBuffer = []DatadogMetric{}
	p.metricsMutex.Unlock()

	// Send to Datadog
	payload := DatadogMetricSeries{Series: metrics}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	url := fmt.Sprintf("https://api.%s/api/v1/series", p.config.Site)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Datadog API returned status %d: %s", resp.StatusCode, string(body))
	}

	ctx.Logger.Info("Sent metrics to Datadog", "count", len(metrics))
	return nil
}

// sendEvent sends an event to Datadog
func (p *DatadogPlugin) sendEvent(ctx *plugins.PluginContext, title, text, alertType, priority string) error {
	if !p.config.Enabled {
		return nil
	}

	event := DatadogEvent{
		Title:     title,
		Text:      text,
		Priority:  priority,
		Tags:      p.config.GlobalTags,
		AlertType: alertType,
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("https://api.%s/api/v1/events", p.config.Site)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		ctx.Logger.Warn("Failed to send event to Datadog", "status", resp.StatusCode, "body", string(body))
	}

	return nil
}

// Export the plugin
func init() {
	plugins.Register("streamspace-datadog", &DatadogPlugin{})
}
