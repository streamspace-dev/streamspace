package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// PreferencesHandler handles user preferences and settings
type PreferencesHandler struct {
	db *db.Database
}

// NewPreferencesHandler creates a new preferences handler
func NewPreferencesHandler(database *db.Database) *PreferencesHandler {
	return &PreferencesHandler{
		db: database,
	}
}

// RegisterRoutes registers preference routes
func (h *PreferencesHandler) RegisterRoutes(router *gin.RouterGroup) {
	prefs := router.Group("/preferences")
	{
		// General preferences
		prefs.GET("", h.GetPreferences)
		prefs.PUT("", h.UpdatePreferences)
		prefs.DELETE("", h.ResetPreferences)

		// Specific preference categories
		prefs.GET("/ui", h.GetUIPreferences)
		prefs.PUT("/ui", h.UpdateUIPreferences)

		prefs.GET("/notifications", h.GetNotificationPreferences)
		prefs.PUT("/notifications", h.UpdateNotificationPreferences)

		prefs.GET("/defaults", h.GetDefaultsPreferences)
		prefs.PUT("/defaults", h.UpdateDefaultsPreferences)

		// Favorite templates
		prefs.GET("/favorites", h.GetFavorites)
		prefs.POST("/favorites/:templateName", h.AddFavorite)
		prefs.DELETE("/favorites/:templateName", h.RemoveFavorite)

		// Recent sessions
		prefs.GET("/recent", h.GetRecentSessions)
	}
}

// GetPreferences returns all user preferences
func (h *PreferencesHandler) GetPreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	ctx := context.Background()

	// Get preferences from database
	var prefsJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT preferences FROM user_preferences WHERE user_id = $1
	`, userIDStr).Scan(&prefsJSON)

	if err == sql.ErrNoRows {
		// Return default preferences
		c.JSON(http.StatusOK, h.getDefaultPreferences())
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	var prefs map[string]interface{}
	if err := json.Unmarshal(prefsJSON, &prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse preferences"})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdatePreferences updates user preferences
func (h *PreferencesHandler) UpdatePreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var prefs map[string]interface{}
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Serialize preferences
	prefsJSON, err := json.Marshal(prefs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize preferences"})
		return
	}

	// Upsert preferences
	_, err = h.db.DB().ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET preferences = $2, updated_at = CURRENT_TIMESTAMP
	`, userIDStr, prefsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Preferences updated successfully",
		"preferences": prefs,
	})
}

// GetUIPreferences returns UI-specific preferences
func (h *PreferencesHandler) GetUIPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var prefsJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT preferences->'ui' FROM user_preferences WHERE user_id = $1
	`, userIDStr).Scan(&prefsJSON)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, h.getDefaultUIPreferences())
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get UI preferences"})
		return
	}

	var uiPrefs map[string]interface{}
	json.Unmarshal(prefsJSON, &uiPrefs)

	c.JSON(http.StatusOK, uiPrefs)
}

// UpdateUIPreferences updates UI-specific preferences
func (h *PreferencesHandler) UpdateUIPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var uiPrefs map[string]interface{}
	if err := c.ShouldBindJSON(&uiPrefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	uiPrefsJSON, _ := json.Marshal(uiPrefs)

	// Update just the UI section
	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences)
		VALUES ($1, jsonb_build_object('ui', $2::jsonb))
		ON CONFLICT (user_id)
		DO UPDATE SET
			preferences = jsonb_set(user_preferences.preferences, '{ui}', $2::jsonb),
			updated_at = CURRENT_TIMESTAMP
	`, userIDStr, uiPrefsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update UI preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "UI preferences updated",
		"ui":      uiPrefs,
	})
}

// GetNotificationPreferences returns notification preferences
func (h *PreferencesHandler) GetNotificationPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var prefsJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT preferences->'notifications' FROM user_preferences WHERE user_id = $1
	`, userIDStr).Scan(&prefsJSON)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, h.getDefaultNotificationPreferences())
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification preferences"})
		return
	}

	var notifPrefs map[string]interface{}
	json.Unmarshal(prefsJSON, &notifPrefs)

	c.JSON(http.StatusOK, notifPrefs)
}

// UpdateNotificationPreferences updates notification preferences
func (h *PreferencesHandler) UpdateNotificationPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var notifPrefs map[string]interface{}
	if err := c.ShouldBindJSON(&notifPrefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	notifPrefsJSON, _ := json.Marshal(notifPrefs)

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences)
		VALUES ($1, jsonb_build_object('notifications', $2::jsonb))
		ON CONFLICT (user_id)
		DO UPDATE SET
			preferences = jsonb_set(user_preferences.preferences, '{notifications}', $2::jsonb),
			updated_at = CURRENT_TIMESTAMP
	`, userIDStr, notifPrefsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Notification preferences updated",
		"notifications": notifPrefs,
	})
}

// GetDefaultsPreferences returns default session preferences
func (h *PreferencesHandler) GetDefaultsPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var prefsJSON []byte
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT preferences->'defaults' FROM user_preferences WHERE user_id = $1
	`, userIDStr).Scan(&prefsJSON)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, h.getDefaultSessionDefaults())
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get defaults"})
		return
	}

	var defaults map[string]interface{}
	json.Unmarshal(prefsJSON, &defaults)

	c.JSON(http.StatusOK, defaults)
}

// UpdateDefaultsPreferences updates default session preferences
func (h *PreferencesHandler) UpdateDefaultsPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var defaults map[string]interface{}
	if err := c.ShouldBindJSON(&defaults); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	defaultsJSON, _ := json.Marshal(defaults)

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences)
		VALUES ($1, jsonb_build_object('defaults', $2::jsonb))
		ON CONFLICT (user_id)
		DO UPDATE SET
			preferences = jsonb_set(user_preferences.preferences, '{defaults}', $2::jsonb),
			updated_at = CURRENT_TIMESTAMP
	`, userIDStr, defaultsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update defaults"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Default preferences updated",
		"defaults": defaults,
	})
}

// GetFavorites returns user's favorite templates
func (h *PreferencesHandler) GetFavorites(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT template_name, added_at
		FROM user_favorite_templates
		WHERE user_id = $1
		ORDER BY added_at DESC
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get favorites"})
		return
	}
	defer rows.Close()

	favorites := []map[string]interface{}{}
	for rows.Next() {
		var templateName string
		var addedAt interface{}
		if err := rows.Scan(&templateName, &addedAt); err == nil {
			favorites = append(favorites, map[string]interface{}{
				"templateName": templateName,
				"addedAt":      addedAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"favorites": favorites,
		"total":     len(favorites),
	})
}

// AddFavorite adds a template to favorites
func (h *PreferencesHandler) AddFavorite(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	templateName := c.Param("templateName")

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO user_favorite_templates (user_id, template_name)
		VALUES ($1, $2)
		ON CONFLICT (user_id, template_name) DO NOTHING
	`, userIDStr, templateName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Template added to favorites",
		"templateName": templateName,
	})
}

// RemoveFavorite removes a template from favorites
func (h *PreferencesHandler) RemoveFavorite(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	templateName := c.Param("templateName")

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM user_favorite_templates
		WHERE user_id = $1 AND template_name = $2
	`, userIDStr, templateName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Template removed from favorites",
		"templateName": templateName,
	})
}

// GetRecentSessions returns user's recent sessions
func (h *PreferencesHandler) GetRecentSessions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, template_name, state, created_at
		FROM sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 10
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recent sessions"})
		return
	}
	defer rows.Close()

	sessions := []map[string]interface{}{}
	for rows.Next() {
		var id, templateName, state string
		var createdAt interface{}
		if err := rows.Scan(&id, &templateName, &state, &createdAt); err == nil {
			sessions = append(sessions, map[string]interface{}{
				"id":           id,
				"templateName": templateName,
				"state":        state,
				"createdAt":    createdAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// Reset Preferences resets to defaults
func (h *PreferencesHandler) ResetPreferences(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM user_preferences WHERE user_id = $1
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Preferences reset to defaults",
		"preferences": h.getDefaultPreferences(),
	})
}

// Helper functions for default preferences
func (h *PreferencesHandler) getDefaultPreferences() map[string]interface{} {
	return map[string]interface{}{
		"ui":            h.getDefaultUIPreferences(),
		"notifications": h.getDefaultNotificationPreferences(),
		"defaults":      h.getDefaultSessionDefaults(),
	}
}

func (h *PreferencesHandler) getDefaultUIPreferences() map[string]interface{} {
	return map[string]interface{}{
		"theme":          "light",
		"language":       "en",
		"density":        "comfortable",
		"showTutorials":  true,
		"defaultView":    "grid",
		"itemsPerPage":   20,
		"sidebarCollapsed": false,
	}
}

func (h *PreferencesHandler) getDefaultNotificationPreferences() map[string]interface{} {
	return map[string]interface{}{
		"email": map[string]bool{
			"sessionCreated":  false,
			"sessionIdle":     true,
			"sessionShared":   true,
			"quotaWarning":    true,
			"weeklyReport":    false,
		},
		"inApp": map[string]bool{
			"sessionCreated":    true,
			"sessionIdle":       true,
			"sessionShared":     true,
			"quotaWarning":      true,
			"teamInvitations":   true,
		},
		"webhook": map[string]interface{}{
			"enabled": false,
			"url":     "",
			"events":  []string{},
		},
	}
}

func (h *PreferencesHandler) getDefaultSessionDefaults() map[string]interface{} {
	return map[string]interface{}{
		"autoStart":      true,
		"idleTimeout":    "30m",
		"defaultCPU":     "1000m",
		"defaultMemory":  "2Gi",
		"defaultStorage": "10Gi",
		"preferredTeam":  nil,
	}
}
