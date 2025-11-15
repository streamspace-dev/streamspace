package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketMessage represents a real-time update message
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WebSocketClient represents a connected client
type WebSocketClient struct {
	ID       string
	UserID   string
	Conn     *websocket.Conn
	Send     chan WebSocketMessage
	Hub      *WebSocketHub
	Mu       sync.Mutex
}

// WebSocketHub manages all websocket connections
type WebSocketHub struct {
	Clients    map[string]*WebSocketClient
	Register   chan *WebSocketClient
	Unregister chan *WebSocketClient
	Broadcast  chan WebSocketMessage
	Mu         sync.RWMutex
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// Allow same-origin requests (no Origin header)
			if origin == "" {
				return true
			}

			// Get allowed origins from environment variables
			// Default to localhost for development
			allowedOrigins := []string{
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_1"),
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_2"),
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_3"),
				"http://localhost:5173", // Development default (Vite)
				"http://localhost:3000", // Development default (React)
			}

			// Check if origin is in allowed list
			for _, allowed := range allowedOrigins {
				if allowed != "" && strings.TrimSpace(allowed) == strings.TrimSpace(origin) {
					return true
				}
			}

			// Log rejected origin for security monitoring
			log.Printf("[WebSocket Security] Rejected connection from unauthorized origin: %s", origin)
			return false
		},
	}

	// Global hub instance
	hub *WebSocketHub
	once sync.Once
)

// GetWebSocketHub returns the singleton hub instance
func GetWebSocketHub() *WebSocketHub {
	once.Do(func() {
		hub = &WebSocketHub{
			Clients:    make(map[string]*WebSocketClient),
			Register:   make(chan *WebSocketClient),
			Unregister: make(chan *WebSocketClient),
			Broadcast:  make(chan WebSocketMessage, 256),
		}
		go hub.Run()
	})
	return hub
}

// Run starts the websocket hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Mu.Lock()
			h.Clients[client.ID] = client
			h.Mu.Unlock()
			log.Printf("WebSocket client registered: %s (user: %s)", client.ID, client.UserID)

		case client := <-h.Unregister:
			h.Mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				close(client.Send)
				delete(h.Clients, client.ID)
			}
			h.Mu.Unlock()
			log.Printf("WebSocket client unregistered: %s", client.ID)

		case message := <-h.Broadcast:
			// Collect clients to remove (don't modify map during iteration)
			clientsToRemove := make([]*WebSocketClient, 0)

			h.Mu.RLock()
			for _, client := range h.Clients {
				select {
				case client.Send <- message:
					// Message sent successfully
				default:
					// Client buffer full - mark for removal
					clientsToRemove = append(clientsToRemove, client)
				}
			}
			h.Mu.RUnlock()

			// Now safely remove disconnected clients with write lock
			if len(clientsToRemove) > 0 {
				h.Mu.Lock()
				for _, client := range clientsToRemove {
					if _, exists := h.Clients[client.ID]; exists {
						close(client.Send)
						delete(h.Clients, client.ID)
						log.Printf("WebSocket client removed (buffer full): %s", client.ID)
					}
				}
				h.Mu.Unlock()
			}
		}
	}
}

// BroadcastToUser sends a message to a specific user's connections
func (h *WebSocketHub) BroadcastToUser(userID string, message WebSocketMessage) {
	h.Mu.RLock()
	defer h.Mu.RUnlock()

	for _, client := range h.Clients {
		if client.UserID == userID {
			select {
			case client.Send <- message:
			default:
				log.Printf("Failed to send to client %s (buffer full)", client.ID)
			}
		}
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *WebSocketHub) BroadcastToAll(message WebSocketMessage) {
	h.Broadcast <- message
}

// HandleEnterpriseWebSocket handles WebSocket connections for enterprise features
func HandleEnterpriseWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}

	// Get user from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		conn.Close()
		return
	}

	// Create client
	client := &WebSocketClient{
		ID:     fmt.Sprintf("%s-%d", userID, time.Now().UnixNano()),
		UserID: userID.(string),
		Conn:   conn,
		Send:   make(chan WebSocketMessage, 256),
		Hub:    GetWebSocketHub(),
	}

	// Register client
	client.Hub.Register <- client

	// Start goroutines
	go client.writePump()
	go client.readPump()

	// Send welcome message
	client.Send <- WebSocketMessage{
		Type:      "connection",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"status":  "connected",
			"message": "Enterprise WebSocket connected",
		},
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				continue
			}

			w.Write(data)

			// Add queued messages to current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				msg := <-c.Send
				data, _ := json.Marshal(msg)
				w.Write(data)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *WebSocketClient) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// Client messages can be handled here if needed
	}
}

// Helper functions to broadcast enterprise events

// BroadcastWebhookDelivery broadcasts webhook delivery status
func BroadcastWebhookDelivery(userID string, webhookID int, deliveryID int, status string) {
	message := WebSocketMessage{
		Type:      "webhook.delivery",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"webhook_id":  webhookID,
			"delivery_id": deliveryID,
			"status":      status,
		},
	}
	GetWebSocketHub().BroadcastToUser(userID, message)
}

// BroadcastSecurityAlert broadcasts security alert
func BroadcastSecurityAlert(userID string, alertType string, severity string, message string) {
	msg := WebSocketMessage{
		Type:      "security.alert",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"alert_type": alertType,
			"severity":   severity,
			"message":    message,
		},
	}
	GetWebSocketHub().BroadcastToUser(userID, msg)
}

// BroadcastScheduledSessionEvent broadcasts scheduled session event
func BroadcastScheduledSessionEvent(userID string, scheduleID int, event string, sessionID string) {
	message := WebSocketMessage{
		Type:      "schedule.event",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"schedule_id": scheduleID,
			"event":       event, // "started", "completed", "failed"
			"session_id":  sessionID,
		},
	}
	GetWebSocketHub().BroadcastToUser(userID, message)
}

// BroadcastNodeHealthUpdate broadcasts node health status (admin only)
func BroadcastNodeHealthUpdate(nodeName string, status string, cpu float64, memory float64) {
	message := WebSocketMessage{
		Type:      "node.health",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"node_name":      nodeName,
			"health_status":  status,
			"cpu_percent":    cpu,
			"memory_percent": memory,
		},
	}
	// Broadcast to all admins
	GetWebSocketHub().BroadcastToAll(message)
}

// BroadcastScalingEvent broadcasts scaling event (admin only)
func BroadcastScalingEvent(policyID int, action string, result string) {
	message := WebSocketMessage{
		Type:      "scaling.event",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"policy_id": policyID,
			"action":    action, // "scale_up", "scale_down"
			"result":    result, // "success", "failed"
		},
	}
	GetWebSocketHub().BroadcastToAll(message)
}

// BroadcastComplianceViolation broadcasts compliance violation
func BroadcastComplianceViolation(userID string, violationID int, policyID int, severity string) {
	message := WebSocketMessage{
		Type:      "compliance.violation",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"violation_id": violationID,
			"policy_id":    policyID,
			"severity":     severity,
		},
	}
	if userID != "" {
		GetWebSocketHub().BroadcastToUser(userID, message)
	} else {
		// Broadcast to all admins
		GetWebSocketHub().BroadcastToAll(message)
	}
}
