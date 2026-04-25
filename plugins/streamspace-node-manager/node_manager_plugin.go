package nodemanagerplugin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/plugins"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// NodeManagerPlugin implements Kubernetes node management
type NodeManagerPlugin struct {
	plugins.BasePlugin
	clientset        *kubernetes.Clientset
	metricsClientset *metricsclientset.Clientset
}

// NewNodeManagerPlugin creates a new node manager plugin instance
func NewNodeManagerPlugin() *NodeManagerPlugin {
	return &NodeManagerPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-node-manager"},
	}
}

// OnLoad is called when the plugin is loaded
func (p *NodeManagerPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Node Manager plugin loading", map[string]interface{}{
		"version": "1.0.0",
	})

	// Initialize Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	p.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Try to initialize metrics client (optional)
	p.metricsClientset, err = metricsclientset.NewForConfig(config)
	if err != nil {
		ctx.Logger.Warn("Failed to create metrics clientset, metrics will be unavailable", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Register API endpoints
	p.registerEndpoints(ctx)

	// Start health check scheduler if enabled
	healthCheckInterval, _ := ctx.Config["healthCheckInterval"].(float64)
	if healthCheckInterval > 0 {
		ctx.Scheduler.Schedule(fmt.Sprintf("@every %ds", int(healthCheckInterval)), func() {
			p.checkNodeHealth(ctx)
		})
	}

	ctx.Logger.Info("Node Manager plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *NodeManagerPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Node Manager plugin unloading")
	return nil
}

// registerEndpoints registers all API endpoints
func (p *NodeManagerPlugin) registerEndpoints(ctx *plugins.PluginContext) {
	// GET /api/plugins/streamspace-node-manager/nodes
	ctx.APIRegistry.RegisterEndpoint("GET", "/nodes", p.listNodes)

	// GET /api/plugins/streamspace-node-manager/nodes/stats
	ctx.APIRegistry.RegisterEndpoint("GET", "/nodes/stats", p.getClusterStats)

	// GET /api/plugins/streamspace-node-manager/nodes/:name
	ctx.APIRegistry.RegisterEndpoint("GET", "/nodes/:name", p.getNode)

	// PUT /api/plugins/streamspace-node-manager/nodes/:name/labels
	ctx.APIRegistry.RegisterEndpoint("PUT", "/nodes/:name/labels", p.addLabel)

	// DELETE /api/plugins/streamspace-node-manager/nodes/:name/labels/:key
	ctx.APIRegistry.RegisterEndpoint("DELETE", "/nodes/:name/labels/:key", p.removeLabel)

	// POST /api/plugins/streamspace-node-manager/nodes/:name/taints
	ctx.APIRegistry.RegisterEndpoint("POST", "/nodes/:name/taints", p.addTaint)

	// DELETE /api/plugins/streamspace-node-manager/nodes/:name/taints/:key
	ctx.APIRegistry.RegisterEndpoint("DELETE", "/nodes/:name/taints/:key", p.removeTaint)

	// POST /api/plugins/streamspace-node-manager/nodes/:name/cordon
	ctx.APIRegistry.RegisterEndpoint("POST", "/nodes/:name/cordon", p.cordonNode)

	// POST /api/plugins/streamspace-node-manager/nodes/:name/uncordon
	ctx.APIRegistry.RegisterEndpoint("POST", "/nodes/:name/uncordon", p.uncordonNode)

	// POST /api/plugins/streamspace-node-manager/nodes/:name/drain
	ctx.APIRegistry.RegisterEndpoint("POST", "/nodes/:name/drain", p.drainNode)
}

// API Handlers

func (p *NodeManagerPlugin) listNodes(c *gin.Context) {
	nodes, err := p.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes", "message": err.Error()})
		return
	}

	// Get metrics if available
	var metricsMap map[string]*v1beta1.NodeMetrics
	if p.metricsClientset != nil {
		nodeMetrics, err := p.metricsClientset.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err == nil {
			metricsMap = make(map[string]*v1beta1.NodeMetrics)
			for i := range nodeMetrics.Items {
				metricsMap[nodeMetrics.Items[i].Name] = &nodeMetrics.Items[i]
			}
		}
	}

	// Get pod count per node
	pods, err := p.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	podCountMap := make(map[string]int)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			podCountMap[pod.Spec.NodeName]++
		}
	}

	// Convert to response format
	result := make([]map[string]interface{}, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := p.convertNodeToInfo(&node)
		nodeInfo["pods"] = podCountMap[node.Name]

		if metrics, ok := metricsMap[node.Name]; ok {
			nodeInfo["usage"] = p.calculateUsage(&node, metrics)
		}

		result = append(result, nodeInfo)
	}

	c.JSON(http.StatusOK, result)
}

func (p *NodeManagerPlugin) getNode(c *gin.Context) {
	nodeName := c.Param("name")

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found", "message": err.Error()})
		return
	}

	nodeInfo := p.convertNodeToInfo(node)

	// Get metrics if available
	if p.metricsClientset != nil {
		metrics, err := p.metricsClientset.MetricsV1beta1().NodeMetricses().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err == nil {
			nodeInfo["usage"] = p.calculateUsage(node, metrics)
		}
	}

	// Get pod count
	pods, err := p.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err == nil {
		nodeInfo["pods"] = len(pods.Items)
	}

	c.JSON(http.StatusOK, nodeInfo)
}

func (p *NodeManagerPlugin) getClusterStats(c *gin.Context) {
	nodes, err := p.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}

	stats := map[string]interface{}{
		"total_nodes":     len(nodes.Items),
		"ready_nodes":     0,
		"not_ready_nodes": 0,
		"total_pods":      0,
	}

	var totalCPUCap, totalMemCap, totalCPUAlloc, totalMemAlloc resource.Quantity

	for _, node := range nodes.Items {
		// Count ready nodes
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				stats["ready_nodes"] = stats["ready_nodes"].(int) + 1
			}
		}

		// Sum resources
		cpuCap := node.Status.Capacity.Cpu()
		memCap := node.Status.Capacity.Memory()
		cpuAlloc := node.Status.Allocatable.Cpu()
		memAlloc := node.Status.Allocatable.Memory()

		totalCPUCap.Add(*cpuCap)
		totalMemCap.Add(*memCap)
		totalCPUAlloc.Add(*cpuAlloc)
		totalMemAlloc.Add(*memAlloc)
	}

	stats["not_ready_nodes"] = len(nodes.Items) - stats["ready_nodes"].(int)
	stats["total_capacity"] = map[string]string{
		"cpu":    totalCPUCap.String(),
		"memory": totalMemCap.String(),
	}
	stats["total_allocatable"] = map[string]string{
		"cpu":    totalCPUAlloc.String(),
		"memory": totalMemAlloc.String(),
	}

	c.JSON(http.StatusOK, stats)
}

func (p *NodeManagerPlugin) addLabel(c *gin.Context) {
	nodeName := c.Param("name")
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[req.Key] = req.Value

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label added successfully"})
}

func (p *NodeManagerPlugin) removeLabel(c *gin.Context) {
	nodeName := c.Param("name")
	labelKey := c.Param("key")

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	if node.Labels != nil {
		delete(node.Labels, labelKey)
	}

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label removed successfully"})
}

func (p *NodeManagerPlugin) addTaint(c *gin.Context) {
	nodeName := c.Param("name")
	var taint struct {
		Key    string `json:"key" binding:"required"`
		Value  string `json:"value"`
		Effect string `json:"effect" binding:"required"`
	}
	if err := c.ShouldBindJSON(&taint); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Check if taint already exists and update, or add new
	found := false
	for i, t := range node.Spec.Taints {
		if t.Key == taint.Key {
			node.Spec.Taints[i].Value = taint.Value
			node.Spec.Taints[i].Effect = corev1.TaintEffect(taint.Effect)
			found = true
			break
		}
	}

	if !found {
		node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: corev1.TaintEffect(taint.Effect),
		})
	}

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Taint added successfully"})
}

func (p *NodeManagerPlugin) removeTaint(c *gin.Context) {
	nodeName := c.Param("name")
	taintKey := c.Param("key")

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	newTaints := []corev1.Taint{}
	for _, t := range node.Spec.Taints {
		if t.Key != taintKey {
			newTaints = append(newTaints, t)
		}
	}
	node.Spec.Taints = newTaints

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Taint removed successfully"})
}

func (p *NodeManagerPlugin) cordonNode(c *gin.Context) {
	nodeName := c.Param("name")

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	node.Spec.Unschedulable = true

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cordon node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node cordoned successfully"})
}

func (p *NodeManagerPlugin) uncordonNode(c *gin.Context) {
	nodeName := c.Param("name")

	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	node.Spec.Unschedulable = false

	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to uncordon node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node uncordoned successfully"})
}

func (p *NodeManagerPlugin) drainNode(c *gin.Context) {
	nodeName := c.Param("name")
	var req struct {
		GracePeriodSeconds int64 `json:"grace_period_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.GracePeriodSeconds = 30 // Default
	}

	// First cordon the node
	node, err := p.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	node.Spec.Unschedulable = true
	_, err = p.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cordon node"})
		return
	}

	// Get all pods on the node
	pods, err := p.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	// Delete each pod (skip DaemonSet pods)
	deleted := 0
	for _, pod := range pods.Items {
		isDaemonSet := false
		if pod.OwnerReferences != nil {
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					isDaemonSet = true
					break
				}
			}
		}

		if !isDaemonSet {
			err := p.clientset.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &req.GracePeriodSeconds,
			})
			if err == nil {
				deleted++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Node drained successfully",
		"pods_deleted": deleted,
	})
}

// Helper functions

func (p *NodeManagerPlugin) convertNodeToInfo(node *corev1.Node) map[string]interface{} {
	taints := make([]map[string]string, len(node.Spec.Taints))
	for i, t := range node.Spec.Taints {
		taints[i] = map[string]string{
			"key":    t.Key,
			"value":  t.Value,
			"effect": string(t.Effect),
		}
	}

	ready := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return map[string]interface{}{
		"name":   node.Name,
		"labels": node.Labels,
		"taints": taints,
		"status": map[string]interface{}{
			"ready": ready,
			"phase": map[bool]string{true: "Ready", false: "NotReady"}[ready],
		},
		"capacity": map[string]string{
			"cpu":    node.Status.Capacity.Cpu().String(),
			"memory": node.Status.Capacity.Memory().String(),
			"pods":   node.Status.Capacity.Pods().String(),
		},
		"allocatable": map[string]string{
			"cpu":    node.Status.Allocatable.Cpu().String(),
			"memory": node.Status.Allocatable.Memory().String(),
			"pods":   node.Status.Allocatable.Pods().String(),
		},
		"info": map[string]string{
			"architecture":      node.Status.NodeInfo.Architecture,
			"os_image":          node.Status.NodeInfo.OSImage,
			"kernel_version":    node.Status.NodeInfo.KernelVersion,
			"kubelet_version":   node.Status.NodeInfo.KubeletVersion,
			"container_runtime": node.Status.NodeInfo.ContainerRuntimeVersion,
		},
		"age": time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
	}
}

func (p *NodeManagerPlugin) calculateUsage(node *corev1.Node, metrics *v1beta1.NodeMetrics) map[string]interface{} {
	cpuUsage := metrics.Usage.Cpu()
	memUsage := metrics.Usage.Memory()

	cpuCap := node.Status.Capacity.Cpu()
	memCap := node.Status.Capacity.Memory()

	cpuPercent := float64(cpuUsage.MilliValue()) / float64(cpuCap.MilliValue()) * 100
	memPercent := float64(memUsage.Value()) / float64(memCap.Value()) * 100

	return map[string]interface{}{
		"cpu":            cpuUsage.String(),
		"memory":         memUsage.String(),
		"cpu_percent":    cpuPercent,
		"memory_percent": memPercent,
	}
}

func (p *NodeManagerPlugin) checkNodeHealth(ctx *plugins.PluginContext) {
	alertOnFailure, _ := ctx.Config["alertOnNodeFailure"].(bool)
	if !alertOnFailure {
		return
	}

	nodes, err := p.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		ctx.Logger.Error("Failed to check node health", map[string]interface{}{"error": err.Error()})
		return
	}

	for _, node := range nodes.Items {
		ready := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if !ready {
			ctx.Logger.Warn("Node is not ready", map[string]interface{}{
				"node": node.Name,
			})
			// Could emit an event here for other plugins to handle alerts
		}
	}
}

// Auto-register plugin
func init() {
	plugins.Register("streamspace-node-manager", func() plugins.Plugin {
		return NewNodeManagerPlugin()
	})
}
