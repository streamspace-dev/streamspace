# StreamSpace v2.0 Architecture

**Version**: 2.0.0-beta
**Date**: 2025-11-21
**Status**: Production Ready

---

## Executive Summary

StreamSpace v2.0 introduces a revolutionary multi-platform architecture based on a **Control Plane + Agent** model. This architectural shift enables StreamSpace to support multiple computing platforms (Kubernetes, Docker, VMs, Cloud) while maintaining centralized management and providing firewall-friendly deployments.

**Key Architecture Changes:**
- **Centralized Control Plane**: Manages all agents, sessions, and user interactions
- **Platform-Specific Agents**: Execute platform-specific operations (K8s, Docker, etc.)
- **Outbound Agent Connections**: Agents connect TO Control Plane (NAT/firewall friendly)
- **VNC Proxy/Tunneling**: VNC traffic routed through Control Plane (cross-network support)
- **Multi-Platform Abstraction**: Generic "Session" concept independent of platform

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Components](#core-components)
3. [Communication Protocols](#communication-protocols)
4. [Data Flow](#data-flow)
5. [VNC Architecture](#vnc-architecture)
6. [Security Architecture](#security-architecture)
7. [Scalability & High Availability](#scalability--high-availability)
8. [Platform Support](#platform-support)

---

## Architecture Overview

### High-Level Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                         Control Plane                                 │
│                      (Centralized Management)                         │
│                                                                       │
│  ┌────────────┐   ┌──────────────┐   ┌──────────────┐              │
│  │  Web UI    │   │  REST API    │   │  Admin UI    │              │
│  └──────┬─────┘   └──────┬───────┘   └──────┬───────┘              │
│         │                  │                   │                      │
│         └──────────────────┼───────────────────┘                     │
│                            │                                          │
│         ┌──────────────────┴──────────────────────┐                  │
│         │      Control Plane Core Services         │                  │
│         │                                           │                  │
│         │  ┌─────────────────┐  ┌───────────────┐ │                  │
│         │  │  Agent Hub      │  │  Command      │ │                  │
│         │  │  (WebSocket)    │  │  Dispatcher   │ │                  │
│         │  └────────┬────────┘  └───────┬───────┘ │                  │
│         │           │                     │         │                  │
│         │  ┌────────┴─────────┐  ┌───────┴───────┐ │                  │
│         │  │  VNC Proxy       │  │  Session      │ │                  │
│         │  │  /vnc/{id}       │  │  Manager      │ │                  │
│         │  └──────────────────┘  └───────────────┘ │                  │
│         └──────────────────┬──────────────────────┘                  │
│                            │                                          │
│  ┌────────────────────────┴────────────────────────┐                │
│  │           PostgreSQL Database                    │                │
│  │  - Sessions  - Agents  - Commands  - Users      │                │
│  └──────────────────────────────────────────────────┘                │
└───────────────────────────┬───────────────────────────────────────────┘
                            │
                            │ WebSocket (Outbound from Agents)
                            │
        ┌───────────────────┼────────────────────┐
        │                   │                    │
        ▼                   ▼                    ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ K8s Agent     │   │ Docker Agent  │   │ Future Agent  │
│ (Cluster A)   │   │ (Host B)      │   │ (Cloud C)     │
│               │   │               │   │               │
│ • Registration│   │ • Registration│   │ • Registration│
│ • Heartbeat   │   │ • Heartbeat   │   │ • Heartbeat   │
│ • Commands    │   │ • Commands    │   │ • Commands    │
│ • VNC Tunnel  │   │ • VNC Tunnel  │   │ • VNC Tunnel  │
└───────┬───────┘   └───────┬───────┘   └───────┬───────┘
        │                   │                    │
        ▼                   ▼                    ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ Session Pods  │   │ Session Ctnrs │   │ Session VMs   │
│ (Kubernetes)  │   │ (Docker)      │   │ (Cloud)       │
└───────────────┘   └───────────────┘   └───────────────┘
```

### Architecture Principles

**1. Separation of Concerns**
- **Control Plane**: User management, session orchestration, policy enforcement
- **Agents**: Platform-specific execution, resource management
- **Sessions**: User workloads (containers, VMs, etc.)

**2. Platform Abstraction**
- Generic "Session" concept across all platforms
- Agents translate Control Plane commands to platform-specific operations
- UI/API agnostic to underlying platform

**3. Firewall-Friendly Design**
- Agents initiate outbound connections only
- No inbound ports required on agent side
- NAT traversal built-in

**4. Fault Tolerance**
- Automatic agent reconnection
- Session state persisted in database
- Graceful degradation on agent failure

**5. Scalability**
- Horizontal scaling of Control Plane (multiple replicas)
- Multiple agents per platform (load distribution)
- Multi-region support (agents anywhere)

---

## Core Components

### 1. Control Plane

**Responsibilities:**
- User authentication and authorization
- Agent lifecycle management
- Session orchestration and state management
- VNC traffic proxying
- License enforcement and audit logging

**Sub-Components:**

#### 1.1 Agent Hub
**Location**: `api/internal/websocket/agent_hub.go` (506 lines)

**Purpose**: Central registry and communication hub for all connected agents.

**Features:**
- Thread-safe agent connection management
- Heartbeat monitoring (30-second timeout)
- Automatic stale connection cleanup
- Message broadcasting to all or specific agents
- Connection state tracking (online/offline)

**Data Structures:**
```go
type AgentHub struct {
    connections  map[string]*websocket.Conn  // agent_id -> connection
    register     chan *AgentConnection       // Register new agent
    unregister   chan *AgentConnection       // Unregister agent
    broadcast    chan []byte                 // Broadcast to all
    sendToAgent  chan *AgentMessage          // Send to specific agent
}

type AgentConnection struct {
    AgentID    string
    Connection *websocket.Conn
    LastHB     time.Time
    Capacity   AgentCapacity
}
```

**Operations:**
- `RegisterAgent(agentID string, conn *websocket.Conn)`: Register new connection
- `UnregisterAgent(agentID string)`: Remove connection
- `SendToAgent(agentID string, message []byte)`: Send message to specific agent
- `BroadcastMessage(message []byte)`: Send to all connected agents
- `GetConnectedAgents() []string`: List online agents

#### 1.2 Command Dispatcher
**Location**: `api/internal/services/command_dispatcher.go` (356 lines)

**Purpose**: Queue and dispatch commands to agents with retry logic.

**Features:**
- Command queue management (pending, sent, ack, completed, failed)
- Worker pool for concurrent dispatch (default: 10 workers)
- Agent availability checking before dispatch
- Automatic retry for failed commands
- Command lifecycle persistence in database

**Command Flow:**
```
1. Command Created (status: pending)
   ↓
2. Dispatcher picks up command
   ↓
3. Check agent connectivity
   ↓
4. Send command via WebSocket (status: sent)
   ↓
5. Agent acknowledges (status: ack)
   ↓
6. Agent executes and completes (status: completed/failed)
```

**Data Structures:**
```go
type CommandDispatcher struct {
    hub         *AgentHub
    db          *sql.DB
    workers     int           // Number of concurrent workers
    commandCh   chan *Command // Command queue
    stopCh      chan bool     // Graceful shutdown
}

type Command struct {
    ID          string
    AgentID     string
    SessionID   string
    Type        string  // start_session, stop_session, hibernate, wake
    Data        json.RawMessage
    Status      string  // pending, sent, ack, completed, failed
    Result      json.RawMessage
    CreatedAt   time.Time
    SentAt      *time.Time
    CompletedAt *time.Time
}
```

#### 1.3 VNC Proxy
**Location**: `api/internal/handlers/vnc_proxy.go` (430 lines)

**Purpose**: Tunnel VNC traffic between UI clients and session pods via agents.

**Features:**
- WebSocket endpoint: `GET /api/v1/vnc/:sessionId`
- JWT authentication and session ownership verification
- Agent routing based on session's agent_id
- Bidirectional binary data forwarding (Base64-encoded over JSON WebSocket)
- Single connection per session enforcement
- Connection lifecycle management

**VNC Proxy Flow:**
```
UI Client (noVNC)
    ↓ WebSocket Connect
    GET /api/v1/vnc/{session_id}?token=JWT
    ↓
VNC Proxy Handler
    ↓ Verify JWT
    ↓ Lookup session → agent_id
    ↓ Check session state (must be running)
    ↓ Verify agent online
    ↓ Establish VNC tunnel
    ↓
Send vnc_connect command to Agent
    ↓
Agent creates port-forward to pod:5900
    ↓
Agent sends vnc_ready message
    ↓
VNC Proxy starts bidirectional relay
    ↓
UI ←→ Control Plane ←→ Agent ←→ Pod
```

**Data Structures:**
```go
type VNCProxyHandler struct {
    hub    *AgentHub
    db     *sql.DB
    active map[string]*VNCConnection  // session_id -> connection
}

type VNCConnection struct {
    SessionID   string
    AgentID     string
    UIConn      *websocket.Conn
    AgentConn   *websocket.Conn
    CreatedAt   time.Time
}
```

#### 1.4 Session Manager
**Location**: `api/internal/handlers/sessions.go`

**Purpose**: Manage session lifecycle via Control Plane.

**Operations:**
- Create session: Select agent → dispatch start_session command
- Stop session: Dispatch stop_session command
- Hibernate session: Dispatch hibernate_session command
- Wake session: Dispatch wake_session command
- Get session status: Query database + agent real-time status
- List sessions: Filter by user, agent, platform, state

**Agent Selection Logic:**
```go
func SelectAgent(platform string, region string) (*Agent, error) {
    // 1. Filter agents by platform and region
    agents := GetAgentsByPlatform(platform, region)

    // 2. Filter online agents only
    onlineAgents := FilterOnlineAgents(agents)

    // 3. Check capacity (CPU, memory, session count)
    availableAgents := FilterByCapacity(onlineAgents)

    // 4. Select agent with least load
    return SelectLeastLoaded(availableAgents)
}
```

### 2. Kubernetes Agent

**Location**: `agents/k8s-agent/`
**Lines of Code**: 2,450+ lines across 11 files

**Responsibilities:**
- Connect to Control Plane via outbound WebSocket
- Receive and execute commands (start, stop, hibernate, wake)
- Create and manage Kubernetes resources (Deployments, Services, PVCs)
- Port-forward VNC traffic from pods to Control Plane
- Report session status and agent capacity
- Automatic reconnection on network failures

**Sub-Components:**

#### 2.1 Connection Manager
**File**: `agents/k8s-agent/connection.go` (339 lines)

**Features:**
- HTTP registration with Control Plane
- WebSocket connection establishment
- Automatic reconnection with exponential backoff (2s → 32s max)
- Heartbeat sender (every 10 seconds)
- Read/write pumps for concurrent message handling

**Reconnection Logic:**
```go
func (a *Agent) reconnectLoop() {
    backoff := 2 * time.Second
    maxBackoff := 32 * time.Second

    for {
        if err := a.connect(); err == nil {
            backoff = 2 * time.Second  // Reset on success
            return
        }

        log.Errorf("Connection failed, retrying in %v", backoff)
        time.Sleep(backoff)

        // Exponential backoff
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }
}
```

#### 2.2 Command Handlers
**File**: `agents/k8s-agent/handlers.go` (311 lines)

**Handlers:**

1. **StartSessionHandler**: Creates session resources
   - Parse session spec from command data
   - Create Deployment (with LinuxServer.io image)
   - Create Service (ClusterIP for VNC port 3000)
   - Create PVC (if persistent storage requested)
   - Wait for pod to be ready
   - Initialize VNC tunnel
   - Return session details (pod IP, VNC port)

2. **StopSessionHandler**: Cleans up session resources
   - Delete Deployment
   - Delete Service
   - Optionally delete PVC
   - Close VNC tunnel

3. **HibernateSessionHandler**: Pauses session
   - Scale Deployment to 0 replicas
   - Preserve PVC for state
   - Close VNC tunnel

4. **WakeSessionHandler**: Resumes session
   - Scale Deployment to 1 replica
   - Wait for pod ready
   - Reinitialize VNC tunnel
   - Return session details

#### 2.3 Kubernetes Operations
**File**: `agents/k8s-agent/k8s_operations.go` (360 lines)

**Operations:**
- `createSessionDeployment()`: Build and create Deployment manifest
- `createSessionService()`: Build and create Service manifest
- `createSessionPVC()`: Build and create PVC manifest
- `waitForPodReady()`: Poll pod status until Running + Ready
- `scaleDeployment()`: Update replica count (for hibernate/wake)
- `getSessionPodIP()`: Retrieve pod IP for VNC connection
- `deleteDeployment()`, `deleteService()`, `deletePVC()`: Cleanup operations

#### 2.4 VNC Tunnel Manager
**File**: `agents/k8s-agent/vnc_tunnel.go` (400+ lines)

**Purpose**: Port-forward VNC traffic from session pods to Control Plane.

**Features:**
- Kubernetes port-forward using SPDY protocol
- Bidirectional VNC data relay (pod:5900 ←→ Control Plane)
- Base64 encoding for binary data over JSON WebSocket
- Multi-session concurrent tunnel management
- Automatic cleanup on session stop

**Port-Forward Architecture:**
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ K8s Agent   │────▶│ Kubernetes   │────▶│ Session Pod │
│             │     │ API Server   │     │             │
│ VNC Tunnel  │     │              │     │ VNC Server  │
│ Manager     │     │ Port-Forward │     │ :5900       │
│             │◀────│  (SPDY)      │◀────│             │
└─────────────┘     └──────────────┘     └─────────────┘
      ▲                                          ▲
      │                                          │
      │ Base64-encoded VNC data                  │ Binary VNC
      │ over WebSocket JSON                      │ (RFB protocol)
      ▼                                          ▼
┌─────────────┐
│ Control     │
│ Plane       │
│ VNC Proxy   │
└─────────────┘
```

**Data Structures:**
```go
type VNCTunnelManager struct {
    tunnels    map[string]*VNCTunnel  // session_id -> tunnel
    k8sClient  *kubernetes.Clientset
    namespace  string
    mu         sync.RWMutex
}

type VNCTunnel struct {
    SessionID     string
    PodName       string
    LocalPort     int               // Random local port
    StopChan      chan struct{}
    PortForwarder *portforward.PortForwarder
}
```

**Operations:**
- `CreateTunnel(sessionID, podName string)`: Establish port-forward
- `CloseTunnel(sessionID string)`: Stop port-forward and cleanup
- `GetTunnel(sessionID string)`: Retrieve active tunnel
- `RelayVNCData(sessionID string, data []byte)`: Forward VNC frame to pod

---

## Communication Protocols

### 1. Agent ↔ Control Plane Protocol

**Transport**: WebSocket over HTTPS (WSS)
**Encoding**: JSON messages
**Heartbeat**: Every 10 seconds from agent

#### Message Types (Agent → Control Plane)

**1. Heartbeat**
```json
{
  "type": "heartbeat",
  "timestamp": "2025-11-21T12:34:56Z",
  "capacity": {
    "max_cpu": 100,
    "max_memory": 256,
    "max_sessions": 100,
    "current_sessions": 5
  }
}
```

**2. Acknowledgment**
```json
{
  "type": "ack",
  "command_id": "cmd-123",
  "timestamp": "2025-11-21T12:34:56Z"
}
```

**3. Command Complete**
```json
{
  "type": "complete",
  "command_id": "cmd-123",
  "result": {
    "session_id": "sess-456",
    "pod_ip": "10.42.1.5",
    "vnc_port": 5900,
    "status": "running"
  },
  "timestamp": "2025-11-21T12:34:57Z"
}
```

**4. Command Failed**
```json
{
  "type": "failed",
  "command_id": "cmd-123",
  "error": "Failed to create deployment: insufficient resources",
  "timestamp": "2025-11-21T12:34:57Z"
}
```

**5. Status Update**
```json
{
  "type": "status",
  "session_id": "sess-456",
  "state": "hibernated",
  "timestamp": "2025-11-21T12:35:00Z"
}
```

**6. VNC Ready** (Phase 6)
```json
{
  "type": "vnc_ready",
  "session_id": "sess-456",
  "local_port": 35672,
  "timestamp": "2025-11-21T12:35:01Z"
}
```

**7. VNC Data** (Phase 6)
```json
{
  "type": "vnc_data",
  "session_id": "sess-456",
  "data": "<base64-encoded-vnc-frame>",
  "timestamp": "2025-11-21T12:35:01Z"
}
```

**8. VNC Error** (Phase 6)
```json
{
  "type": "vnc_error",
  "session_id": "sess-456",
  "error": "Port-forward connection lost",
  "timestamp": "2025-11-21T12:35:02Z"
}
```

#### Message Types (Control Plane → Agent)

**1. Command**
```json
{
  "type": "command",
  "command_id": "cmd-123",
  "command_type": "start_session",
  "data": {
    "session_id": "sess-456",
    "user": "testuser",
    "template": "firefox-browser",
    "resources": {
      "cpu": "1000m",
      "memory": "2Gi"
    },
    "persistent_home": true
  },
  "timestamp": "2025-11-21T12:34:56Z"
}
```

**2. Ping**
```json
{
  "type": "ping",
  "timestamp": "2025-11-21T12:34:56Z"
}
```

**3. Shutdown**
```json
{
  "type": "shutdown",
  "reason": "Maintenance window",
  "graceful_seconds": 300,
  "timestamp": "2025-11-21T12:34:56Z"
}
```

**4. VNC Connect** (Phase 6)
```json
{
  "type": "vnc_connect",
  "session_id": "sess-456",
  "client_id": "ui-client-789",
  "timestamp": "2025-11-21T12:35:00Z"
}
```

**5. VNC Data** (Phase 6)
```json
{
  "type": "vnc_data",
  "session_id": "sess-456",
  "data": "<base64-encoded-user-input>",
  "timestamp": "2025-11-21T12:35:01Z"
}
```

**6. VNC Disconnect** (Phase 6)
```json
{
  "type": "vnc_disconnect",
  "session_id": "sess-456",
  "timestamp": "2025-11-21T12:35:05Z"
}
```

### 2. UI ↔ Control Plane VNC Protocol

**Transport**: WebSocket (binary frames)
**Encoding**: RFB (Remote Framebuffer Protocol) via noVNC
**Endpoint**: `GET /api/v1/vnc/:sessionId?token=JWT`

**noVNC Client:**
- Static HTML page served by Control Plane: `/vnc-viewer/:sessionId`
- Loads noVNC library from CDN (v1.4.0)
- Connects to VNC proxy WebSocket with JWT authentication
- Handles RFB protocol events (connect, disconnect, clipboard, etc.)

**Binary Data Flow:**
```
1. UI sends RFB protocol data (keyboard/mouse input)
   ↓
2. Control Plane receives binary WebSocket frame
   ↓
3. Base64-encode data → JSON message
   ↓
4. Forward to agent via agent WebSocket
   ↓
5. Agent receives, Base64-decodes
   ↓
6. Forward to pod via port-forward (binary)
   ↓
7. Pod processes input and generates screen updates
   ↓
8. Screen data flows back (reverse path)
```

---

## Data Flow

### Session Creation Flow

```
┌─────┐
│ UI  │
└──┬──┘
   │ 1. POST /api/v1/sessions
   │    {user, template, state: "running"}
   ▼
┌──────────────────┐
│ Control Plane    │
│ Session Handler  │
└────────┬─────────┘
         │ 2. Select agent based on:
         │    - Platform (kubernetes)
         │    - Region (if specified)
         │    - Capacity (least loaded)
         │
         │ 3. Create session record in DB
         │    (status: pending, agent_id: k8s-prod-us-east-1)
         │
         │ 4. Create agent_command record
         │    (type: start_session, status: pending)
         ▼
┌──────────────────┐
│ Command          │
│ Dispatcher       │
└────────┬─────────┘
         │ 5. Pick up command from queue
         │
         │ 6. Check agent online
         │
         │ 7. Send command via WebSocket
         │    (status: sent)
         ▼
┌──────────────────┐
│ K8s Agent        │
│ Message Handler  │
└────────┬─────────┘
         │ 8. Receive command
         │
         │ 9. Send ACK (status: ack)
         │
         │ 10. Execute StartSessionHandler
         ▼
┌──────────────────┐
│ K8s Operations   │
└────────┬─────────┘
         │ 11. Create Deployment
         │     (image: linuxserver/firefox)
         │
         │ 12. Create Service
         │     (ClusterIP, port 3000)
         │
         │ 13. Create PVC (if persistent)
         │
         │ 14. Wait for pod ready
         │     (poll until Running + Ready)
         ▼
┌──────────────────┐
│ VNC Tunnel       │
│ Manager          │
└────────┬─────────┘
         │ 15. Create port-forward
         │     (pod:5900 → local port)
         │
         │ 16. Send vnc_ready
         ▼
┌──────────────────┐
│ Control Plane    │
│ Command          │
│ Dispatcher       │
└────────┬─────────┘
         │ 17. Receive complete
         │
         │ 18. Update session status
         │     (state: running, pod_ip, vnc_port)
         │
         │ 19. Update command status
         │     (status: completed)
         ▼
┌─────┐
│ UI  │ 20. Poll session status
│     │     GET /api/v1/sessions/sess-456
└─────┘     → {state: "running", url: "/vnc-viewer/sess-456"}
```

### VNC Connection Flow

```
┌─────┐
│ UI  │
└──┬──┘
   │ 1. User clicks "Connect" in session viewer
   │
   │ 2. Load iframe: /vnc-viewer/sess-456
   ▼
┌──────────────────┐
│ noVNC Static     │
│ Page             │
└────────┬─────────┘
         │ 3. Extract session_id from URL
         │
         │ 4. Read JWT from sessionStorage
         │
         │ 5. Connect WebSocket:
         │    wss://control.example.com/api/v1/vnc/sess-456?token=JWT
         ▼
┌──────────────────┐
│ VNC Proxy        │
│ Handler          │
└────────┬─────────┘
         │ 6. Verify JWT
         │
         │ 7. Lookup session → agent_id
         │
         │ 8. Check session state (must be running)
         │
         │ 9. Verify agent online
         │
         │ 10. Send vnc_connect to agent
         ▼
┌──────────────────┐
│ K8s Agent        │
│ VNC Handler      │
└────────┬─────────┘
         │ 11. Check if tunnel exists
         │     (created during session start)
         │
         │ 12. Send vnc_ready to Control Plane
         ▼
┌──────────────────┐
│ VNC Proxy        │
│ Handler          │
└────────┬─────────┘
         │ 13. Start bidirectional relay
         │
         │ UI ←→ Control Plane ←→ Agent ←→ Pod
         │
         │ Continuous data flow:
         │
         │ 14. UI sends keyboard/mouse input
         │     (binary WebSocket frame)
         │
         │ 15. Control Plane Base64-encodes
         │     → JSON to agent
         │
         │ 16. Agent Base64-decodes
         │     → forward to pod:5900
         │
         │ 17. Pod VNC server processes
         │     → generates screen update
         │
         │ 18. Pod sends VNC frame
         │     → agent port-forward
         │
         │ 19. Agent Base64-encodes
         │     → JSON to Control Plane
         │
         │ 20. Control Plane Base64-decodes
         │     → binary WebSocket to UI
         ▼
┌─────┐
│ UI  │ 21. noVNC renders screen update
│     │     User sees desktop
└─────┘
```

---

## VNC Architecture

### VNC Traffic Path (v2.0)

**Before (v1.x - Direct Access):**
```
UI Browser → session.status.url (http://10.42.1.5:3000) → Pod noVNC Interface
```
❌ Requires pod IP accessibility
❌ Firewall issues
❌ Single-cluster only

**After (v2.0 - Proxy Architecture):**
```
UI Browser
    ↓
Iframe: /vnc-viewer/{sessionId}
    ↓
noVNC Client (static HTML)
    ↓
WebSocket: /api/v1/vnc/{sessionId}?token=JWT
    ↓
Control Plane VNC Proxy
    ↓
Agent WebSocket (JSON messages with Base64 data)
    ↓
K8s Agent VNC Tunnel
    ↓
Kubernetes Port-Forward (SPDY)
    ↓
Session Pod :5900 (VNC Server)
```
✅ Firewall-friendly
✅ Centralized authentication
✅ Multi-cluster support
✅ Cross-network access

### VNC Components

**1. noVNC Client** (`api/static/vnc-viewer.html`, 238 lines)
- Static HTML page served by Control Plane
- Loads noVNC library from CDN (v1.4.0)
- Connects to VNC proxy with JWT authentication
- Handles RFB protocol events
- Keyboard shortcuts: Ctrl+Alt+Shift+F (fullscreen), Ctrl+Alt+Shift+R (reconnect)

**2. VNC Proxy** (`api/internal/handlers/vnc_proxy.go`, 430 lines)
- WebSocket endpoint for UI connections
- JWT authentication and session ownership verification
- Agent routing based on session's agent_id
- Bidirectional binary data forwarding (Base64-encoded)
- Connection lifecycle management

**3. VNC Tunnel Manager** (`agents/k8s-agent/vnc_tunnel.go`, 400+ lines)
- Kubernetes port-forward to pod:5900
- Binary VNC data relay
- Multi-session concurrent tunnels
- Automatic cleanup

### VNC Protocol Flow

**RFB (Remote Framebuffer) Protocol:**
- Client initiates handshake (protocol version, authentication)
- Server responds with framebuffer parameters (width, height, format)
- Client sends input events (keyboard, mouse, clipboard)
- Server sends framebuffer updates (screen changes)

**Encoding in v2.0:**
- RFB protocol is binary
- WebSocket between UI and Control Plane uses binary frames
- WebSocket between Control Plane and Agent uses JSON
- Binary data Base64-encoded in JSON messages for agent transport

---

## Security Architecture

### Authentication & Authorization

**1. User Authentication**
- JWT tokens (signed with secret key)
- Session expiration (configurable, default 24h)
- Token refresh mechanism
- SSO support (SAML, OIDC)

**2. Agent Authentication**
- Agent ID registration (pre-configured)
- Optional agent secrets/tokens
- WebSocket connection authentication
- TLS/SSL for all communications

**3. VNC Connection Security**
- JWT token required for VNC proxy connection
- Session ownership verification (user can only connect to own sessions)
- Single connection per session enforcement
- TLS/SSL for WebSocket

### Network Security

**1. TLS/SSL Everywhere**
- HTTPS for all API endpoints
- WSS (WebSocket Secure) for agent connections
- WSS for VNC connections
- Certificate-based authentication (optional)

**2. Firewall-Friendly Architecture**
- Agents initiate outbound connections only
- No inbound ports required on agent side
- NAT traversal built-in
- DMZ deployment supported

**3. Network Isolation**
- Session pods isolated by Kubernetes NetworkPolicy
- VNC traffic only accessible via agent (no direct pod access)
- Control Plane can be in different network than agents

### Data Security

**1. Database Encryption**
- Passwords hashed (bcrypt)
- Sensitive data encrypted at rest
- SSL/TLS for database connections

**2. Session Isolation**
- Each session runs in isolated container/VM
- No cross-session communication
- Resource limits enforced (CPU, memory)

**3. Audit Logging**
- All API calls logged
- Agent connections logged
- Session lifecycle events logged
- VNC connections logged
- Compliance support (SOC2, HIPAA, GDPR)

### RBAC (Role-Based Access Control)

**Roles:**
- **Super Admin**: Full system access
- **Admin**: Manage users, sessions, agents
- **User**: Create and access own sessions
- **Viewer**: Read-only access to own sessions

**Permissions:**
- `sessions.create`, `sessions.read`, `sessions.update`, `sessions.delete`
- `agents.register`, `agents.read`, `agents.update`, `agents.delete`
- `users.create`, `users.read`, `users.update`, `users.delete`
- `audit.read`, `config.update`, `license.manage`

---

## Scalability & High Availability

### Horizontal Scaling

**1. Control Plane**
- Deploy multiple replicas (2+ for HA)
- Load balancer distributes requests
- Stateless API (session state in database)
- WebSocket session persistence (sticky sessions or Redis)

**2. Agents**
- Deploy multiple agents per platform
- Each agent handles subset of sessions
- Load distributed by Control Plane agent selection
- Automatic failover on agent failure

**3. Database**
- PostgreSQL with replicas (read replicas for scaling)
- Connection pooling (PgBouncer)
- Partitioning for large tables (sessions, audit logs)

### Fault Tolerance

**1. Agent Failure**
- Agents automatically reconnect on network failure
- Sessions marked as "unknown" if agent offline
- Sessions can be migrated to different agent (manual or automatic)

**2. Control Plane Failure**
- Multiple replicas ensure availability
- Agents retry connection to any Control Plane replica
- Database persistence ensures session state recovery

**3. Session Failure**
- Pod crashes detected by agent
- Session marked as "failed" in database
- User can restart session
- Persistent storage (PVC) preserves user data

### Performance Optimization

**1. Caching**
- Agent status cached (30-second TTL)
- Session status cached (10-second TTL)
- Template metadata cached

**2. Database Optimization**
- Indexes on frequently queried columns (agent_id, session_id, user_id)
- Prepared statements for common queries
- Connection pooling

**3. VNC Optimization**
- Binary WebSocket frames for UI ↔ Control Plane
- Base64 encoding only for Control Plane ↔ Agent
- Compression for VNC data (optional)

---

## Platform Support

### Current Platforms

**1. Kubernetes (v2.0-beta)**
- Production-ready K8s Agent
- Supports any Kubernetes cluster (1.19+)
- Resources: Deployments, Services, PVCs
- RBAC permissions included

### Future Platforms

**2. Docker (v2.1 - Planned)**
- Docker Agent implementation
- Runs containers on Docker hosts
- Volume management for persistent storage
- Networking via Docker networks

**3. VMs (v2.2 - Future)**
- VM Agent for hypervisors (VMware, Hyper-V, KVM)
- VM provisioning and lifecycle management
- Snapshot support

**4. Cloud (v2.3 - Future)**
- Cloud Agent for AWS, Azure, GCP
- Provision cloud VMs/containers on-demand
- Auto-scaling based on demand

### Adding New Platforms

To add a new platform, create a new agent implementation:

**1. Agent Binary**
- Implement connection to Control Plane (reuse existing code)
- Implement command handlers:
  - `StartSessionHandler`: Platform-specific provisioning
  - `StopSessionHandler`: Platform-specific cleanup
  - `HibernateSessionHandler`: Pause session
  - `WakeSessionHandler`: Resume session
- Implement VNC tunneling (platform-specific port-forwarding)

**2. Platform-Specific Operations**
- Resource provisioning (containers, VMs, etc.)
- Networking (expose VNC port)
- Storage management (persistent volumes)

**3. Configuration**
- Define platform type (e.g., "vmware", "aws")
- Define capacity limits
- Define region/zone

**Example Agent Structure:**
```
agents/
├── k8s-agent/         # Kubernetes (v2.0)
├── docker-agent/      # Docker (v2.1)
├── vmware-agent/      # VMware (v2.2)
└── aws-agent/         # AWS EC2 (v2.3)

Each agent follows the same pattern:
- main.go              # Entry point
- connection.go        # WebSocket connection
- handlers.go          # Command handlers
- <platform>_operations.go  # Platform-specific API calls
- vnc_tunnel.go        # VNC tunneling
- config.go            # Configuration
```

---

## Summary

StreamSpace v2.0 introduces a revolutionary architecture that:

✅ **Multi-Platform Support**: Kubernetes, Docker, VMs, Cloud (extensible)
✅ **Firewall-Friendly**: Agents connect outbound, no inbound ports needed
✅ **Centralized Management**: Single Control Plane manages all platforms
✅ **VNC Proxying**: Cross-network session access via Control Plane
✅ **Scalable**: Horizontal scaling of Control Plane and agents
✅ **Fault-Tolerant**: Automatic reconnection and failover
✅ **Secure**: TLS/SSL everywhere, JWT authentication, RBAC
✅ **Production-Ready**: K8s Agent complete and tested

**Next Steps:**
- **Deployment**: See [V2_DEPLOYMENT_GUIDE.md](V2_DEPLOYMENT_GUIDE.md)
- **Migration**: See [V2_MIGRATION_GUIDE.md](V2_MIGRATION_GUIDE.md)
- **API Reference**: See [API_REFERENCE.md](../api/API_REFERENCE.md)

---

**Architecture Version**: 1.0
**Last Updated**: 2025-11-21
**StreamSpace Version**: v2.0.0-beta
