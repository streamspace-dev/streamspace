package discordplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// DiscordPlugin implements Discord notification integration
type DiscordPlugin struct {
	plugins.BasePlugin

	// Rate limiting
	messageCount int
	lastReset    time.Time
}

// DiscordMessage represents a Discord webhook message
type DiscordMessage struct {
	Username  string        `json:"username,omitempty"`
	AvatarURL string        `json:"avatar_url,omitempty"`
	Content   string        `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents the footer of a Discord embed
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// Color constants (decimal values for Discord)
const (
	ColorGreen  = 3066993  // #2ECC71 - Success/good news
	ColorYellow = 16776960 // #FFFF00 - Warning
	ColorBlue   = 3447003  // #3498DB - Info
	ColorRed    = 15158332 // #E74C3C - Error/danger
)

// NewDiscordPlugin creates a new Discord plugin instance
func NewDiscordPlugin() *DiscordPlugin {
	return &DiscordPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-discord"},
		lastReset:  time.Now(),
	}
}

// OnLoad is called when the plugin is loaded
func (p *DiscordPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Discord plugin loading", map[string]interface{}{
		"version": "1.0.0",
		"config":  ctx.Config,
	})

	// Validate configuration
	webhookURL, ok := ctx.Config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("discord webhook URL is required")
	}

	// Test webhook connectivity
	if err := p.testWebhook(ctx, webhookURL); err != nil {
		ctx.Logger.Warn("Failed to test Discord webhook", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail on test error
	}

	ctx.Logger.Info("Discord plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *DiscordPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Discord plugin unloading")
	return nil
}

// OnSessionCreated is called when a session is created
func (p *DiscordPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
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

	// Build Discord embed
	fields := []DiscordEmbedField{
		{Name: "User", Value: user, Inline: true},
		{Name: "Template", Value: template, Inline: true},
		{Name: "Session ID", Value: sessionID, Inline: false},
	}

	// Include additional details if configured
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			memory := p.getString(resources, "memory")
			cpu := p.getString(resources, "cpu")

			if memory != "" {
				fields = append(fields, DiscordEmbedField{Name: "Memory", Value: memory, Inline: true})
			}
			if cpu != "" {
				fields = append(fields, DiscordEmbedField{Name: "CPU", Value: cpu, Inline: true})
			}
		}
	}

	embed := DiscordEmbed{
		Title:       "🚀 New Session Created",
		Description: "A new session has been created in StreamSpace",
		Color:       ColorGreen,
		Fields:      fields,
		Footer: &DiscordEmbedFooter{
			Text: "StreamSpace",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  p.getString(ctx.Config, "username"),
		AvatarURL: p.getString(ctx.Config, "avatarUrl"),
		Embeds:    []DiscordEmbed{embed},
	}

	return p.sendMessage(ctx, message)
}

// OnSessionHibernated is called when a session is hibernated
func (p *DiscordPlugin) OnSessionHibernated(ctx *plugins.PluginContext, session interface{}) error {
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

	embed := DiscordEmbed{
		Title:       "💤 Session Hibernated",
		Description: "A session has been hibernated due to inactivity",
		Color:       ColorYellow,
		Fields: []DiscordEmbedField{
			{Name: "User", Value: user, Inline: true},
			{Name: "Session ID", Value: sessionID, Inline: false},
		},
		Footer: &DiscordEmbedFooter{
			Text: "StreamSpace",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  p.getString(ctx.Config, "username"),
		AvatarURL: p.getString(ctx.Config, "avatarUrl"),
		Embeds:    []DiscordEmbed{embed},
	}

	return p.sendMessage(ctx, message)
}

// OnUserCreated is called when a user is created
func (p *DiscordPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
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

	embed := DiscordEmbed{
		Title:       "👤 New User Created",
		Description: "A new user has been created in StreamSpace",
		Color:       ColorBlue,
		Fields: []DiscordEmbedField{
			{Name: "Username", Value: username, Inline: true},
			{Name: "Full Name", Value: fullName, Inline: true},
			{Name: "Email", Value: email, Inline: false},
			{Name: "Tier", Value: tier, Inline: true},
		},
		Footer: &DiscordEmbedFooter{
			Text: "StreamSpace",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  p.getString(ctx.Config, "username"),
		AvatarURL: p.getString(ctx.Config, "avatarUrl"),
		Embeds:    []DiscordEmbed{embed},
	}

	return p.sendMessage(ctx, message)
}

// sendMessage sends a message to Discord
func (p *DiscordPlugin) sendMessage(ctx *plugins.PluginContext, message DiscordMessage) error {
	webhookURL := p.getString(ctx.Config, "webhookUrl")
	if webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Marshal message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord message: %w", err)
	}

	// Send HTTP POST to Discord webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send Discord message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord webhook returned status: %d", resp.StatusCode)
	}

	ctx.Logger.Debug("Discord notification sent successfully")

	return nil
}

// testWebhook tests the Discord webhook connection
func (p *DiscordPlugin) testWebhook(ctx *plugins.PluginContext, webhookURL string) error {
	embed := DiscordEmbed{
		Title:       "🎉 StreamSpace Discord Plugin Activated",
		Description: "Your Discord integration is now configured and ready to send notifications.",
		Color:       ColorGreen,
		Footer: &DiscordEmbedFooter{
			Text: "StreamSpace",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  p.getString(ctx.Config, "username"),
		AvatarURL: p.getString(ctx.Config, "avatarUrl"),
		Embeds:    []DiscordEmbed{embed},
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("webhook test failed with status: %d", resp.StatusCode)
	}

	return nil
}

// checkRateLimit checks if we're within the rate limit
func (p *DiscordPlugin) checkRateLimit(ctx *plugins.PluginContext) bool {
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
func (p *DiscordPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *DiscordPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-discord", func() plugins.PluginHandler {
		return NewDiscordPlugin()
	})
}
