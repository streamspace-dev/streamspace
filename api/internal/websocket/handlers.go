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

// OrgContext contains the organization context for WebSocket connections.
// SECURITY: This is REQUIRED for all WebSocket connections to ensure org isolation.
type OrgContext struct {
	// OrgID is the organization this connection belongs to.
	OrgID string

	// K8sNamespace is the Kubernetes namespace for this org.
	K8sNamespace string

	// UserID is the authenticated user's ID.
	UserID string
}

// HandleSessionsWebSocket handles WebSocket connections for session updates (deprecated)
// DEPRECATED: Use HandleSessionsWebSocketWithOrg for multi-tenant deployments.
// Supports subscribing to user-specific or session-specific events via query params:
// - ?user_id=<userID> - Subscribe to all events for a specific user
// - ?session_id=<sessionID> - Subscribe to events for a specific session
func (m *Manager) HandleSessionsWebSocket(conn *websocket.Conn, userID, sessionID string) {
	// Default to "default-org" for backward compatibility
	m.HandleSessionsWebSocketWithOrg(conn, userID, sessionID, &OrgContext{
		OrgID:        "default-org",
		K8sNamespace: "streamspace",
		UserID:       userID,
	})
}

// HandleSessionsWebSocketWithOrg handles WebSocket connections for session updates with org context.
// SECURITY: This function requires org context for multi-tenant isolation.
// All session updates will be scoped to the specified organization.
//
// Parameters:
//   - conn: WebSocket connection
//   - userID: User ID to subscribe to user-specific events
//   - sessionID: Session ID to subscribe to session-specific events
//   - orgCtx: Organization context (REQUIRED for multi-tenancy)
func (m *Manager) HandleSessionsWebSocketWithOrg(conn *websocket.Conn, userID, sessionID string, orgCtx *OrgContext) {
	// SECURITY: Reject connections without org context
	if orgCtx == nil || orgCtx.OrgID == "" {
		log.Printf("WebSocket connection rejected: missing org context")
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "org context required"))
		conn.Close()
		return
	}

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

	// Use org-scoped client registration
	m.sessionsHub.ServeClientWithOrg(conn, clientID, orgCtx.OrgID, orgCtx.K8sNamespace, orgCtx.UserID)
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

// HandleMetricsWebSocket handles WebSocket connections for metrics updates (deprecated)
// DEPRECATED: Use HandleMetricsWebSocketWithOrg for multi-tenant deployments.
func (m *Manager) HandleMetricsWebSocket(conn *websocket.Conn) {
	m.HandleMetricsWebSocketWithOrg(conn, &OrgContext{
		OrgID:        "default-org",
		K8sNamespace: "streamspace",
	})
}

// HandleMetricsWebSocketWithOrg handles WebSocket connections for metrics updates with org context.
// SECURITY: This function requires org context for multi-tenant isolation.
// All metrics will be scoped to the specified organization.
func (m *Manager) HandleMetricsWebSocketWithOrg(conn *websocket.Conn, orgCtx *OrgContext) {
	// SECURITY: Reject connections without org context
	if orgCtx == nil || orgCtx.OrgID == "" {
		log.Printf("WebSocket metrics connection rejected: missing org context")
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "org context required"))
		conn.Close()
		return
	}

	clientID := uuid.New().String()
	m.metricsHub.ServeClientWithOrg(conn, clientID, orgCtx.OrgID, orgCtx.K8sNamespace, orgCtx.UserID)
}

// HandleLogsWebSocket handles WebSocket connections for pod logs streaming (deprecated)
// DEPRECATED: Use HandleLogsWebSocketWithOrg for multi-tenant deployments.
func (m *Manager) HandleLogsWebSocket(conn *websocket.Conn, namespace, podName string) {
	// For backward compatibility, use provided namespace
	m.HandleLogsWebSocketWithOrg(conn, podName, &OrgContext{
		OrgID:        "default-org",
		K8sNamespace: namespace,
	})
}

// HandleLogsWebSocketWithOrg handles WebSocket connections for pod logs streaming with org context.
// SECURITY: This function requires org context for multi-tenant isolation.
// Pod logs will only be accessible within the org's K8s namespace.
func (m *Manager) HandleLogsWebSocketWithOrg(conn *websocket.Conn, podName string, orgCtx *OrgContext) {
	defer conn.Close()

	// SECURITY: Reject connections without org context
	if orgCtx == nil || orgCtx.OrgID == "" || orgCtx.K8sNamespace == "" {
		log.Printf("WebSocket logs connection rejected: missing org context")
		conn.WriteMessage(websocket.TextMessage, []byte("Error: org context required"))
		return
	}

	ctx := context.Background()

	// SECURITY: Use org's K8s namespace to prevent cross-tenant access
	namespace := orgCtx.K8sNamespace

	// Get pod logs stream
	req := m.k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		TailLines:  int64Ptr(100),
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("Failed to get pod logs stream for %s/%s: %v", namespace, podName, err)
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
				log.Printf("Error reading logs for %s/%s: %v", namespace, podName, err)
			}
			break
		}

		// Send log line to WebSocket
		if err := conn.WriteMessage(websocket.TextMessage, line); err != nil {
			log.Printf("Error writing to WebSocket for %s/%s: %v", namespace, podName, err)
			break
		}
	}
}

// broadcastSessionUpdates periodically fetches and broadcasts session updates
// SECURITY: Sessions are now broadcast per-org to prevent cross-tenant data leakage.
func (m *Manager) broadcastSessionUpdates() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if m.sessionsHub.ClientCount() == 0 {
			continue // No clients, skip
		}

		ctx := context.Background()

		// SECURITY: Broadcast sessions per-org to prevent cross-tenant leakage
		// Get unique orgs with connected clients
		orgs := m.sessionsHub.GetUniqueOrgs()

		for _, orgID := range orgs {
			// v2.0 ARCHITECTURE: Read from database (source of truth), not Kubernetes
			// The database is populated by agents via WebSocket commands
			// This ensures platform-agnostic operation (K8s, Docker, VM, Cloud)

			// Fetch sessions from database for this org
			rows, err := m.db.DB().QueryContext(ctx, `
				SELECT id, user_id, template_name, state, namespace, created_at,
				       active_connections, url, pod_name, memory, cpu,
				       idle_timeout, last_activity, platform, agent_id
				FROM sessions
				WHERE org_id = $1 AND state != 'terminated'
				ORDER BY created_at DESC
			`, orgID)
			if err != nil {
				log.Printf("Failed to fetch sessions for org %s from database: %v", orgID, err)
				continue
			}

			// Build enriched session list
			enrichedSessions := make([]map[string]interface{}, 0)
			for rows.Next() {
				var (
					id, userID, templateName, state, namespace string
					createdAt                                   time.Time
					activeConns                                 int
					url, podName, memory, cpu, idleTimeout      *string
					lastActivity                                *time.Time
					platform, agentID                           *string
				)

				if err := rows.Scan(&id, &userID, &templateName, &state, &namespace,
					&createdAt, &activeConns, &url, &podName, &memory, &cpu,
					&idleTimeout, &lastActivity, &platform, &agentID); err != nil {
					log.Printf("Failed to scan session row: %v", err)
					continue
				}

				sessionData := map[string]interface{}{
					"name":              id,
					"namespace":         namespace,
					"user":              userID,
					"template":          templateName,
					"state":             state,
					"createdAt":         createdAt.Format(time.RFC3339),
					"activeConnections": activeConns,
				}

				// Add status info
				status := make(map[string]interface{})
				status["phase"] = state
				if podName != nil {
					status["podName"] = *podName
				}
				if url != nil {
					status["url"] = *url
				}
				sessionData["status"] = status

				// Add resources if present
				if (memory != nil && *memory != "") || (cpu != nil && *cpu != "") {
					resources := make(map[string]string)
					if memory != nil {
						resources["memory"] = *memory
					}
					if cpu != nil {
						resources["cpu"] = *cpu
					}
					sessionData["resources"] = resources
				}

				// Add platform info (v2.0)
				if platform != nil {
					sessionData["platform"] = *platform
				}
				if agentID != nil {
					sessionData["agent_id"] = *agentID
				}

				// Add activity status
				if lastActivity != nil {
					sessionData["lastActivity"] = lastActivity.Format(time.RFC3339)

					// Calculate idle status
					if idleTimeout != nil && *idleTimeout != "" {
						if threshold, err := time.ParseDuration(*idleTimeout); err == nil && threshold > 0 {
							idleDuration := time.Since(*lastActivity)
							sessionData["idleDuration"] = int64(idleDuration.Seconds())
							sessionData["idleThreshold"] = int64(threshold.Seconds())
							sessionData["isIdle"] = idleDuration >= threshold
							sessionData["isActive"] = idleDuration < threshold
						}
					}
				}

				enrichedSessions = append(enrichedSessions, sessionData)
			}
			rows.Close()

			// Broadcast to clients in this org only
			message := map[string]interface{}{
				"type":      "sessions_update",
				"sessions":  enrichedSessions,
				"count":     len(enrichedSessions),
				"org_id":    orgID,
				"timestamp": time.Now().Format(time.RFC3339),
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Failed to marshal sessions update for org %s: %v", orgID, err)
				continue
			}

			// SECURITY: Broadcast only to clients in this org
			m.sessionsHub.BroadcastToOrg(orgID, data)
		}
	}
}

// broadcastMetrics periodically fetches and broadcasts metrics
// SECURITY: Metrics are now broadcast per-org to prevent cross-tenant data leakage.
func (m *Manager) broadcastMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if m.metricsHub.ClientCount() == 0 {
			continue // No clients, skip
		}

		ctx := context.Background()

		// SECURITY: Broadcast metrics per-org to prevent cross-tenant leakage
		orgs := m.metricsHub.GetUniqueOrgs()

		for _, orgID := range orgs {
			// Get session counts by state for this org
			var runningCount, hibernatedCount, totalCount int

			err := m.db.DB().QueryRowContext(ctx, `
				SELECT
					COUNT(*) FILTER (WHERE state = 'running') as running,
					COUNT(*) FILTER (WHERE state = 'hibernated') as hibernated,
					COUNT(*) as total
				FROM sessions
				WHERE org_id = $1
			`, orgID).Scan(&runningCount, &hibernatedCount, &totalCount)

			if err != nil {
				log.Printf("Failed to fetch session metrics for org %s: %v", orgID, err)
				continue
			}

			// Get total active connections for this org
			var activeConnections int
			err = m.db.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM connections c
				JOIN sessions s ON c.session_id = s.id
				WHERE c.last_heartbeat > NOW() - INTERVAL '2 minutes'
				AND s.org_id = $1
			`, orgID).Scan(&activeConnections)

			if err != nil {
				log.Printf("Failed to fetch connection metrics for org %s: %v", orgID, err)
				activeConnections = 0
			}

			// Get repository count (global for now - could be org-scoped in future)
			var repoCount int
			err = m.db.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM repositories
			`).Scan(&repoCount)

			if err != nil {
				log.Printf("Failed to fetch repository count: %v", err)
				repoCount = 0
			}

			// Get template count (global for now - could be org-scoped in future)
			var templateCount int
			err = m.db.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM catalog_templates
			`).Scan(&templateCount)

			if err != nil {
				log.Printf("Failed to fetch template count: %v", err)
				templateCount = 0
			}

			// Broadcast metrics to clients in this org only
			message := map[string]interface{}{
				"type":   "metrics_update",
				"org_id": orgID,
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
				log.Printf("Failed to marshal metrics update for org %s: %v", orgID, err)
				continue
			}

			// SECURITY: Broadcast only to clients in this org
			m.metricsHub.BroadcastToOrg(orgID, data)
		}
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
