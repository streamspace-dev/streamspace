# StreamSpace Controller

This is the Kubernetes controller for StreamSpace, built using the controller-runtime framework.

## What's Implemented

### CRD Types
- **Session CRD** (`api/v1alpha1/session_types.go`): Defines user session resources with states (running/hibernated/terminated)
- **Template CRD** (`api/v1alpha1/template_types.go`): Defines application templates with VNC-agnostic configuration

### Controllers
- **Session Controller** (`controllers/session_controller.go`): Manages session lifecycle
  - Creates/scales Deployments based on session state
  - Handles running (replicas=1), hibernated (replicas=0), and terminated (delete) states
  - Uses generic VNC configuration (not Kasm-specific)
  - Updates session status with pod name, URL, phase

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

### 1. Build Docker image

```bash
docker build -t streamspace-controller:latest .
docker tag streamspace-controller:latest your-registry/streamspace-controller:v0.1.0
docker push your-registry/streamspace-controller:v0.1.0
```

### 2. Deploy controller

```bash
# Update image in ../manifests/config/controller-deployment.yaml
# Then apply:
kubectl apply -f ../manifests/config/rbac.yaml
kubectl apply -f ../manifests/config/controller-deployment.yaml
kubectl apply -f ../manifests/config/controller-configmap.yaml
```

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

## Next Steps

To complete the prototype:

1. **Build and test**: Once network access is available, run `go mod tidy` and `make test`
2. **Add PVC provisioning**: Create persistent home directories for users
3. **Add Service creation**: Expose VNC ports via Kubernetes Services
4. **Add Ingress creation**: Create ingress routes for browser access
5. **Implement hibernation logic**: Add idle timeout detection
6. **Add metrics**: Expose Prometheus metrics for monitoring

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
│   └── samples/            # Example resources
├── controllers/            # Reconciliation logic
│   ├── session_controller.go
│   └── template_controller.go
├── go.mod                  # Go module definition
├── Makefile               # Build automation
└── README.md              # This file
```

## Development Notes

- **API Group**: `stream.streamspace.io` (for CRDs)
- **Domain**: `streamspace.io` (for Kubebuilder)
- **Go Module**: `github.com/streamspace/streamspace`
- **Kubernetes Version**: 1.19+
- **Go Version**: 1.21+

## Strategic Vision

StreamSpace is being built as a 100% open source platform. All references to proprietary software (Kasm) are temporary and will be replaced in Phase 3 with TigerVNC + noVNC stack. See `/ROADMAP.md` for the complete development plan.
