package plugins

import "fmt"

// BasePlugin provides default implementations for the PluginHandler interface
// Plugins can embed this to only override the methods they need
type BasePlugin struct {
	Name string
}

// OnLoad is called when the plugin is loaded (default: no-op)
func (p *BasePlugin) OnLoad(ctx *PluginContext) error {
	return nil
}

// OnUnload is called when the plugin is unloaded (default: no-op)
func (p *BasePlugin) OnUnload(ctx *PluginContext) error {
	return nil
}

// OnEnable is called when the plugin is enabled (default: no-op)
func (p *BasePlugin) OnEnable(ctx *PluginContext) error {
	return nil
}

// OnDisable is called when the plugin is disabled (default: no-op)
func (p *BasePlugin) OnDisable(ctx *PluginContext) error {
	return nil
}

// OnSessionCreated is called when a session is created (default: no-op)
func (p *BasePlugin) OnSessionCreated(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnSessionStarted is called when a session is started (default: no-op)
func (p *BasePlugin) OnSessionStarted(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnSessionStopped is called when a session is stopped (default: no-op)
func (p *BasePlugin) OnSessionStopped(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnSessionHibernated is called when a session is hibernated (default: no-op)
func (p *BasePlugin) OnSessionHibernated(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnSessionWoken is called when a session wakes from hibernation (default: no-op)
func (p *BasePlugin) OnSessionWoken(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnSessionDeleted is called when a session is deleted (default: no-op)
func (p *BasePlugin) OnSessionDeleted(ctx *PluginContext, session interface{}) error {
	return nil
}

// OnUserCreated is called when a user is created (default: no-op)
func (p *BasePlugin) OnUserCreated(ctx *PluginContext, user interface{}) error {
	return nil
}

// OnUserUpdated is called when a user is updated (default: no-op)
func (p *BasePlugin) OnUserUpdated(ctx *PluginContext, user interface{}) error {
	return nil
}

// OnUserDeleted is called when a user is deleted (default: no-op)
func (p *BasePlugin) OnUserDeleted(ctx *PluginContext, user interface{}) error {
	return nil
}

// OnUserLogin is called when a user logs in (default: no-op)
func (p *BasePlugin) OnUserLogin(ctx *PluginContext, user interface{}) error {
	return nil
}

// OnUserLogout is called when a user logs out (default: no-op)
func (p *BasePlugin) OnUserLogout(ctx *PluginContext, user interface{}) error {
	return nil
}

// Built-in plugin registry
var builtinPlugins = make(map[string]PluginHandler)

// RegisterBuiltinPlugin registers a built-in plugin
func RegisterBuiltinPlugin(name string, plugin PluginHandler) {
	builtinPlugins[name] = plugin
	fmt.Printf("[Plugin Registry] Registered built-in plugin: %s\n", name)
}

// GetBuiltinPlugin retrieves a built-in plugin
func GetBuiltinPlugin(name string) PluginHandler {
	return builtinPlugins[name]
}

// ListBuiltinPlugins returns all registered built-in plugins
func ListBuiltinPlugins() []string {
	names := make([]string, 0, len(builtinPlugins))
	for name := range builtinPlugins {
		names = append(names, name)
	}
	return names
}
