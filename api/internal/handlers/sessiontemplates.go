package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// SessionTemplatesHandler handles custom session templates and presets
type SessionTemplatesHandler struct {
	db *db.Database
}

// NewSessionTemplatesHandler creates a new session templates handler
func NewSessionTemplatesHandler(database *db.Database) *SessionTemplatesHandler {
	return &SessionTemplatesHandler{
		db: database,
	}
}

// SessionTemplate represents a user-defined session configuration template
type SessionTemplate struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"userId"`
	TeamID      string                 `json:"teamId,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Visibility  string                 `json:"visibility"` // private, team, public
	BaseTemplate string                `json:"baseTemplate"` // Reference to catalog template
	Configuration map[string]interface{} `json:"configuration"`
	Resources   map[string]interface{} `json:"resources"`
	Environment map[string]string      `json:"environment,omitempty"`
	IsDefault   bool                   `json:"isDefault"`
	UsageCount  int                    `json:"usageCount"`
	Version     string                 `json:"version"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
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

// CreateSessionTemplate creates a new session template
func (h *SessionTemplatesHandler) CreateSessionTemplate(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Name          string                 `json:"name" binding:"required"`
		Description   string                 `json:"description"`
		Icon          string                 `json:"icon"`
		Category      string                 `json:"category"`
		Tags          []string               `json:"tags"`
		Visibility    string                 `json:"visibility"` // private, team, public
		TeamID        string                 `json:"teamId"`
		BaseTemplate  string                 `json:"baseTemplate" binding:"required"`
		Configuration map[string]interface{} `json:"configuration"`
		Resources     map[string]interface{} `json:"resources"`
		Environment   map[string]string      `json:"environment"`
		IsDefault     bool                   `json:"isDefault"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

// UpdateSessionTemplate updates a template
func (h *SessionTemplatesHandler) UpdateSessionTemplate(c *gin.Context) {
	templateID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Name          string                 `json:"name"`
		Description   string                 `json:"description"`
		Icon          string                 `json:"icon"`
		Category      string                 `json:"category"`
		Tags          []string               `json:"tags"`
		Configuration map[string]interface{} `json:"configuration"`
		Resources     map[string]interface{} `json:"resources"`
		Environment   map[string]string      `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

	ctx := context.Background()

	// Increment usage count
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE user_session_templates SET usage_count = usage_count + 1 WHERE id = $1
	`, templateID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to use template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template usage recorded",
		"templateId": templateID,
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

// Placeholder methods for sharing and versioning (simplified implementations)

func (h *SessionTemplatesHandler) ListTemplateShares(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"shares": []interface{}{}, "count": 0})
}

func (h *SessionTemplatesHandler) ShareSessionTemplate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Template shared"})
}

func (h *SessionTemplatesHandler) RevokeTemplateShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Share revoked"})
}

func (h *SessionTemplatesHandler) ListTemplateVersions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"versions": []interface{}{}, "count": 0})
}

func (h *SessionTemplatesHandler) CreateTemplateVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Version created"})
}

func (h *SessionTemplatesHandler) RestoreTemplateVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Version restored"})
}
