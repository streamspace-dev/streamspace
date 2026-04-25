// Package handlers provides HTTP and WebSocket handlers for the StreamSpace API.
// This file implements enterprise WebSocket functionality for real-time updates.
//
// Security Features:
// - Origin validation to prevent Cross-Site WebSocket Hijacking (CSWSH)
// - Race condition protection with proper mutex usage
// - User authentication required for all connections
// - Graceful disconnect handling
//
// Architecture:
// - Hub-and-spoke model: Central hub broadcasts to all clients
// - Each client has dedicated read/write goroutines
// - Buffered channels prevent blocking
// - Automatic cleanup of disconnected clients
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

// WebSocketMessage represents a real-time update message sent to clients.
//
// Type field determines the message category (e.g., "webhook.delivery", "security.alert").
// Timestamp is set server-side to ensure accurate event timing.
// Data contains the message payload as a flexible map.
//
// Example message:
//   {
//     "type": "security.alert",
//     "timestamp": "2025-11-15T10:30:00Z",
//     "data": {
//       "alert_type": "mfa_failed",
//       "severity": "high",
//       "message": "Multiple failed MFA attempts detected"
//     }
//   }
type WebSocketMessage struct {
	Type      string                 `json:"type"`      // Message type/category for client-side routing
	Timestamp time.Time              `json:"timestamp"` // Server timestamp for accurate event ordering
	Data      map[string]interface{} `json:"data"`      // Flexible payload containing event-specific data
}

// WebSocketClient represents a single connected WebSocket client.
//
// Each client has:
// - Unique ID for tracking (format: "userID-timestamp")
// - UserID for authorization and targeted messaging
// - WebSocket connection (Conn)
// - Buffered send channel to prevent blocking
// - Reference to hub for broadcasting
// - Mutex for thread-safe operations (currently unused but available for future state)
//
// The Send channel is buffered (256 messages) to handle burst traffic without blocking.
// If the buffer fills, the client is considered slow/disconnected and removed.
type WebSocketClient struct {
	ID     string              // Unique client identifier (format: "userID-unixnano")
	UserID string              // User ID for authorization and targeted broadcasts
	Conn   *websocket.Conn     // Underlying WebSocket connection
	Send   chan WebSocketMessage // Buffered channel for outbound messages (prevents blocking)
	Hub    *WebSocketHub       // Reference to hub for broadcasting
	Mu     sync.Mutex          // Mutex for thread-safe client state operations
}

// WebSocketHub is the central manager for all WebSocket connections.
//
// It uses a hub-and-spoke architecture:
// - Hub maintains all active clients in a map
// - Clients register/unregister via channels
// - Broadcast channel for sending to all clients
// - RWMutex for safe concurrent access to the clients map
//
// Thread Safety:
// - Register/Unregister: Processed sequentially in Run() with write lock
// - Broadcast: Uses read lock for iteration, write lock for cleanup
// - BroadcastToUser: Uses read lock only (no modifications)
//
// The hub runs in a single goroutine (via Run()) to avoid race conditions
// when modifying the clients map.
type WebSocketHub struct {
	Clients    map[string]*WebSocketClient // All connected clients (key: client ID)
	Register   chan *WebSocketClient       // Channel for new client registrations
	Unregister chan *WebSocketClient       // Channel for client disconnections
	Broadcast  chan WebSocketMessage       // Buffered channel for broadcast messages
	Mu         sync.RWMutex                // Read-write mutex for thread-safe map access
}

var (
	// upgrader configures the WebSocket connection upgrade with security settings.
	//
	// SECURITY: CheckOrigin prevents Cross-Site WebSocket Hijacking (CSWSH) attacks.
	//
	// CSWSH Attack Scenario:
	// 1. User logs into StreamSpace (gets session cookie)
	// 2. User visits malicious site evil.com
	// 3. evil.com JavaScript tries to connect to ws://streamspace.io
	// 4. Without origin validation, browser sends session cookie
	// 5. Attacker can now hijack WebSocket connection
	//
	// Protection:
	// - Validates Origin header against whitelist
	// - Environment variables for production origins
	// - Localhost defaults for development
	// - Logs rejected connections for security monitoring
	//
	// Configuration:
	//   export ALLOWED_WEBSOCKET_ORIGIN_1="https://streamspace.yourdomain.com"
	//   export ALLOWED_WEBSOCKET_ORIGIN_2="https://app.yourdomain.com"
	//   export ALLOWED_WEBSOCKET_ORIGIN_3="https://admin.yourdomain.com"
	upgrader = websocket.Upgrader{
		ReadBufferSize:  WebSocketReadBufferSize,  // 1024 bytes - buffer for incoming messages
		WriteBufferSize: WebSocketWriteBufferSize, // 1024 bytes - buffer for outgoing messages
		CheckOrigin: func(r *http.Request) bool {
			// Get the Origin header from the HTTP request
			// This header is automatically set by browsers and cannot be modified by JavaScript
			origin := r.Header.Get("Origin")

			// Allow same-origin requests (no Origin header)
			// This happens when the WebSocket connection is initiated from the same domain
			// Example: ws://localhost:8080 from page at http://localhost:8080
			if origin == "" {
				return true // Same-origin, safe to allow
			}

			// Get allowed origins from environment variables
			// In production, set these to your actual domains
			// Empty strings are ignored (allows partial configuration)
			allowedOrigins := []string{
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_1"), // Production domain 1
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_2"), // Production domain 2 (e.g., admin panel)
				os.Getenv("ALLOWED_WEBSOCKET_ORIGIN_3"), // Production domain 3 (e.g., mobile app)
				"http://localhost:5173",                 // Development default (Vite dev server)
				"http://localhost:3000",                 // Development default (Create React App)
			}

			// Check if the request's origin matches any allowed origin
			// TrimSpace handles whitespace in environment variables
			for _, allowed := range allowedOrigins {
				if allowed != "" && strings.TrimSpace(allowed) == strings.TrimSpace(origin) {
					return true // Origin is whitelisted, allow connection
				}
			}

			// Origin not in whitelist - reject connection and log for security monitoring
			// This log entry helps detect potential CSWSH attack attempts
			log.Printf("[WebSocket Security] Rejected connection from unauthorized origin: %s", origin)
			return false // Reject connection
		},
	}

	// Global hub instance - singleton pattern ensures all connections use the same hub
	hub *WebSocketHub

	// once ensures the hub is initialized exactly once (thread-safe singleton)
	// This prevents multiple goroutines from creating multiple hubs
	once sync.Once
)

// GetWebSocketHub returns the singleton hub instance using thread-safe lazy initialization.
//
// This function uses sync.Once to ensure the hub is created exactly once, even if called
// concurrently from multiple goroutines. This is the standard Go singleton pattern.
//
// The hub is initialized with:
// - Empty clients map
// - Unbuffered register/unregister channels (sequential processing)
// - Buffered broadcast channel (256 messages) to handle burst traffic
// - Background goroutine running hub.Run() for message processing
//
// Thread Safety: sync.Once guarantees Run() is called exactly once
//
// Returns:
//   - *WebSocketHub: The global hub instance
func GetWebSocketHub() *WebSocketHub {
	// once.Do executes the function exactly once, even with concurrent calls
	// Subsequent calls to GetWebSocketHub() will skip this and return existing hub
	once.Do(func() {
		// Initialize the hub with empty maps and channels
		hub = &WebSocketHub{
			Clients:    make(map[string]*WebSocketClient),          // Initially empty, clients added via Register channel
			Register:   make(chan *WebSocketClient),                // Unbuffered - blocks until Run() processes
			Unregister: make(chan *WebSocketClient),                // Unbuffered - blocks until Run() processes
			Broadcast:  make(chan WebSocketMessage, WebSocketBufferSize), // Buffered (256) - non-blocking sends
		}
		// Start the hub's main event loop in a background goroutine
		// This goroutine runs for the lifetime of the application
		go hub.Run()
	})
	return hub
}

// Run is the main event loop for the WebSocket hub.
//
// This function runs in a dedicated goroutine for the lifetime of the application.
// It processes three types of events via select statement:
//
// 1. Register: Add new client connections
// 2. Unregister: Remove disconnected clients
// 3. Broadcast: Send message to all connected clients
//
// CRITICAL RACE CONDITION FIX:
// The broadcast case uses a two-phase approach to prevent race conditions:
//   Phase 1: Read lock - Iterate clients and collect slow/disconnected ones
//   Phase 2: Write lock - Remove collected clients from map
//
// This prevents:
// - Concurrent map read/write errors
// - Deadlocks from holding write lock during iteration
// - Message loss from blocking sends
//
// Why not hold write lock during broadcast?
// - Broadcasting can be slow (network I/O)
// - Write lock blocks ALL reads (BroadcastToUser, Register, Unregister)
// - This would freeze the entire system during broadcasts
//
// Thread Safety:
// - All map modifications use write lock (h.Mu.Lock)
// - Map iteration uses read lock (h.Mu.RLock)
// - Locks are held for minimum time necessary
func (h *WebSocketHub) Run() {
	// Infinite loop - runs for application lifetime
	for {
		// Block until one of the channels has data
		select {
		// New client wants to connect
		case client := <-h.Register:
			// Acquire write lock to modify clients map
			h.Mu.Lock()
			h.Clients[client.ID] = client // Add client to map
			h.Mu.Unlock()
			log.Printf("WebSocket client registered: %s (user: %s)", client.ID, client.UserID)

		// Client disconnected (called from readPump when connection closes)
		case client := <-h.Unregister:
			// Acquire write lock to modify clients map
			h.Mu.Lock()
			// Check if client still exists (could have been removed elsewhere)
			if _, ok := h.Clients[client.ID]; ok {
				close(client.Send)          // Close send channel to stop writePump
				delete(h.Clients, client.ID) // Remove from map
			}
			h.Mu.Unlock()
			log.Printf("WebSocket client unregistered: %s", client.ID)

		// Broadcast message to all clients
		case message := <-h.Broadcast:
			// PHASE 1: Iterate with READ lock to find slow clients
			// We use read lock here because:
			// - Multiple goroutines can broadcast simultaneously
			// - We're only reading the map, not modifying it
			// - Sending to channels doesn't require write lock
			clientsToRemove := make([]*WebSocketClient, 0)

			h.Mu.RLock() // Acquire read lock - allows concurrent reads
			for _, client := range h.Clients {
				// Try to send message to client
				select {
				case client.Send <- message:
					// Message sent successfully to client's buffer
					// Client's writePump goroutine will send it over WebSocket
				default:
					// Client's send buffer is full (256 messages backlog)
					// This indicates:
					// - Client is too slow (network issues)
					// - Client has disconnected but cleanup hasn't finished
					// - Client is unresponsive
					// Mark for removal instead of blocking here
					clientsToRemove = append(clientsToRemove, client)
				}
			}
			h.Mu.RUnlock() // Release read lock

			// PHASE 2: Remove slow clients with WRITE lock
			// This two-phase approach prevents race conditions:
			// - We don't modify map while holding read lock (would panic)
			// - We don't hold write lock during iteration (would block everything)
			// - We double-check existence (client might have been removed in Phase 1)
			if len(clientsToRemove) > 0 {
				h.Mu.Lock() // Acquire write lock for map modification
				for _, client := range clientsToRemove {
					// Double-check client still exists (might have been removed by Unregister)
					if _, exists := h.Clients[client.ID]; exists {
						close(client.Send)                                      // Stop writePump goroutine
						delete(h.Clients, client.ID)                            // Remove from map
						log.Printf("WebSocket client removed (buffer full): %s", client.ID) // Log for monitoring
					}
				}
				h.Mu.Unlock() // Release write lock
			}
		}
	}
}

// BroadcastToUser sends a message to all connections belonging to a specific user.
//
// A single user can have multiple WebSocket connections open simultaneously
// (e.g., multiple browser tabs, mobile app + desktop app). This function sends
// the message to ALL of that user's connections.
//
// Use cases:
// - User-specific notifications (session started, MFA required, etc.)
// - Account security alerts (new login detected, password changed, etc.)
// - Personal updates (quota warnings, scheduled session reminders, etc.)
//
// Thread Safety:
// - Uses read lock only (no map modifications)
// - Non-blocking send via select/default
// - Safe to call concurrently from multiple goroutines
//
// Parameters:
//   - userID: The user ID to target (from authentication context)
//   - message: The WebSocketMessage to send
func (h *WebSocketHub) BroadcastToUser(userID string, message WebSocketMessage) {
	// Acquire read lock - allows concurrent reads
	h.Mu.RLock()
	defer h.Mu.RUnlock() // Release lock when function returns

	// Iterate all clients looking for matching userID
	for _, client := range h.Clients {
		if client.UserID == userID {
			// Try to send message without blocking
			select {
			case client.Send <- message:
				// Message sent successfully to client's buffer
			default:
				// Client's buffer is full - skip this client
				// The Run() goroutine will remove them during next broadcast
				log.Printf("Failed to send to client %s (buffer full)", client.ID)
			}
		}
	}
}

// BroadcastToAll sends a message to all connected clients (typically for admin-level events).
//
// This function sends the message to the hub's broadcast channel, where it's
// processed by the Run() goroutine and distributed to all clients.
//
// Use cases:
// - System-wide notifications (maintenance window, new features, etc.)
// - Admin-level events (node health changes, scaling events, etc.)
// - Platform status updates (high load warnings, service degradation, etc.)
//
// IMPORTANT: This sends to ALL users regardless of role. For admin-only messages,
// the client-side code should filter based on the user's role.
//
// Thread Safety:
// - Broadcast channel is buffered (256 messages)
// - Non-blocking as long as buffer isn't full
// - Safe to call concurrently from multiple goroutines
//
// Parameters:
//   - message: The WebSocketMessage to broadcast to all clients
func (h *WebSocketHub) BroadcastToAll(message WebSocketMessage) {
	// Send message to broadcast channel
	// The Run() goroutine will process it and send to all clients
	h.Broadcast <- message
}

// HandleEnterpriseWebSocket is the HTTP handler for WebSocket upgrade requests.
//
// This function:
// 1. Upgrades the HTTP connection to WebSocket protocol
// 2. Authenticates the user (via middleware context)
// 3. Creates a WebSocketClient instance
// 4. Registers the client with the hub
// 5. Starts read/write goroutines
// 6. Sends a welcome message
//
// SECURITY:
// - Origin validation is enforced by the upgrader's CheckOrigin function
// - User authentication is required (set by auth middleware before this handler)
// - Unauthenticated requests are rejected by closing the connection
//
// Flow:
//   HTTP Request (with Upgrade: websocket header)
//     ↓
//   CheckOrigin validation (prevents CSWSH attacks)
//     ↓
//   WebSocket upgrade (protocol switch)
//     ↓
//   Authentication check (requires auth middleware)
//     ↓
//   Client creation and registration
//     ↓
//   Start read/write goroutines (run until disconnect)
//     ↓
//   Send welcome message
//
// Parameters:
//   - c: Gin context containing the HTTP request and response
//
// Middleware Requirements:
//   - Authentication middleware must set "userID" in context
func HandleEnterpriseWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket protocol
	// The upgrader.CheckOrigin function validates the request's Origin header
	// Returns upgraded connection and error (if upgrade fails)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade failed - could be:
		// - Invalid Origin header (CSWSH protection)
		// - Missing Upgrade header
		// - Protocol negotiation failure
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}

	// Get user ID from Gin context (set by authentication middleware)
	// This ensures only authenticated users can connect
	userID, exists := c.Get("userID")
	if !exists {
		// User not authenticated - close connection immediately
		// This should never happen if auth middleware is configured correctly
		conn.Close()
		return
	}

	// Create a new WebSocket client instance
	// ID format: "userID-nanosecondTimestamp" (ensures uniqueness)
	client := &WebSocketClient{
		ID:     fmt.Sprintf("%s-%d", userID, time.Now().UnixNano()), // Unique ID: user123-1699999999999999999
		UserID: userID.(string),                                     // Type assertion safe because auth middleware sets this
		Conn:   conn,                                                // WebSocket connection
		Send:   make(chan WebSocketMessage, WebSocketBufferSize),    // Buffered channel (256 messages)
		Hub:    GetWebSocketHub(),                                   // Reference to global hub
	}

	// Register client with hub (thread-safe via channel)
	// This blocks until the hub's Run() goroutine processes it
	client.Hub.Register <- client

	// Start two goroutines for bidirectional communication:
	// - writePump: Reads from Send channel and writes to WebSocket
	// - readPump: Reads from WebSocket and handles client messages
	// Both run until connection closes, then clean up automatically
	go client.writePump()
	go client.readPump()

	// Send welcome message to client
	// This confirms successful connection and provides connection details
	client.Send <- WebSocketMessage{
		Type:      "connection",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"status":  "connected",
			"message": "Enterprise WebSocket connected",
		},
	}
}

// writePump is a goroutine that reads messages from the client's Send channel
// and writes them to the WebSocket connection.
//
// This function runs for the lifetime of the WebSocket connection and handles:
// 1. Sending messages from the Send channel to the client
// 2. Sending periodic ping messages to keep the connection alive
// 3. Batching multiple queued messages into a single WebSocket frame (optimization)
// 4. Graceful cleanup when the connection closes
//
// WebSocket Protocol:
// - Uses TextMessage frames (not Binary)
// - Sends ping frames every 54 seconds to prevent timeout
// - Sets write deadline to prevent hanging on slow clients
//
// Message Batching Optimization:
// When sending a message, if there are more messages waiting in the channel,
// they are batched together (newline-separated) into a single WebSocket frame.
// This reduces overhead when the system is broadcasting many messages quickly.
//
// Lifecycle:
// - Starts when client connects (via HandleEnterpriseWebSocket)
// - Runs until Send channel closes or WebSocket error occurs
// - Automatically closes WebSocket connection on exit (defer)
// - Stops ping ticker on exit to prevent goroutine leak
//
// Thread Safety:
// - Only this goroutine writes to the WebSocket (safe)
// - Multiple goroutines can send to Send channel (safe, buffered)
func (c *WebSocketClient) writePump() {
	// Create ticker for periodic ping messages (every 54 seconds)
	// Ping messages keep the connection alive and detect dead clients
	ticker := time.NewTicker(WebSocketPingInterval)

	// Cleanup when this goroutine exits
	defer func() {
		ticker.Stop()   // Stop ticker to prevent goroutine leak
		c.Conn.Close()  // Close WebSocket connection
	}()

	// Infinite loop - runs until connection closes or error occurs
	for {
		select {
		// Message received from Send channel
		case message, ok := <-c.Send:
			// Set write deadline to prevent hanging on slow clients
			// If write takes longer than 10 seconds, it fails
			_ = c.Conn.SetWriteDeadline(time.Now().Add(WebSocketWriteDeadline))

			// Check if channel was closed (ok == false)
			if !ok {
				// Hub closed our Send channel (client being removed)
				// Send close message to client and exit gracefully
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Get a writer for a text message frame
			// This starts building a WebSocket frame
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				// Connection error - client probably disconnected
				return
			}

			// Marshal message to JSON
			data, err := json.Marshal(message)
			if err != nil {
				// This shouldn't happen unless Data contains un-marshalable types
				log.Printf("Failed to marshal message: %v", err)
				continue // Skip this message, try next one
			}

			// Write the message to the frame
			_, _ = w.Write(data)

			// OPTIMIZATION: Batch queued messages into this WebSocket frame
			// If there are more messages waiting, send them together
			// This reduces WebSocket frame overhead during high traffic
			n := len(c.Send) // Check how many messages are waiting
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})     // Newline separator between messages
				msg := <-c.Send           // Get next message from channel
				data, _ := json.Marshal(msg) // Marshal to JSON (ignore error for batching)
				_, _ = w.Write(data)              // Add to current frame
			}

			// Close the writer to finish and send the WebSocket frame
			if err := w.Close(); err != nil {
				// Connection error during write - client probably disconnected
				return
			}

		// Ticker fired - time to send ping message
		case <-ticker.C:
			// Set write deadline for ping message
			_ = c.Conn.SetWriteDeadline(time.Now().Add(WebSocketWriteDeadline))

			// Send ping message
			// Client should respond with pong (handled in readPump)
			// If ping fails, client is disconnected - exit goroutine
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump is a goroutine that reads messages from the WebSocket connection.
//
// This function runs for the lifetime of the WebSocket connection and handles:
// 1. Reading messages from the client (currently not processed, reserved for future)
// 2. Responding to ping messages with pong (keep-alive mechanism)
// 3. Detecting client disconnections
// 4. Unregistering client from hub on disconnect
//
// WebSocket Protocol:
// - Sets read deadline (60 seconds) to detect dead connections
// - Pong handler resets read deadline when pong received
// - Distinguishes between expected closes (user navigated away) and errors
//
// Current Implementation:
// Currently, this is a "read and discard" pump. Client-to-server messages are
// received but not processed. This could be extended in the future to:
// - Allow clients to subscribe to specific event types
// - Let clients request specific data updates
// - Enable two-way communication for interactive features
//
// Lifecycle:
// - Starts when client connects (via HandleEnterpriseWebSocket)
// - Runs until WebSocket error or client disconnects
// - Automatically unregisters from hub on exit (defer)
// - Closes WebSocket connection on exit
//
// Thread Safety:
// - Only this goroutine reads from the WebSocket (safe)
// - Unregister via channel is thread-safe
func (c *WebSocketClient) readPump() {
	// Cleanup when this goroutine exits
	defer func() {
		c.Hub.Unregister <- c  // Tell hub to remove us (thread-safe via channel)
		c.Conn.Close()         // Close WebSocket connection
	}()

	// Set initial read deadline (60 seconds)
	// If no message received in 60 seconds, read will timeout
	// This is reset every time we receive a pong message
	_ = c.Conn.SetReadDeadline(time.Now().Add(WebSocketReadDeadline))

	// Set pong handler - called when client responds to our ping
	// This proves the client is still alive and resets the read deadline
	c.Conn.SetPongHandler(func(string) error {
		// Reset read deadline (client is alive)
		_ = c.Conn.SetReadDeadline(time.Now().Add(WebSocketReadDeadline))
		return nil // No error
	})

	// Infinite loop - read messages until connection closes
	for {
		// Read a message from the client
		// Currently we discard the message (_, _) because we're using
		// WebSocket primarily for server-to-client updates.
		// This could be extended to handle client messages if needed.
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			// Check if this is an unexpected error
			// Expected closes include:
			// - CloseGoingAway: User navigated to another page
			// - CloseAbnormalClosure: Network disruption (expected in mobile/WiFi)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Unexpected error - log for debugging
				log.Printf("WebSocket error: %v", err)
			}
			// Exit loop - connection is dead
			// defer will handle cleanup (unregister + close)
			break
		}

		// Client messages can be handled here if needed
		// Example future use cases:
		// - Parse JSON command from client
		// - Subscribe to specific event types
		// - Request data updates
		// - Send client-side metrics/telemetry
	}
}

// ============================================================================
// Helper Functions for Broadcasting Enterprise Events
// ============================================================================
//
// These are convenience functions that wrap BroadcastToUser and BroadcastToAll
// with predefined message types and data structures. They provide a consistent
// API for broadcasting different types of enterprise events.
//
// Usage Pattern:
//   // From any handler or service
//   handlers.BroadcastWebhookDelivery("user123", webhookID, deliveryID, "success")
//
// The client-side JavaScript will receive:
//   {
//     "type": "webhook.delivery",
//     "timestamp": "2025-11-15T10:30:00Z",
//     "data": {
//       "webhook_id": 456,
//       "delivery_id": 789,
//       "status": "success"
//     }
//   }

// BroadcastWebhookDelivery sends webhook delivery status updates to a user.
//
// This is called after a webhook HTTP request completes to notify the user
// of success or failure in real-time. The user can see webhook deliveries
// updating live in their dashboard without polling or refreshing.
//
// Parameters:
//   - userID: The user who owns the webhook
//   - webhookID: The webhook configuration ID
//   - deliveryID: The specific delivery attempt ID (for tracking retries)
//   - status: Delivery status ("success", "failed", "retrying")
//
// Example usage:
//   BroadcastWebhookDelivery("user123", 456, 789, "success")
func BroadcastWebhookDelivery(userID string, webhookID int, deliveryID int, status string) {
	message := WebSocketMessage{
		Type:      "webhook.delivery", // Message type for client-side routing
		Timestamp: time.Now(),         // Server timestamp
		Data: map[string]interface{}{
			"webhook_id":  webhookID,  // Which webhook configuration
			"delivery_id": deliveryID, // Specific delivery attempt (for retry tracking)
			"status":      status,     // "success", "failed", "retrying"
		},
	}
	// Send only to the webhook owner
	GetWebSocketHub().BroadcastToUser(userID, message)
}

// BroadcastSecurityAlert sends security alerts to a user in real-time.
//
// This provides immediate notification of security events such as:
// - Failed MFA attempts
// - Unusual login locations
// - API key usage from new IPs
// - Password change attempts
// - Session hijacking attempts
//
// The user sees these alerts instantly in a notification banner or modal.
//
// Parameters:
//   - userID: The user being alerted
//   - alertType: Type of alert ("mfa_failed", "new_login", "api_key_used", etc.)
//   - severity: Alert severity ("low", "medium", "high", "critical")
//   - message: Human-readable alert message
//
// Example usage:
//   BroadcastSecurityAlert("user123", "mfa_failed", "high",
//     "Multiple failed MFA attempts detected from IP 203.0.113.42")
func BroadcastSecurityAlert(userID string, alertType string, severity string, message string) {
	msg := WebSocketMessage{
		Type:      "security.alert", // Message type for client-side routing
		Timestamp: time.Now(),       // Server timestamp
		Data: map[string]interface{}{
			"alert_type": alertType, // Type of security event
			"severity":   severity,  // "low", "medium", "high", "critical"
			"message":    message,   // Human-readable description
		},
	}
	// Send only to the affected user
	GetWebSocketHub().BroadcastToUser(userID, msg)
}

// BroadcastScheduledSessionEvent sends updates about scheduled session execution.
//
// This notifies users when their scheduled sessions start, complete, or fail.
// Users can see session status updating live without refreshing the page.
//
// Parameters:
//   - userID: The user who scheduled the session
//   - scheduleID: The schedule configuration ID
//   - event: Event type ("started", "completed", "failed")
//   - sessionID: The Kubernetes session ID (for linking to logs/details)
//
// Example usage:
//   BroadcastScheduledSessionEvent("user123", 789, "started", "user123-firefox-abc")
func BroadcastScheduledSessionEvent(userID string, scheduleID int, event string, sessionID string) {
	message := WebSocketMessage{
		Type:      "schedule.event", // Message type for client-side routing
		Timestamp: time.Now(),       // Server timestamp
		Data: map[string]interface{}{
			"schedule_id": scheduleID, // Which schedule triggered this
			"event":       event,      // "started", "completed", "failed"
			"session_id":  sessionID,  // Kubernetes session ID
		},
	}
	// Send only to the user who owns the schedule
	GetWebSocketHub().BroadcastToUser(userID, message)
}

// BroadcastNodeHealthUpdate sends Kubernetes node health updates to admins.
//
// This provides real-time cluster monitoring in the admin dashboard. Admins
// can see node health, CPU, and memory usage updating live without refreshing.
//
// SECURITY: This is broadcast to ALL connected clients. The frontend should
// filter this message type to only display for admin users.
//
// Parameters:
//   - nodeName: Kubernetes node name (e.g., "worker-01")
//   - status: Health status ("healthy", "degraded", "unhealthy", "unknown")
//   - cpu: CPU usage as percentage (0.0 to 100.0)
//   - memory: Memory usage as percentage (0.0 to 100.0)
//
// Example usage:
//   BroadcastNodeHealthUpdate("worker-01", "healthy", 45.2, 67.8)
func BroadcastNodeHealthUpdate(nodeName string, status string, cpu float64, memory float64) {
	message := WebSocketMessage{
		Type:      "node.health", // Message type for client-side routing
		Timestamp: time.Now(),    // Server timestamp
		Data: map[string]interface{}{
			"node_name":      nodeName, // Kubernetes node name
			"health_status":  status,   // "healthy", "degraded", "unhealthy", "unknown"
			"cpu_percent":    cpu,      // CPU usage percentage
			"memory_percent": memory,   // Memory usage percentage
		},
	}
	// Broadcast to all connected clients (frontend filters for admins)
	GetWebSocketHub().BroadcastToAll(message)
}

// BroadcastScalingEvent sends auto-scaling events to admins.
//
// This notifies admins when the platform automatically scales up or down
// in response to resource usage or scaling policies. Admins see these events
// live in the admin dashboard.
//
// SECURITY: This is broadcast to ALL connected clients. The frontend should
// filter this message type to only display for admin users.
//
// Parameters:
//   - policyID: The scaling policy ID that triggered this event
//   - action: Scaling action ("scale_up", "scale_down")
//   - result: Action result ("success", "failed")
//
// Example usage:
//   BroadcastScalingEvent(123, "scale_up", "success")
func BroadcastScalingEvent(policyID int, action string, result string) {
	message := WebSocketMessage{
		Type:      "scaling.event", // Message type for client-side routing
		Timestamp: time.Now(),      // Server timestamp
		Data: map[string]interface{}{
			"policy_id": policyID, // Which scaling policy triggered this
			"action":    action,   // "scale_up", "scale_down"
			"result":    result,   // "success", "failed"
		},
	}
	// Broadcast to all connected clients (frontend filters for admins)
	GetWebSocketHub().BroadcastToAll(message)
}

// BroadcastComplianceViolation sends compliance violation alerts.
//
// This notifies users (or admins) of compliance policy violations such as:
// - Data retention policy violations
// - Unauthorized resource access attempts
// - Quota exceeded violations
// - Security policy violations
//
// If userID is provided, sends to that specific user. If userID is empty,
// broadcasts to all admins for system-wide violations.
//
// Parameters:
//   - userID: User who caused the violation (empty string for admin broadcast)
//   - violationID: Database ID of the violation record
//   - policyID: The compliance policy that was violated
//   - severity: Violation severity ("low", "medium", "high", "critical")
//
// Example usage:
//   // User-specific violation
//   BroadcastComplianceViolation("user123", 456, 789, "high")
//
//   // System-wide violation (admin broadcast)
//   BroadcastComplianceViolation("", 457, 790, "critical")
func BroadcastComplianceViolation(userID string, violationID int, policyID int, severity string) {
	message := WebSocketMessage{
		Type:      "compliance.violation", // Message type for client-side routing
		Timestamp: time.Now(),             // Server timestamp
		Data: map[string]interface{}{
			"violation_id": violationID, // Database violation record ID
			"policy_id":    policyID,    // Which policy was violated
			"severity":     severity,    // "low", "medium", "high", "critical"
		},
	}

	// Send to specific user or broadcast to all admins
	if userID != "" {
		// User-specific violation - send only to that user
		GetWebSocketHub().BroadcastToUser(userID, message)
	} else {
		// System-wide violation - broadcast to all admins
		// Frontend should filter this to only show to admin users
		GetWebSocketHub().BroadcastToAll(message)
	}
}
