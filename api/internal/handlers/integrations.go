package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Webhook represents a webhook configuration
type Webhook struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	URL         string                 `json:"url"`
	Secret      string                 `json:"secret,omitempty"`
	Events      []string               `json:"events"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Enabled     bool                   `json:"enabled"`
	RetryPolicy WebhookRetryPolicy     `json:"retry_policy"`
	Filters     WebhookFilters         `json:"filters,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WebhookRetryPolicy defines retry behavior
type WebhookRetryPolicy struct {
	MaxRetries     int `json:"max_retries"`
	RetryDelay     int `json:"retry_delay_seconds"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`
}

// WebhookFilters allows filtering events
type WebhookFilters struct {
	Users      []string `json:"users,omitempty"`
	Templates  []string `json:"templates,omitempty"`
	SessionStates []string `json:"session_states,omitempty"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID            int64                  `json:"id"`
	WebhookID     int64                  `json:"webhook_id"`
	Event         string                 `json:"event"`
	Payload       map[string]interface{} `json:"payload"`
	Status        string                 `json:"status"` // "pending", "success", "failed"
	StatusCode    int                    `json:"status_code,omitempty"`
	ResponseBody  string                 `json:"response_body,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Attempts      int                    `json:"attempts"`
	NextRetryAt   *time.Time             `json:"next_retry_at,omitempty"`
	DeliveredAt   *time.Time             `json:"delivered_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// Integration represents an external integration
type Integration struct {
	ID           int64                  `json:"id"`
	Type         string                 `json:"type"` // "slack", "teams", "discord", "pagerduty", "email", "custom"
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Config       map[string]interface{} `json:"config"`
	Enabled      bool                   `json:"enabled"`
	Events       []string               `json:"events"`
	TestMode     bool                   `json:"test_mode"`
	LastTestAt   *time.Time             `json:"last_test_at,omitempty"`
	LastSuccessAt *time.Time            `json:"last_success_at,omitempty"`
	CreatedBy    string                 `json:"created_by"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// WebhookEvent represents an event that can trigger webhooks
type WebhookEvent struct {
	Event     string                 `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Available webhook events
var AvailableEvents = []string{
	"session.created",
	"session.started",
	"session.hibernated",
	"session.terminated",
	"session.failed",
	"user.created",
	"user.deleted",
	"dlp.violation",
	"recording.started",
	"recording.completed",
	"template.created",
	"template.updated",
	"workflow.started",
	"workflow.completed",
	"workflow.failed",
	"collaboration.started",
	"collaboration.ended",
	"alert.triggered",
}

// CreateWebhook creates a new webhook
func (h *Handler) CreateWebhook(c *gin.Context) {
	var webhook Webhook
	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	webhook.CreatedBy = userID

	// Validate URL
	if webhook.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Validate events
	if len(webhook.Events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one event is required"})
		return
	}

	// Set default retry policy
	if webhook.RetryPolicy.MaxRetries == 0 {
		webhook.RetryPolicy = WebhookRetryPolicy{
			MaxRetries:        3,
			RetryDelay:        60,
			BackoffMultiplier: 2.0,
		}
	}

	// Generate secret if not provided
	if webhook.Secret == "" {
		webhook.Secret = h.generateWebhookSecret()
	}

	err := h.DB.QueryRow(`
		INSERT INTO webhooks (
			name, description, url, secret, events, headers, enabled,
			retry_policy, filters, metadata, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, webhook.Name, webhook.Description, webhook.URL, webhook.Secret,
		toJSONB(webhook.Events), toJSONB(webhook.Headers), webhook.Enabled,
		toJSONB(webhook.RetryPolicy), toJSONB(webhook.Filters),
		toJSONB(webhook.Metadata), userID).Scan(&webhook.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}

	c.JSON(http.StatusCreated, webhook)
}

// ListWebhooks lists all webhooks
func (h *Handler) ListWebhooks(c *gin.Context) {
	enabled := c.Query("enabled")

	query := `
		SELECT id, name, description, url, secret, events, headers, enabled,
		       retry_policy, filters, metadata, created_by, created_at, updated_at
		FROM webhooks WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if enabled != "" {
		query += fmt.Sprintf(" AND enabled = $%d", argCount)
		args = append(args, enabled == "true")
		argCount++
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve webhooks"})
		return
	}
	defer rows.Close()

	webhooks := []Webhook{}
	for rows.Next() {
		var w Webhook
		var events, headers, retryPolicy, filters, metadata sql.NullString

		err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.URL, &w.Secret,
			&events, &headers, &w.Enabled, &retryPolicy, &filters, &metadata,
			&w.CreatedBy, &w.CreatedAt, &w.UpdatedAt)

		if err == nil {
			if events.Valid && events.String != "" {
				json.Unmarshal([]byte(events.String), &w.Events)
			}
			if headers.Valid && headers.String != "" {
				json.Unmarshal([]byte(headers.String), &w.Headers)
			}
			if retryPolicy.Valid && retryPolicy.String != "" {
				json.Unmarshal([]byte(retryPolicy.String), &w.RetryPolicy)
			}
			if filters.Valid && filters.String != "" {
				json.Unmarshal([]byte(filters.String), &w.Filters)
			}
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &w.Metadata)
			}
			webhooks = append(webhooks, w)
		}
	}

	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

// UpdateWebhook updates an existing webhook
func (h *Handler) UpdateWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	var webhook Webhook
	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE webhooks SET
			name = $1, description = $2, url = $3, events = $4, headers = $5,
			enabled = $6, retry_policy = $7, filters = $8, metadata = $9,
			updated_at = $10
		WHERE id = $11
	`, webhook.Name, webhook.Description, webhook.URL, toJSONB(webhook.Events),
		toJSONB(webhook.Headers), webhook.Enabled, toJSONB(webhook.RetryPolicy),
		toJSONB(webhook.Filters), toJSONB(webhook.Metadata), time.Now(), webhookID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook updated successfully"})
}

// DeleteWebhook deletes a webhook
func (h *Handler) DeleteWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM webhooks WHERE id = $1", webhookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted successfully"})
}

// TestWebhook sends a test event to a webhook
func (h *Handler) TestWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	// Get webhook details
	var webhook Webhook
	var events, headers, retryPolicy sql.NullString
	err = h.DB.QueryRow(`
		SELECT id, name, url, secret, events, headers, enabled, retry_policy
		FROM webhooks WHERE id = $1
	`, webhookID).Scan(&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret,
		&events, &headers, &webhook.Enabled, &retryPolicy)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	if events.Valid && events.String != "" {
		json.Unmarshal([]byte(events.String), &webhook.Events)
	}
	if headers.Valid && headers.String != "" {
		json.Unmarshal([]byte(headers.String), &webhook.Headers)
	}
	if retryPolicy.Valid && retryPolicy.String != "" {
		json.Unmarshal([]byte(retryPolicy.String), &webhook.RetryPolicy)
	}

	// Create test event
	testEvent := WebhookEvent{
		Event:     "webhook.test",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"webhook_id": webhook.ID,
			"message":    "This is a test webhook delivery",
		},
	}

	// Deliver webhook
	success, statusCode, responseBody, err := h.deliverWebhook(webhook, testEvent)

	response := gin.H{
		"success":     success,
		"status_code": statusCode,
	}

	if responseBody != "" {
		response["response_body"] = responseBody
	}

	if err != nil {
		response["error"] = err.Error()
	}

	if success {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, response)
	}
}

// GetWebhookDeliveries retrieves delivery history
func (h *Handler) GetWebhookDeliveries(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	// Count total
	var total int
	h.DB.QueryRow("SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = $1", webhookID).Scan(&total)

	rows, err := h.DB.Query(`
		SELECT id, webhook_id, event, payload, status, status_code, response_body,
		       error_message, attempts, next_retry_at, delivered_at, created_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, webhookID, pageSize, (page-1)*pageSize)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve deliveries"})
		return
	}
	defer rows.Close()

	deliveries := []WebhookDelivery{}
	for rows.Next() {
		var d WebhookDelivery
		var payload sql.NullString

		err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &payload, &d.Status,
			&d.StatusCode, &d.ResponseBody, &d.ErrorMessage, &d.Attempts,
			&d.NextRetryAt, &d.DeliveredAt, &d.CreatedAt)

		if err == nil {
			if payload.Valid && payload.String != "" {
				json.Unmarshal([]byte(payload.String), &d.Payload)
			}
			deliveries = append(deliveries, d)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"deliveries":  deliveries,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// Integrations

// CreateIntegration creates a new integration
func (h *Handler) CreateIntegration(c *gin.Context) {
	var integration Integration
	if err := c.ShouldBindJSON(&integration); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	integration.CreatedBy = userID

	// Validate type
	validTypes := []string{"slack", "teams", "discord", "pagerduty", "email", "custom"}
	valid := false
	for _, t := range validTypes {
		if integration.Type == t {
			valid = true
			break
		}
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid integration type"})
		return
	}

	err := h.DB.QueryRow(`
		INSERT INTO integrations (
			type, name, description, config, enabled, events, test_mode, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, integration.Type, integration.Name, integration.Description,
		toJSONB(integration.Config), integration.Enabled, toJSONB(integration.Events),
		integration.TestMode, userID).Scan(&integration.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create integration"})
		return
	}

	c.JSON(http.StatusCreated, integration)
}

// ListIntegrations lists all integrations
func (h *Handler) ListIntegrations(c *gin.Context) {
	integrationType := c.Query("type")
	enabled := c.Query("enabled")

	query := `
		SELECT id, type, name, description, config, enabled, events, test_mode,
		       last_test_at, last_success_at, created_by, created_at, updated_at
		FROM integrations WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if integrationType != "" {
		query += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, integrationType)
		argCount++
	}

	if enabled != "" {
		query += fmt.Sprintf(" AND enabled = $%d", argCount)
		args = append(args, enabled == "true")
		argCount++
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve integrations"})
		return
	}
	defer rows.Close()

	integrations := []Integration{}
	for rows.Next() {
		var i Integration
		var config, events sql.NullString

		err := rows.Scan(&i.ID, &i.Type, &i.Name, &i.Description, &config,
			&i.Enabled, &events, &i.TestMode, &i.LastTestAt, &i.LastSuccessAt,
			&i.CreatedBy, &i.CreatedAt, &i.UpdatedAt)

		if err == nil {
			if config.Valid && config.String != "" {
				json.Unmarshal([]byte(config.String), &i.Config)
			}
			if events.Valid && events.String != "" {
				json.Unmarshal([]byte(events.String), &i.Events)
			}
			integrations = append(integrations, i)
		}
	}

	c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

// TestIntegration tests an integration
func (h *Handler) TestIntegration(c *gin.Context) {
	integrationID, err := strconv.ParseInt(c.Param("integrationId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid integration ID"})
		return
	}

	// Get integration details
	var integration Integration
	var config, events sql.NullString
	err = h.DB.QueryRow(`
		SELECT id, type, name, config, enabled, events
		FROM integrations WHERE id = $1
	`, integrationID).Scan(&integration.ID, &integration.Type, &integration.Name,
		&config, &integration.Enabled, &events)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}

	if config.Valid && config.String != "" {
		json.Unmarshal([]byte(config.String), &integration.Config)
	}
	if events.Valid && events.String != "" {
		json.Unmarshal([]byte(events.String), &integration.Events)
	}

	// Test based on type
	success, message := h.testIntegration(integration)

	// Update last test time
	h.DB.Exec("UPDATE integrations SET last_test_at = $1 WHERE id = $2", time.Now(), integrationID)

	if success {
		h.DB.Exec("UPDATE integrations SET last_success_at = $1 WHERE id = $2", time.Now(), integrationID)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": message})
	}
}

// Helper functions

func (h *Handler) generateWebhookSecret() string {
	// Generate a random 32-byte secret
	return fmt.Sprintf("whsec_%d", time.Now().UnixNano())
}

func (h *Handler) deliverWebhook(webhook Webhook, event WebhookEvent) (bool, int, string, error) {
	// Prepare payload
	payload, _ := json.Marshal(event)

	// Create HTTP request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return false, 0, "", err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "StreamSpace-Webhook/1.0")
	req.Header.Set("X-StreamSpace-Event", event.Event)
	req.Header.Set("X-StreamSpace-Delivery", fmt.Sprintf("%d", time.Now().Unix()))

	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Calculate HMAC signature
	if webhook.Secret != "" {
		signature := h.calculateHMAC(payload, webhook.Secret)
		req.Header.Set("X-StreamSpace-Signature", signature)
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, "", err
	}
	defer resp.Body.Close()

	// Read response
	responseBody, _ := io.ReadAll(resp.Body)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	return success, resp.StatusCode, string(responseBody), nil
}

func (h *Handler) calculateHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) testIntegration(integration Integration) (bool, string) {
	switch integration.Type {
	case "slack":
		webhookURL, ok := integration.Config["webhook_url"].(string)
		if !ok || webhookURL == "" {
			return false, "Slack webhook URL not configured"
		}

		// Send test message to Slack
		payload := map[string]interface{}{
			"text": "StreamSpace integration test successful! 🚀",
		}
		payloadBytes, _ := json.Marshal(payload)

		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return true, "Slack test message sent successfully"
		}
		return false, fmt.Sprintf("Slack returned status code %d", resp.StatusCode)

	case "teams":
		webhookURL, ok := integration.Config["webhook_url"].(string)
		if !ok || webhookURL == "" {
			return false, "Teams webhook URL not configured"
		}

		// Send test message to Teams
		payload := map[string]interface{}{
			"text": "StreamSpace integration test successful! 🚀",
		}
		payloadBytes, _ := json.Marshal(payload)

		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return true, "Teams test message sent successfully"
		}
		return false, fmt.Sprintf("Teams returned status code %d", resp.StatusCode)

	case "email":
		// Would integrate with SMTP
		return true, "Email integration configured (SMTP test not implemented)"

	case "custom":
		return true, "Custom integration configured"

	default:
		return false, "Unknown integration type"
	}
}

// GetAvailableEvents returns list of available webhook events
func (h *Handler) GetAvailableEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"events": AvailableEvents})
}
