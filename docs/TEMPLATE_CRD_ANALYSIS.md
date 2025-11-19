# Template CRD Structure Analysis: VNC Configuration in StreamSpace

**Analysis Date**: November 19, 2025  
**Status**: Complete - Shows current state with legacy "kasmvnc" field and modern "VNC" struct

---

## CRITICAL FINDING: CRD/Code Mismatch in Transition

The codebase is currently in a **partially migrated state**:

| Component | Status | Current | Target |
|-----------|--------|---------|--------|
| **Go Type Definitions** | MIGRATED | `VNC` (generic) | VNC-agnostic ✓ |
| **Template CRD YAML** | LEGACY | `kasmvnc` (proprietary) | `vnc` (generic) |
| **Template Manifests** | LEGACY | `kasmvnc` (40+ files) | `vnc` (generic) |
| **Database Schema** | LEGACY | `kasmvnc_*` columns | `vnc_*` columns |
| **API Handlers** | MIGRATED | Generic VNC handling | VNC-agnostic ✓ |

---

## Complete Template CRD Specification

### CRD YAML Definition
**Location**: `/home/user/streamspace/manifests/crds/template.yaml`

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: templates.stream.space
spec:
  group: stream.space
  scope: Namespaced
  names:
    plural: templates
    singular: template
    kind: Template
    shortNames:
      - tpl
```

### Go Type Definitions
**Location**: `/home/user/streamspace/k8s-controller/api/v1alpha1/template_types.go`

#### TemplateSpec Structure (REFACTORED - VNC-Generic)
```go
type TemplateSpec struct {
    // Core Fields (Required)
    DisplayName string                          // e.g., "Firefox Web Browser"
    BaseImage   string                          // e.g., "lscr.io/linuxserver/firefox:latest"

    // Metadata Fields (Optional)
    Description       string                    // Detailed description
    Category          string                    // e.g., "Web Browsers"
    Icon              string                    // URL to icon image
    
    // Resource Configuration
    DefaultResources  corev1.ResourceRequirements // Memory & CPU limits/requests
    
    // Container Configuration
    Ports             []corev1.ContainerPort    // Port definitions
    Env               []corev1.EnvVar           // Environment variables
    VolumeMounts      []corev1.VolumeMount      // Volume mount points
    
    // VNC CONFIGURATION (MIGRATED - GENERIC, NOT KASM-SPECIFIC!)
    VNC               VNCConfig                 // Generic VNC settings
    
    // Feature/Capability Declaration
    Capabilities      []string                  // Network, Audio, Clipboard, USB, Printing
    Tags              []string                  // Search/filter tags
}
```

#### VNCConfig Structure (VNC-AGNOSTIC - NOT Kasm-Specific!)
**CRITICAL**: This is designed for VNC migration, NOT proprietary!

```go
type VNCConfig struct {
    // Enabled determines if VNC streaming is available
    // When true: VNC port exposed, WebSocket proxy created, UI shows "Launch" button
    // When false: Headless/CLI-only application
    // Default: true
    Enabled bool `json:"enabled"`

    // Port specifies the VNC server port inside container
    // Valid values:
    //   - 5900: RFB protocol standard (future TigerVNC)
    //   - 3000: LinuxServer.io convention (current)
    //   - 6080: noVNC HTTP port (alternative)
    // Default: 5900
    Port int `json:"port,omitempty"`

    // Protocol specifies VNC protocol variant
    // Valid values:
    //   - "rfb": Raw RFB protocol (standard VNC)
    //   - "websocket": WebSocket-wrapped RFB (for browser)
    // Default: "rfb"
    Protocol string `json:"protocol,omitempty"`

    // Encryption enables TLS for VNC connections
    // When true: VNC traffic encrypted with TLS
    // When false: Unencrypted (rely on ingress TLS)
    // Default: false
    Encryption bool `json:"encryption,omitempty"`
}
```

---

## Current Template Manifests: LEGACY kasmvnc Field

### File Count Analysis
```
manifests/templates/              1 template
  └─ firefox.yaml                 Uses "kasmvnc:" field

manifests/templates-generated/    35 templates
  ├─ web-browsers/               5 templates (firefox, chromium, brave, etc.)
  ├─ design-graphics/            7 templates (gimp, blender, inkscape, etc.)
  ├─ development/                3 templates (code-server with vnc disabled)
  ├─ gaming/                      2 templates
  ├─ audio-video/                3 templates
  ├─ desktop-environments/        3 templates
  ├─ productivity/                3 templates
  ├─ communication/               2 templates
  ├─ file-management/             3 templates
  └─ remote-access/               1 template
  
Total: 36 YAML template manifests using LEGACY "kasmvnc" field
```

### Example 1: Firefox (VNC-Enabled Desktop App)
**Location**: `/home/user/streamspace/manifests/templates/browsers/firefox.yaml`

```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: workspaces
spec:
  displayName: Firefox Web Browser
  description: Modern, privacy-focused web browser with extensive extension support
  category: Web Browsers
  icon: https://raw.githubusercontent.com/linuxserver/docker-templates/master/linuxserver.io/img/firefox-logo.png
  baseImage: lscr.io/linuxserver/firefox:latest
  
  # Resource Configuration
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  
  # Port Configuration (VNC on port 3000)
  ports:
    - name: vnc
      containerPort: 3000          # LinuxServer.io KasmVNC port (temporary)
      protocol: TCP
  
  # Environment Variables (standard for LinuxServer.io)
  env:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
    - name: TZ
      value: "America/New_York"
  
  # Volume Mounts (user persistent home)
  volumeMounts:
    - name: user-home
      mountPath: /config
  
  # LEGACY: "kasmvnc" field (should be "vnc")
  kasmvnc:
    enabled: true
    port: 3000
  
  # Capabilities
  capabilities:
    - Network
    - Audio
    - Clipboard
  
  # Tags for discovery
  tags:
    - browser
    - web
    - privacy
    - mozilla
```

### Example 2: Code Server (Non-VNC HTTP App)
**Location**: `/home/user/streamspace/manifests/templates-generated/development/code-server.yaml`

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: code-server
  namespace: streamspace
spec:
  displayName: VS Code Server
  description: Visual Studio Code running in the browser with full IDE features
  category: Development
  baseImage: lscr.io/linuxserver/code-server:latest
  
  defaultResources:
    requests:
      memory: 4Gi
      cpu: 2000m
    limits:
      memory: 4Gi
      cpu: 4000m
  
  # Port Configuration (HTTP, not VNC)
  ports:
    - name: http
      containerPort: 8443           # Code Server HTTPS port
      protocol: TCP
  
  env:
    - name: PUID
      value: '1000'
    - name: PGID
      value: '1000'
    - name: TZ
      value: America/New_York
  
  volumeMounts:
    - name: user-home
      mountPath: /config
  
  # LEGACY: VNC disabled for this app (HTTP-based, not desktop)
  kasmvnc:
    enabled: false                   # Not a VNC-based desktop app
    port: null
  
  capabilities:
    - Network
    - Clipboard
  
  tags:
    - code-server
    - development
```

### Example 3: GIMP (VNC-Enabled Desktop App)
**Location**: `/home/user/streamspace/manifests/templates-generated/design-graphics/gimp.yaml`

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: gimp
spec:
  displayName: GIMP
  description: GNU Image Manipulation Program for photo editing and graphics design
  category: Design & Graphics
  baseImage: lscr.io/linuxserver/gimp:latest
  
  defaultResources:
    requests:
      memory: 4Gi
      cpu: 2000m
    limits:
      memory: 4Gi
      cpu: 4000m
  
  ports:
    - name: vnc
      containerPort: 3000            # KasmVNC (temporary)
      protocol: TCP
  
  env:
    - name: PUID
      value: '1000'
    - name: PGID
      value: '1000'
    - name: TZ
      value: America/New_York
  
  volumeMounts:
    - name: user-home
      mountPath: /config
  
  # LEGACY: kasmvnc configuration
  kasmvnc:
    enabled: true
    port: 3000
  
  capabilities:
    - Network
    - Clipboard
  
  tags:
    - gimp
    - design-graphics
```

---

## Port Configuration Patterns

### VNC-Enabled Desktop Applications
All desktop/GUI apps use:
- **Container Port**: 3000 (LinuxServer.io KasmVNC convention)
- **Port Name**: "vnc"
- **Protocol**: TCP
- **VNC Field**: enabled=true, port=3000

Examples:
- Firefox: port 3000
- Chromium: port 3000
- GIMP: port 3000
- Blender: port 3000
- VS Code: port 8443 (HTTP, not VNC)

### Non-VNC Applications
Code-based editors/IDEs use HTTP:
- **Container Port**: 8443 (Code Server), varies
- **Port Name**: "http" or service-specific
- **VNC Field**: enabled=false, port=null

---

## Environment Variable Configuration

### Standard Variables (LinuxServer.io Convention)
All templates define:
```yaml
env:
  - name: PUID
    value: "1000"           # Process UID (Linux user)
  - name: PGID
    value: "1000"           # Process GID (Linux group)
  - name: TZ
    value: "America/New_York"  # Timezone
```

### Application-Specific Variables
Added per template based on requirements.

---

## Volume Mount Configuration

### Standard Mount Points
All templates define:
```yaml
volumeMounts:
  - name: user-home
    mountPath: /config      # User's persistent home directory
```

**Note**: The `/config` mount is provided by the SessionReconciler in the controller when creating the pod.

---

## Capabilities Declaration

Valid capabilities:
- **Network**: Requires internet access
- **Audio**: Supports audio streaming
- **Clipboard**: Supports clipboard sharing
- **USB**: Supports USB device access
- **Printing**: Supports printer access

Examples:
- Browsers: Network, Audio, Clipboard
- GIMP: Network, Clipboard
- Media Apps: Network, Audio
- Development: Network, Clipboard

---

## Tags for Discovery

Format: lowercase, hyphenated strings

Examples:
```yaml
tags:
  - browser          # Application type
  - web              # Category
  - privacy          # Feature
  - mozilla          # Vendor
  - firefox          # Alternative name
```

---

## Database Schema: kasmvnc Columns (LEGACY)

**Location**: `/home/user/streamspace/manifests/config/database-init.yaml`

Current schema in `templates` table:
```sql
kasmvnc_enabled BOOLEAN DEFAULT true   -- VNC enabled flag
kasmvnc_port INTEGER DEFAULT 3000      -- VNC port number
```

Should be migrated to:
```sql
vnc_enabled BOOLEAN DEFAULT true
vnc_port INTEGER DEFAULT 5900
vnc_protocol VARCHAR(50) DEFAULT 'rfb'
vnc_encryption BOOLEAN DEFAULT false
```

---

## API Integration Points

### Template Parser
**Location**: `/home/user/streamspace/api/internal/sync/parser.go`

```go
type ParsedTemplate struct {
    Name        string   // metadata.name
    DisplayName string   // spec.displayName
    Description string   // spec.description
    Category    string   // spec.category
    AppType     string   // "desktop" (VNC) or "webapp" (HTTP)
    Icon        string   // spec.icon
    Manifest    string   // Full YAML as JSON
    Tags        []string // spec.tags
}
```

Parser infers `AppType` from:
- Presence of VNC configuration in spec
- Port naming conventions
- Application category

---

## CRD Version Discrepancies

### Legacy CRD (Backward Compatibility)
**Location**: `/home/user/streamspace/manifests/crds/workspacetemplate.yaml`

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: workspacetemplates.workspaces.aiinfra.io
```

Still uses old schema with `kasmvnc` field.

### Current CRD
**Location**: `/home/user/streamspace/manifests/crds/template.yaml`

```yaml
metadata:
  name: templates.stream.space
```

Also still uses `kasmvnc` field (needs update).

### Generated Templates
Mixed API versions:
- Some use: `stream.space/v1alpha1` (new)
- Some use: `stream.streamspace.io/v1alpha1` (transitional)
- Some use: `workspaces.aiinfra.io/v1alpha1` (legacy)

---

## VNC Streaming Implementation Details

### Current: LinuxServer.io + KasmVNC (Temporary)
```
Container: lscr.io/linuxserver/<app>:latest
├─ Application (GUI)
├─ Window Manager (XFCE/KDE)
├─ Xvfb (Virtual Framebuffer)
└─ KasmVNC Server
    ├─ Port: 3000 (internal)
    └─ WebSocket enabled for browser access
```

### Future: StreamSpace + TigerVNC (Phase 6)
```
Container: ghcr.io/streamspace/<app>:latest
├─ Application (GUI)
├─ Window Manager (XFCE/i3)
├─ Xvfb (Virtual Framebuffer)
└─ TigerVNC Server
    ├─ Port: 5900 (standard RFB)
    └─ WebSocket proxy via API backend
```

---

## Template Usage in Sessions

### Session CRD References Template
**Location**: `/home/user/streamspace/manifests/crds/session.yaml`

```yaml
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: user1-firefox
spec:
  user: user1
  template: firefox-browser      # References Template by name
  state: running
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
```

The controller:
1. Retrieves the Template CRD by name
2. Extracts VNC configuration (via `spec.vnc` or legacy `spec.kasmvnc`)
3. Creates a Pod with the template's container image
4. Exposes the VNC port via Service
5. Creates WebSocket proxy route in API backend

---

## Migration Roadmap

### Phase 1: Update Go Types (COMPLETE)
- [x] Refactor TemplateSpec to use generic VNCConfig
- [x] Remove Kasm-specific terminology from comments
- [x] Design VNC-agnostic configuration structure

### Phase 2: Update CRD YAML (PENDING)
- [ ] Update `manifests/crds/template.yaml` to use `vnc:` instead of `kasmvnc:`
- [ ] Add migration documentation for existing templates
- [ ] Support dual-field reading (backward compatibility)

### Phase 3: Migrate Template Manifests (PENDING)
- [ ] Convert 40+ template YAML files from `kasmvnc:` to `vnc:`
- [ ] Update API versions to `stream.space/v1alpha1`
- [ ] Update port configurations (3000 → 5900 for future)
- [ ] Add protocol field specifications

### Phase 4: Update Database Schema (PENDING)
- [ ] Rename columns: `kasmvnc_*` → `vnc_*`
- [ ] Add new columns: `vnc_protocol`, `vnc_encryption`
- [ ] Create migration script for existing data

### Phase 5: Build StreamSpace Container Images (PENDING)
- [ ] Create base images with TigerVNC + open source VNC stack
- [ ] Generate 100+ application container images
- [ ] Update templates to use new images

---

## Key Files for Migration

| File | Purpose | Current Status |
|------|---------|-----------------|
| `manifests/crds/template.yaml` | CRD definition | Uses `kasmvnc` field |
| `k8s-controller/api/v1alpha1/template_types.go` | Go types | Uses generic VNCConfig |
| `manifests/templates/browsers/firefox.yaml` | Example template | Uses `kasmvnc` field |
| `manifests/templates-generated/**/*.yaml` | 35 generated templates | Use `kasmvnc` field |
| `manifests/config/database-init.yaml` | DB schema | Has `kasmvnc_*` columns |
| `api/internal/sync/parser.go` | Template parser | VNC-agnostic handling |
| `TEMPLATE_MIGRATION_GUIDE.md` | Migration guide | References `kasmvnc` |
| `scripts/migrate-templates.sh` | Migration tool | Updates template structure |

---

## Summary: CRD Specification

### Required Fields (All Templates)
- `spec.displayName`: Human-readable name (required)
- `spec.baseImage`: Container image reference (required)

### Recommended Fields
- `spec.description`: 2-3 sentence explanation
- `spec.category`: Category for organization
- `spec.icon`: Icon URL (256x256 PNG)
- `spec.defaultResources`: Memory/CPU recommendations

### Optional Fields
- `spec.env`: Environment variables
- `spec.volumeMounts`: Volume mount points
- `spec.ports`: Port definitions
- `spec.vnc` or `spec.kasmvnc`: VNC configuration (currently "kasmvnc", should be "vnc")
- `spec.capabilities`: Feature capabilities
- `spec.tags`: Search tags

### VNC Field Structure
Currently (LEGACY):
```yaml
spec.kasmvnc:
  enabled: boolean
  port: integer
```

Target (MODERN):
```yaml
spec.vnc:
  enabled: boolean
  port: integer
  protocol: string (rfb|websocket)
  encryption: boolean
```

