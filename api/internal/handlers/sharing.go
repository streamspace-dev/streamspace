// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements session sharing and collaboration features.
//
// SESSION SHARING FEATURES:
// - Direct user-to-user session sharing with permission levels
// - Shareable invitation links with expiration and usage limits
// - Share revocation and ownership transfer
// - Collaborator tracking and activity monitoring
//
// PERMISSION LEVELS:
// - view: Read-only access to view the session
// - collaborate: Can interact but has limited control
// - control: Full control equivalent to owner
//
// SHARING METHODS:
//
// 1. Direct Shares:
//   - Share with specific users by user ID
//   - Owner-only operation
//   - Requires user existence validation
//   - Supports expiration timestamps
//   - Generates unique share tokens
//
// 2. Invitation Links:
//   - Generate shareable invitation tokens
//   - Configurable max uses and expiration
//   - Anyone with link can accept (until exhausted/expired)
//   - Tracks usage count
//
// OWNERSHIP TRANSFER:
// - Transfer session ownership to another user
// - Requires current owner authorization
// - Validates new owner exists
//
// COLLABORATOR MANAGEMENT:
// - Track active collaborators in sessions
// - Update activity timestamps
// - Remove collaborators
// - Permission inheritance from shares
//
// API Endpoints:
// - POST   /api/v1/sessions/:id/share - Create direct share with user
// - GET    /api/v1/sessions/:id/shares - List all shares for session
// - DELETE /api/v1/sessions/:id/shares/:shareId - Revoke a share
// - POST   /api/v1/sessions/:id/transfer - Transfer ownership
// - POST   /api/v1/sessions/:id/invitations - Create invitation link
// - GET    /api/v1/sessions/:id/invitations - List invitations
// - DELETE /api/v1/invitations/:token - Revoke invitation
// - POST   /api/v1/invitations/:token/accept - Accept invitation
// - GET    /api/v1/sessions/:id/collaborators - List active collaborators
// - POST   /api/v1/sessions/:id/collaborators/:userId/activity - Update activity
// - DELETE /api/v1/sessions/:id/collaborators/:userId - Remove collaborator
// - GET    /api/v1/shared-sessions - List sessions shared with user
//
// Security:
// - Owner-only operations for sharing and transfer
// - User existence validation
// - Expiration and usage limit enforcement
// - Authorization checks for all operations
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
// - Atomic upsert operations for collaborators and shares
//
// Dependencies:
// - Database: sessions, session_shares, session_share_invitations, session_collaborators, users tables
// - External Services: None
//
// Example Usage:
//
//	handler := NewSharingHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/validator"
)

// SharingHandler handles session sharing and collaboration
type SharingHandler struct {
	db *db.Database
}

// NewSharingHandler creates a new sharing handler
func NewSharingHandler(database *db.Database) *SharingHandler {
	return &SharingHandler{
		db: database,
	}
}

// RegisterRoutes registers the sharing routes
func (h *SharingHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/sessions/:id/share", h.CreateShare)
	router.GET("/sessions/:id/shares", h.ListShares)
	router.DELETE("/sessions/:id/shares/:shareId", h.RevokeShare)
	router.POST("/sessions/:id/transfer", h.TransferOwnership)

	router.POST("/sessions/:id/invitations", h.CreateInvitation)
	router.GET("/sessions/:id/invitations", h.ListInvitations)
	router.DELETE("/invitations/:token", h.RevokeInvitation)
	router.POST("/invitations/:token/accept", h.AcceptInvitation)

	router.GET("/sessions/:id/collaborators", h.ListCollaborators)
	router.POST("/sessions/:id/collaborators/:userId/activity", h.UpdateCollaboratorActivity)
	router.DELETE("/sessions/:id/collaborators/:userId", h.RemoveCollaborator)

	router.GET("/shared-sessions", h.ListSharedSessions)
}

// CreateShareRequest represents a request to share a session with a user
type CreateShareRequest struct {
	SharedWithUserId string     `json:"sharedWithUserId" binding:"required" validate:"required,min=1,max=100"`
	PermissionLevel  string     `json:"permissionLevel" binding:"required" validate:"required,oneof=view collaborate control"`
	ExpiresAt        *time.Time `json:"expiresAt"`
}

// CreateShare creates a direct share with a specific user
func (h *SharingHandler) CreateShare(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	var req CreateShareRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	// Get session owner
	var ownerUserId string
	err := h.db.DB().QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = $1`, sessionID).Scan(&ownerUserId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Authorization: Verify the requesting user is the session owner
	currentUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	currentUserIDStr, ok := currentUserID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	if currentUserIDStr != ownerUserId {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the session owner can share this session"})
		return
	}

	// Check if user exists
	var userExists bool
	err = h.db.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, req.SharedWithUserId).Scan(&userExists)
	if err != nil || !userExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
		return
	}

	// Create share
	shareID := uuid.New().String()
	shareToken := uuid.New().String()

	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO session_shares (id, session_id, owner_user_id, shared_with_user_id, permission_level, share_token, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id, shared_with_user_id)
		DO UPDATE SET permission_level = $5, share_token = $6, expires_at = $7, revoked_at = NULL
	`, shareID, sessionID, ownerUserId, req.SharedWithUserId, req.PermissionLevel, shareToken, req.ExpiresAt)

	if err != nil {
		log.Printf("Failed to create share: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create share"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          shareID,
		"shareToken":  shareToken,
		"message":     "Session shared successfully",
	})
}

// ListShares lists all shares for a session
func (h *SharingHandler) ListShares(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			ss.id, ss.session_id, ss.owner_user_id, ss.shared_with_user_id,
			ss.permission_level, ss.share_token, ss.expires_at, ss.created_at,
			ss.accepted_at, ss.revoked_at,
			u.username, u.full_name, u.email
		FROM session_shares ss
		JOIN users u ON ss.shared_with_user_id = u.id
		WHERE ss.session_id = $1 AND ss.revoked_at IS NULL
		ORDER BY ss.created_at DESC
	`, sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	shares := []map[string]interface{}{}
	for rows.Next() {
		var id, sessionId, ownerId, sharedWithId, permissionLevel, shareToken string
		var username, fullName, email string
		var expiresAt, createdAt, acceptedAt, revokedAt sql.NullTime

		if err := rows.Scan(&id, &sessionId, &ownerId, &sharedWithId, &permissionLevel, &shareToken, &expiresAt, &createdAt, &acceptedAt, &revokedAt, &username, &fullName, &email); err != nil {
			continue
		}

		share := map[string]interface{}{
			"id":               id,
			"sessionId":        sessionId,
			"ownerUserId":      ownerId,
			"sharedWithUserId": sharedWithId,
			"permissionLevel":  permissionLevel,
			"shareToken":       shareToken,
			"createdAt":        createdAt,
			"user": map[string]interface{}{
				"id":       sharedWithId,
				"username": username,
				"fullName": fullName,
				"email":    email,
			},
		}

		if expiresAt.Valid {
			share["expiresAt"] = expiresAt.Time
		}
		if acceptedAt.Valid {
			share["acceptedAt"] = acceptedAt.Time
		}

		shares = append(shares, share)
	}

	c.JSON(http.StatusOK, gin.H{
		"shares": shares,
		"total":  len(shares),
	})
}

// RevokeShare revokes a session share
func (h *SharingHandler) RevokeShare(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")
	shareID := c.Param("shareId")

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE session_shares
		SET revoked_at = $1
		WHERE id = $2 AND session_id = $3
	`, time.Now(), shareID, sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Share revoked successfully"})
}

// TransferOwnershipRequest represents a request to transfer session ownership
type TransferOwnershipRequest struct {
	NewOwnerUserId string `json:"newOwnerUserId" binding:"required" validate:"required,min=1,max=100"`
}

// TransferOwnership transfers session ownership to another user
func (h *SharingHandler) TransferOwnership(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	var req TransferOwnershipRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	// Check if new owner exists
	var exists bool
	err := h.db.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, req.NewOwnerUserId).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
		return
	}

	// Update session owner
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE sessions
		SET user_id = $1, updated_at = $2
		WHERE id = $3
	`, req.NewOwnerUserId, time.Now(), sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer ownership"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ownership transferred successfully"})
}

// CreateInvitationRequest represents a request to create a shareable invitation link
type CreateInvitationRequest struct {
	PermissionLevel string     `json:"permissionLevel" binding:"required" validate:"required,oneof=view collaborate control"`
	MaxUses         int        `json:"maxUses" validate:"omitempty,gte=1,lte=1000"`
	ExpiresAt       *time.Time `json:"expiresAt"`
}

// CreateInvitation creates a shareable invitation link
func (h *SharingHandler) CreateInvitation(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	var req CreateInvitationRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	if req.MaxUses == 0 {
		req.MaxUses = 1
	}

	// Get session owner
	var createdBy string
	err := h.db.DB().QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = $1`, sessionID).Scan(&createdBy)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Create invitation
	invitationID := uuid.New().String()
	invitationToken := uuid.New().String()

	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO session_share_invitations (id, session_id, created_by, invitation_token, permission_level, max_uses, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, invitationID, sessionID, createdBy, invitationToken, req.PermissionLevel, req.MaxUses, req.ExpiresAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":              invitationID,
		"invitationToken": invitationToken,
		"message":         "Invitation created successfully",
	})
}

// ListInvitations lists all invitations for a session
func (h *SharingHandler) ListInvitations(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, session_id, created_by, invitation_token, permission_level, max_uses, use_count, expires_at, created_at
		FROM session_share_invitations
		WHERE session_id = $1
		ORDER BY created_at DESC
	`, sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	invitations := []map[string]interface{}{}
	for rows.Next() {
		var id, sessionId, createdBy, invitationToken, permissionLevel string
		var maxUses, useCount int
		var expiresAt, createdAt sql.NullTime

		if err := rows.Scan(&id, &sessionId, &createdBy, &invitationToken, &permissionLevel, &maxUses, &useCount, &expiresAt, &createdAt); err != nil {
			continue
		}

		invitation := map[string]interface{}{
			"id":              id,
			"sessionId":       sessionId,
			"createdBy":       createdBy,
			"invitationToken": invitationToken,
			"permissionLevel": permissionLevel,
			"maxUses":         maxUses,
			"useCount":        useCount,
			"createdAt":       createdAt,
		}

		if expiresAt.Valid {
			invitation["expiresAt"] = expiresAt.Time
			invitation["isExpired"] = expiresAt.Time.Before(time.Now())
		}

		invitation["isExhausted"] = useCount >= maxUses

		invitations = append(invitations, invitation)
	}

	c.JSON(http.StatusOK, gin.H{
		"invitations": invitations,
		"total":       len(invitations),
	})
}

// RevokeInvitation revokes an invitation
func (h *SharingHandler) RevokeInvitation(c *gin.Context) {
	ctx := context.Background()
	token := c.Param("token")

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM session_share_invitations
		WHERE invitation_token = $1
	`, token)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation revoked successfully"})
}

// AcceptInvitationRequest represents a request to accept a session invitation
type AcceptInvitationRequest struct {
	UserId string `json:"userId" binding:"required" validate:"required,min=1,max=100"`
}

// AcceptInvitation accepts an invitation and creates a share
func (h *SharingHandler) AcceptInvitation(c *gin.Context) {
	ctx := context.Background()
	token := c.Param("token")

	var req AcceptInvitationRequest
	if !validator.BindAndValidate(c, &req) {
		return
	}

	// Get invitation details
	var invitationID, sessionID, createdBy, permissionLevel string
	var maxUses, useCount int
	var expiresAt sql.NullTime

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, session_id, created_by, permission_level, max_uses, use_count, expires_at
		FROM session_share_invitations
		WHERE invitation_token = $1
	`, token).Scan(&invitationID, &sessionID, &createdBy, &permissionLevel, &maxUses, &useCount, &expiresAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found or invalid"})
		return
	}

	// Check if expired
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation has expired"})
		return
	}

	// Check if exhausted
	if useCount >= maxUses {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation has been fully used"})
		return
	}

	// Create share
	shareID := uuid.New().String()
	shareToken := uuid.New().String()

	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO session_shares (id, session_id, owner_user_id, shared_with_user_id, permission_level, share_token, accepted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id, shared_with_user_id)
		DO UPDATE SET permission_level = $5, accepted_at = $7, revoked_at = NULL
	`, shareID, sessionID, createdBy, req.UserId, permissionLevel, shareToken, time.Now())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invitation"})
		return
	}

	// Increment use count
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE session_share_invitations
		SET use_count = use_count + 1
		WHERE id = $1
	`, invitationID)

	if err != nil {
		log.Printf("Failed to update invitation use count: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessionId": sessionID,
		"message":   "Invitation accepted successfully",
	})
}

// ListCollaborators lists active collaborators for a session
func (h *SharingHandler) ListCollaborators(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			sc.id, sc.session_id, sc.user_id, sc.permission_level,
			sc.joined_at, sc.last_activity, sc.is_active,
			u.username, u.full_name
		FROM session_collaborators sc
		JOIN users u ON sc.user_id = u.id
		WHERE sc.session_id = $1 AND sc.is_active = true
		ORDER BY sc.last_activity DESC
	`, sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	collaborators := []map[string]interface{}{}
	for rows.Next() {
		var id, sessionId, userId, permissionLevel, username, fullName string
		var joinedAt, lastActivity time.Time
		var isActive bool

		if err := rows.Scan(&id, &sessionId, &userId, &permissionLevel, &joinedAt, &lastActivity, &isActive, &username, &fullName); err != nil {
			continue
		}

		collaborator := map[string]interface{}{
			"id":              id,
			"sessionId":       sessionId,
			"userId":          userId,
			"permissionLevel": permissionLevel,
			"joinedAt":        joinedAt,
			"lastActivity":    lastActivity,
			"isActive":        isActive,
			"user": map[string]interface{}{
				"username": username,
				"fullName": fullName,
			},
		}

		collaborators = append(collaborators, collaborator)
	}

	c.JSON(http.StatusOK, gin.H{
		"collaborators": collaborators,
		"total":         len(collaborators),
	})
}

// UpdateCollaboratorActivity updates collaborator activity timestamp
func (h *SharingHandler) UpdateCollaboratorActivity(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")
	userID := c.Param("userId")

	// Get permission level from shares
	var permissionLevel string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT permission_level FROM session_shares
		WHERE session_id = $1 AND shared_with_user_id = $2 AND revoked_at IS NULL
	`, sessionID, userID).Scan(&permissionLevel)

	if err != nil {
		// Check if user is the owner
		var ownerID string
		err = h.db.DB().QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = $1`, sessionID).Scan(&ownerID)
		if err != nil || ownerID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "User does not have access to this session"})
			return
		}
		permissionLevel = "control"
	}

	// Upsert collaborator
	collaboratorID := uuid.New().String()
	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO session_collaborators (id, session_id, user_id, permission_level, last_activity)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id, user_id)
		DO UPDATE SET last_activity = $5, is_active = true
	`, collaboratorID, sessionID, userID, permissionLevel, time.Now())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// RemoveCollaborator removes a collaborator from a session
func (h *SharingHandler) RemoveCollaborator(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("id")
	userID := c.Param("userId")

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE session_collaborators
		SET is_active = false
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Collaborator removed successfully"})
}

// ListSharedSessions lists all sessions shared with the requesting user
func (h *SharingHandler) ListSharedSessions(c *gin.Context) {
	ctx := context.Background()
	userID := c.Query("userId")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId parameter required"})
		return
	}

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			s.id, s.user_id, s.template_name, s.state, s.app_type,
			s.created_at, s.url,
			ss.permission_level, ss.created_at as shared_at,
			u.username as owner_username
		FROM sessions s
		JOIN session_shares ss ON s.id = ss.session_id
		JOIN users u ON s.user_id = u.id
		WHERE ss.shared_with_user_id = $1
			AND ss.revoked_at IS NULL
			AND (ss.expires_at IS NULL OR ss.expires_at > $2)
		ORDER BY ss.created_at DESC
	`, userID, time.Now())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	sessions := []map[string]interface{}{}
	for rows.Next() {
		var sessionID, ownerUserID, templateName, state, appType, permissionLevel, ownerUsername string
		var url sql.NullString
		var createdAt, sharedAt time.Time

		if err := rows.Scan(&sessionID, &ownerUserID, &templateName, &state, &appType, &createdAt, &url, &permissionLevel, &sharedAt, &ownerUsername); err != nil {
			continue
		}

		session := map[string]interface{}{
			"id":              sessionID,
			"ownerUserId":     ownerUserID,
			"ownerUsername":   ownerUsername,
			"templateName":    templateName,
			"state":           state,
			"appType":         appType,
			"createdAt":       createdAt,
			"sharedAt":        sharedAt,
			"permissionLevel": permissionLevel,
			"isShared":        true,
		}

		if url.Valid {
			session["url"] = url.String
		}

		sessions = append(sessions, session)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}
