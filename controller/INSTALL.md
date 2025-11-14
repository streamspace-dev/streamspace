# StreamSpace Controller Installation Guide

This guide covers installing and deploying the StreamSpace controller to a Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured to access your cluster
- For persistent storage: NFS provisioner or ReadWriteMany-capable storage class
- Docker or Podman for building images

## Quick Start (Using Kustomize)

The fastest way to deploy StreamSpace:

```bash
# Deploy everything with kustomize
kubectl apply -k config/default/

# Verify installation
kubectl get pods -n streamspace
kubectl get crds | grep streamspace
```

This will install:
- ✅ streamspace namespace
- ✅ Session and Template CRDs
- ✅ Controller deployment
- ✅ RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)
- ✅ Metrics service

## Manual Installation

If you prefer step-by-step installation:

### 1. Create Namespace

```bash
kubectl create namespace streamspace
```

### 2. Install CRDs

```bash
kubectl apply -f config/crd/bases/stream.streamspace.io_sessions.yaml
kubectl apply -f config/crd/bases/stream.streamspace.io_templates.yaml

# Verify
kubectl get crds | grep stream.streamspace.io
```

### 3. Install RBAC

```bash
kubectl apply -f config/rbac/rbac.yaml

# Verify
kubectl get serviceaccount -n streamspace
kubectl get clusterrole streamspace-controller-role
```

### 4. Build Controller Image

```bash
# Option 1: Build locally
docker build -t streamspace-controller:latest .

# Option 2: Build for specific registry
docker build -t ghcr.io/your-org/streamspace-controller:v0.1.0 .
docker push ghcr.io/your-org/streamspace-controller:v0.1.0
```

### 5. Update Image Reference

Edit `config/manager/deployment.yaml` and update the image:

```yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        image: ghcr.io/your-org/streamspace-controller:v0.1.0  # Update this
```

### 6. Deploy Controller

```bash
kubectl apply -f config/manager/deployment.yaml
kubectl apply -f config/manager/service.yaml

# Verify
kubectl get pods -n streamspace
kubectl logs -n streamspace deployment/streamspace-controller
```

## Installing Sample Templates

StreamSpace includes 6 pre-built application templates:

| Template | Description | Category | Base Image |
|----------|-------------|----------|------------|
| firefox-browser | Mozilla Firefox | Web Browsers | lscr.io/linuxserver/firefox |
| chrome-browser | Google Chrome | Web Browsers | lscr.io/linuxserver/chromium |
| vscode | Visual Studio Code | Development | lscr.io/linuxserver/code-server |
| libreoffice | LibreOffice Suite | Productivity | lscr.io/linuxserver/libreoffice |
| gimp | GIMP Image Editor | Design | lscr.io/linuxserver/gimp |
| ubuntu-desktop | Full Ubuntu Desktop | Desktop Environments | lscr.io/linuxserver/webtop |

**Install all templates**:
```bash
kubectl apply -f config/samples/template_*.yaml

# Or use kustomize (automatically includes all templates)
kubectl apply -k config/default/
```

**Install specific template**:
```bash
kubectl apply -f config/samples/template_firefox.yaml

# Verify
kubectl get templates -n streamspace
kubectl describe template firefox-browser -n streamspace
```

## Creating Your First Session

Create a test session:

```bash
kubectl apply -f config/samples/session_test.yaml

# Watch it come up
kubectl get sessions,deployments,services,pods -n streamspace -w
```

You should see:
- Session resource created
- Deployment created with 1 replica
- Service created for VNC access
- PVC created for user home directory (if persistentHome: true)
- Pod running

## Verify Session Details

```bash
# Get session status
kubectl get session testuser-firefox -n streamspace -o wide

# Check detailed status
kubectl describe session testuser-firefox -n streamspace

# View pod logs
kubectl logs -n streamspace -l session=testuser-firefox
```

## Using Helper Scripts

StreamSpace includes helper scripts for common operations. See [scripts/README.md](scripts/README.md) for full documentation.

### Create a Session

```bash
# Create Firefox session for user Alice
./scripts/create-session.sh alice firefox-browser alice-firefox

# Output shows:
# ✓ Session created
# ✓ Session is running
# 🌐 Access your session at: https://alice-firefox.streamspace.local
```

### List Sessions

```bash
./scripts/list-sessions.sh

# Output:
# NAME            USER   TEMPLATE         STATE     PHASE     URL
# alice-firefox   alice  firefox-browser  running   Running   https://alice-firefox.streamspace.local
```

### Hibernate/Wake Sessions

```bash
# Hibernate to save resources
./scripts/hibernate-session.sh alice-firefox

# Wake when needed
./scripts/wake-session.sh alice-firefox
```

### View Metrics

```bash
./scripts/get-metrics.sh

# Opens port-forward and displays StreamSpace metrics
# Press Ctrl+C to exit
```

## Configuration

### Storage Configuration

By default, sessions request ReadWriteMany PVCs for persistent user homes. Configure your storage class:

```yaml
# If using NFS provisioner
apiVersion: v1
kind: PersistentVolumeClaim
spec:
  accessModes:
  - ReadWriteMany
  storageClassName: nfs-client  # Your NFS storage class
```

### Resource Limits

Controller resources can be adjusted in `config/manager/deployment.yaml`:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### Leader Election

The controller supports leader election for high availability. To run multiple replicas:

```bash
# Edit deployment
kubectl edit deployment streamspace-controller -n streamspace

# Change replicas
spec:
  replicas: 3  # Run 3 controller instances
```

## Monitoring

### Metrics Endpoint

The controller exposes Prometheus metrics:

```bash
# Port forward to metrics
kubectl port-forward -n streamspace svc/streamspace-controller-metrics 8080:8080

# Query metrics
curl http://localhost:8080/metrics
```

### Health Checks

Health endpoints:

```bash
# Liveness probe
curl http://localhost:8081/healthz

# Readiness probe
curl http://localhost:8081/readyz
```

### Prometheus ServiceMonitor

If using Prometheus Operator, deploy a ServiceMonitor:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: streamspace-controller
  namespace: streamspace
spec:
  selector:
    matchLabels:
      app: streamspace-controller
  endpoints:
  - port: metrics
    interval: 30s
```

## Upgrading

### Upgrade Controller

```bash
# Build new image
docker build -t streamspace-controller:v0.2.0 .
docker push ghcr.io/your-org/streamspace-controller:v0.2.0

# Update deployment
kubectl set image -n streamspace deployment/streamspace-controller \
  manager=ghcr.io/your-org/streamspace-controller:v0.2.0

# Verify rollout
kubectl rollout status -n streamspace deployment/streamspace-controller
```

### Upgrade CRDs

**IMPORTANT**: Always backup your resources before upgrading CRDs!

```bash
# Backup existing sessions
kubectl get sessions -n streamspace -o yaml > sessions-backup.yaml

# Apply new CRD
kubectl apply -f config/crd/bases/stream.streamspace.io_sessions.yaml

# Verify
kubectl get crds stream.streamspace.io -o yaml
```

## Uninstalling

### Remove Sessions (preserves PVCs)

```bash
# Delete all sessions
kubectl delete sessions --all -n streamspace

# PVCs will remain for data preservation
```

### Remove Controller

```bash
# Using kustomize
kubectl delete -k config/default/

# Or manually
kubectl delete deployment streamspace-controller -n streamspace
kubectl delete service streamspace-controller-metrics -n streamspace
kubectl delete -f config/rbac/rbac.yaml
kubectl delete -f config/crd/bases/
kubectl delete namespace streamspace
```

### Clean Up User Data

**WARNING**: This deletes all user home directories!

```bash
# Delete all user PVCs
kubectl delete pvc -n streamspace -l app=streamspace-user-home
```

## Troubleshooting

### Controller Not Starting

```bash
# Check pod status
kubectl describe pod -n streamspace -l app=streamspace-controller

# View logs
kubectl logs -n streamspace deployment/streamspace-controller

# Common issues:
# - CRDs not installed: kubectl get crds | grep stream.streamspace.io
# - RBAC issues: kubectl auth can-i create sessions --as=system:serviceaccount:streamspace:streamspace-controller
# - Image pull errors: Check image name and registry access
```

### Session Not Creating

```bash
# Check session status
kubectl describe session <name> -n streamspace

# Check controller logs
kubectl logs -n streamspace deployment/streamspace-controller | grep <session-name>

# Common issues:
# - Template not found: kubectl get template <template-name> -n streamspace
# - Image pull failures: Check template baseImage
# - Storage issues: kubectl describe pvc -n streamspace
```

### PVC Not Binding

```bash
# Check PVC status
kubectl describe pvc home-<username> -n streamspace

# Check storage class
kubectl get storageclass

# Common issues:
# - No storage class configured
# - NFS provisioner not running
# - Storage class doesn't support ReadWriteMany
```

## Development Mode

Run controller locally for development:

```bash
# Install CRDs to cluster
kubectl apply -f config/crd/bases/

# Run controller locally (connects to cluster via kubeconfig)
go run cmd/main.go

# In another terminal, create test resources
kubectl apply -f config/samples/
```

## Next Steps

- **Add More Templates**: Create templates for your applications
- **Configure Ingress**: Set up ingress for browser access to sessions
- **Enable Monitoring**: Deploy Prometheus and Grafana
- **Scale Up**: Run multiple controller replicas with leader election
- **Phase 3**: Plan TigerVNC migration (see `/docs/VNC_MIGRATION.md`)

## Support

- Documentation: `/controller/README.md`
- Architecture: `/docs/ARCHITECTURE.md`
- Roadmap: `/ROADMAP.md`
- Issues: GitHub Issues
