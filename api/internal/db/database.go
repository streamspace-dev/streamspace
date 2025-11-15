package db

import (
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Database represents the database connection
type Database struct {
	db *sql.DB
}

// validateConfig validates database configuration to prevent SQL injection
func validateConfig(config Config) error {
	// Validate host (must be valid hostname or IP)
	if config.Host == "" {
		return fmt.Errorf("database host cannot be empty")
	}
	// Check if it's a valid IP or hostname
	if net.ParseIP(config.Host) == nil {
		// Not an IP, validate as hostname
		hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]{0,253}[a-zA-Z0-9])?$`)
		if !hostnameRegex.MatchString(config.Host) {
			return fmt.Errorf("invalid database host: %s", config.Host)
		}
	}

	// Validate port (must be numeric and in valid range)
	if config.Port == "" {
		return fmt.Errorf("database port cannot be empty")
	}
	port, err := strconv.Atoi(config.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid database port: %s (must be 1-65535)", config.Port)
	}

	// Validate user (alphanumeric, underscore, hyphen only)
	if config.User == "" {
		return fmt.Errorf("database user cannot be empty")
	}
	userRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !userRegex.MatchString(config.User) {
		return fmt.Errorf("invalid database user: %s (only alphanumeric, underscore, and hyphen allowed)", config.User)
	}

	// Validate database name (alphanumeric, underscore, hyphen only)
	if config.DBName == "" {
		return fmt.Errorf("database name cannot be empty")
	}
	dbNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !dbNameRegex.MatchString(config.DBName) {
		return fmt.Errorf("invalid database name: %s (only alphanumeric, underscore, and hyphen allowed)", config.DBName)
	}

	// Validate SSL mode (must be one of the allowed values)
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	if config.SSLMode != "" {
		valid := false
		for _, mode := range validSSLModes {
			if config.SSLMode == mode {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid SSL mode: %s (must be one of: %s)", config.SSLMode, strings.Join(validSSLModes, ", "))
		}
	}

	// SECURITY: Warn if SSL is disabled (insecure for production)
	if config.SSLMode == "" || config.SSLMode == "disable" {
		fmt.Println("WARNING: Database SSL/TLS is DISABLED - This is INSECURE for production!")
		fmt.Println("         Set DB_SSL_MODE to 'require', 'verify-ca', or 'verify-full'")
		fmt.Println("         Example: export DB_SSL_MODE=require")
	}

	return nil
}

// NewDatabase creates a new database connection with connection pooling
func NewDatabase(config Config) (*Database, error) {
	// SECURITY: Validate configuration to prevent SQL injection
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for optimal performance
	// These settings balance performance with resource usage
	db.SetMaxOpenConns(25)                // Maximum number of open connections to the database
	db.SetMaxIdleConns(5)                 // Maximum number of connections in the idle connection pool
	db.SetConnMaxLifetime(5 * 60 * 1000)  // Maximum amount of time a connection may be reused (5 minutes)
	db.SetConnMaxIdleTime(1 * 60 * 1000)  // Maximum amount of time a connection may be idle (1 minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying sql.DB
func (d *Database) DB() *sql.DB {
	return d.db
}

// Migrate runs database migrations
func (d *Database) Migrate() error {
	migrations := []string{
		// Users table (comprehensive user management)
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			full_name VARCHAR(255),
			role VARCHAR(50) DEFAULT 'user',
			provider VARCHAR(50) DEFAULT 'local',
			active BOOLEAN DEFAULT true,
			password_hash VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP
		)`,

		// User quotas table
		`CREATE TABLE IF NOT EXISTS user_quotas (
			user_id VARCHAR(255) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			max_sessions INT DEFAULT 5,
			max_cpu VARCHAR(50) DEFAULT '4000m',
			max_memory VARCHAR(50) DEFAULT '16Gi',
			max_storage VARCHAR(50) DEFAULT '100Gi',
			used_sessions INT DEFAULT 0,
			used_cpu VARCHAR(50) DEFAULT '0',
			used_memory VARCHAR(50) DEFAULT '0',
			used_storage VARCHAR(50) DEFAULT '0',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Groups table
		`CREATE TABLE IF NOT EXISTS groups (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL,
			display_name VARCHAR(255),
			description TEXT,
			type VARCHAR(50) DEFAULT 'team',
			parent_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Group quotas table
		`CREATE TABLE IF NOT EXISTS group_quotas (
			group_id VARCHAR(255) PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
			max_sessions INT DEFAULT 20,
			max_cpu VARCHAR(50) DEFAULT '16000m',
			max_memory VARCHAR(50) DEFAULT '64Gi',
			max_storage VARCHAR(50) DEFAULT '500Gi',
			used_sessions INT DEFAULT 0,
			used_cpu VARCHAR(50) DEFAULT '0',
			used_memory VARCHAR(50) DEFAULT '0',
			used_storage VARCHAR(50) DEFAULT '0',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Group memberships table (many-to-many users <-> groups)
		`CREATE TABLE IF NOT EXISTS group_memberships (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			group_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
			role VARCHAR(50) DEFAULT 'member',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, group_id)
		)`,

		// Create indexes for user/group management
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)`,
		`CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_type ON groups(type)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_parent_id ON groups(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_memberships_user_id ON group_memberships(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_memberships_group_id ON group_memberships(group_id)`,

		// Team role permissions (defines what each team role can do)
		`CREATE TABLE IF NOT EXISTS team_role_permissions (
			id SERIAL PRIMARY KEY,
			role VARCHAR(50) NOT NULL,
			permission VARCHAR(100) NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(role, permission)
		)`,

		// Insert default team role permissions
		`INSERT INTO team_role_permissions (role, permission, description) VALUES
			('owner', 'team.manage', 'Manage team settings and delete team'),
			('owner', 'team.members.manage', 'Add/remove team members and change roles'),
			('owner', 'team.sessions.create', 'Create new team sessions'),
			('owner', 'team.sessions.view', 'View all team sessions'),
			('owner', 'team.sessions.update', 'Update team session settings'),
			('owner', 'team.sessions.delete', 'Delete team sessions'),
			('owner', 'team.sessions.connect', 'Connect to team sessions'),
			('owner', 'team.quota.view', 'View team quota and usage'),
			('owner', 'team.quota.manage', 'Manage team resource quotas'),

			('admin', 'team.members.manage', 'Add/remove team members'),
			('admin', 'team.sessions.create', 'Create new team sessions'),
			('admin', 'team.sessions.view', 'View all team sessions'),
			('admin', 'team.sessions.update', 'Update team session settings'),
			('admin', 'team.sessions.delete', 'Delete team sessions'),
			('admin', 'team.sessions.connect', 'Connect to team sessions'),
			('admin', 'team.quota.view', 'View team quota and usage'),

			('member', 'team.sessions.create', 'Create new team sessions'),
			('member', 'team.sessions.view', 'View all team sessions'),
			('member', 'team.sessions.connect', 'Connect to team sessions'),
			('member', 'team.quota.view', 'View team quota and usage'),

			('viewer', 'team.sessions.view', 'View all team sessions'),
			('viewer', 'team.quota.view', 'View team quota and usage')
		ON CONFLICT (role, permission) DO NOTHING`,

		// Sessions table (cache of K8s Sessions)
		`CREATE TABLE IF NOT EXISTS sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			team_id VARCHAR(255) REFERENCES groups(id) ON DELETE SET NULL,
			template_name VARCHAR(255),
			state VARCHAR(50),
			app_type VARCHAR(50) DEFAULT 'desktop',
			active_connections INT DEFAULT 0,
			url TEXT,
			namespace VARCHAR(255) DEFAULT 'streamspace',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_connection TIMESTAMP,
			last_disconnect TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes on sessions
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_team_id ON sessions(team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state)`,

		// Connections table (active connections)
		`CREATE TABLE IF NOT EXISTS connections (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			client_ip VARCHAR(45),
			user_agent TEXT,
			connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_heartbeat TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create index on session_id
		`CREATE INDEX IF NOT EXISTS idx_connections_session_id ON connections(session_id)`,

		// Template repositories
		`CREATE TABLE IF NOT EXISTS repositories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE,
			url TEXT NOT NULL,
			branch VARCHAR(100) DEFAULT 'main',
			auth_type VARCHAR(50) DEFAULT 'none',
			auth_secret VARCHAR(255),
			last_sync TIMESTAMP,
			template_count INT DEFAULT 0,
			status VARCHAR(50) DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Catalog templates (cache of templates from repos)
		`CREATE TABLE IF NOT EXISTS catalog_templates (
			id SERIAL PRIMARY KEY,
			repository_id INT REFERENCES repositories(id) ON DELETE CASCADE,
			name VARCHAR(255),
			display_name VARCHAR(255),
			description TEXT,
			category VARCHAR(100),
			app_type VARCHAR(50) DEFAULT 'desktop',
			icon_url TEXT,
			manifest JSONB,
			tags TEXT[],
			install_count INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(repository_id, name)
		)`,

		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_category ON catalog_templates(category)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_app_type ON catalog_templates(app_type)`,

		// Template versions (track template version history)
		`CREATE TABLE IF NOT EXISTS template_versions (
			id SERIAL PRIMARY KEY,
			template_id INT REFERENCES catalog_templates(id) ON DELETE CASCADE,
			version VARCHAR(50) NOT NULL,
			changelog TEXT,
			manifest JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(template_id, version)
		)`,

		// Template ratings (user ratings for templates)
		`CREATE TABLE IF NOT EXISTS template_ratings (
			id SERIAL PRIMARY KEY,
			template_id INT REFERENCES catalog_templates(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			rating INT CHECK (rating >= 1 AND rating <= 5),
			review TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(template_id, user_id)
		)`,

		// Create indexes for ratings
		`CREATE INDEX IF NOT EXISTS idx_template_ratings_template_id ON template_ratings(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_ratings_user_id ON template_ratings(user_id)`,

		// Template statistics (view and usage tracking)
		`CREATE TABLE IF NOT EXISTS template_stats (
			template_id INT PRIMARY KEY REFERENCES catalog_templates(id) ON DELETE CASCADE,
			view_count INT DEFAULT 0,
			install_count INT DEFAULT 0,
			session_count INT DEFAULT 0,
			avg_rating DECIMAL(3,2) DEFAULT 0.0,
			rating_count INT DEFAULT 0,
			last_viewed TIMESTAMP,
			last_installed TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// User template favorites (bookmarks for quick access)
		`CREATE TABLE IF NOT EXISTS user_template_favorites (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			template_name VARCHAR(255) NOT NULL,
			favorited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, template_name)
		)`,

		// Create indexes for favorites
		`CREATE INDEX IF NOT EXISTS idx_user_template_favorites_user_id ON user_template_favorites(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_template_favorites_template ON user_template_favorites(template_name)`,

		// Featured templates (admin curated highlights)
		`CREATE TABLE IF NOT EXISTS featured_templates (
			id SERIAL PRIMARY KEY,
			template_id INT REFERENCES catalog_templates(id) ON DELETE CASCADE,
			title VARCHAR(255),
			description TEXT,
			display_order INT DEFAULT 0,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) REFERENCES users(id)
		)`,

		// Add featured column to catalog_templates
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS is_featured BOOLEAN DEFAULT false`,
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS version VARCHAR(50) DEFAULT '1.0.0'`,
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS view_count INT DEFAULT 0`,
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS avg_rating DECIMAL(3,2) DEFAULT 0.0`,
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS rating_count INT DEFAULT 0`,

		// Create indexes for featured templates
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_featured ON catalog_templates(is_featured)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_rating ON catalog_templates(avg_rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_views ON catalog_templates(view_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_installs ON catalog_templates(install_count DESC)`,

		// Configuration table
		`CREATE TABLE IF NOT EXISTS configuration (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT,
			type VARCHAR(50) DEFAULT 'string',
			category VARCHAR(100),
			description TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_by VARCHAR(255)
		)`,

		// Audit log
		`CREATE TABLE IF NOT EXISTS audit_log (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255),
			action VARCHAR(100),
			resource_type VARCHAR(100),
			resource_id VARCHAR(255),
			changes JSONB,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ip_address VARCHAR(45)
		)`,

		// Create index on timestamp for audit log
		`CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON audit_log(user_id)`,

		// Insert default admin user if not exists
		`INSERT INTO users (id, username, email, full_name, role, provider, active)
		VALUES ('admin', 'admin', 'admin@streamspace.local', 'Administrator', 'admin', 'local', true)
		ON CONFLICT (id) DO NOTHING`,

		// Insert default admin user quota
		`INSERT INTO user_quotas (user_id, max_sessions, max_cpu, max_memory, max_storage)
		VALUES ('admin', 100, '64000m', '256Gi', '1Ti')
		ON CONFLICT (user_id) DO NOTHING`,

		// Insert default configuration values
		`INSERT INTO configuration (key, value, category, description) VALUES
			('ingress.domain', 'streamspace.local', 'ingress', 'Default ingress domain'),
			('ingress.class', 'traefik', 'ingress', 'Ingress class to use'),
			('storage.defaultSize', '50Gi', 'storage', 'Default PVC size'),
			('storage.className', 'nfs-client', 'storage', 'Storage class name'),
			('resources.defaultMemory', '2Gi', 'resources', 'Default memory limit'),
			('resources.defaultCPU', '1000m', 'resources', 'Default CPU limit'),
			('features.enableMetrics', 'true', 'features', 'Enable Prometheus metrics'),
			('features.enableIngress', 'true', 'features', 'Enable ingress creation'),
			('session.defaultIdleTimeout', '30m', 'session', 'Default idle timeout'),
			('session.enableAutoHibernation', 'true', 'session', 'Enable auto-hibernation')
		ON CONFLICT (key) DO NOTHING`,

		// Session shares (collaboration)
		`CREATE TABLE IF NOT EXISTS session_shares (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			owner_user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			shared_with_user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			permission_level VARCHAR(50) DEFAULT 'view',
			share_token VARCHAR(255) UNIQUE,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			accepted_at TIMESTAMP,
			revoked_at TIMESTAMP,
			UNIQUE(session_id, shared_with_user_id)
		)`,

		// Create indexes for session shares
		`CREATE INDEX IF NOT EXISTS idx_session_shares_session_id ON session_shares(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_shares_owner ON session_shares(owner_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_shares_shared_with ON session_shares(shared_with_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token)`,

		// Session share invitations (for email/link based sharing)
		`CREATE TABLE IF NOT EXISTS session_share_invitations (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			invitation_token VARCHAR(255) UNIQUE NOT NULL,
			permission_level VARCHAR(50) DEFAULT 'view',
			max_uses INT DEFAULT 1,
			use_count INT DEFAULT 0,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create index for invitations
		`CREATE INDEX IF NOT EXISTS idx_session_share_invitations_session_id ON session_share_invitations(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_share_invitations_token ON session_share_invitations(invitation_token)`,

		// Session collaborators (active participants)
		`CREATE TABLE IF NOT EXISTS session_collaborators (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			permission_level VARCHAR(50) DEFAULT 'view',
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT true,
			UNIQUE(session_id, user_id)
		)`,

		// Create indexes for collaborators
		`CREATE INDEX IF NOT EXISTS idx_session_collaborators_session_id ON session_collaborators(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_collaborators_user_id ON session_collaborators(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_collaborators_active ON session_collaborators(is_active)`,

		// Performance optimization: Composite indexes for common query patterns
		// Sessions by user and state (dashboard queries showing active/hibernated sessions)
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_state ON sessions(user_id, state)`,

		// Audit log by user and timestamp (user activity history queries)
		`CREATE INDEX IF NOT EXISTS idx_audit_log_user_timestamp ON audit_log(user_id, timestamp DESC)`,

		// Session shares - active shares (not revoked, not expired)
		`CREATE INDEX IF NOT EXISTS idx_session_shares_active ON session_shares(session_id, shared_with_user_id) WHERE revoked_at IS NULL`,

		// Catalog templates by category and rating (catalog page with sorting)
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_category_rating ON catalog_templates(category, avg_rating DESC)`,

		// Catalog templates by category and popularity (catalog page sorted by installs)
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_category_installs ON catalog_templates(category, install_count DESC)`,

		// Session collaborators - active collaborators by session (real-time collaboration queries)
		`CREATE INDEX IF NOT EXISTS idx_session_collaborators_session_active ON session_collaborators(session_id, is_active) WHERE is_active = true`,

		// Connections by session and heartbeat (detecting stale connections)
		`CREATE INDEX IF NOT EXISTS idx_connections_session_heartbeat ON connections(session_id, last_heartbeat DESC)`,

		// Templates by app_type and rating (filtering by app type with sorting)
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_apptype_rating ON catalog_templates(app_type, avg_rating DESC)`,

		// ========== Session Activity Recording ==========

		// Session activity log (detailed event tracking for compliance and analytics)
		`CREATE TABLE IF NOT EXISTS session_activity_log (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			event_type VARCHAR(100) NOT NULL,
			event_category VARCHAR(50) DEFAULT 'general',
			description TEXT,
			metadata JSONB,
			ip_address VARCHAR(45),
			user_agent TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for session activity queries
		`CREATE INDEX IF NOT EXISTS idx_session_activity_session_id ON session_activity_log(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_activity_user_id ON session_activity_log(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_activity_timestamp ON session_activity_log(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_session_activity_event_type ON session_activity_log(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_session_activity_category ON session_activity_log(event_category)`,

		// Composite index for session activity timeline queries
		`CREATE INDEX IF NOT EXISTS idx_session_activity_session_timestamp ON session_activity_log(session_id, timestamp DESC)`,

		// Session recordings metadata (for future video/screen recording feature)
		`CREATE TABLE IF NOT EXISTS session_recordings (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			recording_type VARCHAR(50) DEFAULT 'screen',
			storage_path TEXT,
			file_size_bytes BIGINT DEFAULT 0,
			duration_seconds INT DEFAULT 0,
			started_at TIMESTAMP,
			ended_at TIMESTAMP,
			status VARCHAR(50) DEFAULT 'recording',
			error_message TEXT,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for recordings
		`CREATE INDEX IF NOT EXISTS idx_session_recordings_session_id ON session_recordings(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_recordings_status ON session_recordings(status)`,
		`CREATE INDEX IF NOT EXISTS idx_session_recordings_created_at ON session_recordings(created_at DESC)`,

		// ========== Plugin System ==========

		// Catalog plugins (available plugins from repositories)
		`CREATE TABLE IF NOT EXISTS catalog_plugins (
			id SERIAL PRIMARY KEY,
			repository_id INT REFERENCES repositories(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			version VARCHAR(50) NOT NULL,
			display_name VARCHAR(255),
			description TEXT,
			category VARCHAR(100),
			plugin_type VARCHAR(50) DEFAULT 'extension',
			icon_url TEXT,
			manifest JSONB,
			tags TEXT[],
			install_count INT DEFAULT 0,
			avg_rating DECIMAL(3,2) DEFAULT 0.00,
			rating_count INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(repository_id, name, version)
		)`,

		// Create indexes for plugins
		`CREATE INDEX IF NOT EXISTS idx_catalog_plugins_category ON catalog_plugins(category)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_plugins_type ON catalog_plugins(plugin_type)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_plugins_category_rating ON catalog_plugins(category, avg_rating DESC)`,

		// Installed plugins (user-installed plugins)
		`CREATE TABLE IF NOT EXISTS installed_plugins (
			id SERIAL PRIMARY KEY,
			catalog_plugin_id INT REFERENCES catalog_plugins(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			version VARCHAR(50) NOT NULL,
			enabled BOOLEAN DEFAULT true,
			config JSONB,
			installed_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name)
		)`,

		// Create indexes for installed plugins
		`CREATE INDEX IF NOT EXISTS idx_installed_plugins_enabled ON installed_plugins(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_installed_plugins_user ON installed_plugins(installed_by)`,

		// Plugin versions (track plugin version history)
		`CREATE TABLE IF NOT EXISTS plugin_versions (
			id SERIAL PRIMARY KEY,
			plugin_id INT REFERENCES catalog_plugins(id) ON DELETE CASCADE,
			version VARCHAR(50) NOT NULL,
			changelog TEXT,
			manifest JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(plugin_id, version)
		)`,

		// Plugin ratings (user ratings for plugins)
		`CREATE TABLE IF NOT EXISTS plugin_ratings (
			id SERIAL PRIMARY KEY,
			plugin_id INT REFERENCES catalog_plugins(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			rating INT CHECK (rating >= 1 AND rating <= 5),
			review TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(plugin_id, user_id)
		)`,

		// Create indexes for plugin ratings
		`CREATE INDEX IF NOT EXISTS idx_plugin_ratings_plugin_id ON plugin_ratings(plugin_id)`,
		`CREATE INDEX IF NOT EXISTS idx_plugin_ratings_user_id ON plugin_ratings(user_id)`,

		// Plugin statistics (view and usage tracking)
		`CREATE TABLE IF NOT EXISTS plugin_stats (
			plugin_id INT PRIMARY KEY REFERENCES catalog_plugins(id) ON DELETE CASCADE,
			view_count INT DEFAULT 0,
			install_count INT DEFAULT 0,
			last_viewed_at TIMESTAMP,
			last_installed_at TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// ========== API Key Management ==========

		// API keys for programmatic access and integrations
		`CREATE TABLE IF NOT EXISTS api_keys (
			id SERIAL PRIMARY KEY,
			key_hash VARCHAR(255) UNIQUE NOT NULL,
			key_prefix VARCHAR(20) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			scopes TEXT[],
			rate_limit INT DEFAULT 1000,
			expires_at TIMESTAMP,
			last_used_at TIMESTAMP,
			use_count INT DEFAULT 0,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL
		)`,

		// Create indexes for API keys
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at)`,

		// API key usage log (for auditing and rate limiting)
		`CREATE TABLE IF NOT EXISTS api_key_usage_log (
			id SERIAL PRIMARY KEY,
			api_key_id INT REFERENCES api_keys(id) ON DELETE CASCADE,
			endpoint VARCHAR(255),
			method VARCHAR(10),
			status_code INT,
			ip_address VARCHAR(45),
			user_agent TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for usage log
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_log_api_key_id ON api_key_usage_log(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_log_timestamp ON api_key_usage_log(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_log_key_timestamp ON api_key_usage_log(api_key_id, timestamp DESC)`,

		// ========== User Preferences & Settings ==========

		// User preferences (flexible JSONB storage for UI settings, notification preferences, defaults)
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id VARCHAR(255) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			preferences JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// User favorite templates (quick access to frequently used templates)
		`CREATE TABLE IF NOT EXISTS user_favorite_templates (
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			template_name VARCHAR(255) NOT NULL,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, template_name)
		)`,

		// Create indexes for user preferences
		`CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id ON user_preferences(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_favorite_templates_user_id ON user_favorite_templates(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_favorite_templates_template ON user_favorite_templates(template_name)`,

		// ========== Notifications System ==========

		// In-app notifications (stored notifications for users)
		`CREATE TABLE IF NOT EXISTS notifications (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(100) NOT NULL,
			title VARCHAR(500) NOT NULL,
			message TEXT NOT NULL,
			data JSONB DEFAULT '{}',
			priority VARCHAR(20) DEFAULT 'normal',
			is_read BOOLEAN DEFAULT false,
			action_url TEXT,
			action_text VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			read_at TIMESTAMP
		)`,

		// Create indexes for notifications
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_priority ON notifications(priority)`,

		// Composite index for unread notifications query (most common)
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC) WHERE is_read = false`,

		// Notification delivery log (tracks webhook/email delivery attempts)
		`CREATE TABLE IF NOT EXISTS notification_delivery_log (
			id SERIAL PRIMARY KEY,
			notification_id VARCHAR(255) REFERENCES notifications(id) ON DELETE CASCADE,
			channel VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			error_message TEXT,
			delivered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for delivery log
		`CREATE INDEX IF NOT EXISTS idx_notification_delivery_log_notification_id ON notification_delivery_log(notification_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_delivery_log_channel ON notification_delivery_log(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_delivery_log_status ON notification_delivery_log(status)`,

		// ========== Advanced Search & Filtering ==========

		// Saved searches (user-defined search queries)
		`CREATE TABLE IF NOT EXISTS saved_searches (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			query TEXT NOT NULL,
			filters JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for saved searches
		`CREATE INDEX IF NOT EXISTS idx_saved_searches_user_id ON saved_searches(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_searches_updated_at ON saved_searches(updated_at DESC)`,

		// Search history (recent user searches for suggestions and analytics)
		`CREATE TABLE IF NOT EXISTS search_history (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			query TEXT NOT NULL,
			search_type VARCHAR(50) DEFAULT 'universal',
			filters JSONB DEFAULT '{}',
			searched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for search history
		`CREATE INDEX IF NOT EXISTS idx_search_history_user_id ON search_history(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_search_history_searched_at ON search_history(searched_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_search_history_query ON search_history(query)`,

		// Composite index for user search history queries
		`CREATE INDEX IF NOT EXISTS idx_search_history_user_time ON search_history(user_id, searched_at DESC)`,

		// ========== Session Snapshots & Restore ==========

		// Session snapshots (point-in-time session backups)
		`CREATE TABLE IF NOT EXISTS session_snapshots (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			type VARCHAR(50) DEFAULT 'manual',
			status VARCHAR(50) DEFAULT 'creating',
			storage_path TEXT,
			size_bytes BIGINT DEFAULT 0,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			expires_at TIMESTAMP,
			error_message TEXT
		)`,

		// Create indexes for snapshots
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_session_id ON session_snapshots(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_user_id ON session_snapshots(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_status ON session_snapshots(status)`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_type ON session_snapshots(type)`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_created_at ON session_snapshots(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_expires_at ON session_snapshots(expires_at)`,

		// Composite index for user's available snapshots
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_user_available ON session_snapshots(user_id, status) WHERE status = 'available'`,

		// Snapshot restore jobs (tracks restore operations)
		`CREATE TABLE IF NOT EXISTS snapshot_restore_jobs (
			id VARCHAR(255) PRIMARY KEY,
			snapshot_id VARCHAR(255) REFERENCES session_snapshots(id) ON DELETE CASCADE,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE SET NULL,
			target_session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE SET NULL,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			status VARCHAR(50) DEFAULT 'pending',
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			error_message TEXT
		)`,

		// Create indexes for restore jobs
		`CREATE INDEX IF NOT EXISTS idx_snapshot_restore_jobs_snapshot_id ON snapshot_restore_jobs(snapshot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_restore_jobs_user_id ON snapshot_restore_jobs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_restore_jobs_status ON snapshot_restore_jobs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_restore_jobs_started_at ON snapshot_restore_jobs(started_at DESC)`,

		// Add snapshot_config column to sessions table
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS snapshot_config JSONB DEFAULT '{}'`,
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	return nil
}
