// Package handlers provides HTTP handlers for the StreamSpace API.
//
// This file contains tests for the SessionActivity handler (session activity logging).
//
// Test Coverage:
//   - LogActivityEvent (success, validation, defaults)
//   - GetSessionActivity (pagination, filtering by type/category)
//   - GetActivityStats (event types, categories, totals)
//   - GetSessionTimeline (timeline view, duration calculation)
//   - GetUserSessionActivity (user-specific activity, pagination)
//   - Error handling and edge cases
package handlers

import (
	"bytes"
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

// setupSessionActivityTest creates a test setup with mock database
func setupSessionActivityTest(t *testing.T) (*SessionActivityHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSessionActivityHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewSessionActivityHandler tests handler creation
func TestNewSessionActivityHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSessionActivityHandler(database)

	assert.NotNil(t, handler, "Handler should not be nil")
	assert.NotNil(t, handler.db, "Database should be set")
}

// TestLogActivityEvent_Success tests logging an activity event
func TestLogActivityEvent_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"sessionId":     "sess-123",
		"eventType":     "session.created",
		"eventCategory": "lifecycle",
		"description":   "Session created successfully",
		"metadata": map[string]interface{}{
			"template": "firefox",
			"user":     "alice",
		},
	}
	body, _ := json.Marshal(reqBody)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO session_activity_log`).
		WithArgs("sess-123", "user-789", "session.created", "lifecycle",
			"Session created successfully", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "timestamp"}).AddRow(1, now))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/activity/log", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", "user-789")

	handler.LogActivityEvent(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, "Event logged successfully", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogActivityEvent_DefaultCategory tests default category assignment
func TestLogActivityEvent_DefaultCategory(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"sessionId": "sess-456",
		"eventType": "session.started",
		// eventCategory intentionally omitted
	}
	body, _ := json.Marshal(reqBody)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO session_activity_log`).
		WithArgs("sess-456", "", "session.started", EventCategoryLifecycle,
			"", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "timestamp"}).AddRow(2, now))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-456/activity/log", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.LogActivityEvent(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogActivityEvent_ValidationError tests missing required fields
func TestLogActivityEvent_ValidationError(t *testing.T) {
	handler, _, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"eventType": "session.created",
		// Missing sessionId
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/activity/log", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.LogActivityEvent(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetSessionActivity_Success tests getting session activity
func TestGetSessionActivity_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	sessionID := "sess-789"
	now := time.Now()

	// Mock count query
	// Mock the COUNT query first (flexible regex to match whitespace variations)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Mock events query
	rows := sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "event_type", "event_category",
		"description", "metadata", "ip_address", "user_agent", "timestamp",
	}).
		AddRow(1, sessionID, "user-1", "session.created", "lifecycle",
			"Created", []byte(`{"template":"firefox"}`), "192.168.1.1", "Mozilla/5.0", now).
		AddRow(2, sessionID, "user-1", "session.started", "lifecycle",
			"Started", []byte(`{}`), "192.168.1.1", "Mozilla/5.0", now.Add(5*time.Second))

	mock.ExpectQuery(`SELECT id, session_id, user_id, event_type, event_category`).
		WithArgs(sessionID, 100, 0).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-789/activity", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.GetSessionActivity(c)

	if w.Code != http.StatusOK {
		t.Logf("Response body: %s", w.Body.String())
	}
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	events := response["events"].([]interface{})
	assert.Len(t, events, 2)

	event1 := events[0].(map[string]interface{})
	assert.Equal(t, "session.created", event1["eventType"])
	assert.Equal(t, "lifecycle", event1["eventCategory"])

	assert.Equal(t, float64(2), response["total"])
	assert.Equal(t, sessionID, response["sessionId"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetSessionActivity_WithFilters tests filtering by event type and category
func TestGetSessionActivity_WithFilters(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	sessionID := "sess-101"
	now := time.Now()

	// Mock count query with filters
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM`).
		WithArgs(sessionID, "session.created", "lifecycle").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Mock events query with filters
	rows := sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "event_type", "event_category",
		"description", "metadata", "ip_address", "user_agent", "timestamp",
	}).
		AddRow(1, sessionID, "user-1", "session.created", "lifecycle",
			"Created", []byte(`{}`), "192.168.1.1", "Mozilla/5.0", now)

	mock.ExpectQuery(`SELECT id, session_id, user_id, event_type, event_category`).
		WithArgs(sessionID, "session.created", "lifecycle", 100, 0).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-101/activity?event_type=session.created&category=lifecycle", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.GetSessionActivity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	events := response["events"].([]interface{})
	assert.Len(t, events, 1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetSessionActivity_Pagination tests pagination parameters
func TestGetSessionActivity_Pagination(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	sessionID := "sess-202"

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))

	// Mock events query with pagination
	mock.ExpectQuery(`SELECT id, session_id, user_id, event_type, event_category`).
		WithArgs(sessionID, 25, 50).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "user_id", "event_type", "event_category",
			"description", "metadata", "ip_address", "user_agent", "timestamp",
		}))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-202/activity?limit=25&offset=50", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.GetSessionActivity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(25), response["limit"])
	assert.Equal(t, float64(50), response["offset"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetActivityStats_Success tests getting activity statistics
func TestGetActivityStats_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	// Mock event type stats query
	eventTypeRows := sqlmock.NewRows([]string{"event_type", "count"}).
		AddRow("session.created", 50).
		AddRow("session.started", 45).
		AddRow("user.connected", 120)

	mock.ExpectQuery(`SELECT event_type, COUNT\(\*\) as count FROM session_activity_log WHERE timestamp >= NOW\(\) - INTERVAL '7 days' GROUP BY event_type ORDER BY count DESC LIMIT 10`).
		WillReturnRows(eventTypeRows)

	// Mock category stats query
	categoryRows := sqlmock.NewRows([]string{"event_category", "count"}).
		AddRow("lifecycle", 95).
		AddRow("connection", 120).
		AddRow("state", 30)

	mock.ExpectQuery(`SELECT event_category, COUNT\(\*\) as count FROM session_activity_log WHERE timestamp >= NOW\(\) - INTERVAL '7 days' GROUP BY event_category ORDER BY count DESC`).
		WillReturnRows(categoryRows)

	// Mock total count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM session_activity_log`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))

	// Mock recent events query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM session_activity_log WHERE timestamp >= NOW\(\) - INTERVAL '24 hours'`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(250))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/activity/stats", nil)

	handler.GetActivityStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1000), response["totalEvents"])
	assert.Equal(t, float64(250), response["recentEvents24h"])

	topEventTypes := response["topEventTypes"].([]interface{})
	assert.Len(t, topEventTypes, 3)

	byCategory := response["byCategory"].([]interface{})
	assert.Len(t, byCategory, 3)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetSessionTimeline_Success tests getting session timeline
func TestGetSessionTimeline_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	sessionID := "sess-303"
	now := time.Now()

	// Mock timeline query
	rows := sqlmock.NewRows([]string{
		"id", "event_type", "event_category", "description",
		"metadata", "user_id", "timestamp",
	}).
		AddRow(1, "session.created", "lifecycle", "Created",
			[]byte(`{"template":"firefox"}`), "user-1", now).
		AddRow(2, "session.started", "lifecycle", "Started",
			[]byte(`{}`), "user-1", now.Add(5*time.Second)).
		AddRow(3, "user.connected", "connection", "User connected",
			[]byte(`{}`), "user-1", now.Add(10*time.Second))

	mock.ExpectQuery(`SELECT id, event_type, event_category, description, metadata, user_id, timestamp FROM session_activity_log WHERE session_id = \$1 ORDER BY timestamp ASC LIMIT 1000`).
		WithArgs(sessionID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-303/timeline", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.GetSessionTimeline(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	timeline := response["timeline"].([]interface{})
	assert.Len(t, timeline, 3)

	// Verify duration calculation
	event2 := timeline[1].(map[string]interface{})
	assert.Equal(t, float64(5), event2["durationSince"], "Should have 5 seconds since previous event")

	event3 := timeline[2].(map[string]interface{})
	assert.Equal(t, float64(5), event3["durationSince"], "Should have 5 seconds since previous event")

	assert.Equal(t, float64(3), response["total"])
	assert.Equal(t, sessionID, response["sessionId"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUserSessionActivity_Success tests getting user-specific activity
func TestGetUserSessionActivity_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	userID := "user-404"
	now := time.Now()

	// Mock activity query
	rows := sqlmock.NewRows([]string{
		"id", "session_id", "event_type", "event_category",
		"description", "metadata", "timestamp",
	}).
		AddRow(1, "sess-1", "session.created", "lifecycle",
			"Created session 1", []byte(`{}`), now).
		AddRow(2, "sess-2", "session.created", "lifecycle",
			"Created session 2", []byte(`{}`), now.Add(-1*time.Hour))

	mock.ExpectQuery(`SELECT id, session_id, event_type, event_category, description, metadata, timestamp FROM session_activity_log WHERE user_id = \$1 ORDER BY timestamp DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(userID, 50, 0).
		WillReturnRows(rows)

	// Mock total count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM session_activity_log WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/users/user-404/activity", nil)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}

	handler.GetUserSessionActivity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	events := response["events"].([]interface{})
	assert.Len(t, events, 2)

	event1 := events[0].(map[string]interface{})
	assert.Equal(t, "sess-1", event1["sessionId"])
	assert.Equal(t, userID, event1["userId"])

	assert.Equal(t, float64(2), response["total"])
	assert.Equal(t, userID, response["userId"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUserSessionActivity_Pagination tests pagination for user activity
func TestGetUserSessionActivity_Pagination(t *testing.T) {
	handler, mock, cleanup := setupSessionActivityTest(t)
	defer cleanup()

	userID := "user-505"

	// Mock activity query with pagination
	mock.ExpectQuery(`SELECT id, session_id, event_type, event_category, description, metadata, timestamp FROM session_activity_log WHERE user_id = \$1 ORDER BY timestamp DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(userID, 20, 40).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "event_type", "event_category",
			"description", "metadata", "timestamp",
		}))

	// Mock total count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM session_activity_log WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/users/user-505/activity?limit=20&offset=40", nil)
	c.Params = []gin.Param{{Key: "userId", Value: userID}}

	handler.GetUserSessionActivity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(20), response["limit"])
	assert.Equal(t, float64(40), response["offset"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
