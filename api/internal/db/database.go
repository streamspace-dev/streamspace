package db

import (
	"database/sql"
	"fmt"

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

// NewDatabase creates a new database connection with connection pooling
func NewDatabase(config Config) (*Database, error) {
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

		// Sessions table (cache of K8s Sessions)
		`CREATE TABLE IF NOT EXISTS sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
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

		// Create index on user_id
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
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
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	return nil
}
