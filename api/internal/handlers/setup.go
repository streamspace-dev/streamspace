// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements the first-run setup wizard for admin user onboarding.
//
// Purpose:
// - Provides a secure setup wizard for initial admin password configuration
// - Enables account recovery when admin password is lost or not set
// - Automatically disables after admin account is configured
// - Works as fallback when Helm secret or environment variable not available
//
// Security Features:
// - Only accessible when admin account has no password set
// - Password strength validation (minimum 12 characters)
// - Password confirmation to prevent typos
// - Email validation for admin contact
// - Single-use wizard (auto-disables after setup)
// - Atomic database transaction
// - Input sanitization and validation
//
// Integration:
// - Part of multi-layered admin onboarding strategy
// - Priority 3 fallback after Helm secret and environment variable
// - Works with database migration admin user creation
// - Compatible with all authentication modes (local, SAML, OIDC)
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - No shared mutable state between requests
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// SetupHandler handles initial admin setup wizard
type SetupHandler struct {
	DB *db.Database
}

// NewSetupHandler creates a new setup handler
func NewSetupHandler(database *db.Database) *SetupHandler {
	return &SetupHandler{
		DB: database,
	}
}

// ============================================================================
// SETUP WIZARD STATUS
// ============================================================================

// SetupStatusResponse returns whether the setup wizard should be displayed
type SetupStatusResponse struct {
	SetupRequired bool   `json:"setupRequired"`
	AdminExists   bool   `json:"adminExists"`
	HasPassword   bool   `json:"hasPassword"`
	Message       string `json:"message,omitempty"`
}

// GetSetupStatus checks if the setup wizard should be enabled
// GET /api/v1/auth/setup/status
//
// Returns:
//   200 OK: Setup status information
//   500 Internal Server Error: Database error
//
// Response body:
//   {
//     "setupRequired": true/false,
//     "adminExists": true/false,
//     "hasPassword": true/false,
//     "message": "Setup wizard is enabled/disabled"
//   }
func (h *SetupHandler) GetSetupStatus(c *gin.Context) {
	setupRequired, adminExists, hasPassword := h.isSetupRequired()

	// Debug logging
	fmt.Printf("DEBUG GetSetupStatus: setupRequired=%v, adminExists=%v, hasPassword=%v\n",
		setupRequired, adminExists, hasPassword)

	var message string
	if setupRequired {
		message = "Setup wizard is available - admin account needs password configuration"
	} else if !adminExists {
		message = "Setup wizard unavailable - admin user not created yet (check database migration)"
	} else if hasPassword {
		message = "Setup wizard disabled - admin account is already configured"
	}

	response := SetupStatusResponse{
		SetupRequired: setupRequired,
		AdminExists:   adminExists,
		HasPassword:   hasPassword,
		Message:       message,
	}

	// Debug logging
	fmt.Printf("DEBUG GetSetupStatus response: %+v\n", response)

	c.JSON(http.StatusOK, response)
}

// isSetupRequired checks if the setup wizard should be accessible
// Returns: (setupRequired, adminExists, hasPassword)
func (h *SetupHandler) isSetupRequired() (bool, bool, bool) {
	var passwordHash sql.NullString
	err := h.DB.DB().QueryRow("SELECT password_hash FROM users WHERE id = 'admin'").Scan(&passwordHash)

	if err != nil {
		if err == sql.ErrNoRows {
			// Admin user doesn't exist yet
			fmt.Printf("DEBUG isSetupRequired: Admin user not found (sql.ErrNoRows)\n")
			return false, false, false
		}
		// Database error - don't allow setup
		fmt.Printf("DEBUG isSetupRequired: Database error: %v\n", err)
		return false, true, false
	}

	// Admin exists, check if password is set
	hasPassword := passwordHash.Valid && passwordHash.String != ""
	fmt.Printf("DEBUG isSetupRequired: Admin found - passwordHash.Valid=%v, passwordHash.String=%q, hasPassword=%v\n",
		passwordHash.Valid, passwordHash.String, hasPassword)

	// Setup required if admin exists but has no password
	setupRequired := !hasPassword
	fmt.Printf("DEBUG isSetupRequired: setupRequired=%v\n", setupRequired)
	return setupRequired, true, hasPassword
}

// ============================================================================
// SETUP WIZARD EXECUTION
// ============================================================================

// SetupAdminRequest is the request body for admin setup
type SetupAdminRequest struct {
	Password        string `json:"password" binding:"required"`
	PasswordConfirm string `json:"passwordConfirm" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
}

// SetupAdminResponse is the response after successful setup
type SetupAdminResponse struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// SetupAdmin configures the initial admin password
// POST /api/v1/auth/setup
//
// This endpoint is only available when the admin account exists but has no password.
// After successful setup, it automatically disables the setup wizard.
//
// Request body:
//   {
//     "password": "secure-password-min-12-chars",
//     "passwordConfirm": "secure-password-min-12-chars",
//     "email": "admin@example.com"
//   }
//
// Returns:
//   200 OK: Setup completed successfully
//   400 Bad Request: Invalid input or validation error
//   403 Forbidden: Setup wizard is disabled (admin already configured)
//   500 Internal Server Error: Database error
func (h *SetupHandler) SetupAdmin(c *gin.Context) {
	// Check if setup is allowed
	setupRequired, adminExists, hasPassword := h.isSetupRequired()

	if !setupRequired {
		if !adminExists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Setup wizard is not available - admin user not created yet",
				"hint":  "Check database migration logs for errors",
			})
			return
		}
		if hasPassword {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Setup wizard is disabled - admin account is already configured",
				"hint":  "Use the login page or password reset mechanism instead",
			})
			return
		}
	}

	// Parse and validate request
	var req SetupAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate password confirmation
	if req.Password != req.PasswordConfirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Passwords do not match",
		})
		return
	}

	// Validate password strength
	if err := validatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Validate email format (additional validation beyond Gin binding)
	if err := validateEmailFormat(req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process password",
		})
		return
	}

	// Update admin user in a transaction to ensure atomicity
	tx, err := h.DB.DB().Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start database transaction",
		})
		return
	}
	defer tx.Rollback()

	// Update admin user (only if password is still NULL - prevents race conditions)
	result, err := tx.Exec(`
		UPDATE users
		SET password_hash = $1, email = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = 'admin' AND (password_hash IS NULL OR password_hash = '')
	`, string(hashedPassword), req.Email)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to configure admin account",
		})
		return
	}

	// Check if the update actually modified a row
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify setup completion",
		})
		return
	}

	if rowsAffected == 0 {
		// Another request beat us to it (race condition) or password was already set
		c.JSON(http.StatusConflict, gin.H{
			"error": "Admin account was already configured by another request",
			"hint":  "Setup wizard is now disabled - use the login page",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to commit admin configuration",
		})
		return
	}

	// Log success (for audit and monitoring)
	c.JSON(http.StatusOK, SetupAdminResponse{
		Message:  "Admin account configured successfully - setup wizard is now disabled",
		Username: "admin",
		Email:    req.Email,
	})
}

// ============================================================================
// INPUT VALIDATION HELPERS
// ============================================================================

// validatePasswordStrength checks if a password meets minimum security requirements
//
// Requirements:
// - Minimum 12 characters (NIST 800-63B recommendation)
// - No maximum length restriction (allows passphrases)
// - Future: Can add complexity requirements if needed
//
// Returns:
//   error: Validation error message or nil if valid
func validatePasswordStrength(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters long (NIST recommendation for admin accounts)")
	}

	if len(password) > 128 {
		return fmt.Errorf("password must be 128 characters or less")
	}

	// Check for common weak passwords (optional - can expand this list)
	weakPasswords := []string{
		"123456789012",
		"password1234",
		"admin1234567",
		"changeme1234",
	}

	for _, weak := range weakPasswords {
		if password == weak {
			return fmt.Errorf("password is too common - please choose a stronger password")
		}
	}

	return nil
}

// validateEmailFormat validates email address format
//
// Uses RFC 5322 simplified regex pattern for validation
// Additional validation beyond Gin's email binding tag
//
// Returns:
//   error: Validation error message or nil if valid
func validateEmailFormat(email string) error {
	if len(email) == 0 {
		return fmt.Errorf("email is required")
	}

	if len(email) > 254 {
		return fmt.Errorf("email must be 254 characters or less (RFC 5321 limit)")
	}

	// Simplified RFC 5322 regex (catches most common errors)
	// Full RFC 5322 regex is extremely complex and unnecessary for basic validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// ============================================================================
// ROUTE REGISTRATION
// ============================================================================

// RegisterRoutes registers setup wizard endpoints
// These routes are public (no authentication required) as they are needed
// for initial admin account setup before authentication is possible.
//
// Routes:
//   GET  /setup/status - Check if setup wizard is enabled
//   POST /setup        - Configure admin account
func (h *SetupHandler) RegisterRoutes(router *gin.RouterGroup) {
	// GET /api/v1/auth/setup/status - Check setup status
	router.GET("/setup/status", h.GetSetupStatus)

	// POST /api/v1/auth/setup - Execute setup wizard
	router.POST("/setup", h.SetupAdmin)
}
