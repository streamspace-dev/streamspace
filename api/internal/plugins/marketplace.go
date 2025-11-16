package plugins

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
)

// PluginMarketplace manages plugin discovery, download, and installation
type PluginMarketplace struct {
	db               *db.Database
	repositoryURL    string
	pluginDir        string
	cacheTTL         time.Duration
	lastSync         time.Time
	availablePlugins map[string]*MarketplacePlugin
}

// MarketplacePlugin represents a plugin available in the marketplace
type MarketplacePlugin struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags"`
	IconURL     string                 `json:"iconUrl"`
	Manifest    models.PluginManifest  `json:"manifest"`
	DownloadURL string                 `json:"downloadUrl"`
	Installed   bool                   `json:"installed"`
	Enabled     bool                   `json:"enabled"`
}

// NewPluginMarketplace creates a new plugin marketplace instance
func NewPluginMarketplace(database *db.Database, repositoryURL, pluginDir string) *PluginMarketplace {
	if repositoryURL == "" {
		repositoryURL = "https://raw.githubusercontent.com/JoshuaAFerguson/streamspace-plugins/main"
	}

	if pluginDir == "" {
		pluginDir = "/plugins"
	}

	return &PluginMarketplace{
		db:               database,
		repositoryURL:    repositoryURL,
		pluginDir:        pluginDir,
		cacheTTL:         15 * time.Minute,
		availablePlugins: make(map[string]*MarketplacePlugin),
	}
}

// SyncCatalog syncs the plugin catalog from the remote repository
func (m *PluginMarketplace) SyncCatalog(ctx context.Context) error {
	// Check cache
	if time.Since(m.lastSync) < m.cacheTTL {
		log.Printf("[Plugin Marketplace] Using cached catalog (age: %v)", time.Since(m.lastSync))
		return nil
	}

	log.Println("[Plugin Marketplace] Syncing plugin catalog from repository...")

	// Fetch catalog from repository
	catalogURL := fmt.Sprintf("%s/catalog.json", m.repositoryURL)
	resp, err := http.Get(catalogURL)
	if err != nil {
		return fmt.Errorf("failed to fetch plugin catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch catalog: HTTP %d", resp.StatusCode)
	}

	// Parse catalog
	var plugins []*MarketplacePlugin
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		return fmt.Errorf("failed to parse plugin catalog: %w", err)
	}

	// Update local cache
	m.availablePlugins = make(map[string]*MarketplacePlugin)
	for _, plugin := range plugins {
		m.availablePlugins[plugin.Name] = plugin
	}

	// Mark installed plugins
	if err := m.markInstalledPlugins(ctx); err != nil {
		log.Printf("[Plugin Marketplace] Warning: Failed to mark installed plugins: %v", err)
	}

	// Update database catalog
	if err := m.updateDatabaseCatalog(ctx, plugins); err != nil {
		log.Printf("[Plugin Marketplace] Warning: Failed to update database catalog: %v", err)
	}

	m.lastSync = time.Now()
	log.Printf("[Plugin Marketplace] Synced %d plugins from catalog", len(plugins))

	return nil
}

// ListAvailable returns all available plugins in the marketplace
func (m *PluginMarketplace) ListAvailable(ctx context.Context) ([]*MarketplacePlugin, error) {
	// Ensure catalog is synced
	if err := m.SyncCatalog(ctx); err != nil {
		return nil, err
	}

	plugins := make([]*MarketplacePlugin, 0, len(m.availablePlugins))
	for _, plugin := range m.availablePlugins {
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// GetPlugin retrieves a specific plugin from the marketplace
func (m *PluginMarketplace) GetPlugin(ctx context.Context, name string) (*MarketplacePlugin, error) {
	// Ensure catalog is synced
	if err := m.SyncCatalog(ctx); err != nil {
		return nil, err
	}

	plugin, exists := m.availablePlugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found in marketplace", name)
	}

	return plugin, nil
}

// InstallPlugin downloads and installs a plugin from the marketplace
func (m *PluginMarketplace) InstallPlugin(ctx context.Context, name string, config map[string]interface{}) error {
	log.Printf("[Plugin Marketplace] Installing plugin: %s", name)

	// Get plugin info
	plugin, err := m.GetPlugin(ctx, name)
	if err != nil {
		return err
	}

	// Download plugin
	if err := m.downloadPlugin(ctx, plugin); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Register in database
	if err := m.registerPluginInDatabase(ctx, plugin, config); err != nil {
		return fmt.Errorf("failed to register plugin in database: %w", err)
	}

	log.Printf("[Plugin Marketplace] Plugin installed successfully: %s", name)
	return nil
}

// UninstallPlugin removes a plugin
func (m *PluginMarketplace) UninstallPlugin(ctx context.Context, name string) error {
	log.Printf("[Plugin Marketplace] Uninstalling plugin: %s", name)

	// Remove from database
	_, err := m.db.DB().ExecContext(ctx, `
		DELETE FROM installed_plugins WHERE name = $1
	`, name)
	if err != nil {
		return fmt.Errorf("failed to remove from database: %w", err)
	}

	// Remove plugin files
	pluginPath := filepath.Join(m.pluginDir, name)
	if err := os.RemoveAll(pluginPath); err != nil {
		log.Printf("[Plugin Marketplace] Warning: Failed to remove plugin files: %v", err)
	}

	log.Printf("[Plugin Marketplace] Plugin uninstalled successfully: %s", name)
	return nil
}

// downloadPlugin downloads a plugin from the repository
func (m *PluginMarketplace) downloadPlugin(ctx context.Context, plugin *MarketplacePlugin) error {
	log.Printf("[Plugin Marketplace] Downloading plugin %s from %s", plugin.Name, plugin.DownloadURL)

	// Create plugin directory
	pluginPath := filepath.Join(m.pluginDir, plugin.Name)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download plugin archive
	downloadURL := plugin.DownloadURL
	if downloadURL == "" {
		// Default: GitHub raw content
		downloadURL = fmt.Sprintf("%s/%s/plugin.tar.gz", m.repositoryURL, plugin.Name)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Extract archive
	if strings.HasSuffix(downloadURL, ".tar.gz") || strings.HasSuffix(downloadURL, ".tgz") {
		if err := m.extractTarGz(resp.Body, pluginPath); err != nil {
			return fmt.Errorf("failed to extract archive: %w", err)
		}
	} else {
		// Fallback: Download individual files
		if err := m.downloadPluginFiles(plugin.Name, pluginPath); err != nil {
			return fmt.Errorf("failed to download plugin files: %w", err)
		}
	}

	log.Printf("[Plugin Marketplace] Plugin downloaded and extracted: %s", plugin.Name)
	return nil
}

// downloadPluginFiles downloads individual plugin files
func (m *PluginMarketplace) downloadPluginFiles(pluginName, pluginPath string) error {
	// Download manifest.json
	manifestURL := fmt.Sprintf("%s/%s/manifest.json", m.repositoryURL, pluginName)
	if err := m.downloadFile(manifestURL, filepath.Join(pluginPath, "manifest.json")); err != nil {
		return err
	}

	// Download README.md
	readmeURL := fmt.Sprintf("%s/%s/README.md", m.repositoryURL, pluginName)
	m.downloadFile(readmeURL, filepath.Join(pluginPath, "README.md")) // Optional, ignore errors

	// Download plugin code (could be .go, .js, etc.)
	// Try multiple extensions
	for _, ext := range []string{".go", ".js", ".py", "_plugin.go"} {
		codeURL := fmt.Sprintf("%s/%s/%s%s", m.repositoryURL, pluginName, pluginName, ext)
		if err := m.downloadFile(codeURL, filepath.Join(pluginPath, pluginName+ext)); err == nil {
			break // Success
		}
	}

	return nil
}

// downloadFile downloads a file from URL to local path
func (m *PluginMarketplace) downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractTarGz extracts a tar.gz archive
func (m *PluginMarketplace) extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

// registerPluginInDatabase registers a plugin in the database
func (m *PluginMarketplace) registerPluginInDatabase(ctx context.Context, plugin *MarketplacePlugin, config map[string]interface{}) error {
	// Marshal config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert or update plugin
	_, err = m.db.DB().ExecContext(ctx, `
		INSERT INTO installed_plugins (name, version, enabled, config, installed_by, installed_at)
		VALUES ($1, $2, true, $3, 'marketplace', NOW())
		ON CONFLICT (name)
		DO UPDATE SET
			version = $2,
			config = $3,
			updated_at = NOW()
	`, plugin.Name, plugin.Version, configJSON)

	return err
}

// updateDatabaseCatalog updates the catalog_plugins table
func (m *PluginMarketplace) updateDatabaseCatalog(ctx context.Context, plugins []*MarketplacePlugin) error {
	for _, plugin := range plugins {
		// Marshal manifest
		manifestJSON, err := json.Marshal(plugin.Manifest)
		if err != nil {
			log.Printf("[Plugin Marketplace] Error marshaling manifest for %s: %v", plugin.Name, err)
			continue
		}

		// Upsert to catalog
		_, err = m.db.DB().ExecContext(ctx, `
			INSERT INTO catalog_plugins (
				repository_id, name, version, display_name, description,
				category, plugin_type, icon_url, manifest, tags, created_at, updated_at
			)
			VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
			ON CONFLICT (name)
			DO UPDATE SET
				version = $2,
				display_name = $3,
				description = $4,
				category = $5,
				plugin_type = $6,
				icon_url = $7,
				manifest = $8,
				tags = $9,
				updated_at = NOW()
		`, plugin.Name, plugin.Version, plugin.DisplayName, plugin.Description,
			plugin.Category, plugin.Manifest.Type, plugin.IconURL, manifestJSON, plugin.Tags)

		if err != nil {
			log.Printf("[Plugin Marketplace] Error updating catalog for %s: %v", plugin.Name, err)
		}
	}

	return nil
}

// markInstalledPlugins marks which plugins are installed
func (m *PluginMarketplace) markInstalledPlugins(ctx context.Context) error {
	rows, err := m.db.DB().QueryContext(ctx, `
		SELECT name, enabled FROM installed_plugins
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var enabled bool

		if err := rows.Scan(&name, &enabled); err != nil {
			continue
		}

		if plugin, exists := m.availablePlugins[name]; exists {
			plugin.Installed = true
			plugin.Enabled = enabled
		}
	}

	return nil
}
