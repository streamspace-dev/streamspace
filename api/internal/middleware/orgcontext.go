// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements organization context extraction and enforcement for multi-tenancy.
//
// SECURITY: This middleware is CRITICAL for preventing cross-tenant data access.
// All protected routes MUST use this middleware to ensure org_id is available
// in the request context for database query filtering.
//
// Multi-Tenancy Architecture:
//   - Each user belongs to exactly one organization (org_id)
//   - org_id is embedded in JWT claims during authentication
//   - This middleware extracts org_id from JWT and adds to request context
//   - All handlers MUST use GetOrgID() to filter database queries
//   - Requests without valid org_id are rejected with 401 Unauthorized
//
// Context Keys:
//   - "org_id": Organization ID (string)
//   - "org_name": Organization display name (string)
//   - "k8s_namespace": Kubernetes namespace for this org (string)
//   - "org_role": User's role within the org (string)
//   - "user_id": User's unique ID (string)
//   - "username": User's username (string)
//   - "role": User's system-wide role (string)
//
// Usage:
//
//	// Apply middleware to protected routes
//	protected := router.Group("/api/v1")
//	protected.Use(middleware.OrgContextMiddleware(jwtManager))
//
//	// In handler, extract org_id for filtering
//	func MyHandler(c *gin.Context) {
//	    orgID, err := middleware.GetOrgID(c)
//	    if err != nil {
//	        c.JSON(401, gin.H{"error": "unauthorized"})
//	        return
//	    }
//	    // Use orgID to filter database queries
//	    sessions, err := db.ListSessionsByOrg(ctx, orgID)
//	}
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/auth"
)

// Context keys for org-scoped data
const (
	// ContextKeyOrgID is the key for organization ID in request context
	ContextKeyOrgID = "org_id"

	// ContextKeyOrgName is the key for organization name in request context
	ContextKeyOrgName = "org_name"

	// ContextKeyK8sNamespace is the key for Kubernetes namespace in request context
	ContextKeyK8sNamespace = "k8s_namespace"

	// ContextKeyOrgRole is the key for user's org role in request context
	ContextKeyOrgRole = "org_role"

	// ContextKeyUserID is the key for user ID in request context
	ContextKeyUserID = "user_id"

	// ContextKeyUsername is the key for username in request context
	ContextKeyUsername = "username"

	// ContextKeyRole is the key for system role in request context
	ContextKeyRole = "role"

	// ContextKeySessionID is the key for JWT session ID in request context
	ContextKeySessionID = "session_id"
)

// OrgContextMiddleware extracts organization context from JWT claims and
// populates it in the request context for use by handlers.
//
// SECURITY: This middleware is CRITICAL for multi-tenancy isolation.
// All protected routes MUST use this middleware.
//
// The middleware:
//  1. Extracts JWT from Authorization header (Bearer token)
//  2. Validates the JWT signature and expiration
//  3. Extracts org_id and other claims
//  4. Populates claims in request context
//  5. Rejects requests without valid org_id
//
// Request Flow:
//
//	Client -> [Bearer Token] -> OrgContextMiddleware -> [org_id in context] -> Handler
//
// Error Responses:
//   - 401 Unauthorized: Missing, invalid, or expired token
//   - 401 Unauthorized: Token missing org_id claim
func OrgContextMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Validate Bearer prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid authorization header format (expected: Bearer <token>)",
			})
			c.Abort()
			return
		}

		// Extract token string
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Token required",
			})
			c.Abort()
			return
		}

		// Validate token and extract claims
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired token",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// SECURITY: Require org_id in claims for multi-tenancy
		// Tokens without org_id are rejected to prevent cross-tenant access
		if claims.OrgID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Token missing organization context (org_id)",
			})
			c.Abort()
			return
		}

		// Populate org context in request context
		// Handlers use these values to filter database queries
		c.Set(ContextKeyOrgID, claims.OrgID)
		c.Set(ContextKeyOrgName, claims.OrgName)
		c.Set(ContextKeyK8sNamespace, claims.K8sNamespace)
		c.Set(ContextKeyOrgRole, claims.OrgRole)

		// Populate user context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)

		// Populate session ID for session tracking
		c.Set(ContextKeySessionID, claims.ID) // JWT ID (jti) is the session ID

		c.Next()
	}
}

// GetOrgID extracts the organization ID from the request context.
// Returns error if org_id is not present (middleware not applied or token invalid).
//
// SECURITY: Always use this function to get org_id for database queries.
// Never trust client-provided org_id values.
func GetOrgID(c *gin.Context) (string, error) {
	orgID, exists := c.Get(ContextKeyOrgID)
	if !exists {
		return "", ErrMissingOrgContext
	}
	orgIDStr, ok := orgID.(string)
	if !ok || orgIDStr == "" {
		return "", ErrMissingOrgContext
	}
	return orgIDStr, nil
}

// GetK8sNamespace extracts the Kubernetes namespace from the request context.
// Returns the org's K8s namespace for scoping WebSocket and K8s operations.
func GetK8sNamespace(c *gin.Context) (string, error) {
	ns, exists := c.Get(ContextKeyK8sNamespace)
	if !exists {
		return "", ErrMissingOrgContext
	}
	nsStr, ok := ns.(string)
	if !ok || nsStr == "" {
		// Default to "streamspace" if not set
		return "streamspace", nil
	}
	return nsStr, nil
}

// GetUserID extracts the user ID from the request context.
func GetUserID(c *gin.Context) (string, error) {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return "", ErrMissingUserContext
	}
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return "", ErrMissingUserContext
	}
	return userIDStr, nil
}

// GetOrgRole extracts the user's org role from the request context.
func GetOrgRole(c *gin.Context) (string, error) {
	role, exists := c.Get(ContextKeyOrgRole)
	if !exists {
		return "", ErrMissingOrgContext
	}
	roleStr, ok := role.(string)
	if !ok {
		return "", ErrMissingOrgContext
	}
	return roleStr, nil
}

// GetRole extracts the user's system role from the request context.
func GetRole(c *gin.Context) (string, error) {
	role, exists := c.Get(ContextKeyRole)
	if !exists {
		return "", ErrMissingUserContext
	}
	roleStr, ok := role.(string)
	if !ok {
		return "", ErrMissingUserContext
	}
	return roleStr, nil
}

// MustGetOrgID extracts org_id from context, panics if not present.
// Use only in handlers where OrgContextMiddleware is guaranteed to run.
func MustGetOrgID(c *gin.Context) string {
	orgID, err := GetOrgID(c)
	if err != nil {
		panic("MustGetOrgID: " + err.Error())
	}
	return orgID
}

// RequireOrgRole checks if the user has one of the required org roles.
// Returns gin.HandlerFunc that can be used as route-level middleware.
//
// Usage:
//
//	router.GET("/admin", RequireOrgRole("org_admin"), adminHandler)
//	router.GET("/manage", RequireOrgRole("org_admin", "maintainer"), manageHandler)
func RequireOrgRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, err := GetOrgRole(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Organization context required",
			})
			c.Abort()
			return
		}

		// Check if user has one of the allowed roles
		for _, role := range allowedRoles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "Insufficient permissions",
			"required_roles": allowedRoles,
			"your_role":      userRole,
		})
		c.Abort()
	}
}

// ErrMissingOrgContext indicates org_id is not in request context
var ErrMissingOrgContext = &OrgContextError{message: "organization context not found in request"}

// ErrMissingUserContext indicates user_id is not in request context
var ErrMissingUserContext = &OrgContextError{message: "user context not found in request"}

// OrgContextError represents an error extracting org context
type OrgContextError struct {
	message string
}

func (e *OrgContextError) Error() string {
	return e.message
}
