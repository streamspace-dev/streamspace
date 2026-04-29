package plugins

import "testing"

func TestNewRuntimeV2IncludesGloballyRegisteredPluginsInDiscovery(t *testing.T) {
	const pluginName = "test-runtimev2-global-plugin"

	Register(pluginName, func() PluginHandler {
		return &BasePlugin{Name: pluginName}
	})

	runtime := NewRuntimeV2(nil)
	available := runtime.ListAvailablePlugins()

	found := false
	for _, name := range available {
		if name == pluginName {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected %s to be discoverable via global registry", pluginName)
	}
}
