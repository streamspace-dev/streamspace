// Package handlers provides HTTP handlers for the StreamSpace API.
//
// This file contains comprehensive tests for the Controllers handler (platform controller management).
//
// Test Coverage:
//   - ListControllers with filters (platform, status)
//   - GetController (success and not found cases)
//   - RegisterController (validation, conflicts, defaults)
//   - UpdateController (dynamic field updates, validation)
//   - UnregisterController (success and not found cases)
//   - UpdateHeartbeat (success and not found cases)
//   - Error handling and edge cases
//   - Route registration
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
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupControllerTest creates a test setup with mock database
func setupControllerTest(t *testing.T) (*ControllerHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewControllerHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewControllerHandler tests handler creation
func TestNewControllerHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewControllerHandler(database)

	assert.NotNil(t, handler, "Handler should not be nil")
	assert.NotNil(t, handler.database, "Database should be set")
}

// TestListControllers_NoFilters tests listing all controllers
func TestListControllers_NoFilters(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	// Mock database query
	rows := sqlmock.NewRows([]string{
		"id", "controller_id", "platform", "display_name", "status", "version",
		"capabilities", "cluster_info", "last_heartbeat", "created_at", "updated_at",
	}).
		AddRow("ctrl-1", "k8s-prod-1", "kubernetes", "K8s Production", "connected", "1.0.0",
			[]byte(`["sessions","hibernation"]`), []byte(`{"nodes":3}`), time.Now(), time.Now(), time.Now()).
		AddRow("ctrl-2", "docker-dev-1", "docker", "Docker Dev", "disconnected", "1.0.0",
			[]byte(`["sessions"]`), []byte(`{}`), nil, time.Now(), time.Now())

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE 1=1 ORDER BY created_at DESC`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers", nil)

	handler.ListControllers(c)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	var controllers []Controller
	err := json.Unmarshal(w.Body.Bytes(), &controllers)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Len(t, controllers, 2, "Should return 2 controllers")
	assert.Equal(t, "k8s-prod-1", controllers[0].ControllerID)
	assert.Equal(t, "docker-dev-1", controllers[1].ControllerID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListControllers_WithPlatformFilter tests filtering by platform
func TestListControllers_WithPlatformFilter(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "controller_id", "platform", "display_name", "status", "version",
		"capabilities", "cluster_info", "last_heartbeat", "created_at", "updated_at",
	}).
		AddRow("ctrl-1", "k8s-prod-1", "kubernetes", "K8s Production", "connected", "1.0.0",
			[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(), time.Now())

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE 1=1 AND platform = \$1 ORDER BY`).
		WithArgs("kubernetes").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers?platform=kubernetes", nil)

	handler.ListControllers(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var controllers []Controller
	json.Unmarshal(w.Body.Bytes(), &controllers)
	assert.Len(t, controllers, 1)
	assert.Equal(t, "kubernetes", controllers[0].Platform)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListControllers_WithStatusFilter tests filtering by status
func TestListControllers_WithStatusFilter(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "controller_id", "platform", "display_name", "status", "version",
		"capabilities", "cluster_info", "last_heartbeat", "created_at", "updated_at",
	}).
		AddRow("ctrl-1", "k8s-prod-1", "kubernetes", "K8s Production", "connected", "1.0.0",
			[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(), time.Now())

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE 1=1 AND status = \$1 ORDER BY`).
		WithArgs("connected").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers?status=connected", nil)

	handler.ListControllers(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var controllers []Controller
	json.Unmarshal(w.Body.Bytes(), &controllers)
	assert.Len(t, controllers, 1)
	assert.Equal(t, "connected", controllers[0].Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListControllers_DatabaseError tests database error handling
func TestListControllers_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT .* FROM platform_controllers`).
		WillReturnError(fmt.Errorf("database connection lost"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers", nil)

	handler.ListControllers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Failed to retrieve controllers", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetController_Success tests getting a controller by ID
func TestGetController_Success(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "controller_id", "platform", "display_name", "status", "version",
		"capabilities", "cluster_info", "last_heartbeat", "created_at", "updated_at",
	}).
		AddRow("ctrl-1", "k8s-prod-1", "kubernetes", "K8s Production", "connected", "1.0.0",
			[]byte(`["sessions","hibernation"]`), []byte(`{"nodes":3,"version":"1.28"}`), now, now, now)

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE id = \$1 OR controller_id = \$1`).
		WithArgs("ctrl-1").
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers/ctrl-1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "ctrl-1"}}

	handler.GetController(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var ctrl Controller
	err := json.Unmarshal(w.Body.Bytes(), &ctrl)
	require.NoError(t, err)
	assert.Equal(t, "ctrl-1", ctrl.ID)
	assert.Equal(t, "k8s-prod-1", ctrl.ControllerID)
	assert.Equal(t, "kubernetes", ctrl.Platform)
	assert.Len(t, ctrl.Capabilities, 2)
	assert.Equal(t, float64(3), ctrl.ClusterInfo["nodes"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetController_NotFound tests controller not found
func TestGetController_NotFound(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE id = \$1 OR controller_id = \$1`).
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/controllers/nonexistent", nil)
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

	handler.GetController(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRegisterController_Success tests successful controller registration
func TestRegisterController_Success(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	// Mock check for existing controller
	mock.ExpectQuery(`SELECT id FROM platform_controllers WHERE controller_id = \$1`).
		WithArgs("k8s-prod-1").
		WillReturnError(sql.ErrNoRows)

	// Mock insert
	mock.ExpectExec(`INSERT INTO platform_controllers`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := RegisterControllerRequest{
		ControllerID: "k8s-prod-1",
		Platform:     "kubernetes",
		DisplayName:  "K8s Production",
		Version:      "1.0.0",
		Capabilities: []string{"sessions", "hibernation"},
		ClusterInfo:  map[string]interface{}{"nodes": 3},
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterController(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var ctrl Controller
	err := json.Unmarshal(w.Body.Bytes(), &ctrl)
	require.NoError(t, err)
	assert.Equal(t, "k8s-prod-1", ctrl.ControllerID)
	assert.Equal(t, "kubernetes", ctrl.Platform)
	assert.Equal(t, "K8s Production", ctrl.DisplayName)
	assert.Equal(t, "connected", ctrl.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRegisterController_DefaultDisplayName tests display name generation
func TestRegisterController_DefaultDisplayName(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT id FROM platform_controllers WHERE controller_id = \$1`).
		WithArgs("docker-1").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(`INSERT INTO platform_controllers`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := RegisterControllerRequest{
		ControllerID: "docker-1",
		Platform:     "docker",
		// DisplayName intentionally omitted
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterController(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var ctrl Controller
	json.Unmarshal(w.Body.Bytes(), &ctrl)
	assert.Equal(t, "docker Controller", ctrl.DisplayName, "Should generate default display name")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRegisterController_ValidationError tests missing required fields
func TestRegisterController_ValidationError(t *testing.T) {
	handler, _, cleanup := setupControllerTest(t)
	defer cleanup()

	// Missing controller_id and platform
	reqBody := map[string]interface{}{
		"display_name": "Test Controller",
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterController(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Invalid request", response.Error)
}

// TestRegisterController_AlreadyExists tests duplicate controller
func TestRegisterController_AlreadyExists(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	// Mock finding existing controller
	rows := sqlmock.NewRows([]string{"id"}).AddRow("ctrl-1")
	mock.ExpectQuery(`SELECT id FROM platform_controllers WHERE controller_id = \$1`).
		WithArgs("k8s-prod-1").
		WillReturnRows(rows)

	reqBody := RegisterControllerRequest{
		ControllerID: "k8s-prod-1",
		Platform:     "kubernetes",
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterController(c)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller already registered", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateController_Success tests updating controller fields
func TestUpdateController_Success(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	displayName := "Updated K8s"
	status := "disconnected"

	// Mock update
	mock.ExpectExec(`UPDATE platform_controllers SET display_name = \$1, status = \$2, updated_at = \$3 WHERE id = \$4 OR controller_id = \$4`).
		WithArgs(displayName, status, sqlmock.AnyArg(), "ctrl-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock GetController (called after update)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "controller_id", "platform", "display_name", "status", "version",
		"capabilities", "cluster_info", "last_heartbeat", "created_at", "updated_at",
	}).
		AddRow("ctrl-1", "k8s-prod-1", "kubernetes", displayName, status, "1.0.0",
			[]byte(`[]`), []byte(`{}`), now, now, now)

	mock.ExpectQuery(`SELECT .* FROM platform_controllers WHERE id = \$1 OR controller_id = \$1`).
		WithArgs("ctrl-1").
		WillReturnRows(rows)

	reqBody := UpdateControllerRequest{
		DisplayName: &displayName,
		Status:      &status,
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/controllers/ctrl-1", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "ctrl-1"}}

	handler.UpdateController(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var ctrl Controller
	json.Unmarshal(w.Body.Bytes(), &ctrl)
	assert.Equal(t, displayName, ctrl.DisplayName)
	assert.Equal(t, status, ctrl.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateController_NoFieldsToUpdate tests empty update request
func TestUpdateController_NoFieldsToUpdate(t *testing.T) {
	handler, _, cleanup := setupControllerTest(t)
	defer cleanup()

	reqBody := UpdateControllerRequest{}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/controllers/ctrl-1", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "ctrl-1"}}

	handler.UpdateController(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "No fields to update", response.Error)
}

// TestUpdateController_NotFound tests updating non-existent controller
func TestUpdateController_NotFound(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	status := "connected"

	mock.ExpectExec(`UPDATE platform_controllers`).
		WithArgs(status, sqlmock.AnyArg(), "nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	reqBody := UpdateControllerRequest{
		Status: &status,
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/controllers/nonexistent", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

	handler.UpdateController(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUnregisterController_Success tests successful deletion
func TestUnregisterController_Success(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM platform_controllers WHERE id = \$1 OR controller_id = \$1`).
		WithArgs("ctrl-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/admin/controllers/ctrl-1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "ctrl-1"}}

	handler.UnregisterController(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller unregistered successfully", response["message"])
	assert.Equal(t, "ctrl-1", response["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUnregisterController_NotFound tests deleting non-existent controller
func TestUnregisterController_NotFound(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM platform_controllers WHERE id = \$1 OR controller_id = \$1`).
		WithArgs("nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/admin/controllers/nonexistent", nil)
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

	handler.UnregisterController(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestControllerUpdateHeartbeat_Success tests successful heartbeat update
func TestControllerUpdateHeartbeat_Success(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE platform_controllers SET last_heartbeat = \$1, status = 'connected', updated_at = \$1 WHERE id = \$2 OR controller_id = \$2`).
		WithArgs(sqlmock.AnyArg(), "ctrl-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/ctrl-1/heartbeat", nil)
	c.Params = []gin.Param{{Key: "id", Value: "ctrl-1"}}

	handler.UpdateHeartbeat(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Heartbeat updated successfully", response["message"])
	assert.NotNil(t, response["last_heartbeat"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestControllerUpdateHeartbeat_NotFound tests heartbeat for non-existent controller
func TestControllerUpdateHeartbeat_NotFound(t *testing.T) {
	handler, mock, cleanup := setupControllerTest(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE platform_controllers SET last_heartbeat`).
		WithArgs(sqlmock.AnyArg(), "nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/controllers/nonexistent/heartbeat", nil)
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

	handler.UpdateHeartbeat(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Controller not found", response.Error)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestControllerRegisterRoutes tests route registration
func TestControllerRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupControllerTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1/admin")

	handler.RegisterRoutes(group)

	// Verify all routes are registered
	routes := router.Routes()
	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/controllers"},
		{"GET", "/api/v1/admin/controllers/:id"},
		{"POST", "/api/v1/admin/controllers/register"},
		{"PUT", "/api/v1/admin/controllers/:id"},
		{"DELETE", "/api/v1/admin/controllers/:id"},
		{"POST", "/api/v1/admin/controllers/:id/heartbeat"},
	}

	foundCount := 0
	for _, expected := range expectedRoutes {
		for _, route := range routes {
			if route.Method == expected.method && route.Path == expected.path {
				foundCount++
				break // Found this expected route, move to next
			}
		}
	}

	assert.Equal(t, 6, foundCount, "All 6 controller routes should be registered")
}
