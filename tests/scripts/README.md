# StreamSpace Test Scripts

This directory contains all test scripts used for validating StreamSpace v2.0-beta functionality during integration testing and bug fix validation.

**Last Updated**: 2025-11-21
**Version**: v2.0-beta
**Branch**: claude/v2-validator

---

## Prerequisites

Before running any test scripts, ensure:

1. **Kubernetes cluster** is running with StreamSpace deployed
2. **Port-forward to API** is active: `kubectl port-forward -n streamspace svc/streamspace-api 8000:8000`
3. **kubectl** is configured and connected to the cluster
4. **jq** is installed for JSON parsing
5. **curl** is available for API requests

---

## Test Scripts

### Core Integration Tests

#### test_e2e_vnc_streaming.sh
**Purpose**: End-to-end VNC streaming validation
**Tests**: Session creation → Pod provisioning → VNC tunnel creation → Session termination
**Usage**: `./test_e2e_vnc_streaming.sh`
**Duration**: ~60 seconds
**Key Validations**:
- JWT authentication
- Session creation API
- Pod startup time (~6 seconds)
- VNC tunnel establishment
- Resource cleanup

**Expected Output**:
```
✅ Token obtained
✅ Session created: admin-firefox-browser-<ID>
✅ Pod running: admin-firefox-browser-<ID>-<hash>
✅ Session terminated
```

---

#### test_multi_sessions_admin.sh
**Purpose**: Multi-user concurrent session testing (Test 1.3)
**Tests**: 5 concurrent sessions with resource isolation
**Usage**: `./test_multi_sessions_admin.sh`
**Duration**: ~90 seconds
**Key Validations**:
- Concurrent session creation (5 sessions)
- Pod provisioning (target: 100%, typical: 80%)
- Resource isolation (pod, deployment, service per session)
- VNC tunnel creation for all sessions
- Concurrent termination
- Complete cleanup

**Expected Output**:
```
✅ All 5 sessions created
✅ All 5 pods ready in 62 seconds
✅ All sessions have isolated resources
✅ Terminated 5 sessions
✅ All test session pods cleaned up
```

---

#### test_multi_user_concurrent_sessions.sh
**Purpose**: Multi-user concurrent session testing (original version with separate users)
**Tests**: 5 concurrent sessions for different users
**Usage**: `./test_multi_user_concurrent_sessions.sh`
**Duration**: ~90 seconds
**Note**: Requires users (user1-user5) to exist in the system. Use `test_multi_sessions_admin.sh` for testing with a single admin user.

---

### Bug Fix Validation Tests

#### test_vnc_tunnel_fix.sh
**Purpose**: P1-VNC-RBAC-001 fix validation
**Tests**: VNC tunnel creation with pods/portforward RBAC permission
**Usage**: `./test_vnc_tunnel_fix.sh`
**Duration**: ~40 seconds
**Key Validations**:
- Agent can create port-forwards to session pods
- No RBAC permission errors
- VNC tunnel established successfully
- Port-forward connection verified

**Expected Output**:
```
✅ Token obtained
✅ Session created: admin-firefox-browser-<ID>
✅ Pod running: admin-firefox-browser-<ID>-<hash>
✅ VNC tunnel created successfully!
```

**Agent Logs Should Show**:
```
[VNCTunnel] Port-forward established: localhost:<port> -> <pod>:3000
[VNCTunnel] Tunnel created successfully
```

---

#### test_complete_lifecycle_p1_all_fixes.sh
**Purpose**: Complete session lifecycle with all P1 fixes applied
**Tests**: Full session lifecycle after P0-RBAC-001, P0-MANIFEST-001, P1-VNC-RBAC-001 fixes
**Usage**: `./test_complete_lifecycle_p1_all_fixes.sh`
**Duration**: ~60 seconds
**Key Validations**:
- Template manifest parsing (lowercase JSON fields)
- Pod provisioning with correct resources
- VNC tunnel creation without RBAC errors
- Clean termination

---

#### test_termination_fix.sh
**Purpose**: Session termination fix validation
**Tests**: Proper session termination and resource cleanup
**Usage**: `./test_termination_fix.sh`
**Duration**: ~45 seconds
**Key Validations**:
- Session DELETE request returns 202
- Pod termination
- Deployment cleanup
- Service cleanup

---

#### test_termination_p1.sh
**Purpose**: Session termination P1 validation
**Tests**: Session termination after P1 fixes
**Usage**: `./test_termination_p1.sh`
**Duration**: ~45 seconds

---

### Session Lifecycle Tests

#### test_session_creation.sh
**Purpose**: Basic session creation test
**Tests**: Session creation API and pod provisioning
**Usage**: `./test_session_creation.sh`
**Duration**: ~30 seconds
**Key Validations**:
- Session creation API response
- Session ID extraction
- Pod creation
- Pod startup

---

#### test_session_creation_p1.sh
**Purpose**: Session creation test (P1 version)
**Tests**: Session creation after P1 fixes
**Usage**: `./test_session_creation_p1.sh`
**Duration**: ~30 seconds

---

#### test_session_termination.sh
**Purpose**: Session termination test
**Tests**: Session deletion API and cleanup
**Usage**: `./test_session_termination.sh`
**Duration**: ~45 seconds
**Key Validations**:
- DELETE /api/v1/sessions/{id} returns success
- Pod deleted
- Deployment deleted
- Service deleted

---

#### test_session_termination_new.sh
**Purpose**: Updated session termination test
**Tests**: Session termination with improved cleanup verification
**Usage**: `./test_session_termination_new.sh`
**Duration**: ~45 seconds
**Key Validations**:
- Termination API response
- 30-second cleanup window
- Complete resource removal

---

### Debugging & Utility Tests

#### check_api_response.sh
**Purpose**: API response format debugging
**Tests**: Session creation API response structure
**Usage**: `./check_api_response.sh`
**Duration**: ~10 seconds
**Output**: Full JSON response from session creation API

**Example Output**:
```json
{
  "name": "testuser-firefox-browser-abc123",
  "user": "testuser",
  "template": "firefox-browser",
  "state": "pending",
  "createdAt": "2025-11-22T05:00:00Z"
}
```

---

#### test_error_scenarios.sh
**Purpose**: Error handling validation
**Tests**: Various error scenarios and API error responses
**Usage**: `./test_error_scenarios.sh`
**Duration**: ~20 seconds
**Key Validations**:
- Invalid authentication (401)
- Missing template (404)
- Invalid resources (400)
- Permission errors

---

## Test Organization

### By Test Phase

**Phase 1: Session Lifecycle Tests**
- `test_session_creation.sh`
- `test_session_termination.sh`
- `test_e2e_vnc_streaming.sh`

**Phase 2: Multi-User/Concurrent Tests**
- `test_multi_sessions_admin.sh` (Integration Test 1.3)
- `test_multi_user_concurrent_sessions.sh`

**Phase 3: Bug Fix Validation**
- `test_vnc_tunnel_fix.sh` (P1-VNC-RBAC-001)
- `test_complete_lifecycle_p1_all_fixes.sh` (All P0/P1 fixes)
- `test_termination_fix.sh`

**Phase 4: Debugging**
- `check_api_response.sh`
- `test_error_scenarios.sh`

---

## Common Patterns

### Authentication
All tests use the same admin credentials:
```bash
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')
```

### Session Creation
Standard session creation request:
```bash
curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "512Mi", "cpu": "250m"},
    "persistentHome": false
  }'
```

### Pod Wait Loop
Wait for pod to be ready:
```bash
for i in {1..30}; do
  POD_STATUS=$(kubectl get pods -n streamspace -l session=${SESSION_ID} \
    -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
  if [ "$POD_STATUS" = "Running" ]; then
    break
  fi
  sleep 1
done
```

### Resource Cleanup Verification
Check for remaining resources:
```bash
kubectl get pods -n streamspace -l session=${SESSION_ID} --no-headers
kubectl get deployment -n streamspace ${SESSION_ID}
kubectl get service -n streamspace ${SESSION_ID}
```

---

## Test Results Documentation

Test results are documented in the repository root:

- `INTEGRATION_TEST_1.3_MULTI_USER_CONCURRENT_SESSIONS.md`
- `P1_VNC_RBAC_001_VALIDATION_RESULTS.md`
- `P0_MANIFEST_001_VALIDATION_RESULTS.md`
- `P0_RBAC_001_VALIDATION_RESULTS.md`

---

## Troubleshooting

### Port-forward not active
**Symptom**: `curl: (7) Failed to connect to localhost port 8000`
**Fix**: Start port-forward:
```bash
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000
```

### Authentication failed
**Symptom**: `{"error": "Unauthorized"}`
**Fix**: Verify admin password in API configuration

### Pod not starting
**Symptom**: Pod stuck in Pending/ImagePullBackOff
**Fix**: Check cluster resources and image availability:
```bash
kubectl describe pod -n streamspace <pod-name>
```

### VNC tunnel RBAC error
**Symptom**: `pods "..." is forbidden: cannot create resource "pods/portforward"`
**Fix**: Verify P1-VNC-RBAC-001 fix is applied:
```bash
kubectl get role streamspace-agent -n streamspace -o yaml | grep portforward
```

---

## Running All Tests

To run a comprehensive test suite:

```bash
# Phase 1: Basic session lifecycle
./test_session_creation.sh
./test_session_termination.sh

# Phase 2: VNC streaming validation
./test_e2e_vnc_streaming.sh
./test_vnc_tunnel_fix.sh

# Phase 3: Multi-session testing
./test_multi_sessions_admin.sh

# Phase 4: Complete lifecycle validation
./test_complete_lifecycle_p1_all_fixes.sh
```

---

## Contributing

When adding new test scripts:

1. Follow naming convention: `test_<feature>_<variant>.sh`
2. Include script header with purpose and description
3. Use consistent authentication and error handling
4. Document expected output and duration
5. Update this README with script details

---

**Generated**: 2025-11-21
**Validator**: Claude (v2-validator branch)
**Status**: All scripts validated and production-ready
