# Docker Agent HA Testing Report

**Date**: 2025-11-22
**Test Environment**: Docker Swarm (4 nodes @ 192.168.0.11-14)
**Control Plane**: K8s cluster @ 192.168.0.60:8000
**Agent Version**: streamspace/docker-agent:latest (built from source)

---

## Executive Summary

Tested docker-agent deployment to Docker Swarm cluster with HA configuration. Successfully built and deployed agent, verified connectivity to Control Plane, and identified issues with both Swarm-native and file-based leader election backends.

**Status**: ⚠️ **PARTIAL SUCCESS** - Agents connect successfully, but leader election requires fixes

---

## Test Objectives

1. ✅ Build docker-agent image from source
2. ✅ Deploy to Docker Swarm with HA configuration (3 replicas)
3. ⚠️ Verify leader election functionality
4. ✅ Test agent connectivity to Control Plane
5. ✅ Document findings and issues

---

## Test Environment Setup

### Docker Swarm Cluster
```
Swarm Nodes:
  - 192.168.0.11 (Docker-Host1) - Manager, Leader
  - 192.168.0.12 (Docker-Host2) - Down
  - 192.168.0.13 (Docker-Host3) - Down
  - 192.168.0.14 (Docker-Host4) - Down

Note: Only manager node (Docker-Host1) was accessible.
      Nodes 2-4 showed SSH host key verification failures.
```

### Control Plane Access
```
K8s Cluster: Local K3s
Port Forward: kubectl port-forward --address 0.0.0.0 svc/streamspace-api 8000:8000
Local IP: 192.168.0.60
Agent URL: ws://192.168.0.60:8000
```

---

## Build Process

### Docker Image Build

**Location**: `/tmp/agents/docker-agent` on Docker Swarm manager
**Command**: `docker build --load -t streamspace/docker-agent:latest .`
**Result**: ✅ Success

```
Build Time: ~35 seconds
Image Size: 25.2 MB
Base Image: golang:1.21-alpine (builder), alpine:latest (runtime)
```

**Build Stages**:
1. Builder stage: Go 1.21 compilation with CGO disabled
2. Runtime stage: Alpine with CA certificates
3. **Issue Found**: Dockerfile creates non-root user 'agent' (UID 1000)
   - Required override to `user: root` in docker-compose.yaml
   - Reason: Docker socket access requires root permissions

---

## Deployment Testing

### Attempt 1: Swarm-Native Leader Election Backend

**Configuration**:
```yaml
Environment:
  ENABLE_HA: "true"
  LEADER_ELECTION_BACKEND: "swarm"
  CONTROL_PLANE_URL: "ws://192.168.0.60:8000"

Deployment:
  replicas: 3
  placement: node.role == manager
```

**Result**: ❌ **FAILED**

**Error**:
```
[DockerAgent] Running in HA mode (backend: swarm)
[DockerAgent] Failed to create leader elector: failed to create swarm backend:
  no task found with ID: 3f29d7487b6e
```

**Root Cause Analysis**:

File: `agents/docker-agent/internal/leaderelection/swarm_backend.go:68-92`

```go
// Get current task/container ID from hostname
hostname, err := os.Hostname()
// ...
taskID := hostname
if len(hostname) > 25 {
    // Docker task IDs are 25 characters
    taskID = hostname[:25]
}

// Find service ID by filtering tasks
taskFilter := filters.NewArgs()
taskFilter.Add("id", taskID)
tasks, err := dockerClient.TaskList(context.Background(), types.TaskListOptions{
    Filters: taskFilter,
})
if len(tasks) == 0 {
    return nil, fmt.Errorf("no task found with ID: %s", taskID)
}
```

**Problem**:
- Code assumes hostname is task ID
- Truncates to 25 characters
- Docker Swarm task API query fails with truncated/incorrect ID

**Recommendation**: Fix task ID detection logic to properly query Swarm API

---

### Attempt 2: File-Based Leader Election Backend

**Configuration**:
```yaml
Environment:
  ENABLE_HA: "true"
  LEADER_ELECTION_BACKEND: "file"
  LEADER_LOCK_FILE: "/tmp/streamspace-leader.lock"

Volumes:
  - leader-lock:/tmp  # Swarm volume for shared lock file
```

**Result**: ⚠️ **PARTIAL SUCCESS**

**Startup Logs**:
```
2025/11/23 00:14:21 [DockerAgent] Running in HA mode (backend: file)
2025/11/23 00:14:21 [LeaderElection:File] Using lock file: /var/run/streamspace/...
2025/11/23 00:14:23 [LeaderElection:File] Acquired lock: /var/run/streamspace/...
2025/11/23 00:14:23 [LeaderElection] 🎖️  Became leader for agent: docker-agent-swarm
2025/11/23 00:14:23 [DockerAgent] Connected to Control Plane: ws://192.168.0.60:8000
```

**Issue Found**:
- **All 3 replicas acquired leadership** (split-brain scenario)
- Indicates shared volume not actually shared between containers

**Possible Causes**:
1. Docker Swarm volume not properly configured for sharing
2. Each container created its own lock file copy
3. File locking not working across container boundaries

**Evidence from Logs**:
```
Instance 1: b2b814ad7c64 - Became leader
Instance 2: 6e40f5b9083b - Became leader
Instance 3: 6946dfb5f22f - Became leader
```

All three instances successfully acquired the lock simultaneously, which should be impossible with proper file-based locking.

---

## Control Plane Connectivity

### Connection Success

**API Logs** (`streamspace-api`):
```
2025/11/23 00:14:23 [AgentWebSocket] Agent docker-agent-swarm connected (platform: docker)
2025/11/23 00:14:23 [AgentHub] Registered agent: docker-agent-swarm (platform: docker),
                               total connections: 2
2025/11/23 00:14:23 [AgentHub] Agent docker-agent-swarm already connected,
                               closing old connection
2025/11/23 00:14:23 [AgentWebSocket] Agent docker-agent-swarm disconnected
2025/11/23 00:14:23 [AgentHub] Unregistered agent: docker-agent-swarm,
                               remaining connections: 1
```

**Observations**:

✅ **Positive**:
- All 3 agents successfully connected to Control Plane
- AgentHub correctly detected duplicate agent_id connections
- AgentHub properly closed old connections when new ones arrived
- Connection handling logic working as expected

⚠️ **Issues Found**:

1. **Invalid Heartbeat Message Format**:
```
2025/11/23 00:14:53 [AgentWebSocket] Invalid message from agent docker-agent-swarm:
                                     Time.UnmarshalJSON: input is not a JSON string
```

**Root Cause**: Heartbeat message timestamp field not properly JSON-encoded

2. **Stale Connection Detection**:
```
2025/11/23 00:15:10 [AgentHub] Detected stale connection for agent docker-agent-swarm
                               (no heartbeat for >45s)
2025/11/23 00:15:10 [AgentWebSocket] Agent docker-agent-swarm disconnected
```

**Root Cause**: Heartbeat messages failing due to JSON format issue above

---

## Issues Summary

### Critical Issues (P0)

1. **Swarm Backend Leader Election Broken**
   - File: `agents/docker-agent/internal/leaderelection/swarm_backend.go:68-92`
   - Issue: Task ID detection logic fails
   - Impact: Swarm-native HA mode unusable
   - Fix Required: Rewrite task ID detection to properly query Swarm API

2. **Heartbeat Message JSON Format**
   - Issue: Time field not properly serialized to JSON
   - Impact: Heartbeats rejected, agents disconnected after 45s
   - Fix Required: Ensure timestamp fields use proper JSON encoding

### High Priority Issues (P1)

3. **File Backend Volume Sharing**
   - Issue: Docker volume not properly shared between containers
   - Impact: All replicas become leaders (split-brain)
   - Fix Required: Investigate Docker Swarm volume sharing configuration
   - Alternative: Use Redis backend for distributed locking

### Medium Priority Issues (P2)

4. **Docker Socket Permissions**
   - Issue: Non-root user can't access Docker socket
   - Current Workaround: Override to root user in deployment
   - Fix Required: Add agent user to docker group in Dockerfile

5. **Swarm Node Connectivity**
   - Issue: Only manager node accessible, worker nodes unreachable
   - Impact: Cannot test true multi-node HA scenarios
   - Fix Required: Resolve SSH host key issues for worker nodes

---

## Test Results Matrix

| Test Case | Expected | Actual | Status |
|-----------|----------|--------|--------|
| Build docker-agent image | Image built successfully | Image built (25.2 MB) | ✅ PASS |
| Deploy to Swarm | 3 replicas running | 3 replicas running | ✅ PASS |
| Swarm leader election | 1 leader elected | All failed to start | ❌ FAIL |
| File leader election | 1 leader elected | All 3 became leaders | ❌ FAIL |
| Connect to Control Plane | Agents connect via WebSocket | All 3 connected | ✅ PASS |
| AgentHub registration | Agents registered | Registered with duplicate handling | ✅ PASS |
| Heartbeat mechanism | Regular heartbeats sent | JSON format error | ❌ FAIL |
| Connection persistence | Agents stay connected | Disconnected after 45s (stale) | ❌ FAIL |

**Overall Pass Rate**: 4/8 (50%)

---

## Positive Findings

Despite issues, several components worked correctly:

1. **Build System**: Docker multi-stage build works properly
2. **Deployment**: Docker Swarm deployment configuration is sound
3. **Networking**: Agents can reach Control Plane across network boundaries
4. **Connection Handling**: AgentHub properly manages connections
5. **Duplicate Detection**: AgentHub correctly identifies and handles duplicate agent IDs
6. **Code Structure**: Agent codebase is well-organized and maintainable

---

## Recommendations

### Immediate Actions (for next testing session)

1. **Fix Heartbeat JSON Format**
   - Priority: P0
   - Estimated Effort: 30 minutes
   - Impact: Enables persistent connections

2. **Switch to Redis Leader Election Backend**
   - Priority: P0
   - Estimated Effort: 1 hour
   - Reason: More reliable than file-based in distributed environments
   - Benefit: Proven solution (works in K8s agent)

3. **Fix Swarm Backend Task ID Detection**
   - Priority: P1
   - Estimated Effort: 2 hours
   - Approach: Use Docker container environment variables or API inspection

### Future Improvements

4. **Update Dockerfile for Docker Socket Access**
   - Add agent user to docker group
   - Test with non-root user

5. **Resolve Worker Node Connectivity**
   - Clear SSH known_hosts
   - Retest multi-node deployment

6. **Add Integration Tests**
   - Test leader election scenarios
   - Test failover behavior
   - Test session creation/termination

---

## Comparison: K8s Agent vs Docker Agent

### Working Features (K8s Agent)

| Feature | K8s Agent | Docker Agent |
|---------|-----------|--------------|
| Leader Election | ✅ Working (K8s leases) | ❌ Broken (both backends) |
| Control Plane Connection | ✅ Working | ✅ Working |
| Heartbeat | ✅ Working | ❌ JSON format issue |
| HA Mode | ✅ 3 replicas tested | ⚠️ Deployed but not functional |
| Failover | ✅ ~7 seconds | ❌ Not tested (LE broken) |

### Architectural Differences

**K8s Agent**:
- Uses Kubernetes leases API (native leader election)
- Proven robust through extensive testing
- Automatic pod replacement by K8s

**Docker Agent**:
- 3 leader election backends: file, redis, swarm
- File backend: Issues with volume sharing
- Swarm backend: Task ID detection bug
- Redis backend: Not tested (requires Redis deployment)

---

## Next Steps

### For Validator (Claude)

1. Create bug report for Swarm backend task ID detection
2. Create bug report for heartbeat JSON format issue
3. Test Redis backend leader election (requires Redis in Swarm)
4. Document workarounds for current issues

### For Builder (if available)

1. Fix heartbeat JSON format encoding
2. Fix Swarm backend task ID detection logic
3. Add docker group membership to Dockerfile
4. Add integration tests for leader election

---

## Appendix: Deployment Configurations

### Final Working Configuration (Partial)

**File**: `/tmp/docker-swarm-file-backend.yaml`

```yaml
version: '3.8'

services:
  docker-agent:
    image: streamspace/docker-agent:latest
    user: root  # Required for Docker socket access

    deploy:
      mode: replicated
      replicas: 3
      placement:
        constraints:
          - node.role == manager
        preferences:
          - spread: node.id
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M

    environment:
      AGENT_ID: docker-agent-swarm
      CONTROL_PLANE_URL: ws://192.168.0.60:8000
      PLATFORM: docker
      REGION: default
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: "file"
      LEADER_LOCK_FILE: "/tmp/streamspace-leader.lock"

    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:rw
      - leader-lock:/tmp

    networks:
      - streamspace

volumes:
  leader-lock:
    driver: local

networks:
  streamspace:
    driver: overlay
    attachable: true
```

---

## Conclusion

Docker agent successfully builds, deploys, and connects to Control Plane, demonstrating fundamental functionality. However, leader election and persistent connections require fixes before production readiness.

The architecture is sound, and most issues are fixable with targeted code changes. K8s agent success provides confidence that Docker agent can achieve similar reliability once identified issues are resolved.

**Recommendation**: Address P0 issues (heartbeat JSON format, leader election) before proceeding with further testing or production deployment.

---

**Testing Completed**: 2025-11-22 16:20 PST
**Report Generated By**: Claude (Validator)
**Total Test Duration**: ~45 minutes
