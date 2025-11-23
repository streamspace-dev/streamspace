// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements license management for platform licensing and feature enforcement.
//
// LICENSE MANAGEMENT:
// - License activation and validation
// - Feature toggling based on tier (Community, Pro, Enterprise)
// - Resource limit enforcement (users, sessions, nodes)
// - Usage tracking and trending
// - License expiration monitoring
//
// LICENSE TIERS:
// - Community (Free): 10 users, 20 sessions, 3 nodes, basic auth only
// - Pro: 100 users, 200 sessions, 10 nodes, SAML/OIDC/MFA/recordings
// - Enterprise: Unlimited users/sessions/nodes, all features + SLA
//
// API Endpoints:
// - GET /api/v1/admin/license - Get current license details
// - POST /api/v1/admin/license/activate - Activate new license key
// - PUT /api/v1/admin/license/update - Update/renew license
// - GET /api/v1/admin/license/usage - Current usage vs. limits
// - POST /api/v1/admin/license/validate - Validate license key
// - GET /api/v1/admin/license/history - Usage history
//
// Thread Safety:
// - Database operations are thread-safe
// - License checks cached for performance
//
// Dependencies:
// - Database: PostgreSQL licenses and license_usage tables
//
// Example Usage:
//
//	handler := NewLicenseHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1/admin"))
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// LicenseHandler handles license management endpoints
type LicenseHandler struct {
	database *db.Database
}

// NewLicenseHandler creates a new license handler
func NewLicenseHandler(database *db.Database) *LicenseHandler {
	return &LicenseHandler{
		database: database,
	}
}

// RegisterRoutes registers license routes
func (h *LicenseHandler) RegisterRoutes(router *gin.RouterGroup) {
	license := router.Group("/license")
	{
		license.GET("", h.GetCurrentLicense)
		license.POST("/activate", h.ActivateLicense)
		license.PUT("/update", h.UpdateLicense)
		license.GET("/usage", h.GetLicenseUsage)
		license.POST("/validate", h.ValidateLicense)
		license.GET("/history", h.GetUsageHistory)
	}
}

// License represents a platform license
type License struct {
	ID          int                    `json:"id"`
	LicenseKey  string                 `json:"license_key"`
	Tier        string                 `json:"tier"` // community, pro, enterprise
	Features    map[string]interface{} `json:"features"`
	MaxUsers    *int                   `json:"max_users"`    // nil = unlimited
	MaxSessions *int                   `json:"max_sessions"` // nil = unlimited
	MaxNodes    *int                   `json:"max_nodes"`    // nil = unlimited
	IssuedAt    time.Time              `json:"issued_at"`
	ExpiresAt   time.Time              `json:"expires_at"`
	ActivatedAt *time.Time             `json:"activated_at"`
	Status      string                 `json:"status"` // active, expired, revoked
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// LicenseUsage represents usage snapshot for a specific date
type LicenseUsage struct {
	ID            int       `json:"id"`
	LicenseID     int       `json:"license_id"`
	SnapshotDate  string    `json:"snapshot_date"` // YYYY-MM-DD
	ActiveUsers   int       `json:"active_users"`
	ActiveSessions int      `json:"active_sessions"`
	ActiveNodes   int       `json:"active_nodes"`
	CreatedAt     time.Time `json:"created_at"`
}

// CurrentLicenseResponse represents current license with usage information
type CurrentLicenseResponse struct {
	License          License              `json:"license"`
	Usage            LicenseUsageStats    `json:"usage"`
	DaysUntilExpiry  int                  `json:"days_until_expiry"`
	IsExpired        bool                 `json:"is_expired"`
	IsExpiringSoon   bool                 `json:"is_expiring_soon"` // < 30 days
	LimitWarnings    []LimitWarning       `json:"limit_warnings"`
}

// LicenseUsageStats represents current usage statistics
type LicenseUsageStats struct {
	CurrentUsers    int     `json:"current_users"`
	CurrentSessions int     `json:"current_sessions"`
	CurrentNodes    int     `json:"current_nodes"`
	MaxUsers        *int    `json:"max_users"`        // nil = unlimited
	MaxSessions     *int    `json:"max_sessions"`     // nil = unlimited
	MaxNodes        *int    `json:"max_nodes"`        // nil = unlimited
	UserPercent     *float64 `json:"user_percent"`    // nil if unlimited
	SessionPercent  *float64 `json:"session_percent"` // nil if unlimited
	NodePercent     *float64 `json:"node_percent"`    // nil if unlimited
}

// LimitWarning represents a warning when approaching limits
type LimitWarning struct {
	Resource    string  `json:"resource"`     // users, sessions, nodes
	Current     int     `json:"current"`
	Limit       int     `json:"limit"`
	Percentage  float64 `json:"percentage"`
	Severity    string  `json:"severity"`     // warning (80%), critical (90%), exceeded (100%)
	Message     string  `json:"message"`
}

// GetCurrentLicense godoc
// @Summary Get current active license
// @Description Retrieves the currently active license with usage statistics
// @Tags admin, license
// @Accept json
// @Produce json
// @Success 200 {object} CurrentLicenseResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/license [get]
func (h *LicenseHandler) GetCurrentLicense(c *gin.Context) {
	// Get active license
	query := `
		SELECT id, license_key, tier, features, max_users, max_sessions, max_nodes,
		       issued_at, expires_at, activated_at, status, metadata, created_at, updated_at
		FROM licenses
		WHERE status = 'active'
		ORDER BY activated_at DESC
		LIMIT 1
	`

	var license License
	var featuresJSON, metadataJSON []byte

	err := h.database.DB().QueryRow(query).Scan(
		&license.ID,
		&license.LicenseKey,
		&license.Tier,
		&featuresJSON,
		&license.MaxUsers,
		&license.MaxSessions,
		&license.MaxNodes,
		&license.IssuedAt,
		&license.ExpiresAt,
		&license.ActivatedAt,
		&license.Status,
		&metadataJSON,
		&license.CreatedAt,
		&license.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "No active license found",
				Message: "Platform has no active license configured",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve license",
			Message: err.Error(),
		})
		return
	}

	// Parse JSONB fields
	if err := json.Unmarshal(featuresJSON, &license.Features); err != nil {
		license.Features = make(map[string]interface{})
	}
	if err := json.Unmarshal(metadataJSON, &license.Metadata); err != nil {
		license.Metadata = make(map[string]interface{})
	}

	// Get current usage statistics
	usage, err := h.getCurrentUsage(&license)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve usage statistics",
			Message: err.Error(),
		})
		return
	}

	// Calculate expiration info
	now := time.Now()
	daysUntilExpiry := int(license.ExpiresAt.Sub(now).Hours() / 24)
	isExpired := now.After(license.ExpiresAt)
	isExpiringSoon := daysUntilExpiry <= 30 && daysUntilExpiry > 0

	// Generate limit warnings
	warnings := h.generateLimitWarnings(usage)

	c.JSON(http.StatusOK, CurrentLicenseResponse{
		License:         license,
		Usage:           usage,
		DaysUntilExpiry: daysUntilExpiry,
		IsExpired:       isExpired,
		IsExpiringSoon:  isExpiringSoon,
		LimitWarnings:   warnings,
	})
}

// getCurrentUsage calculates current resource usage
func (h *LicenseHandler) getCurrentUsage(license *License) (LicenseUsageStats, error) {
	var stats LicenseUsageStats

	// Get current user count
	err := h.database.DB().QueryRow("SELECT COUNT(*) FROM users WHERE active = true").Scan(&stats.CurrentUsers)
	if err != nil {
		return stats, fmt.Errorf("failed to count users: %w", err)
	}

	// Get current session count
	err = h.database.DB().QueryRow("SELECT COUNT(*) FROM sessions WHERE status IN ('running', 'hibernated')").Scan(&stats.CurrentSessions)
	if err != nil {
		return stats, fmt.Errorf("failed to count sessions: %w", err)
	}

	// Get current node count (assuming controllers table exists)
	err = h.database.DB().QueryRow("SELECT COUNT(*) FROM controllers WHERE status = 'connected'").Scan(&stats.CurrentNodes)
	if err != nil {
		// If controllers table doesn't exist, default to 0
		stats.CurrentNodes = 0
	}

	// Set limits from license
	stats.MaxUsers = license.MaxUsers
	stats.MaxSessions = license.MaxSessions
	stats.MaxNodes = license.MaxNodes

	// Calculate percentages (nil if unlimited)
	if license.MaxUsers != nil && *license.MaxUsers > 0 {
		percent := float64(stats.CurrentUsers) / float64(*license.MaxUsers) * 100
		stats.UserPercent = &percent
	}
	if license.MaxSessions != nil && *license.MaxSessions > 0 {
		percent := float64(stats.CurrentSessions) / float64(*license.MaxSessions) * 100
		stats.SessionPercent = &percent
	}
	if license.MaxNodes != nil && *license.MaxNodes > 0 {
		percent := float64(stats.CurrentNodes) / float64(*license.MaxNodes) * 100
		stats.NodePercent = &percent
	}

	return stats, nil
}

// generateLimitWarnings creates warnings for resources approaching limits
func (h *LicenseHandler) generateLimitWarnings(usage LicenseUsageStats) []LimitWarning {
	var warnings []LimitWarning

	// Check user limits
	if usage.MaxUsers != nil && *usage.MaxUsers > 0 {
		percent := float64(usage.CurrentUsers) / float64(*usage.MaxUsers) * 100
		if percent >= 80 {
			severity := "warning"
			if percent >= 90 {
				severity = "critical"
			}
			if percent >= 100 {
				severity = "exceeded"
			}
			warnings = append(warnings, LimitWarning{
				Resource:   "users",
				Current:    usage.CurrentUsers,
				Limit:      *usage.MaxUsers,
				Percentage: percent,
				Severity:   severity,
				Message:    fmt.Sprintf("Using %d of %d users (%.1f%%)", usage.CurrentUsers, *usage.MaxUsers, percent),
			})
		}
	}

	// Check session limits
	if usage.MaxSessions != nil && *usage.MaxSessions > 0 {
		percent := float64(usage.CurrentSessions) / float64(*usage.MaxSessions) * 100
		if percent >= 80 {
			severity := "warning"
			if percent >= 90 {
				severity = "critical"
			}
			if percent >= 100 {
				severity = "exceeded"
			}
			warnings = append(warnings, LimitWarning{
				Resource:   "sessions",
				Current:    usage.CurrentSessions,
				Limit:      *usage.MaxSessions,
				Percentage: percent,
				Severity:   severity,
				Message:    fmt.Sprintf("Using %d of %d sessions (%.1f%%)", usage.CurrentSessions, *usage.MaxSessions, percent),
			})
		}
	}

	// Check node limits
	if usage.MaxNodes != nil && *usage.MaxNodes > 0 {
		percent := float64(usage.CurrentNodes) / float64(*usage.MaxNodes) * 100
		if percent >= 80 {
			severity := "warning"
			if percent >= 90 {
				severity = "critical"
			}
			if percent >= 100 {
				severity = "exceeded"
			}
			warnings = append(warnings, LimitWarning{
				Resource:   "nodes",
				Current:    usage.CurrentNodes,
				Limit:      *usage.MaxNodes,
				Percentage: percent,
				Severity:   severity,
				Message:    fmt.Sprintf("Using %d of %d nodes (%.1f%%)", usage.CurrentNodes, *usage.MaxNodes, percent),
			})
		}
	}

	return warnings
}

// ActivateLicenseRequest represents license activation request
type ActivateLicenseRequest struct {
	LicenseKey string `json:"license_key" binding:"required" validate:"required,min=10,max=256"`
}

// ActivateLicense godoc
// @Summary Activate a new license key
// @Description Activates a new license key and deactivates the current license
// @Tags admin, license
// @Accept json
// @Produce json
// @Param body body ActivateLicenseRequest true "License key to activate"
// @Success 200 {object} License
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/license/activate [post]
func (h *LicenseHandler) ActivateLicense(c *gin.Context) {
	var req ActivateLicenseRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Begin transaction
	tx, err := h.database.DB().Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to start transaction",
			Message: err.Error(),
		})
		return
	}
	defer tx.Rollback()

	// Deactivate current license
	_, err = tx.Exec("UPDATE licenses SET status = 'inactive' WHERE status = 'active'")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to deactivate current license",
			Message: err.Error(),
		})
		return
	}

	// Check if license key exists
	var license License
	var featuresJSON, metadataJSON []byte

	query := `
		SELECT id, license_key, tier, features, max_users, max_sessions, max_nodes,
		       issued_at, expires_at, activated_at, status, metadata, created_at, updated_at
		FROM licenses
		WHERE license_key = $1
	`

	err = tx.QueryRow(query, req.LicenseKey).Scan(
		&license.ID,
		&license.LicenseKey,
		&license.Tier,
		&featuresJSON,
		&license.MaxUsers,
		&license.MaxSessions,
		&license.MaxNodes,
		&license.IssuedAt,
		&license.ExpiresAt,
		&license.ActivatedAt,
		&license.Status,
		&metadataJSON,
		&license.CreatedAt,
		&license.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "License key not found",
				Message: fmt.Sprintf("No license found with key %s", req.LicenseKey),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve license",
			Message: err.Error(),
		})
		return
	}

	// Parse JSONB fields
	if err := json.Unmarshal(featuresJSON, &license.Features); err != nil {
		license.Features = make(map[string]interface{})
	}
	if err := json.Unmarshal(metadataJSON, &license.Metadata); err != nil {
		license.Metadata = make(map[string]interface{})
	}

	// Check if license is expired
	if time.Now().After(license.ExpiresAt) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "License expired",
			Message: fmt.Sprintf("License expired on %s", license.ExpiresAt.Format("2006-01-02")),
		})
		return
	}

	// Activate license
	now := time.Now()
	_, err = tx.Exec(
		"UPDATE licenses SET status = 'active', activated_at = $1, updated_at = $2 WHERE license_key = $3",
		now, now, req.LicenseKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to activate license",
			Message: err.Error(),
		})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to commit activation",
			Message: err.Error(),
		})
		return
	}

	// Update license object with activation time
	license.ActivatedAt = &now
	license.Status = "active"
	license.UpdatedAt = now

	c.JSON(http.StatusOK, license)
}

// UpdateLicenseRequest represents license update request
type UpdateLicenseRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

// UpdateLicense godoc
// @Summary Update/renew license
// @Description Updates the current license (for renewals or upgrades)
// @Tags admin, license
// @Accept json
// @Produce json
// @Param body body UpdateLicenseRequest true "New license key"
// @Success 200 {object} License
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/license/update [put]
func (h *LicenseHandler) UpdateLicense(c *gin.Context) {
	// Same as ActivateLicense for now
	h.ActivateLicense(c)
}

// GetLicenseUsage godoc
// @Summary Get current license usage
// @Description Retrieves current usage statistics vs. license limits
// @Tags admin, license
// @Accept json
// @Produce json
// @Success 200 {object} LicenseUsageStats
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/license/usage [get]
func (h *LicenseHandler) GetLicenseUsage(c *gin.Context) {
	// Get active license
	query := `
		SELECT id, license_key, tier, features, max_users, max_sessions, max_nodes,
		       issued_at, expires_at, activated_at, status, metadata, created_at, updated_at
		FROM licenses
		WHERE status = 'active'
		ORDER BY activated_at DESC
		LIMIT 1
	`

	var license License
	var featuresJSON, metadataJSON []byte

	err := h.database.DB().QueryRow(query).Scan(
		&license.ID,
		&license.LicenseKey,
		&license.Tier,
		&featuresJSON,
		&license.MaxUsers,
		&license.MaxSessions,
		&license.MaxNodes,
		&license.IssuedAt,
		&license.ExpiresAt,
		&license.ActivatedAt,
		&license.Status,
		&metadataJSON,
		&license.CreatedAt,
		&license.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "No active license found",
				Message: "Platform has no active license configured",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve license",
			Message: err.Error(),
		})
		return
	}

	// Get current usage statistics
	usage, err := h.getCurrentUsage(&license)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve usage statistics",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// ValidateLicenseRequest represents license validation request
type ValidateLicenseRequest struct {
	LicenseKey string `json:"license_key" binding:"required" validate:"required,min=10,max=256"`
}

// ValidateLicenseResponse represents license validation result
type ValidateLicenseResponse struct {
	Valid     bool                   `json:"valid"`
	Tier      string                 `json:"tier,omitempty"`
	Features  map[string]interface{} `json:"features,omitempty"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
	Message   string                 `json:"message"`
}

// ValidateLicense godoc
// @Summary Validate a license key
// @Description Validates a license key without activating it
// @Tags admin, license
// @Accept json
// @Produce json
// @Param body body ValidateLicenseRequest true "License key to validate"
// @Success 200 {object} ValidateLicenseResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/admin/license/validate [post]
func (h *LicenseHandler) ValidateLicense(c *gin.Context) {
	var req ValidateLicenseRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Check if license key exists
	query := `
		SELECT tier, features, expires_at
		FROM licenses
		WHERE license_key = $1
	`

	var tier string
	var featuresJSON []byte
	var expiresAt time.Time

	err := h.database.DB().QueryRow(query, req.LicenseKey).Scan(&tier, &featuresJSON, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, ValidateLicenseResponse{
				Valid:   false,
				Message: "License key not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to validate license",
			Message: err.Error(),
		})
		return
	}

	// Parse features
	var features map[string]interface{}
	if err := json.Unmarshal(featuresJSON, &features); err != nil {
		features = make(map[string]interface{})
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		c.JSON(http.StatusOK, ValidateLicenseResponse{
			Valid:     false,
			Tier:      tier,
			ExpiresAt: &expiresAt,
			Message:   fmt.Sprintf("License expired on %s", expiresAt.Format("2006-01-02")),
		})
		return
	}

	c.JSON(http.StatusOK, ValidateLicenseResponse{
		Valid:     true,
		Tier:      tier,
		Features:  features,
		ExpiresAt: &expiresAt,
		Message:   "License is valid",
	})
}

// GetUsageHistory godoc
// @Summary Get usage history
// @Description Retrieves historical usage data for the active license
// @Tags admin, license
// @Accept json
// @Produce json
// @Param days query int false "Number of days to retrieve (default: 30)"
// @Success 200 {array} LicenseUsage
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/license/history [get]
func (h *LicenseHandler) GetUsageHistory(c *gin.Context) {
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := fmt.Sscanf(daysParam, "%d", &days); err == nil && d == 1 {
			if days < 1 {
				days = 1
			}
			if days > 365 {
				days = 365
			}
		}
	}

	// Get active license ID
	var licenseID int
	err := h.database.DB().QueryRow("SELECT id FROM licenses WHERE status = 'active' ORDER BY activated_at DESC LIMIT 1").Scan(&licenseID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "No active license found",
				Message: "Platform has no active license configured",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve license",
			Message: err.Error(),
		})
		return
	}

	// Get usage history
	query := `
		SELECT id, license_id, snapshot_date, active_users, active_sessions, active_nodes, created_at
		FROM license_usage
		WHERE license_id = $1 AND snapshot_date >= CURRENT_DATE - $2
		ORDER BY snapshot_date DESC
	`

	rows, err := h.database.DB().Query(query, licenseID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve usage history",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	var history []LicenseUsage
	for rows.Next() {
		var usage LicenseUsage
		err := rows.Scan(
			&usage.ID,
			&usage.LicenseID,
			&usage.SnapshotDate,
			&usage.ActiveUsers,
			&usage.ActiveSessions,
			&usage.ActiveNodes,
			&usage.CreatedAt,
		)
		if err != nil {
			continue
		}
		history = append(history, usage)
	}

	if history == nil {
		history = []LicenseUsage{}
	}

	c.JSON(http.StatusOK, history)
}
