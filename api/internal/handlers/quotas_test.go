package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
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

func setupQuotasTest(t *testing.T) (*QuotasHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewQuotasHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// GET USER QUOTA TESTS
// ============================================================================

func TestGetUserQuota_Success(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"
	now := time.Now()

	mock.ExpectQuery(`SELECT id, user_id, max_sessions, max_cpu, max_memory, max_storage, created_at, updated_at FROM resource_quotas WHERE user_id = \$1 AND team_id IS NULL`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "max_sessions", "max_cpu", "max_memory", "max_storage", "created_at", "updated_at",
		}).AddRow("quota1", userID, 10, 4000, 8192, 100, now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/quotas/users/"+userID, nil)
	c.Request = req

	handler.GetUserQuota(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(10), response["maxSessions"])
	assert.Equal(t, float64(4000), response["maxCPU"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserQuota_Default(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"

	mock.ExpectQuery(`SELECT .+ FROM resource_quotas WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/quotas/users/"+userID, nil)
	c.Request = req

	handler.GetUserQuota(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, float64(10), response["maxSessions"])
	assert.Equal(t, true, response["isDefault"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// SET USER QUOTA TESTS
// ============================================================================

func TestSetUserQuota_Success(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"

	mock.ExpectExec(`INSERT INTO resource_quotas`).
		WithArgs(sqlmock.AnyArg(), userID, 20, 8000, 16384, 200).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}

	reqBody := map[string]interface{}{
		"maxSessions": 20,
		"maxCPU":      8000,
		"maxMemory":   16384,
		"maxStorage":  200,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/quotas/users/"+userID, bytes.NewBuffer(bodyBytes))
	c.Request = req

	handler.SetUserQuota(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET USER USAGE TESTS
// ============================================================================

func TestGetUserUsage_Success(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"

	// Active sessions count
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Resource usage
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(\(resources->>'cpu'\)::int\), 0\), COALESCE\(SUM\(\(resources->>'memory'\)::int\), 0\) FROM sessions`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"cpu", "memory"}).AddRow(2000, 4096))

	// Storage usage
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(size_bytes\), 0\) FROM session_snapshots`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"size"}).AddRow(1024 * 1024 * 1024)) // 1GB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/quotas/users/"+userID+"/usage", nil)
	c.Request = req

	handler.GetUserUsage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["activeSessions"])
	resources := response["resources"].(map[string]interface{})
	assert.Equal(t, float64(2000), resources["cpu"])
	assert.Equal(t, float64(4096), resources["memory"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET USER QUOTA STATUS TESTS
// ============================================================================

func TestGetUserQuotaStatus_Ok(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"

	// Quota
	mock.ExpectQuery(`SELECT max_sessions, max_cpu, max_memory, max_storage FROM resource_quotas`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"max_sessions", "max_cpu", "max_memory", "max_storage"}).
			AddRow(10, 10000, 20480, 100))

	// Usage queries
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT COALESCE\(SUM.+`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"cpu", "memory"}).AddRow(2000, 4096))

	mock.ExpectQuery(`SELECT COALESCE\(SUM.+`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"size"}).AddRow(10))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/quotas/users/"+userID+"/status", nil)
	c.Request = req

	handler.GetUserQuotaStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Empty(t, response["warnings"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserQuotaStatus_Exceeded(t *testing.T) {
	handler, mock, cleanup := setupQuotasTest(t)
	defer cleanup()

	userID := "user123"

	// Quota (small limits)
	mock.ExpectQuery(`SELECT max_sessions, max_cpu, max_memory, max_storage FROM resource_quotas`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"max_sessions", "max_cpu", "max_memory", "max_storage"}).
			AddRow(2, 2000, 4096, 10))

	// Usage queries (exceeding limits)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3)) // 3 > 2

	mock.ExpectQuery(`SELECT COALESCE\(SUM.+`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"cpu", "memory"}).AddRow(3000, 6144))

	mock.ExpectQuery(`SELECT COALESCE\(SUM.+`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"size"}).AddRow(10))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}
	req := httptest.NewRequest("GET", "/api/v1/quotas/users/"+userID+"/status", nil)
	c.Request = req

	handler.GetUserQuotaStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "exceeded", response["status"])
	assert.Contains(t, response["warnings"], "Session limit exceeded")
	assert.Contains(t, response["warnings"], "CPU quota exceeded")

	assert.NoError(t, mock.ExpectationsWereMet())
}
