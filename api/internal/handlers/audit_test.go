// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests audit log retrieval and export functionality.
//
// Test Coverage:
// - ListAuditLogs: Pagination, filtering, edge cases
// - GetAuditLog: Success and not found scenarios
// - ExportAuditLogs: CSV and JSON export formats
//
// Testing Strategy:
// - Use sqlmock for database mocking
// - Test all query parameters and filters
// - Verify response formats and status codes
// - Test error handling and edge cases
package handlers

import (
	"database/sql"
	"encoding/csv"
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

// setupAuditTest creates a test environment with mocked database
func setupAuditTest(t *testing.T) (*AuditHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Use the new test constructor to inject mock database
	database := db.NewDatabaseForTesting(mockDB)

	handler := &AuditHandler{
		database: database,
	}

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// LIST AUDIT LOGS TESTS
// ============================================================================

func TestListAuditLogs_Success_DefaultPagination(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(250))

	// Mock logs query with default pagination (page=1, page_size=100)
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{"old":"val1"}`, timestamp, "192.168.1.1").
		AddRow(2, "user2", "POST", "/api/users", "user2", `{"new":"val2"}`, timestamp, "192.168.1.2")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log`).
		WillReturnRows(rows)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(250), response.Total)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 100, response.PageSize)
	assert.Equal(t, 3, response.TotalPages) // 250 / 100 = 3 pages
	assert.Len(t, response.Logs, 2)
	assert.Equal(t, int64(1), response.Logs[0].ID)
	assert.Equal(t, "user1", response.Logs[0].UserID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Success_CustomPagination(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))

	// Mock logs query with custom pagination (page=2, page_size=50)
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(51, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 50). // page 2 with page_size 50 = offset 50
		WillReturnRows(rows)

	// Create test context with pagination params
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?page=2&page_size=50", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(500), response.Total)
	assert.Equal(t, 2, response.Page)
	assert.Equal(t, 50, response.PageSize)
	assert.Equal(t, 10, response.TotalPages) // 500 / 50 = 10 pages

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Success_WithUserIDFilter(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock count query with WHERE clause
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log WHERE user_id = \$1`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	// Mock logs query with user_id filter
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "testuser", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE user_id = \$1`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Create test context with filter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?user_id=testuser", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), response.Total)
	assert.Len(t, response.Logs, 1)
	assert.Equal(t, "testuser", response.Logs[0].UserID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Success_WithActionFilter(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock count query with action filter
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log WHERE action = \$1`).
		WithArgs("POST").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	// Mock logs query
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "POST", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE action = \$1`).
		WithArgs("POST").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?action=POST", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Logs, 1)
	assert.Equal(t, "POST", response.Logs[0].Action)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Success_WithDateRangeFilter(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	startDate := "2025-01-01T00:00:00Z"
	endDate := "2025-01-31T23:59:59Z"

	// Mock count query with date range
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log WHERE timestamp >= \$1 AND timestamp <= \$2`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	// Mock logs query
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE timestamp >= \$1 AND timestamp <= \$2`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/admin/audit?start_date=%s&end_date=%s", startDate, endDate), nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), response.Total)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Success_WithMultipleFilters(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock count query with multiple filters
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log WHERE user_id = \$1 AND action = \$2 AND resource_type = \$3`).
		WithArgs("testuser", "POST", "/api/sessions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	// Mock logs query
	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "testuser", "POST", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE user_id = \$1 AND action = \$2 AND resource_type = \$3`).
		WithArgs("testuser", "POST", "/api/sessions").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?user_id=testuser&action=POST&resource_type=/api/sessions", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), response.Total)
	assert.Len(t, response.Logs, 1)
	assert.Equal(t, "testuser", response.Logs[0].UserID)
	assert.Equal(t, "POST", response.Logs[0].Action)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_EdgeCase_InvalidPage(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Invalid page should default to 1
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?page=0", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 1, response.Page) // Should default to page 1

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_EdgeCase_PageSizeExceedsMax(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Page size > 1000 should default to 100
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit?page_size=2000", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLogListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 100, response.PageSize) // Should cap at 100

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAuditLogs_Error_DatabaseFailure(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	// Mock database error
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnError(sql.ErrConnDone)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit", nil)
	c.Request = req

	handler.ListAuditLogs(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET AUDIT LOG TESTS
// ============================================================================

func TestGetAuditLog_Success(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	timestamp := time.Now()
	row := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(123, "testuser", "POST", "/api/sessions", "sess1", `{"key":"value"}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE id = \$1`).
		WithArgs("123").
		WillReturnRows(row)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "123"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/123", nil)
	c.Request = req

	handler.GetAuditLog(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuditLog
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), response.ID)
	assert.Equal(t, "testuser", response.UserID)
	assert.Equal(t, "POST", response.Action)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditLog_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log WHERE id = \$1`).
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/999", nil)
	c.Request = req

	handler.GetAuditLog(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditLog_InvalidID(t *testing.T) {
	handler, _, cleanup := setupAuditTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/invalid", nil)
	c.Request = req

	handler.GetAuditLog(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// EXPORT AUDIT LOGS TESTS
// ============================================================================

func TestExportAuditLogs_JSON_Success(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1").
		AddRow(2, "user2", "POST", "/api/users", "user2", `{}`, timestamp, "192.168.1.2")

	// Export has max limit of 100,000 records
	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log LIMIT 100000`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/export?format=json", nil)
	c.Request = req

	handler.ExportAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "audit_logs_")

	var logs []AuditLog
	err := json.Unmarshal(w.Body.Bytes(), &logs)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	assert.Equal(t, int64(1), logs[0].ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExportAuditLogs_CSV_Success(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1").
		AddRow(2, "user2", "POST", "/api/users", "user2", `{}`, timestamp, "192.168.1.2")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log LIMIT 100000`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/export?format=csv", nil)
	c.Request = req

	handler.ExportAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	// Parse CSV
	reader := csv.NewReader(w.Body)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 3) // Header + 2 data rows
	assert.Equal(t, "ID", records[0][0])
	assert.Equal(t, "1", records[1][0])
	assert.Equal(t, "2", records[2][0])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExportAuditLogs_DefaultFormat_JSON(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
		AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log LIMIT 100000`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/export", nil) // No format param
	c.Request = req

	handler.ExportAuditLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type")) // Should default to JSON

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExportAuditLogs_InvalidFormat(t *testing.T) {
	handler, _, cleanup := setupAuditTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/export?format=xml", nil)
	c.Request = req

	handler.ExportAuditLogs(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportAuditLogs_Error_DatabaseFailure(t *testing.T) {
	handler, mock, cleanup := setupAuditTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log LIMIT 100000`).
		WillReturnError(sql.ErrConnDone)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit/export?format=json", nil)
	c.Request = req

	handler.ExportAuditLogs(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkListAuditLogs(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler, mock, cleanup := setupAuditTest(&testing.T{})
	defer cleanup()

	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
		rows := sqlmock.NewRows([]string{"id", "user_id", "action", "resource_type", "resource_id", "changes", "timestamp", "ip_address"}).
			AddRow(1, "user1", "GET", "/api/sessions", "sess1", `{}`, timestamp, "192.168.1.1")
		mock.ExpectQuery(`SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address FROM audit_log`).
			WillReturnRows(rows)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/api/v1/admin/audit", nil)
		c.Request = req

		handler.ListAuditLogs(c)
	}
}
