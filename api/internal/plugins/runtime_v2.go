// Package plugins provides the plugin system for StreamSpace API.
//
// The runtime_v2 component is the central orchestrator that manages the entire
// plugin lifecycle, from discovery to loading, execution, and cleanup.
//
// Design Rationale - Why RuntimeV2:
//
// RuntimeV2 is an evolution of the original Runtime that adds:
//   1. Automatic discovery of available plugins (filesystem + built-in)
//   2. Database-driven plugin loading (loads only enabled plugins)
//   3. Auto-start capability (plugins load on API startup)
//   4. Integrated event bus for inter-plugin communication
//   5. Centralized registries (API, UI, Events, Scheduler)
//
// Plugin Lifecycle Flow:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│ 1. DISCOVERY                                                │
//	│    - Scan plugin directories for .so files                 │
//	│    - Enumerate built-in plugins                            │
//	│    - Build catalog of available plugins                    │
//	└────────────────────────┬────────────────────────────────────┘
//	                         ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│ 2. DATABASE QUERY                                           │
//	│    - SELECT * FROM installed_plugins WHERE enabled = true  │
//	│    - Load plugin configuration from database               │
//	│    - Load plugin manifest (metadata, permissions, etc.)    │
//	└────────────────────────┬────────────────────────────────────┘
//	                         ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│ 3. PLUGIN LOADING                                           │
//	│    - Load plugin handler via discovery system              │
//	│    - Create PluginContext with all helper components       │
//	│    - Initialize plugin instance                            │
//	│    - Call OnLoad() lifecycle hook                          │
//	└────────────────────────┬────────────────────────────────────┘
//	                         ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│ 4. RUNTIME EXECUTION                                        │
//	│    - Handle lifecycle events (sessions, users, etc.)       │
//	│    - Execute scheduled jobs via cron scheduler             │
//	│    - Process API requests via registered endpoints         │
//	│    - Render UI components via registered components        │
//	└────────────────────────┬────────────────────────────────────┘
//	                         ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│ 5. SHUTDOWN                                                 │
//	│    - Call OnUnload() lifecycle hook for each plugin        │
//	│    - Remove scheduled jobs                                 │
//	│    - Unregister API endpoints                              │
//	│    - Unregister UI components                              │
//	│    - Cleanup event subscriptions                           │
//	└─────────────────────────────────────────────────────────────┘
//
// Event-Driven Architecture:
//
// RuntimeV2 acts as an event hub, broadcasting system events to all loaded
// plugins. This enables plugins to react to platform events without tight coupling:
//
//	// When a session is created, runtime broadcasts to all plugins:
//	runtime.EmitEvent("session.created", sessionData)
//
//	// Each loaded plugin receives the event via its OnSessionCreated hook:
//	func (p *MyPlugin) OnSessionCreated(ctx *PluginContext, session interface{}) error {
//	    // React to session creation
//	    return nil
//	}
//
// Event Types:
//   - session.created, session.started, session.stopped
//   - session.hibernated, session.woken, session.deleted
//   - user.created, user.updated, user.deleted
//   - user.login, user.logout
//
// Automatic Discovery vs Manual Loading:
//
// RuntimeV2 supports two plugin loading modes:
//
//	1. Auto-start (default): Automatically loads all enabled plugins from database
//	   - Best for: Production deployments
//	   - Use case: Plugins are managed via UI/API, enabled state in database
//	   - Example: Admin enables "slack-notifications" via UI → loads on restart
//
//	2. Manual loading: Plugins must be loaded via API calls
//	   - Best for: Development, testing, debugging
//	   - Use case: Fine-grained control over plugin loading
//	   - Example: Load specific plugin version for testing
//
// Database Schema Integration:
//
// RuntimeV2 relies on two main database tables:
//
//	installed_plugins:
//	  - id, name, version, enabled, config, catalog_plugin_id
//	  - Tracks which plugins are installed and their configuration
//	  - enabled=true → plugin loads on startup (auto-start mode)
//
//	catalog_plugins:
//	  - id, name, version, manifest, source_url, ...
//	  - Plugin catalog metadata (description, icon, permissions, etc.)
//	  - Linked to installed_plugins via catalog_plugin_id
//
// Plugin Context Components:
//
// Each loaded plugin receives a PluginContext with access to:
//   - Database: Namespaced table access (plugin_name_*)
//   - Events: Pub/sub event system (subscribe to platform events)
//   - API: HTTP endpoint registration (/api/plugins/{name}/*)
//   - UI: Component registration (widgets, pages, menu items)
//   - Storage: Key-value storage (plugin configuration)
//   - Logger: Structured JSON logging with plugin name tagging
//   - Scheduler: Cron-based job scheduling
//
// Thread Safety:
//
// RuntimeV2 uses sync.RWMutex for thread-safe plugin registry access:
//   - Read lock: GetPlugin, ListPlugins (concurrent reads allowed)
//   - Write lock: LoadPlugin, UnloadPlugin (exclusive access)
//   - Event emission: Read lock + goroutines (non-blocking)
//
// Performance Characteristics:
//
//   - Discovery: O(n) filesystem scan + O(m) built-in enumeration
//   - Database query: Single SELECT with indexed enabled column
//   - Plugin loading: Sequential, ~100-500ms per plugin (OnLoad hook latency)
//   - Event emission: O(n) plugins, each in separate goroutine (non-blocking)
//   - Shutdown: Sequential unload, ~50-200ms per plugin (OnUnload hook latency)
//
// Example Usage:
//
//	// Create runtime with plugin directories
//	runtime := NewRuntimeV2(database, "/opt/plugins", "/usr/local/plugins")
//
//	// Optional: Disable auto-start for development
//	runtime.SetAutoStart(false)
//
//	// Optional: Register built-in plugins
//	runtime.RegisterBuiltinPlugin("analytics", &AnalyticsPlugin{})
//
//	// Start runtime (auto-loads enabled plugins from database)
//	if err := runtime.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Emit events during platform operation
//	runtime.EmitEvent("session.created", sessionData)
//
//	// Graceful shutdown
//	defer runtime.Stop(ctx)
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

// RuntimeV2 manages the lifecycle and execution of plugins with automatic discovery.
//
// This is the central orchestrator for the entire plugin system, responsible for:
//   - Discovering available plugins (filesystem + built-in)
//   - Loading enabled plugins from database on startup
//   - Managing plugin lifecycle (load, enable, disable, unload)
//   - Broadcasting events to all loaded plugins
//   - Providing centralized registries (API, UI, Events, Scheduler)
//
// Thread Safety: All methods are thread-safe via sync.RWMutex protection.
type RuntimeV2 struct {
	// db is the database connection for querying installed/enabled plugins.
	// Used to load plugin list and configuration on startup.
	db *db.Database

	// discovery handles plugin discovery from filesystem and built-in registry.
	// Scans plugin directories for .so files and enumerates built-in plugins.
	discovery *PluginDiscovery

	// plugins is the registry of currently loaded plugins.
	// Map key is plugin name, value is loaded plugin instance.
	// Protected by pluginsMux for thread-safe access.
	plugins map[string]*LoadedPlugin

	// pluginsMux protects concurrent access to the plugins map.
	// Read lock: GetPlugin, ListPlugins, EmitEvent
	// Write lock: LoadPlugin, UnloadPlugin
	pluginsMux sync.RWMutex

	// eventBus is the centralized event system for inter-plugin communication.
	// Plugins can subscribe to events via ctx.Events.Subscribe(eventType, handler).
	eventBus *EventBus

	// scheduler is the cron scheduler for time-based plugin jobs.
	// Plugins can schedule jobs via ctx.Scheduler.Schedule(spec, func).
	scheduler *cron.Cron

	// apiRegistry is the centralized HTTP API endpoint registry.
	// Plugins register endpoints via ctx.API.RegisterEndpoint(opts).
	apiRegistry *APIRegistry

	// uiRegistry is the centralized UI component registry.
	// Plugins register UI components via ctx.UI.RegisterWidget/Page/etc.
	uiRegistry *UIRegistry

	// autoStart controls whether plugins are auto-loaded on Start().
	// If true: Loads all enabled plugins from database on startup.
	// If false: Plugins must be loaded manually via LoadPlugin API.
	autoStart bool
}

// NewRuntimeV2 creates a new plugin runtime with automatic discovery.
//
// Parameters:
//   - database: Database connection for loading installed plugins
//   - pluginDirs: Optional directories to scan for dynamic plugins (.so files)
//
// Returns a new RuntimeV2 instance with:
//   - Auto-start enabled by default (loads plugins from database on Start())
//   - Empty plugin registry (no plugins loaded yet)
//   - Initialized event bus, scheduler, and registries
//
// Example:
//
//	// Create runtime with custom plugin directories
//	runtime := NewRuntimeV2(db, "/opt/plugins", "/usr/local/plugins")
//
//	// Create runtime without plugin directories (built-in plugins only)
//	runtime := NewRuntimeV2(db)
//
// Thread Safety: Constructor is not thread-safe. Do not call concurrently.
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

// SetAutoStart enables/disables automatic plugin loading on Start().
//
// When auto-start is enabled (default):
//   - Start() queries database for enabled plugins
//   - Loads each enabled plugin automatically
//   - Best for production deployments
//
// When auto-start is disabled:
//   - Start() only initializes the runtime (no plugin loading)
//   - Plugins must be loaded manually via LoadPlugin API
//   - Best for development, testing, or dynamic loading scenarios
//
// Parameters:
//   - enabled: true to enable auto-start, false to disable
//
// Example:
//
//	// Disable auto-start for testing
//	runtime := NewRuntimeV2(db)
//	runtime.SetAutoStart(false)
//	runtime.Start(ctx)  // No plugins loaded
//
//	// Manually load specific plugin
//	runtime.LoadPluginWithConfig(ctx, "test-plugin", "1.0.0", config, manifest)
//
// Thread Safety: Not thread-safe. Call before Start().
func (r *RuntimeV2) SetAutoStart(enabled bool) {
	r.autoStart = enabled
}

// RegisterBuiltinPlugin registers a built-in plugin for automatic discovery.
//
// Built-in plugins are compiled into the API binary and don't require
// external .so files. This is typically called during package init():
//
//	func init() {
//	    plugins.RegisterBuiltinPlugin("analytics", NewAnalyticsPlugin)
//	}
//
// Parameters:
//   - name: Plugin identifier (must be unique)
//   - factory: Function that creates new plugin instances
//
// The plugin becomes available for loading but is not automatically loaded.
// To load the plugin, either:
//   1. Enable it in database (for auto-start mode)
//   2. Call LoadPluginWithConfig manually
//
// Example:
//
//	// Define plugin factory
//	func NewAnalyticsPlugin() PluginHandler {
//	    return &AnalyticsPlugin{}
//	}
//
//	// Register as built-in
//	runtime.RegisterBuiltinPlugin("analytics", NewAnalyticsPlugin)
//
//	// Plugin is now discoverable
//	available := runtime.ListAvailablePlugins()  // Contains "analytics"
//
// Thread Safety: Not thread-safe. Call before Start().
func (r *RuntimeV2) RegisterBuiltinPlugin(name string, factory PluginFactory) {
	r.discovery.RegisterBuiltin(name, factory)
}

// Start initializes the plugin runtime and auto-loads enabled plugins.
//
// Startup sequence:
//  1. Start the cron scheduler (for plugin scheduled jobs)
//  2. Discover all available plugins (filesystem + built-in)
//  3. If auto-start is enabled: Load all enabled plugins from database
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns:
//   - nil on success
//   - error if plugin discovery or loading fails critically
//
// Behavior:
//   - Plugin discovery errors are logged as warnings but don't fail startup
//   - Individual plugin loading errors are logged but don't fail startup
//   - Only critical errors (database connection, etc.) return error
//
// Example:
//
//	runtime := NewRuntimeV2(db, "/opt/plugins")
//
//	// Start with timeout
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	if err := runtime.Start(ctx); err != nil {
//	    log.Fatalf("Failed to start plugin runtime: %v", err)
//	}
//
//	log.Printf("Plugin runtime started, %d plugins loaded", len(runtime.ListPlugins()))
//
// Thread Safety: Safe to call concurrently, but typically called once at startup.
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

// loadEnabledPlugins loads all enabled plugins from the database.
//
// This is an internal method called by Start() when auto-start is enabled.
//
// Database Query:
//   - SELECT * FROM installed_plugins WHERE enabled = true
//   - For each plugin, also loads manifest from catalog_plugins table
//   - Handles missing catalog gracefully (plugin loads without manifest)
//
// Parameters:
//   - ctx: Context for query cancellation
//
// Returns:
//   - Number of successfully loaded plugins
//   - Error only on critical database failures
//
// Error Handling:
//   - Individual plugin loading errors are logged but don't fail the method
//   - Config parsing errors result in empty config (plugin still loads)
//   - Missing manifest is logged as warning (plugin still loads)
//
// Thread Safety: Not thread-safe. Called by Start() which manages locking.
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

// LoadPluginByName loads a single plugin from the database by its name.
//
// This method is useful for enabling a plugin at runtime after it was previously
// disabled. It queries the database for the plugin's configuration and manifest,
// then loads it into the runtime.
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Plugin name to load
//
// Returns:
//   - nil on success
//   - error if plugin not found in database or loading fails
//
// Thread Safety: Thread-safe via internal LoadPluginWithConfig locking.
func (r *RuntimeV2) LoadPluginByName(ctx context.Context, name string) error {
	// Query plugin from database
	var plugin models.InstalledPlugin
	var catalogID sql.NullInt64
	var configJSON []byte

	err := r.db.DB().QueryRowContext(ctx, `
		SELECT id, name, version, enabled, config, catalog_plugin_id
		FROM installed_plugins
		WHERE name = $1
	`, name).Scan(
		&plugin.ID,
		&plugin.Name,
		&plugin.Version,
		&plugin.Enabled,
		&configJSON,
		&catalogID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("plugin %s not found in database", name)
		}
		return fmt.Errorf("failed to query plugin %s: %w", name, err)
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
	return r.LoadPluginWithConfig(ctx, plugin.Name, plugin.Version, config, manifest)
}

// ReloadPlugin unloads and reloads a plugin with updated configuration.
//
// This is useful for applying configuration changes without restarting the API.
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Plugin name to reload
//
// Returns:
//   - nil on success
//   - error if unload or load fails
//
// Thread Safety: Thread-safe via internal locking.
func (r *RuntimeV2) ReloadPlugin(ctx context.Context, name string) error {
	// Unload if currently loaded
	if err := r.UnloadPlugin(ctx, name); err != nil {
		// Log but continue - plugin might not be loaded
		log.Printf("[Plugin Runtime] Note: Could not unload %s before reload: %v", name, err)
	}

	// Load with fresh config from database
	return r.LoadPluginByName(ctx, name)
}

// LoadPluginWithConfig loads and initializes a plugin with specific configuration.
//
// This is the core plugin loading method that:
//  1. Checks if plugin is already loaded (prevents duplicates)
//  2. Loads plugin handler via discovery system
//  3. Creates PluginContext with all helper components
//  4. Calls plugin's OnLoad lifecycle hook
//  5. Registers plugin in runtime registry
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Plugin identifier (must be discoverable)
//   - version: Plugin version string (for tracking/display)
//   - config: Plugin-specific configuration map
//   - manifest: Plugin manifest with metadata and permissions
//
// Returns:
//   - nil on success
//   - error if plugin already loaded, not found, or OnLoad fails
//
// Plugin Context Components:
//
// Each plugin receives a PluginContext with access to:
//   - Database: Namespaced table access (plugin_name_*)
//   - Events: Pub/sub event system
//   - API: HTTP endpoint registration (/api/plugins/{name}/*)
//   - UI: Component registration (widgets, pages, menus)
//   - Storage: Key-value storage
//   - Logger: Structured JSON logging
//   - Scheduler: Cron job scheduling
//
// Example:
//
//	config := map[string]interface{}{
//	    "api_key": "secret-key",
//	    "webhook_url": "https://hooks.slack.com/...",
//	}
//
//	err := runtime.LoadPluginWithConfig(ctx, "slack-notifications", "1.0.0", config, manifest)
//	if err != nil {
//	    log.Fatalf("Failed to load plugin: %v", err)
//	}
//
//	// Plugin is now active and can receive events
//	runtime.EmitEvent("session.created", sessionData)
//
// Thread Safety: Thread-safe via write lock (blocks other loads/unloads).
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

// Stop gracefully shuts down the plugin runtime.
//
// Shutdown sequence:
//  1. Unload all loaded plugins (calls OnUnload hooks)
//  2. Remove all scheduled jobs from cron scheduler
//  3. Unregister all API endpoints
//  4. Unregister all UI components
//  5. Remove all event subscriptions
//  6. Stop the cron scheduler (waits for running jobs)
//
// Parameters:
//   - ctx: Context for cancellation (currently not used, reserved for future)
//
// Returns:
//   - Always returns nil (errors are logged, not returned)
//
// Behavior:
//   - Individual plugin OnUnload errors are logged but don't stop shutdown
//   - All plugins are unloaded even if some fail
//   - Scheduler waits for all running jobs to complete
//
// Example:
//
//	runtime := NewRuntimeV2(db)
//	runtime.Start(ctx)
//
//	// On shutdown (e.g., SIGTERM handler)
//	defer runtime.Stop(context.Background())
//
//	// Or with timeout
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	runtime.Stop(ctx)
//
// Thread Safety: Thread-safe via write lock.
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

// UnloadPlugin unloads a specific plugin.
//
// This removes the plugin from the runtime and cleans up all its resources:
//   - Calls plugin's OnUnload lifecycle hook
//   - Removes all scheduled cron jobs
//   - Unregisters all HTTP API endpoints
//   - Unregisters all UI components
//   - Removes all event subscriptions
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Plugin name to unload
//
// Returns:
//   - nil on success
//   - error if plugin is not loaded
//
// Example:
//
//	// Unload a plugin manually
//	if err := runtime.UnloadPlugin(ctx, "slack-notifications"); err != nil {
//	    log.Printf("Failed to unload plugin: %v", err)
//	}
//
//	// Plugin is now unloaded and won't receive events
//	runtime.EmitEvent("session.created", data)  // slack-notifications won't see this
//
// Thread Safety: Thread-safe via write lock.
func (r *RuntimeV2) UnloadPlugin(ctx context.Context, name string) error {
	r.pluginsMux.Lock()
	defer r.pluginsMux.Unlock()

	return r.unloadPluginLocked(ctx, name)
}

// unloadPluginLocked is the internal unload implementation.
//
// Called by UnloadPlugin and Stop with lock already held.
// This avoids deadlock from nested locking.
//
// Thread Safety: NOT thread-safe. Caller must hold r.pluginsMux write lock.
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

// EmitEvent emits an event to all listening plugins.
//
// This is the core event distribution mechanism that broadcasts platform
// events to all loaded and enabled plugins. Each plugin receives the event
// via its corresponding lifecycle hook method.
//
// Parameters:
//   - eventType: Event type identifier (e.g., "session.created", "user.login")
//   - data: Event payload (typically a session or user struct)
//
// Event Types and Hooks:
//
//	Session Events:
//	  - "session.created" → OnSessionCreated(ctx, session)
//	  - "session.started" → OnSessionStarted(ctx, session)
//	  - "session.stopped" → OnSessionStopped(ctx, session)
//	  - "session.hibernated" → OnSessionHibernated(ctx, session)
//	  - "session.woken" → OnSessionWoken(ctx, session)
//	  - "session.deleted" → OnSessionDeleted(ctx, session)
//
//	User Events:
//	  - "user.created" → OnUserCreated(ctx, user)
//	  - "user.updated" → OnUserUpdated(ctx, user)
//	  - "user.deleted" → OnUserDeleted(ctx, user)
//	  - "user.login" → OnUserLogin(ctx, user)
//	  - "user.logout" → OnUserLogout(ctx, user)
//
// Behavior:
//   - Only enabled plugins receive events (plugin.Enabled == true)
//   - Each plugin runs in a separate goroutine (non-blocking)
//   - Plugin panics are recovered and logged (don't crash runtime)
//   - Plugin hook errors are logged but don't stop other plugins
//   - Events are also emitted to event bus for custom subscriptions
//
// Example:
//
//	// In API handler after session creation
//	session := &models.Session{
//	    ID:       1,
//	    UserID:   "user123",
//	    Name:     "firefox-session",
//	}
//
//	// Emit event to all plugins
//	runtime.EmitEvent("session.created", session)
//
//	// Each plugin's OnSessionCreated hook is called:
//	// - Slack plugin sends notification
//	// - Analytics plugin tracks usage
//	// - Audit plugin logs creation
//
// Performance:
//   - O(n) where n = number of loaded, enabled plugins
//   - Non-blocking: Plugins run in parallel goroutines
//   - No timeout: Long-running plugin hooks don't block other plugins
//
// Thread Safety: Thread-safe via read lock (allows concurrent event emission).
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

// GetPlugin retrieves a loaded plugin by name.
//
// Returns the LoadedPlugin struct containing:
//   - Name, Version, Enabled status
//   - Plugin configuration and manifest
//   - Plugin handler and instance
//   - LoadedAt timestamp, IsBuiltin flag
//
// Parameters:
//   - name: Plugin identifier
//
// Returns:
//   - Loaded plugin on success
//   - Error if plugin is not loaded
//
// Example:
//
//	plugin, err := runtime.GetPlugin("slack-notifications")
//	if err != nil {
//	    log.Printf("Plugin not loaded: %v", err)
//	    return
//	}
//
//	log.Printf("Plugin: %s v%s (loaded at %v)", plugin.Name, plugin.Version, plugin.LoadedAt)
//	log.Printf("Builtin: %v, Enabled: %v", plugin.IsBuiltin, plugin.Enabled)
//
// Thread Safety: Thread-safe via read lock.
func (r *RuntimeV2) GetPlugin(name string) (*LoadedPlugin, error) {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugin, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not loaded", name)
	}

	return plugin, nil
}

// ListPlugins returns all currently loaded plugins.
//
// Returns a slice of LoadedPlugin structs, one for each loaded plugin.
// The order is non-deterministic (map iteration order).
//
// Returns:
//   - Slice of all loaded plugins (empty slice if none loaded)
//
// Example:
//
//	plugins := runtime.ListPlugins()
//	log.Printf("Loaded %d plugins:", len(plugins))
//
//	for _, p := range plugins {
//	    status := "disabled"
//	    if p.Enabled {
//	        status = "enabled"
//	    }
//	    log.Printf("  - %s v%s (%s)", p.Name, p.Version, status)
//	}
//
// Use Cases:
//   - Admin UI plugin list page
//   - Status endpoints (GET /api/plugins/loaded)
//   - Metrics collection (number of loaded plugins)
//
// Thread Safety: Thread-safe via read lock.
func (r *RuntimeV2) ListPlugins() []*LoadedPlugin {
	r.pluginsMux.RLock()
	defer r.pluginsMux.RUnlock()

	plugins := make([]*LoadedPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// ListAvailablePlugins returns names of all discoverable plugins.
//
// This includes both loaded and unloaded plugins:
//   - Built-in plugins (registered via RegisterBuiltinPlugin)
//   - Dynamic plugins (discovered from plugin directories)
//
// Returns:
//   - Slice of plugin names (empty if discovery fails)
//
// Example:
//
//	available := runtime.ListAvailablePlugins()
//	loaded := runtime.ListPlugins()
//
//	log.Printf("Available: %d, Loaded: %d", len(available), len(loaded))
//
//	// Show unloaded plugins
//	loadedMap := make(map[string]bool)
//	for _, p := range loaded {
//	    loadedMap[p.Name] = true
//	}
//
//	for _, name := range available {
//	    if !loadedMap[name] {
//	        log.Printf("Available but not loaded: %s", name)
//	    }
//	}
//
// Use Cases:
//   - Plugin catalog page (show all installable plugins)
//   - Discovery endpoint (GET /api/plugins/available)
//
// Thread Safety: Thread-safe (discovery has internal locking).
func (r *RuntimeV2) ListAvailablePlugins() []string {
	plugins, _ := r.discovery.DiscoverAll()
	return plugins
}

// GetEventBus returns the event bus for direct access.
//
// This allows external code to:
//   - Subscribe to events (bus.Subscribe(eventType, handler))
//   - Emit custom events (bus.Emit(eventType, data))
//
// Use Cases:
//   - Plugin code subscribing to custom events
//   - Testing event emission
//
// Example:
//
//	bus := runtime.GetEventBus()
//
//	// Subscribe to custom event
//	bus.Subscribe("custom.event", func(data interface{}) {
//	    log.Printf("Received custom event: %v", data)
//	})
//
//	// Emit custom event
//	bus.Emit("custom.event", map[string]string{"key": "value"})
//
// Thread Safety: EventBus has internal locking.
func (r *RuntimeV2) GetEventBus() *EventBus {
	return r.eventBus
}

// GetAPIRegistry returns the API registry for direct access.
//
// This allows external code to:
//   - Enumerate registered endpoints (registry.GetAll())
//   - Mount endpoints to Gin router (for each endpoint)
//
// Primary Use Case: HTTP server initialization.
//
// Example:
//
//	registry := runtime.GetAPIRegistry()
//	endpoints := registry.GetAll()
//
//	for _, endpoint := range endpoints {
//	    log.Printf("Plugin %s registered: %s %s", endpoint.PluginName, endpoint.Method, endpoint.Path)
//	}
//
// Thread Safety: APIRegistry has internal locking.
func (r *RuntimeV2) GetAPIRegistry() *APIRegistry {
	return r.apiRegistry
}

// GetUIRegistry returns the UI registry for direct access.
//
// This allows external code to:
//   - Enumerate registered UI components (registry.GetWidgets(), etc.)
//   - Serialize component definitions for frontend
//
// Primary Use Case: UI component manifest endpoint.
//
// Example:
//
//	registry := runtime.GetUIRegistry()
//	widgets := registry.GetWidgets()
//
//	// Send to frontend
//	for _, widget := range widgets {
//	    log.Printf("Widget: %s (component: %s)", widget.Title, widget.Component)
//	}
//
// Thread Safety: UIRegistry has internal locking.
func (r *RuntimeV2) GetUIRegistry() *UIRegistry {
	return r.uiRegistry
}
