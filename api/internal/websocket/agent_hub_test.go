package websocket

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/websocket"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
)

// setupHubTest creates a test database and AgentHub
func setupHubTest(t *testing.T) (*AgentHub, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}

	database := &db.Database{}
	database.SetDB(mockDB)

	hub := NewAgentHub(database)

	cleanup := func() {
		hub.Stop()
		mockDB.Close()
	}

	return hub, mock, cleanup
}

// TestNewAgentHub tests hub initialization
func TestNewAgentHub(t *testing.T) {
	hub, _, cleanup := setupHubTest(t)
	defer cleanup()

	if hub == nil {
		t.Fatal("Expected hub to be initialized")
	}

	if hub.connections == nil {
		t.Error("Expected connections map to be initialized")
	}

	if hub.register == nil {
		t.Error("Expected register channel to be initialized")
	}

	if hub.unregister == nil {
		t.Error("Expected unregister channel to be initialized")
	}

	if hub.broadcast == nil {
		t.Error("Expected broadcast channel to be initialized")
	}
}

// TestRegisterAgent tests agent registration
func TestRegisterAgent(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	// Start hub in background
	go hub.Run()

	// Mock database update
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Create mock WebSocket connection
	conn := &websocket.Conn{}

	// Create agent connection
	agentConn := &AgentConnection{
		AgentID:  "test-agent",
		Conn:     conn,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	// Register agent
	err := hub.RegisterAgent(agentConn)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Wait for registration to process
	time.Sleep(100 * time.Millisecond)

	// Verify agent is connected
	if !hub.IsAgentConnected("test-agent") {
		t.Error("Expected agent to be connected")
	}

	// Verify connection is in map
	agents := hub.GetConnectedAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 connected agent, got %d", len(agents))
	}

	// Clean up - manually close channels since we're using a mock connection
	close(agentConn.Send)
}

// TestUnregisterAgent tests agent unregistration
func TestUnregisterAgent(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	// Start hub in background
	go hub.Run()

	// Mock database updates
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE agents SET status = 'offline'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Create and register agent
	conn := &websocket.Conn{}
	agentConn := &AgentConnection{
		AgentID:  "test-agent",
		Conn:     conn,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	err := hub.RegisterAgent(agentConn)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Unregister agent
	hub.UnregisterAgent("test-agent")
	time.Sleep(100 * time.Millisecond)

	// Verify agent is disconnected
	if hub.IsAgentConnected("test-agent") {
		t.Error("Expected agent to be disconnected")
	}

	// Verify connections map is empty
	agents := hub.GetConnectedAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 connected agents, got %d", len(agents))
	}
}

// TestGetConnection tests retrieving a connection
func TestGetConnection(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn := &websocket.Conn{}
	agentConn := &AgentConnection{
		AgentID:  "test-agent",
		Conn:     conn,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	err := hub.RegisterAgent(agentConn)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Get connection
	retrieved := hub.GetConnection("test-agent")
	if retrieved == nil {
		t.Fatal("Expected to retrieve connection")
	}

	if retrieved.AgentID != "test-agent" {
		t.Errorf("Expected agent ID 'test-agent', got '%s'", retrieved.AgentID)
	}

	if retrieved.Platform != "kubernetes" {
		t.Errorf("Expected platform 'kubernetes', got '%s'", retrieved.Platform)
	}

	// Get non-existent connection
	nonExistent := hub.GetConnection("non-existent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent connection")
	}

	close(agentConn.Send)
}

// TestUpdateAgentHeartbeat tests heartbeat updates
func TestUpdateAgentHeartbeat(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn := &websocket.Conn{}
	agentConn := &AgentConnection{
		AgentID:  "test-agent",
		Conn:     conn,
		Platform: "kubernetes",
		LastPing: time.Now().Add(-10 * time.Second), // 10 seconds ago
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	err := hub.RegisterAgent(agentConn)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Mock database update for heartbeat
	mock.ExpectExec(`UPDATE agents SET last_heartbeat`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Update heartbeat
	err = hub.UpdateAgentHeartbeat("test-agent")
	if err != nil {
		t.Fatalf("Failed to update heartbeat: %v", err)
	}

	// Verify LastPing was updated (should be recent)
	retrieved := hub.GetConnection("test-agent")
	if retrieved == nil {
		t.Fatal("Expected to retrieve connection")
	}

	retrieved.Mutex.RLock()
	lastPing := retrieved.LastPing
	retrieved.Mutex.RUnlock()

	if time.Since(lastPing) > 1*time.Second {
		t.Errorf("Expected LastPing to be recent, got %v", lastPing)
	}

	close(agentConn.Send)
}

// TestSendCommandToAgent tests sending commands to agents
func TestSendCommandToAgent(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn := &websocket.Conn{}
	agentConn := &AgentConnection{
		AgentID:  "test-agent",
		Conn:     conn,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	err := hub.RegisterAgent(agentConn)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create command
	payload := models.CommandPayload{
		"sessionId": "sess-123",
		"user":      "alice",
		"template":  "firefox",
	}

	command := &models.AgentCommand{
		CommandID: "cmd-123",
		AgentID:   "test-agent",
		Action:    "start_session",
		Payload:   &payload,
	}

	// Send command
	err = hub.SendCommandToAgent("test-agent", command)
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}

	// Verify message was sent to Send channel
	select {
	case msg := <-agentConn.Send:
		if len(msg) == 0 {
			t.Error("Expected message to have content")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for message")
	}

	close(agentConn.Send)
}

// TestSendCommandToDisconnectedAgent tests error when agent is offline
func TestSendCommandToDisconnectedAgent(t *testing.T) {
	hub, _, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	payload := models.CommandPayload{
		"sessionId": "sess-123",
	}

	command := &models.AgentCommand{
		CommandID: "cmd-123",
		AgentID:   "offline-agent",
		Action:    "start_session",
		Payload:   &payload,
	}

	err := hub.SendCommandToAgent("offline-agent", command)
	if err == nil {
		t.Error("Expected error when sending to disconnected agent")
	}

	if err.Error() != "agent offline-agent is not connected" {
		t.Errorf("Expected 'agent offline-agent is not connected' error, got: %v", err)
	}
}

// TestBroadcastToAllAgents tests broadcasting messages
func TestBroadcastToAllAgents(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	// Register two agents
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn1 := &websocket.Conn{}
	agentConn1 := &AgentConnection{
		AgentID:  "agent-1",
		Conn:     conn1,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	conn2 := &websocket.Conn{}
	agentConn2 := &AgentConnection{
		AgentID:  "agent-2",
		Conn:     conn2,
		Platform: "docker",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	hub.RegisterAgent(agentConn1)
	hub.RegisterAgent(agentConn2)

	time.Sleep(100 * time.Millisecond)

	// Broadcast message
	message := []byte(`{"type":"shutdown"}`)
	hub.BroadcastToAllAgents(message, "")

	// Verify both agents received the message
	select {
	case <-agentConn1.Send:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for broadcast to agent-1")
	}

	select {
	case <-agentConn2.Send:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for broadcast to agent-2")
	}

	close(agentConn1.Send)
	close(agentConn2.Send)
}

// TestBroadcastWithExclusion tests broadcasting with exclusion
func TestBroadcastWithExclusion(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	// Register two agents
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn1 := &websocket.Conn{}
	agentConn1 := &AgentConnection{
		AgentID:  "agent-1",
		Conn:     conn1,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	conn2 := &websocket.Conn{}
	agentConn2 := &AgentConnection{
		AgentID:  "agent-2",
		Conn:     conn2,
		Platform: "docker",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	hub.RegisterAgent(agentConn1)
	hub.RegisterAgent(agentConn2)

	time.Sleep(100 * time.Millisecond)

	// Broadcast message, excluding agent-1
	message := []byte(`{"type":"shutdown"}`)
	hub.BroadcastToAllAgents(message, "agent-1")

	// Verify agent-1 did NOT receive the message
	select {
	case <-agentConn1.Send:
		t.Error("Agent-1 should not receive the broadcast")
	case <-time.After(500 * time.Millisecond):
		// Good - timeout means no message
	}

	// Verify agent-2 received the message
	select {
	case <-agentConn2.Send:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for broadcast to agent-2")
	}

	close(agentConn1.Send)
	close(agentConn2.Send)
}

// TestGetConnectedAgents tests retrieving list of connected agents
func TestGetConnectedAgents(t *testing.T) {
	hub, mock, cleanup := setupHubTest(t)
	defer cleanup()

	go hub.Run()

	// Initially empty
	agents := hub.GetConnectedAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 connected agents initially, got %d", len(agents))
	}

	// Register two agents
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "agent-2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn1 := &websocket.Conn{}
	agentConn1 := &AgentConnection{
		AgentID:  "agent-1",
		Conn:     conn1,
		Platform: "kubernetes",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	conn2 := &websocket.Conn{}
	agentConn2 := &AgentConnection{
		AgentID:  "agent-2",
		Conn:     conn2,
		Platform: "docker",
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	hub.RegisterAgent(agentConn1)
	hub.RegisterAgent(agentConn2)

	time.Sleep(100 * time.Millisecond)

	// Get connected agents
	agents = hub.GetConnectedAgents()
	if len(agents) != 2 {
		t.Errorf("Expected 2 connected agents, got %d", len(agents))
	}

	// Verify agent IDs
	agentMap := make(map[string]bool)
	for _, id := range agents {
		agentMap[id] = true
	}

	if !agentMap["agent-1"] {
		t.Error("Expected agent-1 in connected agents")
	}

	if !agentMap["agent-2"] {
		t.Error("Expected agent-2 in connected agents")
	}

	close(agentConn1.Send)
	close(agentConn2.Send)
}
