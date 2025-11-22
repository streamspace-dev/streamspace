package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// AgentMessage represents a message received from Control Plane.
type AgentMessage struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// CommandMessage represents a command payload.
type CommandMessage struct {
	CommandID string                 `json:"commandId"`
	Action    string                 `json:"action"`
	Payload   map[string]interface{} `json:"payload"`
}

// handleMessage processes incoming WebSocket messages.
func (a *DockerAgent) handleMessage(message []byte) {
	log.Printf("[MessageHandler] Received message: %s", string(message))

	// Parse agent message
	var agentMsg AgentMessage
	if err := json.Unmarshal(message, &agentMsg); err != nil {
		log.Printf("[MessageHandler] Failed to parse agent message: %v", err)
		return
	}

	// Route by message type
	switch agentMsg.Type {
	case "command":
		a.handleCommand(agentMsg.Payload)

	case "ping":
		a.handlePing()

	case "shutdown":
		a.handleShutdown()

	default:
		log.Printf("[MessageHandler] Unknown message type: %s", agentMsg.Type)
	}
}

// handleCommand processes command messages.
func (a *DockerAgent) handleCommand(payload json.RawMessage) {
	// Parse command message
	var cmd CommandMessage
	if err := json.Unmarshal(payload, &cmd); err != nil {
		log.Printf("[CommandHandler] Failed to parse command: %v", err)
		return
	}

	log.Printf("[CommandHandler] Command: %s (ID: %s)", cmd.Action, cmd.CommandID)

	// Get handler for this action
	handler, ok := a.commandHandlers[cmd.Action]
	if !ok {
		log.Printf("[CommandHandler] Unknown command action: %s", cmd.Action)
		a.sendCommandError(cmd.CommandID, fmt.Sprintf("unknown command action: %s", cmd.Action))
		return
	}

	// Convert payload to JSON for handler
	payloadBytes, err := json.Marshal(cmd.Payload)
	if err != nil {
		log.Printf("[CommandHandler] Failed to marshal payload: %v", err)
		a.sendCommandError(cmd.CommandID, fmt.Sprintf("failed to marshal payload: %v", err))
		return
	}

	// Execute handler
	if err := handler.Handle(payloadBytes); err != nil {
		log.Printf("[CommandHandler] Command failed: %v", err)
		a.sendCommandError(cmd.CommandID, fmt.Sprintf("command failed: %v", err))
		return
	}

	log.Printf("[CommandHandler] Command %s completed successfully", cmd.CommandID)
}

// handlePing responds to ping messages.
func (a *DockerAgent) handlePing() {
	log.Println("[MessageHandler] Received ping")

	response := map[string]interface{}{
		"type":    "pong",
		"agentId": a.config.AgentID,
	}

	if err := a.sendMessage(response); err != nil {
		log.Printf("[MessageHandler] Failed to send pong: %v", err)
	}
}

// handleShutdown processes shutdown requests.
func (a *DockerAgent) handleShutdown() {
	log.Println("[MessageHandler] Received shutdown request")

	// Send acknowledgment
	response := map[string]interface{}{
		"type":    "shutdown_ack",
		"agentId": a.config.AgentID,
	}

	if err := a.sendMessage(response); err != nil {
		log.Printf("[MessageHandler] Failed to send shutdown ack: %v", err)
	}

	// Trigger shutdown
	close(a.stopChan)
}

// sendCommandError sends a command error response.
func (a *DockerAgent) sendCommandError(commandID, errorMsg string) {
	response := map[string]interface{}{
		"type":      "command_error",
		"commandId": commandID,
		"error":     errorMsg,
	}

	if err := a.sendMessage(response); err != nil {
		log.Printf("[MessageHandler] Failed to send command error: %v", err)
	}
}
