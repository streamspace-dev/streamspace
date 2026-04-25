// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements license enforcement middleware.
//
// LICENSE ENFORCEMENT:
// - Check license limits before creating resources (users, sessions, nodes)
// - Block actions that exceed license limits
// - Warn when approaching limits (80%, 90%, 95%)
// - Cache license information for performance
//
// LICENSE TIERS:
// - Community (Free): 10 users, 20 sessions, 3 nodes
// - Pro: 100 users, 200 sessions, 10 nodes
// - Enterprise: Unlimited users, sessions, nodes
//
// USAGE:
//
//	router.Use(middleware.LicenseEnforcement(database))
//	router.POST("/users", handler.CreateUser) // Will check license limits
//
// Thread Safety:
// - License cache is thread-safe with mutex
// - Database operations are thread-safe
//
// Dependencies:
// - Database: PostgreSQL licenses table
//
// Example Usage:
//
//	// Apply middleware to admin routes
//	admin := router.Group("/api/v1/admin")
//	admin.Use(middleware.LicenseEnforcement(database))
package middleware

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
)

// LicenseInfo holds cached license information
type LicenseInfo struct {
	ID          int
	Tier        string
	MaxUsers    *int
	MaxSessions *int
	MaxNodes    *int
	ExpiresAt   time.Time
	Status      string
	Features    map[string]interface{}
	LastChecked time.Time
}

var (
	licenseCache      *LicenseInfo
	licenseCacheMutex sync.RWMutex
	cacheTTL          = 5 * time.Minute // Cache license for 5 minutes
)

// LicenseEnforcement middleware checks license limits before resource creation
func LicenseEnforcement(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only check on POST (create) requests
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		// Get license info (from cache or database)
		license, err := getLicenseInfo(database)
		if err != nil {
			// If no license found, allow operation (fail open for Community tier)
			c.Next()
			return
		}

		// Check if license is expired
		if time.Now().After(license.ExpiresAt) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "License expired",
				"message": fmt.Sprintf("Platform license expired on %s. Please renew your license.", license.ExpiresAt.Format("2006-01-02")),
			})
			c.Abort()
			return
		}

		// Determine resource type based on path
		path := c.Request.URL.Path
		var resourceType string
		var currentCount int
		var limit *int

		if contains(path, "/users") {
			resourceType = "users"
			limit = license.MaxUsers
			// Count active users
			err := database.DB().QueryRow("SELECT COUNT(*) FROM users WHERE active = true").Scan(&currentCount)
			if err != nil {
				// Fail open if count query fails
				c.Next()
				return
			}
		} else if contains(path, "/sessions") {
			resourceType = "sessions"
			limit = license.MaxSessions
			// Count active sessions
			err := database.DB().QueryRow("SELECT COUNT(*) FROM sessions WHERE status IN ('running', 'hibernated')").Scan(&currentCount)
			if err != nil {
				// Fail open if count query fails
				c.Next()
				return
			}
		} else if contains(path, "/nodes") || contains(path, "/controllers") {
			resourceType = "nodes"
			limit = license.MaxNodes
			// Count active nodes
			err := database.DB().QueryRow("SELECT COUNT(*) FROM controllers WHERE status = 'connected'").Scan(&currentCount)
			if err != nil {
				// Fail open if count query fails (table might not exist yet)
				c.Next()
				return
			}
		} else {
			// Not a resource we enforce limits on
			c.Next()
			return
		}

		// Check if limit is set (nil = unlimited)
		if limit == nil {
			c.Next()
			return
		}

		// Check if at limit
		if currentCount >= *limit {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "License limit exceeded",
				"message": fmt.Sprintf("Cannot create %s: license limit of %d reached. Current: %d. Upgrade your license to increase limits.", resourceType, *limit, currentCount),
				"resource": resourceType,
				"current":  currentCount,
				"limit":    *limit,
				"tier":     license.Tier,
			})
			c.Abort()
			return
		}

		// Add warning header if approaching limit (80%+)
		percentage := float64(currentCount) / float64(*limit) * 100
		if percentage >= 80 {
			c.Header("X-License-Warning", fmt.Sprintf("Approaching %s limit: %d/%d (%.1f%%)", resourceType, currentCount, *limit, percentage))
		}

		// Set license info in context for handlers to use
		c.Set("license", license)

		c.Next()
	}
}

// getLicenseInfo retrieves license from cache or database
func getLicenseInfo(database *db.Database) (*LicenseInfo, error) {
	// Check cache first
	licenseCacheMutex.RLock()
	if licenseCache != nil && time.Since(licenseCache.LastChecked) < cacheTTL {
		defer licenseCacheMutex.RUnlock()
		return licenseCache, nil
	}
	licenseCacheMutex.RUnlock()

	// Fetch from database
	licenseCacheMutex.Lock()
	defer licenseCacheMutex.Unlock()

	// Double-check after acquiring write lock
	if licenseCache != nil && time.Since(licenseCache.LastChecked) < cacheTTL {
		return licenseCache, nil
	}

	query := `
		SELECT id, tier, max_users, max_sessions, max_nodes, expires_at, status, features
		FROM licenses
		WHERE status = 'active'
		ORDER BY activated_at DESC
		LIMIT 1
	`

	var license LicenseInfo
	var featuresJSON []byte

	err := database.DB().QueryRow(query).Scan(
		&license.ID,
		&license.Tier,
		&license.MaxUsers,
		&license.MaxSessions,
		&license.MaxNodes,
		&license.ExpiresAt,
		&license.Status,
		&featuresJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active license found")
		}
		return nil, fmt.Errorf("failed to retrieve license: %w", err)
	}

	// Parse features
	if err := json.Unmarshal(featuresJSON, &license.Features); err != nil {
		license.Features = make(map[string]interface{})
	}

	license.LastChecked = time.Now()

	// Update cache
	licenseCache = &license

	return &license, nil
}

// CheckFeatureEnabled checks if a specific feature is enabled in the license
func CheckFeatureEnabled(database *db.Database, feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		license, err := getLicenseInfo(database)
		if err != nil {
			// Fail open for Community tier (basic features allowed)
			if feature == "basic_auth" {
				c.Next()
				return
			}

			c.JSON(http.StatusForbidden, gin.H{
				"error":   "License required",
				"message": fmt.Sprintf("Feature '%s' requires an active license", feature),
			})
			c.Abort()
			return
		}

		// Check if feature is enabled
		if enabled, ok := license.Features[feature].(bool); ok && enabled {
			c.Next()
			return
		}

		// Feature not enabled
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Feature not available",
			"message": fmt.Sprintf("Feature '%s' is not available in your %s license tier. Upgrade to access this feature.", feature, license.Tier),
			"tier":    license.Tier,
			"feature": feature,
		})
		c.Abort()
	}
}

// ClearLicenseCache clears the license cache (call after license activation)
func ClearLicenseCache() {
	licenseCacheMutex.Lock()
	defer licenseCacheMutex.Unlock()
	licenseCache = nil
}

// GetCachedLicense returns the cached license (for read-only access)
func GetCachedLicense(database *db.Database) (*LicenseInfo, error) {
	return getLicenseInfo(database)
}

// contains checks if string contains substring (case-sensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
