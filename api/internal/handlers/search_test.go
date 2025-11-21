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
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSearchTest creates a test handler with mocked database
func setupSearchTest(t *testing.T) (*SearchHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSearchHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewSearchHandler tests handler initialization
func TestNewSearchHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSearchHandler(database)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
}

// TestSearchRegisterRoutes tests route registration
func TestSearchRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupSearchTest(t)
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
		{"GET", "/api/v1/search"},
		{"GET", "/api/v1/search/templates"},
		{"GET", "/api/v1/search/sessions"},
		{"GET", "/api/v1/search/suggest"},
		{"GET", "/api/v1/search/advanced"},
		{"GET", "/api/v1/search/filters/categories"},
		{"GET", "/api/v1/search/filters/tags"},
		{"GET", "/api/v1/search/filters/app-types"},
		{"GET", "/api/v1/search/saved"},
		{"POST", "/api/v1/search/saved"},
		{"GET", "/api/v1/search/saved/:id"},
		{"PUT", "/api/v1/search/saved/:id"},
		{"DELETE", "/api/v1/search/saved/:id"},
		{"POST", "/api/v1/search/saved/:id/execute"},
		{"GET", "/api/v1/search/history"},
		{"DELETE", "/api/v1/search/history"},
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

// TestSearch_Success tests universal search
func TestSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	query := "firefox"

	// Mock template search query
	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "category", "tags", "icon", "app_type", "avg_rating", "install_count",
	}).AddRow("tpl-1", "firefox", "Firefox Browser", "Web browser", "Browsers", []byte(`["browser","web"]`), "firefox.png", "browser", 4.5, 1000)

	mock.ExpectQuery(`SELECT id, name, display_name, description`).
		WithArgs("%"+query+"%", 20).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search?q=firefox", nil)

	handler.Search(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, query, response["query"])
	assert.Equal(t, float64(1), response["count"])

	results := response["results"].([]interface{})
	assert.Len(t, results, 1)

	result := results[0].(map[string]interface{})
	assert.Equal(t, "template", result["type"])
	assert.Equal(t, "tpl-1", result["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearch_MissingQuery tests missing query parameter
func TestSearch_MissingQuery(t *testing.T) {
	handler, _, cleanup := setupSearchTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search", nil)

	handler.Search(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "query required")
}

// TestSearchTemplates_Success tests template search with filters
func TestSearchTemplates_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "category", "tags", "icon", "app_type",
		"avg_rating", "install_count", "view_count", "is_featured",
	}).
		AddRow("tpl-1", "firefox", "Firefox Browser", "Web browser", "Browsers",
			[]byte(`["browser","web"]`), "firefox.png", "browser", 4.5, 1000, 5000, true).
		AddRow("tpl-2", "chrome", "Chrome Browser", "Web browser", "Browsers",
			[]byte(`["browser"]`), "chrome.png", "browser", 4.3, 800, 3000, false)

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/templates?q=browser&category=Browsers&sort_by=popularity", nil)

	handler.SearchTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "browser", response["query"])
	assert.Equal(t, "Browsers", response["category"])
	assert.Equal(t, float64(2), response["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchTemplates_WithTagsFilter tests template search with tags filter
func TestSearchTemplates_WithTagsFilter(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "category", "tags", "icon", "app_type",
		"avg_rating", "install_count", "view_count", "is_featured",
	}).AddRow("tpl-1", "firefox", "Firefox", "Browser", "Browsers", []byte(`["browser"]`), "icon.png", "browser", 4.5, 100, 200, false)

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/templates?q=browser&tags=browser,web", nil)

	handler.SearchTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "browser,web", response["tags"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchTemplates_DatabaseError tests database failure
func TestSearchTemplates_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/templates?q=test", nil)

	handler.SearchTemplates(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "failed")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchSessions_Success tests session search
func TestSearchSessions_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "template_name", "state", "created_at", "last_connection"}).
		AddRow("sess-1", "firefox", "running", now, now).
		AddRow("sess-2", "chrome", "hibernated", now, nil)

	// Handler adds query parameter as additional filter
	mock.ExpectQuery(`SELECT id, template_name, state, created_at, last_connection`).
		WithArgs(userID, "%fire%").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/sessions?q=fire", nil)
	c.Set("userID", userID)

	handler.SearchSessions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["count"])

	results := response["results"].([]interface{})
	assert.Len(t, results, 2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchSessions_WithStateFilter tests session search with state filter
func TestSearchSessions_WithStateFilter(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "template_name", "state", "created_at", "last_connection"}).
		AddRow("sess-1", "firefox", "running", now, now)

	mock.ExpectQuery(`SELECT id, template_name, state, created_at, last_connection`).
		WithArgs(userID, "running").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/sessions?state=running", nil)
	c.Set("userID", userID)

	handler.SearchSessions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "running", response["state"])
	assert.Equal(t, float64(1), response["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchSuggestions_Success tests auto-complete suggestions
func TestSearchSuggestions_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"display_name"}).
		AddRow("Firefox Browser").
		AddRow("Firefox Developer Edition").
		AddRow("Firefox ESR")

	mock.ExpectQuery(`SELECT DISTINCT display_name`).
		WithArgs("fire%").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/suggest?q=fire", nil)

	handler.SearchSuggestions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	suggestions := response["suggestions"].([]interface{})
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "Firefox Browser", suggestions[0])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSearchSuggestions_ShortQuery tests suggestions with short query
func TestSearchSuggestions_ShortQuery(t *testing.T) {
	handler, _, cleanup := setupSearchTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/suggest?q=f", nil)

	handler.SearchSuggestions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	suggestions := response["suggestions"].([]interface{})
	assert.Empty(t, suggestions)
}

// TestAdvancedSearch_Success tests advanced multi-criteria search
func TestAdvancedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "category", "tags", "icon", "app_type",
		"avg_rating", "install_count", "view_count", "is_featured",
	}).AddRow("tpl-1", "firefox", "Firefox", "Browser", "Browsers", []byte(`[]`), "icon.png", "browser", 4.5, 100, 200, false)

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query":"firefox","filters":{"category":"Browsers"},"sort":"popularity","limit":50}`
	c.Request = httptest.NewRequest("POST", "/api/v1/search/advanced", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AdvancedSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAdvancedSearch_InvalidJSON tests invalid JSON request
func TestAdvancedSearch_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupSearchTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{invalid json}`
	c.Request = httptest.NewRequest("POST", "/api/v1/search/advanced", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AdvancedSearch(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetCategories_Success tests getting all categories
func TestGetCategories_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"category", "count"}).
		AddRow("Browsers", 10).
		AddRow("IDEs", 8).
		AddRow("Utilities", 5)

	mock.ExpectQuery(`SELECT category, COUNT`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/filters/categories", nil)

	handler.GetCategories(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	categories := response["categories"].([]interface{})
	assert.Len(t, categories, 3)

	cat1 := categories[0].(map[string]interface{})
	assert.Equal(t, "Browsers", cat1["name"])
	assert.Equal(t, float64(10), cat1["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPopularTags_Success tests getting popular tags
func TestGetPopularTags_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"tag", "count"}).
		AddRow("browser", 15).
		AddRow("web", 12).
		AddRow("development", 8)

	mock.ExpectQuery(`SELECT tag, COUNT`).
		WithArgs(50).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/filters/tags", nil)

	handler.GetPopularTags(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	tags := response["tags"].([]interface{})
	assert.Len(t, tags, 3)

	tag1 := tags[0].(map[string]interface{})
	assert.Equal(t, "browser", tag1["name"])
	assert.Equal(t, float64(15), tag1["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetAppTypes_Success tests getting all app types
func TestGetAppTypes_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"app_type", "count"}).
		AddRow("browser", 20).
		AddRow("ide", 15).
		AddRow("utility", 10)

	mock.ExpectQuery(`SELECT app_type, COUNT`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/filters/app-types", nil)

	handler.GetAppTypes(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	appTypes := response["appTypes"].([]interface{})
	assert.Len(t, appTypes, 3)

	type1 := appTypes[0].(map[string]interface{})
	assert.Equal(t, "browser", type1["name"])
	assert.Equal(t, float64(20), type1["count"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListSavedSearches_Success tests listing saved searches
func TestListSavedSearches_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "name", "description", "query", "filters", "created_at", "updated_at",
	}).
		AddRow("search-1", userID, "My Browsers", "Browser templates", "firefox",
			[]byte(`{"category":"Browsers"}`), now, now).
		AddRow("search-2", userID, "Dev Tools", "Development IDEs", "vscode",
			[]byte(`{"category":"IDEs"}`), now, now)

	mock.ExpectQuery(`SELECT id, user_id, name, description, query, filters`).
		WithArgs(userID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/saved", nil)
	c.Set("userID", userID)

	handler.ListSavedSearches(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	searches := response["searches"].([]interface{})
	assert.Len(t, searches, 2)

	search1 := searches[0].(map[string]interface{})
	assert.Equal(t, "search-1", search1["id"])
	assert.Equal(t, "My Browsers", search1["name"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateSavedSearch_Success tests creating a saved search
func TestCreateSavedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectExec(`INSERT INTO saved_searches`).
		WithArgs(sqlmock.AnyArg(), userID, "My Search", "Description", "firefox", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"name":"My Search","description":"Description","query":"firefox","filters":{"category":"Browsers"}}`
	c.Request = httptest.NewRequest("POST", "/api/v1/search/saved", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.CreateSavedSearch(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "successfully")
	assert.Contains(t, response, "searchId")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateSavedSearch_InvalidRequest tests invalid request
func TestCreateSavedSearch_InvalidRequest(t *testing.T) {
	handler, _, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"description":"Missing required fields"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/search/saved", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.CreateSavedSearch(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetSavedSearch_Success tests getting a specific saved search
func TestGetSavedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-456"
	now := time.Now()

	mock.ExpectQuery(`SELECT id, user_id, name, description, query, filters`).
		WithArgs(searchID, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "description", "query", "filters", "created_at", "updated_at",
		}).AddRow(searchID, userID, "My Search", "Description", "firefox", []byte(`{}`), now, now))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/saved/search-456", nil)
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.GetSavedSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SavedSearch
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, searchID, response.ID)
	assert.Equal(t, "My Search", response.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetSavedSearch_NotFound tests getting non-existent saved search
func TestGetSavedSearch_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-999"

	mock.ExpectQuery(`SELECT id, user_id, name, description, query, filters`).
		WithArgs(searchID, userID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/saved/search-999", nil)
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.GetSavedSearch(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateSavedSearch_Success tests updating a saved search
func TestUpdateSavedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-456"

	mock.ExpectExec(`UPDATE saved_searches`).
		WithArgs("Updated Search", "New description", "chrome", sqlmock.AnyArg(), searchID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"name":"Updated Search","description":"New description","query":"chrome","filters":{}}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/search/saved/search-456", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.UpdateSavedSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "updated successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteSavedSearch_Success tests deleting a saved search
func TestDeleteSavedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-456"

	mock.ExpectExec(`DELETE FROM saved_searches`).
		WithArgs(searchID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/search/saved/search-456", nil)
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.DeleteSavedSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "deleted successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExecuteSavedSearch_Success tests executing a saved search
func TestExecuteSavedSearch_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-456"

	// Mock getting saved search
	mock.ExpectQuery(`SELECT query, filters FROM saved_searches`).
		WithArgs(searchID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"query", "filters"}).
			AddRow("firefox", []byte(`{"category":"Browsers"}`)))

	// Mock template search execution
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "display_name", "description", "category", "tags", "icon", "app_type",
			"avg_rating", "install_count", "view_count", "is_featured",
		}))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/search/saved/search-456/execute", nil)
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.ExecuteSavedSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExecuteSavedSearch_NotFound tests executing non-existent saved search
func TestExecuteSavedSearch_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	searchID := "search-999"

	mock.ExpectQuery(`SELECT query, filters FROM saved_searches`).
		WithArgs(searchID, userID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/search/saved/search-999/execute", nil)
	c.Params = []gin.Param{{Key: "id", Value: searchID}}
	c.Set("userID", userID)

	handler.ExecuteSavedSearch(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetSearchHistory_Success tests getting search history
func TestGetSearchHistory_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"query", "search_type", "filters", "searched_at"}).
		AddRow("firefox", "templates", []byte(`{"category":"Browsers"}`), now).
		AddRow("chrome", "universal", []byte(`{}`), now.Add(-1*time.Hour))

	mock.ExpectQuery(`SELECT query, search_type, filters, searched_at`).
		WithArgs(userID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/history", nil)
	c.Set("userID", userID)

	handler.GetSearchHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	history := response["history"].([]interface{})
	assert.Len(t, history, 2)

	item1 := history[0].(map[string]interface{})
	assert.Equal(t, "firefox", item1["query"])
	assert.Equal(t, "templates", item1["type"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestClearSearchHistory_Success tests clearing search history
func TestClearSearchHistory_Success(t *testing.T) {
	handler, mock, cleanup := setupSearchTest(t)
	defer cleanup()

	userID := "user-123"

	mock.ExpectExec(`DELETE FROM search_history`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 5))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/search/history", nil)
	c.Set("userID", userID)

	handler.ClearSearchHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "cleared")

	assert.NoError(t, mock.ExpectationsWereMet())
}
