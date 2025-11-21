package errors

import stderrors "errors"

// Configuration errors
var (
	ErrMissingAgentID         = stderrors.New("agent ID is required")
	ErrMissingControlPlaneURL = stderrors.New("control plane URL is required")
	ErrInvalidPlatform        = stderrors.New("invalid platform type")
)

// Connection errors
var (
	ErrNotConnected       = stderrors.New("not connected to Control Plane")
	ErrConnectionClosed   = stderrors.New("connection closed")
	ErrRegistrationFailed = stderrors.New("agent registration failed")
	ErrWebSocketUpgrade   = stderrors.New("WebSocket upgrade failed")
)

// Command errors
var (
	ErrUnknownCommand    = stderrors.New("unknown command action")
	ErrInvalidPayload    = stderrors.New("invalid command payload")
	ErrCommandFailed     = stderrors.New("command execution failed")
	ErrSessionNotFound   = stderrors.New("session not found")
	ErrTemplateNotFound  = stderrors.New("template not found")
	ErrResourceNotFound  = stderrors.New("Kubernetes resource not found")
)

// Kubernetes errors
var (
	ErrDeploymentCreation = stderrors.New("failed to create deployment")
	ErrServiceCreation    = stderrors.New("failed to create service")
	ErrPVCCreation        = stderrors.New("failed to create PVC")
	ErrPodNotReady        = stderrors.New("pod not ready")
	ErrScalingFailed      = stderrors.New("scaling failed")
)
