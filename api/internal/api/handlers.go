package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/quota"
	"github.com/streamspace/streamspace/api/internal/sync"
	"github.com/streamspace/streamspace/api/internal/tracker"
	"github.com/streamspace/streamspace/api/internal/websocket"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	sessionGVR = schema.GroupVersionResource{
		Group:    "stream.streamspace.io",
		Version:  "v1alpha1",
		Resource: "sessions",
	}
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
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "streamspace" // Default namespace
	}
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
		User               string   `json:"user" binding:"required"`
		Template           string   `json:"template" binding:"required"`
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
	// Parse CPU and memory to int64
	requestedCPU, requestedMemory, err := h.quotaEnforcer.ValidateResourceRequest(cpu, memory)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid resource request",
			"message": err.Error(),
		})
		return
	}

	// Get current usage by listing user's pods
	podList, err := h.k8sClient.GetPods(ctx, h.namespace)
	if err != nil {
		log.Printf("Failed to get pods for quota check: %v", err)
		// Continue with empty usage if we can't get pods
		podList = &corev1.PodList{}
	}

	// Filter pods for this user
	userPods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if user, ok := pod.Labels["user"]; ok && user == req.User {
			userPods = append(userPods, pod)
		}
	}

	currentUsage := h.quotaEnforcer.CalculateUsage(userPods)

	// Check if user can create session
	if err := h.quotaEnforcer.CheckSessionCreation(ctx, req.User, requestedCPU, requestedMemory, 0, currentUsage); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Quota exceeded",
			"message": err.Error(),
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

	if len(req.Tags) > 0 {
		session.Tags = req.Tags
	}

	// Create in Kubernetes
	created, err := h.k8sClient.CreateSession(ctx, session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

	// Verify session exists before deletion
	_, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Delete from Kubernetes
	if err := h.k8sClient.DeleteSession(ctx, h.namespace, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

// UpdateSessionTags updates tags for a session
func (h *Handler) UpdateSessionTags(c *gin.Context) {
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()

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

// AddTemplateFavorite adds a template to user's favorites
func (h *Handler) AddTemplateFavorite(c *gin.Context) {
	ctx := context.Background()
	templateID := c.Param("id")

	// Get user ID from context (set by auth middleware)
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
	ctx := context.Background()
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
	ctx := context.Background()

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
	ctx := context.Background()
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
	ctx := c.Request.Context()
	catalogID := c.Param("id")

	// Get template manifest and metadata from database
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
