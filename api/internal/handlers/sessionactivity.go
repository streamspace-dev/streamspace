package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// SessionActivityHandler handles session activity logging and queries
type SessionActivityHandler struct {
	db *db.Database
}

// NewSessionActivityHandler creates a new session activity handler
func NewSessionActivityHandler(database *db.Database) *SessionActivityHandler {
	return &SessionActivityHandler{
		db: database,
	}
}

// Event categories for classification
const (
	EventCategoryLifecycle    = "lifecycle"
	EventCategoryConnection   = "connection"
	EventCategoryState        = "state"
	EventCategoryConfiguration = "configuration"
	EventCategoryAccess       = "access"
	EventCategoryError        = "error"
)

// Event types for tracking
const (
	EventSessionCreated    = "session.created"
	EventSessionStarted    = "session.started"
	EventSessionStopped    = "session.stopped"
	EventSessionHibernated = "session.hibernated"
	EventSessionWoken      = "session.woken"
	EventSessionTerminated = "session.terminated"
	EventSessionDeleted    = "session.deleted"

	EventUserConnected    = "user.connected"
	EventUserDisconnected = "user.disconnected"
	EventUserHeartbeat    = "user.heartbeat"

	EventStateChanged       = "state.changed"
	EventResourcesUpdated   = "resources.updated"
	EventConfigUpdated      = "config.updated"
	EventTagsUpdated        = "tags.updated"

	EventAccessGranted = "access.granted"
	EventAccessDenied  = "access.denied"
	EventShareCreated  = "share.created"
	EventShareRevoked  = "share.revoked"

	EventError = "error.occurred"
)

// SessionActivityEvent represents a session activity event
type SessionActivityEvent struct {
	ID            int                    `json:"id"`
	SessionID     string                 `json:"sessionId"`
	UserID        string                 `json:"userId,omitempty"`
	EventType     string                 `json:"eventType"`
	EventCategory string                 `json:"eventCategory"`
	Description   string                 `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	IPAddress     string                 `json:"ipAddress,omitempty"`
	UserAgent     string                 `json:"userAgent,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

// LogActivityEvent logs a session activity event
func (h *SessionActivityHandler) LogActivityEvent(c *gin.Context) {
	ctx := context.Background()

	var req struct {
		SessionID     string                 `json:"sessionId" binding:"required"`
		EventType     string                 `json:"eventType" binding:"required"`
		EventCategory string                 `json:"eventCategory"`
		Description   string                 `json:"description"`
		Metadata      map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context (if authenticated)
	userID, _ := c.Get("userID")
	userIDStr := ""
	if userID != nil {
		if id, ok := userID.(string); ok {
			userIDStr = id
		}
	}

	// Default category if not provided
	if req.EventCategory == "" {
		req.EventCategory = EventCategoryLifecycle
	}

	// Serialize metadata
	var metadataJSON []byte
	if req.Metadata != nil {
		metadataJSON, _ = json.Marshal(req.Metadata)
	}

	// Insert event
	query := `
		INSERT INTO session_activity_log
		(session_id, user_id, event_type, event_category, description, metadata, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, timestamp
	`

	var eventID int
	var timestamp time.Time
	err := h.db.DB().QueryRowContext(
		ctx,
		query,
		req.SessionID,
		userIDStr,
		req.EventType,
		req.EventCategory,
		req.Description,
		metadataJSON,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	).Scan(&eventID, &timestamp)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log event"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        eventID,
		"timestamp": timestamp,
		"message":   "Event logged successfully",
	})
}

// GetSessionActivity returns activity log for a specific session
func (h *SessionActivityHandler) GetSessionActivity(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("sessionId")

	// Pagination
	limit := 100
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 500 {
			limit = parsedLimit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Filters
	eventType := c.Query("event_type")
	category := c.Query("category")

	// Build query
	query := `
		SELECT id, session_id, user_id, event_type, event_category,
		       description, metadata, ip_address, user_agent, timestamp
		FROM session_activity_log
		WHERE session_id = $1
	`
	args := []interface{}{sessionID}
	argIdx := 2

	if eventType != "" {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, eventType)
		argIdx++
	}

	if category != "" {
		query += fmt.Sprintf(" AND event_category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS filtered", query)
	var total int
	h.db.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total)

	// Add ordering and pagination
	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := h.db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Collect events
	events := []SessionActivityEvent{}
	for rows.Next() {
		var event SessionActivityEvent
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.UserID,
			&event.EventType,
			&event.EventCategory,
			&event.Description,
			&metadataJSON,
			&event.IPAddress,
			&event.UserAgent,
			&event.Timestamp,
		)
		if err != nil {
			continue
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	c.JSON(http.StatusOK, gin.H{
		"events":    events,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"sessionId": sessionID,
	})
}

// GetActivityStats returns activity statistics
func (h *SessionActivityHandler) GetActivityStats(c *gin.Context) {
	ctx := context.Background()

	// Get top event types
	eventTypeStatsQuery := `
		SELECT event_type, COUNT(*) as count
		FROM session_activity_log
		WHERE timestamp >= NOW() - INTERVAL '7 days'
		GROUP BY event_type
		ORDER BY count DESC
		LIMIT 10
	`

	rows, err := h.db.DB().QueryContext(ctx, eventTypeStatsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	defer rows.Close()

	eventTypeStats := []map[string]interface{}{}
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err == nil {
			eventTypeStats = append(eventTypeStats, map[string]interface{}{
				"eventType": eventType,
				"count":     count,
			})
		}
	}

	// Get event count by category
	categoryStatsQuery := `
		SELECT event_category, COUNT(*) as count
		FROM session_activity_log
		WHERE timestamp >= NOW() - INTERVAL '7 days'
		GROUP BY event_category
		ORDER BY count DESC
	`

	rows2, err := h.db.DB().QueryContext(ctx, categoryStatsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get category stats"})
		return
	}
	defer rows2.Close()

	categoryStats := []map[string]interface{}{}
	for rows2.Next() {
		var category string
		var count int
		if err := rows2.Scan(&category, &count); err == nil {
			categoryStats = append(categoryStats, map[string]interface{}{
				"category": category,
				"count":    count,
			})
		}
	}

	// Get total event count
	var totalEvents int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM session_activity_log
	`).Scan(&totalEvents)

	// Get recent events (last 24 hours)
	var recentEvents int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM session_activity_log
		WHERE timestamp >= NOW() - INTERVAL '24 hours'
	`).Scan(&recentEvents)

	c.JSON(http.StatusOK, gin.H{
		"totalEvents":     totalEvents,
		"recentEvents24h": recentEvents,
		"topEventTypes":   eventTypeStats,
		"byCategory":      categoryStats,
		"timestamp":       time.Now(),
	})
}

// GetSessionTimeline returns a timeline view of session activity
func (h *SessionActivityHandler) GetSessionTimeline(c *gin.Context) {
	ctx := context.Background()
	sessionID := c.Param("sessionId")

	query := `
		SELECT id, event_type, event_category, description,
		       metadata, user_id, timestamp
		FROM session_activity_log
		WHERE session_id = $1
		ORDER BY timestamp ASC
		LIMIT 1000
	`

	rows, err := h.db.DB().QueryContext(ctx, query, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TimelineEvent struct {
		ID            int                    `json:"id"`
		EventType     string                 `json:"eventType"`
		EventCategory string                 `json:"eventCategory"`
		Description   string                 `json:"description,omitempty"`
		Metadata      map[string]interface{} `json:"metadata,omitempty"`
		UserID        string                 `json:"userId,omitempty"`
		Timestamp     time.Time              `json:"timestamp"`
		DurationSince int64                  `json:"durationSince,omitempty"` // Seconds since previous event
	}

	events := []TimelineEvent{}
	var previousTimestamp *time.Time

	for rows.Next() {
		var event TimelineEvent
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.EventCategory,
			&event.Description,
			&metadataJSON,
			&event.UserID,
			&event.Timestamp,
		)
		if err != nil {
			continue
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		// Calculate duration since previous event
		if previousTimestamp != nil {
			event.DurationSince = int64(event.Timestamp.Sub(*previousTimestamp).Seconds())
		}
		previousTimestamp = &event.Timestamp

		events = append(events, event)
	}

	c.JSON(http.StatusOK, gin.H{
		"timeline":  events,
		"total":     len(events),
		"sessionId": sessionID,
	})
}

// GetUserSessionActivity returns all session activity for a specific user
func (h *SessionActivityHandler) GetUserSessionActivity(c *gin.Context) {
	ctx := context.Background()
	userID := c.Param("userId")

	// Pagination
	limit := 50
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	query := `
		SELECT id, session_id, event_type, event_category,
		       description, metadata, timestamp
		FROM session_activity_log
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := h.db.DB().QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	events := []SessionActivityEvent{}
	for rows.Next() {
		var event SessionActivityEvent
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.EventType,
			&event.EventCategory,
			&event.Description,
			&metadataJSON,
			&event.Timestamp,
		)
		if err != nil {
			continue
		}

		event.UserID = userID

		// Parse metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	// Get total count
	var total int
	h.db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM session_activity_log WHERE user_id = $1
	`, userID).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"userId": userID,
	})
}
