package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/streamspace-dev/streamspace/agents/k8s-agent/internal/config"
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
				AgentID:         "k8s-test-local",
				ControlPlaneURL: "ws://localhost:8000",
				Platform:        "kubernetes",
				Region:          "us-east-1",
				Namespace:       "streamspace",
			},
			wantErr: false,
		},
		{
			name: "Missing agent ID",
			config: &config.AgentConfig{
				ControlPlaneURL: "ws://localhost:8000",
			},
			wantErr: true,
		},
		{
			name: "Missing control plane URL",
			config: &config.AgentConfig{
				AgentID: "k8s-test-local",
			},
			wantErr: true,
		},
		{
			name: "Default values applied",
			config: &config.AgentConfig{
				AgentID:         "k8s-test-local",
				ControlPlaneURL: "ws://localhost:8000",
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

			if !tt.wantErr {
				// Check defaults were applied
				if tt.config.Platform == "" {
					t.Error("Platform should have default value")
				}
				if tt.config.Namespace == "" {
					t.Error("Namespace should have default value")
				}
				if tt.config.HeartbeatInterval == 0 {
					t.Error("HeartbeatInterval should have default value")
				}
			}
		})
	}
}

// TestConvertToHTTPURL tests WebSocket URL to HTTP URL conversion
func TestConvertToHTTPURL(t *testing.T) {
	tests := []struct {
		name  string
		wsURL string
		want  string
	}{
		{
			name:  "wss to https",
			wsURL: "wss://control.example.com",
			want:  "https://control.example.com",
		},
		{
			name:  "ws to http",
			wsURL: "ws://localhost:8000",
			want:  "http://localhost:8000",
		},
		{
			name:  "already http",
			wsURL: "http://localhost:8000",
			want:  "http://localhost:8000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToHTTPURL(tt.wsURL)
			if got != tt.want {
				t.Errorf("convertToHTTPURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAgentMessageParsing tests parsing of agent messages
func TestAgentMessageParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		msgType string
	}{
		{
			name:    "Valid command message",
			json:    `{"type":"command","timestamp":"2024-01-01T00:00:00Z","payload":{"commandId":"cmd-123","action":"start_session","payload":{}}}`,
			wantErr: false,
			msgType: "command",
		},
		{
			name:    "Valid ping message",
			json:    `{"type":"ping","timestamp":"2024-01-01T00:00:00Z","payload":{}}`,
			wantErr: false,
			msgType: "ping",
		},
		{
			name:    "Valid shutdown message",
			json:    `{"type":"shutdown","timestamp":"2024-01-01T00:00:00Z","payload":{"reason":"maintenance"}}`,
			wantErr: false,
			msgType: "shutdown",
		},
		{
			name:    "Invalid JSON",
			json:    `{invalid json}`,
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
	json := `{"commandId":"cmd-123","action":"start_session","payload":{"sessionId":"sess-123","user":"alice","template":"firefox"}}`

	var cmd CommandMessage
	err := json.Unmarshal([]byte(json), &cmd)
	if err != nil {
		t.Fatalf("Failed to parse command: %v", err)
	}

	if cmd.CommandID != "cmd-123" {
		t.Errorf("CommandID = %v, want cmd-123", cmd.CommandID)
	}

	if cmd.Action != "start_session" {
		t.Errorf("Action = %v, want start_session", cmd.Action)
	}

	sessionID, ok := cmd.Payload["sessionId"].(string)
	if !ok || sessionID != "sess-123" {
		t.Errorf("sessionId = %v, want sess-123", sessionID)
	}
}

// TestHelperFunctions tests utility functions
func TestHelperFunctions(t *testing.T) {
	t.Run("getBoolOrDefault", func(t *testing.T) {
		payload := map[string]interface{}{
			"enabled": true,
		}

		if !getBoolOrDefault(payload, "enabled", false) {
			t.Error("Should return true for existing key")
		}

		if getBoolOrDefault(payload, "missing", false) {
			t.Error("Should return false for missing key")
		}

		if !getBoolOrDefault(payload, "missing", true) {
			t.Error("Should return default true for missing key")
		}
	})

	t.Run("getStringOrDefault", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "test",
		}

		if getStringOrDefault(payload, "name", "default") != "test" {
			t.Error("Should return 'test' for existing key")
		}

		if getStringOrDefault(payload, "missing", "default") != "default" {
			t.Error("Should return 'default' for missing key")
		}
	})
}

// TestGetTemplateImage tests template image mapping
func TestGetTemplateImage(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "Firefox template",
			template: "firefox",
			want:     "lscr.io/linuxserver/firefox:latest",
		},
		{
			name:     "Chrome template",
			template: "chrome",
			want:     "lscr.io/linuxserver/chromium:latest",
		},
		{
			name:     "VS Code template",
			template: "vscode",
			want:     "lscr.io/linuxserver/code-server:latest",
		},
		{
			name:     "Unknown template",
			template: "unknown",
			want:     "lscr.io/linuxserver/firefox:latest", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTemplateImage(tt.template)
			if got != tt.want {
				t.Errorf("getTemplateImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSessionSpec tests session specification parsing
func TestSessionSpec(t *testing.T) {
	payload := map[string]interface{}{
		"sessionId":      "sess-123",
		"user":           "alice",
		"template":       "firefox",
		"persistentHome": true,
		"memory":         "2Gi",
		"cpu":            "1000m",
	}

	sessionID, _ := payload["sessionId"].(string)
	user, _ := payload["user"].(string)
	template, _ := payload["template"].(string)

	spec := &SessionSpec{
		SessionID:      sessionID,
		User:           user,
		Template:       template,
		PersistentHome: getBoolOrDefault(payload, "persistentHome", false),
		Memory:         getStringOrDefault(payload, "memory", "2Gi"),
		CPU:            getStringOrDefault(payload, "cpu", "1000m"),
	}

	if spec.SessionID != "sess-123" {
		t.Errorf("SessionID = %v, want sess-123", spec.SessionID)
	}

	if spec.User != "alice" {
		t.Errorf("User = %v, want alice", spec.User)
	}

	if !spec.PersistentHome {
		t.Error("PersistentHome should be true")
	}
}

// TestCommandResult tests command result structure
func TestCommandResult(t *testing.T) {
	result := &CommandResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId": "sess-123",
			"state":     "running",
			"podIP":     "10.0.0.1",
		},
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	sessionID, ok := result.Data["sessionId"].(string)
	if !ok || sessionID != "sess-123" {
		t.Errorf("sessionId = %v, want sess-123", sessionID)
	}

	state, ok := result.Data["state"].(string)
	if !ok || state != "running" {
		t.Errorf("state = %v, want running", state)
	}
}

// Benchmark tests
func BenchmarkAgentMessageParsing(b *testing.B) {
	jsonStr := `{"type":"command","timestamp":"2024-01-01T00:00:00Z","payload":{"commandId":"cmd-123","action":"start_session","payload":{}}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var msg AgentMessage
		json.Unmarshal([]byte(jsonStr), &msg)
	}
}

func BenchmarkConvertToHTTPURL(b *testing.B) {
	wsURL := "wss://control.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		convertToHTTPURL(wsURL)
	}
}
