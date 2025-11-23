# BUG REPORT: P0 - Docker Agent Heartbeat JSON Parsing Error

**Priority**: P0 (Critical)
**Component**: Docker Agent → Control Plane WebSocket Communication
**Reported**: 2025-11-23
**Reporter**: Claude (Validator)
**Status**: Open - Requires Builder Investigation

---

## Summary

Docker agent sends heartbeat messages successfully, but Control Plane API rejects them with "unexpected end of JSON input" error, causing connections to be marked as stale and disconnected after 45 seconds.

---

## Environment

**Control Plane:**
- Version: feature/streamspace-v2-agent-refactor (commit 40904ca)
- Deployment: K8s cluster @ 192.168.0.60:8000
- API Image: Latest from feature branch

**Docker Agent:**
- Version: feature/streamspace-v2-agent-refactor (commit 40904ca)
- Deployment: Docker Swarm @ 192.168.0.11
- Mode: HA with Swarm backend leader election
- Replicas: 3

**Network:**
- WebSocket: ws://192.168.0.60:8000/api/v1/agents/connect
- Authentication: API Key (working)

---

## Symptoms

### Agent Logs (Successful Send)
```
2025/11/23 01:39:51 [Heartbeat] Sent heartbeat (activeSessions: 0)
```

### API Logs (Parse Error)
```
2025/11/23 01:39:51 [AgentWebSocket] Invalid heartbeat from agent docker-agent-swarm: unexpected end of JSON input
2025/11/23 01:40:10 [AgentHub] Detected stale connection for agent docker-agent-swarm (no heartbeat for >45s)
2025/11/23 01:40:10 [AgentWebSocket] Agent docker-agent-swarm disconnected
```

---

## Impact

**Severity**: P0 - Blocks production deployment of docker-agent

**Effects:**
1. ❌ **Connection Instability**: Agents disconnected every ~45 seconds
2. ❌ **Heartbeat Monitoring Broken**: Cannot track agent health
3. ❌ **Session Management Impaired**: Potential session interruptions
4. ⚠️ **HA Failover Risk**: Standby replicas cannot properly monitor leader health

**What Still Works:**
- ✅ Agent registration with API key
- ✅ WebSocket connection establishment
- ✅ Leader election (Swarm backend)
- ✅ Standby replica monitoring

---

## Root Cause Analysis

### Agent Heartbeat Code

File: `agents/docker-agent/main.go:495-524`

```go
func (a *DockerAgent) SendHeartbeats() {
    ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // BUG FIX P0-001: Use time.Now() instead of time.Now().Unix()
            // API expects RFC3339 JSON string, not Unix timestamp int64
            heartbeat := map[string]interface{}{
                "type":      "heartbeat",
                "timestamp": time.Now(), // Marshals to RFC3339 string in JSON
                "agentId":   a.config.AgentID,
                "status":    "online",
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
```

### Message Serialization

File: `agents/docker-agent/main.go:390-404`

```go
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
```

### Expected JSON Format

```json
{
  "type": "heartbeat",
  "timestamp": "2025-11-23T01:39:51Z",
  "agentId": "docker-agent-swarm",
  "status": "online",
  "activeSessions": 0
}
```

### Hypothesis: Possible Causes

1. **WebSocket Frame Fragmentation**
   - Large JSON messages may be split across multiple frames
   - API reads partial frame, gets incomplete JSON
   - Error: "unexpected end of JSON input"

2. **Buffer Truncation**
   - WritePump or ReadPump buffer size insufficient
   - Message truncated during send/receive
   - API receives partial JSON

3. **Race Condition**
   - Concurrent writes to WebSocket
   - Messages interleaved or corrupted
   - JSON parser receives malformed data

4. **Encoding Mismatch**
   - Agent sends in one encoding (e.g., binary)
   - API expects another (e.g., text)
   - JSON parser fails on unexpected bytes

---

## Comparison: K8s Agent (Working) vs Docker Agent (Broken)

### K8s Agent Heartbeat (WORKING)

File: `agents/k8s-agent/internal/agent/websocket.go` (approximate)

```go
// K8s agent successfully sends heartbeats
// No "unexpected end of JSON input" errors
// Connections remain stable for hours
```

**Key Difference to Investigate:**
- Does K8s agent use different WebSocket library?
- Does K8s agent use different JSON serialization?
- Does K8s agent send heartbeats differently?

---

## Previous Related Issues

### Original Issue (Fixed)

From testing report `.claude/reports/DOCKER_AGENT_HA_TESTING.md:206-210`:

```
**Invalid Heartbeat Message Format**:
2025/11/23 00:14:53 [AgentWebSocket] Invalid message from agent docker-agent-swarm:
                                     Time.UnmarshalJSON: input is not a JSON string

**Root Cause**: Heartbeat message timestamp field not properly JSON-encoded
```

**Fix Applied:** Changed `time.Now().Unix()` to `time.Now()` for RFC3339 string marshaling

**Result:** Different error - now "unexpected end of JSON input" instead of "Time.UnmarshalJSON"

---

## Reproduction Steps

1. Build docker-agent from feature/streamspace-v2-agent-refactor branch
2. Deploy to Docker Swarm with API key authentication:
   ```yaml
   environment:
     AGENT_ID: docker-agent-swarm
     CONTROL_PLANE_URL: ws://192.168.0.60:8000
     AGENT_API_KEY: <generated-key>
     ENABLE_HA: "true"
     LEADER_ELECTION_BACKEND: swarm
   ```
3. Deploy stack: `docker stack deploy -c config.yaml streamspace-agent`
4. Monitor API logs: `kubectl logs -n streamspace deployment/streamspace-api -f`
5. Observe: Agent connects successfully, then disconnected after ~45s with heartbeat error

---

## Investigation Needed (For Builder)

### 1. Compare WebSocket Implementations

**Files to Review:**
- `agents/docker-agent/main.go` (writePump, readPump, sendMessage)
- `agents/k8s-agent/internal/agent/websocket.go` (equivalent functions)
- `api/internal/websocket/agent_handler.go` (message parsing)

**Questions:**
- Are WebSocket message types (text/binary) set correctly?
- Are write/read buffers sized appropriately?
- Are concurrent writes properly serialized?

### 2. Debug Message Content

**Add Logging:**

In `agents/docker-agent/main.go:390-404`:
```go
func (a *DockerAgent) sendMessage(message interface{}) error {
    jsonData, err := json.Marshal(message)
    if err != nil {
        return fmt.Errorf("failed to marshal message: %w", err)
    }

    // DEBUG: Log exact JSON being sent
    log.Printf("[DEBUG] Sending JSON (%d bytes): %s", len(jsonData), string(jsonData))

    select {
    case a.writeChan <- jsonData:
        return nil
    // ...
}
```

In `api/internal/websocket/agent_handler.go` (heartbeat parsing):
```go
// DEBUG: Log raw message before parsing
log.Printf("[DEBUG] Received heartbeat raw (%d bytes): %s", len(messageBytes), string(messageBytes))

var heartbeat HeartbeatMessage
if err := json.Unmarshal(messageBytes, &heartbeat); err != nil {
    log.Printf("[AgentWebSocket] Invalid heartbeat from agent %s: %v", agentID, err)
    return
}
```

### 3. Check WebSocket Message Type

In `agents/docker-agent/main.go` writePump:
```go
// Ensure using TextMessage for JSON
err := a.ws.WriteMessage(websocket.TextMessage, message)
```

In `api/internal/websocket/agent_handler.go`:
```go
// Ensure reading TextMessage for JSON
messageType, message, err := conn.ReadMessage()
if messageType != websocket.TextMessage {
    log.Printf("[WARN] Expected TextMessage, got type %d", messageType)
}
```

### 4. Test Message Integrity

**Write Test:**
```go
func TestHeartbeatJSONIntegrity(t *testing.T) {
    heartbeat := map[string]interface{}{
        "type":           "heartbeat",
        "timestamp":      time.Now(),
        "agentId":        "test-agent",
        "status":         "online",
        "activeSessions": 0,
    }

    jsonData, err := json.Marshal(heartbeat)
    require.NoError(t, err)

    // Verify JSON is valid
    var decoded map[string]interface{}
    err = json.Unmarshal(jsonData, &decoded)
    require.NoError(t, err)

    // Verify all fields present
    assert.Equal(t, "heartbeat", decoded["type"])
    assert.NotNil(t, decoded["timestamp"])
    assert.Equal(t, "test-agent", decoded["agentId"])
}
```

---

## Workaround

**None Available** - Heartbeat is critical for connection stability.

**Not Recommended:** Disable heartbeat timeout (would mask agent failures)

---

## Recommended Fix Priority

**Priority**: P0 - Critical
**Severity**: Blocker for docker-agent production deployment
**Affected Users**: All docker-agent deployments
**Timeline**: Should be fixed before next release

---

## Related Files

### Agent Code
- `agents/docker-agent/main.go:390-404` (sendMessage)
- `agents/docker-agent/main.go:495-524` (SendHeartbeats)
- `agents/docker-agent/main.go:410-444` (writePump)
- `agents/docker-agent/main.go:446-492` (readPump)

### API Code
- `api/internal/websocket/agent_handler.go` (heartbeat parsing)
- `api/internal/websocket/hub.go` (stale connection detection)

### Testing
- `.claude/reports/DOCKER_AGENT_HA_TESTING.md` (original test results)

---

## Verification After Fix

### Success Criteria
1. ✅ Agent sends heartbeat every 30s
2. ✅ API receives and parses heartbeat successfully
3. ✅ No "unexpected end of JSON input" errors in API logs
4. ✅ Connection remains stable for >5 minutes
5. ✅ No stale connection detection/disconnection

### Test Commands

**Monitor Agent Logs:**
```bash
ssh s0v3r1gn@192.168.0.11 'docker service logs streamspace-agent_docker-agent -f' | grep -i heartbeat
```

**Monitor API Logs:**
```bash
kubectl logs -n streamspace deployment/streamspace-api -f | grep -E "docker-agent-swarm|heartbeat|stale"
```

**Expected Output (Success):**
```
# Agent Logs
[Heartbeat] Sent heartbeat (activeSessions: 0)
[Heartbeat] Sent heartbeat (activeSessions: 0)
...

# API Logs
[AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
[AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
...
```

---

## Additional Notes

### Context
- This issue surfaced during P0 bug fix verification for docker-agent
- Swarm leader election fix verified as working perfectly
- Heartbeat was previously broken with different error (Time.UnmarshalJSON)
- Partial fix applied changed error type but didn't resolve core issue

### Testing Environment
- Builder pushed P0 fixes to feature/streamspace-v2-agent-refactor
- Validator merged fixes and rebuilt docker-agent
- Applied database migration for agent API keys
- Generated API key: `162611746592cfb380fe9c3c9e59cefa041e441e8badf7ddd92dd909405444c1`
- Deployed 3-replica Swarm stack with leader election

---

**Report Generated**: 2025-11-23 01:45 PST
**Report Updated**: 2025-11-23 02:20 PST (FIX VERIFIED)
**Generated By**: Claude (Validator)
**Status**: ✅ RESOLVED - Fix verified and working

---

## ✅ FIX VERIFICATION (2025-11-23 02:20 PST)

### Fix Applied

**Commit**: 69e9498 on claude/v2-builder branch
**Fix Description**: "P0-NEW - Fix heartbeat JSON structure to match API expectations"

**Key Change**: Nested heartbeat data under "payload" field to match AgentMessage structure

**Fixed Code** (`agents/docker-agent/main.go:495-524`):
```go
heartbeat := map[string]interface{}{
    "type":      "heartbeat",
    "timestamp": time.Now(),
    "payload": map[string]interface{}{
        "status":         "online",
        "activeSessions": 0,
        "capacity": map[string]interface{}{
            "maxCpu":      a.config.Capacity.MaxCPU,
            "maxMemory":   a.config.Capacity.MaxMemory,
            "maxSessions": a.config.Capacity.MaxSessions,
        },
    },
}
```

### Verification Results

**Test Environment:**
- Control Plane API: K8s cluster @ 192.168.0.60:30800 (NodePort)
- Docker Agent: Swarm @ 192.168.0.11 (3 replicas, HA enabled)
- Deployment: Docker Stack with root user (for socket access)
- Configuration: WebSocket URL ws://192.168.0.60:30800

**Test Duration**: 7+ minutes (02:12:26 - 02:19:26+)

**Success Criteria Met**: ✅ ALL PASSED

1. ✅ **Agent sends heartbeat every 30s**
   - Verified: Consistent 30-second interval
   - Agent logs show: `[Heartbeat] Sent heartbeat (activeSessions: 0)`

2. ✅ **API receives and parses heartbeat successfully**
   - API logs show: `[AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)`
   - NO "unexpected end of JSON input" errors
   - NO "Time.UnmarshalJSON" errors

3. ✅ **No "unexpected end of JSON input" errors in API logs**
   - Zero parsing errors during 7+ minute test period
   - Clean heartbeat processing every 30 seconds

4. ✅ **Connection remains stable for >5 minutes**
   - Stable for 7+ minutes (and continuing)
   - 14+ heartbeats successfully processed
   - Zero connection interruptions

5. ✅ **No stale connection detection/disconnection**
   - No "Detected stale connection" messages for docker-agent-swarm
   - No disconnection after 45 seconds (previous behavior)
   - Connection maintained continuously

### Sample API Logs (Successful)

```
2025/11/23 02:12:26 [AgentWebSocket] Agent docker-agent-swarm connected (platform: docker)
2025/11/23 02:12:26 [AgentHub] Registered agent: docker-agent-swarm (platform: docker), total connections: 2
2025/11/23 02:12:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:13:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:13:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:14:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:14:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:15:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:15:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:16:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:16:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:17:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:17:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:18:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:18:56 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
2025/11/23 02:19:26 [AgentWebSocket] Heartbeat from agent docker-agent-swarm (status: online, activeSessions: 0)
```

### Additional Fixes Required for Deployment

**Issue 1: Docker Socket Permissions**
- **Problem**: Container user (agent:1000) cannot access Docker socket
- **Solution**: Run container as root (user: "0" in compose file)
- **Status**: ✅ Resolved in deployment config

**Issue 2: API Service Exposure**
- **Problem**: API ClusterIP service not accessible from Swarm network
- **Solution**: Changed service type to NodePort (port 30800)
- **Status**: ✅ Resolved via `kubectl patch`

**Issue 3: WebSocket URL Protocol**
- **Problem**: CONTROL_PLANE_URL used `http://` instead of `ws://`
- **Solution**: Changed to `ws://192.168.0.60:30800`
- **Status**: ✅ Resolved in deployment config

### P0 Bug Status Summary

**P0-001: Swarm Leader Election** - ✅ VERIFIED WORKING
**P0-NEW: Heartbeat JSON Parsing** - ✅ VERIFIED WORKING

Both P0 bugs are now resolved and verified in production-like deployment.

---

**Verified By**: Claude (Validator)
**Verification Date**: 2025-11-23 02:20 PST
**Merged From**: claude/v2-builder commit 69e9498
**Deployment**: docker-agent-swarm (3 replicas) @ 192.168.0.11
