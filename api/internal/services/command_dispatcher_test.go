package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/websocket"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	internalWebsocket "github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// setupDispatcherTest creates test database, hub, and dispatcher
func setupDispatcherTest(t *testing.T) (*CommandDispatcher, *internalWebsocket.AgentHub, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}

	database := &db.Database{}
	database.SetDB(mockDB)

	hub := internalWebsocket.NewAgentHub(database)
	go hub.Run()

	dispatcher := NewCommandDispatcher(database, hub)

	cleanup := func() {
		dispatcher.Stop()
		hub.Stop()
		mockDB.Close()
	}

	return dispatcher, hub, mock, cleanup
}

// TestNewCommandDispatcher tests dispatcher initialization
func TestNewCommandDispatcher(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	if dispatcher == nil {
		t.Fatal("Expected dispatcher to be initialized")
	}

	if dispatcher.queue == nil {
		t.Error("Expected queue channel to be initialized")
	}

	if dispatcher.workers != 10 {
		t.Errorf("Expected 10 default workers, got %d", dispatcher.workers)
	}

	if dispatcher.database == nil {
		t.Error("Expected database to be set")
	}

	if dispatcher.hub == nil {
		t.Error("Expected hub to be set")
	}
}

// TestSetWorkers tests worker count configuration
func TestSetWorkers(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Set valid worker count
	dispatcher.SetWorkers(20)
	if dispatcher.workers != 20 {
		t.Errorf("Expected 20 workers, got %d", dispatcher.workers)
	}

	// Try to set invalid worker count (should be ignored)
	dispatcher.SetWorkers(0)
	if dispatcher.workers != 20 {
		t.Error("Expected worker count to remain unchanged for invalid value")
	}

	dispatcher.SetWorkers(-5)
	if dispatcher.workers != 20 {
		t.Error("Expected worker count to remain unchanged for negative value")
	}
}

// TestDispatchCommand tests queueing a command
func TestDispatchCommand(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	payload := models.CommandPayload{
		"sessionId": "sess-123",
	}

	command := &models.AgentCommand{
		CommandID: "cmd-123",
		AgentID:   "test-agent",
		Action:    "start_session",
		Payload:   &payload,
		Status:    "pending",
	}

	err := dispatcher.DispatchCommand(command)
	if err != nil {
		t.Fatalf("Failed to dispatch command: %v", err)
	}

	// Verify command was queued
	if dispatcher.GetQueueLength() != 1 {
		t.Errorf("Expected 1 command in queue, got %d", dispatcher.GetQueueLength())
	}
}

// TestDispatchCommandValidation tests command validation
func TestDispatchCommandValidation(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Test nil command
	err := dispatcher.DispatchCommand(nil)
	if err == nil {
		t.Error("Expected error for nil command")
	}

	// Test empty command ID
	err = dispatcher.DispatchCommand(&models.AgentCommand{
		AgentID: "test-agent",
		Action:  "start_session",
	})
	if err == nil {
		t.Error("Expected error for empty command_id")
	}

	// Test empty agent ID
	err = dispatcher.DispatchCommand(&models.AgentCommand{
		CommandID: "cmd-123",
		Action:    "start_session",
	})
	if err == nil {
		t.Error("Expected error for empty agent_id")
	}
}

// TestGetQueueCapacity tests queue capacity
func TestGetQueueCapacity(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	capacity := dispatcher.GetQueueCapacity()
	if capacity != 1000 {
		t.Errorf("Expected queue capacity of 1000, got %d", capacity)
	}
}

// TestProcessCommandAgentNotConnected tests handling disconnected agent
func TestProcessCommandAgentNotConnected(t *testing.T) {
	dispatcher, _, mock, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Start dispatcher
	go dispatcher.Start()

	// Mock database update for failed command
	mock.ExpectExec(`UPDATE agent_commands SET status = 'failed'`).
		WithArgs("agent is not connected", sqlmock.AnyArg(), "cmd-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	payload := models.CommandPayload{
		"sessionId": "sess-123",
	}

	command := &models.AgentCommand{
		CommandID: "cmd-123",
		AgentID:   "offline-agent",
		Action:    "start_session",
		Payload:   &payload,
		Status:    "pending",
	}

	err := dispatcher.DispatchCommand(command)
	if err != nil {
		t.Fatalf("Failed to dispatch command: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestProcessCommandAgentConnected tests successful command dispatch
func TestProcessCommandAgentConnected(t *testing.T) {
	dispatcher, hub, mock, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Register a mock agent
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn := &websocket.Conn{}
	agentConn := &internalWebsocket.AgentConnection{
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

	// Start dispatcher
	go dispatcher.Start()

	// Mock database update for sent command
	mock.ExpectExec(`UPDATE agent_commands SET status = 'sent'`).
		WithArgs(sqlmock.AnyArg(), "cmd-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	payload := models.CommandPayload{
		"sessionId": "sess-123",
	}

	command := &models.AgentCommand{
		CommandID: "cmd-123",
		AgentID:   "test-agent",
		Action:    "start_session",
		Payload:   &payload,
		Status:    "pending",
	}

	err = dispatcher.DispatchCommand(command)
	if err != nil {
		t.Fatalf("Failed to dispatch command: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify command was sent to agent
	select {
	case msg := <-agentConn.Send:
		if len(msg) == 0 {
			t.Error("Expected message to have content")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for command to be sent to agent")
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}

	close(agentConn.Send)
}

// TestDispatchPendingCommands tests recovery of pending commands
func TestDispatchPendingCommands(t *testing.T) {
	dispatcher, _, mock, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Mock query for pending commands
	rows := sqlmock.NewRows([]string{
		"id", "command_id", "agent_id", "session_id", "action", "payload",
		"status", "error_message", "created_at", "sent_at", "acknowledged_at", "completed_at",
	}).
		AddRow(
			"uuid-1", "cmd-1", "test-agent", "sess-1", "start_session", nil,
			"pending", "", time.Now(), nil, nil, nil,
		).
		AddRow(
			"uuid-2", "cmd-2", "test-agent", "sess-2", "stop_session", nil,
			"pending", "", time.Now(), nil, nil, nil,
		)

	mock.ExpectQuery(`SELECT .+ FROM agent_commands WHERE status = 'pending'`).
		WillReturnRows(rows)

	err := dispatcher.DispatchPendingCommands()
	if err != nil {
		t.Fatalf("Failed to dispatch pending commands: %v", err)
	}

	// Verify both commands were queued
	if dispatcher.GetQueueLength() != 2 {
		t.Errorf("Expected 2 commands in queue, got %d", dispatcher.GetQueueLength())
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestDispatchPendingCommandsEmptyQueue tests handling no pending commands
func TestDispatchPendingCommandsEmptyQueue(t *testing.T) {
	dispatcher, _, mock, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Mock query for pending commands (empty result)
	rows := sqlmock.NewRows([]string{
		"id", "command_id", "agent_id", "session_id", "action", "payload",
		"status", "error_message", "created_at", "sent_at", "acknowledged_at", "completed_at",
	})

	mock.ExpectQuery(`SELECT .+ FROM agent_commands WHERE status = 'pending'`).
		WillReturnRows(rows)

	err := dispatcher.DispatchPendingCommands()
	if err != nil {
		t.Fatalf("Failed to dispatch pending commands: %v", err)
	}

	// Verify queue is empty
	if dispatcher.GetQueueLength() != 0 {
		t.Errorf("Expected 0 commands in queue, got %d", dispatcher.GetQueueLength())
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestStopDispatcher tests graceful shutdown
func TestStopDispatcher(t *testing.T) {
	dispatcher, _, _, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Start dispatcher
	go dispatcher.Start()

	// Wait for it to start
	time.Sleep(100 * time.Millisecond)

	// Stop dispatcher
	dispatcher.Stop()

	// Stop should cause Start() to exit (tested by not hanging)
	time.Sleep(100 * time.Millisecond)
}

// TestMultipleWorkers tests worker pool functionality
func TestMultipleWorkers(t *testing.T) {
	dispatcher, hub, mock, cleanup := setupDispatcherTest(t)
	defer cleanup()

	// Set multiple workers
	dispatcher.SetWorkers(5)

	// Register a mock agent
	mock.ExpectExec(`UPDATE agents SET status = 'online'`).
		WithArgs(sqlmock.AnyArg(), "test-agent").
		WillReturnResult(sqlmock.NewResult(1, 1))

	conn := &websocket.Conn{}
	agentConn := &internalWebsocket.AgentConnection{
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

	// Start dispatcher with 5 workers
	go dispatcher.Start()

	// Mock database updates for 5 commands
	for i := 0; i < 5; i++ {
		mock.ExpectExec(`UPDATE agent_commands SET status = 'sent'`).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	// Dispatch 5 commands
	for i := 0; i < 5; i++ {
		payload := models.CommandPayload{
			"sessionId": "sess-" + string(rune(i)),
		}

		command := &models.AgentCommand{
			CommandID: "cmd-" + string(rune(i)),
			AgentID:   "test-agent",
			Action:    "start_session",
			Payload:   &payload,
			Status:    "pending",
		}

		err = dispatcher.DispatchCommand(command)
		if err != nil {
			t.Fatalf("Failed to dispatch command: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify all commands were sent
	messageCount := 0
	for i := 0; i < 5; i++ {
		select {
		case <-agentConn.Send:
			messageCount++
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	if messageCount != 5 {
		t.Errorf("Expected 5 messages to be sent, got %d", messageCount)
	}

	close(agentConn.Send)
}
