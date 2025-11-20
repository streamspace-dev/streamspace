// Package auth provides authentication and authorization mechanisms for StreamSpace.
// This file implements HTTP handlers for authentication endpoints including local,
// SAML, and password management operations.
//
// AUTHENTICATION HANDLERS:
// - Local authentication (username/password)
// - SAML SSO authentication (enterprise identity providers)
// - Token refresh (JWT token renewal)
// - Password change (local users only)
// - Logout (session termination)
//
// SUPPORTED AUTHENTICATION FLOWS:
//
// 1. Local Authentication (POST /auth/login):
//   - User submits username and password
//   - System verifies credentials against database
//   - Returns JWT token for subsequent requests
//   - Supports account status validation (active/disabled)
//
// 2. SAML SSO Authentication (GET /auth/saml/login):
//   - User initiates SSO flow
//   - Redirects to enterprise IdP (Okta, Azure AD, etc.)
//   - IdP sends SAML assertion after authentication
//   - System validates assertion and creates local session
//   - Returns JWT token for API access
//
// 3. Token Refresh (POST /auth/refresh):
//   - Client submits existing JWT token
//   - System validates token is within refresh window
//   - Issues new token with extended expiration
//   - Prevents indefinite token refresh (7-day window)
//
// 4. Password Change (POST /auth/password):
//   - Local users can change their password
//   - Requires current password verification
//   - Not available for SSO users (SAML/OIDC)
//
// SECURITY FEATURES:
//
// - Password verification with bcrypt hashing
// - Account status validation (prevent disabled accounts from logging in)
// - JWT token generation with configurable expiration
// - SAML group synchronization for role-based access control
// - Secure cookie handling for SAML return URLs
// - Auto-provisioning of SSO users on first login
//
// SAML GROUP SYNCHRONIZATION:
//
// When users authenticate via SAML, their group memberships are automatically
// synchronized with StreamSpace groups:
// - SAML assertion contains groups claim from IdP
// - System matches SAML group names to local groups
// - Adds user to matching groups (does not remove from other groups by default)
// - Enables role-based access control based on IdP group membership
//
// SECURITY CONSIDERATIONS:
//
// 1. Password Security:
//   - Passwords hashed with bcrypt (cost factor 10+)
//   - Never return password hashes in API responses
//   - Minimum password length enforced (8 characters)
//
// 2. Account Lockout:
//   - Disabled accounts cannot authenticate
//   - Returns 403 Forbidden for disabled accounts
//   - Prevents unauthorized access to suspended accounts
//
// 3. Token Security:
//   - JWT tokens include user ID, role, and groups
//   - Tokens expire after configured duration (default: 24 hours)
//   - Refresh tokens only valid within 7-day window
//   - See jwt.go for detailed token security
//
// 4. SAML Security:
//   - Assertion signatures validated by middleware
//   - Return URLs stored in secure cookies
//   - SAML groups synced on every login
//   - See saml.go for detailed SAML security
//
// EXAMPLE USAGE:
//
//	// Initialize handler with dependencies
//	handler := NewAuthHandler(userDB, jwtManager, samlAuth)
//
//	// Register routes
//	router := gin.Default()
//	handler.RegisterRoutes(router.Group("/api/v1"))
//
//	// Routes will be available at:
//	// - POST /api/v1/auth/login (local authentication)
//	// - POST /api/v1/auth/refresh (token refresh)
//	// - POST /api/v1/auth/logout (logout)
//	// - GET  /api/v1/auth/saml/login (initiate SAML SSO)
//	// - POST /api/v1/auth/saml/acs (SAML callback)
//	// - GET  /api/v1/auth/saml/metadata (SAML SP metadata)
//
// THREAD SAFETY:
//
// All handler methods are thread-safe and can handle concurrent requests.
// Database operations use connection pooling for safe concurrent access.
package auth

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/models"
)

// validateReturnURL validates that a return URL is safe to redirect to.
//
// Security Considerations:
//   - Only allows relative URLs (starting with /)
//   - Prevents protocol-relative URLs (//evil.com)
//   - Prevents URLs with multiple slashes that could be exploited
//   - Returns "/" as safe default if validation fails
//
// This prevents open redirect vulnerabilities where an attacker could
// craft a URL like ?return_url=//evil.com/steal-token to redirect
// users to malicious sites after authentication.
func validateReturnURL(returnURL string) string {
	// Default to home page
	if returnURL == "" {
		return "/"
	}

	// Must start with a single slash (relative path)
	if !strings.HasPrefix(returnURL, "/") {
		return "/"
	}

	// Prevent protocol-relative URLs (//evil.com)
	if strings.HasPrefix(returnURL, "//") {
		return "/"
	}

	// Prevent URLs that could be manipulated
	// e.g., /\evil.com on some servers
	if strings.ContainsAny(returnURL, "\\") {
		return "/"
	}

	// Prevent URLs with scheme-like patterns
	if strings.Contains(returnURL, "://") {
		return "/"
	}

	// Prevent URLs with encoded characters that could be exploited
	// after being decoded by the browser
	if strings.Contains(returnURL, "%2f") || strings.Contains(returnURL, "%2F") {
		return "/"
	}

	return returnURL
}

// UserStore defines the interface for user database operations
type UserStore interface {
	VerifyPassword(ctx context.Context, username, password string) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	GetUserGroups(ctx context.Context, userID string) ([]string, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error)
	UpdateUser(ctx context.Context, userID string, req *models.UpdateUserRequest) error
	UpdatePassword(ctx context.Context, userID, password string) error
	AddUserToGroup(ctx context.Context, userID, groupName string) error
	DB() *sql.DB // Kept for backward compatibility if needed, but ideally should be removed
}

// TokenManager defines the interface for JWT operations
type TokenManager interface {
	GenerateTokenWithContext(ctx context.Context, userID, username, email, role string, groups []string, ipAddress, userAgent string) (string, error)
	RefreshToken(token string) (string, error)
	ValidateToken(token string) (*Claims, error)
	InvalidateSession(ctx context.Context, sessionID string) error
	GetTokenDuration() time.Duration
}

// SAMLService defines the interface for SAML operations
type SAMLService interface {
	GetMiddleware() *samlsp.Middleware
	GetServiceProvider() *saml.ServiceProvider
	ExtractUserFromAssertion(assertion *saml.Assertion) (*UserInfo, error)
}

// AuthHandler handles authentication requests
type AuthHandler struct {
	userDB     UserStore
	jwtManager TokenManager
	samlAuth   SAMLService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userDB UserStore, jwtManager TokenManager, samlAuth SAMLService) *AuthHandler {
	return &AuthHandler{
		userDB:     userDB,
		jwtManager: jwtManager,
		samlAuth:   samlAuth,
	}
}

// RegisterRoutes registers authentication routes
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Note: router is already /api/v1/auth from main.go
	router.POST("/login", h.Login)
	router.POST("/refresh", h.RefreshToken)
	router.POST("/logout", h.Logout)
	router.GET("/saml/login", h.SAMLLogin)
	router.POST("/saml/acs", h.SAMLCallback)
	router.GET("/saml/metadata", h.SAMLMetadata)
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expiresAt"`
	User      *models.User `json:"user"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user, err := h.userDB.VerifyPassword(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Account is disabled",
		})
		return
	}

	// Get user groups
	groupIDs, err := h.userDB.GetUserGroups(c.Request.Context(), user.ID)
	if err != nil {
		groupIDs = []string{} // Continue without groups if error
	}

	// Capture client info for session tracking
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Generate JWT token with session tracking
	token, err := h.jwtManager.GenerateTokenWithContext(c.Request.Context(), user.ID, user.Username, user.Email, user.Role, groupIDs, ipAddress, userAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate token",
			"message": err.Error(),
		})
		return
	}

	// Calculate expiration
	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration())

	// Remove sensitive data
	user.PasswordHash = ""

	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
	})
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Validate and refresh token
	newToken, err := h.jwtManager.RefreshToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid or expired token",
			"message": err.Error(),
		})
		return
	}

	// Get user info from token
	claims, err := h.jwtManager.ValidateToken(newToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate new token",
		})
		return
	}

	// Get full user data
	user, err := h.userDB.GetUser(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user data",
		})
		return
	}

	user.PasswordHash = ""

	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration())

	c.JSON(http.StatusOK, LoginResponse{
		Token:     newToken,
		ExpiresAt: expiresAt,
		User:      user,
	})
}

// Logout handles logout and invalidates the session in Redis
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get session ID from context (set by auth middleware)
	sessionID, exists := c.Get("sessionID")
	if exists && sessionID != nil {
		if sid, ok := sessionID.(string); ok && sid != "" {
			// Invalidate session in Redis
			ctx := c.Request.Context()
			if err := h.jwtManager.InvalidateSession(ctx, sid); err != nil {
				// Log error but don't fail logout
				log.Printf("Warning: Failed to invalidate session %s: %v", sid, err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// SAMLLogin initiates SAML authentication flow
func (h *AuthHandler) SAMLLogin(c *gin.Context) {
	// Check if SAML is configured
	if h.samlAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "SAML authentication is not configured",
		})
		return
	}

	// Store return URL in cookie for post-login redirect
	// SECURITY: Validate return URL to prevent open redirect attacks
	returnURL := validateReturnURL(c.Query("return_url"))

	// Set secure cookie with return URL (1 hour expiration)
	c.SetCookie(
		"saml_return_url",
		returnURL,
		3600,                 // 1 hour max age
		"/",                  // path
		"",                   // domain (empty = current domain)
		c.Request.TLS != nil, // secure (HTTPS only)
		true,                 // httpOnly
	)

	// Initiate SAML authentication flow (redirects to IdP)
	h.samlAuth.GetMiddleware().HandleStartAuthFlow(c.Writer, c.Request)
}

// SAMLCallback handles SAML assertion callback
func (h *AuthHandler) SAMLCallback(c *gin.Context) {
	// Check if SAML is configured
	if h.samlAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "SAML authentication is not configured",
		})
		return
	}

	ctx := c.Request.Context()

	// Extract user info from SAML assertion (middleware sets this in context)
	assertionData, exists := c.Get("saml_assertion")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "No SAML assertion found",
		})
		return
	}

	// Type assert the assertion
	assertion, ok := assertionData.(*saml.Assertion)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid SAML assertion type",
		})
		return
	}

	// Extract user attributes from assertion
	userAttrs, err := h.samlAuth.ExtractUserFromAssertion(assertion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to extract user from SAML assertion",
			"message": err.Error(),
		})
		return
	}

	// Validate required fields
	if userAttrs.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "SAML assertion missing required email attribute",
		})
		return
	}

	// Get or create user in database
	user, err := h.userDB.GetUserByEmail(ctx, userAttrs.Email)
	if err != nil {
		// User doesn't exist, create new SAML user
		fullName := userAttrs.FirstName + " " + userAttrs.LastName
		if fullName == " " {
			fullName = userAttrs.Email // Fallback to email if no name
		}
		createReq := &models.CreateUserRequest{
			Username: userAttrs.Email, // Use email as username
			Email:    userAttrs.Email,
			FullName: fullName,
			Provider: "saml",
			Role:     "user", // Default role
		}

		user, err = h.userDB.CreateUser(ctx, createReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create SAML user",
				"message": err.Error(),
			})
			return
		}
	} else {
		// User exists, update attributes from SAML
		fullName := userAttrs.FirstName + " " + userAttrs.LastName
		if fullName != " " {
			updateReq := &models.UpdateUserRequest{
				FullName: &fullName,
			}
			if err := h.userDB.UpdateUser(ctx, user.ID, updateReq); err != nil {
				// Log error but continue (non-critical)
				log.Printf("Warning: Failed to update user %s from SAML: %v", user.ID, err)
			}
		}
	}

	// Check if user is active
	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Account is disabled",
		})
		return
	}

	// Sync user groups from SAML assertion
	if len(userAttrs.Groups) > 0 {
		err := h.syncSAMLGroups(ctx, user.ID, userAttrs.Groups)
		if err != nil {
			// Log error but don't fail authentication
			log.Printf("Warning: Failed to sync SAML groups for user %s: %v", user.ID, err)
		}
	}

	// Get user groups for JWT
	groupIDs, err := h.userDB.GetUserGroups(ctx, user.ID)
	if err != nil {
		groupIDs = []string{} // Continue without groups if error
	}

	// Capture client info for session tracking
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Generate JWT token with session tracking
	token, err := h.jwtManager.GenerateTokenWithContext(ctx, user.ID, user.Username, user.Email, user.Role, groupIDs, ipAddress, userAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate token",
			"message": err.Error(),
		})
		return
	}

	// Calculate expiration
	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration())

	// Remove sensitive data
	user.PasswordHash = ""

	// Get return URL from cookie
	returnURL, err := c.Cookie("saml_return_url")
	if err != nil || returnURL == "" {
		returnURL = "/"
	}

	// Clear the cookie
	c.SetCookie("saml_return_url", "", -1, "/", "", c.Request.TLS != nil, true)

	// Return token and user info
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiresAt,
		"user":      user,
		"returnUrl": returnURL,
	})
}

// SAMLMetadata returns SAML service provider metadata
func (h *AuthHandler) SAMLMetadata(c *gin.Context) {
	// Check if SAML is configured
	if h.samlAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "SAML authentication is not configured",
		})
		return
	}

	// Get service provider from SAML authenticator
	sp := h.samlAuth.GetServiceProvider()
	if sp == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "SAML service provider not initialized",
		})
		return
	}

	// Generate metadata XML
	metadata := sp.Metadata()

	// Marshal to XML bytes
	metadataBytes, err := xml.Marshal(metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to marshal metadata: %v", err),
		})
		return
	}

	// Return XML with proper content type
	c.Header("Content-Type", "application/samlmetadata+xml")
	c.String(http.StatusOK, string(metadataBytes))
}

// PasswordChangeRequest represents a password change request
type PasswordChangeRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8"`
}

// ChangePassword handles password changes for local users
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req PasswordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Get current user from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	ctx := c.Request.Context()

	// Get user
	user, err := h.userDB.GetUser(ctx, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user",
		})
		return
	}

	// Verify user is local provider (not SAML/OIDC)
	if user.Provider != "local" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password change not available for SSO users",
		})
		return
	}

	// Verify old password
	_, err = h.userDB.VerifyPassword(ctx, user.Username, req.OldPassword)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid current password",
		})
		return
	}

	// Update password
	if err := h.userDB.UpdatePassword(ctx, user.ID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update password",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password updated successfully",
	})
}

// syncSAMLGroups synchronizes user's group memberships based on SAML assertion
func (h *AuthHandler) syncSAMLGroups(ctx context.Context, userID string, samlGroups []string) error {
	// For each SAML group, find matching local group and ensure membership
	for _, samlGroupName := range samlGroups {
		err := h.userDB.AddUserToGroup(ctx, userID, samlGroupName)
		if err != nil {
			// Log error but continue with other groups
			// We don't fail the whole sync if one group fails (e.g. group doesn't exist)
			log.Printf("Warning: Failed to add user %s to group %s: %v", userID, samlGroupName, err)
		} else {
			log.Printf("Added user %s to group %s (from SAML)", userID, samlGroupName)
		}
	}

	return nil
}
