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
	"github.com/lib/pq"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCatalogTest creates a test handler with mocked database
func setupCatalogTest(t *testing.T) (*CatalogHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewCatalogHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewCatalogHandler tests handler initialization
func TestNewCatalogHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewCatalogHandler(database)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
}

// TestCatalogRegisterRoutes tests route registration
func TestCatalogRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupCatalogTest(t)
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
		{"GET", "/api/v1/catalog/templates"},
		{"GET", "/api/v1/catalog/templates/:id"},
		{"GET", "/api/v1/catalog/templates/featured"},
		{"GET", "/api/v1/catalog/templates/trending"},
		{"GET", "/api/v1/catalog/templates/popular"},
		{"POST", "/api/v1/catalog/templates/:id/ratings"},
		{"GET", "/api/v1/catalog/templates/:id/ratings"},
		{"PUT", "/api/v1/catalog/templates/:id/ratings/:ratingId"},
		{"DELETE", "/api/v1/catalog/templates/:id/ratings/:ratingId"},
		{"POST", "/api/v1/catalog/templates/:id/view"},
		{"POST", "/api/v1/catalog/templates/:id/install"},
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

// TestListTemplates_Success tests basic template listing
func TestListTemplates_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()

	// Mock templates query
	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "tags", "install_count",
		"is_featured", "version", "view_count", "avg_rating", "rating_count",
		"created_at", "updated_at", "repository_name", "repository_url",
	}).
		AddRow(1, 1, "firefox", "Firefox Browser", "Web browser", "Browsers", "browser",
			"firefox.png", pq.StringArray{"browser", "web"}, 1000, true, "1.0.0", 5000, 4.5, 100, now, now, "default", "https://repo.com").
		AddRow(2, 1, "chrome", "Chrome Browser", "Web browser", "Browsers", "browser",
			"chrome.png", pq.StringArray{"browser"}, 800, false, "1.0.0", 3000, 4.3, 80, now, now, "default", "https://repo.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs(20, 0).
		WillReturnRows(rows)

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates", nil)

	handler.ListTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])
	assert.Equal(t, float64(1), response["page"])
	assert.Equal(t, float64(20), response["limit"])

	templates := response["templates"].([]interface{})
	assert.Len(t, templates, 2)

	template1 := templates[0].(map[string]interface{})
	assert.Equal(t, float64(1), template1["id"])
	assert.Equal(t, "Firefox Browser", template1["displayName"])
	assert.Equal(t, true, template1["isFeatured"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListTemplates_WithSearch tests search filtering
func TestListTemplates_WithSearch(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "tags", "install_count",
		"is_featured", "version", "view_count", "avg_rating", "rating_count",
		"created_at", "updated_at", "repository_name", "repository_url",
	}).AddRow(1, 1, "firefox", "Firefox", "Browser", "Browsers", "browser",
		"icon.png", pq.StringArray{"browser"}, 100, false, "1.0", 200, 4.5, 10, now, now, "default", "https://repo.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs("%firefox%", 20, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(`SELECT COUNT`).
		WithArgs("%firefox%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates?search=firefox", nil)

	handler.ListTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListTemplates_WithFilters tests category and tag filtering
func TestListTemplates_WithFilters(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "tags", "install_count",
		"is_featured", "version", "view_count", "avg_rating", "rating_count",
		"created_at", "updated_at", "repository_name", "repository_url",
	}).AddRow(1, 1, "firefox", "Firefox", "Browser", "Browsers", "browser",
		"icon.png", pq.StringArray{"browser", "web"}, 100, false, "1.0", 200, 4.5, 10, now, now, "default", "https://repo.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs("Browsers", "web", "browser", 20, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(`SELECT COUNT`).
		WithArgs("Browsers", "web", "browser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates?category=Browsers&tag=web&appType=browser", nil)

	handler.ListTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListTemplates_WithPagination tests pagination
func TestListTemplates_WithPagination(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "tags", "install_count",
		"is_featured", "version", "view_count", "avg_rating", "rating_count",
		"created_at", "updated_at", "repository_name", "repository_url",
	}).AddRow(11, 1, "template11", "Template 11", "Description", "Category", "type",
		"icon.png", pq.StringArray{}, 100, false, "1.0", 200, 4.0, 10, now, now, "default", "https://repo.com")

	// Page 2, limit 10, offset = 10
	mock.ExpectQuery(`SELECT`).
		WithArgs(10, 10).
		WillReturnRows(rows)

	mock.ExpectQuery(`SELECT COUNT`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates?page=2&limit=10", nil)

	handler.ListTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["page"])
	assert.Equal(t, float64(10), response["limit"])
	assert.Equal(t, float64(15), response["total"])
	assert.Equal(t, float64(2), response["totalPages"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListTemplates_DatabaseError tests database failure
func TestListTemplates_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates", nil)

	handler.ListTemplates(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Database error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTemplateDetails_Success tests getting template details
func TestGetTemplateDetails_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()
	templateID := "1"

	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "manifest", "tags",
		"install_count", "is_featured", "version", "view_count",
		"avg_rating", "rating_count", "created_at", "updated_at",
		"repository_name", "repository_url",
	}).AddRow(1, 1, "firefox", "Firefox Browser", "Web browser", "Browsers", "browser",
		"firefox.png", `{"vnc": true}`, pq.StringArray{"browser", "web"}, 1000, true, "1.0.0",
		5000, 4.5, 100, now, now, "default", "https://repo.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs(templateID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.GetTemplateDetails(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, "Firefox Browser", response["displayName"])
	assert.Equal(t, true, response["isFeatured"])
	assert.Contains(t, response, "manifest")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTemplateDetails_NotFound tests template not found
func TestGetTemplateDetails_NotFound(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "999"

	mock.ExpectQuery(`SELECT`).
		WithArgs(templateID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.GetTemplateDetails(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetFeaturedTemplates_Success tests getting featured templates
func TestGetFeaturedTemplates_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "repository_id", "name", "display_name", "description",
		"category", "app_type", "icon_url", "tags", "install_count",
		"is_featured", "version", "view_count", "avg_rating", "rating_count",
		"created_at", "updated_at", "repository_name", "repository_url",
	}).AddRow(1, 1, "firefox", "Firefox", "Browser", "Browsers", "browser",
		"icon.png", pq.StringArray{}, 100, true, "1.0", 200, 4.5, 10, now, now, "default", "https://repo.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs(20, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(`SELECT COUNT`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates/featured", nil)

	handler.GetFeaturedTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAddRating_Success tests adding a template rating
func TestAddRating_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"
	userID := "user-123"

	// Mock rating insert/update
	mock.ExpectExec(`INSERT INTO template_ratings`).
		WithArgs(templateID, userID, 5, "Great template!").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock aggregate rating update
	mock.ExpectExec(`UPDATE catalog_templates`).
		WithArgs(templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"rating":5,"review":"Great template!"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/ratings", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	c.Set("userID", userID)

	handler.AddRating(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAddRating_NoAuth tests rating without authentication
func TestAddRating_NoAuth(t *testing.T) {
	handler, _, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"rating":5,"review":"Great!"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/ratings", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	// No userID set

	handler.AddRating(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Unauthorized")
}

// TestAddRating_InvalidRating tests invalid rating value
func TestAddRating_InvalidRating(t *testing.T) {
	handler, _, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"
	userID := "user-123"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"rating":6,"review":"Great!"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/ratings", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	c.Set("userID", userID)

	handler.AddRating(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid request")
}

// TestGetRatings_Success tests getting template ratings
func TestGetRatings_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "rating", "review", "created_at", "updated_at", "username", "full_name",
	}).
		AddRow(1, "user-1", 5, "Great!", now, now, "user1", "User One").
		AddRow(2, "user-2", 4, "Good", now, now, "user2", "User Two")

	mock.ExpectQuery(`SELECT`).
		WithArgs(templateID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/templates/1/ratings", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.GetRatings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	ratings := response["ratings"].([]interface{})
	assert.Len(t, ratings, 2)

	rating1 := ratings[0].(map[string]interface{})
	assert.Equal(t, float64(1), rating1["id"])
	assert.Equal(t, float64(5), rating1["rating"])
	assert.Equal(t, "Great!", rating1["review"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteRating_Success tests deleting a rating
func TestDeleteRating_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"
	ratingID := "10"
	userID := "user-123"

	mock.ExpectExec(`DELETE FROM template_ratings`).
		WithArgs(ratingID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock aggregate rating update
	mock.ExpectExec(`UPDATE catalog_templates`).
		WithArgs(templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/catalog/templates/1/ratings/10", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: templateID},
		{Key: "ratingId", Value: ratingID},
	}
	c.Set("userID", userID)

	handler.DeleteRating(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "deleted successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteRating_NoAuth tests deleting without authentication
func TestDeleteRating_NoAuth(t *testing.T) {
	handler, _, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"
	ratingID := "10"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/catalog/templates/1/ratings/10", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: templateID},
		{Key: "ratingId", Value: ratingID},
	}
	// No userID set

	handler.DeleteRating(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Unauthorized")
}

// TestRecordView_Success tests recording a template view
func TestRecordView_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"

	mock.ExpectExec(`UPDATE catalog_templates`).
		WithArgs(templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/view", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.RecordView(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "View recorded")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRecordInstall_Success tests recording a template installation
func TestRecordInstall_Success(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"

	mock.ExpectExec(`UPDATE catalog_templates`).
		WithArgs(templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/install", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.RecordInstall(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "Install recorded")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRecordInstall_DatabaseError tests database failure
func TestRecordInstall_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupCatalogTest(t)
	defer cleanup()

	templateID := "1"

	mock.ExpectExec(`UPDATE catalog_templates`).
		WithArgs(templateID).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/catalog/templates/1/install", nil)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	handler.RecordInstall(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Database error")

	assert.NoError(t, mock.ExpectationsWereMet())
}
