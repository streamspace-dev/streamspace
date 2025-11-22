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
	"encoding/json"
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
	"github.com/streamspace-dev/streamspace/api/internal/services"
	"github.com/streamspace-dev/streamspace/api/internal/sync"
	"github.com/streamspace-dev/streamspace/api/internal/tracker"
	"github.com/streamspace-dev/streamspace/api/internal/websocket"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// sessionGVR defines the GroupVersionResource for Session custom resources.
//
// This is used with Kubernetes dynamic client to directly manipulate Session CRDs
// when the strongly-typed client is not sufficient (e.g., updating tags field).
//
// Format: {group}/{version}/namespaces/{namespace}/{resource}
// Example: stream.space/v1alpha1/namespaces/streamspace/sessions
const (
	// DefaultNamespace is the default Kubernetes namespace for resources
	// v2.0-beta: API doesn't create K8s resources, but passes namespace to agent in payloads
	DefaultNamespace = "streamspace"
)

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
	templateDB     *db.TemplateDB               // Template database operations
	agentSelector  *services.AgentSelector      // Agent selection for multi-agent routing
	k8sClient      *k8s.Client                  // OPTIONAL: K8s client for cluster management endpoints only
	namespace      string                       // OPTIONAL: K8s namespace for cluster management
	publisher      *events.Publisher            // DEPRECATED: NATS event publisher (stub, no-op)
	dispatcher     CommandDispatcher            // Command dispatcher for agent WebSocket commands
	connTracker    *tracker.ConnectionTracker   // Active connection tracking
	syncService    *sync.SyncService            // Repository synchronization
	wsManager      *websocket.Manager           // WebSocket connection manager
	quotaEnforcer  *quota.Enforcer              // Resource quota enforcement
	platform       string                       // Target platform (kubernetes, docker, etc.)
}

// NOTE ON K8S CLIENT (v2.0-beta):
//
// The k8sClient and namespace fields are OPTIONAL and ONLY used for cluster management
// admin endpoints (ListNodes, ListPods, GetMetrics, etc.).
//
// Session and template operations NEVER use k8sClient - they use database + agent pattern.
// When API runs outside Kubernetes cluster, k8sClient is nil and cluster management
// endpoints return stub data or "not available" responses.

// CommandDispatcher interface for dispatching commands to agents
type CommandDispatcher interface {
	DispatchCommand(command *models.AgentCommand) error
}

// NewHandler creates a new API handler with injected dependencies.
//
// PARAMETERS:
//
// - database: PostgreSQL database connection for caching and metadata
// - publisher: NATS event publisher for platform-agnostic operations
// - dispatcher: Command dispatcher for sending commands to agents
// - connTracker: Connection tracker for active session monitoring
// - syncService: Service for syncing external template repositories
// - wsManager: Manager for WebSocket connections and real-time updates
// - quotaEnforcer: Enforcer for validating resource quotas
// - platform: Target platform (kubernetes, docker, hyperv, vcenter)
// - agentHub: Agent hub for tracking connected agents (required for multi-agent routing)
// - k8sClient: OPTIONAL Kubernetes client for cluster management endpoints (can be nil)
//
// v2.0-beta ARCHITECTURE:
//
// Session and template operations use database + agent pattern (NO K8s dependencies).
// The k8sClient parameter is OPTIONAL and only used for cluster management admin
// endpoints (ListNodes, ListPods, GetMetrics, etc.). When nil, these endpoints
// return stub data.
//
// MULTI-AGENT ROUTING:
//
// The agentHub is required to enable multi-agent routing and load balancing.
// AgentSelector uses agentHub to check which agents are connected and healthy
// before routing session creation requests.
//
// NAMESPACE RESOLUTION:
//
// The Kubernetes namespace is read from NAMESPACE environment variable.
// If not set, defaults to "streamspace". Only used when k8sClient is provided.
//
// EXAMPLE USAGE:
//
//   // API running in Kubernetes with cluster management
//   handler := NewHandler(db, publisher, dispatcher, connTracker, syncService, wsManager, quotaEnforcer, "kubernetes", agentHub, k8sClient)
//
//   // API running standalone (no K8s dependencies)
//   handler := NewHandler(db, publisher, dispatcher, connTracker, syncService, wsManager, quotaEnforcer, "kubernetes", agentHub, nil)
func NewHandler(database *db.Database, publisher *events.Publisher, dispatcher CommandDispatcher, connTracker *tracker.ConnectionTracker, syncService *sync.SyncService, wsManager *websocket.Manager, quotaEnforcer *quota.Enforcer, platform string, agentHub *websocket.AgentHub, k8sClient *k8s.Client) *Handler {
	if platform == "" {
		platform = events.PlatformKubernetes // Default platform
	}

	// Read namespace from environment (only used when k8sClient is provided)
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Create AgentSelector for multi-agent routing and load balancing
	agentSelector := services.NewAgentSelector(database.DB(), agentHub)

	return &Handler{
		db:            database,
		sessionDB:     db.NewSessionDB(database.DB()),
		templateDB:    db.NewTemplateDB(database),
		agentSelector: agentSelector,
		k8sClient:     k8sClient, // Can be nil for standalone API
		namespace:     namespace,
		publisher:     publisher,
		dispatcher:    dispatcher,
		connTracker:   connTracker,
		syncService:   syncService,
		wsManager:     wsManager,
		quotaEnforcer: quotaEnforcer,
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
		// v2.0-beta: Database is source of truth, no K8s fallback
		log.Printf("Database session query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list sessions",
			"message": fmt.Sprintf("Database error: %v", err),
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

	// v2.0-beta: Database is source of truth for all session data
	dbSession, err := h.sessionDB.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("Session %s not found in database: %v", sessionID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"message": fmt.Sprintf("No session found with ID: %s", sessionID),
		})
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
			// v2.0-beta: Check if template exists in database (catalog_templates)
			// This handles cases where the template was synced but status wasn't updated
			if appTemplateName != "" {
				_, templateErr := h.templateDB.GetTemplateByName(ctx, appTemplateName)
				if templateErr == nil {
					// Template exists in database! Update status and continue
					_, updateErr := h.db.DB().ExecContext(ctx, `
						UPDATE installed_applications
						SET install_status = 'installed', install_message = 'Template ready (self-healed)', updated_at = NOW()
						WHERE id = $1
					`, req.ApplicationId)
					if updateErr != nil {
						log.Printf("Failed to update install status for %s: %v", req.ApplicationId, updateErr)
					} else {
						log.Printf("Self-healed application %s status to installed (template found in database)", req.ApplicationId)
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

	// Step 2: v2.0-beta - Fetch template from database (catalog_templates)
	// API includes full template manifest in command payload for agent
	template, err := h.templateDB.GetTemplateByName(ctx, templateName)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Template not found",
			"message": fmt.Sprintf("Template '%s' does not exist in the catalog", templateName),
		})
		return
	}
	if err != nil {
		log.Printf("Failed to fetch template %s: %v", templateName, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch template",
			"message": fmt.Sprintf("Database error: %v", err),
		})
		return
	}
	log.Printf("Fetched template %s from database (ID: %d)", template.Name, template.ID)

	// v2.0-beta FIX: Ensure template manifest is valid for agent
	// If manifest is empty/invalid, construct a basic Template CRD spec
	if len(template.Manifest) == 0 {
		log.Printf("Warning: Template %s has empty manifest, constructing basic Template CRD", template.Name)
		// Create a minimal valid Template CRD manifest
		basicManifest := map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Template",
			"metadata": map[string]interface{}{
				"name":      template.Name,
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"displayName": template.DisplayName,
				"description": template.Description,
				"category":    template.Category,
				"appType":     template.AppType,
				// Use a sensible default image for testing if we don't have one
				"baseImage": "lscr.io/linuxserver/firefox:latest",
				"ports": []map[string]interface{}{
					{
						"name":          "vnc",
						"containerPort": 3000,
						"protocol":      "TCP",
					},
				},
				"defaultResources": map[string]interface{}{
					"memory": "2Gi",
					"cpu":    "1000m",
				},
			},
		}
		manifestJSON, err := json.Marshal(basicManifest)
		if err != nil {
			log.Printf("Failed to marshal basic manifest: %v", err)
		} else {
			template.Manifest = manifestJSON
			log.Printf("Constructed basic manifest for template %s", template.Name)
		}
	}

	// Step 3: Determine resource allocation (memory/CPU)
	// Priority: request > system defaults
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
	// v2.0-beta: Query DATABASE for current usage (NOT Kubernetes directly)
	// The database is the source of truth for session resource tracking
	sessions, err := h.sessionDB.ListSessionsByUser(ctx, req.User)
	if err != nil {
		log.Printf("Failed to get sessions for quota check: %v", err)
		// Continue with empty usage if we can't get sessions (fail-open for availability)
		sessions = []*db.Session{}
	}

	// Calculate current usage from database sessions
	// Only count sessions in active states (running, starting, hibernated, waking)
	var activeSessionCount int
	var totalCPU, totalMemory int64
	for _, session := range sessions {
		if session.State == "running" || session.State == "starting" || session.State == "hibernated" || session.State == "waking" {
			activeSessionCount++

			// Parse CPU and memory from session
			if session.CPU != "" {
				sessionCPU, err := quota.ParseResourceQuantity(session.CPU, "cpu")
				if err == nil {
					totalCPU += sessionCPU
				}
			}
			if session.Memory != "" {
				sessionMemory, err := quota.ParseResourceQuantity(session.Memory, "memory")
				if err == nil {
					totalMemory += sessionMemory
				}
			}
		}
	}

	currentUsage := &quota.Usage{
		ActiveSessions: activeSessionCount,
		TotalCPU:       totalCPU,
		TotalMemory:    totalMemory,
		TotalStorage:   0, // TODO: Calculate from PVC data in database
		TotalGPU:       0, // TODO: Add GPU tracking to sessions table
	}

	// Check if new session would exceed quota
	if err := h.quotaEnforcer.CheckSessionCreation(ctx, req.User, requestedCPU, requestedMemory, 0, currentUsage); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Quota exceeded",
			"message": err.Error(),
		})
		return
	}

	// Step 5: Generate session name: {user}-{template}-{random}
	// Use resolved templateName (from applicationId lookup or req.Template)
	sessionName := fmt.Sprintf("%s-%s-%s", req.User, templateName, uuid.New().String()[:8])

	// Step 6: Determine session configuration
	persistentHome := true // Default
	if req.PersistentHome != nil {
		persistentHome = *req.PersistentHome
	}

	idleTimeout := req.IdleTimeout
	maxSessionDuration := req.MaxSessionDuration
	tags := req.Tags

	// Step 7: v2.0-beta - Select an agent to handle this session using AgentSelector
	// AgentSelector implements intelligent routing with:
	//   - Load balancing (by active session count)
	//   - Health filtering (only online agents with WebSocket connections)
	//   - Platform filtering (kubernetes, docker, etc.)
	//   - Optional cluster/region affinity
	criteria := &services.SelectionCriteria{
		Platform:         h.platform,
		PreferLowLoad:    true,
		RequireConnected: true,
	}

	selectedAgent, err := h.agentSelector.SelectAgent(ctx, criteria)
	if err != nil {
		// No agents available - v2.0-beta: Just return error, no K8s updates
		log.Printf("No agents available for session %s: %v", sessionName, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "No agents available",
			"message": fmt.Sprintf("No online agents are currently available: %v", err),
		})
		return
	}

	agentID := selectedAgent.AgentID
	clusterID := selectedAgent.ClusterID

	log.Printf("Selected agent %s (cluster: %s, load: %d sessions) for session %s",
		agentID, clusterID, selectedAgent.SessionCount, sessionName)

	// Step 8: Create session in DATABASE first (source of truth for v2.0-beta)
	dbSession := &db.Session{
		ID:                 sessionName,
		UserID:             req.User,
		TemplateName:       templateName,
		State:              "pending",
		Namespace:          DefaultNamespace,
		Platform:           h.platform,
		AgentID:            agentID,    // v2.0-beta: Track which agent is managing this session
		ClusterID:          clusterID,  // v2.0-beta: Track which cluster the session runs on
		Memory:             memory,
		CPU:                cpu,
		PersistentHome:     persistentHome,
		IdleTimeout:        idleTimeout,
		MaxSessionDuration: maxSessionDuration,
	}
	if err := h.sessionDB.CreateSession(ctx, dbSession); err != nil {
		log.Printf("Failed to create session %s in database: %v", sessionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create session",
			"message": fmt.Sprintf("Failed to create session in database: %v", err),
		})
		return
	}
	log.Printf("Created session %s in database with state=pending", sessionName)

	// Step 9: Build command payload
	// v2.0-beta: Include full template manifest in payload (agent doesn't fetch from K8s)
	payload := models.CommandPayload{
		"sessionId":           sessionName,
		"user":                req.User,
		"template":            templateName,
		"templateManifest":    template.Manifest, // Full Template CRD spec from database
		"namespace":           DefaultNamespace, // TODO: Remove (agent determines namespace from config)
		"memory":              memory,
		"cpu":                 cpu,
		"persistentHome":      persistentHome,
		"idleTimeout":         idleTimeout,
		"maxSessionDuration":  maxSessionDuration,
		"tags":                tags,
	}

	// 4. Create command in database
	commandID := fmt.Sprintf("cmd-%s", uuid.New().String()[:8])
	now := time.Now()

	// Marshal payload to JSON for database insertion (JSONB column)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal command payload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create command payload",
			"message": fmt.Sprintf("Failed to marshal payload: %v", err),
		})
		return
	}

	var command models.AgentCommand
	var errorMessage sql.NullString // Handle NULL error_message column
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID, sessionName, "start_session", payloadJSON, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&errorMessage, // Scan NULL-able column into sql.NullString
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	// Assign error_message if it's not NULL
	if errorMessage.Valid {
		command.ErrorMessage = &errorMessage.String
	}

	if err != nil {
		log.Printf("Failed to create agent command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create agent command",
			"message": fmt.Sprintf("Failed to create command in database: %v", err),
		})
		return
	}
	log.Printf("Created agent command %s for session %s", commandID, sessionName)

	// Step 10: Dispatch command to agent via WebSocket
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

	// Step 11: Return the session info immediately
	// Agent will create K8s resources (Deployment, Service, Session CRD) and update database
	response := map[string]interface{}{
		"name":               sessionName,
		"namespace":          DefaultNamespace,
		"user":               req.User,
		"template":           templateName,
		"state":              "pending",
		"persistentHome":     persistentHome,
		"idleTimeout":        idleTimeout,
		"maxSessionDuration": maxSessionDuration,
		"resources": map[string]string{
			"memory": memory,
			"cpu":    cpu,
		},
		"status": map[string]string{
			"phase":   "Pending",
			"message": fmt.Sprintf("Session provisioning in progress (agent: %s, command: %s)", agentID, commandID),
		},
		"tags": tags,
	}

	log.Printf("Session %s created successfully - saved to database, command %s dispatched to agent %s", sessionName, commandID, agentID)
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

	// v2.0-beta: Get current session info from database (no K8s access)
	session, err := h.sessionDB.GetSession(ctx, sessionID)
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
			UserID:    session.UserID,
			Platform:  h.platform,
		}
		publishErr = h.publisher.PublishSessionHibernate(ctx, event)
	case "running":
		event := &events.SessionWakeEvent{
			SessionID: sessionID,
			UserID:    session.UserID,
			Platform:  h.platform,
		}
		publishErr = h.publisher.PublishSessionWake(ctx, event)
	case "terminated":
		event := &events.SessionDeleteEvent{
			SessionID: sessionID,
			UserID:    session.UserID,
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

	// 1. Verify session exists in DATABASE and get agent managing it
	// v2.0-beta: API does NOT access Kubernetes directly - agent handles ALL K8s operations
	var agentID sql.NullString // Use sql.NullString for nullable column
	var currentState string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT agent_id, state FROM sessions WHERE id = $1
	`, sessionID).Scan(&agentID, &currentState)

	if err == sql.ErrNoRows {
		log.Printf("Session %s not found in database", sessionID)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"message": "The specified session does not exist",
		})
		return
	}

	if err != nil {
		log.Printf("Failed to query session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to query session",
			"message": fmt.Sprintf("Database error: %v", err),
		})
		return
	}

	// Check if session has an agent assigned
	if !agentID.Valid || agentID.String == "" {
		log.Printf("Session %s has no agent assigned (agent_id is NULL or empty)", sessionID)
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Session not ready",
			"message": "Session has no agent assigned - cannot terminate. Session may still be pending or failed to start.",
		})
		return
	}

	// Check if session is already terminating or terminated
	if currentState == "terminating" || currentState == "terminated" {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Session already terminating",
			"message": fmt.Sprintf("Session is already in %s state", currentState),
		})
		return
	}

	// 2. Create stop_session command in database
	commandID := fmt.Sprintf("cmd-%s", uuid.New().String()[:8])
	now := time.Now()
	payload := map[string]interface{}{
		"sessionId": sessionID,
		"namespace": DefaultNamespace,
	}

	// Marshal payload to JSON for database insertion (JSONB column)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal stop command payload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create stop command",
			"message": fmt.Sprintf("Failed to marshal payload: %v", err),
		})
		return
	}

	var command models.AgentCommand
	var errorMessage sql.NullString
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID.String, sessionID, "stop_session", payloadJSON, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&errorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if errorMessage.Valid {
		command.ErrorMessage = &errorMessage.String
	}

	if err != nil {
		log.Printf("Failed to create stop_session command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create stop command",
			"message": fmt.Sprintf("Failed to create command in database: %v", err),
		})
		return
	}
	log.Printf("Created stop_session command %s for session %s", commandID, sessionID)

	// 3. Update database session state to terminating
	// Agent will update CRD when it processes the command
	if err := h.sessionDB.UpdateSessionState(ctx, sessionID, "terminating"); err != nil {
		log.Printf("Failed to update database session state (non-fatal): %v", err)
	}

	// 4. Dispatch command to agent via WebSocket
	if h.dispatcher != nil {
		if err := h.dispatcher.DispatchCommand(&command); err != nil {
			log.Printf("Failed to dispatch stop command %s: %v", commandID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to dispatch stop command",
				"message": fmt.Sprintf("Failed to dispatch command to agent: %v", err),
			})
			return
		}
		log.Printf("Dispatched stop_session command %s to agent %s for session %s", commandID, agentID.String, sessionID)
	} else {
		log.Printf("Warning: CommandDispatcher is nil, stop command %s not dispatched", commandID)
	}

	// Return accepted response
	// Agent will handle ALL Kubernetes operations (delete Deployment, Service, update CRD)
	c.JSON(http.StatusAccepted, gin.H{
		"name":      sessionID,
		"commandId": commandID,
		"message":   "Session termination requested, agent will delete resources",
	})
}

// HibernateSession handles hibernating a running session (scales to 0 replicas)
func (h *Handler) HibernateSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	// 1. Verify session exists in DATABASE and get agent managing it
	// v2.0-beta: API does NOT access Kubernetes directly - agent handles ALL K8s operations
	var agentID sql.NullString // Use sql.NullString for nullable column
	var currentState string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT agent_id, state FROM sessions WHERE id = $1
	`, sessionID).Scan(&agentID, &currentState)

	if err == sql.ErrNoRows {
		log.Printf("Session %s not found in database", sessionID)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"message": "The specified session does not exist",
		})
		return
	}

	if err != nil {
		log.Printf("Failed to query session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to query session",
			"message": fmt.Sprintf("Database error: %v", err),
		})
		return
	}

	// Check if session has an agent assigned
	if !agentID.Valid || agentID.String == "" {
		log.Printf("Session %s has no agent assigned (agent_id is NULL or empty)", sessionID)
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Session not ready",
			"message": "Session has no agent assigned - cannot hibernate. Session may still be pending or failed to start.",
		})
		return
	}

	// Check if session is in a state that can be hibernated
	if currentState != "running" {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Invalid session state",
			"message": fmt.Sprintf("Session must be in 'running' state to hibernate, currently: %s", currentState),
		})
		return
	}

	// 2. Create hibernate_session command in database
	commandID := fmt.Sprintf("cmd-%s", uuid.New().String()[:8])
	now := time.Now()
	payload := map[string]interface{}{
		"sessionId": sessionID,
		"namespace": DefaultNamespace,
	}

	// Marshal payload to JSON for database insertion (JSONB column)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal hibernate command payload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create hibernate command",
			"message": fmt.Sprintf("Failed to marshal payload: %v", err),
		})
		return
	}

	var command models.AgentCommand
	var errorMessage sql.NullString
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID.String, sessionID, "hibernate_session", payloadJSON, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&errorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if errorMessage.Valid {
		command.ErrorMessage = &errorMessage.String
	}

	if err != nil {
		log.Printf("Failed to create hibernate_session command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create hibernate command",
			"message": fmt.Sprintf("Failed to create command in database: %v", err),
		})
		return
	}
	log.Printf("Created hibernate_session command %s for session %s", commandID, sessionID)

	// 3. Update database session state to hibernating
	// Agent will update CRD when it processes the command
	if err := h.sessionDB.UpdateSessionState(ctx, sessionID, "hibernating"); err != nil {
		log.Printf("Failed to update database session state (non-fatal): %v", err)
	}

	// 4. Dispatch command to agent via WebSocket
	if h.dispatcher != nil {
		if err := h.dispatcher.DispatchCommand(&command); err != nil {
			log.Printf("Failed to dispatch hibernate command %s: %v", commandID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to dispatch hibernate command",
				"message": fmt.Sprintf("Failed to dispatch command to agent: %v", err),
			})
			return
		}
		log.Printf("Dispatched hibernate_session command %s to agent %s for session %s", commandID, agentID.String, sessionID)
	} else {
		log.Printf("Warning: CommandDispatcher is nil, hibernate command %s not dispatched", commandID)
	}

	// Return accepted response
	// Agent will handle ALL Kubernetes operations (scale deployment to 0)
	c.JSON(http.StatusAccepted, gin.H{
		"name":      sessionID,
		"commandId": commandID,
		"message":   "Session hibernation requested, agent will scale down resources",
	})
}

// WakeSession handles waking a hibernated session (scales to 1 replica)
func (h *Handler) WakeSession(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	sessionID := c.Param("id")

	// 1. Verify session exists in DATABASE and get agent managing it
	// v2.0-beta: API does NOT access Kubernetes directly - agent handles ALL K8s operations
	var agentID sql.NullString // Use sql.NullString for nullable column
	var currentState string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT agent_id, state FROM sessions WHERE id = $1
	`, sessionID).Scan(&agentID, &currentState)

	if err == sql.ErrNoRows {
		log.Printf("Session %s not found in database", sessionID)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"message": "The specified session does not exist",
		})
		return
	}

	if err != nil {
		log.Printf("Failed to query session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to query session",
			"message": fmt.Sprintf("Database error: %v", err),
		})
		return
	}

	// Check if session has an agent assigned
	if !agentID.Valid || agentID.String == "" {
		log.Printf("Session %s has no agent assigned (agent_id is NULL or empty)", sessionID)
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Session not ready",
			"message": "Session has no agent assigned - cannot wake. Session may have been terminated.",
		})
		return
	}

	// Check if session is in a state that can be woken
	if currentState != "hibernated" {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Invalid session state",
			"message": fmt.Sprintf("Session must be in 'hibernated' state to wake, currently: %s", currentState),
		})
		return
	}

	// 2. Create wake_session command in database
	commandID := fmt.Sprintf("cmd-%s", uuid.New().String()[:8])
	now := time.Now()
	payload := map[string]interface{}{
		"sessionId": sessionID,
		"namespace": DefaultNamespace,
	}

	// Marshal payload to JSON for database insertion (JSONB column)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal wake command payload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create wake command",
			"message": fmt.Sprintf("Failed to marshal payload: %v", err),
		})
		return
	}

	var command models.AgentCommand
	var errorMessage sql.NullString
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID.String, sessionID, "wake_session", payloadJSON, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&errorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if errorMessage.Valid {
		command.ErrorMessage = &errorMessage.String
	}

	if err != nil {
		log.Printf("Failed to create wake_session command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create wake command",
			"message": fmt.Sprintf("Failed to create command in database: %v", err),
		})
		return
	}
	log.Printf("Created wake_session command %s for session %s", commandID, sessionID)

	// 3. Update database session state to waking
	// Agent will update CRD when it processes the command
	if err := h.sessionDB.UpdateSessionState(ctx, sessionID, "waking"); err != nil {
		log.Printf("Failed to update database session state (non-fatal): %v", err)
	}

	// 4. Dispatch command to agent via WebSocket
	if h.dispatcher != nil {
		if err := h.dispatcher.DispatchCommand(&command); err != nil {
			log.Printf("Failed to dispatch wake command %s: %v", commandID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to dispatch wake command",
				"message": fmt.Sprintf("Failed to dispatch command to agent: %v", err),
			})
			return
		}
		log.Printf("Dispatched wake_session command %s to agent %s for session %s", commandID, agentID.String, sessionID)
	} else {
		log.Printf("Warning: CommandDispatcher is nil, wake command %s not dispatched", commandID)
	}

	// Return accepted response
	// Agent will handle ALL Kubernetes operations (scale deployment to 1, wait for pod ready)
	c.JSON(http.StatusAccepted, gin.H{
		"name":      sessionID,
		"commandId": commandID,
		"message":   "Session wake requested, agent will scale up resources",
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

	// v2.0-beta: Verify session exists in database (no K8s access)
	session, err := h.sessionDB.GetSession(ctx, sessionID)
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
	sessionUrl := session.URL
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

	// v2.0-beta: Update tags in database only (no K8s access)
	if err := h.sessionDB.UpdateSessionTags(ctx, sessionID, req.Tags); err != nil {
		if err.Error() == fmt.Sprintf("session not found: %s", sessionID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the updated session from database
	session, err := h.sessionDB.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
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

	// v2.0-beta: Query database directly for sessions with tags (no K8s access)
	sessions, err := h.sessionDB.ListSessionsByTags(ctx, tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
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
	featured := c.Query("featured")    // Filter featured templates (TODO: implement with featured_templates join)

	// v2.0-beta: Get templates from database (catalog_templates)
	var templates []*db.Template
	var err error

	if category != "" {
		templates, err = h.templateDB.ListTemplatesByCategory(ctx, category)
	} else {
		templates, err = h.templateDB.ListTemplates(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply search filter
	if search != "" {
		filtered := make([]*db.Template, 0)
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
		filtered := make([]*db.Template, 0)
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

	// Apply featured filter (TODO: join with featured_templates table)
	if featured == "true" {
		// Temporarily skip - requires database join with featured_templates
		log.Printf("Featured filter requested but not yet implemented with database")
	}

	// Sort templates
	switch sortBy {
	case "popularity":
		// Sort by install count
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].InstallCount > templates[j].InstallCount
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
	categories := make(map[string][]*db.Template)
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

	// v2.0-beta: Get template from database (catalog_templates)
	template, err := h.templateDB.GetTemplateByName(ctx, templateID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

// CreateTemplate creates a new template (admin only)
func (h *Handler) CreateTemplate(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()

	var template db.Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// v2.0-beta: Create template in database (catalog_templates)
	if err := h.templateDB.CreateTemplate(ctx, &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Created template %s in database (ID: %d)", template.Name, template.ID)

	c.JSON(http.StatusCreated, template)
}

// DeleteTemplate deletes a template (admin only)
func (h *Handler) DeleteTemplate(c *gin.Context) {
	// SECURITY FIX: Use request context for proper cancellation and timeout handling
	ctx := c.Request.Context()
	templateID := c.Param("id")

	// v2.0-beta: Delete template from database (catalog_templates)
	if err := h.templateDB.DeleteTemplate(ctx, templateID); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Deleted template %s from database", templateID)

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

	// v2.0-beta: Verify template exists in database
	_, err := h.templateDB.GetTemplateByName(ctx, templateID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	// v2.0-beta: Fetch full template details from database (catalog_templates)
	templates := make([]*db.Template, 0, len(templateNames))
	for _, name := range templateNames {
		template, err := h.templateDB.GetTemplateByName(ctx, name)
		if err != nil {
			log.Printf("Warning: Favorite template %s not found in database: %v", name, err)
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
			"icon":        tmpl.IconURL,
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
		Namespace:   DefaultNamespace,
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

	// v2.0-beta: Template already exists in database (catalog_templates)
	// "Installing" just increments the install count
	// Agent will fetch template from database when creating sessions
	err = h.templateDB.IncrementInstallCount(ctx, name)
	if err != nil {
		log.Printf("Error incrementing install count: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to install template",
			"message": err.Error(),
		})
		return
	}

	log.Printf("Template %s installed successfully (incremented install_count)", name)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Template installed successfully",
		"name":    name,
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
// v2.0-beta: Database is the single source of truth, no K8s fallback.
func (h *Handler) convertDBSessionToResponse(session *db.Session) map[string]interface{} {
	// v2.0-beta: Use database values directly (agent updates database)
	url := session.URL
	podName := session.PodName
	phase := session.State

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
