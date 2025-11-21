// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements platform controller management.
//
// PLATFORM CONTROLLERS:
// - Register and manage distributed controllers (K8s, Docker, Hyper-V, vCenter, etc.)
// - Monitor controller health and heartbeat
// - Track controller capabilities and cluster information
// - Enable multi-platform workload distribution
//
// CONTROLLER TYPES:
// - kubernetes: Kubernetes platform controller
// - docker: Docker platform controller
// - hyperv: Hyper-V platform controller
// - vcenter: VMware vCenter controller
// - custom: Custom platform integrations
//
// CONTROLLER STATUS:
// - connected: Controller is online and responding
// - disconnected: Controller is offline or not responding
// - unknown: Controller status is unknown
//
// API Endpoints:
// - GET    /api/v1/admin/controllers - List all controllers
// - GET    /api/v1/admin/controllers/:id - Get controller details
// - POST   /api/v1/admin/controllers/register - Register new controller
// - PUT    /api/v1/admin/controllers/:id - Update controller
// - DELETE /api/v1/admin/controllers/:id - Unregister controller
// - POST   /api/v1/admin/controllers/:id/heartbeat - Update heartbeat
//
// Thread Safety:
// - All database operations are thread-safe
//
// Dependencies:
// - Database: platform_controllers table
//
// Example Usage:
//
//	handler := NewControllerHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1/admin"))
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
)

// ControllerHandler handles platform controller management
type ControllerHandler struct {
	database *db.Database
}

// NewControllerHandler creates a new controller handler
func NewControllerHandler(database *db.Database) *ControllerHandler {
	return &ControllerHandler{
		database: database,
	}
}

// RegisterRoutes registers controller routes
func (h *ControllerHandler) RegisterRoutes(router *gin.RouterGroup) {
	controllers := router.Group("/controllers")
	{
		controllers.GET("", h.ListControllers)
		controllers.GET("/:id", h.GetController)
		controllers.POST("/register", h.RegisterController)
		controllers.PUT("/:id", h.UpdateController)
		controllers.DELETE("/:id", h.UnregisterController)
		controllers.POST("/:id/heartbeat", h.UpdateHeartbeat)
	}
}

// Controller represents a platform controller
type Controller struct {
	ID            string                 `json:"id"`
	ControllerID  string                 `json:"controller_id"`
	Platform      string                 `json:"platform"`
	DisplayName   string                 `json:"display_name"`
	Status        string                 `json:"status"`
	Version       string                 `json:"version"`
	Capabilities  []string               `json:"capabilities"`
	ClusterInfo   map[string]interface{} `json:"cluster_info"`
	LastHeartbeat *time.Time             `json:"last_heartbeat"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ListControllers godoc
// @Summary List all platform controllers
// @Description Retrieves all registered platform controllers with their status
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param platform query string false "Filter by platform (kubernetes, docker, hyperv, etc.)"
// @Param status query string false "Filter by status (connected, disconnected, unknown)"
// @Success 200 {array} Controller
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers [get]
func (h *ControllerHandler) ListControllers(c *gin.Context) {
	ctx := context.Background()

	// Build query with optional filters
	query := `
		SELECT id, controller_id, platform, display_name, status, version,
		       capabilities, cluster_info, last_heartbeat, created_at, updated_at
		FROM platform_controllers
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	// Filter by platform
	if platform := c.Query("platform"); platform != "" {
		query += fmt.Sprintf(" AND platform = $%d", argIndex)
		args = append(args, platform)
		argIndex++
	}

	// Filter by status
	if status := c.Query("status"); status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.database.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve controllers",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	controllers := []Controller{}
	for rows.Next() {
		var ctrl Controller
		var capabilitiesJSON, clusterInfoJSON []byte

		err := rows.Scan(
			&ctrl.ID,
			&ctrl.ControllerID,
			&ctrl.Platform,
			&ctrl.DisplayName,
			&ctrl.Status,
			&ctrl.Version,
			&capabilitiesJSON,
			&clusterInfoJSON,
			&ctrl.LastHeartbeat,
			&ctrl.CreatedAt,
			&ctrl.UpdatedAt,
		)
		if err != nil {
			continue
		}

		// Parse JSONB fields
		if err := json.Unmarshal(capabilitiesJSON, &ctrl.Capabilities); err != nil {
			ctrl.Capabilities = []string{}
		}
		if err := json.Unmarshal(clusterInfoJSON, &ctrl.ClusterInfo); err != nil {
			ctrl.ClusterInfo = make(map[string]interface{})
		}

		controllers = append(controllers, ctrl)
	}

	c.JSON(http.StatusOK, controllers)
}

// GetController godoc
// @Summary Get controller details
// @Description Retrieves details of a specific controller
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param id path string true "Controller ID"
// @Success 200 {object} Controller
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers/{id} [get]
func (h *ControllerHandler) GetController(c *gin.Context) {
	controllerID := c.Param("id")
	ctx := context.Background()

	query := `
		SELECT id, controller_id, platform, display_name, status, version,
		       capabilities, cluster_info, last_heartbeat, created_at, updated_at
		FROM platform_controllers
		WHERE id = $1 OR controller_id = $1
	`

	var ctrl Controller
	var capabilitiesJSON, clusterInfoJSON []byte

	err := h.database.DB().QueryRowContext(ctx, query, controllerID).Scan(
		&ctrl.ID,
		&ctrl.ControllerID,
		&ctrl.Platform,
		&ctrl.DisplayName,
		&ctrl.Status,
		&ctrl.Version,
		&capabilitiesJSON,
		&clusterInfoJSON,
		&ctrl.LastHeartbeat,
		&ctrl.CreatedAt,
		&ctrl.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Controller not found",
				Message: fmt.Sprintf("No controller found with ID %s", controllerID),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve controller",
			Message: err.Error(),
		})
		return
	}

	// Parse JSONB fields
	if err := json.Unmarshal(capabilitiesJSON, &ctrl.Capabilities); err != nil {
		ctrl.Capabilities = []string{}
	}
	if err := json.Unmarshal(clusterInfoJSON, &ctrl.ClusterInfo); err != nil {
		ctrl.ClusterInfo = make(map[string]interface{})
	}

	c.JSON(http.StatusOK, ctrl)
}

// RegisterControllerRequest represents controller registration request
type RegisterControllerRequest struct {
	ControllerID string                 `json:"controller_id" binding:"required"`
	Platform     string                 `json:"platform" binding:"required"`
	DisplayName  string                 `json:"display_name"`
	Version      string                 `json:"version"`
	Capabilities []string               `json:"capabilities"`
	ClusterInfo  map[string]interface{} `json:"cluster_info"`
}

// RegisterController godoc
// @Summary Register a new controller
// @Description Registers a new platform controller
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param body body RegisterControllerRequest true "Controller registration data"
// @Success 201 {object} Controller
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers/register [post]
func (h *ControllerHandler) RegisterController(c *gin.Context) {
	var req RegisterControllerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx := context.Background()

	// Check if controller already exists
	var existingID string
	err := h.database.DB().QueryRowContext(ctx,
		"SELECT id FROM platform_controllers WHERE controller_id = $1",
		req.ControllerID,
	).Scan(&existingID)

	if err != sql.ErrNoRows {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "Controller already registered",
			Message: fmt.Sprintf("Controller with ID %s already exists", req.ControllerID),
		})
		return
	}

	// Generate ID
	id := fmt.Sprintf("ctrl_%d", time.Now().UnixNano())

	// Set display name if not provided
	displayName := req.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("%s Controller", req.Platform)
	}

	// Prepare JSONB fields
	capabilitiesJSON, _ := json.Marshal(req.Capabilities)
	if capabilitiesJSON == nil {
		capabilitiesJSON = []byte("[]")
	}

	clusterInfoJSON, _ := json.Marshal(req.ClusterInfo)
	if clusterInfoJSON == nil {
		clusterInfoJSON = []byte("{}")
	}

	// Insert controller
	now := time.Now()
	_, err = h.database.DB().ExecContext(ctx, `
		INSERT INTO platform_controllers (
			id, controller_id, platform, display_name, status, version,
			capabilities, cluster_info, last_heartbeat, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		id, req.ControllerID, req.Platform, displayName, "connected", req.Version,
		capabilitiesJSON, clusterInfoJSON, now, now, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to register controller",
			Message: err.Error(),
		})
		return
	}

	// Return created controller
	ctrl := Controller{
		ID:            id,
		ControllerID:  req.ControllerID,
		Platform:      req.Platform,
		DisplayName:   displayName,
		Status:        "connected",
		Version:       req.Version,
		Capabilities:  req.Capabilities,
		ClusterInfo:   req.ClusterInfo,
		LastHeartbeat: &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	c.JSON(http.StatusCreated, ctrl)
}

// UpdateControllerRequest represents controller update request
type UpdateControllerRequest struct {
	DisplayName  *string                 `json:"display_name,omitempty"`
	Status       *string                 `json:"status,omitempty"`
	Version      *string                 `json:"version,omitempty"`
	Capabilities []string                `json:"capabilities,omitempty"`
	ClusterInfo  map[string]interface{}  `json:"cluster_info,omitempty"`
}

// UpdateController godoc
// @Summary Update controller
// @Description Updates controller metadata and status
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param id path string true "Controller ID"
// @Param body body UpdateControllerRequest true "Controller update data"
// @Success 200 {object} Controller
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers/{id} [put]
func (h *ControllerHandler) UpdateController(c *gin.Context) {
	controllerID := c.Param("id")

	var req UpdateControllerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx := context.Background()

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.DisplayName != nil {
		updates = append(updates, fmt.Sprintf("display_name = $%d", argIndex))
		args = append(args, *req.DisplayName)
		argIndex++
	}

	if req.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *req.Status)
		argIndex++
	}

	if req.Version != nil {
		updates = append(updates, fmt.Sprintf("version = $%d", argIndex))
		args = append(args, *req.Version)
		argIndex++
	}

	if req.Capabilities != nil {
		capabilitiesJSON, _ := json.Marshal(req.Capabilities)
		updates = append(updates, fmt.Sprintf("capabilities = $%d", argIndex))
		args = append(args, capabilitiesJSON)
		argIndex++
	}

	if req.ClusterInfo != nil {
		clusterInfoJSON, _ := json.Marshal(req.ClusterInfo)
		updates = append(updates, fmt.Sprintf("cluster_info = $%d", argIndex))
		args = append(args, clusterInfoJSON)
		argIndex++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "No fields to update",
			Message: "Request must contain at least one field to update",
		})
		return
	}

	// Add updated_at
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	// Add controller ID
	args = append(args, controllerID)

	query := fmt.Sprintf(`
		UPDATE platform_controllers
		SET %s
		WHERE id = $%d OR controller_id = $%d
	`, strings.Join(updates, ", "), argIndex, argIndex)

	result, err := h.database.DB().ExecContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update controller",
			Message: err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Controller not found",
			Message: fmt.Sprintf("No controller found with ID %s", controllerID),
		})
		return
	}

	// Fetch and return updated controller
	h.GetController(c)
}

// UnregisterController godoc
// @Summary Unregister controller
// @Description Removes a controller from the platform
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param id path string true "Controller ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers/{id} [delete]
func (h *ControllerHandler) UnregisterController(c *gin.Context) {
	controllerID := c.Param("id")
	ctx := context.Background()

	result, err := h.database.DB().ExecContext(ctx,
		"DELETE FROM platform_controllers WHERE id = $1 OR controller_id = $1",
		controllerID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to unregister controller",
			Message: err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Controller not found",
			Message: fmt.Sprintf("No controller found with ID %s", controllerID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Controller unregistered successfully",
		"id":      controllerID,
	})
}

// UpdateHeartbeat godoc
// @Summary Update controller heartbeat
// @Description Updates the last heartbeat timestamp for a controller
// @Tags admin, controllers
// @Accept json
// @Produce json
// @Param id path string true "Controller ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/controllers/{id}/heartbeat [post]
func (h *ControllerHandler) UpdateHeartbeat(c *gin.Context) {
	controllerID := c.Param("id")
	ctx := context.Background()

	now := time.Now()
	result, err := h.database.DB().ExecContext(ctx, `
		UPDATE platform_controllers
		SET last_heartbeat = $1, status = 'connected', updated_at = $1
		WHERE id = $2 OR controller_id = $2
	`, now, controllerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update heartbeat",
			Message: err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Controller not found",
			Message: fmt.Sprintf("No controller found with ID %s", controllerID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Heartbeat updated successfully",
		"last_heartbeat": now,
	})
}
