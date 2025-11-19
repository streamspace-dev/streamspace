// Package tracker manages active WebSocket connections and auto-hibernation for StreamSpace sessions.
//
// The tracker provides:
//   - Real-time connection tracking (WebSocket, VNC, HTTP)
//   - Heartbeat monitoring with configurable timeout
//   - Auto-start of hibernated sessions when connections arrive
//   - Auto-hibernate idle sessions with zero active connections
//   - Connection statistics and monitoring
//
// Architecture:
//   - In-memory connection map for fast lookups
//   - PostgreSQL persistence for connection history
//   - Kubernetes integration for session state management
//   - Background goroutine for periodic connection checks
//
// Lifecycle:
//  1. User connects to session → AddConnection()
//  2. Periodic heartbeats → UpdateHeartbeat()
//  3. Connection lost → RemoveConnection()
//  4. No connections + idle timeout → Auto-hibernate
//
// Configuration:
//   - checkInterval: 30 seconds (how often to check connections)
//   - heartbeatWindow: 60 seconds (max time without heartbeat)
//
// Example usage:
//
//	tracker := NewConnectionTracker(database, k8sClient)
//	go tracker.Start()
//
//	// When user connects via WebSocket
//	conn := &Connection{
//	    ID: uuid.New().String(),
//	    SessionID: sessionID,
//	    UserID: userID,
//	    ConnectedAt: time.Now(),
//	    LastHeartbeat: time.Now(),
//	}
//	tracker.AddConnection(ctx, conn)
package tracker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/events"
	"github.com/streamspace/streamspace/api/internal/k8s"
)

// ConnectionTracker manages active connections and implements auto-hibernation.
//
// Thread safety:
//   - Uses sync.RWMutex for concurrent access
//   - Safe for multiple goroutines to call AddConnection/RemoveConnection
//   - Background checker runs in separate goroutine
//
// Hibernation logic:
//   - Session is hibernated when activeConnections == 0 AND idle timeout elapsed
//   - Session is auto-started when new connection arrives while hibernated
//   - Heartbeat window prevents premature disconnection (grace period)
//
// Database persistence:
//   - All connections are persisted to PostgreSQL
//   - Connection history enables usage analytics
//   - On startup, loads active connections from last 5 minutes
type ConnectionTracker struct {
	// db is the PostgreSQL database for connection persistence.
	db *db.Database

	// k8sClient interacts with Kubernetes to manage session state.
	k8sClient *k8s.Client

	// publisher publishes NATS events for platform-agnostic operations.
	publisher *events.Publisher

	// platform identifies the target platform (kubernetes, docker, etc.)
	platform string

	// connections is the in-memory map of active connections.
	// Key: connection ID, Value: Connection struct
	// Protected by mu for thread safety.
	connections map[string]*Connection

	// mu protects concurrent access to connections map.
	mu sync.RWMutex

	// checkInterval is how often to check connection health.
	// Default: 30 seconds
	checkInterval time.Duration

	// heartbeatWindow is the maximum time without heartbeat before disconnect.
	// Default: 60 seconds
	heartbeatWindow time.Duration

	// stopCh signals the background checker to stop.
	stopCh chan struct{}
}

// Connection represents an active user connection to a session.
//
// Connections are created when:
//   - User opens VNC viewer
//   - User opens session in browser
//   - API client establishes WebSocket
//
// Connections are tracked for:
//   - Auto-hibernation (hibernate when count reaches zero)
//   - Usage analytics (connection duration, IP addresses)
//   - Concurrent user limits
//   - Session sharing detection
//
// Example:
//
//	conn := &Connection{
//	    ID: "conn-abc123",
//	    SessionID: "user1-firefox",
//	    UserID: "user1",
//	    ClientIP: "192.168.1.100",
//	    UserAgent: "Mozilla/5.0...",
//	    ConnectedAt: time.Now(),
//	    LastHeartbeat: time.Now(),
//	}
type Connection struct {
	// ID is a unique identifier for this connection.
	// Format: UUID or similar unique string
	ID string

	// SessionID is the session this connection is for.
	// Must match a valid session ID in database
	SessionID string

	// UserID is the authenticated user who owns this connection.
	UserID string

	// ClientIP is the IP address of the client.
	// Used for security auditing and geo-location
	ClientIP string

	// UserAgent is the browser/client user agent string.
	// Used for analytics and compatibility tracking
	UserAgent string

	// ConnectedAt is when this connection was established.
	ConnectedAt time.Time

	// LastHeartbeat is the last time this connection sent a heartbeat.
	// Connections without recent heartbeats are considered stale.
	LastHeartbeat time.Time
}

// NewConnectionTracker creates a new connection tracker instance.
//
// Default configuration:
//   - checkInterval: 30 seconds (connection health checks)
//   - heartbeatWindow: 60 seconds (heartbeat timeout)
//
// The tracker must be started with Start() to begin monitoring.
//
// Example:
//
//	tracker := NewConnectionTracker(database, k8sClient, publisher, "kubernetes")
//	go tracker.Start()  // Run in background
func NewConnectionTracker(database *db.Database, k8sClient *k8s.Client, publisher *events.Publisher, platform string) *ConnectionTracker {
	if platform == "" {
		platform = events.PlatformKubernetes
	}
	return &ConnectionTracker{
		db:              database,
		k8sClient:       k8sClient,
		publisher:       publisher,
		platform:        platform,
		connections:     make(map[string]*Connection),
		checkInterval:   30 * time.Second,  // Check every 30 seconds
		heartbeatWindow: 60 * time.Second,  // Disconnect if no heartbeat for 60s
		stopCh:          make(chan struct{}),
	}
}

// Start begins the connection tracking loop.
//
// This method:
//  1. Loads active connections from database (last 5 minutes)
//  2. Starts periodic checker (runs every checkInterval)
//  3. Checks connection health and performs auto-hibernation
//  4. Runs until Stop() is called
//
// This is a blocking call and should be run in a goroutine:
//
//	go tracker.Start()
//
// The checker performs these operations on each tick:
//   - Count active connections per session
//   - Remove stale connections (no heartbeat)
//   - Update database connection counts
//   - Auto-hibernate sessions with zero connections
func (ct *ConnectionTracker) Start() {
	log.Println("Connection tracker started")

	// Initial load of active connections from database
	if err := ct.loadActiveConnections(); err != nil {
		log.Printf("Failed to load active connections: %v", err)
	}

	ticker := time.NewTicker(ct.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ct.checkConnections()
		case <-ct.stopCh:
			log.Println("Connection tracker stopped")
			return
		}
	}
}

// Stop stops the connection tracker
func (ct *ConnectionTracker) Stop() {
	close(ct.stopCh)
}

// loadActiveConnections loads active connections from database on startup
func (ct *ConnectionTracker) loadActiveConnections() error {
	ctx := context.Background()

	rows, err := ct.db.DB().QueryContext(ctx, `
		SELECT id, session_id, user_id, client_ip, user_agent, connected_at, last_heartbeat
		FROM connections
		WHERE last_heartbeat > NOW() - INTERVAL '5 minutes'
	`)
	if err != nil {
		return fmt.Errorf("failed to query connections: %w", err)
	}
	defer rows.Close()

	ct.mu.Lock()
	defer ct.mu.Unlock()

	for rows.Next() {
		var conn Connection
		if err := rows.Scan(&conn.ID, &conn.SessionID, &conn.UserID, &conn.ClientIP, &conn.UserAgent, &conn.ConnectedAt, &conn.LastHeartbeat); err != nil {
			log.Printf("Failed to scan connection: %v", err)
			continue
		}

		ct.connections[conn.ID] = &conn
	}

	log.Printf("Loaded %d active connections from database", len(ct.connections))
	return nil
}

// checkConnections checks all connections and performs auto-hibernation
func (ct *ConnectionTracker) checkConnections() {
	ctx := context.Background()

	ct.mu.RLock()
	sessionConnections := make(map[string][]*Connection)
	for _, conn := range ct.connections {
		sessionConnections[conn.SessionID] = append(sessionConnections[conn.SessionID], conn)
	}
	ct.mu.RUnlock()

	// Check each session's connections
	for sessionID, conns := range sessionConnections {
		activeConns := 0
		now := time.Now()

		// Count active connections (heartbeat within window)
		for _, conn := range conns {
			if now.Sub(conn.LastHeartbeat) < ct.heartbeatWindow {
				activeConns++
			} else {
				// Connection is stale, remove it
				ct.removeConnection(ctx, conn.ID)
			}
		}

		// Update session active connections count in database
		if err := ct.updateSessionConnectionCount(ctx, sessionID, activeConns); err != nil {
			log.Printf("Failed to update session connection count: %v", err)
		}

		// Auto-hibernate if no active connections
		if activeConns == 0 {
			ct.autoHibernateSession(ctx, sessionID)
		}
	}
}

// AddConnection registers a new connection and triggers auto-start if needed.
//
// This method:
//  1. Adds connection to in-memory map
//  2. Persists connection to database
//  3. Updates session last_connection timestamp
//  4. Increments session active_connections count
//  5. Triggers auto-start if session is hibernated (async)
//
// Auto-start behavior:
//   - Runs in background goroutine (doesn't block)
//   - Only starts sessions in "hibernated" state
//   - Updates Kubernetes Session resource to state="running"
//
// Parameters:
//   - ctx: Context for database operations
//   - conn: Connection to add (must have valid ID, SessionID, UserID)
//
// Returns an error if:
//   - Database insert fails
//   - Session update fails
//
// Example:
//
//	conn := &Connection{
//	    ID: uuid.New().String(),
//	    SessionID: sessionID,
//	    UserID: userID,
//	    ClientIP: r.RemoteAddr,
//	    UserAgent: r.UserAgent(),
//	    ConnectedAt: time.Now(),
//	    LastHeartbeat: time.Now(),
//	}
//	err := tracker.AddConnection(ctx, conn)
func (ct *ConnectionTracker) AddConnection(ctx context.Context, conn *Connection) error {
	ct.mu.Lock()
	ct.connections[conn.ID] = conn
	ct.mu.Unlock()

	// Insert into database
	_, err := ct.db.DB().ExecContext(ctx, `
		INSERT INTO connections (id, session_id, user_id, client_ip, user_agent, connected_at, last_heartbeat)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, conn.ID, conn.SessionID, conn.UserID, conn.ClientIP, conn.UserAgent, conn.ConnectedAt, conn.LastHeartbeat)
	if err != nil {
		return fmt.Errorf("failed to insert connection: %w", err)
	}

	// Update session last_connection timestamp
	_, err = ct.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET last_connection = $1, active_connections = active_connections + 1
		WHERE id = $2
	`, time.Now(), conn.SessionID)
	if err != nil {
		log.Printf("Failed to update session last_connection: %v", err)
	}

	// Auto-start session if hibernated
	go ct.autoStartSession(ctx, conn.SessionID)

	log.Printf("Connection added: %s (session: %s, user: %s)", conn.ID, conn.SessionID, conn.UserID)
	return nil
}

// UpdateHeartbeat updates the last heartbeat time for a connection
func (ct *ConnectionTracker) UpdateHeartbeat(ctx context.Context, connectionID string) error {
	ct.mu.Lock()
	conn, exists := ct.connections[connectionID]
	if exists {
		conn.LastHeartbeat = time.Now()
	}
	ct.mu.Unlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	// Update in database
	_, err := ct.db.DB().ExecContext(ctx, `
		UPDATE connections
		SET last_heartbeat = $1
		WHERE id = $2
	`, time.Now(), connectionID)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return nil
}

// RemoveConnection removes a connection
func (ct *ConnectionTracker) RemoveConnection(ctx context.Context, connectionID string) error {
	return ct.removeConnection(ctx, connectionID)
}

// removeConnection internal method to remove connection
func (ct *ConnectionTracker) removeConnection(ctx context.Context, connectionID string) error {
	ct.mu.Lock()
	conn, exists := ct.connections[connectionID]
	if exists {
		delete(ct.connections, connectionID)
	}
	ct.mu.Unlock()

	if !exists {
		return nil // Already removed
	}

	// Delete from database
	_, err := ct.db.DB().ExecContext(ctx, `
		DELETE FROM connections WHERE id = $1
	`, connectionID)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	// Update session active connections count
	_, err = ct.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET active_connections = GREATEST(0, active_connections - 1),
		    last_disconnect = $1
		WHERE id = $2
	`, time.Now(), conn.SessionID)
	if err != nil {
		log.Printf("Failed to update session active_connections: %v", err)
	}

	log.Printf("Connection removed: %s (session: %s)", connectionID, conn.SessionID)
	return nil
}

// GetActiveConnections returns all active connections for a session
func (ct *ConnectionTracker) GetActiveConnections(sessionID string) []*Connection {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	var conns []*Connection
	now := time.Now()

	for _, conn := range ct.connections {
		if conn.SessionID == sessionID && now.Sub(conn.LastHeartbeat) < ct.heartbeatWindow {
			conns = append(conns, conn)
		}
	}

	return conns
}

// GetConnectionCount returns the number of active connections for a session
func (ct *ConnectionTracker) GetConnectionCount(sessionID string) int {
	conns := ct.GetActiveConnections(sessionID)
	return len(conns)
}

// GetConnection returns a connection by ID, or nil if not found
func (ct *ConnectionTracker) GetConnection(connectionID string) *Connection {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	conn, exists := ct.connections[connectionID]
	if !exists {
		return nil
	}

	// Check if connection is still within heartbeat window
	if time.Since(conn.LastHeartbeat) > ct.heartbeatWindow {
		return nil
	}

	return conn
}

// autoStartSession automatically starts a hibernated session
func (ct *ConnectionTracker) autoStartSession(ctx context.Context, sessionID string) {
	// Get session from K8s
	// Namespace is typically streamspace, but we should query from DB
	namespace := ct.getSessionNamespace(ctx, sessionID)
	if namespace == "" {
		namespace = "streamspace" // Default fallback
	}

	session, err := ct.k8sClient.GetSession(ctx, namespace, sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return
	}

	// Only auto-start if hibernated
	if session.State != "hibernated" {
		return
	}

	log.Printf("Auto-starting hibernated session: %s", sessionID)

	// Update session state to running
	_, err = ct.k8sClient.UpdateSessionState(ctx, namespace, sessionID, "running")
	if err != nil {
		log.Printf("Failed to auto-start session %s: %v", sessionID, err)
		return
	}

	// Publish wake event for controllers
	event := &events.SessionWakeEvent{
		SessionID: sessionID,
		UserID:    session.User,
		Platform:  ct.platform,
	}
	if err := ct.publisher.PublishSessionWake(ctx, event); err != nil {
		log.Printf("Warning: Failed to publish session wake event: %v", err)
	}

	log.Printf("Session auto-started: %s", sessionID)
}

// autoHibernateSession automatically hibernates a session with no connections
func (ct *ConnectionTracker) autoHibernateSession(ctx context.Context, sessionID string) {
	// Check if auto-hibernation is enabled for this session
	enabled, idleTimeout := ct.getAutoHibernationSettings(ctx, sessionID)
	if !enabled {
		return
	}

	// Get last disconnect time
	lastDisconnect, err := ct.getLastDisconnect(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to get last disconnect for session %s: %v", sessionID, err)
		return
	}

	// Only hibernate if idle timeout has passed
	if time.Since(lastDisconnect) < idleTimeout {
		return
	}

	// Get session from K8s
	namespace := ct.getSessionNamespace(ctx, sessionID)
	if namespace == "" {
		namespace = "streamspace"
	}

	session, err := ct.k8sClient.GetSession(ctx, namespace, sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return
	}

	// Only hibernate if running
	if session.State != "running" {
		return
	}

	log.Printf("Auto-hibernating idle session: %s", sessionID)

	// Update session state to hibernated
	_, err = ct.k8sClient.UpdateSessionState(ctx, namespace, sessionID, "hibernated")
	if err != nil {
		log.Printf("Failed to auto-hibernate session %s: %v", sessionID, err)
		return
	}

	// Publish hibernate event for controllers
	event := &events.SessionHibernateEvent{
		SessionID: sessionID,
		UserID:    session.User,
		Platform:  ct.platform,
	}
	if err := ct.publisher.PublishSessionHibernate(ctx, event); err != nil {
		log.Printf("Warning: Failed to publish session hibernate event: %v", err)
	}

	log.Printf("Session auto-hibernated: %s", sessionID)
}

// updateSessionConnectionCount updates the active_connections count in database
func (ct *ConnectionTracker) updateSessionConnectionCount(ctx context.Context, sessionID string, count int) error {
	_, err := ct.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET active_connections = $1, updated_at = $2
		WHERE id = $3
	`, count, time.Now(), sessionID)

	return err
}

// getSessionNamespace gets the namespace for a session from database
func (ct *ConnectionTracker) getSessionNamespace(ctx context.Context, sessionID string) string {
	var namespace string
	err := ct.db.DB().QueryRowContext(ctx, `
		SELECT namespace FROM sessions WHERE id = $1
	`, sessionID).Scan(&namespace)

	if err != nil {
		return ""
	}

	return namespace
}

// getAutoHibernationSettings gets auto-hibernation settings for a session
func (ct *ConnectionTracker) getAutoHibernationSettings(ctx context.Context, sessionID string) (bool, time.Duration) {
	// Check global setting first
	var enabledStr string
	err := ct.db.DB().QueryRowContext(ctx, `
		SELECT value FROM configuration WHERE key = 'session.enableAutoHibernation'
	`).Scan(&enabledStr)

	if err != nil || enabledStr != "true" {
		return false, 0
	}

	// Get default idle timeout
	var timeoutStr string
	err = ct.db.DB().QueryRowContext(ctx, `
		SELECT value FROM configuration WHERE key = 'session.defaultIdleTimeout'
	`).Scan(&timeoutStr)

	if err != nil {
		timeoutStr = "30m" // Default
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Minute // Default
	}

	return true, timeout
}

// getLastDisconnect gets the last disconnect time for a session
func (ct *ConnectionTracker) getLastDisconnect(ctx context.Context, sessionID string) (time.Time, error) {
	var lastDisconnect sql.NullTime
	err := ct.db.DB().QueryRowContext(ctx, `
		SELECT last_disconnect FROM sessions WHERE id = $1
	`, sessionID).Scan(&lastDisconnect)

	if err != nil {
		return time.Time{}, err
	}

	if !lastDisconnect.Valid {
		// No disconnect recorded, use current time
		return time.Now(), nil
	}

	return lastDisconnect.Time, nil
}

// GetStats returns connection statistics
func (ct *ConnectionTracker) GetStats() map[string]interface{} {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalConns := len(ct.connections)
	sessionCounts := make(map[string]int)

	now := time.Now()
	activeConns := 0

	for _, conn := range ct.connections {
		if now.Sub(conn.LastHeartbeat) < ct.heartbeatWindow {
			activeConns++
			sessionCounts[conn.SessionID]++
		}
	}

	return map[string]interface{}{
		"totalConnections":   totalConns,
		"activeConnections":  activeConns,
		"uniqueSessions":     len(sessionCounts),
		"heartbeatWindow":    ct.heartbeatWindow.String(),
		"checkInterval":      ct.checkInterval.String(),
		"sessionConnections": sessionCounts,
	}
}
