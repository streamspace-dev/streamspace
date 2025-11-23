# StreamSpace v2.0-beta.1 Integration Test Plan

**Document Version**: 1.0
**Created**: 2025-11-23
**Status**: Ready for Execution
**Priority**: P0 (Release Blocker)
**Estimated Time**: 16-24 hours

---

## Executive Summary

This document provides a complete integration test plan for the StreamSpace v2.0-beta.1 release. All test scripts, procedures, and success criteria are documented to enable independent execution.

**Scope**: End-to-end validation of StreamSpace v2.0 multi-platform architecture including:
- Session lifecycle management (creation, monitoring, termination)
- Template CRUD operations
- Agent failover and high availability
- Performance benchmarks and capacity testing

**Environment**: Local K3s cluster with 1 API pod, 1 K8s agent pod, PostgreSQL, Redis

**Prerequisites**:
- Docker Desktop with Kubernetes enabled
- kubectl and helm installed (Helm v3.18.0 recommended, NOT v4.0.x)
- Local images built via `./scripts/local-build.sh`

---

## Table of Contents

1. [Environment Setup](#environment-setup)
2. [Phase 1: Session Management Tests](#phase-1-session-management-tests)
3. [Phase 2: Template Management Tests](#phase-2-template-management-tests)
4. [Phase 3: Agent Failover Tests](#phase-3-agent-failover-tests)
5. [Phase 4: Performance Tests](#phase-4-performance-tests)
6. [Test Reporting](#test-reporting)
7. [Success Criteria](#success-criteria)
8. [Troubleshooting](#troubleshooting)

---

## Environment Setup

### Step 1: Verify Prerequisites

```bash
# Check Kubernetes cluster
kubectl cluster-info
kubectl version --client

# Check Helm version (MUST NOT be v4.0.x)
helm version

# Check Docker
docker version
```

**Expected**: All commands succeed, Helm is v3.18.0 or v3.16.x (NOT v4.0.x)

### Step 2: Build Local Images

```bash
cd /path/to/streamspace
./scripts/local-build.sh
```

**Expected**:
- `streamspace-api:local` image built
- `streamspace-k8s-agent:local` image built
- Images loaded into Docker Desktop Kubernetes

**Duration**: 5-10 minutes

### Step 3: Deploy StreamSpace

```bash
./scripts/local-deploy.sh
```

**Expected**:
- Namespace `streamspace` created
- PostgreSQL pod running (1/1 Ready)
- Redis pod running (1/1 Ready)
- API pod running (1/1 Ready)
- K8s Agent pod running (1/1 Ready)

**Verify Deployment**:
```bash
# Check all pods are running
kubectl get pods -n streamspace

# Check API is accessible
kubectl port-forward -n streamspace svc/streamspace-api 8080:8080 &
curl http://localhost:8080/health

# Expected: {"status":"ok"}
```

**Duration**: 3-5 minutes

### Step 4: Create Test Authentication Token

```bash
# Get admin credentials from API logs
kubectl logs -n streamspace -l app=streamspace-api | grep "Admin password"

# Login and get token
./tests/scripts/login.sh
```

**Expected**: Token saved to environment variable `$TOKEN`

**Duration**: 1-2 minutes

### Step 5: Verify Test Infrastructure

```bash
cd tests

# Run basic connectivity test
go test -v ./integration -run TestHealthEndpoint -timeout 30s
```

**Expected**: Test passes, confirming API connectivity

**Total Setup Time**: 10-20 minutes

---

## Phase 1: Session Management Tests

**Priority**: P0 (Core Functionality)
**Duration**: 6-8 hours
**Goal**: Validate complete session lifecycle from creation to termination

### Test 1.1a: Basic Session Creation

**Objective**: Verify sessions can be created via API

**Script**: `tests/scripts/phase1/test_1.1a_basic_session_creation.sh`

**Procedure**:
```bash
# Create a Firefox session
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "resources": {
      "cpu": "1000m",
      "memory": "2Gi"
    }
  }'
```

**Success Criteria**:
- ✅ HTTP 201 Created response
- ✅ Response includes `sessionId`, `name`, `status: "pending"`
- ✅ Session appears in `kubectl get sessions -n streamspace`
- ✅ Pod created with name matching session

**Validation**:
```bash
# Get session ID from response
SESSION_ID="<from-response>"

# Verify session in Kubernetes
kubectl get session $SESSION_ID -n streamspace -o yaml

# Verify pod exists
kubectl get pods -n streamspace -l session=$SESSION_ID
```

**Expected Duration**: 5-10 minutes
**Pass/Fail**: Document in test report with screenshots

---

### Test 1.1b: Session Startup Time

**Objective**: Measure time from creation to Running state

**Script**: `tests/scripts/phase1/test_1.1b_session_startup_time.sh`

**Procedure**:
```bash
# Record start time
START_TIME=$(date +%s)

# Create session
SESSION_RESPONSE=$(curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "resources": {"cpu": "1000m", "memory": "2Gi"}
  }')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.sessionId')

# Poll until Running
while true; do
  STATUS=$(curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID \
    -H "Authorization: Bearer $TOKEN" | jq -r '.status')

  if [ "$STATUS" == "Running" ]; then
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    echo "Session startup time: ${DURATION}s"
    break
  fi

  sleep 2
done
```

**Success Criteria**:
- ✅ Session reaches Running state
- ✅ Startup time < 60 seconds (target: 30-45s)
- ✅ Pod is Ready (1/1)
- ✅ VNC server is listening

**Metrics to Record**:
- Image pull time (if not cached)
- Pod scheduling time
- Container startup time
- VNC server initialization time
- Total end-to-end time

**Expected Duration**: 10-15 minutes (run 5 times, average results)
**Pass/Fail**: Pass if average < 60s, document actual times

---

### Test 1.1c: Resource Provisioning

**Objective**: Verify sessions receive requested resources

**Script**: `tests/scripts/phase1/test_1.1c_resource_provisioning.sh`

**Test Cases**:

1. **Minimum Resources**:
   - Request: 500m CPU, 1Gi memory
   - Verify: Pod gets exactly these limits

2. **Standard Resources**:
   - Request: 1000m CPU, 2Gi memory
   - Verify: Pod gets exactly these limits

3. **Maximum Resources**:
   - Request: 2000m CPU, 4Gi memory
   - Verify: Pod gets exactly these limits

4. **Invalid Resources**:
   - Request: 10000m CPU, 100Gi memory (exceeds node capacity)
   - Verify: Creation rejected with clear error

**Validation**:
```bash
# Check pod resource limits
kubectl get pod $POD_NAME -n streamspace -o jsonpath='{.spec.containers[0].resources}'
```

**Success Criteria**:
- ✅ Resources match request exactly
- ✅ Invalid requests rejected before pod creation
- ✅ Resource limits enforced by Kubernetes

**Expected Duration**: 15-20 minutes
**Pass/Fail**: All test cases pass

---

### Test 1.1d: VNC Browser Access

**Objective**: Verify users can access sessions via web browser

**Script**: `tests/scripts/phase1/test_1.1d_vnc_browser_access.sh`

**Procedure**:
```bash
# Create session and wait for Running
SESSION_ID=$(./tests/scripts/create_session_and_wait.sh firefox-browser)

# Get VNC connection URL
VNC_URL=$(curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID/connect \
  -H "Authorization: Bearer $TOKEN" | jq -r '.url')

echo "VNC URL: $VNC_URL"

# Test VNC proxy connectivity
curl -s -w "%{http_code}" $VNC_URL -o /dev/null
```

**Manual Verification** (Document with screenshots):
1. Open VNC URL in browser
2. Verify noVNC client loads
3. Verify desktop appears
4. Take screenshot of working session

**Success Criteria**:
- ✅ VNC URL returned in API response
- ✅ VNC URL accessible (HTTP 200)
- ✅ noVNC client loads in browser
- ✅ Desktop visible and responsive

**Expected Duration**: 10-15 minutes
**Pass/Fail**: All criteria met + screenshots

---

### Test 1.1e: Mouse and Keyboard Interaction

**Objective**: Verify user input works correctly

**Script**: Manual testing + screenshots

**Procedure**:
1. Open session in browser (from Test 1.1d)
2. Click on desktop - verify click registered
3. Open terminal application
4. Type: `echo "Hello StreamSpace"` + Enter
5. Verify output appears
6. Test special keys: Ctrl+C, Tab, Arrow keys
7. Test mouse scroll
8. Take screenshots at each step

**Success Criteria**:
- ✅ Mouse clicks register accurately
- ✅ Keyboard input appears in applications
- ✅ Special keys work (Ctrl, Alt, Tab, etc.)
- ✅ Mouse scroll works
- ✅ No noticeable input lag (< 100ms)

**Expected Duration**: 15-20 minutes
**Pass/Fail**: All interactions work smoothly

---

### Test 1.2: Session State Persistence

**Objective**: Verify session state survives pod restarts

**Script**: `tests/scripts/phase1/test_1.2_session_state_persistence.sh`

**Procedure**:
```bash
# 1. Create session
SESSION_ID=$(./tests/scripts/create_session_and_wait.sh firefox-browser)

# 2. Create a file in the session
POD_NAME=$(kubectl get pods -n streamspace -l session=$SESSION_ID -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n streamspace $POD_NAME -- bash -c "echo 'test data' > /home/user/test.txt"

# 3. Verify file exists
kubectl exec -n streamspace $POD_NAME -- cat /home/user/test.txt
# Expected: "test data"

# 4. Delete pod (simulate crash)
kubectl delete pod $POD_NAME -n streamspace

# 5. Wait for pod to recreate
kubectl wait --for=condition=ready pod -l session=$SESSION_ID -n streamspace --timeout=120s

# 6. Get new pod name
NEW_POD_NAME=$(kubectl get pods -n streamspace -l session=$SESSION_ID -o jsonpath='{.items[0].metadata.name}')

# 7. Verify file still exists
kubectl exec -n streamspace $NEW_POD_NAME -- cat /home/user/test.txt
# Expected: "test data"
```

**Success Criteria**:
- ✅ File created in session
- ✅ Pod recreates after deletion
- ✅ File persists in new pod
- ✅ PVC mounted correctly

**Expected Duration**: 10-15 minutes
**Pass/Fail**: File persists across pod restart

---

### Test 1.3: Multi-User Concurrent Sessions

**Objective**: Verify multiple users can run sessions simultaneously

**Script**: `tests/scripts/phase1/test_1.3_multi_user_concurrent.sh`

**Procedure**:
```bash
# Create 5 sessions concurrently
for i in {1..5}; do
  (
    curl -X POST http://localhost:8080/api/v1/sessions \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{
        \"user\": \"user${i}\",
        \"template\": \"firefox-browser\",
        \"resources\": {\"cpu\": \"500m\", \"memory\": \"1Gi\"}
      }"
  ) &
done

wait

# Verify all sessions created
kubectl get sessions -n streamspace | grep Running | wc -l
# Expected: 5
```

**Success Criteria**:
- ✅ All 5 sessions created successfully
- ✅ Each session isolated (separate pods)
- ✅ No resource conflicts
- ✅ Each session accessible via VNC
- ✅ Sessions don't interfere with each other

**Expected Duration**: 20-30 minutes
**Pass/Fail**: All sessions run independently

---

### Test 1.4: Session Hibernation and Restore

**Objective**: Verify sessions can hibernate to save resources

**Script**: `tests/scripts/phase1/test_1.4_session_hibernation.sh`

**Procedure**:
```bash
# 1. Create session
SESSION_ID=$(./tests/scripts/create_session_and_wait.sh firefox-browser)

# 2. Hibernate session
curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/hibernate \
  -H "Authorization: Bearer $TOKEN"

# 3. Verify pod scaled to 0
kubectl get pods -n streamspace -l session=$SESSION_ID
# Expected: No pods running

# 4. Verify session status
curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer $TOKEN" | jq -r '.status'
# Expected: "Hibernated"

# 5. Wake session
curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/wake \
  -H "Authorization: Bearer $TOKEN"

# 6. Wait for pod to start
kubectl wait --for=condition=ready pod -l session=$SESSION_ID -n streamspace --timeout=120s

# 7. Verify session running again
curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer $TOKEN" | jq -r '.status'
# Expected: "Running"
```

**Success Criteria**:
- ✅ Hibernation scales pod to 0
- ✅ Status changes to "Hibernated"
- ✅ Wake restarts pod
- ✅ Status returns to "Running"
- ✅ Data persists through hibernate/wake cycle

**Expected Duration**: 15-20 minutes
**Pass/Fail**: Complete cycle works

---

## Phase 2: Template Management Tests

**Priority**: P1 (Important)
**Duration**: 2-4 hours
**Goal**: Validate template CRUD operations

### Test 2.1: Template Creation and Validation

**Objective**: Verify templates can be created and validated

**Script**: `tests/scripts/phase2/test_2.1_template_creation.sh`

**Test Cases**:

1. **Valid Template**:
```json
{
  "name": "custom-firefox",
  "displayName": "Custom Firefox",
  "description": "Firefox with custom settings",
  "image": "streamspace/firefox:latest",
  "category": "browsers",
  "resources": {
    "cpu": "1000m",
    "memory": "2Gi"
  },
  "vnc": {
    "port": 5900
  }
}
```

2. **Missing Required Fields**:
```json
{
  "name": "invalid-template"
  // Missing image, resources
}
```

3. **Invalid Image Format**:
```json
{
  "name": "bad-image",
  "image": "not-a-valid-image:reference::",
  "resources": {"cpu": "1000m", "memory": "2Gi"}
}
```

**Success Criteria**:
- ✅ Valid template creates successfully
- ✅ Template appears in GET /api/v1/templates
- ✅ Invalid templates rejected with clear errors
- ✅ Validation catches all malformed inputs

**Expected Duration**: 30-45 minutes
**Pass/Fail**: All test cases pass

---

### Test 2.2: Template Updates and Versioning

**Objective**: Verify templates can be updated safely

**Script**: `tests/scripts/phase2/test_2.2_template_updates.sh`

**Procedure**:
```bash
# 1. Create template
TEMPLATE_ID=$(curl -X POST http://localhost:8080/api/v1/templates \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-template",
    "image": "streamspace/firefox:v1",
    "resources": {"cpu": "500m", "memory": "1Gi"}
  }' | jq -r '.id')

# 2. Create session using template
SESSION_ID=$(curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"user\": \"testuser\",
    \"template\": \"$TEMPLATE_ID\"
  }" | jq -r '.sessionId')

# 3. Update template
curl -X PUT http://localhost:8080/api/v1/templates/$TEMPLATE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "streamspace/firefox:v2",
    "resources": {"cpu": "1000m", "memory": "2Gi"}
  }'

# 4. Verify existing session unaffected
kubectl get pod -n streamspace -l session=$SESSION_ID -o jsonpath='{.spec.containers[0].image}'
# Expected: streamspace/firefox:v1 (original)

# 5. Create new session with updated template
NEW_SESSION_ID=$(curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"user\": \"testuser2\",
    \"template\": \"$TEMPLATE_ID\"
  }" | jq -r '.sessionId')

# 6. Verify new session uses updated template
kubectl get pod -n streamspace -l session=$NEW_SESSION_ID -o jsonpath='{.spec.containers[0].image}'
# Expected: streamspace/firefox:v2 (updated)
```

**Success Criteria**:
- ✅ Template updates successfully
- ✅ Existing sessions unaffected
- ✅ New sessions use updated template
- ✅ Version history tracked (if implemented)

**Expected Duration**: 45-60 minutes
**Pass/Fail**: Updates work without breaking existing sessions

---

### Test 2.3: Template Deletion Safety

**Objective**: Verify templates can't be deleted while in use

**Script**: `tests/scripts/phase2/test_2.3_template_deletion.sh`

**Procedure**:
```bash
# 1. Create template
TEMPLATE_ID=$(curl -X POST http://localhost:8080/api/v1/templates \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "delete-test",
    "image": "streamspace/firefox:latest",
    "resources": {"cpu": "500m", "memory": "1Gi"}
  }' | jq -r '.id')

# 2. Create session using template
SESSION_ID=$(curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"user\": \"testuser\",
    \"template\": \"$TEMPLATE_ID\"
  }" | jq -r '.sessionId')

# 3. Attempt to delete template (should fail)
HTTP_CODE=$(curl -s -w "%{http_code}" -o /tmp/delete_resp.json \
  -X DELETE http://localhost:8080/api/v1/templates/$TEMPLATE_ID \
  -H "Authorization: Bearer $TOKEN")

echo "Delete attempt returned: $HTTP_CODE"
cat /tmp/delete_resp.json

# Expected: HTTP 409 Conflict or 400 Bad Request
# Expected message: "Template in use by N sessions"

# 4. Terminate session
curl -X DELETE http://localhost:8080/api/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer $TOKEN"

# Wait for cleanup
sleep 10

# 5. Retry delete (should succeed now)
HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
  -X DELETE http://localhost:8080/api/v1/templates/$TEMPLATE_ID \
  -H "Authorization: Bearer $TOKEN")

echo "Second delete attempt returned: $HTTP_CODE"
# Expected: HTTP 200 or 204
```

**Success Criteria**:
- ✅ Cannot delete template while sessions exist
- ✅ Clear error message explaining why
- ✅ Can delete after all sessions terminated
- ✅ Deletion cleanup is complete

**Expected Duration**: 30-45 minutes
**Pass/Fail**: Safety checks work correctly

---

## Phase 3: Agent Failover Tests

**Priority**: P1 (High Availability)
**Duration**: 4-6 hours
**Goal**: Validate agent resilience and failover

### Test 3.1: Agent Disconnection During Active Sessions

**Status**: ✅ **ALREADY COMPLETED** (from previous work)

**Script**: `tests/scripts/phase3/test_3.1_agent_disconnection.sh`

**Verification**: Confirm test still passes

---

### Test 3.2: Command Retry During Agent Downtime

**Status**: ✅ **ALREADY COMPLETED** (from previous work)

**Script**: `tests/scripts/phase3/test_3.2_command_retry.sh`

**Verification**: Confirm test still passes

---

### Test 3.3: Agent Heartbeat and Health Monitoring

**Objective**: Verify agent health monitoring works correctly

**Script**: `tests/scripts/phase3/test_3.3_agent_heartbeat.sh`

**Procedure**:
```bash
# 1. Check agent is online
AGENT_ID=$(kubectl get pods -n streamspace -l app=streamspace-k8s-agent \
  -o jsonpath='{.items[0].metadata.name}')

curl -s http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" | jq '.agents[] | select(.status=="online")'

# 2. Monitor heartbeats (check database or logs)
kubectl logs -n streamspace $AGENT_ID | grep "Heartbeat sent" | tail -5

# 3. Block agent network (simulate network partition)
kubectl exec -n streamspace $AGENT_ID -- iptables -A OUTPUT -p tcp --dport 8080 -j DROP

# 4. Wait 60 seconds for heartbeat timeout
sleep 60

# 5. Check agent status (should be offline)
curl -s http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" | jq '.agents[] | select(.agentId=="'$AGENT_ID'")'

# Expected: status="offline"

# 6. Restore network
kubectl exec -n streamspace $AGENT_ID -- iptables -F OUTPUT

# 7. Wait for reconnection
sleep 30

# 8. Check agent status (should be online again)
curl -s http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" | jq '.agents[] | select(.agentId=="'$AGENT_ID'")'

# Expected: status="online"
```

**Success Criteria**:
- ✅ Heartbeats sent every 30 seconds
- ✅ Agent marked offline after missing 2 heartbeats (60s)
- ✅ Agent auto-reconnects when network restored
- ✅ Status transitions logged correctly

**Expected Duration**: 90-120 minutes
**Pass/Fail**: Health monitoring works as expected

---

### Test 3.4: Multi-Agent Load Balancing

**Objective**: Verify sessions distributed across multiple agents

**Script**: `tests/scripts/phase3/test_3.4_load_balancing.sh`

**Procedure**:
```bash
# 1. Scale K8s agent to 3 replicas
kubectl scale deployment streamspace-k8s-agent -n streamspace --replicas=3

# 2. Wait for all agents online
kubectl wait --for=condition=ready pod -l app=streamspace-k8s-agent -n streamspace --timeout=180s

# 3. Verify all agents connected
curl -s http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" | jq '.agents | length'
# Expected: 3

# 4. Create 15 sessions
for i in {1..15}; do
  curl -X POST http://localhost:8080/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"user\": \"user${i}\",
      \"template\": \"firefox-browser\",
      \"resources\": {\"cpu\": \"500m\", \"memory\": \"1Gi\"}
    }" &
done
wait

# 5. Check session distribution
kubectl get pods -n streamspace -l app.kubernetes.io/component=session \
  -o jsonpath='{range .items[*]}{.spec.nodeName}{"\n"}{end}' | sort | uniq -c

# Expected: Sessions distributed across agents (roughly 5 per agent)

# 6. Verify all sessions Running
kubectl get sessions -n streamspace | grep Running | wc -l
# Expected: 15
```

**Success Criteria**:
- ✅ All 3 agents connect successfully
- ✅ Sessions distributed (not all on one agent)
- ✅ Distribution roughly balanced (±2 sessions)
- ✅ All sessions reach Running state

**Expected Duration**: 90-120 minutes
**Pass/Fail**: Load balancing works

---

## Phase 4: Performance Tests

**Priority**: P1 (Production Readiness)
**Duration**: 4-6 hours
**Goal**: Validate performance meets targets

### Test 4.1: Session Creation Throughput

**Objective**: Measure session creation rate

**Target**: ≥10 sessions/minute

**Script**: `tests/scripts/phase4/test_4.1_creation_throughput.sh`

**Procedure**:
```bash
# Warm up (create 5 sessions, then delete)
for i in {1..5}; do
  SESSION_ID=$(curl -s -X POST http://localhost:8080/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"user":"warmup","template":"firefox-browser","resources":{"cpu":"500m","memory":"1Gi"}}' \
    | jq -r '.sessionId')

  # Wait for Running
  while [ "$(curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID -H "Authorization: Bearer $TOKEN" | jq -r '.status')" != "Running" ]; do
    sleep 2
  done

  # Delete
  curl -X DELETE http://localhost:8080/api/v1/sessions/$SESSION_ID \
    -H "Authorization: Bearer $TOKEN"
done

# Wait for cleanup
sleep 30

# Performance test: Create 20 sessions and measure time
START_TIME=$(date +%s)

for i in {1..20}; do
  curl -X POST http://localhost:8080/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"user\": \"perftest${i}\",
      \"template\": \"firefox-browser\",
      \"resources\": {\"cpu\": \"500m\", \"memory\": \"1Gi\"}
    }" &
done
wait

# Wait for all to reach Running
while [ $(kubectl get sessions -n streamspace | grep Running | wc -l) -lt 20 ]; do
  sleep 5
done

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
RATE=$(echo "scale=2; 60 * 20 / $DURATION" | bc)

echo "Created 20 sessions in ${DURATION}s"
echo "Throughput: ${RATE} sessions/minute"

# Expected: RATE >= 10
```

**Success Criteria**:
- ✅ Throughput ≥ 10 sessions/minute
- ✅ All sessions reach Running state
- ✅ No errors during creation

**Metrics to Record**:
- Total time for 20 sessions
- Sessions per minute
- Average time per session
- Peak resource usage during test

**Expected Duration**: 60-90 minutes (including multiple runs)
**Pass/Fail**: Meets 10 sessions/min target

---

### Test 4.2: Resource Usage Profiling

**Objective**: Profile resource consumption

**Script**: `tests/scripts/phase4/test_4.2_resource_profiling.sh`

**Metrics to Collect**:

1. **Idle Cluster** (no sessions):
   - API pod: CPU, memory
   - Agent pod: CPU, memory
   - PostgreSQL: CPU, memory, disk I/O
   - Redis: CPU, memory

2. **10 Active Sessions**:
   - API pod: CPU, memory
   - Agent pod: CPU, memory
   - Session pods: CPU, memory (average)
   - PostgreSQL: CPU, memory, connection count
   - Redis: CPU, memory, key count

3. **50 Active Sessions** (stress test):
   - Same metrics as above
   - Node resource utilization
   - Network throughput

**Procedure**:
```bash
# Install metrics-server if not present
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# 1. Measure idle
kubectl top pods -n streamspace > /tmp/metrics_idle.txt

# 2. Create 10 sessions
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"perftest${i}\",\"template\":\"firefox-browser\"}" &
done
wait

# Wait for all Running
kubectl wait --for=jsonpath='{.status.phase}'=Running session --all -n streamspace --timeout=300s

# Measure with 10 sessions
kubectl top pods -n streamspace > /tmp/metrics_10_sessions.txt

# 3. Create 40 more sessions (total 50)
for i in {11..50}; do
  curl -X POST http://localhost:8080/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"perftest${i}\",\"template\":\"firefox-browser\"}" &
done
wait

kubectl wait --for=jsonpath='{.status.phase}'=Running session --all -n streamspace --timeout=600s

# Measure with 50 sessions
kubectl top pods -n streamspace > /tmp/metrics_50_sessions.txt
kubectl top nodes > /tmp/metrics_nodes.txt

# Generate report
./tests/scripts/generate_resource_report.sh
```

**Success Criteria**:
- ✅ API pod CPU < 500m at 10 sessions
- ✅ API pod memory < 1Gi at 10 sessions
- ✅ Agent pod CPU < 200m at 10 sessions
- ✅ Agent pod memory < 512Mi at 10 sessions
- ✅ Node capacity not exceeded at 50 sessions

**Expected Duration**: 2-3 hours
**Pass/Fail**: Resource usage within acceptable limits

---

### Test 4.3: VNC Streaming Latency

**Objective**: Measure VNC streaming performance

**Script**: `tests/scripts/phase4/test_4.3_vnc_latency.sh`

**Procedure**:
1. Create session and connect via VNC
2. Use browser dev tools to measure:
   - WebSocket frame latency
   - Frame rate (FPS)
   - Bandwidth usage
3. Perform interactive actions and measure response time
4. Record metrics over 5-minute period

**Success Criteria**:
- ✅ WebSocket latency < 50ms (local network)
- ✅ Frame rate ≥ 15 FPS
- ✅ Mouse input lag < 100ms
- ✅ Keyboard input lag < 50ms

**Expected Duration**: 60-90 minutes
**Pass/Fail**: Latency meets targets

---

### Test 4.4: Concurrent Session Capacity

**Objective**: Determine maximum concurrent sessions

**Script**: `tests/scripts/phase4/test_4.4_concurrent_capacity.sh`

**Procedure**:
```bash
# Gradually increase load
for batch in 10 20 30 40 50 60 70 80; do
  echo "Testing ${batch} concurrent sessions..."

  # Create batch
  for i in $(seq 1 $batch); do
    curl -X POST http://localhost:8080/api/v1/sessions \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user\":\"capacity${i}\",\"template\":\"firefox-browser\"}" &
  done
  wait

  # Wait for all Running or timeout
  timeout 600 bash -c "while [ \$(kubectl get sessions -n streamspace | grep Running | wc -l) -lt $batch ]; do sleep 5; done" || {
    echo "Failed at ${batch} sessions"
    break
  }

  # Measure performance
  kubectl top pods -n streamspace > /tmp/capacity_${batch}.txt

  # Check for failures
  FAILED=$(kubectl get sessions -n streamspace | grep -E "Failed|Error" | wc -l)
  if [ $FAILED -gt 0 ]; then
    echo "Encountered ${FAILED} failures at ${batch} sessions"
    break
  fi

  # Cleanup for next batch
  kubectl delete sessions --all -n streamspace
  sleep 60
done

echo "Maximum capacity: ${batch} concurrent sessions"
```

**Success Criteria**:
- ✅ Determine max sessions before failures
- ✅ Document resource bottlenecks
- ✅ All sessions within capacity run successfully

**Expected Duration**: 3-4 hours
**Pass/Fail**: Capacity documented, no crashes

---

## Test Reporting

### Report Template

Each test phase should generate a report in `.claude/reports/`:

**File**: `INTEGRATION_TEST_RESULTS_PHASE_N_<date>.md`

**Template**:
```markdown
# StreamSpace v2.0-beta.1 Integration Test Results - Phase N

**Date**: YYYY-MM-DD
**Tester**: [Name]
**Environment**: Local K3s
**Duration**: X hours

## Test Summary

| Test ID | Test Name | Status | Duration | Notes |
|---------|-----------|--------|----------|-------|
| N.1 | Test Name | ✅ PASS | 15m | - |
| N.2 | Test Name | ❌ FAIL | 10m | See issue #XXX |

## Detailed Results

### Test N.1: Test Name

**Status**: ✅ PASS
**Duration**: 15 minutes

**Procedure**: [What was tested]

**Results**:
- Metric 1: Value (target: X)
- Metric 2: Value (target: Y)

**Evidence**: Screenshots/logs attached

**Issues Found**: None

### Test N.2: Test Name

**Status**: ❌ FAIL
**Duration**: 10 minutes

**Procedure**: [What was tested]

**Expected**: [What should happen]

**Actual**: [What actually happened]

**Error Details**:
```
[Error message/stack trace]
```

**Root Cause**: [Analysis]

**Issue Filed**: #XXX

## Environment Details

- Kubernetes Version: X.Y.Z
- StreamSpace Version: v2.0-beta
- Node Resources: X CPU, Y GB RAM
- Number of Agents: N

## Performance Metrics

[Any performance data collected]

## Conclusion

[Overall assessment]

## Next Steps

[What needs to be done]
```

---

## Success Criteria

### Phase 1 (Session Management)
- ✅ All session lifecycle tests pass
- ✅ VNC access works reliably
- ✅ State persistence verified
- ✅ Multi-user isolation confirmed

### Phase 2 (Template Management)
- ✅ CRUD operations work correctly
- ✅ Validation catches errors
- ✅ Safety checks prevent data loss

### Phase 3 (Agent Failover)
- ✅ Agents reconnect after failures
- ✅ Sessions survive agent restarts
- ✅ Load balancing distributes sessions
- ✅ Health monitoring accurate

### Phase 4 (Performance)
- ✅ Throughput ≥ 10 sessions/min
- ✅ Resource usage within limits
- ✅ VNC latency acceptable
- ✅ Capacity limits documented

### Overall Release Criteria
- ✅ **Zero P0 bugs** in core functionality
- ✅ **All critical paths tested** (session creation to termination)
- ✅ **Performance targets met**
- ✅ **Documentation complete**

---

## Troubleshooting

### Issue: API Not Accessible

**Symptoms**: `curl http://localhost:8080/health` fails

**Solution**:
```bash
# Check API pod status
kubectl get pods -n streamspace -l app=streamspace-api

# Check logs
kubectl logs -n streamspace -l app=streamspace-api

# Verify port forward
kubectl port-forward -n streamspace svc/streamspace-api 8080:8080
```

### Issue: Sessions Stuck in Pending

**Symptoms**: Sessions never reach Running state

**Solution**:
```bash
# Check session events
kubectl describe session $SESSION_ID -n streamspace

# Check pod events
kubectl get events -n streamspace --sort-by='.lastTimestamp'

# Common causes:
# - Image pull failures
# - Resource constraints
# - Agent not connected
```

### Issue: Agent Not Connecting

**Symptoms**: No agents listed in `/api/v1/agents`

**Solution**:
```bash
# Check agent pod
kubectl get pods -n streamspace -l app=streamspace-k8s-agent

# Check agent logs
kubectl logs -n streamspace -l app=streamspace-k8s-agent | grep -E "error|failed|connection"

# Verify WebSocket connectivity
kubectl logs -n streamspace -l app=streamspace-api | grep -E "agent.*connected"
```

### Issue: Tests Timeout

**Symptoms**: Tests hang or timeout

**Solution**:
- Increase test timeout: `go test -timeout 10m`
- Check for deadlocks in logs
- Verify cluster has sufficient resources

### Issue: Performance Below Targets

**Symptoms**: Throughput or latency worse than expected

**Solution**:
- Check node resources: `kubectl top nodes`
- Check image caching: Images should be pre-pulled
- Reduce session resource requests for testing
- Check database connection pool size

---

## Quick Reference

### Essential Commands

```bash
# Build and deploy
./scripts/local-build.sh && ./scripts/local-deploy.sh

# Check status
kubectl get all -n streamspace

# Get logs
kubectl logs -n streamspace -l app=streamspace-api --tail=100
kubectl logs -n streamspace -l app=streamspace-k8s-agent --tail=100

# Port forward API
kubectl port-forward -n streamspace svc/streamspace-api 8080:8080

# Run specific test
cd tests && go test -v ./integration -run TestName -timeout 30s

# Clean up
kubectl delete namespace streamspace

# Reset Kubernetes (if needed)
kubectl delete --all pods,sessions,templates -n streamspace
```

### Environment Variables

```bash
export STREAMSPACE_API_URL="http://localhost:8080"
export STREAMSPACE_TEST_TOKEN="<token-from-login>"
export NAMESPACE="streamspace"
```

### Test Execution Order

1. Environment Setup (mandatory first)
2. Phase 1: Session Management (must pass before Phase 2)
3. Phase 2: Template Management (can run in parallel with Phase 3)
4. Phase 3: Agent Failover (requires multiple agents)
5. Phase 4: Performance (run last, requires clean environment)

---

## Deliverables Checklist

- [ ] Environment successfully deployed
- [ ] Phase 1 tests completed (8 tests)
- [ ] Phase 2 tests completed (3 tests)
- [ ] Phase 3 tests completed (4 tests)
- [ ] Phase 4 tests completed (4 tests)
- [ ] Test reports generated for each phase
- [ ] Performance metrics documented
- [ ] Screenshots/evidence collected
- [ ] Issues filed for any bugs found
- [ ] Final summary report created
- [ ] v2.0-beta.1 readiness decision documented

---

**End of Integration Test Plan**

For questions or issues during execution, refer to:
- [TROUBLESHOOTING.md](../../docs/TROUBLESHOOTING.md)
- [DEPLOYMENT.md](../../DEPLOYMENT.md)
- GitHub Issues: https://github.com/streamspace-dev/streamspace/issues
