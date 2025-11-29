// Package api provides HTTP handlers and WebSocket endpoints for the StreamSpace API.
// This file contains stub implementations, backwards compatibility handlers,
// Kubernetes resource management, and compliance endpoint stubs.
//
// STUB ENDPOINTS OVERVIEW:
//
// This file serves multiple purposes:
// 1. Backwards compatibility for routes migrated to specialized handlers
// 2. Kubernetes resource CRUD operations (generic resource management)
// 3. Stub endpoints for optional plugins (compliance, etc.)
// 4. WebSocket upgrader configuration with origin validation
//
// KUBERNETES RESOURCE MANAGEMENT:
//
// Generic endpoints for managing any Kubernetes resource:
// - CreateResource: Create any K8s resource (Deployment, Service, ConfigMap, etc.)
// - UpdateResource: Update existing K8s resources
// - DeleteResource: Delete K8s resources by type and name
// - ListNodes, ListPods, ListServices, etc.: List cluster resources
// - GetPodLogs: Stream or retrieve pod logs
//
// BACKWARDS COMPATIBILITY:
//
// Some endpoints like ListNodes() are stubs that redirect to specialized handlers:
// - Node management is now in handlers/nodes.go (NodeHandler)
// - User management is in handlers/users.go (UserHandler)
// - These stubs remain for API backwards compatibility during migration
//
// COMPLIANCE STUBS:
//
// The compliance endpoints return stub data when streamspace-compliance plugin
// is not installed. When the plugin is installed, it registers real handlers
// that override these stubs.
//
// WEBSOCKET CONFIGURATION:
//
// The WebSocket upgrader checks allowed origins from ALLOWED_ORIGINS environment
// variable to prevent CSRF attacks. Set to "*" to allow all origins (development only).
//
// Example ALLOWED_ORIGINS: "http://localhost:3000,https://streamspace.example.com"
package api

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	templateGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "templates",
	}
)

// upgrader configures the WebSocket upgrader with security checks.
// It validates the Origin header to prevent CSRF attacks on WebSocket connections.
//
// Origin Validation:
// - Reads ALLOWED_ORIGINS environment variable (comma-separated list)
// - Default: "http://localhost:3000,http://localhost:5173" (development)
// - Production: Set to your actual domains (e.g., "https://streamspace.example.com")
// - "*" allows all origins (DANGEROUS - development only, never in production)
//
// Security Note:
// WebSocket connections cannot send custom headers from browsers, so we use
// query parameter authentication (?token=...) combined with Origin validation
// to prevent unauthorized cross-origin WebSocket connections.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Get allowed origins from environment variable
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

		// If not set, allow localhost for development only
		if allowedOrigins == "" {
			allowedOrigins = "http://localhost:3000,http://localhost:5173"
		}

		// Special case: "*" means allow all (use with caution)
		if allowedOrigins == "*" {
			log.Println("WARNING: WebSocket accepting connections from all origins")
			return true
		}

		// Check if request origin is in allowed list
		origin := r.Header.Get("Origin")
		for _, allowed := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}

		log.Printf("WebSocket connection rejected from origin: %s", origin)
		return false
	},
}

// ============================================================================
// Health & Version Endpoints
// ============================================================================

// Health returns health status
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "streamspace-api",
	})
}

// Version returns API version
func (h *Handler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "v0.1.0",
		"api":     "v1",
		"phase":   "2.2",
	})
}

// ============================================================================
// Stub Methods (To Be Implemented)
// ============================================================================

// UpdateTemplate updates a template (admin only)
// v2.0-beta: Updates database only (catalog_templates), not Kubernetes CRDs
func (h *Handler) UpdateTemplate(c *gin.Context) {
	templateName := c.Param("id")
	if templateName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template id required"})
		return
	}

	var updateReq struct {
		DisplayName      *string                `json:"displayName"`
		Description      *string                `json:"description"`
		IconURL          *string                `json:"iconUrl"`
		Tags             []string               `json:"tags"`
		Category         *string                `json:"category"`
		AppType          *string                `json:"appType"`
		Manifest         *json.RawMessage       `json:"manifest"` // Full Template CRD spec
		DefaultResources *struct {
			Memory string `json:"memory"`
			CPU    string `json:"cpu"`
		} `json:"defaultResources"` // Optional: updates manifest if provided
	}

	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// v2.0-beta: Get existing template from database
	template, err := h.templateDB.GetTemplateByName(c.Request.Context(), templateName)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply updates to template metadata
	if updateReq.DisplayName != nil {
		template.DisplayName = *updateReq.DisplayName
	}
	if updateReq.Description != nil {
		template.Description = *updateReq.Description
	}
	if updateReq.IconURL != nil {
		template.IconURL = *updateReq.IconURL
	}
	if updateReq.Tags != nil {
		template.Tags = updateReq.Tags
	}
	if updateReq.Category != nil {
		template.Category = *updateReq.Category
	}
	if updateReq.AppType != nil {
		template.AppType = *updateReq.AppType
	}

	// Handle manifest updates
	if updateReq.Manifest != nil {
		template.Manifest = *updateReq.Manifest
	} else if updateReq.DefaultResources != nil {
		// Update defaultResources within the existing manifest
		var manifestMap map[string]interface{}
		if err := json.Unmarshal(template.Manifest, &manifestMap); err == nil {
			if spec, ok := manifestMap["spec"].(map[string]interface{}); ok {
				spec["defaultResources"] = map[string]interface{}{
					"memory": updateReq.DefaultResources.Memory,
					"cpu":    updateReq.DefaultResources.CPU,
				}
				if updatedManifest, err := json.Marshal(manifestMap); err == nil {
					template.Manifest = updatedManifest
				}
			}
		}
	}

	// v2.0-beta: Update template in database (catalog_templates)
	// Agent will fetch updated template from database when creating sessions
	if err := h.templateDB.UpdateTemplate(c.Request.Context(), template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Template updated successfully",
		"template": template,
	})
}

// ListNodes returns cluster nodes
// Note: This is now implemented in handlers/nodes.go via NodeHandler
// This stub remains for backwards compatibility with old routes
// v2.0-beta: Returns stub data when API runs without K8s access
func (h *Handler) ListNodes(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	nodes, err := h.k8sClient.GetNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// ListPods returns pods in namespace
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) ListPods(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = h.namespace
	}

	pods, err := h.k8sClient.GetPods(c.Request.Context(), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pods)
}

// ListDeployments returns deployments
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) ListDeployments(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = h.namespace
	}

	deployments, err := h.k8sClient.GetClientset().AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deployments)
}

// ListServices returns services
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) ListServices(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = h.namespace
	}

	services, err := h.k8sClient.GetServices(c.Request.Context(), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

// ListNamespaces returns namespaces
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) ListNamespaces(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	namespaces, err := h.k8sClient.GetNamespaces(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, namespaces)
}

// CreateResource creates a K8s resource
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) CreateResource(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	var req struct {
		APIVersion string                 `json:"apiVersion" binding:"required"`
		Kind       string                 `json:"kind" binding:"required"`
		Metadata   map[string]interface{} `json:"metadata" binding:"required"`
		Spec       map[string]interface{} `json:"spec"`
		Data       map[string]interface{} `json:"data"` // For ConfigMaps, Secrets, etc.
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Get namespace from metadata or use default
	namespace := h.namespace
	if meta, ok := req.Metadata["namespace"].(string); ok && meta != "" {
		namespace = meta
	}

	// Build the resource object
	resource := map[string]interface{}{
		"apiVersion": req.APIVersion,
		"kind":       req.Kind,
		"metadata":   req.Metadata,
	}

	if req.Spec != nil {
		resource["spec"] = req.Spec
	}
	if req.Data != nil {
		resource["data"] = req.Data
	}

	// Convert to unstructured
	unstructuredObj := &unstructured.Unstructured{Object: resource}

	// Create the resource using dynamic client
	gvr, err := h.getGVRForKind(req.APIVersion, req.Kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid resource type",
			"message": err.Error(),
		})
		return
	}

	created, err := h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).
		Create(ctx, unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create resource",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, created.Object)
}

// UpdateResource updates a K8s resource
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) UpdateResource(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	_ = c.Param("type") // Resource type not used; Kind from request body
	resourceName := c.Param("name")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = h.namespace
	}

	var req struct {
		APIVersion string                 `json:"apiVersion" binding:"required"`
		Kind       string                 `json:"kind" binding:"required"`
		Metadata   map[string]interface{} `json:"metadata" binding:"required"`
		Spec       map[string]interface{} `json:"spec"`
		Data       map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Build the resource object
	resource := map[string]interface{}{
		"apiVersion": req.APIVersion,
		"kind":       req.Kind,
		"metadata":   req.Metadata,
	}

	if req.Spec != nil {
		resource["spec"] = req.Spec
	}
	if req.Data != nil {
		resource["data"] = req.Data
	}

	// Ensure name matches
	if meta, ok := resource["metadata"].(map[string]interface{}); ok {
		meta["name"] = resourceName
		meta["namespace"] = namespace
	}

	// Convert to unstructured
	unstructuredObj := &unstructured.Unstructured{Object: resource}

	// Get GVR for the resource type
	gvr, err := h.getGVRForKind(req.APIVersion, req.Kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid resource type",
			"message": err.Error(),
		})
		return
	}

	// Update the resource
	updated, err := h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).
		Update(ctx, unstructuredObj, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update resource",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, updated.Object)
}

// DeleteResource deletes a K8s resource
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) DeleteResource(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	resourceType := c.Param("type") // e.g., "deployment", "service"
	resourceName := c.Param("name")
	apiVersion := c.Query("apiVersion") // e.g., "apps/v1"
	kind := c.Query("kind")             // e.g., "Deployment"
	namespace := c.Query("namespace")

	if namespace == "" {
		namespace = h.namespace
	}

	if apiVersion == "" || kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "apiVersion and kind query parameters are required",
		})
		return
	}

	ctx := c.Request.Context()

	// Get GVR for the resource type
	gvr, err := h.getGVRForKind(apiVersion, kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid resource type",
			"message": err.Error(),
		})
		return
	}

	// Delete the resource
	err = h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).
		Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete resource",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Resource deleted successfully",
		"name":    resourceName,
		"type":    resourceType,
	})
}

// Helper function to get GroupVersionResource from apiVersion and kind
func (h *Handler) getGVRForKind(apiVersion, kind string) (schema.GroupVersionResource, error) {
	// Parse apiVersion into group and version
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	// Map common kinds to their resource names (plural, lowercase)
	// This is a simplified mapping - in production, use discovery client
	resourceMap := map[string]string{
		"Deployment":  "deployments",
		"Service":     "services",
		"Pod":         "pods",
		"ConfigMap":   "configmaps",
		"Secret":      "secrets",
		"Ingress":     "ingresses",
		"Session":     "sessions",
		"Template":    "templates",
		"StatefulSet": "statefulsets",
		"DaemonSet":   "daemonsets",
		"Job":         "jobs",
		"CronJob":     "cronjobs",
		"Namespace":   "namespaces",
		"Node":        "nodes",
	}

	resource, ok := resourceMap[kind]
	if !ok {
		// Fallback: lowercase + s (not always correct, but common pattern)
		resource = strings.ToLower(kind) + "s"
	}

	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}, nil
}

// GetPodLogs returns pod logs
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) GetPodLogs(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Cluster management not available",
			"message": "API is running without Kubernetes access. Cluster management features are disabled.",
		})
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = h.namespace
	}
	podName := c.Query("pod")
	if podName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pod query parameter required"})
		return
	}

	// Parse optional parameters
	tailLines := int64(100) // Default to last 100 lines
	follow := c.Query("follow") == "true"

	// Get pod logs
	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
		Follow:    follow,
	}

	req := h.k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer stream.Close()

	// If following logs, stream them
	if follow {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			c.Writer.Write([]byte(scanner.Text() + "\n"))
			c.Writer.Flush()
		}
		return
	}

	// Otherwise return all logs
	logs, err := io.ReadAll(stream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, string(logs))
}

// GetConfig returns configuration
// v2.0-beta: Returns default config when API runs without K8s access
func (h *Handler) GetConfig(c *gin.Context) {
	// If no K8s access, return default config
	if h.k8sClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"namespace":     DefaultNamespace,
			"ingressDomain": os.Getenv("INGRESS_DOMAIN"),
			"hibernation": gin.H{
				"enabled":            true,
				"defaultIdleTimeout": "30m",
			},
			"resources": gin.H{
				"defaultMemory": "2Gi",
				"defaultCPU":    "1000m",
			},
		})
		return
	}

	// Get configuration from streamspace-config ConfigMap
	configMap, err := h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Get(
		c.Request.Context(),
		"streamspace-config",
		metav1.GetOptions{},
	)

	if err != nil {
		// Return default config if ConfigMap doesn't exist
		c.JSON(http.StatusOK, gin.H{
			"namespace":     h.namespace,
			"ingressDomain": os.Getenv("INGRESS_DOMAIN"),
			"hibernation": gin.H{
				"enabled":            true,
				"defaultIdleTimeout": "30m",
			},
			"resources": gin.H{
				"defaultMemory": "2Gi",
				"defaultCPU":    "1000m",
			},
		})
		return
	}

	c.JSON(http.StatusOK, configMap.Data)
}

// UpdateConfig updates configuration
// v2.0-beta: Returns error when API runs without K8s access
func (h *Handler) UpdateConfig(c *gin.Context) {
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Configuration management not available",
			"message": "API is running without Kubernetes access. Configuration must be managed via environment variables or database.",
		})
		return
	}

	var config map[string]string
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create ConfigMap
	configMap, err := h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Get(
		c.Request.Context(),
		"streamspace-config",
		metav1.GetOptions{},
	)

	if err != nil {
		// Create new ConfigMap
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "streamspace-config",
				Namespace: h.namespace,
			},
			Data: config,
		}

		_, err = h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Create(
			c.Request.Context(),
			configMap,
			metav1.CreateOptions{},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Update existing ConfigMap
		configMap.Data = config
		_, err = h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Update(
			c.Request.Context(),
			configMap,
			metav1.UpdateOptions{},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
		"config":  config,
	})
}

// NOTE: User management endpoints (ListUsers, CreateUser, GetUser, etc.)
// are fully implemented in api/internal/handlers/users.go by UserHandler.
// Those should be used instead of stub implementations.

// GetMetrics returns comprehensive cluster metrics for the admin dashboard.
//
// This endpoint aggregates data from multiple sources:
// - Kubernetes: Node count, resource capacity, allocatable resources, pod counts
// - Database: Session counts by state, user counts, active users (24hr)
// - Calculations: Resource utilization percentages
//
// Returned Metrics:
// - cluster.nodes: Total, ready, and not-ready node counts
// - cluster.sessions: Total, running, hibernated, and terminated session counts
// - cluster.resources: CPU, memory, and pod capacity/usage/percentages
// - cluster.users: Total user count and active users (logged in last 24 hours)
//
// Resource Estimates:
// Used CPU/memory are estimates based on running session count (1 core, 2GB per session).
// For accurate resource usage, deploy metrics-server and query real pod metrics.
//
// GET /api/v1/metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	ctx := c.Request.Context()

	// Initialize default values
	var err error
	totalAgents := 0
	readyAgents := 0

	// ISSUE #234: Query agents table instead of K8s cluster nodes
	// The dashboard should show registered agents, not K8s infrastructure nodes
	err = h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE approval_status = 'approved') as total,
			COUNT(*) FILTER (WHERE approval_status = 'approved' AND status = 'online') as ready
		FROM agents
	`).Scan(&totalAgents, &readyAgents)

	if err != nil {
		log.Printf("Failed to get agent counts: %v", err)
		// Use zeros if query fails
		totalAgents = 0
		readyAgents = 0
	}

	// Get session counts from database
	var sessionCounts struct {
		Total      int
		Running    int
		Hibernated int
		Terminated int
	}

	err = h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE state = 'running') as running,
			COUNT(*) FILTER (WHERE state = 'hibernated') as hibernated,
			COUNT(*) FILTER (WHERE state = 'terminated') as terminated
		FROM sessions
	`).Scan(&sessionCounts.Total, &sessionCounts.Running, &sessionCounts.Hibernated, &sessionCounts.Terminated)

	if err != nil {
		log.Printf("Failed to get session counts: %v", err)
		// Use zeros if query fails
		sessionCounts = struct {
			Total, Running, Hibernated, Terminated int
		}{0, 0, 0, 0}
	}

	// Get user counts from database
	var userCounts struct {
		Total  int
		Active int
	}

	err = h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE last_login > NOW() - INTERVAL '24 hours') as active
		FROM users
	`).Scan(&userCounts.Total, &userCounts.Active)

	if err != nil {
		log.Printf("Failed to get user counts: %v", err)
		// Use zeros if query fails
		userCounts = struct{ Total, Active int }{0, 0}
	}

	// ISSUE #234: Resource metrics are not available without K8s client
	// For now, return zeros. In the future, could aggregate agent capacity.
	totalCPU := int64(0)
	usedCPU := int64(0)
	totalMemory := int64(0)
	usedMemory := int64(0)
	totalPods := 0
	usedPods := 0
	cpuPercent := float64(0)
	memoryPercent := float64(0)
	podsPercent := float64(0)

	// ISSUE #234: Return agent metrics in the format expected by AdminDashboard
	// Dashboard now shows approved agents instead of K8s cluster nodes
	c.JSON(http.StatusOK, gin.H{
		"cluster": gin.H{
			"nodes": gin.H{
				"total":    totalAgents,
				"ready":    readyAgents,
				"notReady": totalAgents - readyAgents,
			},
			"sessions": gin.H{
				"total":      sessionCounts.Total,
				"running":    sessionCounts.Running,
				"hibernated": sessionCounts.Hibernated,
				"terminated": sessionCounts.Terminated,
			},
			"resources": gin.H{
				"cpu": gin.H{
					"total":   fmt.Sprintf("%dm", totalCPU),
					"used":    fmt.Sprintf("%dm", usedCPU),
					"percent": cpuPercent,
				},
				"memory": gin.H{
					"total":   fmt.Sprintf("%d", totalMemory),
					"used":    fmt.Sprintf("%d", usedMemory),
					"percent": memoryPercent,
				},
				"pods": gin.H{
					"total":   totalPods,
					"used":    usedPods,
					"percent": podsPercent,
				},
			},
			"users": gin.H{
				"total":  userCounts.Total,
				"active": userCounts.Active,
			},
		},
	})
}

// ============================================================================
// WebSocket Endpoints
// ============================================================================

// SessionsWebSocket handles WebSocket for real-time session updates
// Supports query parameters:
// - ?user_id=<userID> - Subscribe to events for a specific user (defaults to authenticated user)
// - ?session_id=<sessionID> - Subscribe to events for a specific session
func (h *Handler) SessionsWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	// Get user ID from context (authenticated user)
	authenticatedUserID, _ := c.Get("userID")
	userIDStr := ""
	if authenticatedUserID != nil {
		if id, ok := authenticatedUserID.(string); ok {
			userIDStr = id
		}
	}

	// Allow overriding user_id from query param (for admins/operators)
	// But for security, regular users can only subscribe to their own events
	queryUserID := c.Query("user_id")
	if queryUserID != "" && queryUserID != userIDStr {
		// Check if user has admin or operator role
		role := c.GetString("role")
		if role != "admin" && role != "operator" {
			// Regular users can only subscribe to their own events
			log.Printf("Unauthorized attempt to subscribe to user %s by user %s (role: %s)", queryUserID, userIDStr, role)
			conn.WriteJSON(map[string]interface{}{
				"error": "Unauthorized: Only admins and operators can subscribe to other users' events",
			})
			conn.Close()
			return
		}
		// Admin/operator can subscribe to any user's events
		userIDStr = queryUserID
	}

	// Get session ID from query params (optional)
	sessionID := c.Query("session_id")

	h.wsManager.HandleSessionsWebSocket(conn, userIDStr, sessionID)
}

// ClusterWebSocket handles WebSocket for real-time cluster updates
// Only admins and operators can view cluster-wide metrics
func (h *Handler) ClusterWebSocket(c *gin.Context) {
	// Check if user has admin or operator role
	role := c.GetString("role")
	if role != "admin" && role != "operator" {
		userID := c.GetString("user_id")
		log.Printf("Unauthorized attempt to access cluster metrics by user %s (role: %s)", userID, role)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Unauthorized: Only admins and operators can view cluster metrics",
		})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	h.wsManager.HandleMetricsWebSocket(conn)
}

// LogsWebSocket handles WebSocket for streaming pod logs
// Only admins and operators can view pod logs
func (h *Handler) LogsWebSocket(c *gin.Context) {
	// Check if user has admin or operator role
	role := c.GetString("role")
	if role != "admin" && role != "operator" {
		userID := c.GetString("user_id")
		log.Printf("Unauthorized attempt to access pod logs by user %s (role: %s)", userID, role)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Unauthorized: Only admins and operators can view pod logs",
		})
		return
	}

	namespace := c.Param("namespace")
	podName := c.Param("pod")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	h.wsManager.HandleLogsWebSocket(conn, namespace, podName)
}

// ============================================================================
// Catalog/Repository Endpoints (Additional)
// ============================================================================

// BrowseCatalog returns catalog templates (alias for ListCatalogTemplates)
func (h *Handler) BrowseCatalog(c *gin.Context) {
	h.ListCatalogTemplates(c)
}

// InstallTemplate installs a template from catalog (alias for InstallCatalogTemplate)
func (h *Handler) InstallTemplate(c *gin.Context) {
	catalogID := c.Query("id")
	if catalogID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter required"})
		return
	}

	c.Params = append(c.Params, gin.Param{Key: "id", Value: catalogID})
	h.InstallCatalogTemplate(c)
}

// SyncCatalog triggers sync for all repositories
func (h *Handler) SyncCatalog(c *gin.Context) {
	go func() {
		if err := h.syncService.SyncAllRepositories(c.Request.Context()); err != nil {
			log.Printf("Catalog sync failed: %v", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Catalog sync triggered",
		"status":  "syncing",
	})
}

// RemoveRepository removes a repository (alias for DeleteRepository)
func (h *Handler) RemoveRepository(c *gin.Context) {
	h.DeleteRepository(c)
}

// ============================================================================
// Webhook Endpoint for Repository Auto-Sync
// ============================================================================

// WebhookRepositorySync handles webhooks from Git providers for auto-sync
func (h *Handler) WebhookRepositorySync(c *gin.Context) {
	var webhook struct {
		RepositoryURL string `json:"repository_url"`
		Branch        string `json:"branch"`
		Ref           string `json:"ref"`
	}

	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find repository by URL
	ctx := c.Request.Context()
	var repoID int
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id FROM repositories WHERE url = $1
	`, webhook.RepositoryURL).Scan(&repoID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	// Trigger sync in background
	go func() {
		if err := h.syncService.SyncRepository(ctx, repoID); err != nil {
			log.Printf("Webhook-triggered sync failed for repository %d: %v", repoID, err)
		} else {
			log.Printf("Webhook-triggered sync completed for repository %d", repoID)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Webhook received, sync triggered",
		"repository":   webhook.RepositoryURL,
		"repositoryID": repoID,
	})
}

// ====================================================================================
// COMPLIANCE STUBS
// ====================================================================================
// NOTE: These are stub endpoints that return empty data when the compliance plugin
// is not installed. When streamspace-compliance plugin is installed, it will
// override these endpoints with actual functionality.
// ====================================================================================

// ListComplianceFrameworks returns available compliance frameworks
// Stub returns empty list - install streamspace-compliance plugin for real data
func (h *Handler) ListComplianceFrameworks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"frameworks": []gin.H{},
	})
}

// CreateComplianceFramework creates a new compliance framework
// Stub returns not implemented - install streamspace-compliance plugin
func (h *Handler) CreateComplianceFramework(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Compliance features require the streamspace-compliance plugin",
		"message": "Please install the streamspace-compliance plugin from Admin → Plugins",
	})
}

// ListCompliancePolicies returns all compliance policies
// Stub returns empty list - install streamspace-compliance plugin for real data
func (h *Handler) ListCompliancePolicies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"policies": []gin.H{},
	})
}

// CreateCompliancePolicy creates a new compliance policy
// Stub returns not implemented - install streamspace-compliance plugin
func (h *Handler) CreateCompliancePolicy(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Compliance features require the streamspace-compliance plugin",
		"message": "Please install the streamspace-compliance plugin from Admin → Plugins",
	})
}

// ListViolations returns all compliance violations
// Stub returns empty list - install streamspace-compliance plugin for real data
func (h *Handler) ListViolations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"violations": []gin.H{},
	})
}

// RecordViolation records a new compliance violation
// Stub returns not implemented - install streamspace-compliance plugin
func (h *Handler) RecordViolation(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Compliance features require the streamspace-compliance plugin",
		"message": "Please install the streamspace-compliance plugin from Admin → Plugins",
	})
}

// ResolveViolation marks a violation as resolved
// Stub returns not implemented - install streamspace-compliance plugin
func (h *Handler) ResolveViolation(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Compliance features require the streamspace-compliance plugin",
		"message": "Please install the streamspace-compliance plugin from Admin → Plugins",
	})
}

// GetComplianceDashboard returns compliance dashboard metrics
// Stub returns zero metrics - install streamspace-compliance plugin for real data
func (h *Handler) GetComplianceDashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"total_policies":   0,
		"active_policies":  0,
		"total_open_violations": 0,
		"violations_by_severity": gin.H{
			"critical": 0,
			"high":     0,
			"medium":   0,
			"low":      0,
		},
	})
}
