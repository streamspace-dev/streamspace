// Package k8s provides Kubernetes client functionality for StreamSpace CRD operations.
//
// This file implements the Kubernetes client wrapper for StreamSpace custom resources.
//
// Purpose:
// - Provide typed access to StreamSpace CRDs (Session, Template)
// - Wrap Kubernetes dynamic client for custom resource operations
// - Simplify CRUD operations on Sessions and Templates
// - Watch for changes to sessions and templates
// - Access core Kubernetes resources (Pods, Services, PVCs, Nodes)
//
// Features:
// - Session management (create, get, list, update, delete, watch)
// - Template management (create, get, list, delete, watch)
// - State transitions (running → hibernated → terminated)
// - Resource filtering (by user, category, labels)
// - Cluster resource introspection (nodes, pods, services)
// - Auto-configuration (in-cluster or kubeconfig)
//
// Custom Resource Definitions:
//
//   - Sessions (stream.space/v1alpha1)
//
//   - Represents a user's containerized workspace session
//
//   - States: running, hibernated, terminated
//
//   - Includes resource limits, idle timeout, persistence settings
//
//   - Templates (stream.space/v1alpha1)
//
//   - Defines application templates (Firefox, VS Code, etc.)
//
//   - Contains container image, VNC/webapp config, resources
//
//   - Categorized for catalog organization
//
// Implementation Details:
// - Uses Kubernetes dynamic client for CRD operations
// - Auto-detects in-cluster vs local configuration
// - Parses unstructured data to typed Go structs
// - Supports namespace isolation (default: "streamspace")
// - Thread-safe client operations
//
// Thread Safety:
// - Kubernetes clients are thread-safe
// - Safe for concurrent access across goroutines
//
// Dependencies:
// - k8s.io/client-go for Kubernetes API access
// - k8s.io/api for core resource types
//
// Example Usage:
//
//	// Initialize client
//	client, err := k8s.NewClient()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a session
//	session := &k8s.Session{
//	    Name:           "user1-firefox",
//	    Namespace:      "streamspace",
//	    User:           "user1",
//	    Template:       "firefox-browser",
//	    State:          "running",
//	    PersistentHome: true,
//	    IdleTimeout:    "30m",
//	}
//	created, err := client.CreateSession(ctx, session)
//
//	// List sessions for a user
//	sessions, err := client.ListSessionsByUser(ctx, "streamspace", "user1")
//
//	// Update session state (hibernate)
//	updated, err := client.UpdateSessionState(ctx, "streamspace", "user1-firefox", "hibernated")
//
//	// Watch for session changes
//	watcher, err := client.WatchSessions(ctx, "streamspace")
//	for event := range watcher.ResultChan() {
//	    session := event.Object.(*unstructured.Unstructured)
//	    // Handle session change
//	}
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Session represents a StreamSpace Session CRD
type Session struct {
	Name      string
	Namespace string
	User      string
	Template  string
	State     string // running, hibernated, terminated
	Resources struct {
		Memory string
		CPU    string
	}
	PersistentHome     bool
	IdleTimeout        string
	MaxSessionDuration string
	Tags               []string
	Status             SessionStatus
	CreatedAt          time.Time
}

// SessionStatus represents the status of a Session
type SessionStatus struct {
	Phase         string // Pending, Running, Hibernated, Failed, Terminated
	PodName       string
	URL           string
	LastActivity  *time.Time
	ResourceUsage struct {
		Memory string
		CPU    string
	}
	Conditions []metav1.Condition
}

// Template represents a StreamSpace Template CRD
type Template struct {
	Name             string
	Namespace        string
	DisplayName      string
	Description      string
	Category         string
	Icon             string
	BaseImage        string
	AppType          string // desktop, webapp
	DefaultResources struct {
		Memory string
		CPU    string
	}
	Ports []struct {
		Name          string
		ContainerPort int32
		Protocol      string
	}
	Env          []corev1.EnvVar
	VolumeMounts []corev1.VolumeMount
	VNC          *VNCConfig
	WebApp       *WebAppConfig
	Capabilities []string
	Tags         []string
	Featured     bool // Whether template is featured in catalog
	UsageCount   int  // Number of times template has been used
	CreatedAt    time.Time
}

// VNCConfig represents VNC configuration for desktop apps
type VNCConfig struct {
	Enabled  bool
	Port     int32
	Protocol string
}

// WebAppConfig represents webapp configuration for native web apps
type WebAppConfig struct {
	Enabled bool
	Port    int32
	Path    string
}

// ApplicationInstall represents a request to install an application
// The controller watches these and creates the corresponding Template CRD
type ApplicationInstall struct {
	Name              string
	Namespace         string
	CatalogTemplateID int
	TemplateName      string
	DisplayName       string
	Description       string
	Category          string
	Icon              string
	Manifest          string // YAML manifest for the Template
	InstalledBy       string
	// Status fields
	Phase                string // Pending, Creating, Ready, Failed
	StatusMessage        string
	LastTransitionTime   *time.Time
	CreatedTemplateName  string
	CreatedTemplateNS    string
}

// Client wraps Kubernetes clients for StreamSpace CRD operations
type Client struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
	namespace     string
}

var (
	sessionGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "sessions",
	}

	templateGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "templates",
	}

	applicationInstallGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "applicationinstalls",
	}
)

// NewClient creates a new Kubernetes client
func NewClient() (*Client, error) {
	config, err := getConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "streamspace"
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		config:        config,
		namespace:     namespace,
	}, nil
}

// getConfig returns Kubernetes config (in-cluster or kubeconfig)
func getConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

// GetDynamicClient returns the underlying dynamic client
func (c *Client) GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// ============================================================================
// Session Operations
// ============================================================================

// CreateSession creates a new Session resource
func (c *Client) CreateSession(ctx context.Context, session *Session) (*Session, error) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":      session.Name,
				"namespace": session.Namespace,
			},
			"spec": map[string]interface{}{
				"user":           session.User,
				"template":       session.Template,
				"state":          session.State,
				"persistentHome": session.PersistentHome,
			},
		},
	}

	// Add optional fields
	spec := obj.Object["spec"].(map[string]interface{})

	if session.Resources.Memory != "" || session.Resources.CPU != "" {
		resources := make(map[string]interface{})
		if session.Resources.Memory != "" {
			resources["memory"] = session.Resources.Memory
		}
		if session.Resources.CPU != "" {
			resources["cpu"] = session.Resources.CPU
		}
		spec["resources"] = resources
	}

	if session.IdleTimeout != "" {
		spec["idleTimeout"] = session.IdleTimeout
	}

	if session.MaxSessionDuration != "" {
		spec["maxSessionDuration"] = session.MaxSessionDuration
	}

	if len(session.Tags) > 0 {
		spec["tags"] = session.Tags
	}

	result, err := c.dynamicClient.Resource(sessionGVR).Namespace(session.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return parseSession(result)
}

// GetSession retrieves a Session by name
func (c *Client) GetSession(ctx context.Context, namespace, name string) (*Session, error) {
	obj, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return parseSession(obj)
}

// ListSessions lists all Sessions in a namespace
func (c *Client) ListSessions(ctx context.Context, namespace string) ([]*Session, error) {
	list, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*Session, 0, len(list.Items))
	for _, item := range list.Items {
		session, err := parseSession(&item)
		if err != nil {
			// Log error but continue
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// ListSessionsByUser lists Sessions for a specific user
func (c *Client) ListSessionsByUser(ctx context.Context, namespace, user string) ([]*Session, error) {
	list, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("user=%s", user),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by user: %w", err)
	}

	sessions := make([]*Session, 0, len(list.Items))
	for _, item := range list.Items {
		session, err := parseSession(&item)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// UpdateSessionState updates the state of a Session (running, hibernated, terminated)
func (c *Client) UpdateSessionState(ctx context.Context, namespace, name, state string) (*Session, error) {
	obj, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid session spec")
	}

	spec["state"] = state

	result, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update session state: %w", err)
	}

	return parseSession(result)
}

// UpdateSession updates a Session resource
func (c *Client) UpdateSession(ctx context.Context, session *Session) error {
	obj, err := c.dynamicClient.Resource(sessionGVR).Namespace(session.Namespace).Get(ctx, session.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update spec
	spec := map[string]interface{}{
		"user":           session.User,
		"template":       session.Template,
		"state":          session.State,
		"persistentHome": session.PersistentHome,
	}

	if session.IdleTimeout != "" {
		spec["idleTimeout"] = session.IdleTimeout
	}

	if session.MaxSessionDuration != "" {
		spec["maxSessionDuration"] = session.MaxSessionDuration
	}

	if session.Resources.Memory != "" || session.Resources.CPU != "" {
		resources := make(map[string]interface{})
		if session.Resources.Memory != "" {
			resources["memory"] = session.Resources.Memory
		}
		if session.Resources.CPU != "" {
			resources["cpu"] = session.Resources.CPU
		}
		spec["resources"] = resources
	}

	if len(session.Tags) > 0 {
		spec["tags"] = session.Tags
	}

	obj.Object["spec"] = spec

	_, err = c.dynamicClient.Resource(sessionGVR).Namespace(session.Namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// UpdateSessionStatus updates a Session's status subresource
func (c *Client) UpdateSessionStatus(ctx context.Context, session *Session) error {
	obj, err := c.dynamicClient.Resource(sessionGVR).Namespace(session.Namespace).Get(ctx, session.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Build status object
	status := make(map[string]interface{})

	if session.Status.Phase != "" {
		status["phase"] = session.Status.Phase
	}

	if session.Status.PodName != "" {
		status["podName"] = session.Status.PodName
	}

	if session.Status.URL != "" {
		status["url"] = session.Status.URL
	}

	if session.Status.LastActivity != nil {
		status["lastActivity"] = session.Status.LastActivity.Format(time.RFC3339)
	}

	if session.Status.ResourceUsage.Memory != "" || session.Status.ResourceUsage.CPU != "" {
		resourceUsage := make(map[string]interface{})
		if session.Status.ResourceUsage.Memory != "" {
			resourceUsage["memory"] = session.Status.ResourceUsage.Memory
		}
		if session.Status.ResourceUsage.CPU != "" {
			resourceUsage["cpu"] = session.Status.ResourceUsage.CPU
		}
		status["resourceUsage"] = resourceUsage
	}

	obj.Object["status"] = status

	// Update status subresource
	_, err = c.dynamicClient.Resource(sessionGVR).Namespace(session.Namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	return nil
}

// DeleteSession deletes a Session
func (c *Client) DeleteSession(ctx context.Context, namespace, name string) error {
	err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// WatchSessions watches for Session changes
func (c *Client) WatchSessions(ctx context.Context, namespace string) (watch.Interface, error) {
	watcher, err := c.dynamicClient.Resource(sessionGVR).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to watch sessions: %w", err)
	}

	return watcher, nil
}

// parseSession converts unstructured Session to typed Session
func parseSession(obj *unstructured.Unstructured) (*Session, error) {
	session := &Session{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	// Parse spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid session spec")
	}

	if user, ok := spec["user"].(string); ok {
		session.User = user
	}

	if template, ok := spec["template"].(string); ok {
		session.Template = template
	}

	if state, ok := spec["state"].(string); ok {
		session.State = state
	}

	if persistentHome, ok := spec["persistentHome"].(bool); ok {
		session.PersistentHome = persistentHome
	}

	if idleTimeout, ok := spec["idleTimeout"].(string); ok {
		session.IdleTimeout = idleTimeout
	}

	if maxSessionDuration, ok := spec["maxSessionDuration"].(string); ok {
		session.MaxSessionDuration = maxSessionDuration
	}

	if resources, ok := spec["resources"].(map[string]interface{}); ok {
		// Try new nested structure first (requests/limits)
		if requests, ok := resources["requests"].(map[string]interface{}); ok {
			if memory, ok := requests["memory"].(string); ok {
				session.Resources.Memory = memory
			}
			if cpu, ok := requests["cpu"].(string); ok {
				session.Resources.CPU = cpu
			}
		}
		// Fall back to flat structure for backwards compatibility
		if session.Resources.Memory == "" {
			if memory, ok := resources["memory"].(string); ok {
				session.Resources.Memory = memory
			}
		}
		if session.Resources.CPU == "" {
			if cpu, ok := resources["cpu"].(string); ok {
				session.Resources.CPU = cpu
			}
		}
	}

	if tags, ok := spec["tags"].([]interface{}); ok {
		session.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				session.Tags = append(session.Tags, tagStr)
			}
		}
	}

	// Parse status
	if status, ok := obj.Object["status"].(map[string]interface{}); ok {
		if phase, ok := status["phase"].(string); ok {
			session.Status.Phase = phase
		}
		if podName, ok := status["podName"].(string); ok {
			session.Status.PodName = podName
		}
		if url, ok := status["url"].(string); ok {
			session.Status.URL = url
		}
		if lastActivity, ok := status["lastActivity"].(string); ok {
			t, err := time.Parse(time.RFC3339, lastActivity)
			if err == nil {
				session.Status.LastActivity = &t
			}
		}
		if resourceUsage, ok := status["resourceUsage"].(map[string]interface{}); ok {
			if memory, ok := resourceUsage["memory"].(string); ok {
				session.Status.ResourceUsage.Memory = memory
			}
			if cpu, ok := resourceUsage["cpu"].(string); ok {
				session.Status.ResourceUsage.CPU = cpu
			}
		}
	}

	return session, nil
}

// ============================================================================
// Template Operations
// ============================================================================

// CreateTemplate creates a new Template resource
func (c *Client) CreateTemplate(ctx context.Context, template *Template) (*Template, error) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Template",
			"metadata": map[string]interface{}{
				"name":      template.Name,
				"namespace": template.Namespace,
			},
			"spec": map[string]interface{}{
				"displayName": template.DisplayName,
				"description": template.Description,
				"category":    template.Category,
				"baseImage":   template.BaseImage,
			},
		},
	}

	spec := obj.Object["spec"].(map[string]interface{})

	// Add optional fields
	if template.Icon != "" {
		spec["icon"] = template.Icon
	}

	if template.AppType != "" {
		spec["appType"] = template.AppType
	}

	if template.DefaultResources.Memory != "" || template.DefaultResources.CPU != "" {
		resources := make(map[string]interface{})
		if template.DefaultResources.Memory != "" {
			resources["memory"] = template.DefaultResources.Memory
		}
		if template.DefaultResources.CPU != "" {
			resources["cpu"] = template.DefaultResources.CPU
		}
		spec["defaultResources"] = resources
	}

	if len(template.Tags) > 0 {
		spec["tags"] = template.Tags
	}

	if len(template.Capabilities) > 0 {
		spec["capabilities"] = template.Capabilities
	}

	result, err := c.dynamicClient.Resource(templateGVR).Namespace(template.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return parseTemplate(result)
}

// GetTemplate retrieves a Template by name
func (c *Client) GetTemplate(ctx context.Context, namespace, name string) (*Template, error) {
	obj, err := c.dynamicClient.Resource(templateGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return parseTemplate(obj)
}

// ListTemplates lists all Templates in a namespace
func (c *Client) ListTemplates(ctx context.Context, namespace string) ([]*Template, error) {
	list, err := c.dynamicClient.Resource(templateGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	templates := make([]*Template, 0, len(list.Items))
	for _, item := range list.Items {
		template, err := parseTemplate(&item)
		if err != nil {
			continue
		}
		templates = append(templates, template)
	}

	return templates, nil
}

// ListTemplatesByCategory lists Templates by category
func (c *Client) ListTemplatesByCategory(ctx context.Context, namespace, category string) ([]*Template, error) {
	list, err := c.dynamicClient.Resource(templateGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("category=%s", category),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list templates by category: %w", err)
	}

	templates := make([]*Template, 0, len(list.Items))
	for _, item := range list.Items {
		template, err := parseTemplate(&item)
		if err != nil {
			continue
		}
		templates = append(templates, template)
	}

	return templates, nil
}

// DeleteTemplate deletes a Template
func (c *Client) DeleteTemplate(ctx context.Context, namespace, name string) error {
	err := c.dynamicClient.Resource(templateGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	return nil
}

// WatchTemplates watches for Template changes
func (c *Client) WatchTemplates(ctx context.Context, namespace string) (watch.Interface, error) {
	watcher, err := c.dynamicClient.Resource(templateGVR).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to watch templates: %w", err)
	}

	return watcher, nil
}

// parseTemplate converts unstructured Template to typed Template
func parseTemplate(obj *unstructured.Unstructured) (*Template, error) {
	template := &Template{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid template spec")
	}

	if displayName, ok := spec["displayName"].(string); ok {
		template.DisplayName = displayName
	}

	if description, ok := spec["description"].(string); ok {
		template.Description = description
	}

	if category, ok := spec["category"].(string); ok {
		template.Category = category
	}

	if icon, ok := spec["icon"].(string); ok {
		template.Icon = icon
	}

	if baseImage, ok := spec["baseImage"].(string); ok {
		template.BaseImage = baseImage
	}

	if appType, ok := spec["appType"].(string); ok {
		template.AppType = appType
	}

	if resources, ok := spec["defaultResources"].(map[string]interface{}); ok {
		if memory, ok := resources["memory"].(string); ok {
			template.DefaultResources.Memory = memory
		}
		if cpu, ok := resources["cpu"].(string); ok {
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

	if featured, ok := spec["featured"].(bool); ok {
		template.Featured = featured
	}

	if usageCount, ok := spec["usageCount"].(float64); ok {
		template.UsageCount = int(usageCount)
	}

	return template, nil
}

// ============================================================================
// ApplicationInstall Operations
// ============================================================================

// CreateApplicationInstall creates a new ApplicationInstall resource
// The controller will watch this and create the corresponding Template CRD
func (c *Client) CreateApplicationInstall(ctx context.Context, appInstall *ApplicationInstall) (*ApplicationInstall, error) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "ApplicationInstall",
			"metadata": map[string]interface{}{
				"name":      appInstall.Name,
				"namespace": appInstall.Namespace,
			},
			"spec": map[string]interface{}{
				"catalogTemplateID": appInstall.CatalogTemplateID,
				"templateName":      appInstall.TemplateName,
				"displayName":       appInstall.DisplayName,
				"description":       appInstall.Description,
				"category":          appInstall.Category,
				"icon":              appInstall.Icon,
				"manifest":          appInstall.Manifest,
				"installedBy":       appInstall.InstalledBy,
			},
		},
	}

	created, err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(appInstall.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create ApplicationInstall: %w", err)
	}

	return parseApplicationInstall(created)
}

// GetApplicationInstall retrieves an ApplicationInstall by name
func (c *Client) GetApplicationInstall(ctx context.Context, namespace, name string) (*ApplicationInstall, error) {
	obj, err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ApplicationInstall: %w", err)
	}

	return parseApplicationInstall(obj)
}

// ListApplicationInstalls lists all ApplicationInstalls in a namespace
func (c *Client) ListApplicationInstalls(ctx context.Context, namespace string) ([]*ApplicationInstall, error) {
	list, err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ApplicationInstalls: %w", err)
	}

	appInstalls := make([]*ApplicationInstall, 0, len(list.Items))
	for _, item := range list.Items {
		appInstall, err := parseApplicationInstall(&item)
		if err != nil {
			continue
		}
		appInstalls = append(appInstalls, appInstall)
	}

	return appInstalls, nil
}

// UpdateApplicationInstallStatus updates the status of an ApplicationInstall
func (c *Client) UpdateApplicationInstallStatus(ctx context.Context, namespace, name string, phase, message string) error {
	obj, err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ApplicationInstall: %w", err)
	}

	// Update status
	status := map[string]interface{}{
		"phase":              phase,
		"message":            message,
		"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
	}
	obj.Object["status"] = status

	_, err = c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ApplicationInstall status: %w", err)
	}

	return nil
}

// DeleteApplicationInstall deletes an ApplicationInstall
func (c *Client) DeleteApplicationInstall(ctx context.Context, namespace, name string) error {
	err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete ApplicationInstall: %w", err)
	}

	return nil
}

// WatchApplicationInstalls watches for ApplicationInstall changes
func (c *Client) WatchApplicationInstalls(ctx context.Context, namespace string) (watch.Interface, error) {
	watcher, err := c.dynamicClient.Resource(applicationInstallGVR).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to watch ApplicationInstalls: %w", err)
	}

	return watcher, nil
}

// parseApplicationInstall converts unstructured to typed ApplicationInstall
func parseApplicationInstall(obj *unstructured.Unstructured) (*ApplicationInstall, error) {
	appInstall := &ApplicationInstall{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ApplicationInstall spec")
	}

	if catalogTemplateID, ok := spec["catalogTemplateID"].(float64); ok {
		appInstall.CatalogTemplateID = int(catalogTemplateID)
	}
	if templateName, ok := spec["templateName"].(string); ok {
		appInstall.TemplateName = templateName
	}
	if displayName, ok := spec["displayName"].(string); ok {
		appInstall.DisplayName = displayName
	}
	if description, ok := spec["description"].(string); ok {
		appInstall.Description = description
	}
	if category, ok := spec["category"].(string); ok {
		appInstall.Category = category
	}
	if icon, ok := spec["icon"].(string); ok {
		appInstall.Icon = icon
	}
	if manifest, ok := spec["manifest"].(string); ok {
		appInstall.Manifest = manifest
	}
	if installedBy, ok := spec["installedBy"].(string); ok {
		appInstall.InstalledBy = installedBy
	}

	// Parse status
	if status, ok := obj.Object["status"].(map[string]interface{}); ok {
		if phase, ok := status["phase"].(string); ok {
			appInstall.Phase = phase
		}
		if message, ok := status["message"].(string); ok {
			appInstall.StatusMessage = message
		}
		if templateName, ok := status["templateName"].(string); ok {
			appInstall.CreatedTemplateName = templateName
		}
		if templateNS, ok := status["templateNamespace"].(string); ok {
			appInstall.CreatedTemplateNS = templateNS
		}
	}

	return appInstall, nil
}

// ============================================================================
// Cluster Operations (for Admin UI)
// ============================================================================

// GetNodes returns all cluster nodes
func (c *Client) GetNodes(ctx context.Context) (*corev1.NodeList, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return nodes, nil
}

// GetPods returns pods in a namespace
func (c *Client) GetPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods, nil
}

// GetServices returns services in a namespace
func (c *Client) GetServices(ctx context.Context, namespace string) (*corev1.ServiceList, error) {
	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	return services, nil
}

// GetPVCs returns PVCs in a namespace
func (c *Client) GetPVCs(ctx context.Context, namespace string) (*corev1.PersistentVolumeClaimList, error) {
	pvcs, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	return pvcs, nil
}

// GetNamespaces returns all namespaces
func (c *Client) GetNamespaces(ctx context.Context) (*corev1.NamespaceList, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return namespaces, nil
}

// ============================================================================
// Node Management Operations
// ============================================================================

// GetNode returns a specific node by name
func (c *Client) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", name, err)
	}

	return node, nil
}

// PatchNode applies a patch to a node
func (c *Client) PatchNode(ctx context.Context, name string, patchData []byte) error {
	_, err := c.clientset.CoreV1().Nodes().Patch(
		ctx,
		name,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch node %s: %w", name, err)
	}

	return nil
}

// UpdateNodeTaints updates the taints on a node
func (c *Client) UpdateNodeTaints(ctx context.Context, name string, taints []corev1.Taint) error {
	node, err := c.GetNode(ctx, name)
	if err != nil {
		return err
	}

	node.Spec.Taints = taints

	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node taints: %w", err)
	}

	return nil
}

// CordonNode marks a node as unschedulable
func (c *Client) CordonNode(ctx context.Context, name string) error {
	patchData := []byte(`{"spec":{"unschedulable":true}}`)
	return c.PatchNode(ctx, name, patchData)
}

// UncordonNode marks a node as schedulable
func (c *Client) UncordonNode(ctx context.Context, name string) error {
	patchData := []byte(`{"spec":{"unschedulable":false}}`)
	return c.PatchNode(ctx, name, patchData)
}

// DrainNode evicts all pods from a node
func (c *Client) DrainNode(ctx context.Context, name string, gracePeriodSeconds *int64) error {
	// First cordon the node
	if err := c.CordonNode(ctx, name); err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	// Get all pods on the node
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods on node: %w", err)
	}

	// Evict each pod
	for _, pod := range pods.Items {
		// Skip daemonset pods and system pods
		if pod.OwnerReferences != nil {
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					continue
				}
			}
		}

		// Skip static pods
		if pod.Annotations != nil {
			if _, ok := pod.Annotations["kubernetes.io/config.mirror"]; ok {
				continue
			}
		}

		// Create eviction object
		eviction := &metav1.DeleteOptions{
			GracePeriodSeconds: gracePeriodSeconds,
		}

		// Evict the pod
		err := c.clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, *eviction)
		if err != nil {
			// Log error but continue with other pods
			fmt.Printf("Warning: failed to evict pod %s/%s: %v\n", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}
