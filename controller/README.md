# StreamSpace Controller

This is the Kubernetes controller for StreamSpace, built using the controller-runtime framework.

## What's Implemented

### CRD Types
- **Session CRD** (`api/v1alpha1/session_types.go`): Defines user session resources with states (running/hibernated/terminated)
- **Template CRD** (`api/v1alpha1/template_types.go`): Defines application templates with VNC-agnostic configuration

### Controllers
- **Session Controller** (`controllers/session_controller.go`): Manages complete session lifecycle
  - Creates/scales Deployments based on session state
  - Handles running (replicas=1), hibernated (replicas=0), and terminated (delete) states
  - **Creates Services** for VNC port exposure
  - **Provisions PVCs** for persistent user home directories
  - Uses generic VNC configuration (not Kasm-specific)
  - Updates session status with pod name, URL, phase
  - Automatic resource cleanup via owner references

- **Template Controller** (`controllers/template_controller.go`): Validates templates
  - Ensures required fields (baseImage, displayName)
  - Validates VNC configuration
  - Sets default VNC port (5900) if not specified
  - Updates template status (Ready/Invalid)

### CRD Manifests
- `config/crd/bases/stream.streamspace.io_sessions.yaml`: Session CRD definition
- `config/crd/bases/stream.streamspace.io_templates.yaml`: Template CRD definition

### Sample Manifests
- `config/samples/template_firefox.yaml`: Firefox browser template using LinuxServer.io image
- `config/samples/session_test.yaml`: Test session for firefox-browser

## Building

```bash
# Download dependencies (requires network access)
go mod tidy

# Build the controller
go build -o bin/manager cmd/main.go

# Or use make
make build
```

## Testing Locally

### 1. Install CRDs

```bash
kubectl apply -f config/crd/bases/stream.streamspace.io_sessions.yaml
kubectl apply -f config/crd/bases/stream.streamspace.io_templates.yaml
```

### 2. Create namespace

```bash
kubectl create namespace streamspace
```

### 3. Create template

```bash
kubectl apply -f config/samples/template_firefox.yaml
```

### 4. Run controller locally

```bash
go run cmd/main.go
```

### 5. Create a test session

```bash
kubectl apply -f config/samples/session_test.yaml
```

### 6. Verify resources

```bash
# Check session status
kubectl get sessions -n streamspace
kubectl describe session testuser-firefox -n streamspace

# Check created deployment
kubectl get deployments -n streamspace -l session=testuser-firefox

# Check pods
kubectl get pods -n streamspace -l session=testuser-firefox
```

## Deployment to Cluster

### Quick Deploy with Kustomize (Recommended)

```bash
# Deploy everything at once
kubectl apply -k config/default/

# Verify
kubectl get pods -n streamspace
kubectl get crds | grep streamspace
```

### Manual Deployment

#### 1. Build Docker image

```bash
docker build -t streamspace-controller:latest .
docker tag streamspace-controller:latest ghcr.io/your-org/streamspace-controller:v0.1.0
docker push ghcr.io/your-org/streamspace-controller:v0.1.0
```

#### 2. Deploy controller

```bash
# Install CRDs
kubectl apply -f config/crd/bases/

# Install RBAC
kubectl apply -f config/rbac/rbac.yaml

# Deploy controller
kubectl apply -f config/manager/deployment.yaml
kubectl apply -f config/manager/service.yaml

# Verify
kubectl get pods -n streamspace
```

See [INSTALL.md](INSTALL.md) for complete installation guide.

## Key Design Features

### VNC-Agnostic Architecture

The controller uses generic VNC configuration, NOT Kasm-specific:

```go
// ✅ GOOD - Generic VNC config
type VNCConfig struct {
    Port     int    `json:"port"`      // 5900 or 3000
    Protocol string `json:"protocol"`  // "rfb", "websocket"
}
```

This prepares for Phase 3 migration to TigerVNC + noVNC (see `/docs/VNC_MIGRATION.md`).

### State-Driven Reconciliation

Sessions use a state machine:
- **running**: Create deployment with replicas=1
- **hibernated**: Scale deployment to replicas=0 (preserves pod spec)
- **terminated**: Delete deployment

### Resource Management

- Sessions can override template default resources
- Owner references ensure garbage collection
- Labels enable efficient querying

## Features Complete

✅ **Core functionality implemented**:
- ✅ Session and Template CRDs
- ✅ State-driven session lifecycle management
- ✅ Deployment creation and scaling
- ✅ Service creation for VNC access
- ✅ **Ingress creation** for browser access
- ✅ PVC provisioning for persistent user homes
- ✅ VNC-agnostic architecture
- ✅ RBAC configuration
- ✅ Kustomize deployment
- ✅ Dockerfile for containerization
- ✅ **Custom Prometheus metrics** (sessions, reconciliations, templates)
- ✅ Health and readiness probes
- ✅ Leader election support
- ✅ Configurable ingress domain and class

## Next Enhancements

Future improvements (not needed for basic functionality):

1. **Idle timeout detection**: Implement automatic hibernation based on activity
2. **Resource quotas**: Per-user resource limits and quotas
3. **Webhooks**: Add validating/mutating webhooks for CRDs
4. **Grafana dashboards**: Pre-built dashboards for metrics
5. **Phase 3**: TigerVNC migration (see `/docs/VNC_MIGRATION.md`)

## File Structure

```
controller/
├── api/v1alpha1/           # CRD type definitions
│   ├── groupversion_info.go
│   ├── session_types.go
│   └── template_types.go
├── cmd/
│   └── main.go             # Controller entry point
├── config/
│   ├── crd/bases/          # Generated CRD manifests
│   │   ├── stream.streamspace.io_sessions.yaml
│   │   └── stream.streamspace.io_templates.yaml
│   ├── default/            # Kustomize deployment
│   │   ├── kustomization.yaml
│   │   └── namespace.yaml
│   ├── manager/            # Controller deployment
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   ├── rbac/               # RBAC configuration
│   │   └── rbac.yaml
│   └── samples/            # Example resources
│       ├── template_firefox.yaml
│       └── session_test.yaml
├── controllers/            # Reconciliation logic
│   ├── session_controller.go  (380+ lines)
│   └── template_controller.go
├── Dockerfile              # Container build
├── go.mod                  # Go module definition
├── Makefile               # Build automation
├── README.md              # This file
└── INSTALL.md             # Installation guide
```

## Development Notes

- **API Group**: `stream.streamspace.io` (for CRDs)
- **Domain**: `streamspace.io` (for Kubebuilder)
- **Go Module**: `github.com/streamspace/streamspace`
- **Kubernetes Version**: 1.19+
- **Go Version**: 1.21+

## Strategic Vision

StreamSpace is being built as a 100% open source platform. All references to proprietary software (Kasm) are temporary and will be replaced in Phase 3 with TigerVNC + noVNC stack. See `/ROADMAP.md` for the complete development plan.
