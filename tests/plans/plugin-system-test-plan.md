# Test Plan: Plugin System

**Test Plan ID**: TP-001
**Author**: Agent 3 (Validator)
**Created**: 2025-11-19
**Status**: Active
**Priority**: Critical

---

## Objective

Validate that the StreamSpace plugin system correctly handles the complete plugin lifecycle: installation, loading, enabling, disabling, and configuration management.

---

## Scope

### In Scope
- Plugin installation from marketplace
- Plugin runtime loading from disk
- Plugin enable/disable functionality
- Plugin configuration updates
- Plugin uninstallation
- Error handling and recovery
- Plugin dependencies

### Out of Scope
- Individual plugin functionality (tested separately)
- Plugin development workflow
- UI components (separate test plan)

---

## Test Environment

### Prerequisites
- StreamSpace API running
- PostgreSQL database with schema
- Test plugins available in fixtures
- Access to plugin marketplace (or mock)

### Test Data
- Test plugin: `streamspace-test-plugin`
- Plugin with dependencies: `streamspace-dependent-plugin`
- Malformed plugin: `streamspace-bad-plugin`

---

## Test Cases

### TC-001: Plugin Installation from Marketplace

**Priority**: Critical
**Type**: Integration
**Related Issue**: Installation Status Never Updates

**Preconditions**:
- API server running
- Plugin not installed

**Steps**:
1. GET /api/v1/plugins/marketplace to list available plugins
2. POST /api/v1/plugins/install with plugin ID
3. Verify installation status transitions: pending -> downloading -> installing -> installed
4. GET /api/v1/plugins/{id} to verify plugin metadata
5. Verify plugin files exist on disk

**Expected Results**:
- Installation completes within 60 seconds
- Status updates visible via API
- Plugin metadata correctly stored in database
- Plugin files extracted to correct directory

**Test File**: `tests/integration/plugin_install_test.go`

---

### TC-002: Plugin Runtime Loading

**Priority**: Critical
**Type**: Integration
**Related Issue**: Plugin Runtime Loading returns "not yet implemented"

**Preconditions**:
- Plugin installed on disk
- Plugin not yet loaded

**Steps**:
1. Call LoadHandler() for installed plugin
2. Verify plugin initializes without errors
3. Verify plugin registers its handlers/hooks
4. Make request to plugin-provided endpoint
5. Verify plugin responds correctly

**Expected Results**:
- LoadHandler() returns nil error
- Plugin initialization hooks execute
- Plugin endpoints respond to requests
- Plugin appears in loaded plugins list

**Test File**: `tests/integration/plugin_runtime_test.go`

---

### TC-003: Plugin Enable Functionality

**Priority**: Critical
**Type**: Integration
**Related Issue**: Plugin Enable Runtime Loading

**Preconditions**:
- Plugin installed but disabled

**Steps**:
1. POST /api/v1/plugins/{id}/enable
2. Verify database updated (enabled = true)
3. Verify plugin loaded into runtime
4. Verify plugin endpoints accessible
5. Verify plugin hooks active

**Expected Results**:
- Enable returns 200 OK
- Database shows enabled = true
- Plugin loaded and functional
- All plugin features available

**Test File**: `tests/integration/plugin_enable_test.go`

---

### TC-004: Plugin Disable Functionality

**Priority**: High
**Type**: Integration

**Preconditions**:
- Plugin installed and enabled

**Steps**:
1. POST /api/v1/plugins/{id}/disable
2. Verify database updated (enabled = false)
3. Verify plugin unloaded from runtime
4. Verify plugin endpoints return 404 or 503
5. Verify plugin hooks deactivated

**Expected Results**:
- Disable returns 200 OK
- Database shows enabled = false
- Plugin endpoints not accessible
- No resource leaks from unloading

**Test File**: `tests/integration/plugin_disable_test.go`

---

### TC-005: Plugin Configuration Update

**Priority**: Critical
**Type**: Integration
**Related Issue**: Plugin Config Update returns success without persisting

**Preconditions**:
- Plugin installed and enabled
- Plugin has configurable settings

**Steps**:
1. GET /api/v1/plugins/{id}/config to get current config
2. PUT /api/v1/plugins/{id}/config with updated values
3. Verify database updated with new config
4. Verify plugin reloaded with new config
5. GET /api/v1/plugins/{id}/config to verify persistence
6. Restart API and verify config persisted

**Expected Results**:
- Config update returns 200 OK
- Database contains new configuration
- Plugin operates with new settings
- Config survives API restart

**Test File**: `tests/integration/plugin_config_test.go`

---

### TC-006: Plugin Uninstallation

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Plugin installed (enabled or disabled)

**Steps**:
1. POST /api/v1/plugins/{id}/uninstall
2. Verify plugin disabled and unloaded
3. Verify database records removed
4. Verify plugin files deleted from disk
5. Verify plugin not in marketplace installed list

**Expected Results**:
- Uninstall returns 200 OK
- Plugin completely removed
- No orphaned files or database records
- Can reinstall same plugin

**Test File**: `tests/integration/plugin_uninstall_test.go`

---

### TC-007: Plugin with Dependencies

**Priority**: Medium
**Type**: Integration

**Preconditions**:
- Plugin A depends on Plugin B
- Plugin B not installed

**Steps**:
1. Attempt to install Plugin A
2. Verify error indicates missing dependency
3. Install Plugin B
4. Install Plugin A
5. Attempt to uninstall Plugin B
6. Verify error indicates dependency

**Expected Results**:
- Clear error messages for missing dependencies
- Dependency resolution works correctly
- Cannot uninstall plugins with dependents

**Test File**: `tests/integration/plugin_dependencies_test.go`

---

### TC-008: Malformed Plugin Handling

**Priority**: High
**Type**: Integration

**Preconditions**:
- Malformed plugin package available

**Steps**:
1. Attempt to install malformed plugin
2. Verify installation fails gracefully
3. Verify error message is descriptive
4. Verify no partial installation
5. Verify system remains stable

**Expected Results**:
- Installation fails with clear error
- No crashes or panics
- Database not corrupted
- Can install valid plugins after

**Test File**: `tests/integration/plugin_error_handling_test.go`

---

### TC-009: Plugin Lifecycle Transitions

**Priority**: High
**Type**: Integration

**Preconditions**:
- Plugin available in marketplace

**Steps**:
1. Install plugin (verify: installed, disabled)
2. Enable plugin (verify: installed, enabled)
3. Update config (verify: still enabled)
4. Disable plugin (verify: installed, disabled)
5. Enable again (verify: installed, enabled)
6. Uninstall (verify: not installed)

**Expected Results**:
- All transitions complete successfully
- State consistent throughout
- No memory leaks
- Can repeat cycle

**Test File**: `tests/integration/plugin_lifecycle_test.go`

---

### TC-010: Concurrent Plugin Operations

**Priority**: Medium
**Type**: Performance

**Preconditions**:
- Multiple plugins available

**Steps**:
1. Concurrently install 5 plugins
2. Wait for all installations
3. Concurrently enable all plugins
4. Concurrently update all configs
5. Verify all plugins functional

**Expected Results**:
- No race conditions
- All operations complete
- No database deadlocks
- Performance within acceptable limits

**Test File**: `tests/performance/plugin_concurrent_test.go`

---

## Test Data Requirements

### Test Plugins

```yaml
# tests/fixtures/plugins/test-plugin/plugin.yaml
name: streamspace-test-plugin
version: 1.0.0
description: Test plugin for validation
author: StreamSpace Testing
endpoints:
  - path: /api/v1/plugins/test/echo
    method: POST
    handler: EchoHandler
config:
  - key: enabled
    type: boolean
    default: true
  - key: message
    type: string
    default: "Hello"
```

### Database Fixtures

```sql
-- Clean plugin test data
DELETE FROM plugins WHERE name LIKE 'streamspace-test-%';
DELETE FROM plugin_configs WHERE plugin_id IN
  (SELECT id FROM plugins WHERE name LIKE 'streamspace-test-%');
```

---

## Success Criteria

### Must Pass
- TC-001: Plugin Installation (Critical)
- TC-002: Plugin Runtime Loading (Critical)
- TC-003: Plugin Enable (Critical)
- TC-005: Plugin Config Update (Critical)

### Should Pass
- TC-004: Plugin Disable
- TC-006: Plugin Uninstallation
- TC-008: Malformed Plugin Handling
- TC-009: Plugin Lifecycle

### Nice to Have
- TC-007: Plugin Dependencies
- TC-010: Concurrent Operations

---

## Risks

1. **Runtime Loading Complexity**: LoadHandler() implementation may require significant changes
2. **Config Persistence**: Multiple storage layers (DB, file, memory) need synchronization
3. **Resource Leaks**: Plugin disable/uninstall must properly clean up resources

---

## Dependencies

- Builder completes Plugin Runtime Loading fix
- Builder completes Plugin Config Update fix
- Builder completes Plugin Enable Runtime Loading fix

---

## Schedule

| Phase | Timeline | Status |
|-------|----------|--------|
| Test plan creation | Week 1 | Complete |
| Test implementation | Week 2-3 | Pending (after Builder fixes) |
| Test execution | Week 4 | Pending |
| Bug reporting | Week 4-5 | Pending |
| Regression testing | Week 6 | Pending |

---

## Reporting

Results will be reported in:
- `tests/reports/plugin-system-test-report.md`
- Updates to `MULTI_AGENT_PLAN.md` Agent Communication Log

Bug reports will follow template in agent instructions with:
- Severity level
- Reproduction steps
- Expected vs actual behavior
- Suggested fix location
