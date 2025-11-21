// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests system configuration management functionality.
//
// Test Coverage:
// - ListConfigurations: All configs, category filtering
// - GetConfiguration: Success and not found scenarios
// - UpdateConfiguration: Validation, type checking, success cases
// - BulkUpdateConfigurations: Transaction handling, partial failures
//
// Testing Strategy:
// - Use sqlmock for database mocking
// - Test all configuration types (boolean, number, duration, array, enum, string, url, email)
// - Verify validation rules for each type
// - Test error handling and edge cases
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

// setupConfigurationTest creates a test environment with mocked database
func setupConfigurationTest(t *testing.T) (*ConfigurationHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Use the test constructor to inject mock database
	database := db.NewDatabaseForTesting(mockDB)

	handler := &ConfigurationHandler{
		database: database,
	}

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// LIST CONFIGURATIONS TESTS
// ============================================================================

func TestListConfigurations_Success_AllConfigs(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock configurations across multiple categories
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("ingress.domain", "streamspace.example.com", "string", "ingress", "Platform domain", timestamp, "admin").
		AddRow("storage.class", "fast-ssd", "string", "storage", "Storage class", timestamp, "admin").
		AddRow("features.hibernation", "true", "boolean", "features", "Enable hibernation", timestamp, "admin").
		AddRow("session.idle_timeout", "30m", "duration", "session", "Idle timeout", timestamp, "admin")

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration ORDER BY category, key`
	mock.ExpectQuery(query).WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	c.Request = req

	handler.ListConfigurations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ConfigurationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Configurations, 4)

	// Verify grouped structure
	assert.Len(t, response.Grouped, 4)
	assert.Len(t, response.Grouped["ingress"], 1)
	assert.Len(t, response.Grouped["storage"], 1)
	assert.Len(t, response.Grouped["features"], 1)
	assert.Len(t, response.Grouped["session"], 1)

	// Verify specific config
	assert.Equal(t, "ingress.domain", response.Configurations[0].Key)
	assert.Equal(t, "streamspace.example.com", response.Configurations[0].Value)
	assert.Equal(t, "string", response.Configurations[0].Type)
	assert.Equal(t, "ingress", response.Configurations[0].Category)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListConfigurations_Success_FilterByCategory(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock only security category configs
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("security.mfa_required", "true", "boolean", "security", "Require MFA", timestamp, "admin").
		AddRow("security.saml_enabled", "false", "boolean", "security", "Enable SAML", timestamp, "admin").
		AddRow("security.ip_whitelist", "192.168.1.0/24", "string", "security", "IP whitelist", timestamp, "admin")

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE category = \$1 ORDER BY category, key`
	mock.ExpectQuery(query).WithArgs("security").WillReturnRows(rows)

	// Create test context with category filter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/config?category=security", nil)
	c.Request = req

	handler.ListConfigurations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ConfigurationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Configurations, 3)

	// Verify all configs are from security category
	for _, config := range response.Configurations {
		assert.Equal(t, "security", config.Category)
	}

	// Verify grouped only has security
	assert.Len(t, response.Grouped, 1)
	assert.Len(t, response.Grouped["security"], 3)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListConfigurations_Success_EmptyResult(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock empty result
	rows := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"})

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration ORDER BY category, key`
	mock.ExpectQuery(query).WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	c.Request = req

	handler.ListConfigurations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ConfigurationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Configurations)
	assert.Len(t, response.Configurations, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListConfigurations_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock database error
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration ORDER BY category, key`
	mock.ExpectQuery(query).WillReturnError(fmt.Errorf("database connection failed"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	c.Request = req

	handler.ListConfigurations(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to fetch configurations", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET CONFIGURATION TESTS
// ============================================================================

func TestGetConfiguration_Success(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("features.hibernation", "true", "boolean", "features", "Enable auto-hibernation", timestamp, "admin")

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("features.hibernation").WillReturnRows(row)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "features.hibernation"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/config/features.hibernation", nil)
	c.Request = req

	handler.GetConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var config Configuration
	err := json.Unmarshal(w.Body.Bytes(), &config)
	require.NoError(t, err)
	assert.Equal(t, "features.hibernation", config.Key)
	assert.Equal(t, "true", config.Value)
	assert.Equal(t, "boolean", config.Type)
	assert.Equal(t, "features", config.Category)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetConfiguration_NotFound(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("nonexistent.key").WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "nonexistent.key"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/config/nonexistent.key", nil)
	c.Request = req

	handler.GetConfiguration(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Configuration not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetConfiguration_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("test.key").WillReturnError(fmt.Errorf("database error"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "test.key"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/config/test.key", nil)
	c.Request = req

	handler.GetConfiguration(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to retrieve configuration", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UPDATE CONFIGURATION TESTS
// ============================================================================

func TestUpdateConfiguration_Success_BooleanType(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("features.hibernation", "false", "boolean", "features", "Enable hibernation", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("features.hibernation").WillReturnRows(row)

	// Mock update
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WithArgs("true", sqlmock.AnyArg(), "", "features.hibernation").WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "features.hibernation"}}

	reqBody := UpdateConfigurationRequest{Value: "true"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/features.hibernation", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var config Configuration
	err := json.Unmarshal(w.Body.Bytes(), &config)
	require.NoError(t, err)
	assert.Equal(t, "features.hibernation", config.Key)
	assert.Equal(t, "true", config.Value)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_Success_NumberType(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("resources.max_cpu", "4000", "number", "resources", "Max CPU millicores", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("resources.max_cpu").WillReturnRows(row)

	// Mock update
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WithArgs("8000", sqlmock.AnyArg(), "", "resources.max_cpu").WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "resources.max_cpu"}}

	reqBody := UpdateConfigurationRequest{Value: "8000"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/resources.max_cpu", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var config Configuration
	err := json.Unmarshal(w.Body.Bytes(), &config)
	require.NoError(t, err)
	assert.Equal(t, "8000", config.Value)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_Success_DurationType(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("session.idle_timeout", "30m", "duration", "session", "Idle timeout", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("session.idle_timeout").WillReturnRows(row)

	// Mock update
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WithArgs("1h", sqlmock.AnyArg(), "", "session.idle_timeout").WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "session.idle_timeout"}}

	reqBody := UpdateConfigurationRequest{Value: "1h"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/session.idle_timeout", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var config Configuration
	err := json.Unmarshal(w.Body.Bytes(), &config)
	require.NoError(t, err)
	assert.Equal(t, "1h", config.Value)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_Success_URLType(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("ingress.domain", "http://localhost", "url", "ingress", "Platform URL", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("ingress.domain").WillReturnRows(row)

	// Mock update
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WithArgs("https://streamspace.example.com", sqlmock.AnyArg(), "", "ingress.domain").WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "ingress.domain"}}

	reqBody := UpdateConfigurationRequest{Value: "https://streamspace.example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/ingress.domain", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_Success_EmailType(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("compliance.contact", "old@example.com", "email", "compliance", "Contact email", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("compliance.contact").WillReturnRows(row)

	// Mock update
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WithArgs("new@example.com", sqlmock.AnyArg(), "", "compliance.contact").WillReturnResult(sqlmock.NewResult(1, 1))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "compliance.contact"}}

	reqBody := UpdateConfigurationRequest{Value: "new@example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/compliance.contact", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_ValidationError_InvalidBoolean(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("features.hibernation", "false", "boolean", "features", "Enable hibernation", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("features.hibernation").WillReturnRows(row)

	// Create test context with invalid boolean value
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "features.hibernation"}}

	reqBody := UpdateConfigurationRequest{Value: "yes"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/features.hibernation", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid value", response.Error)
	assert.Contains(t, response.Message, "boolean")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_ValidationError_InvalidNumber(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("resources.max_cpu", "4000", "number", "resources", "Max CPU", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("resources.max_cpu").WillReturnRows(row)

	// Create test context with invalid number
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "resources.max_cpu"}}

	reqBody := UpdateConfigurationRequest{Value: "not-a-number"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/resources.max_cpu", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid value", response.Error)
	assert.Contains(t, response.Message, "number")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_ValidationError_InvalidDuration(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("session.idle_timeout", "30m", "duration", "session", "Idle timeout", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("session.idle_timeout").WillReturnRows(row)

	// Create test context with invalid duration
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "session.idle_timeout"}}

	reqBody := UpdateConfigurationRequest{Value: "30 minutes"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/session.idle_timeout", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid value", response.Error)
	assert.Contains(t, response.Message, "duration")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_ValidationError_InvalidURL(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("ingress.domain", "http://localhost", "url", "ingress", "Platform URL", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("ingress.domain").WillReturnRows(row)

	// Create test context with invalid URL (no protocol)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "ingress.domain"}}

	reqBody := UpdateConfigurationRequest{Value: "streamspace.example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/ingress.domain", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid value", response.Error)
	assert.Contains(t, response.Message, "http")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_ValidationError_InvalidEmail(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("compliance.contact", "old@example.com", "email", "compliance", "Contact email", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("compliance.contact").WillReturnRows(row)

	// Create test context with invalid email
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "compliance.contact"}}

	reqBody := UpdateConfigurationRequest{Value: "not-an-email"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/compliance.contact", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid value", response.Error)
	assert.Contains(t, response.Message, "email")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_NotFound(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("nonexistent.key").WillReturnError(sql.ErrNoRows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "nonexistent.key"}}

	reqBody := UpdateConfigurationRequest{Value: "value"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/nonexistent.key", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Configuration not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateConfiguration_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Create test context with invalid JSON
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "test.key"}}

	req := httptest.NewRequest("PUT", "/api/v1/admin/config/test.key", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid request", response.Error)
}

func TestUpdateConfiguration_UpdateError(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	timestamp := time.Now()

	// Mock getting current config
	row := sqlmock.NewRows([]string{"key", "value", "type", "category", "description", "updated_at", "updated_by"}).
		AddRow("test.key", "old", "string", "test", "Test config", timestamp, "admin")
	query := `SELECT key, value, type, category, description, updated_at, updated_by FROM configuration WHERE key = \$1`
	mock.ExpectQuery(query).WithArgs("test.key").WillReturnRows(row)

	// Mock update failure
	updateQuery := `UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`
	mock.ExpectExec(updateQuery).WillReturnError(fmt.Errorf("update failed"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "key", Value: "test.key"}}

	reqBody := UpdateConfigurationRequest{Value: "new"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/admin/config/test.key", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateConfiguration(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to update configuration", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// BULK UPDATE CONFIGURATIONS TESTS
// ============================================================================

func TestBulkUpdateConfigurations_Success_AllValid(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock transaction
	mock.ExpectBegin()

	// Mock type checks for each config
	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("features.hibernation").
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("boolean"))

	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("session.idle_timeout").
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("duration"))

	// Mock updates
	mock.ExpectExec(`UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`).
		WithArgs("true", sqlmock.AnyArg(), "", "features.hibernation").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`).
		WithArgs("45m", sqlmock.AnyArg(), "", "session.idle_timeout").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := BulkUpdateRequest{
		Updates: map[string]string{
			"features.hibernation":   "true",
			"session.idle_timeout": "45m",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/config/bulk", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.BulkUpdateConfigurations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	updated := response["updated"].([]interface{})
	assert.Len(t, updated, 2)
	assert.Contains(t, updated, "features.hibernation")
	assert.Contains(t, updated, "session.idle_timeout")

	failed := response["failed"].(map[string]interface{})
	assert.Len(t, failed, 0)

	assert.Equal(t, float64(2), response["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkUpdateConfigurations_PartialSuccess(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock transaction
	mock.ExpectBegin()

	// First config - success
	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("features.hibernation").
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("boolean"))

	mock.ExpectExec(`UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`).
		WithArgs("true", sqlmock.AnyArg(), "", "features.hibernation").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Second config - not found
	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("nonexistent.key").
		WillReturnError(sql.ErrNoRows)

	// Third config - invalid value (will fail validation)
	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("resources.max_cpu").
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("number"))

	mock.ExpectCommit()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := BulkUpdateRequest{
		Updates: map[string]string{
			"features.hibernation": "true",
			"nonexistent.key":      "value",
			"resources.max_cpu":    "not-a-number",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/config/bulk", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.BulkUpdateConfigurations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	updated := response["updated"].([]interface{})
	assert.Len(t, updated, 1)
	assert.Contains(t, updated, "features.hibernation")

	failed := response["failed"].(map[string]interface{})
	assert.Len(t, failed, 2)
	assert.Contains(t, failed, "nonexistent.key")
	assert.Contains(t, failed, "resources.max_cpu")

	assert.Equal(t, float64(3), response["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkUpdateConfigurations_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Create test context with invalid JSON
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/api/v1/admin/config/bulk", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.BulkUpdateConfigurations(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid request", response.Error)
}

func TestBulkUpdateConfigurations_TransactionBeginError(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock transaction begin failure
	mock.ExpectBegin().WillReturnError(fmt.Errorf("transaction begin failed"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := BulkUpdateRequest{
		Updates: map[string]string{
			"test.key": "value",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/config/bulk", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.BulkUpdateConfigurations(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to start transaction", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkUpdateConfigurations_CommitError(t *testing.T) {
	handler, mock, cleanup := setupConfigurationTest(t)
	defer cleanup()

	// Mock transaction
	mock.ExpectBegin()

	// Mock successful update
	mock.ExpectQuery(`SELECT type FROM configuration WHERE key = \$1`).
		WithArgs("test.key").
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("string"))

	mock.ExpectExec(`UPDATE configuration SET value = \$1, updated_at = \$2, updated_by = \$3 WHERE key = \$4`).
		WithArgs("value", sqlmock.AnyArg(), "", "test.key").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock commit failure
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := BulkUpdateRequest{
		Updates: map[string]string{
			"test.key": "value",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/config/bulk", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.BulkUpdateConfigurations(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to commit changes", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}
