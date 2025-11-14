package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"streamspace/internal/nodes"
)

// NodeHandler handles node-related API requests
type NodeHandler struct {
	nodeManager *nodes.NodeManager
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(nodeManager *nodes.NodeManager) *NodeHandler {
	return &NodeHandler{
		nodeManager: nodeManager,
	}
}

// RegisterRoutes registers node management routes
func (h *NodeHandler) RegisterRoutes(router *gin.RouterGroup) {
	nodeRoutes := router.Group("/nodes")
	{
		// List all nodes
		nodeRoutes.GET("", h.ListNodes)

		// Get cluster stats
		nodeRoutes.GET("/stats", h.GetClusterStats)

		// Get specific node
		nodeRoutes.GET("/:name", h.GetNode)

		// Manage node labels
		nodeRoutes.PUT("/:name/labels", h.AddLabel)
		nodeRoutes.DELETE("/:name/labels/:key", h.RemoveLabel)

		// Manage node taints
		nodeRoutes.POST("/:name/taints", h.AddTaint)
		nodeRoutes.DELETE("/:name/taints/:key", h.RemoveTaint)

		// Node scheduling
		nodeRoutes.POST("/:name/cordon", h.CordonNode)
		nodeRoutes.POST("/:name/uncordon", h.UncordonNode)
		nodeRoutes.POST("/:name/drain", h.DrainNode)
	}
}

// ListNodes godoc
// @Summary List all nodes
// @Description Get a list of all Kubernetes nodes in the cluster
// @Tags nodes
// @Accept json
// @Produce json
// @Success 200 {array} nodes.NodeInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes [get]
func (h *NodeHandler) ListNodes(c *gin.Context) {
	nodeList, err := h.nodeManager.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list nodes",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, nodeList)
}

// GetNode godoc
// @Summary Get node details
// @Description Get detailed information about a specific node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Success 200 {object} nodes.NodeInfo
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name} [get]
func (h *NodeHandler) GetNode(c *gin.Context) {
	nodeName := c.Param("name")

	nodeInfo, err := h.nodeManager.GetNode(c.Request.Context(), nodeName)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Node not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, nodeInfo)
}

// GetClusterStats godoc
// @Summary Get cluster statistics
// @Description Get overall cluster resource statistics
// @Tags nodes
// @Accept json
// @Produce json
// @Success 200 {object} nodes.ClusterStats
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/stats [get]
func (h *NodeHandler) GetClusterStats(c *gin.Context) {
	stats, err := h.nodeManager.GetClusterStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get cluster stats",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AddLabel godoc
// @Summary Add label to node
// @Description Add or update a label on a node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Param label body LabelRequest true "Label to add"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/labels [put]
func (h *NodeHandler) AddLabel(c *gin.Context) {
	nodeName := c.Param("name")

	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.nodeManager.AddLabel(c.Request.Context(), nodeName, req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add label",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Label added successfully",
	})
}

// RemoveLabel godoc
// @Summary Remove label from node
// @Description Remove a label from a node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Param key path string true "Label key"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/labels/{key} [delete]
func (h *NodeHandler) RemoveLabel(c *gin.Context) {
	nodeName := c.Param("name")
	labelKey := c.Param("key")

	if err := h.nodeManager.RemoveLabel(c.Request.Context(), nodeName, labelKey); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to remove label",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Label removed successfully",
	})
}

// AddTaint godoc
// @Summary Add taint to node
// @Description Add or update a taint on a node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Param taint body nodes.Taint true "Taint to add"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/taints [post]
func (h *NodeHandler) AddTaint(c *gin.Context) {
	nodeName := c.Param("name")

	var taint nodes.Taint
	if err := c.ShouldBindJSON(&taint); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.nodeManager.AddTaint(c.Request.Context(), nodeName, taint); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add taint",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Taint added successfully",
	})
}

// RemoveTaint godoc
// @Summary Remove taint from node
// @Description Remove a taint from a node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Param key path string true "Taint key"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/taints/{key} [delete]
func (h *NodeHandler) RemoveTaint(c *gin.Context) {
	nodeName := c.Param("name")
	taintKey := c.Param("key")

	if err := h.nodeManager.RemoveTaint(c.Request.Context(), nodeName, taintKey); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to remove taint",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Taint removed successfully",
	})
}

// CordonNode godoc
// @Summary Cordon node
// @Description Mark node as unschedulable
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/cordon [post]
func (h *NodeHandler) CordonNode(c *gin.Context) {
	nodeName := c.Param("name")

	if err := h.nodeManager.CordonNode(c.Request.Context(), nodeName); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to cordon node",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Node cordoned successfully",
	})
}

// UncordonNode godoc
// @Summary Uncordon node
// @Description Mark node as schedulable
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/uncordon [post]
func (h *NodeHandler) UncordonNode(c *gin.Context) {
	nodeName := c.Param("name")

	if err := h.nodeManager.UncordonNode(c.Request.Context(), nodeName); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to uncordon node",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Node uncordoned successfully",
	})
}

// DrainNode godoc
// @Summary Drain node
// @Description Drain all pods from a node
// @Tags nodes
// @Accept json
// @Produce json
// @Param name path string true "Node name"
// @Param drain body DrainRequest false "Drain options"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/admin/nodes/{name}/drain [post]
func (h *NodeHandler) DrainNode(c *gin.Context) {
	nodeName := c.Param("name")

	var req DrainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default grace period if not provided
		req.GracePeriodSeconds = 30
	}

	if err := h.nodeManager.DrainNode(c.Request.Context(), nodeName, req.GracePeriodSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to drain node",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Node drained successfully",
	})
}

// Request/Response types

type LabelRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
}

type DrainRequest struct {
	GracePeriodSeconds int64 `json:"grace_period_seconds"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
