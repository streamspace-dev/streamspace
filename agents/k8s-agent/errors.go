package main

import "errors"

// Configuration errors
var (
	ErrMissingAgentID         = errors.New("agent ID is required")
	ErrMissingControlPlaneURL = errors.New("control plane URL is required")
	ErrInvalidPlatform        = errors.New("invalid platform type")
)

// Connection errors
var (
	ErrNotConnected       = errors.New("not connected to Control Plane")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrRegistrationFailed = errors.New("agent registration failed")
	ErrWebSocketUpgrade   = errors.New("WebSocket upgrade failed")
)

// Command errors
var (
	ErrUnknownCommand    = errors.New("unknown command action")
	ErrInvalidPayload    = errors.New("invalid command payload")
	ErrCommandFailed     = errors.New("command execution failed")
	ErrSessionNotFound   = errors.New("session not found")
	ErrTemplateNotFound  = errors.New("template not found")
	ErrResourceNotFound  = errors.New("Kubernetes resource not found")
)

// Kubernetes errors
var (
	ErrDeploymentCreation = errors.New("failed to create deployment")
	ErrServiceCreation    = errors.New("failed to create service")
	ErrPVCCreation        = errors.New("failed to create PVC")
	ErrPodNotReady        = errors.New("pod not ready")
	ErrScalingFailed      = errors.New("scaling failed")
)
