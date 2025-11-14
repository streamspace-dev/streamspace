package tracker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
)

// ConnectionTracker manages active connections and auto-hibernation
type ConnectionTracker struct {
	db              *db.Database
	k8sClient       *k8s.Client
	connections     map[string]*Connection // sessionID -> Connection
	mu              sync.RWMutex
	checkInterval   time.Duration
	heartbeatWindow time.Duration
	stopCh          chan struct{}
}

// Connection represents an active connection to a session
type Connection struct {
	ID            string
	SessionID     string
	UserID        string
	ClientIP      string
	UserAgent     string
	ConnectedAt   time.Time
	LastHeartbeat time.Time
}

// NewConnectionTracker creates a new connection tracker
func NewConnectionTracker(database *db.Database, k8sClient *k8s.Client) *ConnectionTracker {
	return &ConnectionTracker{
		db:              database,
		k8sClient:       k8sClient,
		connections:     make(map[string]*Connection),
		checkInterval:   30 * time.Second,  // Check every 30 seconds
		heartbeatWindow: 60 * time.Second,  // Disconnect if no heartbeat for 60s
		stopCh:          make(chan struct{}),
	}
}

// Start begins the connection tracking loop
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

// AddConnection registers a new connection
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
