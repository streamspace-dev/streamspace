# StreamSpace Plugin Integration Guide

**Automatic Plugin System - Zero Code Changes Required**

This guide explains how StreamSpace's fully automatic plugin system works, requiring **zero manual code changes** for users.

---

## Overview

StreamSpace's plugin system is designed to be **completely automatic**:

1. ✅ **Auto-discovery** - Plugins are discovered from filesystem and marketplace
2. ✅ **Database-driven** - Enable/disable via UI or API, not code
3. ✅ **Automatic loading** - Enabled plugins load on startup
4. ✅ **Hot reload** - Install/uninstall without restart
5. ✅ **Marketplace integration** - Download and activate from GitHub

**Users never need to modify code or restart the server.**

---

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────┐
│                     StreamSpace Core                    │
│                                                         │
│  ┌──────────────────────────────────────────────────┐  │
│  │         Plugin Runtime V2 (Automatic)           │  │
│  │                                                  │  │
│  │  ┌────────────┐  ┌──────────────┐  ┌──────────┐│  │
│  │  │  Discovery │  │  Marketplace │  │ Registry ││  │
│  │  └────────────┘  └──────────────┘  └──────────┘│  │
│  └──────────────────────────────────────────────────┘  │
│                                                         │
│  ┌──────────────────────────────────────────────────┐  │
│  │              Event Bus                           │  │
│  │  (Distributes events to loaded plugins)          │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                           ↓
        ┌──────────────────────────────────────┐
        │         Loaded Plugins              │
        │                                     │
        │  • Slack  • Teams  • Billing       │
        │  • Discord • Compliance • Nodes    │
        └──────────────────────────────────────┘
```

### Plugin Discovery Sources

1. **Built-in Plugins** (compiled into binary)
   - Auto-registered via `init()` functions
   - No dynamic loading needed
   - Example: Core integration plugins

2. **Filesystem Plugins** (`.so` shared libraries)
   - Scanned from `/plugins` directory
   - Dynamically loaded at runtime
   - Hot-reload capable

3. **Marketplace Plugins** (downloaded from GitHub)
   - Fetched from `streamspace-plugins` repository
   - Downloaded and extracted automatically
   - Cached locally

---

## For Developers: Adding to StreamSpace Core

### One-Time Setup in `main.go`

Add this **once** to your `api/cmd/main.go`:

```go
package main

import (
	"context"
	"log"

	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/handlers"
	"github.com/streamspace/streamspace/api/internal/plugins"

	// Import plugin packages for auto-registration
	// Each plugin's init() will call plugins.Register()
	_ "github.com/streamspace/streamspace/plugins/streamspace-slack"
	// Add more built-in plugins here if desired
)

func main() {
	// ... existing setup code ...

	// Initialize database
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize plugin runtime with automatic discovery
	pluginRuntime := plugins.NewRuntimeV2(database)

	// Apply globally registered plugins (from init() functions)
	plugins.GetGlobalRegistry().ApplyToDiscovery(pluginRuntime.discovery)

	// Start plugin runtime (auto-loads enabled plugins from database)
	if err := pluginRuntime.Start(context.Background()); err != nil {
		log.Printf("Warning: Plugin runtime failed to start: %v", err)
		// Don't fatal - continue without plugins
	}
	defer pluginRuntime.Stop(context.Background())

	// Initialize plugin marketplace
	marketplace := plugins.NewPluginMarketplace(
		database,
		"https://raw.githubusercontent.com/JoshuaAFerguson/streamspace-plugins/main",
		"/plugins",
	)

	// Register API handlers
	router := gin.Default()
	api := router.Group("/api")

	// Existing handlers...
	sessionHandler := handlers.NewSessionHandler(database)
	sessionHandler.RegisterRoutes(api)

	// Plugin marketplace handler
	marketplaceHandler := handlers.NewPluginMarketplaceHandler(database, marketplace, pluginRuntime)
	marketplaceHandler.RegisterRoutes(api)

	// Attach plugin API endpoints (registered by plugins)
	pluginRuntime.GetAPIRegistry().AttachToRouter(api)

	// ... rest of setup ...

	// Emit events throughout your application:
	// When session created:
	pluginRuntime.EmitEvent("session.created", sessionData)

	// When user logs in:
	pluginRuntime.EmitEvent("user.login", userData)

	// etc.
}
```

### Emitting Events

Throughout your codebase, emit events for plugins to consume:

```go
// In session handler when session is created:
func (h *SessionHandler) CreateSession(c *gin.Context) {
	// ... create session ...

	// Emit event to plugins
	h.pluginRuntime.EmitEvent("session.created", session)
}

// In user handler when user logs in:
func (h *UserHandler) Login(c *gin.Context) {
	// ... authenticate user ...

	// Emit event to plugins
	h.pluginRuntime.EmitEvent("user.login", user)
}
```

**That's it!** No other code changes needed.

---

## For Users: Using the Plugin System

Users interact with plugins entirely through the UI or API - **no code changes required**.

### Via Web UI

1. **Browse Plugins**
   - Navigate to **Admin** → **Plugins** → **Marketplace**
   - View available plugins from the catalog
   - See descriptions, ratings, and screenshots

2. **Install a Plugin**
   - Click **Install** on desired plugin
   - Configure settings in the modal
   - Click **Activate**
   - Plugin downloads, installs, and activates automatically

3. **Configure Plugin**
   - Navigate to **Admin** → **Plugins** → **Installed**
   - Click plugin name
   - Modify configuration
   - Save (takes effect immediately)

4. **Enable/Disable**
   - Toggle switch on plugin card
   - Plugin loads/unloads immediately
   - No restart required

5. **Uninstall**
   - Click **Uninstall** button
   - Confirm removal
   - Plugin removed from system

### Via API

#### List Available Plugins

```bash
GET /api/plugins/marketplace/catalog

Response:
{
  "plugins": [
    {
      "name": "streamspace-slack",
      "displayName": "Slack Integration",
      "description": "Send notifications to Slack",
      "category": "Integrations",
      "version": "1.0.0",
      "installed": false,
      "enabled": false
    },
    ...
  ],
  "count": 12
}
```

#### Install Plugin

```bash
POST /api/plugins/marketplace/install/streamspace-slack
Content-Type: application/json

{
  "config": {
    "webhookUrl": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
    "channel": "#general",
    "notifyOnSessionCreated": true
  }
}

Response:
{
  "message": "Plugin installed and activated successfully",
  "plugin": { ... }
}
```

#### Enable/Disable Plugin

```bash
POST /api/plugins/marketplace/enable/streamspace-slack
POST /api/plugins/marketplace/disable/streamspace-slack
```

#### Uninstall Plugin

```bash
DELETE /api/plugins/marketplace/uninstall/streamspace-slack
```

---

## For Plugin Developers

### Creating a Plugin

Plugins auto-register themselves via `init()` function:

```go
package myplugin

import "github.com/streamspace/streamspace/api/internal/plugins"

type MyPlugin struct {
	plugins.BasePlugin
}

func NewMyPlugin() *MyPlugin {
	return &MyPlugin{
		BasePlugin: plugins.BasePlugin{Name: "my-plugin"},
	}
}

func (p *MyPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("My plugin loaded!")

	// Register API endpoints
	ctx.API.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Hello from plugin!"})
	})

	return nil
}

func (p *MyPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	ctx.Logger.Info("Session created!", map[string]interface{}{
		"session": session,
	})
	return nil
}

// AUTO-REGISTER on import
func init() {
	plugins.Register("my-plugin", func() plugins.PluginHandler {
		return NewMyPlugin()
	})
}
```

### Publishing to Marketplace

1. **Create plugin directory**:
   ```
   streamspace-plugins/
   └── my-plugin/
       ├── manifest.json
       ├── my_plugin.go
       ├── README.md
       └── icon.png
   ```

2. **Add to catalog.json**:
   ```json
   {
     "name": "my-plugin",
     "version": "1.0.0",
     "displayName": "My Plugin",
     "description": "Does amazing things",
     "author": "Your Name",
     "category": "Utilities",
     "tags": ["utility", "automation"],
     "downloadUrl": "https://github.com/JoshuaAFerguson/streamspace-plugins/raw/main/my-plugin/plugin.tar.gz",
     "manifest": { ... }
   }
   ```

3. **Create release archive**:
   ```bash
   cd my-plugin
   tar -czf plugin.tar.gz manifest.json my_plugin.go README.md icon.png
   ```

4. **Commit to repository**:
   ```bash
   git add my-plugin/ catalog.json
   git commit -m "Add my-plugin"
   git push
   ```

**Done!** Plugin is now available in the marketplace.

---

## Database Schema

### Tables Used by Plugin System

```sql
-- Installed plugins
CREATE TABLE installed_plugins (
    id SERIAL PRIMARY KEY,
    catalog_plugin_id INTEGER REFERENCES catalog_plugins(id),
    name TEXT UNIQUE NOT NULL,
    version TEXT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    config JSONB,
    installed_by TEXT,
    installed_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Plugin catalog (synced from marketplace)
CREATE TABLE catalog_plugins (
    id SERIAL PRIMARY KEY,
    repository_id INTEGER REFERENCES catalog_repositories(id),
    name TEXT UNIQUE NOT NULL,
    version TEXT NOT NULL,
    display_name TEXT,
    description TEXT,
    category TEXT,
    plugin_type TEXT,
    icon_url TEXT,
    manifest JSONB,
    tags TEXT[],
    install_count INTEGER DEFAULT 0,
    avg_rating NUMERIC(3,2),
    rating_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Plugin storage (key-value for plugin data)
CREATE TABLE plugin_storage (
    plugin_name TEXT NOT NULL,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (plugin_name, key)
);
```

---

## Event Reference

### Available Events

Plugins can subscribe to these events:

- `session.created` - New session created
- `session.started` - Session started
- `session.stopped` - Session stopped
- `session.hibernated` - Session hibernated
- `session.woken` - Session woken from hibernation
- `session.deleted` - Session deleted
- `user.created` - New user created
- `user.updated` - User updated
- `user.deleted` - User deleted
- `user.login` - User logged in
- `user.logout` - User logged out

### Custom Events

Plugins can also emit custom events:

```go
// In plugin:
ctx.Events.Emit("custom-event", data)

// Other plugins can listen:
ctx.Events.On("plugin.my-plugin.custom-event", func(data interface{}) error {
	// Handle event
	return nil
})
```

---

## Configuration Management

### Plugin Configuration Schema

Plugins define configuration via JSON Schema in `manifest.json`:

```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "apiKey": {
        "type": "string",
        "title": "API Key",
        "description": "Your service API key",
        "sensitive": true
      },
      "enabled": {
        "type": "boolean",
        "title": "Enable Notifications",
        "default": true
      }
    },
    "required": ["apiKey"]
  }
}
```

### Runtime Configuration Access

```go
func (p *MyPlugin) OnLoad(ctx *PluginContext) error {
	apiKey := ctx.Config["apiKey"].(string)
	enabled := ctx.Config["enabled"].(bool)

	// Use configuration...
}
```

---

## Security

### Plugin Sandboxing

- Plugins run in same process but have controlled API access
- Permissions declared in `manifest.json`
- Network access gated by `network` permission
- Database access scoped to plugin-specific tables
- No direct filesystem access (use Storage API)

### Permissions

```json
{
  "permissions": [
    "network",        // HTTP requests
    "read:sessions",  // Read session data
    "write:sessions", // Modify sessions
    "admin"          // Admin-level access
  ]
}
```

---

## Troubleshooting

### Plugin Not Loading

1. Check plugin is enabled in database:
   ```sql
   SELECT name, enabled FROM installed_plugins;
   ```

2. Check logs for error messages:
   ```bash
   kubectl logs -n streamspace deploy/streamspace-api | grep "Plugin"
   ```

3. Verify plugin directory exists and has correct permissions:
   ```bash
   ls -la /plugins/streamspace-slack/
   ```

### Plugin Crashes

Plugins are isolated - if one crashes, it won't affect others:

```
[Plugin Runtime] Plugin slack panicked on event session.created: ...
```

The runtime will continue operating.

### Marketplace Sync Issues

Force sync catalog:

```bash
POST /api/plugins/marketplace/sync
```

---

## Performance

- **Startup Impact**: ~50-100ms per plugin
- **Event Overhead**: Events processed async, no blocking
- **Memory**: ~10-20MB per plugin average
- **CPU**: Minimal (<1% per plugin at idle)

---

## Summary

StreamSpace's plugin system is **fully automatic**:

✅ **For users**: Install plugins via UI, no code changes
✅ **For developers**: Add 10 lines to `main.go`, done
✅ **For plugin authors**: Write plugin, commit to repo, available in marketplace

**Zero manual intervention, maximum flexibility.**
