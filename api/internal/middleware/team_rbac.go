// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements team-based role-based access control (RBAC).
//
// Purpose:
// The team RBAC middleware provides fine-grained access control for multi-tenant
// StreamSpace deployments where users belong to teams/groups and have different
// permission levels within each team. This enables enterprise features like
// shared sessions, team resource quotas, and delegated administration.
//
// Implementation Details:
// - Database-backed permissions: Roles and permissions stored in PostgreSQL
// - Per-team roles: Users can have different roles in different teams
// - Permission-based checks: Middleware validates specific permissions (not just roles)
// - Session-level access: Can check if user can access sessions owned by team
// - Hierarchical model: Teams → Users → Roles → Permissions
//
// Permission Model:
//
// 1. Teams (groups table):
//    - Organizations or departments
//    - Example: "Engineering", "Sales", "Data Science"
//
// 2. Team Memberships (group_memberships table):
//    - Links users to teams with specific roles
//    - Example: alice@example.com is "admin" in Engineering team
//
// 3. Roles (team_role_permissions table):
//    - Named permission sets
//    - Example: "admin", "member", "viewer"
//
// 4. Permissions:
//    - Fine-grained capabilities
//    - Example: "sessions.create", "sessions.delete", "team.manage"
//
// Common Permissions:
// - sessions.view: View team sessions
// - sessions.create: Create sessions for team
// - sessions.delete: Delete team sessions
// - sessions.share: Share sessions with team members
// - team.manage: Add/remove team members
// - team.billing: View/manage team billing
//
// Security Notes:
// This middleware enforces the principle of least privilege:
// - Users only have access to their own sessions OR team sessions where they have permission
// - Team admins can manage team resources but not other teams
// - Platform admins have global permissions across all teams
//
// Thread Safety:
// Safe for concurrent use. Database queries are isolated per request.
//
// Usage:
//   // Create team RBAC middleware
//   teamRBAC := middleware.NewTeamRBAC(database)
//
//   // Require specific team permission
//   router.POST("/api/teams/:teamId/sessions",
//       teamRBAC.RequireTeamPermission("sessions.create"),
//       handlers.CreateTeamSession,
//   )
//
//   // Require session access (owner OR team member with permission)
//   router.GET("/api/sessions/:id",
//       teamRBAC.RequireSessionAccess("sessions.view"),
//       handlers.GetSession,
//   )
//
//   // Check permissions manually in handler
//   hasPermission, err := teamRBAC.CheckTeamPermission(ctx, userID, teamID, "sessions.delete")
//   if !hasPermission {
//       return c.JSON(403, gin.H{"error": "Insufficient permissions"})
//   }
package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// TeamRBAC provides team-based role-based access control
type TeamRBAC struct {
	database *sql.DB
}

// NewTeamRBAC creates a new team RBAC middleware
func NewTeamRBAC(database *sql.DB) *TeamRBAC {
	return &TeamRBAC{
		database: database,
	}
}

// RequireTeamPermission creates middleware that checks if user has specific team permission
func (t *TeamRBAC) RequireTeamPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get team ID from URL param or query
		teamID := c.Param("teamId")
		if teamID == "" {
			teamID = c.Query("team_id")
		}

		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID",
			})
			c.Abort()
			return
		}

		// Check if team ID is required for this permission
		if teamID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Team ID required",
			})
			c.Abort()
			return
		}

		// Check if user has the required permission in this team
		hasPermission, err := t.CheckTeamPermission(context.Background(), userIDStr, teamID, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to check team permission: %v", err),
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Insufficient permissions",
				"permission": permission,
				"teamId":     teamID,
			})
			c.Abort()
			return
		}

		// Permission granted, continue
		c.Next()
	}
}

// CheckTeamPermission checks if a user has a specific permission in a team
func (t *TeamRBAC) CheckTeamPermission(ctx context.Context, userID, teamID, permission string) (bool, error) {
	// Get user's role in the team
	var role string
	err := t.database.QueryRowContext(ctx, `
		SELECT role FROM group_memberships
		WHERE user_id = $1 AND group_id = $2
	`, userID, teamID).Scan(&role)

	if err == sql.ErrNoRows {
		// User is not a member of this team
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Check if this role has the required permission
	var exists bool
	err = t.database.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM team_role_permissions
			WHERE role = $1 AND permission = $2
		)
	`, role, permission).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetUserTeamRole returns the user's role in a specific team
func (t *TeamRBAC) GetUserTeamRole(ctx context.Context, userID, teamID string) (string, error) {
	var role string
	err := t.database.QueryRowContext(ctx, `
		SELECT role FROM group_memberships
		WHERE user_id = $1 AND group_id = $2
	`, userID, teamID).Scan(&role)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user is not a member of team %s", teamID)
	}
	if err != nil {
		return "", err
	}

	return role, nil
}

// GetUserTeamPermissions returns all permissions for a user in a specific team
func (t *TeamRBAC) GetUserTeamPermissions(ctx context.Context, userID, teamID string) ([]string, error) {
	// Get user's role
	role, err := t.GetUserTeamRole(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Get all permissions for this role
	rows, err := t.database.QueryContext(ctx, `
		SELECT permission FROM team_role_permissions
		WHERE role = $1
		ORDER BY permission
	`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := []string{}
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			continue
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// ListUserTeams returns all teams a user is a member of
func (t *TeamRBAC) ListUserTeams(ctx context.Context, userID string) ([]db.TeamMembership, error) {
	rows, err := t.database.QueryContext(ctx, `
		SELECT
			gm.group_id,
			g.name,
			g.display_name,
			g.type,
			gm.role,
			gm.created_at
		FROM group_memberships gm
		JOIN groups g ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY gm.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := []db.TeamMembership{}
	for rows.Next() {
		var tm db.TeamMembership
		if err := rows.Scan(
			&tm.TeamID,
			&tm.TeamName,
			&tm.TeamDisplayName,
			&tm.TeamType,
			&tm.Role,
			&tm.JoinedAt,
		); err != nil {
			continue
		}
		teams = append(teams, tm)
	}

	return teams, nil
}

// CanAccessSession checks if a user can access a session (either owner or team member with permission)
func (t *TeamRBAC) CanAccessSession(ctx context.Context, userID, sessionID string, permission string) (bool, error) {
	// Get session details
	var sessionUserID string
	var teamID sql.NullString
	err := t.database.QueryRowContext(ctx, `
		SELECT user_id, team_id FROM sessions WHERE id = $1
	`, sessionID).Scan(&sessionUserID, &teamID)

	if err == sql.ErrNoRows {
		return false, fmt.Errorf("session not found")
	}
	if err != nil {
		return false, err
	}

	// If user is the session owner, grant access
	if sessionUserID == userID {
		return true, nil
	}

	// If session belongs to a team, check team permissions
	if teamID.Valid && teamID.String != "" {
		return t.CheckTeamPermission(ctx, userID, teamID.String, permission)
	}

	// Session doesn't belong to a team and user is not owner
	return false, nil
}

// RequireSessionAccess middleware checks if user can access a specific session
func (t *TeamRBAC) RequireSessionAccess(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		if sessionID == "" {
			sessionID = c.Param("sessionId")
		}

		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Session ID required",
			})
			c.Abort()
			return
		}

		// Get user ID from context
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID",
			})
			c.Abort()
			return
		}

		// Check access
		canAccess, err := t.CanAccessSession(context.Background(), userIDStr, sessionID, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to check session access: %v", err),
			})
			c.Abort()
			return
		}

		if !canAccess {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Insufficient permissions to access this session",
				"sessionId":  sessionID,
				"permission": permission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
