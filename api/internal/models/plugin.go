// Package models defines plugin-related data structures for the StreamSpace plugin system.
//
// The plugin system enables extending StreamSpace with:
//   - Templates from external repositories
//   - Plugins for additional functionality
//   - Catalog discovery and ratings
//   - Automatic synchronization from Git repositories
//
// Architecture:
//   - Repositories: External Git repos containing plugins and templates
//   - CatalogPlugin: Plugins available for installation from repositories
//   - InstalledPlugin: Plugins currently installed and running
//   - PluginManifest: Metadata and configuration schema for plugins
//
// Example workflow:
//  1. Add repository (streamspace-plugins GitHub repo)
//  2. Sync repository (clone/pull from Git, parse manifests)
//  3. Browse catalog (list available plugins with ratings)
//  4. Install plugin (copy to installed_plugins table, enable)
//  5. Configure plugin (set config via UI or API)
package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Repository represents an external Git repository containing plugins or templates.
//
// Repositories are:
//   - Added by platform administrators
//   - Synchronized periodically (default: every 1 hour)
//   - Can be enabled/disabled without deletion
//   - Support authentication for private repos
//
// Example repositories:
//   - https://github.com/JoshuaAFerguson/streamspace-plugins (official plugins)
//   - https://github.com/JoshuaAFerguson/streamspace-templates (official templates)
//
// Example:
//
//	{
//	  "id": 1,
//	  "name": "Official Plugins",
//	  "url": "https://github.com/JoshuaAFerguson/streamspace-plugins",
//	  "type": "git",
//	  "description": "Official StreamSpace plugin repository",
//	  "enabled": true
//	}
type Repository struct {
	// ID is a unique database identifier for this repository.
	ID int `json:"id"`

	// Name is a human-readable name for this repository.
	// Example: "Official Plugins", "Community Templates", "Enterprise Add-ons"
	Name string `json:"name"`

	// URL is the Git repository URL or HTTP endpoint.
	// Supported formats:
	//   - HTTPS Git: https://github.com/user/repo
	//   - SSH Git: git@github.com:user/repo.git
	//   - HTTP archive: https://example.com/plugins.zip (future)
	URL string `json:"url"`

	// Type indicates the repository source type.
	// Valid values:
	//   - "git": Git repository (GitHub, GitLab, Bitbucket, etc.)
	//   - "http": HTTP archive download (future)
	//
	// Default: "git"
	Type string `json:"type"`

	// Description provides information about this repository's contents.
	Description string `json:"description,omitempty"`

	// Enabled determines whether this repository should be synced.
	// When false:
	//   - Repository is not synced during scheduled sync
	//   - Plugins from this repo remain installed but won't update
	//   - Can be re-enabled without losing configuration
	Enabled bool `json:"enabled"`

	// CreatedAt is when this repository was added.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when repository metadata was last modified (not last sync).
	UpdatedAt time.Time `json:"updatedAt"`
}

// CatalogPlugin represents a plugin available for installation from a repository.
//
// Catalog plugins are:
//   - Discovered during repository sync
//   - Parsed from plugin.json manifest files
//   - Indexed with ratings and install statistics
//   - Searchable by name, tags, category
//
// Lifecycle:
//  1. Repository sync discovers new plugins
//  2. Manifests are parsed and validated
//  3. Plugins appear in catalog API/UI
//  4. Users can browse, rate, and install
//
// Example:
//
//	{
//	  "id": 42,
//	  "name": "streamspace-analytics-advanced",
//	  "displayName": "Advanced Analytics",
//	  "description": "Comprehensive analytics and reporting for sessions",
//	  "category": "Analytics",
//	  "pluginType": "api",
//	  "version": "1.2.0",
//	  "installCount": 127,
//	  "avgRating": 4.5,
//	  "tags": ["analytics", "reporting", "metrics"]
//	}
type CatalogPlugin struct {
	// ID is a unique database identifier for this catalog entry.
	ID int `json:"id"`

	// RepositoryID links this plugin to its source repository.
	RepositoryID int `json:"repositoryId"`

	// Name is the machine-readable plugin identifier (must match manifest).
	// Format: lowercase, hyphens, no spaces
	// Example: "streamspace-analytics-advanced", "streamspace-billing"
	Name string `json:"name"`

	// Version is the semantic version from the manifest.
	// Format: MAJOR.MINOR.PATCH (e.g., "1.2.0", "2.0.0-beta.1")
	Version string `json:"version"`

	// DisplayName is the human-readable plugin name shown in UI.
	DisplayName string `json:"displayName"`

	// Description explains what this plugin does.
	Description string `json:"description"`

	// Category organizes plugins in the catalog.
	// Examples: "Analytics", "Security", "Integrations", "UI Enhancements"
	Category string `json:"category"`

	// PluginType indicates the plugin's architecture.
	// Valid values:
	//   - "extension": General-purpose extension (most common)
	//   - "webhook": Responds to webhook events
	//   - "api": Adds new API endpoints
	//   - "ui": Adds UI components or pages
	//   - "theme": Visual theme customization
	PluginType string `json:"pluginType"`

	// IconURL is the URL to the plugin's icon image.
	IconURL string `json:"iconUrl"`

	// Manifest contains the full plugin metadata and configuration schema.
	Manifest PluginManifest `json:"manifest"`

	// Tags are keywords for search and filtering.
	Tags []string `json:"tags"`

	// InstallCount is how many times this plugin has been installed.
	InstallCount int `json:"installCount"`

	// AvgRating is the average user rating (1-5 stars).
	AvgRating float64 `json:"avgRating"`

	// RatingCount is the number of ratings submitted.
	RatingCount int `json:"ratingCount"`

	// Repository contains the source repository information.
	// Embedded via JOIN query for convenience.
	Repository Repository `json:"repository"`

	// CreatedAt is when this plugin first appeared in the catalog.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the plugin manifest or metadata was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// InstalledPlugin represents a plugin that is currently installed and potentially running.
//
// Installation process:
//  1. User selects plugin from catalog
//  2. Plugin files are copied/downloaded
//  3. Default configuration is applied
//  4. Plugin is enabled (starts running)
//
// Installed plugins can be:
//   - Enabled/disabled without uninstalling
//   - Configured via JSON config
//   - Updated to newer versions
//   - Uninstalled (removes from this table)
//
// Example:
//
//	{
//	  "id": 7,
//	  "catalogPluginId": 42,
//	  "name": "streamspace-analytics-advanced",
//	  "version": "1.2.0",
//	  "enabled": true,
//	  "config": {"retentionDays": 90, "exportFormat": "json"},
//	  "installedBy": "admin",
//	  "installedAt": "2025-01-01T00:00:00Z"
//	}
type InstalledPlugin struct {
	// ID is a unique database identifier for this installation.
	ID int `json:"id"`

	// CatalogPluginID links to the catalog entry this was installed from.
	// Nil for manually installed plugins (uploaded directly).
	CatalogPluginID *int `json:"catalogPluginId,omitempty"`

	// Name is the plugin identifier (must match manifest).
	Name string `json:"name"`

	// Version is the installed version.
	Version string `json:"version"`

	// Enabled determines whether this plugin is currently running.
	// When false:
	//   - Plugin code is not loaded
	//   - API endpoints are not registered
	//   - Webhooks are not processed
	//   - Can be re-enabled without reinstalling
	Enabled bool `json:"enabled"`

	// Config is the JSON configuration for this plugin.
	// Schema is defined in the plugin's manifest (configSchema field).
	Config json.RawMessage `json:"config,omitempty"`

	// InstalledBy is the username who installed this plugin.
	InstalledBy string `json:"installedBy"`

	// InstalledAt is when this plugin was first installed.
	InstalledAt time.Time `json:"installedAt"`

	// UpdatedAt is when configuration or version was last changed.
	UpdatedAt time.Time `json:"updatedAt"`

	// The following fields are populated from the catalog via JOIN.
	// They provide convenience for API responses without extra queries.

	// DisplayName is the human-readable plugin name.
	DisplayName string `json:"displayName,omitempty"`

	// Description explains what this plugin does.
	Description string `json:"description,omitempty"`

	// PluginType indicates the plugin architecture.
	PluginType string `json:"pluginType,omitempty"`

	// IconURL is the plugin's icon image.
	IconURL string `json:"iconUrl,omitempty"`

	// Manifest contains the full plugin metadata.
	Manifest *PluginManifest `json:"manifest,omitempty"`
}

// PluginManifest contains complete metadata and configuration schema for a plugin.
//
// The manifest defines:
//   - Basic metadata (name, version, author, license)
//   - System requirements and dependencies
//   - Configuration schema (what settings the plugin accepts)
//   - Entry points (where to load plugin code)
//   - Required permissions
//
// Manifest files are:
//   - Located at {plugin-dir}/plugin.json in repositories
//   - Validated during sync
//   - Stored in database as JSONB
//   - Used to generate UI forms for configuration
//
// Example manifest:
//
//	{
//	  "name": "streamspace-analytics-advanced",
//	  "version": "1.2.0",
//	  "displayName": "Advanced Analytics",
//	  "description": "Comprehensive analytics and reporting",
//	  "author": "StreamSpace Team",
//	  "license": "MIT",
//	  "type": "api",
//	  "category": "Analytics",
//	  "configSchema": {
//	    "retentionDays": {"type": "number", "default": 90},
//	    "exportFormat": {"type": "string", "enum": ["json", "csv"]}
//	  },
//	  "permissions": ["sessions:read", "analytics:write"]
//	}
type PluginManifest struct {
	// Name is the unique plugin identifier (lowercase, hyphens).
	Name string `json:"name"`

	// Version is the semantic version (MAJOR.MINOR.PATCH).
	Version string `json:"version"`

	// DisplayName is the human-readable plugin name.
	DisplayName string `json:"displayName"`

	// Description explains the plugin's purpose and features.
	Description string `json:"description"`

	// Author is the plugin developer/organization.
	Author string `json:"author"`

	// Homepage is a URL to the plugin's website or documentation.
	Homepage string `json:"homepage,omitempty"`

	// Repository is the source code repository URL.
	Repository string `json:"repository,omitempty"`

	// License is the SPDX license identifier (e.g., "MIT", "Apache-2.0").
	License string `json:"license,omitempty"`

	// Type is the plugin architecture type.
	// Valid values: "extension", "webhook", "api", "ui", "theme"
	Type string `json:"type"`

	// Category organizes plugins in the catalog.
	Category string `json:"category,omitempty"`

	// Tags are keywords for search and filtering.
	Tags []string `json:"tags,omitempty"`

	// Icon is a relative path to the icon file in the plugin directory.
	Icon string `json:"icon,omitempty"`

	// Requirements specifies platform version and dependency requirements.
	Requirements PluginRequirements `json:"requirements,omitempty"`

	// Entrypoints define where to load plugin code.
	Entrypoints PluginEntrypoints `json:"entrypoints,omitempty"`

	// ConfigSchema is a JSON Schema defining valid configuration.
	// Used to generate UI forms and validate config on save.
	ConfigSchema map[string]interface{} `json:"configSchema,omitempty"`

	// DefaultConfig provides default values for configuration.
	DefaultConfig map[string]interface{} `json:"defaultConfig,omitempty"`

	// Permissions lists required API permissions.
	// Examples: "sessions:read", "sessions:write", "analytics:write"
	Permissions []string `json:"permissions,omitempty"`

	// Dependencies lists other required plugins with version constraints.
	// Format: {"plugin-name": ">=1.0.0", "other-plugin": "^2.0.0"}
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// PluginRequirements specifies plugin requirements
type PluginRequirements struct {
	StreamSpaceVersion string            `json:"streamspaceVersion,omitempty"` // e.g., ">=0.2.0"
	MinimumVersion     string            `json:"minimumVersion,omitempty"`
	MaximumVersion     string            `json:"maximumVersion,omitempty"`
	Plugins            []string          `json:"plugins,omitempty"` // Required plugins
}

// PluginEntrypoints defines plugin entry points
type PluginEntrypoints struct {
	Main      string `json:"main,omitempty"`      // Main entry point
	UI        string `json:"ui,omitempty"`        // UI component entry point
	API       string `json:"api,omitempty"`       // API routes entry point
	Webhook   string `json:"webhook,omitempty"`   // Webhook handler
	CLI       string `json:"cli,omitempty"`       // CLI command entry point
}

// PluginVersion represents a version of a plugin
type PluginVersion struct {
	ID        int             `json:"id"`
	PluginID  int             `json:"pluginId"`
	Version   string          `json:"version"`
	Changelog string          `json:"changelog,omitempty"`
	Manifest  PluginManifest  `json:"manifest"`
	CreatedAt time.Time       `json:"createdAt"`
}

// PluginRating represents a user's rating for a plugin
type PluginRating struct {
	ID        int       `json:"id"`
	PluginID  int       `json:"pluginId"`
	UserID    string    `json:"userId"`
	Rating    int       `json:"rating"` // 1-5
	Review    string    `json:"review,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PluginStats represents usage statistics for a plugin
type PluginStats struct {
	PluginID        int        `json:"pluginId"`
	ViewCount       int        `json:"viewCount"`
	InstallCount    int        `json:"installCount"`
	LastViewedAt    *time.Time `json:"lastViewedAt,omitempty"`
	LastInstalledAt *time.Time `json:"lastInstalledAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// Scan implements sql.Scanner for PluginManifest
func (m *PluginManifest) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, m)
}

// Value implements driver.Valuer for PluginManifest
func (m PluginManifest) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// InstallPluginRequest represents a request to install a plugin
type InstallPluginRequest struct {
	PluginID int             `json:"pluginId" validate:"required,gt=0"` // From catalog
	Config   json.RawMessage `json:"config,omitempty"`
}

// UpdatePluginRequest represents a request to update a plugin
type UpdatePluginRequest struct {
	Enabled *bool           `json:"enabled,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// RatePluginRequest represents a request to rate a plugin
type RatePluginRequest struct {
	Rating int    `json:"rating" validate:"required,min=1,max=5"` // 1-5
	Review string `json:"review,omitempty" validate:"omitempty,max=2000"`
}
