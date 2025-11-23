// Package models defines the core data structures for the StreamSpace API.
//
// This file contains models for the v2.0 multi-platform agent architecture:
//   - Agent: Platform-specific execution agents (Kubernetes, Docker, etc.)
//   - AgentCommand: Commands dispatched from Control Plane to Agents
//
// These models support the Control Plane + Agent refactor where:
//   - Control Plane (this API) manages sessions centrally
//   - Agents connect via WebSocket and execute platform-specific operations
//   - VNC traffic is tunneled through the Control Plane
package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// AgentCapacity represents the resource capacity of an agent.
//
// This is stored as JSONB in the database and contains information about
// how many sessions the agent can handle and its resource limits.
//
// Example:
//
//	{
//	  "maxSessions": 100,
//	  "cpu": "64 cores",
//	  "memory": "256Gi",
//	  "storage": "1Ti"
//	}
type AgentCapacity struct {
	MaxSessions int    `json:"maxSessions"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	Storage     string `json:"storage,omitempty"`
}

// Scan implements the sql.Scanner interface for AgentCapacity.
func (a *AgentCapacity) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

// Value implements the driver.Valuer interface for AgentCapacity.
func (a AgentCapacity) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// AgentMetadata represents arbitrary metadata about an agent.
//
// This is stored as JSONB in the database and can contain platform-specific
// information, deployment details, or other custom data.
//
// Example:
//
//	{
//	  "version": "2.0.0",
//	  "clusterName": "prod-us-east-1",
//	  "kubernetesVersion": "1.28",
//	  "nodeSelector": {"role": "streamspace"}
//	}
type AgentMetadata map[string]interface{}

// Scan implements the sql.Scanner interface for AgentMetadata.
func (a *AgentMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

// Value implements the driver.Valuer interface for AgentMetadata.
func (a AgentMetadata) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Agent represents a platform-specific execution agent connected to the Control Plane.
//
// Agents are responsible for:
//   - Connecting to Control Plane via WebSocket (outbound connection)
//   - Receiving commands to start/stop/hibernate sessions
//   - Translating generic session specs to platform-specific resources
//   - Tunneling VNC traffic back to Control Plane
//   - Reporting session status and health
//
// Supported platforms:
//   - kubernetes: Kubernetes cluster agent
//   - docker: Docker host agent
//   - vm: Virtual machine agent (future)
//   - cloud: Cloud provider agent (future)
//
// Example:
//
//	{
//	  "id": "550e8400-e29b-41d4-a716-446655440000",
//	  "agentId": "k8s-prod-us-east-1",
//	  "platform": "kubernetes",
//	  "region": "us-east-1",
//	  "status": "online",
//	  "capacity": {
//	    "maxSessions": 100,
//	    "cpu": "64 cores",
//	    "memory": "256Gi"
//	  },
//	  "lastHeartbeat": "2025-11-21T10:30:00Z"
//	}
type Agent struct {
	// ID is the auto-generated UUID for this agent (database primary key).
	ID string `json:"id" db:"id"`

	// AgentID is a unique identifier for this agent (user-defined).
	// This is what the agent uses to identify itself when connecting.
	//
	// Examples: "k8s-prod-us-east-1", "docker-dev-host-1", "vm-agent-london-2"
	AgentID string `json:"agentId" db:"agent_id"`

	// Platform identifies the execution platform this agent manages.
	//
	// Valid values:
	//   - "kubernetes": Kubernetes cluster
	//   - "docker": Docker host
	//   - "vm": Virtual machines
	//   - "cloud": Cloud provider (AWS, Azure, GCP)
	Platform string `json:"platform" db:"platform"`

	// Region is the geographical or logical region where this agent operates.
	// Used for geo-aware session placement.
	//
	// Examples: "us-east-1", "eu-west-1", "on-prem-dc1"
	Region string `json:"region,omitempty" db:"region"`

	// Status indicates the current health status of the agent.
	//
	// Valid values:
	//   - "online": Agent is connected and healthy
	//   - "offline": Agent is disconnected
	//   - "draining": Agent is not accepting new sessions
	Status string `json:"status" db:"status"`

	// Capacity describes the resource limits of this agent.
	// Stored as JSONB in the database.
	Capacity *AgentCapacity `json:"capacity,omitempty" db:"capacity"`

	// LastHeartbeat is the timestamp of the last heartbeat received from this agent.
	// Agents send heartbeats every 10 seconds.
	LastHeartbeat *time.Time `json:"lastHeartbeat,omitempty" db:"last_heartbeat"`

	// WebSocketID is the internal identifier for the active WebSocket connection.
	// Used by the Control Plane to route commands to the correct connection.
	// Can be nil if the agent is not currently connected.
	WebSocketID *string `json:"websocketId,omitempty" db:"websocket_id"`

	// Metadata contains arbitrary platform-specific or deployment-specific data.
	// Stored as JSONB in the database.
	Metadata *AgentMetadata `json:"metadata,omitempty" db:"metadata"`

	// APIKeyHash is the bcrypt hash of the agent's API key.
	// SECURITY: Never expose this field in JSON responses (json:"-")
	// Used for authenticating agent registration and WebSocket connections.
	APIKeyHash *string `json:"-" db:"api_key_hash"`

	// APIKeyCreatedAt is when the API key was generated.
	// Used for key rotation policies and security auditing.
	APIKeyCreatedAt *time.Time `json:"-" db:"api_key_created_at"`

	// APIKeyLastUsedAt is when the API key was last used successfully.
	// Used for anomaly detection and security auditing.
	APIKeyLastUsedAt *time.Time `json:"-" db:"api_key_last_used_at"`

	// CreatedAt is when this agent was first registered.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when this agent record was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// CommandPayload represents the payload of a command sent to an agent.
//
// This is stored as JSONB and contains the session spec or other command data.
//
// Example (start_session):
//
//	{
//	  "sessionId": "sess-123",
//	  "user": "alice",
//	  "template": "firefox-browser",
//	  "resources": {
//	    "memory": "2Gi",
//	    "cpu": "1000m"
//	  }
//	}
type CommandPayload map[string]interface{}

// Scan implements the sql.Scanner interface for CommandPayload.
func (c *CommandPayload) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// Value implements the driver.Valuer interface for CommandPayload.
func (c CommandPayload) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// AgentCommand represents a command dispatched from the Control Plane to an Agent.
//
// Commands are queued in the database and sent to agents over WebSocket.
// The lifecycle of a command is:
//   1. Created (status: pending)
//   2. Sent to agent over WebSocket (status: sent, sent_at timestamp)
//   3. Agent acknowledges receipt (status: ack, acknowledged_at timestamp)
//   4. Agent completes execution (status: completed, completed_at timestamp)
//   5. Or agent fails (status: failed, error_message populated)
//
// Supported actions:
//   - start_session: Create a new session
//   - stop_session: Terminate a session
//   - hibernate_session: Hibernate a running session
//   - wake_session: Wake a hibernated session
//
// Example:
//
//	{
//	  "id": "550e8400-e29b-41d4-a716-446655440000",
//	  "commandId": "cmd-abc123",
//	  "agentId": "k8s-prod-us-east-1",
//	  "sessionId": "sess-456",
//	  "action": "start_session",
//	  "payload": {
//	    "user": "alice",
//	    "template": "firefox-browser"
//	  },
//	  "status": "completed",
//	  "createdAt": "2025-11-21T10:30:00Z",
//	  "completedAt": "2025-11-21T10:30:05Z"
//	}
type AgentCommand struct {
	// ID is the auto-generated UUID for this command (database primary key).
	ID string `json:"id" db:"id"`

	// CommandID is a unique identifier for this command.
	// Used to track the command through its lifecycle.
	CommandID string `json:"commandId" db:"command_id"`

	// AgentID identifies which agent should execute this command.
	AgentID string `json:"agentId" db:"agent_id"`

	// SessionID is the session this command affects (if applicable).
	// Uses pointer type to handle NULL values for commands without sessions.
	SessionID *string `json:"sessionId,omitempty" db:"session_id"`

	// Action is the operation to perform.
	//
	// Valid values:
	//   - "start_session": Create a new session
	//   - "stop_session": Terminate a session
	//   - "hibernate_session": Hibernate a running session
	//   - "wake_session": Wake a hibernated session
	Action string `json:"action" db:"action"`

	// Payload contains the command-specific data (e.g., session spec for start_session).
	// Stored as JSONB in the database.
	Payload *CommandPayload `json:"payload,omitempty" db:"payload"`

	// Status tracks the command lifecycle.
	//
	// Valid values:
	//   - "pending": Command queued, not yet sent
	//   - "sent": Sent to agent over WebSocket
	//   - "ack": Agent acknowledged receipt
	//   - "completed": Agent completed execution successfully
	//   - "failed": Agent failed to execute
	Status string `json:"status" db:"status"`

	// ErrorMessage contains the error details if status is "failed".
	// Uses pointer type to handle NULL values for pending/successful commands.
	ErrorMessage *string `json:"errorMessage,omitempty" db:"error_message"`

	// CreatedAt is when this command was created in the database.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// SentAt is when this command was sent to the agent over WebSocket.
	SentAt *time.Time `json:"sentAt,omitempty" db:"sent_at"`

	// AcknowledgedAt is when the agent acknowledged receipt of this command.
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty" db:"acknowledged_at"`

	// CompletedAt is when the agent completed execution of this command.
	CompletedAt *time.Time `json:"completedAt,omitempty" db:"completed_at"`
}

// AgentRegistrationRequest represents the request to register a new agent.
//
// This is sent by the agent when it first connects to the Control Plane.
//
// Example:
//
//	{
//	  "agentId": "k8s-prod-us-east-1",
//	  "platform": "kubernetes",
//	  "region": "us-east-1",
//	  "capacity": {
//	    "maxSessions": 100,
//	    "cpu": "64 cores",
//	    "memory": "256Gi"
//	  },
//	  "metadata": {
//	    "kubernetesVersion": "1.28",
//	    "nodeSelector": {"role": "streamspace"}
//	  }
//	}
type AgentRegistrationRequest struct {
	AgentID  string          `json:"agentId" binding:"required" validate:"required,min=3,max=100"`
	Platform string          `json:"platform" binding:"required,oneof=kubernetes docker vm cloud" validate:"required,oneof=kubernetes docker vm cloud"`
	Region   string          `json:"region,omitempty" validate:"omitempty,min=2,max=50"`
	Capacity *AgentCapacity  `json:"capacity,omitempty"`
	Metadata *AgentMetadata  `json:"metadata,omitempty"`
}

// AgentHeartbeatRequest represents a heartbeat sent by an agent.
//
// Agents send heartbeats every 10 seconds to indicate they are still alive.
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
type AgentHeartbeatRequest struct {
	Status         string         `json:"status" binding:"required,oneof=online draining" validate:"required,oneof=online draining"`
	ActiveSessions int            `json:"activeSessions" validate:"gte=0"`
	Capacity       *AgentCapacity `json:"capacity,omitempty"`
}

// AgentStatusUpdate represents a status update from an agent about a session.
//
// Sent by the agent when a session changes state.
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
type AgentStatusUpdate struct {
	SessionID        string                 `json:"sessionId" binding:"required"`
	State            string                 `json:"state" binding:"required"`
	VNCReady         bool                   `json:"vncReady"`
	VNCPort          int                    `json:"vncPort,omitempty"`
	PlatformMetadata map[string]interface{} `json:"platformMetadata,omitempty"`
}

// CreateSessionCommand represents the payload for a "start_session" command.
//
// This is sent from the Control Plane to an agent to create a new session.
//
// Example:
//
//	{
//	  "sessionId": "sess-456",
//	  "user": "alice",
//	  "template": "firefox-browser",
//	  "resources": {
//	    "memory": "2Gi",
//	    "cpu": "1000m"
//	  },
//	  "persistentHome": true
//	}
type CreateSessionCommand struct {
	SessionID      string            `json:"sessionId"`
	User           string            `json:"user"`
	Template       string            `json:"template"`
	Resources      map[string]string `json:"resources,omitempty"`
	PersistentHome bool              `json:"persistentHome"`
	Environment    map[string]string `json:"environment,omitempty"`
}
