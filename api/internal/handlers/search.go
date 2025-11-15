package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// SearchHandler handles advanced search and filtering
type SearchHandler struct {
	db *db.Database
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(database *db.Database) *SearchHandler {
	return &SearchHandler{
		db: database,
	}
}

// SearchResult represents a search result item
type SearchResult struct {
	Type        string                 `json:"type"` // template, session, user, etc.
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Score       float64                `json:"score"` // Relevance score
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SavedSearch represents a saved search query
type SavedSearch struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"userId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Query       string                 `json:"query"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// RegisterRoutes registers search routes
func (h *SearchHandler) RegisterRoutes(router *gin.RouterGroup) {
	search := router.Group("/search")
	{
		// Search endpoints
		search.GET("", h.Search)                    // Universal search
		search.GET("/templates", h.SearchTemplates) // Template-specific search
		search.GET("/sessions", h.SearchSessions)   // Session search
		search.GET("/suggest", h.SearchSuggestions) // Auto-complete suggestions
		search.GET("/advanced", h.AdvancedSearch)   // Advanced multi-filter search

		// Filter endpoints
		search.GET("/filters/categories", h.GetCategories) // List all categories
		search.GET("/filters/tags", h.GetPopularTags)      // List popular tags
		search.GET("/filters/app-types", h.GetAppTypes)    // List app types

		// Saved searches
		search.GET("/saved", h.ListSavedSearches)
		search.POST("/saved", h.CreateSavedSearch)
		search.GET("/saved/:id", h.GetSavedSearch)
		search.PUT("/saved/:id", h.UpdateSavedSearch)
		search.DELETE("/saved/:id", h.DeleteSavedSearch)
		search.POST("/saved/:id/execute", h.ExecuteSavedSearch)

		// Search history
		search.GET("/history", h.GetSearchHistory)
		search.DELETE("/history", h.ClearSearchHistory)
	}
}

// Search performs universal search across all entities
func (h *SearchHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	limit := 20
	ctx := context.Background()

	// Record search history
	userID, exists := c.Get("userID")
	if exists {
		h.recordSearchHistory(ctx, userID.(string), query, "universal", nil)
	}

	results := []SearchResult{}

	// Search templates (full-text search on name, display_name, description, tags)
	templateResults := h.searchTemplatesInternal(ctx, query, limit)
	results = append(results, templateResults...)

	// Could add session search, user search, etc. here

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

// SearchTemplates performs advanced template search
func (h *SearchHandler) SearchTemplates(c *gin.Context) {
	query := c.Query("q")
	category := c.Query("category")
	appType := c.Query("app_type")
	tags := c.Query("tags")      // Comma-separated
	sortBy := c.Query("sort_by") // popularity, rating, name, recent
	limit := 50

	ctx := context.Background()

	// Record search history
	userID, exists := c.Get("userID")
	if exists {
		h.recordSearchHistory(ctx, userID.(string), query, "templates", map[string]interface{}{
			"category": category,
			"app_type": appType,
			"tags":     tags,
		})
	}

	// Build dynamic SQL query
	sqlQuery := `
		SELECT
			id, name, display_name, description, category, tags, icon, app_type,
			avg_rating, install_count, view_count, is_featured
		FROM catalog_templates
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	// Add search term filter
	if query != "" {
		sqlQuery += fmt.Sprintf(` AND (
			name ILIKE $%d OR
			display_name ILIKE $%d OR
			description ILIKE $%d OR
			tags::text ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex)
		args = append(args, "%"+query+"%")
		argIndex++
	}

	// Add category filter
	if category != "" {
		sqlQuery += fmt.Sprintf(` AND category = $%d`, argIndex)
		args = append(args, category)
		argIndex++
	}

	// Add app_type filter
	if appType != "" {
		sqlQuery += fmt.Sprintf(` AND app_type = $%d`, argIndex)
		args = append(args, appType)
		argIndex++
	}

	// Add tags filter
	if tags != "" {
		tagList := strings.Split(tags, ",")
		tagConditions := []string{}
		for _, tag := range tagList {
			tagConditions = append(tagConditions, fmt.Sprintf(`tags::text ILIKE $%d`, argIndex))
			args = append(args, "%"+strings.TrimSpace(tag)+"%")
			argIndex++
		}
		if len(tagConditions) > 0 {
			sqlQuery += ` AND (` + strings.Join(tagConditions, " OR ") + `)`
		}
	}

	// Add sorting
	switch sortBy {
	case "popularity":
		sqlQuery += ` ORDER BY install_count DESC, view_count DESC`
	case "rating":
		sqlQuery += ` ORDER BY avg_rating DESC, rating_count DESC`
	case "name":
		sqlQuery += ` ORDER BY display_name ASC`
	case "recent":
		sqlQuery += ` ORDER BY created_at DESC`
	default:
		// Default: featured first, then popularity
		sqlQuery += ` ORDER BY is_featured DESC, install_count DESC`
	}

	sqlQuery += fmt.Sprintf(` LIMIT %d`, limit)

	rows, err := h.db.DB().QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		var tagsJSON []byte
		var icon, description sql.NullString
		var avgRating sql.NullFloat64
		var installCount, viewCount sql.NullInt64
		var isFeatured bool

		if err := rows.Scan(&r.ID, &r.Name, &r.DisplayName, &description, &r.Category, &tagsJSON, &icon, &r.Type, &avgRating, &installCount, &viewCount, &isFeatured); err == nil {
			r.Type = "template"

			if description.Valid {
				r.Description = description.String
			}
			if icon.Valid {
				r.Icon = icon.String
			}
			if len(tagsJSON) > 0 {
				json.Unmarshal(tagsJSON, &r.Tags)
			}

			// Calculate relevance score
			score := 0.0
			if isFeatured {
				score += 50.0
			}
			if avgRating.Valid {
				score += avgRating.Float64 * 10.0
			}
			if installCount.Valid {
				score += float64(installCount.Int64) * 0.1
			}
			if viewCount.Valid {
				score += float64(viewCount.Int64) * 0.01
			}
			r.Score = score

			r.Metadata = map[string]interface{}{
				"rating":   avgRating.Float64,
				"installs": installCount.Int64,
				"views":    viewCount.Int64,
				"featured": isFeatured,
			}

			results = append(results, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":    query,
		"category": category,
		"app_type": appType,
		"tags":     tags,
		"sort_by":  sortBy,
		"results":  results,
		"count":    len(results),
	})
}

// SearchSessions searches user sessions
func (h *SearchHandler) SearchSessions(c *gin.Context) {
	query := c.Query("q")
	state := c.Query("state") // running, hibernated, terminated

	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	sqlQuery := `
		SELECT id, template_name, state, created_at, last_connection
		FROM sessions
		WHERE user_id = $1
	`
	args := []interface{}{userIDStr}
	argIndex := 2

	if query != "" {
		sqlQuery += fmt.Sprintf(` AND (id ILIKE $%d OR template_name ILIKE $%d)`, argIndex, argIndex)
		args = append(args, "%"+query+"%")
		argIndex++
	}

	if state != "" {
		sqlQuery += fmt.Sprintf(` AND state = $%d`, argIndex)
		args = append(args, state)
		argIndex++
	}

	sqlQuery += ` ORDER BY created_at DESC LIMIT 50`

	rows, err := h.db.DB().QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		var templateName, state string
		var createdAt time.Time
		var lastConnection sql.NullTime

		if err := rows.Scan(&r.ID, &templateName, &state, &createdAt, &lastConnection); err == nil {
			r.Type = "session"
			r.Name = r.ID
			r.DisplayName = templateName
			r.Metadata = map[string]interface{}{
				"state":     state,
				"createdAt": createdAt,
			}
			if lastConnection.Valid {
				r.Metadata["lastConnection"] = lastConnection.Time
			}

			results = append(results, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"state":   state,
		"results": results,
		"count":   len(results),
	})
}

// SearchSuggestions provides auto-complete suggestions
func (h *SearchHandler) SearchSuggestions(c *gin.Context) {
	query := c.Query("q")
	if query == "" || len(query) < 2 {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT DISTINCT display_name
		FROM catalog_templates
		WHERE display_name ILIKE $1
		ORDER BY install_count DESC
		LIMIT 10
	`, query+"%")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get suggestions"})
		return
	}
	defer rows.Close()

	suggestions := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			suggestions = append(suggestions, name)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":       query,
		"suggestions": suggestions,
	})
}

// AdvancedSearch performs multi-criteria search
func (h *SearchHandler) AdvancedSearch(c *gin.Context) {
	var req struct {
		Query   string                 `json:"query"`
		Filters map[string]interface{} `json:"filters"`
		Sort    string                 `json:"sort"`
		Limit   int                    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit == 0 || req.Limit > 100 {
		req.Limit = 50
	}

	// Convert to query params and call SearchTemplates
	c.Request.URL.RawQuery = fmt.Sprintf("q=%s&sort_by=%s", req.Query, req.Sort)

	if category, ok := req.Filters["category"].(string); ok {
		c.Request.URL.RawQuery += fmt.Sprintf("&category=%s", category)
	}
	if appType, ok := req.Filters["app_type"].(string); ok {
		c.Request.URL.RawQuery += fmt.Sprintf("&app_type=%s", appType)
	}

	h.SearchTemplates(c)
}

// GetCategories returns all template categories
func (h *SearchHandler) GetCategories(c *gin.Context) {
	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT category, COUNT(*) as count
		FROM catalog_templates
		GROUP BY category
		ORDER BY count DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get categories"})
		return
	}
	defer rows.Close()

	categories := []map[string]interface{}{}
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err == nil {
			categories = append(categories, map[string]interface{}{
				"name":  category,
				"count": count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
		"total":      len(categories),
	})
}

// GetPopularTags returns most popular tags
func (h *SearchHandler) GetPopularTags(c *gin.Context) {
	ctx := context.Background()
	limit := 50

	// Proper JSONB array handling for production
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT tag, COUNT(*) as count
		FROM (
			SELECT jsonb_array_elements_text(tags) as tag
			FROM catalog_templates
			WHERE tags IS NOT NULL
			  AND jsonb_typeof(tags) = 'array'
			  AND jsonb_array_length(tags) > 0
		) subquery
		WHERE tag IS NOT NULL AND tag != ''
		GROUP BY tag
		ORDER BY count DESC, tag ASC
		LIMIT $1
	`, limit)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tags"})
		return
	}
	defer rows.Close()

	tags := []map[string]interface{}{}
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			log.Printf("[ERROR] Failed to scan tag row: %v", err)
			continue
		}

		// jsonb_array_elements_text() already returns clean strings
		// No manual cleanup needed
		tags = append(tags, map[string]interface{}{
			"name":  tag,
			"count": count,
		})
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Printf("[ERROR] Error iterating tag rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tags":  tags,
		"total": len(tags),
	})
}

// GetAppTypes returns all app types
func (h *SearchHandler) GetAppTypes(c *gin.Context) {
	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT app_type, COUNT(*) as count
		FROM catalog_templates
		WHERE app_type IS NOT NULL AND app_type != ''
		GROUP BY app_type
		ORDER BY count DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get app types"})
		return
	}
	defer rows.Close()

	appTypes := []map[string]interface{}{}
	for rows.Next() {
		var appType string
		var count int
		if err := rows.Scan(&appType, &count); err == nil {
			appTypes = append(appTypes, map[string]interface{}{
				"name":  appType,
				"count": count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"appTypes": appTypes,
		"total":    len(appTypes),
	})
}

// Saved searches management

// ListSavedSearches returns user's saved searches
func (h *SearchHandler) ListSavedSearches(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, user_id, name, description, query, filters, created_at, updated_at
		FROM saved_searches
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get saved searches"})
		return
	}
	defer rows.Close()

	searches := []SavedSearch{}
	for rows.Next() {
		var s SavedSearch
		var description sql.NullString
		var filtersJSON []byte

		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &description, &s.Query, &filtersJSON, &s.CreatedAt, &s.UpdatedAt); err == nil {
			if description.Valid {
				s.Description = description.String
			}
			if len(filtersJSON) > 0 {
				json.Unmarshal(filtersJSON, &s.Filters)
			}
			searches = append(searches, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"searches": searches,
		"count":    len(searches),
	})
}

// CreateSavedSearch creates a new saved search
func (h *SearchHandler) CreateSavedSearch(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Query       string                 `json:"query" binding:"required"`
		Filters     map[string]interface{} `json:"filters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	searchID := fmt.Sprintf("search_%d", time.Now().UnixNano())
	filtersJSON, _ := json.Marshal(req.Filters)

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO saved_searches (id, user_id, name, description, query, filters)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, searchID, userIDStr, req.Name, req.Description, req.Query, filtersJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save search"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Search saved successfully",
		"searchId": searchID,
	})
}

// GetSavedSearch retrieves a specific saved search
func (h *SearchHandler) GetSavedSearch(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	searchID := c.Param("id")

	ctx := context.Background()

	var s SavedSearch
	var description sql.NullString
	var filtersJSON []byte

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, user_id, name, description, query, filters, created_at, updated_at
		FROM saved_searches
		WHERE id = $1 AND user_id = $2
	`, searchID, userIDStr).Scan(&s.ID, &s.UserID, &s.Name, &description, &s.Query, &filtersJSON, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get saved search"})
		return
	}

	if description.Valid {
		s.Description = description.String
	}
	if len(filtersJSON) > 0 {
		json.Unmarshal(filtersJSON, &s.Filters)
	}

	c.JSON(http.StatusOK, s)
}

// UpdateSavedSearch updates a saved search
func (h *SearchHandler) UpdateSavedSearch(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	searchID := c.Param("id")

	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Query       string                 `json:"query"`
		Filters     map[string]interface{} `json:"filters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	filtersJSON, _ := json.Marshal(req.Filters)

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE saved_searches
		SET name = $1, description = $2, query = $3, filters = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 AND user_id = $6
	`, req.Name, req.Description, req.Query, filtersJSON, searchID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update saved search"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Search updated successfully",
		"searchId": searchID,
	})
}

// DeleteSavedSearch deletes a saved search
func (h *SearchHandler) DeleteSavedSearch(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	searchID := c.Param("id")

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM saved_searches WHERE id = $1 AND user_id = $2
	`, searchID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete saved search"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Search deleted successfully",
		"searchId": searchID,
	})
}

// ExecuteSavedSearch executes a saved search
func (h *SearchHandler) ExecuteSavedSearch(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	searchID := c.Param("id")

	ctx := context.Background()

	var query string
	var filtersJSON []byte

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT query, filters FROM saved_searches WHERE id = $1 AND user_id = $2
	`, searchID, userIDStr).Scan(&query, &filtersJSON)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get saved search"})
		return
	}

	// Set query params and execute
	c.Request.URL.RawQuery = "q=" + query
	h.SearchTemplates(c)
}

// GetSearchHistory returns user's recent searches
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT query, search_type, filters, searched_at
		FROM search_history
		WHERE user_id = $1
		ORDER BY searched_at DESC
		LIMIT 50
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get search history"})
		return
	}
	defer rows.Close()

	history := []map[string]interface{}{}
	for rows.Next() {
		var query, searchType string
		var filtersJSON []byte
		var searchedAt time.Time

		if err := rows.Scan(&query, &searchType, &filtersJSON, &searchedAt); err == nil {
			item := map[string]interface{}{
				"query":      query,
				"type":       searchType,
				"searchedAt": searchedAt,
			}
			if len(filtersJSON) > 0 {
				var filters map[string]interface{}
				json.Unmarshal(filtersJSON, &filters)
				item["filters"] = filters
			}
			history = append(history, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"count":   len(history),
	})
}

// ClearSearchHistory clears user's search history
func (h *SearchHandler) ClearSearchHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM search_history WHERE user_id = $1
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear search history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Search history cleared",
	})
}

// Helper functions

func (h *SearchHandler) searchTemplatesInternal(ctx context.Context, query string, limit int) []SearchResult {
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, name, display_name, description, category, tags, icon, app_type, avg_rating, install_count
		FROM catalog_templates
		WHERE name ILIKE $1 OR display_name ILIKE $1 OR description ILIKE $1 OR tags::text ILIKE $1
		ORDER BY install_count DESC
		LIMIT $2
	`, "%"+query+"%", limit)

	if err != nil {
		return []SearchResult{}
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		var tagsJSON []byte
		var icon, description sql.NullString
		var avgRating sql.NullFloat64
		var installCount sql.NullInt64

		if err := rows.Scan(&r.ID, &r.Name, &r.DisplayName, &description, &r.Category, &tagsJSON, &icon, &r.Type, &avgRating, &installCount); err == nil {
			r.Type = "template"
			if description.Valid {
				r.Description = description.String
			}
			if icon.Valid {
				r.Icon = icon.String
			}
			if len(tagsJSON) > 0 {
				json.Unmarshal(tagsJSON, &r.Tags)
			}

			score := 0.0
			if avgRating.Valid {
				score += avgRating.Float64 * 10.0
			}
			if installCount.Valid {
				score += float64(installCount.Int64) * 0.1
			}
			r.Score = score

			results = append(results, r)
		}
	}

	return results
}

func (h *SearchHandler) recordSearchHistory(ctx context.Context, userID, query, searchType string, filters map[string]interface{}) {
	filtersJSON, _ := json.Marshal(filters)

	h.db.DB().ExecContext(ctx, `
		INSERT INTO search_history (user_id, query, search_type, filters)
		VALUES ($1, $2, $3, $4)
	`, userID, query, searchType, filtersJSON)
}
