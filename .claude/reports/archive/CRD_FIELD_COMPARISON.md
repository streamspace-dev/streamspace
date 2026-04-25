# Template CRD: Current vs. Target VNC Field Structure

## Side-by-Side Comparison

### CRD YAML Schema

#### Current State (LEGACY - kasmvnc)
```yaml
# manifests/crds/template.yaml
kasmvnc:
  type: object
  properties:
    enabled:
      type: boolean
      default: true
    port:
      type: integer
      default: 3000
```

#### Target State (MODERN - vnc)
```yaml
# manifests/crds/template.yaml
vnc:
  type: object
  properties:
    enabled:
      type: boolean
      default: true
    port:
      type: integer
      default: 5900
    protocol:
      type: string
      default: rfb
      enum: [rfb, websocket]
    encryption:
      type: boolean
      default: false
```

---

## Go Type Definitions

### Current State (ALREADY MIGRATED!)
```go
// k8s-controller/api/v1alpha1/template_types.go

type TemplateSpec struct {
    // ... other fields ...
    
    // VNC configures the VNC streaming settings for this template.
    //
    // IMPORTANT: This is VNC-agnostic and designed for migration.
    // Currently supports:
    //   - LinuxServer.io images with KasmVNC (temporary)
    //
    // Future target:
    //   - StreamSpace images with TigerVNC + noVNC (100% open source)
    VNC VNCConfig `json:"vnc,omitempty"`
}

type VNCConfig struct {
    // Enabled determines whether VNC streaming is available
    // Default: true
    Enabled bool `json:"enabled"`

    // Port specifies the VNC server port inside the container
    // Default: 5900
    Port int `json:"port,omitempty"`

    // Protocol specifies the VNC protocol variant
    // Valid: "rfb" (default) or "websocket"
    Protocol string `json:"protocol,omitempty"`

    // Encryption enables TLS encryption for VNC connections
    // Default: false
    Encryption bool `json:"encryption,omitempty"`
}
```

**Status**: READY - Go types are already VNC-agnostic!

---

## Template Manifest Examples

### Firefox Browser

#### Current (LEGACY - kasmvnc)
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: workspaces
spec:
  displayName: Firefox Web Browser
  description: Modern, privacy-focused web browser
  category: Web Browsers
  baseImage: lscr.io/linuxserver/firefox:latest
  
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  
  ports:
    - name: vnc
      containerPort: 3000
      protocol: TCP
  
  env:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
    - name: TZ
      value: "America/New_York"
  
  volumeMounts:
    - name: user-home
      mountPath: /config
  
  # LEGACY FIELD (PROPRIETARY)
  kasmvnc:
    enabled: true
    port: 3000
  
  capabilities:
    - Network
    - Audio
    - Clipboard
  
  tags:
    - browser
    - web
    - privacy
```

#### Target (MODERN - vnc)
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: workspaces
spec:
  displayName: Firefox Web Browser
  description: Modern, privacy-focused web browser
  category: Web Browsers
  baseImage: lscr.io/linuxserver/firefox:latest
  
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  
  ports:
    - name: vnc
      containerPort: 3000          # Keep 3000 for LinuxServer.io (for now)
      protocol: TCP
  
  env:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
    - name: TZ
      value: "America/New_York"
  
  volumeMounts:
    - name: user-home
      mountPath: /config
  
  # MODERN FIELD (GENERIC VNC)
  vnc:
    enabled: true
    port: 3000                     # 3000 for LinuxServer.io
    protocol: websocket            # WebSocket for browser
    encryption: false              # TLS at ingress level
  
  capabilities:
    - Network
    - Audio
    - Clipboard
  
  tags:
    - browser
    - web
    - privacy
```

**Changes**:
- `kasmvnc:` → `vnc:` (field name)
- Added `protocol: websocket`
- Added `encryption: false`

---

### Code Server (HTTP-based, no VNC)

#### Current (LEGACY)
```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: code-server
spec:
  displayName: VS Code Server
  baseImage: lscr.io/linuxserver/code-server:latest
  
  ports:
    - name: http
      containerPort: 8443
      protocol: TCP
  
  # LEGACY: VNC disabled
  kasmvnc:
    enabled: false
    port: null
  
  tags:
    - code-server
    - development
```

#### Target (MODERN)
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: code-server
spec:
  displayName: VS Code Server
  baseImage: lscr.io/linuxserver/code-server:latest
  
  ports:
    - name: http
      containerPort: 8443
      protocol: TCP
  
  # MODERN: VNC disabled
  vnc:
    enabled: false
    port: null
    protocol: null
    encryption: null
  
  tags:
    - code-server
    - development
```

**Changes**:
- `kasmvnc:` → `vnc:`
- Added `protocol: null`
- Added `encryption: null`

---

## Database Schema

### Current (LEGACY)
```sql
CREATE TABLE templates (
  -- ... other columns ...
  kasmvnc_enabled BOOLEAN DEFAULT true,
  kasmvnc_port INTEGER DEFAULT 3000,
  -- ... other columns ...
);
```

### Target (MODERN)
```sql
CREATE TABLE templates (
  -- ... other columns ...
  vnc_enabled BOOLEAN DEFAULT true,
  vnc_port INTEGER DEFAULT 5900,
  vnc_protocol VARCHAR(50) DEFAULT 'rfb',
  vnc_encryption BOOLEAN DEFAULT false,
  -- ... other columns ...
);
```

**Changes**:
- `kasmvnc_enabled` → `vnc_enabled`
- `kasmvnc_port` → `vnc_port` (default: 5900 instead of 3000)
- Added `vnc_protocol` column
- Added `vnc_encryption` column

**Migration Note**: Requires database migration script to rename columns and preserve existing data.

---

## Migration Path

### Step 1: CRD Schema Update
```diff
- kasmvnc:
-   type: object
-   properties:
-     enabled:
-       type: boolean
-       default: true
-     port:
-       type: integer
-       default: 3000

+ vnc:
+   type: object
+   properties:
+     enabled:
+       type: boolean
+       default: true
+     port:
+       type: integer
+       default: 5900
+     protocol:
+       type: string
+       default: rfb
+       enum: [rfb, websocket]
+     encryption:
+       type: boolean
+       default: false
```

### Step 2: Template Manifest Updates
```diff
- kasmvnc:
-   enabled: true
-   port: 3000

+ vnc:
+   enabled: true
+   port: 3000
+   protocol: websocket
+   encryption: false
```

### Step 3: Database Schema Migration
```sql
-- Rename columns
ALTER TABLE templates 
  RENAME COLUMN kasmvnc_enabled TO vnc_enabled,
  RENAME COLUMN kasmvnc_port TO vnc_port;

-- Add new columns
ALTER TABLE templates
  ADD COLUMN vnc_protocol VARCHAR(50) DEFAULT 'rfb',
  ADD COLUMN vnc_encryption BOOLEAN DEFAULT false;

-- Update port defaults for future
UPDATE templates SET vnc_port = 5900 WHERE vnc_port = 3000;
```

### Step 4: API Handler Updates
- Update template parser to read `vnc` field instead of `kasmvnc`
- Add backward compatibility layer if needed (read both fields)
- Update WebSocket proxy to use new config fields

---

## Validation Rules

### Current Validation (kasmvnc)
- `kasmvnc.enabled`: boolean (required)
- `kasmvnc.port`: integer, 1-65535 (optional, default 3000)

### Target Validation (vnc)
- `vnc.enabled`: boolean (required)
- `vnc.port`: integer, 1-65535 (optional, default 5900)
- `vnc.protocol`: string, enum [rfb, websocket] (optional, default rfb)
- `vnc.encryption`: boolean (optional, default false)

### Validation Logic
```go
// Validate VNC configuration
if spec.VNC.Enabled {
    if spec.VNC.Port < 1 || spec.VNC.Port > 65535 {
        return fmt.Errorf("invalid VNC port: %d", spec.VNC.Port)
    }
    
    if spec.VNC.Protocol != "" && 
       spec.VNC.Protocol != "rfb" && 
       spec.VNC.Protocol != "websocket" {
        return fmt.Errorf("invalid VNC protocol: %s", spec.VNC.Protocol)
    }
}
```

---

## Backward Compatibility Strategy

### Option 1: Dual-Field Support (Recommended)
Support both `kasmvnc` and `vnc` fields during a deprecation period:

```go
// During migration period, accept both
type TemplateSpec struct {
    // Modern field
    VNC VNCConfig `json:"vnc,omitempty"`
    
    // Legacy field (deprecated, will be removed in v2.0)
    KasmVNC VNCConfig `json:"kasmvnc,omitempty"`
}

// Conversion logic in API layer
func (spec *TemplateSpec) GetVNCConfig() VNCConfig {
    if spec.VNC.Enabled || spec.VNC.Port > 0 {
        return spec.VNC
    }
    if spec.KasmVNC.Enabled || spec.KasmVNC.Port > 0 {
        // Legacy: use kasmvnc if present
        return spec.KasmVNC
    }
    // Default
    return VNCConfig{Enabled: true, Port: 5900}
}
```

### Option 2: Gradual Migration Timeline
1. **v1.1**: Support both `vnc` and `kasmvnc` (dual-field)
2. **v1.2-v1.5**: Warn on use of `kasmvnc` (deprecation period)
3. **v2.0**: Remove `kasmvnc` support entirely

### Option 3: Automatic Conversion
Use Kubernetes conversion webhook to automatically convert old manifests:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: templates.stream.space
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: template-conversion-webhook
          port: 443
      conversionReviewVersions: [v1]
```

---

## Impact Summary

| Aspect | Current | Target | Impact |
|--------|---------|--------|--------|
| **Field Name** | `kasmvnc` | `vnc` | User-facing (template YAML) |
| **Field Structure** | Minimal (2 fields) | Extended (4 fields) | Backward compatible |
| **Default Port** | 3000 | 5900 | Breaking change for future |
| **Protocol Support** | Implicit WebSocket | Explicit (rfb\|websocket) | Feature addition |
| **Encryption Support** | None | Optional TLS | Feature addition |
| **Database Columns** | 2 (`kasmvnc_*`) | 4 (`vnc_*`) | Schema migration required |
| **API Code** | References `kasmvnc` | Uses `vnc` | Code update required |
| **Documentation** | References Kasm | References generic VNC | Doc update required |

---

## Files Requiring Updates

| File | Type | Change | Priority |
|------|------|--------|----------|
| `manifests/crds/template.yaml` | CRD | Rename field, add properties | Critical |
| `manifests/crds/workspacetemplate.yaml` | CRD (legacy) | Rename field | High |
| `manifests/templates/browsers/firefox.yaml` | Template | Update field name | Critical |
| `manifests/templates-generated/**/*.yaml` | Templates (35) | Update field name | Critical |
| `manifests/config/database-init.yaml` | Schema | Rename columns | Critical |
| `k8s-controller/api/v1alpha1/template_types.go` | Code | Already done! | N/A |
| `api/internal/sync/parser.go` | Code | Update field reading | High |
| `api/internal/handlers/` | Code | Update field access | High |
| `docs/*.md` | Docs | Update examples | Medium |
| `scripts/generate-templates.py` | Script | Update generation | High |
| `scripts/migrate-templates.sh` | Script | Update references | Medium |

