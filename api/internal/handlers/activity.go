// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements session activity tracking and heartbeat management.
//
// ACTIVITY TRACKING:
// - Session heartbeat recording to prevent auto-hibernation
// - Activity status queries (active, idle, hibernation eligibility)
// - Idle duration calculation
// - Last activity timestamp tracking
//
// HEARTBEAT MECHANISM:
// - Clients send periodic heartbeats to indicate active usage
// - Updates session's lastActivity timestamp
// - Prevents idle timeout and auto-hibernation
// - Typically sent every 30-60 seconds from active sessions
//
// ACTIVITY STATUS:
// - Is session currently active (recent heartbeat)
// - Is session idle (exceeded idle threshold)
// - Time since last activity
// - Should session be hibernated (idle + threshold exceeded)
// - Configurable idle thresholds per session
//
// USE CASES:
// - Auto-hibernation prevention for active sessions
// - Resource optimization by hibernating idle sessions
// - User activity analytics
// - Session health monitoring
//
// API Endpoints:
// - POST /api/v1/sessions/:id/heartbeat - Record session heartbeat
// - GET  /api/v1/sessions/:id/activity - Get session activity status
//
// Thread Safety:
// - Activity tracker is thread-safe with mutex protection
// - Kubernetes client operations are thread-safe
//
// Dependencies:
// - Kubernetes: Session CRDs with status updates
// - Activity Tracker: In-memory tracking with persistence
// - External Services: None
//
// Example Usage:
//
//	handler := NewActivityHandler(k8sClient, activityTracker)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/activity"
	"github.com/streamspace/streamspace/api/internal/k8s"
)

// ActivityHandler handles session activity-related endpoints
type ActivityHandler struct {
	k8sClient *k8s.Client
	tracker   *activity.Tracker
}

// NewActivityHandler creates a new activity handler
func NewActivityHandler(k8sClient *k8s.Client, tracker *activity.Tracker) *ActivityHandler {
	return &ActivityHandler{
		k8sClient: k8sClient,
		tracker:   tracker,
	}
}

// RegisterRoutes registers activity-related routes
func (h *ActivityHandler) RegisterRoutes(router *gin.RouterGroup) {
	sessions := router.Group("/sessions")
	{
		sessions.POST("/:id/heartbeat", h.RecordHeartbeat)
		sessions.GET("/:id/activity", h.GetActivity)
	}
}

// HeartbeatRequest represents a session heartbeat request
type HeartbeatRequest struct {
	SessionID string `json:"sessionId"`
}

// ActivityResponse represents session activity status
type ActivityResponse struct {
	SessionID       string  `json:"sessionId"`
	IsActive        bool    `json:"isActive"`
	IsIdle          bool    `json:"isIdle"`
	LastActivity    *string `json:"lastActivity"`
	IdleDuration    int64   `json:"idleDuration"`    // seconds
	IdleThreshold   int64   `json:"idleThreshold"`   // seconds
	ShouldHibernate bool    `json:"shouldHibernate"`
}

// RecordHeartbeat godoc
// @Summary Record session activity heartbeat
// @Description Updates the lastActivity timestamp for a session to indicate it's being actively used
// @Tags sessions, activity
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions/{id}/heartbeat [post]
func (h *ActivityHandler) RecordHeartbeat(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: "Session ID is required",
		})
		return
	}

	namespace := getNamespace(c)

	// Update session activity
	err := h.tracker.UpdateSessionActivity(c.Request.Context(), namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update activity",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Activity recorded",
		"sessionId": sessionID,
	})
}

// GetActivity godoc
// @Summary Get session activity status
// @Description Returns the current activity status of a session including idle state
// @Tags sessions, activity
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} ActivityResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions/{id}/activity [get]
func (h *ActivityHandler) GetActivity(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: "Session ID is required",
		})
		return
	}

	namespace := getNamespace(c)

	// Get session
	session, err := h.k8sClient.GetSession(c.Request.Context(), namespace, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Session not found",
			Message: err.Error(),
		})
		return
	}

	// Get activity status
	status := h.tracker.GetActivityStatus(session)

	response := ActivityResponse{
		SessionID:       sessionID,
		IsActive:        status.IsActive,
		IsIdle:          status.IsIdle,
		IdleDuration:    int64(status.IdleDuration.Seconds()),
		IdleThreshold:   int64(status.IdleThreshold.Seconds()),
		ShouldHibernate: status.ShouldHibernate,
	}

	if status.LastActivity != nil {
		lastActivityStr := status.LastActivity.Format("2006-01-02T15:04:05Z07:00")
		response.LastActivity = &lastActivityStr
	}

	c.JSON(http.StatusOK, response)
}

// getNamespace gets namespace from context or returns default
func getNamespace(c *gin.Context) string {
	if ns, exists := c.Get("namespace"); exists {
		return ns.(string)
	}
	return "streamspace"
}
