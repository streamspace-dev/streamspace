# Test Plan: Core Platform (Session & Application System)

**Test Plan ID**: TP-003
**Author**: Agent 3 (Validator)
**Created**: 2025-11-19
**Status**: Active
**Priority**: Critical

---

## Objective

Validate that core platform functionality works correctly: session creation, template resolution, VNC connections, and application lifecycle. These are the CRITICAL issues preventing users from using basic platform features.

---

## Scope

### In Scope
- Session Name/ID mismatch in API response
- Template name resolution in session creation
- UseSessionTemplate session creation
- VNC URL availability on connection
- Heartbeat connection validation
- Installation status updates

### Out of Scope
- VNC quality/performance (Phase 6)
- Plugin functionality (separate test plan)
- UI component testing

---

## Test Environment

### Prerequisites
- StreamSpace API running
- Kubernetes cluster accessible
- Controller deployed and running
- Templates installed
- PostgreSQL database

### Test Data
- Firefox template
- Test user accounts
- Various session configurations

---

## Test Cases

### TC-CORE-001: Session Name Returned in API Response

**Priority**: Critical
**Type**: Integration
**Related Issue**: Session Name/ID Mismatch - API returns database ID instead of session name

**Preconditions**:
- API server running
- At least one session exists

**Steps**:
1. Create a session with name "test-session-001"
2. GET /api/v1/sessions
3. Verify response includes "name" field with value "test-session-001"
4. GET /api/v1/sessions/{name}
5. Verify response includes correct name
6. Verify UI SessionViewer can find session by name

**Expected Results**:
```json
{
  "sessions": [
    {
      "name": "test-session-001",  // NOT database ID
      "user": "testuser",
      "template": "firefox-browser",
      "status": "Running"
    }
  ]
}
```

**Verification**:
- `response.sessions[0].name` equals "test-session-001"
- NOT a UUID or numeric ID

**Test File**: `tests/integration/session_name_test.go`

---

### TC-CORE-002: Template Name Used in Session Creation

**Priority**: Critical
**Type**: Integration
**Related Issue**: Template Name Not Used - uses req.Template instead of resolved templateName

**Preconditions**:
- API server running
- Firefox template exists

**Steps**:
1. Get application ID for Firefox: GET /api/v1/applications
2. Create session using applicationId:
   ```json
   {
     "applicationId": "app-firefox-123",
     "user": "testuser"
   }
   ```
3. Verify session created with correct template name
4. GET the created session
5. Verify template field is "firefox-browser" (not empty, not applicationId)
6. Verify controller can find template

**Expected Results**:
- Session created successfully
- session.spec.template = "firefox-browser"
- Controller creates deployment using correct image
- Session reaches Running state

**Test File**: `tests/integration/session_template_test.go`

---

### TC-CORE-003: UseSessionTemplate Creates Session

**Priority**: Critical
**Type**: Integration
**Related Issue**: UseSessionTemplate only increments counter, doesn't create session

**Preconditions**:
- Session template exists
- API server running

**Steps**:
1. Create a session template: POST /api/v1/session-templates
2. Use the template: POST /api/v1/session-templates/{id}/use
3. Verify response includes session details
4. Verify session actually created in Kubernetes
5. GET /api/v1/sessions to find new session
6. Verify session is functional

**Expected Results**:
```json
{
  "session": {
    "name": "generated-session-name",
    "template": "from-session-template",
    "status": "Pending"
  }
}
```

**Verification**:
- Response includes session details
- Session exists in Kubernetes
- Session reaches Running state
- Use count incremented

**Test File**: `tests/integration/session_template_use_test.go`

---

### TC-CORE-004: VNC URL Available on Connection

**Priority**: Critical
**Type**: Integration
**Related Issue**: VNC URL Empty When Connecting - session.Status.URL may be empty

**Preconditions**:
- Session exists
- Session is in Running state (or will be)

**Steps**:
1. Create a new session
2. Immediately call connect: POST /api/v1/sessions/{name}/connect
3. Verify response includes VNC URL
4. Verify URL is valid and accessible
5. Test with session that was just created (not yet ready)
6. Verify API waits or polls for URL

**Expected Results**:
```json
{
  "url": "https://testuser-firefox.streamspace.local",  // NOT empty
  "connectionId": "conn-123"
}
```

**Verification**:
- URL is never empty string
- URL resolves to actual endpoint
- VNC frame loads correctly
- Handles pod startup delay gracefully

**Test File**: `tests/integration/session_vnc_url_test.go`

---

### TC-CORE-005: Heartbeat Validates Connection

**Priority**: Critical
**Type**: Integration
**Related Issue**: Heartbeat Has No Connection Validation

**Preconditions**:
- Active session with connection
- connectionId obtained from connect

**Steps**:
1. Create session and connect
2. Send heartbeat with correct connectionId
3. Verify heartbeat accepted
4. Send heartbeat with wrong connectionId (from different session)
5. Verify heartbeat rejected
6. Send heartbeat with expired/invalid connectionId
7. Verify heartbeat rejected
8. Verify stale connections cleaned up after timeout

**Expected Results**:
- Valid heartbeat: 200 OK
- Wrong session's connection: 403 Forbidden
- Invalid connectionId: 404 Not Found
- Stale connections expire after timeout

**Test File**: `tests/integration/session_heartbeat_test.go`

---

### TC-CORE-006: Installation Status Updates

**Priority**: Critical
**Type**: Integration
**Related Issue**: Installation Status Never Updates from 'pending'

**Preconditions**:
- Application available in catalog
- Application not yet installed

**Steps**:
1. Install application: POST /api/v1/applications/{id}/install
2. Verify initial status is "pending" or "installing"
3. Wait for Template CRD to be created
4. Poll status: GET /api/v1/applications/{id}
5. Verify status changes to "installed"
6. Verify installation time is set
7. Launch application and verify it works

**Expected Results**:
- Status transitions: pending -> installing -> installed
- Status is "installed" within 60 seconds
- Template CRD exists in cluster
- Application can be launched

**Test File**: `tests/integration/application_install_test.go`

---

### TC-CORE-007: Session Lifecycle E2E

**Priority**: Critical
**Type**: E2E

**Preconditions**:
- Full StreamSpace stack running

**Steps**:
1. Install application (if not installed)
2. Launch session from application
3. Wait for session to be Running
4. Connect to session
5. Verify VNC URL works
6. Send heartbeats
7. Hibernate session
8. Wake session
9. Verify session still works
10. Delete session

**Expected Results**:
- Complete lifecycle works end-to-end
- All API responses correct
- Session functional after hibernate/wake
- Cleanup successful

**Test File**: `tests/e2e/session_lifecycle_test.sh`

---

### TC-CORE-008: Session Name Uniqueness

**Priority**: High
**Type**: Integration

**Preconditions**:
- API server running

**Steps**:
1. Create session with name "unique-test"
2. Attempt to create another session with same name
3. Verify error returned
4. Delete first session
5. Create session with same name
6. Verify success

**Expected Results**:
- Duplicate name: 409 Conflict
- After deletion: Creation succeeds
- Clear error message

**Test File**: `tests/integration/session_uniqueness_test.go`

---

### TC-CORE-009: Template Not Found Handling

**Priority**: High
**Type**: Integration

**Preconditions**:
- API server running
- Template "nonexistent-template" does not exist

**Steps**:
1. Create session with nonexistent template
2. Verify error returned with helpful message
3. Verify no partial resources created
4. Check controller logs for clear error

**Expected Results**:
- 404 Not Found or 400 Bad Request
- Error message mentions template not found
- No orphaned resources
- Controller handles gracefully

**Test File**: `tests/integration/session_template_missing_test.go`

---

### TC-CORE-010: Session Status Conditions

**Priority**: Medium
**Type**: Integration
**Related Issue**: Session Status Conditions TODOs

**Preconditions**:
- API and controller running

**Steps**:
1. Create session that will fail (e.g., bad image)
2. Wait for failure
3. GET session status
4. Verify Status.Conditions contains failure reason
5. Create session with resource limit exceeded
6. Verify condition indicates resource issue

**Expected Results**:
- Conditions array populated with details
- Type, Status, Reason, Message present
- Failure reason is actionable
- User can understand what went wrong

**Test File**: `tests/integration/session_conditions_test.go`

---

## Test Data Requirements

### Session Fixtures

```yaml
# tests/fixtures/sessions/valid-session.yaml
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: test-valid-session
  namespace: streamspace-test
spec:
  user: testuser
  template: firefox-browser
  state: running
  resources:
    requests:
      memory: "2Gi"
      cpu: "1000m"
```

### Application Database Records

```sql
-- Test application
INSERT INTO applications (id, name, template_name, category)
VALUES ('app-test-001', 'Test Firefox', 'firefox-browser', 'browsers');
```

---

## Success Criteria

### Must Pass (CRITICAL - Blocks Basic Usage)
- TC-CORE-001: Session Name Returned
- TC-CORE-002: Template Name Used
- TC-CORE-003: UseSessionTemplate Creates Session
- TC-CORE-004: VNC URL Available
- TC-CORE-005: Heartbeat Validates
- TC-CORE-006: Installation Status Updates

### Should Pass
- TC-CORE-007: Session Lifecycle E2E
- TC-CORE-008: Session Name Uniqueness
- TC-CORE-009: Template Not Found Handling

### Nice to Have
- TC-CORE-010: Session Status Conditions

---

## Risks

1. **Kubernetes Dependency**: Tests require cluster access
2. **Timing Issues**: Pod startup can be slow
3. **State Dependencies**: Tests may affect each other

---

## Dependencies

- Builder completes Session Name/ID fix
- Builder completes Template Name fix
- Builder completes UseSessionTemplate fix
- Builder completes VNC URL fix
- Builder completes Heartbeat Validation fix
- Builder completes Installation Status fix

---

## Schedule

| Phase | Timeline | Status |
|-------|----------|--------|
| Test plan creation | Week 1 | Complete |
| Test implementation | Week 2 | Pending (after Builder Day 1-4 fixes) |
| Test execution | Week 2-3 | Pending |
| Bug reporting | Week 3 | Pending |
| Regression testing | Week 4 | Pending |

---

## Reporting

Results will be reported in:
- `tests/reports/core-platform-test-report.md`
- Updates to `MULTI_AGENT_PLAN.md` Agent Communication Log

Critical failures will trigger immediate notification to Builder with:
- Exact API call that failed
- Expected vs actual response
- Steps to reproduce
- Impact on user workflow
