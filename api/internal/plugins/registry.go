package plugins

import (
	"log"
	"sync"
)

// Global plugin registry for automatic registration
var (
	globalRegistry     = &GlobalPluginRegistry{plugins: make(map[string]PluginFactory)}
	globalRegistryOnce sync.Once
)

// GlobalPluginRegistry manages global plugin registration
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
