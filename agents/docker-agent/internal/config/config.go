package config

import "github.com/streamspace-dev/streamspace/agents/docker-agent/internal/errors"

// AgentConfig holds the configuration for the Docker Agent.
//
// Configuration can be provided via:
//   - Command-line flags
//   - Environment variables
//   - Configuration file
type AgentConfig struct {
	// AgentID is the unique identifier for this agent
	// Format: docker-{environment}-{region} (e.g., docker-prod-us-east-1)
	AgentID string

	// ControlPlaneURL is the WebSocket URL for the Control Plane
	// Format: wss://control.example.com or ws://localhost:8000 (for dev)
	ControlPlaneURL string

	// Platform identifies the agent type
	// Value: "docker" (fixed for Docker Agent)
	Platform string

	// Region is the deployment region (optional)
	// Examples: us-east-1, eu-west-1, ap-southeast-1
	Region string

	// DockerHost is the Docker daemon socket
	// Default: "unix:///var/run/docker.sock"
	// Can also be "tcp://host:2375" for remote Docker
	DockerHost string

	// NetworkName is the Docker network to use for session containers
	// Default: "streamspace"
	NetworkName string

	// VolumeDriver is the Docker volume driver to use for persistent storage
	// Default: "local"
	// Can be "nfs", "rexray", etc.
	VolumeDriver string

	// Capacity defines the maximum resources available on this agent
	Capacity AgentCapacity

	// HeartbeatInterval is the interval for sending heartbeats
	// Default: 10 seconds
	HeartbeatInterval int // in seconds

	// ReconnectBackoff defines the reconnection strategy
	// Default: [2s, 4s, 8s, 16s, 32s]
	ReconnectBackoff []int // in seconds
}

// AgentCapacity defines the maximum resources available on the agent.
type AgentCapacity struct {
	// MaxCPU is the maximum CPU cores available (in millicores)
	// Example: 100 cores = 100000 millicores
	MaxCPU int `json:"maxCpu"`

	// MaxMemory is the maximum memory available (in GB)
	// Example: 128 GB
	MaxMemory int `json:"maxMemory"`

	// MaxSessions is the maximum number of concurrent sessions
	// Example: 100 sessions
	MaxSessions int `json:"maxSessions"`
}

// Validate validates the agent configuration.
func (c *AgentConfig) Validate() error {
	if c.AgentID == "" {
		return errors.ErrMissingAgentID
	}

	if c.ControlPlaneURL == "" {
		return errors.ErrMissingControlPlaneURL
	}

	if c.Platform == "" {
		c.Platform = "docker"
	}

	if c.DockerHost == "" {
		c.DockerHost = "unix:///var/run/docker.sock"
	}

	if c.NetworkName == "" {
		c.NetworkName = "streamspace"
	}

	if c.VolumeDriver == "" {
		c.VolumeDriver = "local"
	}

	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 10 // default 10 seconds
	}

	if c.ReconnectBackoff == nil || len(c.ReconnectBackoff) == 0 {
		c.ReconnectBackoff = []int{2, 4, 8, 16, 32} // default exponential backoff
	}

	return nil
}
