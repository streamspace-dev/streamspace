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
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSetupTest(t *testing.T) (*SetupHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSetupHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// SETUP STATUS TESTS
// ============================================================================

func TestGetSetupStatus_SetupRequired(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Admin exists but has no password
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/auth/setup/status", nil)
	c.Request = req

	handler.GetSetupStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SetupStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.SetupRequired)
	assert.True(t, response.AdminExists)
	assert.False(t, response.HasPassword)
	assert.Contains(t, response.Message, "Setup wizard is available")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSetupStatus_AdminNotCreated(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Admin user doesn't exist yet
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/auth/setup/status", nil)
	c.Request = req

	handler.GetSetupStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SetupStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.SetupRequired)
	assert.False(t, response.AdminExists)
	assert.False(t, response.HasPassword)
	assert.Contains(t, response.Message, "admin user not created yet")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSetupStatus_AlreadyConfigured(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Admin exists and has password
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).
			AddRow(sql.NullString{String: "$2a$10$hashedpassword", Valid: true}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/auth/setup/status", nil)
	c.Request = req

	handler.GetSetupStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SetupStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.SetupRequired)
	assert.True(t, response.AdminExists)
	assert.True(t, response.HasPassword)
	assert.Contains(t, response.Message, "already configured")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// SETUP ADMIN TESTS
// ============================================================================

func TestSetupAdmin_Success(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Check setup is required
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	// Begin transaction
	mock.ExpectBegin()

	// Update admin user
	mock.ExpectExec(`UPDATE users SET password_hash = \$1, email = \$2, updated_at = CURRENT_TIMESTAMP WHERE id = 'admin' AND \(password_hash IS NULL OR password_hash = ''\)`).
		WithArgs(sqlmock.AnyArg(), "admin@example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Commit transaction
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "securepassword123",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SetupAdminResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "admin", response.Username)
	assert.Equal(t, "admin@example.com", response.Email)
	assert.Contains(t, response.Message, "configured successfully")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_AlreadyConfigured(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Admin already has password
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).
			AddRow(sql.NullString{String: "$2a$10$hashedpassword", Valid: true}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "securepassword123",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_PasswordMismatch(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Setup is required
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "differentpassword",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Passwords do not match")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_WeakPassword(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Setup is required
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "short", // Too short
		PasswordConfirm: "short",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least 12 characters")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_InvalidEmail(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Setup is required
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "securepassword123",
		Email:           "invalid-email",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request format")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_RaceCondition(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Setup is required
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(sql.NullString{Valid: false}))

	// Begin transaction
	mock.ExpectBegin()

	// Update returns 0 rows (another request already set it)
	mock.ExpectExec(`UPDATE users SET password_hash = \$1, email = \$2, updated_at = CURRENT_TIMESTAMP WHERE id = 'admin' AND \(password_hash IS NULL OR password_hash = ''\)`).
		WithArgs(sqlmock.AnyArg(), "admin@example.com").
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "securepassword123",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "already configured")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupAdmin_AdminNotExists(t *testing.T) {
	handler, mock, cleanup := setupSetupTest(t)
	defer cleanup()

	// Admin doesn't exist yet
	mock.ExpectQuery(`SELECT password_hash FROM users WHERE id = 'admin'`).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := SetupAdminRequest{
		Password:        "securepassword123",
		PasswordConfirm: "securepassword123",
		Email:           "admin@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.SetupAdmin(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "admin user not created yet")
	assert.NoError(t, mock.ExpectationsWereMet())
}
