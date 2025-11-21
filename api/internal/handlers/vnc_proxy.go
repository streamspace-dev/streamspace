// Package handlers provides HTTP request handlers for the StreamSpace API.
//
// This file implements the VNC proxy handler for v2.0 multi-platform architecture.
//
// VNC Traffic Flow (v2.0):
//   UI Client → Control Plane VNC Proxy → Agent → Pod
//
// The VNC proxy:
//  1. Receives WebSocket connections from UI clients
//  2. Looks up which agent is hosting the session
//  3. Routes VNC traffic between UI and agent over WebSocket
//  4. Handles bidirectional VNC data relay
//
// Protocol:
//   - UI sends/receives raw VNC binary data (base64-encoded)
//   - Proxy wraps VNC data in agent protocol messages (vnc_data, vnc_close)
//   - Agent unwraps and relays to/from pod via port-forward
//
// Security:
//   - Requires valid JWT token
//   - Verifies user has access to the session
//   - Only one active VNC connection per session (prevents hijacking)
//
// Example:
//   UI connects to: ws://control-plane/api/v1/vnc/sess-123?token=<JWT>
//   Proxy routes to: agent "k8s-prod-us-east-1" via WebSocket
//   Agent tunnels to: pod "sess-123-abc" via port-forward
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
	ws "github.com/streamspace/streamspace/api/internal/websocket"
)

// VNCProxyHandler manages VNC WebSocket connections from UI clients.
//
// It proxies VNC traffic between UI clients and platform agents, enabling
// remote access to session desktops through the Control Plane.
type VNCProxyHandler struct {
	// db is the database connection
	db *db.Database

	// agentHub manages agent WebSocket connections
	agentHub *ws.AgentHub

	// upgrader upgrades HTTP connections to WebSocket
	upgrader websocket.Upgrader

	// activeConnections tracks active VNC connections (sessionID -> client conn)
	activeConnections map[string]*websocket.Conn
	connMutex         sync.RWMutex
}

// NewVNCProxyHandler creates a new VNC proxy handler.
//
// Example:
//
//	handler := NewVNCProxyHandler(database, agentHub)
//	router.GET("/vnc/:sessionId", handler.HandleVNCConnection)
func NewVNCProxyHandler(database *db.Database, agentHub *ws.AgentHub) *VNCProxyHandler {
	return &VNCProxyHandler{
		db:                database,
		agentHub:          agentHub,
		activeConnections: make(map[string]*websocket.Conn),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  32 * 1024, // 32KB for VNC data
			WriteBufferSize: 32 * 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper CORS validation
				return true
			},
		},
	}
}

// HandleVNCConnection handles VNC WebSocket connections from UI clients.
//
// Endpoint: GET /api/v1/vnc/:sessionId
//
// Query Parameters:
//   - token: JWT authentication token (required)
//
// Flow:
//  1. Authenticate user via JWT
//  2. Verify user has access to session
//  3. Look up agent hosting the session
//  4. Verify agent is connected
//  5. Upgrade HTTP to WebSocket
//  6. Proxy VNC traffic bidirectionally
//
// Example:
//
//	ws://control-plane/api/v1/vnc/sess-123?token=eyJhbGc...
func (h *VNCProxyHandler) HandleVNCConnection(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	// Get user from JWT (set by auth middleware)
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDInterface.(string)

	// Look up session in database
	var agentID string
	var sessionState string
	var sessionOwner string
	err := h.db.DB().QueryRow(`
		SELECT agent_id, state, user_id
		FROM sessions
		WHERE id = $1
	`, sessionID).Scan(&agentID, &sessionState, &sessionOwner)

	if err != nil {
		log.Printf("[VNCProxy] Session %s not found: %v", sessionID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Verify user has access to session
	if sessionOwner != userID {
		// TODO: Check if user is admin or has shared access
		log.Printf("[VNCProxy] User %s denied access to session %s (owner: %s)", userID, sessionID, sessionOwner)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify session is running
	if sessionState != "running" {
		log.Printf("[VNCProxy] Session %s is not running (state: %s)", sessionID, sessionState)
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Session is not running (state: %s)", sessionState),
		})
		return
	}

	// Verify agent_id is set
	if agentID == "" {
		log.Printf("[VNCProxy] Session %s has no agent assigned", sessionID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Session has no agent assigned"})
		return
	}

	// Verify agent is connected
	if !h.agentHub.IsAgentConnected(agentID) {
		log.Printf("[VNCProxy] Agent %s is not connected", agentID)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": fmt.Sprintf("Agent %s is not connected", agentID),
		})
		return
	}

	// Check for existing VNC connection
	h.connMutex.RLock()
	if existingConn, exists := h.activeConnections[sessionID]; exists {
		h.connMutex.RUnlock()
		// Close existing connection (only one VNC connection allowed per session)
		log.Printf("[VNCProxy] Closing existing VNC connection for session %s", sessionID)
		existingConn.Close()
		h.connMutex.Lock()
		delete(h.activeConnections, sessionID)
		h.connMutex.Unlock()
	} else {
		h.connMutex.RUnlock()
	}

	// Upgrade HTTP connection to WebSocket
	wsConn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[VNCProxy] Failed to upgrade connection: %v", err)
		return
	}

	// Register active connection
	h.connMutex.Lock()
	h.activeConnections[sessionID] = wsConn
	h.connMutex.Unlock()

	log.Printf("[VNCProxy] VNC connection established for session %s (agent: %s, user: %s)",
		sessionID, agentID, userID)

	// Update session active_connections count
	h.db.DB().Exec(`
		UPDATE sessions
		SET active_connections = active_connections + 1,
		    last_connection = $1
		WHERE id = $2
	`, time.Now(), sessionID)

	// Start bidirectional VNC data relay
	go h.relayVNCData(sessionID, agentID, wsConn)
}

// relayVNCData relays VNC data bidirectionally between UI and agent.
//
// Flow:
//   - UI → Proxy: Read VNC data from UI WebSocket
//   - Proxy → Agent: Send vnc_data message to agent
//   - Agent → Proxy: Receive vnc_data message from agent
//   - Proxy → UI: Write VNC data to UI WebSocket
func (h *VNCProxyHandler) relayVNCData(sessionID string, agentID string, uiConn *websocket.Conn) {
	defer func() {
		// Cleanup on disconnect
		uiConn.Close()

		h.connMutex.Lock()
		delete(h.activeConnections, sessionID)
		h.connMutex.Unlock()

		// Update session active_connections count
		h.db.DB().Exec(`
			UPDATE sessions
			SET active_connections = active_connections - 1,
			    last_disconnect = $1
			WHERE id = $2 AND active_connections > 0
		`, time.Now(), sessionID)

		// Send vnc_close to agent
		h.sendVNCCloseToAgent(agentID, sessionID, "client_disconnect")

		log.Printf("[VNCProxy] VNC connection closed for session %s", sessionID)
	}()

	// Get agent connection to receive vnc_data messages
	agentConn := h.agentHub.GetConnection(agentID)
	if agentConn == nil {
		log.Printf("[VNCProxy] Agent %s connection lost", agentID)
		return
	}

	// Channel to signal goroutine termination
	stopChan := make(chan struct{})
	defer close(stopChan)

	// Goroutine 1: UI → Agent (read from UI, send to agent)
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				// Read VNC data from UI
				messageType, data, err := uiConn.ReadMessage()
				if err != nil {
					log.Printf("[VNCProxy] Error reading from UI: %v", err)
					stopChan <- struct{}{}
					return
				}

				// Only process binary or text messages
				if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
					continue
				}

				// Send vnc_data to agent
				if err := h.sendVNCDataToAgent(agentID, sessionID, data); err != nil {
					log.Printf("[VNCProxy] Error sending to agent: %v", err)
					stopChan <- struct{}{}
					return
				}
			}
		}
	}()

	// Goroutine 2: Agent → UI (read from agent Receive channel, send to UI)
	for {
		select {
		case <-stopChan:
			return
		case msgBytes, ok := <-agentConn.Receive:
			if !ok {
				log.Printf("[VNCProxy] Agent %s receive channel closed", agentID)
				return
			}

			// Parse agent message
			var agentMsg models.AgentMessage
			if err := json.Unmarshal(msgBytes, &agentMsg); err != nil {
				log.Printf("[VNCProxy] Failed to parse agent message: %v", err)
				continue
			}

			// Only process vnc_data messages for this session
			if agentMsg.Type == models.MessageTypeVNCData {
				var vncData models.VNCDataMessage
				if err := json.Unmarshal(agentMsg.Payload, &vncData); err != nil {
					log.Printf("[VNCProxy] Failed to parse vnc_data: %v", err)
					continue
				}

				// Only relay if it's for this session
				if vncData.SessionID == sessionID {
					// Decode base64 VNC data
					// Actually, for WebSocket we can send base64 directly
					// The UI will decode it, or we send binary
					// For simplicity, send the base64 string as text
					if err := uiConn.WriteMessage(websocket.TextMessage, []byte(vncData.Data)); err != nil {
						log.Printf("[VNCProxy] Error writing to UI: %v", err)
						return
					}
				}
			} else if agentMsg.Type == models.MessageTypeVNCError {
				// VNC tunnel error from agent
				var vncError models.VNCErrorMessage
				if err := json.Unmarshal(agentMsg.Payload, &vncError); err == nil {
					if vncError.SessionID == sessionID {
						log.Printf("[VNCProxy] VNC error from agent: %s", vncError.Error)
						// Close UI connection
						return
					}
				}
			}
		}
	}
}

// sendVNCDataToAgent sends VNC data to the agent.
func (h *VNCProxyHandler) sendVNCDataToAgent(agentID, sessionID string, data []byte) error {
	// Base64-encode the data for JSON transport
	// Actually, if data is already base64 from UI, we can use it directly
	// For now, assume we receive raw binary and need to encode
	// base64Data := base64.StdEncoding.EncodeToString(data)

	// Create vnc_data message
	vncData := models.VNCDataMessage{
		SessionID: sessionID,
		Data:      string(data), // Assuming UI sends base64-encoded data
	}

	vncDataBytes, err := json.Marshal(vncData)
	if err != nil {
		return fmt.Errorf("failed to marshal vnc_data: %w", err)
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeVNCData,
		Timestamp: time.Now(),
		Payload:   vncDataBytes,
	}

	msgBytes, err := json.Marshal(agentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal agent message: %w", err)
	}

	// Send to agent via AgentHub
	agentConn := h.agentHub.GetConnection(agentID)
	if agentConn == nil {
		return fmt.Errorf("agent %s not connected", agentID)
	}

	select {
	case agentConn.Send <- msgBytes:
		return nil
	default:
		return fmt.Errorf("agent %s send buffer full", agentID)
	}
}

// sendVNCCloseToAgent sends a vnc_close message to the agent.
func (h *VNCProxyHandler) sendVNCCloseToAgent(agentID, sessionID, reason string) error {
	closeMsg := models.VNCCloseMessage{
		SessionID: sessionID,
		Reason:    reason,
	}

	closeMsgBytes, err := json.Marshal(closeMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal vnc_close: %w", err)
	}

	agentMsg := models.AgentMessage{
		Type:      models.MessageTypeVNCClose,
		Timestamp: time.Now(),
		Payload:   closeMsgBytes,
	}

	msgBytes, err := json.Marshal(agentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal agent message: %w", err)
	}

	// Send to agent via AgentHub
	agentConn := h.agentHub.GetConnection(agentID)
	if agentConn == nil {
		return fmt.Errorf("agent %s not connected", agentID)
	}

	select {
	case agentConn.Send <- msgBytes:
		log.Printf("[VNCProxy] Sent vnc_close to agent %s for session %s", agentID, sessionID)
		return nil
	default:
		return fmt.Errorf("agent %s send buffer full", agentID)
	}
}

// RegisterRoutes registers the VNC proxy routes.
//
// Routes:
//   - GET /vnc/:sessionId - VNC WebSocket connection
//
// Example:
//
//	vncProxyHandler.RegisterRoutes(router)
func (h *VNCProxyHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/vnc/:sessionId", h.HandleVNCConnection)
}

// GetActiveConnections returns the number of active VNC connections.
func (h *VNCProxyHandler) GetActiveConnections() int {
	h.connMutex.RLock()
	defer h.connMutex.RUnlock()
	return len(h.activeConnections)
}
