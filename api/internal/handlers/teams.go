// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements team-based Role-Based Access Control (RBAC) operations.
//
// TEAM RBAC FEATURES:
// - Team permissions and role management
// - User permission queries within teams
// - Team session access control
// - Permission checking for authorization
//
// TEAM PERMISSIONS:
// - Role-based permissions (owner, admin, member, viewer, etc.)
// - Permission inheritance from team roles
// - User-specific permission queries
// - Permission validation for resource access
//
// TEAM SESSIONS:
// - List sessions belonging to a specific team
// - Permission-based access control (requires team.sessions.view)
// - Team member authorization
//
// API Endpoints:
// - GET /api/v1/teams/:teamId/permissions - Get all team role permissions
// - GET /api/v1/teams/:teamId/role-info - Get available team roles
// - GET /api/v1/teams/:teamId/my-permissions - Get current user's permissions
// - GET /api/v1/teams/:teamId/check-permission/:permission - Check specific permission
// - GET /api/v1/teams/:teamId/sessions - List team sessions (requires permission)
// - GET /api/v1/teams/my-teams - Get current user's team memberships
//
// Security:
// - Authentication required for all endpoints
// - Permission-based authorization for sensitive operations
// - Safe type assertions to prevent panics
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: teams, team_members, team_role_permissions, sessions tables
// - Middleware: TeamRBAC for permission checks
// - External Services: None
//
// Example Usage:
//
//	handler := NewTeamHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/middleware"
)

// TeamHandler handles team-related API requests with RBAC
type TeamHandler struct {
	database  *db.Database
	teamRBAC  *middleware.TeamRBAC
}

// NewTeamHandler creates a new team handler
func NewTeamHandler(database *db.Database) *TeamHandler {
	return &TeamHandler{
		database: database,
		teamRBAC: middleware.NewTeamRBAC(database.DB()),
	}
}

// RegisterRoutes registers team RBAC routes
func (h *TeamHandler) RegisterRoutes(router *gin.RouterGroup) {
	teamRoutes := router.Group("/teams")
	{
		// Team permissions and roles
		teamRoutes.GET("/:teamId/permissions", h.GetTeamPermissions)
		teamRoutes.GET("/:teamId/role-info", h.GetTeamRoleInfo)
		teamRoutes.GET("/:teamId/my-permissions", h.GetMyTeamPermissions)
		teamRoutes.GET("/:teamId/check-permission/:permission", h.CheckPermission)

		// Team sessions (requires team permission)
		teamRoutes.GET("/:teamId/sessions", h.ListTeamSessions)

		// User's teams
		teamRoutes.GET("/my-teams", h.GetMyTeams)
	}
}

// GetTeamPermissions returns all permissions defined for team roles
func (h *TeamHandler) GetTeamPermissions(c *gin.Context) {
	ctx := context.Background()

	// Get all team role permissions
	rows, err := h.database.DB().QueryContext(ctx, `
		SELECT role, permission, description
		FROM team_role_permissions
		ORDER BY role, permission
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get team permissions",
		})
		return
	}
	defer rows.Close()

	permissionsByRole := make(map[string][]map[string]string)
	for rows.Next() {
		var role, permission, description string
		if err := rows.Scan(&role, &permission, &description); err != nil {
			continue
		}

		if _, exists := permissionsByRole[role]; !exists {
			permissionsByRole[role] = []map[string]string{}
		}

		permissionsByRole[role] = append(permissionsByRole[role], map[string]string{
			"permission":  permission,
			"description": description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"permissions": permissionsByRole,
	})
}

// GetTeamRoleInfo returns information about available team roles
func (h *TeamHandler) GetTeamRoleInfo(c *gin.Context) {
	ctx := context.Background()

	// Get all unique roles
	rows, err := h.database.DB().QueryContext(ctx, `
		SELECT DISTINCT role FROM team_role_permissions ORDER BY role
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get team roles",
		})
		return
	}
	defer rows.Close()

	roles := []db.TeamRoleInfo{}
	for rows.Next() {
		var roleName string
		if err := rows.Scan(&roleName); err != nil {
			continue
		}

		// Get permissions for this role
		permRows, err := h.database.DB().QueryContext(ctx, `
			SELECT permission FROM team_role_permissions
			WHERE role = $1
			ORDER BY permission
		`, roleName)
		if err != nil {
			continue
		}

		permissions := []string{}
		for permRows.Next() {
			var perm string
			if err := permRows.Scan(&perm); err == nil {
				permissions = append(permissions, perm)
			}
		}
		permRows.Close()

		roles = append(roles, db.TeamRoleInfo{
			Role:        roleName,
			Permissions: permissions,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"roles": roles,
	})
}

// GetMyTeamPermissions returns the authenticated user's permissions in a team
func (h *TeamHandler) GetMyTeamPermissions(c *gin.Context) {
	teamID := c.Param("teamId")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Get user's role
	role, err := h.teamRBAC.GetUserTeamRole(context.Background(), userIDStr, teamID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not a member of this team",
		})
		return
	}

	// Get permissions
	permissions, err := h.teamRBAC.GetUserTeamPermissions(context.Background(), userIDStr, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get permissions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teamId":      teamID,
		"role":        role,
		"permissions": permissions,
	})
}

// CheckPermission checks if the authenticated user has a specific permission in a team
func (h *TeamHandler) CheckPermission(c *gin.Context) {
	teamID := c.Param("teamId")
	permission := c.Param("permission")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Check permission
	hasPermission, err := h.teamRBAC.CheckTeamPermission(context.Background(), userIDStr, teamID, permission)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check permission",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teamId":        teamID,
		"permission":    permission,
		"hasPermission": hasPermission,
	})
}

// ListTeamSessions returns all sessions belonging to a team
func (h *TeamHandler) ListTeamSessions(c *gin.Context) {
	teamID := c.Param("teamId")

	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Check if user has permission to view team sessions
	hasPermission, err := h.teamRBAC.CheckTeamPermission(context.Background(), userIDStr, teamID, "team.sessions.view")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check permission",
		})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have permission to view this team's sessions",
		})
		return
	}

	// Get team sessions
	rows, err := h.database.DB().QueryContext(context.Background(), `
		SELECT id, user_id, template_name, state, active_connections,
		       url, created_at, updated_at
		FROM sessions
		WHERE team_id = $1
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get team sessions",
		})
		return
	}
	defer rows.Close()

	sessions := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, templateName, state, url string
		var activeConns int
		var createdAt, updatedAt interface{}

		if err := rows.Scan(&id, &userID, &templateName, &state, &activeConns, &url, &createdAt, &updatedAt); err != nil {
			continue
		}

		sessions = append(sessions, map[string]interface{}{
			"id":                id,
			"userId":            userID,
			"templateName":      templateName,
			"state":             state,
			"activeConnections": activeConns,
			"url":               url,
			"createdAt":         createdAt,
			"updatedAt":         updatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"teamId":   teamID,
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetMyTeams returns all teams the authenticated user is a member of
func (h *TeamHandler) GetMyTeams(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Get user's teams
	teams, err := h.teamRBAC.ListUserTeams(context.Background(), userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user teams",
		})
		return
	}

	// For each team, get the user's permissions
	enrichedTeams := []map[string]interface{}{}
	for _, team := range teams {
		permissions, err := h.teamRBAC.GetUserTeamPermissions(context.Background(), userIDStr, team.TeamID)
		if err != nil {
			permissions = []string{}
		}

		enrichedTeams = append(enrichedTeams, map[string]interface{}{
			"teamId":          team.TeamID,
			"teamName":        team.TeamName,
			"teamDisplayName": team.TeamDisplayName,
			"teamType":        team.TeamType,
			"role":            team.Role,
			"permissions":     permissions,
			"joinedAt":        team.JoinedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"teams": enrichedTeams,
		"total": len(enrichedTeams),
	})
}
