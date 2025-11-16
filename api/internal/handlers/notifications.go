// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements notification delivery and management across multiple channels.
//
// NOTIFICATION FEATURES:
// - In-app notification management (create, read, delete)
// - Email notifications via SMTP
// - Webhook notifications with HMAC signatures
// - Multiple notification types (session events, quotas, teams, alerts)
// - Priority levels (low, normal, high, urgent)
// - Read/unread tracking
// - Notification preferences per user
//
// NOTIFICATION TYPES:
// - session.created: New session launched
// - session.idle: Session idle timeout warning
// - session.shared: Session shared with user
// - quota.warning: Approaching quota limits (80% threshold)
// - quota.exceeded: Quota limits exceeded
// - team.invitation: Invited to join team
// - system.alert: System-wide alerts
//
// NOTIFICATION CHANNELS:
// - In-app: Stored in database, retrieved via API
// - Email: Sent via SMTP with HTML templates
// - Webhook: HTTP POST to user-configured URLs
// - Integration: Slack, Teams, Discord (via integrations handler)
//
// PRIORITY LEVELS:
// - low: Non-urgent informational messages
// - normal: Standard notifications
// - high: Important notifications requiring attention
// - urgent: Critical notifications requiring immediate action
//
// IN-APP NOTIFICATIONS:
// - Persistent storage in database
// - Unread count tracking
// - Mark as read individually or in bulk
// - Delete individually or clear all
// - Action URLs for quick navigation
// - Pagination support
//
// EMAIL NOTIFICATIONS:
// - HTML template rendering
// - SMTP configuration (host, port, auth)
// - Configurable from/reply-to addresses
// - Environment variable configuration
// - Test email endpoint for debugging
//
// WEBHOOK NOTIFICATIONS:
// - HMAC-SHA256 signature for verification
// - JSON payload with notification data
// - User-configured webhook URLs
// - Test webhook endpoint for debugging
//
// API Endpoints:
// - GET    /api/v1/notifications - List user notifications
// - GET    /api/v1/notifications/unread - Get unread notifications
// - GET    /api/v1/notifications/count - Get unread count
// - POST   /api/v1/notifications/:id/read - Mark as read
// - POST   /api/v1/notifications/read-all - Mark all as read
// - DELETE /api/v1/notifications/:id - Delete notification
// - DELETE /api/v1/notifications/clear-all - Clear all notifications
// - POST   /api/v1/notifications/send - Send notification (admin)
// - GET    /api/v1/notifications/preferences - Get notification preferences
// - PUT    /api/v1/notifications/preferences - Update notification preferences
// - POST   /api/v1/notifications/test/email - Test email delivery
// - POST   /api/v1/notifications/test/webhook - Test webhook delivery
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - SMTP client is created per-request
//
// Dependencies:
// - Database: notifications, user_preferences tables
// - External Services: SMTP server for email delivery
//
// Example Usage:
//
//	handler := NewNotificationsHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// NotificationsHandler handles notification delivery and management
type NotificationsHandler struct {
	db *db.Database
}

// NewNotificationsHandler creates a new notifications handler
func NewNotificationsHandler(database *db.Database) *NotificationsHandler {
	return &NotificationsHandler{
		db: database,
	}
}

// Notification types
const (
	NotificationTypeSessionCreated  = "session.created"
	NotificationTypeSessionIdle     = "session.idle"
	NotificationTypeSessionShared   = "session.shared"
	NotificationTypeQuotaWarning    = "quota.warning"
	NotificationTypeQuotaExceeded   = "quota.exceeded"
	NotificationTypeTeamInvitation  = "team.invitation"
	NotificationTypeSystemAlert     = "system.alert"
)

// Notification represents an in-app notification
type Notification struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"userId"`
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Priority   string                 `json:"priority"` // low, normal, high, urgent
	Read       bool                   `json:"read"`
	ActionURL  string                 `json:"actionUrl,omitempty"`
	ActionText string                 `json:"actionText,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	ReadAt     *time.Time             `json:"readAt,omitempty"`
}

// RegisterRoutes registers notification routes
func (h *NotificationsHandler) RegisterRoutes(router *gin.RouterGroup) {
	notifications := router.Group("/notifications")
	{
		// In-app notifications
		notifications.GET("", h.ListNotifications)
		notifications.GET("/unread", h.GetUnreadNotifications)
		notifications.GET("/count", h.GetUnreadCount)
		notifications.POST("/:id/read", h.MarkAsRead)
		notifications.POST("/read-all", h.MarkAllAsRead)
		notifications.DELETE("/:id", h.DeleteNotification)
		notifications.DELETE("/clear-all", h.ClearAllNotifications)

		// Send notification (for internal/admin use)
		notifications.POST("/send", h.SendNotification)

		// Notification preferences (integrated with user preferences)
		notifications.GET("/preferences", h.GetNotificationPreferences)
		notifications.PUT("/preferences", h.UpdateNotificationPreferences)

		// Test endpoints (for debugging)
		notifications.POST("/test/email", h.TestEmailNotification)
		notifications.POST("/test/webhook", h.TestWebhookNotification)
	}
}

// ListNotifications returns paginated user notifications
func (h *NotificationsHandler) ListNotifications(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	limit := 50
	offset := 0

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, user_id, type, title, message, data, priority, is_read, action_url, action_text, created_at, read_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userIDStr, limit, offset)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var n Notification
		var dataJSON []byte
		var actionURL, actionText sql.NullString
		var readAt sql.NullTime

		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &dataJSON, &n.Priority, &n.Read, &actionURL, &actionText, &n.CreatedAt, &readAt); err == nil {
			if len(dataJSON) > 0 {
				json.Unmarshal(dataJSON, &n.Data)
			}
			if actionURL.Valid {
				n.ActionURL = actionURL.String
			}
			if actionText.Valid {
				n.ActionText = actionText.String
			}
			if readAt.Valid {
				n.ReadAt = &readAt.Time
			}
			notifications = append(notifications, n)
		}
	}

	// Get total count
	var total int
	h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userIDStr).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetUnreadNotifications returns only unread notifications
func (h *NotificationsHandler) GetUnreadNotifications(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, user_id, type, title, message, data, priority, action_url, action_text, created_at
		FROM notifications
		WHERE user_id = $1 AND is_read = false
		ORDER BY created_at DESC
		LIMIT 50
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread notifications"})
		return
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var n Notification
		var dataJSON []byte
		var actionURL, actionText sql.NullString

		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &dataJSON, &n.Priority, &actionURL, &actionText, &n.CreatedAt); err == nil {
			n.Read = false
			if len(dataJSON) > 0 {
				json.Unmarshal(dataJSON, &n.Data)
			}
			if actionURL.Valid {
				n.ActionURL = actionURL.String
			}
			if actionText.Valid {
				n.ActionText = actionText.String
			}
			notifications = append(notifications, n)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"count":         len(notifications),
	})
}

// GetUnreadCount returns count of unread notifications
func (h *NotificationsHandler) GetUnreadCount(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var count int
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false
	`, userIDStr).Scan(&count)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": count,
	})
}

// MarkAsRead marks a notification as read
func (h *NotificationsHandler) MarkAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	notificationID := c.Param("id")

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE notifications
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2
	`, notificationID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notification marked as read",
		"id":      notificationID,
	})
}

// MarkAllAsRead marks all notifications as read
func (h *NotificationsHandler) MarkAllAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	result, err := h.db.DB().ExecContext(ctx, `
		UPDATE notifications
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND is_read = false
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}

	rowsAffected, _ := result.RowsAffected()

	c.JSON(http.StatusOK, gin.H{
		"message": "All notifications marked as read",
		"count":   rowsAffected,
	})
}

// DeleteNotification deletes a notification
func (h *NotificationsHandler) DeleteNotification(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	notificationID := c.Param("id")

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM notifications WHERE id = $1 AND user_id = $2
	`, notificationID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notification deleted",
		"id":      notificationID,
	})
}

// ClearAllNotifications deletes all read notifications
func (h *NotificationsHandler) ClearAllNotifications(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	result, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM notifications WHERE user_id = $1 AND is_read = true
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear notifications"})
		return
	}

	rowsAffected, _ := result.RowsAffected()

	c.JSON(http.StatusOK, gin.H{
		"message": "Read notifications cleared",
		"count":   rowsAffected,
	})
}

// SendNotification sends a notification via all enabled channels
func (h *NotificationsHandler) SendNotification(c *gin.Context) {
	var req struct {
		UserID     string                 `json:"userId" binding:"required"`
		Type       string                 `json:"type" binding:"required"`
		Title      string                 `json:"title" binding:"required"`
		Message    string                 `json:"message" binding:"required"`
		Data       map[string]interface{} `json:"data"`
		Priority   string                 `json:"priority"` // low, normal, high, urgent
		ActionURL  string                 `json:"actionUrl"`
		ActionText string                 `json:"actionText"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Default priority to normal
	if req.Priority == "" {
		req.Priority = "normal"
	}

	// Get user's notification preferences
	prefs, err := h.getUserNotificationPreferences(ctx, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user preferences"})
		return
	}

	// Send in-app notification (always enabled)
	notificationID, err := h.createInAppNotification(ctx, req.UserID, req.Type, req.Title, req.Message, req.Data, req.Priority, req.ActionURL, req.ActionText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create in-app notification"})
		return
	}

	// Send email notification if enabled for this event type
	if h.shouldSendEmail(prefs, req.Type) {
		go h.sendEmailNotification(req.UserID, req.Type, req.Title, req.Message, req.ActionURL)
	}

	// Send webhook notification if enabled
	if h.shouldSendWebhook(prefs, req.Type) {
		go h.sendWebhookNotification(prefs, req.UserID, req.Type, req.Title, req.Message, req.Data)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Notification sent",
		"notificationId": notificationID,
	})
}

// createInAppNotification creates an in-app notification in the database
func (h *NotificationsHandler) createInAppNotification(ctx context.Context, userID, notifType, title, message string, data map[string]interface{}, priority, actionURL, actionText string) (string, error) {
	notificationID := fmt.Sprintf("notif_%d", time.Now().UnixNano())

	dataJSON, _ := json.Marshal(data)

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO notifications (id, user_id, type, title, message, data, priority, action_url, action_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, notificationID, userID, notifType, title, message, dataJSON, priority, actionURL, actionText)

	return notificationID, err
}

// getUserNotificationPreferences gets user's notification preferences
func (h *NotificationsHandler) getUserNotificationPreferences(ctx context.Context, userID string) (map[string]interface{}, error) {
	var prefsJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT preferences->'notifications' FROM user_preferences WHERE user_id = $1
	`, userID).Scan(&prefsJSON)

	if err == sql.ErrNoRows {
		// Return default preferences
		return h.getDefaultNotificationPreferences(), nil
	}

	if err != nil {
		return nil, err
	}

	var prefs map[string]interface{}
	json.Unmarshal(prefsJSON, &prefs)
	return prefs, nil
}

// shouldSendEmail determines if email should be sent for this event type
func (h *NotificationsHandler) shouldSendEmail(prefs map[string]interface{}, eventType string) bool {
	emailPrefs, ok := prefs["email"].(map[string]interface{})
	if !ok {
		return false
	}

	// Map event types to preference keys
	prefKey := eventType
	if val, exists := emailPrefs[prefKey]; exists {
		if enabled, ok := val.(bool); ok {
			return enabled
		}
	}

	return false
}

// shouldSendWebhook determines if webhook should be sent
func (h *NotificationsHandler) shouldSendWebhook(prefs map[string]interface{}, eventType string) bool {
	webhookPrefs, ok := prefs["webhook"].(map[string]interface{})
	if !ok {
		return false
	}

	enabled, ok := webhookPrefs["enabled"].(bool)
	if !ok || !enabled {
		return false
	}

	// Check if this event type is in the events list
	events, ok := webhookPrefs["events"].([]interface{})
	if !ok {
		return false
	}

	for _, event := range events {
		if eventStr, ok := event.(string); ok && eventStr == eventType {
			return true
		}
	}

	return false
}

// sendEmailNotification sends an email notification
func (h *NotificationsHandler) sendEmailNotification(userID, eventType, title, message, actionURL string) error {
	// Get user email
	ctx := context.Background()
	var email string
	err := h.db.DB().QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email)
	if err != nil {
		return err
	}

	// Get SMTP configuration from environment
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	if smtpHost == "" || smtpPort == "" {
		return fmt.Errorf("SMTP not configured")
	}

	if smtpFrom == "" {
		smtpFrom = "noreply@streamspace.local"
	}

	// Create email template
	emailTemplate := `
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
		.content { padding: 20px; background-color: #f9f9f9; }
		.button { display: inline-block; padding: 10px 20px; background-color: #4CAF50; color: white; text-decoration: none; border-radius: 5px; margin-top: 15px; }
		.footer { text-align: center; padding: 20px; color: #777; font-size: 12px; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>StreamSpace Notification</h1>
		</div>
		<div class="content">
			<h2>{{.Title}}</h2>
			<p>{{.Message}}</p>
			{{if .ActionURL}}
			<a href="{{.ActionURL}}" class="button">View Details</a>
			{{end}}
		</div>
		<div class="footer">
			<p>This is an automated notification from StreamSpace.</p>
			<p>To manage your notification preferences, visit your account settings.</p>
		</div>
	</div>
</body>
</html>
`

	tmpl, err := template.New("email").Parse(emailTemplate)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	tmpl.Execute(&body, map[string]string{
		"Title":     title,
		"Message":   message,
		"ActionURL": actionURL,
	})

	// Send email
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", email, smtpFrom, title, body.String()))

	return smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpFrom, []string{email}, msg)
}

// sendWebhookNotification sends a webhook notification
func (h *NotificationsHandler) sendWebhookNotification(prefs map[string]interface{}, userID, eventType, title, message string, data map[string]interface{}) error {
	webhookPrefs, ok := prefs["webhook"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("webhook preferences not found")
	}

	webhookURL, ok := webhookPrefs["url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Create webhook payload
	payload := map[string]interface{}{
		"event":     eventType,
		"userId":    userID,
		"title":     title,
		"message":   message,
		"data":      data,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	payloadJSON, _ := json.Marshal(payload)

	// Create signature (HMAC-SHA256)
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = "default-secret"
	}

	h := hmac.New(sha256.New, []byte(webhookSecret))
	h.Write(payloadJSON)
	signature := hex.EncodeToString(h.Sum(nil))

	// Send HTTP POST request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-StreamSpace-Signature", signature)
	req.Header.Set("X-StreamSpace-Event", eventType)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// GetNotificationPreferences returns user's notification preferences
func (h *NotificationsHandler) GetNotificationPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	prefs, err := h.getUserNotificationPreferences(ctx, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdateNotificationPreferences updates user's notification preferences
func (h *NotificationsHandler) UpdateNotificationPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var prefs map[string]interface{}
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	prefsJSON, _ := json.Marshal(prefs)

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences)
		VALUES ($1, jsonb_build_object('notifications', $2::jsonb))
		ON CONFLICT (user_id)
		DO UPDATE SET
			preferences = jsonb_set(user_preferences.preferences, '{notifications}', $2::jsonb),
			updated_at = CURRENT_TIMESTAMP
	`, userIDStr, prefsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Notification preferences updated",
		"notifications": prefs,
	})
}

// TestEmailNotification sends a test email
func (h *NotificationsHandler) TestEmailNotification(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	err := h.sendEmailNotification(
		userIDStr,
		"test.email",
		"Test Email Notification",
		"This is a test email notification from StreamSpace. If you received this, your email notifications are working correctly!",
		"https://streamspace.local/settings/notifications",
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send test email: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test email sent successfully",
	})
}

// TestWebhookNotification sends a test webhook
func (h *NotificationsHandler) TestWebhookNotification(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	prefs, err := h.getUserNotificationPreferences(ctx, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	err = h.sendWebhookNotification(
		prefs,
		userIDStr,
		"test.webhook",
		"Test Webhook Notification",
		"This is a test webhook notification from StreamSpace",
		map[string]interface{}{"test": true},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send test webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test webhook sent successfully",
	})
}

// getDefaultNotificationPreferences returns default notification preferences
func (h *NotificationsHandler) getDefaultNotificationPreferences() map[string]interface{} {
	return map[string]interface{}{
		"email": map[string]bool{
			"session.created": false,
			"session.idle":    true,
			"session.shared":  true,
			"quota.warning":   true,
			"quota.exceeded":  true,
		},
		"inApp": map[string]bool{
			"session.created":   true,
			"session.idle":      true,
			"session.shared":    true,
			"quota.warning":     true,
			"quota.exceeded":    true,
			"team.invitation":   true,
			"system.alert":      true,
		},
		"webhook": map[string]interface{}{
			"enabled": false,
			"url":     "",
			"events":  []string{},
		},
	}
}
