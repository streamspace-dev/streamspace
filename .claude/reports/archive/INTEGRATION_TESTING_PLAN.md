# Integration Testing Plan - v2.0-beta

**Status**: 🔄 IN PROGRESS
**Priority**: P0 - CRITICAL
**Validator**: Claude Code (Agent 3)
**Date Started**: 2025-11-21
**Estimated Duration**: 1-2 days
**Dependencies**: ✅ All P1 fixes validated (NULL handling, agent_id tracking, JSON marshaling)

---

## Executive Summary

This document outlines the comprehensive integration testing plan for StreamSpace v2.0-beta. With all P1 fixes validated, we can now test the complete end-to-end system integration including VNC streaming, multi-agent coordination, failover scenarios, and performance characteristics.

**Prerequisites Met**:
- ✅ Session creation working with agent assignment
- ✅ Session termination working via WebSocket commands
- ✅ Agent-to-Control Plane communication stable
- ✅ Database tracking agent_id correctly
- ✅ Command payload JSON marshaling working

**Testing Environment**:
- Platform: Docker Desktop Kubernetes (macOS)
- Namespace: streamspace
- Components: API, K8s Agent, Controller, PostgreSQL, VNC pods

---

## Test Categories

### 1. E2E VNC Streaming Validation (P0 - CRITICAL)

**Objective**: Validate complete session lifecycle from API call to browser VNC access.

**Test Scenarios**:

#### 1.1 Basic Session Creation and VNC Access
```bash
# Test Steps:
1. Create session via API
2. Wait for session to reach "running" state
3. Verify pod is running with VNC container
4. Verify service is created with VNC port exposed
5. Access VNC via browser (port-forward or ingress)
6. Verify VNC display is responsive
7. Perform mouse/keyboard interactions
8. Terminate session
9. Verify VNC connection closes
10. Verify resources cleaned up
```

**Expected Results**:
- Session transitions: pending → starting → running
- Pod has 2 containers: app + VNC proxy
- Service exposes port 3000 (VNC)
- VNC accessible via browser at http://<service>:3000
- Mouse/keyboard input functional
- Clean termination with no orphaned resources

**Success Criteria**:
- ✅ Session creation < 30 seconds
- ✅ VNC accessible within 10 seconds of "running" state
- ✅ No connection drops during 5-minute session
- ✅ Termination completes within 10 seconds

---

#### 1.2 Session State Persistence
```bash
# Test Steps:
1. Create session and access VNC
2. Open application in VNC session (e.g., Firefox)
3. Navigate to a website
4. Hibernate session (if implemented)
5. Wait 30 seconds
6. Wake session
7. Verify application state preserved
8. Verify VNC reconnects automatically
```

**Expected Results**:
- Application state preserved across hibernation
- VNC session resumes without re-authentication
- No data loss during state transitions

**Success Criteria**:
- ✅ Application state 100% preserved
- ✅ VNC reconnection < 5 seconds
- ✅ No user-visible disruption

---

#### 1.3 Multi-User Concurrent Sessions
```bash
# Test Steps:
1. Create 5 sessions simultaneously (different users)
2. Access VNC for all 5 sessions
3. Perform interactions in each session concurrently
4. Monitor resource usage (CPU, memory, network)
5. Terminate 2 sessions
6. Create 2 new sessions
7. Verify no cross-session interference
8. Terminate all sessions
```

**Expected Results**:
- All 5 sessions reach "running" state
- Each VNC session isolated (no shared state)
- Resource limits enforced per session
- Clean session separation

**Success Criteria**:
- ✅ All sessions functional concurrently
- ✅ No resource contention errors
- ✅ No cross-session data leakage
- ✅ Clean creation/termination under load

---

### 2. Multi-Agent Session Creation Tests (P0)

**Objective**: Validate load distribution across multiple agents and agent selection logic.

**Test Scenarios**:

#### 2.1 Single Agent Load Distribution
```bash
# Test Steps:
1. Verify only 1 agent connected (k8s-prod-cluster)
2. Create 10 sessions rapidly
3. Verify all assigned to same agent
4. Check agent load (active_sessions count)
5. Terminate 5 sessions
6. Create 5 new sessions
7. Verify assignment still to same agent
```

**Expected Results**:
- All sessions assigned to k8s-prod-cluster
- Database shows correct agent_id for all sessions
- Agent handles load without errors

**Success Criteria**:
- ✅ 100% assignment success rate
- ✅ No "no agents available" errors
- ✅ Agent reports correct active_sessions count

---

#### 2.2 Multi-Agent Load Balancing (Future)
```bash
# Note: Requires 2+ agents configured
# Test Steps:
1. Connect 3 agents (k8s-prod-cluster, k8s-dev-cluster, k8s-test-cluster)
2. Create 15 sessions rapidly
3. Verify load distributed across agents
4. Check agent_commands table for command distribution
5. Verify each agent processes commands correctly
6. Terminate sessions
7. Verify commands sent to correct agents
```

**Expected Results**:
- Sessions distributed evenly (5-5-5 or 6-5-4)
- Least-loaded agent selected for each new session
- Each agent receives correct commands

**Success Criteria**:
- ✅ Load variance < 2 sessions between agents
- ✅ No agent overloaded while others idle
- ✅ 100% command routing success

---

### 3. Agent Failover and Reconnection Tests (P0)

**Objective**: Validate system resilience when agents disconnect and reconnect.

**Test Scenarios**:

#### 3.1 Agent Disconnection During Active Sessions
```bash
# Test Steps:
1. Create 5 sessions via API
2. Verify all sessions running
3. Restart k8s-agent deployment (kubectl rollout restart)
4. Monitor agent WebSocket connection
5. Wait for agent to reconnect
6. Verify sessions still accessible
7. Create new session post-reconnection
8. Terminate all sessions
```

**Expected Results**:
- Agent disconnects and reconnects within 30 seconds
- Existing sessions remain running (pods not deleted)
- New sessions can be created after reconnection
- Command processing resumes

**Success Criteria**:
- ✅ Agent reconnects within 30 seconds
- ✅ Zero session data loss
- ✅ Commands queued during disconnect processed after reconnection
- ✅ No manual intervention required

---

#### 3.2 Command Retry During Agent Downtime
```bash
# Test Steps:
1. Create session (session reaches "running")
2. Kill agent deployment (kubectl delete pod)
3. Immediately attempt session termination via API
4. Verify API returns HTTP 202 (command dispatched)
5. Verify command stored in agent_commands table
6. Wait for agent to restart
7. Monitor agent logs for command processing
8. Verify session terminated post-reconnection
```

**Expected Results**:
- API accepts termination request even with agent down
- Command stored in database with "pending" status
- Agent processes pending commands on reconnection
- Session terminated successfully

**Success Criteria**:
- ✅ API remains responsive during agent downtime
- ✅ Commands queued in database
- ✅ 100% command delivery after reconnection
- ✅ No lost commands

---

#### 3.3 Agent Heartbeat and Health Monitoring
```bash
# Test Steps:
1. Monitor agent WebSocket connections
2. Check agent heartbeat frequency
3. Simulate network latency (if possible)
4. Verify agent marked as unhealthy after timeout
5. Verify no new sessions assigned to unhealthy agent
6. Restore network
7. Verify agent marked as healthy
8. Verify new sessions assigned
```

**Expected Results**:
- Agent sends heartbeat every 30 seconds
- Unhealthy agents not assigned new sessions
- Agent recovery automatic

**Success Criteria**:
- ✅ Health status accurate within 1 minute
- ✅ No sessions assigned to unhealthy agents
- ✅ Automatic recovery without manual intervention

---

### 4. Performance Testing (P1)

**Objective**: Establish baseline performance metrics for v2.0-beta.

**Test Scenarios**:

#### 4.1 VNC Latency Testing
```bash
# Test Steps:
1. Create session with VNC
2. Measure latency metrics:
   - API response time (session creation)
   - Pod startup time (pending → running)
   - VNC connection time (first frame)
   - VNC frame rate (FPS)
   - Input lag (mouse/keyboard)
3. Repeat test 10 times
4. Calculate average, min, max, p95, p99
```

**Expected Metrics**:
- API response time: < 200ms
- Pod startup time: < 30 seconds
- VNC connection time: < 5 seconds
- VNC frame rate: 15-30 FPS
- Input lag: < 100ms

**Success Criteria**:
- ✅ P95 latency within targets
- ✅ Consistent performance across runs
- ✅ No degradation over time

---

#### 4.2 Throughput Testing
```bash
# Test Steps:
1. Create 20 sessions concurrently
2. Measure:
   - Sessions created per minute
   - Concurrent sessions supported
   - API request throughput
   - Database query performance
3. Monitor resource usage:
   - API CPU/memory
   - Agent CPU/memory
   - PostgreSQL CPU/memory
   - Node CPU/memory
```

**Expected Metrics**:
- Session creation rate: > 5 sessions/minute
- Concurrent sessions: 50+ (resource dependent)
- API throughput: > 100 req/sec
- Database query time: < 50ms

**Success Criteria**:
- ✅ Throughput meets targets
- ✅ Resource usage within limits
- ✅ No bottlenecks identified

---

## Test Execution Plan

### Phase 1: E2E VNC Validation (Day 1 - Morning)
1. Basic session creation and VNC access ✅
2. Session state persistence (if hibernate implemented)
3. Multi-user concurrent sessions

### Phase 2: Multi-Agent Testing (Day 1 - Afternoon)
1. Single agent load distribution ✅
2. Multi-agent load balancing (if multiple agents available)

### Phase 3: Failover Testing (Day 2 - Morning)
1. Agent disconnection during active sessions
2. Command retry during agent downtime
3. Agent heartbeat and health monitoring

### Phase 4: Performance Testing (Day 2 - Afternoon)
1. VNC latency testing
2. Throughput testing
3. Resource usage profiling

### Phase 5: Documentation (Day 2 - End of Day)
1. Compile test results
2. Document findings and recommendations
3. Update MULTI_AGENT_PLAN.md

---

## Test Environment Setup

### Prerequisites
```bash
# Verify environment
kubectl get nodes
kubectl get ns streamspace
kubectl get deployments -n streamspace
kubectl get pods -n streamspace

# Verify components
kubectl get deploy -n streamspace | grep -E "api|agent|postgres"

# Verify port-forward capability
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &
curl http://localhost:8000/health
```

### Test Tools
- `curl` - API testing
- `kubectl` - Resource verification
- `jq` - JSON parsing
- Browser - VNC access testing
- `psql` - Database verification

---

## Success Criteria Summary

### Must Pass (P0)
- ✅ E2E VNC streaming functional
- ✅ Session creation/termination reliable
- ✅ Agent failover with zero data loss
- ✅ Multi-user sessions isolated

### Should Pass (P1)
- ✅ Performance metrics within targets
- ✅ Load balancing functional (if multi-agent)
- ✅ Resource usage optimal

### Documentation Required
- ✅ All test results documented
- ✅ Performance baselines established
- ✅ Known issues logged
- ✅ Recommendations for v2.0 final release

---

## Risk Assessment

### High Risk Areas
1. **VNC Stability**: First full integration test of VNC stack
2. **Agent Failover**: Complex state management during disconnects
3. **Performance**: Unknown bottlenecks under load

### Mitigation Strategies
1. **Incremental Testing**: Test one scenario at a time
2. **Detailed Logging**: Capture all component logs during tests
3. **Rollback Plan**: Can revert to previous working state if critical issues found

---

## Next Steps

1. ✅ Create integration testing plan (this document)
2. Execute Phase 1: E2E VNC Validation
3. Execute Phase 2: Multi-Agent Testing
4. Execute Phase 3: Failover Testing
5. Execute Phase 4: Performance Testing
6. Document all findings in INTEGRATION_TEST_RESULTS.md
7. Update MULTI_AGENT_PLAN.md with completion status

---

**Validator**: Claude Code (Agent 3)
**Branch**: claude/v2-validator
**Status**: 🔄 Ready to Execute
**Last Updated**: 2025-11-21
