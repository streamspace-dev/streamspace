package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Workflow represents an automated workflow
type Workflow struct {
	ID            int64                  `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Trigger       WorkflowTrigger        `json:"trigger"`
	Steps         []WorkflowStep         `json:"steps"`
	Enabled       bool                   `json:"enabled"`
	ExecutionMode string                 `json:"execution_mode"` // "sequential", "parallel"
	TimeoutMinutes int                   `json:"timeout_minutes"`
	RetryPolicy   WorkflowRetryPolicy    `json:"retry_policy"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// WorkflowTrigger defines when a workflow should execute
type WorkflowTrigger struct {
	Type       string                 `json:"type"` // "manual", "schedule", "event", "webhook"
	Schedule   string                 `json:"schedule,omitempty"` // cron expression
	EventType  string                 `json:"event_type,omitempty"` // "session_created", "session_terminated", etc.
	Conditions []WorkflowCondition    `json:"conditions,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"` // "create_session", "update_session", "send_notification", "wait", "condition", "http_request", "run_script"
	Action       string                 `json:"action"`
	Parameters   map[string]interface{} `json:"parameters"`
	Conditions   []WorkflowCondition    `json:"conditions,omitempty"`
	OnSuccess    string                 `json:"on_success,omitempty"` // Next step ID
	OnFailure    string                 `json:"on_failure,omitempty"` // Next step ID
	RetryCount   int                    `json:"retry_count"`
	TimeoutSeconds int                  `json:"timeout_seconds"`
}

// WorkflowCondition defines a conditional check
type WorkflowCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // "eq", "ne", "gt", "lt", "contains", "matches"
	Value    interface{} `json:"value"`
	LogicOp  string      `json:"logic_op,omitempty"` // "AND", "OR"
}

// WorkflowRetryPolicy defines how to handle failures
type WorkflowRetryPolicy struct {
	MaxRetries     int    `json:"max_retries"`
	RetryDelaySeconds int `json:"retry_delay_seconds"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`
}

// WorkflowExecution represents a workflow execution instance
type WorkflowExecution struct {
	ID              int64                  `json:"id"`
	WorkflowID      int64                  `json:"workflow_id"`
	WorkflowName    string                 `json:"workflow_name"`
	Status          string                 `json:"status"` // "pending", "running", "completed", "failed", "cancelled"
	CurrentStep     string                 `json:"current_step,omitempty"`
	StepResults     []StepResult           `json:"step_results"`
	TriggerData     map[string]interface{} `json:"trigger_data,omitempty"`
	Context         map[string]interface{} `json:"context,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	Duration        int                    `json:"duration"` // in seconds
	TriggeredBy     string                 `json:"triggered_by,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// StepResult represents the result of a workflow step execution
type StepResult struct {
	StepID      string                 `json:"step_id"`
	StepName    string                 `json:"step_name"`
	Status      string                 `json:"status"` // "pending", "running", "completed", "failed", "skipped"
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    int                    `json:"duration"` // in seconds
	RetryCount  int                    `json:"retry_count"`
}

// CreateWorkflow creates a new workflow
func (h *Handler) CreateWorkflow(c *gin.Context) {
	var workflow Workflow
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	workflow.CreatedBy = userID

	err := h.DB.QueryRow(`
		INSERT INTO workflows (
			name, description, trigger, steps, enabled, execution_mode,
			timeout_minutes, retry_policy, metadata, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, workflow.Name, workflow.Description, toJSONB(workflow.Trigger), toJSONB(workflow.Steps),
		workflow.Enabled, workflow.ExecutionMode, workflow.TimeoutMinutes,
		toJSONB(workflow.RetryPolicy), toJSONB(workflow.Metadata), userID).Scan(&workflow.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workflow"})
		return
	}

	c.JSON(http.StatusCreated, workflow)
}

// ListWorkflows lists all workflows
func (h *Handler) ListWorkflows(c *gin.Context) {
	enabled := c.Query("enabled")
	triggerType := c.Query("trigger_type")

	query := `
		SELECT id, name, description, trigger, steps, enabled, execution_mode,
		       timeout_minutes, retry_policy, metadata, created_by, created_at, updated_at
		FROM workflows WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if enabled != "" {
		query += fmt.Sprintf(" AND enabled = $%d", argCount)
		args = append(args, enabled == "true")
		argCount++
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve workflows"})
		return
	}
	defer rows.Close()

	workflows := []Workflow{}
	for rows.Next() {
		var w Workflow
		var trigger, steps, retryPolicy, metadata sql.NullString
		err := rows.Scan(&w.ID, &w.Name, &w.Description, &trigger, &steps,
			&w.Enabled, &w.ExecutionMode, &w.TimeoutMinutes, &retryPolicy,
			&metadata, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt)

		if err == nil {
			if trigger.Valid && trigger.String != "" {
				json.Unmarshal([]byte(trigger.String), &w.Trigger)
			}
			if steps.Valid && steps.String != "" {
				json.Unmarshal([]byte(steps.String), &w.Steps)
			}
			if retryPolicy.Valid && retryPolicy.String != "" {
				json.Unmarshal([]byte(retryPolicy.String), &w.RetryPolicy)
			}
			if metadata.Valid && metadata.String != "" {
				json.Unmarshal([]byte(metadata.String), &w.Metadata)
			}

			// Filter by trigger type if specified
			if triggerType == "" || w.Trigger.Type == triggerType {
				workflows = append(workflows, w)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"workflows": workflows})
}

// GetWorkflow retrieves a specific workflow
func (h *Handler) GetWorkflow(c *gin.Context) {
	workflowID, err := strconv.ParseInt(c.Param("workflowId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow ID"})
		return
	}

	var w Workflow
	var trigger, steps, retryPolicy, metadata sql.NullString

	err = h.DB.QueryRow(`
		SELECT id, name, description, trigger, steps, enabled, execution_mode,
		       timeout_minutes, retry_policy, metadata, created_by, created_at, updated_at
		FROM workflows WHERE id = $1
	`, workflowID).Scan(&w.ID, &w.Name, &w.Description, &trigger, &steps,
		&w.Enabled, &w.ExecutionMode, &w.TimeoutMinutes, &retryPolicy,
		&metadata, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve workflow"})
		return
	}

	if trigger.Valid && trigger.String != "" {
		json.Unmarshal([]byte(trigger.String), &w.Trigger)
	}
	if steps.Valid && steps.String != "" {
		json.Unmarshal([]byte(steps.String), &w.Steps)
	}
	if retryPolicy.Valid && retryPolicy.String != "" {
		json.Unmarshal([]byte(retryPolicy.String), &w.RetryPolicy)
	}
	if metadata.Valid && metadata.String != "" {
		json.Unmarshal([]byte(metadata.String), &w.Metadata)
	}

	c.JSON(http.StatusOK, w)
}

// UpdateWorkflow updates an existing workflow
func (h *Handler) UpdateWorkflow(c *gin.Context) {
	workflowID, err := strconv.ParseInt(c.Param("workflowId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow ID"})
		return
	}

	var workflow Workflow
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE workflows SET
			name = $1, description = $2, trigger = $3, steps = $4,
			enabled = $5, execution_mode = $6, timeout_minutes = $7,
			retry_policy = $8, metadata = $9, updated_at = $10
		WHERE id = $11
	`, workflow.Name, workflow.Description, toJSONB(workflow.Trigger), toJSONB(workflow.Steps),
		workflow.Enabled, workflow.ExecutionMode, workflow.TimeoutMinutes,
		toJSONB(workflow.RetryPolicy), toJSONB(workflow.Metadata), time.Now(), workflowID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "workflow updated successfully"})
}

// DeleteWorkflow deletes a workflow
func (h *Handler) DeleteWorkflow(c *gin.Context) {
	workflowID, err := strconv.ParseInt(c.Param("workflowId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow ID"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM workflows WHERE id = $1", workflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "workflow deleted successfully"})
}

// ExecuteWorkflow triggers a workflow execution
func (h *Handler) ExecuteWorkflow(c *gin.Context) {
	workflowID, err := strconv.ParseInt(c.Param("workflowId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow ID"})
		return
	}

	userID := c.GetString("user_id")

	var req struct {
		TriggerData map[string]interface{} `json:"trigger_data"`
		Context     map[string]interface{} `json:"context"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for manual triggers
		req.TriggerData = make(map[string]interface{})
		req.Context = make(map[string]interface{})
	}

	// Get workflow details
	var workflowName string
	var enabled bool
	err = h.DB.QueryRow(`
		SELECT name, enabled FROM workflows WHERE id = $1
	`, workflowID).Scan(&workflowName, &enabled)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	if !enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow is disabled"})
		return
	}

	// Create execution record
	var executionID int64
	err = h.DB.QueryRow(`
		INSERT INTO workflow_executions (
			workflow_id, workflow_name, status, trigger_data, context, triggered_by
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, workflowID, workflowName, "pending", toJSONB(req.TriggerData), toJSONB(req.Context), userID).Scan(&executionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create execution"})
		return
	}

	// Queue workflow execution for background worker processing
	_, err = h.DB.Exec(`
		INSERT INTO workflow_execution_queue (execution_id, workflow_id, priority, status, created_at)
		VALUES ($1, $2, $3, 'queued', NOW())
	`, executionID, workflowID, 5) // Default priority of 5

	if err != nil {
		// Log error but don't fail - execution record is created
		fmt.Printf("Failed to queue workflow execution %d: %v\n", executionID, err)
		// Update execution status to indicate queuing failure
		h.DB.Exec(`UPDATE workflow_executions SET status = 'failed', error_message = $1 WHERE id = $2`,
			"Failed to queue for execution", executionID)
	}

	c.JSON(http.StatusCreated, gin.H{
		"execution_id": executionID,
		"status":       "pending",
		"message":      "workflow execution queued",
	})
}

// ListWorkflowExecutions lists workflow executions
func (h *Handler) ListWorkflowExecutions(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	query := `
		SELECT id, workflow_id, workflow_name, status, current_step, step_results,
		       trigger_data, context, error_message, started_at, completed_at,
		       duration, triggered_by, created_at
		FROM workflow_executions WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if workflowID != "" {
		wid, err := strconv.ParseInt(workflowID, 10, 64)
		if err == nil {
			query += fmt.Sprintf(" AND workflow_id = $%d", argCount)
			args = append(args, wid)
			argCount++
		}
	}

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	// Count total
	countQuery := strings.Replace(query, "SELECT id, workflow_id, workflow_name, status, current_step, step_results, trigger_data, context, error_message, started_at, completed_at, duration, triggered_by, created_at", "SELECT COUNT(*)", 1)
	var total int
	h.DB.QueryRow(countQuery, args...).Scan(&total)

	// Add pagination
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve executions"})
		return
	}
	defer rows.Close()

	executions := []WorkflowExecution{}
	for rows.Next() {
		var e WorkflowExecution
		var stepResults, triggerData, ctx sql.NullString
		err := rows.Scan(&e.ID, &e.WorkflowID, &e.WorkflowName, &e.Status, &e.CurrentStep,
			&stepResults, &triggerData, &ctx, &e.ErrorMessage, &e.StartedAt,
			&e.CompletedAt, &e.Duration, &e.TriggeredBy, &e.CreatedAt)

		if err == nil {
			if stepResults.Valid && stepResults.String != "" {
				json.Unmarshal([]byte(stepResults.String), &e.StepResults)
			}
			if triggerData.Valid && triggerData.String != "" {
				json.Unmarshal([]byte(triggerData.String), &e.TriggerData)
			}
			if ctx.Valid && ctx.String != "" {
				json.Unmarshal([]byte(ctx.String), &e.Context)
			}
			executions = append(executions, e)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"executions":  executions,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// GetWorkflowExecution retrieves a specific execution
func (h *Handler) GetWorkflowExecution(c *gin.Context) {
	executionID, err := strconv.ParseInt(c.Param("executionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution ID"})
		return
	}

	var e WorkflowExecution
	var stepResults, triggerData, ctx sql.NullString

	err = h.DB.QueryRow(`
		SELECT id, workflow_id, workflow_name, status, current_step, step_results,
		       trigger_data, context, error_message, started_at, completed_at,
		       duration, triggered_by, created_at
		FROM workflow_executions WHERE id = $1
	`, executionID).Scan(&e.ID, &e.WorkflowID, &e.WorkflowName, &e.Status, &e.CurrentStep,
		&stepResults, &triggerData, &ctx, &e.ErrorMessage, &e.StartedAt,
		&e.CompletedAt, &e.Duration, &e.TriggeredBy, &e.CreatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve execution"})
		return
	}

	if stepResults.Valid && stepResults.String != "" {
		json.Unmarshal([]byte(stepResults.String), &e.StepResults)
	}
	if triggerData.Valid && triggerData.String != "" {
		json.Unmarshal([]byte(triggerData.String), &e.TriggerData)
	}
	if ctx.Valid && ctx.String != "" {
		json.Unmarshal([]byte(ctx.String), &e.Context)
	}

	c.JSON(http.StatusOK, e)
}

// CancelWorkflowExecution cancels a running workflow execution
func (h *Handler) CancelWorkflowExecution(c *gin.Context) {
	executionID, err := strconv.ParseInt(c.Param("executionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution ID"})
		return
	}

	// Check if execution is running
	var status string
	err = h.DB.QueryRow("SELECT status FROM workflow_executions WHERE id = $1", executionID).Scan(&status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}

	if status != "pending" && status != "running" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution is not active"})
		return
	}

	// Update status to cancelled
	completedAt := time.Now()
	_, err = h.DB.Exec(`
		UPDATE workflow_executions
		SET status = 'cancelled', completed_at = $1
		WHERE id = $2
	`, completedAt, executionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel execution"})
		return
	}

	// Signal the execution worker to stop processing
	_, err = h.DB.Exec(`
		UPDATE workflow_execution_queue
		SET status = 'cancelled', updated_at = NOW()
		WHERE execution_id = $1 AND status IN ('queued', 'processing')
	`, executionID)

	if err != nil {
		fmt.Printf("Failed to signal workflow cancellation for execution %d: %v\n", executionID, err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "execution cancelled successfully"})
}

// GetWorkflowStats returns workflow statistics
func (h *Handler) GetWorkflowStats(c *gin.Context) {
	var stats struct {
		TotalWorkflows    int64            `json:"total_workflows"`
		EnabledWorkflows  int64            `json:"enabled_workflows"`
		TotalExecutions   int64            `json:"total_executions"`
		ExecutionsToday   int64            `json:"executions_today"`
		ExecutionsByStatus map[string]int64 `json:"executions_by_status"`
		SuccessRate       float64          `json:"success_rate"`
		AvgDuration       float64          `json:"avg_duration_seconds"`
	}

	h.DB.QueryRow("SELECT COUNT(*) FROM workflows").Scan(&stats.TotalWorkflows)
	h.DB.QueryRow("SELECT COUNT(*) FROM workflows WHERE enabled = true").Scan(&stats.EnabledWorkflows)
	h.DB.QueryRow("SELECT COUNT(*) FROM workflow_executions").Scan(&stats.TotalExecutions)
	h.DB.QueryRow(`
		SELECT COUNT(*) FROM workflow_executions
		WHERE started_at > CURRENT_DATE
	`).Scan(&stats.ExecutionsToday)

	// Executions by status
	stats.ExecutionsByStatus = make(map[string]int64)
	rows, _ := h.DB.Query("SELECT status, COUNT(*) FROM workflow_executions GROUP BY status")
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		rows.Scan(&status, &count)
		stats.ExecutionsByStatus[status] = count
	}

	// Success rate and avg duration
	var completed, successful int64
	h.DB.QueryRow("SELECT COUNT(*) FROM workflow_executions WHERE status IN ('completed', 'failed')").Scan(&completed)
	h.DB.QueryRow("SELECT COUNT(*) FROM workflow_executions WHERE status = 'completed'").Scan(&successful)

	if completed > 0 {
		stats.SuccessRate = float64(successful) / float64(completed) * 100
	}

	h.DB.QueryRow("SELECT COALESCE(AVG(duration), 0) FROM workflow_executions WHERE status = 'completed'").Scan(&stats.AvgDuration)

	c.JSON(http.StatusOK, stats)
}
