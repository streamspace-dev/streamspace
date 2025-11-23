package errors

import (
	"errors"
	"testing"
)

// TestConfigurationErrors tests that configuration errors are defined correctly
func TestConfigurationErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantText string
	}{
		{
			name:     "ErrMissingAgentID",
			err:      ErrMissingAgentID,
			wantText: "agent ID is required",
		},
		{
			name:     "ErrMissingControlPlaneURL",
			err:      ErrMissingControlPlaneURL,
			wantText: "control plane URL is required",
		},
		{
			name:     "ErrMissingAPIKey",
			err:      ErrMissingAPIKey,
			wantText: "agent API key is required",
		},
		{
			name:     "ErrInvalidPlatform",
			err:      ErrInvalidPlatform,
			wantText: "invalid platform type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error is nil")
			}

			if tt.err.Error() != tt.wantText {
				t.Errorf("error text = %q, want %q", tt.err.Error(), tt.wantText)
			}
		})
	}
}

// TestConnectionErrors tests that connection errors are defined correctly
func TestConnectionErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantText string
	}{
		{
			name:     "ErrNotConnected",
			err:      ErrNotConnected,
			wantText: "not connected to Control Plane",
		},
		{
			name:     "ErrConnectionClosed",
			err:      ErrConnectionClosed,
			wantText: "connection closed",
		},
		{
			name:     "ErrRegistrationFailed",
			err:      ErrRegistrationFailed,
			wantText: "agent registration failed",
		},
		{
			name:     "ErrWebSocketUpgrade",
			err:      ErrWebSocketUpgrade,
			wantText: "WebSocket upgrade failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error is nil")
			}

			if tt.err.Error() != tt.wantText {
				t.Errorf("error text = %q, want %q", tt.err.Error(), tt.wantText)
			}
		})
	}
}

// TestCommandErrors tests that command errors are defined correctly
func TestCommandErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantText string
	}{
		{
			name:     "ErrUnknownCommand",
			err:      ErrUnknownCommand,
			wantText: "unknown command action",
		},
		{
			name:     "ErrInvalidPayload",
			err:      ErrInvalidPayload,
			wantText: "invalid command payload",
		},
		{
			name:     "ErrCommandFailed",
			err:      ErrCommandFailed,
			wantText: "command execution failed",
		},
		{
			name:     "ErrSessionNotFound",
			err:      ErrSessionNotFound,
			wantText: "session not found",
		},
		{
			name:     "ErrTemplateNotFound",
			err:      ErrTemplateNotFound,
			wantText: "template not found",
		},
		{
			name:     "ErrResourceNotFound",
			err:      ErrResourceNotFound,
			wantText: "Docker resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error is nil")
			}

			if tt.err.Error() != tt.wantText {
				t.Errorf("error text = %q, want %q", tt.err.Error(), tt.wantText)
			}
		})
	}
}

// TestDockerErrors tests that Docker errors are defined correctly
func TestDockerErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantText string
	}{
		{
			name:     "ErrContainerCreation",
			err:      ErrContainerCreation,
			wantText: "failed to create container",
		},
		{
			name:     "ErrNetworkCreation",
			err:      ErrNetworkCreation,
			wantText: "failed to create network",
		},
		{
			name:     "ErrVolumeCreation",
			err:      ErrVolumeCreation,
			wantText: "failed to create volume",
		},
		{
			name:     "ErrContainerNotReady",
			err:      ErrContainerNotReady,
			wantText: "container not ready",
		},
		{
			name:     "ErrContainerStart",
			err:      ErrContainerStart,
			wantText: "failed to start container",
		},
		{
			name:     "ErrContainerStop",
			err:      ErrContainerStop,
			wantText: "failed to stop container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error is nil")
			}

			if tt.err.Error() != tt.wantText {
				t.Errorf("error text = %q, want %q", tt.err.Error(), tt.wantText)
			}
		})
	}
}

// TestErrorIs tests that errors.Is works correctly with our errors
func TestErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		target error
		err    error
		want   bool
	}{
		{
			name:   "same error",
			target: ErrMissingAgentID,
			err:    ErrMissingAgentID,
			want:   true,
		},
		{
			name:   "different errors",
			target: ErrMissingAgentID,
			err:    ErrMissingAPIKey,
			want:   false,
		},
		{
			name:   "nil error",
			target: ErrMissingAgentID,
			err:    nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorUniqueness tests that all errors have unique messages
func TestErrorUniqueness(t *testing.T) {
	allErrors := []error{
		// Configuration
		ErrMissingAgentID,
		ErrMissingControlPlaneURL,
		ErrMissingAPIKey,
		ErrInvalidPlatform,
		// Connection
		ErrNotConnected,
		ErrConnectionClosed,
		ErrRegistrationFailed,
		ErrWebSocketUpgrade,
		// Command
		ErrUnknownCommand,
		ErrInvalidPayload,
		ErrCommandFailed,
		ErrSessionNotFound,
		ErrTemplateNotFound,
		ErrResourceNotFound,
		// Docker
		ErrContainerCreation,
		ErrNetworkCreation,
		ErrVolumeCreation,
		ErrContainerNotReady,
		ErrContainerStart,
		ErrContainerStop,
	}

	messages := make(map[string]bool)

	for _, err := range allErrors {
		msg := err.Error()
		if messages[msg] {
			t.Errorf("duplicate error message found: %q", msg)
		}
		messages[msg] = true
	}

	t.Logf("Verified %d unique error messages", len(messages))
}
