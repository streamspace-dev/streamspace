// Package websocket provides WebSocket connection management for agents.
//
// This file implements the AgentHub, which is the central hub managing all
// agent WebSocket connections in the v2.0 multi-platform architecture.
//
// The AgentHub:
//   - Maintains a registry of all connected agents
//   - Routes messages between Control Plane and agents
//   - Monitors agent health via heartbeats
//   - Detects and cleans up stale connections
//   - Updates agent status in the database
//
// Connection Lifecycle:
//   1. Agent connects via WebSocket (/api/v1/agents/connect)
//   2. Hub registers the connection (updates DB status to "online")
//   3. Agent sends heartbeats every 10 seconds
//   4. Hub monitors LastPing timestamp
//   5. If no heartbeat for >30 seconds, connection is considered stale
//   6. On disconnect, hub unregisters connection (updates DB status to "offline")
//
// Thread Safety:
//   - All hub operations use channels for synchronization
//   - Connection map is protected by RWMutex
//   - Safe for concurrent use from multiple goroutines
package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
)

// AgentConnection represents a single agent's WebSocket connection.
//
// Each connected agent has one AgentConnection containing:
//   - Conn: The underlying WebSocket connection
//   - Send: Channel for outbound messages to agent
//   - Receive: Channel for inbound messages from agent
//   - LastPing: Timestamp of last heartbeat (for stale detection)
//
// Thread Safety: Mutex protects concurrent access to connection fields
type AgentConnection struct {
	// AgentID is the unique identifier for this agent
	AgentID string

	// Conn is the underlying WebSocket connection
	Conn *websocket.Conn

	// Platform identifies the agent type (kubernetes, docker, vm, cloud)
	Platform string

	// LastPing is the timestamp of the last heartbeat received
	LastPing time.Time

	// Send is a buffered channel for outbound messages to the agent
	Send chan []byte

	// Receive is a buffered channel for inbound messages from the agent
	Receive chan []byte

	// Mutex protects concurrent access to connection fields
	Mutex sync.RWMutex
}

// BroadcastMessage represents a message to be sent to multiple agents.
//
// Used by the hub's broadcast channel to send messages to all connected agents
// (e.g., shutdown notifications, global announcements).
type BroadcastMessage struct {
	// Message is the raw JSON bytes to send
	Message []byte

	// ExcludeAgentID optionally excludes a specific agent from the broadcast
	ExcludeAgentID string
}

// AgentHub is the central manager for all agent WebSocket connections.
//
// The hub runs a main event loop that processes:
//   - register: New agent connections
//   - unregister: Agent disconnections
//   - broadcast: Messages to all agents
//   - staleCheck: Periodic cleanup of stale connections
//
// Thread Safety: All operations use channels for synchronization.
type AgentHub struct {
	// connections maps agent_id -> AgentConnection
	connections map[string]*AgentConnection

	// mutex protects concurrent access to the connections map
	mutex sync.RWMutex

	// register channel receives new agent connections
	register chan *AgentConnection

	// unregister channel receives disconnecting agents
	unregister chan string

	// broadcast channel receives messages to send to all agents
	broadcast chan BroadcastMessage

	// database is used to persist agent status changes
	database *db.Database

	// stopChan is used to signal the hub to stop running
	stopChan chan struct{}
}

// NewAgentHub creates a new AgentHub instance.
//
// The hub is initialized with empty connection map and buffered channels.
// Call Run() to start the hub's event loop.
//
// Example:
//
//	hub := websocket.NewAgentHub(database)
//	go hub.Run()
func NewAgentHub(database *db.Database) *AgentHub {
	return &AgentHub{
		connections: make(map[string]*AgentConnection),
		register:    make(chan *AgentConnection, 10),
		unregister:  make(chan string, 10),
		broadcast:   make(chan BroadcastMessage, 100),
		database:    database,
		stopChan:    make(chan struct{}),
	}
}

// Run starts the hub's main event loop.
//
// This function blocks and should be run in a goroutine.
// It processes registration, unregistration, broadcasts, and stale connection checks.
//
// The loop runs until Stop() is called.
//
// Example:
//
//	hub := websocket.NewAgentHub(database)
//	go hub.Run()
func (h *AgentHub) Run() {
	log.Println("[AgentHub] Starting event loop")

	// Start periodic stale connection checker
	staleCheckTicker := time.NewTicker(10 * time.Second)
	defer staleCheckTicker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.handleRegister(conn)

		case agentID := <-h.unregister:
			h.handleUnregister(agentID)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)

		case <-staleCheckTicker.C:
			h.checkStaleConnections()

		case <-h.stopChan:
			log.Println("[AgentHub] Stopping event loop")
			return
		}
	}
}

// Stop signals the hub to stop running.
//
// This closes the stopChan, causing Run() to exit.
func (h *AgentHub) Stop() {
	close(h.stopChan)
}

// handleRegister processes a new agent connection.
//
// Updates the database to set agent status to "online" and stores the connection
// in the hub's connections map.
func (h *AgentHub) handleRegister(conn *AgentConnection) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// If agent is already connected, close the old connection
	if existing, ok := h.connections[conn.AgentID]; ok {
		log.Printf("[AgentHub] Agent %s already connected, closing old connection", conn.AgentID)
		close(existing.Send)
		existing.Conn.Close()
	}

	// Add new connection
	h.connections[conn.AgentID] = conn
	log.Printf("[AgentHub] Registered agent: %s (platform: %s), total connections: %d",
		conn.AgentID, conn.Platform, len(h.connections))

	// Update database status to "online"
	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agents
		SET status = 'online', last_heartbeat = $1, updated_at = $1
		WHERE agent_id = $2
	`, now, conn.AgentID)

	if err != nil {
		log.Printf("[AgentHub] Error updating agent status to online: %v", err)
	}
}

// handleUnregister processes an agent disconnection.
//
// Updates the database to set agent status to "offline" and removes the connection
// from the hub's connections map.
func (h *AgentHub) handleUnregister(agentID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	conn, ok := h.connections[agentID]
	if !ok {
		log.Printf("[AgentHub] Agent %s not found in connections (already unregistered?)", agentID)
		return
	}

	// Close channels and connection
	close(conn.Send)
	conn.Conn.Close()

	// Remove from connections map
	delete(h.connections, agentID)
	log.Printf("[AgentHub] Unregistered agent: %s, remaining connections: %d",
		agentID, len(h.connections))

	// Update database status to "offline"
	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agents
		SET status = 'offline', updated_at = $1
		WHERE agent_id = $2
	`, now, agentID)

	if err != nil {
		log.Printf("[AgentHub] Error updating agent status to offline: %v", err)
	}
}

// handleBroadcast sends a message to all connected agents.
//
// Optionally excludes a specific agent from the broadcast.
func (h *AgentHub) handleBroadcast(msg BroadcastMessage) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	count := 0
	for agentID, conn := range h.connections {
		if msg.ExcludeAgentID != "" && agentID == msg.ExcludeAgentID {
			continue
		}

		select {
		case conn.Send <- msg.Message:
			count++
		default:
			log.Printf("[AgentHub] Failed to send broadcast to agent %s (send buffer full)", agentID)
		}
	}

	log.Printf("[AgentHub] Broadcast message sent to %d agents", count)
}

// checkStaleConnections detects and closes connections with no heartbeat for >30 seconds.
//
// This runs periodically (every 10 seconds) to clean up stale connections.
func (h *AgentHub) checkStaleConnections() {
	h.mutex.RLock()
	staleAgents := make([]string, 0)
	now := time.Now()

	for agentID, conn := range h.connections {
		conn.Mutex.RLock()
		lastPing := conn.LastPing
		conn.Mutex.RUnlock()

		if now.Sub(lastPing) > 30*time.Second {
			staleAgents = append(staleAgents, agentID)
		}
	}
	h.mutex.RUnlock()

	// Unregister stale agents
	for _, agentID := range staleAgents {
		log.Printf("[AgentHub] Detected stale connection for agent %s (no heartbeat for >30s)", agentID)
		h.unregister <- agentID
	}
}

// RegisterAgent adds a new agent connection to the hub.
//
// This should be called when a new WebSocket connection is established.
// The connection will be processed by the hub's event loop.
//
// Example:
//
//	conn := &AgentConnection{
//	    AgentID: "k8s-prod-us-east-1",
//	    Conn: wsConn,
//	    Platform: "kubernetes",
//	    LastPing: time.Now(),
//	    Send: make(chan []byte, 256),
//	    Receive: make(chan []byte, 256),
//	}
//	err := hub.RegisterAgent(conn)
func (h *AgentHub) RegisterAgent(conn *AgentConnection) error {
	if conn.AgentID == "" {
		return fmt.Errorf("agent_id cannot be empty")
	}
	if conn.Conn == nil {
		return fmt.Errorf("websocket connection cannot be nil")
	}

	h.register <- conn
	return nil
}

// UnregisterAgent removes an agent connection from the hub.
//
// This should be called when a WebSocket connection is closed.
// The disconnection will be processed by the hub's event loop.
//
// Example:
//
//	hub.UnregisterAgent("k8s-prod-us-east-1")
func (h *AgentHub) UnregisterAgent(agentID string) {
	h.unregister <- agentID
}

// SendCommandToAgent sends a command to a specific agent over WebSocket.
//
// The command is wrapped in an AgentMessage with type="command" and sent
// to the agent's Send channel.
//
// Returns an error if the agent is not connected.
//
// Example:
//
//	command := &models.AgentCommand{
//	    CommandID: "cmd-123",
//	    Action: "start_session",
//	    Payload: map[string]interface{}{"sessionId": "sess-456"},
//	}
//	err := hub.SendCommandToAgent("k8s-prod-us-east-1", command)
func (h *AgentHub) SendCommandToAgent(agentID string, command *models.AgentCommand) error {
	h.mutex.RLock()
	conn, ok := h.connections[agentID]
	h.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("agent %s is not connected", agentID)
	}

	// Create command message
	commandMsg := models.CommandMessage{
		CommandID: command.CommandID,
		Action:    command.Action,
		Payload:   make(map[string]interface{}),
	}

	// Copy payload if present
	if command.Payload != nil {
		for k, v := range *command.Payload {
			commandMsg.Payload[k] = v
		}
	}

	// Wrap in AgentMessage
	payloadBytes, err := json.Marshal(commandMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal command payload: %w", err)
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeCommand,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	msgBytes, err := json.Marshal(agentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal agent message: %w", err)
	}

	// Send to agent's Send channel
	select {
	case conn.Send <- msgBytes:
		log.Printf("[AgentHub] Sent command %s to agent %s", command.CommandID, agentID)
		return nil
	default:
		return fmt.Errorf("agent %s send buffer is full", agentID)
	}
}

// BroadcastToAllAgents sends a message to all connected agents.
//
// Optionally excludes a specific agent from the broadcast.
//
// Example:
//
//	message := []byte(`{"type":"shutdown","payload":{}}`)
//	hub.BroadcastToAllAgents(message, "")
func (h *AgentHub) BroadcastToAllAgents(message []byte, excludeAgentID string) {
	h.broadcast <- BroadcastMessage{
		Message:        message,
		ExcludeAgentID: excludeAgentID,
	}
}

// GetConnectedAgents returns a list of all currently connected agent IDs.
//
// Example:
//
//	agents := hub.GetConnectedAgents()
//	fmt.Printf("Connected agents: %v\n", agents)
func (h *AgentHub) GetConnectedAgents() []string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	agents := make([]string, 0, len(h.connections))
	for agentID := range h.connections {
		agents = append(agents, agentID)
	}

	return agents
}

// IsAgentConnected checks if a specific agent is currently connected.
//
// Example:
//
//	if hub.IsAgentConnected("k8s-prod-us-east-1") {
//	    fmt.Println("Agent is online")
//	}
func (h *AgentHub) IsAgentConnected(agentID string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	_, ok := h.connections[agentID]
	return ok
}

// UpdateAgentHeartbeat updates the LastPing timestamp for an agent.
//
// This should be called when a heartbeat message is received from the agent.
//
// Example:
//
//	hub.UpdateAgentHeartbeat("k8s-prod-us-east-1")
func (h *AgentHub) UpdateAgentHeartbeat(agentID string) error {
	h.mutex.RLock()
	conn, ok := h.connections[agentID]
	h.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("agent %s is not connected", agentID)
	}

	conn.Mutex.Lock()
	conn.LastPing = time.Now()
	conn.Mutex.Unlock()

	// Also update database heartbeat timestamp and status
	// FIX P1-AGENT-STATUS-001: Update status to 'online' on every heartbeat
	// to ensure database state matches in-memory WebSocket connection state
	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agents
		SET status = 'online', last_heartbeat = $1, updated_at = $1
		WHERE agent_id = $2
	`, now, agentID)

	if err != nil {
		log.Printf("[AgentHub] Error updating agent heartbeat in database: %v", err)
		return err
	}

	return nil
}

// GetConnection returns the AgentConnection for a specific agent.
//
// Returns nil if the agent is not connected.
// Use IsAgentConnected to check before calling this.
//
// Thread Safety: The returned connection should not be modified directly.
//
// Example:
//
//	if conn := hub.GetConnection("k8s-prod-us-east-1"); conn != nil {
//	    fmt.Printf("Agent platform: %s\n", conn.Platform)
//	}
func (h *AgentHub) GetConnection(agentID string) *AgentConnection {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.connections[agentID]
}
