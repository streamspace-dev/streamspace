package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPreferencesTest creates a test handler with mocked database
func setupPreferencesTest(t *testing.T) (*PreferencesHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewPreferencesHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewPreferencesHandler tests handler initialization
func TestNewPreferencesHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewPreferencesHandler(database)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
}

// TestPreferencesRegisterRoutes tests route registration
func TestPreferencesRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupPreferencesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	routes := router.Routes()

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/preferences"},
		{"PUT", "/api/v1/preferences"},
		{"DELETE", "/api/v1/preferences"},
		{"GET", "/api/v1/preferences/ui"},
		{"PUT", "/api/v1/preferences/ui"},
		{"GET", "/api/v1/preferences/notifications"},
		{"PUT", "/api/v1/preferences/notifications"},
		{"GET", "/api/v1/preferences/defaults"},
		{"PUT", "/api/v1/preferences/defaults"},
		{"GET", "/api/v1/preferences/favorites"},
		{"POST", "/api/v1/preferences/favorites/:templateName"},
		{"DELETE", "/api/v1/preferences/favorites/:templateName"},
		{"GET", "/api/v1/preferences/recent"},
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

	assert.Equal(t, len(expectedRoutes), foundCount, "All expected routes should be registered")
}

// TestGetPreferences_Success tests getting all preferences
func TestGetPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	prefs := map[string]interface{}{
		"ui": map[string]interface{}{
			"theme": "dark",
		},
		"notifications": map[string]interface{}{
			"email": true,
		},
	}
	prefsJSON, _ := json.Marshal(prefs)

	mock.ExpectQuery(`SELECT preferences FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow(prefsJSON))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences", nil)
	c.Set("userID", userID)

	handler.GetPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "ui")
	assert.Contains(t, response, "notifications")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPreferences_NoPreferences tests getting preferences when none exist
func TestGetPreferences_NoPreferences(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectQuery(`SELECT preferences FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences", nil)
	c.Set("userID", userID)

	handler.GetPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should return default preferences
	assert.Contains(t, response, "ui")
	assert.Contains(t, response, "notifications")
	assert.Contains(t, response, "defaults")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPreferences_NoAuth tests missing authentication
func TestGetPreferences_NoAuth(t *testing.T) {
	handler, _, cleanup := setupPreferencesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences", nil)
	// No userID set

	handler.GetPreferences(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not authenticated")
}

// TestUpdatePreferences_Success tests updating all preferences
func TestUpdatePreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	prefs := map[string]interface{}{
		"ui": map[string]interface{}{
			"theme": "dark",
		},
	}
	prefsJSON, _ := json.Marshal(prefs)

	mock.ExpectExec(`INSERT INTO user_preferences`).
		WithArgs(userID, prefsJSON).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"ui":{"theme":"dark"}}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/preferences", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdatePreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "updated successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdatePreferences_ValidationError tests invalid JSON
func TestUpdatePreferences_ValidationError(t *testing.T) {
	handler, _, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/preferences", strings.NewReader("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdatePreferences(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResetPreferences_Success tests resetting preferences
func TestResetPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectExec(`DELETE FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/preferences", nil)
	c.Set("userID", userID)

	handler.ResetPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "reset to defaults")
	assert.Contains(t, response, "preferences")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUIPreferences_Success tests getting UI preferences
func TestGetUIPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	uiPrefs := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
	}
	uiPrefsJSON, _ := json.Marshal(uiPrefs)

	mock.ExpectQuery(`SELECT preferences->'ui' FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow(uiPrefsJSON))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/ui", nil)
	c.Set("userID", userID)

	handler.GetUIPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "dark", response["theme"])
	assert.Equal(t, "en", response["language"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateUIPreferences_Success tests updating UI preferences
func TestUpdateUIPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	uiPrefs := map[string]interface{}{"theme": "dark"}
	uiPrefsJSON, _ := json.Marshal(uiPrefs)

	mock.ExpectExec(`INSERT INTO user_preferences`).
		WithArgs(userID, uiPrefsJSON).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"theme":"dark"}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/preferences/ui", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdateUIPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPreferencesGetNotificationPreferences_Success tests getting notification preferences
func TestPreferencesGetNotificationPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	notifPrefs := map[string]interface{}{
		"email": map[string]bool{
			"sessionCreated": true,
		},
	}
	notifPrefsJSON, _ := json.Marshal(notifPrefs)

	mock.ExpectQuery(`SELECT preferences->'notifications' FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow(notifPrefsJSON))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/notifications", nil)
	c.Set("userID", userID)

	handler.GetNotificationPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "email")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPreferencesUpdateNotificationPreferences_Success tests updating notification preferences
func TestPreferencesUpdateNotificationPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	notifPrefs := map[string]interface{}{"email": true}
	notifPrefsJSON, _ := json.Marshal(notifPrefs)

	mock.ExpectExec(`INSERT INTO user_preferences`).
		WithArgs(userID, notifPrefsJSON).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"email":true}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/preferences/notifications", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdateNotificationPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetDefaultsPreferences_Success tests getting default session preferences
func TestGetDefaultsPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	defaults := map[string]interface{}{
		"defaultCPU":    "2000m",
		"defaultMemory": "4Gi",
	}
	defaultsJSON, _ := json.Marshal(defaults)

	mock.ExpectQuery(`SELECT preferences->'defaults' FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow(defaultsJSON))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/defaults", nil)
	c.Set("userID", userID)

	handler.GetDefaultsPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2000m", response["defaultCPU"])
	assert.Equal(t, "4Gi", response["defaultMemory"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateDefaultsPreferences_Success tests updating default session preferences
func TestUpdateDefaultsPreferences_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	defaults := map[string]interface{}{"defaultCPU": "2000m"}
	defaultsJSON, _ := json.Marshal(defaults)

	mock.ExpectExec(`INSERT INTO user_preferences`).
		WithArgs(userID, defaultsJSON).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"defaultCPU":"2000m"}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/preferences/defaults", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.UpdateDefaultsPreferences(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetFavorites_Success tests getting favorite templates
func TestGetFavorites_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"template_name", "added_at"}).
		AddRow("firefox", now).
		AddRow("vscode", now.Add(-1*time.Hour))

	mock.ExpectQuery(`SELECT template_name, added_at FROM user_favorite_templates WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/favorites", nil)
	c.Set("userID", userID)

	handler.GetFavorites(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	favorites := response["favorites"].([]interface{})
	assert.Len(t, favorites, 2)

	fav1 := favorites[0].(map[string]interface{})
	assert.Equal(t, "firefox", fav1["templateName"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetFavorites_Empty tests getting favorites when none exist
func TestGetFavorites_Empty(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectQuery(`SELECT template_name, added_at FROM user_favorite_templates WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"template_name", "added_at"}))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/favorites", nil)
	c.Set("userID", userID)

	handler.GetFavorites(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(0), response["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAddFavorite_Success tests adding a template to favorites
func TestAddFavorite_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	templateName := "firefox"

	mock.ExpectExec(`INSERT INTO user_favorite_templates`).
		WithArgs(userID, templateName).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/preferences/favorites/firefox", nil)
	c.Params = []gin.Param{{Key: "templateName", Value: templateName}}
	c.Set("userID", userID)

	handler.AddFavorite(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "added to favorites")
	assert.Equal(t, templateName, response["templateName"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRemoveFavorite_Success tests removing a template from favorites
func TestRemoveFavorite_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	templateName := "firefox"

	mock.ExpectExec(`DELETE FROM user_favorite_templates WHERE user_id`).
		WithArgs(userID, templateName).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/preferences/favorites/firefox", nil)
	c.Params = []gin.Param{{Key: "templateName", Value: templateName}}
	c.Set("userID", userID)

	handler.RemoveFavorite(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "removed from favorites")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetRecentSessions_Success tests getting recent sessions
func TestGetRecentSessions_Success(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "template_name", "state", "created_at"}).
		AddRow("sess-1", "firefox", "running", now).
		AddRow("sess-2", "vscode", "hibernated", now.Add(-1*time.Hour))

	mock.ExpectQuery(`SELECT id, template_name, state, created_at FROM sessions WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/recent", nil)
	c.Set("userID", userID)

	handler.GetRecentSessions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	sessions := response["sessions"].([]interface{})
	assert.Len(t, sessions, 2)

	sess1 := sessions[0].(map[string]interface{})
	assert.Equal(t, "sess-1", sess1["id"])
	assert.Equal(t, "firefox", sess1["templateName"])
	assert.Equal(t, "running", sess1["state"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetRecentSessions_Empty tests getting recent sessions when none exist
func TestGetRecentSessions_Empty(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectQuery(`SELECT id, template_name, state, created_at FROM sessions WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "template_name", "state", "created_at"}))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences/recent", nil)
	c.Set("userID", userID)

	handler.GetRecentSessions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(0), response["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPreferences_DatabaseError tests database failure
func TestGetPreferences_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupPreferencesTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectQuery(`SELECT preferences FROM user_preferences WHERE user_id`).
		WithArgs(userID).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/preferences", nil)
	c.Set("userID", userID)

	handler.GetPreferences(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Failed to get preferences")

	assert.NoError(t, mock.ExpectationsWereMet())
}
