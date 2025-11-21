// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements the WebSocket handler for agent connections.
//
// AGENT WEBSOCKET CONNECTION:
// - Agents connect to GET /api/v1/agents/connect?agent_id=xxx
// - Connection is upgraded from HTTP to WebSocket
// - Agent sends heartbeats every 10 seconds
// - Control Plane sends commands to agent
// - Agent sends back ack/complete/failed/status messages
//
// MESSAGE FLOW:
// Control Plane → Agent:
//   - command: Execute a session command
//   - ping: Keep-alive ping
//   - shutdown: Graceful shutdown
//
// Agent → Control Plane:
//   - heartbeat: Regular status update
//   - ack: Command acknowledged
//   - complete: Command completed
//   - failed: Command failed
//   - status: Session status update
//
// CONNECTION LIFECYCLE:
// 1. Agent sends HTTP GET to /api/v1/agents/connect?agent_id=xxx
// 2. Handler validates agent exists in database
// 3. HTTP connection upgraded to WebSocket
// 4. Connection registered with AgentHub
// 5. readPump and writePump goroutines started
// 6. Agent sends heartbeats every 10 seconds
// 7. On disconnect, connection unregistered from hub
//
// Thread Safety:
// - readPump and writePump run concurrently
// - Each connection has dedicated Send/Receive channels
// - Hub handles all synchronization
package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
	wsocket "github.com/streamspace/streamspace/api/internal/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512 KB
)

// AgentWebSocketHandler handles WebSocket connections for agents.
//
// The handler is responsible for:
//   - Upgrading HTTP connections to WebSocket
//   - Validating agent authentication
//   - Managing connection lifecycle
//   - Starting read/write pumps
type AgentWebSocketHandler struct {
	// hub is the central agent connection manager
	hub *wsocket.AgentHub

	// upgrader handles HTTP to WebSocket upgrade
	upgrader websocket.Upgrader

	// database is used to validate agents
	database *db.Database
}

// NewAgentWebSocketHandler creates a new WebSocket handler for agents.
//
// Example:
//
//	handler := NewAgentWebSocketHandler(hub, database)
//	router.GET("/api/v1/agents/connect", handler.HandleAgentConnection)
func NewAgentWebSocketHandler(hub *wsocket.AgentHub, database *db.Database) *AgentWebSocketHandler {
	return &AgentWebSocketHandler{
		hub:      hub,
		database: database,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for agent connections
				// In production, this should validate agent certificates or tokens
				return true
			},
		},
	}
}

// RegisterRoutes registers WebSocket routes for agent connections.
//
// Example:
//
//	handler := NewAgentWebSocketHandler(hub, database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
func (h *AgentWebSocketHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/agents/connect", h.HandleAgentConnection)
}

// HandleAgentConnection handles the WebSocket upgrade and connection lifecycle.
//
// Query Parameters:
//   - agent_id (required): The unique identifier for the agent
//
// Flow:
//  1. Validate agent_id query parameter
//  2. Verify agent exists in database
//  3. Upgrade HTTP connection to WebSocket
//  4. Create AgentConnection with channels
//  5. Register connection with hub
//  6. Start readPump and writePump goroutines
//  7. Wait for connection to close
//
// Example Agent Connection:
//
//	ws, err := websocket.Dial("ws://localhost:8080/api/v1/agents/connect?agent_id=k8s-prod-us-east-1", "", "http://localhost/")
func (h *AgentWebSocketHandler) HandleAgentConnection(c *gin.Context) {
	// Get agent_id from query parameter
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing agent_id",
			"details": "agent_id query parameter is required",
		})
		return
	}

	// Verify agent exists in database
	var agent models.Agent
	err := h.database.DB().QueryRow(`
		SELECT id, agent_id, platform, region, status
		FROM agents
		WHERE agent_id = $1
	`, agentID).Scan(
		&agent.ID,
		&agent.AgentID,
		&agent.Platform,
		&agent.Region,
		&agent.Status,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Agent not found",
			"details": "Agent must register before connecting via WebSocket",
			"agentId": agentID,
		})
		return
	}

	if err != nil {
		log.Printf("[AgentWebSocket] Database error checking agent: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database error",
			"details": err.Error(),
		})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[AgentWebSocket] Failed to upgrade connection for agent %s: %v", agentID, err)
		return
	}

	log.Printf("[AgentWebSocket] Agent %s connected (platform: %s)", agentID, agent.Platform)

	// Create agent connection
	agentConn := &wsocket.AgentConnection{
		AgentID:  agentID,
		Conn:     conn,
		Platform: agent.Platform,
		LastPing: time.Now(),
		Send:     make(chan []byte, 256),
		Receive:  make(chan []byte, 256),
	}

	// Register with hub
	if err := h.hub.RegisterAgent(agentConn); err != nil {
		log.Printf("[AgentWebSocket] Failed to register agent %s: %v", agentID, err)
		conn.Close()
		return
	}

	// Start read and write pumps
	go h.writePump(agentConn)
	go h.readPump(agentConn)
}

// readPump reads messages from the WebSocket connection and processes them.
//
// This function runs in a dedicated goroutine for each agent connection.
// It continuously reads messages from the WebSocket and routes them based on type.
//
// Message Processing:
//   - heartbeat: Update agent heartbeat timestamp
//   - ack: Update command status to acknowledged
//   - complete: Update command status to completed
//   - failed: Update command status to failed
//   - status: Update session status in database
//
// The readPump exits when:
//   - WebSocket connection is closed
//   - Read error occurs
//   - Invalid message received
//
// On exit, the connection is unregistered from the hub.
func (h *AgentWebSocketHandler) readPump(conn *wsocket.AgentConnection) {
	defer func() {
		h.hub.UnregisterAgent(conn.AgentID)
		conn.Conn.Close()
	}()

	conn.Conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.Conn.SetReadLimit(maxMessageSize)
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[AgentWebSocket] Unexpected close for agent %s: %v", conn.AgentID, err)
			} else {
				log.Printf("[AgentWebSocket] Agent %s disconnected", conn.AgentID)
			}
			break
		}

		// Parse agent message
		var agentMsg models.AgentMessage
		if err := json.Unmarshal(messageBytes, &agentMsg); err != nil {
			log.Printf("[AgentWebSocket] Invalid message from agent %s: %v", conn.AgentID, err)
			continue
		}

		// Route message based on type
		switch agentMsg.Type {
		case models.MessageTypeHeartbeat:
			h.handleHeartbeat(conn, agentMsg)

		case models.MessageTypeAck:
			h.handleAck(conn, agentMsg)

		case models.MessageTypeComplete:
			h.handleComplete(conn, agentMsg)

		case models.MessageTypeFailed:
			h.handleFailed(conn, agentMsg)

		case models.MessageTypeStatus:
			h.handleStatus(conn, agentMsg)

		case models.MessageTypeVNCReady, models.MessageTypeVNCData, models.MessageTypeVNCError:
			// Forward VNC messages to Receive channel for VNC proxy
			select {
			case conn.Receive <- messageBytes:
				// Message forwarded to VNC proxy
			default:
				log.Printf("[AgentWebSocket] VNC receive buffer full for agent %s", conn.AgentID)
			}

		default:
			log.Printf("[AgentWebSocket] Unknown message type from agent %s: %s", conn.AgentID, agentMsg.Type)
		}
	}
}

// writePump writes messages from the Send channel to the WebSocket connection.
//
// This function runs in a dedicated goroutine for each agent connection.
// It continuously reads from the Send channel and writes to the WebSocket.
//
// The writePump also sends periodic ping messages to keep the connection alive.
//
// The writePump exits when:
//   - Send channel is closed
//   - Write error occurs
//
// On exit, the WebSocket connection is closed.
func (h *AgentWebSocketHandler) writePump(conn *wsocket.AgentConnection) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("[AgentWebSocket] Write error for agent %s: %v", conn.AgentID, err)
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(conn.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-conn.Send)
			}

			if err := w.Close(); err != nil {
				log.Printf("[AgentWebSocket] Close writer error for agent %s: %v", conn.AgentID, err)
				return
			}

		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[AgentWebSocket] Ping error for agent %s: %v", conn.AgentID, err)
				return
			}
		}
	}
}

// handleHeartbeat processes a heartbeat message from an agent.
//
// Updates the agent's LastPing timestamp in memory and last_heartbeat in database.
func (h *AgentWebSocketHandler) handleHeartbeat(conn *wsocket.AgentConnection, msg models.AgentMessage) {
	var heartbeat models.HeartbeatMessage
	if err := json.Unmarshal(msg.Payload, &heartbeat); err != nil {
		log.Printf("[AgentWebSocket] Invalid heartbeat from agent %s: %v", conn.AgentID, err)
		return
	}

	// Update hub heartbeat (also updates database)
	if err := h.hub.UpdateAgentHeartbeat(conn.AgentID); err != nil {
		log.Printf("[AgentWebSocket] Failed to update heartbeat for agent %s: %v", conn.AgentID, err)
	}

	// Optionally update capacity if provided
	if heartbeat.Capacity != nil {
		_, err := h.database.DB().Exec(`
			UPDATE agents
			SET capacity = $1, updated_at = $2
			WHERE agent_id = $3
		`, heartbeat.Capacity, time.Now(), conn.AgentID)
		if err != nil {
			log.Printf("[AgentWebSocket] Failed to update capacity for agent %s: %v", conn.AgentID, err)
		}
	}

	log.Printf("[AgentWebSocket] Heartbeat from agent %s (status: %s, activeSessions: %d)",
		conn.AgentID, heartbeat.Status, heartbeat.ActiveSessions)
}

// handleAck processes a command acknowledgment from an agent.
//
// Updates the command status to "ack" and sets acknowledged_at timestamp.
func (h *AgentWebSocketHandler) handleAck(conn *wsocket.AgentConnection, msg models.AgentMessage) {
	var ack models.AckMessage
	if err := json.Unmarshal(msg.Payload, &ack); err != nil {
		log.Printf("[AgentWebSocket] Invalid ack from agent %s: %v", conn.AgentID, err)
		return
	}

	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agent_commands
		SET status = 'ack', acknowledged_at = $1, updated_at = $1
		WHERE command_id = $2 AND agent_id = $3
	`, now, ack.CommandID, conn.AgentID)

	if err != nil {
		log.Printf("[AgentWebSocket] Failed to update command ack for %s: %v", ack.CommandID, err)
		return
	}

	log.Printf("[AgentWebSocket] Agent %s acknowledged command %s", conn.AgentID, ack.CommandID)
}

// handleComplete processes a command completion from an agent.
//
// Updates the command status to "completed" and sets completed_at timestamp.
func (h *AgentWebSocketHandler) handleComplete(conn *wsocket.AgentConnection, msg models.AgentMessage) {
	var complete models.CompleteMessage
	if err := json.Unmarshal(msg.Payload, &complete); err != nil {
		log.Printf("[AgentWebSocket] Invalid complete from agent %s: %v", conn.AgentID, err)
		return
	}

	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agent_commands
		SET status = 'completed', completed_at = $1, updated_at = $1
		WHERE command_id = $2 AND agent_id = $3
	`, now, complete.CommandID, conn.AgentID)

	if err != nil {
		log.Printf("[AgentWebSocket] Failed to update command completion for %s: %v", complete.CommandID, err)
		return
	}

	log.Printf("[AgentWebSocket] Agent %s completed command %s", conn.AgentID, complete.CommandID)
}

// handleFailed processes a command failure from an agent.
//
// Updates the command status to "failed" and stores the error message.
func (h *AgentWebSocketHandler) handleFailed(conn *wsocket.AgentConnection, msg models.AgentMessage) {
	var failed models.FailedMessage
	if err := json.Unmarshal(msg.Payload, &failed); err != nil {
		log.Printf("[AgentWebSocket] Invalid failed from agent %s: %v", conn.AgentID, err)
		return
	}

	now := time.Now()
	_, err := h.database.DB().Exec(`
		UPDATE agent_commands
		SET status = 'failed', error_message = $1, updated_at = $2
		WHERE command_id = $3 AND agent_id = $4
	`, failed.Error, now, failed.CommandID, conn.AgentID)

	if err != nil {
		log.Printf("[AgentWebSocket] Failed to update command failure for %s: %v", failed.CommandID, err)
		return
	}

	log.Printf("[AgentWebSocket] Agent %s failed command %s: %s", conn.AgentID, failed.CommandID, failed.Error)
}

// handleStatus processes a session status update from an agent.
//
// This is sent when a session changes state on the agent (e.g., running → terminated).
// Updates the session status in the database.
func (h *AgentWebSocketHandler) handleStatus(conn *wsocket.AgentConnection, msg models.AgentMessage) {
	var status models.StatusMessage
	if err := json.Unmarshal(msg.Payload, &status); err != nil {
		log.Printf("[AgentWebSocket] Invalid status from agent %s: %v", conn.AgentID, err)
		return
	}

	// Log the status update
	log.Printf("[AgentWebSocket] Agent %s status update for session %s: state=%s, vncReady=%v, vncPort=%d",
		conn.AgentID, status.SessionID, status.State, status.VNCReady, status.VNCPort)

	// TODO: Update session status in database when session table is added
	// For now, just log the status update
	// In Phase 4, this will update the sessions table with the new state
}
