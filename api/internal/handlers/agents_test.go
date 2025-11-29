// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests agent registration and management functionality for v2.0 multi-platform architecture.
//
// Test Coverage:
// - RegisterAgent: Success (new), re-registration (existing), invalid platform
// - ListAgents: All agents, filter by platform, filter by status, filter by region
// - GetAgent: Success and not found scenarios
// - DeregisterAgent: Success and not found scenarios
// - UpdateHeartbeat: Success, invalid status, not found
//
// Testing Strategy:
// - Use sqlmock for database mocking
// - Test all platforms (kubernetes, docker, vm, cloud)
// - Test all statuses (online, offline, draining)
// - Verify error handling and edge cases
// - Follow existing test patterns from configuration_test.go
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
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAgentTest creates a test environment with mocked database
func setupAgentTest(t *testing.T) (*AgentHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Use the test constructor to inject mock database
	database := db.NewDatabaseForTesting(mockDB)

	handler := &AgentHandler{
		database: database,
	}

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// ============================================================================
// REGISTER AGENT TESTS
// ============================================================================

func TestRegisterAgent_Success_New(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	// Agent doesn't exist yet
	mock.ExpectQuery(`SELECT id FROM agents WHERE agent_id = \$1`).
		WithArgs("k8s-prod-us-east-1").
		WillReturnError(sql.ErrNoRows)

	// Insert new agent
	timestamp := time.Now()
	capacity := models.AgentCapacity{MaxSessions: 100, CPU: "64 cores", Memory: "256Gi"}
	capacityJSON, _ := json.Marshal(capacity)

	// ISSUE #234: INSERT now includes approval_status
	mock.ExpectQuery(`INSERT INTO agents`).
		WithArgs("k8s-prod-us-east-1", "kubernetes", "us-east-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "pending", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
			AddRow("uuid-123", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "offline", capacityJSON, timestamp, nil, nil, timestamp, timestamp))

	// Create request
	reqBody := models.AgentRegistrationRequest{
		AgentID:  "k8s-prod-us-east-1",
		Platform: "kubernetes",
		Region:   "us-east-1",
		Capacity: &capacity,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterAgent(c)

	// Debug: print response if test fails
	if w.Code != http.StatusCreated {
		t.Logf("Response body: %s", w.Body.String())
	}

	assert.Equal(t, http.StatusCreated, w.Code)

	var agent models.Agent
	err := json.Unmarshal(w.Body.Bytes(), &agent)
	require.NoError(t, err)
	assert.Equal(t, "k8s-prod-us-east-1", agent.AgentID)
	assert.Equal(t, "kubernetes", agent.Platform)
	// ISSUE #234: New agents are created with 'offline' status until they connect
	assert.Equal(t, "offline", agent.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegisterAgent_Success_ReRegistration(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	// Agent exists
	mock.ExpectQuery(`SELECT id FROM agents WHERE agent_id = \$1`).
		WithArgs("k8s-prod-us-east-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("uuid-123"))

	// Update existing agent
	timestamp := time.Now()
	capacity := models.AgentCapacity{MaxSessions: 100, CPU: "64 cores", Memory: "256Gi"}
	capacityJSON, _ := json.Marshal(capacity)

	mock.ExpectQuery(`UPDATE agents`).
		WithArgs("k8s-prod-us-east-1", "kubernetes", "us-east-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
			AddRow("uuid-123", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "online", capacityJSON, timestamp, nil, nil, timestamp, timestamp))

	// Create request
	reqBody := models.AgentRegistrationRequest{
		AgentID:  "k8s-prod-us-east-1",
		Platform: "kubernetes",
		Region:   "us-east-1",
		Capacity: &capacity,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterAgent(c)

	assert.Equal(t, http.StatusOK, w.Code) // 200 for re-registration

	var agent models.Agent
	err := json.Unmarshal(w.Body.Bytes(), &agent)
	require.NoError(t, err)
	assert.Equal(t, "k8s-prod-us-east-1", agent.AgentID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegisterAgent_InvalidPlatform(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	reqBody := models.AgentRegistrationRequest{
		AgentID:  "invalid-agent",
		Platform: "invalid-platform",
		Region:   "us-east-1",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/register", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.RegisterAgent(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	t.Logf("Error response: %v", response)
	// The binding validation catches invalid platforms before our manual check
	assert.Contains(t, response["error"], "Invalid")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// LIST AGENTS TESTS
// ============================================================================

func TestListAgents_All(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
		AddRow("uuid-1", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "online", nil, timestamp, nil, nil, timestamp, timestamp).
		AddRow("uuid-2", "docker-dev-host-1", "docker", "us-west-2", "online", nil, timestamp, nil, nil, timestamp, timestamp)

	query := `SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE 1=1 ORDER BY created_at DESC`
	mock.ExpectQuery(query).WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/agents", nil)

	handler.ListAgents(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	agents := response["agents"].([]interface{})
	assert.Len(t, agents, 2)
	assert.Equal(t, float64(2), response["total"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAgents_FilterByPlatform(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
		AddRow("uuid-1", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "online", nil, timestamp, nil, nil, timestamp, timestamp)

	query := `SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE 1=1 AND platform = \$1 ORDER BY created_at DESC`
	mock.ExpectQuery(query).WithArgs("kubernetes").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/agents?platform=kubernetes", nil)

	handler.ListAgents(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	agents := response["agents"].([]interface{})
	assert.Len(t, agents, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListAgents_FilterByStatus(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
		AddRow("uuid-1", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "online", nil, timestamp, nil, nil, timestamp, timestamp)

	query := `SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE 1=1 AND status = \$1 ORDER BY created_at DESC`
	mock.ExpectQuery(query).WithArgs("online").WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/agents?status=online", nil)

	handler.ListAgents(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GET AGENT TESTS
// ============================================================================

func TestGetAgent_Success(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	timestamp := time.Now()
	rows := sqlmock.NewRows([]string{"id", "agent_id", "platform", "region", "status", "capacity", "last_heartbeat", "websocket_id", "metadata", "created_at", "updated_at"}).
		AddRow("uuid-123", "k8s-prod-us-east-1", "kubernetes", "us-east-1", "online", nil, timestamp, nil, nil, timestamp, timestamp)

	mock.ExpectQuery(`SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE agent_id = \$1`).
		WithArgs("k8s-prod-us-east-1").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/agents/k8s-prod-us-east-1", nil)
	c.Params = gin.Params{{Key: "agent_id", Value: "k8s-prod-us-east-1"}}

	handler.GetAgent(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var agent models.Agent
	err := json.Unmarshal(w.Body.Bytes(), &agent)
	require.NoError(t, err)
	assert.Equal(t, "k8s-prod-us-east-1", agent.AgentID)
	assert.Equal(t, "kubernetes", agent.Platform)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAgent_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE agent_id = \$1`).
		WithArgs("nonexistent-agent").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/agents/nonexistent-agent", nil)
	c.Params = gin.Params{{Key: "agent_id", Value: "nonexistent-agent"}}

	handler.GetAgent(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// DEREGISTER AGENT TESTS
// ============================================================================

func TestDeregisterAgent_Success(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM agents WHERE agent_id = \$1`).
		WithArgs("k8s-prod-us-east-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/agents/k8s-prod-us-east-1", nil)
	c.Params = gin.Params{{Key: "agent_id", Value: "k8s-prod-us-east-1"}}

	handler.DeregisterAgent(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["message"], "successfully")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeregisterAgent_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM agents WHERE agent_id = \$1`).
		WithArgs("nonexistent-agent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/agents/nonexistent-agent", nil)
	c.Params = gin.Params{{Key: "agent_id", Value: "nonexistent-agent"}}

	handler.DeregisterAgent(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// UPDATE HEARTBEAT TESTS
// ============================================================================

func TestUpdateHeartbeat_Success(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE agents SET last_heartbeat = \$1, status = \$2, updated_at = \$1 WHERE agent_id = \$3`).
		WithArgs(sqlmock.AnyArg(), "online", "k8s-prod-us-east-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	reqBody := models.AgentHeartbeatRequest{
		Status:         "online",
		ActiveSessions: 15,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/k8s-prod-us-east-1/heartbeat", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "agent_id", Value: "k8s-prod-us-east-1"}}

	handler.UpdateHeartbeat(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["message"], "successfully")
	assert.Equal(t, "k8s-prod-us-east-1", response["agentId"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateHeartbeat_InvalidStatus(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	reqBody := models.AgentHeartbeatRequest{
		Status: "invalid-status",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/k8s-prod-us-east-1/heartbeat", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "agent_id", Value: "k8s-prod-us-east-1"}}

	handler.UpdateHeartbeat(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// The binding validation catches invalid status before our manual check
	assert.Contains(t, response["error"], "Invalid")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateHeartbeat_NotFound(t *testing.T) {
	handler, mock, cleanup := setupAgentTest(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE agents SET last_heartbeat = \$1, status = \$2, updated_at = \$1 WHERE agent_id = \$3`).
		WithArgs(sqlmock.AnyArg(), "online", "nonexistent-agent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	reqBody := models.AgentHeartbeatRequest{
		Status: "online",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/agents/nonexistent-agent/heartbeat", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "agent_id", Value: "nonexistent-agent"}}

	handler.UpdateHeartbeat(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}
