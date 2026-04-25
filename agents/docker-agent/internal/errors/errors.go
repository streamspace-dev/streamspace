package errors

import stderrors "errors"

// Configuration errors
var (
	ErrMissingAgentID         = stderrors.New("agent ID is required")
	ErrMissingControlPlaneURL = stderrors.New("control plane URL is required")
	ErrMissingAPIKey          = stderrors.New("agent API key is required")
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
	ErrResourceNotFound  = stderrors.New("Docker resource not found")
)

// Docker errors
var (
	ErrContainerCreation = stderrors.New("failed to create container")
	ErrNetworkCreation   = stderrors.New("failed to create network")
	ErrVolumeCreation    = stderrors.New("failed to create volume")
	ErrContainerNotReady = stderrors.New("container not ready")
	ErrContainerStart    = stderrors.New("failed to start container")
	ErrContainerStop     = stderrors.New("failed to stop container")
)
