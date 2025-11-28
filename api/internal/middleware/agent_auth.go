// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements API key authentication middleware for agents.
//
// SECURITY: Agent API Key Authentication Middleware
//
// This middleware validates agent API keys on incoming requests.
// It is used to protect agent-specific endpoints:
//   - POST /api/v1/agents/register (agent self-registration)
//   - GET  /api/v1/agents/connect (WebSocket upgrade)
//
// The middleware:
//   1. Extracts API key from X-Agent-API-Key header
//   2. Validates key format (64 hex chars)
//   3. Looks up agent by agent_id query/path parameter
//   4. Compares provided key against stored bcrypt hash
//   5. Updates api_key_last_used_at on successful auth
//   6. Sets agent_id in Gin context for downstream handlers
//
// Usage:
//
//	agentAuth := middleware.NewAgentAuth(database)
//	router.POST("/agents/register", agentAuth.RequireAPIKey(), handler.RegisterAgent)
//	router.GET("/agents/connect", agentAuth.RequireAPIKey(), handler.HandleAgentConnection)
package middleware

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/auth"
	"github.com/streamspace-dev/streamspace/api/internal/db"
)

// AgentAuth provides API key authentication middleware for agents.
type AgentAuth struct {
	database *db.Database
}

// NewAgentAuth creates a new agent authentication middleware.
//
// Example:
//
//	agentAuth := middleware.NewAgentAuth(database)
//	router.Use(agentAuth.RequireAPIKey())
func NewAgentAuth(database *db.Database) *AgentAuth {
	return &AgentAuth{
		database: database,
	}
}

// RequireAPIKey returns a middleware that requires a valid agent API key.
//
// The middleware:
//   - Extracts API key from X-Agent-API-Key header
//   - Extracts agent_id from query parameter or path parameter
//   - Validates key against database
//   - Updates last used timestamp
//   - Sets authenticated agent_id in context
//
// Returns 401 if API key is missing or invalid.
// Returns 403 if API key doesn't match agent.
//
// Example:
//
//	agentAuth := middleware.NewAgentAuth(database)
//	router.POST("/agents/register", agentAuth.RequireAPIKey(), handler)
func (a *AgentAuth) RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from header
		apiKey := c.GetHeader("X-Agent-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Missing API key",
				"details": "X-Agent-API-Key header is required for agent authentication",
			})
			c.Abort()
			return
		}

		// Validate API key format
		if err := auth.ValidateAPIKeyFormat(apiKey); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid API key format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Extract agent_id from query parameter or path parameter
		agentID := c.Query("agent_id")
		if agentID == "" {
			agentID = c.Param("agent_id")
		}

		// For registration endpoint, agent_id is in request body
		if agentID == "" {
			// Try to parse from JSON body
			var body struct {
				AgentID string `json:"agentId"`
			}
			if err := c.ShouldBindJSON(&body); err == nil {
				agentID = body.AgentID
				// Re-bind the body for downstream handlers
				c.Set("gin.body.buffer", c.Request.Body)
			}
		}

		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing agent_id",
				"details": "agent_id must be provided in query parameter, path parameter, or request body",
			})
			c.Abort()
			return
		}

		// Look up agent in database
		var apiKeyHash sql.NullString
		var agentIDFromDB string
		err := a.database.DB().QueryRow(`
			SELECT agent_id, api_key_hash
			FROM agents
			WHERE agent_id = $1
		`, agentID).Scan(&agentIDFromDB, &apiKeyHash)

		if err == sql.ErrNoRows {
			// ISSUE #226 FIX: Check if using bootstrap key for first-time registration
			// This allows agents to self-register without requiring manual database provisioning
			bootstrapKey := os.Getenv("AGENT_BOOTSTRAP_KEY")
			if bootstrapKey != "" && apiKey == bootstrapKey {
				// Bootstrap key matches - allow first-time registration
				log.Printf("[AgentAuth] Agent %s using bootstrap key for first-time registration from IP %s", agentID, c.ClientIP())
				c.Set("isBootstrapAuth", true)
				c.Set("agentAPIKey", apiKey) // Pass API key to handler for hashing/storage
				c.Set("authenticated_agent_id", agentID)
				c.Set("auth_method", "bootstrap_key")
				c.Next()
				return
			}

			// No bootstrap key or key doesn't match - reject
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Agent not found",
				"details": "Agent must be pre-registered with an API key before connecting, or use a valid bootstrap key for first-time registration",
				"agentId": agentID,
			})
			c.Abort()
			return
		}

		if err != nil {
			log.Printf("[AgentAuth] Database error looking up agent %s: %v", agentID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Database error",
				"details": "Failed to validate agent credentials",
			})
			c.Abort()
			return
		}

		// Check if agent has an API key configured
		if !apiKeyHash.Valid || apiKeyHash.String == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "No API key configured",
				"details": "Agent API key has not been generated. Contact administrator.",
				"agentId": agentID,
			})
			c.Abort()
			return
		}

		// Compare provided API key against stored hash
		if !auth.CompareAPIKey(apiKey, apiKeyHash.String) {
			log.Printf("[AgentAuth] Invalid API key for agent %s from IP %s", agentID, c.ClientIP())
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Invalid API key",
				"details": "The provided API key does not match the agent's registered key",
				"agentId": agentID,
			})
			c.Abort()
			return
		}

		// Update last used timestamp
		now := time.Now()
		_, err = a.database.DB().Exec(`
			UPDATE agents
			SET api_key_last_used_at = $1, updated_at = $1
			WHERE agent_id = $2
		`, now, agentID)

		if err != nil {
			log.Printf("[AgentAuth] Failed to update api_key_last_used_at for agent %s: %v", agentID, err)
			// Don't fail the request, just log the error
		}

		// Set authenticated agent_id in context for downstream handlers
		c.Set("authenticated_agent_id", agentID)
		c.Set("auth_method", "agent_api_key")

		// Set audit log metadata (picked up by audit logging middleware)
		c.Set("userID", "agent:"+agentID) // Prefix with "agent:" to distinguish from regular users
		c.Set("username", agentID)
		c.Set("audit_metadata", map[string]interface{}{
			"agent_id":     agentID,
			"auth_method":  "agent_api_key",
			"auth_success": true,
		})

		log.Printf("[AgentAuth] Agent %s authenticated successfully from IP %s", agentID, c.ClientIP())

		c.Next()
	}
}

// OptionalAPIKey returns a middleware that accepts but does not require an API key.
//
// This is useful for endpoints that should work with or without authentication.
// If a valid API key is provided, the agent_id is set in the context.
// If no API key or invalid key, the request continues without authentication.
//
// Example:
//
//	agentAuth := middleware.NewAgentAuth(database)
//	router.POST("/agents/heartbeat", agentAuth.OptionalAPIKey(), handler)
func (a *AgentAuth) OptionalAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from header
		apiKey := c.GetHeader("X-Agent-API-Key")
		if apiKey == "" {
			// No API key provided, continue without auth
			c.Next()
			return
		}

		// Validate API key format
		if err := auth.ValidateAPIKeyFormat(apiKey); err != nil {
			// Invalid format, continue without auth (don't block the request)
			c.Next()
			return
		}

		// Extract agent_id
		agentID := c.Query("agent_id")
		if agentID == "" {
			agentID = c.Param("agent_id")
		}

		if agentID == "" {
			c.Next()
			return
		}

		// Look up agent and validate
		var apiKeyHash sql.NullString
		err := a.database.DB().QueryRow(`
			SELECT api_key_hash FROM agents WHERE agent_id = $1
		`, agentID).Scan(&apiKeyHash)

		if err != nil || !apiKeyHash.Valid {
			c.Next()
			return
		}

		if auth.CompareAPIKey(apiKey, apiKeyHash.String) {
			// Valid API key, set in context
			c.Set("authenticated_agent_id", agentID)
			c.Set("auth_method", "agent_api_key")
			log.Printf("[AgentAuth] Agent %s authenticated (optional) from IP %s", agentID, c.ClientIP())
		}

		c.Next()
	}
}

// RequireAuth returns a middleware that requires agent authentication via mTLS OR API key.
//
// This is a hybrid middleware that supports both authentication methods:
//   - If client certificate is present (mTLS): validates cert and extracts agent_id from CN
//   - If no client certificate: falls back to API key authentication
//
// Authentication flow:
//   1. Check for client certificate in TLS connection
//   2. If cert present: validate against CA, extract agent_id from CN
//   3. If no cert: require X-Agent-API-Key header and validate
//   4. Set authenticated_agent_id in context
//
// Example:
//
//	agentAuth := middleware.NewAgentAuth(database)
//	router.POST("/agents/register", agentAuth.RequireAuth(), handler)
func (a *AgentAuth) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try mTLS first (if client certificate is present)
		if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
			// Client certificate present - use mTLS authentication
			cert := c.Request.TLS.PeerCertificates[0]

			// Extract agent_id from certificate Common Name (CN)
			agentID := cert.Subject.CommonName
			if agentID == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":   "Invalid client certificate",
					"details": "Certificate Common Name (CN) is empty - must be agent_id",
				})
				c.Abort()
				return
			}

			// Verify agent exists in database
			var exists bool
			err := a.database.DB().QueryRow(`
				SELECT EXISTS(SELECT 1 FROM agents WHERE agent_id = $1)
			`, agentID).Scan(&exists)

			if err != nil {
				log.Printf("[AgentAuth] Database error validating agent %s: %v", agentID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Database error",
					"details": "Failed to validate agent credentials",
				})
				c.Abort()
				return
			}

			if !exists {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "Agent not found",
					"details": "Agent must be pre-registered before connecting",
					"agentId": agentID,
				})
				c.Abort()
				return
			}

			// Certificate is valid and agent exists
			c.Set("authenticated_agent_id", agentID)
			c.Set("auth_method", "mtls")

			// Set audit log metadata
			c.Set("userID", "agent:"+agentID)
			c.Set("username", agentID)
			c.Set("audit_metadata", map[string]interface{}{
				"agent_id":     agentID,
				"auth_method":  "mtls",
				"auth_success": true,
				"cert_cn":      cert.Subject.CommonName,
			})

			log.Printf("[AgentAuth] Agent %s authenticated via mTLS (cert CN: %s) from IP %s",
				agentID, cert.Subject.CommonName, c.ClientIP())

			c.Next()
			return
		}

		// No client certificate - fall back to API key authentication
		apiKey := c.GetHeader("X-Agent-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Missing authentication",
				"details": "Either client certificate (mTLS) or X-Agent-API-Key header is required",
			})
			c.Abort()
			return
		}

		// Validate API key format
		if err := auth.ValidateAPIKeyFormat(apiKey); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid API key format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Extract agent_id
		agentID := c.Query("agent_id")
		if agentID == "" {
			agentID = c.Param("agent_id")
		}
		if agentID == "" {
			var body struct {
				AgentID string `json:"agentId"`
			}
			if err := c.ShouldBindJSON(&body); err == nil {
				agentID = body.AgentID
			}
		}

		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing agent_id",
				"details": "agent_id must be provided",
			})
			c.Abort()
			return
		}

		// Look up agent and validate API key
		var apiKeyHash sql.NullString
		err := a.database.DB().QueryRow(`
			SELECT api_key_hash FROM agents WHERE agent_id = $1
		`, agentID).Scan(&apiKeyHash)

		if err == sql.ErrNoRows {
			// ISSUE #226 FIX: Check if using bootstrap key for first-time registration
			bootstrapKey := os.Getenv("AGENT_BOOTSTRAP_KEY")
			if bootstrapKey != "" && apiKey == bootstrapKey {
				log.Printf("[AgentAuth] Agent %s using bootstrap key for first-time registration from IP %s", agentID, c.ClientIP())
				c.Set("isBootstrapAuth", true)
				c.Set("agentAPIKey", apiKey)
				c.Set("authenticated_agent_id", agentID)
				c.Set("auth_method", "bootstrap_key")
				c.Next()
				return
			}

			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Agent not found",
				"details": "Agent must be pre-registered or use a valid bootstrap key",
				"agentId": agentID,
			})
			c.Abort()
			return
		}

		if err != nil {
			log.Printf("[AgentAuth] Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Database error",
			})
			c.Abort()
			return
		}

		if !apiKeyHash.Valid || !auth.CompareAPIKey(apiKey, apiKeyHash.String) {
			log.Printf("[AgentAuth] Invalid API key for agent %s from %s", agentID, c.ClientIP())
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Update last used timestamp
		a.database.DB().Exec(`
			UPDATE agents SET api_key_last_used_at = $1, updated_at = $1
			WHERE agent_id = $2
		`, time.Now(), agentID)

		c.Set("authenticated_agent_id", agentID)
		c.Set("auth_method", "agent_api_key")

		// Set audit log metadata
		c.Set("userID", "agent:"+agentID)
		c.Set("username", agentID)
		c.Set("audit_metadata", map[string]interface{}{
			"agent_id":     agentID,
			"auth_method":  "agent_api_key",
			"auth_success": true,
		})

		log.Printf("[AgentAuth] Agent %s authenticated via API key from %s", agentID, c.ClientIP())

		c.Next()
	}
}
