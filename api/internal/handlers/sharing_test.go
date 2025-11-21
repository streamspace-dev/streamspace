package handlers

import (
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

// setupSharingTest creates a test handler with mocked database
func setupSharingTest(t *testing.T) (*SharingHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSharingHandler(database)

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewSharingHandler tests handler initialization
func TestNewSharingHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewSharingHandler(database)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
}

// TestSharingRegisterRoutes tests route registration
func TestSharingRegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupSharingTest(t)
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
		{"POST", "/api/v1/sessions/:id/share"},
		{"GET", "/api/v1/sessions/:id/shares"},
		{"DELETE", "/api/v1/sessions/:id/shares/:shareId"},
		{"POST", "/api/v1/sessions/:id/transfer"},
		{"POST", "/api/v1/sessions/:id/invitations"},
		{"GET", "/api/v1/sessions/:id/invitations"},
		{"DELETE", "/api/v1/invitations/:token"},
		{"POST", "/api/v1/invitations/:token/accept"},
		{"GET", "/api/v1/sessions/:id/collaborators"},
		{"POST", "/api/v1/sessions/:id/collaborators/:userId/activity"},
		{"DELETE", "/api/v1/sessions/:id/collaborators/:userId"},
		{"GET", "/api/v1/shared-sessions"},
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

// TestCreateShare_Success tests creating a direct share
func TestCreateShare_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	ownerID := "owner-456"
	sharedWithID := "user-789"
	userID := ownerID

	// Mock session owner query
	mock.ExpectQuery(`SELECT user_id FROM sessions WHERE id`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))

	// Mock user existence check
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE id`).
		WithArgs(sharedWithID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock share creation
	mock.ExpectExec(`INSERT INTO session_shares`).
		WithArgs(sqlmock.AnyArg(), sessionID, ownerID, sharedWithID, "view", sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"sharedWithUserId":"user-789","permissionLevel":"view"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/share", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}
	c.Set("userID", userID)

	handler.CreateShare(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "id")
	assert.Contains(t, response, "shareToken")
	assert.Contains(t, response["message"], "successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateShare_InvalidPermission tests invalid permission level
func TestCreateShare_InvalidPermission(t *testing.T) {
	handler, _, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	userID := "owner-456"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"sharedWithUserId":"user-789","permissionLevel":"invalid"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/share", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}
	c.Set("userID", userID)

	handler.CreateShare(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid permission level")
}

// TestCreateShare_NotOwner tests sharing when not the owner
func TestCreateShare_NotOwner(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	ownerID := "owner-456"
	userID := "other-user-789"

	// Mock session owner query
	mock.ExpectQuery(`SELECT user_id FROM sessions WHERE id`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"sharedWithUserId":"user-999","permissionLevel":"view"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/share", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}
	c.Set("userID", userID)

	handler.CreateShare(c)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Only the session owner")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateShare_UserNotFound tests sharing with non-existent user
func TestCreateShare_UserNotFound(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	ownerID := "owner-456"
	sharedWithID := "user-789"
	userID := ownerID

	// Mock session owner query
	mock.ExpectQuery(`SELECT user_id FROM sessions WHERE id`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))

	// Mock user existence check - user not found
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE id`).
		WithArgs(sharedWithID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"sharedWithUserId":"user-789","permissionLevel":"view"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/share", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}
	c.Set("userID", userID)

	handler.CreateShare(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "User not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListShares_Success tests listing shares
func TestListShares_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "session_id", "owner_user_id", "shared_with_user_id",
		"permission_level", "share_token", "expires_at", "created_at",
		"accepted_at", "revoked_at", "username", "full_name", "email",
	}).
		AddRow("share-1", sessionID, "owner-1", "user-2",
			"view", "token-1", nil, now, now, nil, "user2", "User Two", "user2@example.com").
		AddRow("share-2", sessionID, "owner-1", "user-3",
			"collaborate", "token-2", nil, now, nil, nil, "user3", "User Three", "user3@example.com")

	mock.ExpectQuery(`SELECT`).
		WithArgs(sessionID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-123/shares", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.ListShares(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	shares := response["shares"].([]interface{})
	assert.Len(t, shares, 2)

	share1 := shares[0].(map[string]interface{})
	assert.Equal(t, "share-1", share1["id"])
	assert.Equal(t, "view", share1["permissionLevel"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeShare_Success tests revoking a share
func TestRevokeShare_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	shareID := "share-456"

	mock.ExpectExec(`UPDATE session_shares SET revoked_at`).
		WithArgs(sqlmock.AnyArg(), shareID, sessionID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/sessions/sess-123/shares/share-456", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: sessionID},
		{Key: "shareId", Value: shareID},
	}

	handler.RevokeShare(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "revoked successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestTransferOwnership_Success tests transferring ownership
func TestTransferOwnership_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	newOwnerID := "user-456"

	// Mock user existence check
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE id`).
		WithArgs(newOwnerID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock ownership transfer
	mock.ExpectExec(`UPDATE sessions SET user_id`).
		WithArgs(newOwnerID, sqlmock.AnyArg(), sessionID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"newOwnerUserId":"user-456"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/transfer", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.TransferOwnership(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "transferred successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestTransferOwnership_UserNotFound tests transfer to non-existent user
func TestTransferOwnership_UserNotFound(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	newOwnerID := "user-456"

	// Mock user existence check - user not found
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE id`).
		WithArgs(newOwnerID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"newOwnerUserId":"user-456"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/transfer", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.TransferOwnership(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "User not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateInvitation_Success tests creating an invitation link
func TestCreateInvitation_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	ownerID := "owner-456"

	// Mock session owner query
	mock.ExpectQuery(`SELECT user_id FROM sessions WHERE id`).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))

	// Mock invitation creation
	mock.ExpectExec(`INSERT INTO session_share_invitations`).
		WithArgs(sqlmock.AnyArg(), sessionID, ownerID, sqlmock.AnyArg(), "view", 5, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"permissionLevel":"view","maxUses":5}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/invitations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.CreateInvitation(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "id")
	assert.Contains(t, response, "invitationToken")
	assert.Contains(t, response["message"], "created successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateInvitation_InvalidPermission tests invalid permission
func TestCreateInvitation_InvalidPermission(t *testing.T) {
	handler, _, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"permissionLevel":"invalid","maxUses":5}`
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/invitations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.CreateInvitation(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid permission level")
}

// TestListInvitations_Success tests listing invitations
func TestListInvitations_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "session_id", "created_by", "invitation_token", "permission_level",
		"max_uses", "use_count", "expires_at", "created_at",
	}).
		AddRow("inv-1", sessionID, "owner-1", "token-1", "view", 5, 2, expiresAt, now).
		AddRow("inv-2", sessionID, "owner-1", "token-2", "collaborate", 10, 10, nil, now)

	mock.ExpectQuery(`SELECT id, session_id, created_by, invitation_token`).
		WithArgs(sessionID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-123/invitations", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.ListInvitations(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	invitations := response["invitations"].([]interface{})
	assert.Len(t, invitations, 2)

	inv1 := invitations[0].(map[string]interface{})
	assert.Equal(t, "inv-1", inv1["id"])
	assert.Equal(t, false, inv1["isExpired"])
	assert.Equal(t, false, inv1["isExhausted"])

	inv2 := invitations[1].(map[string]interface{})
	assert.Equal(t, true, inv2["isExhausted"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeInvitation_Success tests revoking an invitation
func TestRevokeInvitation_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	token := "token-123"

	mock.ExpectExec(`DELETE FROM session_share_invitations WHERE invitation_token`).
		WithArgs(token).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/invitations/token-123", nil)
	c.Params = []gin.Param{{Key: "token", Value: token}}

	handler.RevokeInvitation(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "revoked successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAcceptInvitation_Success tests accepting an invitation
func TestAcceptInvitation_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	token := "token-123"
	userID := "user-456"
	sessionID := "sess-789"

	// Mock invitation query
	mock.ExpectQuery(`SELECT id, session_id, created_by, permission_level, max_uses, use_count, expires_at`).
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "created_by", "permission_level", "max_uses", "use_count", "expires_at",
		}).AddRow("inv-1", sessionID, "owner-1", "view", 5, 2, nil))

	// Mock share creation
	mock.ExpectExec(`INSERT INTO session_shares`).
		WithArgs(sqlmock.AnyArg(), sessionID, "owner-1", userID, "view", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock use count increment
	mock.ExpectExec(`UPDATE session_share_invitations SET use_count`).
		WithArgs("inv-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"userId":"user-456"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/invitations/token-123/accept", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "token", Value: token}}

	handler.AcceptInvitation(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, sessionID, response["sessionId"])
	assert.Contains(t, response["message"], "accepted successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAcceptInvitation_Expired tests accepting an expired invitation
func TestAcceptInvitation_Expired(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	token := "token-123"
	pastTime := time.Now().Add(-24 * time.Hour)

	// Mock invitation query with expired date
	mock.ExpectQuery(`SELECT id, session_id, created_by, permission_level, max_uses, use_count, expires_at`).
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "created_by", "permission_level", "max_uses", "use_count", "expires_at",
		}).AddRow("inv-1", "sess-789", "owner-1", "view", 5, 2, pastTime))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"userId":"user-456"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/invitations/token-123/accept", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "token", Value: token}}

	handler.AcceptInvitation(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "expired")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAcceptInvitation_Exhausted tests accepting an exhausted invitation
func TestAcceptInvitation_Exhausted(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	token := "token-123"

	// Mock invitation query with exhausted usage
	mock.ExpectQuery(`SELECT id, session_id, created_by, permission_level, max_uses, use_count, expires_at`).
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "created_by", "permission_level", "max_uses", "use_count", "expires_at",
		}).AddRow("inv-1", "sess-789", "owner-1", "view", 5, 5, nil))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"userId":"user-456"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/invitations/token-123/accept", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "token", Value: token}}

	handler.AcceptInvitation(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "fully used")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListCollaborators_Success tests listing active collaborators
func TestListCollaborators_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "permission_level",
		"joined_at", "last_activity", "is_active", "username", "full_name",
	}).
		AddRow("collab-1", sessionID, "user-1", "view", now, now, true, "user1", "User One").
		AddRow("collab-2", sessionID, "user-2", "collaborate", now, now, true, "user2", "User Two")

	mock.ExpectQuery(`SELECT`).
		WithArgs(sessionID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/sessions/sess-123/collaborators", nil)
	c.Params = []gin.Param{{Key: "id", Value: sessionID}}

	handler.ListCollaborators(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	collaborators := response["collaborators"].([]interface{})
	assert.Len(t, collaborators, 2)

	collab1 := collaborators[0].(map[string]interface{})
	assert.Equal(t, "collab-1", collab1["id"])
	assert.Equal(t, "view", collab1["permissionLevel"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateCollaboratorActivity_Success tests updating activity
func TestUpdateCollaboratorActivity_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	userID := "user-456"

	// Mock permission level query
	mock.ExpectQuery(`SELECT permission_level FROM session_shares`).
		WithArgs(sessionID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"permission_level"}).AddRow("view"))

	// Mock collaborator upsert
	mock.ExpectExec(`INSERT INTO session_collaborators`).
		WithArgs(sqlmock.AnyArg(), sessionID, userID, "view", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/sessions/sess-123/collaborators/user-456/activity", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: sessionID},
		{Key: "userId", Value: userID},
	}

	handler.UpdateCollaboratorActivity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRemoveCollaborator_Success tests removing a collaborator
func TestRemoveCollaborator_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	sessionID := "sess-123"
	userID := "user-456"

	mock.ExpectExec(`UPDATE session_collaborators SET is_active`).
		WithArgs(sessionID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/sessions/sess-123/collaborators/user-456", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: sessionID},
		{Key: "userId", Value: userID},
	}

	handler.RemoveCollaborator(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["message"], "removed successfully")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListSharedSessions_Success tests listing shared sessions
func TestListSharedSessions_Success(t *testing.T) {
	handler, mock, cleanup := setupSharingTest(t)
	defer cleanup()

	userID := "user-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "template_name", "state", "app_type",
		"created_at", "url", "permission_level", "shared_at", "owner_username",
	}).
		AddRow("sess-1", "owner-1", "firefox", "running", "browser",
			now, "http://firefox.local", "view", now, "owner1").
		AddRow("sess-2", "owner-2", "vscode", "hibernated", "ide",
			now, nil, "collaborate", now.Add(-1*time.Hour), "owner2")

	mock.ExpectQuery(`SELECT`).
		WithArgs(userID, sqlmock.AnyArg()).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/shared-sessions?userId=user-123", nil)

	handler.ListSharedSessions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["total"])

	sessions := response["sessions"].([]interface{})
	assert.Len(t, sessions, 2)

	sess1 := sessions[0].(map[string]interface{})
	assert.Equal(t, "sess-1", sess1["id"])
	assert.Equal(t, "view", sess1["permissionLevel"])
	assert.Equal(t, true, sess1["isShared"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListSharedSessions_NoUserID tests missing userId parameter
func TestListSharedSessions_NoUserID(t *testing.T) {
	handler, _, cleanup := setupSharingTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/shared-sessions", nil)

	handler.ListSharedSessions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "userId parameter required")
}
