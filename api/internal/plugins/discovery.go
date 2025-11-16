package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

// PluginDiscovery handles automatic plugin discovery and loading
type PluginDiscovery struct {
	pluginDirs      []string
	builtinPlugins  map[string]PluginFactory
	dynamicPlugins  map[string]*plugin.Plugin
}

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func() PluginHandler

// NewPluginDiscovery creates a new plugin discovery instance
func NewPluginDiscovery(pluginDirs ...string) *PluginDiscovery {
	if len(pluginDirs) == 0 {
		// Default plugin directories
		pluginDirs = []string{
			"/plugins",                    // Container path
			"./plugins",                   // Local development
			"/usr/local/share/streamspace/plugins", // System install
		}
	}

	return &PluginDiscovery{
		pluginDirs:     pluginDirs,
		builtinPlugins: make(map[string]PluginFactory),
		dynamicPlugins: make(map[string]*plugin.Plugin),
	}
}

// RegisterBuiltin registers a built-in plugin factory
func (pd *PluginDiscovery) RegisterBuiltin(name string, factory PluginFactory) {
	pd.builtinPlugins[name] = factory
	log.Printf("[Plugin Discovery] Registered built-in plugin: %s", name)
}

// DiscoverAll discovers all available plugins (built-in and dynamic)
func (pd *PluginDiscovery) DiscoverAll() ([]string, error) {
	plugins := make([]string, 0)

	// Add built-in plugins
	for name := range pd.builtinPlugins {
		plugins = append(plugins, name)
	}

	// Discover dynamic plugins
	dynamicPlugins, err := pd.discoverDynamicPlugins()
	if err != nil {
		log.Printf("[Plugin Discovery] Warning: Failed to discover dynamic plugins: %v", err)
		// Don't fail, just log the error
	} else {
		plugins = append(plugins, dynamicPlugins...)
	}

	log.Printf("[Plugin Discovery] Discovered %d total plugins: %v", len(plugins), plugins)
	return plugins, nil
}

// LoadPlugin loads a plugin by name (built-in or dynamic)
func (pd *PluginDiscovery) LoadPlugin(name string) (PluginHandler, error) {
	// Try built-in first
	if factory, ok := pd.builtinPlugins[name]; ok {
		log.Printf("[Plugin Discovery] Loading built-in plugin: %s", name)
		return factory(), nil
	}

	// Try dynamic plugin
	handler, err := pd.loadDynamicPlugin(name)
	if err != nil {
		return nil, fmt.Errorf("plugin %s not found (checked built-in and dynamic): %w", name, err)
	}

	log.Printf("[Plugin Discovery] Loaded dynamic plugin: %s", name)
	return handler, nil
}

// discoverDynamicPlugins discovers .so plugin files
func (pd *PluginDiscovery) discoverDynamicPlugins() ([]string, error) {
	plugins := make([]string, 0)

	for _, dir := range pd.pluginDirs {
		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// Scan for .so files
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Check if it's a .so file
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".so") {
				// Extract plugin name from filename
				name := strings.TrimSuffix(info.Name(), ".so")
				plugins = append(plugins, name)
				log.Printf("[Plugin Discovery] Found dynamic plugin: %s at %s", name, path)
			}

			return nil
		})

		if err != nil {
			log.Printf("[Plugin Discovery] Error scanning directory %s: %v", dir, err)
		}
	}

	return plugins, nil
}

// loadDynamicPlugin loads a .so plugin file
func (pd *PluginDiscovery) loadDynamicPlugin(name string) (PluginHandler, error) {
	// Check if already loaded
	if p, ok := pd.dynamicPlugins[name]; ok {
		return pd.getPluginHandler(p)
	}

	// Find plugin file
	pluginPath := pd.findPluginFile(name)
	if pluginPath == "" {
		return nil, fmt.Errorf("plugin file not found: %s", name)
	}

	// Open plugin
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %w", name, err)
	}

	// Cache the plugin
	pd.dynamicPlugins[name] = p

	// Get handler from plugin
	return pd.getPluginHandler(p)
}

// getPluginHandler extracts the PluginHandler from a loaded plugin
func (pd *PluginDiscovery) getPluginHandler(p *plugin.Plugin) (PluginHandler, error) {
	// Look for NewPlugin() function
	symbol, err := p.Lookup("NewPlugin")
	if err != nil {
		return nil, fmt.Errorf("plugin missing NewPlugin function: %w", err)
	}

	// Cast to factory function
	factory, ok := symbol.(func() PluginHandler)
	if !ok {
		return nil, fmt.Errorf("NewPlugin has wrong signature, expected func() PluginHandler")
	}

	// Create plugin instance
	return factory(), nil
}

// findPluginFile searches for a plugin .so file in all plugin directories
func (pd *PluginDiscovery) findPluginFile(name string) string {
	possibleNames := []string{
		name + ".so",
		"streamspace-" + name + ".so",
		name + "_plugin.so",
	}

	for _, dir := range pd.pluginDirs {
		for _, filename := range possibleNames {
			path := filepath.Join(dir, filename)
			if _, err := os.Stat(path); err == nil {
				return path
			}

			// Also check in subdirectories
			subPath := filepath.Join(dir, name, filename)
			if _, err := os.Stat(subPath); err == nil {
				return subPath
			}
		}
	}

	return ""
}

// IsBuiltin checks if a plugin is built-in
func (pd *PluginDiscovery) IsBuiltin(name string) bool {
	_, ok := pd.builtinPlugins[name]
	return ok
}

// ListBuiltin returns all built-in plugin names
func (pd *PluginDiscovery) ListBuiltin() []string {
	names := make([]string, 0, len(pd.builtinPlugins))
	for name := range pd.builtinPlugins {
		names = append(names, name)
	}
	return names
}

// ListDynamic returns all discovered dynamic plugin names
func (pd *PluginDiscovery) ListDynamic() []string {
	names := make([]string, 0, len(pd.dynamicPlugins))
	for name := range pd.dynamicPlugins {
		names = append(names, name)
	}
	return names
}
