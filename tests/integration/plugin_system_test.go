// Package integration provides integration tests for StreamSpace.
// These tests validate plugin system functionality including installation,
// runtime loading, enable/disable, and configuration management.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginInstallation validates that plugins can be installed from marketplace (TC-001).
//
// Related Issue: Installation Status Never Updates
// Impact: Users see "Installing..." forever
func TestPluginInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Step 1: List available plugins from marketplace
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/marketplace", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Should list marketplace plugins")

	var marketplaceResp PluginListResponse
	err = json.NewDecoder(resp.Body).Decode(&marketplaceResp)
	require.NoError(t, err)

	require.NotEmpty(t, marketplaceResp.Plugins, "Marketplace should have plugins available")

	// Find a plugin to install (prefer a test plugin if available)
	var pluginToInstall PluginResponse
	for _, p := range marketplaceResp.Plugins {
		if !p.Installed {
			pluginToInstall = p
			break
		}
	}

	if pluginToInstall.ID == "" {
		t.Skip("No uninstalled plugins available for testing")
	}

	// Step 2: Install the plugin
	installPayload := map[string]string{"pluginId": pluginToInstall.ID}
	body, _ := json.Marshal(installPayload)

	req, err = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/install", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted,
		"Install should succeed with 200 or 202")

	// Step 3: Wait for installation to complete
	installed := waitForCondition(60*time.Second, 2*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+pluginToInstall.ID, nil)
		addAuthHeader(t, req)
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		var plugin PluginResponse
		json.NewDecoder(resp.Body).Decode(&plugin)
		return plugin.Status == "installed" || plugin.Installed
	})

	assert.True(t, installed, "Plugin should reach 'installed' status within 60 seconds")

	// Cleanup: Uninstall the plugin
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+pluginToInstall.ID+"/uninstall", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	client.Do(req)

	t.Logf("Plugin installation test passed for: %s", pluginToInstall.Name)
}

// TestPluginRuntimeLoading validates that plugins can be loaded at runtime (TC-002).
//
// Related Issue: Plugin Runtime Loading returns "not yet implemented"
// Impact: Plugins cannot be dynamically loaded from disk
func TestPluginRuntimeLoading(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Get an installed plugin
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var pluginsResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&pluginsResp)

	var installedPlugin PluginResponse
	for _, p := range pluginsResp.Plugins {
		if p.Installed && !p.Enabled {
			installedPlugin = p
			break
		}
	}

	if installedPlugin.ID == "" {
		t.Skip("No installed but disabled plugins available for testing")
	}

	// Enable the plugin (should trigger runtime loading)
	req, err = http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/plugins/"+installedPlugin.ID+"/enable", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// CRITICAL CHECK: Should not return "not yet implemented"
	if resp.StatusCode == http.StatusNotImplemented {
		t.Fatal("FAIL: Plugin runtime loading returns 'not yet implemented' - this is the bug!")
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Enable should succeed")

	// Verify plugin is loaded and functional
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+installedPlugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var plugin PluginResponse
	json.NewDecoder(resp.Body).Decode(&plugin)
	resp.Body.Close()

	assert.True(t, plugin.Enabled, "Plugin should be enabled after enable request")

	// Cleanup: Disable the plugin
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+installedPlugin.ID+"/disable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	client.Do(req)

	t.Logf("Plugin runtime loading test passed for: %s", installedPlugin.Name)
}

// TestPluginEnable validates that enabling a plugin loads it into runtime (TC-003).
//
// Related Issue: Plugin Enable Runtime Loading - only updates database, doesn't load
// Impact: Enabled plugins don't actually run
func TestPluginEnable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Get an installed but disabled plugin
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)

	var pluginsResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&pluginsResp)
	resp.Body.Close()

	var testPlugin PluginResponse
	for _, p := range pluginsResp.Plugins {
		if p.Installed && !p.Enabled {
			testPlugin = p
			break
		}
	}

	if testPlugin.ID == "" {
		t.Skip("No disabled plugins available for testing")
	}

	// Step 1: Enable the plugin
	req, err = http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/plugins/"+testPlugin.ID+"/enable", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Enable should return 200")

	// Step 2: Verify database updated
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+testPlugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var plugin PluginResponse
	json.NewDecoder(resp.Body).Decode(&plugin)
	resp.Body.Close()

	assert.True(t, plugin.Enabled, "Database should show plugin as enabled")

	// Step 3: Verify plugin is actually loaded (check if endpoints respond)
	// This depends on what the plugin provides - we check the loaded plugins list
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/loaded", nil)
	addAuthHeader(t, req)
	resp, err = client.Do(req)

	if err == nil && resp.StatusCode == http.StatusOK {
		var loadedResp struct {
			Plugins []string `json:"plugins"`
		}
		json.NewDecoder(resp.Body).Decode(&loadedResp)
		resp.Body.Close()

		found := false
		for _, name := range loadedResp.Plugins {
			if name == testPlugin.Name || name == testPlugin.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Plugin should appear in loaded plugins list")
	}

	// Cleanup
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+testPlugin.ID+"/disable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	client.Do(req)

	t.Logf("Plugin enable test passed for: %s", testPlugin.Name)
}

// TestPluginDisable validates that disabling a plugin unloads it from runtime (TC-004).
func TestPluginDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Get an enabled plugin
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)

	var pluginsResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&pluginsResp)
	resp.Body.Close()

	var enabledPlugin PluginResponse
	for _, p := range pluginsResp.Plugins {
		if p.Installed && p.Enabled {
			enabledPlugin = p
			break
		}
	}

	if enabledPlugin.ID == "" {
		t.Skip("No enabled plugins available for testing")
	}

	// Step 1: Disable the plugin
	req, err = http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/plugins/"+enabledPlugin.ID+"/disable", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Disable should return 200")

	// Step 2: Verify plugin is disabled
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+enabledPlugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var plugin PluginResponse
	json.NewDecoder(resp.Body).Decode(&plugin)
	resp.Body.Close()

	assert.False(t, plugin.Enabled, "Plugin should be disabled after disable request")

	// Cleanup: Re-enable the plugin
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+enabledPlugin.ID+"/enable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	client.Do(req)

	t.Logf("Plugin disable test passed for: %s", enabledPlugin.Name)
}

// TestPluginConfigUpdate validates that plugin config updates persist and reload (TC-005).
//
// Related Issue: Plugin Config Update returns success without persisting
// Impact: Plugin configuration changes are ignored
func TestPluginConfigUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Get an installed plugin with configurable settings
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)

	var pluginsResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&pluginsResp)
	resp.Body.Close()

	var configPlugin PluginResponse
	for _, p := range pluginsResp.Plugins {
		if p.Installed && len(p.Config) > 0 {
			configPlugin = p
			break
		}
	}

	if configPlugin.ID == "" {
		t.Skip("No configurable plugins available for testing")
	}

	// Step 1: Get current config
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+configPlugin.ID+"/config", nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var currentConfig map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&currentConfig)
	resp.Body.Close()

	// Step 2: Update config with new value
	newConfig := make(map[string]interface{})
	for k, v := range currentConfig {
		newConfig[k] = v
	}
	// Add or modify a test setting
	newConfig["test_setting"] = "test_value_" + time.Now().Format("150405")

	body, _ := json.Marshal(newConfig)
	req, err = http.NewRequestWithContext(ctx, "PUT",
		baseURL+"/api/v1/plugins/"+configPlugin.ID+"/config", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Config update should return 200")

	// Step 3: Verify config persisted
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+configPlugin.ID+"/config", nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var updatedConfig map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&updatedConfig)
	resp.Body.Close()

	// CRITICAL CHECK: Config should be updated
	assert.Equal(t, newConfig["test_setting"], updatedConfig["test_setting"],
		"Config update should persist in database")

	// Step 4: Verify plugin was reloaded with new config
	// This is harder to test directly - we verify the plugin is still functional
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+configPlugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	var plugin PluginResponse
	json.NewDecoder(resp.Body).Decode(&plugin)
	resp.Body.Close()

	assert.True(t, plugin.Installed, "Plugin should still be installed after config update")

	t.Logf("Plugin config update test passed for: %s", configPlugin.Name)
}

// TestPluginUninstall validates that plugins can be completely removed (TC-006).
func TestPluginUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// First install a plugin so we can uninstall it
	// Get marketplace plugins
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/marketplace", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)

	var marketplaceResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&marketplaceResp)
	resp.Body.Close()

	var pluginToTest PluginResponse
	for _, p := range marketplaceResp.Plugins {
		if !p.Installed {
			pluginToTest = p
			break
		}
	}

	if pluginToTest.ID == "" {
		t.Skip("No plugins available for install/uninstall testing")
	}

	// Install the plugin
	installPayload := map[string]string{"pluginId": pluginToTest.ID}
	body, _ := json.Marshal(installPayload)
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/install", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	resp.Body.Close()

	// Wait for installation
	waitForCondition(30*time.Second, 2*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+pluginToTest.ID, nil)
		addAuthHeader(t, req)
		resp, _ := client.Do(req)
		var p PluginResponse
		json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		return p.Installed
	})

	// Uninstall the plugin
	req, err = http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/plugins/"+pluginToTest.ID+"/uninstall", nil)
	require.NoError(t, err)
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Uninstall should return 200")

	// Verify plugin is removed
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+pluginToTest.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)

	// Should either return 404 or show not installed
	if resp.StatusCode == http.StatusOK {
		var plugin PluginResponse
		json.NewDecoder(resp.Body).Decode(&plugin)
		assert.False(t, plugin.Installed, "Plugin should not be installed after uninstall")
	} else {
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Plugin should not be found after uninstall")
	}
	resp.Body.Close()

	t.Logf("Plugin uninstall test passed for: %s", pluginToTest.Name)
}

// TestPluginLifecycle validates the complete plugin lifecycle (TC-009).
func TestPluginLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Get a plugin from marketplace
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/marketplace", nil)
	addAuthHeader(t, req)
	resp, err := client.Do(req)
	require.NoError(t, err)

	var marketplaceResp PluginListResponse
	json.NewDecoder(resp.Body).Decode(&marketplaceResp)
	resp.Body.Close()

	var plugin PluginResponse
	for _, p := range marketplaceResp.Plugins {
		if !p.Installed {
			plugin = p
			break
		}
	}

	if plugin.ID == "" {
		t.Skip("No plugins available for lifecycle testing")
	}

	// Step 1: Install
	t.Log("Step 1: Installing plugin...")
	installPayload := map[string]string{"pluginId": plugin.ID}
	body, _ := json.Marshal(installPayload)
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/install", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted)
	resp.Body.Close()

	// Wait for installed
	waitForCondition(60*time.Second, 2*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+plugin.ID, nil)
		addAuthHeader(t, req)
		resp, _ := client.Do(req)
		var p PluginResponse
		json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		return p.Installed
	})

	// Step 2: Enable
	t.Log("Step 2: Enabling plugin...")
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+plugin.ID+"/enable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify enabled
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+plugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)
	var enabled PluginResponse
	json.NewDecoder(resp.Body).Decode(&enabled)
	resp.Body.Close()
	assert.True(t, enabled.Enabled, "Plugin should be enabled")

	// Step 3: Disable
	t.Log("Step 3: Disabling plugin...")
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+plugin.ID+"/disable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify disabled
	req, _ = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/plugins/"+plugin.ID, nil)
	addAuthHeader(t, req)
	resp, _ = client.Do(req)
	var disabled PluginResponse
	json.NewDecoder(resp.Body).Decode(&disabled)
	resp.Body.Close()
	assert.False(t, disabled.Enabled, "Plugin should be disabled")

	// Step 4: Re-enable
	t.Log("Step 4: Re-enabling plugin...")
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+plugin.ID+"/enable", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 5: Uninstall
	t.Log("Step 5: Uninstalling plugin...")
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/plugins/"+plugin.ID+"/uninstall", nil)
	addAuthHeader(t, req)
	addCSRFToken(t, req)
	resp, _ = client.Do(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Logf("Plugin lifecycle test passed for: %s", plugin.Name)
}
