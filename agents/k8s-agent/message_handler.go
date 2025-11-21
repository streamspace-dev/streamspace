package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// AgentMessage represents a message from the Control Plane.
//
// This matches the protocol defined in api/internal/models/agent_protocol.go
type AgentMessage struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// CommandMessage represents a command from the Control Plane.
type CommandMessage struct {
	CommandID string                 `json:"commandId"`
	Action    string                 `json:"action"`
	Payload   map[string]interface{} `json:"payload"`
}

// PingMessage represents a ping from the Control Plane.
type PingMessage struct {
	Timestamp time.Time `json:"timestamp"`
}

// ShutdownMessage represents a shutdown request from the Control Plane.
type ShutdownMessage struct {
	Reason string `json:"reason,omitempty"`
}

// handleMessage processes an incoming message from the Control Plane.
func (a *K8sAgent) handleMessage(messageBytes []byte) error {
	// Parse the top-level message
	var msg AgentMessage
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Route based on message type
	switch msg.Type {
	case "command":
		return a.handleCommandMessage(msg.Payload)

	case "ping":
		return a.handlePingMessage(msg.Payload)

	case "shutdown":
		return a.handleShutdownMessage(msg.Payload)

	default:
		log.Printf("[K8sAgent] Unknown message type: %s", msg.Type)
		return nil
	}
}

// handleCommandMessage processes a command from the Control Plane.
func (a *K8sAgent) handleCommandMessage(payload json.RawMessage) error {
	var cmd CommandMessage
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	log.Printf("[K8sAgent] Received command: %s (action: %s)", cmd.CommandID, cmd.Action)

	// Send acknowledgment
	if err := a.sendAck(cmd.CommandID); err != nil {
		log.Printf("[K8sAgent] Failed to send ack for %s: %v", cmd.CommandID, err)
	}

	// Find and execute command handler
	handler, ok := a.commandHandlers[cmd.Action]
	if !ok {
		log.Printf("[K8sAgent] Unknown command action: %s", cmd.Action)
		return a.sendFailed(cmd.CommandID, fmt.Sprintf("unknown action: %s", cmd.Action))
	}

	// Execute command
	result, err := handler.Handle(&cmd)
	if err != nil {
		log.Printf("[K8sAgent] Command %s failed: %v", cmd.CommandID, err)
		return a.sendFailed(cmd.CommandID, err.Error())
	}

	// Send completion
	log.Printf("[K8sAgent] Command %s completed successfully", cmd.CommandID)
	return a.sendComplete(cmd.CommandID, result)
}

// handlePingMessage responds to a ping from the Control Plane.
func (a *K8sAgent) handlePingMessage(payload json.RawMessage) error {
	log.Println("[K8sAgent] Received ping, sending pong")

	pong := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now(),
	}

	return a.sendMessage(pong)
}

// handleShutdownMessage initiates graceful shutdown.
func (a *K8sAgent) handleShutdownMessage(payload json.RawMessage) error {
	var shutdown ShutdownMessage
	if err := json.Unmarshal(payload, &shutdown); err != nil {
		log.Printf("[K8sAgent] Failed to parse shutdown message: %v", err)
	}

	log.Printf("[K8sAgent] Shutdown requested by Control Plane: %s", shutdown.Reason)

	// Trigger graceful shutdown
	close(a.stopChan)

	return nil
}

// sendAck sends a command acknowledgment to the Control Plane.
func (a *K8sAgent) sendAck(commandID string) error {
	ack := map[string]interface{}{
		"type":      "ack",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"commandId": commandID,
		},
	}

	return a.sendMessage(ack)
}

// sendComplete sends a command completion to the Control Plane.
func (a *K8sAgent) sendComplete(commandID string, result *CommandResult) error {
	complete := map[string]interface{}{
		"type":      "complete",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"commandId": commandID,
			"result":    result.Data,
		},
	}

	return a.sendMessage(complete)
}

// sendFailed sends a command failure to the Control Plane.
func (a *K8sAgent) sendFailed(commandID string, errorMessage string) error {
	failed := map[string]interface{}{
		"type":      "failed",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"commandId": commandID,
			"error":     errorMessage,
		},
	}

	return a.sendMessage(failed)
}

// sendStatusUpdate sends a session status update to the Control Plane.
func (a *K8sAgent) sendStatusUpdate(sessionID string, state string, vncReady bool, vncPort int, metadata map[string]interface{}) error {
	status := map[string]interface{}{
		"type":      "status",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"sessionId":        sessionID,
			"state":            state,
			"vncReady":         vncReady,
			"vncPort":          vncPort,
			"platformMetadata": metadata,
		},
	}

	return a.sendMessage(status)
}
