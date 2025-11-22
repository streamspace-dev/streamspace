package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/client"
	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/config"
)

// StartSessionHandler handles the start_session command.
type StartSessionHandler struct {
	dockerClient *client.Client
	config       *config.AgentConfig
	agent        *DockerAgent
}

// NewStartSessionHandler creates a new start session handler.
func NewStartSessionHandler(dockerClient *client.Client, cfg *config.AgentConfig, agent *DockerAgent) *StartSessionHandler {
	return &StartSessionHandler{
		dockerClient: dockerClient,
		config:       cfg,
		agent:        agent,
	}
}

// Handle processes the start_session command.
func (h *StartSessionHandler) Handle(payload json.RawMessage) error {
	log.Println("[StartSessionHandler] Processing start_session command")

	// Parse payload
	var commandPayload map[string]interface{}
	if err := json.Unmarshal(payload, &commandPayload); err != nil {
		return fmt.Errorf("failed to parse command payload: %w", err)
	}

	// Extract session details
	sessionID, ok := commandPayload["sessionId"].(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("sessionId not found in payload")
	}

	user, ok := commandPayload["user"].(string)
	if !ok || user == "" {
		return fmt.Errorf("user not found in payload")
	}

	log.Printf("[StartSessionHandler] Session: %s, User: %s", sessionID, user)

	// Parse template from payload
	template, err := parseTemplateFromPayload(commandPayload)
	if err != nil {
		log.Printf("[StartSessionHandler] Failed to parse template: %v", err)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	log.Printf("[StartSessionHandler] Template: %s (image: %s)", template.Name, template.BaseImage)

	// Extract resource requirements
	resources := make(map[string]string)
	if res, ok := commandPayload["resources"].(map[string]interface{}); ok {
		if memory, ok := res["memory"].(string); ok {
			resources["memory"] = memory
		}
		if cpu, ok := res["cpu"].(string); ok {
			resources["cpu"] = cpu
		}
	}

	// Extract persistent home flag
	persistentHome := false
	if ph, ok := commandPayload["persistentHome"].(bool); ok {
		persistentHome = ph
	}

	log.Printf("[StartSessionHandler] Resources: memory=%s, cpu=%s, persistentHome=%v",
		resources["memory"], resources["cpu"], persistentHome)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Ensure network exists
	if err := h.agent.ensureNetwork(ctx); err != nil {
		log.Printf("[StartSessionHandler] Failed to ensure network: %v", err)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Failed to ensure network: %v", err))
	}

	// Create container
	containerID, err := h.agent.createSessionContainer(ctx, sessionID, template, resources, persistentHome)
	if err != nil {
		log.Printf("[StartSessionHandler] Failed to create container: %v", err)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Failed to create container: %v", err))
	}

	// Start container
	if err := h.agent.startContainer(ctx, containerID); err != nil {
		log.Printf("[StartSessionHandler] Failed to start container: %v", err)
		// Try to clean up
		h.agent.removeContainer(ctx, containerID)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Failed to start container: %v", err))
	}

	// Wait for container to be running
	if err := h.agent.waitForContainerRunning(ctx, containerID, 60*time.Second); err != nil {
		log.Printf("[StartSessionHandler] Container failed to start: %v", err)
		// Try to clean up
		h.agent.stopContainer(ctx, containerID)
		h.agent.removeContainer(ctx, containerID)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Container failed to start: %v", err))
	}

	// Get container info for response
	inspect, err := h.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		log.Printf("[StartSessionHandler] Failed to inspect container: %v", err)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Failed to inspect container: %v", err))
	}

	// Get container IP
	containerIP := ""
	if networkSettings, ok := inspect.NetworkSettings.Networks[h.config.NetworkName]; ok {
		containerIP = networkSettings.IPAddress
	}

	log.Printf("[StartSessionHandler] Session %s started successfully (container: %s, IP: %s)",
		sessionID, containerID[:12], containerIP)

	// Send success response
	return h.sendSuccessResponse(sessionID, containerID, containerIP)
}

// sendSuccessResponse sends a success response back to Control Plane.
func (h *StartSessionHandler) sendSuccessResponse(sessionID, containerID, containerIP string) error {
	response := map[string]interface{}{
		"type":      "command_response",
		"success":   true,
		"sessionId": sessionID,
		"status":    "running",
		"container": map[string]interface{}{
			"id": containerID,
			"ip": containerIP,
		},
		"timestamp": time.Now().Unix(),
	}

	return h.agent.sendMessage(response)
}

// sendErrorResponse sends an error response back to Control Plane.
func (h *StartSessionHandler) sendErrorResponse(sessionID, errorMsg string) error {
	response := map[string]interface{}{
		"type":      "command_response",
		"success":   false,
		"sessionId": sessionID,
		"error":     errorMsg,
		"timestamp": time.Now().Unix(),
	}

	return h.agent.sendMessage(response)
}

// StopSessionHandler handles the stop_session command.
type StopSessionHandler struct {
	dockerClient *client.Client
	config       *config.AgentConfig
	agent        *DockerAgent
}

// NewStopSessionHandler creates a new stop session handler.
func NewStopSessionHandler(dockerClient *client.Client, cfg *config.AgentConfig, agent *DockerAgent) *StopSessionHandler {
	return &StopSessionHandler{
		dockerClient: dockerClient,
		config:       cfg,
		agent:        agent,
	}
}

// Handle processes the stop_session command.
func (h *StopSessionHandler) Handle(payload json.RawMessage) error {
	log.Println("[StopSessionHandler] Processing stop_session command")

	// Parse payload
	var commandPayload map[string]interface{}
	if err := json.Unmarshal(payload, &commandPayload); err != nil {
		return fmt.Errorf("failed to parse command payload: %w", err)
	}

	// Extract session ID
	sessionID, ok := commandPayload["sessionId"].(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("sessionId not found in payload")
	}

	log.Printf("[StopSessionHandler] Session: %s", sessionID)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Find container by session ID
	containerID, err := h.agent.getContainerBySession(ctx, sessionID)
	if err != nil {
		log.Printf("[StopSessionHandler] Container not found: %v", err)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Container not found: %v", err))
	}

	log.Printf("[StopSessionHandler] Found container: %s", containerID[:12])

	// Stop container
	if err := h.agent.stopContainer(ctx, containerID); err != nil {
		log.Printf("[StopSessionHandler] Failed to stop container: %v", err)
		return h.sendErrorResponse(sessionID, fmt.Sprintf("Failed to stop container: %v", err))
	}

	// Remove container
	if err := h.agent.removeContainer(ctx, containerID); err != nil {
		log.Printf("[StopSessionHandler] Failed to remove container: %v", err)
		// Container is stopped, so consider this a partial success
		log.Printf("[StopSessionHandler] Container stopped but not removed (may need manual cleanup)")
	}

	log.Printf("[StopSessionHandler] Session %s stopped successfully", sessionID)

	// Send success response
	return h.sendSuccessResponse(sessionID)
}

// sendSuccessResponse sends a success response back to Control Plane.
func (h *StopSessionHandler) sendSuccessResponse(sessionID string) error {
	response := map[string]interface{}{
		"type":      "command_response",
		"success":   true,
		"sessionId": sessionID,
		"status":    "terminated",
		"timestamp": time.Now().Unix(),
	}

	return h.agent.sendMessage(response)
}

// sendErrorResponse sends an error response back to Control Plane.
func (h *StopSessionHandler) sendErrorResponse(sessionID, errorMsg string) error {
	response := map[string]interface{}{
		"type":      "command_response",
		"success":   false,
		"sessionId": sessionID,
		"error":     errorMsg,
		"timestamp": time.Now().Unix(),
	}

	return h.agent.sendMessage(response)
}

// HibernateSessionHandler handles the hibernate_session command.
type HibernateSessionHandler struct {
	dockerClient *client.Client
	config       *config.AgentConfig
}

// NewHibernateSessionHandler creates a new hibernate session handler.
func NewHibernateSessionHandler(dockerClient *client.Client, cfg *config.AgentConfig) *HibernateSessionHandler {
	return &HibernateSessionHandler{
		dockerClient: dockerClient,
		config:       cfg,
	}
}

// Handle processes the hibernate_session command.
func (h *HibernateSessionHandler) Handle(payload json.RawMessage) error {
	log.Println("[HibernateSessionHandler] Processing hibernate_session command")
	// TODO: Implement hibernation (pause container)
	return fmt.Errorf("hibernation not yet implemented for Docker agent")
}

// WakeSessionHandler handles the wake_session command.
type WakeSessionHandler struct {
	dockerClient *client.Client
	config       *config.AgentConfig
}

// NewWakeSessionHandler creates a new wake session handler.
func NewWakeSessionHandler(dockerClient *client.Client, cfg *config.AgentConfig) *WakeSessionHandler {
	return &WakeSessionHandler{
		dockerClient: dockerClient,
		config:       cfg,
	}
}

// Handle processes the wake_session command.
func (h *WakeSessionHandler) Handle(payload json.RawMessage) error {
	log.Println("[WakeSessionHandler] Processing wake_session command")
	// TODO: Implement wake (unpause container)
	return fmt.Errorf("wake not yet implemented for Docker agent")
}
