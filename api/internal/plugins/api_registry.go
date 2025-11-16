package plugins

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// APIRegistry manages plugin API endpoint registrations
type APIRegistry struct {
	endpoints map[string]*PluginEndpoint
	mu        sync.RWMutex
}

// PluginEndpoint represents a registered plugin API endpoint
type PluginEndpoint struct {
	PluginName  string
	Method      string
	Path        string
	Handler     gin.HandlerFunc
	Middleware  []gin.HandlerFunc
	Permissions []string
	Description string
}

// NewAPIRegistry creates a new API registry
func NewAPIRegistry() *APIRegistry {
	return &APIRegistry{
		endpoints: make(map[string]*PluginEndpoint),
	}
}

// Register registers a plugin API endpoint
func (r *APIRegistry) Register(pluginName string, endpoint *PluginEndpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", pluginName, endpoint.Method, endpoint.Path)

	// Check if already registered
	if _, exists := r.endpoints[key]; exists {
		return fmt.Errorf("endpoint %s %s already registered by plugin %s", endpoint.Method, endpoint.Path, pluginName)
	}

	endpoint.PluginName = pluginName
	r.endpoints[key] = endpoint

	log.Printf("[API Registry] Registered endpoint: %s %s (plugin: %s)", endpoint.Method, endpoint.Path, pluginName)
	return nil
}

// Unregister removes a plugin API endpoint
func (r *APIRegistry) Unregister(pluginName string, method string, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", pluginName, method, path)
	delete(r.endpoints, key)

	log.Printf("[API Registry] Unregistered endpoint: %s %s (plugin: %s)", method, path, pluginName)
}

// UnregisterAll removes all endpoints for a plugin
func (r *APIRegistry) UnregisterAll(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	toDelete := []string{}
	for key, endpoint := range r.endpoints {
		if endpoint.PluginName == pluginName {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(r.endpoints, key)
	}

	log.Printf("[API Registry] Unregistered all endpoints for plugin: %s", pluginName)
}

// GetEndpoints returns all registered endpoints
func (r *APIRegistry) GetEndpoints() []*PluginEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]*PluginEndpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// GetPluginEndpoints returns endpoints for a specific plugin
func (r *APIRegistry) GetPluginEndpoints(pluginName string) []*PluginEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]*PluginEndpoint, 0)
	for _, endpoint := range r.endpoints {
		if endpoint.PluginName == pluginName {
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}

// AttachToRouter attaches all registered endpoints to a Gin router
func (r *APIRegistry) AttachToRouter(router *gin.RouterGroup) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, endpoint := range r.endpoints {
		// Create the full handler chain
		handlers := make([]gin.HandlerFunc, 0, len(endpoint.Middleware)+1)
		handlers = append(handlers, endpoint.Middleware...)
		handlers = append(handlers, endpoint.Handler)

		// Register with router
		router.Handle(endpoint.Method, endpoint.Path, handlers...)

		log.Printf("[API Registry] Attached endpoint: %s %s", endpoint.Method, endpoint.Path)
	}
}

// PluginAPI provides API registration for plugins
type PluginAPI struct {
	registry   *APIRegistry
	pluginName string
}

// NewPluginAPI creates a new plugin API instance
func NewPluginAPI(registry *APIRegistry, pluginName string) *PluginAPI {
	return &PluginAPI{
		registry:   registry,
		pluginName: pluginName,
	}
}

// EndpointOptions contains options for registering an endpoint
type EndpointOptions struct {
	Method      string
	Path        string
	Handler     gin.HandlerFunc
	Middleware  []gin.HandlerFunc
	Permissions []string
	Description string
}

// RegisterEndpoint registers an API endpoint
func (pa *PluginAPI) RegisterEndpoint(opts EndpointOptions) error {
	// Ensure path starts with /api/plugins/{pluginName}/
	if len(opts.Path) == 0 || opts.Path[0] != '/' {
		opts.Path = "/" + opts.Path
	}

	// Prefix with plugin namespace
	fullPath := fmt.Sprintf("/api/plugins/%s%s", pa.pluginName, opts.Path)

	endpoint := &PluginEndpoint{
		Method:      opts.Method,
		Path:        fullPath,
		Handler:     opts.Handler,
		Middleware:  opts.Middleware,
		Permissions: opts.Permissions,
		Description: opts.Description,
	}

	return pa.registry.Register(pa.pluginName, endpoint)
}

// GET registers a GET endpoint
func (pa *PluginAPI) GET(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodGet,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// POST registers a POST endpoint
func (pa *PluginAPI) POST(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPost,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// PUT registers a PUT endpoint
func (pa *PluginAPI) PUT(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPut,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// PATCH registers a PATCH endpoint
func (pa *PluginAPI) PATCH(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPatch,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// DELETE registers a DELETE endpoint
func (pa *PluginAPI) DELETE(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodDelete,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// Unregister removes an endpoint
func (pa *PluginAPI) Unregister(method string, path string) {
	fullPath := fmt.Sprintf("/api/plugins/%s%s", pa.pluginName, path)
	pa.registry.Unregister(pa.pluginName, method, fullPath)
}
