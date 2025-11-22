// Package main implements the Docker Agent for StreamSpace v2.0.
//
// The Docker Agent is a standalone binary that runs on a Docker host
// and connects TO the Control Plane via WebSocket. It receives commands
// from the Control Plane and executes them on the local Docker daemon.
//
// Architecture:
//   - Agent connects TO Control Plane (outbound connection)
//   - WebSocket for bidirectional communication
//   - Receives commands (start/stop/hibernate/wake session)
//   - Reports status back to Control Plane
//   - Manages Docker resources (Containers, Networks, Volumes)
//
// Command-line flags:
//   --agent-id: Unique identifier for this agent (e.g., docker-prod-us-east-1)
//   --control-plane-url: Control Plane WebSocket URL (e.g., wss://control.example.com)
//   --platform: Platform type (default: docker)
//   --region: Deployment region (e.g., us-east-1)
//   --docker-host: Docker daemon socket (default: unix:///var/run/docker.sock)
//   --enable-ha: Enable HA mode with leader election (default: false)
//   --leader-election-backend: Backend for leader election (file, redis, swarm)
//   --lock-file-path: Lock file path for file backend (optional)
//   --redis-url: Redis URL for redis backend (e.g., redis://localhost:6379/0)
//
// Environment variables (alternative to flags):
//   AGENT_ID: Agent identifier
//   CONTROL_PLANE_URL: Control Plane URL
//   PLATFORM: Platform type
//   REGION: Deployment region
//   DOCKER_HOST: Docker daemon socket
//   ENABLE_HA: Enable HA mode (true/false)
//   LEADER_ELECTION_BACKEND: Leader election backend (file/redis/swarm)
//   LOCK_FILE_PATH: Lock file path for file backend
//   REDIS_URL: Redis URL for redis backend
//
// Usage:
//   # Standalone mode (single instance)
//   docker-agent --agent-id=docker-prod-us-east-1 --control-plane-url=wss://control.example.com
//
//   # HA mode with file backend (single host, multiple processes)
//   docker-agent --agent-id=docker-prod-us-east-1 --control-plane-url=wss://control.example.com \
//     --enable-ha --leader-election-backend=file
//
//   # HA mode with Redis backend (multi-host)
//   docker-agent --agent-id=docker-prod-us-east-1 --control-plane-url=wss://control.example.com \
//     --enable-ha --leader-election-backend=redis --redis-url=redis://localhost:6379/0
//
//   # HA mode with Swarm backend (Docker Swarm)
//   docker-agent --agent-id=docker-prod-us-east-1 --control-plane-url=wss://control.example.com \
//     --enable-ha --leader-election-backend=swarm
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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/config"
	"github.com/streamspace-dev/streamspace/agents/docker-agent/internal/leaderelection"
)

// DockerAgent represents a Docker agent instance.
//
// The agent maintains a connection to the Control Plane and handles
// session lifecycle commands on the local Docker daemon.
type DockerAgent struct {
	// config is the agent configuration
	config *config.AgentConfig

	// dockerClient is the Docker API client
	dockerClient *client.Client

	// vncManager manages VNC tunnels for sessions (TODO: implement)
	// vncManager *VNCTunnelManager

	// wsConn is the WebSocket connection to Control Plane
	wsConn *websocket.Conn

	// connMutex protects wsConn access
	connMutex sync.RWMutex

	// writeChan queues messages for WebSocket transmission
	// Single-writer pattern to prevent concurrent write panics
	writeChan chan []byte

	// stopChan signals the agent to stop
	stopChan chan struct{}

	// doneChan signals that the agent has stopped
	doneChan chan struct{}

	// commandHandlers maps command actions to handlers (TODO: implement handlers)
	commandHandlers map[string]CommandHandler
}

// CommandHandler is the interface for command handlers.
type CommandHandler interface {
	Handle(payload json.RawMessage) error
}

// NewDockerAgent creates a new Docker agent instance.
//
// It initializes the Docker client and prepares command handlers.
func NewDockerAgent(cfg *config.AgentConfig) (*DockerAgent, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(cfg.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	agent := &DockerAgent{
		config:       cfg,
		dockerClient: dockerClient,
		writeChan:    make(chan []byte, 256), // Buffered channel for WebSocket writes
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}

	// Initialize command handlers (TODO: implement)
	agent.initCommandHandlers()

	return agent, nil
}

// initCommandHandlers initializes the command handler registry.
func (a *DockerAgent) initCommandHandlers() {
	a.commandHandlers = map[string]CommandHandler{
		"start_session":     NewStartSessionHandler(a.dockerClient, a.config, a),
		"stop_session":      NewStopSessionHandler(a.dockerClient, a.config, a),
		"hibernate_session": NewHibernateSessionHandler(a.dockerClient, a.config),
		"wake_session":      NewWakeSessionHandler(a.dockerClient, a.config),
	}
}

// Run starts the agent and blocks until shutdown.
//
// This is the main event loop for the agent.
func (a *DockerAgent) Run() error {
	log.Printf("[DockerAgent] Starting agent: %s (platform: %s, region: %s)",
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
	log.Println("[DockerAgent] Shutdown signal received, stopping...")

	// Graceful shutdown
	a.shutdown()

	// Wait for goroutines to finish
	close(a.doneChan)
	log.Println("[DockerAgent] Agent stopped")

	return nil
}

// WaitForShutdown waits for OS signals and initiates graceful shutdown.
func (a *DockerAgent) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("[DockerAgent] Received signal: %v", sig)
	close(a.stopChan)
}

// shutdown performs graceful shutdown of agent resources.
func (a *DockerAgent) shutdown() {
	// TODO: Close all VNC tunnels
	// if a.vncManager != nil {
	// 	log.Println("[DockerAgent] Closing all VNC tunnels...")
	// 	a.vncManager.CloseAll()
	// }

	// Close write channel to signal writePump to drain and exit
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

	// Close Docker client
	if a.dockerClient != nil {
		a.dockerClient.Close()
	}

	log.Println("[DockerAgent] Graceful shutdown complete")
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
	AgentID  string                `json:"agentId"`
	Platform string                `json:"platform"`
	Region   string                `json:"region,omitempty"`
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
func (a *DockerAgent) Connect() error {
	log.Println("[DockerAgent] Connecting to Control Plane...")

	// Step 1: Register agent
	if err := a.registerAgent(); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Step 2: Connect WebSocket
	if err := a.connectWebSocket(); err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}

	log.Printf("[DockerAgent] Connected to Control Plane: %s", a.config.ControlPlaneURL)
	return nil
}

// registerAgent registers the agent with the Control Plane via HTTP API.
func (a *DockerAgent) registerAgent() error {
	// Prepare registration request
	reqBody := AgentRegistrationRequest{
		AgentID:  a.config.AgentID,
		Platform: a.config.Platform,
		Region:   a.config.Region,
		Capacity: &a.config.Capacity,
		Metadata: map[string]interface{}{
			"dockerHost":    a.config.DockerHost,
			"networkName":   a.config.NetworkName,
			"volumeDriver":  a.config.VolumeDriver,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal registration request: %w", err)
	}

	// Construct registration URL
	u, err := url.Parse(a.config.ControlPlaneURL)
	if err != nil {
		return fmt.Errorf("invalid control plane URL: %w", err)
	}

	// Convert WebSocket URL to HTTP URL
	if u.Scheme == "wss" {
		u.Scheme = "https"
	} else {
		u.Scheme = "http"
	}
	u.Path = "/api/v1/agents/register"

	// Send registration request
	log.Printf("[DockerAgent] Registering agent at: %s", u.String())
	resp, err := http.Post(u.String(), "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var regResp AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	log.Printf("[DockerAgent] Registered successfully (ID: %s, Status: %s)", regResp.ID, regResp.Status)
	return nil
}

// connectWebSocket establishes the WebSocket connection to Control Plane.
func (a *DockerAgent) connectWebSocket() error {
	// Construct WebSocket URL
	u, err := url.Parse(a.config.ControlPlaneURL)
	if err != nil {
		return fmt.Errorf("invalid control plane URL: %w", err)
	}

	// Add agent_id query parameter
	q := u.Query()
	q.Set("agent_id", a.config.AgentID)
	u.RawQuery = q.Encode()
	u.Path = "/api/v1/agents/connect"

	// Connect WebSocket
	log.Printf("[DockerAgent] Connecting WebSocket to: %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	a.connMutex.Lock()
	a.wsConn = conn
	a.connMutex.Unlock()

	log.Println("[DockerAgent] WebSocket connected")
	return nil
}

// sendMessage sends a message through the write channel (single-writer pattern).
func (a *DockerAgent) sendMessage(message interface{}) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	select {
	case a.writeChan <- jsonData:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending message")
	case <-a.stopChan:
		return fmt.Errorf("agent is shutting down")
	}
}

// writePump handles WebSocket writes (single goroutine, single writer).
func (a *DockerAgent) writePump() {
	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-a.writeChan:
			if !ok {
				// Write channel closed, drain and exit
				a.connMutex.RLock()
				if a.wsConn != nil {
					a.wsConn.WriteMessage(websocket.CloseMessage, []byte{})
				}
				a.connMutex.RUnlock()
				return
			}

			a.connMutex.RLock()
			if a.wsConn != nil {
				a.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
				err := a.wsConn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("[writePump] Write error: %v", err)
				}
			}
			a.connMutex.RUnlock()

		case <-ticker.C:
			// Send ping
			a.connMutex.RLock()
			if a.wsConn != nil {
				a.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := a.wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("[writePump] Ping error: %v", err)
				}
			}
			a.connMutex.RUnlock()

		case <-a.stopChan:
			return
		}
	}
}

// readPump handles WebSocket reads.
func (a *DockerAgent) readPump() {
	defer func() {
		close(a.stopChan)
	}()

	a.connMutex.RLock()
	conn := a.wsConn
	a.connMutex.RUnlock()

	if conn == nil {
		log.Println("[readPump] No connection available")
		return
	}

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-a.stopChan:
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[readPump] WebSocket error: %v", err)
				}
				return
			}

			// Handle message
			a.handleMessage(message)
		}
	}
}

// SendHeartbeats sends periodic heartbeat messages to Control Plane.
func (a *DockerAgent) SendHeartbeats() {
	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Unix(),
				"agentId":   a.config.AgentID,
				"status":    "online",
				// TODO: Add actual session count
				"activeSessions": 0,
			}

			if err := a.sendMessage(heartbeat); err != nil {
				log.Printf("[Heartbeat] Failed to send heartbeat: %v", err)
			} else {
				log.Printf("[Heartbeat] Sent heartbeat (activeSessions: 0)")
			}

		case <-a.stopChan:
			return
		}
	}
}

// runStandalone runs the agent without leader election (single instance mode).
func runStandalone(agent *DockerAgent) {
	log.Println("[DockerAgent] Running in standalone mode (no HA)")

	// Run agent in background
	go func() {
		if err := agent.Run(); err != nil {
			log.Printf("[DockerAgent] Agent error: %v", err)
		}
	}()

	// Wait for shutdown signal
	agent.WaitForShutdown()
}

// runWithLeaderElection runs the agent with leader election (HA mode).
//
// Only the leader replica will actively run the agent logic.
// Standby replicas wait for leadership and automatically take over on leader failure.
func runWithLeaderElection(agent *DockerAgent, cfg *config.AgentConfig, backend leaderelection.Backend, redisClient *redis.Client) {
	log.Printf("[DockerAgent] Running in HA mode (backend: %s)", backend)

	// Create leader election configuration
	leConfig := leaderelection.DefaultConfig(cfg.AgentID, backend)

	// Set backend-specific configuration
	if backend == leaderelection.BackendRedis {
		leConfig.RedisClient = redisClient
	} else if backend == leaderelection.BackendFile {
		// Override lock file path if specified
		if lockPath := os.Getenv("LOCK_FILE_PATH"); lockPath != "" {
			leConfig.LockFilePath = lockPath
		}
	}

	// Create leader elector
	elector, err := leaderelection.NewLeaderElector(leConfig)
	if err != nil {
		log.Fatalf("[DockerAgent] Failed to create leader elector: %v", err)
	}

	// Set up leader election callbacks
	onBecomeLeader := func() {
		log.Println("[DockerAgent] 🎖️  I am the LEADER - starting agent...")
		go func() {
			if err := agent.Run(); err != nil {
				log.Printf("[DockerAgent] Agent error: %v", err)
			}
		}()
	}

	onLoseLeadership := func() {
		log.Println("[DockerAgent] ⚠️  Lost leadership - stopping agent...")
		close(agent.stopChan)
	}

	// Run leader election in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := elector.Run(ctx, onBecomeLeader, onLoseLeadership); err != nil {
			log.Printf("[DockerAgent] Leader election error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("[DockerAgent] Received signal: %v", sig)

	// Cancel leader election context
	cancel()

	// Stop agent if running
	select {
	case agent.stopChan <- struct{}{}:
	default:
	}

	// Wait briefly for graceful shutdown
	time.Sleep(500 * time.Millisecond)

	log.Println("[DockerAgent] Shutdown complete")
}

// main is the entry point for the Docker Agent.
func main() {
	// Command-line flags
	agentID := flag.String("agent-id", os.Getenv("AGENT_ID"), "Agent ID (e.g., docker-prod-us-east-1)")
	controlPlaneURL := flag.String("control-plane-url", os.Getenv("CONTROL_PLANE_URL"), "Control Plane WebSocket URL")
	platform := flag.String("platform", getEnvOrDefault("PLATFORM", "docker"), "Platform type")
	region := flag.String("region", os.Getenv("REGION"), "Deployment region")
	dockerHost := flag.String("docker-host", getEnvOrDefault("DOCKER_HOST", "unix:///var/run/docker.sock"), "Docker daemon socket")
	networkName := flag.String("network", getEnvOrDefault("NETWORK_NAME", "streamspace"), "Docker network name")
	volumeDriver := flag.String("volume-driver", getEnvOrDefault("VOLUME_DRIVER", "local"), "Docker volume driver")
	maxCPU := flag.Int("max-cpu", 100, "Maximum CPU cores available")
	maxMemory := flag.Int("max-memory", 128, "Maximum memory in GB")
	maxSessions := flag.Int("max-sessions", 100, "Maximum concurrent sessions")
	heartbeatInterval := flag.Int("heartbeat-interval", getEnvIntOrDefault("HEALTH_CHECK_INTERVAL", 30), "Heartbeat interval in seconds")

	// High Availability flags
	enableHA := flag.Bool("enable-ha", getEnvOrDefault("ENABLE_HA", "false") == "true", "Enable HA mode with leader election")
	leaderBackend := flag.String("leader-election-backend", getEnvOrDefault("LEADER_ELECTION_BACKEND", "file"), "Leader election backend (file, redis, swarm)")
	lockFilePath := flag.String("lock-file-path", getEnvOrDefault("LOCK_FILE_PATH", ""), "Lock file path for file backend")
	redisURL := flag.String("redis-url", os.Getenv("REDIS_URL"), "Redis URL for redis backend (e.g., redis://localhost:6379/0)")

	flag.Parse()

	// Validate required flags
	if *agentID == "" {
		log.Fatal("--agent-id is required")
	}
	if *controlPlaneURL == "" {
		log.Fatal("--control-plane-url is required")
	}

	// Create agent configuration
	cfg := &config.AgentConfig{
		AgentID:           *agentID,
		ControlPlaneURL:   *controlPlaneURL,
		Platform:          *platform,
		Region:            *region,
		DockerHost:        *dockerHost,
		NetworkName:       *networkName,
		VolumeDriver:      *volumeDriver,
		HeartbeatInterval: *heartbeatInterval,
		Capacity: config.AgentCapacity{
			MaxCPU:      *maxCPU,
			MaxMemory:   *maxMemory,
			MaxSessions: *maxSessions,
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create agent
	agent, err := NewDockerAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Check if HA mode is enabled
	if *enableHA {
		// Validate backend
		backend := leaderelection.Backend(*leaderBackend)
		if backend != leaderelection.BackendFile && backend != leaderelection.BackendRedis && backend != leaderelection.BackendSwarm {
			log.Fatalf("Invalid leader election backend: %s (must be file, redis, or swarm)", *leaderBackend)
		}

		// Set up Redis client if needed
		var redisClient *redis.Client
		if backend == leaderelection.BackendRedis {
			if *redisURL == "" {
				log.Fatal("--redis-url is required for redis backend")
			}

			// Parse Redis URL
			opt, err := redis.ParseURL(*redisURL)
			if err != nil {
				log.Fatalf("Invalid Redis URL: %v", err)
			}

			redisClient = redis.NewClient(opt)

			// Test connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Fatalf("Failed to connect to Redis: %v", err)
			}
			log.Println("[DockerAgent] Connected to Redis for leader election")
		}

		// Override lock file path if specified
		if *lockFilePath != "" {
			// This will be used by DefaultConfig in runWithLeaderElection
			os.Setenv("LOCK_FILE_PATH", *lockFilePath)
		}

		// Run with leader election
		runWithLeaderElection(agent, cfg, backend, redisClient)
	} else {
		// Run in standalone mode
		runStandalone(agent)
	}
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
