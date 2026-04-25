// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements user management and user-level resource quota operations.
//
// USER MANAGEMENT:
// - User CRUD operations (list, create, read, update, delete)
// - User profile management (/me endpoints for current user)
// - User filtering by role, provider, or active status
// - Password hash sanitization (never exposed in responses)
//
// QUOTA MANAGEMENT:
// - Per-user resource quotas (sessions, CPU, memory, storage)
// - Quota retrieval for current user and specific users
// - Admin quota management (list all, set, delete/reset)
// - Username-based quota operations for admin convenience
//
// USER ASSOCIATIONS:
// - User sessions (redirects to /sessions?user=id)
// - User group memberships
//
// API Endpoints:
// - GET    /api/v1/users - List all users with optional filters
// - POST   /api/v1/users - Create new user account
// - GET    /api/v1/users/me - Get current authenticated user
// - GET    /api/v1/users/me/quota - Get current user's quota
// - GET    /api/v1/users/:id - Get user by ID
// - PATCH  /api/v1/users/:id - Update user information
// - DELETE /api/v1/users/:id - Delete user account
// - GET    /api/v1/users/:id/sessions - Get user's sessions
// - GET    /api/v1/users/:id/quota - Get user's resource quota
// - PUT    /api/v1/users/:id/quota - Set user's resource quota
// - GET    /api/v1/users/:id/groups - Get user's group memberships
// - GET    /api/v1/admin/quotas - List all user quotas (admin)
// - GET    /api/v1/admin/quotas/:username - Get quota by username (admin)
// - PUT    /api/v1/admin/quotas - Set quota by username (admin)
// - DELETE /api/v1/admin/quotas/:username - Reset quota to defaults (admin)
//
// Security:
// - Password hashes are sanitized before returning user objects
// - Safe type assertions prevent panics
// - Authentication required via middleware for protected endpoints
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: users, user_quotas, user_groups tables
// - External Services: None
//
// Example Usage:
//
//	handler := NewUserHandler(userDB, groupDB)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// UserHandler handles user-related API requests
type UserHandler struct {
	userDB  *db.UserDB
	groupDB *db.GroupDB
}

// NewUserHandler creates a new user handler
func NewUserHandler(userDB *db.UserDB, groupDB *db.GroupDB) *UserHandler {
	return &UserHandler{
		userDB:  userDB,
		groupDB: groupDB,
	}
}

// RegisterRoutes registers user management routes
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	userRoutes := router.Group("/users")
	{
		// User CRUD
		userRoutes.GET("", h.ListUsers)
		userRoutes.POST("", h.CreateUser)
		userRoutes.GET("/me", h.GetCurrentUser)
		userRoutes.GET("/me/quota", h.GetCurrentUserQuota)
		userRoutes.GET("/:id", h.GetUser)
		userRoutes.PATCH("/:id", h.UpdateUser)
		userRoutes.DELETE("/:id", h.DeleteUser)

		// User sessions
		userRoutes.GET("/:id/sessions", h.GetUserSessions)

		// User quotas
		userRoutes.GET("/:id/quota", h.GetUserQuota)
		userRoutes.PUT("/:id/quota", h.SetUserQuota)

		// User groups
		userRoutes.GET("/:id/groups", h.GetUserGroups)
	}

	// Admin quota routes
	adminQuotaRoutes := router.Group("/admin/quotas")
	{
		adminQuotaRoutes.GET("", h.ListAllUserQuotas)
		adminQuotaRoutes.GET("/:username", h.GetAdminUserQuota)
		adminQuotaRoutes.PUT("", h.SetAdminUserQuota)
		adminQuotaRoutes.DELETE("/:username", h.DeleteAdminUserQuota)
	}
}

// ListUsers godoc
// @Summary List all users
// @Description Get a list of all users with optional filtering
// @Tags users
// @Accept json
// @Produce json
// @Param role query string false "Filter by role"
// @Param provider query string false "Filter by auth provider"
// @Param active query boolean false "Filter active users only"
// @Success 200 {array} models.User
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	role := c.Query("role")
	provider := c.Query("provider")
	activeOnly := c.Query("active") == "true"

	users, err := h.userDB.ListUsers(c.Request.Context(), role, provider, activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list users",
			Message: err.Error(),
		})
		return
	}

	// Remove sensitive fields
	for _, user := range users {
		user.PasswordHash = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// CreateUser godoc
// @Summary Create a new user
// @Description Create a new user account
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.CreateUserRequest true "User creation request"
// @Success 201 {object} models.User
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest

	// Bind and validate request using validator utility
	if !validator.BindAndValidate(c, &req) {
		return // Validator already set error response
	}

	// Validate password for local auth users (custom business logic)
	if req.Provider == "" || req.Provider == "local" {
		if req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "Validation failed",
				"fields": map[string]string{"password": "Password is required for local authentication"},
			})
			return
		}
		// Password complexity validated by validator.password tag
	}

	user, err := h.userDB.CreateUser(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create user",
			Message: err.Error(),
		})
		return
	}

	// Remove sensitive fields
	user.PasswordHash = ""

	c.JSON(http.StatusCreated, user)
}

// GetUser godoc
// @Summary Get user by ID
// @Description Get detailed information about a specific user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.userDB.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "User not found",
			Message: err.Error(),
		})
		return
	}

	// Remove sensitive fields
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// GetCurrentUser godoc
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/users/me [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	// Safe type assertion to prevent panic
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal error",
			Message: "Invalid user ID type in context",
		})
		return
	}

	user, err := h.userDB.GetUser(c.Request.Context(), userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "User not found",
			Message: err.Error(),
		})
		return
	}

	// Remove sensitive fields
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// GetCurrentUserQuota godoc
// @Summary Get current user quota
// @Description Get quota information for the currently authenticated user
// @Tags users, quotas
// @Accept json
// @Produce json
// @Success 200 {object} models.UserQuota
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/users/me/quota [get]
func (h *UserHandler) GetCurrentUserQuota(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	// Safe type assertion to prevent panic
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal error",
			Message: "Invalid user ID type in context",
		})
		return
	}

	quota, err := h.userDB.GetUserQuota(c.Request.Context(), userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Quota not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// UpdateUser godoc
// @Summary Update user
// @Description Update user information
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body models.UpdateUserRequest true "User update request"
// @Success 200 {object} models.User
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id} [patch]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.userDB.UpdateUser(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update user",
			Message: err.Error(),
		})
		return
	}

	// Fetch updated user
	user, err := h.userDB.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated user",
			Message: err.Error(),
		})
		return
	}

	// Remove sensitive fields
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Delete a user account
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	if err := h.userDB.DeleteUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete user",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "User deleted successfully",
	})
}

// GetUserSessions godoc
// @Summary Get user sessions
// @Description Get all sessions for a specific user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} gin.H
// @Failure 307 {object} gin.H
// @Router /api/v1/users/{id}/sessions [get]
func (h *UserHandler) GetUserSessions(c *gin.Context) {
	userID := c.Param("id")
	// Redirect to sessions endpoint with user filter
	c.Redirect(http.StatusTemporaryRedirect, "/api/v1/sessions?user="+userID)
}

// GetUserQuota godoc
// @Summary Get user quota
// @Description Get resource quota for a user
// @Tags users, quotas
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.UserQuota
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id}/quota [get]
func (h *UserHandler) GetUserQuota(c *gin.Context) {
	userID := c.Param("id")

	quota, err := h.userDB.GetUserQuota(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Quota not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// SetUserQuota godoc
// @Summary Set user quota
// @Description Set or update resource quota for a user
// @Tags users, quotas
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param quota body models.SetQuotaRequest true "Quota settings"
// @Success 200 {object} models.UserQuota
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id}/quota [put]
func (h *UserHandler) SetUserQuota(c *gin.Context) {
	userID := c.Param("id")

	var req models.SetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.userDB.SetUserQuota(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to set quota",
			Message: err.Error(),
		})
		return
	}

	// Fetch updated quota
	quota, err := h.userDB.GetUserQuota(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated quota",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// GetUserGroups godoc
// @Summary Get user groups
// @Description Get all groups a user belongs to
// @Tags users, groups
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} gin.H
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{id}/groups [get]
func (h *UserHandler) GetUserGroups(c *gin.Context) {
	userID := c.Param("id")

	groupIDs, err := h.userDB.GetUserGroups(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get user groups",
			Message: err.Error(),
		})
		return
	}

	// Fetch full group details
	groups := []interface{}{}
	for _, groupID := range groupIDs {
		group, err := h.groupDB.GetGroup(c.Request.Context(), groupID)
		if err == nil {
			groups = append(groups, group)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// ListAllUserQuotas godoc
// @Summary List all user quotas
// @Description Get resource quotas for all users (admin only)
// @Tags admin, quotas
// @Accept json
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/quotas [get]
func (h *UserHandler) ListAllUserQuotas(c *gin.Context) {
	quotas, err := h.userDB.ListAllUserQuotas(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list quotas",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"quotas": quotas,
		"total":  len(quotas),
	})
}

// GetAdminUserQuota godoc
// @Summary Get user quota by username
// @Description Get resource quota for a user by username (admin only)
// @Tags admin, quotas
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} models.UserQuota
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/quotas/{username} [get]
func (h *UserHandler) GetAdminUserQuota(c *gin.Context) {
	username := c.Param("username")

	// Get user by username
	user, err := h.userDB.GetUserByUsername(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "User not found",
			Message: fmt.Sprintf("User %s not found", username),
		})
		return
	}

	quota, err := h.userDB.GetUserQuota(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Quota not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// SetAdminUserQuota godoc
// @Summary Set user quota by username
// @Description Set or update resource quota for a user by username (admin only)
// @Tags admin, quotas
// @Accept json
// @Produce json
// @Param quota body models.SetQuotaRequest true "Quota settings with username"
// @Success 200 {object} models.UserQuota
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/quotas [put]
func (h *UserHandler) SetAdminUserQuota(c *gin.Context) {
	var req models.SetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if req.Username == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: "Username is required",
		})
		return
	}

	// Get user by username
	user, err := h.userDB.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "User not found",
			Message: fmt.Sprintf("User %s not found", req.Username),
		})
		return
	}

	// Set quota for the user
	if err := h.userDB.SetUserQuota(c.Request.Context(), user.ID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to set quota",
			Message: err.Error(),
		})
		return
	}

	// Fetch updated quota
	quota, err := h.userDB.GetUserQuota(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated quota",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quota)
}

// DeleteAdminUserQuota godoc
// @Summary Delete user quota
// @Description Delete (reset to default) resource quota for a user (admin only)
// @Tags admin, quotas
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} SuccessResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/quotas/{username} [delete]
func (h *UserHandler) DeleteAdminUserQuota(c *gin.Context) {
	username := c.Param("username")

	// Get user by username
	user, err := h.userDB.GetUserByUsername(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "User not found",
			Message: fmt.Sprintf("User %s not found", username),
		})
		return
	}

	// Delete quota (reset to defaults)
	if err := h.userDB.DeleteUserQuota(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete quota",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Quota deleted for user %s", username),
	})
}
