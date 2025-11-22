# Bug Report: P0-MANIFEST-001 - Template Manifest Case Sensitivity Mismatch

**Priority**: P0 (Critical - Blocks Session Provisioning)
**Status**: 🔴 ACTIVE - Blocking E2E VNC Streaming Validation
**Component**: Agent Template Parsing / Database Template Storage
**Discovered**: 2025-11-22 04:30:00 UTC
**Reporter**: Validator Agent
**Impact**: **CRITICAL** - No sessions can be provisioned

---

## Executive Summary

Builder's P0-RBAC-001 fixes were successfully deployed, but session provisioning still fails. The agent receives the template manifest from the API but cannot parse it due to a **case sensitivity mismatch** between the database manifest schema (capitalized fields: `"Spec"`, `"Ports"`) and the agent parsing code (expects lowercase: `"spec"`, `"ports"`).

**Impact**: 🔴 **BLOCKS** all session creation and E2E VNC streaming validation.

---

## Error Details

### Agent Log Error

```
2025/11/22 04:28:57 [StartSessionHandler] Warning: No templateManifest in payload, falling back to K8s fetch: failed to parse template manifest: invalid template spec
2025/11/22 04:28:57 [K8sOps] Fetched template from K8s: firefox-browser (image: lscr.io/linuxserver/firefox:latest, ports: 0)
2025/11/22 04:28:57 [K8sAgent] Command cmd-08acbb47 failed: failed to create deployment: Deployment.apps "admin-firefox-browser-bc0bee20" is invalid: spec.template.spec.containers[0].ports[0].containerPort: Required value
```

### Full Error Breakdown

**Stage 1**: Agent receives WebSocket command with template manifest
**Stage 2**: Agent tries to parse manifest, fails with "invalid template spec"
**Stage 3**: Agent falls back to fetching Template CRD from Kubernetes (RBAC fix working ✅)
**Stage 4**: Template CRD has schema mismatch (`vnc.port: 3000` instead of `ports[].containerPort`)
**Stage 5**: Agent sees "ports: 0" when parsing Template CRD
**Stage 6**: Deployment creation fails due to missing containerPort

---

## Root Cause Analysis

### Database Manifest Schema (Capitalized)

**Query**:
```sql
SELECT name, manifest FROM catalog_templates WHERE name = 'firefox-browser';
```

**Result**:
```json
{
  "Kind": "Template",
  "Spec": {
    "Ports": [
      {
        "Name": "vnc",
        "Protocol": "TCP",
        "ContainerPort": 3000
      }
    ],
    "BaseImage": "lscr.io/linuxserver/firefox:latest",
    "Description": "Modern, privacy-focused web browser...",
    "DefaultResources": {
      "cpu": "1000m",
      "memory": "2Gi"
    }
  },
  "Metadata": {
    "Name": "firefox-browser",
    "Namespace": "workspaces"
  },
  "APIVersion": "stream.space/v1alpha1"
}
```

**Key Observation**: Field names are **capitalized** (`"Spec"`, `"Ports"`, `"BaseImage"`, etc.)

---

### Agent Parsing Code (Expects Lowercase)

**File**: `agents/k8s-agent/agent_k8s_operations.go:139-141`

```go
func parseTemplateCRD(obj *unstructured.Unstructured) (*Template, error) {
    // ...

    spec, ok := obj.Object["spec"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid template spec")  // ← FAILS HERE
    }

    // Parse baseImage
    if baseImage, ok := spec["baseImage"].(string); ok {
        template.BaseImage = baseImage
    } else {
        return nil, fmt.Errorf("template missing baseImage")
    }

    // Parse ports
    if ports, ok := spec["ports"].([]interface{}); ok {
        // ...
    }
}
```

**Lines 139-141**: Looks for `obj.Object["spec"]` (lowercase)
**Database has**: `obj.Object["Spec"]` (capitalized)
**Result**: `ok == false`, returns error "invalid template spec"

---

### Why Capitalized Fields in Database?

**Hypothesis**: Template repository sync process serializes Go structs to JSON

**Go Struct Convention**:
```go
type TemplateSpec struct {
    BaseImage        string         // ← Exported field (capitalized)
    Ports            []PortConfig   // ← Exported field (capitalized)
    DefaultResources ResourceConfig // ← Exported field (capitalized)
}
```

**JSON Marshaling**:
```go
manifestJSON, _ := json.Marshal(templateSpec)
// Results in: {"BaseImage": "...", "Ports": [...], ...}
```

**Issue**: Go's default JSON marshaling uses the field name as-is (capitalized), unless struct tags specify otherwise:

```go
type TemplateSpec struct {
    BaseImage string `json:"baseImage"` // ← Missing json tags
    Ports     []Port `json:"ports"`     // ← Missing json tags
}
```

**Location**: Likely in `api/internal/sync/parser.go` (TemplateManifest struct)

---

## Impact Assessment

### Severity: P0 (Critical)

**Justification**:
- ❌ **ALL session provisioning blocked** (P0-RBAC-001 fixes ineffective due to this issue)
- ❌ **E2E VNC streaming validation blocked**
- ❌ **Integration testing cannot proceed**
- ❌ **Core product functionality broken**

### Affected Features

1. **Session Creation** (POST /api/v1/sessions) - 🔴 BROKEN
2. **Session Provisioning** - 🔴 BROKEN
3. **VNC Streaming** - 🔴 BLOCKED
4. **Template-Based Deployments** - 🔴 BROKEN

### Current Workarounds

**None available** - Case mismatch prevents agent from parsing manifest

---

## Related Issues Chain

This is the **third blocker** in the session provisioning flow:

1. ✅ **P0-RBAC-001a** - Agent RBAC permissions → **FIXED** (commit e22969f)
2. ✅ **P0-RBAC-001b** - API includes template manifest → **FIXED** (commit 8d01529) ← **BUT MANIFEST FORMAT WRONG**
3. 🔴 **P0-MANIFEST-001** - Template manifest case mismatch → **THIS ISSUE**

---

## Recommended Fixes

### Primary Fix: Add JSON Struct Tags to Template Structs

**Rationale**:
- Ensures database stores lowercase field names matching Template CRD schema
- Aligns with Kubernetes conventions (all CRD fields are lowercase)
- Prevents future case sensitivity issues
- No agent code changes required

**Implementation**:

**File**: `api/internal/sync/parser.go` (or wherever TemplateManifest is defined)

```go
// BEFORE (missing json tags)
type TemplateSpec struct {
    DisplayName      string
    Description      string
    Category         string
    AppType          string
    BaseImage        string
    Ports            []PortConfig
    DefaultResources ResourceConfig
    Env              []EnvVar
    VolumeMounts     []VolumeMount
    VNC              *VNCConfig
}

// AFTER (with json tags for lowercase serialization)
type TemplateSpec struct {
    DisplayName      string          `json:"displayName"`
    Description      string          `json:"description"`
    Category         string          `json:"category"`
    AppType          string          `json:"appType"`
    BaseImage        string          `json:"baseImage"`
    Ports            []PortConfig    `json:"ports"`
    DefaultResources ResourceConfig  `json:"defaultResources"`
    Env              []EnvVar        `json:"env,omitempty"`
    VolumeMounts     []VolumeMount   `json:"volumeMounts,omitempty"`
    VNC              *VNCConfig      `json:"vnc,omitempty"`
}

type PortConfig struct {
    Name          string `json:"name"`
    ContainerPort int32  `json:"containerPort"`
    Protocol      string `json:"protocol"`
}

type ResourceConfig struct {
    Memory string `json:"memory"`
    CPU    string `json:"cpu"`
}
```

**Scope**: Add `json:` tags to:
- `TemplateManifest` struct
- `TemplateSpec` struct
- `PortConfig` struct
- `ResourceConfig` struct
- `EnvVar` struct (if custom)
- `VolumeMount` struct (if custom)
- `VNCConfig` struct
- `TemplateMetadata` struct

**Re-sync Templates**: After deploying fix, re-sync template repositories to populate database with lowercase manifests

---

### Secondary Fix (Temporary): Make Agent Parser Case-Insensitive

**Rationale**:
- Quick fix to unblock testing while proper fix is implemented
- Allows agent to parse both capitalized and lowercase manifests
- Defense in depth

**Implementation**:

**File**: `agents/k8s-agent/agent_k8s_operations.go:139`

```go
func parseTemplateCRD(obj *unstructured.Unstructured) (*Template, error) {
    template := &Template{
        Name:      obj.GetName(),
        Namespace: obj.GetNamespace(),
    }

    // BEFORE:
    // spec, ok := obj.Object["spec"].(map[string]interface{})

    // AFTER (case-insensitive lookup):
    var spec map[string]interface{}
    if s, ok := obj.Object["spec"].(map[string]interface{}); ok {
        spec = s
    } else if s, ok := obj.Object["Spec"].(map[string]interface{}); ok {
        spec = s
    } else {
        return nil, fmt.Errorf("invalid template spec (neither 'spec' nor 'Spec' found)")
    }

    // Parse baseImage (try both cases)
    if baseImage, ok := spec["baseImage"].(string); ok {
        template.BaseImage = baseImage
    } else if baseImage, ok := spec["BaseImage"].(string); ok {
        template.BaseImage = baseImage
    } else {
        return nil, fmt.Errorf("template missing baseImage")
    }

    // Parse ports (try both cases)
    if ports, ok := spec["ports"].([]interface{}); ok {
        // lowercase parsing (existing code)
    } else if ports, ok := spec["Ports"].([]interface{}); ok {
        // Capitalize parsing (parse portMap["ContainerPort"], etc.)
    }

    // ... repeat for all fields ...
}
```

**Drawback**: Verbose, error-prone, not a proper solution

---

### Recommended Approach: **PRIMARY FIX ONLY**

**Rationale**:
1. Adding JSON tags is the **correct** solution
2. Aligns database with Kubernetes conventions
3. Prevents future issues
4. Secondary fix is overly complex and not maintainable

**Priority**:
1. **Immediate**: Add JSON struct tags to all template-related structs
2. **Immediate**: Re-sync template repositories (rebuild database manifests)
3. **Immediate**: Test session creation again

---

## Validation Plan

Once fix is deployed, verify:

### Test 1: Template Manifest in Database (Lowercase)

```sql
SELECT name, manifest::text FROM catalog_templates WHERE name = 'firefox-browser';
```

**Expected**:
```json
{
  "spec": {
    "baseImage": "lscr.io/linuxserver/firefox:latest",
    "ports": [
      {
        "name": "vnc",
        "containerPort": 3000,
        "protocol": "TCP"
      }
    ]
  }
}
```

**Validation**: Field names should be lowercase

---

### Test 2: Session Creation Succeeds

```bash
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}'
```

**Expected**: Session created, state transitions to "running" within 30s

---

### Test 3: Agent Logs - No Parsing Errors

```bash
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep -E "(parse|template|manifest)"
```

**Expected**:
```
[K8sOps] Parsed template from payload: firefox-browser (image: lscr.io/linuxserver/firefox:latest, ports: 1)
[StartSessionHandler] Using template: Firefox Browser (image: lscr.io/linuxserver/firefox:latest)
```

**No errors** about "invalid template spec" or "failed to parse template manifest"

---

### Test 4: Pod Created with Correct Port

```bash
kubectl get deployment -n streamspace -l session=admin-firefox-browser-* -o yaml | grep -A10 "ports:"
```

**Expected**:
```yaml
ports:
  - name: vnc
    containerPort: 3000
    protocol: TCP
```

---

## Technical Context

### JSON Struct Tags in Go

**Purpose**: Control JSON serialization/deserialization

**Syntax**:
```go
type Example struct {
    FieldName string `json:"fieldName"`          // lowercase in JSON
    Optional  string `json:"optional,omitempty"` // omit if empty
    Ignored   string `json:"-"`                  // never serialize
}
```

**Documentation**: https://pkg.go.dev/encoding/json

---

### Template CRD Schema (Kubernetes)

**File**: `agents/k8s-agent/deployments/templates-crd.yaml`

**Schema** (lowercase fields):
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  versions:
    - name: v1alpha1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                baseImage:
                  type: string
                ports:
                  type: array
                  items:
                    properties:
                      name:
                        type: string
                      containerPort:
                        type: integer
                      protocol:
                        type: string
```

**All CRD fields use camelCase** (first letter lowercase)

---

## Dependencies

**Blocks**:
- E2E VNC streaming validation
- Integration testing continuation
- Session provisioning for all users

**Depends On**:
- ✅ P0-RBAC-001a (RBAC permissions) - VALIDATED
- ✅ P0-RBAC-001b (API template manifest inclusion) - VALIDATED (but manifest format wrong)

**Related Issues**:
- P0-RBAC-001 (WebSocket concurrent write) - ✅ FIXED
- P1-DATABASE-001 (TEXT[] arrays) - ✅ FIXED
- P1-SCHEMA-001 (cluster_id) - ✅ FIXED
- P1-SCHEMA-002 (tags column) - ✅ FIXED

---

## Additional Notes

### Why This Wasn't Caught Earlier

1. **P0-RBAC-001 blocked testing** - Agent couldn't receive template manifest until RBAC fix deployed
2. **Multi-layered issue** - Required both RBAC fix AND template manifest inclusion to reach this error
3. **Template repository just synced** - Database may have been recently populated with wrong schema

### Case Sensitivity in Other Languages

**Python**: Case-sensitive by default
**JavaScript**: Case-sensitive
**Go**: Case-sensitive
**Kubernetes YAML**: Case-sensitive (all lowercase by convention)

**Best Practice**: Always use lowercase field names in JSON for Kubernetes resources

---

## Evidence

### Test Execution

**Script**: `/tmp/test_e2e_vnc_streaming.sh`
**Session**: `admin-firefox-browser-bc0bee20`
**Result**: Session stuck in "pending", no pod created

### Agent Logs

```
2025/11/22 04:28:57 [StartSessionHandler] Warning: No templateManifest in payload, falling back to K8s fetch: failed to parse template manifest: invalid template spec
```

**Analysis**: Agent received manifest but parsing failed

### Database Query

```sql
SELECT name, manifest->'Spec'->'Ports' AS ports
FROM catalog_templates
WHERE name = 'firefox-browser';
```

**Result**: Shows capitalized field names

---

## Conclusion

**Summary**: Template manifest stored in database has capitalized field names (`"Spec"`, `"Ports"`, `"BaseImage"`), but agent parsing code expects lowercase (`"spec"`, `"ports"`, `"baseImage"`). This case mismatch causes parsing to fail, blocking session provisioning.

**Immediate Action Required**:
1. Add JSON struct tags to all template-related Go structs
2. Re-sync template repositories to populate database with correct schema
3. Test session creation

**Severity**: P0 - Blocks all session provisioning and E2E testing

**Recommendation**: Deploy primary fix (JSON struct tags) immediately, then re-sync templates.

---

**Generated**: 2025-11-22 04:35:00 UTC
**Validator**: Claude (v2-validator branch)
**Next Step**: Builder to add JSON struct tags to TemplateManifest and related structs
