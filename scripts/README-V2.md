# StreamSpace v2.0 Scripts Guide

**Date**: 2025-11-21
**Architecture**: Multi-Platform Agent Architecture (v2.0)

---

## Overview

This directory contains scripts for StreamSpace development and deployment. Many scripts were designed for the v1.0 CRD-based architecture and need updating for v2.0's agent-based architecture.

---

## Script Status for v2.0

### ✅ Still Relevant (No Changes Needed)

These scripts work with v2.0 architecture:

| Script | Purpose | Status |
|--------|---------|--------|
| `generate-templates.py` | Generate application templates from catalog | ✅ Works with v2.0 |
| `generate-from-catalog.py` | Generate templates from LinuxServer.io | ✅ Works with v2.0 |
| `popular-apps.json` | Popular application list | ✅ Data file, architecture-agnostic |

**Usage**:
```bash
# Generate templates (works in v2.0)
python3 scripts/generate-templates.py
```

---

### ⚠️ Needs Updates for v2.0

These scripts reference v1.0 architecture (CRDs, controller) and need updates:

| Script | Purpose | v2.0 Status |
|--------|---------|-------------|
| `local-deploy.sh` | Deploy locally with k3d | ⚠️ Needs agent deployment updates |
| `local-deploy-kubectl.sh` | Deploy with kubectl | ⚠️ References CRDs, needs agent updates |
| `local-deploy-alt.sh` | Alternative deployment | ⚠️ Needs agent updates |
| `local-build.sh` | Build components locally | ⚠️ Should build K8s Agent, not controller |
| `local-stop-apps.sh` | Stop local apps | ⚠️ Minor updates needed |
| `local-teardown.sh` | Teardown local env | ⚠️ Minor updates needed |
| `local-port-forward.sh` | Port-forward services | ✅ Mostly works, add agent logs |
| `local-stop-port-forward.sh` | Stop port-forwards | ✅ Works as-is |
| `build-docker-controller.sh` | Build controller image | ⚠️ Rename to build K8s Agent |
| `docker-dev.sh` | Docker dev environment | ⚠️ Update for Control Plane + Agent |
| `docker-dev-stop.sh` | Stop Docker dev | ✅ Works as-is |
| `test-nats.sh` | Test NATS connectivity | ⚠️ Update for agent WebSocket |
| `migrate-templates.sh` | Migrate v1 templates | ⚠️ Update for v2.0 template sync |
| `create-admin-secret.sh` | Create admin credentials | ✅ Works as-is |

---

## v2.0 Architecture Changes

### What Changed

**v1.0 Architecture (Deprecated)**:
```
User → UI → API → K8s Controller (CRD-based) → Pods
                   ↑
              Watches CRDs
```

**v2.0 Architecture (Current)**:
```
User → UI → Control Plane API → WebSocket → K8s Agent → Pods
                                              ↓
                                         Docker Agent → Containers
```

### Key Differences

1. **No More CRDs** (in Control Plane)
   - Sessions and Templates are now in PostgreSQL
   - Agents receive commands via WebSocket, not CRD watches

2. **Agent-Based**
   - K8s Agent replaces K8s Controller
   - Agents connect TO Control Plane (outbound only)
   - Multi-platform support (K8s, Docker, VMs, Cloud)

3. **VNC Proxy**
   - VNC traffic tunneled through Control Plane
   - No direct pod access required
   - Works across network boundaries

---

## Recommended v2.0 Scripts (To Be Created)

### Priority 1: Development Scripts

1. **`v2-local-deploy.sh`**
   - Deploy Control Plane + K8s Agent locally
   - Create test sessions via Control Plane API
   - Setup example agent configuration

2. **`v2-build-all.sh`**
   - Build K8s Agent, API, UI
   - Build Docker images for v2.0 components
   - Version tagging

3. **`v2-test-agent-connection.sh`**
   - Test K8s Agent → Control Plane WebSocket connection
   - Verify agent registration
   - Check heartbeat

4. **`v2-test-vnc-proxy.sh`**
   - Test VNC proxy functionality
   - Create session and verify VNC streaming
   - Check latency

### Priority 2: Deployment Scripts

5. **`v2-deploy-control-plane.sh`**
   - Deploy Control Plane (API + UI + Database)
   - Initialize database
   - Apply configurations

6. **`v2-deploy-k8s-agent.sh`**
   - Deploy K8s Agent to cluster
   - Configure agent ID and Control Plane URL
   - Verify registration

7. **`v2-health-check.sh`**
   - Check Control Plane health
   - Check agent connections
   - Verify database connectivity
   - Check VNC proxy

### Priority 3: Migration Scripts

8. **`v1-to-v2-migrate.sh`**
   - Migrate v1.0 CRDs to v2.0 database
   - Export v1.0 sessions
   - Import into v2.0 Control Plane
   - Verify migration

9. **`v1-to-v2-cleanup.sh`**
   - Remove v1.0 CRDs
   - Uninstall v1.0 controller
   - Clean up v1.0 resources

---

## Using Makefiles Instead of Scripts

For v2.0, many script functions are now in Makefiles:

### Root Makefile

```bash
# Setup development environment
make dev-setup

# Build all v2.0 components
make build

# Test all components
make test

# Build and push Docker images
make docker-build
make docker-push

# Deploy to Kubernetes
make helm-install

# Run locally
make dev-run-api        # Terminal 1: Control Plane
make dev-run-k8s-agent  # Terminal 2: K8s Agent
make dev-run-ui         # Terminal 3: UI

# Check deployment status
make k8s-status

# View logs
make k8s-logs-api
make k8s-logs-k8s-agent
make k8s-logs-ui
```

### K8s Agent Makefile

```bash
cd agents/k8s-agent

# Build agent binary
make build

# Run tests
make test

# Build Docker image
make docker-build

# Deploy to cluster
make deploy

# View logs
make logs

# Check status
make status
```

---

## Migration Strategy

### For Existing v1.0 Users

1. **Deploy v2.0 alongside v1.0** (parallel deployment)
   ```bash
   # Keep v1.0 running in namespace 'streamspace'
   # Deploy v2.0 in namespace 'streamspace-v2'
   ```

2. **Migrate sessions incrementally**
   - Export v1.0 sessions
   - Create equivalent sessions in v2.0
   - Test VNC connectivity
   - Migrate users in batches

3. **Decommission v1.0**
   ```bash
   # Once v2.0 is validated:
   helm uninstall streamspace -n streamspace
   kubectl delete crd sessions.stream.space
   kubectl delete crd templates.stream.space
   ```

### For New Deployments

Use v2.0 from the start:

```bash
# Option 1: Helm
make helm-install

# Option 2: Manual
make docker-build
make k8s-deploy-control-plane
make k8s-deploy-k8s-agent

# Option 3: Development
make docker-compose-up
cd agents/k8s-agent && make run
```

---

## Development Workflow

### Local Development (v2.0)

```bash
# Terminal 1: Start database
docker-compose up postgres

# Terminal 2: Run Control Plane API
cd api
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=streamspace
export DB_USER=postgres
export DB_PASSWORD=postgres
go run cmd/main.go

# Terminal 3: Run K8s Agent (requires kubeconfig)
cd agents/k8s-agent
export AGENT_ID=k8s-local-dev
export CONTROL_PLANE_URL=ws://localhost:8000
go run .

# Terminal 4: Run UI
cd ui
npm start
```

### Testing Agent Communication

```bash
# Check agent registration
curl http://localhost:8000/api/v1/agents

# Check agent heartbeat
kubectl logs -n streamspace -l component=k8s-agent | grep heartbeat

# Create test session via Control Plane
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "platform": "kubernetes"
  }'
```

---

## Script Updates Needed

### Example: Updating `local-deploy.sh` for v2.0

**Old (v1.0)**:
```bash
# Deploy controller
kubectl apply -f manifests/config/controller-deployment.yaml

# Apply CRDs
kubectl apply -f manifests/crds/session.yaml
kubectl apply -f manifests/crds/template.yaml
```

**New (v2.0)**:
```bash
# Deploy Control Plane
kubectl apply -f manifests/v2/control-plane/

# Deploy K8s Agent
cd agents/k8s-agent
make deploy

# No CRDs needed (sessions in database)
```

### Example: Updating `local-build.sh` for v2.0

**Old (v1.0)**:
```bash
# Build controller
cd controller
make build
```

**New (v2.0)**:
```bash
# Build K8s Agent
cd agents/k8s-agent
make build

# Build Control Plane API
cd api
go build -o bin/api cmd/main.go

# Build UI
cd ui
npm run build
```

---

## Quick Reference

### v1.0 → v2.0 Component Mapping

| v1.0 Component | v2.0 Equivalent | Build Command |
|----------------|-----------------|---------------|
| k8s-controller | agents/k8s-agent | `make build-k8s-agent` |
| api (unchanged) | api (enhanced) | `make build-api` |
| ui (unchanged) | ui (minor updates) | `make build-ui` |
| CRDs | Database tables | N/A (schema in migrations) |
| Controller deployment | Agent deployment | `make k8s-deploy-k8s-agent` |

### v1.0 → v2.0 kubectl Commands

| v1.0 Command | v2.0 Equivalent |
|--------------|-----------------|
| `kubectl get sessions` | `curl $API/api/v1/sessions` |
| `kubectl describe session $ID` | `curl $API/api/v1/sessions/$ID` |
| `kubectl delete session $ID` | `curl -X DELETE $API/api/v1/sessions/$ID` |
| `kubectl get templates` | `curl $API/api/v1/templates` |
| Controller logs | `make k8s-logs-k8s-agent` |

---

## Documentation

**For more details, see**:
- `docs/V2_ARCHITECTURE_STATUS.md` - Complete v2.0 assessment
- `docs/REFACTOR_ARCHITECTURE_V2.md` - Technical architecture spec
- `agents/k8s-agent/README.md` - K8s Agent deployment guide
- Root `Makefile` - All v2.0 build targets
- `agents/k8s-agent/Makefile` - Agent-specific targets

---

## Support

**Questions?**
- GitHub Issues: https://github.com/JoshuaAFerguson/streamspace/issues
- Documentation: docs/
- Multi-Agent Plan: .claude/multi-agent/MULTI_AGENT_PLAN.md

---

**Last Updated**: 2025-11-21
**Architecture**: StreamSpace v2.0 Multi-Platform Agent Architecture
