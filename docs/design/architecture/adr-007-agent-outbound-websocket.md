# ADR-007: Agent Outbound WebSocket (Firewall-Friendly Architecture)
- **Status**: Accepted
- **Date**: 2025-11-18
- **Owners**: Agent 2 (Builder)
- **Implementation**: agents/*/main.go (WebSocket dial)

## Context

StreamSpace v1.x required inbound connectivity to agents (K8s Service, LoadBalancer). This caused deployment issues:
- Agents behind NAT couldn't accept inbound connections
- Corporate firewalls blocked inbound traffic to agents
- Each agent required separate ingress/LoadBalancer (complex, costly)
- Port management challenging (allocate ports for each agent)

Enterprise deployments often have restrictive firewall rules allowing only outbound HTTPS.

## Decision

**Agents initiate outbound WebSocket connections to Control Plane.**

### Architecture

```
┌─────────────────────┐
│   Control Plane     │
│   (API Server)      │ ← Single ingress point (port 443)
│   wss://api:443/ws │
└──────────┬──────────┘
           ↑
           │ ① Agent connects outbound (wss://)
           │ ② Persistent WebSocket connection
           │ ③ Commands pushed via WebSocket
           │
   ┌───────┴───────┬────────────┬────────────┐
   │               │            │            │
┌──┴──────┐  ┌────┴────┐  ┌────┴────┐  ┌────┴────┐
│ Agent 1 │  │ Agent 2 │  │ Agent 3 │  │ Agent N │
│  (K8s)  │  │ (Docker)│  │  (Edge) │  │  (VPC)  │
└─────────┘  └─────────┘  └─────────┘  └─────────┘
   Behind       Behind       Behind       Behind
    NAT       Firewall       NAT       Corporate
                                        Firewall
```

### Implementation

**Agent Connects** (agents/k8s-agent/main.go, agents/docker-agent/main.go):
```go
func connectToControlPlane(controlPlaneURL string) (*websocket.Conn, error) {
    // Outbound WebSocket connection
    conn, _, err := websocket.DefaultDialer.Dial(controlPlaneURL, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    // Send registration message
    err = conn.WriteJSON(RegisterMessage{
        AgentID:  agentID,
        Platform: "kubernetes",  // or "docker"
        Region:   region,
    })

    return conn, nil
}

func main() {
    // Connect to Control Plane via outbound WebSocket
    conn, err := connectToControlPlane(os.Getenv("CONTROL_PLANE_URL"))
    
    // Maintain persistent connection
    go handleReconnection(conn)
    
    // Listen for commands
    for {
        var cmd Command
        err := conn.ReadJSON(&cmd)
        // Process command...
    }
}
```

**Control Plane Accepts** (api/internal/websocket/agent_hub.go):
```go
// AgentHub accepts incoming WebSocket connections
func (h *AgentHub) HandleAgentConnection(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    
    // Read registration
    var reg RegisterMessage
    conn.ReadJSON(&reg)
    
    // Track agent connection
    h.agents[reg.AgentID] = conn
    
    // Keep connection alive
    go h.handleHeartbeat(conn, reg.AgentID)
}
```

## Alternatives Considered

### Alternative A: Inbound to Agents (v1.x) ❌
- **Pros**: Direct connection, simple
- **Cons**: NAT/firewall issues, requires per-agent ingress
- **Verdict**: Rejected - Enterprise unfriendly

### Alternative B: Outbound from Agents (v2.0) ✅
- **Pros**: Works through NAT/firewalls, single ingress
- **Cons**: Control Plane must track connections
- **Verdict**: Accepted - Enterprise-ready

### Alternative C: Bidirectional (Mesh) ❌
- **Pros**: Flexible topology
- **Cons**: Complex, hard to secure
- **Verdict**: Rejected - Unnecessary complexity

### Alternative D: Agent Polling ❌
- **Pros**: Works everywhere
- **Cons**: High latency, inefficient
- **Verdict**: Rejected - Poor UX

## Rationale

### Why Outbound Connections?
1. **Firewall-Friendly**: Outbound HTTPS works through corporate firewalls
2. **NAT Traversal**: Agents behind NAT can connect
3. **Single Ingress**: Control Plane only exposes one endpoint (wss://api/ws)
4. **No Port Allocation**: No need to allocate ports per agent
5. **Security**: Agents authenticate to Control Plane (not vice versa)

### Why WebSocket?
1. **Persistent**: Long-lived connection enables instant command delivery
2. **Bidirectional**: Commands flow both ways (Control Plane ↔ Agent)
3. **Standard**: Built into HTTP stack (no extra ports)
4. **Universal**: Works everywhere (browsers, Go, containers)

## Consequences

### Positive ✅
1. **Enterprise Deployments**: Works behind corporate firewalls
2. **Edge Computing**: Agents in edge locations can connect
3. **Cost Reduction**: No LoadBalancer per agent
4. **Simplified Networking**: Single ingress (Control Plane)
5. **Security**: Centralized access control at Control Plane

### Negative ⚠️
1. **Connection Tracking**: Control Plane must track all agent connections
2. **Scalability**: Control Plane handles many WebSocket connections
3. **Reconnection Logic**: Agents must handle reconnection (exponential backoff)

### Solutions
- **Multi-Pod API**: Redis-backed AgentHub (Issue #211, Wave 17)
- **Connection Limits**: Monitor and alert on connection count
- **Graceful Degradation**: Handle agent disconnects gracefully

## Security Considerations

### Authentication
- **Shared Secret**: Agent authenticates with API key/secret
- **mTLS**: Optional mutual TLS for high-security deployments
- **Token-Based**: JWT in WebSocket handshake (future)

### Authorization
- **Agent Registration**: Agents register with agent_id, platform, region
- **Command Validation**: Control Plane validates agent authorized for command
- **Audit Logging**: All agent connections logged

### Network Security
- **TLS/WSS Only**: Always use encrypted WebSocket (wss://)
- **Origin Validation**: Control Plane validates WebSocket origin
- **Rate Limiting**: Protect against connection flooding

## Performance

### Connection Overhead
- **Per-Agent**: ~10KB memory (WebSocket connection + buffer)
- **1,000 Agents**: ~10MB memory
- **10,000 Agents**: ~100MB memory (acceptable)

### Latency
- **Command Delivery**: < 10ms (persistent connection)
- **Reconnection**: ~5s (exponential backoff: 1s, 2s, 4s, 8s)

### Scalability
- **Single API Pod**: 1,000-5,000 agents (depends on hardware)
- **Horizontal Scaling**: Multiple API pods + Redis AgentHub

## Reconnection Strategy

**Agent Reconnection Logic**:
```go
func maintainConnection() {
    backoff := 1 * time.Second
    maxBackoff := 60 * time.Second

    for {
        conn, err := connectToControlPlane(controlPlaneURL)
        if err != nil {
            log.Printf("Failed to connect, retrying in %v", backoff)
            time.Sleep(backoff)
            backoff = min(backoff*2, maxBackoff)  // Exponential backoff
            continue
        }

        backoff = 1 * time.Second  // Reset on success
        
        // Handle connection
        handleConnection(conn)
        
        // Connection lost, retry
    }
}
```

## Operational Considerations

### Monitoring
- **Metrics**: Active agent connections, connection failures, reconnections
- **Alerts**: No agents connected, high connection failure rate

### Troubleshooting
- **Agent Can't Connect**: Check Control Plane URL, firewall rules, TLS cert
- **Frequent Reconnections**: Check network stability, heartbeat timeout

## References

- **Implementation**:
  - agents/k8s-agent/main.go (K8s agent WebSocket client)
  - agents/docker-agent/main.go (Docker agent WebSocket client)
  - api/internal/websocket/agent_hub.go (Control Plane WebSocket server)
- **Related ADRs**:
  - ADR-005: WebSocket Command Dispatch (command protocol)
  - ADR-008: VNC Proxy via Control Plane (similar outbound pattern)
- **Issues**:
  - #211: Multi-pod API requires Redis AgentHub

## Approval

- **Status**: Accepted (implemented in v2.0-beta)
- **Approved By**: Agent 1 (Architect)
- **Implementation**: Agent 2 (Builder)
- **Release**: v2.0-beta
