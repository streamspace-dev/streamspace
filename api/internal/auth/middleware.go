package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// Middleware creates an authentication middleware
func Middleware(jwtManager *JWTManager, userDB *db.UserDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid or expired token",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Verify user still exists and is active
		user, err := userDB.GetUser(context.Background(), claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
			})
			c.Abort()
			return
		}

		if !user.Active {
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

		// Set user info if valid
		user, err := userDB.GetUser(context.Background(), claims.UserID)
		if err == nil && user.Active {
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("userEmail", claims.Email)
			c.Set("userRole", claims.Role)
			c.Set("userGroups", claims.Groups)
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
