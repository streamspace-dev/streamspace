// Package api provides HTTP request handlers for the StreamSpace API.
//
// This file implements the core REST API endpoints for managing sessions, templates,
// and repositories in the StreamSpace container streaming platform.
//
// HANDLER OVERVIEW:
//
// The API handler provides endpoints for:
// - Session management (create, read, update, delete, connect)
// - Template management (list, search, favorites)
// - Template catalog (marketplace)
// - Repository management (sync external template sources)
// - Connection tracking (active user connections)
// - Quota enforcement (resource limits)
//
// ARCHITECTURE:
//
// The handler acts as a bridge between HTTP requests and:
// - Kubernetes API (via k8s.Client) for Session/Template CRDs
// - PostgreSQL database (via db.Database) for caching and metadata
// - Connection tracker for real-time session monitoring
// - Quota enforcer for resource limit validation
// - Sync service for external repository synchronization
// - WebSocket manager for real-time updates
//
// SECURITY CONSIDERATIONS:
//
// 1. Authentication: All endpoints assume authentication middleware has run
//    - User context available via c.Get("userID"), c.Get("userRole")
//    - Admin-only endpoints should use auth.RequireRole("admin") middleware
//
// 2. Authorization: Session ownership validated before operations
//    - Users can only manage their own sessions
//    - Admins can manage all sessions
//
// 3. Input Validation: All request payloads validated with binding tags
//    - Malformed JSON rejected with 400 Bad Request
//    - Required fields enforced
//
// 4. Quota Enforcement: Resource limits checked before session creation
//    - Prevents resource exhaustion attacks
//    - Enforces fair usage policies
//
// 5. Database Caching: Sessions cached in PostgreSQL for performance
//    - Cache updates are best-effort (failures logged but not blocking)
//    - Kubernetes is source of truth, database is cache
//
// DATA FLOW:
//
// Session Creation:
//   1. Client → POST /api/sessions {user, template, resources}
//   2. Handler validates template exists in Kubernetes
//   3. Handler checks user quota against current usage
//   4. Handler creates Session CRD in Kubernetes
//   5. Handler caches session in PostgreSQL (best-effort)
//   6. Controller watches Session CRD and creates Deployment/Service
//   7. Client polls GET /api/sessions/{id} for status updates
//
// Session Connection:
//   1. Client → POST /api/sessions/{id}/connect?user={userID}
//   2. Handler verifies session exists
//   3. Handler creates connection record in tracker
//   4. Handler returns session URL and connection ID
//   5. Client establishes WebSocket/VNC connection
//   6. Client sends periodic heartbeats to keep connection alive
//   7. On disconnect, client calls disconnect endpoint
//
// Template Sync:
//   1. Admin → POST /api/repositories (add GitHub repo)
//   2. Handler triggers background sync in SyncService
//   3. SyncService clones repo, parses templates, stores in database
//   4. Templates available in catalog endpoint
//   5. User → POST /api/catalog/{id}/install
//   6. Handler creates Template CRD in Kubernetes from catalog manifest
//
// ERROR HANDLING:
//
// All endpoints follow consistent error response format:
//   {
//     "error": "Short error code",
//     "message": "Detailed error message"
//   }
//
// HTTP Status Codes:
// - 200 OK: Successful read operation
// - 201 Created: Successful resource creation
// - 202 Accepted: Async operation started (e.g., sync)
// - 400 Bad Request: Invalid request format or parameters
// - 401 Unauthorized: Authentication required
// - 403 Forbidden: Insufficient permissions or quota exceeded
// - 404 Not Found: Resource does not exist
// - 500 Internal Server Error: Server-side error
package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/events"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/quota"
	"github.com/streamspace-dev/streamspace/api/internal/sync"
	"github.com/streamspace-dev/streamspace/api/internal/tracker"
	"github.com/streamspace-dev/streamspace/api/internal/websocket"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// sessionGVR defines the GroupVersionResource for Session custom resources.
//
// This is used with Kubernetes dynamic client to directly manipulate Session CRDs
// when the strongly-typed client is not sufficient (e.g., updating tags field).
//
// Format: {group}/{version}/namespaces/{namespace}/{resource}
// Example: stream.space/v1alpha1/namespaces/streamspace/sessions
var (
	sessionGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "sessions",
	}
)

// Handler handles all API requests for StreamSpace.
//
// This is the main request handler that routes HTTP requests to appropriate
// business logic and manages interactions with Kubernetes, database, and
// external services.
//
// DEPENDENCIES:
//
// - db: PostgreSQL database for caching and metadata
// - k8sClient: Kubernetes client for managing Session/Template CRDs
// - connTracker: Connection tracker for monitoring active user connections
// - syncService: Repository sync service for external template sources
// - wsManager: WebSocket manager for real-time updates
// - quotaEnforcer: Quota enforcement for resource limits
// - namespace: Kubernetes namespace where resources are created
//
// CONCURRENCY:
//
// Handler is safe for concurrent use by multiple goroutines (one per HTTP request).
// Each request gets its own Gin context with isolated state.
type Handler struct {
	db             *db.Database                 // Database for caching and metadata
	sessionDB      *db.SessionDB                // Session database operations
	k8sClient      *k8s.Client                  // Kubernetes client for CRD operations
	publisher      *events.Publisher            // DEPRECATED: NATS event publisher (stub, no-op)
	dispatcher     CommandDispatcher            // Command dispatcher for agent WebSocket commands
	connTracker    *tracker.ConnectionTracker   // Active connection tracking
	syncService    *sync.SyncService            // Repository synchronization
	wsManager      *websocket.Manager           // WebSocket connection manager
	quotaEnforcer  *quota.Enforcer              // Resource quota enforcement
	namespace      string                       // Kubernetes namespace for resources
	platform       string                       // Target platform (kubernetes, docker, etc.)
}

// CommandDispatcher interface for dispatching commands to agents
type CommandDispatcher interface {
	DispatchCommand(command interface{}) error
}

// NewHandler creates a new API handler with injected dependencies.
//
// PARAMETERS:
//
// - database: PostgreSQL database connection for caching and metadata
// - k8sClient: Kubernetes client for Session/Template CRD operations
// - publisher: NATS event publisher for platform-agnostic operations
// - connTracker: Connection tracker for active session monitoring
// - syncService: Service for syncing external template repositories
// - wsManager: Manager for WebSocket connections and real-time updates
// - quotaEnforcer: Enforcer for validating resource quotas
// - platform: Target platform (kubernetes, docker, hyperv, vcenter)
//
// NAMESPACE RESOLUTION:
//
// The Kubernetes namespace is read from NAMESPACE environment variable.
// If not set, defaults to "streamspace".
//
// EXAMPLE USAGE:
//
//   handler := NewHandler(db, k8sClient, publisher, connTracker, syncService, wsManager, quotaEnforcer, "kubernetes")
//   router := gin.Default()
//   router.GET("/api/sessions", handler.ListSessions)
//   router.POST("/api/sessions", handler.CreateSession)
func NewHandler(database *db.Database, k8sClient *k8s.Client, publisher *events.Publisher, dispatcher CommandDispatcher, connTracker *tracker.ConnectionTracker, syncService *sync.SyncService, wsManager *websocket.Manager, quotaEnforcer *quota.Enforcer, platform string) *Handler {
	// Read namespace from environment variable for deployment flexibility
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "streamspace" // Default namespace
	}
	if platform == "" {
		platform = events.PlatformKubernetes // Default platform
	}
	return &Handler{
		db:            database,
		sessionDB:     db.NewSessionDB(database.DB()),
		k8sClient:     k8sClient,
		publisher:     publisher,
		dispatcher:    dispatcher,
		connTracker:   connTracker,
		syncService:   syncService,
		wsManager:     wsManager,
		quotaEnforcer: quotaEnforcer,
		namespace:     namespace,
		platform:      platform,
	}
}

// ============================================================================
// Session Endpoints
// ============================================================================

// ListSessions retrieves all sessions for a specific user or all sessions (admin).
//
// HTTP Method: GET
// Path: /api/sessions
// Authentication: Required
// Authorization: User can list own sessions; Admin can list all sessions
//
// QUERY PARAMETERS:
//
// - user (optional): Filter sessions by user ID
//   - If provided: Returns sessions for that specific user
//   - If omitted: Returns all sessions (requires admin role)
//
// REQUEST EXAMPLE:
//
//   GET /api/sessions?user=user123
//
// RESPONSE FORMAT:
//
//   {
//     "sessions": [
//       {
//         "name": "user123-firefox-abc",
//         "user": "user123",
//         "template": "firefox",
//         "state": "running",
//         "activeConnections": 2,
//         ...
//       }
//     ],
//     "total": 1
//   }
//
// SECURITY:
//
// - Uses request context for proper timeout and cancellation handling
// - Database enrichment failures are non-fatal (logged but don't block response)
// - Should be paired with authorization middleware to restrict access
//
// ERROR RESPONSES:
//
// - 500 Internal Server Error: Kubernetes API failure
func (h *Handler) ListSessions(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	userID := c.Query("user")

	// Use database as source of truth for multi-platform support
	var dbSessions []*db.Session
	var err error

	if userID != "" {
		dbSessions, err = h.sessionDB.ListSessionsByUser(ctx, userID)
	} else {
		dbSessions, err = h.sessionDB.ListSessions(ctx)
	}

	if err != nil {
		// Fall back to Kubernetes for backward compatibility
		log.Printf("Database session query failed, falling back to k8s: %v", err)
		var k8sSessions []*k8s.Session
		if userID != "" {
			k8sSessions, err = h.k8sClient.ListSessionsByUser(ctx, h.namespace, userID)
		} else {
			k8sSessions, err = h.k8sClient.ListSessions(ctx, h.namespace)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		enriched := h.enrichSessionsWithDBInfo(ctx, k8sSessions)
		c.JSON(http.StatusOK, gin.H{
			"sessions": enriched,
			"total":    len(enriched),
		})
		return
	}

	// Convert database sessions to API response format
	sessions := h.convertDBSessionsToResponse(dbSessions)

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetSession returns a single session by ID
func (h *Handler) GetSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	// Use database as source of truth for multi-platform support
	dbSession, err := h.sessionDB.GetSession(ctx, sessionID)
	if err != nil {
		// Fall back to Kubernetes for backward compatibility
		log.Printf("Database session query failed, falling back to k8s: %v", err)
		k8sSession, k8sErr := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
		if k8sErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		enriched := h.enrichSessionWithDBInfo(ctx, k8sSession)
		c.JSON(http.StatusOK, enriched)
		return
	}

	// Convert to API response format
	session := h.convertDBSessionToResponse(dbSession)
	c.JSON(http.StatusOK, session)
}

// CreateSession creates a new container session for a user.
//
// HTTP Method: POST
// Path: /api/sessions
// Authentication: Required
// Authorization: User can create own sessions; Admin can create for any user
//
// REQUEST BODY:
//   {
//     "user": "user123",                  // REQUIRED: User ID
//     "template": "firefox",               // REQUIRED: Template name
//     "resources": {"memory": "2Gi", "cpu": "1000m"},  // OPTIONAL
//     "persistentHome": true,              // OPTIONAL: Mount persistent storage
//     "idleTimeout": "30m",                // OPTIONAL: Auto-hibernate timeout
//     "maxSessionDuration": "8h",          // OPTIONAL: Maximum lifetime
//     "tags": ["project-a", "dev"]         // OPTIONAL: Organization tags
//   }
//
// SECURITY: Quota Enforcement
//
// This handler enforces resource quotas before creating sessions to prevent:
// - Resource exhaustion attacks (unlimited session creation)
// - Fair usage violations (one user consuming all cluster resources)
// - Cluster instability (out of memory, CPU starvation)
//
// Quota check process:
// 1. Parse and validate requested CPU/memory resources
// 2. Calculate current user resource usage from active pods
// 3. Check if user has quota headroom for new session
// 4. Reject with 403 Forbidden if quota would be exceeded
//
// DATABASE TRANSACTION BOUNDARY:
//
// - No database transaction (Kubernetes is source of truth)
// - Session cached in PostgreSQL after creation (best-effort)
// - Cache failures logged but do NOT block session creation
//
// ERROR RESPONSES:
//
// - 400 Bad Request: Invalid JSON or malformed resource specifications
// - 403 Forbidden: User quota exceeded
// - 404 Not Found: Template does not exist
// - 500 Internal Server Error: Kubernetes API failure
func (h *Handler) CreateSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	var req struct {
		User               string   `json:"user" binding:"required"`
		Template           string   `json:"template"`
		ApplicationId      string   `json:"applicationId"`
		Resources          *struct {
			Memory string `json:"memory"`
			CPU    string `json:"cpu"`
		} `json:"resources"`
		PersistentHome     *bool    `json:"persistentHome"`
		IdleTimeout        string   `json:"idleTimeout"`
		MaxSessionDuration string   `json:"maxSessionDuration"`
		Tags               []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1: Resolve template name from application ID or direct template name
	// If applicationId is provided, look up the application to get the template name
	// This provides better error messages and validation
	templateName := req.Template

	if req.ApplicationId != "" {
		// Look up the installed application in the database
		var appTemplateName, appDisplayName, installStatus, installMessage string
		var enabled bool
		err := h.db.DB().QueryRowContext(ctx, `
			SELECT
				COALESCE(ct.name, '') as template_name,
				ia.display_name,
				ia.enabled,
				COALESCE(ia.install_status, 'unknown') as install_status,
				COALESCE(ia.install_message, '') as install_message
			FROM installed_applications ia
			LEFT JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
			WHERE ia.id = $1
		`, req.ApplicationId).Scan(&appTemplateName, &appDisplayName, &enabled, &installStatus, &installMessage)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Application not found",
				"message": fmt.Sprintf("No application found with ID: %s", req.ApplicationId),
			})
			return
		}

		// Check if the application is enabled
		if !enabled {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Application disabled",
				"message": fmt.Sprintf("The application '%s' is currently disabled", appDisplayName),
			})
			return
		}

		// Check installation status
		if installStatus == "failed" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Application installation failed",
				"message": fmt.Sprintf("The application '%s' failed to install: %s", appDisplayName, installMessage),
			})
			return
		}

		if installStatus == "pending" || installStatus == "creating" {
			// Self-healing: Check if the Template CRD actually exists in Kubernetes
			// This handles cases where the controller created the template but status wasn't updated
			if appTemplateName != "" {
				_, templateErr := h.k8sClient.GetTemplate(ctx, h.namespace, appTemplateName)
				if templateErr == nil {
					// Template exists! Update status in database and continue
					_, updateErr := h.db.DB().ExecContext(ctx, `
						UPDATE installed_applications
						SET install_status = 'installed', install_message = 'Template ready (self-healed)', updated_at = NOW()
						WHERE id = $1
					`, req.ApplicationId)
					if updateErr != nil {
						log.Printf("Failed to update install status for %s: %v", req.ApplicationId, updateErr)
					} else {
						log.Printf("Self-healed application %s status to installed (template found)", req.ApplicationId)
					}
					// Continue with session creation - don't reject
					installStatus = "installed"
				}
			}

			// If still pending after self-healing check, reject
			if installStatus == "pending" || installStatus == "creating" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Application still installing",
					"message": fmt.Sprintf("The application '%s' is still being installed. Please wait and try again.", appDisplayName),
				})
				return
			}
		}

		// Validate template name was found
		if appTemplateName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Application configuration error",
				"message": fmt.Sprintf("The application '%s' does not have a valid template configuration", appDisplayName),
			})
			return
		}

		templateName = appTemplateName
	} else if req.Template == "" {
		// Neither applicationId nor template provided
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"message": "Either 'applicationId' or 'template' must be provided",
		})
		return
	}

	// Step 2: Verify Kubernetes Template CRD exists
	// The template must be created during application installation (see handlers/applications.go)
	// Without a valid template, the session cannot be created
	template, err := h.k8sClient.GetTemplate(ctx, h.namespace, templateName)
	if err != nil {
		// Template is missing - trigger reinstallation if applicationId was provided
		if req.ApplicationId != "" {
			// Query application details for reinstall
			var (
				installID         string
				catalogTemplateID int
				displayName       string
				description       string
				category          string
				iconURL           string
				manifest          string
				installedBy       string
			)
			reinstallErr := h.db.DB().QueryRowContext(ctx, `
				SELECT
					ia.id,
					ia.catalog_template_id,
					ia.display_name,
					COALESCE(ct.description, ''),
					COALESCE(ct.category, ''),
					COALESCE(ct.icon_url, ''),
					COALESCE(ct.manifest, '{}'),
					ia.created_by
				FROM installed_applications ia
				LEFT JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
				WHERE ia.id = $1
			`, req.ApplicationId).Scan(
				&installID, &catalogTemplateID, &displayName, &description,
				&category, &iconURL, &manifest, &installedBy,
			)

			if reinstallErr == nil {
				// Publish AppInstallEvent to trigger controller to create template
				if err := h.publisher.PublishAppInstall(ctx, &events.AppInstallEvent{
					InstallID:         installID,
					CatalogTemplateID: catalogTemplateID,
					TemplateName:      templateName,
					DisplayName:       displayName,
					Description:       description,
					Category:          category,
					IconURL:           iconURL,
					Manifest:          manifest,
					InstalledBy:       installedBy,
					Platform:          h.platform,
				}); err != nil {
					log.Printf("Failed to publish app reinstall event for %s: %v", templateName, err)
				} else {
					log.Printf("Triggered reinstall for missing template %s (app: %s)", templateName, installID)
					// Update status to creating
					h.db.DB().ExecContext(ctx, `
						UPDATE installed_applications
						SET install_status = 'creating', install_message = 'Reinstalling missing template', updated_at = NOW()
						WHERE id = $1
					`, installID)
				}
			}

			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Template reinstalling",
				"message": fmt.Sprintf("The template for '%s' was missing and is being reinstalled. Please try again in a few seconds.", displayName),
			})
			return
		}

		// No applicationId provided - provide generic error
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Template not found: %s. Please ensure the application is properly installed.", templateName),
		})
		return
	}

	// Step 3: Determine resource allocation (memory/CPU)
	// Priority: request > template defaults > system defaults
	memory := "2Gi"   // System default
	cpu := "1000m"    // System default (1 core)
	if req.Resources != nil {
		// User explicitly specified resources
		if req.Resources.Memory != "" {
			memory = req.Resources.Memory
		}
		if req.Resources.CPU != "" {
			cpu = req.Resources.CPU
		}
	} else if template.DefaultResources.Memory != "" || template.DefaultResources.CPU != "" {
		// Fall back to template-defined defaults
		if template.DefaultResources.Memory != "" {
			memory = template.DefaultResources.Memory
		}
		if template.DefaultResources.CPU != "" {
			cpu = template.DefaultResources.CPU
		}
	}

	// Step 4: Validate and parse resource specifications
	// Convert human-readable formats (e.g., "2Gi", "500m") to int64 for quota checking
	requestedCPU, requestedMemory, err := h.quotaEnforcer.ValidateResourceRequest(cpu, memory)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid resource request",
			"message": err.Error(),
		})
		return
	}

	// Step 5: Check user quota before creating session
	// Get current resource usage by listing all pods belonging to this user
	podList, err := h.k8sClient.GetPods(ctx, h.namespace)
	if err != nil {
		log.Printf("Failed to get pods for quota check: %v", err)
		// Continue with empty usage if we can't get pods (fail-open for availability)
		podList = &corev1.PodList{}
	}

	// Filter to only this user's pods based on the "user" label
	userPods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if user, ok := pod.Labels["user"]; ok && user == req.User {
			userPods = append(userPods, pod)
		}
	}

	// Calculate current usage and check if new session would exceed quota
	currentUsage := h.quotaEnforcer.CalculateUsage(userPods)

	if err := h.quotaEnforcer.CheckSessionCreation(ctx, req.User, requestedCPU, requestedMemory, 0, currentUsage); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Quota exceeded",
			"message": err.Error(),
		})
		return
	}

	// Generate session name: {user}-{template}-{random}
	// Use resolved templateName (from applicationId lookup or req.Template)
	sessionName := fmt.Sprintf("%s-%s-%s", req.User, templateName, uuid.New().String()[:8])

	session := &k8s.Session{
		Name:      sessionName,
		Namespace: h.namespace,
		User:      req.User,
		Template:  templateName,
		State:     "running",
	}

	session.Resources.Memory = memory
	session.Resources.CPU = cpu

	if req.PersistentHome != nil {
		session.PersistentHome = *req.PersistentHome
	} else {
		session.PersistentHome = true // Default
	}

	if req.IdleTimeout != "" {
		session.IdleTimeout = req.IdleTimeout
	}

	if req.MaxSessionDuration != "" {
		session.MaxSessionDuration = req.MaxSessionDuration
	}

	if len(req.Tags) > 0 {
		session.Tags = req.Tags
	}

	// v2.0-beta Architecture: Create Session CRD directly and dispatch command to agent
	// 1. Create Session CRD in Kubernetes
	createdSession, err := h.k8sClient.CreateSession(ctx, session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create session CRD",
			"message": fmt.Sprintf("Failed to create Session CRD in Kubernetes: %v", err),
		})
		return
	}
	log.Printf("Created Session CRD: %s in namespace %s", createdSession.Name, createdSession.Namespace)

	// 2. Select an agent to handle this session
	// Query for an online agent (simple strategy: first available)
	var agentID string
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT agent_id FROM agents
		WHERE status = 'online' AND platform = $1
		ORDER BY active_sessions ASC
		LIMIT 1
	`, h.platform).Scan(&agentID)

	if err != nil {
		// No agents available - update Session status to reflect this
		session.Status.Phase = "Failed"
		if updateErr := h.k8sClient.UpdateSessionStatus(ctx, session); updateErr != nil {
			log.Printf("Failed to update Session status after agent selection failure: %v", updateErr)
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "No agents available",
			"message": "No online agents are currently available to handle this session. Please try again later.",
		})
		return
	}
	log.Printf("Selected agent %s for session %s", agentID, sessionName)

	// 3. Build command payload with session and template details
	vncPort := 3000 // Default VNC port
	if template != nil && template.VNC != nil && template.VNC.Port > 0 {
		vncPort = int(template.VNC.Port)
	}

	envMap := make(map[string]string)
	if template != nil {
		for _, env := range template.Env {
			envMap[env.Name] = env.Value
		}
	}

	payload := models.CommandPayload{
		"sessionId":      sessionName,
		"user":           req.User,
		"template":       templateName,
		"namespace":      h.namespace,
		"image":          template.BaseImage,
		"vncPort":        vncPort,
		"memory":         memory,
		"cpu":            cpu,
		"persistentHome": session.PersistentHome,
		"idleTimeout":    session.IdleTimeout,
		"env":            envMap,
	}

	// 4. Create command in database
	commandID := fmt.Sprintf("cmd-%s", uuid.New().String()[:8])
	now := time.Now()
	var command models.AgentCommand
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID, sessionName, "start_session", payload, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&command.ErrorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if err != nil {
		log.Printf("Failed to create agent command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create agent command",
			"message": fmt.Sprintf("Failed to create command in database: %v", err),
		})
		return
	}
	log.Printf("Created agent command %s for session %s", commandID, sessionName)

	// 5. Dispatch command to agent via WebSocket
	if h.dispatcher != nil {
		if err := h.dispatcher.DispatchCommand(&command); err != nil {
			log.Printf("Failed to dispatch command %s: %v", commandID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to dispatch command to agent",
				"message": fmt.Sprintf("Failed to dispatch command: %v", err),
			})
			return
		}
		log.Printf("Dispatched command %s to agent %s for session %s", commandID, agentID, sessionName)
	} else {
		log.Printf("Warning: CommandDispatcher is nil, command %s not dispatched", commandID)
	}

	// Cache session in database so status updates can be applied
	// This is best-effort - failure doesn't block session creation
	dbSession := &db.Session{
		ID:                 sessionName,
		UserID:             req.User,
		TemplateName:       templateName,
		State:              "pending",
		Namespace:          h.namespace,
		Platform:           h.platform,
		Memory:             memory,
		CPU:                cpu,
		PersistentHome:     session.PersistentHome,
		IdleTimeout:        session.IdleTimeout,
		MaxSessionDuration: session.MaxSessionDuration,
	}
	if err := h.sessionDB.CreateSession(ctx, dbSession); err != nil {
		log.Printf("Failed to cache session %s in database (non-fatal): %v", sessionName, err)
	}

	// Return the session info immediately
	// The agent will provision the pod and update the Session CRD status
	response := map[string]interface{}{
		"name":               sessionName,
		"namespace":          h.namespace,
		"user":               req.User,
		"template":           templateName,
		"state":              "pending",
		"persistentHome":     session.PersistentHome,
		"idleTimeout":        session.IdleTimeout,
		"maxSessionDuration": session.MaxSessionDuration,
		"resources": map[string]string{
			"memory": memory,
			"cpu":    cpu,
		},
		"status": map[string]string{
			"phase":   "Pending",
			"message": fmt.Sprintf("Session provisioning in progress (agent: %s, command: %s)", agentID, commandID),
		},
	}

	log.Printf("Session %s created successfully - CRD created, command %s dispatched to agent %s", sessionName, commandID, agentID)
	c.JSON(http.StatusAccepted, response)
}

// UpdateSession updates a session (typically state changes)
func (h *Handler) UpdateSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	var req struct {
		State string `json:"state"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate state
	if req.State != "running" && req.State != "hibernated" && req.State != "terminated" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state. Must be: running, hibernated, or terminated"})
		return
	}

	// Get current session info for the event
	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Publish state change event for controller to handle
	var publishErr error
	switch req.State {
	case "hibernated":
		event := &events.SessionHibernateEvent{
			SessionID: sessionID,
			UserID:    session.User,
			Platform:  h.platform,
		}
		publishErr = h.publisher.PublishSessionHibernate(ctx, event)
	case "running":
		event := &events.SessionWakeEvent{
			SessionID: sessionID,
			UserID:    session.User,
			Platform:  h.platform,
		}
		publishErr = h.publisher.PublishSessionWake(ctx, event)
	case "terminated":
		event := &events.SessionDeleteEvent{
			SessionID: sessionID,
			UserID:    session.User,
			Platform:  h.platform,
		}
		publishErr = h.publisher.PublishSessionDelete(ctx, event)
	}

	if publishErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update session",
			"message": fmt.Sprintf("Failed to publish state change event: %v", publishErr),
		})
		return
	}

	log.Printf("Published session %s event for %s (controller will update resources)", req.State, sessionID)
	c.JSON(http.StatusAccepted, gin.H{
		"name":    sessionID,
		"state":   req.State,
		"message": "State change requested, waiting for controller",
	})
}

// DeleteSession deletes a session
func (h *Handler) DeleteSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	// Verify session exists before deletion and get user info for event
	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Publish session delete event for controller to handle
	deleteEvent := &events.SessionDeleteEvent{
		SessionID: sessionID,
		UserID:    session.User,
		Platform:  h.platform,
	}
	if err := h.publisher.PublishSessionDelete(ctx, deleteEvent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete session",
			"message": fmt.Sprintf("Failed to publish delete event: %v", err),
		})
		return
	}

	log.Printf("Published session delete event for %s (controller will delete resources)", sessionID)
	c.JSON(http.StatusAccepted, gin.H{
		"name":    sessionID,
		"message": "Session deletion requested, waiting for controller",
	})
}

// ConnectSession handles a user connecting to a session
func (h *Handler) ConnectSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")
	userID := c.Query("user")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user parameter required"})
		return
	}

	// Verify session exists
	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Create connection
	conn := &tracker.Connection{
		ID:            uuid.New().String(),
		SessionID:     sessionID,
		UserID:        userID,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
		ConnectedAt:   time.Now(),
		LastHeartbeat: time.Now(),
	}

	if err := h.connTracker.AddConnection(ctx, conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Determine session readiness and URL availability
	sessionUrl := session.Status.URL
	message := "Connection established."
	ready := true

	if session.State == "hibernated" {
		message = "Connection established. Session is waking from hibernation - please wait."
		ready = false
	} else if session.State == "pending" {
		message = "Connection established. Session is starting up - please wait."
		ready = false
	} else if sessionUrl == "" {
		// Session is running but URL not yet available (pod still initializing)
		message = "Connection established. Waiting for session endpoint - please wait."
		ready = false
	}

	c.JSON(http.StatusOK, gin.H{
		"connectionId": conn.ID,
		"sessionUrl":   sessionUrl,
		"state":        session.State,
		"ready":        ready,
		"message":      message,
	})
}

// DisconnectSession handles a user disconnecting from a session
func (h *Handler) DisconnectSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")
	connectionID := c.Query("connectionId")

	if connectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId parameter required"})
		return
	}

	if err := h.connTracker.RemoveConnection(ctx, connectionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	activeConns := h.connTracker.GetConnectionCount(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"message":            "Connection closed",
		"activeConnections":  activeConns,
	})
}

// SessionHeartbeat handles heartbeat pings from active connections
func (h *Handler) SessionHeartbeat(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")
	connectionID := c.Query("connectionId")

	if connectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId parameter required"})
		return
	}

	// Validate that the connection belongs to the specified session
	conn := h.connTracker.GetConnection(connectionID)
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
		return
	}

	if conn.SessionID != sessionID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Connection does not belong to this session"})
		return
	}

	if err := h.connTracker.UpdateHeartbeat(ctx, connectionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetSessionConnections returns active connections for a session
func (h *Handler) GetSessionConnections(c *gin.Context) {
	sessionID := c.Param("id")

	connections := h.connTracker.GetActiveConnections(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"sessionId":    sessionID,
		"connections":  connections,
		"total":        len(connections),
	})
}

// UpdateSessionTags updates tags for a session
func (h *Handler) UpdateSessionTags(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	var req struct {
		Tags []string `json:"tags" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the session first
	obj, err := h.k8sClient.GetDynamicClient().Resource(sessionGVR).Namespace(h.namespace).Get(ctx, sessionID, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Update the tags in spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session spec"})
		return
	}

	spec["tags"] = req.Tags

	// Update the session
	_, err = h.k8sClient.GetDynamicClient().Resource(sessionGVR).Namespace(h.namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the updated session using the k8s client
	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.enrichSessionWithDBInfo(ctx, session))
}

// ListSessionsByTags returns sessions filtered by tags
func (h *Handler) ListSessionsByTags(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	tags := c.QueryArray("tags")

	if len(tags) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one tag is required"})
		return
	}

	// Build label selector for tags
	// Multiple tags are OR'd together
	labelSelectors := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "" {
			labelSelectors = append(labelSelectors, fmt.Sprintf("tag.stream.space/%s=true", tag))
		}
	}

	// Note: Kubernetes label selectors with comma are AND not OR
	// For OR logic, we need to list all sessions and filter in code
	allSessions, err := h.k8sClient.ListSessions(ctx, h.namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter sessions that have any of the requested tags
	filtered := make([]*k8s.Session, 0)
	for _, session := range allSessions {
		for _, sessionTag := range session.Tags {
			for _, requestedTag := range tags {
				if sessionTag == requestedTag {
					filtered = append(filtered, session)
					goto nextSession
				}
			}
		}
	nextSession:
	}

	// Enrich with database info
	enriched := h.enrichSessionsWithDBInfo(ctx, filtered)

	c.JSON(http.StatusOK, gin.H{
		"sessions": enriched,
		"total":    len(enriched),
		"tags":     tags,
	})
}

// ============================================================================
// Template Endpoints
// ============================================================================

// ListTemplates returns all templates with advanced filtering, search, and sorting
func (h *Handler) ListTemplates(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	// Get query parameters
	category := c.Query("category")
	search := c.Query("search")        // Search in name, description, tags
	sortBy := c.Query("sort")          // name, popularity, created (default: name)
	tags := c.QueryArray("tags")       // Filter by tags
	featured := c.Query("featured")    // Filter featured templates

	// Get all templates first
	var templates []*k8s.Template
	var err error

	if category != "" {
		templates, err = h.k8sClient.ListTemplatesByCategory(ctx, h.namespace, category)
	} else {
		templates, err = h.k8sClient.ListTemplates(ctx, h.namespace)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply search filter
	if search != "" {
		filtered := make([]*k8s.Template, 0)
		searchLower := strings.ToLower(search)

		for _, tmpl := range templates {
			// Search in display name
			if strings.Contains(strings.ToLower(tmpl.DisplayName), searchLower) {
				filtered = append(filtered, tmpl)
				continue
			}
			// Search in description
			if strings.Contains(strings.ToLower(tmpl.Description), searchLower) {
				filtered = append(filtered, tmpl)
				continue
			}
			// Search in tags
			for _, tag := range tmpl.Tags {
				if strings.Contains(strings.ToLower(tag), searchLower) {
					filtered = append(filtered, tmpl)
					break
				}
			}
		}
		templates = filtered
	}

	// Apply tag filter
	if len(tags) > 0 {
		filtered := make([]*k8s.Template, 0)
		for _, tmpl := range templates {
			hasAllTags := true
			for _, requiredTag := range tags {
				found := false
				for _, tmplTag := range tmpl.Tags {
					if strings.EqualFold(tmplTag, requiredTag) {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if hasAllTags {
				filtered = append(filtered, tmpl)
			}
		}
		templates = filtered
	}

	// Apply featured filter
	if featured == "true" {
		filtered := make([]*k8s.Template, 0)
		for _, tmpl := range templates {
			if tmpl.Featured {
				filtered = append(filtered, tmpl)
			}
		}
		templates = filtered
	}

	// Sort templates
	switch sortBy {
	case "popularity":
		// Sort by usage count (if tracked)
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].UsageCount > templates[j].UsageCount
		})
	case "created":
		// Sort by creation time (newest first)
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].CreatedAt.After(templates[j].CreatedAt)
		})
	default: // "name" or empty
		// Sort alphabetically by display name
		sort.Slice(templates, func(i, j int) bool {
			return strings.ToLower(templates[i].DisplayName) < strings.ToLower(templates[j].DisplayName)
		})
	}

	// Group templates by category for UI
	categories := make(map[string][]*k8s.Template)
	for _, tmpl := range templates {
		cat := tmpl.Category
		if cat == "" {
			cat = "Other"
		}
		categories[cat] = append(categories[cat], tmpl)
	}

	c.JSON(http.StatusOK, gin.H{
		"templates":  templates,
		"total":      len(templates),
		"categories": categories,
		"filters": gin.H{
			"category": category,
			"search":   search,
			"tags":     tags,
			"sortBy":   sortBy,
			"featured": featured,
		},
	})
}

// GetTemplate returns a single template
func (h *Handler) GetTemplate(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	template, err := h.k8sClient.GetTemplate(ctx, h.namespace, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// CreateTemplate creates a new template (admin only)
func (h *Handler) CreateTemplate(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	var template k8s.Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template.Namespace = h.namespace

	created, err := h.k8sClient.CreateTemplate(ctx, &template)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish template create event for controllers
	createEvent := &events.TemplateCreateEvent{
		TemplateID:  created.Name,
		DisplayName: created.DisplayName,
		Category:    created.Category,
		BaseImage:   created.BaseImage,
		Platform:    h.platform,
	}
	if err := h.publisher.PublishTemplateCreate(ctx, createEvent); err != nil {
		log.Printf("Warning: Failed to publish template create event: %v", err)
	}

	c.JSON(http.StatusCreated, created)
}

// DeleteTemplate deletes a template (admin only)
func (h *Handler) DeleteTemplate(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	if err := h.k8sClient.DeleteTemplate(ctx, h.namespace, templateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish template delete event for controllers
	deleteEvent := &events.TemplateDeleteEvent{
		TemplateName: templateID,
		Platform:     h.platform,
	}
	if err := h.publisher.PublishTemplateDelete(ctx, deleteEvent); err != nil {
		log.Printf("Warning: Failed to publish template delete event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

// AddTemplateFavorite adds a template to the authenticated user's favorites list.
//
// HTTP Method: POST
// Path: /api/templates/{id}/favorite
// Authentication: Required
// Authorization: Any authenticated user
//
// SECURITY: User Context Validation
//
// This handler retrieves user ID from Gin context (populated by auth middleware).
// The authentication middleware MUST run before this handler to ensure:
// - User is authenticated (valid JWT token)
// - User account is active (not disabled)
// - userID context value is set
//
// DATABASE TRANSACTION BOUNDARY:
//
// - Single INSERT query with ON CONFLICT DO NOTHING (idempotent)
// - No explicit transaction needed (single query is atomic)
// - Safe for concurrent calls (unique constraint prevents duplicates)
//
// ERROR RESPONSES:
//
// - 401 Unauthorized: User not authenticated
// - 404 Not Found: Template does not exist
// - 500 Internal Server Error: Database failure
func (h *Handler) AddTemplateFavorite(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	// SECURITY: Get user ID from context (set by auth middleware)
	// This ensures only authenticated users can add favorites
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	// Verify template exists
	_, err := h.k8sClient.GetTemplate(ctx, h.namespace, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	// Add to favorites (INSERT IGNORE if already exists)
	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO user_template_favorites (user_id, template_name)
		VALUES ($1, $2)
		ON CONFLICT (user_id, template_name) DO NOTHING
	`, userIDStr, templateID)

	if err != nil {
		log.Printf("Error adding template to favorites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Template added to favorites",
		"template": templateID,
	})
}

// RemoveTemplateFavorite removes a template from user's favorites
func (h *Handler) RemoveTemplateFavorite(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	// Remove from favorites
	result, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM user_template_favorites
		WHERE user_id = $1 AND template_name = $2
	`, userIDStr, templateID)

	if err != nil {
		log.Printf("Error removing template from favorites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove favorite"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not in favorites"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Template removed from favorites",
		"template": templateID,
	})
}

// ListUserFavoriteTemplates returns user's favorite templates
func (h *Handler) ListUserFavoriteTemplates(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	// Get favorite template names from database
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT template_name, favorited_at
		FROM user_template_favorites
		WHERE user_id = $1
		ORDER BY favorited_at DESC
	`, userIDStr)

	if err != nil {
		log.Printf("Error querying favorites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get favorites"})
		return
	}
	defer rows.Close()

	// Collect template names and timestamps
	type favoriteEntry struct {
		Name        string    `json:"name"`
		FavoritedAt time.Time `json:"favoritedAt"`
	}
	favorites := []favoriteEntry{}
	templateNames := []string{}

	for rows.Next() {
		var entry favoriteEntry
		if err := rows.Scan(&entry.Name, &entry.FavoritedAt); err != nil {
			log.Printf("Error scanning favorite row: %v", err)
			continue
		}
		favorites = append(favorites, entry)
		templateNames = append(templateNames, entry.Name)
	}

	// Fetch full template details from Kubernetes
	templates := make([]*k8s.Template, 0, len(templateNames))
	for _, name := range templateNames {
		template, err := h.k8sClient.GetTemplate(ctx, h.namespace, name)
		if err != nil {
			log.Printf("Warning: Favorite template %s not found in cluster: %v", name, err)
			continue
		}
		templates = append(templates, template)
	}

	// Enrich templates with favorited_at timestamp
	enriched := make([]map[string]interface{}, 0, len(templates))
	for i, tmpl := range templates {
		enrichedTemplate := map[string]interface{}{
			"name":        tmpl.Name,
			"displayName": tmpl.DisplayName,
			"description": tmpl.Description,
			"category":    tmpl.Category,
			"icon":        tmpl.Icon,
			"tags":        tmpl.Tags,
			"favorited":   true,
			"favoritedAt": favorites[i].FavoritedAt,
		}
		enriched = append(enriched, enrichedTemplate)
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": enriched,
		"total":     len(enriched),
	})
}

// CheckTemplateFavorite checks if a template is in user's favorites
func (h *Handler) CheckTemplateFavorite(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	// Check if favorite exists
	var count int
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_template_favorites
		WHERE user_id = $1 AND template_name = $2
	`, userIDStr, templateID).Scan(&count)

	if err != nil {
		log.Printf("Error checking favorite: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"favorited": count > 0,
		"template":  templateID,
	})
}

// ============================================================================
// Catalog Endpoints (Template Marketplace)
// ============================================================================

// ListCatalogTemplates returns templates from the marketplace catalog
func (h *Handler) ListCatalogTemplates(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	category := c.Query("category")
	tag := c.Query("tag")

	query := `
		SELECT ct.id, ct.name, ct.display_name, ct.description, ct.category,
		       ct.icon_url, ct.manifest, ct.tags, ct.install_count,
		       r.name as repository_name, r.url as repository_url
		FROM catalog_templates ct
		JOIN repositories r ON ct.repository_id = r.id
		WHERE r.status = 'synced'
	`

	args := []interface{}{}
	argIdx := 1

	if category != "" {
		query += fmt.Sprintf(" AND ct.category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	if tag != "" {
		query += fmt.Sprintf(" AND $%d = ANY(ct.tags)", argIdx)
		args = append(args, tag)
		argIdx++
	}

	query += " ORDER BY ct.install_count DESC, ct.display_name ASC"

	rows, err := h.db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	templates := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name, displayName, description, category, iconURL, manifest string
		var tags []string
		var installCount int
		var repoName, repoURL string

		if err := rows.Scan(&id, &name, &displayName, &description, &category, &iconURL, &manifest, &tags, &installCount, &repoName, &repoURL); err != nil {
			continue
		}

		templates = append(templates, map[string]interface{}{
			"id":           id,
			"name":         name,
			"displayName":  displayName,
			"description":  description,
			"category":     category,
			"icon":         iconURL,
			"manifest":     manifest,
			"tags":         tags,
			"installCount": installCount,
			"repository": map[string]string{
				"name": repoName,
				"url":  repoURL,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// InstallCatalogTemplate installs a template from the catalog to the Kubernetes cluster.
//
// HTTP Method: POST
// Path: /api/catalog/{id}/install
// Authentication: Required
// Authorization: Admin only (installs cluster-wide resources)
//
// SECURITY: YAML Parsing from External Source
//
// This handler parses YAML manifests from the catalog database, which may originate
// from external repositories. This introduces security risks:
//
// 1. Malicious YAML: Catalog templates may contain crafted YAML to:
//    - Exploit YAML parser vulnerabilities (billion laughs, entity expansion)
//    - Inject malicious container images
//    - Request excessive resources
//    - Escape pod sandboxes
//
// 2. Supply Chain Attacks: If repository is compromised, attacker can:
//    - Modify templates to include backdoors
//    - Inject crypto miners
//    - Exfiltrate data from clusters
//
// MITIGATIONS:
//
// - Validate YAML structure after parsing (check for required fields)
// - Only allow installation by admins (not regular users)
// - Repository sync should validate templates before storing
// - Consider sandboxing template execution (not implemented)
// - Audit log all template installations for forensics
//
// DATABASE TRANSACTION BOUNDARY:
//
// - Two queries: SELECT template, UPDATE install_count
// - No explicit transaction (install_count update is best-effort)
// - Failure to increment counter does NOT fail installation
//
// DATA FLOW:
//
// 1. Retrieve template manifest from database (YAML string)
// 2. Parse YAML to extract spec fields
// 3. Build Template CRD struct from parsed data
// 4. Create Template resource in Kubernetes
// 5. Increment install_count in database (best-effort)
//
// ERROR RESPONSES:
//
// - 400 Bad Request: Invalid YAML manifest structure
// - 404 Not Found: Catalog template not found
// - 500 Internal Server Error: Kubernetes API or database failure
func (h *Handler) InstallCatalogTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	catalogID := c.Param("id")

	// STEP 1: Retrieve template manifest from database
	// Manifest is YAML string parsed from external repository
	var manifest, name, displayName, description, category string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT manifest, name, display_name, description, category
		FROM catalog_templates
		WHERE id = $1
	`, catalogID).Scan(&manifest, &name, &displayName, &description, &category)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog template not found"})
		return
	}

	// Parse the YAML manifest to get template configuration
	// The manifest is stored as YAML, we need to extract key fields
	var templateData map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest), &templateData); err != nil {
		log.Printf("Error parsing template manifest: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid template manifest",
			"message": err.Error(),
		})
		return
	}

	// Build Template struct from manifest
	template := &k8s.Template{
		Name:        name,
		Namespace:   h.namespace,
		DisplayName: displayName,
		Description: description,
		Category:    category,
	}

	// Extract spec fields if they exist in the manifest
	if spec, ok := templateData["spec"].(map[string]interface{}); ok {
		if baseImage, ok := spec["baseImage"].(string); ok {
			template.BaseImage = baseImage
		}
		if icon, ok := spec["icon"].(string); ok {
			template.Icon = icon
		}
		if appType, ok := spec["appType"].(string); ok {
			template.AppType = appType
		}
		if defaultRes, ok := spec["defaultResources"].(map[string]interface{}); ok {
			if memory, ok := defaultRes["memory"].(string); ok {
				template.DefaultResources.Memory = memory
			}
			if cpu, ok := defaultRes["cpu"].(string); ok {
				template.DefaultResources.CPU = cpu
			}
		}
		if tags, ok := spec["tags"].([]interface{}); ok {
			template.Tags = make([]string, 0, len(tags))
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					template.Tags = append(template.Tags, tagStr)
				}
			}
		}
		if capabilities, ok := spec["capabilities"].([]interface{}); ok {
			template.Capabilities = make([]string, 0, len(capabilities))
			for _, cap := range capabilities {
				if capStr, ok := cap.(string); ok {
					template.Capabilities = append(template.Capabilities, capStr)
				}
			}
		}
	}

	// Create Template CRD in Kubernetes
	createdTemplate, err := h.k8sClient.CreateTemplate(ctx, template)
	if err != nil {
		log.Printf("Error creating template in Kubernetes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to install template",
			"message": err.Error(),
		})
		return
	}

	// Increment install count (best effort, don't fail the request if this fails)
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE catalog_templates SET install_count = install_count + 1 WHERE id = $1
	`, catalogID)
	if err != nil {
		// Log error but don't fail the request - install count is not critical
		log.Printf("Warning: Failed to increment install count for template %s: %v", catalogID, err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Template installed successfully",
		"template": createdTemplate,
		"name":     createdTemplate.Name,
		"namespace": createdTemplate.Namespace,
	})
}

// ============================================================================
// Repository Endpoints (Template Repository Management)
// ============================================================================

// ListRepositories returns all template and plugin repositories
func (h *Handler) ListRepositories(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), url, COALESCE(branch, 'main'), COALESCE(type, 'template'), COALESCE(auth_type, 'none'), last_sync, COALESCE(template_count, 0), COALESCE(status, 'pending'), error_message, created_at, updated_at
		FROM repositories
		ORDER BY name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	repos := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name, url, branch, repoType, authType, status string
		var lastSync sql.NullTime
		var errorMessage sql.NullString
		var createdAt, updatedAt time.Time
		var templateCount int

		if err := rows.Scan(&id, &name, &url, &branch, &repoType, &authType, &lastSync, &templateCount, &status, &errorMessage, &createdAt, &updatedAt); err != nil {
			continue
		}

		repo := map[string]interface{}{
			"id":            id,
			"name":          name,
			"url":           url,
			"branch":        branch,
			"type":          repoType,
			"authType":      authType,
			"templateCount": templateCount,
			"status":        status,
			"createdAt":     createdAt,
			"updatedAt":     updatedAt,
		}

		if lastSync.Valid {
			repo["lastSync"] = lastSync.Time
		} else {
			repo["lastSync"] = nil
		}

		if errorMessage.Valid {
			repo["errorMessage"] = errorMessage.String
		} else {
			repo["errorMessage"] = ""
		}

		repos = append(repos, repo)
	}

	c.JSON(http.StatusOK, gin.H{
		"repositories": repos,
		"total":        len(repos),
	})
}

// AddRepository adds a new template repository
func (h *Handler) AddRepository(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	var req struct {
		Name       string `json:"name" binding:"required"`
		URL        string `json:"url" binding:"required"`
		Branch     string `json:"branch"`
		AuthType   string `json:"authType"`
		AuthSecret string `json:"authSecret"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Branch == "" {
		req.Branch = "main"
	}

	if req.AuthType == "" {
		req.AuthType = "none"
	}

	result, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO repositories (name, url, branch, auth_type, auth_secret, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
	`, req.Name, req.URL, req.Branch, req.AuthType, req.AuthSecret)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get repository ID"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      id,
		"message": "Repository added. Sync will begin shortly.",
	})

	// Trigger repository sync in background
	go func() {
		syncCtx := context.Background()
		if err := h.syncService.SyncRepository(syncCtx, int(id)); err != nil {
			log.Printf("Background sync failed for repository %d: %v", id, err)
		} else {
			log.Printf("Background sync completed for repository %d", id)
		}
	}()
}

// SyncRepository triggers a sync for a repository
func (h *Handler) SyncRepository(c *gin.Context) {
	repoIDStr := c.Param("id")

	// Convert repo ID to int
	var repoID int
	if _, err := fmt.Sscanf(repoIDStr, "%d", &repoID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	// Trigger sync in background
	// BUG FIX: Use context.Background() for goroutine - request context will be cancelled when HTTP request completes
	go func() {
		syncCtx := context.Background()
		if err := h.syncService.SyncRepository(syncCtx, repoID); err != nil {
			log.Printf("Repository sync failed for ID %d: %v", repoID, err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": fmt.Sprintf("Sync triggered for repository %d", repoID),
		"status":  "syncing",
	})
}

// DeleteRepository deletes a template repository
func (h *Handler) DeleteRepository(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	repoID := c.Param("id")

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM repositories WHERE id = $1
	`, repoID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Repository deleted"})
}

// ============================================================================
// Helper Methods
// ============================================================================

// enrichSessionsWithDBInfo enriches multiple sessions with database information.
//
// This helper merges Kubernetes session data with database-cached metadata:
// - Active connection count from connection tracker
// - Additional metadata from database cache
//
// PERFORMANCE:
//
// - Calls enrichSessionWithDBInfo for each session (N queries for N sessions)
// - Could be optimized with batch query if needed
// - Current implementation prioritizes code simplicity
//
// CONCURRENCY:
//
// - Safe for concurrent use (each request has own context)
// - Connection tracker uses internal locking
func (h *Handler) enrichSessionsWithDBInfo(ctx context.Context, sessions []*k8s.Session) []map[string]interface{} {
	enriched := make([]map[string]interface{}, 0, len(sessions))

	for _, session := range sessions {
		enriched = append(enriched, h.enrichSessionWithDBInfo(ctx, session))
	}

	return enriched
}

// enrichSessionWithDBInfo enriches a single session with database information.
//
// Combines Kubernetes session data with real-time connection tracking:
// - Session fields from Kubernetes CRD (name, state, resources)
// - Active connection count from connection tracker
//
// This provides a complete view of session state for API clients without
// requiring multiple requests.
//
// ERROR HANDLING:
//
// - Database errors are non-fatal (connection count defaults to 0)
// - Always returns a valid response even if enrichment fails
func (h *Handler) enrichSessionWithDBInfo(ctx context.Context, session *k8s.Session) map[string]interface{} {
	result := map[string]interface{}{
		"name":               session.Name,
		"namespace":          session.Namespace,
		"user":               session.User,
		"template":           session.Template,
		"state":              session.State,
		"persistentHome":     session.PersistentHome,
		"idleTimeout":        session.IdleTimeout,
		"maxSessionDuration": session.MaxSessionDuration,
		"tags":               session.Tags,
		"status":             session.Status,
		"createdAt":          session.CreatedAt,
	}

	if session.Resources.Memory != "" || session.Resources.CPU != "" {
		result["resources"] = map[string]string{
			"memory": session.Resources.Memory,
			"cpu":    session.Resources.CPU,
		}
	}

	// Get active connections count
	activeConns := h.connTracker.GetConnectionCount(session.Name)
	result["activeConnections"] = activeConns

	return result
}

// convertDBSessionsToResponse converts database sessions to API response format.
func (h *Handler) convertDBSessionsToResponse(sessions []*db.Session) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, h.convertDBSessionToResponse(session))
	}
	return result
}

// convertDBSessionToResponse converts a database session to API response format.
// If the database doesn't have the session URL, it fetches the status from Kubernetes.
func (h *Handler) convertDBSessionToResponse(session *db.Session) map[string]interface{} {
	// Fetch Kubernetes status if database is missing URL or phase is empty
	// This handles the case where the controller hasn't yet communicated status back to API
	url := session.URL
	podName := session.PodName
	phase := session.State

	if (url == "" || phase == "") && h.k8sClient != nil {
		ctx := context.Background()
		k8sSession, err := h.k8sClient.GetSession(ctx, h.namespace, session.ID)
		if err == nil && k8sSession != nil {
			if k8sSession.Status.URL != "" {
				url = k8sSession.Status.URL
			}
			if k8sSession.Status.PodName != "" {
				podName = k8sSession.Status.PodName
			}
			if k8sSession.Status.Phase != "" {
				phase = k8sSession.Status.Phase
			}
			// Also update resources from Kubernetes if missing
			if session.Memory == "" && k8sSession.Resources.Memory != "" {
				session.Memory = k8sSession.Resources.Memory
			}
			if session.CPU == "" && k8sSession.Resources.CPU != "" {
				session.CPU = k8sSession.Resources.CPU
			}
		}
	}

	// Capitalize phase for status.phase (UI expects "Running" not "running")
	capitalizedPhase := phase
	if len(phase) > 0 {
		capitalizedPhase = strings.ToUpper(phase[:1]) + phase[1:]
	}

	result := map[string]interface{}{
		"name":               session.ID,
		"namespace":          session.Namespace,
		"user":               session.UserID,
		"template":           session.TemplateName,
		"state":              session.State,
		"persistentHome":     session.PersistentHome,
		"idleTimeout":        session.IdleTimeout,
		"maxSessionDuration": session.MaxSessionDuration,
		"createdAt":          session.CreatedAt,
		"platform":           session.Platform,
		"activeConnections":  session.ActiveConnections,
		"status": map[string]interface{}{
			"phase":   capitalizedPhase,
			"url":     url,
			"podName": podName,
		},
	}

	if session.Memory != "" || session.CPU != "" {
		result["resources"] = map[string]string{
			"memory": session.Memory,
			"cpu":    session.CPU,
		}
	}

	if session.LastActivity != nil {
		result["status"].(map[string]interface{})["lastActivity"] = session.LastActivity
	}

	return result
}

// cacheSessionInDB caches a session in the PostgreSQL database.
//
// DATABASE TRANSACTION BOUNDARY:
//
// - Single UPSERT query (INSERT ... ON CONFLICT DO UPDATE)
// - No explicit transaction needed (single query is atomic)
// - Idempotent: Safe to call multiple times with same session
//
// CACHE STRATEGY:
//
// Kubernetes is the source of truth for sessions. The database cache:
// - Improves query performance (faster than Kubernetes API)
// - Enables complex queries (search, filtering, aggregation)
// - Provides metadata not in Kubernetes (connection count, analytics)
//
// IMPORTANT: Cache updates are best-effort. Callers should:
// - Log errors but NOT fail the request on cache failures
// - Kubernetes state is authoritative, database is supplementary
//
// UPSERT BEHAVIOR:
//
// ON CONFLICT (id) DO UPDATE ensures idempotency:
// - If session doesn't exist: INSERT new row
// - If session exists: UPDATE existing row with new values
// - No error if called multiple times
//
// ERROR HANDLING:
//
// Returns error on database failure, but callers typically ignore it:
//   if err := h.cacheSessionInDB(ctx, session); err != nil {
//       log.Printf("Cache update failed (non-fatal): %v", err)
//   }
func (h *Handler) cacheSessionInDB(ctx context.Context, session *k8s.Session) error {
	dbSession := &db.Session{
		ID:                 session.Name,
		UserID:             session.User,
		TemplateName:       session.Template,
		State:              session.State,
		AppType:            "desktop",
		Namespace:          session.Namespace,
		Platform:           h.platform,
		URL:                session.Status.URL,
		PodName:            session.Status.PodName,
		Memory:             session.Resources.Memory,
		CPU:                session.Resources.CPU,
		PersistentHome:     session.PersistentHome,
		IdleTimeout:        session.IdleTimeout,
		MaxSessionDuration: session.MaxSessionDuration,
		CreatedAt:          session.CreatedAt,
		LastActivity:       session.Status.LastActivity,
	}

	return h.sessionDB.CreateSession(ctx, dbSession)
}

// updateSessionInDB updates a cached session in the database.
//
// DATABASE TRANSACTION BOUNDARY:
//
// - Single UPDATE query
// - No explicit transaction needed
// - Updates state, URL, and timestamp
//
// CACHE CONSISTENCY:
//
// This method updates only fields that change during session lifecycle:
// - state: running → hibernated → terminated
// - url: Updated when session endpoint changes
// - updated_at: Timestamp of last modification
//
// Other fields (user, template, namespace) are immutable and not updated.
//
// ERROR HANDLING:
//
// - Returns error if session not found or database failure
// - Callers typically log and ignore errors (best-effort caching)
func (h *Handler) updateSessionInDB(ctx context.Context, session *k8s.Session) error {
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET state = $1, url = $2, updated_at = $3
		WHERE id = $4
	`, session.State, session.Status.URL, time.Now(), session.Name)

	return err
}

// deleteSessionFromDB removes a session from the database cache.
//
// DATABASE TRANSACTION BOUNDARY:
//
// - Single DELETE query
// - No explicit transaction needed
// - Idempotent: Safe to call even if session doesn't exist
//
// CLEANUP STRATEGY:
//
// When a session is deleted from Kubernetes, we also remove it from
// the database cache to prevent stale data.
//
// CASCADE BEHAVIOR:
//
// Database schema may have CASCADE DELETE for related tables:
// - session_connections (active connections)
// - session_snapshots (saved states)
// - audit_logs (may be preserved)
//
// Check database schema for exact CASCADE behavior.
//
// ERROR HANDLING:
//
// - Returns error on database failure
// - Callers typically log and ignore (best-effort cleanup)
// - Stale cache entries cleaned up by periodic garbage collection
func (h *Handler) deleteSessionFromDB(ctx context.Context, sessionID string) error {
	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM sessions WHERE id = $1
	`, sessionID)

	return err
}
