package multimonitorplugin

import (
	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// MultiMonitorPlugin provides multi-monitor configuration support
type MultiMonitorPlugin struct {
	plugins.BasePlugin
}

// NewMultiMonitorPlugin creates a new multi-monitor plugin instance
func NewMultiMonitorPlugin() *MultiMonitorPlugin {
	return &MultiMonitorPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-multi-monitor"},
	}
}

// OnLoad initializes the plugin
func (p *MultiMonitorPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Multi-Monitor plugin loading")
	
	// TODO: Extract monitor configuration logic from /api/internal/handlers/multimonitor.go
	// TODO: Register API endpoints for monitor management
	// TODO: Initialize database tables (monitor_configurations, monitor_displays)
	
	return nil
}

// Auto-register plugin
func init() {
	plugins.Register("streamspace-multi-monitor", func() plugins.Plugin {
		return NewMultiMonitorPlugin()
	})
}
