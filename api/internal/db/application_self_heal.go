// Package db provides self-healing functions for database integrity.
//
// This file implements automatic repair of catalog_template_id references
// in installed_applications table.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// HealApplicationCatalogLinks repairs installed_applications with NULL catalog_template_id.
//
// This function:
// 1. Finds all applications with catalog_template_id = NULL
// 2. Attempts to match them to catalog_templates by name prefix
// 3. Updates the database with the correct template ID
//
// This is a self-healing mechanism for the architectural issue where applications
// lose their template link. It should run on API startup.
//
// Returns the number of applications healed and any error encountered.
func (a *ApplicationDB) HealApplicationCatalogLinks(ctx context.Context) (int, error) {
	log.Println("[ApplicationDB] Starting self-heal: Checking for applications with missing catalog_template_id...")

	// Find all applications with NULL catalog_template_id
	query := `
		SELECT id, name, display_name
		FROM installed_applications
		WHERE catalog_template_id IS NULL
	`

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query broken applications: %w", err)
	}
	defer rows.Close()

	type brokenApp struct {
		ID          string
		Name        string
		DisplayName string
	}

	var brokenApps []brokenApp
	for rows.Next() {
		var app brokenApp
		if err := rows.Scan(&app.ID, &app.Name, &app.DisplayName); err != nil {
			log.Printf("[ApplicationDB] Warning: Failed to scan broken app: %v", err)
			continue
		}
		brokenApps = append(brokenApps, app)
	}

	if len(brokenApps) == 0 {
		log.Println("[ApplicationDB] ✓ No broken applications found - all catalog links are valid")
		return 0, nil
	}

	log.Printf("[ApplicationDB] ⚠️  Found %d applications with missing catalog_template_id - attempting repair...", len(brokenApps))

	healedCount := 0
	for _, app := range brokenApps {
		// Extract base name (remove GUID suffix)
		// Name format: "templatename-guidhere"
		baseName := app.Name
		if idx := strings.LastIndex(app.Name, "-"); idx > 0 && len(app.Name[idx+1:]) == 8 {
			baseName = app.Name[:idx]
		}

		// Try to find matching catalog template by name
		var catalogTemplateID int
		err := a.db.QueryRowContext(ctx, `
			SELECT id FROM catalog_templates
			WHERE name = $1 OR display_name = $2
			LIMIT 1
		`, baseName, app.DisplayName).Scan(&catalogTemplateID)

		if err == sql.ErrNoRows {
			log.Printf("[ApplicationDB] ⚠️  Could not find catalog template for app '%s' (base: '%s', display: '%s') - manual intervention required",
				app.Name, baseName, app.DisplayName)
			continue
		}

		if err != nil {
			log.Printf("[ApplicationDB] ⚠️  Database error looking up template for app '%s': %v", app.Name, err)
			continue
		}

		// Update the application with the correct catalog_template_id
		_, err = a.db.ExecContext(ctx, `
			UPDATE installed_applications
			SET catalog_template_id = $1
			WHERE id = $2
		`, catalogTemplateID, app.ID)

		if err != nil {
			log.Printf("[ApplicationDB] ⚠️  Failed to heal app '%s': %v", app.Name, err)
			continue
		}

		log.Printf("[ApplicationDB] ✓ Healed '%s' (ID: %s) → catalog_template_id = %d",
			app.Name, app.ID, catalogTemplateID)
		healedCount++
	}

	if healedCount > 0 {
		log.Printf("[ApplicationDB] ✓ Self-heal complete: Repaired %d/%d applications", healedCount, len(brokenApps))
	} else {
		log.Printf("[ApplicationDB] ⚠️  Self-heal complete: Could not repair any applications - manual intervention required")
	}

	return healedCount, nil
}
