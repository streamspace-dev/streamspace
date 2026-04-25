package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq" // PostgreSQL array support
)

// TemplateDB provides database operations for templates.
// v2.0-beta: Templates are stored in database (catalog_templates), not Kubernetes CRDs.
type TemplateDB struct {
	db *Database
}

// NewTemplateDB creates a new template database instance.
func NewTemplateDB(db *Database) *TemplateDB {
	return &TemplateDB{db: db}
}

// Template represents a template stored in the catalog_templates table.
type Template struct {
	ID           int             `json:"id"`
	RepositoryID int             `json:"repository_id"`
	Name         string          `json:"name"`
	DisplayName  string          `json:"display_name"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	AppType      string          `json:"app_type"`
	IconURL      string          `json:"icon_url"`
	Manifest     json.RawMessage `json:"manifest"` // JSONB - Template CRD spec
	Tags         []string        `json:"tags"`
	InstallCount int             `json:"install_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// GetTemplateByName fetches a template by name from the catalog_templates table.
// Returns sql.ErrNoRows if template doesn't exist.
func (t *TemplateDB) GetTemplateByName(ctx context.Context, name string) (*Template, error) {
	query := `
		SELECT
			id, repository_id, name, display_name, description, category, app_type,
			COALESCE(icon_url, ''), manifest, COALESCE(tags, ARRAY[]::TEXT[]),
			install_count, created_at, updated_at
		FROM catalog_templates
		WHERE name = $1
	`

	template := &Template{}
	err := t.db.DB().QueryRowContext(ctx, query, name).Scan(
		&template.ID, &template.RepositoryID, &template.Name, &template.DisplayName,
		&template.Description, &template.Category, &template.AppType, &template.IconURL,
		&template.Manifest, pq.Array(&template.Tags), &template.InstallCount,
		&template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return template, nil
}

// GetTemplateByID fetches a template by ID from the catalog_templates table.
func (t *TemplateDB) GetTemplateByID(ctx context.Context, id int) (*Template, error) {
	query := `
		SELECT
			id, repository_id, name, display_name, description, category, app_type,
			COALESCE(icon_url, ''), manifest, COALESCE(tags, ARRAY[]::TEXT[]),
			install_count, created_at, updated_at
		FROM catalog_templates
		WHERE id = $1
	`

	template := &Template{}
	err := t.db.DB().QueryRowContext(ctx, query, id).Scan(
		&template.ID, &template.RepositoryID, &template.Name, &template.DisplayName,
		&template.Description, &template.Category, &template.AppType, &template.IconURL,
		&template.Manifest, pq.Array(&template.Tags), &template.InstallCount,
		&template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return template, nil
}

// ListTemplates retrieves all templates from the catalog_templates table.
func (t *TemplateDB) ListTemplates(ctx context.Context) ([]*Template, error) {
	query := `
		SELECT
			id, repository_id, name, display_name, description, category, app_type,
			COALESCE(icon_url, ''), manifest, COALESCE(tags, ARRAY[]::TEXT[]),
			install_count, created_at, updated_at
		FROM catalog_templates
		ORDER BY display_name ASC
	`

	rows, err := t.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	return t.scanTemplates(rows)
}

// ListTemplatesByCategory retrieves templates filtered by category.
func (t *TemplateDB) ListTemplatesByCategory(ctx context.Context, category string) ([]*Template, error) {
	query := `
		SELECT
			id, repository_id, name, display_name, description, category, app_type,
			COALESCE(icon_url, ''), manifest, COALESCE(tags, ARRAY[]::TEXT[]),
			install_count, created_at, updated_at
		FROM catalog_templates
		WHERE category = $1
		ORDER BY display_name ASC
	`

	rows, err := t.db.DB().QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates by category: %w", err)
	}
	defer rows.Close()

	return t.scanTemplates(rows)
}

// CreateTemplate creates a new template in the catalog_templates table.
func (t *TemplateDB) CreateTemplate(ctx context.Context, template *Template) error {
	query := `
		INSERT INTO catalog_templates (
			repository_id, name, display_name, description, category, app_type,
			icon_url, manifest, tags, install_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`

	return t.db.DB().QueryRowContext(ctx, query,
		template.RepositoryID, template.Name, template.DisplayName, template.Description,
		template.Category, template.AppType, template.IconURL, template.Manifest,
		pq.Array(template.Tags), template.InstallCount, time.Now(), time.Now(),
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)
}

// UpdateTemplate updates an existing template in the catalog_templates table.
func (t *TemplateDB) UpdateTemplate(ctx context.Context, template *Template) error {
	query := `
		UPDATE catalog_templates
		SET
			display_name = $1, description = $2, category = $3, app_type = $4,
			icon_url = $5, manifest = $6, tags = $7, updated_at = $8
		WHERE name = $9
	`

	result, err := t.db.DB().ExecContext(ctx, query,
		template.DisplayName, template.Description, template.Category, template.AppType,
		template.IconURL, template.Manifest, pq.Array(template.Tags), time.Now(), template.Name,
	)

	if err != nil {
		return fmt.Errorf("failed to update template %s: %w", template.Name, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteTemplate deletes a template from the catalog_templates table.
func (t *TemplateDB) DeleteTemplate(ctx context.Context, name string) error {
	query := `DELETE FROM catalog_templates WHERE name = $1`

	result, err := t.db.DB().ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to delete template %s: %w", name, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// IncrementInstallCount increments the install_count for a template.
func (t *TemplateDB) IncrementInstallCount(ctx context.Context, name string) error {
	query := `
		UPDATE catalog_templates
		SET install_count = install_count + 1, updated_at = $1
		WHERE name = $2
	`

	_, err := t.db.DB().ExecContext(ctx, query, time.Now(), name)
	return err
}

// scanTemplates scans multiple template rows from a query result.
// scanTemplates scans template rows from a query result.
// FIX P1: Use pq.Array() for PostgreSQL TEXT[] column scanning.
func (t *TemplateDB) scanTemplates(rows *sql.Rows) ([]*Template, error) {
	var templates []*Template

	for rows.Next() {
		template := &Template{}
		err := rows.Scan(
			&template.ID, &template.RepositoryID, &template.Name, &template.DisplayName,
			&template.Description, &template.Category, &template.AppType, &template.IconURL,
			&template.Manifest, pq.Array(&template.Tags), &template.InstallCount,
			&template.CreatedAt, &template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template row: %w", err)
		}
		templates = append(templates, template)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating template rows: %w", err)
	}

	return templates, nil
}
