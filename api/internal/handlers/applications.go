// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements installed application management endpoints.
//
// APPLICATION FEATURES:
// - Install applications from catalog templates
// - Custom display names for user dashboards
// - Application configuration management
// - Enable/disable applications
// - Group-based access control
//
// ACCESS CONTROL:
// - Grant/revoke group access to applications
// - Multiple access levels (view, launch, admin)
// - Filter applications by user's group membership
//
// API Endpoints:
// - GET    /api/v1/applications - List all installed applications
// - POST   /api/v1/applications - Install a new application
// - GET    /api/v1/applications/:id - Get application details
// - PUT    /api/v1/applications/:id - Update application
// - DELETE /api/v1/applications/:id - Delete application
// - PUT    /api/v1/applications/:id/enabled - Enable/disable application
// - GET    /api/v1/applications/:id/groups - Get groups with access
// - POST   /api/v1/applications/:id/groups - Add group access
// - PUT    /api/v1/applications/:id/groups/:groupId - Update group access level
// - DELETE /api/v1/applications/:id/groups/:groupId - Remove group access
// - GET    /api/v1/applications/:id/config - Get template config options
// - GET    /api/v1/applications/user - Get applications accessible to current user
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: installed_applications, application_group_access tables
//
// Example Usage:
//
//	handler := NewApplicationHandler(database, k8sClient, "streamspace")
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/models"
	"gopkg.in/yaml.v3"
)

// ApplicationHandler handles installed application endpoints
type ApplicationHandler struct {
	db        *db.Database
	appDB     *db.ApplicationDB
	k8sClient *k8s.Client
	namespace string
}

// NewApplicationHandler creates a new application handler
func NewApplicationHandler(database *db.Database, k8sClient *k8s.Client, namespace string) *ApplicationHandler {
	return &ApplicationHandler{
		db:        database,
		appDB:     db.NewApplicationDB(database.DB()),
		k8sClient: k8sClient,
		namespace: namespace,
	}
}

// RegisterRoutes registers application-related routes
func (h *ApplicationHandler) RegisterRoutes(router *gin.RouterGroup) {
	apps := router.Group("/applications")
	{
		apps.GET("", h.ListApplications)
		apps.POST("", h.InstallApplication)
		apps.GET("/user", h.GetUserApplications)
		apps.GET("/:id", h.GetApplication)
		apps.PUT("/:id", h.UpdateApplication)
		apps.DELETE("/:id", h.DeleteApplication)
		apps.PUT("/:id/enabled", h.SetApplicationEnabled)
		apps.GET("/:id/groups", h.GetApplicationGroups)
		apps.POST("/:id/groups", h.AddGroupAccess)
		apps.PUT("/:id/groups/:groupId", h.UpdateGroupAccess)
		apps.DELETE("/:id/groups/:groupId", h.RemoveGroupAccess)
		apps.GET("/:id/config", h.GetTemplateConfig)
	}
}

// ListApplications godoc
// @Summary List all installed applications
// @Description Get all installed applications with optional filtering
// @Tags applications
// @Accept json
// @Produce json
// @Param enabled query boolean false "Filter by enabled status"
// @Success 200 {object} models.ApplicationListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications [get]
func (h *ApplicationHandler) ListApplications(c *gin.Context) {
	enabledOnly := c.Query("enabled") == "true"

	apps, err := h.appDB.ListApplications(c.Request.Context(), enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Get group access for each application
	for _, app := range apps {
		groups, err := h.appDB.GetApplicationGroups(c.Request.Context(), app.ID)
		if err == nil {
			app.Groups = groups
		}
	}

	c.JSON(http.StatusOK, models.ApplicationListResponse{
		Applications: apps,
		Total:        len(apps),
	})
}

// InstallApplication godoc
// @Summary Install a new application
// @Description Install an application from the catalog
// @Tags applications
// @Accept json
// @Produce json
// @Param request body models.InstallApplicationRequest true "Installation request"
// @Success 201 {object} models.InstalledApplication
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications [post]
func (h *ApplicationHandler) InstallApplication(c *gin.Context) {
	ctx := c.Request.Context()

	var req models.InstallApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	// Get template manifest from catalog
	var manifest, name, displayName, description, category, iconURL string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT manifest, name, display_name, description, category, COALESCE(icon_url, '')
		FROM catalog_templates
		WHERE id = $1
	`, req.CatalogTemplateID).Scan(&manifest, &name, &displayName, &description, &category, &iconURL)

	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Catalog template not found",
			Message: err.Error(),
		})
		return
	}

	// Parse the YAML manifest to get template configuration
	var templateData map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest), &templateData); err != nil {
		log.Printf("Error parsing template manifest: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid template manifest",
			Message: err.Error(),
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
		Icon:        iconURL,
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

	// Create Template CRD in Kubernetes (if k8sClient is available)
	if h.k8sClient != nil {
		_, err = h.k8sClient.CreateTemplate(ctx, template)
		if err != nil {
			// Check if template already exists (might have been installed previously)
			log.Printf("Note: Could not create K8s template %s: %v (may already exist)", name, err)
			// Don't fail - the template might already exist from a previous installation
		}
	}

	// Create database record
	app, err := h.appDB.InstallApplication(ctx, &req, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Installation failed",
			Message: err.Error(),
		})
		return
	}

	// Grant initial group access if specified
	for _, groupID := range req.GroupIDs {
		h.appDB.AddGroupAccess(ctx, app.ID, groupID, "launch")
	}

	// Get full application with template info
	fullApp, err := h.appDB.GetApplication(ctx, app.ID)
	if err == nil {
		app = fullApp
	}

	// Get group access
	groups, err := h.appDB.GetApplicationGroups(ctx, app.ID)
	if err == nil {
		app.Groups = groups
	}

	c.JSON(http.StatusCreated, app)
}

// GetApplication godoc
// @Summary Get application details
// @Description Get detailed information about an installed application
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} models.InstalledApplication
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/applications/{id} [get]
func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	appID := c.Param("id")

	app, err := h.appDB.GetApplication(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Application not found",
			Message: err.Error(),
		})
		return
	}

	// Get group access
	groups, err := h.appDB.GetApplicationGroups(c.Request.Context(), appID)
	if err == nil {
		app.Groups = groups
	}

	c.JSON(http.StatusOK, app)
}

// UpdateApplication godoc
// @Summary Update an application
// @Description Update display name, configuration, or enabled status
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body models.UpdateApplicationRequest true "Update request"
// @Success 200 {object} models.InstalledApplication
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/applications/{id} [put]
func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	appID := c.Param("id")

	var req models.UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	err := h.appDB.UpdateApplication(c.Request.Context(), appID, &req)
	if err != nil {
		if err.Error() == "application not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Application not found",
				Message: "The application does not exist or was deleted",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Update failed",
			Message: err.Error(),
		})
		return
	}

	// Return updated application
	app, err := h.appDB.GetApplication(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Application not found",
			Message: err.Error(),
		})
		return
	}

	// Get group access
	groups, err := h.appDB.GetApplicationGroups(c.Request.Context(), appID)
	if err == nil {
		app.Groups = groups
	}

	c.JSON(http.StatusOK, app)
}

// DeleteApplication godoc
// @Summary Delete an application
// @Description Remove an installed application and all its access rules
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications/{id} [delete]
func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	appID := c.Param("id")

	err := h.appDB.DeleteApplication(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Delete failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Application deleted successfully",
	})
}

// SetApplicationEnabled godoc
// @Summary Enable or disable an application
// @Description Toggle the application's enabled status
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body object true "Enabled status"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/applications/{id}/enabled [put]
func (h *ApplicationHandler) SetApplicationEnabled(c *gin.Context) {
	appID := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	err := h.appDB.SetApplicationEnabled(c.Request.Context(), appID, req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Update failed",
			Message: err.Error(),
		})
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Application " + status + " successfully",
		"enabled": req.Enabled,
	})
}

// GetApplicationGroups godoc
// @Summary Get groups with access to an application
// @Description List all groups that have access to this application
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications/{id}/groups [get]
func (h *ApplicationHandler) GetApplicationGroups(c *gin.Context) {
	appID := c.Param("id")

	groups, err := h.appDB.GetApplicationGroups(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// AddGroupAccess godoc
// @Summary Grant group access to an application
// @Description Add a group with specified access level
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body models.AddGroupAccessRequest true "Access request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/applications/{id}/groups [post]
func (h *ApplicationHandler) AddGroupAccess(c *gin.Context) {
	appID := c.Param("id")

	var req models.AddGroupAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	accessLevel := req.AccessLevel
	if accessLevel == "" {
		accessLevel = "launch"
	}

	err := h.appDB.AddGroupAccess(c.Request.Context(), appID, req.GroupID, accessLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add access",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Group access granted successfully",
	})
}

// UpdateGroupAccess godoc
// @Summary Update group access level
// @Description Change a group's access level for an application
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param groupId path string true "Group ID"
// @Param request body models.UpdateGroupAccessRequest true "Access level"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/applications/{id}/groups/{groupId} [put]
func (h *ApplicationHandler) UpdateGroupAccess(c *gin.Context) {
	appID := c.Param("id")
	groupID := c.Param("groupId")

	var req models.UpdateGroupAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	err := h.appDB.UpdateGroupAccessLevel(c.Request.Context(), appID, groupID, req.AccessLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update access",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Group access updated successfully",
	})
}

// RemoveGroupAccess godoc
// @Summary Remove group access from an application
// @Description Revoke a group's access to an application
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param groupId path string true "Group ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications/{id}/groups/{groupId} [delete]
func (h *ApplicationHandler) RemoveGroupAccess(c *gin.Context) {
	appID := c.Param("id")
	groupID := c.Param("groupId")

	err := h.appDB.RemoveGroupAccess(c.Request.Context(), appID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to remove access",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Group access removed successfully",
	})
}

// GetTemplateConfig godoc
// @Summary Get application template configuration options
// @Description Get the configurable options from the template manifest
// @Tags applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications/{id}/config [get]
func (h *ApplicationHandler) GetTemplateConfig(c *gin.Context) {
	appID := c.Param("id")

	config, err := h.appDB.GetApplicationTemplateConfig(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get config",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// GetUserApplications godoc
// @Summary Get applications accessible to current user
// @Description Get all applications the user can access via their groups
// @Tags applications
// @Accept json
// @Produce json
// @Success 200 {object} models.ApplicationListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications/user [get]
func (h *ApplicationHandler) GetUserApplications(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	apps, err := h.appDB.GetUserAccessibleApplications(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ApplicationListResponse{
		Applications: apps,
		Total:        len(apps),
	})
}
