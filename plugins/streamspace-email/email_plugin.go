package emailplugin

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/streamspace/streamspace/api/internal/plugins"
)

// EmailPlugin implements SMTP email notification integration
type EmailPlugin struct {
	plugins.BasePlugin

	// Rate limiting
	emailCount int
	lastReset  time.Time
}

// NewEmailPlugin creates a new Email plugin instance
func NewEmailPlugin() *EmailPlugin {
	return &EmailPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-email"},
		lastReset:  time.Now(),
	}
}

// OnLoad is called when the plugin is loaded
func (p *EmailPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Email SMTP plugin loading", map[string]interface{}{
		"version": "1.0.0",
		"config":  ctx.Config,
	})

	// Validate configuration
	if err := p.validateConfig(ctx.Config); err != nil {
		return err
	}

	// Test SMTP connectivity
	if err := p.testSMTP(ctx); err != nil {
		ctx.Logger.Warn("Failed to test SMTP connection", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail on test error
	}

	ctx.Logger.Info("Email SMTP plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *EmailPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Email SMTP plugin unloading")
	return nil
}

// OnSessionCreated is called when a session is created
func (p *EmailPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	notify, _ := ctx.Config["notifyOnSessionCreated"].(bool)
	if !notify {
		return nil
	}

	if !p.checkRateLimit(ctx) {
		ctx.Logger.Warn("Rate limit exceeded, skipping email notification")
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	user := p.getString(sessionMap, "user")
	template := p.getString(sessionMap, "template")
	sessionID := p.getString(sessionMap, "id")

	subject := fmt.Sprintf("🚀 New Session Created: %s", template)

	var body string
	if p.getBool(ctx.Config, "htmlFormat") {
		body = p.buildHTMLSessionCreated(user, template, sessionID, sessionMap, ctx)
	} else {
		body = p.buildPlainSessionCreated(user, template, sessionID, sessionMap, ctx)
	}

	return p.sendEmail(ctx, subject, body)
}

// OnSessionHibernated is called when a session is hibernated
func (p *EmailPlugin) OnSessionHibernated(ctx *plugins.PluginContext, session interface{}) error {
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

	subject := fmt.Sprintf("💤 Session Hibernated: %s", sessionID)

	var body string
	if p.getBool(ctx.Config, "htmlFormat") {
		body = p.buildHTMLSessionHibernated(user, sessionID)
	} else {
		body = p.buildPlainSessionHibernated(user, sessionID)
	}

	return p.sendEmail(ctx, subject, body)
}

// OnUserCreated is called when a user is created
func (p *EmailPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
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

	subject := fmt.Sprintf("👤 New User Created: %s", username)

	var body string
	if p.getBool(ctx.Config, "htmlFormat") {
		body = p.buildHTMLUserCreated(username, fullName, email, tier)
	} else {
		body = p.buildPlainUserCreated(username, fullName, email, tier)
	}

	return p.sendEmail(ctx, subject, body)
}

// sendEmail sends an email via SMTP
func (p *EmailPlugin) sendEmail(ctx *plugins.PluginContext, subject, body string) error {
	host := p.getString(ctx.Config, "smtpHost")
	port := p.getInt(ctx.Config, "smtpPort")
	username := p.getString(ctx.Config, "username")
	password := p.getString(ctx.Config, "password")
	fromAddress := p.getString(ctx.Config, "fromAddress")
	fromName := p.getString(ctx.Config, "fromName")
	useTLS := p.getBool(ctx.Config, "useTLS")

	// Build recipient list
	to := p.getStringArray(ctx.Config, "toAddresses")
	cc := p.getStringArray(ctx.Config, "ccAddresses")

	if len(to) == 0 {
		return fmt.Errorf("no recipient addresses configured")
	}

	// Build email headers
	from := fmt.Sprintf("%s <%s>", fromName, fromAddress)
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	if len(cc) > 0 {
		headers["Cc"] = strings.Join(cc, ", ")
	}
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"

	if strings.Contains(body, "<html>") {
		headers["Content-Type"] = "text/html; charset=UTF-8"
	} else {
		headers["Content-Type"] = "text/plain; charset=UTF-8"
	}

	// Build message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// All recipients (to + cc)
	recipients := append(to, cc...)

	// Setup SMTP authentication
	auth := smtp.PlainAuth("", username, password, host)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", host, port)

	// Send email
	if useTLS && port == 587 {
		// Use STARTTLS
		return p.sendMailTLS(addr, auth, fromAddress, recipients, []byte(message))
	} else {
		// Use plain SMTP or implicit TLS
		return smtp.SendMail(addr, auth, fromAddress, recipients, []byte(message))
	}
}

// sendMailTLS sends email with STARTTLS
func (p *EmailPlugin) sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Connect to server
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	// Start TLS
	tlsConfig := &tls.Config{
		ServerName: strings.Split(addr, ":")[0],
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return err
	}

	// Authenticate
	if err = client.Auth(auth); err != nil {
		return err
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return err
	}

	// Set recipients
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// testSMTP tests the SMTP connection
func (p *EmailPlugin) testSMTP(ctx *plugins.PluginContext) error {
	subject := "StreamSpace Email Plugin Test"
	body := p.buildHTMLTest()
	return p.sendEmail(ctx, subject, body)
}

// validateConfig validates the plugin configuration
func (p *EmailPlugin) validateConfig(config map[string]interface{}) error {
	required := []string{"smtpHost", "smtpPort", "username", "password", "fromAddress", "toAddresses"}

	for _, field := range required {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}

	return nil
}

// checkRateLimit checks if we're within the rate limit
func (p *EmailPlugin) checkRateLimit(ctx *plugins.PluginContext) bool {
	maxEmails, _ := ctx.Config["rateLimit"].(float64)
	if maxEmails == 0 {
		maxEmails = 30 // Default
	}

	now := time.Now()
	if now.Sub(p.lastReset) > time.Hour {
		p.emailCount = 0
		p.lastReset = now
	}

	if p.emailCount >= int(maxEmails) {
		return false
	}

	p.emailCount++
	return true
}

// HTML email templates

func (p *EmailPlugin) buildHTMLSessionCreated(user, template, sessionID string, sessionMap map[string]interface{}, ctx *plugins.PluginContext) string {
	details := ""
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			memory := p.getString(resources, "memory")
			cpu := p.getString(resources, "cpu")
			details = fmt.Sprintf(`
				<tr><td><strong>Memory:</strong></td><td>%s</td></tr>
				<tr><td><strong>CPU:</strong></td><td>%s</td></tr>
			`, memory, cpu)
		}
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #28a745; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
		.content { background-color: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
		table { width: 100%%; border-collapse: collapse; margin: 15px 0; }
		td { padding: 8px; border-bottom: 1px solid #dee2e6; }
		.footer { margin-top: 20px; font-size: 12px; color: #6c757d; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h2>🚀 New Session Created</h2>
		</div>
		<div class="content">
			<p>A new session has been created in StreamSpace.</p>
			<table>
				<tr><td><strong>User:</strong></td><td>%s</td></tr>
				<tr><td><strong>Template:</strong></td><td>%s</td></tr>
				<tr><td><strong>Session ID:</strong></td><td>%s</td></tr>
				%s
			</table>
			<p class="footer">StreamSpace Notifications • %s</p>
		</div>
	</div>
</body>
</html>
	`, user, template, sessionID, details, time.Now().Format("2006-01-02 15:04:05 MST"))
}

func (p *EmailPlugin) buildHTMLSessionHibernated(user, sessionID string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #ffc107; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
		.content { background-color: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
		table { width: 100%%; border-collapse: collapse; margin: 15px 0; }
		td { padding: 8px; border-bottom: 1px solid #dee2e6; }
		.footer { margin-top: 20px; font-size: 12px; color: #6c757d; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h2>💤 Session Hibernated</h2>
		</div>
		<div class="content">
			<p>A session has been hibernated due to inactivity.</p>
			<table>
				<tr><td><strong>User:</strong></td><td>%s</td></tr>
				<tr><td><strong>Session ID:</strong></td><td>%s</td></tr>
			</table>
			<p class="footer">StreamSpace Notifications • %s</p>
		</div>
	</div>
</body>
</html>
	`, user, sessionID, time.Now().Format("2006-01-02 15:04:05 MST"))
}

func (p *EmailPlugin) buildHTMLUserCreated(username, fullName, email, tier string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #0078d4; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
		.content { background-color: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
		table { width: 100%%; border-collapse: collapse; margin: 15px 0; }
		td { padding: 8px; border-bottom: 1px solid #dee2e6; }
		.footer { margin-top: 20px; font-size: 12px; color: #6c757d; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h2>👤 New User Created</h2>
		</div>
		<div class="content">
			<p>A new user has been created in StreamSpace.</p>
			<table>
				<tr><td><strong>Username:</strong></td><td>%s</td></tr>
				<tr><td><strong>Full Name:</strong></td><td>%s</td></tr>
				<tr><td><strong>Email:</strong></td><td>%s</td></tr>
				<tr><td><strong>Tier:</strong></td><td>%s</td></tr>
			</table>
			<p class="footer">StreamSpace Notifications • %s</p>
		</div>
	</div>
</body>
</html>
	`, username, fullName, email, tier, time.Now().Format("2006-01-02 15:04:05 MST"))
}

func (p *EmailPlugin) buildHTMLTest() string {
	return `
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #28a745; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
		.content { background-color: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h2>🎉 StreamSpace Email Plugin Activated</h2>
		</div>
		<div class="content">
			<p>Your SMTP email integration is now configured and ready to send notifications.</p>
			<p>This is a test email to verify that your SMTP settings are correct.</p>
		</div>
	</div>
</body>
</html>
	`
}

// Plain text email templates

func (p *EmailPlugin) buildPlainSessionCreated(user, template, sessionID string, sessionMap map[string]interface{}, ctx *plugins.PluginContext) string {
	details := ""
	if p.getBool(ctx.Config, "includeDetails") {
		if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
			memory := p.getString(resources, "memory")
			cpu := p.getString(resources, "cpu")
			details = fmt.Sprintf("\nMemory: %s\nCPU: %s", memory, cpu)
		}
	}

	return fmt.Sprintf(`New Session Created

A new session has been created in StreamSpace.

User: %s
Template: %s
Session ID: %s%s

---
StreamSpace Notifications
%s
	`, user, template, sessionID, details, time.Now().Format("2006-01-02 15:04:05 MST"))
}

func (p *EmailPlugin) buildPlainSessionHibernated(user, sessionID string) string {
	return fmt.Sprintf(`Session Hibernated

A session has been hibernated due to inactivity.

User: %s
Session ID: %s

---
StreamSpace Notifications
%s
	`, user, sessionID, time.Now().Format("2006-01-02 15:04:05 MST"))
}

func (p *EmailPlugin) buildPlainUserCreated(username, fullName, email, tier string) string {
	return fmt.Sprintf(`New User Created

A new user has been created in StreamSpace.

Username: %s
Full Name: %s
Email: %s
Tier: %s

---
StreamSpace Notifications
%s
	`, username, fullName, email, tier, time.Now().Format("2006-01-02 15:04:05 MST"))
}

// Helper functions

func (p *EmailPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *EmailPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func (p *EmailPlugin) getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if i, ok := val.(float64); ok {
			return int(i)
		}
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

func (p *EmailPlugin) getStringArray(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-email", func() plugins.PluginHandler {
		return NewEmailPlugin()
	})
}
