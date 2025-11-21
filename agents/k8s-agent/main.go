// Package main implements the Kubernetes Agent for StreamSpace v2.0.
//
// The K8s Agent is a standalone binary that runs inside a Kubernetes cluster
// and connects TO the Control Plane via WebSocket. It receives commands
// from the Control Plane and executes them on the local Kubernetes cluster.
//
// Architecture:
//   - Agent connects TO Control Plane (outbound connection)
//   - WebSocket for bidirectional communication
//   - Receives commands (start/stop/hibernate/wake session)
//   - Reports status back to Control Plane
//   - Manages Kubernetes resources (Deployments, Services, PVCs)
//
// Command-line flags:
//   --agent-id: Unique identifier for this agent (e.g., k8s-prod-us-east-1)
//   --control-plane-url: Control Plane WebSocket URL (e.g., wss://control.example.com)
//   --platform: Platform type (default: kubernetes)
//   --region: Deployment region (e.g., us-east-1)
//   --namespace: Kubernetes namespace for sessions (default: streamspace)
//
// Environment variables (alternative to flags):
//   AGENT_ID: Agent identifier
//   CONTROL_PLANE_URL: Control Plane URL
//   PLATFORM: Platform type
//   REGION: Deployment region
//   NAMESPACE: Session namespace
//
// Usage:
//   k8s-agent --agent-id=k8s-prod-us-east-1 --control-plane-url=wss://control.example.com
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sAgent represents a Kubernetes agent instance.
//
// The agent maintains a connection to the Control Plane and handles
// session lifecycle commands on the local Kubernetes cluster.
type K8sAgent struct {
	// config is the agent configuration
	config *AgentConfig

	// kubeClient is the Kubernetes API client
	kubeClient *kubernetes.Clientset

	// restConfig is the REST config for Kubernetes API (needed for port-forward)
	restConfig *rest.Config

	// vncManager manages VNC tunnels for sessions
	vncManager *VNCTunnelManager

	// wsConn is the WebSocket connection to Control Plane
	wsConn *websocket.Conn

	// connMutex protects wsConn access
	connMutex sync.RWMutex

	// stopChan signals the agent to stop
	stopChan chan struct{}

	// doneChan signals that the agent has stopped
	doneChan chan struct{}

	// commandHandlers maps command actions to handlers
	commandHandlers map[string]CommandHandler
}

// NewK8sAgent creates a new Kubernetes agent instance.
//
// It initializes the Kubernetes client and prepares command handlers.
func NewK8sAgent(config *AgentConfig) (*K8sAgent, error) {
	// Create Kubernetes client and REST config
	kubeClient, restConfig, err := createKubernetesClient(config.KubeConfig)
	if err != nil {
		return nil, err
	}

	agent := &K8sAgent{
		config:     config,
		kubeClient: kubeClient,
		restConfig: restConfig,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}

	// Initialize VNC tunnel manager
	agent.vncManager = NewVNCTunnelManager(kubeClient, restConfig, config.Namespace, agent)

	// Initialize command handlers
	agent.initCommandHandlers()

	return agent, nil
}

// createKubernetesClient creates a Kubernetes client from config.
//
// If kubeConfigPath is empty, it uses in-cluster config.
// Returns both the clientset and REST config (needed for port-forward).
func createKubernetesClient(kubeConfigPath string) (*kubernetes.Clientset, *rest.Config, error) {
	var config *rest.Config
	var err error

	if kubeConfigPath != "" {
		// Use kubeconfig file (for local development)
		log.Printf("Using kubeconfig from: %s", kubeConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	} else {
		// Use in-cluster config (for production)
		log.Println("Using in-cluster Kubernetes config")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}

// initCommandHandlers initializes the command handler registry.
func (a *K8sAgent) initCommandHandlers() {
	a.commandHandlers = map[string]CommandHandler{
		"start_session":     NewStartSessionHandler(a.kubeClient, a.config, a),
		"stop_session":      NewStopSessionHandler(a.kubeClient, a.config, a),
		"hibernate_session": NewHibernateSessionHandler(a.kubeClient, a.config),
		"wake_session":      NewWakeSessionHandler(a.kubeClient, a.config),
	}
}

// Run starts the agent and blocks until shutdown.
//
// This is the main event loop for the agent.
func (a *K8sAgent) Run() error {
	log.Printf("[K8sAgent] Starting agent: %s (platform: %s, region: %s)",
		a.config.AgentID, a.config.Platform, a.config.Region)

	// Connect to Control Plane
	if err := a.Connect(); err != nil {
		return err
	}

	// Start background goroutines
	go a.SendHeartbeats()
	go a.readPump()
	go a.writePump()

	// Wait for stop signal
	<-a.stopChan
	log.Println("[K8sAgent] Shutdown signal received, stopping...")

	// Graceful shutdown
	a.shutdown()

	// Wait for goroutines to finish
	close(a.doneChan)
	log.Println("[K8sAgent] Agent stopped")

	return nil
}

// WaitForShutdown waits for OS signals and initiates graceful shutdown.
func (a *K8sAgent) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("[K8sAgent] Received signal: %v", sig)
	close(a.stopChan)
}

// shutdown performs graceful shutdown of the agent.
func (a *K8sAgent) shutdown() {
	// Close all VNC tunnels
	if a.vncManager != nil {
		log.Println("[K8sAgent] Closing all VNC tunnels...")
		a.vncManager.CloseAll()
	}

	a.connMutex.Lock()
	defer a.connMutex.Unlock()

	if a.wsConn != nil {
		// Send close message
		a.wsConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Agent shutting down"))

		// Close connection
		a.wsConn.Close()
		a.wsConn = nil
	}

	log.Println("[K8sAgent] Graceful shutdown complete")
}

// main is the entry point for the K8s Agent.
func main() {
	// Command-line flags
	agentID := flag.String("agent-id", os.Getenv("AGENT_ID"), "Agent ID (e.g., k8s-prod-us-east-1)")
	controlPlaneURL := flag.String("control-plane-url", os.Getenv("CONTROL_PLANE_URL"), "Control Plane WebSocket URL")
	platform := flag.String("platform", getEnvOrDefault("PLATFORM", "kubernetes"), "Platform type")
	region := flag.String("region", os.Getenv("REGION"), "Deployment region")
	namespace := flag.String("namespace", getEnvOrDefault("NAMESPACE", "streamspace"), "Kubernetes namespace for sessions")
	kubeConfig := flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file (empty for in-cluster)")
	maxCPU := flag.Int("max-cpu", 100, "Maximum CPU cores available")
	maxMemory := flag.Int("max-memory", 128, "Maximum memory in GB")
	maxSessions := flag.Int("max-sessions", 100, "Maximum concurrent sessions")

	flag.Parse()

	// Validate required flags
	if *agentID == "" {
		log.Fatal("--agent-id is required")
	}
	if *controlPlaneURL == "" {
		log.Fatal("--control-plane-url is required")
	}

	// Create agent configuration
	config := &AgentConfig{
		AgentID:         *agentID,
		ControlPlaneURL: *controlPlaneURL,
		Platform:        *platform,
		Region:          *region,
		Namespace:       *namespace,
		KubeConfig:      *kubeConfig,
		Capacity: AgentCapacity{
			MaxCPU:      *maxCPU,
			MaxMemory:   *maxMemory,
			MaxSessions: *maxSessions,
		},
	}

	// Create agent
	agent, err := NewK8sAgent(config)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Run agent in background
	go func() {
		if err := agent.Run(); err != nil {
			log.Fatalf("Agent error: %v", err)
		}
	}()

	// Wait for shutdown signal
	agent.WaitForShutdown()
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
