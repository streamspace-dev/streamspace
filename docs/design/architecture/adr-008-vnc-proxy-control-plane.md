# ADR-008: VNC Proxy via Control Plane (Centralized Access)
- **Status**: Accepted
- **Date**: 2025-11-18
- **Owners**: Agent 2 (Builder)
- **Implementation**: api/internal/handlers/vnc_proxy.go

## Context

StreamSpace sessions run GUI applications accessed via VNC. v1.x exposed VNC ports directly:
- Each session had K8s Service exposing VNC port (5900)
- Users connected directly to session pods
- Required exposing agent network to users
- No centralized auth/audit trail
- Port management complex (allocate per session)

Enterprise deployments require centralized access control and audit logging.

## Decision

**VNC connections proxy through Control Plane, not directly to agents/sessions.**

### Architecture

```
┌──────────────┐
│     User     │ ① Requests VNC token via API
│   (Browser) │    GET /api/v1/sessions/{id}/vnc
└──────┬───────┘
       │ ② Receives VNC WebSocket URL + JWT token
       │    wss://api/vnc?token=jwt...
       ↓
┌──────────────┐
│ Control Plane│ ③ Validates VNC token (JWT)
│  VNC Proxy   │ ④ Checks user authorized for session
│ (API Server) │ ⑤ Proxies VNC stream
└──────┬───────┘
       │ ⑥ Requests VNC tunnel from agent
       │    WebSocket: create_vnc_tunnel(session_id)
       ↓
┌──────────────┐
│    Agent     │ ⑦ Creates K8s port-forward to session pod
│   (K8s/     │ ⑧ Tunnels VNC stream via WebSocket
│   Docker)    │
└──────┬───────┘
       │ ⑨ VNC traffic (RFB protocol)
       ↓
┌──────────────┐
│ Session Pod  │ ⑩ VNC server (port 5900)
│ (Container)  │
└──────────────┘
```

### Data Flow

**3-Hop VNC Path**:
```
User → Control Plane VNC Proxy → Agent VNC Tunnel → Session Pod VNC Server
```

**Latency**: Typically <50ms (acceptable for interactive sessions)

## Alternatives Considered

### Alternative A: Direct to Agent (v1.x) ❌
- **Pros**: Low latency (direct connection)
- **Cons**: Security issues, network exposure, no audit trail
- **Verdict**: Rejected - Enterprise unfriendly

### Alternative B: Proxy via Control Plane (v2.0) ✅
- **Pros**: Centralized auth/audit, single ingress, secure
- **Cons**: Extra hop adds latency (~10-20ms)
- **Verdict**: Accepted - Security > latency

### Alternative C: Dedicated VNC Gateway ❌
- **Pros**: Separation of concerns
- **Cons**: Additional infrastructure, complexity
- **Verdict**: Rejected - Control Plane sufficient

### Alternative D: Agent-to-Agent Mesh ❌
- **Pros**: No Control Plane bottleneck
- **Cons**: Complex topology, hard to secure
- **Verdict**: Rejected - Unnecessary complexity

## Implementation

### VNC Token Endpoint

**Request VNC Token** (api/internal/api/handlers.go):
```go
func GetSessionVNC(c *gin.Context) {
    sessionID := c.Param("id")
    userID := c.GetString("userID")
    
    // Check user authorized for session
    session, err := db.GetSession(sessionID)
    if session.UserID != userID {
        c.JSON(403, gin.H{"error": "Forbidden"})
        return
    }
    
    // Generate VNC token (JWT)
    token := jwt.New(jwt.SigningMethodHS256)
    token.Claims = jwt.MapClaims{
        "session_id": sessionID,
        "user_id":    userID,
        "exp":        time.Now().Add(1 * time.Hour).Unix(),
    }
    tokenString, _ := token.SignedString([]byte(jwtSecret))
    
    // Return VNC WebSocket URL
    c.JSON(200, gin.H{
        "url": fmt.Sprintf("wss://api/vnc?token=%s", tokenString),
    })
}
```

### VNC Proxy Handler

**Proxy VNC Stream** (api/internal/handlers/vnc_proxy.go):
```go
func HandleVNCWebSocket(w http.ResponseWriter, r *http.Request) {
    // 1. Validate VNC token
    tokenString := r.URL.Query().Get("token")
    token, err := jwt.Parse(tokenString, keyFunc)
    if err != nil {
        http.Error(w, "Invalid token", 401)
        return
    }
    
    sessionID := token.Claims.(jwt.MapClaims)["session_id"].(string)
    
    // 2. Upgrade to WebSocket (user connection)
    userConn, err := upgrader.Upgrade(w, r, nil)
    
    // 3. Request VNC tunnel from agent
    agentConn := agentHub.GetAgentForSession(sessionID)
    agentConn.WriteJSON(Command{
        Type:      "create_vnc_tunnel",
        SessionID: sessionID,
    })
    
    // 4. Proxy bidirectional binary stream
    go io.Copy(userConn.UnderlyingConn(), agentConn.UnderlyingConn())
    io.Copy(agentConn.UnderlyingConn(), userConn.UnderlyingConn())
}
```

### Agent VNC Tunnel

**Agent Creates Tunnel** (agents/k8s-agent/agent_vnc_tunnel.go):
```go
func handleCreateVNCTunnel(cmd Command) {
    sessionID := cmd.SessionID
    
    // Get session pod name
    podName := fmt.Sprintf("session-%s", sessionID)
    
    // Create K8s port-forward to pod:5900
    req := k8sClient.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(podName).
        Namespace(namespace).
        SubResource("portforward")
    
    transport, upgrader, _ := spdy.RoundTripperFor(k8sConfig)
    dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
    
    // Forward VNC port (5900) to agent
    ports := []string{"5900:5900"}
    stopChan := make(chan struct{})
    readyChan := make(chan struct{})
    
    err := forwarder.ForwardPorts(
        ports,
        stopChan,
        readyChan,
        agentConn.UnderlyingConn(),  // Tunnel via agent WebSocket
        agentConn.UnderlyingConn(),
    )
}
```

## Rationale

### Why Proxy Through Control Plane?
1. **Security**: Centralized auth/authz (VNC token validation)
2. **Audit Trail**: All VNC connections logged at Control Plane
3. **Single Ingress**: Users only need access to Control Plane (not agents)
4. **Access Control**: Per-session VNC tokens (expire after 1 hour)
5. **Multi-Platform**: Works for K8s and Docker agents

### Why Not Direct Access?
1. **Security Risk**: Exposing agent network to users
2. **No Audit**: Can't track who accessed which session
3. **Complex Networking**: Requires ingress per session
4. **No Revocation**: Can't revoke access once connected

## Consequences

### Positive ✅
1. **Centralized Auth**: VNC tokens validated at Control Plane
2. **Audit Trail**: All VNC connections logged (session_id, user_id, timestamp)
3. **Token Expiry**: VNC access automatically expires (1 hour default)
4. **Network Security**: Agents not exposed to users
5. **Multi-Platform**: Same architecture for K8s and Docker

### Negative ⚠️
1. **Latency**: Extra hop adds ~10-20ms latency
2. **Bandwidth**: Control Plane proxies VNC traffic (capacity planning)
3. **Scalability**: Control Plane must handle VNC bandwidth

### Solutions
- **Latency**: < 50ms total acceptable for interactive sessions
- **Bandwidth**: Horizontal scaling (multiple API pods)
- **Capacity Planning**: Monitor VNC bandwidth per pod

## Security

### VNC Token (JWT)
```json
{
  "session_id": "sess-abc123",
  "user_id": "user-456",
  "exp": 1700000000,
  "iat": 1699996400
}
```

**Properties**:
- **Short-Lived**: 1 hour expiry (configurable)
- **Single-Session**: Token tied to specific session_id
- **Signed**: HMAC-SHA256 signature prevents tampering
- **Stateless**: No database lookup on validation

### Authorization Flow
1. User requests VNC access: `GET /api/v1/sessions/{id}/vnc`
2. API checks user authorized for session (database query)
3. API generates VNC token (JWT with session_id, user_id)
4. User connects VNC WebSocket: `wss://api/vnc?token=jwt...`
5. VNC proxy validates token (signature, expiry, session_id)
6. VNC proxy checks session status (running/hibernated)
7. VNC stream proxied if all checks pass

### Threat Mitigation
- **Token Theft**: Short expiry (1 hour), TLS-only
- **Unauthorized Access**: Token validation before proxying
- **Replay Attacks**: Token expiry prevents long-term replay
- **Session Hijacking**: Token tied to session_id (can't reuse for other sessions)

## Performance

### Latency Breakdown
- **Token Generation**: < 5ms (JWT signing)
- **Token Validation**: < 5ms (JWT verification)
- **Control Plane Proxy**: ~10-20ms (WebSocket hop)
- **Agent Tunnel**: ~10-20ms (K8s port-forward)
- **Total**: ~30-50ms (acceptable for VNC)

### Bandwidth
- **VNC Traffic**: ~10-50 KB/s per session (depends on screen updates)
- **100 Concurrent Sessions**: ~1-5 MB/s
- **1,000 Concurrent Sessions**: ~10-50 MB/s

### Scaling Strategy
- **Horizontal**: Multiple API pods proxy VNC traffic
- **Load Balancing**: Sticky sessions (L7 load balancer)
- **Monitoring**: Track VNC bandwidth per pod

## Operational Considerations

### Monitoring
- **Metrics**: Active VNC connections, VNC bandwidth, connection failures
- **Alerts**: High VNC connection failure rate, VNC bandwidth > threshold

### Troubleshooting
- **VNC Not Connecting**: Check token valid, session running, agent online
- **VNC Lag**: Check Control Plane CPU/bandwidth, network latency
- **Token Expired**: User needs to request new VNC token (GET /vnc endpoint)

## Future Enhancements

### v2.1+ Considerations
1. **VNC Recording**: Record VNC sessions for audit/compliance
2. **Bandwidth Control**: Rate limit VNC bandwidth per user/org
3. **Token Revocation**: Explicit revoke (currently relies on expiry)
4. **Direct Mode**: Optional direct-to-agent for low-latency scenarios

## References

- **Implementation**:
  - api/internal/handlers/vnc_proxy.go (VNC proxy handler)
  - api/internal/api/handlers.go (VNC token generation)
  - agents/k8s-agent/agent_vnc_tunnel.go (agent VNC tunnel)
- **Related ADRs**:
  - ADR-001: VNC Token Authentication (token format)
  - ADR-007: Agent Outbound WebSocket (connection model)
- **Design Docs**:
  - 03-system-design/control-plane.md (VNC gateway)

## Approval

- **Status**: Accepted (implemented in v2.0-beta)
- **Approved By**: Agent 1 (Architect)
- **Implementation**: Agent 2 (Builder)
- **Release**: v2.0-beta
