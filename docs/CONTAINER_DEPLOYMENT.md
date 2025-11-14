# Container Deployment Guide

This guide covers building, deploying, and managing StreamSpace containers.

## Overview

StreamSpace provides container images for all components:
- **Controller**: Kubernetes controller for managing Sessions and Templates
- **API**: REST API backend with database and auth
- **UI**: React-based web frontend (Phase 4)

All images are multi-architecture (amd64/arm64) and available on GitHub Container Registry.

## Table of Contents

- [Quick Start](#quick-start)
- [Building Images](#building-images)
- [Running with Docker Compose](#running-with-docker-compose)
- [Deploying to Kubernetes](#deploying-to-kubernetes)
- [CI/CD Pipeline](#cicd-pipeline)
- [Image Versioning](#image-versioning)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Prerequisites

- Docker 20.10+ with Buildx support
- Docker Compose 2.0+
- make (optional, for using Makefile targets)

### Local Development with Docker Compose

```bash
# Start all services
make docker-compose-up

# Or manually
docker-compose up -d

# Access the API
curl http://localhost:8000/health

# View logs
docker-compose logs -f api

# Stop services
docker-compose down
```

### With Monitoring Stack

```bash
# Start with Prometheus and Grafana
make docker-compose-up-dev

# Access services
# - API:        http://localhost:8000
# - pgAdmin:    http://localhost:5050
# - Prometheus: http://localhost:9090
# - Grafana:    http://localhost:3000
```

## Building Images

### Using Makefile

```bash
# Build all images
make docker-build

# Build specific component
make docker-build-api
make docker-build-controller

# Build with multi-arch support
make docker-build-multiarch
```

### Manual Docker Build

```bash
# Set build variables
VERSION=v0.2.0
COMMIT=$(git rev-parse --short HEAD)
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build API image
docker build \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$COMMIT \
  --build-arg BUILD_DATE=$BUILD_DATE \
  -t ghcr.io/streamspace/streamspace-api:$VERSION \
  -t ghcr.io/streamspace/streamspace-api:latest \
  -f api/Dockerfile \
  api/

# Build Controller image
docker build \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$COMMIT \
  --build-arg BUILD_DATE=$BUILD_DATE \
  -t ghcr.io/streamspace/streamspace-controller:$VERSION \
  -t ghcr.io/streamspace/streamspace-controller:latest \
  -f controller/Dockerfile \
  controller/
```

### Multi-Architecture Builds

```bash
# Create and use buildx builder
docker buildx create --name streamspace-builder --use

# Build for amd64 and arm64
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$COMMIT \
  --build-arg BUILD_DATE=$BUILD_DATE \
  -t ghcr.io/streamspace/streamspace-api:$VERSION \
  --push \
  -f api/Dockerfile \
  api/
```

## Running with Docker Compose

### Configuration Files

**docker-compose.yml**: Main configuration with services:
- `postgres`: PostgreSQL database
- `api`: StreamSpace API backend
- `pgadmin`: Database management UI (dev profile)
- `prometheus`: Metrics collection (monitoring profile)
- `grafana`: Dashboards (monitoring profile)

### Service Profiles

**Default**: API + Database
```bash
docker-compose up -d
```

**Development**: API + Database + pgAdmin
```bash
docker-compose --profile dev up -d
```

**Full Stack**: All services including monitoring
```bash
docker-compose --profile dev --profile monitoring up -d
```

### Environment Variables

Create `.env` file in project root:

```env
# Version
VERSION=v0.2.0
COMMIT=local
BUILD_DATE=2025-11-14T00:00:00Z

# Database
DB_HOST=postgres
DB_PORT=5432
DB_USER=streamspace
DB_PASSWORD=streamspace
DB_NAME=streamspace

# API
API_PORT=8000
GIN_MODE=debug

# Authentication
JWT_SECRET=dev-secret-change-in-production

# Sync
SYNC_INTERVAL=1h
```

### Persistent Data

Docker Compose creates named volumes:
- `streamspace-postgres-data`: Database data
- `streamspace-pgadmin-data`: pgAdmin configuration
- `streamspace-prometheus-data`: Prometheus metrics
- `streamspace-grafana-data`: Grafana dashboards

To reset data:
```bash
docker-compose down -v  # WARNING: Deletes all data
```

### Networking

All services run on the `streamspace` bridge network:
- Internal DNS resolution (e.g., `api`, `postgres`)
- Port mappings for external access
- Isolated from other Docker networks

## Deploying to Kubernetes

### Using Helm

```bash
# Add Helm repository (future)
helm repo add streamspace https://streamspace.github.io/charts
helm repo update

# Install with default values
helm install streamspace streamspace/streamspace \
  --namespace streamspace \
  --create-namespace

# Or install from local chart
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace \
  --set controller.image.tag=v0.2.0 \
  --set api.image.tag=v0.2.0
```

### Custom Values

Create `custom-values.yaml`:

```yaml
# Image configuration
controller:
  image:
    repository: ghcr.io/streamspace/streamspace-controller
    tag: v0.2.0
  replicas: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi

api:
  image:
    repository: ghcr.io/streamspace/streamspace-api
    tag: v0.2.0
  replicas: 2
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

# Database configuration
postgresql:
  enabled: true
  auth:
    username: streamspace
    password: streamspace
    database: streamspace
  persistence:
    enabled: true
    size: 10Gi

# Ingress
ingress:
  enabled: true
  className: traefik
  hosts:
    - host: streamspace.local
      paths:
        - path: /
          pathType: Prefix
```

Install with custom values:
```bash
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace \
  -f custom-values.yaml
```

### Manual Kubernetes Deployment

```bash
# Create namespace
kubectl create namespace streamspace

# Apply CRDs
kubectl apply -f manifests/crds/

# Deploy controller
kubectl apply -f manifests/config/controller-deployment.yaml

# Deploy API
kubectl apply -f manifests/config/api-deployment.yaml

# Check status
kubectl get pods -n streamspace
```

## CI/CD Pipeline

### GitHub Actions Workflows

**Location**: `.github/workflows/`

#### docker.yml - Container Build and Push

Triggers:
- Push to `main` branch
- Git tags (`v*`)
- Manual workflow dispatch

Features:
- Multi-architecture builds (amd64, arm64)
- Automatic tagging based on git metadata
- GitHub Container Registry push
- Build caching with GitHub Actions cache
- Automatic Helm chart version updates

Secrets required:
- `GITHUB_TOKEN` (automatically provided)

#### ci.yml - Continuous Integration

Triggers:
- Pull requests
- Push to any branch

Runs:
- Go tests
- Go linting
- Build verification

#### release.yml - Release Management

Triggers:
- Git tags (`v*`)

Runs:
- Full test suite
- Multi-arch Docker builds
- Helm chart packaging
- GitHub Release creation

### Automated Builds

On every push to `main`:
```
git push origin main
→ GitHub Actions builds and pushes:
  - ghcr.io/streamspace/streamspace-controller:main
  - ghcr.io/streamspace/streamspace-api:main
  - ghcr.io/streamspace/streamspace-controller:latest
  - ghcr.io/streamspace/streamspace-api:latest
```

On git tag:
```
git tag v0.2.0
git push origin v0.2.0
→ GitHub Actions builds and pushes:
  - ghcr.io/streamspace/streamspace-controller:v0.2.0
  - ghcr.io/streamspace/streamspace-controller:0.2.0
  - ghcr.io/streamspace/streamspace-controller:0.2
  - ghcr.io/streamspace/streamspace-controller:0
  - ghcr.io/streamspace/streamspace-controller:latest
```

### Manual Workflow Dispatch

From GitHub UI:
1. Navigate to Actions tab
2. Select "Docker Build and Push"
3. Click "Run workflow"
4. Select branch and click "Run workflow"

## Image Versioning

### Tag Strategy

**Semantic Versioning**: `vMAJOR.MINOR.PATCH`

Examples:
- `v0.1.0` - Initial release
- `v0.2.0` - Feature release
- `v0.2.1` - Patch release
- `v1.0.0` - Stable release

### Docker Tags

For each release, multiple tags are created:

**Full version**:
- `v0.2.0` - Complete semver

**Partial versions**:
- `0.2.0` - Without 'v' prefix
- `0.2` - Major.minor
- `0` - Major only

**Special tags**:
- `latest` - Latest stable release
- `main` - Latest from main branch
- `main-abc1234` - Main branch with commit SHA

### Build Arguments

All images include build-time metadata:

```dockerfile
ARG VERSION=dev        # Git tag or semver
ARG COMMIT=unknown     # Git commit SHA
ARG BUILD_DATE         # ISO 8601 timestamp
```

Access in container:
```bash
# These are compiled into the binary
./api-server --version
# Output: StreamSpace API v0.2.0 (commit: abc1234, built: 2025-11-14T00:00:00Z)
```

### Image Labels

OCI-compliant labels:
```
org.opencontainers.image.title=StreamSpace API
org.opencontainers.image.description=StreamSpace API Backend
org.opencontainers.image.vendor=StreamSpace
org.opencontainers.image.source=https://github.com/yourusername/streamspace
org.opencontainers.image.version=v0.2.0
org.opencontainers.image.created=2025-11-14T00:00:00Z
org.opencontainers.image.revision=abc1234
```

Inspect labels:
```bash
docker inspect ghcr.io/streamspace/streamspace-api:v0.2.0 | jq '.[0].Config.Labels'
```

## Registry Authentication

### GitHub Container Registry (GHCR)

**Public images** (no auth required):
```bash
docker pull ghcr.io/streamspace/streamspace-api:latest
```

**Private images** (auth required):
```bash
# Create Personal Access Token (PAT) with read:packages scope
echo $GITHUB_PAT | docker login ghcr.io -u USERNAME --password-stdin

# Pull image
docker pull ghcr.io/streamspace/streamspace-api:v0.2.0
```

**In Kubernetes**:
```bash
# Create image pull secret
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=USERNAME \
  --docker-password=$GITHUB_PAT \
  -n streamspace

# Reference in deployment
spec:
  imagePullSecrets:
    - name: ghcr-secret
```

## Troubleshooting

### Build Issues

**Problem**: Docker build fails with "no space left on device"

```bash
# Clean up Docker resources
docker system prune -a --volumes

# Check disk usage
docker system df
```

**Problem**: Buildx build fails for arm64

```bash
# Install QEMU emulators
docker run --privileged --rm tonistiigi/binfmt --install all

# Verify platforms
docker buildx inspect --bootstrap
```

### Runtime Issues

**Problem**: API container exits immediately

```bash
# Check logs
docker logs streamspace-api

# Common issues:
# - Database connection failed: Check DB_HOST, DB_PORT
# - Migration errors: Check database credentials
# - Port already in use: Change API_PORT
```

**Problem**: Cannot connect to database from API

```bash
# Check network
docker network inspect streamspace

# Verify postgres is running
docker-compose ps postgres

# Test database connection
docker-compose exec api sh
apk add postgresql-client
psql -h postgres -U streamspace -d streamspace
```

**Problem**: Image pull rate limit (Docker Hub)

```bash
# Use GitHub Container Registry instead
# Edit docker-compose.yml or Helm values to use ghcr.io
```

### Health Checks

**API Health Check**:
```bash
curl http://localhost:8000/health
# Expected: {"status":"healthy","service":"streamspace-api"}
```

**Database Health**:
```bash
docker-compose exec postgres pg_isready -U streamspace
# Expected: /var/run/postgresql:5432 - accepting connections
```

**Container Restarts**:
```bash
# Check restart count
docker ps -a | grep streamspace

# View restart reasons
docker inspect streamspace-api | jq '.[0].State'
```

## Best Practices

### Development

1. **Use Docker Compose** for local development
2. **Mount volumes** for hot-reload (future feature)
3. **Use `.env` file** for environment variables
4. **Run tests** before building images
5. **Tag images** with meaningful versions

### Production

1. **Pin image versions** (avoid `:latest` in production)
2. **Use multi-stage builds** (already implemented)
3. **Run as non-root** (already implemented)
4. **Set resource limits** in Kubernetes
5. **Enable health checks** (already implemented)
6. **Use secrets management** (Kubernetes Secrets, not env vars)
7. **Enable TLS** for API and database connections
8. **Implement backup** strategy for database
9. **Monitor** with Prometheus and Grafana
10. **Scan images** for vulnerabilities (Trivy, Snyk)

### Security

1. **Minimal base images** (Alpine for API, distroless for controller)
2. **No secrets in images** (use environment variables or mounted secrets)
3. **Scan for vulnerabilities** before deploying
4. **Use private registries** for sensitive images
5. **Rotate secrets** regularly (JWT_SECRET, DB_PASSWORD)
6. **Enable network policies** in Kubernetes
7. **Use Pod Security Standards** (restricted mode)

## Next Steps

- Set up image vulnerability scanning
- Implement image signing with Cosign
- Create staging environment
- Set up automated deployments
- Configure monitoring alerts
- Implement backup and restore procedures

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [OCI Image Spec](https://github.com/opencontainers/image-spec)
