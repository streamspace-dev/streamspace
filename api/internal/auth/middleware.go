// Package auth provides authentication and authorization mechanisms for StreamSpace.
// This file implements Gin middleware for JWT token validation and role-based access control.
//
// MIDDLEWARE COMPONENTS:
// - JWT authentication middleware (required authentication)
// - Optional authentication middleware (authentication not required)
// - Role-based authorization middleware (require specific roles)
// - Helper functions for extracting user context
//
// AUTHENTICATION FLOW:
//
// 1. Client Request:
//    - Client includes JWT token in Authorization header
//    - Format: "Authorization: Bearer <token>"
//    - Example: "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
//
// 2. Token Extraction:
//    - Middleware extracts token from Authorization header
//    - Validates header format (must start with "Bearer ")
//    - Rejects requests with missing or malformed headers
//
// 3. Token Validation:
//    - Validates JWT signature using secret key
//    - Checks token expiration (exp claim)
//    - Verifies token issuer (iss claim)
//    - Ensures algorithm is HMAC (prevents algorithm substitution)
//
// 4. User Validation:
//    - Extracts user ID from validated token claims
//    - Queries database to verify user still exists
//    - Checks if user account is active (not disabled)
//    - Rejects requests from disabled or deleted users
//
// 5. Context Population:
//    - Stores user information in Gin context
//    - Available to downstream handlers via c.Get()
//    - Includes: userID, username, email, role, groups
//
// MIDDLEWARE TYPES:
//
// 1. Middleware (Required Authentication):
//    - Rejects requests without valid JWT token
//    - Returns 401 Unauthorized for invalid/missing tokens
//    - Returns 403 Forbidden for disabled accounts
//    - Use for protected API endpoints
//
// 2. OptionalAuth (Optional Authentication):
//    - Accepts requests with or without token
//    - Validates token if present, ignores if absent
//    - Useful for endpoints that behave differently for authenticated users
//    - Example: Public catalog with favorites for logged-in users
//
// 3. RequireRole (Role-Based Authorization):
//    - Requires specific role (admin, operator, user)
//    - Must be used after Middleware (requires authentication)
//    - Returns 403 Forbidden if user lacks required role
//
// 4. RequireAnyRole (Multi-Role Authorization):
//    - Accepts any of multiple roles
//    - Example: RequireAnyRole("admin", "operator") for management endpoints
//    - Returns 403 if user has none of the allowed roles
//
// SECURITY FEATURES:
//
// - Token signature validation prevents tampering
// - Expiration checking limits token lifetime
// - Active user validation prevents disabled account access
// - Role checking enforces principle of least privilege
// - Context isolation prevents request cross-contamination
//
// SECURITY CONSIDERATIONS:
//
// 1. Token Transmission:
//    - Tokens must be sent over HTTPS in production
//    - Never log or expose tokens in error messages
//    - Clear tokens from memory after use
//
// 2. Account Status:
//    - Always check user.Active before allowing access
//    - Disabled accounts cannot authenticate even with valid token
//    - Supports immediate access revocation
//
// 3. Token Expiration:
//    - Tokens expire after configured duration (default: 24 hours)
//    - Expired tokens rejected with 401 Unauthorized
//    - Forces periodic re-authentication
//
// 4. Role Validation:
//    - Roles stored in JWT claims (tamper-proof via signature)
//    - Role hierarchy: admin > operator > user
//    - Always validate role before privileged operations
//
// EXAMPLE USAGE:
//
//   // Require authentication for all /api routes
//   api := router.Group("/api")
//   api.Use(auth.Middleware(jwtManager, userDB))
//   {
//       api.GET("/sessions", listSessions)  // Requires valid token
//   }
//
//   // Admin-only endpoints
//   admin := api.Group("/admin")
//   admin.Use(auth.RequireRole("admin"))
//   {
//       admin.GET("/users", listAllUsers)  // Requires admin role
//   }
//
//   // Optional authentication (public + user features)
//   router.GET("/catalog", auth.OptionalAuth(jwtManager, userDB), showCatalog)
//
//   // Extract user info in handler
//   func listSessions(c *gin.Context) {
//       userID, _ := auth.GetUserID(c)
//       role, _ := auth.GetUserRole(c)
//       // ... use user info
//   }
//
// CONTEXT KEYS:
//
// The middleware stores the following keys in Gin context:
// - "userID": string - Unique user identifier
// - "username": string - Username for display
// - "userEmail": string - User's email address
// - "userRole": string - Role (admin, operator, user)
// - "userGroups": []string - Group memberships
// - "claims": *Claims - Full JWT claims object
//
// THREAD SAFETY:
//
// All middleware functions are thread-safe and can handle concurrent requests.
// Each request gets its own Gin context, preventing data leakage between requests.
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
)

// Middleware creates an authentication middleware that validates JWT tokens
// and ensures user accounts are active.
//
// WEBSOCKET HANDLING:
// WebSocket upgrade requests receive special treatment to maintain protocol compatibility:
// - Detected by checking Upgrade=websocket and Connection=Upgrade headers
// - On auth failure: Returns status code only (no JSON body) via AbortWithStatus
// - Rationale: WebSocket upgrader expects clean HTTP responses without body content
// - Standard requests: Returns JSON error messages as usual
//
// This dual-response approach was added to fix WebSocket connection issues where
// JSON error responses would interfere with the WebSocket handshake protocol.
func Middleware(jwtManager *JWTManager, userDB *db.UserDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a WebSocket upgrade request
		// WebSocket requests need special error handling (status code only, no JSON body)
		// Note: Connection header may contain multiple values like "keep-alive, Upgrade"
		upgrade := strings.ToLower(c.GetHeader("Upgrade"))
		connection := strings.ToLower(c.GetHeader("Connection"))
		isWebSocket := upgrade == "websocket" && strings.Contains(connection, "upgrade")

		var tokenString string

		// For WebSocket connections, try query parameter first (browsers can't send custom headers)
		if isWebSocket {
			tokenString = c.Query("token")
		}

		// If no token from query parameter, try Authorization header
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				// For WebSocket, abort without writing response (let upgrader handle it)
				if isWebSocket {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Authorization header required",
				})
				c.Abort()
				return
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				if isWebSocket {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid authorization header format. Use: Bearer <token>",
				})
				c.Abort()
				return
			}

			tokenString = parts[1]
		}

		// Validate token
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			if isWebSocket {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid or expired token",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate session exists in Redis (server-side session tracking)
		// This ensures tokens can be invalidated on logout or server restart
		if claims.ID != "" {
			valid, err := jwtManager.ValidateSession(c.Request.Context(), claims.ID)
			if err != nil || !valid {
				if isWebSocket {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Session expired or invalidated",
				})
				c.Abort()
				return
			}
		}

		// Verify user still exists and is active
		user, err := userDB.GetUser(c.Request.Context(), claims.UserID)
		if err != nil {
			if isWebSocket {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
			})
			c.Abort()
			return
		}

		if !user.Active {
			if isWebSocket {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.JSON(http.StatusForbidden, gin.H{
				"error": "User account is disabled",
			})
			c.Abort()
			return
		}

		// Set user info in context for handlers to use
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)
		c.Set("userGroups", claims.Groups)
		c.Set("claims", claims)
		c.Set("sessionID", claims.ID) // For logout/session management

		c.Next()
	}
}

// OptionalAuth middleware allows both authenticated and unauthenticated requests
func OptionalAuth(jwtManager *JWTManager, userDB *db.UserDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No authentication, continue without user context
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// Validate session exists in Redis
		if claims.ID != "" {
			valid, err := jwtManager.ValidateSession(c.Request.Context(), claims.ID)
			if err != nil || !valid {
				// Session invalid, continue without user context
				c.Next()
				return
			}
		}

		// Set user info if valid
		user, err := userDB.GetUser(c.Request.Context(), claims.UserID)
		if err == nil && user.Active {
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("userEmail", claims.Email)
			c.Set("userRole", claims.Role)
			c.Set("userGroups", claims.Groups)
			c.Set("sessionID", claims.ID)
		}

		c.Next()
	}
}

// RequireRole middleware requires a specific role
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("userRole")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userRole, ok := role.(string)
		if !ok || userRole != requiredRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware requires one of multiple roles
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("userRole")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userRole, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid role",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		for _, requiredRole := range roles {
			if userRole == requiredRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "Insufficient permissions",
		})
		c.Abort()
	}
}

// GetUserID extracts the user ID from the Gin context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	id, ok := userID.(string)
	return id, ok
}

// GetUsername extracts the username from the Gin context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}

// GetUserRole extracts the user role from the Gin context
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("userRole")
	if !exists {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	role, ok := GetUserRole(c)
	return ok && role == "admin"
}

// IsOperator checks if the current user is an operator or admin
func IsOperator(c *gin.Context) bool {
	role, ok := GetUserRole(c)
	return ok && (role == "admin" || role == "operator")
}
