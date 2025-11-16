package calendarplugin

import (
	"github.com/streamspace/streamspace/api/internal/plugins"
)

// CalendarPlugin provides Google/Outlook calendar integration
type CalendarPlugin struct {
	plugins.BasePlugin
}

// NewCalendarPlugin creates a new calendar plugin instance
func NewCalendarPlugin() *CalendarPlugin {
	return &CalendarPlugin{
		BasePlugin: plugins.BasePlugin{Name: "streamspace-calendar"},
	}
}

// OnLoad initializes the plugin
func (p *CalendarPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Calendar plugin loading")
	
	// TODO: Extract calendar logic from /api/internal/handlers/scheduling.go
	// TODO: Register API endpoints for calendar operations
	// TODO: Initialize database tables (calendar_integrations, calendar_oauth_states, calendar_events)
	// TODO: Set up OAuth handlers for Google and Microsoft
	// TODO: Schedule auto-sync job based on autoSyncInterval config
	
	return nil
}

// Auto-register plugin
func init() {
	plugins.Register("streamspace-calendar", func() plugins.Plugin {
		return NewCalendarPlugin()
	})
}
