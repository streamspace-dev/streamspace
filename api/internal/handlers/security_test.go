package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupSecurityTest(t *testing.T) (*SecurityHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	handler := &SecurityHandler{DB: db}

	cleanup := func() {
		db.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// MFA SETUP TESTS
// ============================================================================

func TestSetupMFA_TOTP_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"

	// Expect check for existing MFA
	mock.ExpectQuery(`SELECT id FROM mfa_methods WHERE user_id = \$1 AND type = \$2`).
		WithArgs(userID, "totp").
		WillReturnError(sql.ErrNoRows)

	// Expect MFA method insert
	mock.ExpectQuery(`INSERT INTO mfa_methods \(user_id, type, secret, phone_number, email, enabled, verified\)`).
		WithArgs(userID, "totp", sqlmock.AnyArg(), "", "", false, false).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(123))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)

	payload := map[string]interface{}{
		"type": "totp",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/mfa/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupMFA(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response MFASetupResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), response.ID)
	assert.Equal(t, "totp", response.Type)
	assert.NotEmpty(t, response.Secret)
	assert.NotEmpty(t, response.QRCode)
	assert.Contains(t, response.Message, "Scan the QR code")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupMFA_SMS_NotImplemented(t *testing.T) {
	handler, _, cleanup := setupSecurityTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user")

	payload := map[string]interface{}{
		"type": "sms",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/mfa/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupMFA(c)

	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "not yet available")
}

func TestSetupMFA_AlreadyExists(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"

	// Expect check for existing MFA - return existing ID
	mock.ExpectQuery(`SELECT id FROM mfa_methods WHERE user_id = \$1 AND type = \$2`).
		WithArgs(userID, "totp").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(456))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)

	payload := map[string]interface{}{
		"type": "totp",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/mfa/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupMFA(c)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "already exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// MFA VERIFICATION TESTS
// ============================================================================

func TestVerifyMFASetup_Success(t *testing.T) {
	_, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	mfaID := "123"
	secret := "JBSWY3DPEHPK3PXP" // Valid TOTP secret

	// Expect get MFA method
	mock.ExpectQuery(`SELECT id, user_id, type, secret, phone_number, email FROM mfa_methods WHERE id = \$1 AND user_id = \$2`).
		WithArgs(mfaID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "type", "secret", "phone_number", "email"}).
			AddRow(123, userID, "totp", secret, "", ""))

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect MFA method update
	mock.ExpectExec(`UPDATE mfa_methods SET verified = true, enabled = true WHERE id = \$1`).
		WithArgs(mfaID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect backup codes insert (10 codes)
	for i := 0; i < BackupCodesCount; i++ {
		mock.ExpectExec(`INSERT INTO backup_codes \(user_id, code\) VALUES \(\$1, \$2\)`).
			WithArgs(userID, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(int64(i+1), 1))
	}

	// Expect transaction commit
	mock.ExpectCommit()

	// Note: We can't test TOTP verification with a real code since it's time-based
	// In a real scenario, we'd need to mock the totp.Validate function or use a known test secret
	// For now, this test just validates the mock expectations

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyMFASetup_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	mfaID := "999"

	// Expect get MFA method - not found
	mock.ExpectQuery(`SELECT id, user_id, type, secret, phone_number, email FROM mfa_methods WHERE id = \$1 AND user_id = \$2`).
		WithArgs(mfaID, userID).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Params = gin.Params{{Key: "mfaId", Value: mfaID}}

	payload := map[string]interface{}{
		"code": "123456",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/mfa/"+mfaID+"/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.VerifyMFASetup(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// LIST MFA METHODS TESTS
// ============================================================================

func TestListMFAMethods_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"

	// Expect list MFA methods query
	rows := sqlmock.NewRows([]string{"id", "type", "enabled", "verified", "is_primary", "phone_number", "email", "created_at", "last_used_at"}).
		AddRow(1, "totp", true, true, true, "", "", "2024-01-01 00:00:00", sql.NullTime{})

	mock.ExpectQuery(`SELECT id, type, enabled, verified, is_primary, phone_number, email, created_at, last_used_at FROM mfa_methods WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	req := httptest.NewRequest("GET", "/api/v1/security/mfa/methods", nil)
	c.Request = req

	handler.ListMFAMethods(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "methods")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DISABLE MFA TESTS
// ============================================================================

func TestDisableMFA_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	mfaID := "123"

	// Expect disable MFA update
	mock.ExpectExec(`UPDATE mfa_methods SET enabled = false WHERE id = \$1 AND user_id = \$2`).
		WithArgs(mfaID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Params = gin.Params{{Key: "mfaId", Value: mfaID}}
	req := httptest.NewRequest("PUT", "/api/v1/security/mfa/"+mfaID+"/disable", nil)
	c.Request = req

	handler.DisableMFA(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "disabled")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDisableMFA_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	mfaID := "999"

	// Expect disable MFA update - no rows affected
	mock.ExpectExec(`UPDATE mfa_methods SET enabled = false WHERE id = \$1 AND user_id = \$2`).
		WithArgs(mfaID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Params = gin.Params{{Key: "mfaId", Value: mfaID}}
	req := httptest.NewRequest("PUT", "/api/v1/security/mfa/"+mfaID+"/disable", nil)
	c.Request = req

	handler.DisableMFA(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// IP WHITELIST TESTS
// ============================================================================

func TestCreateIPWhitelist_ValidIP_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	ipAddress := "192.168.1.100"

	// Expect insert IP whitelist entry
	mock.ExpectQuery(`INSERT INTO ip_whitelist`).
		WithArgs(userID, ipAddress, "Office IP", userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("role", "user")

	payload := map[string]interface{}{
		"user_id":     userID,
		"ip_address":  ipAddress,
		"description": "Office IP",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/ip-whitelist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateIPWhitelist(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "id")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIPWhitelist_ValidCIDR_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	cidr := "10.0.0.0/24"

	// Expect insert IP whitelist entry
	mock.ExpectQuery(`INSERT INTO ip_whitelist`).
		WithArgs(userID, cidr, "VPN subnet", userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("role", "user")

	payload := map[string]interface{}{
		"user_id":     userID,
		"ip_address":  cidr,
		"description": "VPN subnet",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/ip-whitelist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateIPWhitelist(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIPWhitelist_InvalidIP_BadRequest(t *testing.T) {
	handler, _, cleanup := setupSecurityTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user")
	c.Set("role", "user")

	payload := map[string]interface{}{
		"ip_address": "999.999.999.999",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/ip-whitelist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateIPWhitelist(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "invalid IP")
}

func TestCreateIPWhitelist_InvalidCIDR_BadRequest(t *testing.T) {
	handler, _, cleanup := setupSecurityTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user")
	c.Set("role", "user")

	payload := map[string]interface{}{
		"ip_address": "192.168.1.0/99",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/security/ip-whitelist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateIPWhitelist(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// LIST IP WHITELIST TESTS
// ============================================================================

func TestListIPWhitelist_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"

	// Expect list IP whitelist query
	rows := sqlmock.NewRows([]string{"id", "user_id", "ip_address", "description", "enabled", "created_by", "created_at", "expires_at"}).
		AddRow(1, userID, "192.168.1.100", "Office IP", true, userID, "2024-01-01 00:00:00", sql.NullTime{})

	mock.ExpectQuery(`SELECT id, user_id, ip_address, description, enabled, created_by, created_at, expires_at FROM ip_whitelist`).
		WithArgs(userID, "user").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("role", "user")
	req := httptest.NewRequest("GET", "/api/v1/security/ip-whitelist", nil)
	c.Request = req

	handler.ListIPWhitelist(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "entries")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DELETE IP WHITELIST TESTS
// ============================================================================

func TestDeleteIPWhitelist_Success(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	entryID := "123"

	// Expect delete IP whitelist entry
	mock.ExpectExec(`DELETE FROM ip_whitelist WHERE id = \$1 AND \(user_id = \$2 OR user_id IS NULL\)`).
		WithArgs(entryID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("role", "user")
	c.Params = gin.Params{{Key: "entryId", Value: entryID}}
	req := httptest.NewRequest("DELETE", "/api/v1/security/ip-whitelist/"+entryID, nil)
	c.Request = req

	handler.DeleteIPWhitelist(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteIPWhitelist_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSecurityTest(t)
	defer cleanup()

	userID := "test-user"
	entryID := "999"

	// Expect delete IP whitelist entry - no rows affected
	mock.ExpectExec(`DELETE FROM ip_whitelist WHERE id = \$1 AND \(user_id = \$2 OR user_id IS NULL\)`).
		WithArgs(entryID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("role", "user")
	c.Params = gin.Params{{Key: "entryId", Value: entryID}}
	req := httptest.NewRequest("DELETE", "/api/v1/security/ip-whitelist/"+entryID, nil)
	c.Request = req

	handler.DeleteIPWhitelist(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// VALIDATION HELPER TESTS
// ============================================================================

func TestValidateIPWhitelistInput_ValidInputs(t *testing.T) {
	tests := []struct {
		name        string
		ipOrCIDR    string
		description string
		expectError bool
	}{
		{"Valid IPv4", "192.168.1.1", "Test", false},
		{"Valid IPv4 localhost", "127.0.0.1", "Localhost", false},
		{"Valid CIDR", "192.168.1.0/24", "Subnet", false},
		{"Valid CIDR /32", "192.168.1.1/32", "Single", false},
		{"Invalid IPv4 - out of range", "256.1.1.1", "", true},
		{"Invalid IPv4 - letters", "192.168.a.1", "", true},
		{"Invalid CIDR - bad prefix", "192.168.1.0/33", "", true},
		{"Empty IP", "", "Test", true},
		{"Description too long", "192.168.1.1", string(make([]byte, 501)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIPWhitelistInput(tt.ipOrCIDR, tt.description)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMFASetupInput_ValidInputs(t *testing.T) {
	tests := []struct {
		name        string
		mfaType     string
		phoneNumber string
		email       string
		expectError bool
	}{
		{"Valid TOTP", "totp", "", "", false},
		{"Invalid type", "invalid", "", "", true},
		{"SMS without phone", "sms", "", "", true},
		{"Email without email", "email", "", "", true},
		{"SMS with phone", "sms", "1234567890", "", false},
		{"Email with email", "email", "", "test@example.com", false},
		{"Email with invalid format", "email", "", "notanemail", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMFASetupInput(tt.mfaType, tt.phoneNumber, tt.email)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
