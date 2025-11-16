package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
)

// RuntimeV2 manages the lifecycle and execution of plugins with automatic discovery
type RuntimeV2 struct {
	db          *db.Database
	discovery   *PluginDiscovery
	plugins     map[string]*LoadedPlugin
	pluginsMux  sync.RWMutex
	eventBus    *EventBus
	scheduler   *cron.Cron
	apiRegistry *APIRegistry
	uiRegistry  *UIRegistry
	autoStart   bool
}

// NewRuntimeV2 creates a new plugin runtime with automatic discovery
func NewRuntimeV2(database *db.Database, pluginDirs ...string) *RuntimeV2 {
	return &RuntimeV2{
		db:          database,
		discovery:   NewPluginDiscovery(pluginDirs...),
		plugins:     make(map[string]*LoadedPlugin),
		eventBus:    NewEventBus(),
		scheduler:   cron.New(),
		apiRegistry: NewAPIRegistry(),
		uiRegistry:  NewUIRegistry(),
		autoStart:   true,
	}
}

// SetAutoStart enables/disables automatic plugin loading on Start()
func (r *RuntimeV2) SetAutoStart(enabled bool) {
	r.autoStart = enabled
}

// RegisterBuiltinPlugin registers a built-in plugin for automatic discovery
func (r *RuntimeV2) RegisterBuiltinPlugin(name string, factory PluginFactory) {
	r.discovery.RegisterBuiltin(name, factory)
}

// Start initializes the plugin runtime and auto-loads enabled plugins
func (r *RuntimeV2) Start(ctx context.Context) error {
	log.Println("[Plugin Runtime] Starting with automatic discovery...")

	// Start scheduler
	r.scheduler.Start()

	// Discover all available plugins
	availablePlugins, err := r.discovery.DiscoverAll()
	if err != nil {
		log.Printf("[Plugin Runtime] Warning: Plugin discovery had errors: %v", err)
	}

	log.Printf("[Plugin Runtime] Discovered %d available plugins", len(availablePlugins))

	if !r.autoStart {
		log.Println("[Plugin Runtime] Auto-start disabled, plugins must be loaded manually")
		return nil
	}

	// Load enabled plugins from database
	loadedCount, err := r.loadEnabledPlugins(ctx)
	if err != nil {
		return fmt.Errorf("failed to load enabled plugins: %w", err)
	}

	log.Printf("[Plugin Runtime] Started successfully, loaded %d plugins", loadedCount)
	return nil
}

// loadEnabledPlugins loads all enabled plugins from the database
func (r *RuntimeV2) loadEnabledPlugins(ctx context.Context) (int, error) {
	// Query enabled plugins from database
	rows, err := r.db.DB().QueryContext(ctx, `
		SELECT id, name, version, enabled, config, catalog_plugin_id
		FROM installed_plugins
		WHERE enabled = true
		ORDER BY name
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("[Plugin Runtime] No enabled plugins found in database")
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query installed plugins: %w", err)
	}
	defer rows.Close()

	loadedCount := 0
	for rows.Next() {
		var plugin models.InstalledPlugin
		var catalogID sql.NullInt64
		var configJSON []byte

		err := rows.Scan(
			&plugin.ID,
			&plugin.Name,
			&plugin.Version,
			&plugin.Enabled,
			&configJSON,
			&catalogID,
		)
		if err != nil {
			log.Printf("[Plugin Runtime] Error scanning plugin: %v", err)
			continue
		}

		// Parse config
		var config map[string]interface{}
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &config); err != nil {
				log.Printf("[Plugin Runtime] Error parsing config for %s: %v", plugin.Name, err)
				config = make(map[string]interface{})
			}
		}

		// Load manifest from catalog if available
		var manifest models.PluginManifest
		if catalogID.Valid {
			err = r.db.DB().QueryRowContext(ctx, `
				SELECT manifest FROM catalog_plugins WHERE id = $1
			`, catalogID.Int64).Scan(&manifest)
			if err != nil {
				log.Printf("[Plugin Runtime] Warning: Could not load manifest for %s: %v", plugin.Name, err)
				// Continue without manifest
			}
		}

		// Load the plugin
		if err := r.LoadPluginWithConfig(ctx, plugin.Name, plugin.Version, config, manifest); err != nil {
			log.Printf("[Plugin Runtime] Error loading plugin %s: %v", plugin.Name, err)
			continue
		}

		loadedCount++
	}

	return loadedCount, nil
}

// LoadPluginWithConfig loads and initializes a plugin with specific configuration
func (r *RuntimeV2) LoadPluginWithConfig(ctx context.Context, name, version string, config map[string]interface{}, manifest models.PluginManifest) error {
	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	// Check if already loaded
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s is already loaded", name)
	}

	log.Printf("[Plugin Runtime] Loading plugin: %s@%s", name, version)

	// Load plugin handler via discovery
	handler, err := r.discovery.LoadPlugin(name)
	if err != nil {
		return fmt.Errorf("failed to load plugin handler: %w", err)
	}

	// Create plugin context
	pluginCtx := &PluginContext{
		PluginName: name,
		Config:     config,
		Manifest:   manifest,
		runtime:    (*Runtime)(nil), // Will be set if needed
	}

	// Initialize plugin components
	pluginCtx.Database = NewPluginDatabase(r.db, name)
	pluginCtx.Events = NewPluginEvents(r.eventBus, name)
	pluginCtx.API = NewPluginAPI(r.apiRegistry, name)
	pluginCtx.UI = NewPluginUI(r.uiRegistry, name)
	pluginCtx.Storage = NewPluginStorage(r.db, name)
	pluginCtx.Logger = NewPluginLogger(name)
	pluginCtx.Scheduler = NewPluginScheduler(r.scheduler, name)

	// Create plugin instance
	instance := &PluginInstance{
		Context:   pluginCtx,
		Storage:   pluginCtx.Storage,
		Logger:    pluginCtx.Logger,
		Scheduler: pluginCtx.Scheduler,
	}

	// Create loaded plugin
	loaded := &LoadedPlugin{
		Name:      name,
		Version:   version,
		Enabled:   true,
		Config:    config,
		Manifest:  manifest,
		Handler:   handler,
		Instance:  instance,
		LoadedAt:  time.Now(),
		IsBuiltin: r.discovery.IsBuiltin(name),
	}

	// Call OnLoad hook
	if err := handler.OnLoad(pluginCtx); err != nil {
		return fmt.Errorf("plugin OnLoad failed: %w", err)
	}

	// Register plugin
	r.plugins[name] = loaded

	log.Printf("[Plugin Runtime] Plugin loaded successfully: %s@%s (builtin: %v)", name, version, loaded.IsBuiltin)
	return nil
}

// Stop gracefully shuts down the plugin runtime
func (r *RuntimeV2) Stop(ctx context.Context) error {
	log.Println("[Plugin Runtime] Stopping...")

	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	// Unload all plugins
	for name := range r.plugins {
		if err := r.unloadPluginLocked(ctx, name); err != nil {
			log.Printf("[Plugin Runtime] Error unloading plugin %s: %v", name, err)
		}
	}

	// Stop scheduler
	schedCtx := r.scheduler.Stop()
	<-schedCtx.Done()

	log.Println("[Plugin Runtime] Stopped successfully")
	return nil
}

// UnloadPlugin unloads a plugin
func (r *RuntimeV2) UnloadPlugin(ctx context.Context, name string) error {
	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	return r.unloadPluginLocked(ctx, name)
}

func (r *RuntimeV2) unloadPluginLocked(ctx context.Context, name string) error {
	plugin, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s is not loaded", name)
	}

	log.Printf("[Plugin Runtime] Unloading plugin: %s", name)

	// Call OnUnload hook
	if err := plugin.Handler.OnUnload(plugin.Instance.Context); err != nil {
		log.Printf("[Plugin Runtime] Plugin OnUnload failed: %v", err)
		// Continue unloading even if hook fails
	}

	// Cleanup plugin resources
	plugin.Instance.Scheduler.RemoveAll()
	r.apiRegistry.UnregisterAll(name)
	r.uiRegistry.UnregisterAll(name)
	r.eventBus.UnsubscribeAll(name)

	// Remove from registry
	delete(r.plugins, name)

	log.Printf("[Plugin Runtime] Plugin unloaded successfully: %s", name)
	return nil
}

// EmitEvent emits an event to all listening plugins
func (r *RuntimeV2) EmitEvent(eventType string, data interface{}) {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	// Emit to event bus
	r.eventBus.Emit(eventType, data)

	// Call appropriate lifecycle hooks
	for name, plugin := range r.plugins {
		if !plugin.Enabled {
			continue
		}

		// Run in goroutine to not block
		go func(p *LoadedPlugin, pName string) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("[Plugin Runtime] Plugin %s panicked on event %s: %v", pName, eventType, err)
				}
			}()

			var err error
			switch eventType {
			case "session.created":
				err = p.Handler.OnSessionCreated(p.Instance.Context, data)
			case "session.started":
				err = p.Handler.OnSessionStarted(p.Instance.Context, data)
			case "session.stopped":
				err = p.Handler.OnSessionStopped(p.Instance.Context, data)
			case "session.hibernated":
				err = p.Handler.OnSessionHibernated(p.Instance.Context, data)
			case "session.woken":
				err = p.Handler.OnSessionWoken(p.Instance.Context, data)
			case "session.deleted":
				err = p.Handler.OnSessionDeleted(p.Instance.Context, data)
			case "user.created":
				err = p.Handler.OnUserCreated(p.Instance.Context, data)
			case "user.updated":
				err = p.Handler.OnUserUpdated(p.Instance.Context, data)
			case "user.deleted":
				err = p.Handler.OnUserDeleted(p.Instance.Context, data)
			case "user.login":
				err = p.Handler.OnUserLogin(p.Instance.Context, data)
			case "user.logout":
				err = p.Handler.OnUserLogout(p.Instance.Context, data)
			}

			if err != nil {
				log.Printf("[Plugin Runtime] Plugin %s error on event %s: %v", pName, eventType, err)
			}
		}(plugin, name)
	}
}

// GetPlugin retrieves a loaded plugin
func (r *RuntimeV2) GetPlugin(name string) (*LoadedPlugin, error) {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugin, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not loaded", name)
	}

	return plugin, nil
}

// ListPlugins returns all loaded plugins
func (r *RuntimeV2) ListPlugins() []*LoadedPlugin {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugins := make([]*LoadedPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// ListAvailablePlugins returns all discoverable plugins (loaded or not)
func (r *RuntimeV2) ListAvailablePlugins() []string {
	plugins, _ := r.discovery.DiscoverAll()
	return plugins
}

// GetEventBus returns the event bus for direct access
func (r *RuntimeV2) GetEventBus() *EventBus {
	return r.eventBus
}

// GetAPIRegistry returns the API registry for direct access
func (r *RuntimeV2) GetAPIRegistry() *APIRegistry {
	return r.apiRegistry
}

// GetUIRegistry returns the UI registry for direct access
func (r *RuntimeV2) GetUIRegistry() *UIRegistry {
	return r.uiRegistry
}
