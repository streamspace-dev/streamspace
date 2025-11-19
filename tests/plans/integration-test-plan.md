# Test Plan: Integration (Batch Operations & Workflows)

**Test Plan ID**: TP-004
**Author**: Agent 3 (Validator)
**Created**: 2025-11-19
**Status**: Active
**Priority**: Medium

---

## Objective

Validate integration between components including batch operations, error handling, and cross-component workflows.

---

## Scope

### In Scope
- Batch session operations
- Error collection and reporting
- API to Controller communication
- Database to Kubernetes synchronization
- Webhook event delivery

### Out of Scope
- Individual component unit tests
- UI integration (separate test)

---

## Test Cases

### TC-INT-001: Batch Session Hibernate

**Priority**: Medium
**Type**: Integration
**Related Issue**: Batch Operations Error Collection

**Preconditions**:
- Multiple sessions running

**Steps**:
1. Create 5 sessions
2. POST /api/v1/sessions/batch/hibernate with all session names
3. Verify all sessions hibernated
4. Check response for success/error counts
5. Verify errors array populated for any failures

**Expected Results**:
```json
{
  "total": 5,
  "succeeded": 4,
  "failed": 1,
  "errors": [
    {
      "name": "session-3",
      "error": "Session already hibernated"
    }
  ]
}
```

**Test File**: `tests/integration/batch_hibernate_test.go`

---

### TC-INT-002: Batch Session Delete

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Multiple sessions exist

**Steps**:
1. Create 5 sessions
2. POST /api/v1/sessions/batch/delete with all session names
3. Verify all sessions deleted
4. Check response for success/error counts
5. Verify errors reported for sessions that couldn't be deleted

**Expected Results**:
- All deletable sessions removed
- Errors clearly reported
- No orphaned resources

**Test File**: `tests/integration/batch_delete_test.go`

---

### TC-INT-003: Batch Session Wake

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Multiple hibernated sessions

**Steps**:
1. Create and hibernate 5 sessions
2. POST /api/v1/sessions/batch/wake with all session names
3. Verify all sessions waking
4. Wait for all to reach Running
5. Verify errors collected for any failures

**Expected Results**:
- All sessions wake successfully
- Error array includes any failures
- Sessions reach Running state

**Test File**: `tests/integration/batch_wake_test.go`

---

### TC-INT-004: Batch Partial Failure

**Priority**: High
**Type**: Integration

**Preconditions**:
- Mix of valid and invalid sessions

**Steps**:
1. Create batch request with:
   - 2 valid session names
   - 1 nonexistent session name
   - 1 already-deleted session
2. Execute batch operation
3. Verify successful operations completed
4. Verify failures collected in errors array
5. Verify clear error messages

**Expected Results**:
- Partial success (200 OK, not 4xx)
- succeeded + failed = total
- Each error has name and message
- Transaction handling correct

**Test File**: `tests/integration/batch_partial_failure_test.go`

---

### TC-INT-005: Webhook Event Delivery

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Webhook configured with test endpoint
- Webhook endpoint logging enabled

**Steps**:
1. Create webhook for "session.created" event
2. Create a session
3. Verify webhook received event
4. Verify payload contains correct data
5. Verify retry on failure
6. Verify webhook signature valid

**Expected Results**:
- Webhook delivered within 5 seconds
- Payload includes session details
- Signature can be verified
- Retries on 5xx errors

**Test File**: `tests/integration/webhook_delivery_test.go`

---

### TC-INT-006: API to Controller Sync

**Priority**: High
**Type**: Integration

**Preconditions**:
- API and controller running

**Steps**:
1. Create session via API
2. Verify CRD created in Kubernetes
3. Update session via API
4. Verify CRD updated
5. Controller updates status
6. Verify API reflects status
7. Delete session via API
8. Verify CRD deleted

**Expected Results**:
- API creates CRDs correctly
- Controller reconciles immediately
- Status updates flow back to API
- Delete cascades properly

**Test File**: `tests/integration/api_controller_sync_test.go`

---

### TC-INT-007: Database-Kubernetes Consistency

**Priority**: High
**Type**: Integration

**Preconditions**:
- Database and cluster accessible

**Steps**:
1. Create session via API
2. Verify database record exists
3. Verify Kubernetes CRD exists
4. Manually delete CRD
5. Verify API detects missing CRD
6. Create CRD manually
7. Verify API syncs state

**Expected Results**:
- Database and K8s stay in sync
- Inconsistencies detected and reported
- System recovers from manual changes

**Test File**: `tests/integration/db_k8s_consistency_test.go`

---

### TC-INT-008: Concurrent Session Operations

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- API server running

**Steps**:
1. Concurrently create 10 sessions
2. Verify all created successfully
3. Concurrently delete all 10
4. Verify all deleted
5. Check for race conditions or deadlocks

**Expected Results**:
- All operations complete
- No race conditions
- No deadlocks
- Performance acceptable

**Test File**: `tests/integration/concurrent_operations_test.go`

---

### TC-INT-009: Event Audit Logging

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Audit logging enabled

**Steps**:
1. Perform session CRUD operations
2. Query audit log
3. Verify all operations logged
4. Verify log includes user, timestamp, action
5. Verify sensitive data not logged

**Expected Results**:
- All operations logged
- Logs queryable and filterable
- No passwords/tokens in logs
- Timestamps accurate

**Test File**: `tests/integration/audit_logging_test.go`

---

### TC-INT-010: Error Propagation

**Priority**: High
**Type**: Integration

**Preconditions**:
- Simulated failure conditions

**Steps**:
1. Create session with invalid resources
2. Verify API returns clear error
3. Create session when database down
4. Verify appropriate error handling
5. Create session when cluster unreachable
6. Verify graceful degradation

**Expected Results**:
- Errors propagate with context
- No stack traces in responses
- Appropriate HTTP status codes
- Errors logged for debugging

**Test File**: `tests/integration/error_propagation_test.go`

---

## Success Criteria

### Must Pass
- TC-INT-001: Batch Session Hibernate
- TC-INT-004: Batch Partial Failure
- TC-INT-006: API to Controller Sync

### Should Pass
- TC-INT-002: Batch Session Delete
- TC-INT-003: Batch Session Wake
- TC-INT-005: Webhook Event Delivery
- TC-INT-007: Database-Kubernetes Consistency

### Nice to Have
- TC-INT-008: Concurrent Session Operations
- TC-INT-009: Event Audit Logging
- TC-INT-010: Error Propagation

---

## Dependencies

- Builder completes Batch Operations Error Collection fix
- Core platform fixes completed first

---

## Schedule

| Phase | Timeline | Status |
|-------|----------|--------|
| Test plan creation | Week 1 | Complete |
| Test implementation | Week 3 | Pending (after core fixes) |
| Test execution | Week 4 | Pending |
| Bug reporting | Week 4-5 | Pending |

---

## Reporting

Results will be reported in:
- `tests/reports/integration-test-report.md`
- Updates to `MULTI_AGENT_PLAN.md` Agent Communication Log
