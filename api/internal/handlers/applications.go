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
//	handler := NewApplicationHandler(database, publisher, "kubernetes")
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/events"
	"github.com/streamspace/streamspace/api/internal/k8s"
	"github.com/streamspace/streamspace/api/internal/models"
)

// ApplicationHandler handles installed application endpoints
type ApplicationHandler struct {
	db        *db.Database
	appDB     *db.ApplicationDB
	publisher *events.Publisher
	k8sClient *k8s.Client
	platform  string
	namespace string
}

// NewApplicationHandler creates a new application handler
func NewApplicationHandler(database *db.Database, publisher *events.Publisher, k8sClient *k8s.Client, platform string) *ApplicationHandler {
	if platform == "" {
		platform = events.PlatformKubernetes
	}
	namespace := "streamspace" // Default namespace
	return &ApplicationHandler{
		db:        database,
		appDB:     db.NewApplicationDB(database.DB()),
		publisher: publisher,
		k8sClient: k8sClient,
		platform:  platform,
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
		apps.GET("/:id/icon", h.GetApplicationIcon)
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

// updateInstallStatus updates the installation status of an application in the database
func (h *ApplicationHandler) updateInstallStatus(ctx context.Context, appID, status, message string) {
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE installed_applications
		SET install_status = $1, install_message = $2, updated_at = NOW()
		WHERE id = $3
	`, status, message, appID)
	if err != nil {
		log.Printf("Failed to update install status for %s: %v", appID, err)
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
// @Description Install an application from the catalog. This creates both a Kubernetes
// Template CRD and a database record for the installed application. The K8s Template
// is required for users to launch sessions from this application.
// @Tags applications
// @Accept json
// @Produce json
// @Param request body models.InstallApplicationRequest true "Installation request"
// @Success 201 {object} models.InstalledApplication
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/applications [post]
//
// Installation Flow:
// 1. Validate request and authenticate user
// 2. Fetch template manifest from catalog_templates database
// 3. Create installed_applications database record (status: pending)
// 4. Grant group access permissions if specified
// 5. Publish NATS event for controller to process
// 6. Return the created application with full details
//
// The controller subscribes to NATS events and creates platform-specific resources
// (Kubernetes Template CRD, Docker container, Hyper-V VM, etc.). This pattern
// decouples the API from platform-specific operations.
func (h *ApplicationHandler) InstallApplication(c *gin.Context) {
	ctx := c.Request.Context()

	// Step 1: Parse and validate the installation request
	var req models.InstallApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Step 1b: Get authenticated user ID from JWT context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	// Step 2: Fetch template manifest from catalog database
	// The manifest will be included in the ApplicationInstall for the controller to process
	var manifest, name, displayName, description, category, iconURL, templatePlatform string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT manifest, name, display_name, description, category, COALESCE(icon_url, ''), COALESCE(platform, 'kubernetes')
		FROM catalog_templates
		WHERE id = $1
	`, req.CatalogTemplateID).Scan(&manifest, &name, &displayName, &description, &category, &iconURL, &templatePlatform)

	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Catalog template not found",
			Message: err.Error(),
		})
		return
	}

	// Step 2b: Validate manifest is not empty (indicates repository sync issue)
	if manifest == "" {
		log.Printf("Warning: Empty manifest for template %s (id: %d)", name, req.CatalogTemplateID)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Template manifest is empty",
			Message: "The catalog template has no manifest data. Please sync the repository.",
		})
		return
	}

	// Step 3: Create database record in installed_applications table
	// The record is created with install_status = 'pending'
	app, err := h.appDB.InstallApplication(ctx, &req, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Installation failed",
			Message: err.Error(),
		})
		return
	}

	// Step 4: Grant initial group access permissions if specified in request
	for _, groupID := range req.GroupIDs {
		h.appDB.AddGroupAccess(ctx, app.ID, groupID, "launch")
	}

	// Step 5: Publish NATS event for controller to process
	// The controller will create the platform-specific resources (Template CRD, Docker container, etc.)
	// Use platform from request if specified, otherwise use template's platform
	targetPlatform := req.Platform
	if targetPlatform == "" {
		targetPlatform = templatePlatform
	}

	installEvent := &events.AppInstallEvent{
		InstallID:         app.ID,
		CatalogTemplateID: req.CatalogTemplateID,
		TemplateName:      name,
		DisplayName:       displayName,
		Description:       description,
		Category:          category,
		IconURL:           iconURL,
		Manifest:          manifest,
		InstalledBy:       userID.(string),
		Platform:          targetPlatform,
	}

	if err := h.publisher.PublishAppInstall(ctx, installEvent); err != nil {
		// Log error but don't fail - the database record exists and controller can retry
		log.Printf("Warning: Failed to publish app install event for %s: %v", app.ID, err)
		// Update install status to indicate event publishing failed
		h.updateInstallStatus(ctx, app.ID, events.InstallStatusPending, "Event publish failed, waiting for retry")
	} else {
		log.Printf("Published app install event for %s (controller will create resources)", app.ID)
	}

	// Step 7: Fetch complete application record with template info and group access
	fullApp, err := h.appDB.GetApplication(ctx, app.ID)
	if err == nil {
		app = fullApp
	}

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
	ctx := c.Request.Context()

	app, err := h.appDB.GetApplication(ctx, appID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Application not found",
			Message: err.Error(),
		})
		return
	}

	// Self-healing: Check if installation status needs to be updated
	// If status is 'pending' or 'creating', check if the Template CRD exists
	if app.InstallStatus == "pending" || app.InstallStatus == "creating" {
		if h.k8sClient != nil && app.TemplateName != "" {
			// Check if Template CRD exists in Kubernetes
			_, err := h.k8sClient.GetTemplate(ctx, h.namespace, app.TemplateName)
			if err == nil {
				// Template exists! Update status to installed
				h.updateInstallStatus(ctx, appID, "installed", "Template ready")
				app.InstallStatus = "installed"
				app.InstallMessage = "Template ready"
				log.Printf("Updated installation status for %s to installed (template found)", appID)
			}
		}
	}

	// Get group access
	groups, err := h.appDB.GetApplicationGroups(ctx, appID)
	if err == nil {
		app.Groups = groups
	}

	c.JSON(http.StatusOK, app)
}

// GetApplicationIcon godoc
// @Summary Get application icon
// @Description Get the icon image for an installed application
// @Tags applications
// @Produce image/png,image/svg+xml,image/jpeg
// @Param id path string true "Application ID"
// @Success 200 {file} binary
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/applications/{id}/icon [get]
func (h *ApplicationHandler) GetApplicationIcon(c *gin.Context) {
	appID := c.Param("id")
	ctx := c.Request.Context()

	// Get icon data from database
	var iconData []byte
	var iconMediaType string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT icon_data, COALESCE(icon_media_type, 'image/png')
		FROM installed_applications
		WHERE id = $1 AND icon_data IS NOT NULL
	`, appID).Scan(&iconData, &iconMediaType)

	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Icon not found",
			Message: "Application icon not available",
		})
		return
	}

	// Set appropriate headers
	c.Header("Content-Type", iconMediaType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.Data(http.StatusOK, iconMediaType, iconData)
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
	ctx := c.Request.Context()
	appID := c.Param("id")

	// Get application info before deleting (for the uninstall event)
	app, err := h.appDB.GetApplication(ctx, appID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Application not found",
			Message: err.Error(),
		})
		return
	}

	// Delete from database
	err = h.appDB.DeleteApplication(ctx, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Delete failed",
			Message: err.Error(),
		})
		return
	}

	// Publish uninstall event for controller to clean up platform resources
	uninstallEvent := &events.AppUninstallEvent{
		InstallID:    appID,
		TemplateName: app.TemplateName,
		Platform:     h.platform,
	}

	if err := h.publisher.PublishAppUninstall(ctx, uninstallEvent); err != nil {
		// Log error but don't fail - database record is already deleted
		log.Printf("Warning: Failed to publish app uninstall event for %s: %v", appID, err)
	} else {
		log.Printf("Published app uninstall event for %s", appID)
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
