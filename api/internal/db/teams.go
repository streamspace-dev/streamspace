// Package db provides PostgreSQL database access and management for StreamSpace.
//
// This file defines team-related data structures for team management and collaboration.
//
// Purpose:
// - Define team membership data structures
// - Define team role permission structures
// - Support team-based organization and collaboration
//
// Features:
// - Team membership tracking with roles
// - Role-based permission system
// - Team hierarchy support
//
// Data Structures:
//   - TeamMembership: Represents a user's membership in a team
//   - TeamPermission: Defines permissions for team roles
//   - TeamRoleInfo: Aggregates role information with permissions
//
// Team Roles:
//   - owner: Full control (manage team, members, sessions, quotas)
//   - admin: Administrative access (manage members, sessions)
//   - member: Standard access (create sessions, view team data)
//   - viewer: Read-only access (view sessions and quotas)
//
// Implementation Details:
// - This file contains only type definitions
// - Actual database operations are in groups.go (teams are a type of group)
// - Team roles and permissions stored in team_role_permissions table
// - Initialized with default permissions during database migration
//
// Example Usage:
//
//	membership := &TeamMembership{
//	    TeamID:          "team-abc",
//	    TeamName:        "frontend-team",
//	    TeamDisplayName: "Frontend Team",
//	    TeamType:        "team",
//	    Role:            "member",
//	    JoinedAt:        time.Now(),
//	}
package db

import "time"

// TeamMembership represents a user's membership in a team
type TeamMembership struct {
	TeamID          string    `json:"teamId"`
	TeamName        string    `json:"teamName"`
	TeamDisplayName string    `json:"teamDisplayName"`
	TeamType        string    `json:"teamType"`
	Role            string    `json:"role"`
	JoinedAt        time.Time `json:"joinedAt"`
}

// TeamPermission represents a permission for a team role
type TeamPermission struct {
	ID          int       `json:"id"`
	Role        string    `json:"role"`
	Permission  string    `json:"permission"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

// TeamRoleInfo represents information about a team role and its permissions
type TeamRoleInfo struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}
