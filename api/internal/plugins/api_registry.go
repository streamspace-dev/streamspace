// Package plugins provides the plugin system for StreamSpace API.
//
// The api_registry component enables plugins to register custom HTTP API endpoints
// that are dynamically mounted into the main API router. This allows plugins to
// extend the API surface without modifying core code.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                    Main API Router (Gin)                    │
//	│  /api/sessions, /api/users, /api/templates, etc.           │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │ AttachToRouter()
//	                           ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│                      APIRegistry                            │
//	│  - Stores plugin endpoint registrations                     │
//	│  - Enforces namespace isolation (/api/plugins/{name}/...)  │
//	│  - Thread-safe registration/unregistration                  │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │ Manages
//	                           ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│                   PluginEndpoint Records                    │
//	│  plugin-slack:    POST /api/plugins/slack/send              │
//	│  plugin-billing:  GET  /api/plugins/billing/invoices        │
//	│  plugin-sentry:   POST /api/plugins/sentry/report           │
//	└─────────────────────────────────────────────────────────────┘
//
// Endpoint Lifecycle:
//  1. Plugin calls api.RegisterEndpoint() during OnLoad()
//  2. APIRegistry stores endpoint with namespace prefix
//  3. AttachToRouter() mounts all endpoints to main router
//  4. Requests to /api/plugins/{name}/... route to plugin handlers
//  5. Plugin calls api.Unregister() or runtime unloads plugin
//  6. Endpoints are removed from registry (router cleanup on restart)
//
// Namespace Isolation:
//
// All plugin endpoints are automatically prefixed with /api/plugins/{pluginName}/
// to prevent conflicts between plugins and with core API routes.
//
//	// Plugin code
//	api.RegisterEndpoint(EndpointOptions{
//	    Method:  "POST",
//	    Path:    "/send",  // Plugin provides relative path
//	    Handler: sendHandler,
//	})
//
//	// Results in: POST /api/plugins/slack/send
//
// Thread Safety:
//
// The registry uses sync.RWMutex for thread-safe concurrent access:
//   - Register/Unregister: Exclusive lock (write)
//   - GetEndpoints/AttachToRouter: Shared lock (read)
//   - Safe for plugins to register during parallel OnLoad() calls
//
// Middleware Support:
//
// Endpoints can specify middleware chains (authentication, rate limiting, etc.):
//
//	api.RegisterEndpoint(EndpointOptions{
//	    Method:     "POST",
//	    Path:       "/admin/settings",
//	    Handler:    settingsHandler,
//	    Middleware: []gin.HandlerFunc{authMiddleware, adminOnlyMiddleware},
//	})
//
// Permission Model:
//
// Endpoints can declare required permissions for documentation/UI purposes.
// Actual enforcement happens in middleware, not the registry:
//
//	api.RegisterEndpoint(EndpointOptions{
//	    Permissions: []string{"plugin.slack.send", "sessions.read"},
//	})
//
// Cleanup on Unload:
//
// When a plugin is unloaded:
//   - UnregisterAll(pluginName) removes all endpoints for that plugin
//   - Prevents orphaned routes from unloaded plugins
//   - Router rebuild required to apply changes (done on restart)
//
// Performance:
//   - Registration: O(1) map insertion
//   - Lookup: O(1) map access
//   - AttachToRouter: O(n) iteration over all endpoints
//   - Memory: ~200 bytes per endpoint registration
//
// Future Enhancements:
//   - Dynamic route reloading without restart
//   - Endpoint versioning (/api/plugins/slack/v1/send)
//   - Rate limiting per plugin
//   - Request/response logging and metrics
//   - OpenAPI/Swagger spec generation from registered endpoints
package plugins

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// APIRegistry manages plugin API endpoint registrations.
//
// The registry provides centralized management of all plugin-contributed API
// endpoints, ensuring namespace isolation and thread-safe registration.
//
// Key responsibilities:
//   - Store endpoint registrations with plugin attribution
//   - Enforce /api/plugins/{name}/ namespace prefix
//   - Prevent endpoint conflicts between plugins
//   - Provide thread-safe concurrent access
//   - Support bulk cleanup on plugin unload
//
// Registry Structure:
//
//	endpoints: map[string]*PluginEndpoint
//	  Key format: "{pluginName}:{method}:{path}"
//	  Example: "slack:POST:/api/plugins/slack/send"
//	  Value: Full endpoint metadata
//
// Concurrency Model:
//
//	Register/Unregister: Write lock (exclusive)
//	GetEndpoints/Attach:  Read lock (shared)
//	Multiple plugins can query concurrently
//	Registration is serialized to prevent conflicts
type APIRegistry struct {
	// endpoints stores all registered plugin API endpoints.
	// Map key format: "{pluginName}:{method}:{path}"
	// Thread-safe access via mu.
	endpoints map[string]*PluginEndpoint

	// mu protects concurrent access to the endpoints map.
	// Read operations (GetEndpoints, AttachToRouter) use RLock.
	// Write operations (Register, Unregister) use Lock.
	mu sync.RWMutex
}

// PluginEndpoint represents a registered plugin API endpoint.
//
// Each endpoint contains all metadata needed to mount it to the Gin router:
//   - HTTP method and path
//   - Handler function
//   - Middleware chain
//   - Permission requirements
//   - Documentation description
//
// Endpoints are namespaced under /api/plugins/{pluginName}/ to ensure isolation.
//
// Example:
//
//	&PluginEndpoint{
//	    PluginName:  "slack",
//	    Method:      "POST",
//	    Path:        "/api/plugins/slack/send",  // Full path with namespace
//	    Handler:     sendMessageHandler,
//	    Middleware:  []gin.HandlerFunc{authMiddleware},
//	    Permissions: []string{"plugin.slack.send"},
//	    Description: "Send a Slack message to a channel",
//	}
type PluginEndpoint struct {
	// PluginName identifies which plugin registered this endpoint.
	// Used for cleanup when plugin is unloaded.
	PluginName string

	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE, etc.)
	Method string

	// Path is the full URL path including namespace prefix.
	// Format: /api/plugins/{pluginName}/{relative-path}
	// Example: /api/plugins/slack/send
	Path string

	// Handler is the Gin handler function that processes requests.
	// Receives gin.Context with request data, writes response.
	Handler gin.HandlerFunc

	// Middleware is an optional chain of middleware functions.
	// Executed before the handler in array order.
	// Common uses: authentication, rate limiting, logging.
	Middleware []gin.HandlerFunc

	// Permissions lists required permissions for this endpoint.
	// Used for documentation and UI permission checks.
	// Actual enforcement must happen in middleware.
	Permissions []string

	// Description provides human-readable documentation.
	// Used in API documentation and admin UI.
	Description string
}

// NewAPIRegistry creates a new API registry.
//
// Returns an initialized registry ready to accept plugin endpoint registrations.
//
// Usage:
//
//	registry := NewAPIRegistry()
//	runtime.apiRegistry = registry
func NewAPIRegistry() *APIRegistry {
	return &APIRegistry{
		endpoints: make(map[string]*PluginEndpoint),
	}
}

// Register registers a plugin API endpoint in the registry.
//
// This method stores the endpoint metadata and associates it with the plugin.
// The endpoint will be mounted to the router when AttachToRouter() is called.
//
// Parameters:
//   - pluginName: Name of the plugin registering the endpoint
//   - endpoint: Endpoint metadata (method, path, handler, etc.)
//
// Returns:
//   - error: Conflict error if endpoint already registered, nil on success
//
// Thread Safety:
//
//	This method acquires an exclusive write lock. It's safe to call
//	concurrently from multiple plugins during startup.
//
// Conflict Detection:
//
//	Endpoints are uniquely identified by (pluginName, method, path).
//	Attempting to register a duplicate returns an error.
//
// Example:
//
//	err := registry.Register("slack", &PluginEndpoint{
//	    Method:  "POST",
//	    Path:    "/api/plugins/slack/send",
//	    Handler: sendHandler,
//	})
func (r *APIRegistry) Register(pluginName string, endpoint *PluginEndpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", pluginName, endpoint.Method, endpoint.Path)

	// Check if already registered (prevents duplicate routes)
	if _, exists := r.endpoints[key]; exists {
		return fmt.Errorf("endpoint %s %s already registered by plugin %s", endpoint.Method, endpoint.Path, pluginName)
	}

	endpoint.PluginName = pluginName
	r.endpoints[key] = endpoint

	log.Printf("[API Registry] Registered endpoint: %s %s (plugin: %s)", endpoint.Method, endpoint.Path, pluginName)
	return nil
}

// Unregister removes a specific plugin API endpoint from the registry.
//
// This method removes a single endpoint by its method and path. The endpoint
// will no longer be available after the next router rebuild (typically on restart).
//
// Parameters:
//   - pluginName: Name of the plugin that owns the endpoint
//   - method: HTTP method (GET, POST, etc.)
//   - path: Full URL path including namespace prefix
//
// Thread Safety:
//
//	Acquires exclusive write lock. Safe for concurrent calls.
//
// Note:
//
//	This does not immediately remove the route from the Gin router.
//	Router rebuilding happens on application restart.
//
// Example:
//
//	registry.Unregister("slack", "POST", "/api/plugins/slack/send")
func (r *APIRegistry) Unregister(pluginName string, method string, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", pluginName, method, path)
	delete(r.endpoints, key)

	log.Printf("[API Registry] Unregistered endpoint: %s %s (plugin: %s)", method, path, pluginName)
}

// UnregisterAll removes all endpoints for a plugin.
//
// This method is called during plugin unload to clean up all endpoints
// registered by that plugin. Prevents orphaned routes after unload.
//
// Parameters:
//   - pluginName: Name of the plugin to clean up
//
// Thread Safety:
//
//	Acquires exclusive write lock. Safe for concurrent calls.
//
// Implementation:
//
//	Uses two-pass approach to avoid modifying map during iteration:
//	  1. Collect keys to delete
//	  2. Delete collected keys
//
// Example:
//
//	// During plugin unload
//	registry.UnregisterAll("slack")
//	// All endpoints like /api/plugins/slack/* are removed
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

// GetEndpoints returns all registered endpoints across all plugins.
//
// Returns a snapshot of all endpoints currently registered. The returned
// slice is safe to iterate without holding locks.
//
// Returns:
//   - []*PluginEndpoint: Slice of all registered endpoints
//
// Thread Safety:
//
//	Acquires shared read lock. Multiple callers can execute concurrently.
//	Returned slice is a copy, safe to modify.
//
// Use Cases:
//   - Generate API documentation
//   - List all plugin endpoints in admin UI
//   - Export endpoint catalog for testing
//
// Example:
//
//	endpoints := registry.GetEndpoints()
//	for _, ep := range endpoints {
//	    fmt.Printf("%s %s - %s\n", ep.Method, ep.Path, ep.Description)
//	}
func (r *APIRegistry) GetEndpoints() []*PluginEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]*PluginEndpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// GetPluginEndpoints returns endpoints for a specific plugin.
//
// Filters the endpoint registry to return only endpoints owned by the
// specified plugin. Useful for plugin-specific introspection.
//
// Parameters:
//   - pluginName: Name of the plugin to query
//
// Returns:
//   - []*PluginEndpoint: Endpoints registered by that plugin
//
// Thread Safety:
//
//	Acquires shared read lock. Safe for concurrent calls.
//
// Performance:
//
//	O(n) iteration over all endpoints with filtering.
//	For large registries, consider adding an index by plugin.
//
// Example:
//
//	slackEndpoints := registry.GetPluginEndpoints("slack")
//	fmt.Printf("Slack plugin has %d endpoints\n", len(slackEndpoints))
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

// AttachToRouter attaches all registered endpoints to a Gin router.
//
// This method mounts all plugin endpoints to the main API router. It should
// be called once during API server initialization, after all plugins have
// registered their endpoints.
//
// Parameters:
//   - router: Gin router group to mount endpoints on
//
// Behavior:
//
//	For each registered endpoint:
//	  1. Build middleware chain (endpoint.Middleware + endpoint.Handler)
//	  2. Register with router: router.Handle(method, path, handlers...)
//	  3. Log the attachment
//
// Thread Safety:
//
//	Acquires shared read lock. Safe to call while plugins are querying.
//	Should not be called concurrently with Register() during startup.
//
// Middleware Chain:
//
//	The handler chain is built as: [middleware1, middleware2, ..., handler]
//	Middleware executes in array order before the handler.
//
// Example:
//
//	router := gin.Default()
//	apiGroup := router.Group("/api")
//	registry.AttachToRouter(apiGroup)
//	// All plugin endpoints now available under /api/plugins/...
//
// Note:
//
//	This does not support dynamic route reloading. Endpoint changes
//	require application restart to take effect.
func (r *APIRegistry) AttachToRouter(router *gin.RouterGroup) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, endpoint := range r.endpoints {
		// Create the full handler chain: [middleware..., handler]
		handlers := make([]gin.HandlerFunc, 0, len(endpoint.Middleware)+1)
		handlers = append(handlers, endpoint.Middleware...)
		handlers = append(handlers, endpoint.Handler)

		// Register with router
		router.Handle(endpoint.Method, endpoint.Path, handlers...)

		log.Printf("[API Registry] Attached endpoint: %s %s", endpoint.Method, endpoint.Path)
	}
}

// PluginAPI provides API registration interface for plugins.
//
// This is the plugin-facing API that abstracts the underlying APIRegistry.
// Each plugin receives a PluginAPI instance pre-configured with its name,
// ensuring automatic namespace isolation.
//
// Design Pattern:
//
//	Instead of giving plugins direct access to the global registry,
//	we provide a scoped interface that automatically applies the
//	plugin's namespace prefix. This prevents plugins from interfering
//	with each other's routes.
//
// Example Usage in Plugin:
//
//	func (p *SlackPlugin) OnLoad(ctx *PluginContext) error {
//	    // ctx.API is pre-configured for this plugin
//	    return ctx.API.POST("/send", p.handleSend, "plugin.slack.send")
//	}
//	// Results in: POST /api/plugins/slack/send
type PluginAPI struct {
	// registry is the global API registry.
	// All registrations go through this registry.
	registry *APIRegistry

	// pluginName is the name of the plugin this API instance serves.
	// Used to automatically namespace all endpoints.
	pluginName string
}

// NewPluginAPI creates a new plugin API instance.
//
// Creates a scoped API interface for a specific plugin, with automatic
// namespace isolation. This is called by the plugin runtime during
// initialization, not by plugins directly.
//
// Parameters:
//   - registry: The global API registry
//   - pluginName: Name of the plugin (used for namespacing)
//
// Returns:
//   - *PluginAPI: Scoped API instance for the plugin
//
// Example:
//
//	// In plugin runtime
//	pluginCtx.API = NewPluginAPI(runtime.apiRegistry, "slack")
func NewPluginAPI(registry *APIRegistry, pluginName string) *PluginAPI {
	return &PluginAPI{
		registry:   registry,
		pluginName: pluginName,
	}
}

// EndpointOptions contains options for registering an endpoint.
//
// This struct provides a flexible API for endpoint registration with
// optional middleware, permissions, and documentation.
//
// Fields:
//   - Method: HTTP method (GET, POST, PUT, PATCH, DELETE)
//   - Path: Relative path (will be prefixed with /api/plugins/{name})
//   - Handler: Gin handler function
//   - Middleware: Optional middleware chain
//   - Permissions: Permission strings for documentation
//   - Description: Human-readable endpoint description
type EndpointOptions struct {
	Method      string
	Path        string
	Handler     gin.HandlerFunc
	Middleware  []gin.HandlerFunc
	Permissions []string
	Description string
}

// RegisterEndpoint registers an API endpoint with full options.
//
// This is the low-level registration method that supports all endpoint
// configuration options. Most plugins should use the convenience methods
// (GET, POST, etc.) instead.
//
// Parameters:
//   - opts: Complete endpoint configuration
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Automatic Namespace Prefix:
//
//	The path is automatically prefixed with /api/plugins/{pluginName}/.
//	Plugin provides: "/send"
//	Results in: "/api/plugins/slack/send"
//
// Example:
//
//	err := api.RegisterEndpoint(EndpointOptions{
//	    Method:      "POST",
//	    Path:        "/send",
//	    Handler:     sendHandler,
//	    Middleware:  []gin.HandlerFunc{authMiddleware},
//	    Permissions: []string{"plugin.slack.send"},
//	    Description: "Send a Slack message",
//	})
func (pa *PluginAPI) RegisterEndpoint(args ...interface{}) error {
	var opts EndpointOptions
	switch len(args) {
	case 1:
		var ok bool
		opts, ok = args[0].(EndpointOptions)
		if !ok {
			return fmt.Errorf("RegisterEndpoint requires EndpointOptions or method/path/handler")
		}
	case 3:
		method, ok := args[0].(string)
		if !ok {
			return fmt.Errorf("RegisterEndpoint method must be a string")
		}
		path, ok := args[1].(string)
		if !ok {
			return fmt.Errorf("RegisterEndpoint path must be a string")
		}
		handler, ok := args[2].(gin.HandlerFunc)
		if !ok {
			return fmt.Errorf("RegisterEndpoint handler must be a gin.HandlerFunc")
		}
		opts = EndpointOptions{
			Method:  method,
			Path:    path,
			Handler: handler,
		}
	default:
		return fmt.Errorf("RegisterEndpoint requires EndpointOptions or method/path/handler")
	}

	// Ensure path starts with / (normalize input)
	if len(opts.Path) == 0 || opts.Path[0] != '/' {
		opts.Path = "/" + opts.Path
	}

	// Apply plugin namespace prefix automatically
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

// GET registers a GET endpoint.
//
// Convenience method for registering GET endpoints with minimal configuration.
// Automatically applies plugin namespace prefix.
//
// Parameters:
//   - path: Relative path (e.g., "/messages")
//   - handler: Gin handler function
//   - permissions: Optional permission strings (variadic)
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Example:
//
//	err := api.GET("/messages", listMessagesHandler, "plugin.slack.read")
//	// Results in: GET /api/plugins/slack/messages
func (pa *PluginAPI) GET(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodGet,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// POST registers a POST endpoint.
//
// Convenience method for registering POST endpoints with minimal configuration.
//
// Parameters:
//   - path: Relative path (e.g., "/send")
//   - handler: Gin handler function
//   - permissions: Optional permission strings (variadic)
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Example:
//
//	err := api.POST("/send", sendMessageHandler, "plugin.slack.send")
//	// Results in: POST /api/plugins/slack/send
func (pa *PluginAPI) POST(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPost,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// PUT registers a PUT endpoint.
//
// Convenience method for registering PUT endpoints for resource updates.
//
// Parameters:
//   - path: Relative path (e.g., "/config")
//   - handler: Gin handler function
//   - permissions: Optional permission strings (variadic)
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Example:
//
//	err := api.PUT("/config", updateConfigHandler, "plugin.slack.config.write")
//	// Results in: PUT /api/plugins/slack/config
func (pa *PluginAPI) PUT(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPut,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// PATCH registers a PATCH endpoint.
//
// Convenience method for registering PATCH endpoints for partial updates.
//
// Parameters:
//   - path: Relative path (e.g., "/settings")
//   - handler: Gin handler function
//   - permissions: Optional permission strings (variadic)
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Example:
//
//	err := api.PATCH("/settings", patchSettingsHandler, "plugin.slack.settings.write")
//	// Results in: PATCH /api/plugins/slack/settings
func (pa *PluginAPI) PATCH(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodPatch,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// DELETE registers a DELETE endpoint.
//
// Convenience method for registering DELETE endpoints for resource deletion.
//
// Parameters:
//   - path: Relative path (e.g., "/webhooks/:id")
//   - handler: Gin handler function
//   - permissions: Optional permission strings (variadic)
//
// Returns:
//   - error: Registration error if endpoint conflicts, nil on success
//
// Example:
//
//	err := api.DELETE("/webhooks/:id", deleteWebhookHandler, "plugin.slack.webhooks.delete")
//	// Results in: DELETE /api/plugins/slack/webhooks/:id
func (pa *PluginAPI) DELETE(path string, handler gin.HandlerFunc, permissions ...string) error {
	return pa.RegisterEndpoint(EndpointOptions{
		Method:      http.MethodDelete,
		Path:        path,
		Handler:     handler,
		Permissions: permissions,
	})
}

// Unregister removes an endpoint.
//
// Removes a previously registered endpoint by method and path. The path
// should be the relative path used during registration, not the full path.
//
// Parameters:
//   - method: HTTP method (GET, POST, etc.)
//   - path: Relative path (e.g., "/send", not "/api/plugins/slack/send")
//
// Example:
//
//	// Register
//	api.POST("/send", handler)
//
//	// Later, unregister
//	api.Unregister("POST", "/send")
func (pa *PluginAPI) Unregister(method string, path string) {
	fullPath := fmt.Sprintf("/api/plugins/%s%s", pa.pluginName, path)
	pa.registry.Unregister(pa.pluginName, method, fullPath)
}
