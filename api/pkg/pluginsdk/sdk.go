package plugins

import internalplugins "github.com/streamspace-dev/streamspace/api/internal/plugins"

type (
	APIRegistry          = internalplugins.APIRegistry
	BasePlugin           = internalplugins.BasePlugin
	EventBus             = internalplugins.EventBus
	GlobalPluginRegistry = internalplugins.GlobalPluginRegistry
	LoadedPlugin         = internalplugins.LoadedPlugin
	Plugin               = internalplugins.PluginHandler
	PluginAPI            = internalplugins.PluginAPI
	PluginContext        = internalplugins.PluginContext
	PluginDatabase       = internalplugins.PluginDatabase
	PluginEvents         = internalplugins.PluginEvents
	PluginFactory        = internalplugins.PluginFactory
	PluginHandler        = internalplugins.PluginHandler
	PluginLogger         = internalplugins.PluginLogger
	PluginScheduler      = internalplugins.PluginScheduler
	PluginStorage        = internalplugins.PluginStorage
	PluginUI             = internalplugins.PluginUI
	RuntimeV2            = internalplugins.RuntimeV2
	UIAdminPage          = internalplugins.UIAdminPage
	UIMenuItem           = internalplugins.UIMenuItem
	UIRegistry           = internalplugins.UIRegistry
	UIWidget             = internalplugins.UIWidget
)

func GetGlobalRegistry() *GlobalPluginRegistry {
	return internalplugins.GetGlobalRegistry()
}

// Register accepts either a factory function or a concrete plugin instance.
func Register(name string, plugin interface{}) {
	switch p := plugin.(type) {
	case PluginFactory:
		internalplugins.Register(name, p)
	case func() PluginHandler:
		internalplugins.Register(name, internalplugins.PluginFactory(p))
	case PluginHandler:
		internalplugins.Register(name, func() internalplugins.PluginHandler {
			return p
		})
	default:
		panic("plugins.Register requires a plugin factory or PluginHandler")
	}
}
