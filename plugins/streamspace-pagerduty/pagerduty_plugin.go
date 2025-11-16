package pagerdutyplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/streamspace/streamspace/api/internal/plugins"
)

// PagerDutyPlugin implements PagerDuty incident alerting integration
type PagerDutyPlugin struct {
	plugins.BasePlugin

	// Rate limiting
	eventCount int
	lastReset  time.Time
}

// PagerDutyEvent represents a PagerDuty Events API v2 event
type PagerDutyEvent struct {
	RoutingKey  string              `json:"routing_key"`
	EventAction string              `json:"event_action"` // trigger, acknowledge, resolve
	DedupKey    string              `json:"dedup_key,omitempty"`
	Payload     PagerDutyPayload    `json:"payload"`
	Links       []PagerDutyLink     `json:"links,omitempty"`
	Images      []PagerDutyImage    `json:"images,omitempty"`
}

// PagerDutyPayload represents the event payload
type PagerDutyPayload struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"` // info, warning, error, critical
	Timestamp     string                 `json:"timestamp,omitempty"`
	Component     string                 `json:"component,omitempty"`
	Group         string                 `json:"group,omitempty"`
	Class         string                 `json:"class,omitempty"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
}

// PagerDutyLink represents a link in the event
type PagerDutyLink struct {
	Href string `json:"href"`
	Text string `json:"text"`
}

// PagerDutyImage represents an image in the event
type PagerDutyImage struct {
	Src  string `json:"src"`
	Href string `json:"href,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

// PagerDuty Events API endpoint
const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// NewPagerDutyPlugin creates a new PagerDuty plugin instance
func NewPagerDutyPlugin() *PagerDutyPlugin {
	return &PagerDutyPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-pagerduty"},
		lastReset:  time.Now(),
	}
}

// OnLoad is called when the plugin is loaded
func (p *PagerDutyPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("PagerDuty plugin loading", map[string]interface{}{
		"version": "1.0.0",
		"config":  ctx.Config,
	})

	// Validate configuration
	routingKey, ok := ctx.Config["routingKey"].(string)
	if !ok || routingKey == "" {
		return fmt.Errorf("pagerduty routing key is required")
	}

	// Test integration connectivity
	if err := p.testIntegration(ctx, routingKey); err != nil {
		ctx.Logger.Warn("Failed to test PagerDuty integration", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail on test error - PagerDuty might rate limit or have issues
	}

	ctx.Logger.Info("PagerDuty plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *PagerDutyPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("PagerDuty plugin unloading")
	return nil
}

// OnSessionCreated is called when a session is created
func (p *PagerDutyPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	notify, _ := ctx.Config["notifyOnSessionCreated"].(bool)
	if !notify {
		return nil
	}

	if !p.checkRateLimit(ctx) {
		ctx.Logger.Warn("Rate limit exceeded, skipping PagerDuty event")
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	user := p.getString(sessionMap, "user")
	template := p.getString(sessionMap, "template")
	sessionID := p.getString(sessionMap, "id")

	// Build custom details
	customDetails := map[string]interface{}{
		"user":      user,
		"template":  template,
		"sessionId": sessionID,
		"eventType": "session.created",
	}

	// Include resource details if configured
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			customDetails["memory"] = p.getString(resources, "memory")
			customDetails["cpu"] = p.getString(resources, "cpu")
		}
	}

	severity := p.getString(ctx.Config, "sessionCreatedSeverity")
	if severity == "" {
		severity = "info"
	}

	event := PagerDutyEvent{
		RoutingKey:  p.getString(ctx.Config, "routingKey"),
		EventAction: "trigger",
		DedupKey:    fmt.Sprintf("streamspace-session-%s", sessionID),
		Payload: PagerDutyPayload{
			Summary:       fmt.Sprintf("StreamSpace Session Created: %s by %s", template, user),
			Source:        "streamspace",
			Severity:      severity,
			Timestamp:     time.Now().Format(time.RFC3339),
			Component:     "sessions",
			Class:         "session.created",
			CustomDetails: customDetails,
		},
	}

	if err := p.sendEvent(ctx, event); err != nil {
		return err
	}

	// Auto-resolve if configured
	if p.getBool(ctx.Config, "autoResolve") {
		return p.resolveEvent(ctx, event.DedupKey)
	}

	return nil
}

// OnSessionHibernated is called when a session is hibernated
func (p *PagerDutyPlugin) OnSessionHibernated(ctx *plugins.PluginContext, session interface{}) error {
	notify, _ := ctx.Config["notifyOnSessionHibernated"].(bool)
	if !notify {
		return nil
	}

	if !p.checkRateLimit(ctx) {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	user := p.getString(sessionMap, "user")
	sessionID := p.getString(sessionMap, "id")

	customDetails := map[string]interface{}{
		"user":      user,
		"sessionId": sessionID,
		"eventType": "session.hibernated",
		"reason":    "inactivity",
	}

	severity := p.getString(ctx.Config, "sessionHibernatedSeverity")
	if severity == "" {
		severity = "warning"
	}

	event := PagerDutyEvent{
		RoutingKey:  p.getString(ctx.Config, "routingKey"),
		EventAction: "trigger",
		DedupKey:    fmt.Sprintf("streamspace-session-hibernated-%s", sessionID),
		Payload: PagerDutyPayload{
			Summary:       fmt.Sprintf("StreamSpace Session Hibernated: %s (User: %s)", sessionID, user),
			Source:        "streamspace",
			Severity:      severity,
			Timestamp:     time.Now().Format(time.RFC3339),
			Component:     "sessions",
			Class:         "session.hibernated",
			CustomDetails: customDetails,
		},
	}

	if err := p.sendEvent(ctx, event); err != nil {
		return err
	}

	// Auto-resolve if configured
	if p.getBool(ctx.Config, "autoResolve") {
		return p.resolveEvent(ctx, event.DedupKey)
	}

	return nil
}

// OnUserCreated is called when a user is created
func (p *PagerDutyPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	notify, _ := ctx.Config["notifyOnUserCreated"].(bool)
	if !notify {
		return nil
	}

	if !p.checkRateLimit(ctx) {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data type")
	}

	username := p.getString(userMap, "username")
	fullName := p.getString(userMap, "fullName")
	email := p.getString(userMap, "email")
	tier := p.getString(userMap, "tier")

	customDetails := map[string]interface{}{
		"username":  username,
		"fullName":  fullName,
		"email":     email,
		"tier":      tier,
		"eventType": "user.created",
	}

	severity := p.getString(ctx.Config, "userCreatedSeverity")
	if severity == "" {
		severity = "info"
	}

	event := PagerDutyEvent{
		RoutingKey:  p.getString(ctx.Config, "routingKey"),
		EventAction: "trigger",
		DedupKey:    fmt.Sprintf("streamspace-user-%s", username),
		Payload: PagerDutyPayload{
			Summary:       fmt.Sprintf("StreamSpace User Created: %s (%s)", fullName, username),
			Source:        "streamspace",
			Severity:      severity,
			Timestamp:     time.Now().Format(time.RFC3339),
			Component:     "users",
			Class:         "user.created",
			CustomDetails: customDetails,
		},
	}

	if err := p.sendEvent(ctx, event); err != nil {
		return err
	}

	// Auto-resolve if configured
	if p.getBool(ctx.Config, "autoResolve") {
		return p.resolveEvent(ctx, event.DedupKey)
	}

	return nil
}

// sendEvent sends an event to PagerDuty
func (p *PagerDutyPlugin) sendEvent(ctx *plugins.PluginContext, event PagerDutyEvent) error {
	// Marshal event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty event: %w", err)
	}

	// Send HTTP POST to PagerDuty Events API
	resp, err := http.Post(pagerDutyEventsURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pagerduty API returned status: %d", resp.StatusCode)
	}

	ctx.Logger.Debug("PagerDuty event sent successfully", map[string]interface{}{
		"dedupKey": event.DedupKey,
		"severity": event.Payload.Severity,
	})

	return nil
}

// resolveEvent resolves an event in PagerDuty
func (p *PagerDutyPlugin) resolveEvent(ctx *plugins.PluginContext, dedupKey string) error {
	event := PagerDutyEvent{
		RoutingKey:  p.getString(ctx.Config, "routingKey"),
		EventAction: "resolve",
		DedupKey:    dedupKey,
		Payload: PagerDutyPayload{
			Summary:  "Event auto-resolved",
			Source:   "streamspace",
			Severity: "info",
		},
	}

	return p.sendEvent(ctx, event)
}

// testIntegration tests the PagerDuty integration
func (p *PagerDutyPlugin) testIntegration(ctx *plugins.PluginContext, routingKey string) error {
	event := PagerDutyEvent{
		RoutingKey:  routingKey,
		EventAction: "trigger",
		DedupKey:    fmt.Sprintf("streamspace-test-%d", time.Now().Unix()),
		Payload: PagerDutyPayload{
			Summary:   "StreamSpace PagerDuty Plugin Test",
			Source:    "streamspace",
			Severity:  "info",
			Timestamp: time.Now().Format(time.RFC3339),
			Component: "plugin-test",
			CustomDetails: map[string]interface{}{
				"message": "PagerDuty integration is configured and working",
			},
		},
	}

	if err := p.sendEvent(ctx, event); err != nil {
		return err
	}

	// Auto-resolve the test event
	time.Sleep(2 * time.Second) // Small delay before resolving
	return p.resolveEvent(ctx, event.DedupKey)
}

// checkRateLimit checks if we're within the rate limit
func (p *PagerDutyPlugin) checkRateLimit(ctx *plugins.PluginContext) bool {
	maxEvents, _ := ctx.Config["rateLimit"].(float64)
	if maxEvents == 0 {
		maxEvents = 50 // Default
	}

	now := time.Now()
	if now.Sub(p.lastReset) > time.Hour {
		p.eventCount = 0
		p.lastReset = now
	}

	if p.eventCount >= int(maxEvents) {
		return false
	}

	p.eventCount++
	return true
}

// Helper functions to safely extract values from maps
func (p *PagerDutyPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *PagerDutyPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-pagerduty", func() plugins.PluginHandler {
		return NewPagerDutyPlugin()
	})
}
