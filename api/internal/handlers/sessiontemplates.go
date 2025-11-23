// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements custom session template management and presets.
//
// SESSION TEMPLATE FEATURES:
// - User-defined session configuration templates
// - Template CRUD operations (create, read, update, delete)
// - Template visibility (private, team, public)
// - Template cloning and versioning
// - Template sharing within teams
// - Create templates from existing sessions
// - Usage tracking and analytics
//
// TEMPLATE VISIBILITY:
// - private: Only visible to template owner
// - team: Visible to team members
// - public: Visible to all users (requires approval)
//
// TEMPLATE STRUCTURE:
// - Based on catalog template (base template reference)
// - Custom configuration overrides
// - Resource allocations (CPU, memory)
// - Environment variables
// - Tags and categorization
// - Version tracking
//
// TEMPLATE OPERATIONS:
// - Create: Define new template from scratch or from session
// - Clone: Duplicate existing template
// - Use: Launch session from template
// - Publish/Unpublish: Make template public or private
// - Share: Share with specific users or teams
//
// TEMPLATE SHARING:
// - Share templates with users or teams
// - Permission levels (view, use, edit)
// - Revoke shares
// - Track who has access
//
// TEMPLATE VERSIONING:
// - Version history tracking
// - Restore previous versions
// - Version comparison
// - Change logs
//
// QUICK ACTIONS:
// - Create template from running session
// - Set as default template for user
// - Clone template with modifications
//
// API Endpoints:
// - GET    /api/v1/session-templates - List session templates
// - POST   /api/v1/session-templates - Create session template
// - GET    /api/v1/session-templates/:id - Get template details
// - PUT    /api/v1/session-templates/:id - Update template
// - DELETE /api/v1/session-templates/:id - Delete template
// - POST   /api/v1/session-templates/:id/clone - Clone template
// - POST   /api/v1/session-templates/:id/use - Launch session from template
// - POST   /api/v1/session-templates/:id/publish - Make template public
// - POST   /api/v1/session-templates/:id/unpublish - Make template private
// - GET    /api/v1/session-templates/:id/shares - List template shares
// - POST   /api/v1/session-templates/:id/share - Share template
// - DELETE /api/v1/session-templates/:id/shares/:shareId - Revoke share
// - GET    /api/v1/session-templates/:id/versions - List template versions
// - POST   /api/v1/session-templates/:id/versions - Create version
// - POST   /api/v1/session-templates/:id/versions/:version/restore - Restore version
// - POST   /api/v1/session-templates/from-session/:sessionId - Create from session
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: session_templates, template_shares, template_versions tables
// - External Services: None
//
// Example Usage:
//
//	handler := NewSessionTemplatesHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/events"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// SessionTemplatesHandler handles custom session templates and presets
type SessionTemplatesHandler struct {
	db        *db.Database
	k8sClient *k8s.Client
	publisher *events.Publisher
	platform  string
	namespace string
}

// NewSessionTemplatesHandler creates a new session templates handler
func NewSessionTemplatesHandler(database *db.Database, k8sClient *k8s.Client, publisher *events.Publisher, platform string) *SessionTemplatesHandler {
	namespace := "streamspace" // Default namespace
	return &SessionTemplatesHandler{
		db:        database,
		k8sClient: k8sClient,
		publisher: publisher,
		platform:  platform,
		namespace: namespace,
	}
}

// SessionTemplate represents a user-defined session configuration template
type SessionTemplate struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"userId"`
	TeamID        string                 `json:"teamId,omitempty"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Icon          string                 `json:"icon,omitempty"`
	Category      string                 `json:"category,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Visibility    string                 `json:"visibility"`   // private, team, public
	BaseTemplate  string                 `json:"baseTemplate"` // Reference to catalog template
	Configuration map[string]interface{} `json:"configuration"`
	Resources     map[string]interface{} `json:"resources"`
	Environment   map[string]string      `json:"environment,omitempty"`
	IsDefault     bool                   `json:"isDefault"`
	UsageCount    int                    `json:"usageCount"`
	Version       string                 `json:"version"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

// RegisterRoutes registers session template routes
func (h *SessionTemplatesHandler) RegisterRoutes(router *gin.RouterGroup) {
	templates := router.Group("/session-templates")
	{
		// Template CRUD
		templates.GET("", h.ListSessionTemplates)
		templates.POST("", h.CreateSessionTemplate)
		templates.GET("/:id", h.GetSessionTemplate)
		templates.PUT("/:id", h.UpdateSessionTemplate)
		templates.DELETE("/:id", h.DeleteSessionTemplate)

		// Template operations
		templates.POST("/:id/clone", h.CloneSessionTemplate)
		templates.POST("/:id/use", h.UseSessionTemplate)
		templates.POST("/:id/publish", h.PublishSessionTemplate)
		templates.POST("/:id/unpublish", h.UnpublishSessionTemplate)

		// Template sharing
		templates.GET("/:id/shares", h.ListTemplateShares)
		templates.POST("/:id/share", h.ShareSessionTemplate)
		templates.DELETE("/:id/shares/:shareId", h.RevokeTemplateShare)

		// Template versions
		templates.GET("/:id/versions", h.ListTemplateVersions)
		templates.POST("/:id/versions", h.CreateTemplateVersion)
		templates.POST("/:id/versions/:version/restore", h.RestoreTemplateVersion)

		// Quick actions
		templates.POST("/from-session/:sessionId", h.CreateTemplateFromSession)
		templates.GET("/defaults", h.GetDefaultTemplates)
		templates.POST("/:id/set-default", h.SetAsDefaultTemplate)
		templates.GET("/public", h.ListPublicTemplates)
		templates.GET("/team/:teamId", h.ListTeamTemplates)
	}
}

// ListSessionTemplates returns user's session templates
func (h *SessionTemplatesHandler) ListSessionTemplates(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	visibility := c.Query("visibility") // private, team, public, all
	category := c.Query("category")

	ctx := context.Background()

	sqlQuery := `
		SELECT id, user_id, team_id, name, description, icon, category, tags, visibility,
		       base_template, configuration, resources, environment, is_default,
		       usage_count, version, created_at, updated_at
		FROM user_session_templates
		WHERE user_id = $1
	`
	args := []interface{}{userIDStr}
	argIndex := 2

	if visibility != "" && visibility != "all" {
		sqlQuery += fmt.Sprintf(` AND visibility = $%d`, argIndex)
		args = append(args, visibility)
		argIndex++
	}

	if category != "" {
		sqlQuery += fmt.Sprintf(` AND category = $%d`, argIndex)
		args = append(args, category)
		argIndex++
	}

	sqlQuery += ` ORDER BY is_default DESC, usage_count DESC, created_at DESC`

	rows, err := h.db.DB().QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}
	defer rows.Close()

	templates := []SessionTemplate{}
	for rows.Next() {
		var t SessionTemplate
		var description, icon, category, teamID sql.NullString
		var tagsJSON, configJSON, resourcesJSON, envJSON []byte

		if err := rows.Scan(&t.ID, &t.UserID, &teamID, &t.Name, &description, &icon, &category, &tagsJSON, &t.Visibility, &t.BaseTemplate, &configJSON, &resourcesJSON, &envJSON, &t.IsDefault, &t.UsageCount, &t.Version, &t.CreatedAt, &t.UpdatedAt); err == nil {
			if description.Valid {
				t.Description = description.String
			}
			if icon.Valid {
				t.Icon = icon.String
			}
			if category.Valid {
				t.Category = category.String
			}
			if teamID.Valid {
				t.TeamID = teamID.String
			}
			if len(tagsJSON) > 0 {
				json.Unmarshal(tagsJSON, &t.Tags)
			}
			if len(configJSON) > 0 {
				json.Unmarshal(configJSON, &t.Configuration)
			}
			if len(resourcesJSON) > 0 {
				json.Unmarshal(resourcesJSON, &t.Resources)
			}
			if len(envJSON) > 0 {
				json.Unmarshal(envJSON, &t.Environment)
			}

			templates = append(templates, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// CreateSessionTemplateRequest is the request body for creating a session template
type CreateSessionTemplateRequest struct {
	Name          string                 `json:"name" binding:"required" validate:"required,min=3,max=100"`
	Description   string                 `json:"description" validate:"omitempty,max=1000"`
	Icon          string                 `json:"icon" validate:"omitempty,max=100"`
	Category      string                 `json:"category" validate:"omitempty,min=2,max=50"`
	Tags          []string               `json:"tags" validate:"omitempty,dive,min=2,max=50"`
	Visibility    string                 `json:"visibility" validate:"omitempty,oneof=private team public"`
	TeamID        string                 `json:"teamId" validate:"omitempty,uuid"`
	BaseTemplate  string                 `json:"baseTemplate" binding:"required" validate:"required,min=3,max=100"`
	Configuration map[string]interface{} `json:"configuration"`
	Resources     map[string]interface{} `json:"resources"`
	Environment   map[string]string      `json:"environment" validate:"omitempty,dive,keys,min=1,max=100,endkeys,min=0,max=10000"`
	IsDefault     bool                   `json:"isDefault"`
}

// CreateSessionTemplate creates a new session template
func (h *SessionTemplatesHandler) CreateSessionTemplate(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req CreateSessionTemplateRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Default visibility to private
	if req.Visibility == "" {
		req.Visibility = "private"
	}

	ctx := context.Background()

	templateID := fmt.Sprintf("usertpl_%d", time.Now().UnixNano())

	tagsJSON, _ := json.Marshal(req.Tags)
	configJSON, _ := json.Marshal(req.Configuration)
	resourcesJSON, _ := json.Marshal(req.Resources)
	envJSON, _ := json.Marshal(req.Environment)

	// If setting as default, unset other defaults
	if req.IsDefault {
		h.db.DB().ExecContext(ctx, `UPDATE user_session_templates SET is_default = false WHERE user_id = $1`, userIDStr)
	}

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_session_templates (id, user_id, team_id, name, description, icon, category, tags, visibility, base_template, configuration, resources, environment, is_default, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, '1.0.0')
	`, templateID, userIDStr, req.TeamID, req.Name, req.Description, req.Icon, req.Category, tagsJSON, req.Visibility, req.BaseTemplate, configJSON, resourcesJSON, envJSON, req.IsDefault)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Session template created",
		"templateId": templateID,
	})
}

// GetSessionTemplate retrieves a specific template
func (h *SessionTemplatesHandler) GetSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var t SessionTemplate
	var description, icon, category, teamID sql.NullString
	var tagsJSON, configJSON, resourcesJSON, envJSON []byte

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, user_id, team_id, name, description, icon, category, tags, visibility,
		       base_template, configuration, resources, environment, is_default,
		       usage_count, version, created_at, updated_at
		FROM user_session_templates
		WHERE id = $1 AND (user_id = $2 OR visibility = 'public')
	`, templateID, userIDStr).Scan(&t.ID, &t.UserID, &teamID, &t.Name, &description, &icon, &category, &tagsJSON, &t.Visibility, &t.BaseTemplate, &configJSON, &resourcesJSON, &envJSON, &t.IsDefault, &t.UsageCount, &t.Version, &t.CreatedAt, &t.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	if description.Valid {
		t.Description = description.String
	}
	if icon.Valid {
		t.Icon = icon.String
	}
	if category.Valid {
		t.Category = category.String
	}
	if teamID.Valid {
		t.TeamID = teamID.String
	}
	if len(tagsJSON) > 0 {
		json.Unmarshal(tagsJSON, &t.Tags)
	}
	if len(configJSON) > 0 {
		json.Unmarshal(configJSON, &t.Configuration)
	}
	if len(resourcesJSON) > 0 {
		json.Unmarshal(resourcesJSON, &t.Resources)
	}
	if len(envJSON) > 0 {
		json.Unmarshal(envJSON, &t.Environment)
	}

	c.JSON(http.StatusOK, t)
}

// UpdateSessionTemplateRequest is the request body for updating a session template
type UpdateSessionTemplateRequest struct {
	Name          string                 `json:"name" validate:"omitempty,min=3,max=100"`
	Description   string                 `json:"description" validate:"omitempty,max=1000"`
	Icon          string                 `json:"icon" validate:"omitempty,max=100"`
	Category      string                 `json:"category" validate:"omitempty,min=2,max=50"`
	Tags          []string               `json:"tags" validate:"omitempty,dive,min=2,max=50"`
	Configuration map[string]interface{} `json:"configuration"`
	Resources     map[string]interface{} `json:"resources"`
	Environment   map[string]string      `json:"environment" validate:"omitempty,dive,keys,min=1,max=100,endkeys,min=0,max=10000"`
}

// UpdateSessionTemplate updates a template
func (h *SessionTemplatesHandler) UpdateSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req UpdateSessionTemplateRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	ctx := context.Background()

	tagsJSON, _ := json.Marshal(req.Tags)
	configJSON, _ := json.Marshal(req.Configuration)
	resourcesJSON, _ := json.Marshal(req.Resources)
	envJSON, _ := json.Marshal(req.Environment)

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates
		SET name = $1, description = $2, icon = $3, category = $4, tags = $5,
		    configuration = $6, resources = $7, environment = $8, updated_at = CURRENT_TIMESTAMP
		WHERE id = $9 AND user_id = $10
	`, req.Name, req.Description, req.Icon, req.Category, tagsJSON, configJSON, resourcesJSON, envJSON, templateID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template updated",
		"templateId": templateID,
	})
}

// DeleteSessionTemplate deletes a template
func (h *SessionTemplatesHandler) DeleteSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM user_session_templates WHERE id = $1 AND user_id = $2
	`, templateID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template deleted",
		"templateId": templateID,
	})
}

// CloneSessionTemplate creates a copy of a template
func (h *SessionTemplatesHandler) CloneSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req)

	ctx := context.Background()

	// Get original template
	var originalName, baseTemplate string
	var configJSON, resourcesJSON, envJSON, tagsJSON []byte
	var description, icon, category sql.NullString

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT name, description, icon, category, tags, base_template, configuration, resources, environment
		FROM user_session_templates
		WHERE id = $1
	`, templateID).Scan(&originalName, &description, &icon, &category, &tagsJSON, &baseTemplate, &configJSON, &resourcesJSON, &envJSON)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	newID := fmt.Sprintf("usertpl_%d", time.Now().UnixNano())
	newName := req.Name
	if newName == "" {
		newName = originalName + " (Copy)"
	}

	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO user_session_templates (id, user_id, name, description, icon, category, tags, visibility, base_template, configuration, resources, environment, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'private', $8, $9, $10, $11, '1.0.0')
	`, newID, userIDStr, newName, description, icon, category, tagsJSON, baseTemplate, configJSON, resourcesJSON, envJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clone template"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Template cloned",
		"templateId": newID,
	})
}

// UseSessionTemplate creates a session from a template
func (h *SessionTemplatesHandler) UseSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := c.Request.Context()

	// Get the user session template configuration
	var baseTemplate string
	var configJSON, resourcesJSON, envJSON sql.NullString
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT base_template, configuration, resources, environment
		FROM user_session_templates
		WHERE id = $1 AND (user_id = $2 OR visibility = 'public')
	`, templateID, userIDStr).Scan(&baseTemplate, &configJSON, &resourcesJSON, &envJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found or access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		}
		return
	}

	// Parse resources configuration
	memory := "2Gi"
	cpu := "1000m"
	if resourcesJSON.Valid && resourcesJSON.String != "" {
		var resources map[string]string
		if err := json.Unmarshal([]byte(resourcesJSON.String), &resources); err == nil {
			if m, ok := resources["memory"]; ok && m != "" {
				memory = m
			}
			if c, ok := resources["cpu"]; ok && c != "" {
				cpu = c
			}
		}
	}

	// Verify the base Kubernetes template exists and get its configuration
	k8sTemplate, err := h.k8sClient.GetTemplate(ctx, h.namespace, baseTemplate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Base template not found",
			"message": fmt.Sprintf("Template '%s' is not available. Please check if the application is installed.", baseTemplate),
		})
		return
	}

	// Generate session name
	sessionName := fmt.Sprintf("%s-%s-%s", userIDStr, baseTemplate, uuid.New().String()[:8])

	// Create the Kubernetes session
	session := &k8s.Session{
		Name:           sessionName,
		Namespace:      h.namespace,
		User:           userIDStr,
		Template:       baseTemplate,
		State:          "running",
		PersistentHome: true,
	}
	session.Resources.Memory = memory
	session.Resources.CPU = cpu

	created, err := h.k8sClient.CreateSession(ctx, session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
		return
	}

	// Increment usage count
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates SET usage_count = usage_count + 1 WHERE id = $1
	`, templateID)
	if err != nil {
		log.Printf("Failed to update usage count for template %s: %v", templateID, err)
	}

	// Publish session create event for controllers
	createEvent := &events.SessionCreateEvent{
		SessionID:      sessionName,
		UserID:         userIDStr,
		TemplateID:     baseTemplate,
		Platform:       h.platform,
		Resources:      events.ResourceSpec{Memory: memory, CPU: cpu},
		PersistentHome: true,
	}

	// Add template configuration for Docker controller
	if k8sTemplate != nil {
		vncPort := 3000 // Default VNC port
		if k8sTemplate.VNC != nil && k8sTemplate.VNC.Port > 0 {
			vncPort = int(k8sTemplate.VNC.Port)
		}

		// Convert env vars to map
		envMap := make(map[string]string)
		for _, env := range k8sTemplate.Env {
			envMap[env.Name] = env.Value
		}

		createEvent.TemplateConfig = &events.TemplateConfig{
			Image:       k8sTemplate.BaseImage,
			VNCPort:     vncPort,
			DisplayName: k8sTemplate.DisplayName,
			Env:         envMap,
		}
	}

	if err := h.publisher.PublishSessionCreate(ctx, createEvent); err != nil {
		log.Printf("Warning: Failed to publish session create event: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Session created from template",
		"templateId": templateID,
		"sessionId":  created.Name,
		"session": map[string]interface{}{
			"name":      created.Name,
			"namespace": created.Namespace,
			"user":      created.User,
			"template":  created.Template,
			"state":     created.State,
			"resources": map[string]string{
				"memory": created.Resources.Memory,
				"cpu":    created.Resources.CPU,
			},
		},
	})
}

// CreateTemplateFromSession creates a template from an existing session
func (h *SessionTemplatesHandler) CreateTemplateFromSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Visibility  string `json:"visibility"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Get session details
	var templateName string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT template_name FROM sessions WHERE id = $1 AND user_id = $2
	`, sessionID, userIDStr).Scan(&templateName)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if req.Visibility == "" {
		req.Visibility = "private"
	}

	templateID := fmt.Sprintf("usertpl_%d", time.Now().UnixNano())

	// Create template with session configuration
	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO user_session_templates (id, user_id, name, description, category, visibility, base_template, configuration, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{}', '1.0.0')
	`, templateID, userIDStr, req.Name, req.Description, req.Category, req.Visibility, templateName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template from session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Template created from session",
		"templateId": templateID,
	})
}

// SetAsDefaultTemplate sets a template as the user's default
func (h *SessionTemplatesHandler) SetAsDefaultTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Unset other defaults
	h.db.DB().ExecContext(ctx, `UPDATE user_session_templates SET is_default = false WHERE user_id = $1`, userIDStr)

	// Set this as default
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates SET is_default = true WHERE id = $1 AND user_id = $2
	`, templateID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Default template set",
		"templateId": templateID,
	})
}

// GetDefaultTemplates returns user's default templates
func (h *SessionTemplatesHandler) GetDefaultTemplates(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, name, description, base_template, usage_count
		FROM user_session_templates
		WHERE user_id = $1 AND is_default = true
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default templates"})
		return
	}
	defer rows.Close()

	defaults := []map[string]interface{}{}
	for rows.Next() {
		var id, name, baseTemplate string
		var description sql.NullString
		var usageCount int

		if err := rows.Scan(&id, &name, &description, &baseTemplate, &usageCount); err == nil {
			item := map[string]interface{}{
				"id":           id,
				"name":         name,
				"baseTemplate": baseTemplate,
				"usageCount":   usageCount,
			}
			if description.Valid {
				item["description"] = description.String
			}
			defaults = append(defaults, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": defaults,
		"count":     len(defaults),
	})
}

// PublishSessionTemplate makes a template public
func (h *SessionTemplatesHandler) PublishSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates SET visibility = 'public' WHERE id = $1 AND user_id = $2
	`, templateID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template published",
		"templateId": templateID,
	})
}

// UnpublishSessionTemplate makes a template private
func (h *SessionTemplatesHandler) UnpublishSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates SET visibility = 'private' WHERE id = $1 AND user_id = $2
	`, templateID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template unpublished",
		"templateId": templateID,
	})
}

// ListPublicTemplates returns all public templates
func (h *SessionTemplatesHandler) ListPublicTemplates(c *gin.Context) {
	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, user_id, name, description, icon, category, tags, base_template, usage_count, created_at
		FROM user_session_templates
		WHERE visibility = 'public'
		ORDER BY usage_count DESC, created_at DESC
		LIMIT 100
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list public templates"})
		return
	}
	defer rows.Close()

	templates := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, name, baseTemplate string
		var description, icon, category sql.NullString
		var tagsJSON []byte
		var usageCount int
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &name, &description, &icon, &category, &tagsJSON, &baseTemplate, &usageCount, &createdAt); err == nil {
			item := map[string]interface{}{
				"id":           id,
				"userId":       userID,
				"name":         name,
				"baseTemplate": baseTemplate,
				"usageCount":   usageCount,
				"createdAt":    createdAt,
			}
			if description.Valid {
				item["description"] = description.String
			}
			if icon.Valid {
				item["icon"] = icon.String
			}
			if category.Valid {
				item["category"] = category.String
			}
			if len(tagsJSON) > 0 {
				var tags []string
				json.Unmarshal(tagsJSON, &tags)
				item["tags"] = tags
			}
			templates = append(templates, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// ListTeamTemplates returns team templates
func (h *SessionTemplatesHandler) ListTeamTemplates(c *gin.Context) {
	teamID := c.Param("teamId")

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, user_id, name, description, base_template, usage_count
		FROM user_session_templates
		WHERE team_id = $1 AND visibility IN ('team', 'public')
		ORDER BY usage_count DESC
	`, teamID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list team templates"})
		return
	}
	defer rows.Close()

	templates := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, name, baseTemplate string
		var description sql.NullString
		var usageCount int

		if err := rows.Scan(&id, &userID, &name, &description, &baseTemplate, &usageCount); err == nil {
			item := map[string]interface{}{
				"id":           id,
				"userId":       userID,
				"name":         name,
				"baseTemplate": baseTemplate,
				"usageCount":   usageCount,
			}
			if description.Valid {
				item["description"] = description.String
			}
			templates = append(templates, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// Template sharing implementations

// TemplateShare represents a template share record
type TemplateShare struct {
	ID                 string     `json:"id"`
	TemplateID         string     `json:"templateId"`
	SharedBy           string     `json:"sharedBy"`
	SharedWithUserID   *string    `json:"sharedWithUserId,omitempty"`
	SharedWithTeamID   *string    `json:"sharedWithTeamId,omitempty"`
	SharedWithUserName string     `json:"sharedWithUserName,omitempty"`
	SharedWithTeamName string     `json:"sharedWithTeamName,omitempty"`
	PermissionLevel    string     `json:"permissionLevel"` // read, write, manage
	CreatedAt          time.Time  `json:"createdAt"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
}

func (h *SessionTemplatesHandler) ListTemplateShares(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Verify user owns the template or has manage permission
	if !h.canManageTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view shares for this template"})
		return
	}

	// Query template shares with user/team names
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			ts.id, ts.template_id, ts.shared_by, ts.shared_with_user_id, ts.shared_with_team_id,
			ts.permission_level, ts.created_at, ts.revoked_at,
			COALESCE(u.username, '') as user_name,
			COALESCE(g.name, '') as team_name
		FROM template_shares ts
		LEFT JOIN users u ON ts.shared_with_user_id = u.id
		LEFT JOIN groups g ON ts.shared_with_team_id = g.id
		WHERE ts.template_id = $1 AND ts.revoked_at IS NULL
		ORDER BY ts.created_at DESC
	`, templateID)

	if err != nil {
		log.Printf("[ERROR] Failed to list template shares: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list shares"})
		return
	}
	defer rows.Close()

	shares := []TemplateShare{}
	for rows.Next() {
		var share TemplateShare
		var userName, teamName sql.NullString

		err := rows.Scan(
			&share.ID, &share.TemplateID, &share.SharedBy, &share.SharedWithUserID, &share.SharedWithTeamID,
			&share.PermissionLevel, &share.CreatedAt, &share.RevokedAt,
			&userName, &teamName,
		)

		if err != nil {
			log.Printf("[ERROR] Failed to scan share: %v", err)
			continue
		}

		if userName.Valid {
			share.SharedWithUserName = userName.String
		}
		if teamName.Valid {
			share.SharedWithTeamName = teamName.String
		}

		shares = append(shares, share)
	}

	c.JSON(http.StatusOK, gin.H{
		"shares": shares,
		"count":  len(shares),
	})
}

func (h *SessionTemplatesHandler) ShareSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SharedWithUserID *string `json:"sharedWithUserId"`
		SharedWithTeamID *string `json:"sharedWithTeamId"`
		PermissionLevel  string  `json:"permissionLevel"` // read, write, manage
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate that either user or team is specified, not both
	if (req.SharedWithUserID == nil && req.SharedWithTeamID == nil) ||
		(req.SharedWithUserID != nil && req.SharedWithTeamID != nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Must specify either sharedWithUserId or sharedWithTeamId, not both"})
		return
	}

	// Validate permission level
	if req.PermissionLevel != "read" && req.PermissionLevel != "write" && req.PermissionLevel != "manage" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permission level. Must be 'read', 'write', or 'manage'"})
		return
	}

	ctx := context.Background()

	// Verify user owns the template or has manage permission
	if !h.canManageTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to share this template"})
		return
	}

	// Prevent self-sharing
	if req.SharedWithUserID != nil && *req.SharedWithUserID == userIDStr {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot share template with yourself"})
		return
	}

	// Check if share already exists
	var existingID string
	checkQuery := `SELECT id FROM template_shares WHERE template_id = $1 AND `
	var checkArgs []interface{}
	checkArgs = append(checkArgs, templateID)

	if req.SharedWithUserID != nil {
		checkQuery += `shared_with_user_id = $2 AND revoked_at IS NULL`
		checkArgs = append(checkArgs, *req.SharedWithUserID)
	} else {
		checkQuery += `shared_with_team_id = $2 AND revoked_at IS NULL`
		checkArgs = append(checkArgs, *req.SharedWithTeamID)
	}

	err := h.db.DB().QueryRowContext(ctx, checkQuery, checkArgs...).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Template is already shared with this user/team"})
		return
	} else if err != sql.ErrNoRows {
		log.Printf("[ERROR] Failed to check existing share: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing share"})
		return
	}

	// Create new share
	shareID := uuid.New().String()
	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO template_shares
		(id, template_id, shared_by, shared_with_user_id, shared_with_team_id, permission_level)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, shareID, templateID, userIDStr, req.SharedWithUserID, req.SharedWithTeamID, req.PermissionLevel)

	if err != nil {
		log.Printf("[ERROR] Failed to create template share: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share template"})
		return
	}

	// Log audit event
	targetType := "user"
	targetID := ""
	if req.SharedWithUserID != nil {
		targetID = *req.SharedWithUserID
	} else {
		targetType = "team"
		targetID = *req.SharedWithTeamID
	}

	log.Printf("[INFO] Template %s shared by %s with %s %s (permission: %s)",
		templateID, userIDStr, targetType, targetID, req.PermissionLevel)

	c.JSON(http.StatusOK, gin.H{
		"message": "Template shared successfully",
		"shareId": shareID,
	})
}

func (h *SessionTemplatesHandler) RevokeTemplateShare(c *gin.Context) {
	templateID := c.Param("id")
	shareID := c.Param("shareId")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Verify user owns the template or has manage permission
	if !h.canManageTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to revoke shares for this template"})
		return
	}

	// Verify share exists and belongs to this template
	var existingTemplateID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT template_id FROM template_shares WHERE id = $1 AND revoked_at IS NULL
	`, shareID).Scan(&existingTemplateID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Share not found or already revoked"})
		return
	} else if err != nil {
		log.Printf("[ERROR] Failed to check share: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify share"})
		return
	}

	if existingTemplateID != templateID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Share does not belong to this template"})
		return
	}

	// Revoke the share (soft delete)
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE template_shares SET revoked_at = CURRENT_TIMESTAMP WHERE id = $1
	`, shareID)

	if err != nil {
		log.Printf("[ERROR] Failed to revoke template share: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke share"})
		return
	}

	log.Printf("[INFO] Template share %s revoked by %s for template %s", shareID, userIDStr, templateID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Share revoked successfully",
	})
}

// canManageTemplate checks if user owns the template or has manage permission
func (h *SessionTemplatesHandler) canManageTemplate(ctx context.Context, templateID, userID string) bool {
	// Check if user is the owner
	var ownerID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT user_id FROM user_session_templates WHERE id = $1
	`, templateID).Scan(&ownerID)

	if err == nil && ownerID == userID {
		return true
	}

	// Check if user has manage permission through a share
	var shareID string
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT id FROM template_shares
		WHERE template_id = $1
		AND shared_with_user_id = $2
		AND permission_level = 'manage'
		AND revoked_at IS NULL
	`, templateID, userID).Scan(&shareID)

	return err == nil
}

// Template versioning implementations

// TemplateSnapshot represents a version snapshot of a template
type TemplateSnapshot struct {
	ID            int                    `json:"id"`
	TemplateID    string                 `json:"templateId"`
	VersionNumber int                    `json:"versionNumber"`
	TemplateData  map[string]interface{} `json:"templateData"`
	Description   string                 `json:"description,omitempty"`
	CreatedBy     string                 `json:"createdBy"`
	CreatedAt     time.Time              `json:"createdAt"`
	Tags          []string               `json:"tags,omitempty"`
}

func (h *SessionTemplatesHandler) ListTemplateVersions(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Verify user has access to the template
	if !h.canAccessTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view versions for this template"})
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 50
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := fmt.Sscanf(pageStr, "%d", &page); err == nil && p == 1 && page > 0 {
			// page is valid
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l == 1 && limit > 0 && limit <= 100 {
			// limit is valid
		}
	}
	offset := (page - 1) * limit

	// Query template versions with pagination
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, template_id, version_number, template_data, description, created_by, created_at, tags
		FROM user_session_template_versions
		WHERE template_id = $1
		ORDER BY version_number DESC
		LIMIT $2 OFFSET $3
	`, templateID, limit, offset)

	if err != nil {
		log.Printf("[ERROR] Failed to list template versions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list versions"})
		return
	}
	defer rows.Close()

	versions := []TemplateSnapshot{}
	for rows.Next() {
		var version TemplateSnapshot
		var templateDataJSON []byte
		var tagsArray sql.NullString

		err := rows.Scan(
			&version.ID, &version.TemplateID, &version.VersionNumber,
			&templateDataJSON, &version.Description, &version.CreatedBy,
			&version.CreatedAt, &tagsArray,
		)

		if err != nil {
			log.Printf("[ERROR] Failed to scan version: %v", err)
			continue
		}

		// Parse JSONB template data
		if err := json.Unmarshal(templateDataJSON, &version.TemplateData); err != nil {
			log.Printf("[ERROR] Failed to unmarshal template data: %v", err)
			continue
		}

		// Parse tags array
		if tagsArray.Valid && tagsArray.String != "" {
			// PostgreSQL array format: {tag1,tag2,tag3}
			tagsStr := tagsArray.String
			if len(tagsStr) > 2 && tagsStr[0] == '{' && tagsStr[len(tagsStr)-1] == '}' {
				tagsStr = tagsStr[1 : len(tagsStr)-1]
				if tagsStr != "" {
					version.Tags = splitPostgresArray(tagsStr)
				}
			}
		}

		versions = append(versions, version)
	}

	// Get total count
	var totalCount int
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_session_template_versions WHERE template_id = $1
	`, templateID).Scan(&totalCount)

	if err != nil {
		log.Printf("[ERROR] Failed to get version count: %v", err)
		totalCount = len(versions)
	}

	c.JSON(http.StatusOK, gin.H{
		"versions":   versions,
		"count":      len(versions),
		"totalCount": totalCount,
		"page":       page,
		"limit":      limit,
	})
}

func (h *SessionTemplatesHandler) CreateTemplateVersion(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ctx := context.Background()

	// Verify user has write access to the template
	if !h.canModifyTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to create versions for this template"})
		return
	}

	// Get current template data
	var templateData map[string]interface{}
	var templateDataJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT row_to_json(t)
		FROM (
			SELECT name, description, icon, category, tags, visibility,
			       base_template, configuration, resources, environment,
			       is_default, version
			FROM user_session_templates
			WHERE id = $1
		) t
	`, templateID).Scan(&templateDataJSON)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	} else if err != nil {
		log.Printf("[ERROR] Failed to get template data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve template"})
		return
	}

	// Parse template data
	if err := json.Unmarshal(templateDataJSON, &templateData); err != nil {
		log.Printf("[ERROR] Failed to unmarshal template data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse template data"})
		return
	}

	// Get next version number
	var nextVersion int
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM user_session_template_versions
		WHERE template_id = $1
	`, templateID).Scan(&nextVersion)

	if err != nil {
		log.Printf("[ERROR] Failed to get next version number: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine version number"})
		return
	}

	// Convert template data back to JSON for storage
	templateDataBytes, err := json.Marshal(templateData)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal template data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save template data"})
		return
	}

	// Convert tags to PostgreSQL array format
	tagsArray := "{}"
	if len(req.Tags) > 0 {
		tagsArray = "{" + joinPostgresArray(req.Tags) + "}"
	}

	// Insert new version
	var versionID int
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO user_session_template_versions
		(template_id, version_number, template_data, description, created_by, tags)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, templateID, nextVersion, string(templateDataBytes), req.Description, userIDStr, tagsArray).Scan(&versionID)

	if err != nil {
		log.Printf("[ERROR] Failed to create template version: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create version"})
		return
	}

	log.Printf("[INFO] Created template version %d for template %s by user %s", nextVersion, templateID, userIDStr)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Version created successfully",
		"versionId":     versionID,
		"versionNumber": nextVersion,
	})
}

func (h *SessionTemplatesHandler) RestoreTemplateVersion(c *gin.Context) {
	templateID := c.Param("id")
	versionStr := c.Param("version")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	// Parse version number
	var versionNumber int
	if _, err := fmt.Sscanf(versionStr, "%d", &versionNumber); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version number"})
		return
	}

	ctx := context.Background()

	// Verify user has write access to the template
	if !h.canModifyTemplate(ctx, templateID, userIDStr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to restore versions for this template"})
		return
	}

	// Get the version data
	var templateDataJSON []byte
	var description string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT template_data, description
		FROM user_session_template_versions
		WHERE template_id = $1 AND version_number = $2
	`, templateID, versionNumber).Scan(&templateDataJSON, &description)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	} else if err != nil {
		log.Printf("[ERROR] Failed to get version data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve version"})
		return
	}

	// Parse version data
	var versionData map[string]interface{}
	if err := json.Unmarshal(templateDataJSON, &versionData); err != nil {
		log.Printf("[ERROR] Failed to unmarshal version data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse version data"})
		return
	}

	// Create a backup version before restoring (safety mechanism)
	backupDescription := fmt.Sprintf("Auto-backup before restoring version %d", versionNumber)
	_, createErr := h.createVersionSnapshot(ctx, templateID, userIDStr, backupDescription, []string{"auto-backup"})
	if createErr != nil {
		log.Printf("[WARN] Failed to create backup version: %v", createErr)
		// Continue anyway - backup is optional safety measure
	}

	// Update the template with version data
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates
		SET name = $2,
		    description = $3,
		    icon = $4,
		    category = $5,
		    tags = $6,
		    visibility = $7,
		    base_template = $8,
		    configuration = $9,
		    resources = $10,
		    environment = $11,
		    is_default = $12,
		    version = $13,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`,
		templateID,
		versionData["name"],
		versionData["description"],
		versionData["icon"],
		versionData["category"],
		versionData["tags"],
		versionData["visibility"],
		versionData["base_template"],
		versionData["configuration"],
		versionData["resources"],
		versionData["environment"],
		versionData["is_default"],
		versionData["version"],
	)

	if err != nil {
		log.Printf("[ERROR] Failed to restore template version: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore version"})
		return
	}

	log.Printf("[INFO] Restored template %s to version %d by user %s", templateID, versionNumber, userIDStr)

	c.JSON(http.StatusOK, gin.H{
		"message": "Version restored successfully",
		"version": versionNumber,
	})
}

// Helper functions

// canAccessTemplate checks if user can view the template
func (h *SessionTemplatesHandler) canAccessTemplate(ctx context.Context, templateID, userID string) bool {
	// Check if user is the owner
	var ownerID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT user_id FROM user_session_templates WHERE id = $1
	`, templateID).Scan(&ownerID)

	if err == nil && ownerID == userID {
		return true
	}

	// Check if user has access through a share
	var shareID string
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT id FROM template_shares
		WHERE template_id = $1
		AND shared_with_user_id = $2
		AND revoked_at IS NULL
	`, templateID, userID).Scan(&shareID)

	return err == nil
}

// canModifyTemplate checks if user can modify the template (write or manage permission)
func (h *SessionTemplatesHandler) canModifyTemplate(ctx context.Context, templateID, userID string) bool {
	// Check if user is the owner
	var ownerID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT user_id FROM user_session_templates WHERE id = $1
	`, templateID).Scan(&ownerID)

	if err == nil && ownerID == userID {
		return true
	}

	// Check if user has write or manage permission through a share
	var shareID string
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT id FROM template_shares
		WHERE template_id = $1
		AND shared_with_user_id = $2
		AND permission_level IN ('write', 'manage')
		AND revoked_at IS NULL
	`, templateID, userID).Scan(&shareID)

	return err == nil
}

// createVersionSnapshot creates a version snapshot (used internally for backups)
func (h *SessionTemplatesHandler) createVersionSnapshot(ctx context.Context, templateID, userID, description string, tags []string) (int, error) {
	// Get current template data
	var templateDataJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT row_to_json(t)
		FROM (
			SELECT name, description, icon, category, tags, visibility,
			       base_template, configuration, resources, environment,
			       is_default, version
			FROM user_session_templates
			WHERE id = $1
		) t
	`, templateID).Scan(&templateDataJSON)

	if err != nil {
		return 0, fmt.Errorf("failed to get template data: %w", err)
	}

	// Get next version number
	var nextVersion int
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM user_session_template_versions
		WHERE template_id = $1
	`, templateID).Scan(&nextVersion)

	if err != nil {
		return 0, fmt.Errorf("failed to get next version number: %w", err)
	}

	// Convert tags to PostgreSQL array format
	tagsArray := "{}"
	if len(tags) > 0 {
		tagsArray = "{" + joinPostgresArray(tags) + "}"
	}

	// Insert new version
	var versionID int
	err = h.db.DB().QueryRowContext(ctx, `
		INSERT INTO user_session_template_versions
		(template_id, version_number, template_data, description, created_by, tags)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, templateID, nextVersion, string(templateDataJSON), description, userID, tagsArray).Scan(&versionID)

	if err != nil {
		return 0, fmt.Errorf("failed to create version: %w", err)
	}

	return versionID, nil
}

// splitPostgresArray splits a PostgreSQL array string into a Go slice
func splitPostgresArray(s string) []string {
	if s == "" {
		return []string{}
	}
	// Simple split by comma (assumes no commas in values)
	parts := []string{}
	for _, part := range splitByComma(s) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// joinPostgresArray joins a Go slice into a PostgreSQL array string
func joinPostgresArray(tags []string) string {
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		// Escape quotes in tag
		escaped := strings.ReplaceAll(tag, `"`, `\"`)
		quoted[i] = `"` + escaped + `"`
	}
	return strings.Join(quoted, ",")
}

// splitByComma splits a string by comma
func splitByComma(s string) []string {
	result := []string{}
	current := ""
	inQuotes := false
	for _, ch := range s {
		if ch == '"' {
			inQuotes = !inQuotes
		} else if ch == ',' && !inQuotes {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
