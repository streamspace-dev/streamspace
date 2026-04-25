package main

import (
	"encoding/json"
	"testing"

	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/config"
)

// TestStartSessionHandler_PayloadValidation tests payload validation
func TestStartSessionHandler_PayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
		errText string
	}{
		{
			name:    "missing sessionId",
			payload: `{"user": "alice"}`,
			wantErr: true,
			errText: "sessionId not found in payload",
		},
		{
			name:    "missing user",
			payload: `{"sessionId": "session-123"}`,
			wantErr: true,
			errText: "user not found in payload",
		},
		{
			name:    "empty sessionId",
			payload: `{"sessionId": "", "user": "alice"}`,
			wantErr: true,
			errText: "sessionId not found in payload",
		},
		{
			name:    "empty user",
			payload: `{"sessionId": "session-123", "user": ""}`,
			wantErr: true,
			errText: "user not found in payload",
		},
		{
			name:    "invalid JSON",
			payload: `{"sessionId": "session-123"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal handler for payload validation testing
			handler := &StartSessionHandler{
				dockerClient: nil,
				config: &config.AgentConfig{
					AgentID: "test-agent",
				},
			}

			err := handler.Handle(json.RawMessage(tt.payload))

			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errText != "" && err != nil && err.Error() != tt.errText {
				t.Logf("Error message: %v", err.Error())
			}
		})
	}
}


// TestStopSessionHandler_PayloadValidation tests payload validation
func TestStopSessionHandler_PayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
		errText string
	}{
		{
			name:    "missing sessionId",
			payload: `{"user": "alice"}`,
			wantErr: true,
			errText: "sessionId not found in payload",
		},
		{
			name:    "empty sessionId",
			payload: `{"sessionId": ""}`,
			wantErr: true,
			errText: "sessionId not found in payload",
		},
		{
			name:    "invalid JSON",
			payload: `{"sessionId": "session-123"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &StopSessionHandler{
				dockerClient: nil,
				config: &config.AgentConfig{
					AgentID: "test-agent",
				},
			}

			err := handler.Handle(json.RawMessage(tt.payload))

			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errText != "" && err != nil && err.Error() != tt.errText {
				t.Logf("Error message: %v", err.Error())
			}
		})
	}
}

// TestHibernateSessionHandler_Handle tests the hibernate session handler
func TestHibernateSessionHandler_Handle(t *testing.T) {
	handler := &HibernateSessionHandler{}

	payload := json.RawMessage(`{"sessionId": "session-123"}`)
	err := handler.Handle(payload)

	if err == nil {
		t.Error("Handle() error = nil, want error (not yet implemented)")
	}

	expectedMsg := "hibernation not yet implemented for Docker agent"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %v, want %v", err.Error(), expectedMsg)
	}
}

// TestWakeSessionHandler_Handle tests the wake session handler
func TestWakeSessionHandler_Handle(t *testing.T) {
	handler := &WakeSessionHandler{}

	payload := json.RawMessage(`{"sessionId": "session-123"}`)
	err := handler.Handle(payload)

	if err == nil {
		t.Error("Handle() error = nil, want error (not yet implemented)")
	}

	expectedMsg := "wake not yet implemented for Docker agent"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %v, want %v", err.Error(), expectedMsg)
	}
}

// TestNewStartSessionHandler tests creating a new start session handler
func TestNewStartSessionHandler(t *testing.T) {
	cfg := &config.AgentConfig{
		AgentID: "test-agent",
	}

	agent := &DockerAgent{
		config: cfg,
	}

	handler := NewStartSessionHandler(nil, cfg, agent)

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.config != cfg {
		t.Error("config not set correctly")
	}

	if handler.agent != agent {
		t.Error("agent not set correctly")
	}
}

// TestNewStopSessionHandler tests creating a new stop session handler
func TestNewStopSessionHandler(t *testing.T) {
	cfg := &config.AgentConfig{
		AgentID: "test-agent",
	}

	agent := &DockerAgent{
		config: cfg,
	}

	handler := NewStopSessionHandler(nil, cfg, agent)

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.config != cfg {
		t.Error("config not set correctly")
	}

	if handler.agent != agent {
		t.Error("agent not set correctly")
	}
}

// TestNewHibernateSessionHandler tests creating a new hibernate session handler
func TestNewHibernateSessionHandler(t *testing.T) {
	cfg := &config.AgentConfig{
		AgentID: "test-agent",
	}

	handler := NewHibernateSessionHandler(nil, cfg)

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.config != cfg {
		t.Error("config not set correctly")
	}
}

// TestNewWakeSessionHandler tests creating a new wake session handler
func TestNewWakeSessionHandler(t *testing.T) {
	cfg := &config.AgentConfig{
		AgentID: "test-agent",
	}

	handler := NewWakeSessionHandler(nil, cfg)

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.config != cfg {
		t.Error("config not set correctly")
	}
}

