// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements template catalog browsing, ratings, and statistics.
//
// CATALOG FEATURES:
// - Template discovery with advanced search and filtering
// - Featured, trending, and popular template lists
// - User ratings and reviews system
// - View and install tracking for analytics
//
// SEARCH AND FILTERING:
// - Full-text search across template names and descriptions
// - Filter by category, tags, app type
// - Sort by popularity, rating, recency, or install count
// - Pagination support with customizable page size
//
// RATINGS AND REVIEWS:
// - Add, update, and delete template ratings
// - Star ratings (1-5 scale)
// - User comments and reviews
// - Average rating calculation
// - Rating count tracking
//
// STATISTICS:
// - View count tracking (catalog impressions)
// - Install count tracking (template usage)
// - Trending calculation based on recent activity
// - Popular templates based on install count
//
// API Endpoints:
// - GET    /api/v1/catalog/templates - List templates with filters and search
// - GET    /api/v1/catalog/templates/:id - Get template details
// - GET    /api/v1/catalog/templates/featured - List featured templates
// - GET    /api/v1/catalog/templates/trending - List trending templates
// - GET    /api/v1/catalog/templates/popular - List popular templates
// - POST   /api/v1/catalog/templates/:id/ratings - Add rating/review
// - GET    /api/v1/catalog/templates/:id/ratings - Get template ratings
// - PUT    /api/v1/catalog/templates/:id/ratings/:ratingId - Update rating
// - DELETE /api/v1/catalog/templates/:id/ratings/:ratingId - Delete rating
// - POST   /api/v1/catalog/templates/:id/view - Record template view
// - POST   /api/v1/catalog/templates/:id/install - Record template install
//
// Thread Safety:
// - All database operations are thread-safe via connection pooling
//
// Dependencies:
// - Database: catalog_templates, repositories, template_ratings tables
// - External Services: Repository sync for template metadata
//
// Example Usage:
//
//	handler := NewCatalogHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/streamspace/streamspace/api/internal/db"
)

// CatalogHandler handles template catalog-related endpoints
type CatalogHandler struct {
	db *db.Database
}

// NewCatalogHandler creates a new catalog handler
func NewCatalogHandler(database *db.Database) *CatalogHandler {
	return &CatalogHandler{
		db: database,
	}
}

// RegisterRoutes registers catalog-related routes
func (h *CatalogHandler) RegisterRoutes(router *gin.RouterGroup) {
	catalog := router.Group("/catalog")
	{
		catalog.GET("/templates", h.ListTemplates)
		catalog.GET("/templates/:id", h.GetTemplateDetails)
		catalog.GET("/templates/featured", h.GetFeaturedTemplates)
		catalog.GET("/templates/trending", h.GetTrendingTemplates)
		catalog.GET("/templates/popular", h.GetPopularTemplates)

		// Ratings and reviews
		catalog.POST("/templates/:id/ratings", h.AddRating)
		catalog.GET("/templates/:id/ratings", h.GetRatings)
		catalog.PUT("/templates/:id/ratings/:ratingId", h.UpdateRating)
		catalog.DELETE("/templates/:id/ratings/:ratingId", h.DeleteRating)

		// Statistics
		catalog.POST("/templates/:id/view", h.RecordView)
		catalog.POST("/templates/:id/install", h.RecordInstall)
	}
}

// ListTemplates godoc
// @Summary List catalog templates with advanced filtering
// @Description Get templates from catalog with search, filtering, and sorting
// @Tags catalog
// @Accept json
// @Produce json
// @Param search query string false "Search query"
// @Param category query string false "Filter by category"
// @Param tag query string false "Filter by tag"
// @Param appType query string false "Filter by app type"
// @Param featured query boolean false "Show only featured"
// @Param sort query string false "Sort by (popular, rating, recent, installs)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/catalog/templates [get]
func (h *CatalogHandler) ListTemplates(c *gin.Context) {
	search := c.Query("search")
	category := c.Query("category")
	tag := c.Query("tag")
	appType := c.Query("appType")
	featured := c.Query("featured") == "true"
	sortBy := c.DefaultQuery("sort", "popular")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Build query
	query := `
		SELECT
			ct.id, ct.repository_id, ct.name, ct.display_name, ct.description,
			ct.category, ct.app_type, ct.icon_url, ct.tags, ct.install_count,
			ct.is_featured, ct.version, ct.view_count, ct.avg_rating, ct.rating_count,
			ct.created_at, ct.updated_at,
			r.name as repository_name, r.url as repository_url
		FROM catalog_templates ct
		JOIN repositories r ON ct.repository_id = r.id
		WHERE r.status = 'synced'
	`

	args := []interface{}{}
	argIdx := 1

	// Apply filters
	if search != "" {
		query += ` AND (ct.display_name ILIKE $` + strconv.Itoa(argIdx) +
			` OR ct.description ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if category != "" {
		query += ` AND ct.category = $` + strconv.Itoa(argIdx)
		args = append(args, category)
		argIdx++
	}

	if tag != "" {
		query += ` AND $` + strconv.Itoa(argIdx) + ` = ANY(ct.tags)`
		args = append(args, tag)
		argIdx++
	}

	if appType != "" {
		query += ` AND ct.app_type = $` + strconv.Itoa(argIdx)
		args = append(args, appType)
		argIdx++
	}

	if featured {
		query += ` AND ct.is_featured = true`
	}

	// Apply sorting
	switch sortBy {
	case "rating":
		query += ` ORDER BY ct.avg_rating DESC, ct.rating_count DESC`
	case "recent":
		query += ` ORDER BY ct.created_at DESC`
	case "installs":
		query += ` ORDER BY ct.install_count DESC`
	case "views":
		query += ` ORDER BY ct.view_count DESC`
	default: // popular
		query += ` ORDER BY (ct.install_count * 3 + ct.view_count + ct.rating_count * 10) DESC`
	}

	// Add pagination
	query += ` LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.db.DB().QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	templates := []map[string]interface{}{}
	for rows.Next() {
		var id, repositoryID, installCount, viewCount, ratingCount int
		var name, displayName, description, category, appType, iconURL, version, repoName, repoURL string
		var tags pq.StringArray
		var isFeatured bool
		var avgRating float64
		var createdAt, updatedAt interface{}

		err := rows.Scan(
			&id, &repositoryID, &name, &displayName, &description,
			&category, &appType, &iconURL, &tags, &installCount,
			&isFeatured, &version, &viewCount, &avgRating, &ratingCount,
			&createdAt, &updatedAt, &repoName, &repoURL,
		)
		if err != nil {
			continue
		}

		templates = append(templates, map[string]interface{}{
			"id":           id,
			"repositoryId": repositoryID,
			"name":         name,
			"displayName":  displayName,
			"description":  description,
			"category":     category,
			"appType":      appType,
			"icon":         iconURL,
			"tags":         tags,
			"installCount": installCount,
			"isFeatured":   isFeatured,
			"version":      version,
			"viewCount":    viewCount,
			"avgRating":    avgRating,
			"ratingCount":  ratingCount,
			"createdAt":    createdAt,
			"updatedAt":    updatedAt,
			"repository": map[string]string{
				"name": repoName,
				"url":  repoURL,
			},
		})
	}

	// Get total count for pagination
	countQuery := `
		SELECT COUNT(*)
		FROM catalog_templates ct
		JOIN repositories r ON ct.repository_id = r.id
		WHERE r.status = 'synced'
	`
	countArgs := []interface{}{}
	countArgIdx := 1

	if search != "" {
		countQuery += ` AND (ct.display_name ILIKE $` + strconv.Itoa(countArgIdx) +
			` OR ct.description ILIKE $` + strconv.Itoa(countArgIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		countArgIdx++
	}
	if category != "" {
		countQuery += ` AND ct.category = $` + strconv.Itoa(countArgIdx)
		countArgs = append(countArgs, category)
		countArgIdx++
	}
	if tag != "" {
		countQuery += ` AND $` + strconv.Itoa(countArgIdx) + ` = ANY(ct.tags)`
		countArgs = append(countArgs, tag)
		countArgIdx++
	}
	if appType != "" {
		countQuery += ` AND ct.app_type = $` + strconv.Itoa(countArgIdx)
		countArgs = append(countArgs, appType)
		countArgIdx++
	}
	if featured {
		countQuery += ` AND ct.is_featured = true`
	}

	var total int
	h.db.DB().QueryRowContext(c.Request.Context(), countQuery, countArgs...).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     total,
		"page":      page,
		"limit":     limit,
		"totalPages": (total + limit - 1) / limit,
	})
}

// GetTemplateDetails godoc
// @Summary Get detailed template information
// @Description Get complete template details including ratings and stats
// @Tags catalog
// @Accept json
// @Produce json
// @Param id path int true "Template ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/catalog/templates/{id} [get]
func (h *CatalogHandler) GetTemplateDetails(c *gin.Context) {
	templateID := c.Param("id")

	query := `
		SELECT
			ct.id, ct.repository_id, ct.name, ct.display_name, ct.description,
			ct.category, ct.app_type, ct.icon_url, ct.manifest, ct.tags,
			ct.install_count, ct.is_featured, ct.version, ct.view_count,
			ct.avg_rating, ct.rating_count, ct.created_at, ct.updated_at,
			r.name as repository_name, r.url as repository_url
		FROM catalog_templates ct
		JOIN repositories r ON ct.repository_id = r.id
		WHERE ct.id = $1
	`

	var id, repositoryID, installCount, viewCount, ratingCount int
	var name, displayName, description, category, appType, iconURL, manifest, version, repoName, repoURL string
	var tags pq.StringArray
	var isFeatured bool
	var avgRating float64
	var createdAt, updatedAt interface{}

	err := h.db.DB().QueryRowContext(c.Request.Context(), query, templateID).Scan(
		&id, &repositoryID, &name, &displayName, &description,
		&category, &appType, &iconURL, &manifest, &tags,
		&installCount, &isFeatured, &version, &viewCount,
		&avgRating, &ratingCount, &createdAt, &updatedAt, &repoName, &repoURL,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Template not found",
			Message: "The requested template does not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"id":           id,
		"repositoryId": repositoryID,
		"name":         name,
		"displayName":  displayName,
		"description":  description,
		"category":     category,
		"appType":      appType,
		"icon":         iconURL,
		"manifest":     manifest,
		"tags":         tags,
		"installCount": installCount,
		"isFeatured":   isFeatured,
		"version":      version,
		"viewCount":    viewCount,
		"avgRating":    avgRating,
		"ratingCount":  ratingCount,
		"createdAt":    createdAt,
		"updatedAt":    updatedAt,
		"repository": map[string]string{
			"name": repoName,
			"url":  repoURL,
		},
	})
}

// GetFeaturedTemplates godoc
// @Summary Get featured templates
// @Description Get curated featured templates
// @Tags catalog
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/catalog/templates/featured [get]
func (h *CatalogHandler) GetFeaturedTemplates(c *gin.Context) {
	c.Set("featured", "true")
	h.ListTemplates(c)
}

// GetTrendingTemplates godoc
// @Summary Get trending templates
// @Description Get templates with recent activity
// @Tags catalog
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/catalog/templates/trending [get]
func (h *CatalogHandler) GetTrendingTemplates(c *gin.Context) {
	c.Request.URL.RawQuery = "sort=recent&limit=12"
	h.ListTemplates(c)
}

// GetPopularTemplates godoc
// @Summary Get popular templates
// @Description Get most installed templates
// @Tags catalog
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/catalog/templates/popular [get]
func (h *CatalogHandler) GetPopularTemplates(c *gin.Context) {
	c.Request.URL.RawQuery = "sort=installs&limit=12"
	h.ListTemplates(c)
}

// AddRating godoc
// @Summary Add or update template rating
// @Description Rate a template with 1-5 stars and optional review
// @Tags catalog, ratings
// @Accept json
// @Produce json
// @Param id path int true "Template ID"
// @Param rating body object true "Rating data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/catalog/templates/{id}/ratings [post]
func (h *CatalogHandler) AddRating(c *gin.Context) {
	templateID := c.Param("id")
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	var req struct {
		Rating int    `json:"rating" binding:"required,min=1,max=5"`
		Review string `json:"review"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Insert or update rating
	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		INSERT INTO template_ratings (template_id, user_id, rating, review)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (template_id, user_id)
		DO UPDATE SET rating = $3, review = $4, updated_at = CURRENT_TIMESTAMP
	`, templateID, userID, req.Rating, req.Review)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Update template aggregated rating
	h.updateTemplateRating(c.Request.Context(), templateID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Rating submitted successfully",
	})
}

// GetRatings godoc
// @Summary Get template ratings
// @Description Get all ratings and reviews for a template
// @Tags catalog, ratings
// @Accept json
// @Produce json
// @Param id path int true "Template ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/catalog/templates/{id}/ratings [get]
func (h *CatalogHandler) GetRatings(c *gin.Context) {
	templateID := c.Param("id")

	rows, err := h.db.DB().QueryContext(c.Request.Context(), `
		SELECT
			tr.id, tr.user_id, tr.rating, tr.review, tr.created_at, tr.updated_at,
			u.username, u.full_name
		FROM template_ratings tr
		JOIN users u ON tr.user_id = u.id
		WHERE tr.template_id = $1
		ORDER BY tr.created_at DESC
	`, templateID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	ratings := []map[string]interface{}{}
	for rows.Next() {
		var id, rating int
		var userID, username, fullName string
		var review sql.NullString
		var createdAt, updatedAt interface{}

		if err := rows.Scan(&id, &userID, &rating, &review, &createdAt, &updatedAt, &username, &fullName); err != nil {
			continue
		}

		reviewText := ""
		if review.Valid {
			reviewText = review.String
		}

		ratings = append(ratings, map[string]interface{}{
			"id":        id,
			"userId":    userID,
			"username":  username,
			"fullName":  fullName,
			"rating":    rating,
			"review":    reviewText,
			"createdAt": createdAt,
			"updatedAt": updatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ratings": ratings,
		"total":   len(ratings),
	})
}

// UpdateRating updates a rating
func (h *CatalogHandler) UpdateRating(c *gin.Context) {
	// Similar to AddRating but with specific rating ID
	h.AddRating(c)
}

// DeleteRating deletes a rating
func (h *CatalogHandler) DeleteRating(c *gin.Context) {
	templateID := c.Param("id")
	ratingID := c.Param("ratingId")
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		DELETE FROM template_ratings
		WHERE id = $1 AND user_id = $2
	`, ratingID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	// Update template aggregated rating
	h.updateTemplateRating(c.Request.Context(), templateID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Rating deleted successfully",
	})
}

// RecordView records a template view
func (h *CatalogHandler) RecordView(c *gin.Context) {
	templateID := c.Param("id")

	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		UPDATE catalog_templates
		SET view_count = view_count + 1
		WHERE id = $1
	`, templateID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "View recorded",
	})
}

// RecordInstall records a template installation
func (h *CatalogHandler) RecordInstall(c *gin.Context) {
	templateID := c.Param("id")

	_, err := h.db.DB().ExecContext(c.Request.Context(), `
		UPDATE catalog_templates
		SET install_count = install_count + 1
		WHERE id = $1
	`, templateID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Database error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install recorded",
	})
}

// updateTemplateRating updates the aggregated rating for a template
func (h *CatalogHandler) updateTemplateRating(ctx interface{}, templateID string) {
	h.db.DB().ExecContext(ctx.(*gin.Context).Request.Context(), `
		UPDATE catalog_templates ct
		SET
			avg_rating = COALESCE((
				SELECT AVG(rating)::DECIMAL(3,2)
				FROM template_ratings
				WHERE template_id = ct.id
			), 0.0),
			rating_count = COALESCE((
				SELECT COUNT(*)
				FROM template_ratings
				WHERE template_id = ct.id
			), 0)
		WHERE ct.id = $1
	`, templateID)
}
