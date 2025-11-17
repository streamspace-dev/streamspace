// Package websocket provides real-time WebSocket communication for StreamSpace.
//
// The WebSocket system enables:
//   - Real-time session status updates to UI
//   - Session event notifications (created, updated, deleted, state changes)
//   - Connection tracking (connect, disconnect, heartbeat)
//   - Resource usage updates
//   - Sharing and collaboration notifications
//
// Architecture:
//   - Hub: Manages all WebSocket connections and broadcasts
//   - Client: Represents individual WebSocket connection
//   - Notifier: Handles event subscriptions and targeted notifications
//   - Manager: Coordinates hubs and notifiers
//
// Message flow:
//  1. Browser establishes WebSocket connection
//  2. Client registers with Hub
//  3. Client subscribes to user/session events via Notifier
//  4. Backend emits events (session created, state changed, etc.)
//  5. Notifier routes events to subscribed clients
//  6. Hub broadcasts messages to clients
//  7. Client writePump sends messages to browser
//
// Concurrency:
//   - Hub.Run() runs in goroutine, handles all channel operations
//   - Each Client has readPump and writePump goroutines
//   - Thread-safe with sync.RWMutex for shared state
//
// Example usage:
//
//	hub := NewHub()
//	go hub.Run()
//
//	// On WebSocket connection
//	hub.ServeClient(conn, clientID)
//
//	// Broadcast message to all clients
//	hub.Broadcast([]byte(`{"type":"session.created","sessionId":"abc"}`))
package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub maintains active WebSocket connections and implements message broadcasting.
//
// The Hub pattern:
//   - Centralizes connection management
//   - Provides thread-safe registration/unregistration
//   - Broadcasts messages to all clients efficiently
//   - Handles slow/disconnected clients gracefully
//
// Channel-based design:
//   - register: New clients connect
//   - unregister: Clients disconnect
//   - broadcast: Messages to send to all clients
//   - All operations go through channels to avoid race conditions
//
// Hub lifecycle:
//  1. Create with NewHub()
//  2. Start with go hub.Run()
//  3. Clients connect via ServeClient()
//  4. Send messages via Broadcast()
//  5. Clients disconnect automatically on connection close
//
// Thread safety:
//   - All client map access protected by sync.RWMutex
//   - Channel operations are inherently thread-safe
//   - Safe to call Broadcast() from multiple goroutines
type Hub struct {
	// clients is the set of registered clients.
	// Map key: *Client, Map value: bool (always true, used as a set)
	clients map[*Client]bool

	// broadcast is the channel for messages to send to all clients.
	// Buffer size: 256 messages
	broadcast chan []byte

	// register is the channel for new client registration requests.
	// Unbuffered channel (synchronous registration)
	register chan *Client

	// unregister is the channel for client disconnection requests.
	// Unbuffered channel (synchronous unregistration)
	unregister chan *Client

	// mu protects concurrent access to clients map.
	// Used when checking client count or iterating clients.
	mu sync.RWMutex
}

// Client represents an individual WebSocket connection.
//
// Each client has:
//   - Unique ID for identification
//   - WebSocket connection for bidirectional communication
//   - Buffered send channel for outbound messages
//   - Reference to hub for registration/unregistration
//
// Client lifecycle:
//  1. Created when browser establishes WebSocket
//  2. Registered with Hub
//  3. readPump goroutine reads messages from browser
//  4. writePump goroutine writes messages to browser
//  5. Unregistered when connection closes
//  6. Send channel closed to signal writePump to stop
//
// Message buffering:
//   - send channel has buffer of 256 messages
//   - If buffer fills, client is slow and gets disconnected
//   - Prevents slow clients from blocking the Hub
//
// Example:
//
//	client := &Client{
//	    hub:  hub,
//	    conn: websocketConn,
//	    send: make(chan []byte, 256),
//	    id:   "user1-session123",
//	}
type Client struct {
	// hub is the Hub this client belongs to.
	hub *Hub

	// conn is the underlying WebSocket connection.
	// gorilla/websocket.Conn
	conn *websocket.Conn

	// send is the buffered channel of outbound messages.
	// Buffer size: 256 messages
	// If buffer fills, client is considered slow and disconnected
	send chan []byte

	// id uniquely identifies this client.
	// Format: "{userID}-{sessionID}" or UUID
	id string
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client registered: %s (total: %d)", client.id, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("WebSocket client unregistered: %s (total: %d)", client.id, len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// BUG FIX: Collect clients to close first, then modify map with write lock
			// Using RLock while iterating, but need write lock to modify map
			h.mu.RLock()
			clientsToClose := make([]*Client, 0)
			for client := range h.clients {
				select {
				case client.send <- message:
					// Successfully sent
				default:
					// Client's send buffer is full, mark for closing
					clientsToClose = append(clientsToClose, client)
				}
			}
			h.mu.RUnlock()

			// Now close and remove blocked clients with write lock
			if len(clientsToClose) > 0 {
				h.mu.Lock()
				for _, client := range clientsToClose {
					close(client.send)
					delete(h.clients, client)
				}
				h.mu.Unlock()
			}
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second) // Send ping every 30 seconds
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			// Set write deadline to prevent hanging on slow connections
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Set read deadline and pong handler to keep connection alive
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Reset read deadline on any message
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// For now, we just log received messages
		// In the future, we could handle client->server messages
		log.Printf("Received message from client %s: %s", c.id, message)
	}
}

// ServeClient handles a new WebSocket connection
func (h *Hub) ServeClient(conn *websocket.Conn, clientID string) {
	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
		id:   clientID,
	}

	client.hub.register <- client

	// Start pumps in separate goroutines
	go client.writePump()
	go client.readPump()
}
