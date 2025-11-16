# Local Testing Guide

This guide explains how to build, deploy, test, and teardown StreamSpace in your local Kubernetes environment (e.g., Docker Desktop).

## Prerequisites

Before using these scripts, ensure you have:

- **Docker Desktop** with Kubernetes enabled
- **kubectl** configured to access your local cluster
- **Helm 3+** installed
- **Git** (for version information)

To verify your setup:

```bash
docker info
kubectl cluster-info
helm version
```

## Quick Start

### 1. Build All Images Locally

Build all three StreamSpace Docker images (controller, API, UI):

```bash
./scripts/local-build.sh
```

This will:
- Build `streamspace/streamspace-controller:local`
- Build `streamspace/streamspace-api:local`
- Build `streamspace/streamspace-ui:local`
- Tag each image with `latest` as well
- Display a summary of built images

**Build individual components:**

```bash
./scripts/local-build.sh controller  # Build only controller
./scripts/local-build.sh api         # Build only API
./scripts/local-build.sh ui          # Build only UI
```

### 2. Deploy to Local Kubernetes

Deploy StreamSpace to your local cluster using Helm:

```bash
./scripts/local-deploy.sh
```

This will:
- Check that all required images exist
- Create the `streamspace` namespace
- Apply Custom Resource Definitions (CRDs)
- Install/upgrade the Helm release
- Wait for all pods to be ready
- Display deployment status and access instructions

**Environment Variables:**

```bash
# Customize namespace (default: streamspace)
NAMESPACE=my-namespace ./scripts/local-deploy.sh

# Customize release name (default: streamspace)
RELEASE_NAME=my-release ./scripts/local-deploy.sh

# Use different version tag (default: local)
VERSION=v0.2.0 ./scripts/local-deploy.sh
```

### 3. Access the Application

After deployment, access StreamSpace:

**UI (Web Interface):**

```bash
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80
```

Then open: http://localhost:3000

**API Backend:**

```bash
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000
```

Then access: http://localhost:8000

**View Logs:**

```bash
# Controller logs
kubectl logs -n streamspace -l app.kubernetes.io/component=controller -f

# API logs
kubectl logs -n streamspace -l app.kubernetes.io/component=api -f

# UI logs
kubectl logs -n streamspace -l app.kubernetes.io/component=ui -f
```

### 4. Teardown and Cleanup

When you're done testing, completely remove StreamSpace and clean up Docker artifacts:

```bash
./scripts/local-teardown.sh
```

This will:
- Uninstall the Helm release
- Delete the namespace and all resources
- Remove Custom Resource Definitions (CRDs)
- Delete local Docker images
- Clean up dangling images and stopped containers
- Display remaining resources and disk usage

**Auto-confirm (skip prompt):**

```bash
AUTO_CONFIRM=true ./scripts/local-teardown.sh
```

**Clean build cache (aggressive cleanup):**

```bash
CLEAN_CACHE=true ./scripts/local-teardown.sh
```

## Complete Development Cycle

Here's a typical development workflow:

```bash
# 1. Build images
./scripts/local-build.sh

# 2. Deploy to cluster
./scripts/local-deploy.sh

# 3. Test your changes
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80

# 4. Make code changes, rebuild specific component
./scripts/local-build.sh api

# 5. Upgrade deployment
./scripts/local-deploy.sh

# 6. Repeat testing...

# 7. When done, clean up everything
./scripts/local-teardown.sh
```

## Troubleshooting

### Images Not Found

If deployment fails with "image not found":

```bash
# Check if images exist
docker images | grep streamspace

# Rebuild images
./scripts/local-build.sh

# Verify version tag matches
VERSION=local ./scripts/local-deploy.sh
```

### Pods Not Starting

Check pod status and logs:

```bash
# Check pod status
kubectl get pods -n streamspace

# Describe pod for events
kubectl describe pod <pod-name> -n streamspace

# View pod logs
kubectl logs <pod-name> -n streamspace
```

### Helm Installation Fails

Check Helm release status:

```bash
# List Helm releases
helm list -n streamspace

# Check release status
helm status streamspace -n streamspace

# Uninstall and retry
helm uninstall streamspace -n streamspace
./scripts/local-deploy.sh
```

### Namespace Stuck in Terminating

If namespace deletion hangs:

```bash
# Force delete namespace (use with caution)
kubectl get namespace streamspace -o json \
  | jq '.spec.finalizers = []' \
  | kubectl replace --raw /api/v1/namespaces/streamspace/finalize -f -
```

### Docker Out of Disk Space

Clean up Docker resources:

```bash
# Remove all unused images, containers, networks
docker system prune -a

# Or use aggressive cleanup
CLEAN_CACHE=true ./scripts/local-teardown.sh
docker system prune -a --volumes
```

## Advanced Usage

### Using Make Targets

The root `Makefile` also provides targets for local development:

```bash
# Build all components
make docker-build

# Deploy with Helm
make helm-install

# Uninstall
make helm-uninstall

# Clean Docker images
make clean-docker
```

### Custom Helm Values

Deploy with custom Helm values:

```bash
# Create custom values file
cat > custom-values.yaml <<EOF
controller:
  replicaCount: 1
  resources:
    requests:
      memory: 128Mi
api:
  replicaCount: 1
postgresql:
  auth:
    password: mypassword
EOF

# Install with custom values
helm install streamspace ./chart \
  -n streamspace \
  --create-namespace \
  -f custom-values.yaml \
  --set controller.image.tag=local \
  --set controller.image.pullPolicy=Never \
  --set api.image.tag=local \
  --set api.image.pullPolicy=Never \
  --set ui.image.tag=local \
  --set ui.image.pullPolicy=Never
```

### Debugging with Remote Debugger

For Go components (controller, API):

```bash
# Build with debug symbols
cd controller
go build -gcflags="all=-N -l" -o bin/manager cmd/main.go

# Run with delve debugger
dlv exec ./bin/manager --headless --listen=:2345 --api-version=2
```

### Testing Without Helm

Deploy manually using kubectl:

```bash
# Apply CRDs
kubectl apply -f chart/crds/

# Apply manifests
kubectl apply -f manifests/config/namespace.yaml
kubectl apply -f manifests/config/rbac.yaml
kubectl apply -f manifests/config/controller-deployment.yaml
# ... etc
```

## Script Reference

### local-build.sh

**Purpose:** Build Docker images locally

**Arguments:**
- No arguments: Build all components
- `controller`: Build only controller
- `api`: Build only API backend
- `ui`: Build only web UI

**Environment Variables:**
- `VERSION`: Image tag (default: `local`)
- `GIT_COMMIT`: Git commit hash (auto-detected)

**Examples:**

```bash
# Build all with custom version
VERSION=v1.0.0 ./scripts/local-build.sh

# Build only API
./scripts/local-build.sh api

# Build controller and UI
./scripts/local-build.sh controller ui
```

### local-deploy.sh

**Purpose:** Deploy StreamSpace to local Kubernetes

**Environment Variables:**
- `NAMESPACE`: Kubernetes namespace (default: `streamspace`)
- `RELEASE_NAME`: Helm release name (default: `streamspace`)
- `VERSION`: Image version tag (default: `local`)

**Examples:**

```bash
# Deploy to different namespace
NAMESPACE=dev ./scripts/local-deploy.sh

# Deploy with different release name
RELEASE_NAME=ss-test ./scripts/local-deploy.sh

# Deploy using different image version
VERSION=v0.2.0 ./scripts/local-deploy.sh
```

### local-teardown.sh

**Purpose:** Remove StreamSpace and clean up artifacts

**Environment Variables:**
- `NAMESPACE`: Kubernetes namespace (default: `streamspace`)
- `RELEASE_NAME`: Helm release name (default: `streamspace`)
- `VERSION`: Image version to remove (default: `local`)
- `AUTO_CONFIRM`: Skip confirmation prompt (default: `false`)
- `CLEAN_CACHE`: Clean Docker build cache (default: `false`)

**Examples:**

```bash
# Standard teardown with confirmation
./scripts/local-teardown.sh

# Auto-confirm without prompt
AUTO_CONFIRM=true ./scripts/local-teardown.sh

# Aggressive cleanup including build cache
CLEAN_CACHE=true AUTO_CONFIRM=true ./scripts/local-teardown.sh

# Teardown specific namespace
NAMESPACE=dev ./scripts/local-teardown.sh
```

## Integration with CI/CD

These scripts can be used in CI/CD pipelines:

```bash
# Example GitHub Actions workflow
- name: Build images
  run: ./scripts/local-build.sh

- name: Deploy to cluster
  run: |
    AUTO_CONFIRM=true ./scripts/local-deploy.sh

- name: Run tests
  run: |
    kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &
    make test-integration

- name: Cleanup
  if: always()
  run: |
    AUTO_CONFIRM=true CLEAN_CACHE=true ./scripts/local-teardown.sh
```

## Additional Resources

- **Main Makefile:** See `make help` for all available targets
- **Helm Chart:** See `chart/values.yaml` for configuration options
- **Architecture:** See `docs/ARCHITECTURE.md`
- **Contributing:** See `CONTRIBUTING.md`

## Support

If you encounter issues:

1. Check the **Troubleshooting** section above
2. Review logs: `kubectl logs -n streamspace <pod-name>`
3. Check pod status: `kubectl describe pod -n streamspace <pod-name>`
4. Open an issue on GitHub with detailed error messages

## License

MIT License - See LICENSE file for details
