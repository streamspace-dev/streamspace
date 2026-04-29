// Package handlers provides HTTP request handlers for the StreamSpace API.
//
// The plugins.go file implements HTTP handlers for plugin management,
// including catalog browsing, installation, configuration, and lifecycle management.
//
// API Endpoint Structure:
//
//	Plugin Catalog (browse/install):
//	  GET    /api/plugins/catalog           - Browse available plugins
//	  GET    /api/plugins/catalog/:id       - Get catalog plugin details
//	  POST   /api/plugins/catalog/:id/rate  - Rate a plugin (1-5 stars)
//	  POST   /api/plugins/catalog/:id/install - Install plugin from catalog
//
//	Installed Plugins (CRUD):
//	  GET    /api/plugins                   - List installed plugins
//	  GET    /api/plugins/:id               - Get installed plugin details
//	  PATCH  /api/plugins/:id               - Update plugin config
//	  DELETE /api/plugins/:id               - Uninstall plugin
//	  POST   /api/plugins/:id/enable        - Enable plugin
//	  POST   /api/plugins/:id/disable       - Disable plugin
//
// Database Tables:
//
//	catalog_plugins:
//	  - Plugins available for installation
//	  - Includes metadata (name, version, description, icon, tags)
//	  - Tracks install count, ratings, view count
//
//	installed_plugins:
//	  - Plugins currently installed
//	  - References catalog_plugins via catalog_plugin_id
//	  - Includes enabled status and configuration
//
//	plugin_ratings:
//	  - User ratings for catalog plugins (1-5 stars + review)
//	  - One rating per user per plugin (upsert on conflict)
//
//	plugin_stats:
//	  - Plugin usage statistics (views, installs, last accessed)
//	  - Updated asynchronously (non-blocking)
//
// Design Patterns:
//
//  1. Async stats updates: View/install counts updated in goroutines
//  2. Graceful errors: Individual row parsing errors don't fail entire query
//  3. SQL injection prevention: Parameterized queries with $1, $2, etc.
//  4. User context: user_id extracted from auth middleware via c.GetString()
//
// Example Usage Flow:
//
//  1. User browses catalog:
//     GET /api/plugins/catalog?category=analytics&sort=popular
//
//  2. User views plugin details:
//     GET /api/plugins/catalog/42
//     (View count incremented async)
//
//  3. User installs plugin:
//     POST /api/plugins/catalog/42/install
//     Body: {"config": {"api_key": "..."}}
//     (Plugin added to installed_plugins, install count incremented)
//
//  4. User enables/disables plugin:
//     POST /api/plugins/123/enable
//     (Plugin enabled in database, runtime loads it on next restart/reload)
package handlers

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	internalplugins "github.com/streamspace-dev/streamspace/api/internal/plugins"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// PluginHandler handles plugin-related HTTP requests.
//
// This handler provides HTTP endpoints for:
//   - Browsing the plugin catalog (search, filter, sort)
//   - Installing plugins from the catalog
//   - Managing installed plugins (enable, disable, configure, uninstall)
//   - Rating plugins (user reviews)
//
// All methods interact with the database to query/modify plugin data.
type PluginHandler struct {
	// db is the database connection for plugin queries and updates.
	db *db.Database
	// pluginDir is the directory where plugins are installed.
	pluginDir string
	// runtime manages immediate load/unload behavior when available.
	runtime *internalplugins.RuntimeV2
}

// NewPluginHandler creates a new plugin handler.
//
// Parameters:
//   - database: Database connection for plugin operations
//   - pluginDir: Directory where plugins will be installed
//
// Returns:
//   - Configured PluginHandler ready to register routes
//
// Example:
//
//	handler := NewPluginHandler(db, "/plugins")
//	handler.RegisterRoutes(router.Group("/api"))
func NewPluginHandler(database *db.Database, pluginDir string, runtime ...*internalplugins.RuntimeV2) *PluginHandler {
	// Create plugins directory if it doesn't exist
	if pluginDir != "" {
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			log.Printf("[PluginHandler] Warning: Failed to create plugins directory: %v", err)
		}
	}

	var pluginRuntime *internalplugins.RuntimeV2
	if len(runtime) > 0 {
		pluginRuntime = runtime[0]
	}
	return &PluginHandler{
		db:        database,
		pluginDir: pluginDir,
		runtime:   pluginRuntime,
	}
}

// downloadPluginFromRepository downloads a plugin from its repository to the local plugins directory.
// It prefers a packaged archive and only falls back to manifest-driven asset fetches.
func (h *PluginHandler) downloadPluginFromRepository(pluginName, repoURL, sourcePath string) error {
	if h.pluginDir == "" {
		log.Printf("[PluginHandler] No plugins directory configured, skipping download")
		return nil
	}

	if sourcePath == "" {
		sourcePath = pluginName
	}

	pluginPath := filepath.Join(h.pluginDir, pluginName)

	// Create plugin directory
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Try to download as archive first
	archiveURL := fmt.Sprintf("%s/%s/plugin.tar.gz", strings.TrimSuffix(repoURL, "/"), sourcePath)
	if err := h.downloadAndExtractArchive(archiveURL, pluginPath); err == nil {
		log.Printf("[PluginHandler] Downloaded plugin %s as archive", pluginName)
		return nil
	}

	// Fallback: download files referenced by manifest.json.
	log.Printf("[PluginHandler] Archive not available, downloading runtime assets for %s", pluginName)
	return h.downloadPluginFiles(pluginName, sourcePath, repoURL, pluginPath)
}

// downloadAndExtractArchive downloads a .tar.gz archive and extracts it to the target directory.
func (h *PluginHandler) downloadAndExtractArchive(url string, targetDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download archive: status %d", resp.StatusCode)
	}

	gzr, err := gzip.NewReader(resp.Body)
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

		// Sanitize the path to prevent directory traversal
		target := filepath.Join(targetDir, filepath.Clean(header.Name))
		if !strings.HasPrefix(target, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
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

// downloadPluginFiles downloads manifest-driven plugin assets from the repository.
func (h *PluginHandler) downloadPluginFiles(pluginName, sourcePath, repoURL, targetDir string) error {
	baseURL := strings.TrimSuffix(repoURL, "/")
	manifestURL := fmt.Sprintf("%s/%s/manifest.json", baseURL, sourcePath)
	manifestPath := filepath.Join(targetDir, "manifest.json")
	if err := h.downloadFile(manifestURL, manifestPath); err != nil {
		return fmt.Errorf("failed to download manifest.json: %w", err)
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest models.PluginManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	_ = h.downloadFile(fmt.Sprintf("%s/%s/README.md", baseURL, sourcePath), filepath.Join(targetDir, "README.md"))
	_ = h.downloadFile(fmt.Sprintf("%s/%s/LICENSE", baseURL, sourcePath), filepath.Join(targetDir, "LICENSE"))
	if manifest.Icon != "" {
		_ = h.downloadFile(
			fmt.Sprintf("%s/%s/%s", baseURL, sourcePath, manifest.Icon),
			filepath.Join(targetDir, filepath.Base(manifest.Icon)),
		)
	}

	entrypoints := handlerEntrypoints(manifest)
	if !handlerHasSharedObjectEntrypoint(entrypoints) {
		return fmt.Errorf(
			"plugin %s does not expose a runtime-loadable .so entrypoint; publish plugin.tar.gz or a bundle with .so assets",
			pluginName,
		)
	}

	for _, entry := range entrypoints {
		entryURL := fmt.Sprintf("%s/%s/%s", baseURL, sourcePath, entry)
		targetPath := filepath.Join(targetDir, filepath.Base(entry))
		if err := h.downloadFile(entryURL, targetPath); err != nil {
			return fmt.Errorf("failed to download plugin entrypoint %s: %w", entry, err)
		}
	}

	return nil
}

func handlerEntrypoints(manifest models.PluginManifest) []string {
	entrypoints := []string{
		manifest.Entrypoints.Main,
		manifest.Entrypoints.API,
		manifest.Entrypoints.UI,
		manifest.Entrypoints.Webhook,
		manifest.Entrypoints.CLI,
	}

	seen := make(map[string]struct{}, len(entrypoints))
	unique := make([]string, 0, len(entrypoints))
	for _, entry := range entrypoints {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		unique = append(unique, entry)
	}

	return unique
}

func handlerHasSharedObjectEntrypoint(entrypoints []string) bool {
	for _, entry := range entrypoints {
		if strings.HasSuffix(entry, ".so") {
			return true
		}
	}

	return false
}

// downloadFile downloads a single file from URL to the target path.
func (h *PluginHandler) downloadFile(url string, targetPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// RegisterRoutes registers plugin routes to the provided router group.
//
// Mounts all plugin endpoints under /plugins prefix:
//   - Catalog endpoints: /plugins/catalog, /plugins/catalog/:id, etc.
//   - Installed endpoints: /plugins, /plugins/:id, /plugins/:id/enable, etc.
//
// Parameters:
//   - r: Gin router group to mount routes on (typically /api)
//
// Example:
//
//	api := router.Group("/api")
//	handler.RegisterRoutes(api)
//	// Routes available at: /api/plugins/catalog, /api/plugins, etc.
func (h *PluginHandler) RegisterRoutes(r *gin.RouterGroup) {
	plugins := r.Group("/plugins")
	{
		// Plugin catalog
		plugins.GET("/catalog", h.BrowsePluginCatalog)
		plugins.GET("/catalog/:id", h.GetCatalogPlugin)
		plugins.POST("/catalog/:id/rate", h.RatePlugin)
		plugins.POST("/catalog/:id/install", h.InstallPlugin)

		// Installed plugins
		plugins.GET("", h.ListInstalledPlugins)
		plugins.GET("/:id", h.GetInstalledPlugin)
		plugins.PATCH("/:id", h.UpdateInstalledPlugin)
		plugins.DELETE("/:id", h.UninstallPlugin)
		plugins.POST("/:id/enable", h.EnablePlugin)
		plugins.POST("/:id/disable", h.DisablePlugin)
	}
}

// BrowsePluginCatalog browses available plugins from the catalog.
//
// Endpoint: GET /api/plugins/catalog
//
// Query Parameters:
//   - category: Filter by category (e.g., "analytics", "notifications")
//   - type: Filter by plugin type (e.g., "builtin", "community")
//   - search: Search in display_name, description, tags (case-insensitive)
//   - sort: Sort order (popular, rating, newest, name) - default: popular
//
// Response: JSON with plugins array and total count
//
// Example Requests:
//
//	GET /api/plugins/catalog?category=analytics&sort=rating
//	GET /api/plugins/catalog?search=slack&sort=popular
//	GET /api/plugins/catalog?type=builtin&sort=name
//
// Example Response:
//
//	{
//	  "plugins": [
//	    {
//	      "id": 1,
//	      "name": "analytics-tracker",
//	      "display_name": "Analytics Tracker",
//	      "description": "Track session usage metrics",
//	      "category": "analytics",
//	      "plugin_type": "community",
//	      "icon_url": "https://...",
//	      "tags": ["analytics", "metrics"],
//	      "install_count": 1500,
//	      "avg_rating": 4.5,
//	      "rating_count": 42
//	    }
//	  ],
//	  "total": 1
//	}
//
// Sorting Options:
//   - popular: By install count desc, then rating desc
//   - rating: By average rating desc, then rating count desc
//   - newest: By created_at desc
//   - name: By display_name asc
//
// HTTP Status Codes:
//   - 200: Success (may return empty array if no matches)
//   - 500: Database error
func (h *PluginHandler) BrowsePluginCatalog(c *gin.Context) {
	category := c.Query("category")
	pluginType := c.Query("type")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "popular") // popular, rating, newest, name

	query := `
		SELECT
			cp.id, cp.repository_id, cp.name, cp.version, cp.display_name,
			cp.description, cp.category, cp.plugin_type, cp.icon_url,
			cp.manifest, cp.tags, cp.install_count, cp.avg_rating, cp.rating_count,
			cp.created_at, cp.updated_at,
			r.id as repo_id, r.name as repo_name, r.url as repo_url, r.type as repo_type
		FROM catalog_plugins cp
		JOIN repositories r ON cp.repository_id = r.id
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	if category != "" {
		query += ` AND cp.category = $` + strconv.Itoa(argIndex)
		args = append(args, category)
		argIndex++
	}

	if pluginType != "" {
		query += ` AND cp.plugin_type = $` + strconv.Itoa(argIndex)
		args = append(args, pluginType)
		argIndex++
	}

	if search != "" {
		query += ` AND (cp.display_name ILIKE $` + strconv.Itoa(argIndex) +
			` OR cp.description ILIKE $` + strconv.Itoa(argIndex) +
			` OR $` + strconv.Itoa(argIndex) + ` = ANY(cp.tags))`
		args = append(args, "%"+search+"%")
	}

	// Sorting
	switch sortBy {
	case "popular":
		query += ` ORDER BY cp.install_count DESC, cp.avg_rating DESC`
	case "rating":
		query += ` ORDER BY cp.avg_rating DESC, cp.rating_count DESC`
	case "newest":
		query += ` ORDER BY cp.created_at DESC`
	case "name":
		query += ` ORDER BY cp.display_name ASC`
	default:
		query += ` ORDER BY cp.install_count DESC`
	}

	rows, err := h.db.DB().Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugins", "details": err.Error()})
		return
	}
	defer rows.Close()

	var plugins []models.CatalogPlugin
	for rows.Next() {
		var plugin models.CatalogPlugin
		var manifestJSON []byte
		var tags sql.NullString

		err := rows.Scan(
			&plugin.ID, &plugin.RepositoryID, &plugin.Name, &plugin.Version,
			&plugin.DisplayName, &plugin.Description, &plugin.Category, &plugin.PluginType,
			&plugin.IconURL, &manifestJSON, &tags, &plugin.InstallCount,
			&plugin.AvgRating, &plugin.RatingCount, &plugin.CreatedAt, &plugin.UpdatedAt,
			&plugin.Repository.ID, &plugin.Repository.Name, &plugin.Repository.URL, &plugin.Repository.Type,
		)
		if err != nil {
			continue
		}

		// Parse manifest
		if len(manifestJSON) > 0 {
			_ = json.Unmarshal(manifestJSON, &plugin.Manifest)
		}

		// Parse tags
		if tags.Valid {
			// PostgreSQL array format: {tag1,tag2,tag3}
			tagsStr := tags.String
			if len(tagsStr) > 2 {
				tagsStr = tagsStr[1 : len(tagsStr)-1] // Remove { }
				_ = json.Unmarshal([]byte(`["`+tagsStr+`"]`), &plugin.Tags)
			}
		}

		plugins = append(plugins, plugin)
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"total":   len(plugins),
	})
}

// GetCatalogPlugin gets a specific plugin from the catalog by ID.
//
// Endpoint: GET /api/plugins/catalog/:id
//
// Path Parameters:
//   - id: Catalog plugin ID
//
// Response: JSON with complete plugin details including repository info
//
// Side Effects:
//   - Increments view count asynchronously (non-blocking)
//   - Updates last_viewed_at timestamp
//
// Example Request:
//
//	GET /api/plugins/catalog/42
//
// Example Response:
//
//	{
//	  "id": 42,
//	  "name": "slack-notifications",
//	  "version": "1.2.3",
//	  "display_name": "Slack Notifications",
//	  "description": "Send session notifications to Slack",
//	  "category": "notifications",
//	  "plugin_type": "community",
//	  "icon_url": "https://...",
//	  "manifest": {...},
//	  "tags": ["notifications", "slack"],
//	  "install_count": 500,
//	  "avg_rating": 4.8,
//	  "rating_count": 20,
//	  "repository": {
//	    "id": 1,
//	    "name": "official",
//	    "url": "https://plugins.streamspace.io",
//	    "type": "official"
//	  }
//	}
//
// HTTP Status Codes:
//   - 200: Success
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) GetCatalogPlugin(c *gin.Context) {
	id := c.Param("id")

	query := `
		SELECT
			cp.id, cp.repository_id, cp.name, cp.version, cp.display_name,
			cp.description, cp.category, cp.plugin_type, cp.icon_url,
			cp.manifest, cp.tags, cp.install_count, cp.avg_rating, cp.rating_count,
			cp.created_at, cp.updated_at,
			r.id as repo_id, r.name as repo_name, r.url as repo_url, r.type as repo_type
		FROM catalog_plugins cp
		JOIN repositories r ON cp.repository_id = r.id
		WHERE cp.id = $1
	`

	var plugin models.CatalogPlugin
	var manifestJSON []byte
	var tags sql.NullString

	err := h.db.DB().QueryRow(query, id).Scan(
		&plugin.ID, &plugin.RepositoryID, &plugin.Name, &plugin.Version,
		&plugin.DisplayName, &plugin.Description, &plugin.Category, &plugin.PluginType,
		&plugin.IconURL, &manifestJSON, &tags, &plugin.InstallCount,
		&plugin.AvgRating, &plugin.RatingCount, &plugin.CreatedAt, &plugin.UpdatedAt,
		&plugin.Repository.ID, &plugin.Repository.Name, &plugin.Repository.URL, &plugin.Repository.Type,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	// Parse manifest
	if len(manifestJSON) > 0 {
		_ = json.Unmarshal(manifestJSON, &plugin.Manifest)
	}

	// Parse tags
	if tags.Valid {
		tagsStr := tags.String
		if len(tagsStr) > 2 {
			tagsStr = tagsStr[1 : len(tagsStr)-1]
			_ = json.Unmarshal([]byte(`["`+tagsStr+`"]`), &plugin.Tags)
		}
	}

	// Get view count and update stats
	go func() {
		_, _ = h.db.DB().Exec(`
			INSERT INTO plugin_stats (plugin_id, view_count, last_viewed_at)
			VALUES ($1, 1, $2)
			ON CONFLICT (plugin_id) DO UPDATE
			SET view_count = plugin_stats.view_count + 1,
			    last_viewed_at = $2,
			    updated_at = $2
		`, plugin.ID, time.Now())
	}()

	c.JSON(http.StatusOK, plugin)
}

// RatePlugin allows a user to rate a catalog plugin.
//
// Endpoint: POST /api/plugins/catalog/:id/rate
//
// Path Parameters:
//   - id: Catalog plugin ID to rate
//
// Request Body:
//
//	{
//	  "rating": 5,          // Required: 1-5 stars
//	  "review": "Great!"    // Optional: Text review
//	}
//
// Behavior:
//   - Upserts rating (inserts new or updates existing for this user)
//   - Updates plugin's avg_rating and rating_count
//   - user_id extracted from auth middleware (c.GetString("user_id"))
//
// Example Request:
//
//	POST /api/plugins/catalog/42/rate
//	{
//	  "rating": 5,
//	  "review": "Excellent plugin, works perfectly!"
//	}
//
// HTTP Status Codes:
//   - 200: Rating submitted successfully
//   - 400: Invalid rating (not 1-5) or invalid request body
//   - 500: Database error
func (h *PluginHandler) RatePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	userID := c.GetString("user_id") // From auth middleware

	var req models.RatePluginRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	// Insert or update rating
	_, err := h.db.DB().Exec(`
		INSERT INTO plugin_ratings (plugin_id, user_id, rating, review)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (plugin_id, user_id) DO UPDATE
		SET rating = $3, review = $4, updated_at = NOW()
	`, pluginID, userID, req.Rating, req.Review)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save rating", "details": err.Error()})
		return
	}

	// Update plugin average rating
	_, _ = h.db.DB().Exec(`
		UPDATE catalog_plugins
		SET avg_rating = (SELECT AVG(rating) FROM plugin_ratings WHERE plugin_id = $1),
		    rating_count = (SELECT COUNT(*) FROM plugin_ratings WHERE plugin_id = $1),
		    updated_at = NOW()
		WHERE id = $1
	`, pluginID)

	c.JSON(http.StatusOK, gin.H{"message": "Rating submitted successfully"})
}

// InstallPlugin installs a plugin from the catalog.
//
// Endpoint: POST /api/plugins/catalog/:id/install
//
// Path Parameters:
//   - id: Catalog plugin ID to install
//
// Request Body (optional):
//
//	{
//	  "config": {"api_key": "..."}  // Plugin-specific configuration
//	}
//
// Behavior:
//  1. Fetches plugin details from catalog_plugins
//  2. Checks if already installed (returns 409 if yes)
//  3. Inserts into installed_plugins with enabled=true
//  4. Increments install count asynchronously
//  5. Updates plugin_stats table
//
// Side Effects:
//   - Plugin install count incremented (async, non-blocking)
//   - Plugin stats updated with last_installed_at timestamp
//   - user_id saved as installed_by
//
// Example Request:
//
//	POST /api/plugins/catalog/42/install
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
//	  "message": "Plugin installed successfully",
//	  "pluginId": 123
//	}
//
// HTTP Status Codes:
//   - 201: Plugin installed successfully
//   - 404: Catalog plugin not found
//   - 409: Plugin already installed
//   - 500: Database error
func (h *PluginHandler) InstallPlugin(c *gin.Context) {
	catalogPluginID := c.Param("id")
	userID := c.GetString("user_id")

	var req models.InstallPluginRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	// Set default config if not provided
	if len(req.Config) == 0 {
		req.Config = json.RawMessage("{}")
	}

	// Get catalog plugin details with repository URL
	var catalogPlugin models.CatalogPlugin
	var manifestJSON []byte
	var repoURL sql.NullString
	var sourcePath sql.NullString
	err := h.db.DB().QueryRow(`
		SELECT cp.id, cp.name, cp.version, cp.display_name, cp.description, cp.plugin_type, cp.icon_url, cp.manifest, r.url, cp.source_path
		FROM catalog_plugins cp
		LEFT JOIN repositories r ON cp.repository_id = r.id
		WHERE cp.id = $1
	`, catalogPluginID).Scan(
		&catalogPlugin.ID, &catalogPlugin.Name, &catalogPlugin.Version,
		&catalogPlugin.DisplayName, &catalogPlugin.Description,
		&catalogPlugin.PluginType, &catalogPlugin.IconURL, &manifestJSON, &repoURL, &sourcePath,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found in catalog"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	// Parse manifest
	if len(manifestJSON) > 0 {
		_ = json.Unmarshal(manifestJSON, &catalogPlugin.Manifest)
	}

	// Check if already installed
	var existingID int
	err = h.db.DB().QueryRow(`
		SELECT id FROM installed_plugins WHERE name = $1
	`, catalogPlugin.Name).Scan(&existingID)

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Plugin already installed", "pluginId": existingID})
		return
	}

	// Install plugin
	var installedID int
	err = h.db.DB().QueryRow(`
		INSERT INTO installed_plugins (catalog_plugin_id, name, version, enabled, config, installed_by)
		VALUES ($1, $2, $3, true, $4, $5)
		RETURNING id
	`, catalogPlugin.ID, catalogPlugin.Name, catalogPlugin.Version, req.Config, userID).Scan(&installedID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to install plugin", "details": err.Error()})
		return
	}

	// Download plugin files to local plugins directory
	if repoURL.Valid && h.pluginDir != "" {
		go func() {
			if err := h.downloadPluginFromRepository(catalogPlugin.Name, repoURL.String, sourcePath.String); err != nil {
				log.Printf("[PluginHandler] Warning: Failed to download plugin files for %s: %v", catalogPlugin.Name, err)
			} else {
				log.Printf("[PluginHandler] Plugin files downloaded to %s/%s", h.pluginDir, catalogPlugin.Name)
			}
		}()
	}

	// Update install count
	go func() {
		_, _ = h.db.DB().Exec(`
			UPDATE catalog_plugins
			SET install_count = install_count + 1
			WHERE id = $1
		`, catalogPlugin.ID)

		_, _ = h.db.DB().Exec(`
			INSERT INTO plugin_stats (plugin_id, install_count, last_installed_at)
			VALUES ($1, 1, $2)
			ON CONFLICT (plugin_id) DO UPDATE
			SET install_count = plugin_stats.install_count + 1,
			    last_installed_at = $2,
			    updated_at = $2
		`, catalogPlugin.ID, time.Now())
	}()

	var warning string
	if h.runtime != nil {
		var config map[string]interface{}
		if len(req.Config) > 0 {
			if err := json.Unmarshal(req.Config, &config); err != nil {
				warning = "Plugin installed, but config could not be parsed for runtime load."
			}
		}
		if config == nil {
			config = make(map[string]interface{})
		}
		if err := h.runtime.LoadPluginWithConfig(c.Request.Context(), catalogPlugin.Name, catalogPlugin.Version, config, catalogPlugin.Manifest); err != nil {
			warning = err.Error()
		}
	}

	response := gin.H{
		"message":  "Plugin installed successfully",
		"pluginId": installedID,
	}
	if warning != "" {
		response["warning"] = warning
	}

	c.JSON(http.StatusCreated, response)
}

// ListInstalledPlugins lists all installed plugins.
//
// Endpoint: GET /api/plugins
//
// Query Parameters:
//   - enabled: Filter by enabled status ("true" for enabled only)
//
// Response: JSON with plugins array and total count
//
// Example Requests:
//
//	GET /api/plugins              // All installed plugins
//	GET /api/plugins?enabled=true // Only enabled plugins
//
// Example Response:
//
//	{
//	  "plugins": [
//	    {
//	      "id": 123,
//	      "catalog_plugin_id": 42,
//	      "name": "slack-notifications",
//	      "version": "1.2.3",
//	      "enabled": true,
//	      "config": {"webhook_url": "..."},
//	      "installed_by": "user123",
//	      "installed_at": "2025-01-15T10:30:00Z",
//	      "display_name": "Slack Notifications",
//	      "description": "...",
//	      "plugin_type": "community",
//	      "icon_url": "..."
//	    }
//	  ],
//	  "total": 1
//	}
//
// HTTP Status Codes:
//   - 200: Success (may return empty array if no plugins installed)
//   - 500: Database error
func (h *PluginHandler) ListInstalledPlugins(c *gin.Context) {
	enabledOnly := c.Query("enabled") == "true"

	query := `
		SELECT
			ip.id, ip.catalog_plugin_id, ip.name, ip.version, ip.enabled,
			ip.config, ip.installed_by, ip.installed_at, ip.updated_at,
			cp.display_name, cp.description, cp.plugin_type, cp.icon_url, cp.manifest
		FROM installed_plugins ip
		LEFT JOIN catalog_plugins cp ON ip.catalog_plugin_id = cp.id
	`

	if enabledOnly {
		query += ` WHERE ip.enabled = true`
	}

	query += ` ORDER BY ip.installed_at DESC`

	rows, err := h.db.DB().Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugins", "details": err.Error()})
		return
	}
	defer rows.Close()

	var plugins []models.InstalledPlugin
	for rows.Next() {
		var plugin models.InstalledPlugin
		var catalogPluginID sql.NullInt64
		var displayName, description, pluginType, iconURL sql.NullString
		var manifestJSON []byte

		err := rows.Scan(
			&plugin.ID, &catalogPluginID, &plugin.Name, &plugin.Version, &plugin.Enabled,
			&plugin.Config, &plugin.InstalledBy, &plugin.InstalledAt, &plugin.UpdatedAt,
			&displayName, &description, &pluginType, &iconURL, &manifestJSON,
		)
		if err != nil {
			continue
		}

		if catalogPluginID.Valid {
			id := int(catalogPluginID.Int64)
			plugin.CatalogPluginID = &id
		}

		if displayName.Valid {
			plugin.DisplayName = displayName.String
		}
		if description.Valid {
			plugin.Description = description.String
		}
		if pluginType.Valid {
			plugin.PluginType = pluginType.String
		}
		if iconURL.Valid {
			plugin.IconURL = iconURL.String
		}

		if len(manifestJSON) > 0 {
			var manifest models.PluginManifest
			if json.Unmarshal(manifestJSON, &manifest) == nil {
				plugin.Manifest = &manifest
			}
		}

		plugins = append(plugins, plugin)
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"total":   len(plugins),
	})
}

// GetInstalledPlugin gets details of a specific installed plugin.
//
// Endpoint: GET /api/plugins/:id
//
// Path Parameters:
//   - id: Installed plugin ID (not catalog ID)
//
// Response: JSON with complete plugin details
//
// Example Request:
//
//	GET /api/plugins/123
//
// HTTP Status Codes:
//   - 200: Success
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) GetInstalledPlugin(c *gin.Context) {
	id := c.Param("id")

	query := `
		SELECT
			ip.id, ip.catalog_plugin_id, ip.name, ip.version, ip.enabled,
			ip.config, ip.installed_by, ip.installed_at, ip.updated_at,
			cp.display_name, cp.description, cp.plugin_type, cp.icon_url, cp.manifest
		FROM installed_plugins ip
		LEFT JOIN catalog_plugins cp ON ip.catalog_plugin_id = cp.id
		WHERE ip.id = $1
	`

	var plugin models.InstalledPlugin
	var catalogPluginID sql.NullInt64
	var displayName, description, pluginType, iconURL sql.NullString
	var manifestJSON []byte

	err := h.db.DB().QueryRow(query, id).Scan(
		&plugin.ID, &catalogPluginID, &plugin.Name, &plugin.Version, &plugin.Enabled,
		&plugin.Config, &plugin.InstalledBy, &plugin.InstalledAt, &plugin.UpdatedAt,
		&displayName, &description, &pluginType, &iconURL, &manifestJSON,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	if catalogPluginID.Valid {
		id := int(catalogPluginID.Int64)
		plugin.CatalogPluginID = &id
	}

	if displayName.Valid {
		plugin.DisplayName = displayName.String
	}
	if description.Valid {
		plugin.Description = description.String
	}
	if pluginType.Valid {
		plugin.PluginType = pluginType.String
	}
	if iconURL.Valid {
		plugin.IconURL = iconURL.String
	}

	if len(manifestJSON) > 0 {
		var manifest models.PluginManifest
		if json.Unmarshal(manifestJSON, &manifest) == nil {
			plugin.Manifest = &manifest
		}
	}

	c.JSON(http.StatusOK, plugin)
}

// UpdateInstalledPlugin updates a plugin's configuration or enabled status.
//
// Endpoint: PATCH /api/plugins/:id
//
// Path Parameters:
//   - id: Installed plugin ID
//
// Request Body (all fields optional):
//
//	{
//	  "enabled": true,                // Enable/disable plugin
//	  "config": {"api_key": "new..."}  // Update configuration
//	}
//
// Behavior:
//   - Only provided fields are updated
//   - updated_at timestamp automatically set
//
// Example Request:
//
//	PATCH /api/plugins/123
//	{
//	  "config": {"webhook_url": "https://new-url.com"}
//	}
//
// HTTP Status Codes:
//   - 200: Plugin updated successfully
//   - 400: Invalid request body
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) UpdateInstalledPlugin(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdatePluginRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	query := `UPDATE installed_plugins SET `
	args := []interface{}{}
	argIndex := 1

	if req.Enabled != nil {
		query += `enabled = $` + strconv.Itoa(argIndex) + `, `
		args = append(args, *req.Enabled)
		argIndex++
	}

	if req.Config != nil {
		query += `config = $` + strconv.Itoa(argIndex) + `, `
		args = append(args, req.Config)
		argIndex++
	}

	query += `updated_at = NOW() WHERE id = $` + strconv.Itoa(argIndex)
	args = append(args, id)

	result, err := h.db.DB().Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update plugin", "details": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	var warning string
	if h.runtime != nil {
		var pluginName string
		if err := h.db.DB().QueryRow(`SELECT name FROM installed_plugins WHERE id = $1`, id).Scan(&pluginName); err == nil {
			if req.Enabled != nil && !*req.Enabled {
				if err := h.runtime.UnloadPlugin(c.Request.Context(), pluginName); err != nil {
					warning = err.Error()
				}
			} else if err := h.runtime.ReloadPlugin(c.Request.Context(), pluginName); err != nil {
				warning = err.Error()
			}
		}
	}

	response := gin.H{"message": "Plugin updated successfully"}
	if warning != "" {
		response["warning"] = warning
	}

	c.JSON(http.StatusOK, response)
}

// UninstallPlugin removes a plugin from the system.
//
// Endpoint: DELETE /api/plugins/:id
//
// Path Parameters:
//   - id: Installed plugin ID
//
// Behavior:
//   - Deletes plugin from installed_plugins table
//   - Plugin runtime should unload the plugin
//
// WARNING: This does not clean up plugin data tables or configuration.
// Plugin should implement cleanup in OnUnload hook.
//
// Example Request:
//
//	DELETE /api/plugins/123
//
// HTTP Status Codes:
//   - 200: Plugin uninstalled successfully
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) UninstallPlugin(c *gin.Context) {
	id := c.Param("id")

	// Get plugin name before deleting (for file cleanup)
	var pluginName string
	err := h.db.DB().QueryRow(`SELECT name FROM installed_plugins WHERE id = $1`, id).Scan(&pluginName)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	// Delete from database
	result, err := h.db.DB().Exec(`DELETE FROM installed_plugins WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to uninstall plugin", "details": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	warning := ""
	if h.runtime != nil {
		if err := h.runtime.UnloadPlugin(c.Request.Context(), pluginName); err != nil {
			warning = err.Error()
		}
	}

	// Remove plugin files from plugins directory
	if h.pluginDir != "" && pluginName != "" {
		pluginPath := filepath.Join(h.pluginDir, pluginName)
		if err := os.RemoveAll(pluginPath); err != nil {
			log.Printf("[PluginHandler] Warning: Failed to remove plugin files for %s: %v", pluginName, err)
		} else {
			log.Printf("[PluginHandler] Plugin files removed from %s", pluginPath)
		}
	}

	response := gin.H{"message": "Plugin uninstalled successfully"}
	if warning != "" {
		response["warning"] = warning
	}

	c.JSON(http.StatusOK, response)
}

// EnablePlugin enables an installed plugin.
//
// Endpoint: POST /api/plugins/:id/enable
//
// Path Parameters:
//   - id: Installed plugin ID
//
// Behavior:
//   - Sets enabled=true in database
//   - Plugin runtime should load the plugin on next startup/reload
//
// Example Request:
//
//	POST /api/plugins/123/enable
//
// HTTP Status Codes:
//   - 200: Plugin enabled successfully
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) EnablePlugin(c *gin.Context) {
	id := c.Param("id")

	var pluginName string
	if err := h.db.DB().QueryRow(`SELECT name FROM installed_plugins WHERE id = $1`, id).Scan(&pluginName); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	result, err := h.db.DB().Exec(`
		UPDATE installed_plugins
		SET enabled = true, updated_at = NOW()
		WHERE id = $1
	`, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable plugin", "details": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	response := gin.H{"message": "Plugin enabled successfully"}
	if h.runtime != nil {
		if err := h.runtime.LoadPluginByName(c.Request.Context(), pluginName); err != nil {
			response["warning"] = err.Error()
		}
	}

	c.JSON(http.StatusOK, response)
}

// DisablePlugin disables an installed plugin.
//
// Endpoint: POST /api/plugins/:id/disable
//
// Path Parameters:
//   - id: Installed plugin ID
//
// Behavior:
//   - Sets enabled=false in database
//   - Plugin runtime should unload the plugin on next reload
//
// Example Request:
//
//	POST /api/plugins/123/disable
//
// HTTP Status Codes:
//   - 200: Plugin disabled successfully
//   - 404: Plugin not found
//   - 500: Database error
func (h *PluginHandler) DisablePlugin(c *gin.Context) {
	id := c.Param("id")

	var pluginName string
	if err := h.db.DB().QueryRow(`SELECT name FROM installed_plugins WHERE id = $1`, id).Scan(&pluginName); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugin", "details": err.Error()})
		return
	}

	result, err := h.db.DB().Exec(`
		UPDATE installed_plugins
		SET enabled = false, updated_at = NOW()
		WHERE id = $1
	`, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable plugin", "details": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	response := gin.H{"message": "Plugin disabled successfully"}
	if h.runtime != nil {
		if err := h.runtime.UnloadPlugin(c.Request.Context(), pluginName); err != nil {
			response["warning"] = err.Error()
		}
	}

	c.JSON(http.StatusOK, response)
}
