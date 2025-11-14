# StreamSpace Testing Guide

Complete testing guide for StreamSpace using Docker Desktop with Kubernetes enabled.

**Last Updated**: 2025-11-14
**Platform**: Docker Desktop (macOS/Windows) with Kubernetes
**Version**: v0.2.0

---

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [Docker Desktop Setup](#docker-desktop-setup)
- [Pre-Testing Setup](#pre-testing-setup)
- [Component Testing](#component-testing)
- [Complete Testing Checklist](#complete-testing-checklist)
- [Troubleshooting](#troubleshooting)
- [Cleanup](#cleanup)

---

## Prerequisites

### Required Software

- [ ] **Docker Desktop** - Latest version with Kubernetes enabled
- [ ] **kubectl** - Kubernetes command-line tool
- [ ] **Helm 3.x** - Package manager for Kubernetes
- [ ] **Git** - For cloning the repository
- [ ] **curl** - For API testing (usually pre-installed)
- [ ] **Web Browser** - Chrome, Firefox, or Safari

### System Requirements

- **CPU**: 4+ cores (8+ recommended)
- **RAM**: 8GB minimum (16GB recommended)
- **Disk**: 20GB free space
- **OS**: macOS 10.15+, Windows 10/11 Pro

---

## Docker Desktop Setup

### Step 1: Install Docker Desktop

Download and install from [https://www.docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop)

### Step 2: Enable Kubernetes

1. Open Docker Desktop
2. Go to **Settings** → **Kubernetes**
3. Check **Enable Kubernetes**
4. Click **Apply & Restart**
5. Wait for Kubernetes to start (green indicator)

### Step 3: Configure Resources

Go to **Settings** → **Resources**:

```
CPUs: 4 (or more)
Memory: 8GB (or more)
Swap: 2GB
Disk: 60GB
```

Click **Apply & Restart**

### Step 4: Verify Kubernetes

```bash
# Check kubectl works
kubectl version --client

# Check cluster is running
kubectl cluster-info

# Expected output:
# Kubernetes control plane is running at https://kubernetes.docker.internal:6443
```

### Step 5: Set kubectl Context

```bash
# Use docker-desktop context
kubectl config use-context docker-desktop

# Verify current context
kubectl config current-context
# Should show: docker-desktop
```

---

## Pre-Testing Setup

### 1. Clone Repository

```bash
git clone https://github.com/JoshuaAFerguson/streamspace.git
cd streamspace
```

### 2. Install Storage Provisioner (NFS)

Docker Desktop doesn't include NFS by default. Use local-path-provisioner:

```bash
# Install local-path-provisioner
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml

# Verify it's running
kubectl get pods -n local-path-storage

# Set as default storage class
kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

# Verify
kubectl get storageclass
```

### 3. Create Testing Namespace

```bash
kubectl create namespace streamspace
kubectl config set-context --current --namespace=streamspace
```

### 4. Apply CRDs

```bash
# Apply Session CRD
kubectl apply -f manifests/crds/session.yaml

# Apply Template CRD
kubectl apply -f manifests/crds/template.yaml

# Verify CRDs are installed
kubectl get crds | grep stream.space
```

### 5. Install StreamSpace

Create a test values file:

```bash
cat > test-values.yaml <<EOF
controller:
  replicaCount: 1
  config:
    ingressDomain: "streamspace.local"
    ingressClass: "nginx"

api:
  replicaCount: 1

ui:
  replicaCount: 1

postgresql:
  enabled: true
  auth:
    postgresPassword: "testpassword"
    database: "streamspace"

ingress:
  enabled: false  # We'll use port-forward for testing

monitoring:
  enabled: true
EOF
```

Install with Helm:

```bash
helm install streamspace ./chart \
  --namespace streamspace \
  --values test-values.yaml \
  --timeout 10m
```

### 6. Wait for Pods to Start

```bash
# Watch pods until all are running
kubectl get pods -n streamspace -w

# Expected pods (press Ctrl+C when all Running):
# streamspace-controller-xxx    1/1     Running
# streamspace-api-xxx           1/1     Running
# streamspace-ui-xxx            1/1     Running
# postgresql-xxx                1/1     Running

# Check all pods are ready
kubectl get pods -n streamspace

# Check logs if any pod is not running
kubectl logs -n streamspace <pod-name>
```

---

## Component Testing

### Test 1: Controller

#### 1.1 Verify Controller is Running

```bash
# Check controller pod
kubectl get pods -n streamspace -l app=streamspace-controller

# Check controller logs
kubectl logs -n streamspace -l app=streamspace-controller --tail=50

# Expected: No errors, should show "Starting Controller"
```

#### 1.2 Check Prometheus Metrics

```bash
# Port forward to controller
kubectl port-forward -n streamspace svc/streamspace-controller 8080:8080 &

# Query metrics
curl http://localhost:8080/metrics | grep streamspace

# Expected metrics:
# streamspace_active_sessions_total
# streamspace_hibernated_sessions_total
# streamspace_session_starts_total

# Stop port forward
pkill -f "port-forward.*8080:8080"
```

#### 1.3 Check Controller RBAC

```bash
# Verify ServiceAccount exists
kubectl get serviceaccount -n streamspace streamspace-controller

# Verify ClusterRole exists
kubectl get clusterrole streamspace-controller

# Verify ClusterRoleBinding exists
kubectl get clusterrolebinding streamspace-controller
```

### Test 2: API Backend

#### 2.1 Verify API is Running

```bash
# Check API pod
kubectl get pods -n streamspace -l app=streamspace-api

# Check API logs
kubectl logs -n streamspace -l app=streamspace-api --tail=50

# Expected: Server started on port 8000
```

#### 2.2 Test API Endpoints

```bash
# Port forward to API
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &

# Test health endpoint
curl http://localhost:8000/health

# Expected: {"status":"ok"}

# Test sessions endpoint
curl http://localhost:8000/api/v1/sessions

# Expected: [] (empty array, no sessions yet)

# Test templates endpoint
curl http://localhost:8000/api/v1/templates

# Expected: Array of templates

# Stop port forward
pkill -f "port-forward.*8000:8000"
```

#### 2.3 Check Database Connection

```bash
# Port forward to PostgreSQL
kubectl port-forward -n streamspace svc/postgresql 5432:5432 &

# Test connection (requires psql client)
PGPASSWORD=testpassword psql -h localhost -U postgres -d streamspace -c "SELECT version();"

# Or check from API logs
kubectl logs -n streamspace -l app=streamspace-api | grep -i "database"

# Expected: "Database connected" or similar

# Stop port forward
pkill -f "port-forward.*5432:5432"
```

### Test 3: Web UI

#### 3.1 Verify UI is Running

```bash
# Check UI pod
kubectl get pods -n streamspace -l app=streamspace-ui

# Check UI logs
kubectl logs -n streamspace -l app=streamspace-ui --tail=50
```

#### 3.2 Access Web UI

```bash
# Port forward to UI
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80

# Open in browser
open http://localhost:3000
# Or navigate to: http://localhost:3000
```

**Manual UI Checks**:
- [ ] Login page loads
- [ ] Can enter demo credentials or skip auth
- [ ] Dashboard displays
- [ ] Navigation menu works
- [ ] No console errors (F12 → Console)

### Test 4: Templates

#### 4.1 Install Sample Templates

```bash
# Apply browser templates
kubectl apply -f manifests/templates/browsers/

# Verify templates are created
kubectl get templates -n streamspace

# Expected: firefox-browser, chromium-browser, etc.
```

#### 4.2 Verify Template Details

```bash
# Get firefox template
kubectl get template firefox-browser -n streamspace -o yaml

# Check required fields
kubectl get template firefox-browser -n streamspace -o jsonpath='{.spec.displayName}'

# Expected: "Firefox Web Browser"
```

### Test 5: Session Lifecycle

#### 5.1 Create a Test Session

```bash
# Create Firefox session
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: test-firefox
  namespace: streamspace
spec:
  user: testuser
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
EOF
```

#### 5.2 Verify Session Creation

```bash
# Wait for session to be ready (may take 2-3 minutes)
kubectl get session test-firefox -n streamspace -w

# Check session status
kubectl describe session test-firefox -n streamspace

# Expected status.phase: Running
```

#### 5.3 Verify Pod Creation

```bash
# Check session pod
kubectl get pods -n streamspace -l session=test-firefox

# Expected: ss-test-firefox-xxx  1/1  Running
```

#### 5.4 Verify Service Creation

```bash
# Check session service
kubectl get svc -n streamspace -l session=test-firefox

# Expected: ss-test-firefox-svc
```

#### 5.5 Verify PVC Creation

```bash
# Check user PVC
kubectl get pvc -n streamspace -l user=testuser

# Expected: home-testuser
```

#### 5.6 Test Session Hibernation

```bash
# Hibernate the session
kubectl patch session test-firefox -n streamspace \
  --type merge -p '{"spec":{"state":"hibernated"}}'

# Wait for deployment to scale down
kubectl get deployment -n streamspace -l session=test-firefox -w

# Expected replicas: 0/0

# Verify pod is terminated
kubectl get pods -n streamspace -l session=test-firefox

# Expected: No resources found (pod terminated)
```

#### 5.7 Test Session Wake-Up

```bash
# Wake the session
kubectl patch session test-firefox -n streamspace \
  --type merge -p '{"spec":{"state":"running"}}'

# Wait for deployment to scale up
kubectl get deployment -n streamspace -l session=test-firefox -w

# Expected replicas: 1/1

# Verify pod is running
kubectl get pods -n streamspace -l session=test-firefox

# Expected: ss-test-firefox-xxx  1/1  Running
```

#### 5.8 Test Session Deletion

```bash
# Delete the session
kubectl delete session test-firefox -n streamspace

# Verify all resources are cleaned up
kubectl get all -n streamspace -l session=test-firefox

# Expected: No resources found

# Note: PVC should remain (persistentHome: true)
kubectl get pvc -n streamspace -l user=testuser

# Expected: home-testuser still exists
```

### Test 6: Plugin System

#### 6.1 Access Plugin Catalog via UI

```bash
# Ensure UI port forward is running
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80

# Open http://localhost:3000/plugins/catalog
```

**Manual Checks**:
- [ ] Plugin catalog page loads
- [ ] Can search plugins
- [ ] Can filter by category
- [ ] Can filter by type
- [ ] Plugin cards display correctly

#### 6.2 Test Plugin API

```bash
# Port forward to API
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &

# Browse plugin catalog
curl http://localhost:8000/api/v1/plugins/catalog

# List installed plugins
curl http://localhost:8000/api/v1/plugins/installed

# List plugin repositories
curl http://localhost:8000/api/v1/plugins/repositories

# Stop port forward
pkill -f "port-forward.*8000:8000"
```

#### 6.3 Test Plugin Installation (via UI)

**Manual Steps**:
1. Navigate to Plugin Catalog
2. Find a test plugin
3. Click "Install"
4. Verify installation in "My Plugins"
5. Test enable/disable toggle
6. Test configuration editor
7. Test uninstall

### Test 7: User Management

#### 7.1 Access Admin Panel

```bash
# Ensure UI port forward is running
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80

# Open http://localhost:3000/admin/users
```

**Manual Checks**:
- [ ] Admin dashboard loads
- [ ] User list displays
- [ ] Can create new user
- [ ] Can edit user details
- [ ] Can set user quotas
- [ ] Can delete user

#### 7.2 Test User API

```bash
# Port forward to API
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &

# List users
curl http://localhost:8000/api/v1/users

# Create test user
curl -X POST http://localhost:8000/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser2",
    "fullName": "Test User 2",
    "email": "test2@example.com",
    "tier": "free"
  }'

# Get user details
curl http://localhost:8000/api/v1/users/testuser2

# Delete test user
curl -X DELETE http://localhost:8000/api/v1/users/testuser2

# Stop port forward
pkill -f "port-forward.*8000:8000"
```

### Test 8: Repository Sync

#### 8.1 Create Test Repository

```bash
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Repository
metadata:
  name: test-repo
  namespace: streamspace
spec:
  url: https://github.com/linuxserver/docker-firefox
  branch: master
  syncInterval: 1h
  enabled: true
EOF
```

#### 8.2 Trigger Manual Sync

```bash
# Port forward to API
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &

# Get repository ID
curl http://localhost:8000/api/v1/repositories

# Trigger sync (replace {id} with actual ID)
curl -X POST http://localhost:8000/api/v1/repositories/{id}/sync

# Check sync status
curl http://localhost:8000/api/v1/repositories/{id}

# Stop port forward
pkill -f "port-forward.*8000:8000"
```

### Test 9: WebSocket Real-Time Updates

#### 9.1 Test WebSocket Connection

```bash
# Port forward to API
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &

# Use wscat to test WebSocket (requires: npm install -g wscat)
# NOTE: ws://localhost is acceptable for local testing. Production uses wss://
wscat -c ws://localhost:8000/api/v1/ws/sessions

# Should receive periodic session updates every 3 seconds

# In another terminal, create a session to trigger updates
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: websocket-test
  namespace: streamspace
spec:
  user: wstest
  template: firefox-browser
  state: running
EOF

# Verify update received in wscat terminal

# Cleanup
kubectl delete session websocket-test -n streamspace
pkill -f "port-forward.*8000:8000"
```

#### 9.2 Test UI Real-Time Updates

**Manual Steps**:
1. Open UI in browser (http://localhost:3000)
2. Navigate to Dashboard or Sessions page
3. In terminal, create/update/delete sessions
4. Verify UI updates automatically without refresh

### Test 10: Monitoring & Metrics

#### 10.1 Check ServiceMonitor

```bash
# Verify ServiceMonitor exists
kubectl get servicemonitor -n streamspace

# Expected: streamspace-controller
```

#### 10.2 Check PrometheusRule

```bash
# Verify alert rules exist
kubectl get prometheusrule -n streamspace

# View alert definitions
kubectl get prometheusrule -n streamspace -o yaml
```

#### 10.3 Test Metrics Endpoints

```bash
# Controller metrics
kubectl port-forward -n streamspace svc/streamspace-controller 8080:8080 &
curl http://localhost:8080/metrics | grep streamspace
pkill -f "port-forward.*8080:8080"

# API metrics (if exposed)
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &
curl http://localhost:8000/metrics 2>/dev/null | head -20
pkill -f "port-forward.*8000:8000"
```

---

## Complete Testing Checklist

### ✅ Infrastructure

- [ ] Docker Desktop installed and running
- [ ] Kubernetes enabled in Docker Desktop
- [ ] Resources allocated (4+ CPU, 8GB+ RAM)
- [ ] kubectl configured with docker-desktop context
- [ ] Helm 3.x installed
- [ ] Storage provisioner installed (local-path)
- [ ] Default storage class set

### ✅ Installation

- [ ] Repository cloned
- [ ] Namespace created (streamspace)
- [ ] Session CRD applied
- [ ] Template CRD applied
- [ ] CRDs verified with `kubectl get crds`
- [ ] Helm chart installed successfully
- [ ] All pods running (controller, api, ui, postgresql)
- [ ] No pod errors in logs
- [ ] Services created
- [ ] ConfigMaps created

### ✅ Controller Component

- [ ] Controller pod running
- [ ] Controller logs show no errors
- [ ] Prometheus metrics endpoint accessible
- [ ] Metrics being exported (streamspace_*)
- [ ] ServiceAccount created
- [ ] ClusterRole created
- [ ] ClusterRoleBinding created
- [ ] Leader election working (if HA enabled)
- [ ] CRD watch loops active
- [ ] Reconciliation working

### ✅ API Backend Component

- [ ] API pod running
- [ ] API logs show "Server started"
- [ ] Health endpoint returns OK
- [ ] Database connection successful
- [ ] PostgreSQL pod running
- [ ] Can query sessions endpoint
- [ ] Can query templates endpoint
- [ ] Can query users endpoint
- [ ] Can query plugins endpoint
- [ ] Can query repositories endpoint
- [ ] WebSocket endpoint accessible
- [ ] JWT authentication working (if enabled)

### ✅ Web UI Component

- [ ] UI pod running
- [ ] UI accessible via port-forward
- [ ] Login page loads
- [ ] Dashboard displays
- [ ] Navigation menu works
- [ ] Sessions page loads
- [ ] Templates page loads
- [ ] Plugin catalog loads
- [ ] Installed plugins page loads
- [ ] Admin dashboard accessible
- [ ] Users page loads (admin)
- [ ] Groups page loads (admin)
- [ ] Plugin management page loads (admin)
- [ ] No browser console errors
- [ ] Responsive on mobile (test with DevTools)

### ✅ Template System

- [ ] Can list templates with kubectl
- [ ] Can view template details
- [ ] Browser templates installed
- [ ] Development templates available
- [ ] Design templates available
- [ ] Template CRD validation working
- [ ] Templates display in UI catalog
- [ ] Can filter templates by category
- [ ] Can search templates
- [ ] Template details modal works

### ✅ Session Lifecycle

- [ ] Can create session via kubectl
- [ ] Session CRD is created
- [ ] Deployment is created
- [ ] Pod starts successfully
- [ ] Service is created
- [ ] Ingress is created (if enabled)
- [ ] PVC is created for user
- [ ] Session status updates to Running
- [ ] Session URL is set in status
- [ ] Can view session in UI
- [ ] Can connect to session (if ingress enabled)

### ✅ Hibernation System

- [ ] Can hibernate session (set state: hibernated)
- [ ] Deployment scales to 0 replicas
- [ ] Pod is terminated
- [ ] Session status updates to Hibernated
- [ ] Service remains (not deleted)
- [ ] PVC remains (not deleted)
- [ ] Can wake session (set state: running)
- [ ] Deployment scales to 1 replica
- [ ] Pod starts again
- [ ] Session status updates to Running
- [ ] Data persists after wake

### ✅ Session Management

- [ ] Can update session resources
- [ ] Can update session labels
- [ ] Can update idleTimeout
- [ ] Can delete session
- [ ] Owner references working (cascading delete)
- [ ] Deployment deleted on session delete
- [ ] Service deleted on session delete
- [ ] Pod deleted on session delete
- [ ] PVC persists (if persistentHome: true)
- [ ] Can create multiple sessions per user
- [ ] Can create sessions for different users

### ✅ Plugin System Backend

- [ ] Plugin database tables created
- [ ] repositories table exists
- [ ] catalog_plugins table exists
- [ ] installed_plugins table exists
- [ ] plugin_ratings table exists
- [ ] Can query plugin catalog API
- [ ] Can install plugin via API
- [ ] Can list installed plugins
- [ ] Can enable/disable plugin
- [ ] Can configure plugin
- [ ] Can uninstall plugin
- [ ] Can rate plugin
- [ ] Plugin config JSON storage works
- [ ] Plugin manifest validation works

### ✅ Plugin System UI

- [ ] Plugin catalog page loads
- [ ] Can browse plugins
- [ ] Can search plugins
- [ ] Can filter by category
- [ ] Can filter by type (extension, webhook, etc.)
- [ ] Can sort plugins
- [ ] Plugin cards display correctly
- [ ] Plugin type colors correct
- [ ] Plugin detail modal opens
- [ ] Tabs work (Details/Reviews)
- [ ] Can view plugin permissions
- [ ] Permission risk indicators show (low/medium/high)
- [ ] Can install plugin from catalog
- [ ] Installed plugins page loads
- [ ] Can enable/disable plugin
- [ ] Configuration form generates from schema
- [ ] JSON editor works
- [ ] Form/JSON sync works bidirectionally
- [ ] Can uninstall plugin
- [ ] Skeleton loaders display during loading
- [ ] Empty states show correctly
- [ ] Admin plugin page works

### ✅ User Management

- [ ] Can create user via UI
- [ ] Can create user via API
- [ ] User list displays in admin panel
- [ ] Can edit user details
- [ ] Can set user quotas (CPU, memory, sessions, storage)
- [ ] Can assign user to groups
- [ ] Can delete user
- [ ] User sessions show in user detail
- [ ] User PVC created automatically
- [ ] User authentication works (if enabled)
- [ ] User roles enforced (user vs admin)

### ✅ Group Management

- [ ] Can create group via UI
- [ ] Group list displays in admin panel
- [ ] Can add members to group
- [ ] Can remove members from group
- [ ] Can set group quotas
- [ ] Can edit group details
- [ ] Can delete group
- [ ] Group members inherit quotas

### ✅ Repository Sync

- [ ] Can create repository CRD
- [ ] Repository shows in UI
- [ ] Can trigger manual sync
- [ ] Sync status updates
- [ ] Templates populated from repository
- [ ] Sync interval respected
- [ ] Can sync multiple repositories
- [ ] Can delete repository
- [ ] Git authentication works (if configured)
- [ ] Webhook sync works (if configured)

### ✅ Real-Time Updates (WebSocket)

- [ ] WebSocket connection established
- [ ] Session updates broadcast every 3s
- [ ] Cluster metrics broadcast every 5s
- [ ] UI receives updates
- [ ] Dashboard updates automatically
- [ ] Sessions page updates automatically
- [ ] Connection status indicator works
- [ ] Reconnection works after disconnect
- [ ] Exponential backoff working

### ✅ Monitoring & Observability

- [ ] ServiceMonitor created
- [ ] PrometheusRule created
- [ ] Controller metrics exported
- [ ] Active sessions metric working
- [ ] Hibernated sessions metric working
- [ ] Session starts counter working
- [ ] Resource usage metrics working
- [ ] Grafana dashboard available (if enabled)
- [ ] Alert rules defined
- [ ] Audit logs generated (if enabled)

### ✅ Storage & Persistence

- [ ] StorageClass available
- [ ] Can create PVC
- [ ] PVC binds to PV
- [ ] User home directories persist
- [ ] Data survives session restart
- [ ] Data survives hibernation/wake cycle
- [ ] Multiple sessions share same PVC
- [ ] Storage quotas enforced
- [ ] Can backup/restore PVCs

### ✅ Networking

- [ ] Services have ClusterIP
- [ ] Can access services from pods
- [ ] Port-forward works for all services
- [ ] Ingress created (if enabled)
- [ ] Session URLs correct
- [ ] DNS resolution works
- [ ] Network policies work (if enabled)
- [ ] Cross-namespace communication (if needed)

### ✅ Security

- [ ] RBAC roles configured
- [ ] ServiceAccounts created
- [ ] ClusterRole bindings correct
- [ ] Pod security context set
- [ ] Secrets created for credentials
- [ ] TLS enabled (if configured)
- [ ] JWT tokens working (if enabled)
- [ ] Plugin permissions enforced
- [ ] User permissions enforced
- [ ] Audit logging (if enabled)

### ✅ API Endpoints

**Session Endpoints:**
- [ ] GET /api/v1/sessions
- [ ] POST /api/v1/sessions
- [ ] GET /api/v1/sessions/:id
- [ ] PUT /api/v1/sessions/:id
- [ ] DELETE /api/v1/sessions/:id
- [ ] POST /api/v1/sessions/:id/connect
- [ ] POST /api/v1/sessions/:id/disconnect
- [ ] POST /api/v1/sessions/:id/heartbeat

**Template Endpoints:**
- [ ] GET /api/v1/templates
- [ ] GET /api/v1/templates/:id
- [ ] POST /api/v1/templates
- [ ] DELETE /api/v1/templates/:id
- [ ] GET /api/v1/catalog/templates

**User Endpoints:**
- [ ] GET /api/v1/users
- [ ] POST /api/v1/users
- [ ] GET /api/v1/users/:username
- [ ] PUT /api/v1/users/:username
- [ ] DELETE /api/v1/users/:username

**Plugin Endpoints:**
- [ ] GET /api/v1/plugins/catalog
- [ ] POST /api/v1/plugins/install
- [ ] GET /api/v1/plugins/installed
- [ ] POST /api/v1/plugins/:id/enable
- [ ] POST /api/v1/plugins/:id/disable
- [ ] PUT /api/v1/plugins/:id/config
- [ ] DELETE /api/v1/plugins/:id
- [ ] POST /api/v1/plugins/:id/rate
- [ ] GET /api/v1/plugins/repositories
- [ ] POST /api/v1/plugins/repositories
- [ ] POST /api/v1/plugins/repositories/:id/sync

**Repository Endpoints:**
- [ ] GET /api/v1/repositories
- [ ] POST /api/v1/repositories
- [ ] POST /api/v1/repositories/:id/sync
- [ ] DELETE /api/v1/repositories/:id

**WebSocket Endpoints:**
- [ ] WS /api/v1/ws/sessions
- [ ] WS /api/v1/ws/cluster
- [ ] WS /api/v1/ws/logs/:namespace/:pod

### ✅ Error Handling

- [ ] Invalid session creation rejected
- [ ] Invalid template creation rejected
- [ ] Quota exceeded errors shown
- [ ] Resource limits enforced
- [ ] Validation errors clear
- [ ] API errors return correct status codes
- [ ] UI shows error messages
- [ ] Logs show detailed errors
- [ ] Failed pods restart correctly
- [ ] Database connection retries work

### ✅ Performance

- [ ] UI loads in < 3 seconds
- [ ] API responds in < 500ms
- [ ] Session creation completes in < 2 minutes
- [ ] Hibernation completes in < 30 seconds
- [ ] Wake completes in < 1 minute
- [ ] WebSocket latency < 100ms
- [ ] Multiple concurrent sessions work
- [ ] Resource usage acceptable

### ✅ Documentation

- [ ] README is accurate
- [ ] CLAUDE.md is helpful
- [ ] Getting started guide works
- [ ] API documentation matches reality
- [ ] Plugin development guide clear
- [ ] Helm chart README accurate
- [ ] Architecture docs current
- [ ] Troubleshooting guide helpful

---

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n streamspace

# Describe pod to see events
kubectl describe pod <pod-name> -n streamspace

# Check logs
kubectl logs <pod-name> -n streamspace

# Common issues:
# - Image pull errors: Check image name and registry
# - Resource limits: Check node capacity
# - PVC mount errors: Check storage provisioner
```

### CRD Issues

```bash
# Verify CRDs installed
kubectl get crds | grep stream.space

# Check CRD definition
kubectl get crd sessions.stream.space -o yaml

# Reinstall CRDs
kubectl apply -f manifests/crds/
```

### Database Connection Errors

```bash
# Check PostgreSQL pod
kubectl get pods -n streamspace -l app=postgresql

# Check PostgreSQL logs
kubectl logs -n streamspace -l app=postgresql

# Verify password matches in API config
kubectl get configmap -n streamspace streamspace-api-config -o yaml
```

### API Not Responding

```bash
# Check API pod
kubectl get pods -n streamspace -l app=streamspace-api

# Check API logs
kubectl logs -n streamspace -l app=streamspace-api --tail=100

# Restart API pod
kubectl delete pod -n streamspace -l app=streamspace-api
```

### UI Not Loading

```bash
# Check UI pod
kubectl get pods -n streamspace -l app=streamspace-ui

# Check UI logs
kubectl logs -n streamspace -l app=streamspace-ui

# Check nginx config (if applicable)
kubectl exec -n streamspace -it <ui-pod> -- cat /etc/nginx/nginx.conf

# Clear browser cache and retry
```

### Session Not Creating

```bash
# Check session CRD
kubectl get session <session-name> -n streamspace -o yaml

# Check controller logs
kubectl logs -n streamspace -l app=streamspace-controller --tail=100

# Check events
kubectl get events -n streamspace --sort-by=.metadata.creationTimestamp

# Verify template exists
kubectl get template <template-name> -n streamspace
```

### Storage Issues

```bash
# Check storage class
kubectl get storageclass

# Check PVC status
kubectl get pvc -n streamspace

# Describe PVC for events
kubectl describe pvc <pvc-name> -n streamspace

# Check provisioner logs
kubectl logs -n local-path-storage -l app=local-path-provisioner
```

---

## Cleanup

### Remove Test Resources

```bash
# Delete test sessions
kubectl delete sessions --all -n streamspace

# Delete test templates (optional)
kubectl delete templates --all -n streamspace

# Delete test repositories
kubectl delete repositories --all -n streamspace
```

### Uninstall StreamSpace

```bash
# Uninstall Helm release
helm uninstall streamspace -n streamspace

# Delete CRDs
kubectl delete -f manifests/crds/

# Delete namespace
kubectl delete namespace streamspace

# Delete storage class (if desired)
kubectl delete storageclass local-path
```

### Reset Docker Desktop Kubernetes

If you need to start fresh:

1. Open Docker Desktop
2. Go to **Settings** → **Kubernetes**
3. Click **Reset Kubernetes Cluster**
4. Confirm reset
5. Wait for Kubernetes to restart

---

## Test Results Template

Use this template to document your test results:

```markdown
# StreamSpace Test Results

**Date**: YYYY-MM-DD
**Tester**: Your Name
**Environment**: Docker Desktop vX.X.X / macOS|Windows
**StreamSpace Version**: vX.X.X

## Summary

- Total Tests: X
- Passed: X
- Failed: X
- Skipped: X

## Component Results

### Controller
- Status: ✅ PASS / ❌ FAIL
- Notes:

### API Backend
- Status: ✅ PASS / ❌ FAIL
- Notes:

### Web UI
- Status: ✅ PASS / ❌ FAIL
- Notes:

### Session Lifecycle
- Status: ✅ PASS / ❌ FAIL
- Notes:

### Plugin System
- Status: ✅ PASS / ❌ FAIL
- Notes:

## Issues Found

1. [Issue description]
   - Severity: High/Medium/Low
   - Steps to reproduce:
   - Expected behavior:
   - Actual behavior:

## Recommendations

1. [Recommendation]

## Screenshots

[Attach relevant screenshots]
```

---

## Next Steps

After completing testing:

1. **Document Issues** - Create GitHub issues for any bugs found
2. **Update Documentation** - Fix any inaccuracies in docs
3. **Share Results** - Post test results in GitHub Discussions
4. **Production Planning** - Plan production deployment based on findings
5. **Performance Tuning** - Optimize based on test observations

---

## Additional Resources

- [Docker Desktop Documentation](https://docs.docker.com/desktop/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [StreamSpace GitHub](https://github.com/JoshuaAFerguson/streamspace)
- [StreamSpace Issues](https://github.com/JoshuaAFerguson/streamspace/issues)

---

**Happy Testing!** 🧪

If you encounter any issues not covered in this guide, please:
1. Check the logs for detailed error messages
2. Search GitHub Issues for similar problems
3. Create a new issue with full details and logs
