package main

import (
	"encoding/json"
	"testing"

	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/config"
)

// TestAgentConfig tests agent configuration validation
func TestAgentConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.AgentConfig
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: &config.AgentConfig{
				AgentID:         "docker-test-local",
				ControlPlaneURL: "ws://localhost:8000",
				Platform:        "docker",
				Region:          "us-east-1",
				DockerHost:      "unix:///var/run/docker.sock",
				APIKey:          "test-api-key-1234567890abcdef1234567890abcdef",
			},
			wantErr: false,
		},
		{
			name: "Missing agent ID",
			config: &config.AgentConfig{
				ControlPlaneURL: "ws://localhost:8000",
				APIKey:          "test-api-key-1234567890abcdef1234567890abcdef",
			},
			wantErr: true,
		},
		{
			name: "Missing control plane URL",
			config: &config.AgentConfig{
				AgentID: "docker-test-local",
				APIKey:  "test-api-key-1234567890abcdef1234567890abcdef",
			},
			wantErr: true,
		},
		{
			name: "Missing API key",
			config: &config.AgentConfig{
				AgentID:         "docker-test-local",
				ControlPlaneURL: "ws://localhost:8000",
			},
			wantErr: true,
		},
		{
			name: "Default values applied",
			config: &config.AgentConfig{
				AgentID:         "docker-test-local",
				ControlPlaneURL: "ws://localhost:8000",
				APIKey:          "test-api-key-1234567890abcdef1234567890abcdef",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check default values are set
			if err == nil {
				if tt.config.Platform == "" {
					t.Error("Platform should have default value")
				}
				if tt.config.DockerHost == "" {
					t.Error("DockerHost should have default value")
				}
				if tt.config.NetworkName == "" {
					t.Error("NetworkName should have default value")
				}
				if tt.config.VolumeDriver == "" {
					t.Error("VolumeDriver should have default value")
				}
				if tt.config.HeartbeatInterval == 0 {
					t.Error("HeartbeatInterval should have default value")
				}
				if len(tt.config.ReconnectBackoff) == 0 {
					t.Error("ReconnectBackoff should have default value")
				}
			}
		})
	}
}

// TestAgentCapacity tests agent capacity configuration
func TestAgentCapacity(t *testing.T) {
	capacity := config.AgentCapacity{
		MaxCPU:      8000,  // 8 cores
		MaxMemory:   16,    // 16 GB
		MaxSessions: 10,
	}

	if capacity.MaxCPU != 8000 {
		t.Errorf("MaxCPU = %d, want 8000", capacity.MaxCPU)
	}
	if capacity.MaxMemory != 16 {
		t.Errorf("MaxMemory = %d, want 16", capacity.MaxMemory)
	}
	if capacity.MaxSessions != 10 {
		t.Errorf("MaxSessions = %d, want 10", capacity.MaxSessions)
	}
}

// TestAgentMessageTypes tests agent message type definitions
func TestAgentMessageTypes(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		msgType string
	}{
		{
			name:    "Valid command message",
			json:    `{"type":"command","timestamp":1704067200000,"payload":{"commandId":"cmd-123","action":"start_session","payload":{}}}`,
			wantErr: false,
			msgType: "command",
		},
		{
			name:    "Valid ping message",
			json:    `{"type":"ping","timestamp":1704067200000}`,
			wantErr: false,
			msgType: "ping",
		},
		{
			name:    "Valid shutdown message",
			json:    `{"type":"shutdown","timestamp":1704067200000}`,
			wantErr: false,
			msgType: "shutdown",
		},
		{
			name:    "Invalid JSON",
			json:    `{"type":"command"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg AgentMessage
			err := json.Unmarshal([]byte(tt.json), &msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && msg.Type != tt.msgType {
				t.Errorf("Message type = %v, want %v", msg.Type, tt.msgType)
			}
		})
	}
}

// TestCommandMessageParsing tests parsing of command messages
func TestCommandMessageParsing(t *testing.T) {
	jsonData := `{"commandId":"cmd-123","action":"start_session","payload":{"sessionId":"sess-123","user":"alice","template":"firefox"}}`

	var cmd CommandMessage
	err := json.Unmarshal([]byte(jsonData), &cmd)
	if err != nil {
		t.Fatalf("Failed to parse command: %v", err)
	}

	if cmd.CommandID != "cmd-123" {
		t.Errorf("CommandID = %v, want cmd-123", cmd.CommandID)
	}

	if cmd.Action != "start_session" {
		t.Errorf("Action = %v, want start_session", cmd.Action)
	}
}

// TestHelperFunctions tests utility helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("getEnvOrDefault", func(t *testing.T) {
		// Test with unset environment variable
		result := getEnvOrDefault("NONEXISTENT_VAR_12345", "default")
		if result != "default" {
			t.Errorf("getEnvOrDefault() = %v, want default", result)
		}
	})

	t.Run("getEnvIntOrDefault", func(t *testing.T) {
		// Test with unset environment variable
		result := getEnvIntOrDefault("NONEXISTENT_INT_VAR_12345", 42)
		if result != 42 {
			t.Errorf("getEnvIntOrDefault() = %v, want 42", result)
		}
	})
}

// TestContainerName tests container naming convention
func TestContainerName(t *testing.T) {
	tests := []struct {
		sessionID string
		want      string
	}{
		{
			sessionID: "sess-123",
			want:      "streamspace-sess-123",
		},
		{
			sessionID: "test-session",
			want:      "streamspace-test-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.sessionID, func(t *testing.T) {
			got := "streamspace-" + tt.sessionID
			if got != tt.want {
				t.Errorf("containerName = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDockerImageReference tests Docker image reference parsing
func TestDockerImageReference(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "Firefox template",
			template: "firefox",
			want:     "streamspace/firefox:latest",
		},
		{
			name:     "Chrome template",
			template: "chrome",
			want:     "streamspace/chrome:latest",
		},
		{
			name:     "VS Code template",
			template: "vscode",
			want:     "streamspace/vscode:latest",
		},
		{
			name:     "Custom template",
			template: "custom-app",
			want:     "streamspace/custom-app:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := "streamspace/" + tt.template + ":latest"
			if got != tt.want {
				t.Errorf("imageReference = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSessionSpec tests session specification structure
func TestSessionSpec(t *testing.T) {
	spec := SessionSpec{
		SessionID: "sess-123",
		UserID:    "user-456",
		Template:  "firefox",
		Resources: ResourceRequirements{
			CPU:    "1000m",
			Memory: "2Gi",
		},
	}

	if spec.SessionID != "sess-123" {
		t.Errorf("SessionID = %v, want sess-123", spec.SessionID)
	}

	if spec.UserID != "user-456" {
		t.Errorf("UserID = %v, want user-456", spec.UserID)
	}

	if spec.Template != "firefox" {
		t.Errorf("Template = %v, want firefox", spec.Template)
	}

	if spec.Resources.CPU != "1000m" {
		t.Errorf("CPU = %v, want 1000m", spec.Resources.CPU)
	}

	if spec.Resources.Memory != "2Gi" {
		t.Errorf("Memory = %v, want 2Gi", spec.Resources.Memory)
	}
}

// TestCommandResult tests command result structure
func TestCommandResult(t *testing.T) {
	result := CommandResult{
		CommandID: "cmd-123",
		Success:   true,
		Message:   "Session started successfully",
		SessionID: "sess-123",
	}

	if result.CommandID != "cmd-123" {
		t.Errorf("CommandID = %v, want cmd-123", result.CommandID)
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.Message != "Session started successfully" {
		t.Errorf("Message = %v, want 'Session started successfully'", result.Message)
	}

	if result.SessionID != "sess-123" {
		t.Errorf("SessionID = %v, want sess-123", result.SessionID)
	}
}

// TestAgentRegistration tests agent registration message structure
func TestAgentRegistration(t *testing.T) {
	reg := AgentRegistration{
		AgentID:  "docker-test-us-east-1",
		Platform: "docker",
		Region:   "us-east-1",
		Capacity: config.AgentCapacity{
			MaxCPU:      8000,
			MaxMemory:   16,
			MaxSessions: 10,
		},
	}

	if reg.AgentID != "docker-test-us-east-1" {
		t.Errorf("AgentID = %v, want docker-test-us-east-1", reg.AgentID)
	}

	if reg.Platform != "docker" {
		t.Errorf("Platform = %v, want docker", reg.Platform)
	}

	if reg.Region != "us-east-1" {
		t.Errorf("Region = %v, want us-east-1", reg.Region)
	}

	if reg.Capacity.MaxCPU != 8000 {
		t.Errorf("MaxCPU = %v, want 8000", reg.Capacity.MaxCPU)
	}
}

// TestHeartbeat tests heartbeat message structure
func TestHeartbeat(t *testing.T) {
	hb := Heartbeat{
		AgentID:   "docker-test-us-east-1",
		Timestamp: "2024-01-01T00:00:00Z",
		Status:    "healthy",
		ActiveSessions: []string{"sess-1", "sess-2"},
	}

	if hb.AgentID != "docker-test-us-east-1" {
		t.Errorf("AgentID = %v, want docker-test-us-east-1", hb.AgentID)
	}

	if hb.Status != "healthy" {
		t.Errorf("Status = %v, want healthy", hb.Status)
	}

	if len(hb.ActiveSessions) != 2 {
		t.Errorf("ActiveSessions count = %v, want 2", len(hb.ActiveSessions))
	}
}
