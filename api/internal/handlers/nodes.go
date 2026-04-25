// Package handlers provides HTTP handlers for the StreamSpace API.
// This file contains DEPRECATED stub handlers for node management.
//
// ⚠️ DEPRECATED: Node management has been moved to the streamspace-node-manager plugin.
//
// MIGRATION GUIDE:
//
// The node management functionality has been extracted into a plugin for better modularity.
// To restore node management functionality:
//
// 1. Install the streamspace-node-manager plugin:
//    - Via Admin UI: Admin → Plugins → Browse → streamspace-node-manager → Install
//    - Via CLI: kubectl apply -f https://plugins.streamspace.io/node-manager/install.yaml
//
// 2. API endpoints will be available at:
//    - /api/plugins/streamspace-node-manager/nodes (list)
//    - /api/plugins/streamspace-node-manager/nodes/:name (get)
//    - /api/plugins/streamspace-node-manager/nodes/:name/labels (add/remove)
//    - /api/plugins/streamspace-node-manager/nodes/:name/taints (add/remove)
//    - /api/plugins/streamspace-node-manager/nodes/:name/cordon (cordon)
//    - /api/plugins/streamspace-node-manager/nodes/:name/uncordon (uncordon)
//    - /api/plugins/streamspace-node-manager/nodes/:name/drain (drain)
//    - /api/plugins/streamspace-node-manager/nodes/stats (cluster stats)
//
// 3. The plugin provides enhanced features:
//    - Auto-scaling integration
//    - Advanced health monitoring
//    - Node selection strategies
//    - Metrics collection (requires metrics-server)
//    - Alert integration
//    - Configurable health checks
//
// WHY WAS THIS MOVED TO A PLUGIN?
//
// - Reduced core complexity: Node management is advanced functionality not needed by all users
// - Optional dependency: Single-node deployments don't need cluster management
// - Enhanced features: Plugin can provide more advanced capabilities
// - Modular architecture: Easier to maintain and extend independently
// - Performance: Core stays lean for basic deployments
//
// BACKWARDS COMPATIBILITY:
//
// These stub handlers remain in core to provide clear migration messages to existing users.
// They will be removed in v2.0.0.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/events"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
)

// NodeHandler provides deprecated stub handlers for node management
type NodeHandler struct {
	db        *db.Database
	k8sClient *k8s.Client
	publisher *events.Publisher
	platform  string
}

// NewNodeHandler creates a new node handler (deprecated)
func NewNodeHandler(database *db.Database, k8sClient *k8s.Client, publisher *events.Publisher, platform string) *NodeHandler {
	return &NodeHandler{
		db:        database,
		k8sClient: k8sClient,
		publisher: publisher,
		platform:  platform,
	}
}

// deprecationResponse returns a standardized deprecation message
func (h *NodeHandler) deprecationResponse(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":   "Node management has been moved to a plugin",
		"message": "This functionality has been extracted into the streamspace-node-manager plugin for better modularity",
		"migration": gin.H{
			"install": "Admin → Plugins → streamspace-node-manager",
			"api_base": "/api/plugins/streamspace-node-manager",
			"documentation": "https://docs.streamspace.io/plugins/node-manager",
		},
		"benefits": []string{
			"Enhanced auto-scaling capabilities",
			"Advanced health monitoring",
			"Configurable node selection strategies",
			"Optional for single-node deployments",
		},
		"status": "deprecated",
		"removed_in": "v2.0.0",
	})
}

// ListNodes returns a deprecation message
func (h *NodeHandler) ListNodes(c *gin.Context) {
	h.deprecationResponse(c)
}

// GetClusterStats returns a deprecation message
func (h *NodeHandler) GetClusterStats(c *gin.Context) {
	h.deprecationResponse(c)
}

// GetNode returns a deprecation message
func (h *NodeHandler) GetNode(c *gin.Context) {
	h.deprecationResponse(c)
}

// AddNodeLabel returns a deprecation message
func (h *NodeHandler) AddNodeLabel(c *gin.Context) {
	h.deprecationResponse(c)
}

// RemoveNodeLabel returns a deprecation message
func (h *NodeHandler) RemoveNodeLabel(c *gin.Context) {
	h.deprecationResponse(c)
}

// AddNodeTaint returns a deprecation message
func (h *NodeHandler) AddNodeTaint(c *gin.Context) {
	h.deprecationResponse(c)
}

// RemoveNodeTaint returns a deprecation message
func (h *NodeHandler) RemoveNodeTaint(c *gin.Context) {
	h.deprecationResponse(c)
}

// CordonNode returns a deprecation message
func (h *NodeHandler) CordonNode(c *gin.Context) {
	h.deprecationResponse(c)
}

// UncordonNode returns a deprecation message
func (h *NodeHandler) UncordonNode(c *gin.Context) {
	h.deprecationResponse(c)
}

// DrainNode returns a deprecation message
func (h *NodeHandler) DrainNode(c *gin.Context) {
	h.deprecationResponse(c)
}
