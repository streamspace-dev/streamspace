// Package db provides PostgreSQL database access and management for StreamSpace.
//
// This file implements installed application management and access control.
//
// Purpose:
// - CRUD operations for installed applications
// - Application configuration management
// - Group-based access control for applications
// - Application enable/disable functionality
//
// Features:
// - Install applications from catalog templates
// - Custom display names for user dashboard
// - Configuration storage in JSONB
// - Group access permissions
// - Enable/disable applications
//
// Database Schema:
//   - installed_applications table: Installed application instances
//     - id (varchar): Primary key (UUID)
//     - catalog_template_id (int): Foreign key to catalog_templates
//     - name (varchar): Internal name with GUID suffix
//     - display_name (varchar): Custom display name for dashboard
//     - folder_path (varchar): Path to configuration folder
//     - enabled (boolean): Whether application is active
//     - configuration (jsonb): Application-specific settings
//     - created_by (varchar): User who installed the application
//     - created_at, updated_at: Timestamps
//
//   - application_group_access table: Group permissions for applications
//     - id (varchar): Primary key (UUID)
//     - application_id (varchar): Foreign key to installed_applications
//     - group_id (varchar): Foreign key to groups
//     - access_level (varchar): Permission level (view, launch, admin)
//     - created_at: When access was granted
//
// Thread Safety:
// - All database operations are thread-safe via database/sql pool
//
// Example Usage:
//
//	appDB := db.NewApplicationDB(database.DB())
//
//	// Install application
//	app, err := appDB.InstallApplication(ctx, &models.InstallApplicationRequest{
//	    CatalogTemplateID: 1,
//	    DisplayName:       "Firefox Browser",
//	})
//
//	// Grant group access
//	err := appDB.AddGroupAccess(ctx, appID, groupID, "launch")
//
//	// Enable/disable application
//	err := appDB.SetApplicationEnabled(ctx, appID, true)
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/streamspace/streamspace/api/internal/models"
)

// ApplicationDB handles database operations for installed applications
type ApplicationDB struct {
	db *sql.DB
}

// NewApplicationDB creates a new ApplicationDB instance
func NewApplicationDB(db *sql.DB) *ApplicationDB {
	return &ApplicationDB{db: db}
}

// InstallApplication installs a new application from the catalog
func (a *ApplicationDB) InstallApplication(ctx context.Context, req *models.InstallApplicationRequest, userID string) (*models.InstalledApplication, error) {
	appID := uuid.New().String()
	guidSuffix := uuid.New().String()[:8]

	// Get template info for default name
	var templateName, templateDisplayName string
	err := a.db.QueryRowContext(ctx, `
		SELECT name, display_name FROM catalog_templates WHERE id = $1
	`, req.CatalogTemplateID).Scan(&templateName, &templateDisplayName)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Set default display name if not provided
	displayName := req.DisplayName
	if displayName == "" {
		displayName = templateDisplayName
	}

	// Create internal name with GUID suffix
	name := fmt.Sprintf("%s-%s", templateName, guidSuffix)
	folderPath := fmt.Sprintf("apps/%s", name)

	// Serialize configuration
	configJSON := []byte("{}")
	if req.Configuration != nil {
		configJSON, err = json.Marshal(req.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal configuration: %w", err)
		}
	}

	app := &models.InstalledApplication{
		ID:                appID,
		CatalogTemplateID: req.CatalogTemplateID,
		Name:              name,
		DisplayName:       displayName,
		FolderPath:        folderPath,
		Enabled:           true,
		Configuration:     req.Configuration,
		CreatedBy:         userID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	query := `
		INSERT INTO installed_applications (
			id, catalog_template_id, name, display_name, folder_path,
			enabled, configuration, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = a.db.ExecContext(ctx, query,
		app.ID, app.CatalogTemplateID, app.Name, app.DisplayName, app.FolderPath,
		app.Enabled, configJSON, app.CreatedBy, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to install application: %w", err)
	}

	return app, nil
}

// GetApplication retrieves an installed application by ID
func (a *ApplicationDB) GetApplication(ctx context.Context, appID string) (*models.InstalledApplication, error) {
	app := &models.InstalledApplication{}
	var configJSON []byte

	query := `
		SELECT
			ia.id, ia.catalog_template_id, ia.name, ia.display_name, ia.folder_path,
			ia.enabled, ia.configuration, ia.created_by, ia.created_at, ia.updated_at,
			ct.name as template_name, ct.display_name as template_display_name,
			ct.description, ct.category, ct.app_type, ct.icon_url, ct.manifest
		FROM installed_applications ia
		JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
		WHERE ia.id = $1
	`

	err := a.db.QueryRowContext(ctx, query, appID).Scan(
		&app.ID, &app.CatalogTemplateID, &app.Name, &app.DisplayName, &app.FolderPath,
		&app.Enabled, &configJSON, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt,
		&app.TemplateName, &app.TemplateDisplayName, &app.Description,
		&app.Category, &app.AppType, &app.IconURL, &app.Manifest,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("application not found")
		}
		return nil, err
	}

	// Unmarshal configuration
	if len(configJSON) > 0 {
		json.Unmarshal(configJSON, &app.Configuration)
	}

	return app, nil
}

// ListApplications retrieves all installed applications with optional filtering
func (a *ApplicationDB) ListApplications(ctx context.Context, enabledOnly bool) ([]*models.InstalledApplication, error) {
	query := `
		SELECT
			ia.id, ia.catalog_template_id, ia.name, ia.display_name, ia.folder_path,
			ia.enabled, ia.configuration, ia.created_by, ia.created_at, ia.updated_at,
			ct.name as template_name, ct.display_name as template_display_name,
			ct.description, ct.category, ct.app_type, ct.icon_url
		FROM installed_applications ia
		JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
		WHERE 1=1
	`

	if enabledOnly {
		query += " AND ia.enabled = true"
	}

	query += " ORDER BY ia.display_name ASC"

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := []*models.InstalledApplication{}
	for rows.Next() {
		app := &models.InstalledApplication{}
		var configJSON []byte

		err := rows.Scan(
			&app.ID, &app.CatalogTemplateID, &app.Name, &app.DisplayName, &app.FolderPath,
			&app.Enabled, &configJSON, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt,
			&app.TemplateName, &app.TemplateDisplayName, &app.Description,
			&app.Category, &app.AppType, &app.IconURL,
		)
		if err != nil {
			continue
		}

		// Unmarshal configuration
		if len(configJSON) > 0 {
			json.Unmarshal(configJSON, &app.Configuration)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// UpdateApplication updates an installed application
func (a *ApplicationDB) UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) error {
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.DisplayName != nil {
		updates = append(updates, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *req.DisplayName)
		argIdx++
	}

	if req.Enabled != nil {
		updates = append(updates, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *req.Enabled)
		argIdx++
	}

	if req.Configuration != nil {
		configJSON, err := json.Marshal(req.Configuration)
		if err != nil {
			return fmt.Errorf("failed to marshal configuration: %w", err)
		}
		updates = append(updates, fmt.Sprintf("configuration = $%d", argIdx))
		args = append(args, configJSON)
		argIdx++
	}

	if len(updates) == 0 {
		return nil // Nothing to update
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, appID)

	query := fmt.Sprintf("UPDATE installed_applications SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argIdx)

	_, err := a.db.ExecContext(ctx, query, args...)
	return err
}

// DeleteApplication deletes an installed application
func (a *ApplicationDB) DeleteApplication(ctx context.Context, appID string) error {
	// Delete group access first
	_, err := a.db.ExecContext(ctx, "DELETE FROM application_group_access WHERE application_id = $1", appID)
	if err != nil {
		return err
	}

	// Delete application
	_, err = a.db.ExecContext(ctx, "DELETE FROM installed_applications WHERE id = $1", appID)
	return err
}

// SetApplicationEnabled enables or disables an application
func (a *ApplicationDB) SetApplicationEnabled(ctx context.Context, appID string, enabled bool) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE installed_applications
		SET enabled = $1, updated_at = $2
		WHERE id = $3
	`, enabled, time.Now(), appID)

	return err
}

// === Group Access Operations ===

// AddGroupAccess grants a group access to an application
func (a *ApplicationDB) AddGroupAccess(ctx context.Context, appID, groupID, accessLevel string) error {
	if accessLevel == "" {
		accessLevel = "launch"
	}

	id := uuid.New().String()

	query := `
		INSERT INTO application_group_access (id, application_id, group_id, access_level, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (application_id, group_id) DO UPDATE
		SET access_level = $4
	`

	_, err := a.db.ExecContext(ctx, query, id, appID, groupID, accessLevel, time.Now())
	return err
}

// RemoveGroupAccess removes a group's access to an application
func (a *ApplicationDB) RemoveGroupAccess(ctx context.Context, appID, groupID string) error {
	_, err := a.db.ExecContext(ctx, `
		DELETE FROM application_group_access
		WHERE application_id = $1 AND group_id = $2
	`, appID, groupID)

	return err
}

// GetApplicationGroups retrieves all groups with access to an application
func (a *ApplicationDB) GetApplicationGroups(ctx context.Context, appID string) ([]*models.ApplicationGroupAccess, error) {
	query := `
		SELECT
			aga.id, aga.application_id, aga.group_id, aga.access_level, aga.created_at,
			g.name, g.display_name
		FROM application_group_access aga
		JOIN groups g ON aga.group_id = g.id
		WHERE aga.application_id = $1
		ORDER BY g.display_name ASC
	`

	rows, err := a.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accessList := []*models.ApplicationGroupAccess{}
	for rows.Next() {
		access := &models.ApplicationGroupAccess{}
		err := rows.Scan(
			&access.ID, &access.ApplicationID, &access.GroupID,
			&access.AccessLevel, &access.CreatedAt,
			&access.GroupName, &access.GroupDisplayName,
		)
		if err != nil {
			continue
		}
		accessList = append(accessList, access)
	}

	return accessList, nil
}

// UpdateGroupAccessLevel updates a group's access level for an application
func (a *ApplicationDB) UpdateGroupAccessLevel(ctx context.Context, appID, groupID, accessLevel string) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE application_group_access
		SET access_level = $1
		WHERE application_id = $2 AND group_id = $3
	`, accessLevel, appID, groupID)

	return err
}

// HasGroupAccess checks if a group has access to an application
func (a *ApplicationDB) HasGroupAccess(ctx context.Context, appID, groupID string) (bool, error) {
	var exists bool
	err := a.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM application_group_access WHERE application_id = $1 AND group_id = $2)
	`, appID, groupID).Scan(&exists)

	return exists, err
}

// GetUserAccessibleApplications retrieves applications accessible to a user (via their groups, as creator, or public)
func (a *ApplicationDB) GetUserAccessibleApplications(ctx context.Context, userID string) ([]*models.InstalledApplication, error) {
	// Simplified query: return all enabled applications where user has group access,
	// is the creator, or the app has no group restrictions (public to all)
	query := `
		SELECT DISTINCT
			ia.id, ia.catalog_template_id, ia.name, ia.display_name, ia.folder_path,
			ia.enabled, ia.configuration, ia.created_by, ia.created_at, ia.updated_at,
			ct.name as template_name, ct.display_name as template_display_name,
			ct.description, ct.category, ct.app_type, ct.icon_url
		FROM installed_applications ia
		JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
		WHERE ia.enabled = true
		AND (
			ia.created_by = $1
			OR EXISTS (
				SELECT 1 FROM application_group_access aga
				JOIN group_memberships gm ON aga.group_id = gm.group_id
				WHERE aga.application_id = ia.id AND gm.user_id = $1
			)
			OR NOT EXISTS (
				SELECT 1 FROM application_group_access aga2
				WHERE aga2.application_id = ia.id
			)
		)
		ORDER BY ia.display_name ASC
	`

	rows, err := a.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := []*models.InstalledApplication{}
	for rows.Next() {
		app := &models.InstalledApplication{}
		var configJSON []byte

		err := rows.Scan(
			&app.ID, &app.CatalogTemplateID, &app.Name, &app.DisplayName, &app.FolderPath,
			&app.Enabled, &configJSON, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt,
			&app.TemplateName, &app.TemplateDisplayName, &app.Description,
			&app.Category, &app.AppType, &app.IconURL,
		)
		if err != nil {
			continue
		}

		// Unmarshal configuration
		if len(configJSON) > 0 {
			json.Unmarshal(configJSON, &app.Configuration)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// GetApplicationTemplateConfig retrieves the template's configurable options
func (a *ApplicationDB) GetApplicationTemplateConfig(ctx context.Context, appID string) (map[string]interface{}, error) {
	var manifest string
	err := a.db.QueryRowContext(ctx, `
		SELECT ct.manifest
		FROM installed_applications ia
		JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
		WHERE ia.id = $1
	`, appID).Scan(&manifest)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if manifest != "" {
		json.Unmarshal([]byte(manifest), &config)
	}

	return config, nil
}
