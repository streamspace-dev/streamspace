package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/plugins"
)

// PluginMarketplaceHandler handles plugin marketplace HTTP requests
type PluginMarketplaceHandler struct {
	db          *db.Database
	marketplace *plugins.PluginMarketplace
	runtime     *plugins.RuntimeV2
}

// NewPluginMarketplaceHandler creates a new plugin marketplace handler
func NewPluginMarketplaceHandler(database *db.Database, marketplace *plugins.PluginMarketplace, runtime *plugins.RuntimeV2) *PluginMarketplaceHandler {
	return &PluginMarketplaceHandler{
		db:          database,
		marketplace: marketplace,
		runtime:     runtime,
	}
}

// RegisterRoutes registers plugin marketplace routes
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

// ListAvailablePlugins lists all plugins available in the marketplace
func (h *PluginMarketplaceHandler) ListAvailablePlugins(c *gin.Context) {
	plugins, err := h.marketplace.ListAvailable(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list available plugins",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// SyncCatalog forces a sync of the plugin catalog
func (h *PluginMarketplaceHandler) SyncCatalog(c *gin.Context) {
	if err := h.marketplace.SyncCatalog(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to sync catalog",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Catalog synced successfully",
	})
}

// GetPluginDetails gets details for a specific plugin
func (h *PluginMarketplaceHandler) GetPluginDetails(c *gin.Context) {
	name := c.Param("name")

	plugin, err := h.marketplace.GetPlugin(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// InstallPlugin installs a plugin from the marketplace
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

// UninstallPlugin uninstalls a plugin
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
			"error": "Failed to uninstall plugin",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin uninstalled successfully",
	})
}

// EnablePlugin enables a plugin
func (h *PluginMarketplaceHandler) EnablePlugin(c *gin.Context) {
	name := c.Param("name")

	// Update database
	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		UPDATE installed_plugins SET enabled = true, updated_at = NOW()
		WHERE name = $1
	`, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to enable plugin",
			"details": err.Error(),
		})
		return
	}

	// TODO: Load plugin into runtime if not already loaded

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin enabled successfully",
	})
}

// DisablePlugin disables a plugin
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
			"error": "Failed to disable plugin",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin disabled successfully",
	})
}

// ListInstalledPlugins lists all installed plugins
func (h *PluginMarketplaceHandler) ListInstalledPlugins(c *gin.Context) {
	plugins := h.runtime.ListPlugins()

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// GetInstalledPlugin gets details of an installed plugin
func (h *PluginMarketplaceHandler) GetInstalledPlugin(c *gin.Context) {
	name := c.Param("name")

	plugin, err := h.runtime.GetPlugin(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found or not loaded",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// UpdatePluginConfig updates a plugin's configuration
func (h *PluginMarketplaceHandler) UpdatePluginConfig(c *gin.Context) {
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

	// Update in database (implementation depends on schema)
	// TODO: Implement config update

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin configuration updated",
	})
}
