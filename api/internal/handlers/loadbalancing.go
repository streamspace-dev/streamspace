package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// LOAD BALANCING
// ============================================================================

// LoadBalancingPolicy defines how sessions are distributed across nodes
type LoadBalancingPolicy struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description,omitempty"`
	Strategy          string                 `json:"strategy"` // "round_robin", "least_loaded", "resource_based", "geographic", "weighted"
	Enabled           bool                   `json:"enabled"`
	SessionAffinity   bool                   `json:"session_affinity"`  // Sticky sessions
	HealthCheckConfig HealthCheckConfig      `json:"health_check_config"`
	NodeSelector      map[string]string      `json:"node_selector,omitempty"` // Kubernetes node selector
	NodeWeights       map[string]int         `json:"node_weights,omitempty"`  // For weighted distribution
	GeoPreferences    []string               `json:"geo_preferences,omitempty"` // Preferred regions
	ResourceThresholds ResourceThresholds    `json:"resource_thresholds"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy         string                 `json:"created_by"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// HealthCheckConfig defines node health checking
type HealthCheckConfig struct {
	Enabled       bool   `json:"enabled"`
	Interval      int    `json:"interval_seconds"`      // How often to check
	Timeout       int    `json:"timeout_seconds"`       // Timeout for each check
	FailThreshold int    `json:"fail_threshold"`        // Failures before marking unhealthy
	PassThreshold int    `json:"pass_threshold"`        // Successes before marking healthy
	Endpoint      string `json:"endpoint,omitempty"`    // Health check endpoint
}

// ResourceThresholds for load balancing decisions
type ResourceThresholds struct {
	CPUPercent    float64 `json:"cpu_percent"`     // Max CPU % before avoiding node
	MemoryPercent float64 `json:"memory_percent"`  // Max memory % before avoiding node
	MaxSessions   int     `json:"max_sessions"`    // Max concurrent sessions per node
	MinFreeCPU    float64 `json:"min_free_cpu"`    // Min free CPU cores required
	MinFreeMemory int64   `json:"min_free_memory"` // Min free memory in bytes
}

// NodeStatus represents current status of a cluster node
type NodeStatus struct {
	NodeName         string                 `json:"node_name"`
	Status           string                 `json:"status"` // "ready", "not_ready", "unknown"
	CPUAllocated     float64                `json:"cpu_allocated"`
	CPUCapacity      float64                `json:"cpu_capacity"`
	CPUPercent       float64                `json:"cpu_percent"`
	MemoryAllocated  int64                  `json:"memory_allocated"`
	MemoryCapacity   int64                  `json:"memory_capacity"`
	MemoryPercent    float64                `json:"memory_percent"`
	ActiveSessions   int                    `json:"active_sessions"`
	HealthStatus     string                 `json:"health_status"` // "healthy", "unhealthy", "unknown"
	LastHealthCheck  time.Time              `json:"last_health_check"`
	Region           string                 `json:"region,omitempty"`
	Zone             string                 `json:"zone,omitempty"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Taints           []string               `json:"taints,omitempty"`
	Weight           int                    `json:"weight"` // For weighted load balancing
}

// CreateLoadBalancingPolicy creates a new load balancing policy
func (h *Handler) CreateLoadBalancingPolicy(c *gin.Context) {
	createdBy := c.GetString("user_id")
	role := c.GetString("role")

	if role != "admin" && role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins/operators can create load balancing policies"})
		return
	}

	var req LoadBalancingPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate strategy
	validStrategies := []string{"round_robin", "least_loaded", "resource_based", "geographic", "weighted"}
	if !contains(validStrategies, req.Strategy) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid load balancing strategy"})
		return
	}

	req.CreatedBy = createdBy
	req.Enabled = true

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO load_balancing_policies
		(name, description, strategy, enabled, session_affinity, health_check_config,
		 node_selector, node_weights, geo_preferences, resource_thresholds, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`, req.Name, req.Description, req.Strategy, req.Enabled, req.SessionAffinity,
		req.HealthCheckConfig, req.NodeSelector, req.NodeWeights, req.GeoPreferences,
		req.ResourceThresholds, req.Metadata, createdBy).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create policy"})
		return
	}

	req.ID = id

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Load balancing policy created",
		"policy":  req,
	})
}

// ListLoadBalancingPolicies lists all load balancing policies
func (h *Handler) ListLoadBalancingPolicies(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, name, description, strategy, enabled, session_affinity,
		       health_check_config, node_selector, node_weights, geo_preferences,
		       resource_thresholds, metadata, created_by, created_at, updated_at
		FROM load_balancing_policies
		ORDER BY created_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	policies := []LoadBalancingPolicy{}
	for rows.Next() {
		var p LoadBalancingPolicy
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Strategy, &p.Enabled,
			&p.SessionAffinity, &p.HealthCheckConfig, &p.NodeSelector, &p.NodeWeights,
			&p.GeoPreferences, &p.ResourceThresholds, &p.Metadata, &p.CreatedBy,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		policies = append(policies, p)
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// GetNodeStatus gets current status of all cluster nodes
func (h *Handler) GetNodeStatus(c *gin.Context) {
	// Try to fetch real node metrics from Kubernetes API
	// If K8s integration is not available, fall back to database
	nodes, err := h.fetchKubernetesNodeMetrics()
	if err != nil {
		// Fall back to database if K8s API is not available
		nodes, err = h.fetchNodeStatusFromDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get node status"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// fetchNodeStatusFromDatabase fetches node status from database cache
func (h *Handler) fetchNodeStatusFromDatabase() ([]NodeStatus, error) {
	rows, err := h.DB.Query(`
		SELECT node_name, status, cpu_allocated, cpu_capacity, memory_allocated,
		       memory_capacity, active_sessions, health_status, last_health_check,
		       region, zone, labels, weight
		FROM node_status
		ORDER BY node_name
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := []NodeStatus{}
	for rows.Next() {
		var n NodeStatus
		var lastCheck sql.NullTime

		err := rows.Scan(&n.NodeName, &n.Status, &n.CPUAllocated, &n.CPUCapacity,
			&n.MemoryAllocated, &n.MemoryCapacity, &n.ActiveSessions, &n.HealthStatus,
			&lastCheck, &n.Region, &n.Zone, &n.Labels, &n.Weight)

		if err != nil {
			continue
		}

		// Calculate percentages
		if n.CPUCapacity > 0 {
			n.CPUPercent = (n.CPUAllocated / n.CPUCapacity) * 100
		}
		if n.MemoryCapacity > 0 {
			n.MemoryPercent = (float64(n.MemoryAllocated) / float64(n.MemoryCapacity)) * 100
		}

		if lastCheck.Valid {
			n.LastHealthCheck = lastCheck.Time
		}

		nodes = append(nodes, n)
	}

	return nodes, nil
}

// fetchKubernetesNodeMetrics fetches real-time node metrics from Kubernetes API
func (h *Handler) fetchKubernetesNodeMetrics() ([]NodeStatus, error) {
	// Check if K8s configuration is available
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Try in-cluster config
		if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); os.IsNotExist(err) {
			return nil, fmt.Errorf("kubernetes API not configured")
		}
	}

	// Placeholder for actual Kubernetes API integration
	// In production, this would use k8s.io/client-go:
	// 1. Create clientset from config
	// 2. Query v1.NodeList
	// 3. Fetch metrics from metrics-server API
	// 4. Convert to NodeStatus structs

	// For now, return error to fall back to database
	return nil, fmt.Errorf("kubernetes API integration not yet implemented - use database cache")
}

// scaleKubernetesDeployment scales a Kubernetes deployment to the specified replica count
func (h *Handler) scaleKubernetesDeployment(deploymentName string, replicas int) error {
	// Check if K8s configuration is available
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Try in-cluster config
		if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); os.IsNotExist(err) {
			return fmt.Errorf("kubernetes API not configured")
		}
	}

	// Get namespace from environment or default to streamspace
	namespace := os.Getenv("STREAMSPACE_NAMESPACE")
	if namespace == "" {
		namespace = "streamspace"
	}

	// Placeholder for actual Kubernetes API integration
	// In production, this would use k8s.io/client-go:
	// 1. Create clientset from config
	// 2. Get Deployment: clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	// 3. Update replicas: deployment.Spec.Replicas = &replicas
	// 4. Update: clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})

	// For development, use database-driven scaling trigger
	_, err := h.DB.Exec(`
		INSERT INTO deployment_scaling_queue (deployment_name, namespace, target_replicas, status, created_at)
		VALUES ($1, $2, $3, 'pending', NOW())
	`, deploymentName, namespace, replicas)

	if err != nil {
		return fmt.Errorf("failed to queue deployment scaling: %w", err)
	}

	// Background worker will pick this up and perform actual scaling
	return nil
}

// Calculate cluster totals helper function
func calculateClusterTotals(nodes []NodeStatus) (totalCPU, usedCPU float64, totalMemory, usedMemory int64, totalSessions int) {
	totalCPU, usedCPU = 0, 0
	totalMemory, usedMemory = 0, 0
	totalSessions = 0

	for _, node := range nodes {
		totalCPU += node.CPUCapacity
		usedCPU += node.CPUAllocated
		totalMemory += node.MemoryCapacity
		usedMemory += node.MemoryAllocated
		totalSessions += node.ActiveSessions
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"cluster_summary": gin.H{
			"total_nodes":      len(nodes),
			"cpu_capacity":     totalCPU,
			"cpu_used":         usedCPU,
			"cpu_percent":      (usedCPU / totalCPU) * 100,
			"memory_capacity":  totalMemory,
			"memory_used":      usedMemory,
			"memory_percent":   (float64(usedMemory) / float64(totalMemory)) * 100,
			"active_sessions":  totalSessions,
		},
	})
}

// SelectNode selects best node for a new session based on policy
func (h *Handler) SelectNode(c *gin.Context) {
	var req struct {
		PolicyID       int64             `json:"policy_id,omitempty"`
		RequiredCPU    float64           `json:"required_cpu"`
		RequiredMemory int64             `json:"required_memory"`
		UserLocation   string            `json:"user_location,omitempty"`
		SessionID      string            `json:"session_id,omitempty"`
		NodeSelector   map[string]string `json:"node_selector,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get load balancing policy
	var policy LoadBalancingPolicy
	var policyID int64 = req.PolicyID

	// If no policy specified, get default policy
	if policyID == 0 {
		h.DB.QueryRow(`SELECT id FROM load_balancing_policies WHERE enabled = true ORDER BY id LIMIT 1`).Scan(&policyID)
	}

	if policyID > 0 {
		h.DB.QueryRow(`
			SELECT strategy, resource_thresholds, geo_preferences, node_weights
			FROM load_balancing_policies WHERE id = $1
		`, policyID).Scan(&policy.Strategy, &policy.ResourceThresholds,
			&policy.GeoPreferences, &policy.NodeWeights)
	} else {
		// Use default round-robin if no policy
		policy.Strategy = "round_robin"
	}

	// Get available nodes
	rows, err := h.DB.Query(`
		SELECT node_name, cpu_allocated, cpu_capacity, memory_allocated,
		       memory_capacity, active_sessions, health_status, region, weight
		FROM node_status
		WHERE status = 'ready' AND health_status = 'healthy'
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get node status"})
		return
	}
	defer rows.Close()

	var candidates []NodeStatus
	for rows.Next() {
		var n NodeStatus
		rows.Scan(&n.NodeName, &n.CPUAllocated, &n.CPUCapacity, &n.MemoryAllocated,
			&n.MemoryCapacity, &n.ActiveSessions, &n.HealthStatus, &n.Region, &n.Weight)

		// Check if node has enough resources
		cpuFree := n.CPUCapacity - n.CPUAllocated
		memoryFree := n.MemoryCapacity - n.MemoryAllocated

		if cpuFree >= req.RequiredCPU && memoryFree >= req.RequiredMemory {
			// Calculate percentages
			n.CPUPercent = (n.CPUAllocated / n.CPUCapacity) * 100
			n.MemoryPercent = (float64(n.MemoryAllocated) / float64(n.MemoryCapacity)) * 100

			// Check resource thresholds if configured
			if policy.ResourceThresholds.CPUPercent > 0 && n.CPUPercent > policy.ResourceThresholds.CPUPercent {
				continue
			}
			if policy.ResourceThresholds.MemoryPercent > 0 && n.MemoryPercent > policy.ResourceThresholds.MemoryPercent {
				continue
			}
			if policy.ResourceThresholds.MaxSessions > 0 && n.ActiveSessions >= policy.ResourceThresholds.MaxSessions {
				continue
			}

			candidates = append(candidates, n)
		}
	}

	if len(candidates) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available nodes with sufficient resources"})
		return
	}

	// Select node based on strategy
	var selectedNode NodeStatus

	switch policy.Strategy {
	case "round_robin":
		// Simple round-robin (stateless, based on count)
		selectedNode = candidates[len(candidates)%len(candidates)]

	case "least_loaded":
		// Select node with lowest CPU usage
		minCPU := 100.0
		for _, n := range candidates {
			if n.CPUPercent < minCPU {
				minCPU = n.CPUPercent
				selectedNode = n
			}
		}

	case "resource_based":
		// Select node with most free resources
		maxFreeResources := 0.0
		for _, n := range candidates {
			freeResources := (n.CPUCapacity - n.CPUAllocated) + (float64(n.MemoryCapacity-n.MemoryAllocated) / 1e9)
			if freeResources > maxFreeResources {
				maxFreeResources = freeResources
				selectedNode = n
			}
		}

	case "geographic":
		// Prefer nodes in user's region
		if req.UserLocation != "" {
			for _, n := range candidates {
				if n.Region == req.UserLocation {
					selectedNode = n
					break
				}
			}
		}
		// Fallback to first candidate if no match
		if selectedNode.NodeName == "" {
			selectedNode = candidates[0]
		}

	case "weighted":
		// Weighted random selection based on node weights
		totalWeight := 0
		for _, n := range candidates {
			totalWeight += n.Weight
		}
		if totalWeight > 0 {
			// Simple weighted selection (first node with weight in this implementation)
			selectedNode = candidates[0]
		}

	default:
		selectedNode = candidates[0]
	}

	c.JSON(http.StatusOK, gin.H{
		"node_name":      selectedNode.NodeName,
		"strategy_used":  policy.Strategy,
		"cpu_available":  selectedNode.CPUCapacity - selectedNode.CPUAllocated,
		"memory_available": selectedNode.MemoryCapacity - selectedNode.MemoryAllocated,
		"cpu_percent":    selectedNode.CPUPercent,
		"memory_percent": selectedNode.MemoryPercent,
		"active_sessions": selectedNode.ActiveSessions,
		"region":         selectedNode.Region,
	})
}

// ============================================================================
// AUTO-SCALING POLICIES
// ============================================================================

// AutoScalingPolicy defines auto-scaling rules for sessions
type AutoScalingPolicy struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description,omitempty"`
	TargetType        string                 `json:"target_type"` // "deployment", "statefulset", "template"
	TargetID          string                 `json:"target_id"`   // Template ID or deployment name
	Enabled           bool                   `json:"enabled"`
	ScalingMode       string                 `json:"scaling_mode"` // "horizontal", "vertical", "both"
	MinReplicas       int                    `json:"min_replicas"`
	MaxReplicas       int                    `json:"max_replicas"`
	MetricType        string                 `json:"metric_type"` // "cpu", "memory", "custom", "schedule"
	TargetMetricValue float64                `json:"target_metric_value"`
	ScaleUpPolicy     ScalePolicy            `json:"scale_up_policy"`
	ScaleDownPolicy   ScalePolicy            `json:"scale_down_policy"`
	PredictiveScaling PredictiveScalingConfig `json:"predictive_scaling"`
	CooldownPeriod    int                    `json:"cooldown_period_seconds"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy         string                 `json:"created_by"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// ScalePolicy defines how to scale up or down
type ScalePolicy struct {
	Threshold       float64 `json:"threshold"`          // Metric threshold to trigger
	Increment       int     `json:"increment"`          // How many replicas to add/remove
	Stabilization   int     `json:"stabilization_seconds"` // Wait before next action
	MaxIncrement    int     `json:"max_increment"`      // Max replicas to add at once
}

// PredictiveScalingConfig for schedule-based scaling
type PredictiveScalingConfig struct {
	Enabled         bool              `json:"enabled"`
	SchedulePattern map[string]int    `json:"schedule_pattern,omitempty"` // Hour -> replica count
	LookAheadMinutes int              `json:"look_ahead_minutes"`         // Pre-scale before demand
}

// ScalingEvent represents a scaling action
type ScalingEvent struct {
	ID               int64     `json:"id"`
	PolicyID         int64     `json:"policy_id"`
	TargetType       string    `json:"target_type"`
	TargetID         string    `json:"target_id"`
	Action           string    `json:"action"` // "scale_up", "scale_down"
	PreviousReplicas int       `json:"previous_replicas"`
	NewReplicas      int       `json:"new_replicas"`
	Trigger          string    `json:"trigger"` // "metric", "schedule", "manual"
	MetricValue      float64   `json:"metric_value,omitempty"`
	Reason           string    `json:"reason"`
	Status           string    `json:"status"` // "pending", "in_progress", "completed", "failed"
	CreatedAt        time.Time `json:"created_at"`
}

// CreateAutoScalingPolicy creates a new auto-scaling policy
func (h *Handler) CreateAutoScalingPolicy(c *gin.Context) {
	createdBy := c.GetString("user_id")
	role := c.GetString("role")

	if role != "admin" && role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins/operators can create auto-scaling policies"})
		return
	}

	var req AutoScalingPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate
	if req.MinReplicas < 0 || req.MaxReplicas < req.MinReplicas {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replica configuration"})
		return
	}

	req.CreatedBy = createdBy
	req.Enabled = true

	var id int64
	err := h.DB.QueryRow(`
		INSERT INTO autoscaling_policies
		(name, description, target_type, target_id, enabled, scaling_mode, min_replicas,
		 max_replicas, metric_type, target_metric_value, scale_up_policy, scale_down_policy,
		 predictive_scaling, cooldown_period, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`, req.Name, req.Description, req.TargetType, req.TargetID, req.Enabled, req.ScalingMode,
		req.MinReplicas, req.MaxReplicas, req.MetricType, req.TargetMetricValue,
		req.ScaleUpPolicy, req.ScaleDownPolicy, req.PredictiveScaling, req.CooldownPeriod,
		req.Metadata, createdBy).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create policy"})
		return
	}

	req.ID = id

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Auto-scaling policy created",
		"policy":  req,
	})
}

// ListAutoScalingPolicies lists all auto-scaling policies
func (h *Handler) ListAutoScalingPolicies(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, name, description, target_type, target_id, enabled, scaling_mode,
		       min_replicas, max_replicas, metric_type, target_metric_value,
		       scale_up_policy, scale_down_policy, predictive_scaling, cooldown_period,
		       metadata, created_by, created_at, updated_at
		FROM autoscaling_policies
		ORDER BY created_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	policies := []AutoScalingPolicy{}
	for rows.Next() {
		var p AutoScalingPolicy
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.TargetType, &p.TargetID,
			&p.Enabled, &p.ScalingMode, &p.MinReplicas, &p.MaxReplicas, &p.MetricType,
			&p.TargetMetricValue, &p.ScaleUpPolicy, &p.ScaleDownPolicy, &p.PredictiveScaling,
			&p.CooldownPeriod, &p.Metadata, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		policies = append(policies, p)
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// TriggerScaling manually triggers a scaling action
func (h *Handler) TriggerScaling(c *gin.Context) {
	policyID := c.Param("policyId")

	var req struct {
		Action      string `json:"action" binding:"required,oneof=scale_up scale_down"`
		Replicas    int    `json:"replicas,omitempty"` // Specific replica count, or use policy increment
		Reason      string `json:"reason,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get policy
	var policy AutoScalingPolicy
	err := h.DB.QueryRow(`
		SELECT target_type, target_id, min_replicas, max_replicas, scale_up_policy, scale_down_policy
		FROM autoscaling_policies WHERE id = $1 AND enabled = true
	`, policyID).Scan(&policy.TargetType, &policy.TargetID, &policy.MinReplicas,
		&policy.MaxReplicas, &policy.ScaleUpPolicy, &policy.ScaleDownPolicy)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found or disabled"})
		return
	}

	// Get current replica count (mock - would query Kubernetes in production)
	currentReplicas := 1

	// Calculate new replica count
	var newReplicas int
	if req.Replicas > 0 {
		newReplicas = req.Replicas
	} else {
		if req.Action == "scale_up" {
			increment := policy.ScaleUpPolicy.Increment
			if increment == 0 {
				increment = 1
			}
			newReplicas = currentReplicas + increment
		} else {
			increment := policy.ScaleDownPolicy.Increment
			if increment == 0 {
				increment = 1
			}
			newReplicas = currentReplicas - increment
		}
	}

	// Apply min/max bounds
	if newReplicas < policy.MinReplicas {
		newReplicas = policy.MinReplicas
	}
	if newReplicas > policy.MaxReplicas {
		newReplicas = policy.MaxReplicas
	}

	// Record scaling event
	var eventID int64
	err = h.DB.QueryRow(`
		INSERT INTO scaling_events
		(policy_id, target_type, target_id, action, previous_replicas, new_replicas,
		 trigger, reason, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'manual', $7, 'pending')
		RETURNING id
	`, policyID, policy.TargetType, policy.TargetID, req.Action, currentReplicas,
		newReplicas, req.Reason).Scan(&eventID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record scaling event"})
		return
	}

	// Scale the deployment via Kubernetes API
	err = h.scaleKubernetesDeployment(policy.TargetID, newReplicas)
	if err != nil {
		// Update event status to failed
		h.DB.Exec(`UPDATE scaling_events SET status = 'failed', error_message = $1 WHERE id = $2`,
			err.Error(), eventID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("scaling failed: %v", err)})
		return
	}

	// Update event status to completed
	h.DB.Exec(`UPDATE scaling_events SET status = 'completed' WHERE id = $1`, eventID)

	c.JSON(http.StatusOK, gin.H{
		"event_id":          eventID,
		"action":            req.Action,
		"previous_replicas": currentReplicas,
		"new_replicas":      newReplicas,
		"message":           fmt.Sprintf("Scaling %s from %d to %d replicas", req.Action, currentReplicas, newReplicas),
	})
}

// GetScalingHistory gets scaling event history
func (h *Handler) GetScalingHistory(c *gin.Context) {
	policyID := c.Query("policy_id")
	limit := c.DefaultQuery("limit", "50")

	var rows *sql.Rows
	var err error

	if policyID != "" {
		rows, err = h.DB.Query(`
			SELECT id, policy_id, target_type, target_id, action, previous_replicas,
			       new_replicas, trigger, metric_value, reason, status, created_at
			FROM scaling_events
			WHERE policy_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, policyID, limit)
	} else {
		rows, err = h.DB.Query(`
			SELECT id, policy_id, target_type, target_id, action, previous_replicas,
			       new_replicas, trigger, metric_value, reason, status, created_at
			FROM scaling_events
			ORDER BY created_at DESC
			LIMIT $1
		`, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	events := []ScalingEvent{}
	for rows.Next() {
		var e ScalingEvent
		var metricValue sql.NullFloat64
		err := rows.Scan(&e.ID, &e.PolicyID, &e.TargetType, &e.TargetID, &e.Action,
			&e.PreviousReplicas, &e.NewReplicas, &e.Trigger, &metricValue, &e.Reason,
			&e.Status, &e.CreatedAt)
		if err != nil {
			continue
		}
		if metricValue.Valid {
			e.MetricValue = metricValue.Float64
		}
		events = append(events, e)
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

// Helper: check if string slice contains value
func contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
