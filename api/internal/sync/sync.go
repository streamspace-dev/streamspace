package sync

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/streamspace/streamspace/api/internal/db"
)

// SyncService manages template repository synchronization
type SyncService struct {
	db        *db.Database
	workDir   string
	gitClient *GitClient
	parser    *TemplateParser
}

// NewSyncService creates a new sync service
func NewSyncService(database *db.Database) (*SyncService, error) {
	workDir := os.Getenv("SYNC_WORK_DIR")
	if workDir == "" {
		workDir = "/tmp/streamspace-repos"
	}

	// Ensure work directory exists
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	gitClient := NewGitClient()
	parser := NewTemplateParser()

	return &SyncService{
		db:        database,
		workDir:   workDir,
		gitClient: gitClient,
		parser:    parser,
	}, nil
}

// SyncRepository synchronizes a template repository
func (s *SyncService) SyncRepository(ctx context.Context, repoID int) error {
	log.Printf("Starting sync for repository %d", repoID)

	// Get repository details
	repo, err := s.getRepository(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Update status to Syncing
	if err := s.updateRepositoryStatus(ctx, repoID, "syncing", ""); err != nil {
		log.Printf("Failed to update repository status: %v", err)
	}

	// Clone or update repository
	repoPath := filepath.Join(s.workDir, fmt.Sprintf("repo-%d", repoID))

	var cloneErr error
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone repository
		log.Printf("Cloning repository %s to %s", repo.URL, repoPath)
		cloneErr = s.gitClient.Clone(ctx, repo.URL, repoPath, repo.Branch, repo.AuthConfig)
	} else {
		// Pull latest changes
		log.Printf("Pulling latest changes for repository %s", repo.URL)
		cloneErr = s.gitClient.Pull(ctx, repoPath, repo.Branch, repo.AuthConfig)
	}

	if cloneErr != nil {
		errMsg := fmt.Sprintf("Git operation failed: %v", cloneErr)
		s.updateRepositoryStatus(ctx, repoID, "failed", errMsg)
		return fmt.Errorf("git operation failed: %w", cloneErr)
	}

	// Parse templates from repository
	templates, err := s.parser.ParseRepository(repoPath)
	if err != nil {
		errMsg := fmt.Sprintf("Template parsing failed: %v", err)
		s.updateRepositoryStatus(ctx, repoID, "failed", errMsg)
		return fmt.Errorf("template parsing failed: %w", err)
	}

	log.Printf("Found %d templates in repository %d", len(templates), repoID)

	// Update catalog
	if err := s.updateCatalog(ctx, repoID, templates); err != nil {
		errMsg := fmt.Sprintf("Catalog update failed: %v", err)
		s.updateRepositoryStatus(ctx, repoID, "failed", errMsg)
		return fmt.Errorf("catalog update failed: %w", err)
	}

	// Update repository status to synced
	if err := s.updateRepositoryStatus(ctx, repoID, "synced", ""); err != nil {
		log.Printf("Failed to update repository status: %v", err)
	}

	// Update last_sync timestamp and template_count
	_, err = s.db.DB().ExecContext(ctx, `
		UPDATE repositories
		SET last_sync = $1, template_count = $2, updated_at = $3
		WHERE id = $4
	`, time.Now(), len(templates), time.Now(), repoID)
	if err != nil {
		log.Printf("Failed to update repository sync time: %v", err)
	}

	log.Printf("Successfully synced repository %d with %d templates", repoID, len(templates))
	return nil
}

// SyncAllRepositories synchronizes all repositories
func (s *SyncService) SyncAllRepositories(ctx context.Context) error {
	rows, err := s.db.DB().QueryContext(ctx, `
		SELECT id FROM repositories
		WHERE status != 'syncing'
		ORDER BY id ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var repoIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			log.Printf("Failed to scan repository ID: %v", err)
			continue
		}
		repoIDs = append(repoIDs, id)
	}

	log.Printf("Syncing %d repositories", len(repoIDs))

	successCount := 0
	failCount := 0

	for _, repoID := range repoIDs {
		if err := s.SyncRepository(ctx, repoID); err != nil {
			log.Printf("Failed to sync repository %d: %v", repoID, err)
			failCount++
		} else {
			successCount++
		}
	}

	log.Printf("Sync completed: %d succeeded, %d failed", successCount, failCount)
	return nil
}

// getRepository retrieves repository details from database
func (s *SyncService) getRepository(ctx context.Context, repoID int) (*Repository, error) {
	repo := &Repository{}

	var authType, authSecret sql.NullString
	err := s.db.DB().QueryRowContext(ctx, `
		SELECT id, name, url, branch, auth_type, auth_secret
		FROM repositories
		WHERE id = $1
	`, repoID).Scan(&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &authType, &authSecret)

	if err != nil {
		return nil, err
	}

	if authType.Valid {
		repo.AuthConfig = &AuthConfig{
			Type:   authType.String,
			Secret: authSecret.String,
		}
	}

	return repo, nil
}

// updateRepositoryStatus updates the repository status
func (s *SyncService) updateRepositoryStatus(ctx context.Context, repoID int, status, errorMsg string) error {
	_, err := s.db.DB().ExecContext(ctx, `
		UPDATE repositories
		SET status = $1, error_message = $2, updated_at = $3
		WHERE id = $4
	`, status, errorMsg, time.Now(), repoID)

	return err
}

// updateCatalog updates the catalog_templates table with parsed templates
func (s *SyncService) updateCatalog(ctx context.Context, repoID int, templates []*ParsedTemplate) error {
	// Start transaction
	tx, err := s.db.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing templates for this repository
	_, err = tx.ExecContext(ctx, `
		DELETE FROM catalog_templates WHERE repository_id = $1
	`, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete old templates: %w", err)
	}

	// Insert new templates
	for _, template := range templates {
		// Convert manifest to JSON string for storage
		manifestJSON := template.Manifest

		_, err = tx.ExecContext(ctx, `
			INSERT INTO catalog_templates (
				repository_id, name, display_name, description, category,
				app_type, icon_url, manifest, tags, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, repoID, template.Name, template.DisplayName, template.Description,
			template.Category, template.AppType, template.Icon, manifestJSON,
			template.Tags, time.Now(), time.Now())

		if err != nil {
			return fmt.Errorf("failed to insert template %s: %w", template.Name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Updated catalog with %d templates for repository %d", len(templates), repoID)
	return nil
}

// StartScheduledSync starts the scheduled sync loop
func (s *SyncService) StartScheduledSync(ctx context.Context, interval time.Duration) {
	log.Printf("Starting scheduled sync with interval: %s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial sync
	go func() {
		if err := s.SyncAllRepositories(ctx); err != nil {
			log.Printf("Initial sync failed: %v", err)
		}
	}()

	for {
		select {
		case <-ticker.C:
			log.Println("Running scheduled repository sync")
			if err := s.SyncAllRepositories(ctx); err != nil {
				log.Printf("Scheduled sync failed: %v", err)
			}
		case <-ctx.Done():
			log.Println("Scheduled sync stopped")
			return
		}
	}
}

// Repository represents a template repository
type Repository struct {
	ID         int
	Name       string
	URL        string
	Branch     string
	AuthConfig *AuthConfig
}

// AuthConfig represents authentication configuration for Git
type AuthConfig struct {
	Type   string // none, ssh, token, basic
	Secret string // Secret value (SSH key, token, or password)
}
