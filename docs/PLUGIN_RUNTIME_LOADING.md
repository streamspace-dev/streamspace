# Plugin Runtime Loading Guide

> **Status**: Implementation Complete
> **Version**: 1.1.0
> **Last Updated**: 2025-11-19

---

## Overview

This guide documents the plugin runtime loading system that allows plugins to be dynamically loaded from disk after StreamSpace has started. This is a critical feature for production deployments where plugins need to be installed without restarting the API server.

## Table of Contents

- [How Runtime Loading Works](#how-runtime-loading-works)
- [Loading Plugins](#loading-plugins)
- [Plugin Discovery](#plugin-discovery)
- [Configuration Management](#configuration-management)
- [Hot Reloading](#hot-reloading)
- [Error Handling](#error-handling)
- [Troubleshooting](#troubleshooting)

---

## How Runtime Loading Works

### Architecture

```
┌─────────────────────────────────────────────────┐
│  StreamSpace API Server                         │
│                                                 │
│  ┌─────────────────┐    ┌──────────────────┐   │
│  │ Plugin Manager  │◄───│ Plugin Registry  │   │
│  └────────┬────────┘    └──────────────────┘   │
│           │                                     │
│           ▼                                     │
│  ┌─────────────────┐                           │
│  │ Runtime Loader  │                           │
│  └────────┬────────┘                           │
│           │                                     │
└───────────┼─────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────┐
│  Plugin Directory (/var/lib/streamspace/plugins)│
│                                                 │
│  ├── plugin-a/                                  │
│  │   ├── manifest.json                          │
│  │   └── plugin-a.so                            │
│  ├── plugin-b/                                  │
│  │   ├── manifest.json                          │
│  │   └── plugin-b.so                            │
│  └── ...                                        │
└─────────────────────────────────────────────────┘
```

### Loading Process

StreamSpace uses Go's native plugin system for runtime loading:

1. **Discovery**: Scanner detects new plugin directory with `.so` file
2. **Validation**: Manifest and shared object validated
3. **Loading**: Plugin opened using `plugin.Open()`
4. **Symbol Lookup**: `Handler` symbol located and type-checked
5. **Initialization**: Plugin's `OnLoad()` method called

### Implementation

The `LoadHandler()` function uses Go's plugin package:

```go
func (r *Runtime) LoadHandler(name string) (PluginHandler, error) {
    pluginPath := filepath.Join(r.pluginDir, name, name+".so")

    // Open the plugin
    p, err := plugin.Open(pluginPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open plugin %s: %w", name, err)
    }

    // Look up the Handler symbol
    sym, err := p.Lookup("Handler")
    if err != nil {
        return nil, fmt.Errorf("plugin %s missing Handler: %w", name, err)
    }

    // Assert to PluginHandler interface
    handler, ok := sym.(PluginHandler)
    if !ok {
        return nil, fmt.Errorf("plugin %s Handler has wrong type", name)
    }

    return handler, nil
}
```

### Design Rationale

- **Native Go performance**: No interpreter overhead
- **Type-safe interfaces**: Compile-time checking of plugin contracts
- **Standard mechanism**: Uses Go's built-in plugin package
- **Alternative rejected**: Yaegi interpreter was considered but rejected due to performance and security concerns

---

## Loading Plugins

### From Disk

<!-- TODO: Add code examples after implementation -->

**API Endpoint**: `POST /api/v1/plugins/{pluginId}/load`

```bash
# Load a plugin from disk
curl -X POST https://streamspace.example.com/api/v1/plugins/my-plugin/load \
  -H "Authorization: Bearer $TOKEN"
```

**Expected Response**:
```json
{
  "status": "loaded",
  "plugin": {
    "id": "my-plugin",
    "version": "1.0.0",
    "loadedAt": "2025-11-19T10:30:00Z"
  }
}
```

### From Archive

<!-- TODO: Document archive loading -->

```bash
# Upload and load plugin from tar.gz
curl -X POST https://streamspace.example.com/api/v1/plugins/install \
  -F "file=@my-plugin.tar.gz" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Plugin Discovery

### Automatic Discovery

<!-- TODO: Document file watcher implementation -->

The plugin manager monitors the plugin directory for changes:

- New directories trigger plugin discovery
- Modified files trigger reload
- Deleted directories trigger unload

**Configuration** (`values.yaml`):
```yaml
plugins:
  directory: /var/lib/streamspace/plugins
  autoDiscovery: true
  watchInterval: 30s
```

### Manual Discovery

```bash
# Trigger plugin discovery manually
curl -X POST https://streamspace.example.com/api/v1/plugins/discover \
  -H "Authorization: Bearer $TOKEN"
```

---

## Configuration Management

### Storing Configuration

<!-- TODO: Document after Builder implements UpdateConfiguration() -->

Plugin configurations are stored in the database and persisted across restarts.

**API Endpoint**: `PUT /api/v1/plugins/{pluginId}/config`

```bash
curl -X PUT https://streamspace.example.com/api/v1/plugins/my-plugin/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "apiKey": "sk-xxx",
    "enabled": true
  }'
```

### Configuration Schema Validation

Configurations are validated against the plugin's `configSchema` from manifest.json:

```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "apiKey": {
        "type": "string",
        "minLength": 10
      },
      "enabled": {
        "type": "boolean",
        "default": true
      }
    },
    "required": ["apiKey"]
  }
}
```

### Configuration Reload

When configuration changes:

1. Validate against schema
2. Update database
3. Call plugin's configuration handler (if defined)
4. Optionally reload plugin

**Reload on Config Change**:
```json
{
  "configSchema": {
    "reloadOnChange": true
  }
}
```

---

## Hot Reloading

### When to Use

Hot reloading allows plugins to be updated without restarting StreamSpace:

- Bug fixes
- Configuration changes
- Feature updates

### Reload Process

<!-- TODO: Document after implementation -->

```bash
# Reload a specific plugin
curl -X POST https://streamspace.example.com/api/v1/plugins/my-plugin/reload \
  -H "Authorization: Bearer $TOKEN"
```

### Graceful Reload

1. Call `onUnload()` on existing instance
2. Load new plugin version
3. Migrate state (if plugin supports it)
4. Call `onLoad()` on new instance
5. Re-register all handlers

---

## Error Handling

### Load Errors

| Error | Cause | Resolution |
|-------|-------|------------|
| `ManifestNotFound` | Missing manifest.json | Ensure manifest exists in plugin root |
| `InvalidManifest` | Malformed manifest | Validate JSON syntax |
| `EntrypointNotFound` | Missing main entry | Check entrypoints.main path |
| `LoadError` | JavaScript error | Check plugin code for syntax errors |
| `PermissionDenied` | Missing permissions | Update manifest permissions |

### Runtime Errors

<!-- TODO: Document error codes after implementation -->

```go
// Example error response
{
  "error": "PluginLoadError",
  "message": "Failed to load plugin: entrypoint not found",
  "details": {
    "pluginId": "my-plugin",
    "expectedPath": "index.js"
  }
}
```

---

## Troubleshooting

### Plugin Not Loading

1. **Check plugin directory permissions**
   ```bash
   ls -la /var/lib/streamspace/plugins/my-plugin
   ```

2. **Validate manifest.json**
   ```bash
   cat /var/lib/streamspace/plugins/my-plugin/manifest.json | jq .
   ```

3. **Check API logs**
   ```bash
   kubectl logs -n streamspace -l app=streamspace-api | grep "my-plugin"
   ```

4. **Test entrypoint**
   ```bash
   node /var/lib/streamspace/plugins/my-plugin/index.js
   ```

### Plugin Crashes on Load

<!-- TODO: Add specific error messages after implementation -->

1. Check for missing dependencies
2. Verify Node.js version compatibility
3. Check for syntax errors in plugin code

### Configuration Not Persisting

1. Verify database connection
2. Check plugin has `write:config` permission
3. Validate configuration against schema

---

## Implementation Status

> **IMPORTANT**: This documentation is an outline. The following sections require Builder implementation:

### Pending Implementation

- [ ] `LoadHandler()` in `/api/internal/plugins/runtime.go:1043`
- [ ] Configuration persistence in `UpdateConfiguration()`
- [ ] Plugin reload functionality
- [ ] File watcher for auto-discovery

### Acceptance Criteria

- Plugins load successfully from disk
- Configuration changes persist and reload plugins
- Errors are returned gracefully (no panics)
- Hot reload works without data loss

---

## Related Documentation

- [Plugin Development Guide](../PLUGIN_DEVELOPMENT.md)
- [Plugin API Reference](PLUGIN_API.md)
- [Plugin Manifest Schema](PLUGIN_MANIFEST.md)

---

*This document will be updated once the Builder completes the runtime loading implementation.*
