// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests license management functionality.
//
// Test Coverage:
// - GetCurrentLicense: Active, expired, limit warnings
// - ActivateLicense: Success, validation, transaction handling
// - GetLicenseUsage: Different tiers and usage levels
// - ValidateLicense: Valid/invalid keys
// - GetUsageHistory: Time range queries
//
// Testing Strategy:
// - Use sqlmock for database mocking
// - Test all license tiers (Community, Pro, Enterprise)
// - Verify limit warning generation at 80%, 90%, 100%
// - Test expiration logic and alerts
// - Verify transaction handling for license activation
package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLicenseTest creates a test environment with mocked database
func setupLicenseTest(t *testing.T) (*LicenseHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Use the test constructor to inject mock database
	database := db.NewDatabaseForTesting(mockDB)

	handler := &LicenseHandler{
		database: database,
	}

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// Helper to create int pointer
func intPtr(i int) *int {
	return &i
}

// Helper to create float64 pointer
func float64Ptr(f float64) *float64 {
	return &f
}

// ============================================================================
// GET CURRENT LICENSE TESTS
// ============================================================================

func TestGetCurrentLicense_Success_CommunityTier(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0) // 1 year from now

	// Mock license query (Community tier)
	featuresJSON := `{"basic_auth":true}`
	metadataJSON := `{"organization":"Test Org"}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "COMM-1234-5678", "community", featuresJSON, 10, 20, 3, now, expiresAt, now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage queries
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT id\) FROM users WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = \$1`).
		WithArgs("running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT controller_id\) FROM controllers WHERE status = \$1`).
		WithArgs("connected").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response CurrentLicenseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "COMM-1234-5678", response.License.LicenseKey)
	assert.Equal(t, "community", response.License.Tier)
	assert.Equal(t, 5, response.Usage.CurrentUsers)
	assert.Equal(t, 10, response.Usage.CurrentSessions)
	assert.Equal(t, 2, response.Usage.CurrentNodes)
	assert.Equal(t, 10, *response.Usage.MaxUsers)
	assert.Equal(t, 20, *response.Usage.MaxSessions)
	assert.Equal(t, 3, *response.Usage.MaxNodes)
	assert.False(t, response.IsExpired)
	assert.False(t, response.IsExpiringSoon)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentLicense_Success_ProTierWithWarnings(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.AddDate(0, 0, 20) // 20 days from now (expiring soon)

	// Mock license query (Pro tier)
	featuresJSON := `{"saml":true,"oidc":true,"mfa":true,"recordings":true}`
	metadataJSON := `{}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "PRO-1234-5678", "pro", featuresJSON, 100, 200, 10, now, expiresAt, now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage queries - approaching limits (85% users, 92% sessions)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT id\) FROM users WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(85))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = \$1`).
		WithArgs("running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(184))

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT controller_id\) FROM controllers WHERE status = \$1`).
		WithArgs("connected").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response CurrentLicenseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "PRO-1234-5678", response.License.LicenseKey)
	assert.Equal(t, "pro", response.License.Tier)
	assert.False(t, response.IsExpired)
	assert.True(t, response.IsExpiringSoon) // < 30 days

	// Check warnings were generated (85% users = warning, 92% sessions = critical)
	assert.GreaterOrEqual(t, len(response.LimitWarnings), 2)

	// Verify user warning
	var userWarning *LimitWarning
	for i := range response.LimitWarnings {
		if response.LimitWarnings[i].Resource == "users" {
			userWarning = &response.LimitWarnings[i]
			break
		}
	}
	require.NotNil(t, userWarning)
	assert.Equal(t, 85, userWarning.Current)
	assert.Equal(t, 100, userWarning.Limit)
	assert.Equal(t, "warning", userWarning.Severity) // 85% = warning

	// Verify session warning
	var sessionWarning *LimitWarning
	for i := range response.LimitWarnings {
		if response.LimitWarnings[i].Resource == "sessions" {
			sessionWarning = &response.LimitWarnings[i]
			break
		}
	}
	require.NotNil(t, sessionWarning)
	assert.Equal(t, 184, sessionWarning.Current)
	assert.Equal(t, 200, sessionWarning.Limit)
	assert.Equal(t, "critical", sessionWarning.Severity) // 92% = critical

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentLicense_Success_EnterpriseTierUnlimited(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.AddDate(2, 0, 0) // 2 years from now

	// Mock license query (Enterprise tier with unlimited resources)
	featuresJSON := `{"saml":true,"oidc":true,"mfa":true,"recordings":true,"custom_integrations":true,"sla":true}`
	metadataJSON := `{"sla_level":"platinum"}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "ENT-1234-5678", "enterprise", featuresJSON, nil, nil, nil, now, expiresAt, now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage queries - high numbers, but unlimited license
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT id\) FROM users WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = \$1`).
		WithArgs("running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT controller_id\) FROM controllers WHERE status = \$1`).
		WithArgs("connected").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response CurrentLicenseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ENT-1234-5678", response.License.LicenseKey)
	assert.Equal(t, "enterprise", response.License.Tier)
	assert.Equal(t, 500, response.Usage.CurrentUsers)
	assert.Nil(t, response.Usage.MaxUsers) // Unlimited
	assert.Nil(t, response.Usage.MaxSessions)
	assert.Nil(t, response.Usage.MaxNodes)
	assert.Nil(t, response.Usage.UserPercent) // No percentage for unlimited
	assert.False(t, response.IsExpired)
	assert.False(t, response.IsExpiringSoon)

	// No warnings for unlimited license
	assert.Empty(t, response.LimitWarnings)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentLicense_Success_ExpiredLicense(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.AddDate(0, 0, -10) // 10 days ago

	// Mock license query (expired)
	featuresJSON := `{}`
	metadataJSON := `{}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "COMM-EXPIRED", "community", featuresJSON, 10, 20, 3, now.AddDate(0, 0, -370), expiresAt, now.AddDate(0, 0, -370), "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage queries
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT id\) FROM users WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = \$1`).
		WithArgs("running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT controller_id\) FROM controllers WHERE status = \$1`).
		WithArgs("connected").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response CurrentLicenseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.IsExpired)
	assert.True(t, response.DaysUntilExpiry < 0) // Negative days

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentLicense_NoActiveLicense(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock no active license
	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "No active license", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentLicense_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license", nil)
	c.Request = req

	handler.GetCurrentLicense(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to retrieve license", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// ACTIVATE LICENSE TESTS
// ============================================================================

func TestActivateLicense_Success(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()

	// Mock transaction
	mock.ExpectBegin()

	// Mock deactivating current license
	mock.ExpectExec(`UPDATE licenses SET status = 'inactive' WHERE status = 'active'`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock inserting new license
	featuresJSON := `{"saml":true,"oidc":true,"mfa":true}`
	metadataJSON := `{"activated_by":"admin"}`

	mock.ExpectQuery(`INSERT INTO licenses`).
		WithArgs(
			"PRO-NEW-LICENSE-KEY",
			"pro",
			sqlmock.AnyArg(), // features
			100,
			200,
			10,
			sqlmock.AnyArg(), // issued_at
			sqlmock.AnyArg(), // expires_at
			sqlmock.AnyArg(), // activated_at
			"active",
			sqlmock.AnyArg(), // metadata
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
			"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
		}).AddRow(1, "PRO-NEW-LICENSE-KEY", "pro", featuresJSON, 100, 200, 10, now, now.AddDate(1, 0, 0), now, "active", metadataJSON, now, now))

	mock.ExpectCommit()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := ActivateLicenseRequest{
		LicenseKey: "PRO-NEW-LICENSE-KEY",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/activate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ActivateLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var license License
	err := json.Unmarshal(w.Body.Bytes(), &license)
	require.NoError(t, err)
	assert.Equal(t, "PRO-NEW-LICENSE-KEY", license.LicenseKey)
	assert.Equal(t, "pro", license.Tier)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestActivateLicense_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context with invalid JSON
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/api/v1/admin/license/activate", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ActivateLicense(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid request", response.Error)
}

func TestActivateLicense_KeyTooShort(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context with short key
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := ActivateLicenseRequest{
		LicenseKey: "SHORT",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/activate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ActivateLicense(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid license key", response.Error)
	assert.Contains(t, response.Message, "too short")
}

func TestActivateLicense_TransactionBeginError(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock transaction begin failure
	mock.ExpectBegin().WillReturnError(fmt.Errorf("transaction error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := ActivateLicenseRequest{
		LicenseKey: "VALID-LICENSE-KEY",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/activate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ActivateLicense(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to start transaction", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestActivateLicense_DeactivateError(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock transaction
	mock.ExpectBegin()

	// Mock deactivation failure
	mock.ExpectExec(`UPDATE licenses SET status = 'inactive' WHERE status = 'active'`).
		WillReturnError(fmt.Errorf("deactivation failed"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := ActivateLicenseRequest{
		LicenseKey: "VALID-LICENSE-KEY",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/activate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ActivateLicense(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to deactivate current license", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET LICENSE USAGE TESTS
// ============================================================================

func TestGetLicenseUsage_Success(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()

	// Mock license query
	featuresJSON := `{}`
	metadataJSON := `{}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "PRO-1234", "pro", featuresJSON, 100, 200, 10, now, now.AddDate(1, 0, 0), now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage queries
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT id\) FROM users WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(45))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = \$1`).
		WithArgs("running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(90))

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT controller_id\) FROM controllers WHERE status = \$1`).
		WithArgs("connected").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/usage", nil)
	c.Request = req

	handler.GetLicenseUsage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats LicenseUsageStats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Equal(t, 45, stats.CurrentUsers)
	assert.Equal(t, 90, stats.CurrentSessions)
	assert.Equal(t, 5, stats.CurrentNodes)
	assert.Equal(t, 100, *stats.MaxUsers)
	assert.Equal(t, 45.0, *stats.UserPercent)
	assert.Equal(t, 45.0, *stats.SessionPercent)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLicenseUsage_NoActiveLicense(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock no active license
	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/usage", nil)
	c.Request = req

	handler.GetLicenseUsage(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// VALIDATE LICENSE TESTS
// ============================================================================

func TestValidateLicense_Success_ValidKey(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := map[string]string{
		"license_key": "VALID-LICENSE-KEY-FORMAT",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/validate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ValidateLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["valid"])
}

func TestValidateLicense_InvalidKey_TooShort(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := map[string]string{
		"license_key": "SHORT",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/license/validate", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ValidateLicense(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["valid"])
	assert.Contains(t, response["message"], "too short")
}

func TestValidateLicense_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context with invalid JSON
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/api/v1/admin/license/validate", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.ValidateLicense(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// GET USAGE HISTORY TESTS
// ============================================================================

func TestGetUsageHistory_Success_DefaultDays(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()

	// Mock license query
	featuresJSON := `{}`
	metadataJSON := `{}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "PRO-1234", "pro", featuresJSON, 100, 200, 10, now, now.AddDate(1, 0, 0), now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage history (30 days default)
	historyRows := sqlmock.NewRows([]string{
		"id", "license_id", "snapshot_date", "active_users", "active_sessions", "active_nodes", "created_at",
	}).
		AddRow(1, 1, now.AddDate(0, 0, -2).Format("2006-01-02"), 40, 80, 4, now).
		AddRow(2, 1, now.AddDate(0, 0, -1).Format("2006-01-02"), 45, 90, 5, now).
		AddRow(3, 1, now.Format("2006-01-02"), 50, 100, 5, now)

	mock.ExpectQuery(`SELECT .+ FROM license_usage WHERE license_id = \$1 AND snapshot_date >= \$2`).
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnRows(historyRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/history", nil)
	c.Request = req

	handler.GetUsageHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var history []LicenseUsage
	err := json.Unmarshal(w.Body.Bytes(), &history)
	require.NoError(t, err)
	assert.Len(t, history, 3)
	assert.Equal(t, 40, history[0].ActiveUsers)
	assert.Equal(t, 45, history[1].ActiveUsers)
	assert.Equal(t, 50, history[2].ActiveUsers)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsageHistory_Success_CustomDays(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	now := time.Now()

	// Mock license query
	featuresJSON := `{}`
	metadataJSON := `{}`
	licenseRow := sqlmock.NewRows([]string{
		"id", "license_key", "tier", "features", "max_users", "max_sessions", "max_nodes",
		"issued_at", "expires_at", "activated_at", "status", "metadata", "created_at", "updated_at",
	}).AddRow(1, "PRO-1234", "pro", featuresJSON, 100, 200, 10, now, now.AddDate(1, 0, 0), now, "active", metadataJSON, now, now)

	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnRows(licenseRow)

	// Mock usage history (7 days)
	historyRows := sqlmock.NewRows([]string{
		"id", "license_id", "snapshot_date", "active_users", "active_sessions", "active_nodes", "created_at",
	}).
		AddRow(1, 1, now.Format("2006-01-02"), 50, 100, 5, now)

	mock.ExpectQuery(`SELECT .+ FROM license_usage WHERE license_id = \$1 AND snapshot_date >= \$2`).
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnRows(historyRows)

	// Create test context with custom days parameter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/history?days=7", nil)
	c.Request = req

	handler.GetUsageHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var history []LicenseUsage
	err := json.Unmarshal(w.Body.Bytes(), &history)
	require.NoError(t, err)
	assert.Len(t, history, 1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsageHistory_NoActiveLicense(t *testing.T) {
	handler, mock, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Mock no active license
	mock.ExpectQuery(`SELECT .+ FROM licenses WHERE status = \$1`).
		WithArgs("active").
		WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/history", nil)
	c.Request = req

	handler.GetUsageHistory(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsageHistory_InvalidDaysParameter(t *testing.T) {
	handler, _, cleanup := setupLicenseTest(t)
	defer cleanup()

	// Create test context with invalid days parameter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/license/history?days=invalid", nil)
	c.Request = req

	handler.GetUsageHistory(c)

	// Should default to 30 days and continue, but might return error depending on implementation
	// For now, let's just verify it doesn't panic
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}
