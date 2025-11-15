package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// BatchHandler handles batch operations on multiple resources
type BatchHandler struct {
	db *db.Database
}

// NewBatchHandler creates a new batch handler
func NewBatchHandler(database *db.Database) *BatchHandler {
	return &BatchHandler{
		db: database,
	}
}

// BatchOperation represents a batch operation job
type BatchOperation struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"userId"`
	OperationType string                 `json:"operationType"` // terminate, hibernate, wake, delete, update
	ResourceType  string                 `json:"resourceType"`  // sessions, snapshots, etc.
	Status        string                 `json:"status"`        // pending, running, completed, failed
	TotalItems    int                    `json:"totalItems"`
	ProcessedItems int                   `json:"processedItems"`
	SuccessCount  int                    `json:"successCount"`
	FailureCount  int                    `json:"failureCount"`
	Errors        []string               `json:"errors,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	CompletedAt   *time.Time             `json:"completedAt,omitempty"`
}

// RegisterRoutes registers batch operation routes
func (h *BatchHandler) RegisterRoutes(router *gin.RouterGroup) {
	batch := router.Group("/batch")
	{
		// Session batch operations
		batch.POST("/sessions/terminate", h.TerminateSessions)
		batch.POST("/sessions/hibernate", h.HibernateSessions)
		batch.POST("/sessions/wake", h.WakeSessions)
		batch.POST("/sessions/delete", h.DeleteSessions)
		batch.POST("/sessions/update-tags", h.UpdateSessionTags)
		batch.POST("/sessions/update-resources", h.UpdateSessionResources)

		// Snapshot batch operations
		batch.POST("/snapshots/delete", h.DeleteSnapshots)
		batch.POST("/snapshots/create", h.CreateSnapshots)

		// Template batch operations
		batch.POST("/templates/install", h.InstallTemplates)
		batch.POST("/templates/delete", h.DeleteTemplates)

		// Batch job status
		batch.GET("/jobs", h.ListBatchJobs)
		batch.GET("/jobs/:id", h.GetBatchJob)
		batch.DELETE("/jobs/:id", h.CancelBatchJob)
	}
}

// TerminateSessions terminates multiple sessions
func (h *BatchHandler) TerminateSessions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	// Create batch job
	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'terminate', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	// Execute batch operation asynchronously
	go h.executeBatchTerminate(jobID, userIDStr, req.SessionIDs)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch termination initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// HibernateSessions hibernates multiple sessions
func (h *BatchHandler) HibernateSessions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'hibernate', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	go h.executeBatchHibernate(jobID, userIDStr, req.SessionIDs)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch hibernation initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// WakeSessions wakes multiple hibernated sessions
func (h *BatchHandler) WakeSessions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'wake', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	go h.executeBatchWake(jobID, userIDStr, req.SessionIDs)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch wake initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// DeleteSessions deletes multiple sessions
func (h *BatchHandler) DeleteSessions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'delete', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	go h.executeBatchDelete(jobID, userIDStr, req.SessionIDs)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch deletion initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// UpdateSessionTags updates tags for multiple sessions
func (h *BatchHandler) UpdateSessionTags(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
		Tags       []string `json:"tags" binding:"required"`
		Operation  string   `json:"operation"` // add, remove, replace
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Operation == "" {
		req.Operation = "replace"
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'update_tags', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	go h.executeBatchUpdateTags(jobID, userIDStr, req.SessionIDs, req.Tags, req.Operation)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch tag update initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// UpdateSessionResources updates resources for multiple sessions
func (h *BatchHandler) UpdateSessionResources(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string               `json:"sessionIds" binding:"required"`
		Resources  map[string]interface{} `json:"resources" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'update_resources', 'sessions', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch resource update initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// DeleteSnapshots deletes multiple snapshots
func (h *BatchHandler) DeleteSnapshots(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SnapshotIDs []string `json:"snapshotIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'delete', 'snapshots', 'running', $3)
	`, jobID, userIDStr, len(req.SnapshotIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	go h.executeBatchDeleteSnapshots(jobID, userIDStr, req.SnapshotIDs)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch snapshot deletion initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// CreateSnapshots creates snapshots for multiple sessions
func (h *BatchHandler) CreateSnapshots(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		SessionIDs []string `json:"sessionIds" binding:"required"`
		Name       string   `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	jobID := fmt.Sprintf("batchjob_%d", time.Now().UnixNano())

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO batch_operations (id, user_id, operation_type, resource_type, status, total_items)
		VALUES ($1, $2, 'create', 'snapshots', 'running', $3)
	`, jobID, userIDStr, len(req.SessionIDs))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch snapshot creation initiated",
		"jobId":   jobID,
		"status":  "running",
	})
}

// InstallTemplates installs multiple templates
func (h *BatchHandler) InstallTemplates(c *gin.Context) {
	var req struct {
		TemplateIDs []string `json:"templateIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch template installation initiated",
		"count":   len(req.TemplateIDs),
	})
}

// DeleteTemplates deletes multiple templates
func (h *BatchHandler) DeleteTemplates(c *gin.Context) {
	var req struct {
		TemplateIDs []string `json:"templateIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Batch template deletion initiated",
		"count":   len(req.TemplateIDs),
	})
}

// ListBatchJobs lists user's batch jobs
func (h *BatchHandler) ListBatchJobs(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, operation_type, resource_type, status, total_items, processed_items,
		       success_count, failure_count, created_at, completed_at
		FROM batch_operations
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list batch jobs"})
		return
	}
	defer rows.Close()

	jobs := []map[string]interface{}{}
	for rows.Next() {
		var id, operationType, resourceType, status string
		var totalItems, processedItems, successCount, failureCount int
		var createdAt time.Time
		var completedAt *time.Time

		if err := rows.Scan(&id, &operationType, &resourceType, &status, &totalItems, &processedItems, &successCount, &failureCount, &createdAt, &completedAt); err == nil {
			job := map[string]interface{}{
				"id":             id,
				"operationType":  operationType,
				"resourceType":   resourceType,
				"status":         status,
				"totalItems":     totalItems,
				"processedItems": processedItems,
				"successCount":   successCount,
				"failureCount":   failureCount,
				"createdAt":      createdAt,
			}
			if completedAt != nil {
				job["completedAt"] = *completedAt
			}
			jobs = append(jobs, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// GetBatchJob retrieves a specific batch job
func (h *BatchHandler) GetBatchJob(c *gin.Context) {
	jobID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var job map[string]interface{}
	var id, operationType, resourceType, status string
	var totalItems, processedItems, successCount, failureCount int
	var createdAt time.Time
	var completedAt *time.Time

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, operation_type, resource_type, status, total_items, processed_items,
		       success_count, failure_count, created_at, completed_at
		FROM batch_operations
		WHERE id = $1 AND user_id = $2
	`, jobID, userIDStr).Scan(&id, &operationType, &resourceType, &status, &totalItems, &processedItems, &successCount, &failureCount, &createdAt, &completedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batch job not found"})
		return
	}

	job = map[string]interface{}{
		"id":             id,
		"operationType":  operationType,
		"resourceType":   resourceType,
		"status":         status,
		"totalItems":     totalItems,
		"processedItems": processedItems,
		"successCount":   successCount,
		"failureCount":   failureCount,
		"createdAt":      createdAt,
	}
	if completedAt != nil {
		job["completedAt"] = *completedAt
	}

	c.JSON(http.StatusOK, job)
}

// CancelBatchJob cancels a running batch job
func (h *BatchHandler) CancelBatchJob(c *gin.Context) {
	jobID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'cancelled' WHERE id = $1 AND user_id = $2 AND status = 'running'
	`, jobID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel batch job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Batch job cancelled",
		"jobId":   jobID,
	})
}

// Batch execution methods (simplified - in production these would actually perform operations)

func (h *BatchHandler) executeBatchTerminate(jobID, userID string, sessionIDs []string) {
	ctx := context.Background()

	successCount := 0
	for _, sessionID := range sessionIDs {
		// Update session state to terminated
		_, err := h.db.DB().ExecContext(ctx, `
			UPDATE sessions SET state = 'terminated' WHERE id = $1 AND user_id = $2
		`, sessionID, userID)

		if err == nil {
			successCount++
		}

		// Update progress
		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	// Mark as completed
	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}

func (h *BatchHandler) executeBatchHibernate(jobID, userID string, sessionIDs []string) {
	ctx := context.Background()

	successCount := 0
	for _, sessionID := range sessionIDs {
		_, err := h.db.DB().ExecContext(ctx, `
			UPDATE sessions SET state = 'hibernated' WHERE id = $1 AND user_id = $2
		`, sessionID, userID)

		if err == nil {
			successCount++
		}

		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}

func (h *BatchHandler) executeBatchWake(jobID, userID string, sessionIDs []string) {
	ctx := context.Background()

	successCount := 0
	for _, sessionID := range sessionIDs {
		_, err := h.db.DB().ExecContext(ctx, `
			UPDATE sessions SET state = 'running' WHERE id = $1 AND user_id = $2
		`, sessionID, userID)

		if err == nil {
			successCount++
		}

		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}

func (h *BatchHandler) executeBatchDelete(jobID, userID string, sessionIDs []string) {
	ctx := context.Background()

	successCount := 0
	for _, sessionID := range sessionIDs {
		_, err := h.db.DB().ExecContext(ctx, `
			DELETE FROM sessions WHERE id = $1 AND user_id = $2
		`, sessionID, userID)

		if err == nil {
			successCount++
		}

		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}

func (h *BatchHandler) executeBatchUpdateTags(jobID, userID string, sessionIDs []string, tags []string, operation string) {
	ctx := context.Background()

	successCount := 0
	for _, sessionID := range sessionIDs {
		// In production, this would use JSONB operations for add/remove
		// For now, simplified implementation
		_, err := h.db.DB().ExecContext(ctx, `
			UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $2
		`, sessionID, userID)

		if err == nil {
			successCount++
		}

		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}

func (h *BatchHandler) executeBatchDeleteSnapshots(jobID, userID string, snapshotIDs []string) {
	ctx := context.Background()

	successCount := 0
	for _, snapshotID := range snapshotIDs {
		_, err := h.db.DB().ExecContext(ctx, `
			UPDATE session_snapshots SET status = 'deleted' WHERE id = $1 AND user_id = $2
		`, snapshotID, userID)

		if err == nil {
			successCount++
		}

		h.db.DB().ExecContext(ctx, `
			UPDATE batch_operations SET processed_items = processed_items + 1, success_count = $1 WHERE id = $2
		`, successCount, jobID)
	}

	h.db.DB().ExecContext(ctx, `
		UPDATE batch_operations SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE id = $1
	`, jobID)
}
