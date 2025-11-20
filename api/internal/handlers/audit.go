// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements audit log retrieval and export for compliance and security.
//
// AUDIT LOG VIEWER:
// - Retrieve audit logs with filtering and pagination
// - Export audit logs for compliance reports (CSV/JSON)
// - Search and analyze security events
// - Track user activity and system changes
//
// COMPLIANCE SUPPORT:
// - SOC2: Audit trail of all system changes (1 year retention)
// - HIPAA: PHI access logging (6 year retention)
// - GDPR: Data processing activity records
// - ISO 27001: User activity logging
//
// FILTERING CAPABILITIES:
// - Filter by user ID or username
// - Filter by action (GET, POST, PUT, DELETE)
// - Filter by resource type (/api/sessions, /api/users, etc.)
// - Filter by date range (start_date, end_date)
// - Filter by IP address (security investigations)
// - Filter by status code (200, 401, 500, etc.)
//
// EXPORT FORMATS:
// - JSON: Machine-readable, full details
// - CSV: Human-readable, spreadsheet-compatible
// - Both formats include all relevant fields for compliance
//
// USE CASES:
// - Security incident investigation
// - Compliance audits and reporting
// - User activity analysis
// - System change tracking
// - Failed access attempt detection
//
// API Endpoints:
// - GET /api/v1/admin/audit - List audit logs (with filters)
// - GET /api/v1/admin/audit/:id - Get specific audit log entry
// - GET /api/v1/admin/audit/export - Export audit logs to CSV/JSON
//
// Thread Safety:
// - Database operations are thread-safe
// - Read-only queries, no state modification
//
// Dependencies:
// - Database: PostgreSQL audit_log table
// - Middleware: Audit logging middleware (captures all requests)
//
// Example Usage:
//
//	handler := NewAuditHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1/admin"))
package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// AuditHandler handles audit log retrieval endpoints
type AuditHandler struct {
	database *db.Database
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(database *db.Database) *AuditHandler {
	return &AuditHandler{
		database: database,
	}
}

// RegisterRoutes registers audit log routes
func (h *AuditHandler) RegisterRoutes(router *gin.RouterGroup) {
	audit := router.Group("/audit")
	{
		audit.GET("", h.ListAuditLogs)
		audit.GET("/:id", h.GetAuditLog)
		audit.GET("/export", h.ExportAuditLogs)
	}
}

// AuditLog represents an audit log entry from the database
type AuditLog struct {
	ID           int64                  `json:"id"`
	UserID       string                 `json:"user_id,omitempty"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	Changes      map[string]interface{} `json:"changes,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	IPAddress    string                 `json:"ip_address"`
}

// AuditLogListResponse represents a paginated list of audit logs
type AuditLogListResponse struct {
	Logs       []AuditLog `json:"logs"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// ListAuditLogs godoc
// @Summary List audit logs with filtering and pagination
// @Description Retrieves audit logs with optional filters for compliance and security investigations
// @Tags admin, audit
// @Accept json
// @Produce json
// @Param user_id query string false "Filter by user ID"
// @Param username query string false "Filter by username (searches in changes JSONB)"
// @Param action query string false "Filter by action (GET, POST, PUT, DELETE, etc.)"
// @Param resource_type query string false "Filter by resource type (/api/sessions, etc.)"
// @Param resource_id query string false "Filter by specific resource ID"
// @Param ip_address query string false "Filter by IP address"
// @Param status_code query int false "Filter by HTTP status code"
// @Param start_date query string false "Filter from date (ISO 8601: 2025-01-01T00:00:00Z)"
// @Param end_date query string false "Filter to date (ISO 8601: 2025-12-31T23:59:59Z)"
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 100, max: 1000)"
// @Success 200 {object} AuditLogListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/audit [get]
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	// Build WHERE clauses based on filters
	var whereClauses []string
	var args []interface{}
	argCounter := 1

	// Filter by user_id
	if userID := c.Query("user_id"); userID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argCounter))
		args = append(args, userID)
		argCounter++
	}

	// Filter by username (search in changes JSONB)
	if username := c.Query("username"); username != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("changes->>'username' = $%d", argCounter))
		args = append(args, username)
		argCounter++
	}

	// Filter by action
	if action := c.Query("action"); action != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("action = $%d", argCounter))
		args = append(args, action)
		argCounter++
	}

	// Filter by resource_type
	if resourceType := c.Query("resource_type"); resourceType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_type = $%d", argCounter))
		args = append(args, resourceType)
		argCounter++
	}

	// Filter by resource_id
	if resourceID := c.Query("resource_id"); resourceID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_id = $%d", argCounter))
		args = append(args, resourceID)
		argCounter++
	}

	// Filter by ip_address
	if ipAddress := c.Query("ip_address"); ipAddress != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("ip_address = $%d", argCounter))
		args = append(args, ipAddress)
		argCounter++
	}

	// Filter by status_code (in changes JSONB)
	if statusCode := c.Query("status_code"); statusCode != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("changes->>'status_code' = $%d", argCounter))
		args = append(args, statusCode)
		argCounter++
	}

	// Filter by date range
	if startDate := c.Query("start_date"); startDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid start_date format",
				Message: "Use ISO 8601 format: 2025-01-01T00:00:00Z",
			})
			return
		}
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp >= $%d", argCounter))
		args = append(args, parsedDate)
		argCounter++
	}

	if endDate := c.Query("end_date"); endDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid end_date format",
				Message: "Use ISO 8601 format: 2025-12-31T23:59:59Z",
			})
			return
		}
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp <= $%d", argCounter))
		args = append(args, parsedDate)
		argCounter++
	}

	// Build WHERE clause
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_log %s", whereSQL)
	var total int64
	err := h.database.DB().QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to count audit logs",
			Message: err.Error(),
		})
		return
	}

	// Retrieve audit logs with pagination
	query := fmt.Sprintf(`
		SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address
		FROM audit_log
		%s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argCounter, argCounter+1)

	args = append(args, pageSize, offset)

	rows, err := h.database.DB().Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve audit logs",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var changesJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&changesJSON,
			&log.Timestamp,
			&log.IPAddress,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to scan audit log",
				Message: err.Error(),
			})
			return
		}

		// Parse changes JSONB
		if len(changesJSON) > 0 {
			json.Unmarshal(changesJSON, &log.Changes)
		}

		logs = append(logs, log)
	}

	if logs == nil {
		logs = []AuditLog{} // Return empty array instead of null
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	c.JSON(http.StatusOK, AuditLogListResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetAuditLog godoc
// @Summary Get specific audit log entry
// @Description Retrieves a single audit log entry by ID with full details
// @Tags admin, audit
// @Accept json
// @Produce json
// @Param id path int true "Audit Log ID"
// @Success 200 {object} AuditLog
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/audit/{id} [get]
func (h *AuditHandler) GetAuditLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid audit log ID",
			Message: "ID must be a valid integer",
		})
		return
	}

	query := `
		SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address
		FROM audit_log
		WHERE id = $1
	`

	var log AuditLog
	var changesJSON []byte

	err = h.database.DB().QueryRow(query, id).Scan(
		&log.ID,
		&log.UserID,
		&log.Action,
		&log.ResourceType,
		&log.ResourceID,
		&changesJSON,
		&log.Timestamp,
		&log.IPAddress,
	)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Audit log not found",
				Message: fmt.Sprintf("No audit log with ID %d", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve audit log",
			Message: err.Error(),
		})
		return
	}

	// Parse changes JSONB
	if len(changesJSON) > 0 {
		json.Unmarshal(changesJSON, &log.Changes)
	}

	c.JSON(http.StatusOK, log)
}

// ExportAuditLogs godoc
// @Summary Export audit logs to CSV or JSON
// @Description Exports filtered audit logs for compliance reports and analysis
// @Tags admin, audit
// @Accept json
// @Produce text/csv,application/json
// @Param format query string true "Export format: 'csv' or 'json'" Enums(csv, json)
// @Param user_id query string false "Filter by user ID"
// @Param action query string false "Filter by action"
// @Param resource_type query string false "Filter by resource type"
// @Param start_date query string false "Filter from date"
// @Param end_date query string false "Filter to date"
// @Param limit query int false "Maximum records to export (default: 10000, max: 100000)"
// @Success 200 {file} file "CSV or JSON file"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/audit/export [get]
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	if format != "csv" && format != "json" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid format",
			Message: "Format must be 'csv' or 'json'",
		})
		return
	}

	// Parse limit
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10000"))
	if limit < 1 || limit > 100000 {
		limit = 10000
	}

	// Build WHERE clauses (same as ListAuditLogs but without pagination)
	var whereClauses []string
	var args []interface{}
	argCounter := 1

	if userID := c.Query("user_id"); userID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argCounter))
		args = append(args, userID)
		argCounter++
	}

	if action := c.Query("action"); action != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("action = $%d", argCounter))
		args = append(args, action)
		argCounter++
	}

	if resourceType := c.Query("resource_type"); resourceType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_type = $%d", argCounter))
		args = append(args, resourceType)
		argCounter++
	}

	if startDate := c.Query("start_date"); startDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid start_date format",
				Message: "Use ISO 8601 format",
			})
			return
		}
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp >= $%d", argCounter))
		args = append(args, parsedDate)
		argCounter++
	}

	if endDate := c.Query("end_date"); endDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid end_date format",
				Message: "Use ISO 8601 format",
			})
			return
		}
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp <= $%d", argCounter))
		args = append(args, parsedDate)
		argCounter++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Retrieve audit logs
	query := fmt.Sprintf(`
		SELECT id, user_id, action, resource_type, resource_id, changes, timestamp, ip_address
		FROM audit_log
		%s
		ORDER BY timestamp DESC
		LIMIT $%d
	`, whereSQL, argCounter)

	args = append(args, limit)

	rows, err := h.database.DB().Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve audit logs",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var changesJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&changesJSON,
			&log.Timestamp,
			&log.IPAddress,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to scan audit log",
				Message: err.Error(),
			})
			return
		}

		if len(changesJSON) > 0 {
			json.Unmarshal(changesJSON, &log.Changes)
		}

		logs = append(logs, log)
	}

	if logs == nil {
		logs = []AuditLog{}
	}

	// Export based on format
	if format == "json" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=audit_logs_%s.json", time.Now().Format("20060102_150405")))
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, logs)
	} else {
		// CSV export
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=audit_logs_%s.csv", time.Now().Format("20060102_150405")))
		c.Header("Content-Type", "text/csv")

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Write CSV header
		header := []string{"ID", "Timestamp", "User ID", "Action", "Resource Type", "Resource ID", "IP Address", "Status Code", "Duration (ms)", "Error"}
		writer.Write(header)

		// Write data rows
		for _, log := range logs {
			statusCode := ""
			durationMS := ""
			errorMsg := ""

			if log.Changes != nil {
				if sc, ok := log.Changes["status_code"]; ok {
					statusCode = fmt.Sprintf("%v", sc)
				}
				if dm, ok := log.Changes["duration_ms"]; ok {
					durationMS = fmt.Sprintf("%v", dm)
				}
				if em, ok := log.Changes["error"]; ok {
					errorMsg = fmt.Sprintf("%v", em)
				}
			}

			row := []string{
				fmt.Sprintf("%d", log.ID),
				log.Timestamp.Format(time.RFC3339),
				log.UserID,
				log.Action,
				log.ResourceType,
				log.ResourceID,
				log.IPAddress,
				statusCode,
				durationMS,
				errorMsg,
			}
			writer.Write(row)
		}
	}
}
