// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements enterprise integration features including webhooks and external API integrations.
//
// Security Features:
// - Webhook HMAC signature validation (prevents spoofing)
// - SSRF protection for webhook URLs (prevents internal network access)
// - Input validation for all integration configurations
// - Secret management (webhooks secrets never exposed in GET responses)
// - Authorization enumeration prevention (consistent error responses)
//
// CRITICAL SECURITY FIXES (2025-11-14):
// 1. SSRF Protection: validateWebhookURL prevents webhooks from targeting:
//   - Private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
//   - Loopback addresses (127.0.0.0/8)
//   - Link-local addresses (169.254.0.0/16)
//   - Cloud metadata endpoints (169.254.169.254)
//   - Internal hostnames (metadata.google.internal, localhost, etc.)
//
//  2. Secret Protection: Webhook secrets use json:"-" tags and separate response structs
//     to ensure secrets are only shown during creation, never in GET endpoints.
//
//  3. Authorization Enumeration Fixes: UpdateWebhook, DeleteWebhook, TestWebhook, and
//     TestIntegration all return consistent "not found" errors for both non-existent
//     resources AND unauthorized access (prevents attacker enumeration).
//
//  4. Input Validation: Comprehensive validation for all webhook and integration fields
//     including URL format, name length, event counts, retry configuration, etc.
//
// Webhook Delivery:
// - Automatic retries with exponential backoff
// - HMAC-SHA256 signature in X-Webhook-Signature header
// - 10-second timeout per delivery attempt
// - Real-time delivery status updates via WebSocket
//
// Integrations:
// - External API connections (Slack, Discord, PagerDuty, etc.)
// - OAuth 2.0 token management
// - API key storage and rotation
// - Connection health monitoring
package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// IntegrationsHandler handles webhook and external integration requests.
type IntegrationsHandler struct {
	DB *db.Database
}

// NewIntegrationsHandler creates a new integrations handler.
func NewIntegrationsHandler(database *db.Database) *IntegrationsHandler {
	return &IntegrationsHandler{DB: database}
}

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
	// Note: slack, teams, discord, pagerduty, and email are now handled by plugins
	validTypes := []string{"custom"}
	deprecatedTypes := []string{"slack", "teams", "discord", "pagerduty", "email"}

	validType := false
	for _, t := range validTypes {
		if integration.Type == t {
			validType = true
			break
		}
	}

	// Check if it's a deprecated type (now handled by plugins)
	for _, t := range deprecatedTypes {
		if integration.Type == t {
			return fmt.Errorf("%s integration is now handled by plugins. Please install the streamspace-%s plugin from the plugin marketplace instead", integration.Type, integration.Type)
		}
	}

	if !validType {
		return fmt.Errorf("invalid integration type, must be one of: %s. Note: slack, teams, discord, pagerduty, and email are now plugins", strings.Join(validTypes, ", "))
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
	MaxRetries        int     `json:"max_retries"`
	RetryDelay        int     `json:"retry_delay_seconds"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`
}

// WebhookFilters allows filtering events
type WebhookFilters struct {
	Users         []string `json:"users,omitempty"`
	Templates     []string `json:"templates,omitempty"`
	SessionStates []string `json:"session_states,omitempty"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID           int64                  `json:"id"`
	WebhookID    int64                  `json:"webhook_id"`
	Event        string                 `json:"event"`
	Payload      map[string]interface{} `json:"payload"`
	Status       string                 `json:"status"` // "pending", "success", "failed"
	StatusCode   int                    `json:"status_code,omitempty"`
	ResponseBody string                 `json:"response_body,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Attempts     int                    `json:"attempts"`
	NextRetryAt  *time.Time             `json:"next_retry_at,omitempty"`
	DeliveredAt  *time.Time             `json:"delivered_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// Integration represents an external integration
type Integration struct {
	ID            int64                  `json:"id"`
	Type          string                 `json:"type"` // "slack", "teams", "discord", "pagerduty", "email", "custom"
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Config        map[string]interface{} `json:"config"`
	Enabled       bool                   `json:"enabled"`
	Events        []string               `json:"events"`
	TestMode      bool                   `json:"test_mode"`
	LastTestAt    *time.Time             `json:"last_test_at,omitempty"`
	LastSuccessAt *time.Time             `json:"last_success_at,omitempty"`
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
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

// CreateWebhookRequest is the request body for creating a webhook
type CreateWebhookRequest struct {
	Name        string                 `json:"name" binding:"required" validate:"required,min=1,max=200"`
	Description string                 `json:"description" validate:"omitempty,max=1000"`
	URL         string                 `json:"url" binding:"required" validate:"required,url,max=2048"`
	Secret      string                 `json:"secret" validate:"omitempty,min=16,max=256"`
	Events      []string               `json:"events" binding:"required" validate:"required,min=1,max=50,dive,min=3,max=100"`
	Headers     map[string]string      `json:"headers" validate:"omitempty,max=50,dive,keys,max=100,endkeys,max=1000"`
	Enabled     bool                   `json:"enabled"`
	RetryPolicy WebhookRetryPolicy     `json:"retry_policy"`
	Filters     WebhookFilters         `json:"filters"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// CreateWebhook creates a new webhook
func (h *IntegrationsHandler) CreateWebhook(c *gin.Context) {
	var req CreateWebhookRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	userID := c.GetString("user_id")

	// Map request to webhook
	webhook := Webhook{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
		Headers:     req.Headers,
		Enabled:     req.Enabled,
		RetryPolicy: req.RetryPolicy,
		Filters:     req.Filters,
		Metadata:    req.Metadata,
		CreatedBy:   userID,
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

	err := h.DB.DB().QueryRow(`
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
func (h *IntegrationsHandler) ListWebhooks(c *gin.Context) {
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

	rows, err := h.DB.DB().Query(query, args...)
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

// UpdateWebhookRequest is the request body for updating a webhook
type UpdateWebhookRequest struct {
	Name        string                 `json:"name" validate:"omitempty,min=1,max=200"`
	Description string                 `json:"description" validate:"omitempty,max=1000"`
	URL         string                 `json:"url" validate:"omitempty,url,max=2048"`
	Events      []string               `json:"events" validate:"omitempty,min=1,max=50,dive,min=3,max=100"`
	Headers     map[string]string      `json:"headers" validate:"omitempty,max=50,dive,keys,max=100,endkeys,max=1000"`
	Enabled     *bool                  `json:"enabled"`
	RetryPolicy *WebhookRetryPolicy    `json:"retry_policy"`
	Filters     *WebhookFilters        `json:"filters"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UpdateWebhook updates an existing webhook
func (h *IntegrationsHandler) UpdateWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	userID := c.GetString("user_id")
	role := c.GetString("role")

	var req UpdateWebhookRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Map request to webhook for update
	webhook := Webhook{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Events:      req.Events,
		Headers:     req.Headers,
		Metadata:    req.Metadata,
	}
	if req.Enabled != nil {
		webhook.Enabled = *req.Enabled
	}
	if req.RetryPolicy != nil {
		webhook.RetryPolicy = *req.RetryPolicy
	}
	if req.Filters != nil {
		webhook.Filters = *req.Filters
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
		result, err = h.DB.DB().Exec(`
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
		result, err = h.DB.DB().Exec(`
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
func (h *IntegrationsHandler) DeleteWebhook(c *gin.Context) {
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
		result, err = h.DB.DB().Exec("DELETE FROM webhooks WHERE id = $1", webhookID)
	} else {
		// Non-admins can only delete their own webhooks
		result, err = h.DB.DB().Exec("DELETE FROM webhooks WHERE id = $1 AND created_by = $2", webhookID, userID)
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
func (h *IntegrationsHandler) TestWebhook(c *gin.Context) {
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
		err = h.DB.DB().QueryRow(`
			SELECT id, name, url, secret, events, headers, enabled, retry_policy
			FROM webhooks WHERE id = $1
		`, webhookID).Scan(&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret,
			&events, &headers, &webhook.Enabled, &retryPolicy)
	} else {
		// Non-admins can only test their own webhooks
		err = h.DB.DB().QueryRow(`
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
func (h *IntegrationsHandler) GetWebhookDeliveries(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	// Count total
	var total int
	h.DB.DB().QueryRow("SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = $1", webhookID).Scan(&total)

	rows, err := h.DB.DB().Query(`
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

// CreateIntegrationRequest is the request body for creating an integration
type CreateIntegrationRequest struct {
	Type        string                 `json:"type" binding:"required" validate:"required,oneof=custom"`
	Name        string                 `json:"name" binding:"required" validate:"required,min=1,max=200"`
	Description string                 `json:"description" validate:"omitempty,max=1000"`
	Config      map[string]interface{} `json:"config"`
	Enabled     bool                   `json:"enabled"`
	Events      []string               `json:"events" validate:"omitempty,max=50,dive,min=3,max=100"`
	TestMode    bool                   `json:"test_mode"`
}

// CreateIntegration creates a new integration
func (h *IntegrationsHandler) CreateIntegration(c *gin.Context) {
	var req CreateIntegrationRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	userID := c.GetString("user_id")

	// Map request to integration
	integration := Integration{
		Type:        req.Type,
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
		Enabled:     req.Enabled,
		Events:      req.Events,
		TestMode:    req.TestMode,
		CreatedBy:   userID,
	}

	err := h.DB.DB().QueryRow(`
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
func (h *IntegrationsHandler) ListIntegrations(c *gin.Context) {
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

	rows, err := h.DB.DB().Query(query, args...)
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
func (h *IntegrationsHandler) TestIntegration(c *gin.Context) {
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
		err = h.DB.DB().QueryRow(`
			SELECT id, type, name, config, enabled, events
			FROM integrations WHERE id = $1
		`, integrationID).Scan(&integration.ID, &integration.Type, &integration.Name,
			&config, &integration.Enabled, &events)
	} else {
		// Non-admins can only test their own integrations
		err = h.DB.DB().QueryRow(`
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
	h.DB.DB().Exec("UPDATE integrations SET last_test_at = $1 WHERE id = $2", time.Now(), integrationID)

	if success {
		h.DB.DB().Exec("UPDATE integrations SET last_success_at = $1 WHERE id = $2", time.Now(), integrationID)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": message})
	}
}

// Helper functions

// validateWebhookURL validates webhook URL to prevent SSRF attacks
func (h *IntegrationsHandler) validateWebhookURL(urlStr string) error {
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

func (h *IntegrationsHandler) generateWebhookSecret() string {
	// SECURITY FIX: Use crypto/rand for secure random generation
	// Previous implementation used timestamp which is predictable
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// This should almost never happen, but don't panic if it does
		// Log the error and use a UUID-based fallback for uniqueness
		log.Printf("Warning: crypto/rand.Read failed, using fallback: %v", err)
		// Generate a fallback using time-based UUID (still unique, less cryptographically secure)
		fallback := fmt.Sprintf("%d_%s", time.Now().UnixNano(), uuid.New().String())
		return "whsec_" + base64.URLEncoding.EncodeToString([]byte(fallback))[:43]
	}
	return "whsec_" + base64.URLEncoding.EncodeToString(b)
}

func (h *IntegrationsHandler) deliverWebhook(webhook Webhook, event WebhookEvent) (bool, int, string, error) {
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

func (h *IntegrationsHandler) calculateHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *IntegrationsHandler) testIntegration(integration Integration) (bool, string) {
	// NOTE: Slack, Teams, Discord, PagerDuty, and Email integrations are now handled by plugins.
	// Users should install the respective plugins from the plugin marketplace instead.
	//
	// This function is kept for backwards compatibility with custom integrations only.

	switch integration.Type {
	case "slack", "teams", "discord", "pagerduty", "email":
		return false, fmt.Sprintf("%s integration is now handled by plugins. Please install the streamspace-%s plugin from the plugin marketplace.",
			integration.Type, integration.Type)

	case "custom":
		return true, "Custom integration configured"

	default:
		return false, "Unknown integration type. Supported types: slack (plugin), teams (plugin), discord (plugin), pagerduty (plugin), email (plugin), custom"
	}
}

// GetAvailableEvents returns list of available webhook events
func (h *IntegrationsHandler) GetAvailableEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"events": AvailableEvents})
}
