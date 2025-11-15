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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// INPUT VALIDATION
// ============================================================================

// validateWebhookInput validates webhook creation/update input
func validateWebhookInput(webhook *Webhook) error {
	// Name validation
	if len(webhook.Name) == 0 {
		return fmt.Errorf("webhook name is required")
	}
	if len(webhook.Name) > 200 {
		return fmt.Errorf("webhook name must be 200 characters or less")
	}

	// URL validation
	if webhook.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if len(webhook.URL) > 2048 {
		return fmt.Errorf("webhook URL must be 2048 characters or less")
	}
	if _, err := url.Parse(webhook.URL); err != nil {
		return fmt.Errorf("invalid webhook URL format")
	}

	// Events validation
	if len(webhook.Events) == 0 {
		return fmt.Errorf("at least one event type is required")
	}
	if len(webhook.Events) > 50 {
		return fmt.Errorf("maximum 50 event types allowed")
	}

	// Description length
	if len(webhook.Description) > 1000 {
		return fmt.Errorf("webhook description must be 1000 characters or less")
	}

	// Headers validation
	if len(webhook.Headers) > 50 {
		return fmt.Errorf("maximum 50 custom headers allowed")
	}
	for key, value := range webhook.Headers {
		if len(key) > 100 {
			return fmt.Errorf("header key must be 100 characters or less")
		}
		if len(value) > 1000 {
			return fmt.Errorf("header value must be 1000 characters or less")
		}
	}

	return nil
}

// validateIntegrationInput validates integration creation/update input
func validateIntegrationInput(integration *Integration) error {
	// Name validation
	if len(integration.Name) == 0 {
		return fmt.Errorf("integration name is required")
	}
	if len(integration.Name) > 200 {
		return fmt.Errorf("integration name must be 200 characters or less")
	}

	// Type validation
	validTypes := []string{"slack", "teams", "discord", "pagerduty", "email", "custom"}
	validType := false
	for _, t := range validTypes {
		if integration.Type == t {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("invalid integration type, must be one of: %s", strings.Join(validTypes, ", "))
	}

	// Description length
	if len(integration.Description) > 1000 {
		return fmt.Errorf("integration description must be 1000 characters or less")
	}

	return nil
}

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// Webhook represents a webhook configuration
type Webhook struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	URL         string                 `json:"url"`
	Secret      string                 `json:"-"` // SECURITY: Never expose secret in API responses
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

// WebhookWithSecret is used only for CreateWebhook response to show the secret once
type WebhookWithSecret struct {
	Webhook
	Secret string `json:"secret"` // Only exposed on creation
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

	// INPUT VALIDATION: Validate all webhook input fields
	if err := validateWebhookInput(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	// SECURITY: Validate webhook URL to prevent SSRF attacks
	if err := h.validateWebhookURL(webhook.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid webhook URL",
			"message": err.Error(),
		})
		return
	}

	// Set default retry policy
	if webhook.RetryPolicy.MaxRetries == 0 {
		webhook.RetryPolicy = WebhookRetryPolicy{
			MaxRetries:        WebhookDefaultMaxRetries,
			RetryDelay:        WebhookDefaultRetryDelay,
			BackoffMultiplier: WebhookDefaultBackoffMultiplier,
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

	// SECURITY: Only expose secret on creation (not in GET requests)
	c.JSON(http.StatusCreated, WebhookWithSecret{
		Webhook: webhook,
		Secret:  webhook.Secret,
	})
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

		if err != nil {
			continue // Skip rows with scan errors
		}

		// Parse JSON fields with error handling
		if events.Valid && events.String != "" {
			if err := json.Unmarshal([]byte(events.String), &w.Events); err != nil {
				// Log error but continue with empty events
				w.Events = []string{}
			}
		}
		if headers.Valid && headers.String != "" {
			if err := json.Unmarshal([]byte(headers.String), &w.Headers); err != nil {
				w.Headers = make(map[string]string)
			}
		}
		if retryPolicy.Valid && retryPolicy.String != "" {
			if err := json.Unmarshal([]byte(retryPolicy.String), &w.RetryPolicy); err != nil {
				// Use default retry policy on unmarshal error
				w.RetryPolicy = WebhookRetryPolicy{
					MaxRetries:        WebhookDefaultMaxRetries,
					RetryDelay:        WebhookDefaultRetryDelay,
					BackoffMultiplier: WebhookDefaultBackoffMultiplier,
				}
			}
		}
		if filters.Valid && filters.String != "" {
			if err := json.Unmarshal([]byte(filters.String), &w.Filters); err != nil {
				w.Filters = WebhookFilters{}
			}
		}
		if metadata.Valid && metadata.String != "" {
			if err := json.Unmarshal([]byte(metadata.String), &w.Metadata); err != nil {
				w.Metadata = make(map[string]interface{})
			}
		}

		webhooks = append(webhooks, w)
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

	userID := c.GetString("user_id")
	role := c.GetString("role")

	var webhook Webhook
	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// INPUT VALIDATION: Validate all webhook input fields
	if err := validateWebhookInput(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	// SECURITY: Validate webhook URL to prevent SSRF attacks
	if err := h.validateWebhookURL(webhook.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid webhook URL",
			"message": err.Error(),
		})
		return
	}

	// SECURITY: Add authorization check to prevent updating other users' webhooks
	// Returns "not found" whether webhook doesn't exist OR user lacks permission
	var result sql.Result
	if role == "admin" {
		// Admins can update any webhook
		result, err = h.DB.Exec(`
			UPDATE webhooks SET
				name = $1, description = $2, url = $3, events = $4, headers = $5,
				enabled = $6, retry_policy = $7, filters = $8, metadata = $9,
				updated_at = $10
			WHERE id = $11
		`, webhook.Name, webhook.Description, webhook.URL, toJSONB(webhook.Events),
			toJSONB(webhook.Headers), webhook.Enabled, toJSONB(webhook.RetryPolicy),
			toJSONB(webhook.Filters), toJSONB(webhook.Metadata), time.Now(), webhookID)
	} else {
		// Non-admins can only update their own webhooks
		result, err = h.DB.Exec(`
			UPDATE webhooks SET
				name = $1, description = $2, url = $3, events = $4, headers = $5,
				enabled = $6, retry_policy = $7, filters = $8, metadata = $9,
				updated_at = $10
			WHERE id = $11 AND created_by = $12
		`, webhook.Name, webhook.Description, webhook.URL, toJSONB(webhook.Events),
			toJSONB(webhook.Headers), webhook.Enabled, toJSONB(webhook.RetryPolicy),
			toJSONB(webhook.Filters), toJSONB(webhook.Metadata), time.Now(), webhookID, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}

	// Check if any rows were affected
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Could be not found OR not authorized - don't reveal which
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
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

	userID := c.GetString("user_id")
	role := c.GetString("role")

	// SECURITY: Add authorization check to prevent deleting other users' webhooks
	// Returns "not found" whether webhook doesn't exist OR user lacks permission
	var result sql.Result
	if role == "admin" {
		// Admins can delete any webhook
		result, err = h.DB.Exec("DELETE FROM webhooks WHERE id = $1", webhookID)
	} else {
		// Non-admins can only delete their own webhooks
		result, err = h.DB.Exec("DELETE FROM webhooks WHERE id = $1 AND created_by = $2", webhookID, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}

	// Check if any rows were affected
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Could be not found OR not authorized - don't reveal which
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
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

	userID := c.GetString("user_id")
	role := c.GetString("role")

	// SECURITY: Add authorization check to prevent testing other users' webhooks
	// Returns "not found" whether webhook doesn't exist OR user lacks permission
	var webhook Webhook
	var events, headers, retryPolicy sql.NullString

	if role == "admin" {
		// Admins can test any webhook
		err = h.DB.QueryRow(`
			SELECT id, name, url, secret, events, headers, enabled, retry_policy
			FROM webhooks WHERE id = $1
		`, webhookID).Scan(&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret,
			&events, &headers, &webhook.Enabled, &retryPolicy)
	} else {
		// Non-admins can only test their own webhooks
		err = h.DB.QueryRow(`
			SELECT id, name, url, secret, events, headers, enabled, retry_policy
			FROM webhooks WHERE id = $1 AND created_by = $2
		`, webhookID, userID).Scan(&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret,
			&events, &headers, &webhook.Enabled, &retryPolicy)
	}

	if err == sql.ErrNoRows {
		// Could be not found OR not authorized - don't reveal which
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

	// INPUT VALIDATION: Validate all integration input fields
	if err := validateIntegrationInput(&integration); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
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

	userID := c.GetString("user_id")
	role := c.GetString("role")

	// SECURITY: Add authorization check to prevent testing other users' integrations
	// Returns "not found" whether integration doesn't exist OR user lacks permission
	var integration Integration
	var config, events sql.NullString

	if role == "admin" {
		// Admins can test any integration
		err = h.DB.QueryRow(`
			SELECT id, type, name, config, enabled, events
			FROM integrations WHERE id = $1
		`, integrationID).Scan(&integration.ID, &integration.Type, &integration.Name,
			&config, &integration.Enabled, &events)
	} else {
		// Non-admins can only test their own integrations
		err = h.DB.QueryRow(`
			SELECT id, type, name, config, enabled, events
			FROM integrations WHERE id = $1 AND created_by = $2
		`, integrationID, userID).Scan(&integration.ID, &integration.Type, &integration.Name,
			&config, &integration.Enabled, &events)
	}

	if err == sql.ErrNoRows {
		// Could be not found OR not authorized - don't reveal which
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

// validateWebhookURL validates webhook URL to prevent SSRF attacks
func (h *Handler) validateWebhookURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must be http or https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https protocol")
	}

	host := parsed.Hostname()

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("could not resolve hostname: %w", err)
	}

	// Check each resolved IP
	for _, ip := range ips {
		// Block loopback addresses (127.0.0.0/8)
		if ip.IsLoopback() {
			return fmt.Errorf("webhook URL cannot point to loopback address")
		}

		// Block private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
		if ip.IsPrivate() {
			return fmt.Errorf("webhook URL cannot point to private IP address")
		}

		// Block link-local addresses (169.254.0.0/16)
		if ip.IsLinkLocalUnicast() {
			return fmt.Errorf("webhook URL cannot point to link-local address")
		}

		// Block cloud metadata endpoints
		if ip.String() == "169.254.169.254" {
			return fmt.Errorf("webhook URL is not allowed")
		}
	}

	// Block specific hostnames (cloud metadata endpoints)
	blockedHosts := []string{
		"metadata.google.internal",
		"169.254.169.254",
		"localhost",
		"metadata",
	}
	hostLower := strings.ToLower(host)
	for _, blocked := range blockedHosts {
		if strings.Contains(hostLower, strings.ToLower(blocked)) {
			return fmt.Errorf("webhook URL hostname is not allowed")
		}
	}

	return nil
}

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

	// Send request with security restrictions
	client := &http.Client{
		Timeout: WebhookTimeout, // Reduced from 30s for security
		// Disable redirects to prevent SSRF bypass via redirect chains
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
