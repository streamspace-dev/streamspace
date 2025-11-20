# StreamSpace v2.0 Architecture Refactor: Control Plane + Multi-Platform Agents

**Version**: 2.0.0-alpha
**Date**: 2025-11-21
**Status**: Implementation in Progress

---

## Executive Summary

Refactoring StreamSpace from a **Kubernetes-native** architecture to a **multi-platform Control Plane + Agent** architecture.

**Key Changes:**
1. **Control Plane**: Centralized API managing sessions across all platforms
2. **Platform-Specific Agents**: K8s Agent, Docker Agent, future platform agents
3. **Outbound Connections**: Agents connect TO Control Plane (firewall-friendly)
4. **VNC Tunneling**: VNC traffic tunneled through Control Plane (multi-network support)
5. **Platform Abstraction**: Generic "Session" concept, agents handle platform specifics

---

## Current Architecture (v1.x - Kubernetes-Native)

```
┌─────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster (Single Cluster Required)               │
│                                                             │
│  ┌──────────┐      ┌─────────────────┐                    │
│  │ Web UI   │─────▶│ API (REST)      │                    │
│  └──────────┘      └─────────────────┘                    │
│       │                     │                              │
│       │                     │                              │
│       │                     ▼                              │
│       │            ┌─────────────────┐                     │
│       │            │ Kubebuilder     │                     │
│       │            │ Controller      │                     │
│       │            │                 │                     │
│       │            │ - Watches CRDs  │                     │
│       │            │ - Reconcile Loop│                     │
│       │            │ - Creates Pods  │                     │
│       │            └─────────────────┘                     │
│       │                     │                              │
│       │                     ▼                              │
│       │            ┌─────────────────┐                     │
│       │            │ Session Pods    │                     │
│       │            │ (with VNC)      │                     │
│       │            └─────────────────┘                     │
│       │                     │                              │
│       └─────────────────────┘                              │
│         Direct VNC Connection                              │
│         (Requires same cluster)                            │
└─────────────────────────────────────────────────────────────┘
```

**Limitations:**
- ❌ Kubernetes-only (no Docker, VM, or other platforms)
- ❌ Single cluster requirement (API, UI, Controller, Sessions all in one cluster)
- ❌ Direct VNC access requires network connectivity to pods
- ❌ Tight coupling to Kubernetes API
- ❌ No multi-region, multi-cluster support

---

## Target Architecture (v2.0 - Multi-Platform Control Plane)

```
┌─────────────────────────────────────────────────────────────────────┐
│ Control Plane (Centralized - Any Deployment)                        │
│                                                                      │
│  ┌──────────┐      ┌─────────────────────────────────┐             │
│  │ Web UI   │─────▶│ Control Plane API               │             │
│  └──────────┘      │                                 │             │
│       │            │ - Agent Registration            │             │
│       │            │ - WebSocket Hub (Agent Comms)   │             │
│       │            │ - Command Dispatcher            │             │
│       │            │ - VNC Proxy/Tunnel              │             │
│       │            │ - Session State Manager         │             │
│       │            └─────────────────────────────────┘             │
│       │                          │                                  │
│       │                          │ WebSocket (Outbound from Agents) │
│       │                          ▼                                  │
│       │            ┌──────────────────────────────┐                 │
│       │            │ VNC Proxy Endpoint           │                 │
│       │            │ /vnc/{session_id}            │                 │
│       │            │                              │                 │
│       │            │ - Accepts UI connections     │                 │
│       │            │ - Tunnels to appropriate Agent│                │
│       │            │ - Multiplexes VNC streams    │                 │
│       │            └──────────────────────────────┘                 │
│       │                          │                                  │
│       └──────────────────────────┘                                  │
│         VNC via Control Plane Proxy                                 │
└──────────────────────────────────────────────────────────────────────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
        ▼                          ▼                          ▼
┌────────────────┐      ┌────────────────┐       ┌────────────────┐
│ K8s Agent      │      │ Docker Agent   │       │ Future Agents  │
│ (Cluster 1)    │      │ (Host 1)       │       │ (VM, Cloud)    │
│                │      │                │       │                │
│ - K8s Client   │      │ - Docker API   │       │ - Platform API │
│ - Creates Pods │      │ - Runs Contnrs │       │ - Provisions   │
│ - Exposes VNC  │      │ - Exposes VNC  │       │ - Exposes VNC  │
│ - Tunnels to CP│      │ - Tunnels to CP│       │ - Tunnels to CP│
└────────────────┘      └────────────────┘       └────────────────┘
        │                       │                         │
        ▼                       ▼                         ▼
┌────────────────┐      ┌────────────────┐       ┌────────────────┐
│ Session Pod    │      │ Session Contnr │       │ Session VM     │
│ (K8s)          │      │ (Docker)       │       │ (Cloud)        │
└────────────────┘      └────────────────┘       └────────────────┘
```

**Benefits:**
- ✅ Multi-platform support (K8s, Docker, VMs, Cloud)
- ✅ Multi-cluster, multi-region support
- ✅ Agents can be anywhere (only need outbound HTTPS/WSS)
- ✅ VNC works across network boundaries
- ✅ Centralized control and monitoring
- ✅ Easy to add new platforms (write new agent)

---

## Component Architecture

### 1. Control Plane (Enhanced API)

**Location**: Can be deployed anywhere (K8s, VM, Docker, Cloud)

**Responsibilities:**
- Manage agent lifecycle (registration, heartbeat, deregistration)
- Maintain WebSocket connections with all agents
- Dispatch commands to agents (start, stop, hibernate sessions)
- Aggregate session status from all agents
- Proxy/tunnel VNC traffic between UI and agents
- Enforce licensing and resource limits
- Audit logging

**New Endpoints:**

```go
// Agent Management
POST   /api/v1/agents/register          // Agent registers itself
DELETE /api/v1/agents/{agent_id}        // Deregister agent
GET    /api/v1/agents                   // List all agents
GET    /api/v1/agents/{agent_id}        // Get agent details

// WebSocket for Agents
WS     /api/v1/agents/connect           // Agent establishes WebSocket

// VNC Proxy
WS     /vnc/{session_id}                // UI connects for VNC (proxied to agent)

// Session Management (Updated)
POST   /api/v1/sessions                 // Create session (CP dispatches to agent)
GET    /api/v1/sessions/{id}            // Get session (queries agent)
PUT    /api/v1/sessions/{id}/state      // Update state (hibernate, wake, terminate)
```

**WebSocket Protocol (Control Plane ↔ Agent):**

```json
// Agent → Control Plane (Registration)
{
  "type": "register",
  "payload": {
    "agent_id": "k8s-cluster-1",
    "platform": "kubernetes",
    "region": "us-east-1",
    "capacity": {
      "max_sessions": 100,
      "cpu": "64 cores",
      "memory": "256Gi"
    }
  }
}

// Control Plane → Agent (Command)
{
  "type": "command",
  "command_id": "cmd-123",
  "payload": {
    "action": "start_session",
    "session": {
      "id": "sess-456",
      "user": "john",
      "template": "firefox-browser",
      "resources": {
        "memory": "2Gi",
        "cpu": "1000m"
      }
    }
  }
}

// Agent → Control Plane (Status Update)
{
  "type": "status",
  "command_id": "cmd-123",
  "payload": {
    "session_id": "sess-456",
    "state": "running",
    "vnc_ready": true,
    "vnc_port": 5900,
    "pod_name": "sess-456-abc123" // Platform-specific details
  }
}

// VNC Tunnel Data (Bidirectional)
{
  "type": "vnc_data",
  "session_id": "sess-456",
  "data": "<base64-encoded-vnc-traffic>"
}
```

**Database Schema Changes:**

```sql
-- New: Agent Registry
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(255) UNIQUE NOT NULL,         -- "k8s-cluster-1"
    platform VARCHAR(50) NOT NULL,                 -- "kubernetes", "docker"
    region VARCHAR(100),                           -- "us-east-1", "eu-west-1"
    status VARCHAR(50) DEFAULT 'offline',          -- "online", "offline", "draining"
    capacity JSONB,                                -- max_sessions, cpu, memory
    last_heartbeat TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Updated: Sessions (Platform-Agnostic)
ALTER TABLE sessions ADD COLUMN agent_id VARCHAR(255) REFERENCES agents(agent_id);
ALTER TABLE sessions ADD COLUMN platform VARCHAR(50);
ALTER TABLE sessions ADD COLUMN platform_metadata JSONB;  -- Pod name, container ID, etc.

-- New: Command Queue
CREATE TABLE agent_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id VARCHAR(255) UNIQUE NOT NULL,
    agent_id VARCHAR(255) REFERENCES agents(agent_id),
    session_id UUID REFERENCES sessions(id),
    action VARCHAR(50) NOT NULL,                   -- "start_session", "stop_session"
    payload JSONB,
    status VARCHAR(50) DEFAULT 'pending',          -- "pending", "sent", "ack", "completed", "failed"
    created_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);
```

---

### 2. Kubernetes Agent

**What It Is:**
- Converted from current Kubebuilder controller
- Lightweight agent connecting to Control Plane
- Manages sessions as Kubernetes Pods

**Responsibilities:**
- Connect to Control Plane via WebSocket (outbound connection)
- Listen for commands from Control Plane
- Translate generic session specs → Kubernetes Pods/Services
- Report session status back to Control Plane
- Tunnel VNC traffic from pods to Control Plane

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│ Kubernetes Agent (Running in K8s Cluster)              │
│                                                         │
│  ┌────────────────────────────────────┐                │
│  │ Agent Manager (Main Loop)          │                │
│  │                                    │                │
│  │ - Connects to Control Plane WSS   │                │
│  │ - Sends heartbeats every 10s      │                │
│  │ - Listens for commands            │                │
│  └────────────────────────────────────┘                │
│           │                  │                          │
│           │                  │                          │
│           ▼                  ▼                          │
│  ┌───────────────┐  ┌──────────────────┐              │
│  │ K8s Client    │  │ VNC Tunnel Mgr   │              │
│  │               │  │                  │              │
│  │ - Create Pods │  │ - Port Forward   │              │
│  │ - Watch Status│  │ - Tunnel to CP   │              │
│  └───────────────┘  └──────────────────┘              │
│           │                  │                          │
│           ▼                  ▼                          │
│  ┌──────────────────────────────────┐                  │
│  │ Session Pods                     │                  │
│  │ - Application + VNC Container    │                  │
│  │ - VNC on port 5900               │                  │
│  └──────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────┘
```

**Command Handlers:**

```go
// Agent command handlers
func (a *KubernetesAgent) HandleCommand(cmd *Command) {
    switch cmd.Action {
    case "start_session":
        a.startSession(cmd.Payload)
    case "stop_session":
        a.stopSession(cmd.Payload)
    case "hibernate_session":
        a.hibernateSession(cmd.Payload)
    case "wake_session":
        a.wakeSession(cmd.Payload)
    }
}

func (a *KubernetesAgent) startSession(spec SessionSpec) {
    // 1. Translate generic spec → K8s Pod
    pod := a.buildPodFromSpec(spec)

    // 2. Create Pod in cluster
    _, err := a.k8sClient.CoreV1().Pods(a.namespace).Create(ctx, pod, metav1.CreateOptions{})

    // 3. Wait for Pod to be Running
    a.waitForPodRunning(pod.Name)

    // 4. Start VNC tunnel
    a.startVNCTunnel(spec.SessionID, pod.Name)

    // 5. Report status to Control Plane
    a.sendStatus(spec.SessionID, "running")
}
```

**VNC Tunneling:**

```go
// Tunnel VNC traffic from Pod to Control Plane WebSocket
func (a *KubernetesAgent) startVNCTunnel(sessionID, podName string) {
    // Port-forward to pod's VNC port (5900)
    portForwarder := a.createPortForward(podName, 5900)

    // Connect to local forwarded port
    vncConn, _ := net.Dial("tcp", "localhost:5900")

    // Tunnel all traffic through Control Plane WebSocket
    go func() {
        buffer := make([]byte, 32768)
        for {
            n, _ := vncConn.Read(buffer)
            a.sendVNCData(sessionID, buffer[:n])
        }
    }()

    // Receive VNC data from Control Plane and write to local connection
    go func() {
        for data := range a.vncDataChannel[sessionID] {
            vncConn.Write(data)
        }
    }()
}
```

---

### 3. Docker Agent

**What It Is:**
- Brand new agent for Docker platform
- Similar to K8s Agent but uses Docker API
- Manages sessions as Docker containers

**Responsibilities:**
- Connect to Control Plane via WebSocket
- Listen for commands
- Translate generic session specs → Docker containers
- Report session status
- Tunnel VNC traffic from containers to Control Plane

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│ Docker Agent (Running on Docker Host)                  │
│                                                         │
│  ┌────────────────────────────────────┐                │
│  │ Agent Manager (Main Loop)          │                │
│  │                                    │                │
│  │ - Connects to Control Plane WSS   │                │
│  │ - Sends heartbeats every 10s      │                │
│  │ - Listens for commands            │                │
│  └────────────────────────────────────┘                │
│           │                  │                          │
│           │                  │                          │
│           ▼                  ▼                          │
│  ┌───────────────┐  ┌──────────────────┐              │
│  │ Docker Client │  │ VNC Tunnel Mgr   │              │
│  │               │  │                  │              │
│  │ - Run Contnrs │  │ - Connect to VNC │              │
│  │ - Watch Status│  │ - Tunnel to CP   │              │
│  └───────────────┘  └──────────────────┘              │
│           │                  │                          │
│           ▼                  ▼                          │
│  ┌──────────────────────────────────┐                  │
│  │ Session Containers               │                  │
│  │ - Application + VNC              │                  │
│  │ - VNC on port 5900               │                  │
│  └──────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────┘
```

**Command Handlers:**

```go
func (a *DockerAgent) startSession(spec SessionSpec) {
    // 1. Translate generic spec → Docker container config
    containerConfig := a.buildContainerConfig(spec)

    // 2. Create container
    container, err := a.dockerClient.ContainerCreate(
        ctx,
        containerConfig,
        nil,
        nil,
        nil,
        spec.SessionID,
    )

    // 3. Start container
    a.dockerClient.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})

    // 4. Start VNC tunnel
    a.startVNCTunnel(spec.SessionID, container.ID)

    // 5. Report status
    a.sendStatus(spec.SessionID, "running")
}
```

---

### 4. UI Changes

**VNC Viewer Update:**

**Current** (Direct connection to pod):
```javascript
// ui/src/components/VNCViewer.jsx
const vncUrl = `ws://${podIP}:5900`;
const rfb = new RFB(canvas, vncUrl);
```

**New** (Proxy through Control Plane):
```javascript
// ui/src/components/VNCViewer.jsx
const vncUrl = `/vnc/${sessionId}`;  // Control Plane proxy endpoint
const rfb = new RFB(canvas, vncUrl);
```

**Session Creation Update:**

```javascript
// User selects platform when creating session (optional)
const createSession = async (template, platform = "auto") => {
    const response = await fetch('/api/v1/sessions', {
        method: 'POST',
        body: JSON.stringify({
            user: currentUser,
            template: template,
            platform: platform,  // "auto", "kubernetes", "docker"
            resources: {
                memory: "2Gi",
                cpu: "1000m"
            }
        })
    });
};
```

---

## Implementation Phases

### Phase 1: Design & Documentation ✅
- **Status**: In Progress
- **Tasks**:
  - ✅ Document target architecture
  - ✅ Define WebSocket protocol
  - ✅ Design database schema
  - Create sequence diagrams
  - Update API specifications

### Phase 2: Control Plane - Agent Registration & Management
- **Duration**: 3-5 days
- **Tasks**:
  - Add `agents` table to database
  - Implement `POST /api/v1/agents/register` endpoint
  - Implement `GET /api/v1/agents` (list/get agents)
  - Add agent heartbeat tracking
  - Add agent status monitoring

### Phase 3: Control Plane - WebSocket Command Channel
- **Duration**: 5-7 days
- **Tasks**:
  - Implement WebSocket hub for agent connections
  - Add command queue (`agent_commands` table)
  - Implement command dispatcher
  - Add command acknowledgment tracking
  - Handle agent reconnection logic

### Phase 4: Control Plane - VNC Proxy/Tunnel
- **Duration**: 5-7 days
- **Tasks**:
  - Implement `/vnc/{session_id}` WebSocket endpoint
  - Add VNC traffic multiplexer
  - Route VNC traffic between UI and appropriate agent
  - Handle connection failures and reconnection
  - Add bandwidth throttling (optional)

### Phase 5: K8s Agent - Convert Controller to Agent
- **Duration**: 7-10 days
- **Tasks**:
  - Extract current controller logic
  - Implement agent connection to Control Plane
  - Convert reconciliation loop → command handlers
  - Translate generic session spec → K8s Pod
  - Report session status to Control Plane
  - Handle agent reconnection

### Phase 6: K8s Agent - VNC Tunneling
- **Duration**: 3-5 days
- **Tasks**:
  - Implement port-forwarding to pod VNC port
  - Tunnel VNC traffic through WebSocket to Control Plane
  - Handle VNC connection lifecycle
  - Add error handling and reconnection

### Phase 7: Docker Agent - Build from Scratch
- **Duration**: 7-10 days
- **Tasks**:
  - Create Docker agent skeleton
  - Implement Docker client integration
  - Translate generic session spec → Docker container
  - Implement container lifecycle management
  - Implement VNC tunneling for Docker containers
  - Add Docker-specific features (volumes, networks)

### Phase 8: UI - Update VNC Viewer
- **Duration**: 2-3 days
- **Tasks**:
  - Update VNC viewer to use Control Plane proxy
  - Add platform selection to session creation
  - Update session list to show platform/agent info
  - Handle VNC connection errors gracefully

### Phase 9: Database Schema Updates
- **Duration**: 2-3 days
- **Tasks**:
  - Create migration for `agents` table
  - Update `sessions` table (add `agent_id`, `platform`, `platform_metadata`)
  - Create `agent_commands` table
  - Create indexes for performance
  - Test migrations

### Phase 10: Testing & Migration
- **Duration**: 5-7 days
- **Tasks**:
  - Test Control Plane with K8s Agent
  - Test Control Plane with Docker Agent
  - Test multi-agent scenarios
  - Test VNC streaming across network boundaries
  - Load testing (100+ concurrent sessions)
  - Create migration guide from v1.x
  - Document deployment patterns

---

## Recommended Implementation Order

### Option A: Bottom-Up (Recommended)
1. Phase 9: Database Schema (foundation)
2. Phase 2: Agent Registration (basic infrastructure)
3. Phase 3: WebSocket Command Channel (core communication)
4. Phase 5: K8s Agent (convert existing, validate architecture)
5. Phase 4: VNC Proxy (enable VNC streaming)
6. Phase 6: K8s Agent VNC Tunneling (complete K8s support)
7. Phase 8: UI Updates (user-facing changes)
8. Phase 7: Docker Agent (add second platform)
9. Phase 10: Testing & Migration (validation)

### Option B: Top-Down
1. Phase 9: Database Schema
2. Phase 2: Agent Registration
3. Phase 3: WebSocket Command Channel
4. Phase 7: Docker Agent (new platform, clean slate)
5. Phase 5: K8s Agent (convert existing)
6. Phase 4: VNC Proxy
7. Phase 6: K8s Agent VNC Tunneling
8. Phase 8: UI Updates
9. Phase 10: Testing & Migration

---

## Success Criteria

### Functional Requirements ✅
- [ ] Multiple agents (K8s + Docker minimum) can register with Control Plane
- [ ] Control Plane can dispatch session commands to agents
- [ ] Agents can create sessions on their respective platforms
- [ ] VNC streaming works across network boundaries
- [ ] UI can connect to sessions on any platform
- [ ] Sessions can be hibernated and woken across agents
- [ ] System handles agent failures gracefully

### Non-Functional Requirements ✅
- [ ] VNC latency < 100ms (with reasonable network)
- [ ] Support 100+ concurrent sessions across agents
- [ ] Agent reconnection within 30 seconds of network failure
- [ ] Zero downtime for Control Plane upgrades
- [ ] Backward compatibility with existing sessions (migration path)

### Documentation ✅
- [ ] Architecture documentation (this document)
- [ ] API specification updates
- [ ] Agent development guide
- [ ] Deployment guide (multi-platform)
- [ ] Migration guide from v1.x

---

## Migration Path from v1.x

### For Existing Users

1. **Deploy Control Plane** (v2.0 API)
2. **Deploy K8s Agent** in existing cluster (replaces controller)
3. **Migrate existing sessions** (optional, can recreate)
4. **Update UI** to use Control Plane proxy
5. **Test VNC connectivity**
6. **Decommission old controller**

### Backward Compatibility

- v2.0 API remains compatible with v1.x UI (with feature flags)
- Existing sessions can continue running during migration
- Gradual rollout: Run v1.x and v2.0 side-by-side temporarily

---

## Risks & Mitigations

### Risk 1: VNC Performance over WebSocket Tunnel
- **Impact**: High latency, choppy user experience
- **Mitigation**:
  - Use binary WebSocket frames (not base64)
  - Implement compression for VNC traffic
  - Add bandwidth throttling to prevent congestion
  - Benchmark early in Phase 4

### Risk 2: Agent Reconnection Complexity
- **Impact**: Lost sessions during network failures
- **Mitigation**:
  - Implement robust reconnection logic with exponential backoff
  - Persist command queue in database
  - Resume commands after reconnection
  - Test network failure scenarios extensively

### Risk 3: Database Bottleneck (Command Queue)
- **Impact**: Slow command dispatch at scale
- **Mitigation**:
  - Use database connection pooling
  - Implement in-memory command cache
  - Consider Redis for command queue (future optimization)
  - Load test with 1000+ agents

### Risk 4: Breaking Changes for v1.x Users
- **Impact**: Difficult migration, user frustration
- **Mitigation**:
  - Maintain v1.x API compatibility with feature flags
  - Provide automated migration tools
  - Document migration path clearly
  - Offer migration support

---

## Future Enhancements (v2.1+)

1. **Additional Platforms**
   - AWS EC2 Agent
   - Azure VM Agent
   - GCP Compute Agent
   - LXC/LXD Agent

2. **Advanced Features**
   - Session migration between agents
   - Load balancing across agents
   - Geo-aware agent selection
   - Multi-tenant agent isolation

3. **Performance Optimizations**
   - Direct agent-to-agent VNC routing (bypass Control Plane)
   - UDP-based VNC tunneling for lower latency
   - Hardware acceleration for VNC encoding

4. **Monitoring & Operations**
   - Agent health dashboard
   - Real-time VNC traffic metrics
   - Agent auto-scaling
   - Anomaly detection

---

## Questions for User

Before we proceed with implementation:

1. **Implementation Order**: Option A (Bottom-Up) or Option B (Top-Down)?
2. **VNC Tunneling**: Binary WebSocket or base64? Compression?
3. **Database**: PostgreSQL only or add Redis for command queue?
4. **Docker Agent Priority**: High (build early) or Low (after K8s Agent proven)?
5. **Testing**: Unit tests only, or also integration/E2E tests?
6. **Migration**: Support v1.x API compatibility or break compatibility cleanly?

---

**Next Steps**: Waiting for user decision on implementation approach, then begin Phase 2 (Control Plane - Agent Registration).
