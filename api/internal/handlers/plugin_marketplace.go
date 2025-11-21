// Package handlers provides HTTP request handlers for the StreamSpace API.
//
// The plugin_marketplace.go file implements HTTP handlers for the plugin marketplace,
// which provides a higher-level API that combines catalog management, installation,
// and runtime lifecycle management.
//
// Marketplace vs Catalog:
//
//	Catalog (plugins.go):
//	  - Database-driven plugin catalog (catalog_plugins table)
//	  - Install by ID, manage by ID
//	  - Tracks ratings, statistics, metadata
//	  - More suitable for production UI with detailed plugin info
//
//	Marketplace (plugin_marketplace.go):
//	  - Runtime-driven plugin marketplace (PluginMarketplace + RuntimeV2)
//	  - Install by name, manage by name
//	  - Immediate load/unload (affects runtime state)
//	  - Catalog sync from external repositories
//	  - More suitable for programmatic API access
//
// API Endpoint Structure:
//
//	Marketplace Catalog:
//	  GET    /api/plugins/marketplace/catalog      - List available plugins
//	  POST   /api/plugins/marketplace/sync         - Force catalog sync
//	  GET    /api/plugins/marketplace/catalog/:name - Get plugin details
//
//	Plugin Installation (immediate load/unload):
//	  POST   /api/plugins/marketplace/install/:name   - Install + load plugin
//	  DELETE /api/plugins/marketplace/uninstall/:name - Unload + uninstall plugin
//	  POST   /api/plugins/marketplace/enable/:name    - Enable plugin
//	  POST   /api/plugins/marketplace/disable/:name   - Unload + disable plugin
//
//	Installed Plugins (runtime queries):
//	  GET    /api/plugins/marketplace/installed       - List loaded plugins
//	  GET    /api/plugins/marketplace/installed/:name - Get loaded plugin
//	  PUT    /api/plugins/marketplace/installed/:name/config - Update config
//
// Design Decisions:
//
//  1. Immediate Effect: Install/uninstall/enable/disable affect runtime immediately
//     - plugins.go: Changes database only, requires restart/reload
//     - marketplace.go: Changes database AND runtime state
//
//  2. Plugin Identification: Uses plugin name instead of database ID
//     - plugins.go: Uses database ID (/api/plugins/123)
//     - marketplace.go: Uses plugin name (/api/plugins/marketplace/install/slack-notifications)
//
//  3. Catalog Sync: External repository synchronization
//     - POST /api/plugins/marketplace/sync fetches latest plugins from repository
//     - Populates catalog_plugins and catalog_repositories tables
//
// Example Usage Flow:
//
//	1. Sync catalog from external repository:
//	   POST /api/plugins/marketplace/sync
//	   (Updates catalog_plugins from https://plugins.streamspace.io)
//
//	2. Browse available plugins:
//	   GET /api/plugins/marketplace/catalog
//
//	3. Install and load plugin immediately:
//	   POST /api/plugins/marketplace/install/slack-notifications
//	   Body: {"config": {"webhook_url": "..."}}
//	   (Plugin installed to database AND loaded into runtime)
//
//	4. Disable plugin (unloads from runtime):
//	   POST /api/plugins/marketplace/disable/slack-notifications
//	   (Plugin unloaded AND marked disabled in database)
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// PluginMarketplaceHandler handles plugin marketplace HTTP requests.
//
// This handler provides higher-level plugin management endpoints that:
//   - Sync catalog from external repositories
//   - Install/uninstall with immediate runtime effect
//   - Query runtime state directly (loaded plugins)
//
// Dependencies:
//   - database: For plugin metadata and state persistence
//   - marketplace: For catalog sync and plugin discovery
//   - runtime: For immediate load/unload operations
type PluginMarketplaceHandler struct {
	// db is the database connection for plugin state persistence.
	db *db.Database

	// marketplace handles catalog sync and plugin discovery.
	marketplace *plugins.PluginMarketplace

	// runtime manages loaded plugins and lifecycle.
	runtime *plugins.RuntimeV2
}

// NewPluginMarketplaceHandler creates a new plugin marketplace handler.
//
// Parameters:
//   - database: Database connection
//   - marketplace: Plugin marketplace for catalog operations
//   - runtime: Plugin runtime for load/unload operations
//
// Returns:
//   - Configured PluginMarketplaceHandler ready to register routes
//
// Example:
//
//	marketplace := plugins.NewPluginMarketplace(db, repoURL)
//	runtime := plugins.NewRuntimeV2(db)
//	handler := NewPluginMarketplaceHandler(db, marketplace, runtime)
//	handler.RegisterRoutes(router.Group("/api"))
func NewPluginMarketplaceHandler(database *db.Database, marketplace *plugins.PluginMarketplace, runtime *plugins.RuntimeV2) *PluginMarketplaceHandler {
	return &PluginMarketplaceHandler{
		db:          database,
		marketplace: marketplace,
		runtime:     runtime,
	}
}

// RegisterRoutes registers plugin marketplace routes to the provided router group.
//
// Mounts all marketplace endpoints under /plugins/marketplace prefix:
//   - Catalog endpoints: /plugins/marketplace/catalog, /plugins/marketplace/sync
//   - Installation endpoints: /plugins/marketplace/install/:name, etc.
//   - Installed endpoints: /plugins/marketplace/installed
//
// Parameters:
//   - r: Gin router group to mount routes on (typically /api)
//
// Example:
//
//	api := router.Group("/api")
//	handler.RegisterRoutes(api)
//	// Routes available at: /api/plugins/marketplace/catalog, etc.
func (h *PluginMarketplaceHandler) RegisterRoutes(r *gin.RouterGroup) {
	marketplace := r.Group("/plugins/marketplace")
	{
		// Marketplace catalog
		marketplace.GET("/catalog", h.ListAvailablePlugins)
		marketplace.POST("/sync", h.SyncCatalog)
		marketplace.GET("/catalog/:name", h.GetPluginDetails)

		// Plugin installation
		marketplace.POST("/install/:name", h.InstallPlugin)
		marketplace.DELETE("/uninstall/:name", h.UninstallPlugin)
		marketplace.POST("/enable/:name", h.EnablePlugin)
		marketplace.POST("/disable/:name", h.DisablePlugin)

		// Installed plugins
		marketplace.GET("/installed", h.ListInstalledPlugins)
		marketplace.GET("/installed/:name", h.GetInstalledPlugin)
		marketplace.PUT("/installed/:name/config", h.UpdatePluginConfig)
	}
}

// ListAvailablePlugins lists all plugins available in the marketplace.
//
// Endpoint: GET /api/plugins/marketplace/catalog
//
// Response: JSON with plugins array and count
//
// Data Source:
//   - marketplace.ListAvailable() queries catalog_plugins table
//   - Populated by POST /sync or periodic background sync
//
// Example Request:
//
//	GET /api/plugins/marketplace/catalog
//
// Example Response:
//
//	{
//	  "plugins": [
//	    {
//	      "name": "slack-notifications",
//	      "version": "1.2.3",
//	      "display_name": "Slack Notifications",
//	      "description": "...",
//	      "manifest": {...}
//	    }
//	  ],
//	  "count": 1
//	}
//
// HTTP Status Codes:
//   - 200: Success (may return empty array if catalog not synced)
//   - 500: Database or marketplace error
func (h *PluginMarketplaceHandler) ListAvailablePlugins(c *gin.Context) {
	plugins, err := h.marketplace.ListAvailable(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list available plugins",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// SyncCatalog forces a synchronization of the plugin catalog from external repository.
//
// Endpoint: POST /api/plugins/marketplace/sync
//
// Behavior:
//   - Fetches latest plugin list from configured repository URL
//   - Updates catalog_plugins table with new/updated plugins
//   - Updates catalog_repositories table with repository metadata
//
// Example Request:
//
//	POST /api/plugins/marketplace/sync
//
// Example Response:
//
//	{
//	  "message": "Catalog synced successfully"
//	}
//
// Use Cases:
//   - Manual catalog refresh after repository update
//   - Troubleshooting catalog sync issues
//   - Initial catalog population
//
// HTTP Status Codes:
//   - 200: Catalog synced successfully
//   - 500: Sync failed (network error, invalid catalog format, etc.)
func (h *PluginMarketplaceHandler) SyncCatalog(c *gin.Context) {
	if err := h.marketplace.SyncCatalog(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to sync catalog",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Catalog synced successfully",
	})
}

// GetPluginDetails gets details for a specific plugin from the marketplace catalog.
//
// Endpoint: GET /api/plugins/marketplace/catalog/:name
//
// Path Parameters:
//   - name: Plugin name (e.g., "slack-notifications")
//
// Response: JSON with complete plugin metadata and manifest
//
// Example Request:
//
//	GET /api/plugins/marketplace/catalog/slack-notifications
//
// Example Response:
//
//	{
//	  "name": "slack-notifications",
//	  "version": "1.2.3",
//	  "display_name": "Slack Notifications",
//	  "description": "Send session notifications to Slack",
//	  "manifest": {
//	    "permissions": ["sessions.read", "users.read"],
//	    "config_schema": {...}
//	  }
//	}
//
// HTTP Status Codes:
//   - 200: Success
//   - 404: Plugin not found in catalog
//   - 500: Database or marketplace error
func (h *PluginMarketplaceHandler) GetPluginDetails(c *gin.Context) {
	name := c.Param("name")

	plugin, err := h.marketplace.GetPlugin(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Plugin not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// InstallPlugin installs a plugin from the marketplace and loads it immediately.
//
// Endpoint: POST /api/plugins/marketplace/install/:name
//
// Path Parameters:
//   - name: Plugin name to install
//
// Request Body:
//
//	{
//	  "config": {"api_key": "..."}  // Plugin-specific configuration
//	}
//
// Behavior:
//  1. Calls marketplace.InstallPlugin (adds to installed_plugins table)
//  2. Fetches plugin metadata from marketplace.GetPlugin
//  3. Calls runtime.LoadPluginWithConfig (loads into runtime immediately)
//
// This is the key difference from plugins.go:
//   - plugins.go: Install to database only, requires restart/reload
//   - marketplace.go: Install to database AND load into runtime
//
// Example Request:
//
//	POST /api/plugins/marketplace/install/slack-notifications
//	{
//	  "config": {
//	    "webhook_url": "https://hooks.slack.com/...",
//	    "channel": "#general"
//	  }
//	}
//
// Example Response:
//
//	{
//	  "message": "Plugin installed and activated successfully",
//	  "plugin": {
//	    "name": "slack-notifications",
//	    "version": "1.2.3",
//	    "manifest": {...}
//	  }
//	}
//
// HTTP Status Codes:
//   - 200: Plugin installed and loaded successfully
//   - 400: Invalid request body
//   - 500: Install or load failed
func (h *PluginMarketplaceHandler) InstallPlugin(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		Config map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Install plugin
	if err := h.marketplace.InstallPlugin(c.Request.Context(), name, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to install plugin",
			"details": err.Error(),
		})
		return
	}

	// Load the plugin immediately
	plugin, err := h.marketplace.GetPlugin(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Plugin installed but failed to load metadata",
			"details": err.Error(),
		})
		return
	}

	if err := h.runtime.LoadPluginWithConfig(c.Request.Context(), name, plugin.Version, req.Config, plugin.Manifest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Plugin installed but failed to load",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin installed and activated successfully",
		"plugin":  plugin,
	})
}

// UninstallPlugin unloads and uninstalls a plugin.
//
// Endpoint: DELETE /api/plugins/marketplace/uninstall/:name
//
// Path Parameters:
//   - name: Plugin name to uninstall
//
// Behavior:
//  1. Calls runtime.UnloadPlugin (unloads from runtime)
//  2. Calls marketplace.UninstallPlugin (removes from database)
//
// Note: Unload errors are logged but don't fail the request
// (plugin might not be loaded).
//
// Example Request:
//
//	DELETE /api/plugins/marketplace/uninstall/slack-notifications
//
// HTTP Status Codes:
//   - 200: Plugin uninstalled successfully
//   - 500: Uninstall failed
func (h *PluginMarketplaceHandler) UninstallPlugin(c *gin.Context) {
	name := c.Param("name")

	// Unload plugin from runtime first
	if err := h.runtime.UnloadPlugin(c.Request.Context(), name); err != nil {
		// Log but don't fail - plugin might not be loaded
		c.Request.Context().Value("logger").(interface{ Printf(string, ...interface{}) }).
			Printf("Warning: Failed to unload plugin %s: %v", name, err)
	}

	// Uninstall from marketplace
	if err := h.marketplace.UninstallPlugin(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to uninstall plugin",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin uninstalled successfully",
	})
}

// EnablePlugin enables a plugin in the database.
//
// Endpoint: POST /api/plugins/marketplace/enable/:name
//
// Path Parameters:
//   - name: Plugin name to enable
//
// Behavior:
//   - Sets enabled=true in installed_plugins table
//   - TODO: Should load plugin into runtime (not currently implemented)
//
// Example Request:
//
//	POST /api/plugins/marketplace/enable/slack-notifications
//
// HTTP Status Codes:
//   - 200: Plugin enabled successfully
//   - 500: Database update failed
func (h *PluginMarketplaceHandler) EnablePlugin(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Update database
	result, err := h.db.DB().ExecContext(ctx, `
		UPDATE installed_plugins SET enabled = true, updated_at = NOW()
		WHERE name = $1
	`, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to enable plugin",
			"details": err.Error(),
		})
		return
	}

	// Check if plugin was found
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}

	// Load plugin into runtime
	if err := h.runtime.LoadPluginByName(ctx, name); err != nil {
		// Log the error but return success since DB was updated
		// Plugin will be loaded on next restart
		log.Printf("Warning: Failed to load plugin %s into runtime: %v", name, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "Plugin enabled in database. Note: Failed to load into runtime - will load on restart.",
			"warning": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin enabled and loaded successfully",
	})
}

// DisablePlugin unloads and disables a plugin.
//
// Endpoint: POST /api/plugins/marketplace/disable/:name
//
// Path Parameters:
//   - name: Plugin name to disable
//
// Behavior:
//  1. Calls runtime.UnloadPlugin (unloads from runtime)
//  2. Sets enabled=false in database
//
// Example Request:
//
//	POST /api/plugins/marketplace/disable/slack-notifications
//
// HTTP Status Codes:
//   - 200: Plugin disabled successfully
//   - 500: Database update failed
func (h *PluginMarketplaceHandler) DisablePlugin(c *gin.Context) {
	name := c.Param("name")

	// Unload from runtime
	if err := h.runtime.UnloadPlugin(c.Request.Context(), name); err != nil {
		// Log but don't fail
	}

	// Update database
	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		UPDATE installed_plugins SET enabled = false, updated_at = NOW()
		WHERE name = $1
	`, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to disable plugin",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin disabled successfully",
	})
}

// ListInstalledPlugins lists all plugins currently loaded in the runtime.
//
// Endpoint: GET /api/plugins/marketplace/installed
//
// Response: JSON with plugins array and count
//
// Data Source:
//   - runtime.ListPlugins() returns currently loaded plugins from memory
//   - This shows runtime state, not database state
//
// Example Request:
//
//	GET /api/plugins/marketplace/installed
//
// Example Response:
//
//	{
//	  "plugins": [
//	    {
//	      "name": "slack-notifications",
//	      "version": "1.2.3",
//	      "enabled": true,
//	      "loaded_at": "2025-01-15T10:30:00Z",
//	      "is_builtin": false
//	    }
//	  ],
//	  "count": 1
//	}
//
// HTTP Status Codes:
//   - 200: Success (always succeeds, may return empty array)
func (h *PluginMarketplaceHandler) ListInstalledPlugins(c *gin.Context) {
	plugins := h.runtime.ListPlugins()

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// GetInstalledPlugin gets details of a specific loaded plugin from runtime.
//
// Endpoint: GET /api/plugins/marketplace/installed/:name
//
// Path Parameters:
//   - name: Plugin name
//
// Response: JSON with loaded plugin details
//
// Example Request:
//
//	GET /api/plugins/marketplace/installed/slack-notifications
//
// HTTP Status Codes:
//   - 200: Success
//   - 404: Plugin not loaded in runtime
func (h *PluginMarketplaceHandler) GetInstalledPlugin(c *gin.Context) {
	name := c.Param("name")

	plugin, err := h.runtime.GetPlugin(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Plugin not found or not loaded",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// UpdatePluginConfig updates a plugin's configuration.
//
// Endpoint: PUT /api/plugins/marketplace/installed/:name/config
//
// Path Parameters:
//   - name: Plugin name
//
// Request Body:
//
//	{
//	  "config": {"api_key": "new-value"}
//	}
//
// TODO: Implementation needed
//   - Update installed_plugins.config column
//   - Reload plugin with new config
//
// Example Request:
//
//	PUT /api/plugins/marketplace/installed/slack-notifications/config
//	{
//	  "config": {"webhook_url": "https://new-url.com"}
//	}
//
// HTTP Status Codes:
//   - 200: Config updated (currently always succeeds - TODO)
//   - 400: Invalid request body
func (h *PluginMarketplaceHandler) UpdatePluginConfig(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	var req struct {
		Config map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Marshal config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid config format",
			"details": err.Error(),
		})
		return
	}

	// Update config in database
	result, err := h.db.DB().ExecContext(ctx, `
		UPDATE installed_plugins
		SET config = $1, updated_at = NOW()
		WHERE name = $2
	`, configJSON, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update plugin config",
			"details": err.Error(),
		})
		return
	}

	// Check if plugin was found
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}

	// Reload plugin with new config
	if err := h.runtime.ReloadPlugin(ctx, name); err != nil {
		// Log but return success since DB was updated
		log.Printf("Warning: Failed to reload plugin %s with new config: %v", name, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "Config updated in database. Note: Failed to reload plugin - will apply on restart.",
			"warning": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin configuration updated and reloaded successfully",
	})
}
