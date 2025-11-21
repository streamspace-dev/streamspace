// Package plugins - registry.go
//
// This file implements the global plugin registry for automatic plugin discovery.
//
// The global registry provides a centralized location for plugins to register
// themselves at initialization time, enabling automatic plugin discovery without
// explicit configuration or hardcoded plugin lists.
//
// # Auto-Registration Pattern
//
// Plugins register themselves using Go's init() function pattern:
//
//	// In plugin file: plugins/my-plugin/main.go
//	package main
//
//	import "github.com/streamspace-dev/streamspace/api/internal/plugins"
//
//	func init() {
//	    plugins.Register("my-plugin", func() plugins.PluginHandler {
//	        return &MyPlugin{}
//	    })
//	}
//
// This registration happens automatically when the plugin package is imported,
// without requiring explicit registration calls in application code.
//
// # Benefits of Auto-Registration
//
//  1. **No hardcoded plugin lists**: Add new plugin = just import it
//  2. **Compile-time discovery**: Plugins discovered at build time
//  3. **Type safety**: Factory functions enforce PluginHandler interface
//  4. **Clean initialization**: No manual "register all plugins" code
//
// # How It Works
//
// The registration flow:
//
//	┌─────────────────────────────────────────────────────────┐
//	│  1. Go Program Startup                                  │
//	│     - All imported packages' init() functions run       │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  2. Plugin init() Functions Execute                     │
//	│     - Each plugin calls plugins.Register()              │
//	│     - Factory functions stored in globalRegistry        │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  3. Runtime Startup                                     │
//	│     - Runtime queries globalRegistry.GetAll()           │
//	│     - Calls factory functions to create plugin instances│
//	│     - Plugins loaded into runtime                       │
//	└─────────────────────────────────────────────────────────┘
//
// # Factory Function Pattern
//
// Plugins are registered using factory functions, not instances:
//
//	type PluginFactory func() PluginHandler
//
// Why factory functions?
//   - Allows runtime to create fresh instances (stateless)
//   - Supports multiple instances if needed
//   - Enables testing with mock implementations
//   - Defer initialization until runtime starts
//
// Example factory:
//
//	func MyPluginFactory() plugins.PluginHandler {
//	    return &MyPlugin{
//	        config: make(map[string]interface{}),
//	        state:  "initialized",
//	    }
//	}
//
// # Global vs. Local Registries
//
// **Global Registry** (this file):
//   - Package-level singleton
//   - Populated at program startup (init functions)
//   - Used for built-in plugins
//   - Thread-safe for concurrent access
//
// **Discovery Registry** (discovery.go):
//   - Instance-level registry
//   - Combines global registry + catalog plugins
//   - Handles external plugins from database
//   - Used by runtime for plugin loading
//
// # Thread Safety
//
// The global registry is thread-safe:
//   - RWMutex protects the plugins map
//   - Multiple goroutines can call Register() concurrently
//   - Readers (Get, GetAll) don't block each other
//   - Safe to access during and after initialization
//
// # Duplicate Registration
//
// If a plugin is registered twice:
//   - Warning is logged to console
//   - Second registration overwrites the first
//   - This allows hot-reload scenarios (reload = re-register)
//
// # Known Limitations
//
//  1. **No unregister**: Once registered, plugins can't be removed
//  2. **No versioning**: Can't register multiple versions of same plugin
//  3. **Build-time only**: Can't dynamically register plugins at runtime
//  4. **No dependencies**: Can't express plugin dependencies
//
// Future enhancements:
//   - Support for plugin versioning (multiple versions co-existing)
//   - Dependency graph resolution
//   - Runtime dynamic registration (hot plugin upload)
//   - Unregister for cleanup/testing
package plugins

import (
	"log"
	"sync"
)

// Global plugin registry for automatic registration.
//
// This singleton is initialized at package import time and populated
// by plugin init() functions. It provides the foundation for automatic
// plugin discovery without explicit configuration.
//
// Access pattern:
//   - Plugins call Register() to add themselves
//   - Runtime calls GetGlobalRegistry() to discover all plugins
//   - Discovery applies global registry to runtime
var (
	globalRegistry     = &GlobalPluginRegistry{plugins: make(map[string]PluginFactory)}
	globalRegistryOnce sync.Once
)

// GlobalPluginRegistry manages global plugin registration and discovery.
//
// This registry maintains a map of plugin names to factory functions,
// enabling automatic plugin discovery at runtime startup. Plugins register
// themselves using Go's init() pattern for zero-configuration discovery.
//
// Thread safety:
//   - All methods are thread-safe using RWMutex
//   - Safe for concurrent registration and access
//   - Multiple readers don't block each other
//
// Typical usage:
//
//	// Plugin registration (in plugin's init)
//	func init() {
//	    plugins.Register("my-plugin", NewMyPlugin)
//	}
//
//	// Runtime discovery
//	registry := plugins.GetGlobalRegistry()
//	allPlugins := registry.GetAll()
//	for name, factory := range allPlugins {
//	    handler := factory()
//	    // Load handler into runtime
//	}
type GlobalPluginRegistry struct {
	plugins map[string]PluginFactory
	mu      sync.RWMutex
}

// Register registers a plugin globally (called from plugin init())
func Register(name string, factory PluginFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.plugins[name]; exists {
		log.Printf("[Plugin Registry] Warning: Plugin %s already registered, overwriting", name)
	}

	globalRegistry.plugins[name] = factory
	log.Printf("[Plugin Registry] Auto-registered plugin: %s", name)
}

// GetGlobalRegistry returns the global plugin registry
func GetGlobalRegistry() *GlobalPluginRegistry {
	return globalRegistry
}

// GetAll returns all registered plugins
func (r *GlobalPluginRegistry) GetAll() map[string]PluginFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent modification
	plugins := make(map[string]PluginFactory, len(r.plugins))
	for name, factory := range r.plugins {
		plugins[name] = factory
	}

	return plugins
}

// Get retrieves a specific plugin factory
func (r *GlobalPluginRegistry) Get(name string) (PluginFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.plugins[name]
	return factory, exists
}

// List returns all registered plugin names
func (r *GlobalPluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}

	return names
}

// ApplyToDiscovery applies all globally registered plugins to a discovery instance
func (r *GlobalPluginRegistry) ApplyToDiscovery(discovery *PluginDiscovery) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, factory := range r.plugins {
		discovery.RegisterBuiltin(name, factory)
	}

	log.Printf("[Plugin Registry] Applied %d globally registered plugins to discovery", len(r.plugins))
}
