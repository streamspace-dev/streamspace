package main
import "github.com/streamspace/streamspace/agents/k8s-agent/internal/config"

import (
	"fmt"
	"log"

	"k8s.io/client-go/kubernetes"
)

// CommandHandler defines the interface for command execution.
type CommandHandler interface {
	Handle(cmd *CommandMessage) (*CommandResult, error)
}

// CommandResult represents the result of a command execution.
type CommandResult struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// SessionSpec represents a session specification from the command payload.
type SessionSpec struct {
	SessionID       string `json:"sessionId"`
	User            string `json:"user"`
	Template        string `json:"template"`
	PersistentHome  bool   `json:"persistentHome"`
	Memory          string `json:"memory"`
	CPU             string `json:"cpu"`
}

// StartSessionHandler handles start_session commands.
type StartSessionHandler struct {
	kubeClient *kubernetes.Clientset
	config     *config.AgentConfig
	agent      *K8sAgent
}

// NewStartSessionHandler creates a new start session handler.
func NewStartSessionHandler(kubeClient *kubernetes.Clientset, config *config.AgentConfig, agent *K8sAgent) *StartSessionHandler {
	return &StartSessionHandler{
		kubeClient: kubeClient,
		config:     config,
		agent:      agent,
	}
}

// Handle executes the start_session command.
//
// Steps:
//  1. Parse session spec from command payload
//  2. Create Deployment (from template)
//  3. Create Service (ClusterIP)
//  4. Create PVC (if persistentHome enabled)
//  5. Wait for pod to be Running
//  6. Get pod IP and VNC port
//  7. Return result with session metadata
func (h *StartSessionHandler) Handle(cmd *CommandMessage) (*CommandResult, error) {
	log.Printf("[StartSessionHandler] Starting session from command %s", cmd.CommandID)

	// Parse session spec
	sessionID, ok := cmd.Payload["sessionId"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("missing or invalid sessionId")
	}

	user, ok := cmd.Payload["user"].(string)
	if !ok || user == "" {
		return nil, fmt.Errorf("missing or invalid user")
	}

	template, ok := cmd.Payload["template"].(string)
	if !ok || template == "" {
		return nil, fmt.Errorf("missing or invalid template")
	}

	spec := &SessionSpec{
		SessionID:      sessionID,
		User:           user,
		Template:       template,
		PersistentHome: getBoolOrDefault(cmd.Payload, "persistentHome", false),
		Memory:         getStringOrDefault(cmd.Payload, "memory", "2Gi"),
		CPU:            getStringOrDefault(cmd.Payload, "cpu", "1000m"),
	}

	log.Printf("[StartSessionHandler] Session spec: user=%s, template=%s, persistent=%v",
		spec.User, spec.Template, spec.PersistentHome)

	// Create Kubernetes resources
	deployment, err := createSessionDeployment(h.kubeClient, h.config.Namespace, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	service, err := createSessionService(h.kubeClient, h.config.Namespace, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	var pvcName string
	if spec.PersistentHome {
		pvc, err := createSessionPVC(h.kubeClient, h.config.Namespace, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create PVC: %w", err)
		}
		pvcName = pvc.Name
	}

	// Wait for pod to be ready
	podIP, err := waitForPodReady(h.kubeClient, h.config.Namespace, sessionID, 120)
	if err != nil {
		return nil, fmt.Errorf("pod not ready: %w", err)
	}

	log.Printf("[StartSessionHandler] Session %s started successfully (pod IP: %s)", sessionID, podIP)

	// Initialize VNC tunnel for this session
	if h.agent != nil {
		if err := h.agent.initVNCTunnelForSession(sessionID); err != nil {
			log.Printf("[StartSessionHandler] Warning: Failed to init VNC tunnel: %v", err)
			// Don't fail the command - VNC can be established later
		}
	}

	// Return success result
	return &CommandResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId":  sessionID,
			"deployment": deployment.Name,
			"service":    service.Name,
			"pvc":        pvcName,
			"podIP":      podIP,
			"vncPort":    3000, // Default VNC port
			"state":      "running",
		},
	}, nil
}

// StopSessionHandler handles stop_session commands.
type StopSessionHandler struct {
	kubeClient *kubernetes.Clientset
	config     *config.AgentConfig
	agent      *K8sAgent
}

// NewStopSessionHandler creates a new stop session handler.
func NewStopSessionHandler(kubeClient *kubernetes.Clientset, config *config.AgentConfig, agent *K8sAgent) *StopSessionHandler {
	return &StopSessionHandler{
		kubeClient: kubeClient,
		config:     config,
		agent:      agent,
	}
}

// Handle executes the stop_session command.
//
// Steps:
//  1. Parse session ID from command payload
//  2. Delete Deployment
//  3. Delete Service
//  4. Optionally delete PVC (if not persistent)
//  5. Return success result
func (h *StopSessionHandler) Handle(cmd *CommandMessage) (*CommandResult, error) {
	log.Printf("[StopSessionHandler] Stopping session from command %s", cmd.CommandID)

	// Parse session ID
	sessionID, ok := cmd.Payload["sessionId"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("missing or invalid sessionId")
	}

	shouldDeletePVC := getBoolOrDefault(cmd.Payload, "deletePVC", false)

	log.Printf("[StopSessionHandler] Deleting resources for session %s (deletePVC: %v)", sessionID, shouldDeletePVC)

	// Close VNC tunnel for this session
	if h.agent != nil && h.agent.vncManager != nil {
		if err := h.agent.vncManager.CloseTunnel(sessionID); err != nil {
			log.Printf("[StopSessionHandler] Warning: Failed to close VNC tunnel: %v", err)
		}
	}

	// Delete Deployment
	if err := deleteDeployment(h.kubeClient, h.config.Namespace, sessionID); err != nil {
		log.Printf("[StopSessionHandler] Warning: Failed to delete deployment: %v", err)
	}

	// Delete Service
	if err := deleteService(h.kubeClient, h.config.Namespace, sessionID); err != nil {
		log.Printf("[StopSessionHandler] Warning: Failed to delete service: %v", err)
	}

	// Delete PVC if requested
	if shouldDeletePVC {
		if err := deletePVC(h.kubeClient, h.config.Namespace, sessionID); err != nil {
			log.Printf("[StopSessionHandler] Warning: Failed to delete PVC: %v", err)
		}
	}

	log.Printf("[StopSessionHandler] Session %s stopped successfully", sessionID)

	return &CommandResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId": sessionID,
			"state":     "terminated",
		},
	}, nil
}

// HibernateSessionHandler handles hibernate_session commands.
type HibernateSessionHandler struct {
	kubeClient *kubernetes.Clientset
	config     *config.AgentConfig
}

// NewHibernateSessionHandler creates a new hibernate session handler.
func NewHibernateSessionHandler(kubeClient *kubernetes.Clientset, config *config.AgentConfig) *HibernateSessionHandler {
	return &HibernateSessionHandler{
		kubeClient: kubeClient,
		config:     config,
	}
}

// Handle executes the hibernate_session command.
//
// Steps:
//  1. Parse session ID
//  2. Scale deployment to 0 replicas
//  3. Return success result
func (h *HibernateSessionHandler) Handle(cmd *CommandMessage) (*CommandResult, error) {
	log.Printf("[HibernateSessionHandler] Hibernating session from command %s", cmd.CommandID)

	// Parse session ID
	sessionID, ok := cmd.Payload["sessionId"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("missing or invalid sessionId")
	}

	log.Printf("[HibernateSessionHandler] Scaling deployment to 0 replicas for session %s", sessionID)

	// Scale deployment to 0
	if err := scaleDeployment(h.kubeClient, h.config.Namespace, sessionID, 0); err != nil {
		return nil, fmt.Errorf("failed to scale deployment: %w", err)
	}

	log.Printf("[HibernateSessionHandler] Session %s hibernated successfully", sessionID)

	return &CommandResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId": sessionID,
			"state":     "hibernated",
		},
	}, nil
}

// WakeSessionHandler handles wake_session commands.
type WakeSessionHandler struct {
	kubeClient *kubernetes.Clientset
	config     *config.AgentConfig
}

// NewWakeSessionHandler creates a new wake session handler.
func NewWakeSessionHandler(kubeClient *kubernetes.Clientset, config *config.AgentConfig) *WakeSessionHandler {
	return &WakeSessionHandler{
		kubeClient: kubeClient,
		config:     config,
	}
}

// Handle executes the wake_session command.
//
// Steps:
//  1. Parse session ID
//  2. Scale deployment to 1 replica
//  3. Wait for pod to be Running
//  4. Get new pod IP
//  5. Return result with updated metadata
func (h *WakeSessionHandler) Handle(cmd *CommandMessage) (*CommandResult, error) {
	log.Printf("[WakeSessionHandler] Waking session from command %s", cmd.CommandID)

	// Parse session ID
	sessionID, ok := cmd.Payload["sessionId"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("missing or invalid sessionId")
	}

	log.Printf("[WakeSessionHandler] Scaling deployment to 1 replica for session %s", sessionID)

	// Scale deployment to 1
	if err := scaleDeployment(h.kubeClient, h.config.Namespace, sessionID, 1); err != nil {
		return nil, fmt.Errorf("failed to scale deployment: %w", err)
	}

	// Wait for pod to be ready
	podIP, err := waitForPodReady(h.kubeClient, h.config.Namespace, sessionID, 120)
	if err != nil {
		return nil, fmt.Errorf("pod not ready after wake: %w", err)
	}

	log.Printf("[WakeSessionHandler] Session %s woke successfully (pod IP: %s)", sessionID, podIP)

	return &CommandResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId": sessionID,
			"podIP":     podIP,
			"vncPort":   3000,
			"state":     "running",
		},
	}, nil
}

// Helper functions

func getBoolOrDefault(payload map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := payload[key].(bool); ok {
		return val
	}
	return defaultValue
}

func getStringOrDefault(payload map[string]interface{}, key string, defaultValue string) string {
	if val, ok := payload[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}
