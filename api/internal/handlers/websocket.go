package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/streamspace/streamspace/api/internal/db"
)

// WebSocketHandler handles WebSocket connections for real-time updates
type WebSocketHandler struct {
	db             *db.Database
	upgrader       websocket.Upgrader
	sessions       map[string]*WebSocketSession
	sessionsMutex  sync.RWMutex
	broadcast      chan *BroadcastMessage
	register       chan *WebSocketSession
	unregister     chan *WebSocketSession
}

// WebSocketSession represents a client WebSocket connection
type WebSocketSession struct {
	ID         string
	UserID     string
	Conn       *websocket.Conn
	Send       chan []byte
	handler    *WebSocketHandler
	Filters    *SubscriptionFilters
}

// SubscriptionFilters defines what updates a client wants to receive
type SubscriptionFilters struct {
	SessionIDs []string `json:"sessionIds"`
	UserID     string   `json:"userId"`
	TeamID     string   `json:"teamId"`
	EventTypes []string `json:"eventTypes"`
}

// BroadcastMessage represents a message to be broadcast
type BroadcastMessage struct {
	Type      string                 `json:"type"`
	Event     string                 `json:"event"`
	SessionID string                 `json:"sessionId,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	TeamID    string                 `json:"teamId,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(database *db.Database) *WebSocketHandler {
	h := &WebSocketHandler{
		db: database,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking
				return true
			},
		},
		sessions:   make(map[string]*WebSocketSession),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *WebSocketSession),
		unregister: make(chan *WebSocketSession),
	}

	// Start the hub
	go h.run()

	return h
}

// RegisterRoutes registers WebSocket routes
func (h *WebSocketHandler) RegisterRoutes(router *gin.RouterGroup) {
	ws := router.Group("/ws")
	{
		ws.GET("/sessions", h.SessionUpdates)
		ws.GET("/notifications", h.NotificationUpdates)
		ws.GET("/metrics", h.MetricsUpdates)
		ws.GET("/alerts", h.AlertUpdates)
	}
}

// run manages the WebSocket hub
func (h *WebSocketHandler) run() {
	for {
		select {
		case session := <-h.register:
			h.sessionsMutex.Lock()
			h.sessions[session.ID] = session
			h.sessionsMutex.Unlock()

		case session := <-h.unregister:
			h.sessionsMutex.Lock()
			if _, ok := h.sessions[session.ID]; ok {
				delete(h.sessions, session.ID)
				close(session.Send)
			}
			h.sessionsMutex.Unlock()

		case message := <-h.broadcast:
			h.sessionsMutex.RLock()
			for _, session := range h.sessions {
				if h.shouldReceiveMessage(session, message) {
					select {
					case session.Send <- h.serializeMessage(message):
					default:
						close(session.Send)
						delete(h.sessions, session.ID)
					}
				}
			}
			h.sessionsMutex.RUnlock()
		}
	}
}

// shouldReceiveMessage checks if a session should receive a message
func (h *WebSocketHandler) shouldReceiveMessage(session *WebSocketSession, message *BroadcastMessage) bool {
	if session.Filters == nil {
		return true
	}

	// Filter by event type
	if len(session.Filters.EventTypes) > 0 {
		found := false
		for _, eventType := range session.Filters.EventTypes {
			if eventType == message.Event {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by user ID
	if session.Filters.UserID != "" && message.UserID != session.Filters.UserID {
		return false
	}

	// Filter by session IDs
	if len(session.Filters.SessionIDs) > 0 {
		found := false
		for _, sessionID := range session.Filters.SessionIDs {
			if sessionID == message.SessionID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by team ID
	if session.Filters.TeamID != "" && message.TeamID != session.Filters.TeamID {
		return false
	}

	return true
}

// serializeMessage converts a broadcast message to JSON
func (h *WebSocketHandler) serializeMessage(message *BroadcastMessage) []byte {
	data, _ := json.Marshal(message)
	return data
}

// SessionUpdates handles WebSocket connections for session updates
func (h *WebSocketHandler) SessionUpdates(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// Create session
	session := &WebSocketSession{
		ID:      fmt.Sprintf("ws_%s_%d", userIDStr, time.Now().UnixNano()),
		UserID:  userIDStr,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		handler: h,
		Filters: &SubscriptionFilters{
			UserID: userIDStr,
		},
	}

	// Register session
	h.register <- session

	// Start goroutines
	go session.writePump()
	go session.readPump()
}

// NotificationUpdates handles WebSocket connections for notification updates
func (h *WebSocketHandler) NotificationUpdates(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	session := &WebSocketSession{
		ID:      fmt.Sprintf("ws_notif_%s_%d", userIDStr, time.Now().UnixNano()),
		UserID:  userIDStr,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		handler: h,
		Filters: &SubscriptionFilters{
			UserID:     userIDStr,
			EventTypes: []string{"notification.new", "notification.read", "notification.deleted"},
		},
	}

	h.register <- session
	go session.writePump()
	go session.readPump()
}

// MetricsUpdates handles WebSocket connections for metrics updates
func (h *WebSocketHandler) MetricsUpdates(c *gin.Context) {
	// Require operator/admin role
	role, exists := c.Get("role")
	if !exists || (role != "admin" && role != "operator") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	session := &WebSocketSession{
		ID:      fmt.Sprintf("ws_metrics_%s_%d", userIDStr, time.Now().UnixNano()),
		UserID:  userIDStr,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		handler: h,
		Filters: &SubscriptionFilters{
			EventTypes: []string{"metrics.sessions", "metrics.resources", "metrics.users"},
		},
	}

	h.register <- session
	go session.writePump()
	go session.readPump()

	// Start periodic metrics updates
	go h.sendPeriodicMetrics(session)
}

// AlertUpdates handles WebSocket connections for alert updates
func (h *WebSocketHandler) AlertUpdates(c *gin.Context) {
	// Require operator/admin role
	role, exists := c.Get("role")
	if !exists || (role != "admin" && role != "operator") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	session := &WebSocketSession{
		ID:      fmt.Sprintf("ws_alerts_%s_%d", userIDStr, time.Now().UnixNano()),
		UserID:  userIDStr,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		handler: h,
		Filters: &SubscriptionFilters{
			EventTypes: []string{"alert.triggered", "alert.acknowledged", "alert.resolved"},
		},
	}

	h.register <- session
	go session.writePump()
	go session.readPump()
}

// readPump reads messages from the WebSocket connection
func (s *WebSocketSession) readPump() {
	defer func() {
		s.handler.unregister <- s
		s.Conn.Close()
	}()

	s.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := s.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close
			}
			break
		}

		// Handle client messages (e.g., filter updates)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg["type"] == "subscribe" {
				s.handleSubscribe(msg)
			} else if msg["type"] == "unsubscribe" {
				s.handleUnsubscribe(msg)
			}
		}
	}
}

// writePump writes messages to the WebSocket connection
func (s *WebSocketSession) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		s.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-s.Send:
			s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				s.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := s.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(s.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-s.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleSubscribe updates subscription filters
func (s *WebSocketSession) handleSubscribe(msg map[string]interface{}) {
	if filters, ok := msg["filters"].(map[string]interface{}); ok {
		if sessionIDs, ok := filters["sessionIds"].([]interface{}); ok {
			s.Filters.SessionIDs = make([]string, len(sessionIDs))
			for i, id := range sessionIDs {
				s.Filters.SessionIDs[i] = id.(string)
			}
		}
		if eventTypes, ok := filters["eventTypes"].([]interface{}); ok {
			s.Filters.EventTypes = make([]string, len(eventTypes))
			for i, et := range eventTypes {
				s.Filters.EventTypes[i] = et.(string)
			}
		}
	}
}

// handleUnsubscribe removes subscription filters
func (s *WebSocketSession) handleUnsubscribe(msg map[string]interface{}) {
	if filters, ok := msg["filters"].(map[string]interface{}); ok {
		if _, ok := filters["sessionIds"]; ok {
			s.Filters.SessionIDs = nil
		}
		if _, ok := filters["eventTypes"]; ok {
			s.Filters.EventTypes = nil
		}
	}
}

// sendPeriodicMetrics sends metrics updates periodically
func (h *WebSocketHandler) sendPeriodicMetrics(session *WebSocketSession) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := context.Background()

	for {
		select {
		case <-ticker.C:
			// Get current metrics
			var totalSessions, runningSessions, hibernatedSessions int
			h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&totalSessions)
			h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'running'`).Scan(&runningSessions)
			h.db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE state = 'hibernated'`).Scan(&hibernatedSessions)

			message := &BroadcastMessage{
				Type:  "metrics",
				Event: "metrics.sessions",
				Data: map[string]interface{}{
					"total":      totalSessions,
					"running":    runningSessions,
					"hibernated": hibernatedSessions,
				},
				Timestamp: time.Now().UTC(),
			}

			data, _ := json.Marshal(message)
			select {
			case session.Send <- data:
			default:
				return
			}
		}
	}
}

// BroadcastSessionEvent broadcasts a session event to all connected clients
func (h *WebSocketHandler) BroadcastSessionEvent(event string, sessionID string, userID string, data map[string]interface{}) {
	message := &BroadcastMessage{
		Type:      "session",
		Event:     event,
		SessionID: sessionID,
		UserID:    userID,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	select {
	case h.broadcast <- message:
	default:
		// Broadcast channel full, skip
	}
}

// BroadcastNotificationEvent broadcasts a notification event
func (h *WebSocketHandler) BroadcastNotificationEvent(event string, userID string, data map[string]interface{}) {
	message := &BroadcastMessage{
		Type:      "notification",
		Event:     event,
		UserID:    userID,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	select {
	case h.broadcast <- message:
	default:
	}
}

// BroadcastAlertEvent broadcasts an alert event
func (h *WebSocketHandler) BroadcastAlertEvent(event string, data map[string]interface{}) {
	message := &BroadcastMessage{
		Type:      "alert",
		Event:     event,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	select {
	case h.broadcast <- message:
	default:
	}
}

// GetConnectedClients returns the number of connected WebSocket clients
func (h *WebSocketHandler) GetConnectedClients() int {
	h.sessionsMutex.RLock()
	defer h.sessionsMutex.RUnlock()
	return len(h.sessions)
}

// GetClientsByUser returns connected clients for a specific user
func (h *WebSocketHandler) GetClientsByUser(userID string) []*WebSocketSession {
	h.sessionsMutex.RLock()
	defer h.sessionsMutex.RUnlock()

	clients := []*WebSocketSession{}
	for _, session := range h.sessions {
		if session.UserID == userID {
			clients = append(clients, session)
		}
	}
	return clients
}

// DisconnectUser disconnects all WebSocket sessions for a user
func (h *WebSocketHandler) DisconnectUser(userID string) {
	h.sessionsMutex.Lock()
	defer h.sessionsMutex.Unlock()

	for id, session := range h.sessions {
		if session.UserID == userID {
			close(session.Send)
			delete(h.sessions, id)
		}
	}
}
