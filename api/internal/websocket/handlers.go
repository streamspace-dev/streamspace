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
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	corev1 "k8s.io/api/core/v1"
)

// Manager manages all WebSocket hubs
type Manager struct {
	sessionsHub *Hub
	metricsHub  *Hub
	db          *db.Database
	k8sClient   *k8s.Client
}

// NewManager creates a new WebSocket manager
func NewManager(database *db.Database, k8sClient *k8s.Client) *Manager {
	return &Manager{
		sessionsHub: NewHub(),
		metricsHub:  NewHub(),
		db:          database,
		k8sClient:   k8sClient,
	}
}

// Start starts all WebSocket hubs
func (m *Manager) Start() {
	go m.sessionsHub.Run()
	go m.metricsHub.Run()
	go m.broadcastSessionUpdates()
	go m.broadcastMetrics()
}

// HandleSessionsWebSocket handles WebSocket connections for session updates
func (m *Manager) HandleSessionsWebSocket(conn *websocket.Conn) {
	clientID := uuid.New().String()
	m.sessionsHub.ServeClient(conn, clientID)
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

		// Enrich with database info (active connections)
		enrichedSessions := make([]map[string]interface{}, 0, len(sessions))
		for _, session := range sessions {
			// Get active connections count from database
			var activeConns int
			err := m.db.DB().QueryRowContext(ctx, `
				SELECT active_connections FROM sessions WHERE id = $1
			`, session.Name).Scan(&activeConns)

			sessionData := map[string]interface{}{
				"name":               session.Name,
				"namespace":          session.Namespace,
				"user":               session.User,
				"template":           session.Template,
				"state":              session.State,
				"status":             session.Status,
				"createdAt":          session.CreatedAt,
				"activeConnections":  activeConns,
			}

			if session.Resources.Memory != "" || session.Resources.CPU != "" {
				sessionData["resources"] = map[string]string{
					"memory": session.Resources.Memory,
					"cpu":    session.Resources.CPU,
				}
			}

			enrichedSessions = append(enrichedSessions, sessionData)
		}

		// Broadcast to all clients
		message := map[string]interface{}{
			"type":     "sessions_update",
			"sessions": enrichedSessions,
			"count":    len(enrichedSessions),
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
