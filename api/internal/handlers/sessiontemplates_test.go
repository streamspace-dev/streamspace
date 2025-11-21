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
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
)

func setupSessionTemplatesTest(t *testing.T) (*SessionTemplatesHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	database := db.NewDatabaseForTesting(mockDB)

	// K8s client and publisher can be nil for basic tests
	handler := NewSessionTemplatesHandler(database, nil, nil, "kubernetes")

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// LIST TEMPLATES TESTS
// ============================================================================

func TestListSessionTemplates_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	userID := "user123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "name", "description", "is_default", "is_public",
		"config", "tags", "usage_count", "version", "created_at", "updated_at",
	}).
		AddRow("tpl1", userID, "My Template", "Test template", false, false,
			"{}", "{}", 5, "1.0", now, now).
		AddRow("tpl2", userID, "Another Template", "Test 2", true, false,
			"{}", "{}", 10, "1.0", now, now)

	mock.ExpectQuery(`SELECT .+ FROM session_templates WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	req := httptest.NewRequest("GET", "/api/v1/session-templates", nil)
	c.Request = req

	handler.ListSessionTemplates(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "templates")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListSessionTemplates_Unauthorized(t *testing.T) {
	handler, _, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No userID set in context
	req := httptest.NewRequest("GET", "/api/v1/session-templates", nil)
	c.Request = req

	handler.ListSessionTemplates(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// CREATE TEMPLATE TESTS
// ============================================================================

func TestCreateSessionTemplate_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	userID := "user123"

	mock.ExpectExec(`INSERT INTO session_templates`).
		WithArgs(
			sqlmock.AnyArg(), // id
			userID,
			"My Template",
			"Test template",
			sqlmock.AnyArg(), // config
			false,            // is_default
			false,            // is_public
			sqlmock.AnyArg(), // tags
			0,                // usage_count
			"1.0",            // version
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)

	reqBody := map[string]interface{}{
		"name":        "My Template",
		"description": "Test template",
		"config":      map[string]interface{}{"cpu": "2000m"},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/session-templates", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.CreateSessionTemplate(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET TEMPLATE TESTS
// ============================================================================

func TestGetSessionTemplate_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "tpl123"
	userID := "user123"
	now := time.Now()

	mock.ExpectQuery(`SELECT .+ FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "description", "is_default", "is_public",
			"config", "tags", "usage_count", "version", "created_at", "updated_at",
		}).AddRow(templateID, userID, "My Template", "Test", false, false,
			"{}", "{}", 5, "1.0", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	req := httptest.NewRequest("GET", "/api/v1/session-templates/"+templateID, nil)
	c.Request = req

	handler.GetSessionTemplate(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSessionTemplate_NotFound(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "nonexistent"
	userID := "user123"

	mock.ExpectQuery(`SELECT .+ FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	req := httptest.NewRequest("GET", "/api/v1/session-templates/"+templateID, nil)
	c.Request = req

	handler.GetSessionTemplate(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UPDATE TEMPLATE TESTS
// ============================================================================

func TestUpdateSessionTemplate_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "tpl123"
	userID := "user123"
	newName := "Updated Template"

	// Check ownership
	mock.ExpectQuery(`SELECT user_id FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))

	// Update template
	mock.ExpectExec(`UPDATE session_templates SET name = \$1, updated_at = .+ WHERE id = \$2`).
		WithArgs(newName, templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	reqBody := map[string]interface{}{
		"name": newName,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/session-templates/"+templateID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateSessionTemplate(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSessionTemplate_Forbidden(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "tpl123"
	userID := "user123"
	ownerID := "different_user"

	// Template owned by different user
	mock.ExpectQuery(`SELECT user_id FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}

	reqBody := map[string]interface{}{
		"name": "Updated Template",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/session-templates/"+templateID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.UpdateSessionTemplate(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DELETE TEMPLATE TESTS
// ============================================================================

func TestDeleteSessionTemplate_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "tpl123"
	userID := "user123"

	// Check ownership
	mock.ExpectQuery(`SELECT user_id FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))

	// Delete template
	mock.ExpectExec(`DELETE FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	req := httptest.NewRequest("DELETE", "/api/v1/session-templates/"+templateID, nil)
	c.Request = req

	handler.DeleteSessionTemplate(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// CLONE TEMPLATE TESTS
// ============================================================================

func TestCloneSessionTemplate_Success(t *testing.T) {
	handler, mock, cleanup := setupSessionTemplatesTest(t)
	defer cleanup()

	templateID := "tpl123"
	userID := "user123"
	now := time.Now()

	// Get source template
	mock.ExpectQuery(`SELECT .+ FROM session_templates WHERE id = \$1`).
		WithArgs(templateID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "description", "is_default", "is_public",
			"config", "tags", "usage_count", "version", "created_at", "updated_at",
		}).AddRow(templateID, userID, "Source Template", "Test", false, false,
			"{}", "{}", 5, "1.0", now, now))

	// Create cloned template
	mock.ExpectExec(`INSERT INTO session_templates`).
		WithArgs(
			sqlmock.AnyArg(), // id
			userID,
			"Source Template (Clone)",
			sqlmock.AnyArg(), // description
			sqlmock.AnyArg(), // config
			false,            // is_default
			false,            // is_public
			sqlmock.AnyArg(), // tags
			0,                // usage_count
			"1.0",            // version
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "id", Value: templateID}}
	req := httptest.NewRequest("POST", "/api/v1/session-templates/"+templateID+"/clone", nil)
	c.Request = req

	handler.CloneSessionTemplate(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
