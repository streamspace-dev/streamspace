// Package handlers provides HTTP handlers for the StreamSpace API.
//
// This file contains tests for the Notifications handler (in-app notification management).
//
// Test Coverage:
//   - ListNotifications (pagination, user isolation)
//   - GetUnreadNotifications (filtering)
//   - GetUnreadCount (counting logic)
//   - MarkAsRead (single notification)
//   - MarkAllAsRead (bulk update)
//   - DeleteNotification (single delete)
//   - ClearAllNotifications (bulk delete)
//   - GetNotificationPreferences (defaults and custom)
//   - UpdateNotificationPreferences (upsert logic)
//   - SendNotification (in-app creation)
//   - Route registration
//
// Skipped (External Dependencies):
//   - TestEmailNotification (requires SMTP server)
//   - TestWebhookNotification (requires webhook endpoint)
//   - Email/webhook delivery helpers
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

// setupNotificationsTest creates a test setup with mock database
func setupNotificationsTest(t *testing.T) (*NotificationsHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewNotificationsHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewNotificationsHandler tests handler creation
func TestNewNotificationsHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewNotificationsHandler(database)

	assert.NotNil(t, handler, "Handler should not be nil")
	assert.NotNil(t, handler.db, "Database should be set")
}

// TestListNotifications_Success tests listing user notifications
func TestListNotifications_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	// Mock notifications query
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "type", "title", "message", "data", "priority",
		"is_read", "action_url", "action_text", "created_at", "read_at",
	}).
		AddRow("notif-1", userID, "session.created", "Session Created",
			"Your Firefox session is ready", []byte(`{"sessionId":"sess-1"}`), "normal",
			false, "/sessions/sess-1", "View Session", now, nil).
		AddRow("notif-2", userID, "quota.warning", "Quota Warning",
			"You're at 80% of your quota", []byte(`{}`), "high",
			true, "/settings/quota", "View Quota", now.Add(-1*time.Hour), now)

	mock.ExpectQuery(`SELECT id, user_id, type, title, message, data, priority, is_read, action_url, action_text, created_at, read_at FROM notifications WHERE user_id = \$1 ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(userID, 50, 0).
		WillReturnRows(rows)

	// Mock total count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM notifications WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications", nil)
	c.Set("userID", userID)

	handler.ListNotifications(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	notifications := response["notifications"].([]interface{})
	assert.Len(t, notifications, 2)

	notif1 := notifications[0].(map[string]interface{})
	assert.Equal(t, "notif-1", notif1["id"])
	assert.Equal(t, "session.created", notif1["type"])
	assert.False(t, notif1["read"].(bool))

	notif2 := notifications[1].(map[string]interface{})
	assert.Equal(t, "notif-2", notif2["id"])
	assert.True(t, notif2["read"].(bool))

	assert.Equal(t, float64(2), response["total"])
	assert.Equal(t, float64(50), response["limit"])
	assert.Equal(t, float64(0), response["offset"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUnreadNotifications_Success tests fetching only unread notifications
func TestGetUnreadNotifications_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-456"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "type", "title", "message", "data", "priority",
		"action_url", "action_text", "created_at",
	}).
		AddRow("notif-3", userID, "session.idle", "Session Idle",
			"Your session has been idle for 10 minutes", []byte(`{}`), "normal",
			"/sessions/sess-2", "Resume Session", now)

	mock.ExpectQuery(`SELECT id, user_id, type, title, message, data, priority, action_url, action_text, created_at FROM notifications WHERE user_id = \$1 AND is_read = false ORDER BY created_at DESC LIMIT 50`).
		WithArgs(userID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/unread", nil)
	c.Set("userID", userID)

	handler.GetUnreadNotifications(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	notifications := response["notifications"].([]interface{})
	assert.Len(t, notifications, 1)
	assert.Equal(t, float64(1), response["count"])

	notif := notifications[0].(map[string]interface{})
	assert.Equal(t, "notif-3", notif["id"])
	assert.False(t, notif["read"].(bool), "Should be marked as unread")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUnreadCount_Success tests unread count retrieval
func TestGetUnreadCount_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-789"

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM notifications WHERE user_id = \$1 AND is_read = false`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/count", nil)
	c.Set("userID", userID)

	handler.GetUnreadCount(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(5), response["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMarkAsRead_Success tests marking a notification as read
func TestMarkAsRead_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-123"
	notifID := "notif-1"

	mock.ExpectExec(`UPDATE notifications SET is_read = true, read_at = CURRENT_TIMESTAMP WHERE id = \$1 AND user_id = \$2`).
		WithArgs(notifID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/notifications/notif-1/read", nil)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: notifID}}

	handler.MarkAsRead(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Notification marked as read", response["message"])
	assert.Equal(t, notifID, response["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMarkAllAsRead_Success tests bulk mark as read
func TestMarkAllAsRead_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-456"

	mock.ExpectExec(`UPDATE notifications SET is_read = true, read_at = CURRENT_TIMESTAMP WHERE user_id = \$1 AND is_read = false`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 3))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/notifications/read-all", nil)
	c.Set("userID", userID)

	handler.MarkAllAsRead(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "All notifications marked as read", response["message"])
	assert.Equal(t, float64(3), response["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteNotification_Success tests deleting a notification
func TestDeleteNotification_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-789"
	notifID := "notif-5"

	mock.ExpectExec(`DELETE FROM notifications WHERE id = \$1 AND user_id = \$2`).
		WithArgs(notifID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/notifications/notif-5", nil)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: notifID}}

	handler.DeleteNotification(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Notification deleted", response["message"])
	assert.Equal(t, notifID, response["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestClearAllNotifications_Success tests bulk delete of read notifications
func TestClearAllNotifications_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-101"

	mock.ExpectExec(`DELETE FROM notifications WHERE user_id = \$1 AND is_read = true`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 7))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/notifications/clear-all", nil)
	c.Set("userID", userID)

	handler.ClearAllNotifications(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Read notifications cleared", response["message"])
	assert.Equal(t, float64(7), response["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetNotificationPreferences_Success tests fetching user preferences
func TestGetNotificationPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-202"
	prefsJSON := []byte(`{"email":{"session.created":true},"inApp":{"session.created":true},"webhook":{"enabled":false}}`)

	mock.ExpectQuery(`SELECT preferences->'notifications' FROM user_preferences WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow(prefsJSON))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/preferences", nil)
	c.Set("userID", userID)

	handler.GetNotificationPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	email := response["email"].(map[string]interface{})
	assert.True(t, email["session.created"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetNotificationPreferences_Defaults tests default preferences
func TestGetNotificationPreferences_Defaults(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-303"

	// No preferences found, should return defaults
	mock.ExpectQuery(`SELECT preferences->'notifications' FROM user_preferences WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/preferences", nil)
	c.Set("userID", userID)

	handler.GetNotificationPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify default structure exists
	assert.NotNil(t, response["email"])
	assert.NotNil(t, response["inApp"])
	assert.NotNil(t, response["webhook"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateNotificationPreferences_Success tests updating preferences
func TestUpdateNotificationPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	userID := "user-404"
	reqBody := map[string]interface{}{
		"email": map[string]bool{
			"session.created": true,
			"quota.warning":   true,
		},
	}
	body, _ := json.Marshal(reqBody)

	mock.ExpectExec(`INSERT INTO user_preferences`).
		WithArgs(userID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/preferences", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdateNotificationPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Notification preferences updated", response["message"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSendNotification_Success tests creating an in-app notification
func TestSendNotification_Success(t *testing.T) {
	handler, mock, cleanup := setupNotificationsTest(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"userId":     "user-505",
		"type":       "session.created",
		"title":      "Session Created",
		"message":    "Your new session is ready",
		"priority":   "normal",
		"actionUrl":  "/sessions/sess-1",
		"actionText": "View Session",
	}
	body, _ := json.Marshal(reqBody)

	// Mock preference check (returns defaults)
	mock.ExpectQuery(`SELECT preferences->'notifications' FROM user_preferences WHERE user_id = \$1`).
		WithArgs("user-505").
		WillReturnError(sql.ErrNoRows)

	// Mock in-app notification creation
	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs(sqlmock.AnyArg(), "user-505", "session.created", "Session Created",
			"Your new session is ready", sqlmock.AnyArg(), "normal", "/sessions/sess-1", "View Session").
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/notifications/send", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.SendNotification(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Notification sent", response["message"])
	assert.NotEmpty(t, response["notificationId"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSendNotification_ValidationError tests missing required fields
func TestSendNotification_ValidationError(t *testing.T) {
	handler, _, cleanup := setupNotificationsTest(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"type": "session.created",
		// Missing userId, title, message
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/notifications/send", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.SendNotification(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestTestEmailNotification_Skipped tests email testing endpoint (skipped - requires SMTP)
func TestTestEmailNotification_Skipped(t *testing.T) {
	t.Skip("Skipped: Requires SMTP server (integration test territory)")
	// This test requires a real SMTP server which cannot be easily mocked
	// The handler calls smtp.SendMail() which requires network access
	// Integration tests with a real or mock SMTP server should cover this endpoint
}

// TestTestWebhookNotification_Skipped tests webhook testing endpoint (skipped - requires HTTP endpoint)
func TestTestWebhookNotification_Skipped(t *testing.T) {
	t.Skip("Skipped: Requires webhook endpoint (integration test territory)")
	// This test requires a real webhook endpoint to POST to
	// The handler makes HTTP requests which require network access
	// Integration tests with a mock HTTP server should cover this endpoint
}

// TestNotificationRegisterRoutes tests route registration
func TestNotificationRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupNotificationsTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")

	handler.RegisterRoutes(group)

	// Verify all routes are registered
	routes := router.Routes()
	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/notifications"},
		{"GET", "/api/v1/notifications/unread"},
		{"GET", "/api/v1/notifications/count"},
		{"POST", "/api/v1/notifications/:id/read"},
		{"POST", "/api/v1/notifications/read-all"},
		{"DELETE", "/api/v1/notifications/:id"},
		{"DELETE", "/api/v1/notifications/clear-all"},
		{"POST", "/api/v1/notifications/send"},
		{"GET", "/api/v1/notifications/preferences"},
		{"PUT", "/api/v1/notifications/preferences"},
		{"POST", "/api/v1/notifications/test/email"},
		{"POST", "/api/v1/notifications/test/webhook"},
	}

	foundCount := 0
	for _, expected := range expectedRoutes {
		for _, route := range routes {
			if route.Method == expected.method && route.Path == expected.path {
				foundCount++
				break
			}
		}
	}

	assert.Equal(t, 12, foundCount, "All 12 notification routes should be registered")
}
