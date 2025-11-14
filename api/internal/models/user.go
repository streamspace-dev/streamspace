package models

import (
	"time"
)

// User represents a StreamSpace user
type User struct {
	ID        string    `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	FullName  string    `json:"fullName" db:"full_name"`
	Role      string    `json:"role" db:"role"` // user, admin, operator
	Provider  string    `json:"provider" db:"provider"` // local, saml, oidc
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
	LastLogin *time.Time `json:"lastLogin,omitempty" db:"last_login"`

	// Password hash (only for local auth)
	PasswordHash string `json:"-" db:"password_hash"`

	// Quota information (embedded)
	Quota *UserQuota `json:"quota,omitempty"`

	// Group memberships
	Groups []string `json:"groups,omitempty"`
}

// UserQuota represents resource quotas for a user
type UserQuota struct {
	UserID       string    `json:"userId" db:"user_id"`
	MaxSessions  int       `json:"maxSessions" db:"max_sessions"`
	MaxCPU       string    `json:"maxCpu" db:"max_cpu"`
	MaxMemory    string    `json:"maxMemory" db:"max_memory"`
	MaxStorage   string    `json:"maxStorage" db:"max_storage"`

	// Current usage
	UsedSessions int    `json:"usedSessions" db:"used_sessions"`
	UsedCPU      string `json:"usedCpu" db:"used_cpu"`
	UsedMemory   string `json:"usedMemory" db:"used_memory"`
	UsedStorage  string `json:"usedStorage" db:"used_storage"`

	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

// Group represents a user group/team
type Group struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"displayName" db:"display_name"`
	Description string    `json:"description" db:"description"`
	Type        string    `json:"type" db:"type"` // team, department, project
	ParentID    *string   `json:"parentId,omitempty" db:"parent_id"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`

	// Member count (computed)
	MemberCount int `json:"memberCount" db:"member_count"`

	// Group quota (optional)
	Quota *GroupQuota `json:"quota,omitempty"`
}

// GroupQuota represents resource quotas for a group
type GroupQuota struct {
	GroupID      string    `json:"groupId" db:"group_id"`
	MaxSessions  int       `json:"maxSessions" db:"max_sessions"`
	MaxCPU       string    `json:"maxCpu" db:"max_cpu"`
	MaxMemory    string    `json:"maxMemory" db:"max_memory"`
	MaxStorage   string    `json:"maxStorage" db:"max_storage"`

	// Current usage
	UsedSessions int    `json:"usedSessions" db:"used_sessions"`
	UsedCPU      string `json:"usedCpu" db:"used_cpu"`
	UsedMemory   string `json:"usedMemory" db:"used_memory"`
	UsedStorage  string `json:"usedStorage" db:"used_storage"`

	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

// GroupMembership represents a user's membership in a group
type GroupMembership struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"userId" db:"user_id"`
	GroupID   string    `json:"groupId" db:"group_id"`
	Role      string    `json:"role" db:"role"` // member, admin, owner
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"fullName" binding:"required"`
	Password string `json:"password" binding:"required,min=8"` // Only for local auth
	Role     string `json:"role"`
	Provider string `json:"provider"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty"`
	FullName *string `json:"fullName,omitempty"`
	Role     *string `json:"role,omitempty"`
	Active   *bool   `json:"active,omitempty"`
}

// CreateGroupRequest represents a request to create a new group
type CreateGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	DisplayName string  `json:"displayName" binding:"required"`
	Description string  `json:"description"`
	Type        string  `json:"type" binding:"required"`
	ParentID    *string `json:"parentId,omitempty"`
}

// UpdateGroupRequest represents a request to update a group
type UpdateGroupRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
}

// AddGroupMemberRequest represents a request to add a user to a group
type AddGroupMemberRequest struct {
	UserID string `json:"userId" binding:"required"`
	Role   string `json:"role"` // member, admin, owner
}

// SetQuotaRequest represents a request to set user quota
type SetQuotaRequest struct {
	MaxSessions *int    `json:"maxSessions,omitempty"`
	MaxCPU      *string `json:"maxCpu,omitempty"`
	MaxMemory   *string `json:"maxMemory,omitempty"`
	MaxStorage  *string `json:"maxStorage,omitempty"`
}
