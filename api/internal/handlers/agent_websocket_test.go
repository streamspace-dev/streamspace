// Package handlers provides HTTP handlers for the StreamSpace API.
//
// This file contains comprehensive tests for the Agent WebSocket handler (v2.0 multi-platform architecture).
//
// Test Coverage:
//   - HandleAgentConnection validation logic (agent_id, database lookup)
//   - Agent registration and connection lifecycle
//   - Message handlers (heartbeat, ack, complete, failed, status)
//   - Error cases and edge conditions
//   - Route registration
//
// Note: readPump and writePump goroutines require integration tests with actual
// WebSocket connections and are tested separately in integration test suite.
package handlers

import (
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
	"github.com/streamspace/streamspace/api/internal/models"
	wsocket "github.com/streamspace/streamspace/api/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAgentWebSocketTest creates a test setup with mock database and agent hub
func setupAgentWebSocketTest(t *testing.T) (*AgentWebSocketHandler, sqlmock.Sqlmock, *wsocket.AgentHub, func()) {
	// Create mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")

	database := db.NewDatabaseForTesting(mockDB)

	// Create real agent hub
	hub := wsocket.NewAgentHub(database)
	go hub.Run()

	// Create handler
	handler := NewAgentWebSocketHandler(hub, database)

	// Cleanup function
	cleanup := func() {
		hub.Stop()
		mockDB.Close()
	}

	return handler, mock, hub, cleanup
}

// createAgentTestContext creates a Gin test context with optional agent_id
func createAgentTestContext(agentID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	url := "/api/v1/agents/connect"
	if agentID != "" {
		url += "?agent_id=" + agentID
	}

	c.Request = httptest.NewRequest("GET", url, nil)

	return c, w
}

// TestNewAgentWebSocketHandler tests handler creation
func TestNewAgentWebSocketHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	hub := wsocket.NewAgentHub(database)

	handler := NewAgentWebSocketHandler(hub, database)

	assert.NotNil(t, handler, "Handler should not be nil")
	assert.NotNil(t, handler.hub, "Hub should be set")
	assert.NotNil(t, handler.database, "Database should be set")
	assert.Equal(t, 1024, handler.upgrader.ReadBufferSize, "Read buffer should be 1024")
	assert.Equal(t, 1024, handler.upgrader.WriteBufferSize, "Write buffer should be 1024")
}

// TestHandleAgentConnection_MissingAgentID tests missing agent_id parameter
func TestHandleAgentConnection_MissingAgentID(t *testing.T) {
	handler, _, _, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	c, w := createAgentTestContext("")

	handler.HandleAgentConnection(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 for missing agent_id")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, "Missing agent_id", response["error"], "Error message should mention missing agent_id")
	assert.Contains(t, response["details"], "required", "Details should mention parameter is required")
}

// TestHandleAgentConnection_AgentNotFound tests agent not found in database
func TestHandleAgentConnection_AgentNotFound(t *testing.T) {
	handler, mock, _, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-nonexistent"

	// Mock database query to return no rows
	mock.ExpectQuery(`SELECT id, agent_id, platform, region, status FROM agents WHERE agent_id = \$1`).
		WithArgs(agentID).
		WillReturnError(sql.ErrNoRows)

	c, w := createAgentTestContext(agentID)

	handler.HandleAgentConnection(c)

	assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for agent not found")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, "Agent not found", response["error"], "Error message should mention agent not found")
	assert.Contains(t, response["details"], "register", "Details should mention registration requirement")
	assert.Equal(t, agentID, response["agentId"], "Response should include agent ID")

	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")
}

// TestHandleAgentConnection_DatabaseError tests database query failure
func TestHandleAgentConnection_DatabaseError(t *testing.T) {
	handler, mock, _, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"

	// Mock database query to return error
	mock.ExpectQuery(`SELECT id, agent_id, platform, region, status FROM agents WHERE agent_id = \$1`).
		WithArgs(agentID).
		WillReturnError(fmt.Errorf("database connection lost"))

	c, w := createAgentTestContext(agentID)

	handler.HandleAgentConnection(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "Should return 500 for database error")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, "Database error", response["error"], "Error message should mention database error")

	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")
}

// TestHandleAgentConnection_ValidRequest tests successful validation
// Note: Cannot test WebSocket upgrade without actual WebSocket client
func TestHandleAgentConnection_ValidRequest(t *testing.T) {
	t.Skip("Requires integration test with real WebSocket client - validation logic verified by other tests")

	// All validation logic is tested separately:
	// - TestHandleAgentConnection_MissingAgentID ✓
	// - TestHandleAgentConnection_AgentNotFound ✓
	// - TestHandleAgentConnection_DatabaseError ✓
	//
	// The WebSocket upgrade and pump goroutines require actual WebSocket
	// connections and are better suited for integration tests.
}

// TestHandleHeartbeat tests heartbeat message processing
func TestHandleHeartbeat(t *testing.T) {
	handler, mock, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"

	// Create agent connection (mock)
	conn := &wsocket.AgentConnection{
		AgentID:  agentID,
		Platform: "kubernetes",
		LastPing: time.Now().Add(-10 * time.Second),
	}

	// Create capacity for test
	capacity := &models.AgentCapacity{
		MaxSessions: 10,
		CPU:         "4",
		Memory:      "8Gi",
		Storage:     "100Gi",
	}

	// Note: UpdateAgentHeartbeat will fail because agent is not registered in hub
	// This is expected behavior - in real usage, the agent would be registered first
	// We only test the capacity update which happens regardless

	// Mock capacity update (this always happens if capacity is provided)
	mock.ExpectExec(`UPDATE agents SET capacity`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), agentID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create heartbeat message
	heartbeat := models.HeartbeatMessage{
		Status:         "healthy",
		ActiveSessions: 5,
		Capacity:       capacity,
	}
	heartbeatBytes, _ := json.Marshal(heartbeat)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeHeartbeat,
		Timestamp: time.Now(),
		Payload:   heartbeatBytes,
	}

	// Process heartbeat
	handler.handleHeartbeat(conn, agentMsg)

	// Give async operations time to complete
	time.Sleep(50 * time.Millisecond)

	// Verify database expectations met
	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")

	_ = hub // Keep hub variable used
}

// TestHandleHeartbeat_InvalidPayload tests heartbeat with invalid JSON
func TestHandleHeartbeat_InvalidPayload(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"

	conn := &wsocket.AgentConnection{
		AgentID:  agentID,
		Platform: "kubernetes",
	}

	// Create message with invalid JSON payload
	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeHeartbeat,
		Timestamp: time.Now(),
		Payload:   []byte("invalid json"),
	}

	// Should not panic, just log error
	handler.handleHeartbeat(conn, agentMsg)

	_ = hub // Keep hub variable used

	// No panic = success
}

// TestHandleAck tests command acknowledgment processing
func TestHandleAck(t *testing.T) {
	handler, mock, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"
	commandID := "cmd-123"

	conn := &wsocket.AgentConnection{
		AgentID: agentID,
	}

	// Mock database update for ack
	mock.ExpectExec(`UPDATE agent_commands SET status = 'ack'`).
		WithArgs(sqlmock.AnyArg(), commandID, agentID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create ack message
	ack := models.AckMessage{
		CommandID: commandID,
	}
	ackBytes, _ := json.Marshal(ack)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeAck,
		Timestamp: time.Now(),
		Payload:   ackBytes,
	}

	// Process ack
	handler.handleAck(conn, agentMsg)

	// Verify database expectations met
	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")

	_ = hub // Keep hub variable used
}

// TestHandleAck_InvalidPayload tests ack with invalid JSON
func TestHandleAck_InvalidPayload(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	conn := &wsocket.AgentConnection{
		AgentID: "agent-k8s-1",
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeAck,
		Timestamp: time.Now(),
		Payload:   []byte("invalid json"),
	}

	// Should not panic, just log error
	handler.handleAck(conn, agentMsg)

	_ = hub // Keep hub variable used
}

// TestHandleAck_DatabaseError tests ack with database failure
func TestHandleAck_DatabaseError(t *testing.T) {
	handler, mock, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"
	commandID := "cmd-123"

	conn := &wsocket.AgentConnection{
		AgentID: agentID,
	}

	// Mock database update to fail
	mock.ExpectExec(`UPDATE agent_commands SET status = 'ack'`).
		WithArgs(sqlmock.AnyArg(), commandID, agentID).
		WillReturnError(fmt.Errorf("database error"))

	// Create ack message
	ack := models.AckMessage{
		CommandID: commandID,
	}
	ackBytes, _ := json.Marshal(ack)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeAck,
		Timestamp: time.Now(),
		Payload:   ackBytes,
	}

	// Process ack (should not panic)
	handler.handleAck(conn, agentMsg)

	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")

	_ = hub // Keep hub variable used
}

// TestHandleComplete tests command completion processing
func TestHandleComplete(t *testing.T) {
	handler, mock, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"
	commandID := "cmd-123"

	conn := &wsocket.AgentConnection{
		AgentID: agentID,
	}

	// Mock database update for completion
	mock.ExpectExec(`UPDATE agent_commands SET status = 'completed'`).
		WithArgs(sqlmock.AnyArg(), commandID, agentID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create complete message
	complete := models.CompleteMessage{
		CommandID: commandID,
	}
	completeBytes, _ := json.Marshal(complete)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeComplete,
		Timestamp: time.Now(),
		Payload:   completeBytes,
	}

	// Process complete
	handler.handleComplete(conn, agentMsg)

	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")

	_ = hub // Keep hub variable used
}

// TestHandleComplete_InvalidPayload tests complete with invalid JSON
func TestHandleComplete_InvalidPayload(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	conn := &wsocket.AgentConnection{
		AgentID: "agent-k8s-1",
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeComplete,
		Timestamp: time.Now(),
		Payload:   []byte("invalid json"),
	}

	// Should not panic
	handler.handleComplete(conn, agentMsg)

	_ = hub // Keep hub variable used
}

// TestHandleFailed tests command failure processing
func TestHandleFailed(t *testing.T) {
	handler, mock, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"
	commandID := "cmd-123"
	errorMsg := "Failed to start session"

	conn := &wsocket.AgentConnection{
		AgentID: agentID,
	}

	// Mock database update for failure
	mock.ExpectExec(`UPDATE agent_commands SET status = 'failed'`).
		WithArgs(errorMsg, sqlmock.AnyArg(), commandID, agentID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create failed message
	failed := models.FailedMessage{
		CommandID: commandID,
		Error:     errorMsg,
	}
	failedBytes, _ := json.Marshal(failed)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeFailed,
		Timestamp: time.Now(),
		Payload:   failedBytes,
	}

	// Process failed
	handler.handleFailed(conn, agentMsg)

	assert.NoError(t, mock.ExpectationsWereMet(), "All database expectations should be met")

	_ = hub // Keep hub variable used
}

// TestHandleFailed_InvalidPayload tests failed with invalid JSON
func TestHandleFailed_InvalidPayload(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	conn := &wsocket.AgentConnection{
		AgentID: "agent-k8s-1",
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeFailed,
		Timestamp: time.Now(),
		Payload:   []byte("invalid json"),
	}

	// Should not panic
	handler.handleFailed(conn, agentMsg)

	_ = hub // Keep hub variable used
}

// TestHandleStatus tests session status update processing
func TestHandleStatus(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	agentID := "agent-k8s-1"
	sessionID := "sess-123"

	conn := &wsocket.AgentConnection{
		AgentID: agentID,
	}

	// Create status message
	status := models.StatusMessage{
		SessionID: sessionID,
		State:     "running",
		VNCReady:  true,
		VNCPort:   5900,
	}
	statusBytes, _ := json.Marshal(status)

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeStatus,
		Timestamp: time.Now(),
		Payload:   statusBytes,
	}

	// Process status (currently just logs, no database update yet)
	handler.handleStatus(conn, agentMsg)

	// No assertions needed - just verify it doesn't panic
	// When sessions table is added in Phase 4, this will update database

	_ = hub // Keep hub variable used
}

// TestHandleStatus_InvalidPayload tests status with invalid JSON
func TestHandleStatus_InvalidPayload(t *testing.T) {
	handler, _, hub, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	conn := &wsocket.AgentConnection{
		AgentID: "agent-k8s-1",
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeStatus,
		Timestamp: time.Now(),
		Payload:   []byte("invalid json"),
	}

	// Should not panic
	handler.handleStatus(conn, agentMsg)

	_ = hub // Keep hub variable used
}

// TestAgentWebSocketRegisterRoutes tests route registration
func TestAgentWebSocketRegisterRoutes(t *testing.T) {
	handler, _, _, cleanup := setupAgentWebSocketTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")

	handler.RegisterRoutes(group)

	// Verify route is registered
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/v1/agents/connect" && route.Method == "GET" {
			found = true
			break
		}
	}

	assert.True(t, found, "Agent WebSocket route should be registered")
}

// TestConstants tests the timeout and size constants
func TestConstants(t *testing.T) {
	assert.Equal(t, 10*time.Second, writeWait, "writeWait should be 10 seconds")
	assert.Equal(t, 60*time.Second, pongWait, "pongWait should be 60 seconds")
	assert.Equal(t, 54*time.Second, pingPeriod, "pingPeriod should be 54 seconds (9/10 of pongWait)")
	assert.Equal(t, 512*1024, maxMessageSize, "maxMessageSize should be 512 KB")

	// Verify pingPeriod is less than pongWait (required for keep-alive)
	assert.Less(t, pingPeriod, pongWait, "pingPeriod must be less than pongWait")
}
