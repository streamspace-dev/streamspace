package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
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

	user, err := h.userDB.GetUser(c.Request.Context(), userID.(string))
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

	quota, err := h.userDB.GetUserQuota(c.Request.Context(), userID.(string))
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

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string `json:"message"`
}
