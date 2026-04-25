package slackplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// SlackPlugin implements Slack notification integration
type SlackPlugin struct {
	plugins.BasePlugin

	// Rate limiting
	messageCount int
	lastReset    time.Time
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Text        string       `json:"text,omitempty"`
	Channel     string       `json:"channel,omitempty"`
	Username    string       `json:"username,omitempty"`
	IconEmoji   string       `json:"icon_emoji,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a Slack message attachment
type Attachment struct {
	Color      string   `json:"color,omitempty"`
	Title      string   `json:"title,omitempty"`
	Text       string   `json:"text,omitempty"`
	Fields     []Field  `json:"fields,omitempty"`
	Footer     string   `json:"footer,omitempty"`
	FooterIcon string   `json:"footer_icon,omitempty"`
	Timestamp  int64    `json:"ts,omitempty"`
}

// Field represents a field in a Slack attachment
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackPlugin creates a new Slack plugin instance
func NewSlackPlugin() *SlackPlugin {
	return &SlackPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-slack"},
		lastReset:  time.Now(),
	}
}

// OnLoad is called when the plugin is loaded
func (p *SlackPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Slack plugin loading", map[string]interface{}{
		"version": "1.0.0",
		"config":  ctx.Config,
	})

	// Validate configuration
	webhookURL, ok := ctx.Config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("slack webhook URL is required")
	}

	// Test webhook connectivity
	if err := p.testWebhook(ctx, webhookURL); err != nil {
		ctx.Logger.Warn("Failed to test Slack webhook", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail on test error, webhook might have restrictions
	}

	// Log successful load
	ctx.Logger.Info("Slack plugin loaded successfully")

	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *SlackPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Slack plugin unloading")
	return nil
}

// OnSessionCreated is called when a session is created
func (p *SlackPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	// Check if enabled in config
	notify, _ := ctx.Config["notifyOnSessionCreated"].(bool)
	if !notify {
		return nil
	}

	// Check rate limit
	if !p.checkRateLimit(ctx) {
		ctx.Logger.Warn("Rate limit exceeded, skipping notification")
		return nil
	}

	// Extract session data
	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	user := p.getString(sessionMap, "user")
	template := p.getString(sessionMap, "template")
	sessionID := p.getString(sessionMap, "id")

	// Build Slack message
	message := SlackMessage{
		Channel:   p.getString(ctx.Config, "channel"),
		Username:  p.getString(ctx.Config, "username"),
		IconEmoji: p.getString(ctx.Config, "iconEmoji"),
		Text:      "🚀 New Session Created",
		Attachments: []Attachment{
			{
				Color: "good",
				Title: "Session Details",
				Fields: []Field{
					{Title: "User", Value: user, Short: true},
					{Title: "Template", Value: template, Short: true},
					{Title: "Session ID", Value: sessionID, Short: false},
				},
				Footer:    "StreamSpace",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	// Include additional details if configured
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			memory := p.getString(resources, "memory")
			cpu := p.getString(resources, "cpu")

			message.Attachments[0].Fields = append(message.Attachments[0].Fields,
				Field{Title: "Memory", Value: memory, Short: true},
				Field{Title: "CPU", Value: cpu, Short: true},
			)
		}
	}

	// Send to Slack
	return p.sendMessage(ctx, message)
}

// OnSessionHibernated is called when a session is hibernated
func (p *SlackPlugin) OnSessionHibernated(ctx *plugins.PluginContext, session interface{}) error {
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

	message := SlackMessage{
		Channel:   p.getString(ctx.Config, "channel"),
		Username:  p.getString(ctx.Config, "username"),
		IconEmoji: p.getString(ctx.Config, "iconEmoji"),
		Text:      "💤 Session Hibernated",
		Attachments: []Attachment{
			{
				Color: "warning",
				Title: "Session Hibernated Due to Inactivity",
				Fields: []Field{
					{Title: "User", Value: user, Short: true},
					{Title: "Session ID", Value: sessionID, Short: false},
				},
				Footer:    "StreamSpace",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return p.sendMessage(ctx, message)
}

// OnUserCreated is called when a user is created
func (p *SlackPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
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

	message := SlackMessage{
		Channel:   p.getString(ctx.Config, "channel"),
		Username:  p.getString(ctx.Config, "username"),
		IconEmoji: p.getString(ctx.Config, "iconEmoji"),
		Text:      "👤 New User Created",
		Attachments: []Attachment{
			{
				Color: "#36a64f",
				Title: "User Details",
				Fields: []Field{
					{Title: "Username", Value: username, Short: true},
					{Title: "Full Name", Value: fullName, Short: true},
					{Title: "Email", Value: email, Short: false},
					{Title: "Tier", Value: tier, Short: true},
				},
				Footer:    "StreamSpace",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return p.sendMessage(ctx, message)
}

// sendMessage sends a message to Slack
func (p *SlackPlugin) sendMessage(ctx *plugins.PluginContext, message SlackMessage) error {
	webhookURL := p.getString(ctx.Config, "webhookUrl")
	if webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Marshal message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	// Send HTTP POST to Slack webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status: %d", resp.StatusCode)
	}

	ctx.Logger.Debug("Slack notification sent successfully", map[string]interface{}{
		"channel": message.Channel,
	})

	return nil
}

// testWebhook tests the Slack webhook connection
func (p *SlackPlugin) testWebhook(ctx *plugins.PluginContext, webhookURL string) error {
	message := SlackMessage{
		Text: "🎉 StreamSpace Slack plugin activated!",
		Attachments: []Attachment{
			{
				Color: "good",
				Text:  "Your Slack integration is now configured and ready to send notifications.",
			},
		},
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook test failed with status: %d", resp.StatusCode)
	}

	return nil
}

// checkRateLimit checks if we're within the rate limit
func (p *SlackPlugin) checkRateLimit(ctx *plugins.PluginContext) bool {
	maxMessages, _ := ctx.Config["rateLimit"].(float64)
	if maxMessages == 0 {
		maxMessages = 20 // Default
	}

	now := time.Now()
	if now.Sub(p.lastReset) > time.Hour {
		p.messageCount = 0
		p.lastReset = now
	}

	if p.messageCount >= int(maxMessages) {
		return false
	}

	p.messageCount++
	return true
}

// Helper functions to safely extract values from maps
func (p *SlackPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *SlackPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-slack", func() plugins.PluginHandler {
		return NewSlackPlugin()
	})
}
