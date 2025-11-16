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

// Runtime manages the lifecycle and execution of plugins
type Runtime struct {
	db          *db.Database
	plugins     map[string]*LoadedPlugin
	pluginsMux  sync.RWMutex
	eventBus    *EventBus
	scheduler   *cron.Cron
	apiRegistry *APIRegistry
	uiRegistry  *UIRegistry
}

// LoadedPlugin represents a plugin that is loaded and running
type LoadedPlugin struct {
	ID          int
	Name        string
	Version     string
	Enabled     bool
	Config      map[string]interface{}
	Manifest    models.PluginManifest
	Handler     PluginHandler
	Instance    *PluginInstance
	LoadedAt    time.Time
}

// PluginHandler is the interface that all plugins must implement
type PluginHandler interface {
	// Lifecycle hooks
	OnLoad(ctx *PluginContext) error
	OnUnload(ctx *PluginContext) error
	OnEnable(ctx *PluginContext) error
	OnDisable(ctx *PluginContext) error

	// Event handlers (optional)
	OnSessionCreated(ctx *PluginContext, session interface{}) error
	OnSessionStarted(ctx *PluginContext, session interface{}) error
	OnSessionStopped(ctx *PluginContext, session interface{}) error
	OnSessionHibernated(ctx *PluginContext, session interface{}) error
	OnSessionWoken(ctx *PluginContext, session interface{}) error
	OnSessionDeleted(ctx *PluginContext, session interface{}) error

	OnUserCreated(ctx *PluginContext, user interface{}) error
	OnUserUpdated(ctx *PluginContext, user interface{}) error
	OnUserDeleted(ctx *PluginContext, user interface{}) error
	OnUserLogin(ctx *PluginContext, user interface{}) error
	OnUserLogout(ctx *PluginContext, user interface{}) error
}

// PluginInstance holds the runtime state of a plugin
type PluginInstance struct {
	Context   *PluginContext
	Storage   *PluginStorage
	Logger    *PluginLogger
	Scheduler *PluginScheduler
}

// PluginContext provides plugins with access to platform APIs
type PluginContext struct {
	PluginName string
	Config     map[string]interface{}
	Manifest   models.PluginManifest

	// Platform APIs
	Database  *PluginDatabase
	Events    *PluginEvents
	API       *PluginAPI
	UI        *PluginUI
	Storage   *PluginStorage
	Logger    *PluginLogger
	Scheduler *PluginScheduler

	// Platform state
	runtime *Runtime
}

// NewRuntime creates a new plugin runtime
func NewRuntime(database *db.Database) *Runtime {
	return &Runtime{
		db:          database,
		plugins:     make(map[string]*LoadedPlugin),
		eventBus:    NewEventBus(),
		scheduler:   cron.New(),
		apiRegistry: NewAPIRegistry(),
		uiRegistry:  NewUIRegistry(),
	}
}

// Start initializes the plugin runtime and loads enabled plugins
func (r *Runtime) Start(ctx context.Context) error {
	log.Println("[Plugin Runtime] Starting...")

	// Start scheduler
	r.scheduler.Start()

	// Load all enabled plugins from database
	rows, err := r.db.DB().QueryContext(ctx, `
		SELECT id, name, version, enabled, config, catalog_plugin_id
		FROM installed_plugins
		WHERE enabled = true
		ORDER BY name
	`)
	if err != nil {
		return fmt.Errorf("failed to query installed plugins: %w", err)
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
				continue
			}
		}

		// Load manifest from catalog
		var manifest models.PluginManifest
		if catalogID.Valid {
			err = r.db.DB().QueryRowContext(ctx, `
				SELECT manifest FROM catalog_plugins WHERE id = $1
			`, catalogID.Int64).Scan(&manifest)
			if err != nil {
				log.Printf("[Plugin Runtime] Error loading manifest for %s: %v", plugin.Name, err)
				continue
			}
		}

		// Load the plugin
		if err := r.LoadPlugin(ctx, plugin.Name, plugin.Version, config, manifest); err != nil {
			log.Printf("[Plugin Runtime] Error loading plugin %s: %v", plugin.Name, err)
			continue
		}

		loadedCount++
	}

	log.Printf("[Plugin Runtime] Started successfully, loaded %d plugins", loadedCount)
	return nil
}

// Stop gracefully shuts down the plugin runtime
func (r *Runtime) Stop(ctx context.Context) error {
	log.Println("[Plugin Runtime] Stopping...")

	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	// Unload all plugins
	for name, plugin := range r.plugins {
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

// LoadPlugin loads and initializes a plugin
func (r *Runtime) LoadPlugin(ctx context.Context, name, version string, config map[string]interface{}, manifest models.PluginManifest) error {
	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	// Check if already loaded
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s is already loaded", name)
	}

	log.Printf("[Plugin Runtime] Loading plugin: %s@%s", name, version)

	// Create plugin context
	pluginCtx := &PluginContext{
		PluginName: name,
		Config:     config,
		Manifest:   manifest,
		runtime:    r,
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

	// Load plugin handler (this would dynamically load the plugin code)
	// For now, we'll use a placeholder that checks for built-in plugins
	handler, err := r.loadPluginHandler(name, version, manifest)
	if err != nil {
		return fmt.Errorf("failed to load plugin handler: %w", err)
	}

	// Create loaded plugin
	loaded := &LoadedPlugin{
		Name:     name,
		Version:  version,
		Enabled:  true,
		Config:   config,
		Manifest: manifest,
		Handler:  handler,
		Instance: instance,
		LoadedAt: time.Now(),
	}

	// Call OnLoad hook
	if err := handler.OnLoad(pluginCtx); err != nil {
		return fmt.Errorf("plugin OnLoad failed: %w", err)
	}

	// Register plugin
	r.plugins[name] = loaded

	log.Printf("[Plugin Runtime] Plugin loaded successfully: %s@%s", name, version)
	return nil
}

// UnloadPlugin unloads a plugin
func (r *Runtime) UnloadPlugin(ctx context.Context, name string) error {
	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	return r.unloadPluginLocked(ctx, name)
}

func (r *Runtime) unloadPluginLocked(ctx context.Context, name string) error {
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
func (r *Runtime) EmitEvent(eventType string, data interface{}) {
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
func (r *Runtime) GetPlugin(name string) (*LoadedPlugin, error) {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugin, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not loaded", name)
	}

	return plugin, nil
}

// ListPlugins returns all loaded plugins
func (r *Runtime) ListPlugins() []*LoadedPlugin {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugins := make([]*LoadedPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// loadPluginHandler loads the plugin handler implementation
// This is a placeholder that would be replaced with dynamic loading
func (r *Runtime) loadPluginHandler(name, version string, manifest models.PluginManifest) (PluginHandler, error) {
	// Check if it's a built-in plugin
	if handler := GetBuiltinPlugin(name); handler != nil {
		return handler, nil
	}

	// TODO: Implement dynamic plugin loading from filesystem
	// For now, return error
	return nil, fmt.Errorf("dynamic plugin loading not yet implemented (plugin: %s)", name)
}

// GetEventBus returns the event bus for direct access
func (r *Runtime) GetEventBus() *EventBus {
	return r.eventBus
}

// GetAPIRegistry returns the API registry for direct access
func (r *Runtime) GetAPIRegistry() *APIRegistry {
	return r.apiRegistry
}

// GetUIRegistry returns the UI registry for direct access
func (r *Runtime) GetUIRegistry() *UIRegistry {
	return r.uiRegistry
}
