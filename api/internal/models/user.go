// Package models defines the core data structures for the StreamSpace API.
//
// This package contains:
//   - User and authentication models
//   - Group and team membership models
//   - Resource quota models
//   - Request/response types for API handlers
//
// These models are used for:
//   - Database persistence (via sqlx struct tags)
//   - JSON serialization (via json struct tags)
//   - API request validation (via binding tags)
//
// Database tags use the snake_case convention:
//
//	type User struct {
//	    FullName string `json:"fullName" db:"full_name"`
//	}
package models

import (
	"time"
)

// User represents a StreamSpace user with authentication and quota information.
//
// Users can be created via:
//   - Local authentication (username + password)
//   - SAML 2.0 SSO (external identity providers)
//   - OIDC OAuth2 (Google, GitHub, etc.)
//
// Each user has:
//   - A unique ID (UUID)
//   - An organization membership (org_id for multi-tenancy)
//   - Authentication credentials (provider-specific)
//   - Resource quotas (sessions, CPU, memory, storage)
//   - Group memberships (for team-based access control)
//
// SECURITY: All API handlers MUST filter queries by org_id from the
// authenticated user's JWT claims to prevent cross-tenant data access.
//
// Example:
//
//	{
//	  "id": "550e8400-e29b-41d4-a716-446655440000",
//	  "orgId": "org-acme",
//	  "username": "alice",
//	  "email": "alice@example.com",
//	  "fullName": "Alice Smith",
//	  "role": "user",
//	  "orgRole": "user",
//	  "provider": "local",
//	  "active": true,
//	  "quota": {
//	    "maxSessions": 5,
//	    "maxCpu": "4000m",
//	    "maxMemory": "8Gi",
//	    "usedSessions": 2
//	  }
//	}
type User struct {
	// ID is a unique identifier for this user (UUID v4).
	// Generated automatically when the user is created.
	ID string `json:"id" db:"id"`

	// OrgID is the organization this user belongs to.
	// SECURITY: This field is critical for multi-tenancy isolation.
	// All queries MUST filter by org_id to prevent cross-tenant access.
	OrgID string `json:"orgId" db:"org_id"`

	// Username is a unique identifier used for authentication and display.
	// Requirements:
	//   - Must be unique across all users
	//   - 3-32 characters
	//   - Alphanumeric, hyphens, underscores only
	//
	// Example: "alice", "bob-smith", "user_123"
	Username string `json:"username" db:"username"`

	// Email is the user's email address.
	// Requirements:
	//   - Must be a valid email format
	//   - Must be unique across all users
	//   - Used for notifications and password resets
	//
	// Example: "alice@example.com"
	Email string `json:"email" db:"email"`

	// FullName is the user's display name (can include spaces).
	// Example: "Alice Smith", "Bob Jones"
	FullName string `json:"fullName" db:"full_name"`

	// Role defines the user's system-wide permission level.
	//
	// Valid roles:
	//   - "user": Standard user (can manage own sessions)
	//   - "operator": Platform operator (can view all sessions, manage quotas)
	//   - "admin": Administrator (full platform access)
	//
	// Default: "user"
	Role string `json:"role" db:"role"`

	// OrgRole defines the user's role within their organization.
	//
	// Valid org roles:
	//   - "org_admin": Manage users/roles, templates, org settings
	//   - "maintainer": Manage templates, sessions (no user admin)
	//   - "user": Manage own sessions, list org templates
	//   - "viewer": Read-only access to lists/metrics
	//
	// Default: "user"
	OrgRole string `json:"orgRole,omitempty" db:"org_role"`

	// Provider indicates how this user authenticates.
	//
	// Valid providers:
	//   - "local": Username + password authentication
	//   - "saml": SAML 2.0 SSO (Authentik, Keycloak, Okta, etc.)
	//   - "oidc": OIDC OAuth2 (Google, GitHub, Azure AD, etc.)
	//
	// Default: "local"
	Provider string `json:"provider" db:"provider"`

	// Active indicates whether the user account is enabled.
	//
	// When false:
	//   - User cannot log in
	//   - Existing sessions are terminated
	//   - API keys are deactivated
	//
	// Used for account suspension or deactivation.
	Active bool `json:"active" db:"active"`

	// CreatedAt is the timestamp when this user was created.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is the timestamp of the last user update.
	// Updated on any change to user fields (except lastLogin).
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// LastLogin is the timestamp of the user's most recent authentication.
	// Nil if the user has never logged in.
	LastLogin *time.Time `json:"lastLogin,omitempty" db:"last_login"`

	// PasswordHash stores the bcrypt hash of the user's password.
	// Only used for local authentication (provider="local").
	//
	// Security:
	//   - Never exposed in JSON responses (json:"-")
	//   - Stored using bcrypt with cost factor 10
	//   - Minimum password length enforced at 8 characters
	PasswordHash string `json:"-" db:"password_hash"`

	// Quota contains the user's resource limits and current usage.
	// Populated from the user_quotas table via a JOIN query.
	// Nil if no quota has been explicitly set (platform defaults apply).
	Quota *UserQuota `json:"quota,omitempty"`

	// Groups is a list of group IDs this user belongs to.
	// Populated from the group_memberships table.
	// Used for team-based resource quotas and access control.
	Groups []string `json:"groups,omitempty"`
}

// UserQuota represents resource quotas and current usage for a user.
//
// Quotas enforce limits on:
//   - Maximum concurrent sessions
//   - Total CPU allocation across all sessions
//   - Total memory allocation across all sessions
//   - Persistent storage size
//
// Quotas can be set:
//   - Per-user (user_quotas table)
//   - Per-group (group_quotas table)
//   - Platform-wide defaults (in code)
//
// The most restrictive quota applies when a user belongs to multiple groups.
//
// Example:
//
//	{
//	  "userId": "550e8400-e29b-41d4-a716-446655440000",
//	  "maxSessions": 5,
//	  "maxCpu": "4000m",       // 4 CPU cores total
//	  "maxMemory": "8Gi",      // 8 GiB total
//	  "maxStorage": "50Gi",    // 50 GiB persistent storage
//	  "usedSessions": 2,
//	  "usedCpu": "1500m",
//	  "usedMemory": "3Gi"
//	}
type UserQuota struct {
	// UserID links this quota to a specific user.
	UserID string `json:"userId" db:"user_id"`

	// Username is included for convenience in API responses.
	Username string `json:"username" db:"username"`

	// MaxSessions is the maximum number of concurrent sessions allowed.
	// Default: 5 (free tier), unlimited for admins
	MaxSessions int `json:"maxSessions" db:"max_sessions"`

	// MaxCPU is the total CPU allocation across all sessions.
	// Format: Kubernetes quantity (e.g., "4000m" = 4 cores)
	// Default: "4000m"
	MaxCPU string `json:"maxCpu" db:"max_cpu"`

	// MaxMemory is the total memory allocation across all sessions.
	// Format: Kubernetes quantity (e.g., "8Gi" = 8 gibibytes)
	// Default: "8Gi"
	MaxMemory string `json:"maxMemory" db:"max_memory"`

	// MaxStorage is the persistent storage size for the user's home directory.
	// Format: Kubernetes quantity (e.g., "50Gi")
	// Default: "50Gi"
	MaxStorage string `json:"maxStorage" db:"max_storage"`

	// UsedSessions is the current number of active (non-hibernated) sessions.
	// Computed from the sessions table.
	UsedSessions int `json:"usedSessions" db:"used_sessions"`

	// UsedCPU is the total CPU currently allocated to active sessions.
	// Computed from Kubernetes pod resource requests.
	UsedCPU string `json:"usedCpu" db:"used_cpu"`

	// UsedMemory is the total memory currently allocated to active sessions.
	// Computed from Kubernetes pod resource requests.
	UsedMemory string `json:"usedMemory" db:"used_memory"`

	// UsedStorage is the actual storage consumed in the user's PVC.
	// Computed from Kubernetes PVC usage metrics.
	UsedStorage string `json:"usedStorage" db:"used_storage"`

	// CreatedAt is when this quota was first set.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when this quota was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// Group represents a user group/team for organizing users and applying shared quotas.
//
// Groups enable:
//   - Team-based organization
//   - Shared resource quotas
//   - Hierarchical structures (departments → teams → projects)
//   - Bulk permission management
//
// Example use cases:
//   - Engineering department with multiple project teams
//   - Sales team with regional sub-teams
//   - Research groups with shared compute quotas
//
// Example:
//
//	{
//	  "id": "grp-engineering",
//	  "name": "engineering",
//	  "displayName": "Engineering Department",
//	  "description": "Software engineering team",
//	  "type": "department",
//	  "memberCount": 25,
//	  "quota": {
//	    "maxSessions": 100,
//	    "maxCpu": "100000m",
//	    "maxMemory": "200Gi"
//	  }
//	}
type Group struct {
	// ID is a unique identifier for this group.
	// Format: "grp-{name}" or UUID
	ID string `json:"id" db:"id"`

	// Name is a unique machine-readable identifier.
	// Requirements: lowercase, alphanumeric, hyphens only
	// Example: "engineering", "sales-west", "research-ai"
	Name string `json:"name" db:"name"`

	// DisplayName is the human-readable group name.
	// Example: "Engineering Department", "West Coast Sales"
	DisplayName string `json:"displayName" db:"display_name"`

	// Description explains the purpose or scope of this group.
	Description string `json:"description" db:"description"`

	// Type categorizes the group's organizational level.
	//
	// Valid types:
	//   - "team": Small working group
	//   - "department": Organizational department
	//   - "project": Project-based team
	//
	// Default: "team"
	Type string `json:"type" db:"type"`

	// ParentID creates a hierarchical structure.
	// Example: "sales-west" could be a child of "sales"
	// Nil for top-level groups.
	ParentID *string `json:"parentId,omitempty" db:"parent_id"`

	// CreatedAt is when this group was created.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when this group was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// MemberCount is the number of users in this group.
	// Computed from the group_memberships table.
	MemberCount int `json:"memberCount" db:"member_count"`

	// Quota contains resource limits shared across all group members.
	// When set, individual users' quotas are aggregated against this limit.
	Quota *GroupQuota `json:"quota,omitempty"`
}

// GroupQuota represents shared resource quotas for a group.
//
// Group quotas work differently from user quotas:
//   - Limits apply to the sum of all members' usage
//   - Prevents one group from consuming all platform resources
//   - Can be combined with individual user quotas (most restrictive wins)
//
// Example scenario:
//   - Engineering group has quota: 100 sessions, 200Gi RAM
//   - Individual users each have quota: 10 sessions, 16Gi RAM
//   - When group reaches 100 total sessions, no member can create more
//   - Even if individual user only has 5 sessions
//
// Example:
//
//	{
//	  "groupId": "grp-engineering",
//	  "maxSessions": 100,
//	  "maxCpu": "100000m",     // 100 CPU cores shared
//	  "maxMemory": "200Gi",    // 200 GiB shared
//	  "usedSessions": 45,
//	  "usedCpu": "42000m",
//	  "usedMemory": "87Gi"
//	}
type GroupQuota struct {
	// GroupID links this quota to a specific group.
	GroupID string `json:"groupId" db:"group_id"`

	// MaxSessions is the total sessions allowed across all group members.
	MaxSessions int `json:"maxSessions" db:"max_sessions"`

	// MaxCPU is the total CPU allocation for the entire group.
	MaxCPU string `json:"maxCpu" db:"max_cpu"`

	// MaxMemory is the total memory allocation for the entire group.
	MaxMemory string `json:"maxMemory" db:"max_memory"`

	// MaxStorage is the total storage allocation for the entire group.
	MaxStorage string `json:"maxStorage" db:"max_storage"`

	// UsedSessions is the sum of all members' active sessions.
	UsedSessions int `json:"usedSessions" db:"used_sessions"`

	// UsedCPU is the sum of all members' CPU allocations.
	UsedCPU string `json:"usedCpu" db:"used_cpu"`

	// UsedMemory is the sum of all members' memory allocations.
	UsedMemory string `json:"usedMemory" db:"used_memory"`

	// UsedStorage is the sum of all members' storage usage.
	UsedStorage string `json:"usedStorage" db:"used_storage"`

	// CreatedAt is when this quota was first set.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when this quota was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// GroupMembership represents a user's membership in a group.
//
// Each membership defines:
//   - Which user belongs to which group
//   - The user's role within that group
//   - When the membership was created
//
// Example:
//
//	{
//	  "id": "mem-123",
//	  "userId": "user-alice",
//	  "groupId": "grp-engineering",
//	  "role": "member",
//	  "createdAt": "2025-01-01T00:00:00Z"
//	}
type GroupMembership struct {
	// ID is a unique identifier for this membership.
	ID string `json:"id" db:"id"`

	// UserID is the ID of the user who belongs to the group.
	UserID string `json:"userId" db:"user_id"`

	// GroupID is the ID of the group the user belongs to.
	GroupID string `json:"groupId" db:"group_id"`

	// Role defines the user's permissions within the group.
	//
	// Valid roles:
	//   - "member": Standard group member (no special permissions)
	//   - "admin": Can add/remove members, modify group settings
	//   - "owner": Full control including delete group
	//
	// Default: "member"
	Role string `json:"role" db:"role"`

	// CreatedAt is when this membership was created (when user joined the group).
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// CreateUserRequest represents a request to create a new user.
//
// Validation rules:
//   - Username: required, 3-32 chars, alphanumeric + hyphens/underscores
//   - Email: required, valid email format
//   - FullName: required
//   - Password: required for local auth, min 8 chars
//   - Role: optional, defaults to "user"
//   - Provider: optional, defaults to "local"
//
// Example:
//
//	{
//	  "username": "alice",
//	  "email": "alice@example.com",
//	  "fullName": "Alice Smith",
//	  "password": "securepassword123",
//	  "role": "user",
//	  "provider": "local"
//	}
type CreateUserRequest struct {
	Username string `json:"username" binding:"required" validate:"required,username"`
	Email    string `json:"email" binding:"required,email" validate:"required,email"`
	FullName string `json:"fullName" binding:"required" validate:"required,min=1,max=200"`
	Password string `json:"password" validate:"omitempty,password"` // Required for local auth, validated in handler
	Role     string `json:"role" validate:"omitempty,oneof=user admin operator"` // user, admin, operator
	Provider string `json:"provider" validate:"omitempty,oneof=local saml oidc"` // local, saml, oidc
}

// UpdateUserRequest represents a request to update an existing user.
//
// All fields are optional (pointer types) - only provided fields are updated.
//
// Example (update email and role):
//
//	{
//	  "email": "newemail@example.com",
//	  "role": "admin"
//	}
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty" validate:"omitempty,email"`
	FullName *string `json:"fullName,omitempty" validate:"omitempty,min=1,max=200"`
	Role     *string `json:"role,omitempty" validate:"omitempty,oneof=user admin operator"`
	Active   *bool   `json:"active,omitempty"`
}

// CreateGroupRequest represents a request to create a new group.
//
// Validation rules:
//   - Name: required, lowercase, alphanumeric + hyphens
//   - DisplayName: required
//   - Type: required (team, department, project)
//   - ParentID: optional (for hierarchical groups)
//
// Example:
//
//	{
//	  "name": "engineering",
//	  "displayName": "Engineering Department",
//	  "description": "Software engineering team",
//	  "type": "department",
//	  "parentID": null
//	}
type CreateGroupRequest struct {
	Name        string  `json:"name" binding:"required" validate:"required,min=3,max=50,lowercase,alphanum|contains=-"`
	DisplayName string  `json:"displayName" binding:"required" validate:"required,min=3,max=100"`
	Description string  `json:"description" validate:"omitempty,max=500"`
	Type        string  `json:"type" binding:"required" validate:"required,oneof=team department project"`
	ParentID    *string `json:"parentId,omitempty" validate:"omitempty,uuid"`
}

// UpdateGroupRequest represents a request to update an existing group.
//
// All fields are optional (pointer types) - only provided fields are updated.
type UpdateGroupRequest struct {
	DisplayName *string `json:"displayName,omitempty" validate:"omitempty,min=3,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
	Type        *string `json:"type,omitempty" validate:"omitempty,oneof=team department project"`
}

// AddGroupMemberRequest represents a request to add a user to a group.
//
// Example:
//
//	{
//	  "userId": "user-alice",
//	  "role": "member"
//	}
type AddGroupMemberRequest struct {
	UserID string `json:"userId" binding:"required" validate:"required,min=1,max=100"`
	Role   string `json:"role" validate:"omitempty,oneof=member admin owner"`
}

// SetQuotaRequest represents a request to set or update user/group quotas.
//
// All fields are optional (pointer types) - only provided fields are updated.
//
// Example (set max sessions and memory):
//
//	{
//	  "maxSessions": 10,
//	  "maxMemory": "16Gi"
//	}
type SetQuotaRequest struct {
	Username    string  `json:"username,omitempty"` // For admin endpoints only
	MaxSessions *int    `json:"maxSessions,omitempty"`
	MaxCPU      *string `json:"maxCpu,omitempty"`
	MaxMemory   *string `json:"maxMemory,omitempty"`
	MaxStorage  *string `json:"maxStorage,omitempty"`
}

// LoginRequest represents a user login request.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
