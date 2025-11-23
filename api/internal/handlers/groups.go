// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements group management and group-level resource operations.
//
// GROUP MANAGEMENT:
// - Group CRUD operations (list, create, read, update, delete)
// - Group filtering by type or parent group (supports hierarchical groups)
// - Group member management (add, remove, update roles)
// - Member enrichment with user information
//
// GROUP MEMBERSHIP:
// - Add users to groups with specific roles
// - Remove users from groups
// - Update member roles within groups
// - List all members with enriched user details
// - User existence validation before adding to groups
//
// GROUP QUOTAS:
// - Shared resource quotas for group members
// - Group-level limits for sessions, CPU, memory, storage
// - Quota retrieval and modification
//
// API Endpoints:
// - GET    /api/v1/groups - List all groups with optional filters
// - POST   /api/v1/groups - Create new group
// - GET    /api/v1/groups/:id - Get group by ID
// - PATCH  /api/v1/groups/:id - Update group information
// - DELETE /api/v1/groups/:id - Delete group
// - GET    /api/v1/groups/:id/members - List group members
// - POST   /api/v1/groups/:id/members - Add user to group
// - DELETE /api/v1/groups/:id/members/:userId - Remove user from group
// - PATCH  /api/v1/groups/:id/members/:userId - Update member role
// - GET    /api/v1/groups/:id/quota - Get group quota
// - PUT    /api/v1/groups/:id/quota - Set group quota
//
// Security:
// - Password hashes removed from user objects in member lists
// - User existence validated before membership operations
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: groups, group_members, group_quotas, users tables
// - External Services: None
//
// Example Usage:
//
//	handler := NewGroupHandler(groupDB, userDB)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// GroupHandler handles group-related API requests
type GroupHandler struct {
	groupDB *db.GroupDB
	userDB  *db.UserDB
}

// NewGroupHandler creates a new group handler
func NewGroupHandler(groupDB *db.GroupDB, userDB *db.UserDB) *GroupHandler {
	return &GroupHandler{
		groupDB: groupDB,
		userDB:  userDB,
	}
}

// RegisterRoutes registers group management routes
func (h *GroupHandler) RegisterRoutes(router *gin.RouterGroup) {
	groupRoutes := router.Group("/groups")
	{
		// Group CRUD
		groupRoutes.GET("", h.ListGroups)
		groupRoutes.POST("", h.CreateGroup)
		groupRoutes.GET("/:id", h.GetGroup)
		groupRoutes.PATCH("/:id", h.UpdateGroup)
		groupRoutes.DELETE("/:id", h.DeleteGroup)

		// Group members
		groupRoutes.GET("/:id/members", h.GetGroupMembers)
		groupRoutes.POST("/:id/members", h.AddGroupMember)
		groupRoutes.DELETE("/:id/members/:userId", h.RemoveGroupMember)
		groupRoutes.PATCH("/:id/members/:userId", h.UpdateMemberRole)

		// Group quotas
		groupRoutes.GET("/:id/quota", h.GetGroupQuota)
		groupRoutes.PUT("/:id/quota", h.SetGroupQuota)
	}
}

// ListGroups godoc
// @Summary List all groups
// @Description Get a list of all groups with optional filtering
// @Tags groups
// @Accept json
// @Produce json
// @Param type query string false "Filter by group type"
// @Param parentId query string false "Filter by parent group"
// @Success 200 {array} models.Group
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups [get]
func (h *GroupHandler) ListGroups(c *gin.Context) {
	groupType := c.Query("type")
	parentID := c.Query("parentId")

	var parentPtr *string
	if parentID != "" {
		parentPtr = &parentID
	}

	groups, err := h.groupDB.ListGroups(c.Request.Context(), groupType, parentPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list groups",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// CreateGroup godoc
// @Summary Create a new group
// @Description Create a new group/team
// @Tags groups
// @Accept json
// @Produce json
// @Param group body models.CreateGroupRequest true "Group creation request"
// @Success 201 {object} models.Group
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups [post]
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req models.CreateGroupRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	group, err := h.groupDB.CreateGroup(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create group",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// GetGroup godoc
// @Summary Get group by ID
// @Description Get detailed information about a specific group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} models.Group
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id} [get]
func (h *GroupHandler) GetGroup(c *gin.Context) {
	groupID := c.Param("id")

	group, err := h.groupDB.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Group not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, group)
}

// UpdateGroup godoc
// @Summary Update group
// @Description Update group information
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param group body models.UpdateGroupRequest true "Group update request"
// @Success 200 {object} models.Group
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id} [patch]
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	groupID := c.Param("id")

	var req models.UpdateGroupRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	if err := h.groupDB.UpdateGroup(c.Request.Context(), groupID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update group",
			Message: err.Error(),
		})
		return
	}

	// Fetch updated group
	group, err := h.groupDB.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated group",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup godoc
// @Summary Delete group
// @Description Delete a group/team
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id} [delete]
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	groupID := c.Param("id")

	if err := h.groupDB.DeleteGroup(c.Request.Context(), groupID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete group",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Group deleted successfully",
	})
}

// GetGroupMembers godoc
// @Summary Get group members
// @Description Get all members of a group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} gin.H
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/members [get]
func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	groupID := c.Param("id")

	members, err := h.groupDB.GetGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get group members",
			Message: err.Error(),
		})
		return
	}

	// Enrich with user information
	enrichedMembers := []interface{}{}
	for _, member := range members {
		user, err := h.userDB.GetUser(c.Request.Context(), member.UserID)
		if err == nil {
			user.PasswordHash = "" // Remove sensitive data
			enrichedMembers = append(enrichedMembers, gin.H{
				"user": user,
				"role": member.Role,
				"joinedAt": member.CreatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"members": enrichedMembers,
		"total":   len(enrichedMembers),
	})
}

// AddGroupMember godoc
// @Summary Add group member
// @Description Add a user to a group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param member body models.AddGroupMemberRequest true "Member add request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/members [post]
func (h *GroupHandler) AddGroupMember(c *gin.Context) {
	groupID := c.Param("id")

	var req models.AddGroupMemberRequest

	// Bind and validate request
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Verify user exists
	_, err := h.userDB.GetUser(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "User not found",
			Message: err.Error(),
		})
		return
	}

	if err := h.groupDB.AddGroupMember(c.Request.Context(), groupID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add group member",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "User added to group successfully",
	})
}

// RemoveGroupMember godoc
// @Summary Remove group member
// @Description Remove a user from a group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param userId path string true "User ID"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/members/{userId} [delete]
func (h *GroupHandler) RemoveGroupMember(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userId")

	if err := h.groupDB.RemoveGroupMember(c.Request.Context(), groupID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to remove group member",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "User removed from group successfully",
	})
}

// UpdateMemberRole godoc
// @Summary Update member role
// @Description Update a user's role in a group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param userId path string true "User ID"
// @Param role body gin.H true "Role update"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/members/{userId} [patch]
func (h *GroupHandler) UpdateMemberRole(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userId")

	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.groupDB.UpdateGroupMemberRole(c.Request.Context(), groupID, userID, req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update member role",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Member role updated successfully",
	})
}

// GetGroupQuota godoc
// @Summary Get group quota
// @Description Get resource quota for a group
// @Tags groups, quotas
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} models.GroupQuota
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/quota [get]
func (h *GroupHandler) GetGroupQuota(c *gin.Context) {
	groupID := c.Param("id")

	quota, err := h.groupDB.GetGroupQuota(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Quota not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// SetGroupQuota godoc
// @Summary Set group quota
// @Description Set or update resource quota for a group
// @Tags groups, quotas
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param quota body models.SetQuotaRequest true "Quota settings"
// @Success 200 {object} models.GroupQuota
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/groups/{id}/quota [put]
func (h *GroupHandler) SetGroupQuota(c *gin.Context) {
	groupID := c.Param("id")

	var req models.SetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.groupDB.SetGroupQuota(c.Request.Context(), groupID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to set quota",
			Message: err.Error(),
		})
		return
	}

	// Fetch updated quota
	quota, err := h.groupDB.GetGroupQuota(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated quota",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}
