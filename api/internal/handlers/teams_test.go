package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTeamsTest creates a test handler with mocked database
func setupTeamsTest(t *testing.T) (*TeamHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewTeamHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewTeamHandler tests handler initialization
func TestNewTeamHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewTeamHandler(database)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.database)
	assert.NotNil(t, handler.teamRBAC)
}

// TestTeamRegisterRoutes tests route registration
func TestTeamRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupTeamsTest(t)
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
		{"GET", "/api/v1/teams/:teamId/permissions"},
		{"GET", "/api/v1/teams/:teamId/role-info"},
		{"GET", "/api/v1/teams/:teamId/my-permissions"},
		{"GET", "/api/v1/teams/:teamId/check-permission/:permission"},
		{"GET", "/api/v1/teams/:teamId/sessions"},
		{"GET", "/api/v1/teams/my-teams"},
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

// TestGetTeamPermissions_Success tests getting all team role permissions
func TestGetTeamPermissions_Success(t *testing.T) {
	handler, mock, cleanup := setupTeamsTest(t)
	defer cleanup()

	// Mock permissions query
	rows := sqlmock.NewRows([]string{"role", "permission", "description"}).
		AddRow("owner", "team.sessions.create", "Can create team sessions").
		AddRow("owner", "team.sessions.delete", "Can delete team sessions").
		AddRow("admin", "team.sessions.view", "Can view team sessions").
		AddRow("member", "team.sessions.view", "Can view team sessions")

	mock.ExpectQuery(`SELECT role, permission, description FROM team_role_permissions`).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/permissions", nil)
	c.Params = []gin.Param{{Key: "teamId", Value: "team-1"}}

	handler.GetTeamPermissions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	permissions := response["permissions"].(map[string]interface{})
	assert.Contains(t, permissions, "owner")
	assert.Contains(t, permissions, "admin")
	assert.Contains(t, permissions, "member")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTeamPermissions_DatabaseError tests database failure
func TestGetTeamPermissions_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupTeamsTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT role, permission, description FROM team_role_permissions`).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/permissions", nil)

	handler.GetTeamPermissions(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Failed to get team permissions")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTeamRoleInfo_Success tests getting team role information
func TestGetTeamRoleInfo_Success(t *testing.T) {
	handler, mock, cleanup := setupTeamsTest(t)
	defer cleanup()

	// Mock roles query
	rolesRows := sqlmock.NewRows([]string{"role"}).
		AddRow("owner").
		AddRow("admin").
		AddRow("member")

	mock.ExpectQuery(`SELECT DISTINCT role FROM team_role_permissions`).
		WillReturnRows(rolesRows)

	// Mock permissions for owner
	ownerPerms := sqlmock.NewRows([]string{"permission"}).
		AddRow("team.sessions.create").
		AddRow("team.sessions.delete").
		AddRow("team.sessions.view")
	mock.ExpectQuery(`SELECT permission FROM team_role_permissions WHERE role`).
		WithArgs("owner").
		WillReturnRows(ownerPerms)

	// Mock permissions for admin
	adminPerms := sqlmock.NewRows([]string{"permission"}).
		AddRow("team.sessions.view").
		AddRow("team.sessions.create")
	mock.ExpectQuery(`SELECT permission FROM team_role_permissions WHERE role`).
		WithArgs("admin").
		WillReturnRows(adminPerms)

	// Mock permissions for member
	memberPerms := sqlmock.NewRows([]string{"permission"}).
		AddRow("team.sessions.view")
	mock.ExpectQuery(`SELECT permission FROM team_role_permissions WHERE role`).
		WithArgs("member").
		WillReturnRows(memberPerms)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/role-info", nil)
	c.Params = []gin.Param{{Key: "teamId", Value: "team-1"}}

	handler.GetTeamRoleInfo(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	roles := response["roles"].([]interface{})
	assert.Len(t, roles, 3)

	// Verify owner role
	ownerRole := roles[0].(map[string]interface{})
	assert.Equal(t, "owner", ownerRole["role"])
	ownerPermsArray := ownerRole["permissions"].([]interface{})
	assert.Len(t, ownerPermsArray, 3)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTeamRoleInfo_DatabaseError tests database failure
func TestGetTeamRoleInfo_DatabaseError(t *testing.T) {
	handler, mock, cleanup := setupTeamsTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT DISTINCT role FROM team_role_permissions`).
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/role-info", nil)

	handler.GetTeamRoleInfo(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Failed to get team roles")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetMyTeamPermissions_Success tests getting current user's permissions
func TestGetMyTeamPermissions_Success(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestGetMyTeamPermissions_NoAuth tests missing authentication
func TestGetMyTeamPermissions_NoAuth(t *testing.T) {
	handler, _, cleanup := setupTeamsTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/my-permissions", nil)
	c.Params = []gin.Param{{Key: "teamId", Value: "team-1"}}
	// No userID set in context

	handler.GetMyTeamPermissions(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not authenticated")
}

// TestGetMyTeamPermissions_InvalidUserID tests invalid user ID type
func TestGetMyTeamPermissions_InvalidUserID(t *testing.T) {
	handler, _, cleanup := setupTeamsTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/my-permissions", nil)
	c.Params = []gin.Param{{Key: "teamId", Value: "team-1"}}
	c.Set("userID", 12345) // Wrong type

	handler.GetMyTeamPermissions(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid user ID")
}

// TestGetMyTeamPermissions_NotAMember tests user not in team
func TestGetMyTeamPermissions_NotAMember(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestCheckPermission_Success tests checking a specific permission
func TestCheckPermission_Success(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestCheckPermission_NoAuth tests missing authentication
func TestCheckPermission_NoAuth(t *testing.T) {
	handler, _, cleanup := setupTeamsTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/team-1/check-permission/team.sessions.view", nil)
	c.Params = []gin.Param{
		{Key: "teamId", Value: "team-1"},
		{Key: "permission", Value: "team.sessions.view"},
	}
	// No userID set

	handler.CheckPermission(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not authenticated")
}

// TestCheckPermission_NoPermission tests when user lacks permission
func TestCheckPermission_NoPermission(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestListTeamSessions_Success tests listing team sessions
func TestListTeamSessions_Success(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestListTeamSessions_NoPermission tests listing sessions without permission
func TestListTeamSessions_NoPermission(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestGetMyTeams_Success tests getting user's teams
func TestGetMyTeams_Success(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}

// TestGetMyTeams_NoAuth tests missing authentication
func TestGetMyTeams_NoAuth(t *testing.T) {
	handler, _, cleanup := setupTeamsTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/my-teams", nil)
	// No userID set

	handler.GetMyTeams(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not authenticated")
}

// TestGetMyTeams_EmptyResult tests user with no teams
func TestGetMyTeams_EmptyResult(t *testing.T) {
	t.Skip("Skipped: Requires TeamRBAC middleware integration (integration test territory)")
	// This test requires real TeamRBAC middleware with complex query patterns
	// Should be tested in integration test suite with real database
}
