// Package db provides PostgreSQL database access and management for StreamSpace.
//
// This file implements user management and authentication data access.
//
// Purpose:
// - CRUD operations for user accounts
// - Password hashing and verification with bcrypt
// - User authentication and authorization
// - User quota and resource limit management
// - User preferences and settings storage
// - Group membership management
//
// Features:
// - Secure password hashing with bcrypt (cost factor 10)
// - User search and filtering by username/email/role/provider
// - Quota enforcement integration with quota system
// - User group membership tracking
// - OAuth provider linking (OIDC, SAML)
// - MFA (TOTP) configuration storage support
// - Last login tracking for auditing
//
// Database Schema:
//   - users table: Core user account data
//     - id (varchar): Primary key (UUID)
//     - username (varchar): Unique username
//     - email (varchar): Unique email address
//     - password_hash (varchar): bcrypt hashed password (local auth only)
//     - role (varchar): User role (user, admin, superadmin)
//     - provider (varchar): Auth provider (local, saml, oidc)
//     - active (boolean): Account active status
//     - created_at, updated_at: Timestamps
//     - last_login: Last successful authentication
//
//   - user_quotas table: Resource limits per user
//     - user_id: Foreign key to users
//     - max_sessions, max_cpu, max_memory, max_storage: Limits
//     - used_sessions, used_cpu, used_memory, used_storage: Current usage
//
// Implementation Details:
// - Passwords never stored in plaintext (bcrypt with cost 10)
// - User lookups indexed by username and email for performance
// - Quota stored as separate table with foreign key constraint
// - Group membership managed via group_memberships junction table
// - Default quota created automatically on user creation
// - Supports multiple authentication providers (local, SAML, OIDC)
//
// Thread Safety:
// - All database operations are thread-safe via database/sql pool
// - Safe for concurrent access from multiple goroutines
// - bcrypt operations are CPU-intensive but safe for concurrent use
//
// Dependencies:
// - golang.org/x/crypto/bcrypt for password hashing
// - github.com/google/uuid for ID generation
// - models package for data structures
//
// Example Usage:
//
//	userDB := db.NewUserDB(database.DB())
//
//	// Create user with password
//	user, err := userDB.CreateUser(ctx, &models.CreateUserRequest{
//	    Username: "alice",
//	    Email:    "alice@example.com",
//	    Password: "securepassword",
//	    FullName: "Alice Smith",
//	    Role:     "user",
//	    Provider: "local",
//	})
//
//	// Authenticate user
//	user, err := userDB.VerifyPassword(ctx, "alice", "securepassword")
//
//	// Update user quota
//	maxSessions := 10
//	err := userDB.SetUserQuota(ctx, userID, &models.SetQuotaRequest{
//	    MaxSessions: &maxSessions,
//	})
//
//	// Get or create SAML user (SSO)
//	user, err := userDB.GetOrCreateSAMLUser(ctx, "bob", "bob@company.com", "Bob Jones", "saml-provider")
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/streamspace/streamspace/api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// UserDB handles database operations for users
type UserDB struct {
	db *sql.DB
}

// NewUserDB creates a new UserDB instance
func NewUserDB(db *sql.DB) *UserDB {
	return &UserDB{db: db}
}

// DB returns the underlying database connection
func (u *UserDB) DB() *sql.DB {
	return u.db
}

// CreateUser creates a new user
func (u *UserDB) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	user := &models.User{
		ID:        uuid.New().String(),
		Username:  req.Username,
		Email:     req.Email,
		FullName:  req.FullName,
		Role:      req.Role,
		Provider:  req.Provider,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set defaults
	if user.Role == "" {
		user.Role = "user"
	}
	if user.Provider == "" {
		user.Provider = "local"
	}

	// Hash password if local auth
	if user.Provider == "local" && req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = string(hashedPassword)
	}

	query := `
		INSERT INTO users (id, username, email, full_name, role, provider, password_hash, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := u.db.ExecContext(ctx, query,
		user.ID, user.Username, user.Email, user.FullName,
		user.Role, user.Provider, user.PasswordHash, user.Active,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create default quota
	if err := u.createDefaultQuota(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to create default quota: %w", err)
	}

	// Add user to all_users group
	if err := u.addToAllUsersGroup(ctx, user.ID); err != nil {
		// Log but don't fail user creation
		fmt.Printf("Warning: failed to add user %s to all_users group: %v\n", user.ID, err)
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (u *UserDB) GetUser(ctx context.Context, userID string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, full_name, role, provider, active, created_at, updated_at, last_login
		FROM users
		WHERE id = $1
	`

	err := u.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.FullName,
		&user.Role, &user.Provider, &user.Active,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	// Load quota
	quota, err := u.GetUserQuota(ctx, userID)
	if err == nil {
		user.Quota = quota
	}

	// Load groups
	groups, err := u.GetUserGroups(ctx, userID)
	if err == nil {
		user.Groups = groups
	}

	return user, nil
}

// GetUserByUsername retrieves a user by username
func (u *UserDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	// SECURITY FIX: Select password_hash only - this method is used for authentication
	// Note: password_hash is needed here for VerifyPassword() to work
	query := `
		SELECT id, username, email, full_name, role, provider, password_hash, active, created_at, updated_at, last_login
		FROM users
		WHERE username = $1
	`

	err := u.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.FullName,
		&user.Role, &user.Provider, &user.PasswordHash, &user.Active,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email address
func (u *UserDB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	// SECURITY FIX: Don't expose password_hash unless absolutely necessary
	// This method may be used for user lookups where password is not needed
	query := `
		SELECT id, username, email, full_name, role, provider, active, created_at, updated_at, last_login
		FROM users
		WHERE email = $1
	`

	err := u.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.FullName,
		&user.Role, &user.Provider, &user.Active,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	// Load quota
	quota, err := u.GetUserQuota(ctx, user.ID)
	if err == nil {
		user.Quota = quota
	}

	// Load groups
	groups, err := u.GetUserGroups(ctx, user.ID)
	if err == nil {
		user.Groups = groups
	}

	return user, nil
}

// ListUsers retrieves all users with optional filtering
func (u *UserDB) ListUsers(ctx context.Context, role, provider string, activeOnly bool) ([]*models.User, error) {
	query := `
		SELECT id, username, email, full_name, role, provider, active, created_at, updated_at, last_login
		FROM users
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if role != "" {
		query += fmt.Sprintf(" AND role = $%d", argIdx)
		args = append(args, role)
		argIdx++
	}

	if provider != "" {
		query += fmt.Sprintf(" AND provider = $%d", argIdx)
		args = append(args, provider)
		argIdx++
	}

	if activeOnly {
		query += " AND active = true"
	}

	query += " ORDER BY username ASC"

	rows, err := u.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user := &models.User{}
		// BUG FIX: Return error instead of continuing - fail fast on database errors
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.FullName,
			&user.Role, &user.Provider, &user.Active,
			&user.CreatedAt, &user.UpdatedAt, &user.LastLogin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, user)
	}

	// BUG FIX: Check rows.Err() to catch any errors that occurred during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}

// UpdateUser updates user information
func (u *UserDB) UpdateUser(ctx context.Context, userID string, req *models.UpdateUserRequest) error {
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Email != nil {
		updates = append(updates, fmt.Sprintf("email = $%d", argIdx))
		args = append(args, *req.Email)
		argIdx++
	}

	if req.FullName != nil {
		updates = append(updates, fmt.Sprintf("full_name = $%d", argIdx))
		args = append(args, *req.FullName)
		argIdx++
	}

	if req.Role != nil {
		updates = append(updates, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *req.Role)
		argIdx++
	}

	if req.Active != nil {
		updates = append(updates, fmt.Sprintf("active = $%d", argIdx))
		args = append(args, *req.Active)
		argIdx++
	}

	if len(updates) == 0 {
		return nil // Nothing to update
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, userID)

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		join(updates, ", "), argIdx)

	_, err := u.db.ExecContext(ctx, query, args...)
	return err
}

// DeleteUser deletes a user
func (u *UserDB) DeleteUser(ctx context.Context, userID string) error {
	// BUG FIX: Use transaction to ensure atomicity - all deletes succeed or all fail
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Delete quota first
	_, err = tx.ExecContext(ctx, "DELETE FROM user_quotas WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user quotas: %w", err)
	}

	// Delete group memberships
	_, err = tx.ExecContext(ctx, "DELETE FROM group_memberships WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete group memberships: %w", err)
	}

	// Delete user
	_, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the user's last login timestamp
func (u *UserDB) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := u.db.ExecContext(ctx, `
		UPDATE users SET last_login = $1, updated_at = $1 WHERE id = $2
	`, time.Now(), userID)
	return err
}

// UpdatePassword updates a user's password (local auth only)
func (u *UserDB) UpdatePassword(ctx context.Context, userID string, newPassword string) error {
	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update the password in the database
	_, err = u.db.ExecContext(ctx, `
		UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3
	`, string(hashedPassword), time.Now(), userID)
	return err
}

// VerifyPassword verifies a user's password (for local auth)
func (u *UserDB) VerifyPassword(ctx context.Context, username, password string) (*models.User, error) {
	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if user.Provider != "local" {
		return nil, fmt.Errorf("user is not configured for local authentication")
	}

	if !user.Active {
		return nil, fmt.Errorf("user account is disabled")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// Update last login
	_ = u.UpdateLastLogin(ctx, user.ID)

	return user, nil
}

// GetOrCreateSAMLUser gets or creates a user from SAML assertion
func (u *UserDB) GetOrCreateSAMLUser(ctx context.Context, username, email, fullName, provider string) (*models.User, error) {
	// Try to find existing user by username
	user, err := u.GetUserByUsername(ctx, username)
	if err == nil {
		// User exists, update login time and return
		_ = u.UpdateLastLogin(ctx, user.ID)
		return user, nil
	}

	// Create new user
	req := &models.CreateUserRequest{
		Username: username,
		Email:    email,
		FullName: fullName,
		Provider: provider,
		Role:     "user",
	}

	return u.CreateUser(ctx, req)
}

// === User Quota Operations ===

// GetUserQuota retrieves quota for a user
func (u *UserDB) GetUserQuota(ctx context.Context, userID string) (*models.UserQuota, error) {
	quota := &models.UserQuota{}
	query := `
		SELECT user_id, max_sessions, max_cpu, max_memory, max_storage,
		       used_sessions, used_cpu, used_memory, used_storage,
		       created_at, updated_at
		FROM user_quotas
		WHERE user_id = $1
	`

	err := u.db.QueryRowContext(ctx, query, userID).Scan(
		&quota.UserID, &quota.MaxSessions, &quota.MaxCPU, &quota.MaxMemory, &quota.MaxStorage,
		&quota.UsedSessions, &quota.UsedCPU, &quota.UsedMemory, &quota.UsedStorage,
		&quota.CreatedAt, &quota.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("quota not found")
		}
		return nil, err
	}

	return quota, nil
}

// SetUserQuota sets or updates quota for a user
func (u *UserDB) SetUserQuota(ctx context.Context, userID string, req *models.SetQuotaRequest) error {
	// Check if quota exists
	_, err := u.GetUserQuota(ctx, userID)
	quotaExists := err == nil

	if quotaExists {
		// Update existing quota
		updates := []string{}
		args := []interface{}{}
		argIdx := 1

		if req.MaxSessions != nil {
			updates = append(updates, fmt.Sprintf("max_sessions = $%d", argIdx))
			args = append(args, *req.MaxSessions)
			argIdx++
		}

		if req.MaxCPU != nil {
			updates = append(updates, fmt.Sprintf("max_cpu = $%d", argIdx))
			args = append(args, *req.MaxCPU)
			argIdx++
		}

		if req.MaxMemory != nil {
			updates = append(updates, fmt.Sprintf("max_memory = $%d", argIdx))
			args = append(args, *req.MaxMemory)
			argIdx++
		}

		if req.MaxStorage != nil {
			updates = append(updates, fmt.Sprintf("max_storage = $%d", argIdx))
			args = append(args, *req.MaxStorage)
			argIdx++
		}

		if len(updates) == 0 {
			return nil
		}

		updates = append(updates, fmt.Sprintf("updated_at = $%d", argIdx))
		args = append(args, time.Now())
		argIdx++

		args = append(args, userID)

		query := fmt.Sprintf("UPDATE user_quotas SET %s WHERE user_id = $%d",
			join(updates, ", "), argIdx)

		_, err = u.db.ExecContext(ctx, query, args...)
		return err
	} else {
		// Create new quota
		return u.createQuota(ctx, userID, req)
	}
}

// createDefaultQuota creates default quota for a new user
func (u *UserDB) createDefaultQuota(ctx context.Context, userID string) error {
	query := `
		INSERT INTO user_quotas (user_id, max_sessions, max_cpu, max_memory, max_storage,
		                         used_sessions, used_cpu, used_memory, used_storage,
		                         created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 0, '0', '0', '0', $6, $7)
	`

	_, err := u.db.ExecContext(ctx, query,
		userID,
		5,      // Default: 5 sessions
		"4000m", // Default: 4 CPU cores
		"16Gi",  // Default: 16GB memory
		"100Gi", // Default: 100GB storage
		time.Now(), time.Now(),
	)

	return err
}

// addToAllUsersGroup adds a user to the default all_users group
func (u *UserDB) addToAllUsersGroup(ctx context.Context, userID string) error {
	query := `
		INSERT INTO group_memberships (id, user_id, group_id, role, created_at)
		SELECT $1, $2, id, 'member', NOW()
		FROM groups WHERE name = 'all_users'
		ON CONFLICT (user_id, group_id) DO NOTHING
	`

	_, err := u.db.ExecContext(ctx, query, uuid.New().String(), userID)
	return err
}

// createQuota creates quota with custom values
func (u *UserDB) createQuota(ctx context.Context, userID string, req *models.SetQuotaRequest) error {
	maxSessions := 5
	maxCPU := "4000m"
	maxMemory := "16Gi"
	maxStorage := "100Gi"

	if req.MaxSessions != nil {
		maxSessions = *req.MaxSessions
	}
	if req.MaxCPU != nil {
		maxCPU = *req.MaxCPU
	}
	if req.MaxMemory != nil {
		maxMemory = *req.MaxMemory
	}
	if req.MaxStorage != nil {
		maxStorage = *req.MaxStorage
	}

	query := `
		INSERT INTO user_quotas (user_id, max_sessions, max_cpu, max_memory, max_storage,
		                         used_sessions, used_cpu, used_memory, used_storage,
		                         created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 0, '0', '0', '0', $6, $7)
	`

	_, err := u.db.ExecContext(ctx, query,
		userID, maxSessions, maxCPU, maxMemory, maxStorage,
		time.Now(), time.Now(),
	)

	return err
}

// UpdateQuotaUsage updates the current usage for a user's quota
func (u *UserDB) UpdateQuotaUsage(ctx context.Context, userID string, sessions int, cpu, memory, storage string) error {
	_, err := u.db.ExecContext(ctx, `
		UPDATE user_quotas
		SET used_sessions = $1, used_cpu = $2, used_memory = $3, used_storage = $4, updated_at = $5
		WHERE user_id = $6
	`, sessions, cpu, memory, storage, time.Now(), userID)

	return err
}

// ListAllUserQuotas retrieves quotas for all users
func (u *UserDB) ListAllUserQuotas(ctx context.Context) ([]*models.UserQuota, error) {
	query := `
		SELECT uq.user_id, uq.max_sessions, uq.max_cpu, uq.max_memory, uq.max_storage,
		       uq.used_sessions, uq.used_cpu, uq.used_memory, uq.used_storage,
		       uq.created_at, uq.updated_at, u.username
		FROM user_quotas uq
		JOIN users u ON uq.user_id = u.id
		ORDER BY u.username ASC
	`

	rows, err := u.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quotas := []*models.UserQuota{}
	for rows.Next() {
		quota := &models.UserQuota{}
		var username string
		err := rows.Scan(
			&quota.UserID, &quota.MaxSessions, &quota.MaxCPU, &quota.MaxMemory, &quota.MaxStorage,
			&quota.UsedSessions, &quota.UsedCPU, &quota.UsedMemory, &quota.UsedStorage,
			&quota.CreatedAt, &quota.UpdatedAt, &username,
		)
		if err != nil {
			continue
		}
		quota.Username = username
		quotas = append(quotas, quota)
	}

	return quotas, nil
}

// DeleteUserQuota deletes a user's quota (resets to defaults)
func (u *UserDB) DeleteUserQuota(ctx context.Context, userID string) error {
	_, err := u.db.ExecContext(ctx, `DELETE FROM user_quotas WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	// Recreate with defaults
	return u.createDefaultQuota(ctx, userID)
}

// GetUserGroups retrieves all groups a user belongs to
func (u *UserDB) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT g.id
		FROM groups g
		JOIN group_memberships gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.name ASC
	`

	rows, err := u.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupIDs := []string{}
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			continue
		}
		groupIDs = append(groupIDs, groupID)
	}

	return groupIDs, nil
}

// Helper function to join strings
func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
