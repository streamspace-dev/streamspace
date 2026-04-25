package sync

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TemplateParser parses Kubernetes Template manifests from Git repositories.
//
// The parser discovers and validates Template resources in YAML format.
// It walks repository directories, identifies Template manifests, and
// extracts metadata for catalog indexing.
//
// Manifest discovery:
//   - Searches for *.yaml and *.yml files
//   - Validates kind: Template and apiVersion
//   - Skips non-Template YAML files (no errors)
//   - Skips .git directories
//
// Validation:
//   - Required fields: name, displayName, baseImage
//   - API version: stream.space/v1alpha1 (or stream.streamspace.io/v1alpha1 for backward compatibility)
//   - App type inference: desktop (VNC) or webapp (HTTP)
//
// Example usage:
//
//	parser := NewTemplateParser()
//	templates, err := parser.ParseRepository("/tmp/streamspace-templates")
//	for _, t := range templates {
//	    fmt.Printf("Found template: %s (%s)\n", t.DisplayName, t.Category)
//	}
type TemplateParser struct{}

// NewTemplateParser creates a new template parser instance.
//
// The parser is stateless and can be reused for multiple repositories.
//
// Example:
//
//	parser := NewTemplateParser()
//	templates1, _ := parser.ParseRepository("/tmp/repo1")
//	templates2, _ := parser.ParseRepository("/tmp/repo2")
func NewTemplateParser() *TemplateParser {
	return &TemplateParser{}
}

// ParsedTemplate represents a template extracted from a repository manifest.
//
// This structure contains metadata for catalog database insertion.
// The full manifest is stored as JSON for future reference and validation.
//
// Field mappings:
//   - Name: metadata.name from YAML
//   - DisplayName: spec.displayName (UI-friendly name)
//   - Description: spec.description (markdown supported)
//   - Category: spec.category (for catalog grouping)
//   - AppType: "desktop" (VNC) or "webapp" (HTTP)
//   - Icon: URL to icon image
//   - Manifest: Full YAML manifest as JSON string
//   - Tags: Keywords for search/filtering
//
// Example:
//
//	template := &ParsedTemplate{
//	    Name: "firefox-browser",
//	    DisplayName: "Firefox Web Browser",
//	    Category: "Web Browsers",
//	    AppType: "desktop",
//	    Tags: []string{"browser", "web", "privacy"},
//	}
type ParsedTemplate struct {
	// Name is the unique identifier from metadata.name.
	// Format: lowercase, hyphens, no spaces
	// Example: "firefox-browser", "vscode-dev"
	Name string

	// DisplayName is the human-readable name shown in UI.
	// Example: "Firefox Web Browser", "Visual Studio Code"
	DisplayName string

	// Description explains what this template provides.
	// Markdown formatting is supported.
	Description string

	// Category organizes templates in the catalog.
	// Examples: "Web Browsers", "Development", "Design"
	Category string

	// AppType indicates the application streaming type.
	// Valid values: "desktop" (VNC), "webapp" (HTTP)
	AppType string

	// Icon is the URL to the template's icon image.
	// Can be relative (in repo) or absolute (CDN)
	Icon string

	// Manifest is the full YAML manifest encoded as JSON.
	// Stored in database for template instantiation.
	Manifest string

	// Tags are keywords for search and filtering.
	// Example: ["browser", "web", "privacy"]
	Tags []string
}

// TemplateManifest represents the complete YAML structure of a Template resource.
//
// This structure mirrors the Kubernetes Template CRD defined in:
//   controller/api/v1alpha1/template_types.go
//
// The manifest is parsed from YAML files in repositories and validated
// before being stored in the catalog database as JSON.
//
// Example YAML:
//
//	apiVersion: stream.space/v1alpha1
//	kind: Template
//	metadata:
//	  name: firefox-browser
//	spec:
//	  displayName: Firefox Web Browser
//	  description: Modern, privacy-focused web browser
//	  category: Web Browsers
//	  baseImage: lscr.io/linuxserver/firefox:latest
//	  vnc:
//	    enabled: true
//	    port: 3000
//	  tags: [browser, web, privacy]
type TemplateManifest struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name" json:"name"`
		Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
		Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		DisplayName      string            `yaml:"displayName" json:"displayName"`
		Description      string            `yaml:"description" json:"description"`
		Category         string            `yaml:"category" json:"category"`
		AppType          string            `yaml:"appType,omitempty" json:"appType,omitempty"`
		Icon             string            `yaml:"icon,omitempty" json:"icon,omitempty"`
		BaseImage        string            `yaml:"baseImage" json:"baseImage"`
		DefaultResources map[string]string `yaml:"defaultResources,omitempty" json:"defaultResources,omitempty"`
		Ports            []struct {
			Name          string `yaml:"name" json:"name"`
			ContainerPort int    `yaml:"containerPort" json:"containerPort"`
			Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
		} `yaml:"ports,omitempty" json:"ports,omitempty"`
		Env          []map[string]interface{} `yaml:"env,omitempty" json:"env,omitempty"`
		VolumeMounts []map[string]interface{} `yaml:"volumeMounts,omitempty" json:"volumeMounts,omitempty"`
		VNC          *struct {
			Enabled    bool   `yaml:"enabled" json:"enabled"`
			Port       int    `yaml:"port" json:"port"`
			Protocol   string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
			Encryption bool   `yaml:"encryption,omitempty" json:"encryption,omitempty"`
		} `yaml:"vnc,omitempty" json:"vnc,omitempty"`
		WebApp *struct {
			Enabled     bool   `yaml:"enabled" json:"enabled"`
			Port        int    `yaml:"port" json:"port"`
			Path        string `yaml:"path,omitempty" json:"path,omitempty"`
			HealthCheck string `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
		} `yaml:"webapp,omitempty" json:"webapp,omitempty"`
		Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
		Tags         []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	} `yaml:"spec" json:"spec"`
}

// ParseRepository parses all Template manifests in a Git repository.
//
// Discovery process:
//  1. Walk all directories in repository
//  2. Find files with .yaml or .yml extension
//  3. Parse YAML and check if kind: Template
//  4. Extract metadata and validate
//  5. Skip invalid files (continue processing others)
//
// Behavior:
//   - Skips .git directory (performance)
//   - Skips non-Template YAML files (no error)
//   - Logs parse errors but continues (partial success)
//   - Returns all successfully parsed templates
//
// Parameters:
//   - repoPath: Local filesystem path to Git repository
//
// Returns:
//   - Array of parsed templates (may be empty)
//   - Error only if directory walk fails (not for individual parse errors)
//
// Example:
//
//	parser := NewTemplateParser()
//	templates, err := parser.ParseRepository("/tmp/streamspace-templates")
//	if err != nil {
//	    log.Fatal("Failed to walk repository:", err)
//	}
//	log.Printf("Found %d templates", len(templates))
func (p *TemplateParser) ParseRepository(repoPath string) ([]*ParsedTemplate, error) {
	var templates []*ParsedTemplate

	// Walk through repository looking for YAML files
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Skip .git directory
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Parse template file
		template, err := p.ParseTemplateFile(path)
		if err != nil {
			// Log error but continue processing other files
			// (not all YAML files may be templates)
			return nil
		}

		templates = append(templates, template)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return templates, nil
}

// ParseTemplateFile parses a single Template YAML file.
//
// Parsing steps:
//  1. Read file from disk
//  2. Unmarshal YAML into TemplateManifest struct
//  3. Validate kind == "Template"
//  4. Validate apiVersion == "stream.streamspace.io/v1alpha1"
//  5. Validate required fields (name, displayName, baseImage)
//  6. Infer appType from VNC/WebApp config if not specified
//  7. Convert manifest to JSON for database storage
//
// App type inference:
//   - If spec.webapp.enabled: appType = "webapp"
//   - Otherwise: appType = "desktop" (VNC-based)
//
// Parameters:
//   - filePath: Absolute path to YAML file
//
// Returns:
//   - ParsedTemplate with extracted metadata
//   - Error if file cannot be read, parsed, or validated
//
// Example:
//
//	template, err := parser.ParseTemplateFile("/tmp/repo/browsers/firefox.yaml")
//	if err != nil {
//	    log.Printf("Invalid template: %v", err)
//	    return nil, err
//	}
//	fmt.Printf("Parsed: %s\n", template.DisplayName)
func (p *TemplateParser) ParseTemplateFile(filePath string) (*ParsedTemplate, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse YAML
	var manifest TemplateManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate this is a Template resource
	if manifest.Kind != "Template" {
		return nil, fmt.Errorf("not a Template resource (kind: %s)", manifest.Kind)
	}

	// Support both old and new API versions for backward compatibility
	if manifest.APIVersion != "stream.space/v1alpha1" && manifest.APIVersion != "stream.streamspace.io/v1alpha1" {
		return nil, fmt.Errorf("unsupported API version: %s (expected stream.space/v1alpha1)", manifest.APIVersion)
	}

	// Validate required fields
	if manifest.Metadata.Name == "" {
		return nil, fmt.Errorf("template name is required")
	}

	if manifest.Spec.DisplayName == "" {
		return nil, fmt.Errorf("displayName is required")
	}

	if manifest.Spec.BaseImage == "" {
		return nil, fmt.Errorf("baseImage is required")
	}

	// Determine app type
	appType := manifest.Spec.AppType
	if appType == "" {
		// Infer from VNC/WebApp config
		if manifest.Spec.WebApp != nil && manifest.Spec.WebApp.Enabled {
			appType = "webapp"
		} else {
			appType = "desktop"
		}
	}

	// Convert full manifest to JSON for storage
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest to JSON: %w", err)
	}

	// Create parsed template
	template := &ParsedTemplate{
		Name:        manifest.Metadata.Name,
		DisplayName: manifest.Spec.DisplayName,
		Description: manifest.Spec.Description,
		Category:    manifest.Spec.Category,
		AppType:     appType,
		Icon:        manifest.Spec.Icon,
		Manifest:    string(manifestJSON),
		Tags:        manifest.Spec.Tags,
	}

	// Default empty tags to empty array
	if template.Tags == nil {
		template.Tags = []string{}
	}

	return template, nil
}

// ParseTemplateFromString parses a template from a YAML string
func (p *TemplateParser) ParseTemplateFromString(yamlContent string) (*ParsedTemplate, error) {
	// Parse YAML
	var manifest TemplateManifest
	if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate this is a Template resource
	if manifest.Kind != "Template" {
		return nil, fmt.Errorf("not a Template resource (kind: %s)", manifest.Kind)
	}

	// Determine app type
	appType := manifest.Spec.AppType
	if appType == "" {
		if manifest.Spec.WebApp != nil && manifest.Spec.WebApp.Enabled {
			appType = "webapp"
		} else {
			appType = "desktop"
		}
	}

	// Convert full manifest to JSON for storage
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest to JSON: %w", err)
	}

	template := &ParsedTemplate{
		Name:        manifest.Metadata.Name,
		DisplayName: manifest.Spec.DisplayName,
		Description: manifest.Spec.Description,
		Category:    manifest.Spec.Category,
		AppType:     appType,
		Icon:        manifest.Spec.Icon,
		Manifest:    string(manifestJSON),
		Tags:        manifest.Spec.Tags,
	}

	if template.Tags == nil {
		template.Tags = []string{}
	}

	return template, nil
}

// ValidateTemplateManifest validates a template manifest structure
func (p *TemplateParser) ValidateTemplateManifest(yamlContent string) error {
	var manifest TemplateManifest
	if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Check required fields
	if manifest.Kind != "Template" {
		return fmt.Errorf("kind must be 'Template', got '%s'", manifest.Kind)
	}

	// Support both old and new API versions
	if manifest.APIVersion != "stream.space/v1alpha1" && manifest.APIVersion != "stream.streamspace.io/v1alpha1" {
		return fmt.Errorf("apiVersion must be 'stream.space/v1alpha1', got '%s'", manifest.APIVersion)
	}

	if manifest.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	if manifest.Spec.DisplayName == "" {
		return fmt.Errorf("spec.displayName is required")
	}

	if manifest.Spec.BaseImage == "" {
		return fmt.Errorf("spec.baseImage is required")
	}

	// Validate app type if specified
	if manifest.Spec.AppType != "" && manifest.Spec.AppType != "desktop" && manifest.Spec.AppType != "webapp" {
		return fmt.Errorf("spec.appType must be 'desktop' or 'webapp', got '%s'", manifest.Spec.AppType)
	}

	return nil
}

// ========== Plugin Parsing ==========

// PluginParser parses plugin manifests from Git repositories.
//
// The parser discovers and validates plugin manifest.json files.
// Unlike templates (YAML), plugins use JSON manifests with a different
// structure optimized for extension system metadata.
//
// Manifest discovery:
//   - Searches for files named "manifest.json"
//   - Validates required fields (name, version, displayName, type)
//   - Validates plugin type (extension, webhook, api, ui, theme)
//   - Skips .git directories
//
// Plugin types:
//   - extension: General-purpose plugin (most common)
//   - webhook: Responds to webhook events
//   - api: Adds new API endpoints
//   - ui: Adds UI components or pages
//   - theme: Visual theme customization
//
// Example usage:
//
//	parser := NewPluginParser()
//	plugins, err := parser.ParseRepository("/tmp/streamspace-plugins")
//	for _, p := range plugins {
//	    fmt.Printf("Found plugin: %s v%s\n", p.DisplayName, p.Version)
//	}
type PluginParser struct{}

// NewPluginParser creates a new plugin parser instance.
//
// The parser is stateless and can be reused for multiple repositories.
//
// Example:
//
//	parser := NewPluginParser()
//	plugins1, _ := parser.ParseRepository("/tmp/official-plugins")
//	plugins2, _ := parser.ParseRepository("/tmp/community-plugins")
func NewPluginParser() *PluginParser {
	return &PluginParser{}
}

// ParsedPlugin represents a plugin extracted from a repository manifest.
//
// This structure contains metadata for catalog database insertion.
// The full manifest is stored as JSON for configuration and installation.
//
// Field mappings:
//   - Name: Unique plugin identifier (lowercase, hyphens)
//   - Version: Semantic version (MAJOR.MINOR.PATCH)
//   - DisplayName: Human-readable name for UI
//   - Description: Plugin purpose and features
//   - Category: Catalog organization (Analytics, Security, etc.)
//   - PluginType: Architecture type (extension, webhook, api, ui, theme)
//   - Icon: URL to icon image
//   - Manifest: Full manifest.json as JSON string
//   - Tags: Keywords for search/filtering
//
// Example:
//
//	plugin := &ParsedPlugin{
//	    Name: "streamspace-analytics-advanced",
//	    Version: "1.2.0",
//	    DisplayName: "Advanced Analytics",
//	    PluginType: "api",
//	    Tags: []string{"analytics", "reporting"},
//	}
type ParsedPlugin struct {
	// Name is the unique plugin identifier.
	// Format: lowercase, hyphens, no spaces
	// Example: "streamspace-analytics-advanced", "streamspace-billing"
	Name string

	// Version is the semantic version.
	// Format: MAJOR.MINOR.PATCH (e.g., "1.2.0", "2.0.0-beta.1")
	Version string

	// DisplayName is the human-readable plugin name shown in UI.
	// Example: "Advanced Analytics", "Billing Integration"
	DisplayName string

	// Description explains what this plugin does.
	Description string

	// Category organizes plugins in the catalog.
	// Examples: "Analytics", "Security", "Integrations", "UI Enhancements"
	Category string

	// PluginType indicates the plugin's architecture.
	// Valid values: "extension", "webhook", "api", "ui", "theme"
	PluginType string

	// Icon is the URL to the plugin's icon image.
	// Can be relative (in repo) or absolute (CDN)
	Icon string

	// Manifest is the full manifest.json encoded as JSON string.
	// Stored in database for plugin installation and configuration.
	Manifest string

	// Tags are keywords for search and filtering.
	// Example: ["analytics", "reporting", "metrics"]
	Tags []string
}

// PluginManifest represents the complete JSON structure of a plugin manifest.
//
// This structure is read from manifest.json files in plugin repositories.
// It defines all metadata, configuration schema, and requirements for a plugin.
//
// Example manifest.json:
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
	// Name is the unique plugin identifier (required).
	// Format: lowercase, hyphens, no spaces
	Name string `json:"name"`

	// Version is the semantic version (required).
	// Format: MAJOR.MINOR.PATCH
	Version string `json:"version"`

	// DisplayName is the human-readable name (required).
	DisplayName string `json:"displayName"`

	// Description explains the plugin's purpose (required).
	Description string `json:"description"`

	// Author is the plugin developer/organization.
	Author string `json:"author"`

	// Homepage is a URL to the plugin's website or documentation.
	Homepage string `json:"homepage,omitempty"`

	// Repository is the source code repository URL.
	Repository string `json:"repository,omitempty"`

	// License is the SPDX license identifier (e.g., "MIT", "Apache-2.0").
	License string `json:"license,omitempty"`

	// Type is the plugin architecture type (required).
	// Valid values: "extension", "webhook", "api", "ui", "theme"
	Type string `json:"type"`

	// Category organizes plugins in the catalog.
	Category string `json:"category,omitempty"`

	// Tags are keywords for search and filtering.
	Tags []string `json:"tags,omitempty"`

	// Icon is a relative path to the icon file in the plugin directory.
	Icon string `json:"icon,omitempty"`

	// Requirements specifies platform version and dependency requirements.
	// Example: {"streamspaceVersion": ">=0.2.0"}
	Requirements map[string]string `json:"requirements,omitempty"`

	// Entrypoints define where to load plugin code.
	// Example: {"main": "index.js", "api": "api/routes.js"}
	Entrypoints map[string]string `json:"entrypoints,omitempty"`

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

// ParseRepository parses all plugin manifests in a Git repository.
//
// Discovery process:
//  1. Walk all directories in repository
//  2. Find files named "manifest.json"
//  3. Parse JSON and validate structure
//  4. Extract metadata and validate required fields
//  5. Skip invalid files (continue processing others)
//
// Behavior:
//   - Skips .git directory (performance)
//   - Only processes files named exactly "manifest.json"
//   - Logs parse errors but continues (partial success)
//   - Returns all successfully parsed plugins
//
// Parameters:
//   - repoPath: Local filesystem path to Git repository
//
// Returns:
//   - Array of parsed plugins (may be empty)
//   - Error only if directory walk fails (not for individual parse errors)
//
// Example:
//
//	parser := NewPluginParser()
//	plugins, err := parser.ParseRepository("/tmp/streamspace-plugins")
//	if err != nil {
//	    log.Fatal("Failed to walk repository:", err)
//	}
//	log.Printf("Found %d plugins", len(plugins))
func (p *PluginParser) ParseRepository(repoPath string) ([]*ParsedPlugin, error) {
	var plugins []*ParsedPlugin

	// Walk through repository looking for manifest.json files
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Only process manifest.json files
		if !d.IsDir() && d.Name() == "manifest.json" {
			plugin, err := p.ParsePluginFile(path)
			if err != nil {
				// Log error but continue processing other files
				return nil
			}

			plugins = append(plugins, plugin)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return plugins, nil
}

// ParsePluginFile parses a single plugin manifest.json file.
//
// Parsing steps:
//  1. Read file from disk
//  2. Unmarshal JSON into PluginManifest struct
//  3. Validate required fields (name, version, displayName, type)
//  4. Validate plugin type is one of: extension, webhook, api, ui, theme
//  5. Convert manifest to JSON for database storage
//
// Required fields:
//   - name: Unique plugin identifier
//   - version: Semantic version
//   - displayName: Human-readable name
//   - type: Plugin architecture type
//
// Plugin type validation:
//   - "extension": General-purpose extension (most common)
//   - "webhook": Responds to webhook events
//   - "api": Adds new API endpoints
//   - "ui": Adds UI components or pages
//   - "theme": Visual theme customization
//
// Parameters:
//   - filePath: Absolute path to manifest.json file
//
// Returns:
//   - ParsedPlugin with extracted metadata
//   - Error if file cannot be read, parsed, or validated
//
// Example:
//
//	plugin, err := parser.ParsePluginFile("/tmp/repo/analytics/manifest.json")
//	if err != nil {
//	    log.Printf("Invalid plugin: %v", err)
//	    return nil, err
//	}
//	fmt.Printf("Parsed: %s v%s\n", plugin.DisplayName, plugin.Version)
func (p *PluginParser) ParsePluginFile(filePath string) (*ParsedPlugin, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if manifest.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}

	if manifest.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	if manifest.DisplayName == "" {
		return nil, fmt.Errorf("displayName is required")
	}

	if manifest.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Validate plugin type
	validTypes := map[string]bool{
		"extension": true,
		"webhook":   true,
		"api":       true,
		"ui":        true,
		"theme":     true,
	}
	if !validTypes[manifest.Type] {
		return nil, fmt.Errorf("invalid plugin type: %s (must be extension, webhook, api, ui, or theme)", manifest.Type)
	}

	// Convert full manifest to JSON for storage
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to encode manifest: %w", err)
	}

	plugin := &ParsedPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		DisplayName: manifest.DisplayName,
		Description: manifest.Description,
		Category:    manifest.Category,
		PluginType:  manifest.Type,
		Icon:        manifest.Icon,
		Manifest:    string(manifestJSON),
		Tags:        manifest.Tags,
	}

	if plugin.Tags == nil {
		plugin.Tags = []string{}
	}

	return plugin, nil
}

// ValidatePluginManifest validates a plugin manifest structure
func (p *PluginParser) ValidatePluginManifest(jsonContent string) error {
	var manifest PluginManifest
	if err := json.Unmarshal([]byte(jsonContent), &manifest); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check required fields
	if manifest.Name == "" {
		return fmt.Errorf("name is required")
	}

	if manifest.Version == "" {
		return fmt.Errorf("version is required")
	}

	if manifest.DisplayName == "" {
		return fmt.Errorf("displayName is required")
	}

	if manifest.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate plugin type
	validTypes := map[string]bool{
		"extension": true,
		"webhook":   true,
		"api":       true,
		"ui":        true,
		"theme":     true,
	}
	if !validTypes[manifest.Type] {
		return fmt.Errorf("invalid type: %s (must be extension, webhook, api, ui, or theme)", manifest.Type)
	}

	return nil
}
