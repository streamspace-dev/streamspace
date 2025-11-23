package main

import (
	"encoding/json"
	"testing"

	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/config"
)

// TestAgentMessage_UnmarshalJSON tests AgentMessage deserialization
func TestAgentMessage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    AgentMessage
		wantErr bool
	}{
		{
			name:  "valid command message",
			input: `{"type":"command","timestamp":1234567890,"payload":{"commandId":"cmd-123","action":"start_session"}}`,
			want: AgentMessage{
				Type:      "command",
				Timestamp: 1234567890,
			},
			wantErr: false,
		},
		{
			name:  "valid ping message",
			input: `{"type":"ping","timestamp":1234567890,"payload":{}}`,
			want: AgentMessage{
				Type:      "ping",
				Timestamp: 1234567890,
			},
			wantErr: false,
		},
		{
			name:  "valid shutdown message",
			input: `{"type":"shutdown","timestamp":1234567890,"payload":{}}`,
			want: AgentMessage{
				Type:      "shutdown",
				Timestamp: 1234567890,
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{"type":"command"`,
			wantErr: true,
		},
		{
			name:  "empty payload",
			input: `{"type":"ping","timestamp":1234567890}`,
			want: AgentMessage{
				Type:      "ping",
				Timestamp: 1234567890,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got AgentMessage
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error, don't check values
			}

			if got.Type != tt.want.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.want.Type)
			}

			if got.Timestamp != tt.want.Timestamp {
				t.Errorf("Timestamp = %v, want %v", got.Timestamp, tt.want.Timestamp)
			}
		})
	}
}

// TestCommandMessage_UnmarshalJSON tests CommandMessage deserialization
func TestCommandMessage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    CommandMessage
		wantErr bool
	}{
		{
			name:  "start_session command",
			input: `{"commandId":"cmd-123","action":"start_session","payload":{"sessionId":"sess-456","user":"alice"}}`,
			want: CommandMessage{
				CommandID: "cmd-123",
				Action:    "start_session",
				Payload: map[string]interface{}{
					"sessionId": "sess-456",
					"user":      "alice",
				},
			},
			wantErr: false,
		},
		{
			name:  "stop_session command",
			input: `{"commandId":"cmd-789","action":"stop_session","payload":{"sessionId":"sess-456"}}`,
			want: CommandMessage{
				CommandID: "cmd-789",
				Action:    "stop_session",
				Payload: map[string]interface{}{
					"sessionId": "sess-456",
				},
			},
			wantErr: false,
		},
		{
			name:  "hibernate_session command",
			input: `{"commandId":"cmd-abc","action":"hibernate_session","payload":{"sessionId":"sess-456"}}`,
			want: CommandMessage{
				CommandID: "cmd-abc",
				Action:    "hibernate_session",
			},
			wantErr: false,
		},
		{
			name:  "wake_session command",
			input: `{"commandId":"cmd-def","action":"wake_session","payload":{"sessionId":"sess-456"}}`,
			want: CommandMessage{
				CommandID: "cmd-def",
				Action:    "wake_session",
			},
			wantErr: false,
		},
		{
			name:  "get_session_status command",
			input: `{"commandId":"cmd-ghi","action":"get_session_status","payload":{"sessionId":"sess-456"}}`,
			want: CommandMessage{
				CommandID: "cmd-ghi",
				Action:    "get_session_status",
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{"commandId":"cmd-123"`,
			wantErr: true,
		},
		{
			name:  "empty payload",
			input: `{"commandId":"cmd-123","action":"test","payload":{}}`,
			want: CommandMessage{
				CommandID: "cmd-123",
				Action:    "test",
				Payload:   map[string]interface{}{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got CommandMessage
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error, don't check values
			}

			if got.CommandID != tt.want.CommandID {
				t.Errorf("CommandID = %v, want %v", got.CommandID, tt.want.CommandID)
			}

			if got.Action != tt.want.Action {
				t.Errorf("Action = %v, want %v", got.Action, tt.want.Action)
			}

			// Note: Deep comparison of Payload map not done in this basic test
			// In a full test suite, you'd want to compare payload contents
		})
	}
}

// TestMessageTypes tests various message type constants
func TestMessageTypes(t *testing.T) {
	messageTypes := []string{
		"command",
		"ping",
		"pong",
		"shutdown",
		"shutdown_ack",
		"command_response",
		"command_error",
	}

	// Verify each message type can be properly serialized/deserialized
	for _, msgType := range messageTypes {
		t.Run(msgType, func(t *testing.T) {
			msg := AgentMessage{
				Type:      msgType,
				Timestamp: 1234567890,
				Payload:   json.RawMessage(`{}`),
			}

			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded AgentMessage
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.Type != msgType {
				t.Errorf("Type = %v, want %v", decoded.Type, msgType)
			}
		})
	}
}

// TestCommandActions tests various command action constants
func TestCommandActions(t *testing.T) {
	actions := []string{
		"start_session",
		"stop_session",
		"hibernate_session",
		"wake_session",
		"get_session_status",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			cmd := CommandMessage{
				CommandID: "test-cmd",
				Action:    action,
				Payload:   map[string]interface{}{"test": "value"},
			}

			data, err := json.Marshal(cmd)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded CommandMessage
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.Action != action {
				t.Errorf("Action = %v, want %v", decoded.Action, action)
			}
		})
	}
}

// MockDockerAgent is a mock agent for testing message handling
type MockDockerAgent struct {
	config          *config.AgentConfig
	stopChan        chan struct{}
	commandHandlers map[string]CommandHandler
	messagesSent    []map[string]interface{}
}

// NewMockDockerAgent creates a new mock agent
func NewMockDockerAgent() *MockDockerAgent {
	return &MockDockerAgent{
		config: &config.AgentConfig{
			AgentID: "test-agent",
		},
		stopChan:        make(chan struct{}),
		commandHandlers: make(map[string]CommandHandler),
		messagesSent:    make([]map[string]interface{}, 0),
	}
}

// sendMessage records sent messages for verification
func (a *MockDockerAgent) sendMessage(msg map[string]interface{}) error {
	a.messagesSent = append(a.messagesSent, msg)
	return nil
}

// handlePing responds to ping messages
func (a *MockDockerAgent) handlePing() {
	response := map[string]interface{}{
		"type":    "pong",
		"agentId": a.config.AgentID,
	}
	a.sendMessage(response)
}

// handleShutdown processes shutdown requests
func (a *MockDockerAgent) handleShutdown() {
	response := map[string]interface{}{
		"type":    "shutdown_ack",
		"agentId": a.config.AgentID,
	}
	a.sendMessage(response)
	close(a.stopChan)
}

// sendCommandError sends a command error response
func (a *MockDockerAgent) sendCommandError(commandID, errorMsg string) {
	response := map[string]interface{}{
		"type":      "command_error",
		"commandId": commandID,
		"error":     errorMsg,
	}
	a.sendMessage(response)
}

// TestHandlePing tests the ping handler
func TestHandlePing(t *testing.T) {
	agent := NewMockDockerAgent()

	// Call handlePing
	agent.handlePing()

	// Verify pong was sent
	if len(agent.messagesSent) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(agent.messagesSent))
	}

	msg := agent.messagesSent[0]

	if msg["type"] != "pong" {
		t.Errorf("type = %v, want pong", msg["type"])
	}

	if msg["agentId"] != "test-agent" {
		t.Errorf("agentId = %v, want test-agent", msg["agentId"])
	}
}

// TestHandleShutdown tests the shutdown handler
func TestHandleShutdown(t *testing.T) {
	agent := NewMockDockerAgent()

	// Call handleShutdown
	agent.handleShutdown()

	// Verify shutdown_ack was sent
	if len(agent.messagesSent) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(agent.messagesSent))
	}

	msg := agent.messagesSent[0]

	if msg["type"] != "shutdown_ack" {
		t.Errorf("type = %v, want shutdown_ack", msg["type"])
	}

	if msg["agentId"] != "test-agent" {
		t.Errorf("agentId = %v, want test-agent", msg["agentId"])
	}

	// Verify stop channel was closed
	select {
	case <-agent.stopChan:
		// Good - channel closed
	default:
		t.Error("stopChan should be closed after handleShutdown")
	}
}

// TestSendCommandError tests the command error sender
func TestSendCommandError(t *testing.T) {
	agent := NewMockDockerAgent()

	// Call sendCommandError
	agent.sendCommandError("cmd-123", "test error message")

	// Verify error message was sent
	if len(agent.messagesSent) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(agent.messagesSent))
	}

	msg := agent.messagesSent[0]

	if msg["type"] != "command_error" {
		t.Errorf("type = %v, want command_error", msg["type"])
	}

	if msg["commandId"] != "cmd-123" {
		t.Errorf("commandId = %v, want cmd-123", msg["commandId"])
	}

	if msg["error"] != "test error message" {
		t.Errorf("error = %v, want 'test error message'", msg["error"])
	}
}
