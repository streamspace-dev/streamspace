package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMonitoringTest(t *testing.T) (*MonitoringHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewMonitoringHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// HEALTH CHECK TESTS
// ============================================================================

func TestHealthCheck_Success(t *testing.T) {
	handler, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health", nil)

	handler.HealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestDetailedHealthCheck_AllHealthy(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock successful database ping
	mock.ExpectPing()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health/detailed", nil)

	handler.DetailedHealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["overall_status"])
	checks := response["checks"].(map[string]interface{})
	assert.Equal(t, "healthy", checks["database"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDetailedHealthCheck_DatabaseUnhealthy(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock database ping failure
	mock.ExpectPing().WillReturnError(fmt.Errorf("connection refused"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health/detailed", nil)

	handler.DetailedHealthCheck(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response["overall_status"])
	checks := response["checks"].(map[string]interface{})
	assert.Equal(t, "unhealthy", checks["database"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseHealth_Healthy(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock database ping
	mock.ExpectPing()

	// Mock stats query
	rows := sqlmock.NewRows([]string{"max_connections", "current_connections"}).
		AddRow(100, 10)
	mock.ExpectQuery("SELECT.*pg_stat_database").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health/database", nil)

	handler.DatabaseHealth(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["response_time"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseHealth_Unhealthy(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock database ping failure
	mock.ExpectPing().WillReturnError(fmt.Errorf("connection timeout"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health/database", nil)

	handler.DatabaseHealth(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response["status"])
	assert.Contains(t, response["error"].(string), "connection timeout")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// METRICS TESTS
// ============================================================================

func TestSessionMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock session counts query
	rows := sqlmock.NewRows([]string{"total", "running", "hibernated", "terminated"}).
		AddRow(100, 60, 30, 10)
	mock.ExpectQuery("SELECT.*FROM sessions").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/sessions", nil)

	handler.SessionMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(100), response["total_sessions"])
	assert.Equal(t, float64(60), response["running_sessions"])
	assert.Equal(t, float64(30), response["hibernated_sessions"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionMetrics_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery("SELECT.*FROM sessions").
		WillReturnError(fmt.Errorf("database connection lost"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/sessions", nil)

	handler.SessionMetrics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock user counts query
	rows := sqlmock.NewRows([]string{"total", "active", "inactive"}).
		AddRow(500, 350, 150)
	mock.ExpectQuery("SELECT.*FROM users").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/users", nil)

	handler.UserMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(500), response["total_users"])
	assert.Equal(t, float64(350), response["active_users"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResourceMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock resource metrics query
	rows := sqlmock.NewRows([]string{"total_cpu", "used_cpu", "total_memory", "used_memory"}).
		AddRow(64.0, 32.0, 128.0, 80.0)
	mock.ExpectQuery("SELECT.*FROM resource_usage").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/resources", nil)

	handler.ResourceMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(64.0), response["total_cpu_cores"])
	assert.Equal(t, float64(32.0), response["used_cpu_cores"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrometheusMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock metrics queries
	sessionRows := sqlmock.NewRows([]string{"total", "running", "hibernated"}).
		AddRow(100, 60, 30)
	mock.ExpectQuery("SELECT.*FROM sessions").WillReturnRows(sessionRows)

	userRows := sqlmock.NewRows([]string{"total", "active"}).
		AddRow(500, 350)
	mock.ExpectQuery("SELECT.*FROM users").WillReturnRows(userRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/prometheus", nil)

	handler.PrometheusMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "streamspace_sessions_total")
	assert.Contains(t, body, "streamspace_users_total")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// SYSTEM INFO TESTS
// ============================================================================

func TestSystemInfo_Success(t *testing.T) {
	handler, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/system/info", nil)

	handler.SystemInfo(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response["version"])
	assert.NotEmpty(t, response["go_version"])
	assert.NotEmpty(t, response["platform"])
}

func TestSystemStats_Success(t *testing.T) {
	handler, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/system/stats", nil)

	handler.SystemStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response["goroutines"])
	assert.NotNil(t, response["memory"])
	assert.NotNil(t, response["uptime"])
}

// ============================================================================
// ALERT MANAGEMENT TESTS
// ============================================================================

func TestGetAlerts_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "severity", "title", "message", "status", "created_at", "acknowledged_at", "resolved_at",
	}).
		AddRow(1, "critical", "High CPU Usage", "CPU usage above 90%", "active", now, nil, nil).
		AddRow(2, "warning", "Disk Space Low", "Disk usage above 80%", "acknowledged", now, &now, nil)

	mock.ExpectQuery("SELECT.*FROM alerts").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/alerts", nil)

	handler.GetAlerts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	alerts := response["alerts"].([]interface{})
	assert.Len(t, alerts, 2)

	firstAlert := alerts[0].(map[string]interface{})
	assert.Equal(t, "critical", firstAlert["severity"])
	assert.Equal(t, "active", firstAlert["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAlerts_WithFilters(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "severity", "title", "message", "status", "created_at",
	}).
		AddRow(1, "critical", "High CPU", "CPU at 95%", "active", now)

	mock.ExpectQuery("SELECT.*FROM alerts WHERE severity.*").
		WithArgs("critical").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/alerts?severity=critical", nil)

	handler.GetAlerts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	alerts := response["alerts"].([]interface{})
	assert.Len(t, alerts, 1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO alerts").
		WithArgs("critical", "Test Alert", "This is a test", "active").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	reqBody := `{
		"severity": "critical",
		"title": "Test Alert",
		"message": "This is a test"
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/monitoring/alerts", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateAlert(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAlert_ValidationError(t *testing.T) {
	handler, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	reqBody := `{
		"severity": "invalid",
		"title": "Test Alert"
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/monitoring/alerts", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateAlert(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "severity", "title", "message", "status", "created_at",
	}).
		AddRow(1, "critical", "Test Alert", "Test message", "active", now)

	mock.ExpectQuery("SELECT.*FROM alerts WHERE id").
		WithArgs(1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	handler.GetAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Test Alert", response["title"])
	assert.Equal(t, "critical", response["severity"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAlert_NotFound(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectQuery("SELECT.*FROM alerts WHERE id").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	handler.GetAlert(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts SET").
		WithArgs("warning", "Updated Alert", "Updated message", 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	reqBody := `{
		"severity": "warning",
		"title": "Updated Alert",
		"message": "Updated message"
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/monitoring/alerts/1", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	handler.UpdateAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM alerts WHERE id").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	handler.DeleteAlert(c)

	assert.Equal(t, http.StatusNoContent, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAcknowledgeAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts SET status.*acknowledged_at").
		WithArgs("acknowledged", sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Mock fetching updated alert
	rows := sqlmock.NewRows([]string{
		"id", "severity", "title", "message", "status", "created_at", "acknowledged_at",
	}).
		AddRow(1, "warning", "Test", "Message", "acknowledged", now, &now)
	mock.ExpectQuery("SELECT.*FROM alerts WHERE id").
		WithArgs(1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	handler.AcknowledgeAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "acknowledged", response["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts SET status.*resolved_at").
		WithArgs("resolved", sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Mock fetching updated alert
	rows := sqlmock.NewRows([]string{
		"id", "severity", "title", "message", "status", "created_at", "resolved_at",
	}).
		AddRow(1, "warning", "Test", "Message", "resolved", now, &now)
	mock.ExpectQuery("SELECT.*FROM alerts WHERE id").
		WithArgs(1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	handler.ResolveAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "resolved", response["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

func TestGetAlerts_EmptyResult(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectQuery("SELECT.*FROM alerts").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/alerts", nil)

	handler.GetAlerts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	alerts := response["alerts"].([]interface{})
	assert.Len(t, alerts, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAlert_NotFound(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts SET").
		WithArgs("warning", "Updated", "Message", 999).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	reqBody := `{
		"severity": "warning",
		"title": "Updated",
		"message": "Message"
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/monitoring/alerts/999", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	handler.UpdateAlert(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}
