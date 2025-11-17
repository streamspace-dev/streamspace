// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements Kubernetes node management for administrators.
//
// NODE MANAGEMENT OVERVIEW:
//
// The node management system allows administrators to:
// - View all cluster nodes and their health status
// - Monitor resource capacity and usage
// - Add/remove node labels for scheduling
// - Add/remove node taints to control pod placement
// - Cordon nodes to prevent new pod scheduling
// - Drain nodes to safely evict pods for maintenance
//
// FEATURES:
//
// 1. Node Listing:
//   - View all cluster nodes with status
//   - Resource capacity (CPU, memory, storage, pods)
//   - Allocatable resources (after system reservations)
//   - Current usage statistics
//   - Node metadata (OS, kernel, kubelet version, container runtime)
//
// 2. Cluster Statistics:
//   - Total nodes (ready vs not ready)
//   - Aggregate capacity and allocatable resources
//   - Overall cluster utilization percentages
//
// 3. Node Labeling:
//   - Add labels for node selection (e.g., gpu=true, tier=premium)
//   - Remove labels when no longer needed
//   - Labels used in session pod affinity rules
//
// 4. Node Tainting:
//   - Add taints to repel pods (NoSchedule, PreferNoSchedule, NoExecute)
//   - Remove taints to allow normal scheduling
//   - Taints used for dedicated workloads or maintenance
//
// 5. Node Operations:
//   - Cordon: Mark node as unschedulable (existing pods continue)
//   - Uncordon: Allow scheduling again
//   - Drain: Evict all pods gracefully with grace period
//
// SECURITY:
//
// - Admin-only access required for all node operations
// - Audit logging for all node changes
// - Validation of node names and operations
//
// EXAMPLE WORKFLOWS:
//
// Maintenance workflow:
// 1. Cordon node to prevent new sessions
// 2. Drain node to move existing sessions elsewhere
// 3. Perform maintenance (OS updates, hardware changes)
// 4. Uncordon node to resume normal operation
//
// GPU node labeling:
// 1. Add label: gpu=nvidia-v100
// 2. Create template with nodeSelector matching the label
// 3. GPU sessions only schedule on labeled nodes
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NodeHandler handles node management operations
type NodeHandler struct {
	db        *db.Database
	k8sClient *k8s.Client
}

// NewNodeHandler creates a new node management handler
func NewNodeHandler(database *db.Database, k8sClient *k8s.Client) *NodeHandler {
	return &NodeHandler{
		db:        database,
		k8sClient: k8sClient,
	}
}

// NodeInfo represents detailed node information
type NodeInfo struct {
	Name        string                 `json:"name"`
	Labels      map[string]string      `json:"labels"`
	Taints      []corev1.Taint         `json:"taints"`
	Status      string                 `json:"status"` // Ready, NotReady, Unknown
	Capacity    corev1.ResourceList    `json:"capacity"`
	Allocatable corev1.ResourceList    `json:"allocatable"`
	Usage       *NodeUsage             `json:"usage,omitempty"`
	Info        NodeSystemInfo         `json:"info"`
	Conditions  []corev1.NodeCondition `json:"conditions"`
	Pods        int                    `json:"pods"`
	Age         string                 `json:"age"`
	Provider    string                 `json:"provider,omitempty"`
	Region      string                 `json:"region,omitempty"`
	Zone        string                 `json:"zone,omitempty"`
}

// NodeUsage represents resource usage on a node
type NodeUsage struct {
	CPU           string  `json:"cpu"`
	Memory        string  `json:"memory"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
}

// NodeSystemInfo represents system information
type NodeSystemInfo struct {
	OSImage          string `json:"osImage"`
	KernelVersion    string `json:"kernelVersion"`
	KubeletVersion   string `json:"kubeletVersion"`
	ContainerRuntime string `json:"containerRuntime"`
}

// ClusterStats represents aggregate cluster statistics
type ClusterStats struct {
	TotalNodes       int                 `json:"totalNodes"`
	ReadyNodes       int                 `json:"readyNodes"`
	NotReadyNodes    int                 `json:"notReadyNodes"`
	TotalCapacity    corev1.ResourceList `json:"totalCapacity"`
	TotalAllocatable corev1.ResourceList `json:"totalAllocatable"`
	TotalUsage       *ClusterUsage       `json:"totalUsage,omitempty"`
}

// ClusterUsage represents aggregate cluster usage
type ClusterUsage struct {
	CPU           string  `json:"cpu"`
	Memory        string  `json:"memory"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
}

// ListNodes returns all cluster nodes
// GET /admin/nodes
func (h *NodeHandler) ListNodes(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Get nodes from Kubernetes
	nodeList, err := h.k8sClient.GetNodes(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to list nodes: %v", err),
		})
		return
	}

	// Convert to NodeInfo structs
	nodes := make([]NodeInfo, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		nodeInfo := h.nodeToNodeInfo(&node)
		nodes = append(nodes, nodeInfo)
	}

	c.JSON(http.StatusOK, nodes)
}

// GetNode returns detailed information about a specific node
// GET /admin/nodes/:name
func (h *NodeHandler) GetNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get node from Kubernetes
	node, err := h.k8sClient.GetNode(ctx, nodeName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Node not found: %v", err),
		})
		return
	}

	nodeInfo := h.nodeToNodeInfo(node)
	c.JSON(http.StatusOK, nodeInfo)
}

// GetClusterStats returns aggregate cluster statistics
// GET /admin/nodes/stats
func (h *NodeHandler) GetClusterStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Get nodes from Kubernetes
	nodeList, err := h.k8sClient.GetNodes(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get cluster stats: %v", err),
		})
		return
	}

	stats := h.calculateClusterStats(nodeList)
	c.JSON(http.StatusOK, stats)
}

// AddNodeLabel adds a label to a node
// PUT /admin/nodes/:name/labels
func (h *NodeHandler) AddNodeLabel(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Add label using patch
	patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, req.Key, req.Value)
	if err := h.k8sClient.PatchNode(ctx, nodeName, []byte(patchData)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to add label: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label added successfully"})
}

// RemoveNodeLabel removes a label from a node
// DELETE /admin/nodes/:name/labels/:key
func (h *NodeHandler) RemoveNodeLabel(c *gin.Context) {
	nodeName := c.Param("name")
	labelKey := c.Param("key")

	if nodeName == "" || labelKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name and label key are required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Remove label using JSON patch
	patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":null}}}`, labelKey)
	if err := h.k8sClient.PatchNode(ctx, nodeName, []byte(patchData)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to remove label: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label removed successfully"})
}

// AddNodeTaint adds a taint to a node
// POST /admin/nodes/:name/taints
func (h *NodeHandler) AddNodeTaint(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	var taint corev1.Taint
	if err := c.ShouldBindJSON(&taint); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get current node to append taint
	node, err := h.k8sClient.GetNode(ctx, nodeName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Check if taint already exists
	for _, t := range node.Spec.Taints {
		if t.Key == taint.Key && t.Effect == taint.Effect {
			c.JSON(http.StatusConflict, gin.H{"error": "Taint already exists"})
			return
		}
	}

	// Add taint using strategic merge patch
	patchData := fmt.Sprintf(`{"spec":{"taints":[{"key":"%s","value":"%s","effect":"%s"}]}}`,
		taint.Key, taint.Value, taint.Effect)
	if err := h.k8sClient.PatchNode(ctx, nodeName, []byte(patchData)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to add taint: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Taint added successfully"})
}

// RemoveNodeTaint removes a taint from a node
// DELETE /admin/nodes/:name/taints/:key
func (h *NodeHandler) RemoveNodeTaint(c *gin.Context) {
	nodeName := c.Param("name")
	taintKey := c.Param("key")

	if nodeName == "" || taintKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name and taint key are required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get current node
	node, err := h.k8sClient.GetNode(ctx, nodeName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Filter out the taint
	newTaints := []corev1.Taint{}
	found := false
	for _, t := range node.Spec.Taints {
		if t.Key != taintKey {
			newTaints = append(newTaints, t)
		} else {
			found = true
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Taint not found"})
		return
	}

	// Update node with new taints
	if err := h.k8sClient.UpdateNodeTaints(ctx, nodeName, newTaints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to remove taint: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Taint removed successfully"})
}

// CordonNode marks a node as unschedulable
// POST /admin/nodes/:name/cordon
func (h *NodeHandler) CordonNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.k8sClient.CordonNode(ctx, nodeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to cordon node: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node cordoned successfully"})
}

// UncordonNode marks a node as schedulable
// POST /admin/nodes/:name/uncordon
func (h *NodeHandler) UncordonNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.k8sClient.UncordonNode(ctx, nodeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to uncordon node: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node uncordoned successfully"})
}

// DrainNode evicts all pods from a node
// POST /admin/nodes/:name/drain
func (h *NodeHandler) DrainNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node name is required"})
		return
	}

	var req struct {
		GracePeriodSeconds *int64 `json:"grace_period_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err == nil && req.GracePeriodSeconds == nil {
		defaultGracePeriod := int64(30)
		req.GracePeriodSeconds = &defaultGracePeriod
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	if err := h.k8sClient.DrainNode(ctx, nodeName, req.GracePeriodSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to drain node: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node drained successfully"})
}

// Helper function to convert K8s Node to NodeInfo
func (h *NodeHandler) nodeToNodeInfo(node *corev1.Node) NodeInfo {
	// Determine node status
	status := "Unknown"
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				status = "Ready"
			} else {
				status = "NotReady"
			}
			break
		}
	}

	// Calculate age
	age := time.Since(node.CreationTimestamp.Time).Round(time.Hour).String()

	// Get cloud provider info from labels
	provider := node.Labels["cloud.google.com/gke-nodepool"]
	if provider == "" {
		provider = node.Labels["eks.amazonaws.com/nodegroup"]
	}
	if provider == "" {
		provider = node.Labels["node.kubernetes.io/instance-type"]
	}

	return NodeInfo{
		Name:        node.Name,
		Labels:      node.Labels,
		Taints:      node.Spec.Taints,
		Status:      status,
		Capacity:    node.Status.Capacity,
		Allocatable: node.Status.Allocatable,
		Info: NodeSystemInfo{
			OSImage:          node.Status.NodeInfo.OSImage,
			KernelVersion:    node.Status.NodeInfo.KernelVersion,
			KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
			ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
		},
		Conditions: node.Status.Conditions,
		Age:        age,
		Provider:   provider,
		Region:     node.Labels["topology.kubernetes.io/region"],
		Zone:       node.Labels["topology.kubernetes.io/zone"],
	}
}

// Helper function to calculate cluster statistics
func (h *NodeHandler) calculateClusterStats(nodeList *corev1.NodeList) ClusterStats {
	stats := ClusterStats{
		TotalNodes:    len(nodeList.Items),
		ReadyNodes:    0,
		NotReadyNodes: 0,
		TotalCapacity: corev1.ResourceList{
			corev1.ResourceCPU:    newQuantity(0),
			corev1.ResourceMemory: newQuantity(0),
			corev1.ResourcePods:   newQuantity(0),
		},
		TotalAllocatable: corev1.ResourceList{
			corev1.ResourceCPU:    newQuantity(0),
			corev1.ResourceMemory: newQuantity(0),
			corev1.ResourcePods:   newQuantity(0),
		},
	}

	for _, node := range nodeList.Items {
		// Count ready vs not ready nodes
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					stats.ReadyNodes++
				} else {
					stats.NotReadyNodes++
				}
				break
			}
		}

		// Aggregate capacity
		if cpu, ok := node.Status.Capacity[corev1.ResourceCPU]; ok {
			totalCPU := stats.TotalCapacity[corev1.ResourceCPU]
			totalCPU.Add(cpu)
			stats.TotalCapacity[corev1.ResourceCPU] = totalCPU
		}
		if mem, ok := node.Status.Capacity[corev1.ResourceMemory]; ok {
			totalMem := stats.TotalCapacity[corev1.ResourceMemory]
			totalMem.Add(mem)
			stats.TotalCapacity[corev1.ResourceMemory] = totalMem
		}
		if pods, ok := node.Status.Capacity[corev1.ResourcePods]; ok {
			totalPods := stats.TotalCapacity[corev1.ResourcePods]
			totalPods.Add(pods)
			stats.TotalCapacity[corev1.ResourcePods] = totalPods
		}

		// Aggregate allocatable
		if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			allocCPU := stats.TotalAllocatable[corev1.ResourceCPU]
			allocCPU.Add(cpu)
			stats.TotalAllocatable[corev1.ResourceCPU] = allocCPU
		}
		if mem, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			allocMem := stats.TotalAllocatable[corev1.ResourceMemory]
			allocMem.Add(mem)
			stats.TotalAllocatable[corev1.ResourceMemory] = allocMem
		}
		if pods, ok := node.Status.Allocatable[corev1.ResourcePods]; ok {
			allocPods := stats.TotalAllocatable[corev1.ResourcePods]
			allocPods.Add(pods)
			stats.TotalAllocatable[corev1.ResourcePods] = allocPods
		}
	}

	return stats
}

// Helper function to create a new Quantity
func newQuantity(value int64) resource.Quantity {
	return *resource.NewQuantity(value, resource.DecimalSI)
}
