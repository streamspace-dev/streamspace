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
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.FullName,
			&user.Role, &user.Provider, &user.Active,
			&user.CreatedAt, &user.UpdatedAt, &user.LastLogin,
		)
		if err != nil {
			continue
		}
		users = append(users, user)
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
	// Delete quota first
	_, err := u.db.ExecContext(ctx, "DELETE FROM user_quotas WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	// Delete group memberships
	_, err = u.db.ExecContext(ctx, "DELETE FROM group_memberships WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	// Delete user
	_, err = u.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	return err
}

// UpdateLastLogin updates the user's last login timestamp
func (u *UserDB) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := u.db.ExecContext(ctx, `
		UPDATE users SET last_login = $1, updated_at = $1 WHERE id = $2
	`, time.Now(), userID)
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
