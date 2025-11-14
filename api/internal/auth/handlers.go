package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	userDB     *db.UserDB
	jwtManager *JWTManager
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userDB *db.UserDB, jwtManager *JWTManager) *AuthHandler {
	return &AuthHandler{
		userDB:     userDB,
		jwtManager: jwtManager,
	}
}

// RegisterRoutes registers authentication routes
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/logout", h.Logout)
		auth.GET("/saml/login", h.SAMLLogin)
		auth.POST("/saml/acs", h.SAMLCallback)
		auth.GET("/saml/metadata", h.SAMLMetadata)
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string         `json:"token"`
	ExpiresAt time.Time      `json:"expiresAt"`
	User      *models.User   `json:"user"`
}

// Login handles local authentication
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Verify password
	user, err := h.userDB.VerifyPassword(ctx, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	// Check if user is active
	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Account is disabled",
		})
		return
	}

	// Get user groups
	groups, err := h.userDB.GetUserGroups(ctx, user.ID)
	if err != nil {
		groups = []string{} // Continue without groups if error
	}

	groupIDs := make([]string, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.GroupID
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.Email, user.Role, groupIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate token",
			"message": err.Error(),
		})
		return
	}

	// Calculate expiration
	expiresAt := time.Now().Add(h.jwtManager.config.TokenDuration)

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

	expiresAt := time.Now().Add(h.jwtManager.config.TokenDuration)

	c.JSON(http.StatusOK, LoginResponse{
		Token:     newToken,
		ExpiresAt: expiresAt,
		User:      user,
	})
}

// Logout handles logout (client-side token invalidation)
func (h *AuthHandler) Logout(c *gin.Context) {
	// With JWT, logout is primarily client-side (remove token)
	// Could implement token blacklist here if needed
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// SAMLLogin initiates SAML authentication flow
func (h *AuthHandler) SAMLLogin(c *gin.Context) {
	// TODO: Implement SAML authentication flow
	// This would redirect to the SAML IdP
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "SAML authentication not yet implemented",
	})
}

// SAMLCallback handles SAML assertion callback
func (h *AuthHandler) SAMLCallback(c *gin.Context) {
	// TODO: Implement SAML callback handling
	// 1. Validate SAML assertion
	// 2. Extract user attributes (email, name, groups)
	// 3. Create or update user in database
	// 4. Generate JWT token
	// 5. Return token to client

	// Example structure:
	/*
		var assertion SAMLAssertion
		// Parse and validate SAML assertion

		// Extract user info
		email := assertion.Email
		fullName := assertion.FullName
		groups := assertion.Groups

		// Get or create user
		user, err := h.userDB.GetOrCreateSAMLUser(ctx, email, fullName, groups)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process SAML user"})
			return
		}

		// Generate token
		token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.Email, user.Role, user.Groups)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, LoginResponse{Token: token, User: user})
	*/

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "SAML callback not yet implemented",
	})
}

// SAMLMetadata returns SAML service provider metadata
func (h *AuthHandler) SAMLMetadata(c *gin.Context) {
	// TODO: Generate and return SAML SP metadata XML
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "SAML metadata endpoint not yet implemented",
	})
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
	updateReq := &models.UpdateUserRequest{
		Password: &req.NewPassword,
	}

	if err := h.userDB.UpdateUser(ctx, user.ID, updateReq); err != nil {
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
