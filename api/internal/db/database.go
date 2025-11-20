// Package db provides PostgreSQL database access and management for StreamSpace.
//
// This file implements the core database connection and lifecycle management.
//
// Purpose:
// - Establish and maintain PostgreSQL connection pool
// - Initialize database schema on startup (82+ tables)
// - Provide centralized database instance for all API handlers
// - Execute raw SQL queries and transactions
// - Validate database configuration for security
//
// Features:
// - Connection pooling with configurable limits (25 max open, 5 max idle)
// - Comprehensive schema migrations (82+ tables, 200+ indexes)
// - Health check and ping capabilities
// - Graceful connection cleanup on shutdown
// - Configuration validation (prevents SQL injection in connection strings)
// - SSL/TLS warnings for production security
//
// Database Schema:
//   - 82+ tables covering users, sessions, templates, plugins, quotas, audit logs
//   - 200+ indexes for query performance optimization
//   - JSONB columns for flexible metadata storage
//   - Foreign key constraints for referential integrity
//   - Composite indexes for common query patterns
//
// Implementation Details:
// - Uses database/sql with lib/pq PostgreSQL driver
// - Connection pool configured for optimal performance (5min max lifetime)
// - Schema initialization runs CREATE TABLE IF NOT EXISTS on startup
// - Thread-safe connection pooling handled by database/sql package
// - Validates hostname, port, username, database name, SSL mode
//
// Thread Safety:
// - Database connections are thread-safe and managed by database/sql pool
// - Safe for concurrent use across multiple goroutines
// - Connection pool prevents resource exhaustion
//
// Dependencies:
// - PostgreSQL 12+ (required)
// - lib/pq driver for database/sql
//
// Example Usage:
//
//	config := db.Config{
//	    Host:     "localhost",
//	    Port:     "5432",
//	    User:     "streamspace",
//	    Password: "secretpassword",
//	    DBName:   "streamspace",
//	    SSLMode:  "require",
//	}
//
//	database, err := db.NewDatabase(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer database.Close()
//
//	// Run migrations
//	if err := database.Migrate(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use database connection
//	rows, err := database.DB().Query("SELECT * FROM users")
package db

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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
	db.SetMaxOpenConns(25)                     // Maximum number of open connections to the database
	db.SetMaxIdleConns(5)                      // Maximum number of connections in the idle connection pool
	db.SetConnMaxLifetime(5 * time.Minute)     // Maximum amount of time a connection may be reused (5 minutes)
	db.SetConnMaxIdleTime(1 * time.Minute)     // Maximum amount of time a connection may be idle (1 minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}

// NewDatabaseForTesting creates a Database from an existing sql.DB connection.
// This constructor is intended ONLY FOR TESTING to enable dependency injection
// with mock databases (e.g., sqlmock).
//
// DO NOT use this in production code. Use NewDatabase() instead.
//
// Example usage in tests:
//
//	db, mock, err := sqlmock.New()
//	database := db.NewDatabaseForTesting(db)
//	handler := &AuditHandler{database: database}
//	// ... setup mock expectations and run tests
func NewDatabaseForTesting(db *sql.DB) *Database {
	return &Database{db: db}
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

		// Template and plugin repositories
		`CREATE TABLE IF NOT EXISTS repositories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE,
			url TEXT NOT NULL,
			branch VARCHAR(100) DEFAULT 'main',
			type VARCHAR(50) DEFAULT 'template',
			auth_type VARCHAR(50) DEFAULT 'none',
			auth_secret VARCHAR(255),
			last_sync TIMESTAMP,
			template_count INT DEFAULT 0,
			status VARCHAR(50) DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Insert default repositories (plugins and templates)
		`INSERT INTO repositories (name, url, branch, type, auth_type, status) VALUES
			('Official Plugins', 'https://github.com/JoshuaAFerguson/streamspace-plugins', 'main', 'plugin', 'none', 'pending'),
			('Official Templates', 'https://github.com/JoshuaAFerguson/streamspace-templates', 'main', 'template', 'none', 'pending')
		ON CONFLICT (name) DO NOTHING`,

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

		// Catalog template versions (track version history from repositories)
		`CREATE TABLE IF NOT EXISTS catalog_template_versions (
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

		// Add platform column for multi-platform support (kubernetes, docker, hyperv, vcenter)
		// Defaults to 'kubernetes' for backward compatibility
		`ALTER TABLE catalog_templates ADD COLUMN IF NOT EXISTS platform VARCHAR(50) DEFAULT 'kubernetes'`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_platform ON catalog_templates(platform)`,

		// Create indexes for featured templates
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_featured ON catalog_templates(is_featured)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_rating ON catalog_templates(avg_rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_views ON catalog_templates(view_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_templates_installs ON catalog_templates(install_count DESC)`,

		// Installed applications (applications installed from catalog templates)
		`CREATE TABLE IF NOT EXISTS installed_applications (
			id VARCHAR(255) PRIMARY KEY,
			catalog_template_id INT REFERENCES catalog_templates(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			display_name VARCHAR(255) NOT NULL,
			description TEXT,
			category VARCHAR(100),
			folder_path VARCHAR(255),
			enabled BOOLEAN DEFAULT true,
			configuration JSONB DEFAULT '{}',
			icon_url TEXT,
			icon_data BYTEA,
			icon_media_type VARCHAR(100),
			manifest JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for installed applications
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_template ON installed_applications(catalog_template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_enabled ON installed_applications(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_created_by ON installed_applications(created_by)`,

		// Application group access (controls which groups can access which applications)
		`CREATE TABLE IF NOT EXISTS application_group_access (
			id VARCHAR(255) PRIMARY KEY,
			application_id VARCHAR(255) REFERENCES installed_applications(id) ON DELETE CASCADE,
			group_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
			access_level VARCHAR(50) DEFAULT 'launch',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(application_id, group_id)
		)`,

		// Create indexes for application group access
		`CREATE INDEX IF NOT EXISTS idx_application_group_access_app ON application_group_access(application_id)`,
		`CREATE INDEX IF NOT EXISTS idx_application_group_access_group ON application_group_access(group_id)`,

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

		// Insert default all_users group that all users belong to
		`INSERT INTO groups (id, name, display_name, description, type)
		VALUES ('all-users', 'all_users', 'All Users', 'Default group containing all users', 'system')
		ON CONFLICT (name) DO NOTHING`,

		// Add admin user to all_users group
		`INSERT INTO group_memberships (id, user_id, group_id, role, created_at)
		SELECT 'admin-all-users', 'admin', id, 'member', NOW()
		FROM groups WHERE name = 'all_users'
		ON CONFLICT (user_id, group_id) DO NOTHING`,

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

		// Template shares (for sharing user-defined templates)
		`CREATE TABLE IF NOT EXISTS template_shares (
			id VARCHAR(255) PRIMARY KEY,
			template_id VARCHAR(255) NOT NULL,
			shared_by VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			shared_with_user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			shared_with_team_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
			permission_level VARCHAR(50) NOT NULL DEFAULT 'read',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			revoked_at TIMESTAMP,
			CONSTRAINT chk_template_shared_with CHECK (
				(shared_with_user_id IS NOT NULL AND shared_with_team_id IS NULL) OR
				(shared_with_user_id IS NULL AND shared_with_team_id IS NOT NULL)
			),
			UNIQUE(template_id, shared_with_user_id),
			UNIQUE(template_id, shared_with_team_id)
		)`,

		// Create indexes for template shares
		`CREATE INDEX IF NOT EXISTS idx_template_shares_template_id ON template_shares(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_shares_shared_by ON template_shares(shared_by)`,
		`CREATE INDEX IF NOT EXISTS idx_template_shares_user ON template_shares(shared_with_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_shares_team ON template_shares(shared_with_team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_shares_active ON template_shares(template_id) WHERE revoked_at IS NULL`,

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

		// ========== Session Templates & Presets ==========

		// User session templates (custom reusable session configurations)
		`CREATE TABLE IF NOT EXISTS user_session_templates (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			team_id VARCHAR(255) REFERENCES groups(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			icon VARCHAR(500),
			category VARCHAR(100),
			tags JSONB DEFAULT '[]',
			visibility VARCHAR(50) DEFAULT 'private',
			base_template VARCHAR(255) NOT NULL,
			configuration JSONB DEFAULT '{}',
			resources JSONB DEFAULT '{}',
			environment JSONB DEFAULT '{}',
			is_default BOOLEAN DEFAULT false,
			usage_count INT DEFAULT 0,
			version VARCHAR(50) DEFAULT '1.0.0',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for user session templates
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_user_id ON user_session_templates(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_team_id ON user_session_templates(team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_visibility ON user_session_templates(visibility)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_category ON user_session_templates(category)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_usage_count ON user_session_templates(usage_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_is_default ON user_session_templates(is_default) WHERE is_default = true`,

		// Composite index for user's default templates
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_user_default ON user_session_templates(user_id, is_default) WHERE is_default = true`,

		// Composite index for public templates sorted by usage
		`CREATE INDEX IF NOT EXISTS idx_user_session_templates_public_usage ON user_session_templates(visibility, usage_count DESC) WHERE visibility = 'public'`,

		// User session template versions (version control for user templates)
		`CREATE TABLE IF NOT EXISTS user_session_template_versions (
			id SERIAL PRIMARY KEY,
			template_id VARCHAR(255) NOT NULL,
			version_number INT NOT NULL,
			template_data JSONB NOT NULL,
			description TEXT,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			tags TEXT[],
			UNIQUE(template_id, version_number)
		)`,

		// Create indexes for template versions
		`CREATE INDEX IF NOT EXISTS idx_user_session_template_versions_template_id ON user_session_template_versions(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_template_versions_created_by ON user_session_template_versions(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_user_session_template_versions_created_at ON user_session_template_versions(created_at DESC)`,

		// ========== Batch Operations ==========

		// Batch operations table (tracks bulk operation jobs)
		`CREATE TABLE IF NOT EXISTS batch_operations (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			operation_type VARCHAR(100) NOT NULL,
			resource_type VARCHAR(100) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			total_items INT DEFAULT 0,
			processed_items INT DEFAULT 0,
			success_count INT DEFAULT 0,
			failure_count INT DEFAULT 0,
			errors JSONB DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP
		)`,

		// Create indexes for batch operations
		`CREATE INDEX IF NOT EXISTS idx_batch_operations_user_id ON batch_operations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_batch_operations_status ON batch_operations(status)`,
		`CREATE INDEX IF NOT EXISTS idx_batch_operations_created_at ON batch_operations(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_batch_operations_operation_type ON batch_operations(operation_type)`,

		// Composite index for user's active operations
		`CREATE INDEX IF NOT EXISTS idx_batch_operations_user_status ON batch_operations(user_id, status) WHERE status IN ('pending', 'processing')`,

		// ========== Advanced Monitoring & Metrics ==========

		// Monitoring alerts table (tracks system alerts and incidents)
		`CREATE TABLE IF NOT EXISTS monitoring_alerts (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			severity VARCHAR(50) NOT NULL,
			status VARCHAR(50) DEFAULT 'active',
			condition VARCHAR(255) NOT NULL,
			threshold FLOAT NOT NULL,
			triggered_at TIMESTAMP,
			acknowledged_at TIMESTAMP,
			resolved_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for monitoring alerts
		`CREATE INDEX IF NOT EXISTS idx_monitoring_alerts_status ON monitoring_alerts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_alerts_severity ON monitoring_alerts(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_alerts_triggered_at ON monitoring_alerts(triggered_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_alerts_created_at ON monitoring_alerts(created_at DESC)`,

		// Composite index for active alerts by severity
		`CREATE INDEX IF NOT EXISTS idx_monitoring_alerts_active_severity ON monitoring_alerts(status, severity) WHERE status IN ('active', 'triggered')`,

		// ========== Resource Quotas & Limits Enforcement ==========

		// Resource quotas table (user and team resource limits)
		`CREATE TABLE IF NOT EXISTS resource_quotas (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			team_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
			max_sessions INT,
			max_cpu INT,
			max_memory INT,
			max_storage INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for resource quotas
		`CREATE INDEX IF NOT EXISTS idx_resource_quotas_user_id ON resource_quotas(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_quotas_team_id ON resource_quotas(team_id)`,

		// Unique constraint for user quotas (handle NULL team_id with partial indexes)
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_quotas_user_team ON resource_quotas(user_id, team_id) WHERE team_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_quotas_user_only ON resource_quotas(user_id) WHERE team_id IS NULL`,

		// Quota policies table (defines quota enforcement rules)
		`CREATE TABLE IF NOT EXISTS quota_policies (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			rules TEXT NOT NULL,
			priority INT DEFAULT 0,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for quota policies
		`CREATE INDEX IF NOT EXISTS idx_quota_policies_priority ON quota_policies(priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_quota_policies_enabled ON quota_policies(enabled) WHERE enabled = true`,

		// ========== Cost Management & Billing ==========

		// Invoices table (billing invoices)
		`CREATE TABLE IF NOT EXISTS invoices (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			invoice_number VARCHAR(100) NOT NULL UNIQUE,
			period_start TIMESTAMP NOT NULL,
			period_end TIMESTAMP NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			paid_at TIMESTAMP
		)`,

		// Create indexes for invoices
		`CREATE INDEX IF NOT EXISTS idx_invoices_user_id ON invoices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_created_at ON invoices(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_invoice_number ON invoices(invoice_number)`,

		// Payment methods table (stored payment methods)
		`CREATE TABLE IF NOT EXISTS payment_methods (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			last4 VARCHAR(4) NOT NULL,
			is_default BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for payment methods
		`CREATE INDEX IF NOT EXISTS idx_payment_methods_user_id ON payment_methods(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_methods_is_default ON payment_methods(is_default) WHERE is_default = true`,

		// ========== Session Recording Enhancements ==========

		// Extend session_recordings table with additional fields
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS file_hash VARCHAR(64)`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS format VARCHAR(50) DEFAULT 'webm'`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS metadata JSONB`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS retention_days INT DEFAULT 30`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS is_automatic BOOLEAN DEFAULT false`,
		`ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS reason VARCHAR(255)`,

		// Recording policies table
		`CREATE TABLE IF NOT EXISTS recording_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			auto_record BOOLEAN DEFAULT false,
			recording_format VARCHAR(50) DEFAULT 'webm',
			retention_days INT DEFAULT 30,
			apply_to_users JSONB,
			apply_to_teams JSONB,
			apply_to_templates JSONB,
			require_reason BOOLEAN DEFAULT false,
			allow_user_playback BOOLEAN DEFAULT true,
			allow_user_download BOOLEAN DEFAULT true,
			require_approval BOOLEAN DEFAULT false,
			notify_on_recording BOOLEAN DEFAULT true,
			metadata JSONB,
			enabled BOOLEAN DEFAULT true,
			priority INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Recording access log table for audit trail
		`CREATE TABLE IF NOT EXISTS recording_access_log (
			id SERIAL PRIMARY KEY,
			recording_id INT REFERENCES session_recordings(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			action VARCHAR(50) NOT NULL,
			accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ip_address VARCHAR(45),
			user_agent TEXT
		)`,

		// Create indexes for recording enhancements
		`CREATE INDEX IF NOT EXISTS idx_session_recordings_user_id ON session_recordings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_recordings_expires_at ON session_recordings(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_recording_policies_enabled ON recording_policies(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_recording_policies_priority ON recording_policies(priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_recording_access_log_recording_id ON recording_access_log(recording_id)`,
		`CREATE INDEX IF NOT EXISTS idx_recording_access_log_user_id ON recording_access_log(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_recording_access_log_accessed_at ON recording_access_log(accessed_at DESC)`,

		// ========== Data Loss Prevention (DLP) ==========

		// DLP policies table
		`CREATE TABLE IF NOT EXISTS dlp_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			enabled BOOLEAN DEFAULT true,
			priority INT DEFAULT 0,

			-- Clipboard controls
			clipboard_enabled BOOLEAN DEFAULT true,
			clipboard_direction VARCHAR(50) DEFAULT 'bidirectional',
			clipboard_max_size INT DEFAULT 1048576,
			clipboard_content_filter JSONB,

			-- File transfer controls
			file_transfer_enabled BOOLEAN DEFAULT true,
			file_upload_enabled BOOLEAN DEFAULT true,
			file_download_enabled BOOLEAN DEFAULT true,
			file_max_size BIGINT DEFAULT 104857600,
			file_type_whitelist JSONB,
			file_type_blacklist JSONB,
			scan_files_for_malware BOOLEAN DEFAULT false,

			-- Screen capture and printing
			screen_capture_enabled BOOLEAN DEFAULT true,
			printing_enabled BOOLEAN DEFAULT true,
			watermark_enabled BOOLEAN DEFAULT false,
			watermark_text VARCHAR(255),
			watermark_opacity DECIMAL(3,2) DEFAULT 0.3,
			watermark_position VARCHAR(50) DEFAULT 'center',

			-- USB and peripheral devices
			usb_devices_enabled BOOLEAN DEFAULT false,
			audio_enabled BOOLEAN DEFAULT true,
			microphone_enabled BOOLEAN DEFAULT false,
			webcam_enabled BOOLEAN DEFAULT false,

			-- Network controls
			network_access_enabled BOOLEAN DEFAULT true,
			allowed_domains JSONB,
			blocked_domains JSONB,
			allowed_ip_ranges JSONB,
			blocked_ip_ranges JSONB,

			-- Session controls
			idle_timeout INT,
			max_session_duration INT,
			require_reason BOOLEAN DEFAULT false,
			require_approval BOOLEAN DEFAULT false,

			-- Monitoring and logging
			log_all_activity BOOLEAN DEFAULT true,
			alert_on_violation BOOLEAN DEFAULT true,
			block_on_violation BOOLEAN DEFAULT true,
			notify_user BOOLEAN DEFAULT true,
			notify_admin BOOLEAN DEFAULT true,

			-- Application scope
			apply_to_users JSONB,
			apply_to_teams JSONB,
			apply_to_templates JSONB,
			apply_to_sessions JSONB,

			-- Metadata
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// DLP violations table
		`CREATE TABLE IF NOT EXISTS dlp_violations (
			id SERIAL PRIMARY KEY,
			policy_id INT REFERENCES dlp_policies(id) ON DELETE CASCADE,
			policy_name VARCHAR(255) NOT NULL,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			violation_type VARCHAR(100) NOT NULL,
			severity VARCHAR(50) DEFAULT 'medium',
			description TEXT,
			details JSONB,
			action VARCHAR(50) DEFAULT 'blocked',
			resolved BOOLEAN DEFAULT false,
			resolved_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			resolved_at TIMESTAMP,
			occurred_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for DLP
		`CREATE INDEX IF NOT EXISTS idx_dlp_policies_enabled ON dlp_policies(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_policies_priority ON dlp_policies(priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_policy_id ON dlp_violations(policy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_session_id ON dlp_violations(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_user_id ON dlp_violations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_occurred_at ON dlp_violations(occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_resolved ON dlp_violations(resolved) WHERE resolved = false`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_severity ON dlp_violations(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_type ON dlp_violations(violation_type)`,

		// ========== Template Versioning & Testing ==========

		// Template versions table
		`CREATE TABLE IF NOT EXISTS template_versions (
			id SERIAL PRIMARY KEY,
			template_id VARCHAR(255) NOT NULL,
			version VARCHAR(50) NOT NULL,
			major_version INT NOT NULL,
			minor_version INT NOT NULL,
			patch_version INT NOT NULL,
			display_name VARCHAR(255) NOT NULL,
			description TEXT,
			configuration JSONB,
			base_image TEXT,
			parent_template_id VARCHAR(255),
			parent_version VARCHAR(50),
			changelog TEXT,
			status VARCHAR(50) DEFAULT 'draft',
			is_default BOOLEAN DEFAULT false,
			test_results JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			published_at TIMESTAMP,
			deprecated_at TIMESTAMP,
			UNIQUE(template_id, version)
		)`,

		// Template tests table
		`CREATE TABLE IF NOT EXISTS template_tests (
			id SERIAL PRIMARY KEY,
			template_id VARCHAR(255) NOT NULL,
			version_id INT REFERENCES template_versions(id) ON DELETE CASCADE,
			version VARCHAR(50) NOT NULL,
			test_type VARCHAR(50) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			results JSONB,
			duration INT DEFAULT 0,
			error_message TEXT,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for template versioning
		`CREATE INDEX IF NOT EXISTS idx_template_versions_template_id ON template_versions(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_versions_version ON template_versions(version)`,
		`CREATE INDEX IF NOT EXISTS idx_template_versions_status ON template_versions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_template_versions_is_default ON template_versions(is_default) WHERE is_default = true`,
		`CREATE INDEX IF NOT EXISTS idx_template_versions_parent ON template_versions(parent_template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_tests_version_id ON template_tests(version_id)`,
		`CREATE INDEX IF NOT EXISTS idx_template_tests_status ON template_tests(status)`,

		// ========== Workflow Automation ==========

		// Workflows table
		`CREATE TABLE IF NOT EXISTS workflows (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			trigger JSONB NOT NULL,
			steps JSONB NOT NULL,
			enabled BOOLEAN DEFAULT true,
			execution_mode VARCHAR(50) DEFAULT 'sequential',
			timeout_minutes INT DEFAULT 60,
			retry_policy JSONB,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Workflow executions table
		`CREATE TABLE IF NOT EXISTS workflow_executions (
			id SERIAL PRIMARY KEY,
			workflow_id INT REFERENCES workflows(id) ON DELETE CASCADE,
			workflow_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			current_step VARCHAR(255),
			step_results JSONB,
			trigger_data JSONB,
			context JSONB,
			error_message TEXT,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			duration INT DEFAULT 0,
			triggered_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for workflows
		`CREATE INDEX IF NOT EXISTS idx_workflows_enabled ON workflows(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_created_by ON workflows(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflow_executions(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_started_at ON workflow_executions(started_at DESC)`,

		// ========== In-Browser Console & File Manager ==========

		// Console sessions table (terminal and file manager)
		`CREATE TABLE IF NOT EXISTS console_sessions (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			type VARCHAR(50) NOT NULL,
			status VARCHAR(50) DEFAULT 'active',
			current_path TEXT,
			shell_type VARCHAR(50),
			columns INT,
			rows INT,
			metadata JSONB,
			connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			disconnected_at TIMESTAMP
		)`,

		// Console file operations log
		`CREATE TABLE IF NOT EXISTS console_file_operations (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			operation VARCHAR(50) NOT NULL,
			source_path TEXT NOT NULL,
			target_path TEXT,
			bytes_processed BIGINT DEFAULT 0,
			success BOOLEAN DEFAULT true,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for console
		`CREATE INDEX IF NOT EXISTS idx_console_sessions_session_id ON console_sessions(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_console_sessions_user_id ON console_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_console_sessions_status ON console_sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_console_file_operations_session_id ON console_file_operations(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_console_file_operations_created_at ON console_file_operations(created_at DESC)`,

		// ========== Multi-Monitor Support ==========

		// Monitor configurations table
		`CREATE TABLE IF NOT EXISTS monitor_configurations (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			monitors JSONB NOT NULL,
			layout VARCHAR(50) DEFAULT 'horizontal',
			total_width INT NOT NULL,
			total_height INT NOT NULL,
			primary_monitor INT DEFAULT 0,
			metadata JSONB,
			is_active BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for multi-monitor
		`CREATE INDEX IF NOT EXISTS idx_monitor_configurations_session_id ON monitor_configurations(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configurations_user_id ON monitor_configurations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configurations_is_active ON monitor_configurations(is_active) WHERE is_active = true`,

		// ========== Real-time Collaboration ==========

		// Collaboration sessions table
		`CREATE TABLE IF NOT EXISTS collaboration_sessions (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			owner_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			settings JSONB,
			active_users INT DEFAULT 0,
			chat_enabled BOOLEAN DEFAULT true,
			annotations_enabled BOOLEAN DEFAULT true,
			cursor_tracking BOOLEAN DEFAULT true,
			status VARCHAR(50) DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ended_at TIMESTAMP
		)`,

		// Collaboration participants table
		`CREATE TABLE IF NOT EXISTS collaboration_participants (
			id SERIAL PRIMARY KEY,
			collaboration_id VARCHAR(255) REFERENCES collaboration_sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL,
			permissions JSONB,
			cursor_position JSONB,
			color VARCHAR(50),
			is_active BOOLEAN DEFAULT true,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(collaboration_id, user_id)
		)`,

		// Collaboration chat table
		`CREATE TABLE IF NOT EXISTS collaboration_chat (
			id SERIAL PRIMARY KEY,
			collaboration_id VARCHAR(255) REFERENCES collaboration_sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255),
			message TEXT NOT NULL,
			message_type VARCHAR(50) DEFAULT 'text',
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Collaboration annotations table
		`CREATE TABLE IF NOT EXISTS collaboration_annotations (
			id VARCHAR(255) PRIMARY KEY,
			collaboration_id VARCHAR(255) REFERENCES collaboration_sessions(id) ON DELETE CASCADE,
			session_id VARCHAR(255) REFERENCES sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			type VARCHAR(50) NOT NULL,
			color VARCHAR(50),
			thickness INT,
			points JSONB,
			text TEXT,
			is_persistent BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP
		)`,

		// Create indexes for collaboration
		`CREATE INDEX IF NOT EXISTS idx_collaboration_sessions_session_id ON collaboration_sessions(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_sessions_status ON collaboration_sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_participants_collab_id ON collaboration_participants(collaboration_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_participants_user_id ON collaboration_participants(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_participants_is_active ON collaboration_participants(is_active) WHERE is_active = true`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_chat_collab_id ON collaboration_chat(collaboration_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_chat_created_at ON collaboration_chat(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_annotations_collab_id ON collaboration_annotations(collaboration_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_annotations_expires_at ON collaboration_annotations(expires_at)`,

		// ========== Integration Hub & Webhooks ==========

		// Webhooks table
		`CREATE TABLE IF NOT EXISTS webhooks (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			url TEXT NOT NULL,
			secret VARCHAR(255),
			events JSONB NOT NULL,
			headers JSONB,
			enabled BOOLEAN DEFAULT true,
			retry_policy JSONB,
			filters JSONB,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Webhook deliveries table
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id SERIAL PRIMARY KEY,
			webhook_id INT REFERENCES webhooks(id) ON DELETE CASCADE,
			event VARCHAR(100) NOT NULL,
			payload JSONB,
			status VARCHAR(50) DEFAULT 'pending',
			status_code INT,
			response_body TEXT,
			error_message TEXT,
			attempts INT DEFAULT 0,
			next_retry_at TIMESTAMP,
			delivered_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Integrations table
		`CREATE TABLE IF NOT EXISTS integrations (
			id SERIAL PRIMARY KEY,
			type VARCHAR(50) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			config JSONB NOT NULL,
			enabled BOOLEAN DEFAULT true,
			events JSONB,
			test_mode BOOLEAN DEFAULT false,
			last_test_at TIMESTAMP,
			last_success_at TIMESTAMP,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for integrations
		`CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhooks(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_created_by ON webhooks(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE next_retry_at IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_integrations_type ON integrations(type)`,
		`CREATE INDEX IF NOT EXISTS idx_integrations_enabled ON integrations(enabled) WHERE enabled = true`,

		// ========== Advanced Security ==========

		// MFA methods (TOTP, SMS, Email)
		`CREATE TABLE IF NOT EXISTS mfa_methods (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			secret VARCHAR(255),
			phone_number VARCHAR(50),
			email VARCHAR(255),
			enabled BOOLEAN DEFAULT false,
			verified BOOLEAN DEFAULT false,
			is_primary BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_used_at TIMESTAMP,
			UNIQUE(user_id, type)
		)`,

		// Backup codes for MFA recovery
		`CREATE TABLE IF NOT EXISTS backup_codes (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			code VARCHAR(255) NOT NULL,
			used BOOLEAN DEFAULT false,
			used_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Trusted devices for MFA bypass
		`CREATE TABLE IF NOT EXISTS trusted_devices (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_id VARCHAR(255) NOT NULL,
			device_name VARCHAR(255),
			user_agent TEXT,
			ip_address VARCHAR(50),
			trusted_until TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, device_id)
		)`,

		// IP whitelist for access control
		`CREATE TABLE IF NOT EXISTS ip_whitelist (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
			ip_address VARCHAR(100) NOT NULL,
			description TEXT,
			enabled BOOLEAN DEFAULT true,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP
		)`,

		// Session verifications for Zero Trust
		`CREATE TABLE IF NOT EXISTS session_verifications (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_id VARCHAR(255) NOT NULL,
			ip_address VARCHAR(50),
			location VARCHAR(255),
			risk_score INT DEFAULT 0,
			risk_level VARCHAR(50) DEFAULT 'low',
			verified BOOLEAN DEFAULT false,
			last_verified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Device posture checks
		`CREATE TABLE IF NOT EXISTS device_posture_checks (
			id SERIAL PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			compliant BOOLEAN DEFAULT false,
			issues TEXT,
			checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Security alerts
		`CREATE TABLE IF NOT EXISTS security_alerts (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(100) NOT NULL,
			severity VARCHAR(50) DEFAULT 'medium',
			message TEXT NOT NULL,
			details JSONB,
			acknowledged BOOLEAN DEFAULT false,
			acknowledged_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for security tables
		`CREATE INDEX IF NOT EXISTS idx_mfa_methods_user_id ON mfa_methods(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mfa_methods_enabled ON mfa_methods(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_backup_codes_user_id ON backup_codes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_backup_codes_code ON backup_codes(code) WHERE used = false`,
		`CREATE INDEX IF NOT EXISTS idx_trusted_devices_user_id ON trusted_devices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trusted_devices_device_id ON trusted_devices(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trusted_devices_expiry ON trusted_devices(trusted_until)`,
		`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_user_id ON ip_whitelist(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_enabled ON ip_whitelist(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_session_verifications_session_id ON session_verifications(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_verifications_user_id ON session_verifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_verifications_risk_level ON session_verifications(risk_level)`,
		`CREATE INDEX IF NOT EXISTS idx_device_posture_device_id ON device_posture_checks(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_security_alerts_user_id ON security_alerts(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_security_alerts_acknowledged ON security_alerts(acknowledged) WHERE acknowledged = false`,

		// ========== Session Scheduling & Calendar Integration ==========

		// Scheduled sessions
		`CREATE TABLE IF NOT EXISTS scheduled_sessions (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			template_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			timezone VARCHAR(100) DEFAULT 'UTC',
			schedule JSONB NOT NULL,
			resources JSONB,
			auto_terminate BOOLEAN DEFAULT false,
			terminate_after INT DEFAULT 480,
			pre_warm BOOLEAN DEFAULT false,
			pre_warm_minutes INT DEFAULT 5,
			post_cleanup BOOLEAN DEFAULT true,
			enabled BOOLEAN DEFAULT true,
			next_run_at TIMESTAMP,
			last_run_at TIMESTAMP,
			last_session_id VARCHAR(255),
			last_run_status VARCHAR(50),
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Calendar integrations
		`CREATE TABLE IF NOT EXISTS calendar_integrations (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			account_email VARCHAR(255) NOT NULL,
			access_token TEXT,
			refresh_token TEXT,
			token_expiry TIMESTAMP,
			calendar_id VARCHAR(255),
			enabled BOOLEAN DEFAULT true,
			sync_enabled BOOLEAN DEFAULT true,
			auto_create_events BOOLEAN DEFAULT true,
			auto_update_events BOOLEAN DEFAULT true,
			last_synced_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, provider, account_email)
		)`,

		// Calendar events
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id SERIAL PRIMARY KEY,
			schedule_id INT REFERENCES scheduled_sessions(id) ON DELETE CASCADE,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			external_event_id VARCHAR(255),
			title VARCHAR(255) NOT NULL,
			description TEXT,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NOT NULL,
			location TEXT,
			attendees TEXT[],
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(schedule_id, provider)
		)`,

		// Create indexes for scheduling tables
		`CREATE INDEX IF NOT EXISTS idx_scheduled_sessions_user_id ON scheduled_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_sessions_enabled ON scheduled_sessions(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_sessions_next_run ON scheduled_sessions(next_run_at) WHERE next_run_at IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_sessions_template_id ON scheduled_sessions(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_integrations_user_id ON calendar_integrations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_integrations_provider ON calendar_integrations(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_events_schedule_id ON calendar_events(schedule_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_events_user_id ON calendar_events(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_events_start_time ON calendar_events(start_time)`,

		// ========== Load Balancing & Auto-scaling ==========

		// Load balancing policies
		`CREATE TABLE IF NOT EXISTS load_balancing_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			description TEXT,
			strategy VARCHAR(50) NOT NULL,
			enabled BOOLEAN DEFAULT true,
			session_affinity BOOLEAN DEFAULT false,
			health_check_config JSONB,
			node_selector JSONB,
			node_weights JSONB,
			geo_preferences TEXT[],
			resource_thresholds JSONB,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Node status tracking
		`CREATE TABLE IF NOT EXISTS node_status (
			id SERIAL PRIMARY KEY,
			node_name VARCHAR(255) NOT NULL UNIQUE,
			status VARCHAR(50) DEFAULT 'unknown',
			cpu_allocated DECIMAL(10,2) DEFAULT 0,
			cpu_capacity DECIMAL(10,2) DEFAULT 0,
			memory_allocated BIGINT DEFAULT 0,
			memory_capacity BIGINT DEFAULT 0,
			active_sessions INT DEFAULT 0,
			health_status VARCHAR(50) DEFAULT 'unknown',
			last_health_check TIMESTAMP,
			region VARCHAR(100),
			zone VARCHAR(100),
			labels JSONB,
			taints TEXT[],
			weight INT DEFAULT 1,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Auto-scaling policies
		`CREATE TABLE IF NOT EXISTS autoscaling_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			description TEXT,
			target_type VARCHAR(50) NOT NULL,
			target_id VARCHAR(255) NOT NULL,
			enabled BOOLEAN DEFAULT true,
			scaling_mode VARCHAR(50) DEFAULT 'horizontal',
			min_replicas INT DEFAULT 1,
			max_replicas INT DEFAULT 10,
			metric_type VARCHAR(50) DEFAULT 'cpu',
			target_metric_value DECIMAL(10,2),
			scale_up_policy JSONB,
			scale_down_policy JSONB,
			predictive_scaling JSONB,
			cooldown_period INT DEFAULT 300,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Scaling events (audit log for scaling actions)
		`CREATE TABLE IF NOT EXISTS scaling_events (
			id SERIAL PRIMARY KEY,
			policy_id INT REFERENCES autoscaling_policies(id) ON DELETE CASCADE,
			target_type VARCHAR(50) NOT NULL,
			target_id VARCHAR(255) NOT NULL,
			action VARCHAR(50) NOT NULL,
			previous_replicas INT NOT NULL,
			new_replicas INT NOT NULL,
			trigger VARCHAR(50) NOT NULL,
			metric_value DECIMAL(10,2),
			reason TEXT,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for load balancing and autoscaling
		`CREATE INDEX IF NOT EXISTS idx_load_balancing_policies_enabled ON load_balancing_policies(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_load_balancing_policies_strategy ON load_balancing_policies(strategy)`,
		`CREATE INDEX IF NOT EXISTS idx_node_status_status ON node_status(status)`,
		`CREATE INDEX IF NOT EXISTS idx_node_status_health ON node_status(health_status)`,
		`CREATE INDEX IF NOT EXISTS idx_node_status_region ON node_status(region)`,
		`CREATE INDEX IF NOT EXISTS idx_autoscaling_policies_enabled ON autoscaling_policies(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_autoscaling_policies_target ON autoscaling_policies(target_type, target_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scaling_events_policy_id ON scaling_events(policy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scaling_events_created_at ON scaling_events(created_at DESC)`,

		// ========== Compliance & Governance ==========

		// Compliance frameworks (GDPR, HIPAA, SOC2, etc.)
		`CREATE TABLE IF NOT EXISTS compliance_frameworks (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			display_name VARCHAR(255) NOT NULL,
			description TEXT,
			version VARCHAR(50),
			enabled BOOLEAN DEFAULT true,
			controls JSONB,
			requirements JSONB,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Compliance policies
		`CREATE TABLE IF NOT EXISTS compliance_policies (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			framework_id INT REFERENCES compliance_frameworks(id) ON DELETE SET NULL,
			applies_to JSONB NOT NULL,
			enabled BOOLEAN DEFAULT true,
			enforcement_level VARCHAR(50) DEFAULT 'warning',
			data_retention JSONB,
			data_classification JSONB,
			access_controls JSONB,
			audit_requirements JSONB,
			violation_actions JSONB,
			metadata JSONB,
			created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Compliance violations
		`CREATE TABLE IF NOT EXISTS compliance_violations (
			id SERIAL PRIMARY KEY,
			policy_id INT REFERENCES compliance_policies(id) ON DELETE CASCADE,
			user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			violation_type VARCHAR(100) NOT NULL,
			severity VARCHAR(50) DEFAULT 'medium',
			description TEXT NOT NULL,
			details JSONB,
			status VARCHAR(50) DEFAULT 'open',
			resolution TEXT,
			resolved_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			resolved_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Compliance reports
		`CREATE TABLE IF NOT EXISTS compliance_reports (
			id SERIAL PRIMARY KEY,
			framework_id INT REFERENCES compliance_frameworks(id) ON DELETE SET NULL,
			report_type VARCHAR(50) NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			overall_status VARCHAR(50),
			controls_summary JSONB,
			violations JSONB,
			recommendations TEXT[],
			generated_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
			generated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for compliance tables
		`CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_enabled ON compliance_frameworks(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_name ON compliance_frameworks(name)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_policies_framework_id ON compliance_policies(framework_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_policies_enabled ON compliance_policies(enabled) WHERE enabled = true`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_violations_policy_id ON compliance_violations(policy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_violations_user_id ON compliance_violations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_violations_status ON compliance_violations(status)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_violations_severity ON compliance_violations(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_reports_framework_id ON compliance_reports(framework_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compliance_reports_generated_at ON compliance_reports(generated_at DESC)`,

		// ========== NATS Event-Driven Architecture ==========

		// Platform controllers (registered platform controllers - K8s, Docker, Hyper-V, etc.)
		`CREATE TABLE IF NOT EXISTS platform_controllers (
			id VARCHAR(255) PRIMARY KEY,
			controller_id VARCHAR(255) UNIQUE NOT NULL,
			platform VARCHAR(50) NOT NULL,
			display_name VARCHAR(255),
			status VARCHAR(50) DEFAULT 'unknown',
			version VARCHAR(50),
			capabilities JSONB DEFAULT '[]',
			cluster_info JSONB DEFAULT '{}',
			last_heartbeat TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create indexes for platform controllers
		`CREATE INDEX IF NOT EXISTS idx_platform_controllers_platform ON platform_controllers(platform)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_controllers_status ON platform_controllers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_controllers_heartbeat ON platform_controllers(last_heartbeat)`,

		// Event log (audit log of all NATS events for debugging and replay)
		`CREATE TABLE IF NOT EXISTS event_log (
			id BIGSERIAL PRIMARY KEY,
			event_id VARCHAR(255) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP,
			processed_by VARCHAR(255),
			status VARCHAR(50) DEFAULT 'published',
			error_message TEXT
		)`,

		// Create indexes for event log
		`CREATE INDEX IF NOT EXISTS idx_event_log_event_id ON event_log(event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_event_log_subject ON event_log(subject)`,
		`CREATE INDEX IF NOT EXISTS idx_event_log_status ON event_log(status)`,
		`CREATE INDEX IF NOT EXISTS idx_event_log_published_at ON event_log(published_at)`,

		// Add platform fields to installed_applications for async installation tracking
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS install_status VARCHAR(50) DEFAULT 'pending'`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS install_message TEXT`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS platform VARCHAR(50) DEFAULT 'kubernetes'`,

		// Add icon and metadata columns to installed_applications for persistence
		// Icons are downloaded when app is installed, enabling offline/air-gapped deployments
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS description TEXT`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS category VARCHAR(100)`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS icon_url TEXT`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS icon_data BYTEA`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS icon_media_type VARCHAR(100)`,
		`ALTER TABLE installed_applications ADD COLUMN IF NOT EXISTS manifest JSONB`,

		// Create index for install status
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_status ON installed_applications(install_status)`,
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_platform ON installed_applications(platform)`,
		`CREATE INDEX IF NOT EXISTS idx_installed_applications_category ON installed_applications(category)`,

		// Add platform fields to sessions for multi-platform support
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS platform VARCHAR(50) DEFAULT 'kubernetes'`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS controller_id VARCHAR(255)`,

		// Create indexes for session platform tracking
		`CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(platform)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_controller_id ON sessions(controller_id)`,

		// Add additional session fields for multi-platform support
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS pod_name VARCHAR(255)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS memory VARCHAR(50)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cpu VARCHAR(50)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS persistent_home BOOLEAN DEFAULT false`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS idle_timeout VARCHAR(50)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS max_session_duration VARCHAR(50)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_activity TIMESTAMP`,

		// Create index for idle session queries
		`CREATE INDEX IF NOT EXISTS idx_sessions_last_activity ON sessions(last_activity)`,

		// License Management
		// Licenses table - manages platform licensing and feature enforcement
		`CREATE TABLE IF NOT EXISTS licenses (
			id SERIAL PRIMARY KEY,
			license_key VARCHAR(255) UNIQUE NOT NULL,
			tier VARCHAR(50) NOT NULL,
			features JSONB,
			max_users INT,
			max_sessions INT,
			max_nodes INT,
			issued_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			activated_at TIMESTAMP,
			status VARCHAR(50) DEFAULT 'active',
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// License usage tracking - daily snapshots of resource usage
		`CREATE TABLE IF NOT EXISTS license_usage (
			id SERIAL PRIMARY KEY,
			license_id INT REFERENCES licenses(id) ON DELETE CASCADE,
			snapshot_date DATE NOT NULL,
			active_users INT DEFAULT 0,
			active_sessions INT DEFAULT 0,
			active_nodes INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(license_id, snapshot_date)
		)`,

		// License indexes for efficient querying
		`CREATE INDEX IF NOT EXISTS idx_licenses_tier ON licenses(tier)`,
		`CREATE INDEX IF NOT EXISTS idx_licenses_status ON licenses(status)`,
		`CREATE INDEX IF NOT EXISTS idx_licenses_expires_at ON licenses(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_license_usage_license_id ON license_usage(license_id)`,
		`CREATE INDEX IF NOT EXISTS idx_license_usage_snapshot_date ON license_usage(snapshot_date)`,

		// Insert default Community license for initial setup
		`INSERT INTO licenses (license_key, tier, features, max_users, max_sessions, max_nodes, issued_at, expires_at, activated_at, status, metadata)
		VALUES (
			'COMMUNITY-DEFAULT',
			'community',
			'{"basic_auth": true, "saml": false, "oidc": false, "mfa": false, "recordings": false, "advanced_compliance": false, "priority_support": false}',
			10,
			20,
			3,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP + INTERVAL '100 years',
			CURRENT_TIMESTAMP,
			'active',
			'{"description": "Default Community license - free forever", "auto_generated": true}'
		)
		ON CONFLICT (license_key) DO NOTHING`,

		// ========================================================================
		// v2.0 Architecture: Multi-Platform Control Plane + Agents
		// ========================================================================

		// Agents table (platform-specific execution agents)
		// Supports multi-platform deployment (Kubernetes, Docker, VMs, Cloud)
		`CREATE TABLE IF NOT EXISTS agents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			agent_id VARCHAR(255) UNIQUE NOT NULL,
			platform VARCHAR(50) NOT NULL,
			region VARCHAR(100),
			status VARCHAR(50) DEFAULT 'offline',
			capacity JSONB,
			last_heartbeat TIMESTAMP,
			websocket_id VARCHAR(255),
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Agent commands table (command queue for agent communication)
		// Tracks commands sent from Control Plane to Agents
		`CREATE TABLE IF NOT EXISTS agent_commands (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			command_id VARCHAR(255) UNIQUE NOT NULL,
			agent_id VARCHAR(255) REFERENCES agents(agent_id) ON DELETE CASCADE,
			session_id VARCHAR(255),
			action VARCHAR(50) NOT NULL,
			payload JSONB,
			status VARCHAR(50) DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			sent_at TIMESTAMP,
			acknowledged_at TIMESTAMP,
			completed_at TIMESTAMP
		)`,

		// Alter sessions table to add v2.0 platform-agnostic fields
		// NOTE: These columns may already exist from previous runs (IF NOT EXISTS doesn't work on ALTER TABLE)
		// Using DO $$ block to check if columns exist before adding them
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns
				WHERE table_name='sessions' AND column_name='agent_id') THEN
				ALTER TABLE sessions ADD COLUMN agent_id VARCHAR(255) REFERENCES agents(agent_id);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns
				WHERE table_name='sessions' AND column_name='platform') THEN
				ALTER TABLE sessions ADD COLUMN platform VARCHAR(50);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns
				WHERE table_name='sessions' AND column_name='platform_metadata') THEN
				ALTER TABLE sessions ADD COLUMN platform_metadata JSONB;
			END IF;
		END $$`,

		// Indexes for agents table
		`CREATE INDEX IF NOT EXISTS idx_agents_agent_id ON agents(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_platform ON agents(platform)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_region ON agents(region)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON agents(last_heartbeat)`,

		// Indexes for agent_commands table
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_command_id ON agent_commands(command_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_agent_id ON agent_commands(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_session_id ON agent_commands(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_status ON agent_commands(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_created_at ON agent_commands(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_action ON agent_commands(action)`,

		// Composite indexes for common queries
		`CREATE INDEX IF NOT EXISTS idx_agent_commands_agent_status ON agent_commands(agent_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_platform_status ON agents(platform, status)`,

		// Index for sessions table agent_id lookup
		`CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(platform)`,
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	// After migrations, ensure admin password is configured
	if err := d.ensureAdminPassword(); err != nil {
		return fmt.Errorf("failed to configure admin password: %w", err)
	}

	// Check for password reset request
	if err := d.checkPasswordReset(); err != nil {
		return fmt.Errorf("failed to process password reset: %w", err)
	}

	// Ensure default template repository is configured
	if err := d.ensureDefaultRepository(); err != nil {
		return fmt.Errorf("failed to configure default repository: %w", err)
	}

	return nil
}

// ensureAdminPassword configures the admin password using multiple fallback methods
// Priority order:
//  1. ADMIN_PASSWORD environment variable (Kubernetes Secret or manual)
//  2. Leave NULL - enables setup wizard mode
func (d *Database) ensureAdminPassword() error {
	// Check if admin user exists and has a password
	var passwordHash sql.NullString
	err := d.db.QueryRow("SELECT password_hash FROM users WHERE id = 'admin'").Scan(&passwordHash)
	if err != nil {
		// Admin user doesn't exist yet, skip (will be created by migration)
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("failed to check admin user: %w", err)
	}

	// Admin already has a password - don't override
	if passwordHash.Valid && passwordHash.String != "" {
		log.Println("✓ Admin user already has a password configured")
		return nil
	}

	// Priority 1: Check ADMIN_PASSWORD environment variable
	password := os.Getenv("ADMIN_PASSWORD")
	if password != "" {
		log.Println("🔑 Using admin password from ADMIN_PASSWORD environment variable")

		// Validate password strength
		if len(password) < 8 {
			return fmt.Errorf("ADMIN_PASSWORD must be at least 8 characters long")
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}

		// Update admin user
		_, err = d.db.Exec("UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = 'admin'", string(hashedPassword))
		if err != nil {
			return fmt.Errorf("failed to set admin password: %w", err)
		}

		log.Println("✓ Admin password configured successfully from environment variable")
		return nil
	}

	// Priority 2: No password - enable setup wizard mode
	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")
	log.Println("⚠️  ADMIN USER HAS NO PASSWORD SET!")
	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")
	log.Println("⚠️  ")
	log.Println("⚠️  The admin account requires password configuration.")
	log.Println("⚠️  ")
	log.Println("⚠️  Setup wizard mode is ENABLED at: /api/v1/auth/setup")
	log.Println("⚠️  ")
	log.Println("⚠️  Alternative methods:")
	log.Println("⚠️  1. Set ADMIN_PASSWORD environment variable")
	log.Println("⚠️  2. Use the setup wizard in your browser")
	log.Println("⚠️  3. Check Helm chart for auto-generated credentials")
	log.Println("⚠️  ")
	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")

	return nil // Not an error, just informational
}

// checkPasswordReset checks for ADMIN_PASSWORD_RESET environment variable
// and resets the admin password if set. This is for account recovery.
func (d *Database) checkPasswordReset() error {
	resetPassword := os.Getenv("ADMIN_PASSWORD_RESET")
	if resetPassword == "" {
		return nil // No reset requested
	}

	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")
	log.Println("⚠️  ADMIN PASSWORD RESET DETECTED!")
	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")

	// Validate password strength
	if len(resetPassword) < 8 {
		return fmt.Errorf("ADMIN_PASSWORD_RESET must be at least 8 characters long")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(resetPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash reset password: %w", err)
	}

	// Update admin password
	result, err := d.db.Exec("UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = 'admin'", string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to reset admin password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check reset result: %w", err)
	}

	if rowsAffected == 0 {
		log.Println("⚠️  Admin user not found - password reset failed")
		return fmt.Errorf("admin user not found")
	}

	log.Println("✓ Admin password RESET successfully!")
	log.Println("⚠️  ")
	log.Println("⚠️  NEXT STEPS:")
	log.Println("⚠️  1. Remove ADMIN_PASSWORD_RESET environment variable")
	log.Println("⚠️  2. Restart the API deployment")
	log.Println("⚠️  3. Log in with the new password")
	log.Println("⚠️  ")
	log.Println("⚠️  ═══════════════════════════════════════════════════════════════")

	return nil
}

// ensureDefaultRepository ensures the official StreamSpace repositories are configured.
// This ensures both the templates and plugins repositories exist.
//
// The repositories will be automatically synced by the sync service on startup.
//
// Behavior:
//   - Uses INSERT ... ON CONFLICT DO NOTHING for idempotency
//   - Inserts both Official Templates and Official Plugins repositories
//   - Does not fail if repositories already configured
//
// Default repository configurations:
//   - Official Templates: https://github.com/JoshuaAFerguson/streamspace-templates
//   - Official Plugins: https://github.com/JoshuaAFerguson/streamspace-plugins
//   - Branch: main
//   - Auth: none (public repositories)
//   - Status: pending (will be synced automatically)
func (d *Database) ensureDefaultRepository() error {
	// Define default repositories
	type defaultRepo struct {
		name     string
		url      string
		branch   string
		repoType string
	}

	defaultRepos := []defaultRepo{
		{
			name:     "Official Templates",
			url:      "https://github.com/JoshuaAFerguson/streamspace-templates",
			branch:   "main",
			repoType: "template",
		},
		{
			name:     "Official Plugins",
			url:      "https://github.com/JoshuaAFerguson/streamspace-plugins",
			branch:   "main",
			repoType: "plugin",
		},
	}

	log.Println("📦 Ensuring default repositories are configured...")

	for _, repo := range defaultRepos {
		// Use INSERT ... ON CONFLICT DO NOTHING for idempotency
		result, err := d.db.Exec(`
			INSERT INTO repositories (name, url, branch, type, auth_type, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'none', 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (name) DO NOTHING
		`, repo.name, repo.url, repo.branch, repo.repoType)

		if err != nil {
			return fmt.Errorf("failed to ensure repository '%s': %w", repo.name, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("   ✓ Added '%s' (%s)", repo.name, repo.url)
		} else {
			log.Printf("   ✓ '%s' already configured", repo.name)
		}
	}

	log.Println("✓ Default repositories configured successfully")
	log.Println("   Repositories will be synced automatically on startup")

	return nil
}
