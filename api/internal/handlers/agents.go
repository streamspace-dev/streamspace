// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements agent registration and management for the v2.0 multi-platform architecture.
//
// AGENT ARCHITECTURE:
// - Agents are platform-specific execution agents (Kubernetes, Docker, VM, Cloud)
// - Agents connect to Control Plane via outbound WebSocket connection
// - Agents receive commands to start/stop/hibernate sessions
// - Agents tunnel VNC traffic back to Control Plane
// - Agents send heartbeats every 10 seconds
//
// AGENT PLATFORMS:
// - kubernetes: Kubernetes cluster agent
// - docker: Docker host agent
// - vm: Virtual machine agent
// - cloud: Cloud provider agent (AWS, Azure, GCP)
//
// AGENT STATUS:
// - online: Agent is connected and healthy
// - offline: Agent is disconnected
// - draining: Agent is not accepting new sessions
//
// API Endpoints:
// - POST   /api/v1/agents/register - Register agent (or re-register)
// - GET    /api/v1/agents - List all agents (with filters)
// - GET    /api/v1/agents/:agent_id - Get agent details
// - DELETE /api/v1/agents/:agent_id - Deregister agent
// - POST   /api/v1/agents/:agent_id/heartbeat - Update heartbeat
// - POST   /api/v1/agents/:agent_id/command - Send command to agent
//
// Thread Safety:
// - All database operations are thread-safe
//
// Dependencies:
// - Database: agents table (v2.0 schema)
// - Models: api/internal/models/agent.go
// - AgentHub: WebSocket connection manager
// - CommandDispatcher: Command queuing and dispatch
//
// Example Usage:
//
//	handler := NewAgentHandler(database)
//	handler.RegisterRoutes(router.Group("/api/v1"))
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/services"
	"github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// AgentHandler handles agent registration and management
type AgentHandler struct {
	database   *db.Database
	hub        *websocket.AgentHub
	dispatcher *services.CommandDispatcher
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(database *db.Database, hub *websocket.AgentHub, dispatcher *services.CommandDispatcher) *AgentHandler {
	return &AgentHandler{
		database:   database,
		hub:        hub,
		dispatcher: dispatcher,
	}
}

// RegisterRoutes registers agent routes
func (h *AgentHandler) RegisterRoutes(router *gin.RouterGroup) {
	agents := router.Group("/agents")
	{
		agents.POST("/register", h.RegisterAgent)
		agents.GET("", h.ListAgents)
		agents.GET("/:agent_id", h.GetAgent)
		agents.DELETE("/:agent_id", h.DeregisterAgent)
		agents.POST("/:agent_id/heartbeat", h.UpdateHeartbeat)
		agents.POST("/:agent_id/command", h.SendCommand)
	}
}

// RegisterAgent godoc
// @Summary Register an agent with the Control Plane
// @Description Registers a new agent or re-registers an existing agent. Agents use this endpoint when they first connect or reconnect.
// @Tags agents
// @Accept json
// @Produce json
// @Param request body models.AgentRegistrationRequest true "Agent registration request"
// @Success 201 {object} models.Agent "Agent registered successfully (new)"
// @Success 200 {object} models.Agent "Agent re-registered successfully (existing)"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents/register [post]
func (h *AgentHandler) RegisterAgent(c *gin.Context) {
	var req models.AgentRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate platform
	validPlatforms := map[string]bool{
		"kubernetes": true,
		"docker":     true,
		"vm":         true,
		"cloud":      true,
	}
	if !validPlatforms[req.Platform] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid platform",
			"details": fmt.Sprintf("Platform must be one of: kubernetes, docker, vm, cloud. Got: %s", req.Platform),
		})
		return
	}

	// Check if agent already exists
	var existingID string
	err := h.database.DB().QueryRow(
		"SELECT id FROM agents WHERE agent_id = $1",
		req.AgentID,
	).Scan(&existingID)

	now := time.Now()
	var agent models.Agent
	statusCode := http.StatusCreated

	if err == sql.ErrNoRows {
		// Agent doesn't exist - create new
		err = h.database.DB().QueryRow(`
			INSERT INTO agents (agent_id, platform, region, status, capacity, last_heartbeat, metadata, created_at, updated_at)
			VALUES ($1, $2, $3, 'online', $4, $5, $6, $7, $7)
			RETURNING id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at
		`, req.AgentID, req.Platform, req.Region, req.Capacity, now, req.Metadata, now).Scan(
			&agent.ID,
			&agent.AgentID,
			&agent.Platform,
			&agent.Region,
			&agent.Status,
			&agent.Capacity,
			&agent.LastHeartbeat,
			&agent.WebSocketID,
			&agent.Metadata,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to register agent",
				"details": err.Error(),
			})
			return
		}
	} else if err != nil {
		// Database error
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check existing agent",
			"details": err.Error(),
		})
		return
	} else {
		// Agent exists - update (re-registration)
		statusCode = http.StatusOK
		err = h.database.DB().QueryRow(`
			UPDATE agents
			SET platform = $2, region = $3, status = 'online', capacity = $4, last_heartbeat = $5, metadata = $6, updated_at = $5
			WHERE agent_id = $1
			RETURNING id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at
		`, req.AgentID, req.Platform, req.Region, req.Capacity, now, req.Metadata).Scan(
			&agent.ID,
			&agent.AgentID,
			&agent.Platform,
			&agent.Region,
			&agent.Status,
			&agent.Capacity,
			&agent.LastHeartbeat,
			&agent.WebSocketID,
			&agent.Metadata,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to re-register agent",
				"details": err.Error(),
			})
			return
		}
	}

	c.JSON(statusCode, agent)
}

// ListAgents godoc
// @Summary List all agents
// @Description Retrieves all registered agents with optional filters
// @Tags agents
// @Accept json
// @Produce json
// @Param platform query string false "Filter by platform (kubernetes, docker, vm, cloud)"
// @Param status query string false "Filter by status (online, offline, draining)"
// @Param region query string false "Filter by region"
// @Success 200 {object} map[string]interface{} "List of agents"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents [get]
func (h *AgentHandler) ListAgents(c *gin.Context) {
	// Get query parameters
	platform := c.Query("platform")
	status := c.Query("status")
	region := c.Query("region")

	// Build query
	query := "SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at FROM agents WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if platform != "" {
		query += fmt.Sprintf(" AND platform = $%d", argIdx)
		args = append(args, platform)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if region != "" {
		query += fmt.Sprintf(" AND region = $%d", argIdx)
		args = append(args, region)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.database.DB().Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list agents",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var agent models.Agent
		err := rows.Scan(
			&agent.ID,
			&agent.AgentID,
			&agent.Platform,
			&agent.Region,
			&agent.Status,
			&agent.Capacity,
			&agent.LastHeartbeat,
			&agent.WebSocketID,
			&agent.Metadata,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to scan agent",
				"details": err.Error(),
			})
			return
		}
		agents = append(agents, agent)
	}

	if agents == nil {
		agents = []models.Agent{}
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}

// GetAgent godoc
// @Summary Get agent details
// @Description Retrieves details for a specific agent by agent_id
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Success 200 {object} models.Agent "Agent details"
// @Failure 404 {object} map[string]interface{} "Agent not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents/{agent_id} [get]
func (h *AgentHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("agent_id")

	var agent models.Agent
	err := h.database.DB().QueryRow(`
		SELECT id, agent_id, platform, region, status, capacity, last_heartbeat, websocket_id, metadata, created_at, updated_at
		FROM agents
		WHERE agent_id = $1
	`, agentID).Scan(
		&agent.ID,
		&agent.AgentID,
		&agent.Platform,
		&agent.Region,
		&agent.Status,
		&agent.Capacity,
		&agent.LastHeartbeat,
		&agent.WebSocketID,
		&agent.Metadata,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Agent not found",
			"agentId": agentID,
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get agent",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// DeregisterAgent godoc
// @Summary Deregister an agent
// @Description Removes an agent from the Control Plane. CASCADE will delete related commands.
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Success 200 {object} map[string]interface{} "Agent deregistered successfully"
// @Failure 404 {object} map[string]interface{} "Agent not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents/{agent_id} [delete]
func (h *AgentHandler) DeregisterAgent(c *gin.Context) {
	agentID := c.Param("agent_id")

	result, err := h.database.DB().Exec("DELETE FROM agents WHERE agent_id = $1", agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to deregister agent",
			"details": err.Error(),
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check deregistration result",
			"details": err.Error(),
		})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Agent not found",
			"agentId": agentID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Agent deregistered successfully",
		"agentId": agentID,
	})
}

// UpdateHeartbeat godoc
// @Summary Update agent heartbeat
// @Description Updates the last heartbeat timestamp and optionally the status and capacity
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body models.AgentHeartbeatRequest true "Heartbeat request"
// @Success 200 {object} map[string]interface{} "Heartbeat updated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Agent not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents/{agent_id}/heartbeat [post]
func (h *AgentHandler) UpdateHeartbeat(c *gin.Context) {
	agentID := c.Param("agent_id")

	var req models.AgentHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate status
	if req.Status != "online" && req.Status != "draining" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid status",
			"details": "Status must be 'online' or 'draining'",
		})
		return
	}

	now := time.Now()
	var result sql.Result
	var err error

	if req.Capacity != nil {
		// Update with capacity
		result, err = h.database.DB().Exec(`
			UPDATE agents
			SET last_heartbeat = $1, status = $2, capacity = $3, updated_at = $1
			WHERE agent_id = $4
		`, now, req.Status, req.Capacity, agentID)
	} else {
		// Update without capacity
		result, err = h.database.DB().Exec(`
			UPDATE agents
			SET last_heartbeat = $1, status = $2, updated_at = $1
			WHERE agent_id = $3
		`, now, req.Status, agentID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update heartbeat",
			"details": err.Error(),
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check update result",
			"details": err.Error(),
		})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Agent not found",
			"agentId": agentID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Heartbeat updated successfully",
		"agentId":       agentID,
		"status":        req.Status,
		"lastHeartbeat": now,
	})
}

// SendCommand godoc
// @Summary Send a command to an agent
// @Description Creates and dispatches a command to an agent. The command is queued and sent via WebSocket.
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body models.SendCommandRequest true "Command request"
// @Success 201 {object} models.AgentCommand "Command created and queued"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Agent not found"
// @Failure 503 {object} map[string]interface{} "Agent not connected"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /agents/{agent_id}/command [post]
func (h *AgentHandler) SendCommand(c *gin.Context) {
	agentID := c.Param("agent_id")

	// Parse request
	var req struct {
		Action    string                 `json:"action" binding:"required"`
		SessionID string                 `json:"sessionId,omitempty"`
		Payload   map[string]interface{} `json:"payload,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate action
	validActions := map[string]bool{
		"start_session":     true,
		"stop_session":      true,
		"hibernate_session": true,
		"wake_session":      true,
	}
	if !validActions[req.Action] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid action",
			"details": "Action must be one of: start_session, stop_session, hibernate_session, wake_session",
		})
		return
	}

	// Verify agent exists
	var agent models.Agent
	err := h.database.DB().QueryRow(`
		SELECT id, agent_id, platform, status
		FROM agents
		WHERE agent_id = $1
	`, agentID).Scan(&agent.ID, &agent.AgentID, &agent.Platform, &agent.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Agent not found",
			"agentId": agentID,
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database error",
			"details": err.Error(),
		})
		return
	}

	// Check if agent is connected via WebSocket
	if h.hub != nil && !h.hub.IsAgentConnected(agentID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Agent not connected",
			"details": "Agent must be connected via WebSocket to receive commands",
			"agentId": agentID,
			"status":  agent.Status,
		})
		return
	}

	// Generate command ID
	commandID := "cmd-" + uuid.New().String()

	// Convert payload to CommandPayload type
	var payload *models.CommandPayload
	if req.Payload != nil {
		p := models.CommandPayload(req.Payload)
		payload = &p
	}

	// Create command in database
	now := time.Now()
	var command models.AgentCommand
	err = h.database.DB().QueryRow(`
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID, req.SessionID, req.Action, payload, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&command.ErrorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create command",
			"details": err.Error(),
		})
		return
	}

	// Dispatch command to agent
	if h.dispatcher != nil {
		if err := h.dispatcher.DispatchCommand(&command); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to dispatch command",
				"details": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusCreated, command)
}
