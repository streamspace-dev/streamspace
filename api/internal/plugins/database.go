package plugins

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/streamspace/streamspace/api/internal/db"
)

// PluginDatabase provides database access for plugins
type PluginDatabase struct {
	db         *db.Database
	pluginName string
}

// NewPluginDatabase creates a new plugin database instance
func NewPluginDatabase(database *db.Database, pluginName string) *PluginDatabase {
	return &PluginDatabase{
		db:         database,
		pluginName: pluginName,
	}
}

// Exec executes a SQL statement
func (pd *PluginDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return pd.db.DB().Exec(query, args...)
}

// Query executes a SQL query
func (pd *PluginDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return pd.db.DB().Query(query, args...)
}

// QueryRow executes a SQL query that returns a single row
func (pd *PluginDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return pd.db.DB().QueryRow(query, args...)
}

// Transaction executes a function within a transaction
func (pd *PluginDatabase) Transaction(fn func(*sql.Tx) error) error {
	tx, err := pd.db.DB().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rollback err: %w", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

// Migrate executes a migration SQL (for plugin table setup)
func (pd *PluginDatabase) Migrate(migrationSQL string) error {
	_, err := pd.db.DB().Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("migration failed for plugin %s: %w", pd.pluginName, err)
	}
	return nil
}

// CreateTable creates a table for the plugin (namespaced)
func (pd *PluginDatabase) CreateTable(tableName string, schema string) error {
	// Namespace table with plugin name to avoid conflicts
	fullTableName := fmt.Sprintf("plugin_%s_%s", pd.pluginName, tableName)

	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s
		)
	`, fullTableName, schema)

	_, err := pd.db.DB().Exec(createSQL)
	if err != nil {
		return fmt.Errorf("failed to create table %s for plugin %s: %w", fullTableName, pd.pluginName, err)
	}

	return nil
}

// PluginStorage provides key-value storage for plugins
type PluginStorage struct {
	db         *db.Database
	pluginName string
}

// NewPluginStorage creates a new plugin storage instance
func NewPluginStorage(database *db.Database, pluginName string) *PluginStorage {
	return &PluginStorage{
		db:         database,
		pluginName: pluginName,
	}
}

// initStorage ensures the plugin_storage table exists
func (ps *PluginStorage) initStorage() error {
	_, err := ps.db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS plugin_storage (
			plugin_name TEXT NOT NULL,
			key TEXT NOT NULL,
			value JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (plugin_name, key)
		)
	`)
	return err
}

// Get retrieves a value from plugin storage
func (ps *PluginStorage) Get(key string) (interface{}, error) {
	ps.initStorage() // Ensure table exists

	var value interface{}
	err := ps.db.DB().QueryRow(`
		SELECT value FROM plugin_storage
		WHERE plugin_name = $1 AND key = $2
	`, ps.pluginName, key).Scan(&value)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s for plugin %s: %w", key, ps.pluginName, err)
	}

	return value, nil
}

// Set stores a value in plugin storage
func (ps *PluginStorage) Set(key string, value interface{}) error {
	ps.initStorage() // Ensure table exists

	_, err := ps.db.DB().Exec(`
		INSERT INTO plugin_storage (plugin_name, key, value, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (plugin_name, key)
		DO UPDATE SET value = $3, updated_at = NOW()
	`, ps.pluginName, key, value)

	if err != nil {
		return fmt.Errorf("failed to set key %s for plugin %s: %w", key, ps.pluginName, err)
	}

	return nil
}

// Delete removes a value from plugin storage
func (ps *PluginStorage) Delete(key string) error {
	ps.initStorage() // Ensure table exists

	_, err := ps.db.DB().Exec(`
		DELETE FROM plugin_storage
		WHERE plugin_name = $1 AND key = $2
	`, ps.pluginName, key)

	if err != nil {
		return fmt.Errorf("failed to delete key %s for plugin %s: %w", key, ps.pluginName, err)
	}

	return nil
}

// Keys returns all keys for the plugin
func (ps *PluginStorage) Keys(prefix string) ([]string, error) {
	ps.initStorage() // Ensure table exists

	var query string
	var args []interface{}

	if prefix == "" {
		query = `SELECT key FROM plugin_storage WHERE plugin_name = $1 ORDER BY key`
		args = []interface{}{ps.pluginName}
	} else {
		query = `SELECT key FROM plugin_storage WHERE plugin_name = $1 AND key LIKE $2 ORDER BY key`
		args = []interface{}{ps.pluginName, prefix + "%"}
	}

	rows, err := ps.db.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys for plugin %s: %w", ps.pluginName, err)
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// Clear removes all storage for the plugin
func (ps *PluginStorage) Clear() error {
	ps.initStorage() // Ensure table exists

	_, err := ps.db.DB().Exec(`
		DELETE FROM plugin_storage WHERE plugin_name = $1
	`, ps.pluginName)

	if err != nil {
		return fmt.Errorf("failed to clear storage for plugin %s: %w", ps.pluginName, err)
	}

	return nil
}
