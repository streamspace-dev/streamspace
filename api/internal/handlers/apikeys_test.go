// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests API key management functionality.
//
// Test Coverage:
// - CreateAPIKey: Success, validation, key generation
// - ListAllAPIKeys: Admin endpoint, pagination
// - ListAPIKeys: User endpoint, filtering
// - RevokeAPIKey: Deactivation logic
// - DeleteAPIKey: Permanent deletion
// - GetAPIKeyUsage: Usage statistics
//
// Testing Strategy:
// - Use sqlmock for database mocking
// - Test key generation and hashing
// - Verify scope and rate limit handling
// - Test expiration logic
// - Verify security constraints
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
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAPIKeyTest creates a test environment with mocked database
func setupAPIKeyTest(t *testing.T) (*APIKeyHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Use the test constructor to inject mock database
	database := db.NewDatabaseForTesting(mockDB)

	handler := &APIKeyHandler{
		db: database,
	}

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// Helper to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// ============================================================================
// CREATE API KEY TESTS
// ============================================================================

func TestCreateAPIKey_Success(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.AddDate(0, 0, 30)

	// Mock insert query
	mock.ExpectQuery(`INSERT INTO api_keys`).
		WithArgs(
			sqlmock.AnyArg(), // key_prefix
			sqlmock.AnyArg(), // key_hash
			"production-api",
			"API key for production use",
			"user123",
			sqlmock.AnyArg(), // scopes (JSON array)
			1000,
			sqlmock.AnyArg(), // expires_at
			true,
			sqlmock.AnyArg(), // created_at
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
			"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
		}).AddRow(1, "sk_abcde", "production-api", "API key for production use", "user123",
			`["sessions:read","sessions:write"]`, 1000, expiresAt, nil, 0, true, now, now))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")

	reqBody := map[string]interface{}{
		"name":        "production-api",
		"description": "API key for production use",
		"scopes":      []string{"sessions:read", "sessions:write"},
		"rateLimit":   1000,
		"expiresIn":   "30d",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateAPIKey(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify the response contains the API key (only shown once)
	apiKeyData := response["apiKey"].(map[string]interface{})
	assert.Equal(t, "production-api", apiKeyData["name"])
	assert.Equal(t, "user123", apiKeyData["userId"])

	// Verify the plaintext key is returned (starts with sk_)
	key := response["key"].(string)
	assert.Contains(t, key, "sk_")
	assert.Greater(t, len(key), 20) // Should be long cryptographic key

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAPIKey_Success_NoExpiration(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	now := time.Now()

	// Mock insert query with nil expiration
	mock.ExpectQuery(`INSERT INTO api_keys`).
		WithArgs(
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"test-key",
			"",
			"user123",
			sqlmock.AnyArg(),
			500,
			nil, // no expiration
			true,
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
			"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
		}).AddRow(1, "sk_test1", "test-key", "", "user123", `[]`, 500, nil, nil, 0, true, now, now))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")

	reqBody := map[string]interface{}{
		"name":      "test-key",
		"rateLimit": 500,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateAPIKey(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAPIKey_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context with invalid JSON
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")

	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateAPIKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid request", response.Error)
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context without name
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")

	reqBody := map[string]interface{}{
		"rateLimit": 1000,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateAPIKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAPIKey_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`INSERT INTO api_keys`).
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")

	reqBody := map[string]interface{}{
		"name":      "test-key",
		"rateLimit": 1000,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateAPIKey(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// LIST ALL API KEYS TESTS (Admin)
// ============================================================================

func TestListAllAPIKeys_Success(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	now := time.Now()

	// Mock API keys from multiple users
	rows := sqlmock.NewRows([]string{
		"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
		"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
	}).
		AddRow(1, "sk_user1a", "user1-key", "Key 1", "user1", `["sessions:read"]`, 1000, nil, now, 5, true, now, now).
		AddRow(2, "sk_user2a", "user2-key", "Key 2", "user2", `["sessions:write"]`, 500, nil, nil, 0, true, now, now).
		AddRow(3, "sk_user1b", "user1-key2", "Key 3", "user1", `["admin:all"]`, 2000, nil, now, 10, false, now, now)

	mock.ExpectQuery(`SELECT .+ FROM api_keys ORDER BY created_at DESC`).
		WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/apikeys", nil)
	c.Request = req

	handler.ListAllAPIKeys(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var apiKeys []APIKey
	err := json.Unmarshal(w.Body.Bytes(), &apiKeys)
	require.NoError(t, err)
	assert.Len(t, apiKeys, 3)
	assert.Equal(t, "sk_user1a", apiKeys[0].KeyPrefix)
	assert.Equal(t, "user1", apiKeys[0].UserID)
	assert.Equal(t, "sk_user2a", apiKeys[1].KeyPrefix)
	assert.Equal(t, "user2", apiKeys[1].UserID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAllAPIKeys_EmptyResult(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock empty result
	rows := sqlmock.NewRows([]string{
		"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
		"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
	})

	mock.ExpectQuery(`SELECT .+ FROM api_keys ORDER BY created_at DESC`).
		WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/apikeys", nil)
	c.Request = req

	handler.ListAllAPIKeys(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var apiKeys []APIKey
	err := json.Unmarshal(w.Body.Bytes(), &apiKeys)
	require.NoError(t, err)
	assert.Len(t, apiKeys, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAllAPIKeys_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`SELECT .+ FROM api_keys ORDER BY created_at DESC`).
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/apikeys", nil)
	c.Request = req

	handler.ListAllAPIKeys(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// LIST API KEYS TESTS (User)
// ============================================================================

func TestListAPIKeys_Success_UserKeys(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	now := time.Now()

	// Mock API keys for specific user
	rows := sqlmock.NewRows([]string{
		"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
		"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
	}).
		AddRow(1, "sk_test1", "production-key", "Prod key", "user123", `["sessions:read","sessions:write"]`, 1000, nil, now, 50, true, now, now).
		AddRow(2, "sk_test2", "development-key", "Dev key", "user123", `["sessions:read"]`, 500, nil, nil, 0, true, now, now)

	mock.ExpectQuery(`SELECT .+ FROM api_keys WHERE user_id = \$1 ORDER BY created_at DESC`).
		WithArgs("user123").
		WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")
	req := httptest.NewRequest("GET", "/api/v1/apikeys", nil)
	c.Request = req

	handler.ListAPIKeys(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var apiKeys []APIKey
	err := json.Unmarshal(w.Body.Bytes(), &apiKeys)
	require.NoError(t, err)
	assert.Len(t, apiKeys, 2)
	assert.Equal(t, "production-key", apiKeys[0].Name)
	assert.Equal(t, 50, apiKeys[0].UseCount)
	assert.Equal(t, "development-key", apiKeys[1].Name)
	assert.Equal(t, 0, apiKeys[1].UseCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAPIKeys_NoUserID(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context without userID
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/apikeys", nil)
	c.Request = req

	handler.ListAPIKeys(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAPIKeys_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`SELECT .+ FROM api_keys WHERE user_id = \$1 ORDER BY created_at DESC`).
		WithArgs("user123").
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", "user123")
	req := httptest.NewRequest("GET", "/api/v1/apikeys", nil)
	c.Request = req

	handler.ListAPIKeys(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// REVOKE API KEY TESTS
// ============================================================================

func TestRevokeAPIKey_Success(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock revoke update
	mock.ExpectExec(`UPDATE api_keys SET is_active = false, updated_at = \$1 WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("PUT", "/api/v1/apikeys/1/revoke", nil)
	c.Request = req

	handler.RevokeAPIKey(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "API key revoked successfully", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeAPIKey_InvalidID(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context with invalid ID
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "invalid"}}
	req := httptest.NewRequest("PUT", "/api/v1/apikeys/invalid/revoke", nil)
	c.Request = req

	handler.RevokeAPIKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRevokeAPIKey_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectExec(`UPDATE api_keys SET is_active = false, updated_at = \$1 WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("PUT", "/api/v1/apikeys/1/revoke", nil)
	c.Request = req

	handler.RevokeAPIKey(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DELETE API KEY TESTS
// ============================================================================

func TestDeleteAPIKey_Success(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock delete
	mock.ExpectExec(`DELETE FROM api_keys WHERE id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/1", nil)
	c.Request = req

	handler.DeleteAPIKey(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "API key deleted successfully", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAPIKey_InvalidID(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context with invalid ID
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "invalid"}}
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/invalid", nil)
	c.Request = req

	handler.DeleteAPIKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAPIKey_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock no rows affected (key doesn't exist)
	mock.ExpectExec(`DELETE FROM api_keys WHERE id = \$1`).
		WithArgs(999).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/999", nil)
	c.Request = req

	handler.DeleteAPIKey(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAPIKey_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectExec(`DELETE FROM api_keys WHERE id = \$1`).
		WithArgs(1).
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/1", nil)
	c.Request = req

	handler.DeleteAPIKey(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET API KEY USAGE TESTS
// ============================================================================

func TestGetAPIKeyUsage_Success(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	now := time.Now()

	// Mock API key usage query
	row := sqlmock.NewRows([]string{
		"id", "key_prefix", "name", "description", "user_id", "scopes", "rate_limit",
		"expires_at", "last_used_at", "use_count", "is_active", "created_at", "updated_at",
	}).AddRow(1, "sk_test1", "production-key", "Prod key", "user123", `["sessions:read","sessions:write"]`, 1000, nil, now, 150, true, now, now)

	mock.ExpectQuery(`SELECT .+ FROM api_keys WHERE id = \$1`).
		WithArgs(1).
		WillReturnRows(row)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("GET", "/api/v1/apikeys/1/usage", nil)
	c.Request = req

	handler.GetAPIKeyUsage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(150), response["useCount"])
	assert.Equal(t, float64(1000), response["rateLimit"])
	assert.NotNil(t, response["lastUsedAt"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAPIKeyUsage_InvalidID(t *testing.T) {
	handler, _, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Create test context with invalid ID
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "invalid"}}
	req := httptest.NewRequest("GET", "/api/v1/apikeys/invalid/usage", nil)
	c.Request = req

	handler.GetAPIKeyUsage(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAPIKeyUsage_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock not found
	mock.ExpectQuery(`SELECT .+ FROM api_keys WHERE id = \$1`).
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	req := httptest.NewRequest("GET", "/api/v1/apikeys/999/usage", nil)
	c.Request = req

	handler.GetAPIKeyUsage(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAPIKeyUsage_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupAPIKeyTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`SELECT .+ FROM api_keys WHERE id = \$1`).
		WithArgs(1).
		WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	req := httptest.NewRequest("GET", "/api/v1/apikeys/1/usage", nil)
	c.Request = req

	handler.GetAPIKeyUsage(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}
