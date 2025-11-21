package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
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

// AgentRegistrationRequest is the request payload for agent registration.
type AgentRegistrationRequest struct {
	AgentID  string         `json:"agentId"`
	Platform string         `json:"platform"`
	Region   string         `json:"region,omitempty"`
	Capacity *AgentCapacity `json:"capacity,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AgentRegistrationResponse is the response from agent registration.
type AgentRegistrationResponse struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	Platform  string    `json:"platform"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Connect establishes connection to the Control Plane.
//
// Steps:
//  1. Register agent with Control Plane (POST /api/v1/agents/register)
//  2. Connect to WebSocket (/api/v1/agents/connect?agent_id=xxx)
//  3. Start read/write pumps
func (a *K8sAgent) Connect() error {
	log.Println("[K8sAgent] Connecting to Control Plane...")

	// Step 1: Register agent
	if err := a.registerAgent(); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Step 2: Connect WebSocket
	if err := a.connectWebSocket(); err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}

	log.Printf("[K8sAgent] Connected to Control Plane: %s", a.config.ControlPlaneURL)
	return nil
}

// registerAgent registers the agent with the Control Plane via HTTP API.
func (a *K8sAgent) registerAgent() error {
	// Prepare registration request
	reqBody := AgentRegistrationRequest{
		AgentID:  a.config.AgentID,
		Platform: a.config.Platform,
		Region:   a.config.Region,
		Capacity: &a.config.Capacity,
		Metadata: map[string]interface{}{
			"namespace":  a.config.Namespace,
			"kubernetes": true,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Convert WebSocket URL to HTTP URL
	httpURL := convertToHTTPURL(a.config.ControlPlaneURL)
	registerURL := fmt.Sprintf("%s/api/v1/agents/register", httpURL)

	// Send registration request
	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var regResp AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[K8sAgent] Registered successfully: %s (status: %s)", regResp.AgentID, regResp.Status)
	return nil
}

// connectWebSocket establishes the WebSocket connection to Control Plane.
func (a *K8sAgent) connectWebSocket() error {
	// Build WebSocket URL with agent_id query parameter
	wsURL := fmt.Sprintf("%s/api/v1/agents/connect?agent_id=%s",
		a.config.ControlPlaneURL,
		url.QueryEscape(a.config.AgentID))

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Set connection parameters
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	a.connMutex.Lock()
	a.wsConn = conn
	a.connMutex.Unlock()

	log.Println("[K8sAgent] WebSocket connected")
	return nil
}

// Reconnect attempts to reconnect with exponential backoff.
func (a *K8sAgent) Reconnect() error {
	log.Println("[K8sAgent] Connection lost, attempting to reconnect...")

	for attempt, backoff := range a.config.ReconnectBackoff {
		log.Printf("[K8sAgent] Reconnect attempt %d/%d (waiting %ds)",
			attempt+1, len(a.config.ReconnectBackoff), backoff)

		time.Sleep(time.Duration(backoff) * time.Second)

		if err := a.Connect(); err != nil {
			log.Printf("[K8sAgent] Reconnect attempt %d failed: %v", attempt+1, err)
			continue
		}

		log.Println("[K8sAgent] Reconnected successfully")
		return nil
	}

	return fmt.Errorf("reconnection failed after %d attempts", len(a.config.ReconnectBackoff))
}

// SendHeartbeats sends periodic heartbeats to the Control Plane.
//
// Heartbeats include:
//   - Agent status (online/draining)
//   - Active session count
//   - Current capacity usage
func (a *K8sAgent) SendHeartbeats() {
	interval := time.Duration(a.config.HeartbeatInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[K8sAgent] Starting heartbeat sender (interval: %s)", interval)

	for {
		select {
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("[K8sAgent] Failed to send heartbeat: %v", err)
			}

		case <-a.stopChan:
			log.Println("[K8sAgent] Heartbeat sender stopped")
			return
		}
	}
}

// sendHeartbeat sends a single heartbeat message.
func (a *K8sAgent) sendHeartbeat() error {
	// TODO: Calculate active sessions and capacity usage
	activeSessions := 0 // Placeholder

	heartbeat := map[string]interface{}{
		"type":      "heartbeat",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"status":         "online",
			"activeSessions": activeSessions,
			"capacity": map[string]interface{}{
				"maxCpu":      a.config.Capacity.MaxCPU,
				"maxMemory":   a.config.Capacity.MaxMemory,
				"maxSessions": a.config.Capacity.MaxSessions,
			},
		},
	}

	return a.sendMessage(heartbeat)
}

// sendMessage sends a JSON message over the WebSocket connection.
func (a *K8sAgent) sendMessage(message interface{}) error {
	a.connMutex.RLock()
	conn := a.wsConn
	a.connMutex.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// readPump reads messages from the WebSocket connection.
//
// This runs in a dedicated goroutine and continuously reads messages
// from the Control Plane, routing them to appropriate handlers.
func (a *K8sAgent) readPump() {
	defer func() {
		log.Println("[K8sAgent] Read pump stopped")
	}()

	for {
		select {
		case <-a.stopChan:
			return
		default:
			a.connMutex.RLock()
			conn := a.wsConn
			a.connMutex.RUnlock()

			if conn == nil {
				log.Println("[K8sAgent] Connection lost in read pump")
				a.Reconnect()
				continue
			}

			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[K8sAgent] Unexpected close: %v", err)
				}
				log.Println("[K8sAgent] Read error, attempting reconnect...")
				a.Reconnect()
				continue
			}

			// Parse and handle message
			if err := a.handleMessage(messageBytes); err != nil {
				log.Printf("[K8sAgent] Failed to handle message: %v", err)
			}
		}
	}
}

// writePump handles periodic ping messages to keep the connection alive.
//
// This runs in a dedicated goroutine.
func (a *K8sAgent) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		log.Println("[K8sAgent] Write pump stopped")
	}()

	for {
		select {
		case <-ticker.C:
			a.connMutex.RLock()
			conn := a.wsConn
			a.connMutex.RUnlock()

			if conn == nil {
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[K8sAgent] Ping error: %v", err)
				return
			}

		case <-a.stopChan:
			return
		}
	}
}

// convertToHTTPURL converts a WebSocket URL to HTTP URL.
//
// Examples:
//   wss://control.example.com -> https://control.example.com
//   ws://localhost:8000 -> http://localhost:8000
func convertToHTTPURL(wsURL string) string {
	if len(wsURL) > 3 && wsURL[:3] == "wss" {
		return "https" + wsURL[3:]
	}
	if len(wsURL) > 2 && wsURL[:2] == "ws" {
		return "http" + wsURL[2:]
	}
	return wsURL
}
