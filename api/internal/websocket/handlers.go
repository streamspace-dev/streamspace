// Package websocket provides real-time WebSocket communication for StreamSpace.
//
// This file implements WebSocket managers and broadcasting for real-time updates.
//
// Purpose:
// - Manage multiple WebSocket hubs (sessions, metrics, logs)
// - Periodically broadcast session and metric updates to connected clients
// - Stream pod logs in real-time via WebSocket
// - Integrate database and Kubernetes for live data
//
// Features:
// - Multi-hub architecture (sessions, metrics separate channels)
// - Periodic broadcast intervals (sessions: 3s, metrics: 5s)
// - Database-enriched session data (active connections, activity status)
// - Real-time pod log streaming
// - Event-driven notifications via Notifier integration
// - Graceful shutdown with connection cleanup
//
// Architecture:
//   - Manager: Coordinates all hubs and data sources
//   - Hub: Manages WebSocket connections and message delivery
//   - Notifier: Routes targeted notifications to subscribed clients
//   - Broadcast goroutines: Fetch and push updates periodically
//
// Broadcast Strategy:
//   - Sessions: Every 3 seconds, fetch all sessions from Kubernetes
//   - Metrics: Every 5 seconds, aggregate counts from database
//   - Skip broadcasts when no clients connected (performance)
//   - Enriched data includes database fields (active_connections)
//
// Implementation Details:
// - Uses gorilla/websocket for WebSocket protocol
// - Kubernetes client for session data (k8s.Client)
// - Database for enrichment (active connections, metrics)
// - JSON-encoded messages for all broadcasts
//
// Thread Safety:
// - Manager is thread-safe
// - Hub operations use mutex protection
// - Broadcast goroutines run concurrently
//
// Dependencies:
// - github.com/gorilla/websocket for WebSocket protocol
// - internal/db for database access
// - internal/k8s for Kubernetes API
//
// Example Usage:
//
//	// Create manager with database and K8s client
//	manager := websocket.NewManager(database, k8sClient)
//	manager.Start()
//
//	// Handle WebSocket connections
//	router.GET("/ws/sessions", func(c *gin.Context) {
//	    userID := c.Query("user_id")
//	    conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil)
//	    manager.HandleSessionsWebSocket(conn, userID, "")
//	})
//
//	// Shutdown cleanly
//	defer manager.CloseAll()
package websocket

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
	corev1 "k8s.io/api/core/v1"
)

// Manager manages all WebSocket hubs
type Manager struct {
	sessionsHub *Hub
	metricsHub  *Hub
	db          *db.Database
	k8sClient   *k8s.Client
	notifier    *Notifier
}

// NewManager creates a new WebSocket manager
func NewManager(database *db.Database, k8sClient *k8s.Client) *Manager {
	m := &Manager{
		sessionsHub: NewHub(),
		metricsHub:  NewHub(),
		db:          database,
		k8sClient:   k8sClient,
	}
	// Initialize notifier with reference to manager
	m.notifier = NewNotifier(m)
	return m
}

// Start starts all WebSocket hubs
func (m *Manager) Start() {
	go m.sessionsHub.Run()
	go m.metricsHub.Run()
	go m.broadcastSessionUpdates()
	go m.broadcastMetrics()
}

// GetNotifier returns the notifier for event-driven notifications
func (m *Manager) GetNotifier() *Notifier {
	return m.notifier
}

// HandleSessionsWebSocket handles WebSocket connections for session updates
// Supports subscribing to user-specific or session-specific events via query params:
// - ?user_id=<userID> - Subscribe to all events for a specific user
// - ?session_id=<sessionID> - Subscribe to events for a specific session
func (m *Manager) HandleSessionsWebSocket(conn *websocket.Conn, userID, sessionID string) {
	clientID := uuid.New().String()

	// Subscribe to user or session events if specified
	if userID != "" {
		m.notifier.SubscribeUser(clientID, userID)
	}
	if sessionID != "" {
		m.notifier.SubscribeSession(clientID, sessionID)
	}

	// Cleanup subscription on disconnect
	defer m.notifier.UnsubscribeClient(clientID)

	m.sessionsHub.ServeClient(conn, clientID)
}

// CloseAll closes all WebSocket connections and subscriptions
func (m *Manager) CloseAll() {
	log.Println("Closing all WebSocket connections...")

	// Close notifier subscriptions
	if m.notifier != nil {
		m.notifier.CloseAll()
	}

	// Close session hub clients
	if m.sessionsHub != nil {
		m.sessionsHub.mu.Lock()
		for client := range m.sessionsHub.clients {
			close(client.send)
		}
		m.sessionsHub.clients = make(map[*Client]bool)
		m.sessionsHub.mu.Unlock()
	}

	// Close metrics hub clients
	if m.metricsHub != nil {
		m.metricsHub.mu.Lock()
		for client := range m.metricsHub.clients {
			close(client.send)
		}
		m.metricsHub.clients = make(map[*Client]bool)
		m.metricsHub.mu.Unlock()
	}

	log.Println("All WebSocket connections closed")
}

// HandleMetricsWebSocket handles WebSocket connections for metrics updates
func (m *Manager) HandleMetricsWebSocket(conn *websocket.Conn) {
	clientID := uuid.New().String()
	m.metricsHub.ServeClient(conn, clientID)
}

// HandleLogsWebSocket handles WebSocket connections for pod logs streaming
func (m *Manager) HandleLogsWebSocket(conn *websocket.Conn, namespace, podName string) {
	defer conn.Close()

	ctx := context.Background()

	// Get pod logs stream
	req := m.k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		TailLines:  int64Ptr(100),
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("Failed to get pod logs stream: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
		return
	}
	defer stream.Close()

	// Read logs and send to WebSocket
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading logs: %v", err)
			}
			break
		}

		// Send log line to WebSocket
		if err := conn.WriteMessage(websocket.TextMessage, line); err != nil {
			log.Printf("Error writing to WebSocket: %v", err)
			break
		}
	}
}

// broadcastSessionUpdates periodically fetches and broadcasts session updates
func (m *Manager) broadcastSessionUpdates() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if m.sessionsHub.ClientCount() == 0 {
			continue // No clients, skip
		}

		ctx := context.Background()

		// Fetch all sessions
		sessions, err := m.k8sClient.ListSessions(ctx, "streamspace")
		if err != nil {
			log.Printf("Failed to fetch sessions for broadcast: %v", err)
			continue
		}

		// Enrich with database info (active connections) and activity status
		enrichedSessions := make([]map[string]interface{}, 0, len(sessions))
		for _, session := range sessions {
			// Get active connections count from database
			var activeConns int
			if err := m.db.DB().QueryRowContext(ctx, `
				SELECT active_connections FROM sessions WHERE id = $1
			`, session.Name).Scan(&activeConns); err != nil {
				// If query fails, default to 0
				activeConns = 0
			}

			sessionData := map[string]interface{}{
				"name":      session.Name,
				"namespace": session.Namespace,
				"user":      session.User,
				"template":  session.Template,
				"state":     session.State,
				// Convert status to proper JSON format with lowercase keys
				"status": map[string]interface{}{
					"phase":   session.Status.Phase,
					"podName": session.Status.PodName,
					"url":     session.Status.URL,
				},
				"createdAt":         session.CreatedAt,
				"activeConnections": activeConns,
			}

			if session.Resources.Memory != "" || session.Resources.CPU != "" {
				sessionData["resources"] = map[string]string{
					"memory": session.Resources.Memory,
					"cpu":    session.Resources.CPU,
				}
			}

			// Add activity status
			if session.Status.LastActivity != nil {
				sessionData["lastActivity"] = session.Status.LastActivity.Format(time.RFC3339)

				// Calculate idle status
				if session.IdleTimeout != "" {
					idleThreshold, err := time.ParseDuration(session.IdleTimeout)
					if err == nil && idleThreshold > 0 {
						idleDuration := time.Since(*session.Status.LastActivity)
						sessionData["idleDuration"] = int64(idleDuration.Seconds())
						sessionData["idleThreshold"] = int64(idleThreshold.Seconds())
						sessionData["isIdle"] = idleDuration >= idleThreshold
						sessionData["isActive"] = idleDuration < idleThreshold
					}
				}
			}

			enrichedSessions = append(enrichedSessions, sessionData)
		}

		// Broadcast to all clients
		message := map[string]interface{}{
			"type":      "sessions_update",
			"sessions":  enrichedSessions,
			"count":     len(enrichedSessions),
			"timestamp": time.Now().Format(time.RFC3339),
		}

		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Failed to marshal sessions update: %v", err)
			continue
		}

		m.sessionsHub.Broadcast(data)
	}
}

// broadcastMetrics periodically fetches and broadcasts metrics
func (m *Manager) broadcastMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if m.metricsHub.ClientCount() == 0 {
			continue // No clients, skip
		}

		ctx := context.Background()

		// Get session counts by state
		var runningCount, hibernatedCount, totalCount int

		err := m.db.DB().QueryRowContext(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE state = 'running') as running,
				COUNT(*) FILTER (WHERE state = 'hibernated') as hibernated,
				COUNT(*) as total
			FROM sessions
		`).Scan(&runningCount, &hibernatedCount, &totalCount)

		if err != nil {
			log.Printf("Failed to fetch session metrics: %v", err)
			continue
		}

		// Get total active connections
		var activeConnections int
		err = m.db.DB().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM connections
			WHERE last_heartbeat > NOW() - INTERVAL '2 minutes'
		`).Scan(&activeConnections)

		if err != nil {
			log.Printf("Failed to fetch connection metrics: %v", err)
			activeConnections = 0
		}

		// Get repository count
		var repoCount int
		err = m.db.DB().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM repositories
		`).Scan(&repoCount)

		if err != nil {
			log.Printf("Failed to fetch repository count: %v", err)
			repoCount = 0
		}

		// Get template count
		var templateCount int
		err = m.db.DB().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM catalog_templates
		`).Scan(&templateCount)

		if err != nil {
			log.Printf("Failed to fetch template count: %v", err)
			templateCount = 0
		}

		// Broadcast metrics
		message := map[string]interface{}{
			"type": "metrics_update",
			"metrics": map[string]interface{}{
				"sessions": map[string]int{
					"running":    runningCount,
					"hibernated": hibernatedCount,
					"total":      totalCount,
				},
				"activeConnections": activeConnections,
				"repositories":      repoCount,
				"templates":         templateCount,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}

		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Failed to marshal metrics update: %v", err)
			continue
		}

		m.metricsHub.Broadcast(data)
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
