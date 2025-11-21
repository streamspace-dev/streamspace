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
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserTest(t *testing.T) (*UserHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	userDB := db.NewUserDB(mockDB)
	groupDB := db.NewGroupDB(mockDB)

	handler := NewUserHandler(userDB, groupDB)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// LIST USERS TESTS
// ============================================================================

func TestListUsers_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "username", "email", "full_name", "role", "provider", "active", "created_at", "updated_at", "last_login",
	}).
		AddRow("user1", "alice", "alice@example.com", "Alice Smith", "user", "local", true, now, now, nil).
		AddRow("user2", "bob", "bob@example.com", "Bob Jones", "admin", "local", true, now, now, nil)

	mock.ExpectQuery(`SELECT id, username, email, COALESCE\(full_name, ''\), COALESCE\(role, 'user'\), COALESCE\(provider, 'local'\), COALESCE\(active, true\), created_at, updated_at, last_login FROM users WHERE 1=1 ORDER BY username ASC`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	c.Request = req

	handler.ListUsers(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])
	users := response["users"].([]interface{})
	assert.Len(t, users, 2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListUsers_FilterByRole(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "username", "email", "full_name", "role", "provider", "active", "created_at", "updated_at", "last_login",
	}).
		AddRow("user2", "bob", "bob@example.com", "Bob Jones", "admin", "local", true, time.Now(), time.Now(), nil)

	mock.ExpectQuery(`SELECT .+ FROM users WHERE 1=1 AND role = \$1 ORDER BY username ASC`).
		WithArgs("admin").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/users?role=admin", nil)
	c.Request = req

	handler.ListUsers(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListUsers_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT .+ FROM users`).
		WillReturnError(fmt.Errorf("database error"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	c.Request = req

	handler.ListUsers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// CREATE USER TESTS
// ============================================================================

func TestCreateUser_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	// Expect user insert
	mock.ExpectExec(`INSERT INTO users`).
		WithArgs(
			sqlmock.AnyArg(), // id
			"charlie",
			"charlie@example.com",
			"Charlie Brown",
			"user",
			"local",
			sqlmock.AnyArg(), // password_hash
			true,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect default quota creation
	mock.ExpectExec(`INSERT INTO user_quotas`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect adding to all_users group
	mock.ExpectExec(`INSERT INTO group_memberships`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := models.CreateUserRequest{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "securepassword123",
		FullName: "Charlie Brown",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(bodyBytes))
	c.Request = req

	handler.CreateUser(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUser_InvalidPassword(t *testing.T) {
	handler, _, cleanup := setupUserTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := models.CreateUserRequest{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "short", // Too short
		FullName: "Charlie Brown",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(bodyBytes))
	c.Request = req

	handler.CreateUser(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// GET USER TESTS
// ============================================================================

func TestGetUser_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	userID := "user123"
	now := time.Now()

	// Expect user query
	mock.ExpectQuery(`SELECT id, username, email, full_name, role, provider, active, created_at, updated_at, last_login FROM users WHERE id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "full_name", "role", "provider", "active", "created_at", "updated_at", "last_login",
		}).AddRow(userID, "alice", "alice@example.com", "Alice Smith", "user", "local", true, now, now, nil))

	// Expect quota query
	mock.ExpectQuery(`SELECT .+ FROM user_quotas WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "max_sessions", "max_cpu", "max_memory", "max_storage",
			"used_sessions", "used_cpu", "used_memory", "used_storage",
			"created_at", "updated_at",
		}).AddRow(userID, 10, "4000m", "8Gi", "100Gi", 0, "0", "0", "0", now, now))

	// Expect groups query
	mock.ExpectQuery(`SELECT g.id FROM groups g JOIN group_memberships gm`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("group1"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/users/"+userID, nil)
	c.Request = req

	handler.GetUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUser_NotFound(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	userID := "user123"

	mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/users/"+userID, nil)
	c.Request = req

	handler.GetUser(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UPDATE USER TESTS
// ============================================================================

func TestUpdateUser_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	userID := "user123"
	newEmail := "newalice@example.com"

	// Expect update
	mock.ExpectExec(`UPDATE users SET email = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(newEmail, sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect fetch updated user
	mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "full_name", "role", "provider", "active", "created_at", "updated_at", "last_login",
		}).AddRow(userID, "alice", newEmail, "Alice Smith", "user", "local", true, time.Now(), time.Now(), nil))

	// Expect quota query (part of GetUser)
	mock.ExpectQuery(`SELECT .+ FROM user_quotas WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "max_sessions", "max_cpu", "max_memory", "max_storage",
			"used_sessions", "used_cpu", "used_memory", "used_storage",
			"created_at", "updated_at",
		}).AddRow(userID, 10, "4000m", "8Gi", "100Gi", 0, "0", "0", "0", time.Now(), time.Now()))

	// Expect groups query (part of GetUser)
	mock.ExpectQuery(`SELECT g.id FROM groups g JOIN group_memberships gm`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: userID}}

	reqBody := models.UpdateUserRequest{
		Email: &newEmail,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PATCH", "/api/v1/users/"+userID, bytes.NewBuffer(bodyBytes))
	c.Request = req

	handler.UpdateUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DELETE USER TESTS
// ============================================================================

func TestDeleteUser_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	userID := "user123"

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM user_quotas WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM group_memberships WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM users WHERE id = \$1`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: userID}}
	req := httptest.NewRequest("DELETE", "/api/v1/users/"+userID, nil)
	c.Request = req

	handler.DeleteUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// CURRENT USER TESTS
// ============================================================================

func TestGetCurrentUser_Success(t *testing.T) {
	handler, mock, cleanup := setupUserTest(t)
	defer cleanup()

	userID := "user123"

	// Expect user query
	mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "full_name", "role", "provider", "active", "created_at", "updated_at", "last_login",
		}).AddRow(userID, "alice", "alice@example.com", "Alice Smith", "user", "local", true, time.Now(), time.Now(), nil))

	// Expect quota query
	mock.ExpectQuery(`SELECT .+ FROM user_quotas WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "max_sessions", "max_cpu", "max_memory", "max_storage",
			"used_sessions", "used_cpu", "used_memory", "used_storage",
			"created_at", "updated_at",
		}).AddRow(userID, 10, "4000m", "8Gi", "100Gi", 0, "0", "0", "0", time.Now(), time.Now()))

	// Expect groups query
	mock.ExpectQuery(`SELECT g.id FROM groups g JOIN group_memberships gm`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	c.Request = req

	handler.GetCurrentUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCurrentUser_Unauthorized(t *testing.T) {
	handler, _, cleanup := setupUserTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No userID in context
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	c.Request = req

	handler.GetCurrentUser(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
