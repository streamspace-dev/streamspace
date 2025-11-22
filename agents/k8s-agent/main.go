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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/streamspace-dev/streamspace/agents/k8s-agent/internal/config"
	"github.com/streamspace-dev/streamspace/agents/k8s-agent/internal/leaderelection"
)

// K8sAgent represents a Kubernetes agent instance.
//
// The agent maintains a connection to the Control Plane and handles
// session lifecycle commands on the local Kubernetes cluster.
type K8sAgent struct {
	// config is the agent configuration
	config *config.AgentConfig

	// kubeClient is the Kubernetes API client
	kubeClient *kubernetes.Clientset

	// dynamicClient is for accessing Custom Resources (Templates, Sessions)
	dynamicClient dynamic.Interface

	// restConfig is the REST config for Kubernetes API (needed for port-forward)
	restConfig *rest.Config

	// vncManager manages VNC tunnels for sessions
	vncManager *VNCTunnelManager

	// wsConn is the WebSocket connection to Control Plane
	wsConn *websocket.Conn

	// connMutex protects wsConn access
	connMutex sync.RWMutex

	// writeChan queues messages for WebSocket transmission
	// FIX P0-AGENT-001: Single-writer pattern to prevent concurrent write panics
	writeChan chan []byte

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
func NewK8sAgent(config *config.AgentConfig) (*K8sAgent, error) {
	// Create Kubernetes client and REST config
	kubeClient, restConfig, err := createKubernetesClient(config.KubeConfig)
	if err != nil {
		return nil, err
	}

	// Create dynamic client for CRD access
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	agent := &K8sAgent{
		config:        config,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		restConfig:    restConfig,
		writeChan:     make(chan []byte, 256), // FIX P0-AGENT-001: Buffered channel for WebSocket writes
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
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
		"start_session":     NewStartSessionHandler(a.kubeClient, a.dynamicClient, a.config, a),
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
// shutdown performs graceful shutdown of agent resources.
//
// FIX P0-AGENT-001: Properly closes write channel to prevent goroutine leaks.
func (a *K8sAgent) shutdown() {
	// Close all VNC tunnels
	if a.vncManager != nil {
		log.Println("[K8sAgent] Closing all VNC tunnels...")
		a.vncManager.CloseAll()
	}

	// Close write channel to signal writePump to drain and exit
	// Note: stopChan was already closed by caller, so writePump will exit
	close(a.writeChan)

	// Wait briefly for writePump to finish draining write channel
	time.Sleep(100 * time.Millisecond)

	a.connMutex.Lock()
	defer a.connMutex.Unlock()

	if a.wsConn != nil {
		// Close connection (writePump already stopped, safe to close directly)
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
	heartbeatInterval := flag.Int("heartbeat-interval", getEnvIntOrDefault("HEALTH_CHECK_INTERVAL", 30), "Heartbeat interval in seconds")
	enableHA := flag.Bool("enable-ha", getEnvOrDefault("ENABLE_HA", "false") == "true", "Enable high availability mode with leader election")

	flag.Parse()

	// Validate required flags
	if *agentID == "" {
		log.Fatal("--agent-id is required")
	}
	if *controlPlaneURL == "" {
		log.Fatal("--control-plane-url is required")
	}

	// Create agent configuration
	config := &config.AgentConfig{
		AgentID:           *agentID,
		ControlPlaneURL:   *controlPlaneURL,
		Platform:          *platform,
		Region:            *region,
		Namespace:         *namespace,
		KubeConfig:        *kubeConfig,
		HeartbeatInterval: *heartbeatInterval,
		Capacity: config.AgentCapacity{
			MaxCPU:      *maxCPU,
			MaxMemory:   *maxMemory,
			MaxSessions: *maxSessions,
		},
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create agent
	agent, err := NewK8sAgent(config)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Check if HA mode is enabled
	if *enableHA {
		log.Println("[K8sAgent] High Availability mode ENABLED - using leader election")
		runWithLeaderElection(agent, config)
	} else {
		log.Println("[K8sAgent] High Availability mode DISABLED - running as single instance")
		runStandalone(agent)
	}
}

// runStandalone runs the agent in standalone mode (no leader election).
func runStandalone(agent *K8sAgent) {
	// Run agent in background
	go func() {
		if err := agent.Run(); err != nil {
			log.Fatalf("Agent error: %v", err)
		}
	}()

	// Wait for shutdown signal
	agent.WaitForShutdown()
}

// runWithLeaderElection runs the agent with leader election enabled.
//
// Only the leader replica will be active. Standby replicas will wait
// and automatically take over if the leader fails.
func runWithLeaderElection(agent *K8sAgent, config *config.AgentConfig) {
	// Create Kubernetes client for leader election
	kubeClient, _, err := createKubernetesClient(config.KubeConfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client for leader election: %v", err)
	}

	// Create leader election configuration
	leConfig := leaderelection.DefaultConfig(config.AgentID, config.Namespace)
	elector := leaderelection.NewLeaderElector(kubeClient, leConfig)

	// Context for leader election
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track if agent is running
	agentRunning := false
	var agentStopFunc func()

	// Define callbacks for leader election
	onBecomeLeader := func() {
		log.Println("[K8sAgent] 🎖️  I am the LEADER - starting agent...")

		// Start agent
		agentStopFunc = func() {
			log.Println("[K8sAgent] Stopping agent due to leadership loss...")
			close(agent.stopChan)
		}

		go func() {
			if err := agent.Run(); err != nil {
				log.Printf("[K8sAgent] Agent error: %v", err)
			}
		}()

		agentRunning = true
		log.Println("[K8sAgent] Agent is now ACTIVE")
	}

	onLoseLeadership := func() {
		log.Println("[K8sAgent] ⚠️  Lost leadership - stopping agent...")

		if agentRunning && agentStopFunc != nil {
			agentStopFunc()
			agentRunning = false
		}

		log.Println("[K8sAgent] Agent is now STANDBY")
	}

	// Run leader election in background
	go func() {
		if err := elector.Run(ctx, onBecomeLeader, onLoseLeadership); err != nil {
			log.Printf("[K8sAgent] Leader election error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("[K8sAgent] Received signal: %v", sig)

	// Cancel leader election context
	cancel()

	// Stop agent if running
	if agentRunning {
		log.Println("[K8sAgent] Stopping agent...")
		close(agent.stopChan)
	}

	// Wait for graceful shutdown
	time.Sleep(2 * time.Second)
	log.Println("[K8sAgent] Shutdown complete")
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns environment variable value as int or default.
// Supports both duration strings (e.g., "30s", "1m") and integer strings.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Try parsing as duration string (e.g., "30s", "1m")
		if duration, err := time.ParseDuration(value); err == nil {
			return int(duration.Seconds())
		}
		// Try parsing as integer
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512 KB
)

// AgentRegistrationRequest is the request payload for agent registration.
type AgentRegistrationRequest struct {
	AgentID  string         `json:"agentId"`
	Platform string         `json:"platform"`
	Region   string         `json:"region,omitempty"`
	Capacity *config.AgentCapacity `json:"capacity,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AgentRegistrationResponse is the response from agent registration.
type AgentRegistrationResponse struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	Platform  string    `json:"platform"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Connect establishes connection to the Control Plane.
//
// Steps:
//  1. Register agent with Control Plane (POST /api/v1/agents/register)
//  2. Connect to WebSocket (/api/v1/agents/connect?agent_id=xxx)
//  3. Start read/write pumps
func (a *K8sAgent) Connect() error {
	log.Println("[K8sAgent] Connecting to Control Plane...")

	// Step 1: Register agent
	if err := a.registerAgent(); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Step 2: Connect WebSocket
	if err := a.connectWebSocket(); err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}

	log.Printf("[K8sAgent] Connected to Control Plane: %s", a.config.ControlPlaneURL)
	return nil
}

// registerAgent registers the agent with the Control Plane via HTTP API.
func (a *K8sAgent) registerAgent() error {
	// Prepare registration request
	reqBody := AgentRegistrationRequest{
		AgentID:  a.config.AgentID,
		Platform: a.config.Platform,
		Region:   a.config.Region,
		Capacity: &a.config.Capacity,
		Metadata: map[string]interface{}{
			"namespace":  a.config.Namespace,
			"kubernetes": true,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Convert WebSocket URL to HTTP URL
	httpURL := convertToHTTPURL(a.config.ControlPlaneURL)
	registerURL := fmt.Sprintf("%s/api/v1/agents/register", httpURL)

	// Send registration request
	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var regResp AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[K8sAgent] Registered successfully: %s (status: %s)", regResp.AgentID, regResp.Status)
	return nil
}

// connectWebSocket establishes the WebSocket connection to Control Plane.
func (a *K8sAgent) connectWebSocket() error {
	// Build WebSocket URL with agent_id query parameter
	wsURL := fmt.Sprintf("%s/api/v1/agents/connect?agent_id=%s",
		a.config.ControlPlaneURL,
		url.QueryEscape(a.config.AgentID))

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Set connection parameters
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	a.connMutex.Lock()
	a.wsConn = conn
	a.connMutex.Unlock()

	log.Println("[K8sAgent] WebSocket connected")
	return nil
}

// Reconnect attempts to reconnect with exponential backoff.
func (a *K8sAgent) Reconnect() error {
	log.Println("[K8sAgent] Connection lost, attempting to reconnect...")

	for attempt, backoff := range a.config.ReconnectBackoff {
		log.Printf("[K8sAgent] Reconnect attempt %d/%d (waiting %ds)",
			attempt+1, len(a.config.ReconnectBackoff), backoff)

		time.Sleep(time.Duration(backoff) * time.Second)

		if err := a.Connect(); err != nil {
			log.Printf("[K8sAgent] Reconnect attempt %d failed: %v", attempt+1, err)
			continue
		}

		log.Println("[K8sAgent] Reconnected successfully")
		return nil
	}

	return fmt.Errorf("reconnection failed after %d attempts", len(a.config.ReconnectBackoff))
}

// SendHeartbeats sends periodic heartbeats to the Control Plane.
//
// Heartbeats include:
//   - Agent status (online/draining)
//   - Active session count
//   - Current capacity usage
func (a *K8sAgent) SendHeartbeats() {
	interval := time.Duration(a.config.HeartbeatInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[K8sAgent] Starting heartbeat sender (interval: %s)", interval)

	for {
		select {
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("[K8sAgent] Failed to send heartbeat: %v", err)
			}

		case <-a.stopChan:
			log.Println("[K8sAgent] Heartbeat sender stopped")
			return
		}
	}
}

// sendHeartbeat sends a single heartbeat message.
func (a *K8sAgent) sendHeartbeat() error {
	// TODO: Calculate active sessions and capacity usage
	activeSessions := 0 // Placeholder

	heartbeat := map[string]interface{}{
		"type":      "heartbeat",
		"timestamp": time.Now(),
		"payload": map[string]interface{}{
			"status":         "online",
			"activeSessions": activeSessions,
			"capacity": map[string]interface{}{
				"maxCpu":      a.config.Capacity.MaxCPU,
				"maxMemory":   a.config.Capacity.MaxMemory,
				"maxSessions": a.config.Capacity.MaxSessions,
			},
		},
	}

	return a.sendMessage(heartbeat)
}

// sendMessage sends a JSON message over the WebSocket connection.
//
// FIX P0-AGENT-001: Uses write channel to prevent concurrent write panics.
// All WebSocket writes MUST go through writePump goroutine.
func (a *K8sAgent) sendMessage(message interface{}) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send via write channel with timeout to prevent blocking
	select {
	case a.writeChan <- jsonData:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("write channel send timeout (channel may be full or blocked)")
	case <-a.stopChan:
		return fmt.Errorf("agent is shutting down")
	}
}

// readPump reads messages from the WebSocket connection.
//
// This runs in a dedicated goroutine and continuously reads messages
// from the Control Plane, routing them to appropriate handlers.
func (a *K8sAgent) readPump() {
	defer func() {
		log.Println("[K8sAgent] Read pump stopped")
	}()

	for {
		select {
		case <-a.stopChan:
			return
		default:
			a.connMutex.RLock()
			conn := a.wsConn
			a.connMutex.RUnlock()

			if conn == nil {
				log.Println("[K8sAgent] Connection lost in read pump")
				a.Reconnect()
				continue
			}

			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[K8sAgent] Unexpected close: %v", err)
				}
				log.Println("[K8sAgent] Read error, attempting reconnect...")
				a.Reconnect()
				continue
			}

			// Parse and handle message
			if err := a.handleMessage(messageBytes); err != nil {
				log.Printf("[K8sAgent] Failed to handle message: %v", err)
			}
		}
	}
}

// writePump handles periodic ping messages to keep the connection alive.
//
// This runs in a dedicated goroutine.
// writePump handles all WebSocket writes from write channel.
//
// FIX P0-AGENT-001: Single-writer pattern to prevent concurrent write panics.
// This is the ONLY goroutine allowed to write to the WebSocket connection.
// All other code must send messages via the writeChan channel.
//
// Responsibilities:
//   - Read messages from writeChan and write to WebSocket
//   - Send periodic ping messages to keep connection alive
//   - Handle write errors and shutdown gracefully
func (a *K8sAgent) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		log.Println("[K8sAgent] Write pump stopped")
	}()

	for {
		select {
		case message := <-a.writeChan:
			// Write message from channel to WebSocket
			a.connMutex.RLock()
			conn := a.wsConn
			a.connMutex.RUnlock()

			if conn == nil {
				log.Println("[K8sAgent] Warning: Dropped message (connection is nil)")
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[K8sAgent] Write error: %v", err)
				return
			}

		case <-ticker.C:
			// Send periodic ping to keep connection alive
			a.connMutex.RLock()
			conn := a.wsConn
			a.connMutex.RUnlock()

			if conn == nil {
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[K8sAgent] Ping error: %v", err)
				return
			}

		case <-a.stopChan:
			return
		}
	}
}

// convertToHTTPURL converts a WebSocket URL to HTTP URL.
//
// Examples:
//   wss://control.example.com -> https://control.example.com
//   ws://localhost:8000 -> http://localhost:8000
func convertToHTTPURL(wsURL string) string {
	if len(wsURL) > 3 && wsURL[:3] == "wss" {
		return "https" + wsURL[3:]
	}
	if len(wsURL) > 2 && wsURL[:2] == "ws" {
		return "http" + wsURL[2:]
	}
	return wsURL
}
