package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// EventType represents the type of session event
type EventType string

const (
	// Session lifecycle events
	EventSessionCreated    EventType = "session.created"
	EventSessionUpdated    EventType = "session.updated"
	EventSessionDeleted    EventType = "session.deleted"
	EventSessionStateChange EventType = "session.state.changed"

	// Session activity events
	EventSessionConnected    EventType = "session.connected"
	EventSessionDisconnected EventType = "session.disconnected"
	EventSessionHeartbeat    EventType = "session.heartbeat"
	EventSessionIdle         EventType = "session.idle"
	EventSessionActive       EventType = "session.active"

	// Session resource events
	EventSessionResourcesUpdated EventType = "session.resources.updated"
	EventSessionTagsUpdated      EventType = "session.tags.updated"

	// Session sharing events
	EventSessionShared   EventType = "session.shared"
	EventSessionUnshared EventType = "session.unshared"

	// Session error events
	EventSessionError EventType = "session.error"
)

// SessionEvent represents a session-related event
type SessionEvent struct {
	Type      EventType              `json:"type"`
	SessionID string                 `json:"sessionId"`
	UserID    string                 `json:"userId"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Notifier handles real-time event notifications
type Notifier struct {
	manager *Manager
	mu      sync.RWMutex

	// User subscriptions: userID -> set of client IDs
	userSubscriptions map[string]map[string]bool

	// Session subscriptions: sessionID -> set of client IDs
	sessionSubscriptions map[string]map[string]bool

	// Client to user mapping: clientID -> userID
	clientUsers map[string]string
}

// NewNotifier creates a new event notifier
func NewNotifier(manager *Manager) *Notifier {
	return &Notifier{
		manager:              manager,
		userSubscriptions:    make(map[string]map[string]bool),
		sessionSubscriptions: make(map[string]map[string]bool),
		clientUsers:          make(map[string]string),
	}
}

// SubscribeUser subscribes a client to receive events for a specific user
func (n *Notifier) SubscribeUser(clientID, userID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Add to user subscriptions
	if _, exists := n.userSubscriptions[userID]; !exists {
		n.userSubscriptions[userID] = make(map[string]bool)
	}
	n.userSubscriptions[userID][clientID] = true

	// Track client to user mapping
	n.clientUsers[clientID] = userID

	log.Printf("Client %s subscribed to user %s events", clientID, userID)
}

// SubscribeSession subscribes a client to receive events for a specific session
func (n *Notifier) SubscribeSession(clientID, sessionID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.sessionSubscriptions[sessionID]; !exists {
		n.sessionSubscriptions[sessionID] = make(map[string]bool)
	}
	n.sessionSubscriptions[sessionID][clientID] = true

	log.Printf("Client %s subscribed to session %s events", clientID, sessionID)
}

// UnsubscribeClient removes all subscriptions for a client
func (n *Notifier) UnsubscribeClient(clientID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Remove from user subscriptions
	if userID, exists := n.clientUsers[clientID]; exists {
		if clients, exists := n.userSubscriptions[userID]; exists {
			delete(clients, clientID)
			if len(clients) == 0 {
				delete(n.userSubscriptions, userID)
			}
		}
		delete(n.clientUsers, clientID)
	}

	// Remove from session subscriptions
	for sessionID, clients := range n.sessionSubscriptions {
		if clients[clientID] {
			delete(clients, clientID)
			if len(clients) == 0 {
				delete(n.sessionSubscriptions, sessionID)
			}
		}
	}

	log.Printf("Client %s unsubscribed from all events", clientID)
}

// NotifySessionEvent sends a session event to subscribed clients
func (n *Notifier) NotifySessionEvent(event SessionEvent) {
	n.mu.RLock()
	targetClients := make(map[string]bool)

	// Get clients subscribed to this user
	if event.UserID != "" {
		if clients, exists := n.userSubscriptions[event.UserID]; exists {
			for clientID := range clients {
				targetClients[clientID] = true
			}
		}
	}

	// Get clients subscribed to this session
	if clients, exists := n.sessionSubscriptions[event.SessionID]; exists {
		for clientID := range clients {
			targetClients[clientID] = true
		}
	}
	n.mu.RUnlock()

	// No subscribers, skip
	if len(targetClients) == 0 {
		return
	}

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal session event: %v", err)
		return
	}

	// Send to target clients
	n.manager.sessionsHub.mu.RLock()
	sentCount := 0
	for client := range n.manager.sessionsHub.clients {
		if targetClients[client.id] {
			select {
			case client.send <- data:
				sentCount++
			default:
				log.Printf("Failed to send event to client %s (buffer full)", client.id)
			}
		}
	}
	n.manager.sessionsHub.mu.RUnlock()

	log.Printf("Event %s for session %s sent to %d clients", event.Type, event.SessionID, sentCount)
}

// NotifySessionCreated notifies clients when a session is created
func (n *Notifier) NotifySessionCreated(sessionID, userID string, data map[string]interface{}) {
	event := SessionEvent{
		Type:      EventSessionCreated,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      data,
	}
	n.NotifySessionEvent(event)
}

// NotifySessionUpdated notifies clients when a session is updated
func (n *Notifier) NotifySessionUpdated(sessionID, userID string, data map[string]interface{}) {
	event := SessionEvent{
		Type:      EventSessionUpdated,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      data,
	}
	n.NotifySessionEvent(event)
}

// NotifySessionDeleted notifies clients when a session is deleted
func (n *Notifier) NotifySessionDeleted(sessionID, userID string) {
	event := SessionEvent{
		Type:      EventSessionDeleted,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	n.NotifySessionEvent(event)
}

// NotifySessionStateChange notifies clients when a session changes state
func (n *Notifier) NotifySessionStateChange(sessionID, userID, oldState, newState string) {
	event := SessionEvent{
		Type:      EventSessionStateChange,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"oldState": oldState,
			"newState": newState,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionConnected notifies clients when someone connects to a session
func (n *Notifier) NotifySessionConnected(sessionID, userID string, connectionID string) {
	event := SessionEvent{
		Type:      EventSessionConnected,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"connectionId": connectionID,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionDisconnected notifies clients when someone disconnects from a session
func (n *Notifier) NotifySessionDisconnected(sessionID, userID string, connectionID string) {
	event := SessionEvent{
		Type:      EventSessionDisconnected,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"connectionId": connectionID,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionIdle notifies clients when a session becomes idle
func (n *Notifier) NotifySessionIdle(sessionID, userID string, idleDuration int64) {
	event := SessionEvent{
		Type:      EventSessionIdle,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"idleDuration": idleDuration,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionActive notifies clients when a session becomes active again
func (n *Notifier) NotifySessionActive(sessionID, userID string) {
	event := SessionEvent{
		Type:      EventSessionActive,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	n.NotifySessionEvent(event)
}

// NotifySessionResourcesUpdated notifies clients when session resources are updated
func (n *Notifier) NotifySessionResourcesUpdated(sessionID, userID string, resources map[string]interface{}) {
	event := SessionEvent{
		Type:      EventSessionResourcesUpdated,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"resources": resources,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionTagsUpdated notifies clients when session tags are updated
func (n *Notifier) NotifySessionTagsUpdated(sessionID, userID string, tags []string) {
	event := SessionEvent{
		Type:      EventSessionTagsUpdated,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tags": tags,
		},
	}
	n.NotifySessionEvent(event)
}

// NotifySessionShared notifies clients when a session is shared with someone
func (n *Notifier) NotifySessionShared(sessionID, ownerUserID, sharedWithUserID string, permissions []string) {
	// Notify owner
	event := SessionEvent{
		Type:      EventSessionShared,
		SessionID: sessionID,
		UserID:    ownerUserID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"sharedWith":  sharedWithUserID,
			"permissions": permissions,
		},
	}
	n.NotifySessionEvent(event)

	// Notify the user it was shared with
	eventForSharedUser := SessionEvent{
		Type:      EventSessionShared,
		SessionID: sessionID,
		UserID:    sharedWithUserID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"sharedBy":    ownerUserID,
			"permissions": permissions,
		},
	}
	n.NotifySessionEvent(eventForSharedUser)
}

// NotifySessionError notifies clients about session errors
func (n *Notifier) NotifySessionError(sessionID, userID string, errorMsg string) {
	event := SessionEvent{
		Type:      EventSessionError,
		SessionID: sessionID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"error": errorMsg,
		},
	}
	n.NotifySessionEvent(event)
}

// CloseAll closes all subscriptions (used during shutdown)
func (n *Notifier) CloseAll() {
	n.mu.Lock()
	defer n.mu.Unlock()

	log.Println("Closing all WebSocket subscriptions...")

	// Clear all subscriptions
	n.userSubscriptions = make(map[string]map[string]bool)
	n.sessionSubscriptions = make(map[string]map[string]bool)
	n.clientUsers = make(map[string]string)

	log.Println("All subscriptions closed")
}
