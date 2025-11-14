package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now (TODO: restrict in production)
	},
}

// ============================================================================
// Health & Version Endpoints
// ============================================================================

// Health returns health status
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "streamspace-api",
	})
}

// Version returns API version
func (h *Handler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "v0.1.0",
		"api": "v1",
		"phase": "2.2",
	})
}

// ============================================================================
// Stub Methods (To Be Implemented)
// ============================================================================

// UpdateTemplate updates a template (admin only)
func (h *Handler) UpdateTemplate(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListNodes returns cluster nodes
func (h *Handler) ListNodes(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListPods returns pods in namespace
func (h *Handler) ListPods(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListDeployments returns deployments
func (h *Handler) ListDeployments(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListServices returns services
func (h *Handler) ListServices(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListNamespaces returns namespaces
func (h *Handler) ListNamespaces(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// CreateResource creates a K8s resource
func (h *Handler) CreateResource(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// UpdateResource updates a K8s resource
func (h *Handler) UpdateResource(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// DeleteResource deletes a K8s resource
func (h *Handler) DeleteResource(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// GetPodLogs returns pod logs
func (h *Handler) GetPodLogs(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// GetConfig returns configuration
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// UpdateConfig updates configuration
func (h *Handler) UpdateConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// ListUsers returns all users
func (h *Handler) ListUsers(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// CreateUser creates a new user
func (h *Handler) CreateUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// GetCurrentUser returns current user info
func (h *Handler) GetCurrentUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// GetUser returns user by ID
func (h *Handler) GetUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// UpdateUser updates user
func (h *Handler) UpdateUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

// GetUserSessions returns sessions for a user
func (h *Handler) GetUserSessions(c *gin.Context) {
	userID := c.Param("id")
	c.Redirect(http.StatusTemporaryRedirect, "/api/v1/sessions?user="+userID)
}

// GetMetrics returns metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	stats := h.connTracker.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// WebSocket Endpoints
// ============================================================================

// SessionsWebSocket handles WebSocket for real-time session updates
func (h *Handler) SessionsWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	h.wsManager.HandleSessionsWebSocket(conn)
}

// ClusterWebSocket handles WebSocket for real-time cluster updates
func (h *Handler) ClusterWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	h.wsManager.HandleMetricsWebSocket(conn)
}

// LogsWebSocket handles WebSocket for streaming pod logs
func (h *Handler) LogsWebSocket(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	h.wsManager.HandleLogsWebSocket(conn, namespace, podName)
}

// ============================================================================
// Catalog/Repository Endpoints (Additional)
// ============================================================================

// BrowseCatalog returns catalog templates (alias for ListCatalogTemplates)
func (h *Handler) BrowseCatalog(c *gin.Context) {
	h.ListCatalogTemplates(c)
}

// InstallTemplate installs a template from catalog (alias for InstallCatalogTemplate)
func (h *Handler) InstallTemplate(c *gin.Context) {
	catalogID := c.Query("id")
	if catalogID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter required"})
		return
	}

	c.Params = append(c.Params, gin.Param{Key: "id", Value: catalogID})
	h.InstallCatalogTemplate(c)
}

// SyncCatalog triggers sync for all repositories
func (h *Handler) SyncCatalog(c *gin.Context) {
	go func() {
		if err := h.syncService.SyncAllRepositories(c.Request.Context()); err != nil {
			log.Printf("Catalog sync failed: %v", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Catalog sync triggered",
		"status":  "syncing",
	})
}

// RemoveRepository removes a repository (alias for DeleteRepository)
func (h *Handler) RemoveRepository(c *gin.Context) {
	h.DeleteRepository(c)
}

// ============================================================================
// Webhook Endpoint for Repository Auto-Sync
// ============================================================================

// WebhookRepositorySync handles webhooks from Git providers for auto-sync
func (h *Handler) WebhookRepositorySync(c *gin.Context) {
	var webhook struct {
		RepositoryURL string `json:"repository_url"`
		Branch        string `json:"branch"`
		Ref           string `json:"ref"`
	}

	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find repository by URL
	ctx := c.Request.Context()
	var repoID int
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id FROM repositories WHERE url = $1
	`, webhook.RepositoryURL).Scan(&repoID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	// Trigger sync in background
	go func() {
		if err := h.syncService.SyncRepository(ctx, repoID); err != nil {
			log.Printf("Webhook-triggered sync failed for repository %d: %v", repoID, err)
		} else {
			log.Printf("Webhook-triggered sync completed for repository %d", repoID)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Webhook received, sync triggered",
		"repository":   webhook.RepositoryURL,
		"repositoryID": repoID,
	})
}
