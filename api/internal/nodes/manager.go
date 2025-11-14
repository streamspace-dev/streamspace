package nodes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// NodeManager handles Kubernetes node operations
type NodeManager struct {
	clientset        *kubernetes.Clientset
	metricsClientset *metricsv.Clientset
}

// NewNodeManager creates a new node manager
func NewNodeManager(clientset *kubernetes.Clientset, metricsClientset *metricsv.Clientset) *NodeManager {
	return &NodeManager{
		clientset:        clientset,
		metricsClientset: metricsClientset,
	}
}

// NodeInfo represents detailed node information
type NodeInfo struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Taints    []Taint           `json:"taints"`
	Status    NodeStatus        `json:"status"`
	Capacity  ResourceList      `json:"capacity"`
	Allocatable ResourceList    `json:"allocatable"`
	Usage     *ResourceUsage    `json:"usage,omitempty"`
	Info      NodeSystemInfo    `json:"info"`
	Conditions []NodeCondition  `json:"conditions"`
	Pods      int               `json:"pods"`
	Age       string            `json:"age"`
	Provider  string            `json:"provider,omitempty"`
	Region    string            `json:"region,omitempty"`
	Zone      string            `json:"zone,omitempty"`
}

// Taint represents a node taint
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// NodeStatus represents the overall node status
type NodeStatus struct {
	Phase      string `json:"phase"`
	Ready      bool   `json:"ready"`
	Message    string `json:"message,omitempty"`
	LastUpdate string `json:"last_update"`
}

// ResourceList represents node resources
type ResourceList struct {
	CPU              string  `json:"cpu"`
	Memory           string  `json:"memory"`
	Pods             string  `json:"pods"`
	EphemeralStorage string  `json:"ephemeral_storage,omitempty"`
	GPUs             int     `json:"gpus,omitempty"`
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	CPU       string  `json:"cpu"`
	Memory    string  `json:"memory"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
}

// NodeSystemInfo represents node system information
type NodeSystemInfo struct {
	Architecture    string `json:"architecture"`
	OSImage         string `json:"os_image"`
	KernelVersion   string `json:"kernel_version"`
	KubeletVersion  string `json:"kubelet_version"`
	ContainerRuntime string `json:"container_runtime"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ListNodes returns all nodes in the cluster
func (nm *NodeManager) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := nm.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Get metrics for all nodes
	var metricsMap map[string]*v1beta1.NodeMetrics
	if nm.metricsClientset != nil {
		nodeMetrics, err := nm.metricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err == nil {
			metricsMap = make(map[string]*v1beta1.NodeMetrics)
			for i := range nodeMetrics.Items {
				metricsMap[nodeMetrics.Items[i].Name] = &nodeMetrics.Items[i]
			}
		}
	}

	// Get pod count per node
	pods, err := nm.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	podCountMap := make(map[string]int)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			podCountMap[pod.Spec.NodeName]++
		}
	}

	nodeInfos := make([]NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := nm.convertNodeToInfo(&node)

		// Add pod count
		nodeInfo.Pods = podCountMap[node.Name]

		// Add metrics if available
		if metrics, ok := metricsMap[node.Name]; ok {
			nodeInfo.Usage = nm.calculateUsage(&node, metrics)
		}

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// GetNode returns detailed information about a specific node
func (nm *NodeManager) GetNode(ctx context.Context, nodeName string) (*NodeInfo, error) {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	nodeInfo := nm.convertNodeToInfo(node)

	// Get metrics
	if nm.metricsClientset != nil {
		metrics, err := nm.metricsClientset.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil {
			nodeInfo.Usage = nm.calculateUsage(node, metrics)
		}
	}

	// Get pod count
	pods, err := nm.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err == nil {
		nodeInfo.Pods = len(pods.Items)
	}

	return &nodeInfo, nil
}

// AddLabel adds a label to a node
func (nm *NodeManager) AddLabel(ctx context.Context, nodeName, key, value string) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[key] = value

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node labels: %w", err)
	}

	return nil
}

// RemoveLabel removes a label from a node
func (nm *NodeManager) RemoveLabel(ctx context.Context, nodeName, key string) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.Labels != nil {
		delete(node.Labels, key)
	}

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node labels: %w", err)
	}

	return nil
}

// AddTaint adds a taint to a node
func (nm *NodeManager) AddTaint(ctx context.Context, nodeName string, taint Taint) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Check if taint already exists
	for i, existingTaint := range node.Spec.Taints {
		if existingTaint.Key == taint.Key {
			// Update existing taint
			node.Spec.Taints[i].Value = taint.Value
			node.Spec.Taints[i].Effect = corev1.TaintEffect(taint.Effect)
			_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			return err
		}
	}

	// Add new taint
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    taint.Key,
		Value:  taint.Value,
		Effect: corev1.TaintEffect(taint.Effect),
	})

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node taints: %w", err)
	}

	return nil
}

// RemoveTaint removes a taint from a node
func (nm *NodeManager) RemoveTaint(ctx context.Context, nodeName, key string) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Remove taint
	newTaints := []corev1.Taint{}
	for _, taint := range node.Spec.Taints {
		if taint.Key != key {
			newTaints = append(newTaints, taint)
		}
	}
	node.Spec.Taints = newTaints

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node taints: %w", err)
	}

	return nil
}

// CordonNode marks a node as unschedulable
func (nm *NodeManager) CordonNode(ctx context.Context, nodeName string) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node.Spec.Unschedulable = true

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	return nil
}

// UncordonNode marks a node as schedulable
func (nm *NodeManager) UncordonNode(ctx context.Context, nodeName string) error {
	node, err := nm.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node.Spec.Unschedulable = false

	_, err = nm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to uncordon node: %w", err)
	}

	return nil
}

// DrainNode drains all pods from a node
func (nm *NodeManager) DrainNode(ctx context.Context, nodeName string, gracePeriod int64) error {
	// First, cordon the node
	if err := nm.CordonNode(ctx, nodeName); err != nil {
		return err
	}

	// Get all pods on the node
	pods, err := nm.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Delete each pod (respecting gracePeriod)
	for _, pod := range pods.Items {
		// Skip DaemonSet pods (they will be recreated anyway)
		if pod.OwnerReferences != nil {
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					continue
				}
			}
		}

		err := nm.clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

// Helper functions

func (nm *NodeManager) convertNodeToInfo(node *corev1.Node) NodeInfo {
	// Extract taints
	taints := make([]Taint, len(node.Spec.Taints))
	for i, taint := range node.Spec.Taints {
		taints[i] = Taint{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: string(taint.Effect),
		}
	}

	// Determine node status
	ready := false
	var readyCondition *corev1.NodeCondition
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == corev1.NodeReady {
			readyCondition = &node.Status.Conditions[i]
			ready = readyCondition.Status == corev1.ConditionTrue
			break
		}
	}

	status := NodeStatus{
		Phase: "Unknown",
		Ready: ready,
	}
	if readyCondition != nil {
		status.LastUpdate = readyCondition.LastHeartbeatTime.Format(time.RFC3339)
		status.Message = readyCondition.Message
		if ready {
			status.Phase = "Ready"
		} else {
			status.Phase = "NotReady"
		}
	}

	// Extract conditions
	conditions := make([]NodeCondition, len(node.Status.Conditions))
	for i, cond := range node.Status.Conditions {
		conditions[i] = NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	// Extract provider info from labels
	provider := node.Labels["cloud.google.com/gke-nodepool"]
	if provider == "" {
		provider = node.Labels["eks.amazonaws.com/nodegroup"]
	}
	if provider == "" {
		provider = node.Labels["node.kubernetes.io/instance-type"]
	}

	region := node.Labels["topology.kubernetes.io/region"]
	zone := node.Labels["topology.kubernetes.io/zone"]

	// Count GPUs
	gpus := 0
	if gpu, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
		gpus = int(gpu.Value())
	}

	return NodeInfo{
		Name:   node.Name,
		Labels: node.Labels,
		Taints: taints,
		Status: status,
		Capacity: ResourceList{
			CPU:              node.Status.Capacity.Cpu().String(),
			Memory:           node.Status.Capacity.Memory().String(),
			Pods:             node.Status.Capacity.Pods().String(),
			EphemeralStorage: node.Status.Capacity.StorageEphemeral().String(),
			GPUs:             gpus,
		},
		Allocatable: ResourceList{
			CPU:              node.Status.Allocatable.Cpu().String(),
			Memory:           node.Status.Allocatable.Memory().String(),
			Pods:             node.Status.Allocatable.Pods().String(),
			EphemeralStorage: node.Status.Allocatable.StorageEphemeral().String(),
		},
		Info: NodeSystemInfo{
			Architecture:     node.Status.NodeInfo.Architecture,
			OSImage:          node.Status.NodeInfo.OSImage,
			KernelVersion:    node.Status.NodeInfo.KernelVersion,
			KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
			ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
		},
		Conditions: conditions,
		Age:        time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
		Provider:   provider,
		Region:     region,
		Zone:       zone,
	}
}

func (nm *NodeManager) calculateUsage(node *corev1.Node, metrics *v1beta1.NodeMetrics) *ResourceUsage {
	cpuUsage := metrics.Usage.Cpu()
	memUsage := metrics.Usage.Memory()

	cpuCapacity := node.Status.Capacity.Cpu()
	memCapacity := node.Status.Capacity.Memory()

	cpuPercent := float64(cpuUsage.MilliValue()) / float64(cpuCapacity.MilliValue()) * 100
	memPercent := float64(memUsage.Value()) / float64(memCapacity.Value()) * 100

	return &ResourceUsage{
		CPU:           cpuUsage.String(),
		Memory:        memUsage.String(),
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
	}
}

// GetClusterStats returns overall cluster statistics
func (nm *NodeManager) GetClusterStats(ctx context.Context) (*ClusterStats, error) {
	nodes, err := nm.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	stats := &ClusterStats{
		TotalNodes:  len(nodes),
		ReadyNodes:  0,
		NotReadyNodes: 0,
		TotalCapacity: ResourceList{},
		TotalAllocatable: ResourceList{},
		TotalUsage: ResourceUsage{},
		TotalPods: 0,
	}

	var totalCPUCapacity, totalMemCapacity resource.Quantity
	var totalCPUAlloc, totalMemAlloc resource.Quantity
	var totalCPUUsage, totalMemUsage float64

	for _, node := range nodes {
		if node.Status.Ready {
			stats.ReadyNodes++
		} else {
			stats.NotReadyNodes++
		}

		stats.TotalPods += node.Pods

		// Parse and sum capacity
		cpuCap, _ := resource.ParseQuantity(node.Capacity.CPU)
		memCap, _ := resource.ParseQuantity(node.Capacity.Memory)
		totalCPUCapacity.Add(cpuCap)
		totalMemCapacity.Add(memCap)

		// Parse and sum allocatable
		cpuAlloc, _ := resource.ParseQuantity(node.Allocatable.CPU)
		memAlloc, _ := resource.ParseQuantity(node.Allocatable.Memory)
		totalCPUAlloc.Add(cpuAlloc)
		totalMemAlloc.Add(memAlloc)

		// Sum usage
		if node.Usage != nil {
			totalCPUUsage += node.Usage.CPUPercent
			totalMemUsage += node.Usage.MemoryPercent
		}
	}

	stats.TotalCapacity.CPU = totalCPUCapacity.String()
	stats.TotalCapacity.Memory = totalMemCapacity.String()
	stats.TotalAllocatable.CPU = totalCPUAlloc.String()
	stats.TotalAllocatable.Memory = totalMemAlloc.String()

	if len(nodes) > 0 {
		stats.TotalUsage.CPUPercent = totalCPUUsage / float64(len(nodes))
		stats.TotalUsage.MemoryPercent = totalMemUsage / float64(len(nodes))
	}

	return stats, nil
}

// ClusterStats represents overall cluster statistics
type ClusterStats struct {
	TotalNodes       int          `json:"total_nodes"`
	ReadyNodes       int          `json:"ready_nodes"`
	NotReadyNodes    int          `json:"not_ready_nodes"`
	TotalCapacity    ResourceList `json:"total_capacity"`
	TotalAllocatable ResourceList `json:"total_allocatable"`
	TotalUsage       ResourceUsage `json:"total_usage"`
	TotalPods        int          `json:"total_pods"`
}
