// Package models defines WebSocket protocol messages for agent communication.
//
// This file defines the message types and structures used for bidirectional
// communication between the Control Plane and platform-specific agents over WebSocket.
//
// Message Flow:
//
// Control Plane → Agent:
//   - command: Execute a session command (start_session, stop_session, etc.)
//   - ping: Keep-alive ping to check connection health
//   - shutdown: Request graceful agent shutdown
//
// Agent → Control Plane:
//   - heartbeat: Regular status update (every 10 seconds)
//   - ack: Acknowledge command receipt
//   - complete: Report command completion with results
//   - failed: Report command failure with error details
//   - status: Report session state changes
//
// Protocol Design:
//   - All messages are JSON-encoded
//   - Each message has a type field for routing
//   - Timestamps are included for tracking
//   - Command lifecycle: pending → sent → ack → completed/failed
//
// Example Message (Control Plane → Agent):
//
//	{
//	  "type": "command",
//	  "timestamp": "2025-11-21T10:30:00Z",
//	  "payload": {
//	    "commandId": "cmd-abc123",
//	    "action": "start_session",
//	    "payload": {
//	      "sessionId": "sess-456",
//	      "user": "alice",
//	      "template": "firefox-browser"
//	    }
//	  }
//	}
//
// Example Message (Agent → Control Plane):
//
//	{
//	  "type": "complete",
//	  "timestamp": "2025-11-21T10:30:05Z",
//	  "payload": {
//	    "commandId": "cmd-abc123",
//	    "result": {
//	      "sessionId": "sess-456",
//	      "vncPort": 5900,
//	      "podName": "sess-456-abc123"
//	    }
//	  }
//	}
package models

import (
	"encoding/json"
	"time"
)

// AgentMessage is the top-level message structure for all agent communication.
//
// Every message sent between Control Plane and Agent follows this structure.
// The Type field determines how to parse the Payload.
type AgentMessage struct {
	// Type identifies the message type (command, ping, heartbeat, ack, etc.)
	Type string `json:"type"`

	// Timestamp when the message was created
	Timestamp time.Time `json:"timestamp"`

	// Payload contains the message-specific data as raw JSON
	// Parse this based on the Type field
	Payload json.RawMessage `json:"payload"`
}

// Message types sent from Control Plane → Agent
const (
	// MessageTypeCommand instructs agent to execute a command (start_session, stop_session, etc.)
	MessageTypeCommand = "command"

	// MessageTypePing is a keep-alive ping to verify connection health
	MessageTypePing = "ping"

	// MessageTypeShutdown requests graceful agent shutdown
	MessageTypeShutdown = "shutdown"

	// MessageTypeVNCData carries VNC traffic from Control Plane to Agent
	MessageTypeVNCData = "vnc_data"

	// MessageTypeVNCClose closes a VNC tunnel
	MessageTypeVNCClose = "vnc_close"
)

// Message types sent from Agent → Control Plane
const (
	// MessageTypeHeartbeat is a regular status update from agent (every 10 seconds)
	MessageTypeHeartbeat = "heartbeat"

	// MessageTypeAck acknowledges command receipt
	MessageTypeAck = "ack"

	// MessageTypeComplete reports successful command completion
	MessageTypeComplete = "complete"

	// MessageTypeFailed reports command failure
	MessageTypeFailed = "failed"

	// MessageTypeStatus reports session state changes
	MessageTypeStatus = "status"

	// MessageTypeVNCReady indicates VNC tunnel is ready
	MessageTypeVNCReady = "vnc_ready"

	// MessageTypeVNCData carries VNC traffic from Agent to Control Plane
	// (same name, direction determined by message flow)

	// MessageTypeVNCError reports VNC tunnel error
	MessageTypeVNCError = "vnc_error"
)

// CommandMessage is sent from Control Plane to Agent to execute a command.
//
// The Action field determines what operation to perform:
//   - start_session: Create a new session
//   - stop_session: Terminate a session
//   - hibernate_session: Hibernate a running session
//   - wake_session: Wake a hibernated session
//
// Example:
//
//	{
//	  "commandId": "cmd-abc123",
//	  "action": "start_session",
//	  "payload": {
//	    "sessionId": "sess-456",
//	    "user": "alice",
//	    "template": "firefox-browser",
//	    "resources": {"memory": "2Gi", "cpu": "1000m"}
//	  }
//	}
type CommandMessage struct {
	// CommandID uniquely identifies this command
	CommandID string `json:"commandId"`

	// Action specifies the operation to perform
	Action string `json:"action"`

	// Payload contains action-specific data
	Payload map[string]interface{} `json:"payload"`
}

// HeartbeatMessage is sent from Agent to Control Plane every 10 seconds.
//
// Heartbeats keep the connection alive and provide status updates.
//
// Example:
//
//	{
//	  "status": "online",
//	  "activeSessions": 15,
//	  "capacity": {
//	    "maxSessions": 100,
//	    "cpu": "64 cores",
//	    "memory": "256Gi"
//	  }
//	}
type HeartbeatMessage struct {
	// Status is the current agent status (online, draining)
	Status string `json:"status"`

	// ActiveSessions is the number of sessions currently running on this agent
	ActiveSessions int `json:"activeSessions"`

	// Capacity describes the agent's resource limits (optional)
	Capacity *AgentCapacity `json:"capacity,omitempty"`
}

// AckMessage acknowledges command receipt.
//
// Sent immediately when agent receives a command, before execution begins.
//
// Example:
//
//	{
//	  "commandId": "cmd-abc123"
//	}
type AckMessage struct {
	// CommandID identifies which command is being acknowledged
	CommandID string `json:"commandId"`
}

// CompleteMessage reports successful command completion.
//
// Sent when agent successfully completes a command.
//
// Example:
//
//	{
//	  "commandId": "cmd-abc123",
//	  "result": {
//	    "sessionId": "sess-456",
//	    "vncPort": 5900,
//	    "podName": "sess-456-abc123"
//	  }
//	}
type CompleteMessage struct {
	// CommandID identifies which command completed
	CommandID string `json:"commandId"`

	// Result contains command-specific result data (optional)
	Result map[string]interface{} `json:"result,omitempty"`
}

// FailedMessage reports command failure.
//
// Sent when agent fails to execute a command.
//
// Example:
//
//	{
//	  "commandId": "cmd-abc123",
//	  "error": "Failed to create pod: insufficient resources"
//	}
type FailedMessage struct {
	// CommandID identifies which command failed
	CommandID string `json:"commandId"`

	// Error describes why the command failed
	Error string `json:"error"`
}

// StatusMessage reports session state changes.
//
// Sent when a session changes state on the agent.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "state": "running",
//	  "vncReady": true,
//	  "vncPort": 5900,
//	  "platformMetadata": {
//	    "podName": "sess-456-abc123",
//	    "nodeName": "worker-1"
//	  }
//	}
type StatusMessage struct {
	// SessionID identifies which session this update is for
	SessionID string `json:"sessionId"`

	// State is the session state (pending, running, hibernated, terminated)
	State string `json:"state"`

	// VNCReady indicates if VNC is ready for connections
	VNCReady bool `json:"vncReady"`

	// VNCPort is the local VNC port on the agent (for tunneling)
	VNCPort int `json:"vncPort,omitempty"`

	// PlatformMetadata contains platform-specific information
	PlatformMetadata map[string]interface{} `json:"platformMetadata,omitempty"`
}

// PingMessage is a keep-alive ping from Control Plane to Agent.
//
// Example:
//
//	{
//	  "timestamp": "2025-11-21T10:30:00Z"
//	}
type PingMessage struct {
	// Timestamp when the ping was sent
	Timestamp time.Time `json:"timestamp"`
}

// PongMessage is the agent's response to a ping.
//
// Example:
//
//	{
//	  "timestamp": "2025-11-21T10:30:00Z"
//	}
type PongMessage struct {
	// Timestamp when the pong was sent
	Timestamp time.Time `json:"timestamp"`
}

// ShutdownMessage requests graceful agent shutdown.
//
// Example:
//
//	{
//	  "reason": "maintenance"
//	}
type ShutdownMessage struct {
	// Reason for the shutdown request
	Reason string `json:"reason,omitempty"`
}

// VNCDataMessage carries binary VNC traffic between Control Plane and Agent.
//
// VNC traffic is base64-encoded for transport over JSON WebSocket.
// The tunnelId identifies which VNC session this data belongs to.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "data": "UkZCIDAwMy4wMDgK..." (base64-encoded VNC data)
//	}
type VNCDataMessage struct {
	// SessionID identifies which session this VNC data is for
	SessionID string `json:"sessionId"`

	// Data is the base64-encoded VNC binary data
	Data string `json:"data"`
}

// VNCReadyMessage indicates a VNC tunnel is ready for connections.
//
// Sent from Agent to Control Plane when port-forward tunnel is established.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "vncPort": 5900,
//	  "podName": "sess-456-abc123"
//	}
type VNCReadyMessage struct {
	// SessionID identifies which session has VNC ready
	SessionID string `json:"sessionId"`

	// VNCPort is the local VNC port on the agent (typically 5900 or 3000)
	VNCPort int `json:"vncPort"`

	// PodName is the name of the pod (K8s-specific metadata)
	PodName string `json:"podName,omitempty"`
}

// VNCCloseMessage requests closing a VNC tunnel.
//
// Sent from Control Plane to Agent when client disconnects.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "reason": "client_disconnect"
//	}
type VNCCloseMessage struct {
	// SessionID identifies which session's VNC tunnel to close
	SessionID string `json:"sessionId"`

	// Reason explains why the tunnel is being closed (optional)
	Reason string `json:"reason,omitempty"`
}

// VNCErrorMessage reports a VNC tunnel error.
//
// Sent from Agent to Control Plane when VNC tunnel fails.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "error": "Port-forward failed: pod not found"
//	}
type VNCErrorMessage struct {
	// SessionID identifies which session had the error
	SessionID string `json:"sessionId"`

	// Error describes what went wrong
	Error string `json:"error"`
}
