package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/quota"
	"github.com/streamspace/streamspace/api/internal/sync"
	"github.com/streamspace/streamspace/api/internal/tracker"
	"github.com/streamspace/streamspace/api/internal/websocket"
)

// Handler handles all API requests
type Handler struct {
	db             *db.Database
	k8sClient      *k8s.Client
	connTracker    *tracker.ConnectionTracker
	syncService    *sync.SyncService
	wsManager      *websocket.Manager
	quotaEnforcer  *quota.Enforcer
	namespace      string
}

// NewHandler creates a new API handler
func NewHandler(database *db.Database, k8sClient *k8s.Client, connTracker *tracker.ConnectionTracker, syncService *sync.SyncService, wsManager *websocket.Manager, quotaEnforcer *quota.Enforcer) *Handler {
	namespace := "streamspace" // TODO: Make configurable
	return &Handler{
		db:            database,
		k8sClient:     k8sClient,
		connTracker:   connTracker,
		syncService:   syncService,
		wsManager:     wsManager,
		quotaEnforcer: quotaEnforcer,
		namespace:     namespace,
	}
}

// ============================================================================
// Session Endpoints
// ============================================================================

// ListSessions returns all sessions for a user or all sessions (admin)
func (h *Handler) ListSessions(c *gin.Context) {
	ctx := context.Background()
	userID := c.Query("user")

	var sessions []*k8s.Session
	var err error

	if userID != "" {
		sessions, err = h.k8sClient.ListSessionsByUser(ctx, h.namespace, userID)
	} else {
		sessions, err = h.k8sClient.ListSessions(ctx, h.namespace)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enrich with database info (active connections)
	enriched := h.enrichSessionsWithDBInfo(ctx, sessions)

	c.JSON(http.StatusOK, gin.H{
		"sessions": enriched,
		"total":    len(enriched),
	})
}

// GetSession returns a single session by ID
func (h *Handler) GetSession(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Enrich with database info
	enriched := h.enrichSessionWithDBInfo(ctx, session)

	c.JSON(http.StatusOK, enriched)
}

// CreateSession creates a new session
func (h *Handler) CreateSession(c *gin.Context) {
	ctx := context.Background()

	var req struct {
		User               string `json:"user" binding:"required"`
		Template           string `json:"template" binding:"required"`
		Resources          *struct {
			Memory string `json:"memory"`
			CPU    string `json:"cpu"`
		} `json:"resources"`
		PersistentHome     *bool  `json:"persistentHome"`
		IdleTimeout        string `json:"idleTimeout"`
		MaxSessionDuration string `json:"maxSessionDuration"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify template exists
	template, err := h.k8sClient.GetTemplate(ctx, h.namespace, req.Template)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Template not found: %s", req.Template)})
		return
	}

	// Set default resources from template if not provided
	memory := "2Gi"
	cpu := "1000m"
	if req.Resources != nil {
		if req.Resources.Memory != "" {
			memory = req.Resources.Memory
		}
		if req.Resources.CPU != "" {
			cpu = req.Resources.CPU
		}
	} else if template.DefaultResources.Memory != "" || template.DefaultResources.CPU != "" {
		// Use template defaults
		if template.DefaultResources.Memory != "" {
			memory = template.DefaultResources.Memory
		}
		if template.DefaultResources.CPU != "" {
			cpu = template.DefaultResources.CPU
		}
	}

	// Check user quota
	quotaReq := &quota.SessionRequest{
		UserID:  req.User,
		Memory:  memory,
		CPU:     cpu,
		Storage: "50Gi", // Default storage quota check
	}

	quotaResult, err := h.quotaEnforcer.CheckSessionQuota(ctx, quotaReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check quota",
			"message": err.Error(),
		})
		return
	}

	if !quotaResult.Allowed {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Quota exceeded",
			"message": quotaResult.Reason,
			"quota": gin.H{
				"current":   quotaResult.CurrentUsage,
				"requested": quotaResult.RequestedUsage,
				"available": quotaResult.AvailableQuota,
			},
		})
		return
	}

	// Generate session name: {user}-{template}-{random}
	sessionName := fmt.Sprintf("%s-%s-%s", req.User, req.Template, uuid.New().String()[:8])

	session := &k8s.Session{
		Name:      sessionName,
		Namespace: h.namespace,
		User:      req.User,
		Template:  req.Template,
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

	// Create in Kubernetes
	created, err := h.k8sClient.CreateSession(ctx, session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update quota usage
	if err := h.quotaEnforcer.UpdateSessionQuota(ctx, req.User, memory, cpu, "50Gi", true); err != nil {
		log.Printf("Failed to update quota usage: %v", err)
		// Don't fail the request, but log the error
	}

	// Cache in database
	if err := h.cacheSessionInDB(ctx, created); err != nil {
		log.Printf("Failed to cache session in database: %v", err)
	}

	c.JSON(http.StatusCreated, created)
}

// UpdateSession updates a session (typically state changes)
func (h *Handler) UpdateSession(c *gin.Context) {
	ctx := context.Background()
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

	// Update in Kubernetes
	updated, err := h.k8sClient.UpdateSessionState(ctx, h.namespace, sessionID, req.State)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update cache in database
	if err := h.updateSessionInDB(ctx, updated); err != nil {
		log.Printf("Failed to update session in database: %v", err)
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteSession deletes a session
func (h *Handler) DeleteSession(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	// Get session info before deletion (for quota tracking)
	session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Delete from Kubernetes
	if err := h.k8sClient.DeleteSession(ctx, h.namespace, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update quota usage (decrement)
	if session.Resources.Memory != "" && session.Resources.CPU != "" {
		if err := h.quotaEnforcer.UpdateSessionQuota(ctx, session.User,
			session.Resources.Memory, session.Resources.CPU, "50Gi", false); err != nil {
			log.Printf("Failed to update quota usage on session deletion: %v", err)
			// Don't fail the request, quota will be cleaned up later
		}
	}

	// Delete from database cache
	if err := h.deleteSessionFromDB(ctx, sessionID); err != nil {
		log.Printf("Failed to delete session from database: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// ConnectSession handles a user connecting to a session
func (h *Handler) ConnectSession(c *gin.Context) {
	ctx := context.Background()
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

	c.JSON(http.StatusOK, gin.H{
		"connectionId": conn.ID,
		"sessionUrl":   session.Status.URL,
		"state":        session.State,
		"message":      "Connection established. Session will auto-start if hibernated.",
	})
}

// DisconnectSession handles a user disconnecting from a session
func (h *Handler) DisconnectSession(c *gin.Context) {
	ctx := context.Background()
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
	ctx := context.Background()
	connectionID := c.Query("connectionId")

	if connectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId parameter required"})
		return
	}

	if err := h.connTracker.UpdateHeartbeat(ctx, connectionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
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

// ============================================================================
// Template Endpoints
// ============================================================================

// ListTemplates returns all templates
func (h *Handler) ListTemplates(c *gin.Context) {
	ctx := context.Background()
	category := c.Query("category")

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

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetTemplate returns a single template
func (h *Handler) GetTemplate(c *gin.Context) {
	ctx := context.Background()
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
	ctx := context.Background()

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

	c.JSON(http.StatusCreated, created)
}

// DeleteTemplate deletes a template (admin only)
func (h *Handler) DeleteTemplate(c *gin.Context) {
	ctx := context.Background()
	templateID := c.Param("id")

	if err := h.k8sClient.DeleteTemplate(ctx, h.namespace, templateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

// ============================================================================
// Catalog Endpoints (Template Marketplace)
// ============================================================================

// ListCatalogTemplates returns templates from the marketplace catalog
func (h *Handler) ListCatalogTemplates(c *gin.Context) {
	ctx := context.Background()
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

// InstallCatalogTemplate installs a template from the catalog to the cluster
func (h *Handler) InstallCatalogTemplate(c *gin.Context) {
	ctx := context.Background()
	catalogID := c.Param("id")

	// Get template manifest from database
	var manifest string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT manifest FROM catalog_templates WHERE id = $1
	`, catalogID).Scan(&manifest)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog template not found"})
		return
	}

	// TODO: Parse manifest and create Template CRD
	// For now, return the manifest
	c.JSON(http.StatusOK, gin.H{
		"message":  "Template installation not yet implemented",
		"manifest": manifest,
	})

	// Increment install count
	_, _ = h.db.DB().ExecContext(ctx, `
		UPDATE catalog_templates SET install_count = install_count + 1 WHERE id = $1
	`, catalogID)
}

// ============================================================================
// Repository Endpoints (Template Repository Management)
// ============================================================================

// ListRepositories returns all template repositories
func (h *Handler) ListRepositories(c *gin.Context) {
	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, name, url, branch, auth_type, last_sync, template_count, status, error_message, created_at, updated_at
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
		var name, url, branch, authType, status, errorMessage string
		var lastSync, createdAt, updatedAt time.Time
		var templateCount int

		if err := rows.Scan(&id, &name, &url, &branch, &authType, &lastSync, &templateCount, &status, &errorMessage, &createdAt, &updatedAt); err != nil {
			continue
		}

		repos = append(repos, map[string]interface{}{
			"id":            id,
			"name":          name,
			"url":           url,
			"branch":        branch,
			"authType":      authType,
			"lastSync":      lastSync,
			"templateCount": templateCount,
			"status":        status,
			"errorMessage":  errorMessage,
			"createdAt":     createdAt,
			"updatedAt":     updatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"repositories": repos,
		"total":        len(repos),
	})
}

// AddRepository adds a new template repository
func (h *Handler) AddRepository(c *gin.Context) {
	ctx := context.Background()

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

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, gin.H{
		"id":      id,
		"message": "Repository added. Sync will begin shortly.",
	})

	// TODO: Trigger repository sync in background
}

// SyncRepository triggers a sync for a repository
func (h *Handler) SyncRepository(c *gin.Context) {
	ctx := context.Background()
	repoIDStr := c.Param("id")

	// Convert repo ID to int
	var repoID int
	if _, err := fmt.Sscanf(repoIDStr, "%d", &repoID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	// Trigger sync in background
	go func() {
		if err := h.syncService.SyncRepository(ctx, repoID); err != nil {
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
	ctx := context.Background()
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

// enrichSessionsWithDBInfo enriches sessions with database information
func (h *Handler) enrichSessionsWithDBInfo(ctx context.Context, sessions []*k8s.Session) []map[string]interface{} {
	enriched := make([]map[string]interface{}, 0, len(sessions))

	for _, session := range sessions {
		enriched = append(enriched, h.enrichSessionWithDBInfo(ctx, session))
	}

	return enriched
}

// enrichSessionWithDBInfo enriches a session with database information
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

// cacheSessionInDB caches a session in the database
func (h *Handler) cacheSessionInDB(ctx context.Context, session *k8s.Session) error {
	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, template_name, state, app_type, namespace, url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE
		SET user_id = $2, template_name = $3, state = $4, updated_at = $9
	`, session.Name, session.User, session.Template, session.State, "desktop", session.Namespace, session.Status.URL, session.CreatedAt, time.Now())

	return err
}

// updateSessionInDB updates a session in the database cache
func (h *Handler) updateSessionInDB(ctx context.Context, session *k8s.Session) error {
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET state = $1, url = $2, updated_at = $3
		WHERE id = $4
	`, session.State, session.Status.URL, time.Now(), session.Name)

	return err
}

// deleteSessionFromDB deletes a session from the database cache
func (h *Handler) deleteSessionFromDB(ctx context.Context, sessionID string) error {
	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM sessions WHERE id = $1
	`, sessionID)

	return err
}
