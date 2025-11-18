// Package db provides PostgreSQL database access and management for StreamSpace.
//
// This file implements group management and authorization data access.
//
// Purpose:
// - CRUD operations for user groups
// - Group-based access control and permissions
// - Group quota and resource limit management
// - User-to-group membership mapping
// - Group hierarchy support (parent/child groups)
//
// Features:
// - Group-based resource quotas
// - Group permissions and role-based access
// - User membership tracking with roles
// - Group search and filtering by type/parent
// - Quota inheritance from groups to users
// - Member count aggregation
//
// Database Schema:
//   - groups table: Group definitions and metadata
//     - id (varchar): Primary key (UUID)
//     - name (varchar): Unique group name
//     - display_name (varchar): Human-readable name
//     - description (text): Group purpose description
//     - type (varchar): Group type (team, department, etc.)
//     - parent_id (varchar): Optional parent group for hierarchy
//     - created_at, updated_at: Timestamps
//
//   - group_memberships table: User-to-group junction
//     - id (varchar): Primary key
//     - user_id (varchar): Foreign key to users
//     - group_id (varchar): Foreign key to groups
//     - role (varchar): Member role (owner, admin, member)
//     - created_at: When user joined
//
//   - group_quotas table: Resource limits per group
//     - group_id: Foreign key to groups
//     - max_sessions, max_cpu, max_memory, max_storage: Limits
//     - used_sessions, used_cpu, used_memory, used_storage: Current usage
//
// Quota Hierarchy:
//   1. User-specific quotas (most restrictive wins)
//   2. Group quotas (applied to all group members)
//   3. Platform defaults (fallback)
//
// Implementation Details:
// - Groups can have resource quotas that apply to all members
// - Most restrictive quota wins (user vs group vs platform)
// - Quota stored as separate table with foreign key constraint
// - Supports hierarchical groups with parent_id
// - Member counts calculated via JOIN for efficiency
//
// Thread Safety:
// - All database operations are thread-safe via database/sql pool
// - Safe for concurrent access from multiple goroutines
//
// Dependencies:
// - github.com/google/uuid for ID generation
// - models package for data structures
//
// Example Usage:
//
//	groupDB := db.NewGroupDB(database.DB())
//
//	// Create group
//	group, err := groupDB.CreateGroup(ctx, &models.CreateGroupRequest{
//	    Name:        "developers",
//	    DisplayName: "Developers",
//	    Description: "Development team",
//	    Type:        "team",
//	})
//
//	// Set group quota
//	maxSessions := 20
//	err := groupDB.SetGroupQuota(ctx, groupID, &models.SetQuotaRequest{
//	    MaxSessions: &maxSessions,
//	})
//
//	// Add user to group
//	err := groupDB.AddGroupMember(ctx, groupID, &models.AddGroupMemberRequest{
//	    UserID: userID,
//	    Role:   "member",
//	})
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/streamspace/streamspace/api/internal/models"
)

// GroupDB handles database operations for groups
type GroupDB struct {
	db *sql.DB
}

// NewGroupDB creates a new GroupDB instance
func NewGroupDB(db *sql.DB) *GroupDB {
	return &GroupDB{db: db}
}

// CreateGroup creates a new group
func (g *GroupDB) CreateGroup(ctx context.Context, req *models.CreateGroupRequest) (*models.Group, error) {
	group := &models.Group{
		ID:          uuid.New().String(),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Type:        req.Type,
		ParentID:    req.ParentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := `
		INSERT INTO groups (id, name, display_name, description, type, parent_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := g.db.ExecContext(ctx, query,
		group.ID, group.Name, group.DisplayName, group.Description,
		group.Type, group.ParentID, group.CreatedAt, group.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

// GetGroup retrieves a group by ID
func (g *GroupDB) GetGroup(ctx context.Context, groupID string) (*models.Group, error) {
	group := &models.Group{}
	query := `
		SELECT g.id, g.name, COALESCE(g.display_name, '') as display_name,
		       COALESCE(g.description, '') as description, g.type, g.parent_id,
		       g.created_at, g.updated_at, COUNT(gm.user_id) as member_count
		FROM groups g
		LEFT JOIN group_memberships gm ON g.id = gm.group_id
		WHERE g.id = $1
		GROUP BY g.id
	`

	err := g.db.QueryRowContext(ctx, query, groupID).Scan(
		&group.ID, &group.Name, &group.DisplayName, &group.Description,
		&group.Type, &group.ParentID, &group.CreatedAt, &group.UpdatedAt,
		&group.MemberCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, err
	}

	return group, nil
}

// GetGroupByName retrieves a group by name
func (g *GroupDB) GetGroupByName(ctx context.Context, name string) (*models.Group, error) {
	group := &models.Group{}
	query := `
		SELECT g.id, g.name, COALESCE(g.display_name, '') as display_name,
		       COALESCE(g.description, '') as description, g.type, g.parent_id,
		       g.created_at, g.updated_at, COUNT(gm.user_id) as member_count
		FROM groups g
		LEFT JOIN group_memberships gm ON g.id = gm.group_id
		WHERE g.name = $1
		GROUP BY g.id
	`

	err := g.db.QueryRowContext(ctx, query, name).Scan(
		&group.ID, &group.Name, &group.DisplayName, &group.Description,
		&group.Type, &group.ParentID, &group.CreatedAt, &group.UpdatedAt,
		&group.MemberCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, err
	}

	return group, nil
}

// ListGroups retrieves all groups with optional filtering
func (g *GroupDB) ListGroups(ctx context.Context, groupType string, parentID *string) ([]*models.Group, error) {
	query := `
		SELECT g.id, g.name, COALESCE(g.display_name, '') as display_name,
		       COALESCE(g.description, '') as description, COALESCE(g.type, 'team'), g.parent_id,
		       g.created_at, g.updated_at, COUNT(gm.user_id) as member_count
		FROM groups g
		LEFT JOIN group_memberships gm ON g.id = gm.group_id
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if groupType != "" {
		query += fmt.Sprintf(" AND g.type = $%d", argIdx)
		args = append(args, groupType)
		argIdx++
	}

	if parentID != nil {
		query += fmt.Sprintf(" AND g.parent_id = $%d", argIdx)
		args = append(args, *parentID)
		argIdx++
	}

	query += " GROUP BY g.id ORDER BY g.name ASC"

	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []*models.Group{}
	for rows.Next() {
		group := &models.Group{}
		err := rows.Scan(
			&group.ID, &group.Name, &group.DisplayName, &group.Description,
			&group.Type, &group.ParentID, &group.CreatedAt, &group.UpdatedAt,
			&group.MemberCount,
		)
		if err != nil {
			continue
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// UpdateGroup updates group information
func (g *GroupDB) UpdateGroup(ctx context.Context, groupID string, req *models.UpdateGroupRequest) error {
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.DisplayName != nil {
		updates = append(updates, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *req.DisplayName)
		argIdx++
	}

	if req.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}

	if req.Type != nil {
		updates = append(updates, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *req.Type)
		argIdx++
	}

	if len(updates) == 0 {
		return nil // Nothing to update
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, groupID)

	query := fmt.Sprintf("UPDATE groups SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argIdx)

	_, err := g.db.ExecContext(ctx, query, args...)
	return err
}

// DeleteGroup deletes a group
func (g *GroupDB) DeleteGroup(ctx context.Context, groupID string) error {
	// Delete memberships first
	_, err := g.db.ExecContext(ctx, "DELETE FROM group_memberships WHERE group_id = $1", groupID)
	if err != nil {
		return err
	}

	// Delete group quota if exists
	_, err = g.db.ExecContext(ctx, "DELETE FROM group_quotas WHERE group_id = $1", groupID)
	if err != nil {
		return err
	}

	// Delete group
	_, err = g.db.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", groupID)
	return err
}

// === Group Membership Operations ===

// AddGroupMember adds a user to a group
func (g *GroupDB) AddGroupMember(ctx context.Context, groupID string, req *models.AddGroupMemberRequest) error {
	role := req.Role
	if role == "" {
		role = "member"
	}

	membership := &models.GroupMembership{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		GroupID:   groupID,
		Role:      role,
		CreatedAt: time.Now(),
	}

	query := `
		INSERT INTO group_memberships (id, user_id, group_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, group_id) DO UPDATE
		SET role = $4
	`

	_, err := g.db.ExecContext(ctx, query,
		membership.ID, membership.UserID, membership.GroupID,
		membership.Role, membership.CreatedAt,
	)

	return err
}

// RemoveGroupMember removes a user from a group
func (g *GroupDB) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	_, err := g.db.ExecContext(ctx, `
		DELETE FROM group_memberships
		WHERE group_id = $1 AND user_id = $2
	`, groupID, userID)

	return err
}

// GetGroupMembers retrieves all members of a group
func (g *GroupDB) GetGroupMembers(ctx context.Context, groupID string) ([]*models.GroupMembership, error) {
	query := `
		SELECT id, user_id, group_id, role, created_at
		FROM group_memberships
		WHERE group_id = $1
		ORDER BY created_at ASC
	`

	rows, err := g.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []*models.GroupMembership{}
	for rows.Next() {
		member := &models.GroupMembership{}
		err := rows.Scan(
			&member.ID, &member.UserID, &member.GroupID,
			&member.Role, &member.CreatedAt,
		)
		if err != nil {
			continue
		}
		members = append(members, member)
	}

	return members, nil
}

// UpdateGroupMemberRole updates a member's role in a group
func (g *GroupDB) UpdateGroupMemberRole(ctx context.Context, groupID, userID, role string) error {
	_, err := g.db.ExecContext(ctx, `
		UPDATE group_memberships
		SET role = $1
		WHERE group_id = $2 AND user_id = $3
	`, role, groupID, userID)

	return err
}

// IsGroupMember checks if a user is a member of a group
func (g *GroupDB) IsGroupMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	err := g.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM group_memberships WHERE group_id = $1 AND user_id = $2)
	`, groupID, userID).Scan(&exists)

	return exists, err
}

// === Group Quota Operations ===

// GetGroupQuota retrieves quota for a group
func (g *GroupDB) GetGroupQuota(ctx context.Context, groupID string) (*models.GroupQuota, error) {
	quota := &models.GroupQuota{}
	query := `
		SELECT group_id, max_sessions, max_cpu, max_memory, max_storage,
		       used_sessions, used_cpu, used_memory, used_storage,
		       created_at, updated_at
		FROM group_quotas
		WHERE group_id = $1
	`

	err := g.db.QueryRowContext(ctx, query, groupID).Scan(
		&quota.GroupID, &quota.MaxSessions, &quota.MaxCPU, &quota.MaxMemory, &quota.MaxStorage,
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

// SetGroupQuota sets or updates quota for a group
func (g *GroupDB) SetGroupQuota(ctx context.Context, groupID string, req *models.SetQuotaRequest) error {
	// Check if quota exists
	_, err := g.GetGroupQuota(ctx, groupID)
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

		args = append(args, groupID)

		query := fmt.Sprintf("UPDATE group_quotas SET %s WHERE group_id = $%d",
			joinStrings(updates, ", "), argIdx)

		_, err = g.db.ExecContext(ctx, query, args...)
		return err
	} else {
		// Create new quota
		maxSessions := 10
		maxCPU := "8000m"
		maxMemory := "32Gi"
		maxStorage := "500Gi"

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
			INSERT INTO group_quotas (group_id, max_sessions, max_cpu, max_memory, max_storage,
			                          used_sessions, used_cpu, used_memory, used_storage,
			                          created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 0, '0', '0', '0', $6, $7)
		`

		_, err = g.db.ExecContext(ctx, query,
			groupID, maxSessions, maxCPU, maxMemory, maxStorage,
			time.Now(), time.Now(),
		)

		return err
	}
}

// UpdateGroupQuotaUsage updates the current usage for a group's quota
func (g *GroupDB) UpdateGroupQuotaUsage(ctx context.Context, groupID string, sessions int, cpu, memory, storage string) error {
	_, err := g.db.ExecContext(ctx, `
		UPDATE group_quotas
		SET used_sessions = $1, used_cpu = $2, used_memory = $3, used_storage = $4, updated_at = $5
		WHERE group_id = $6
	`, sessions, cpu, memory, storage, time.Now(), groupID)

	return err
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
