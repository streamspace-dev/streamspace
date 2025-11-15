package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// MonitorConfiguration represents a multi-monitor setup
type MonitorConfiguration struct {
	ID          int64                  `json:"id"`
	SessionID   string                 `json:"session_id"`
	UserID      string                 `json:"user_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Monitors    []MonitorDisplay       `json:"monitors"`
	Layout      string                 `json:"layout"` // "horizontal", "vertical", "grid", "custom"
	TotalWidth  int                    `json:"total_width"`
	TotalHeight int                    `json:"total_height"`
	Primary     int                    `json:"primary"` // Index of primary monitor
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	IsActive    bool                   `json:"is_active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// MonitorDisplay represents a single display/monitor
type MonitorDisplay struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	OffsetX     int    `json:"offset_x"`
	OffsetY     int    `json:"offset_y"`
	Rotation    int    `json:"rotation"` // 0, 90, 180, 270
	Scale       float64 `json:"scale"` // 1.0, 1.5, 2.0
	IsPrimary   bool   `json:"is_primary"`
	RefreshRate int    `json:"refresh_rate"` // Hz
}

// MonitorStream represents a VNC stream for a specific monitor
type MonitorStream struct {
	MonitorIndex int    `json:"monitor_index"`
	StreamURL    string `json:"stream_url"`
	WebSocketURL string `json:"websocket_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// CreateMonitorConfiguration creates a new multi-monitor configuration
func (h *Handler) CreateMonitorConfiguration(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	var config MonitorConfiguration
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify session ownership
	if !h.canAccessSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Validate monitors
	if len(config.Monitors) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one monitor required"})
		return
	}
	if len(config.Monitors) > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 8 monitors supported"})
		return
	}

	// Calculate total dimensions based on layout
	config.TotalWidth, config.TotalHeight = h.calculateTotalDimensions(config.Monitors, config.Layout)

	// Deactivate existing configurations
	h.DB.Exec("UPDATE monitor_configurations SET is_active = false WHERE session_id = $1", sessionID)

	// Create new configuration
	var configID int64
	err := h.DB.QueryRow(`
		INSERT INTO monitor_configurations (
			session_id, user_id, name, description, monitors, layout,
			total_width, total_height, primary_monitor, metadata, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, sessionID, userID, config.Name, config.Description, toJSONB(config.Monitors),
		config.Layout, config.TotalWidth, config.TotalHeight, config.Primary,
		toJSONB(config.Metadata), true).Scan(&configID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create monitor configuration"})
		return
	}

	config.ID = configID
	config.IsActive = true

	c.JSON(http.StatusCreated, config)
}

// GetMonitorConfiguration retrieves the active monitor configuration
func (h *Handler) GetMonitorConfiguration(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	if !h.canAccessSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var config MonitorConfiguration
	var monitors, metadata sql.NullString

	err := h.DB.QueryRow(`
		SELECT id, session_id, user_id, name, description, monitors, layout,
		       total_width, total_height, primary_monitor, metadata, is_active,
		       created_at, updated_at
		FROM monitor_configurations
		WHERE session_id = $1 AND is_active = true
		LIMIT 1
	`, sessionID).Scan(&config.ID, &config.SessionID, &config.UserID, &config.Name,
		&config.Description, &monitors, &config.Layout, &config.TotalWidth,
		&config.TotalHeight, &config.Primary, &metadata, &config.IsActive,
		&config.CreatedAt, &config.UpdatedAt)

	if err == sql.ErrNoRows {
		// Return default single-monitor configuration
		config = MonitorConfiguration{
			SessionID: sessionID,
			UserID:    userID,
			Name:      "Default Single Monitor",
			Monitors: []MonitorDisplay{
				{
					Index:       0,
					Name:        "Monitor 1",
					Width:       1920,
					Height:      1080,
					OffsetX:     0,
					OffsetY:     0,
					Rotation:    0,
					Scale:       1.0,
					IsPrimary:   true,
					RefreshRate: 60,
				},
			},
			Layout:      "single",
			TotalWidth:  1920,
			TotalHeight: 1080,
			Primary:     0,
			IsActive:    true,
		}
		c.JSON(http.StatusOK, config)
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve configuration"})
		return
	}

	if monitors.Valid && monitors.String != "" {
		json.Unmarshal([]byte(monitors.String), &config.Monitors)
	}
	if metadata.Valid && metadata.String != "" {
		json.Unmarshal([]byte(metadata.String), &config.Metadata)
	}

	c.JSON(http.StatusOK, config)
}

// UpdateMonitorConfiguration updates an existing configuration
func (h *Handler) UpdateMonitorConfiguration(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("configId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid configuration ID"})
		return
	}

	userID := c.GetString("user_id")

	var config MonitorConfiguration
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify ownership
	var owner string
	h.DB.QueryRow("SELECT user_id FROM monitor_configurations WHERE id = $1", configID).Scan(&owner)
	if owner != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Recalculate dimensions
	config.TotalWidth, config.TotalHeight = h.calculateTotalDimensions(config.Monitors, config.Layout)

	_, err = h.DB.Exec(`
		UPDATE monitor_configurations SET
			name = $1, description = $2, monitors = $3, layout = $4,
			total_width = $5, total_height = $6, primary_monitor = $7,
			metadata = $8, updated_at = $9
		WHERE id = $10
	`, config.Name, config.Description, toJSONB(config.Monitors), config.Layout,
		config.TotalWidth, config.TotalHeight, config.Primary,
		toJSONB(config.Metadata), time.Now(), configID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}

// ActivateMonitorConfiguration activates a specific configuration
func (h *Handler) ActivateMonitorConfiguration(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("configId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid configuration ID"})
		return
	}

	userID := c.GetString("user_id")

	// Get configuration details
	var sessionID, owner string
	err = h.DB.QueryRow(`
		SELECT session_id, user_id FROM monitor_configurations WHERE id = $1
	`, configID).Scan(&sessionID, &owner)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration not found"})
		return
	}

	if owner != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Deactivate all configurations for this session
	h.DB.Exec("UPDATE monitor_configurations SET is_active = false WHERE session_id = $1", sessionID)

	// Activate this configuration
	_, err = h.DB.Exec("UPDATE monitor_configurations SET is_active = true WHERE id = $1", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate configuration"})
		return
	}

	// Apply configuration to running session
	if err := h.applyVNCConfiguration(sessionID, configID); err != nil {
		// Configuration activated in DB, but VNC update failed
		// Session restart may be required
		c.JSON(http.StatusOK, gin.H{
			"message": "configuration activated successfully",
			"warning": "VNC reconfiguration pending - session restart recommended",
			"vnc_apply_error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "configuration activated and applied successfully",
	})
}

// DeleteMonitorConfiguration deletes a configuration
func (h *Handler) DeleteMonitorConfiguration(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("configId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid configuration ID"})
		return
	}

	userID := c.GetString("user_id")

	// Verify ownership
	var owner string
	var isActive bool
	err = h.DB.QueryRow(`
		SELECT user_id, is_active FROM monitor_configurations WHERE id = $1
	`, configID).Scan(&owner, &isActive)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration not found"})
		return
	}

	if owner != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if isActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete active configuration"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM monitor_configurations WHERE id = $1", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration deleted successfully"})
}

// ListMonitorConfigurations lists all configurations for a session
func (h *Handler) ListMonitorConfigurations(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	if !h.canAccessSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, session_id, user_id, name, description, monitors, layout,
		       total_width, total_height, primary_monitor, metadata, is_active,
		       created_at, updated_at
		FROM monitor_configurations
		WHERE session_id = $1
		ORDER BY is_active DESC, created_at DESC
	`, sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve configurations"})
		return
	}
	defer rows.Close()

	configurations := []MonitorConfiguration{}
	for rows.Next() {
		var config MonitorConfiguration
		var monitors, metadata sql.NullString

		err := rows.Scan(&config.ID, &config.SessionID, &config.UserID, &config.Name,
			&config.Description, &monitors, &config.Layout, &config.TotalWidth,
			&config.TotalHeight, &config.Primary, &metadata, &config.IsActive,
			&config.CreatedAt, &config.UpdatedAt)

		if err == nil {
			if monitors.Valid && monitors.String != "" {
				json.Unmarshal([]byte(monitors.String), &config.Monitors)
			}
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &config.Metadata)
			}
			configurations = append(configurations, config)
		}
	}

	c.JSON(http.StatusOK, gin.H{"configurations": configurations})
}

// GetMonitorStreams returns VNC stream URLs for each monitor
func (h *Handler) GetMonitorStreams(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")

	if !h.canAccessSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Get active configuration
	var monitors sql.NullString
	err := h.DB.QueryRow(`
		SELECT monitors FROM monitor_configurations
		WHERE session_id = $1 AND is_active = true
	`, sessionID).Scan(&monitors)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active monitor configuration"})
		return
	}

	var monitorDisplays []MonitorDisplay
	if monitors.Valid && monitors.String != "" {
		json.Unmarshal([]byte(monitors.String), &monitorDisplays)
	}

	// Generate stream URLs for each monitor
	streams := []MonitorStream{}
	for _, monitor := range monitorDisplays {
		stream := MonitorStream{
			MonitorIndex: monitor.Index,
			StreamURL:    fmt.Sprintf("https://%s/api/v1/sessions/%s/stream/monitor/%d", c.Request.Host, sessionID, monitor.Index),
			WebSocketURL: fmt.Sprintf("wss://%s/api/v1/sessions/%s/vnc/monitor/%d", c.Request.Host, sessionID, monitor.Index),
			Width:        monitor.Width,
			Height:       monitor.Height,
		}
		streams = append(streams, stream)
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"streams":    streams,
		"total":      len(streams),
	})
}

// CreatePresetConfiguration creates a preset monitor configuration
func (h *Handler) CreatePresetConfiguration(c *gin.Context) {
	sessionID := c.Param("sessionId")
	userID := c.GetString("user_id")
	preset := c.Param("preset")

	if !h.canAccessSession(userID, sessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var config MonitorConfiguration
	config.SessionID = sessionID
	config.UserID = userID

	switch preset {
	case "dual-horizontal":
		config.Name = "Dual Monitors (Horizontal)"
		config.Layout = "horizontal"
		config.Primary = 0
		config.Monitors = []MonitorDisplay{
			{Index: 0, Name: "Left Monitor", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 0, IsPrimary: true, Scale: 1.0, RefreshRate: 60},
			{Index: 1, Name: "Right Monitor", Width: 1920, Height: 1080, OffsetX: 1920, OffsetY: 0, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
		}
	case "dual-vertical":
		config.Name = "Dual Monitors (Vertical)"
		config.Layout = "vertical"
		config.Primary = 0
		config.Monitors = []MonitorDisplay{
			{Index: 0, Name: "Top Monitor", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 0, IsPrimary: true, Scale: 1.0, RefreshRate: 60},
			{Index: 1, Name: "Bottom Monitor", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 1080, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
		}
	case "triple-horizontal":
		config.Name = "Triple Monitors (Horizontal)"
		config.Layout = "horizontal"
		config.Primary = 1
		config.Monitors = []MonitorDisplay{
			{Index: 0, Name: "Left Monitor", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 0, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
			{Index: 1, Name: "Center Monitor", Width: 1920, Height: 1080, OffsetX: 1920, OffsetY: 0, IsPrimary: true, Scale: 1.0, RefreshRate: 60},
			{Index: 2, Name: "Right Monitor", Width: 1920, Height: 1080, OffsetX: 3840, OffsetY: 0, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
		}
	case "quad-grid":
		config.Name = "Quad Monitors (Grid)"
		config.Layout = "grid"
		config.Primary = 0
		config.Monitors = []MonitorDisplay{
			{Index: 0, Name: "Top Left", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 0, IsPrimary: true, Scale: 1.0, RefreshRate: 60},
			{Index: 1, Name: "Top Right", Width: 1920, Height: 1080, OffsetX: 1920, OffsetY: 0, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
			{Index: 2, Name: "Bottom Left", Width: 1920, Height: 1080, OffsetX: 0, OffsetY: 1080, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
			{Index: 3, Name: "Bottom Right", Width: 1920, Height: 1080, OffsetX: 1920, OffsetY: 1080, IsPrimary: false, Scale: 1.0, RefreshRate: 60},
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid preset"})
		return
	}

	config.TotalWidth, config.TotalHeight = h.calculateTotalDimensions(config.Monitors, config.Layout)

	// Save configuration
	var configID int64
	err := h.DB.QueryRow(`
		INSERT INTO monitor_configurations (
			session_id, user_id, name, description, monitors, layout,
			total_width, total_height, primary_monitor, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, sessionID, userID, config.Name, "Preset configuration", toJSONB(config.Monitors),
		config.Layout, config.TotalWidth, config.TotalHeight, config.Primary, false).Scan(&configID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create preset"})
		return
	}

	config.ID = configID

	c.JSON(http.StatusCreated, config)
}

// Helper functions

// applyVNCConfiguration applies monitor configuration to a running VNC session
func (h *Handler) applyVNCConfiguration(sessionID string, configID int64) error {
	// Fetch the full configuration details
	var monitors, metadata sql.NullString
	var totalWidth, totalHeight int
	var layout string

	err := h.DB.QueryRow(`
		SELECT monitors, layout, total_width, total_height, metadata
		FROM monitor_configurations
		WHERE id = $1
	`, configID).Scan(&monitors, &layout, &totalWidth, &totalHeight, &metadata)

	if err != nil {
		return fmt.Errorf("failed to fetch configuration: %w", err)
	}

	// Parse monitor configuration
	var monitorList []MonitorDisplay
	if monitors.Valid {
		if err := json.Unmarshal([]byte(monitors.String), &monitorList); err != nil {
			return fmt.Errorf("failed to parse monitor configuration: %w", err)
		}
	}

	// Store reconfiguration trigger in database for session to pick up
	// The session's VNC process or a sidecar can poll this table
	_, err = h.DB.Exec(`
		INSERT INTO session_vnc_reconfigs (session_id, config_id, total_width, total_height,
			monitors, layout, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW())
		ON CONFLICT (session_id)
		DO UPDATE SET
			config_id = EXCLUDED.config_id,
			total_width = EXCLUDED.total_width,
			total_height = EXCLUDED.total_height,
			monitors = EXCLUDED.monitors,
			layout = EXCLUDED.layout,
			status = 'pending',
			created_at = NOW()
	`, sessionID, configID, totalWidth, totalHeight, toJSONB(monitorList), layout)

	if err != nil {
		return fmt.Errorf("failed to store VNC reconfiguration trigger: %w", err)
	}

	// TODO: Future enhancement - send WebSocket event to session for immediate reconfiguration
	// Example: h.wsHandler.BroadcastSessionEvent("vnc.reconfigure", sessionID, userID, configData)
	//
	// For now, the session container needs to:
	// 1. Poll session_vnc_reconfigs table for pending reconfigurations
	// 2. Apply new resolution: xrandr --output VNC-0 --mode <totalWidth>x<totalHeight>
	// 3. Restart VNC server with new geometry if needed
	// 4. Update status to 'applied' in session_vnc_reconfigs

	return nil
}

func (h *Handler) calculateTotalDimensions(monitors []MonitorDisplay, layout string) (int, int) {
	if len(monitors) == 0 {
		return 1920, 1080
	}

	var maxX, maxY int
	for _, monitor := range monitors {
		endX := monitor.OffsetX + monitor.Width
		endY := monitor.OffsetY + monitor.Height
		if endX > maxX {
			maxX = endX
		}
		if endY > maxY {
			maxY = endY
		}
	}

	return maxX, maxY
}

// toJSONB converts a value to JSONB format for PostgreSQL
func toJSONB(v interface{}) string {
	if v == nil {
		return "{}"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
