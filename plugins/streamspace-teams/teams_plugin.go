package teamsplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// TeamsPlugin implements Microsoft Teams notification integration
type TeamsPlugin struct {
	plugins.BasePlugin

	// Rate limiting
	messageCount int
	lastReset    time.Time
}

// MessageCard represents a Teams message card
type MessageCard struct {
	Type       string              `json:"@type"`
	Context    string              `json:"@context"`
	ThemeColor string              `json:"themeColor,omitempty"`
	Title      string              `json:"title,omitempty"`
	Summary    string              `json:"summary,omitempty"`
	Text       string              `json:"text,omitempty"`
	Sections   []MessageCardSection `json:"sections,omitempty"`
}

// MessageCardSection represents a section in a message card
type MessageCardSection struct {
	ActivityTitle    string                `json:"activityTitle,omitempty"`
	ActivitySubtitle string                `json:"activitySubtitle,omitempty"`
	ActivityText     string                `json:"activityText,omitempty"`
	ActivityImage    string                `json:"activityImage,omitempty"`
	Facts            []MessageCardFact     `json:"facts,omitempty"`
	Text             string                `json:"text,omitempty"`
}

// MessageCardFact represents a fact in a message card
type MessageCardFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewTeamsPlugin creates a new Teams plugin instance
func NewTeamsPlugin() *TeamsPlugin {
	return &TeamsPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-teams"},
		lastReset:  time.Now(),
	}
}

// OnLoad is called when the plugin is loaded
func (p *TeamsPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Teams plugin loading", map[string]interface{}{
		"version": "1.0.0",
		"config":  ctx.Config,
	})

	// Validate configuration
	webhookURL, ok := ctx.Config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("teams webhook URL is required")
	}

	// Test webhook connectivity
	if err := p.testWebhook(ctx, webhookURL); err != nil {
		ctx.Logger.Warn("Failed to test Teams webhook", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail on test error
	}

	ctx.Logger.Info("Teams plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *TeamsPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Teams plugin unloading")
	return nil
}

// OnSessionCreated is called when a session is created
func (p *TeamsPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	notify, _ := ctx.Config["notifyOnSessionCreated"].(bool)
	if !notify {
		return nil
	}

	if !p.checkRateLimit(ctx) {
		ctx.Logger.Warn("Rate limit exceeded, skipping notification")
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	user := p.getString(sessionMap, "user")
	template := p.getString(sessionMap, "template")
	sessionID := p.getString(sessionMap, "id")

	// Build Teams message card
	card := MessageCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: "28a745", // Green
		Title:      "🚀 New Session Created",
		Summary:    "New session created in StreamSpace",
		Sections: []MessageCardSection{
			{
				Facts: []MessageCardFact{
					{Name: "User", Value: user},
					{Name: "Template", Value: template},
					{Name: "Session ID", Value: sessionID},
				},
			},
		},
	}

	// Include additional details if configured
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			memory := p.getString(resources, "memory")
			cpu := p.getString(resources, "cpu")

			card.Sections[0].Facts = append(card.Sections[0].Facts,
				MessageCardFact{Name: "Memory", Value: memory},
				MessageCardFact{Name: "CPU", Value: cpu},
			)
		}
	}

	return p.sendMessage(ctx, card)
}

// OnSessionHibernated is called when a session is hibernated
func (p *TeamsPlugin) OnSessionHibernated(ctx *plugins.PluginContext, session interface{}) error {
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

	card := MessageCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: "ffc107", // Yellow/Warning
		Title:      "💤 Session Hibernated",
		Summary:    "Session hibernated due to inactivity",
		Sections: []MessageCardSection{
			{
				ActivityTitle: "Session Hibernated",
				ActivityText:  "The session has been hibernated due to inactivity",
				Facts: []MessageCardFact{
					{Name: "User", Value: user},
					{Name: "Session ID", Value: sessionID},
				},
			},
		},
	}

	return p.sendMessage(ctx, card)
}

// OnUserCreated is called when a user is created
func (p *TeamsPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
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

	card := MessageCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: "0078d4", // Teams blue
		Title:      "👤 New User Created",
		Summary:    "New user created in StreamSpace",
		Sections: []MessageCardSection{
			{
				ActivityTitle: "User Created",
				Facts: []MessageCardFact{
					{Name: "Username", Value: username},
					{Name: "Full Name", Value: fullName},
					{Name: "Email", Value: email},
					{Name: "Tier", Value: tier},
				},
			},
		},
	}

	return p.sendMessage(ctx, card)
}

// sendMessage sends a message card to Teams
func (p *TeamsPlugin) sendMessage(ctx *plugins.PluginContext, card MessageCard) error {
	webhookURL := p.getString(ctx.Config, "webhookUrl")
	if webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Marshal message to JSON
	payload, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("failed to marshal Teams message: %w", err)
	}

	// Send HTTP POST to Teams webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send Teams message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("teams webhook returned status: %d", resp.StatusCode)
	}

	ctx.Logger.Debug("Teams notification sent successfully")

	return nil
}

// testWebhook tests the Teams webhook connection
func (p *TeamsPlugin) testWebhook(ctx *plugins.PluginContext, webhookURL string) error {
	card := MessageCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: "28a745",
		Title:      "🎉 StreamSpace Teams Plugin Activated",
		Summary:    "Teams integration activated",
		Text:       "Your Microsoft Teams integration is now configured and ready to send notifications.",
	}

	payload, err := json.Marshal(card)
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
func (p *TeamsPlugin) checkRateLimit(ctx *plugins.PluginContext) bool {
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
func (p *TeamsPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *TeamsPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-teams", func() plugins.PluginHandler {
		return NewTeamsPlugin()
	})
}
