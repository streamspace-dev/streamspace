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

// NewDatabase creates a new database connection
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
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	return nil
}
