package config

import (
	"testing"

	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/errors"
)

// TestAgentConfig_Validate tests the Validate method
func TestAgentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr error
	}{
		{
			name: "valid config with all fields",
			config: &AgentConfig{
				AgentID:           "docker-test-us-east-1",
				ControlPlaneURL:   "ws://localhost:8000",
				APIKey:            "test-api-key-1234567890abcdef1234567890abcdef",
				Platform:          "docker",
				Region:            "us-east-1",
				DockerHost:        "unix:///var/run/docker.sock",
				NetworkName:       "streamspace",
				VolumeDriver:      "local",
				HeartbeatInterval: 10,
				ReconnectBackoff:  []int{2, 4, 8, 16, 32},
			},
			wantErr: nil,
		},
		{
			name: "valid config with minimal fields",
			config: &AgentConfig{
				AgentID:         "docker-test",
				ControlPlaneURL: "ws://localhost:8000",
				APIKey:          "test-api-key",
			},
			wantErr: nil,
		},
		{
			name: "missing agent ID",
			config: &AgentConfig{
				ControlPlaneURL: "ws://localhost:8000",
				APIKey:          "test-api-key",
			},
			wantErr: errors.ErrMissingAgentID,
		},
		{
			name: "missing control plane URL",
			config: &AgentConfig{
				AgentID: "docker-test",
				APIKey:  "test-api-key",
			},
			wantErr: errors.ErrMissingControlPlaneURL,
		},
		{
			name: "missing API key",
			config: &AgentConfig{
				AgentID:         "docker-test",
				ControlPlaneURL: "ws://localhost:8000",
			},
			wantErr: errors.ErrMissingAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if err != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

// TestAgentConfig_Validate_Defaults tests that Validate sets default values
func TestAgentConfig_Validate_Defaults(t *testing.T) {
	config := &AgentConfig{
		AgentID:         "docker-test",
		ControlPlaneURL: "ws://localhost:8000",
		APIKey:          "test-api-key",
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error = %v", err)
	}

	// Check defaults were set
	if config.Platform != "docker" {
		t.Errorf("Platform = %s, want docker", config.Platform)
	}

	if config.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("DockerHost = %s, want unix:///var/run/docker.sock", config.DockerHost)
	}

	if config.NetworkName != "streamspace" {
		t.Errorf("NetworkName = %s, want streamspace", config.NetworkName)
	}

	if config.VolumeDriver != "local" {
		t.Errorf("VolumeDriver = %s, want local", config.VolumeDriver)
	}

	if config.HeartbeatInterval != 10 {
		t.Errorf("HeartbeatInterval = %d, want 10", config.HeartbeatInterval)
	}

	if len(config.ReconnectBackoff) != 5 {
		t.Errorf("ReconnectBackoff length = %d, want 5", len(config.ReconnectBackoff))
	}

	expectedBackoff := []int{2, 4, 8, 16, 32}
	for i, v := range config.ReconnectBackoff {
		if v != expectedBackoff[i] {
			t.Errorf("ReconnectBackoff[%d] = %d, want %d", i, v, expectedBackoff[i])
		}
	}
}

// TestAgentConfig_Validate_CustomValues tests that custom values are preserved
func TestAgentConfig_Validate_CustomValues(t *testing.T) {
	config := &AgentConfig{
		AgentID:           "docker-custom",
		ControlPlaneURL:   "wss://production.example.com",
		APIKey:            "custom-key",
		Platform:          "docker-custom",
		DockerHost:        "tcp://192.168.1.100:2375",
		NetworkName:       "custom-network",
		VolumeDriver:      "rexray",
		HeartbeatInterval: 30,
		ReconnectBackoff:  []int{5, 10, 20},
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error = %v", err)
	}

	// Verify custom values are preserved
	if config.Platform != "docker-custom" {
		t.Errorf("Platform = %s, want docker-custom", config.Platform)
	}

	if config.DockerHost != "tcp://192.168.1.100:2375" {
		t.Errorf("DockerHost = %s, want tcp://192.168.1.100:2375", config.DockerHost)
	}

	if config.NetworkName != "custom-network" {
		t.Errorf("NetworkName = %s, want custom-network", config.NetworkName)
	}

	if config.VolumeDriver != "rexray" {
		t.Errorf("VolumeDriver = %s, want rexray", config.VolumeDriver)
	}

	if config.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval = %d, want 30", config.HeartbeatInterval)
	}

	if len(config.ReconnectBackoff) != 3 {
		t.Errorf("ReconnectBackoff length = %d, want 3", len(config.ReconnectBackoff))
	}
}

// TestAgentCapacity tests the AgentCapacity struct
func TestAgentCapacity(t *testing.T) {
	capacity := AgentCapacity{
		MaxCPU:      100000, // 100 cores
		MaxMemory:   128,    // 128 GB
		MaxSessions: 100,
	}

	if capacity.MaxCPU != 100000 {
		t.Errorf("MaxCPU = %d, want 100000", capacity.MaxCPU)
	}

	if capacity.MaxMemory != 128 {
		t.Errorf("MaxMemory = %d, want 128", capacity.MaxMemory)
	}

	if capacity.MaxSessions != 100 {
		t.Errorf("MaxSessions = %d, want 100", capacity.MaxSessions)
	}
}
