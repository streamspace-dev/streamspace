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

	// Enable ping monitoring for health check tests
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
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
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// HealthCheck pings the database
	mock.ExpectPing()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health", nil)

	handler.HealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.NoError(t, mock.ExpectationsWereMet())
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

	// Response format: {"status": "healthy/unhealthy", "components": {...}, "timestamp": "..."}
	assert.Equal(t, "healthy", response["status"])
	components := response["components"].(map[string]interface{})
	dbComp := components["database"].(map[string]interface{})
	assert.Equal(t, "healthy", dbComp["status"])

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

	// Response format: {"status": "healthy/unhealthy", "components": {...}, "timestamp": "..."}
	assert.Equal(t, "unhealthy", response["status"])
	components := response["components"].(map[string]interface{})
	dbComp := components["database"].(map[string]interface{})
	assert.Equal(t, "unhealthy", dbComp["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseHealth_Healthy(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock database ping
	mock.ExpectPing()

	// Mock database size query
	dbSizeRow := sqlmock.NewRows([]string{"pg_database_size"}).AddRow(1000000)
	mock.ExpectQuery(`SELECT pg_database_size`).WillReturnRows(dbSizeRow)

	// Mock table sizes query
	tableRows := sqlmock.NewRows([]string{"schemaname", "tablename", "size"}).
		AddRow("public", "sessions", 500000).
		AddRow("public", "users", 200000)
	mock.ExpectQuery(`SELECT.*schemaname.*tablename.*pg_total_relation_size`).WillReturnRows(tableRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health/database", nil)

	handler.DatabaseHealth(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["pingLatency"])

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

	// Mock state distribution query: SELECT state, COUNT(*) as count FROM sessions GROUP BY state
	stateRows := sqlmock.NewRows([]string{"state", "count"}).
		AddRow("running", 60).
		AddRow("hibernated", 30).
		AddRow("terminated", 10)
	mock.ExpectQuery(`SELECT state, COUNT\(\*\) as count FROM sessions GROUP BY state`).WillReturnRows(stateRows)

	// Mock top templates query
	templateRows := sqlmock.NewRows([]string{"template_name", "count"}).
		AddRow("ubuntu-desktop", 50).
		AddRow("debian-desktop", 30)
	mock.ExpectQuery(`SELECT template_name, COUNT\(\*\) as count FROM sessions`).WillReturnRows(templateRows)

	// Mock duration statistics query
	durationRow := sqlmock.NewRows([]string{"avg", "max"}).AddRow(3600, 7200)
	mock.ExpectQuery(`SELECT.*COALESCE.*AVG.*FROM sessions`).WillReturnRows(durationRow)

	// Mock hourly creation query
	hourlyRows := sqlmock.NewRows([]string{"hour", "count"}).
		AddRow(9, 10).
		AddRow(10, 15)
	mock.ExpectQuery(`SELECT.*EXTRACT.*HOUR.*FROM sessions`).WillReturnRows(hourlyRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/sessions", nil)

	handler.SessionMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify response structure matches handler output
	assert.NotNil(t, response["stateDistribution"])
	assert.NotNil(t, response["topTemplates"])
	assert.NotNil(t, response["duration"])
	assert.NotNil(t, response["hourlyCreation"])

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

	// Mock DAU query
	dauRow := sqlmock.NewRows([]string{"count"}).AddRow(100)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT user_id\) FROM sessions WHERE created_at >= NOW\(\) - INTERVAL '1 day'`).WillReturnRows(dauRow)

	// Mock WAU query
	wauRow := sqlmock.NewRows([]string{"count"}).AddRow(500)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT user_id\) FROM sessions WHERE created_at >= NOW\(\) - INTERVAL '7 days'`).WillReturnRows(wauRow)

	// Mock MAU query
	mauRow := sqlmock.NewRows([]string{"count"}).AddRow(1500)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT user_id\) FROM sessions WHERE created_at >= NOW\(\) - INTERVAL '30 days'`).WillReturnRows(mauRow)

	// Mock user growth query
	growthRows := sqlmock.NewRows([]string{"date", "new_users"}).
		AddRow(time.Now(), 10)
	mock.ExpectQuery(`SELECT.*DATE\(created_at\).*FROM users`).WillReturnRows(growthRows)

	// Mock top users query
	topUsersRows := sqlmock.NewRows([]string{"user_id", "session_count"}).
		AddRow("user1", 50).
		AddRow("user2", 30)
	mock.ExpectQuery(`SELECT user_id, COUNT\(\*\) as session_count FROM sessions`).WillReturnRows(topUsersRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/users", nil)

	handler.UserMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify response structure matches handler output
	assert.NotNil(t, response["activeUsers"])
	assert.NotNil(t, response["growth"])
	assert.NotNil(t, response["topUsers"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResourceMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock total allocated resources query
	totalRow := sqlmock.NewRows([]string{"cpu", "memory"}).AddRow(64.0, 128.0)
	mock.ExpectQuery(`SELECT.*COALESCE.*SUM.*FROM sessions WHERE state = 'running'`).WillReturnRows(totalRow)

	// Mock resource usage by user query
	userRows := sqlmock.NewRows([]string{"user_id", "session_count", "total_cpu", "total_memory"}).
		AddRow("user1", 5, 10.0, 20.0).
		AddRow("user2", 3, 6.0, 12.0)
	mock.ExpectQuery(`SELECT.*user_id.*COUNT.*FROM sessions WHERE state = 'running'`).WillReturnRows(userRows)

	// Mock resource waste query (hibernated sessions)
	wasteRow := sqlmock.NewRows([]string{"count", "cpu", "memory"}).AddRow(2, 4.0, 8.0)
	mock.ExpectQuery(`SELECT.*COUNT.*FROM sessions WHERE state = 'hibernated'`).WillReturnRows(wasteRow)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/resources", nil)

	handler.ResourceMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify response structure matches handler output
	assert.NotNil(t, response["allocated"])
	assert.NotNil(t, response["topUsers"])
	assert.NotNil(t, response["waste"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrometheusMetrics_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Mock session count queries
	totalSessionsRow := sqlmock.NewRows([]string{"count"}).AddRow(100)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions`).WillReturnRows(totalSessionsRow)

	runningSessionsRow := sqlmock.NewRows([]string{"count"}).AddRow(60)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = 'running'`).WillReturnRows(runningSessionsRow)

	hibernatedSessionsRow := sqlmock.NewRows([]string{"count"}).AddRow(30)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sessions WHERE state = 'hibernated'`).WillReturnRows(hibernatedSessionsRow)

	// Mock user count queries
	totalUsersRow := sqlmock.NewRows([]string{"count"}).AddRow(500)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).WillReturnRows(totalUsersRow)

	activeUsersRow := sqlmock.NewRows([]string{"count"}).AddRow(100)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT user_id\) FROM sessions`).WillReturnRows(activeUsersRow)

	// Mock templates count
	templatesRow := sqlmock.NewRows([]string{"count"}).AddRow(10)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM templates`).WillReturnRows(templatesRow)

	// Mock resource averages
	resourcesRow := sqlmock.NewRows([]string{"avg_cpu", "avg_memory"}).AddRow(2.0, 4.0)
	mock.ExpectQuery(`SELECT.*COALESCE.*AVG.*FROM sessions WHERE state = 'running'`).WillReturnRows(resourcesRow)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/metrics/prometheus", nil)

	handler.PrometheusMetrics(c)

	assert.Equal(t, http.StatusOK, w.Code)

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

	// Verify response structure matches handler output
	assert.NotNil(t, response["goVersion"])
	assert.NotNil(t, response["os"])
	assert.NotNil(t, response["arch"])
	assert.NotNil(t, response["cpus"])
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

	// Match the actual query columns from monitoring_alerts table
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "severity", "status", "condition", "threshold",
		"triggered_at", "acknowledged_at", "resolved_at", "created_at",
	}).
		AddRow(1, "High CPU Alert", "CPU usage too high", "critical", "active", "cpu > 90", 90.0, now, nil, nil, now).
		AddRow(2, "Low Disk Alert", "Disk space low", "warning", "acknowledged", "disk > 80", 80.0, now, &now, nil, now)

	mock.ExpectQuery(`SELECT.*FROM monitoring_alerts`).WillReturnRows(rows)

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

	// Match the actual query columns from monitoring_alerts table
	// Handler filters by status (not severity), so use status=active in URL
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "severity", "status", "condition", "threshold",
		"triggered_at", "acknowledged_at", "resolved_at", "created_at",
	}).
		AddRow(1, "High CPU Alert", "CPU usage too high", "critical", "active", "cpu > 90", 90.0, now, nil, nil, now)

	mock.ExpectQuery(`SELECT.*FROM monitoring_alerts.*AND status = \$1`).
		WithArgs("active").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/alerts?status=active", nil)

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

	// Handler uses ExecContext with INSERT INTO monitoring_alerts (no transaction)
	// Columns: id, name, description, severity, condition, threshold
	mock.ExpectExec(`INSERT INTO monitoring_alerts`).
		WithArgs(sqlmock.AnyArg(), "Test Alert", "This is a test alert", "critical", "cpu > 90", float64(90)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := `{
		"name": "Test Alert",
		"description": "This is a test alert",
		"severity": "critical",
		"condition": "cpu > 90",
		"threshold": 90
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

	assert.NotNil(t, response["id"])
	assert.Equal(t, "Alert created successfully", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAlert_ValidationError(t *testing.T) {
	handler, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Missing required fields: name, severity, condition, threshold
	reqBody := `{
		"description": "Missing required fields"
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

	// Match the 11 columns from the actual GetAlert query in monitoring_alerts table
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "severity", "status", "condition", "threshold",
		"triggered_at", "acknowledged_at", "resolved_at", "created_at",
	}).
		AddRow("alert_123", "Test Alert", "Test description", "critical", "active", "cpu > 90", 90.0, now, nil, nil, now)

	mock.ExpectQuery(`SELECT.*FROM monitoring_alerts.*WHERE id = \$1`).
		WithArgs("alert_123").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "alert_123"}}

	handler.GetAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Test Alert", response["name"])
	assert.Equal(t, "critical", response["severity"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAlert_NotFound(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT.*FROM monitoring_alerts.*WHERE id = \$1`).
		WithArgs("nonexistent_alert").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent_alert"}}

	handler.GetAlert(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Handler uses ExecContext with UPDATE monitoring_alerts (no transaction)
	// Args: name, description, severity, condition, threshold, id
	mock.ExpectExec(`UPDATE monitoring_alerts`).
		WithArgs("Updated Alert", "Updated description", "warning", "cpu > 80", float64(80), "alert_123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := `{
		"name": "Updated Alert",
		"description": "Updated description",
		"severity": "warning",
		"condition": "cpu > 80",
		"threshold": 80
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/monitoring/alerts/alert_123", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "alert_123"}}

	handler.UpdateAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Handler uses ExecContext DELETE (no transaction)
	mock.ExpectExec(`DELETE FROM monitoring_alerts WHERE id = \$1`).
		WithArgs("alert_123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "alert_123"}}

	handler.DeleteAlert(c)

	// Handler returns 200 OK, not 204 No Content
	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAcknowledgeAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Handler uses ExecContext UPDATE (no transaction, no subsequent SELECT)
	mock.ExpectExec(`UPDATE monitoring_alerts.*SET status = 'acknowledged'.*WHERE id = \$1`).
		WithArgs("alert_123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "alert_123"}}

	handler.AcknowledgeAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Alert acknowledged", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveAlert_Success(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Handler uses ExecContext UPDATE (no transaction, no subsequent SELECT)
	mock.ExpectExec(`UPDATE monitoring_alerts.*SET status = 'resolved'.*WHERE id = \$1`).
		WithArgs("alert_123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: "alert_123"}}

	handler.ResolveAlert(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Alert resolved", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

func TestGetAlerts_EmptyResult(t *testing.T) {
	handler, mock, cleanup := setupMonitoringTest(t)
	defer cleanup()

	// Match the actual query columns from monitoring_alerts table (empty result set)
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "severity", "status", "condition", "threshold",
		"triggered_at", "acknowledged_at", "resolved_at", "created_at",
	})

	mock.ExpectQuery(`SELECT.*FROM monitoring_alerts`).WillReturnRows(rows)

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

	// Handler doesn't check rows affected, returns 200 even if no rows matched
	// This test verifies the update executes without error
	mock.ExpectExec(`UPDATE monitoring_alerts`).
		WithArgs("Updated Alert", "Updated desc", "warning", "cpu > 70", float64(70), "nonexistent_alert").
		WillReturnResult(sqlmock.NewResult(0, 0))

	reqBody := `{
		"name": "Updated Alert",
		"description": "Updated desc",
		"severity": "warning",
		"condition": "cpu > 70",
		"threshold": 70
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/monitoring/alerts/nonexistent_alert", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent_alert"}}

	handler.UpdateAlert(c)

	// Handler returns 200 OK even when no rows are affected (doesn't validate existence)
	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}
