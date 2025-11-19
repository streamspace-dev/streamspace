# Stub Endpoints Reference

Quick reference for all intentional stub endpoints in StreamSpace core API.

## Location

File: `/home/user/streamspace/api/internal/api/stubs.go` (lines 1016-1098)

---

## Compliance Endpoints

All compliance endpoints are stubs that return empty/error responses until the `streamspace-compliance` plugin is installed.

### ListComplianceFrameworks()

**Endpoint**: `GET /api/v1/compliance/frameworks`

**Status Code**: 200 OK

**Response** (without plugin):
```json
{
  "frameworks": []
}
```

**Why**: Plugin provides real implementation with framework management

**What to do**: Install `streamspace-compliance` plugin

---

### CreateComplianceFramework()

**Endpoint**: `POST /api/v1/compliance/frameworks`

**Status Code**: 501 Not Implemented (without plugin)

**Response** (without plugin):
```json
{
  "error": "Compliance features require the streamspace-compliance plugin",
  "message": "Please install the streamspace-compliance plugin from Admin → Plugins"
}
```

**Why**: Plugin provides real implementation

**What to do**: Install `streamspace-compliance` plugin

---

### ListCompliancePolicies()

**Endpoint**: `GET /api/v1/compliance/policies`

**Status Code**: 200 OK

**Response** (without plugin):
```json
{
  "policies": []
}
```

**Why**: Plugin provides real implementation with policy management

**What to do**: Install `streamspace-compliance` plugin

---

### CreateCompliancePolicy()

**Endpoint**: `POST /api/v1/compliance/policies`

**Status Code**: 501 Not Implemented (without plugin)

**Response** (without plugin):
```json
{
  "error": "Compliance features require the streamspace-compliance plugin",
  "message": "Please install the streamspace-compliance plugin from Admin → Plugins"
}
```

**Why**: Plugin provides real implementation

**What to do**: Install `streamspace-compliance` plugin

---

### ListViolations()

**Endpoint**: `GET /api/v1/compliance/violations`

**Status Code**: 200 OK

**Response** (without plugin):
```json
{
  "violations": []
}
```

**Why**: Plugin provides violation tracking and reporting

**What to do**: Install `streamspace-compliance` plugin

---

### RecordViolation()

**Endpoint**: `POST /api/v1/compliance/violations`

**Status Code**: 501 Not Implemented (without plugin)

**Response** (without plugin):
```json
{
  "error": "Compliance features require the streamspace-compliance plugin",
  "message": "Please install the streamspace-compliance plugin from Admin → Plugins"
}
```

**Why**: Plugin provides violation recording and tracking

**What to do**: Install `streamspace-compliance` plugin

---

### ResolveViolation()

**Endpoint**: `PATCH /api/v1/compliance/violations/{id}/resolve`

**Status Code**: 501 Not Implemented (without plugin)

**Response** (without plugin):
```json
{
  "error": "Compliance features require the streamspace-compliance plugin",
  "message": "Please install the streamspace-compliance plugin from Admin → Plugins"
}
```

**Why**: Plugin provides violation resolution workflow

**What to do**: Install `streamspace-compliance` plugin

---

### GetComplianceDashboard()

**Endpoint**: `GET /api/v1/compliance/dashboard`

**Status Code**: 200 OK

**Response** (without plugin):
```json
{
  "total_policies": 0,
  "active_policies": 0,
  "total_open_violations": 0,
  "violations_by_severity": {
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0
  }
}
```

**Why**: Plugin provides compliance dashboard with real metrics

**What to do**: Install `streamspace-compliance` plugin

---

## Other Stubs (Backwards Compatibility)

### ListNodes()

**Location**: `stubs.go`, lines 220-230

**Note**: This is a backwards compatibility stub. Real implementation is in `handlers/nodes.go` via `NodeHandler`

**Status**: Routes should use the new handler, but this stub remains for API compatibility

---

## Important Notes

### Design Principle

These stubs follow a **graceful degradation** pattern:

1. **Without Plugin**: Return helpful error or empty data
2. **With Plugin**: Plugin registers real handlers that override these stubs
3. **User Experience**: Users get clear messages directing them to install plugins

### HTTP Status Codes

| Status | When | Meaning |
|--------|------|---------|
| 200 OK | List operations | Feature not available, returning empty array |
| 501 Not Implemented | Write operations | Install plugin to enable |

This distinction allows:
- **List operations**: Graceful fallback to empty results
- **Write operations**: Clear signal that feature requires plugin

### Testing Stubs

When testing without plugins:

```bash
# These should return empty results (200 OK)
curl http://localhost:3000/api/v1/compliance/frameworks
curl http://localhost:3000/api/v1/compliance/policies
curl http://localhost:3000/api/v1/compliance/violations
curl http://localhost:3000/api/v1/compliance/dashboard

# These should return 501 Not Implemented
curl -X POST http://localhost:3000/api/v1/compliance/frameworks
curl -X POST http://localhost:3000/api/v1/compliance/policies
curl -X POST http://localhost:3000/api/v1/compliance/violations
curl -X PATCH http://localhost:3000/api/v1/compliance/violations/{id}/resolve
```

### Installing Compliance Plugin

Once you install the `streamspace-compliance` plugin:

1. Plugin registers real endpoint handlers
2. These override the stubs
3. All compliance features become available
4. Plugin creates 6 database tables for compliance data
5. Plugin adds 5 UI pages for compliance management

---

## Related Documentation

- [Plugin Architecture Reference](./PLUGIN_ARCHITECTURE_REFERENCE.md)
- [Plugin Features Checklist](./PLUGIN_FEATURES_CHECKLIST.md)
- [stubs.go Source](../api/internal/api/stubs.go)

