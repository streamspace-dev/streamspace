package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/streamspace/streamspace/api/internal/middleware"
)

// ============================================================================
// INPUT VALIDATION
// ============================================================================

// validateIPWhitelistInput validates IP whitelist entry input
func validateIPWhitelistInput(ipOrCIDR, description string) error {
	// Validate IP/CIDR is provided
	if ipOrCIDR == "" {
		return fmt.Errorf("ip_address is required")
	}

	// Length check
	if len(ipOrCIDR) > 50 {
		return fmt.Errorf("ip_address must be 50 characters or less")
	}

	// Try parsing as CIDR first
	_, _, err := net.ParseCIDR(ipOrCIDR)
	if err != nil {
		// Not a CIDR, try parsing as IP
		ip := net.ParseIP(ipOrCIDR)
		if ip == nil {
			return fmt.Errorf("invalid IP address or CIDR format")
		}
	}

	// Description length
	if len(description) > 500 {
		return fmt.Errorf("description must be 500 characters or less")
	}

	return nil
}

// validateMFASetupInput validates MFA setup request input
func validateMFASetupInput(mfaType, phoneNumber, email string) error {
	// Type validation
	validTypes := []string{"totp", "sms", "email"}
	valid := false
	for _, t := range validTypes {
		if mfaType == t {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid MFA type, must be one of: %s", strings.Join(validTypes, ", "))
	}

	// Phone number validation for SMS
	if mfaType == "sms" {
		if phoneNumber == "" {
			return fmt.Errorf("phone number is required for SMS MFA")
		}
		if len(phoneNumber) < 10 || len(phoneNumber) > 20 {
			return fmt.Errorf("phone number must be 10-20 characters")
		}
	}

	// Email validation for email MFA
	if mfaType == "email" {
		if email == "" {
			return fmt.Errorf("email is required for Email MFA")
		}
		if len(email) > 255 {
			return fmt.Errorf("email must be 255 characters or less")
		}
		// Basic email format check
		if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
			return fmt.Errorf("invalid email format")
		}
	}

	return nil
}

// ============================================================================
// MULTI-FACTOR AUTHENTICATION (MFA)
// ============================================================================

// MFAMethod represents different MFA verification methods
type MFAMethod struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"user_id"`
	Type        string    `json:"type"` // "totp", "sms", "email", "backup_codes"
	Enabled     bool      `json:"enabled"`
	Secret      string    `json:"-"` // SECURITY: Never expose secret in API responses
	PhoneNumber string    `json:"phone_number,omitempty"`
	Email       string    `json:"email,omitempty"`
	IsPrimary   bool      `json:"is_primary"`
	Verified    bool      `json:"verified"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
}

// MFASetupResponse is used only for SetupMFA response to show secret/QR once
type MFASetupResponse struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	Secret  string `json:"secret,omitempty"`  // Only for TOTP setup
	QRCode  string `json:"qr_code,omitempty"` // Only for TOTP setup
	Message string `json:"message"`
}

// BackupCode represents MFA backup recovery codes
type BackupCode struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Code      string    `json:"code"`      // Hashed in DB
	Used      bool      `json:"used"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TrustedDevice represents a device trusted for MFA bypass
type TrustedDevice struct {
	ID             int64     `json:"id"`
	UserID         string    `json:"user_id"`
	DeviceID       string    `json:"device_id"`       // Browser fingerprint
	DeviceName     string    `json:"device_name"`
	UserAgent      string    `json:"user_agent"`
	IPAddress      string    `json:"ip_address"`
	TrustedUntil   time.Time `json:"trusted_until"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// SetupMFA initializes MFA for a user (Step 1: Generate secret)
func (h *Handler) SetupMFA(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Type        string `json:"type" binding:"required,oneof=totp sms email"`
		PhoneNumber string `json:"phone_number,omitempty"`
		Email       string `json:"email,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// INPUT VALIDATION: Validate MFA setup input
	if err := validateMFASetupInput(req.Type, req.PhoneNumber, req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	// SECURITY: SMS and Email MFA are not yet implemented
	// They would always return "valid=true" which bypasses security
	if req.Type == "sms" || req.Type == "email" {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "MFA type not implemented",
			"message": "SMS and Email MFA are not yet available. Please use TOTP (authenticator app) for multi-factor authentication.",
			"supported_types": []string{"totp"},
		})
		return
	}

	// Check if MFA already exists
	var existingID int64
	err := h.DB.QueryRow(`
		SELECT id FROM mfa_methods
		WHERE user_id = $1 AND type = $2
	`, userID, req.Type).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if existingID > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "MFA method already exists"})
		return
	}

	var secret, qrCode string

	if req.Type == "totp" {
		// Generate TOTP secret
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "StreamSpace",
			AccountName: userID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate TOTP secret"})
			return
		}

		secret = key.Secret()
		qrCode = key.URL()
	}

	// Insert MFA method (not yet verified/enabled)
	var mfaID int64
	err = h.DB.QueryRow(`
		INSERT INTO mfa_methods (user_id, type, secret, phone_number, email, enabled, verified)
		VALUES ($1, $2, $3, $4, $5, false, false)
		RETURNING id
	`, userID, req.Type, secret, req.PhoneNumber, req.Email).Scan(&mfaID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create MFA method"})
		return
	}

	// SECURITY: Use dedicated response struct to only expose secret during setup
	response := MFASetupResponse{
		ID:   mfaID,
		Type: req.Type,
	}

	if req.Type == "totp" {
		response.Secret = secret
		response.QRCode = qrCode
		response.Message = "Scan the QR code with your authenticator app and verify"
	}

	c.JSON(http.StatusOK, response)
}

// VerifyMFASetup verifies and enables MFA method (Step 2: Confirm setup)
func (h *Handler) VerifyMFASetup(c *gin.Context) {
	userID := c.GetString("user_id")
	mfaID := c.Param("mfaId")

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get MFA method (before transaction to verify code)
	var mfaMethod MFAMethod
	err := h.DB.QueryRow(`
		SELECT id, user_id, type, secret, phone_number, email
		FROM mfa_methods
		WHERE id = $1 AND user_id = $2
	`, mfaID, userID).Scan(&mfaMethod.ID, &mfaMethod.UserID, &mfaMethod.Type,
		&mfaMethod.Secret, &mfaMethod.PhoneNumber, &mfaMethod.Email)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "MFA method not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Verify code (before starting transaction)
	valid := false
	if mfaMethod.Type == "totp" {
		valid = totp.Validate(req.Code, mfaMethod.Secret)
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid verification code"})
		return
	}

	// SECURITY: Use transaction to ensure atomicity
	// Either both MFA enable AND backup codes succeed, or neither
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer tx.Rollback() // Rollback if not committed

	// Enable and verify MFA method
	_, err = tx.Exec(`
		UPDATE mfa_methods
		SET verified = true, enabled = true
		WHERE id = $1
	`, mfaID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable MFA"})
		return
	}

	// Generate backup codes within transaction
	backupCodes := make([]string, BackupCodesCount)
	for i := 0; i < BackupCodesCount; i++ {
		code := generateRandomCode(BackupCodeLength)
		backupCodes[i] = code

		// Hash and store
		hash := sha256.Sum256([]byte(code))
		hashStr := hex.EncodeToString(hash[:])

		_, err := tx.Exec(`
			INSERT INTO backup_codes (user_id, code)
			VALUES ($1, $2)
		`, userID, hashStr)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate backup codes"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit changes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "MFA enabled successfully",
		"backup_codes": backupCodes,
	})
}

// VerifyMFA verifies MFA code during login
func (h *Handler) VerifyMFA(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Code       string `json:"code" binding:"required"`
		MethodType string `json:"method_type,omitempty"` // "totp", "sms", "email", "backup_code"
		TrustDevice bool  `json:"trust_device,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MethodType == "" {
		req.MethodType = "totp" // Default to TOTP
	}

	// SECURITY: SMS and Email MFA are not implemented
	if req.MethodType == "sms" || req.MethodType == "email" {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "MFA type not implemented",
			"message": "SMS and Email MFA are not yet available",
		})
		return
	}

	// SECURITY: Rate limiting to prevent brute force attacks
	// Max MFAMaxAttemptsPerMinute attempts per minute per user
	rateLimitKey := fmt.Sprintf("mfa_verify:%s", userID)
	if !middleware.GetRateLimiter().CheckLimit(rateLimitKey, MFAMaxAttemptsPerMinute, MFARateLimitWindow) {
		attempts := middleware.GetRateLimiter().GetAttempts(rateLimitKey, MFARateLimitWindow)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "Too many verification attempts",
			"message":     "Please wait 1 minute before trying again",
			"retry_after": 60,
			"attempts":    attempts,
		})
		return
	}

	valid := false

	if req.MethodType == "backup_code" {
		// Verify backup code
		valid = h.verifyBackupCode(userID, req.Code)
	} else {
		// Get MFA method
		var secret string
		err := h.DB.QueryRow(`
			SELECT secret FROM mfa_methods
			WHERE user_id = $1 AND type = $2 AND enabled = true
		`, userID, req.MethodType).Scan(&secret)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "MFA method not found or not enabled"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Verify TOTP code
		if req.MethodType == "totp" {
			valid = totp.Validate(req.Code, secret)
		}

		// Update last used timestamp
		if valid {
			h.DB.Exec(`UPDATE mfa_methods SET last_used_at = NOW() WHERE user_id = $1 AND type = $2`,
				userID, req.MethodType)
		}
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid MFA code"})
		return
	}

	// SECURITY: Reset rate limit on successful verification
	middleware.GetRateLimiter().ResetLimit(rateLimitKey)

	// Trust device if requested
	if req.TrustDevice {
		deviceID := h.getDeviceFingerprint(c)
		h.trustDevice(userID, deviceID, c.Request.UserAgent(), c.ClientIP(), 30*24*time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "MFA verification successful",
		"verified": true,
	})
}

// ListMFAMethods lists all MFA methods for a user
func (h *Handler) ListMFAMethods(c *gin.Context) {
	userID := c.GetString("user_id")

	rows, err := h.DB.Query(`
		SELECT id, type, enabled, verified, is_primary, phone_number, email, created_at, last_used_at
		FROM mfa_methods
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at DESC
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	methods := []MFAMethod{}
	for rows.Next() {
		var m MFAMethod
		var lastUsed sql.NullTime
		err := rows.Scan(&m.ID, &m.Type, &m.Enabled, &m.Verified, &m.IsPrimary,
			&m.PhoneNumber, &m.Email, &m.CreatedAt, &lastUsed)
		if err != nil {
			continue
		}
		if lastUsed.Valid {
			m.LastUsedAt = lastUsed.Time
		}
		m.UserID = userID
		// Mask sensitive data
		if m.PhoneNumber != "" {
			m.PhoneNumber = maskPhone(m.PhoneNumber)
		}
		if m.Email != "" {
			m.Email = maskEmail(m.Email)
		}
		methods = append(methods, m)
	}

	c.JSON(http.StatusOK, gin.H{"methods": methods})
}

// DisableMFA disables an MFA method
func (h *Handler) DisableMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	mfaID := c.Param("mfaId")

	result, err := h.DB.Exec(`
		UPDATE mfa_methods SET enabled = false
		WHERE id = $1 AND user_id = $2
	`, mfaID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable MFA"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "MFA method not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MFA method disabled"})
}

// GenerateBackupCodes generates new backup codes
func (h *Handler) GenerateBackupCodes(c *gin.Context) {
	userID := c.GetString("user_id")

	// Invalidate old backup codes
	h.DB.Exec(`DELETE FROM backup_codes WHERE user_id = $1`, userID)

	// Generate new codes
	codes := h.generateBackupCodes(userID, BackupCodesCount)

	c.JSON(http.StatusOK, gin.H{
		"backup_codes": codes,
		"message": "Store these codes in a safe place. Each code can only be used once.",
	})
}

// Helper: Generate backup codes
func (h *Handler) generateBackupCodes(userID string, count int) []string {
	codes := make([]string, count)

	for i := 0; i < count; i++ {
		code := generateRandomCode(BackupCodeLength)
		codes[i] = code

		// Hash and store
		hash := sha256.Sum256([]byte(code))
		hashStr := hex.EncodeToString(hash[:])

		h.DB.Exec(`
			INSERT INTO backup_codes (user_id, code)
			VALUES ($1, $2)
		`, userID, hashStr)
	}

	return codes
}

// Helper: Verify backup code
func (h *Handler) verifyBackupCode(userID, code string) bool {
	hash := sha256.Sum256([]byte(code))
	hashStr := hex.EncodeToString(hash[:])

	var codeID int64
	err := h.DB.QueryRow(`
		SELECT id FROM backup_codes
		WHERE user_id = $1 AND code = $2 AND used = false
	`, userID, hashStr).Scan(&codeID)

	if err != nil {
		return false
	}

	// Mark as used
	h.DB.Exec(`UPDATE backup_codes SET used = true, used_at = NOW() WHERE id = $1`, codeID)
	return true
}

// ============================================================================
// IP WHITELISTING
// ============================================================================

// IPWhitelist represents IP access control rules
type IPWhitelist struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"user_id,omitempty"`      // Empty for org-wide rules
	IPAddress   string    `json:"ip_address"`              // Single IP or CIDR
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// GeoRestriction represents geographic access controls
type GeoRestriction struct {
	ID          int64    `json:"id"`
	UserID      string   `json:"user_id,omitempty"` // Empty for org-wide
	Countries   []string `json:"countries"`          // ISO country codes
	Action      string   `json:"action"`             // "allow" or "deny"
	Enabled     bool     `json:"enabled"`
	Description string   `json:"description,omitempty"`
}

// CreateIPWhitelist adds an IP to whitelist
func (h *Handler) CreateIPWhitelist(c *gin.Context) {
	createdBy := c.GetString("user_id")
	role := c.GetString("role")

	var req struct {
		UserID      string    `json:"user_id,omitempty"` // Empty for org-wide (admin only)
		IPAddress   string    `json:"ip_address" binding:"required"`
		Description string    `json:"description"`
		ExpiresAt   time.Time `json:"expires_at,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// INPUT VALIDATION: Validate IP whitelist input
	if err := validateIPWhitelistInput(req.IPAddress, req.Description); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	// Only admins can create org-wide rules
	if req.UserID == "" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can create org-wide IP rules"})
		return
	}

	// Users can only create rules for themselves
	if req.UserID != "" && req.UserID != createdBy && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot create IP rules for other users"})
		return
	}

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO ip_whitelist (user_id, ip_address, description, enabled, created_by, expires_at)
		VALUES ($1, $2, $3, true, $4, $5)
		RETURNING id
	`, req.UserID, req.IPAddress, req.Description, createdBy, req.ExpiresAt).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create IP whitelist entry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "IP whitelist entry created",
	})
}

// CheckIPAccess checks if an IP is allowed access
func (h *Handler) CheckIPAccess(c *gin.Context) {
	userID := c.Query("user_id")
	ipAddress := c.Query("ip_address")

	if ipAddress == "" {
		ipAddress = c.ClientIP()
	}

	allowed := h.isIPAllowed(userID, ipAddress)

	c.JSON(http.StatusOK, gin.H{
		"allowed":    allowed,
		"ip_address": ipAddress,
		"user_id":    userID,
	})
}

// Helper: Check if IP is allowed
func (h *Handler) isIPAllowed(userID, ipAddress string) bool {
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false
	}

	// Check user-specific rules
	rows, err := h.DB.Query(`
		SELECT ip_address FROM ip_whitelist
		WHERE (user_id = $1 OR user_id IS NULL)
		AND enabled = true
		AND (expires_at IS NULL OR expires_at > NOW())
	`, userID)

	if err != nil {
		return false // Deny on error
	}
	defer rows.Close()

	// If no rules exist, allow by default
	hasRules := false
	for rows.Next() {
		hasRules = true
		var allowedIP string
		rows.Scan(&allowedIP)

		// Check if IP matches (support CIDR)
		if strings.Contains(allowedIP, "/") {
			// CIDR notation
			_, ipNet, err := net.ParseCIDR(allowedIP)
			if err == nil && ipNet.Contains(ip) {
				return true
			}
		} else {
			// Single IP
			if allowedIP == ipAddress {
				return true
			}
		}
	}

	// If rules exist but no match found, deny
	return !hasRules
}

// ListIPWhitelist lists IP whitelist entries
func (h *Handler) ListIPWhitelist(c *gin.Context) {
	userID := c.Query("user_id")
	role := c.GetString("role")

	// Non-admins can only see their own rules
	if userID == "" || (userID != c.GetString("user_id") && role != "admin") {
		userID = c.GetString("user_id")
	}

	query := `
		SELECT id, user_id, ip_address, description, enabled, created_by, created_at, expires_at
		FROM ip_whitelist
		WHERE user_id = $1 OR (user_id IS NULL AND $2 = 'admin')
		ORDER BY created_at DESC
	`

	rows, err := h.DB.Query(query, userID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	entries := []IPWhitelist{}
	for rows.Next() {
		var entry IPWhitelist
		var userID, expiresAt sql.NullString
		var expiresAtTime sql.NullTime

		err := rows.Scan(&entry.ID, &userID, &entry.IPAddress, &entry.Description,
			&entry.Enabled, &entry.CreatedBy, &entry.CreatedAt, &expiresAtTime)
		if err != nil {
			continue
		}
		if userID.Valid {
			entry.UserID = userID.String
		}
		if expiresAtTime.Valid {
			entry.ExpiresAt = expiresAtTime.Time
		}
		entries = append(entries, entry)
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// DeleteIPWhitelist removes an IP whitelist entry
func (h *Handler) DeleteIPWhitelist(c *gin.Context) {
	entryID := c.Param("entryId")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	// SECURITY: Combine authorization check with query to prevent enumeration
	// Returns "not found" whether the entry doesn't exist OR user lacks permission
	var result sql.Result
	var err error

	if role == "admin" {
		// Admins can delete any entry
		result, err = h.DB.Exec(`DELETE FROM ip_whitelist WHERE id = $1`, entryID)
	} else {
		// Non-admins can only delete their own entries or org-wide entries (NULL user_id)
		result, err = h.DB.Exec(`
			DELETE FROM ip_whitelist
			WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
		`, entryID, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete entry"})
		return
	}

	// Check if any rows were affected
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Could be not found OR not authorized - don't reveal which
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "IP whitelist entry deleted"})
}

// ============================================================================
// ZERO TRUST / CONTINUOUS AUTHENTICATION
// ============================================================================

// SessionVerification represents continuous session verification
type SessionVerification struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	UserID         string    `json:"user_id"`
	DeviceID       string    `json:"device_id"`
	IPAddress      string    `json:"ip_address"`
	Location       string    `json:"location,omitempty"`
	RiskScore      int       `json:"risk_score"`      // 0-100
	RiskLevel      string    `json:"risk_level"`      // "low", "medium", "high", "critical"
	Verified       bool      `json:"verified"`
	LastVerifiedAt time.Time `json:"last_verified_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// DevicePosture represents device security posture
type DevicePosture struct {
	DeviceID          string                 `json:"device_id"`
	OSVersion         string                 `json:"os_version"`
	BrowserVersion    string                 `json:"browser_version"`
	ScreenResolution  string                 `json:"screen_resolution"`
	Timezone          string                 `json:"timezone"`
	Language          string                 `json:"language"`
	Plugins           []string               `json:"plugins"`
	Extensions        []string               `json:"extensions"`
	AntivirusEnabled  bool                   `json:"antivirus_enabled"`
	FirewallEnabled   bool                   `json:"firewall_enabled"`
	EncryptionEnabled bool                   `json:"encryption_enabled"`
	LastChecked       time.Time              `json:"last_checked"`
	Compliant         bool                   `json:"compliant"`
	Issues            []string               `json:"issues,omitempty"`
}

// VerifySession performs continuous session verification
func (h *Handler) VerifySession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	deviceID := h.getDeviceFingerprint(c)
	ipAddress := c.ClientIP()

	// Calculate risk score
	riskScore := h.calculateRiskScore(userID, deviceID, ipAddress, c.Request.UserAgent())

	riskLevel := "low"
	if riskScore >= 75 {
		riskLevel = "critical"
	} else if riskScore >= 50 {
		riskLevel = "high"
	} else if riskScore >= 25 {
		riskLevel = "medium"
	}

	verified := riskScore < 50 // Auto-verify if risk is low/medium

	// Record verification
	var verificationID int64
	err := h.DB.QueryRow(`
		INSERT INTO session_verifications (session_id, user_id, device_id, ip_address, risk_score, risk_level, verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, sessionID, userID, deviceID, ipAddress, riskScore, riskLevel, verified).Scan(&verificationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record verification"})
		return
	}

	response := gin.H{
		"verification_id": verificationID,
		"risk_score":      riskScore,
		"risk_level":      riskLevel,
		"verified":        verified,
	}

	if !verified {
		response["message"] = "Additional verification required"
		response["required_action"] = "mfa" // Require MFA for high-risk sessions
	}

	c.JSON(http.StatusOK, response)
}

// CheckDevicePosture checks device security posture
func (h *Handler) CheckDevicePosture(c *gin.Context) {
	var req DevicePosture

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check compliance
	issues := []string{}

	if !req.AntivirusEnabled {
		issues = append(issues, "Antivirus not enabled")
	}
	if !req.FirewallEnabled {
		issues = append(issues, "Firewall not enabled")
	}
	if !req.EncryptionEnabled {
		issues = append(issues, "Disk encryption not enabled")
	}

	req.Compliant = len(issues) == 0
	req.Issues = issues
	req.LastChecked = time.Now()

	// Store posture check result
	h.DB.Exec(`
		INSERT INTO device_posture_checks (device_id, compliant, issues, checked_at)
		VALUES ($1, $2, $3, $4)
	`, req.DeviceID, req.Compliant, strings.Join(issues, ","), time.Now())

	c.JSON(http.StatusOK, req)
}

// GetSecurityAlerts gets security alerts for a user
func (h *Handler) GetSecurityAlerts(c *gin.Context) {
	userID := c.GetString("user_id")

	rows, err := h.DB.Query(`
		SELECT type, severity, message, details, created_at
		FROM security_alerts
		WHERE user_id = $1 AND acknowledged = false
		ORDER BY severity DESC, created_at DESC
		LIMIT 50
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	alerts := []map[string]interface{}{}
	for rows.Next() {
		var alertType, severity, message, details string
		var createdAt time.Time
		rows.Scan(&alertType, &severity, &message, &details, &createdAt)
		alerts = append(alerts, map[string]interface{}{
			"type":       alertType,
			"severity":   severity,
			"message":    message,
			"details":    details,
			"created_at": createdAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Get device fingerprint from request
func (h *Handler) getDeviceFingerprint(c *gin.Context) string {
	// Simple fingerprint based on User-Agent and IP
	// In production, use more sophisticated fingerprinting
	data := c.Request.UserAgent() + c.ClientIP()
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Trust a device for MFA bypass
func (h *Handler) trustDevice(userID, deviceID, userAgent, ipAddress string, duration time.Duration) {
	trustedUntil := time.Now().Add(duration)
	deviceName := fmt.Sprintf("%s from %s", userAgent, ipAddress)

	h.DB.Exec(`
		INSERT INTO trusted_devices (user_id, device_id, device_name, user_agent, ip_address, trusted_until)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			last_seen_at = NOW(),
			trusted_until = $6
	`, userID, deviceID, deviceName, userAgent, ipAddress, trustedUntil)
}

// Calculate risk score (0-100)
func (h *Handler) calculateRiskScore(userID, deviceID, ipAddress, userAgent string) int {
	score := 0

	// Check if device is trusted
	var trusted bool
	err := h.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM trusted_devices
			WHERE user_id = $1 AND device_id = $2 AND trusted_until > NOW()
		)
	`, userID, deviceID).Scan(&trusted)

	if err == nil && trusted {
		score -= 30 // Reduce risk for trusted devices
	} else {
		score += 20 // Unknown device
	}

	// Check IP reputation
	if !h.isIPAllowed(userID, ipAddress) {
		score += 40 // IP not whitelisted
	}

	// Check for recent failed login attempts
	var failedAttempts int
	h.DB.QueryRow(`
		SELECT COUNT(*) FROM audit_log
		WHERE user_id = $1 AND action = 'login_failed'
		AND created_at > NOW() - INTERVAL '1 hour'
	`, userID).Scan(&failedAttempts)

	score += failedAttempts * 10

	// Check for location change
	var lastIP string
	h.DB.QueryRow(`
		SELECT ip_address FROM session_verifications
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&lastIP)

	if lastIP != "" && lastIP != ipAddress {
		score += 15 // Location change
	}

	// Clamp score to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// Validate IP or CIDR
func isValidIPOrCIDR(s string) bool {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}
	return net.ParseIP(s) != nil
}

// Generate random code
func generateRandomCode(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)[:length]
}

// Mask phone number
func maskPhone(phone string) string {
	if len(phone) < 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}

// Mask email
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	name := parts[0]
	if len(name) > 2 {
		name = string(name[0]) + "***" + string(name[len(name)-1])
	}
	return name + "@" + parts[1]
}
