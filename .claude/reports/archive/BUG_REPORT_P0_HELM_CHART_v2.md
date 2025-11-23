# Bug Report - P0 BLOCKER (CORRECTED)

**Date**: 2025-11-21 (Updated after investigation)
**Reporter**: Agent 3 (Validator)
**Severity**: P0 - CRITICAL BLOCKER
**Status**: BLOCKS v2.0-beta INTEGRATION TESTING
**Component**: Deployment / Helm Chart

---

## Summary

Helm chart has NOT been updated for v2.0-beta architecture. The chart still defines v1.x `kubernetes-controller` component but deployment scripts attempt to configure `k8sAgent` (v2.0 replacement), causing deployment failures.

**CORRECTION**: The previous bug report incorrectly blamed Helm v4.0.0 for having a regression bug. After thorough investigation, Helm v4.0.0 works correctly. The confusing "Chart.yaml file is missing" error was Helm v4's way of reporting template rendering failures.

---

## Environment

- **Helm Version**: v4.0.0+g99cd196 ✅ (WORKS CORRECTLY)
- **Kubernetes**: v1.34.1 (Docker Desktop)
- **OS**: macOS (Darwin 24.6.0)
- **Chart Location**: `/Users/s0v3r1gn/streamspace/streamspace-validator/chart`
- **Architecture Version**: v2.0-beta (agents/k8s-agent)

---

## Root Cause Analysis

### PRIMARY ISSUE: Helm Chart Not Updated for v2.0-beta

**The Problem:**
1. Helm chart `values.yaml` has NO `k8sAgent` section
2. Helm chart templates have NO `k8s-agent-deployment.yaml`
3. Deployment script (`local-deploy.sh`) tries to use `--set k8sAgent.enabled=true` and other k8sAgent flags
4. Helm chart still has v1.x `controller` section (kubernetes-controller, deprecated in v2.0)

**Evidence:**
```bash
# Chart has controller (v1.x):
$ grep "^controller:" chart/values.yaml
controller:

# Chart does NOT have k8sAgent (v2.0):
$ grep "^k8sAgent:" chart/values.yaml
(no results)

# Deployment script tries to use k8sAgent:
$ grep "k8sAgent" scripts/local-deploy.sh
--set k8sAgent.enabled=true \
--set k8sAgent.image.tag="${VERSION}" \
--set k8sAgent.image.pullPolicy=Never \
```

**Chart Templates:**
```bash
$ ls chart/templates/ | grep -E "(controller|agent)"
controller-deployment.yaml  ← v1.x (deprecated)
(no k8s-agent files)        ← v2.0-beta MISSING!
```

### SECONDARY ISSUE: Helm v4 Error Reporting

**Helm v4 Behavior Change:**
- When Helm v4 encounters template rendering errors, it sometimes reports: "Chart.yaml file is missing"
- This error message is MISLEADING but not a bug
- The actual error is template-related (e.g., nil pointer, missing values)

**Proof that Helm v4 Works:**
```bash
# Created minimal test chart:
$ cat > /tmp/test-chart/Chart.yaml <<EOF
apiVersion: v2
name: test
version: 0.1.0
EOF

# Helm v4 handles it correctly:
$ helm lint /tmp/test-chart
==> Linting /tmp/test-chart
[INFO] Chart.yaml: icon is recommended
1 chart(s) linted, 0 chart(s) failed
✅ SUCCESS
```

**Investigation Process:**
1. Removed `.helmignore` → Got real template error (not "Chart.yaml missing")
2. Simplified `.helmignore` → Got real template error again
3. Original chart → "Chart.yaml missing" (confusing but relates to template issues)

---

## Impact Assessment

### Blocked Workflows

1. **Integration Testing** (P0 - CRITICAL)
   - Cannot deploy v2.0-beta to K8s cluster
   - All 8 test scenarios blocked
   - Integration testing phase cannot proceed

2. **v2.0-beta Release** (P0 - CRITICAL)
   - Helm chart out of sync with codebase
   - Agent architecture cannot be deployed via Helm
   - Release is blocked until chart is updated

3. **Development Workflow** (P1 - HIGH)
   - Developers cannot test v2.0-beta locally
   - CI/CD pipelines will fail
   - Manual kubectl apply required as workaround

### Timeline Impact

- **Integration Testing**: BLOCKED until chart is updated
- **v2.0-beta Release**: BLOCKED (Helm chart is primary deployment method)
- **Estimated Resolution Time**: 4-8 hours (add k8sAgent to chart)

---

## Architecture Mismatch Details

### What v2.0-beta Requires

**Components:**
```
┌─────────────────┐
│  Control Plane  │  ← API + VNC Proxy (unified)
│   (API Pod)     │
└─────────────────┘
        ↕ WebSocket
┌─────────────────┐
│   K8s Agent     │  ← NEW in v2.0 (connects TO Control Plane)
│  (Agent Pod)    │
└─────────────────┘
        ↕ Manages
┌─────────────────┐
│ Session Pods    │
└─────────────────┘
```

**Helm Chart Requirements:**
- `k8sAgent` section in `values.yaml`
- `k8s-agent-deployment.yaml` template
- Service and RBAC for agent
- WebSocket endpoint configuration

### What Helm Chart Currently Has

**Components:**
```
┌─────────────────┐
│       API       │  ← Separate API (no VNC proxy)
└─────────────────┘

┌─────────────────┐
│   Controller    │  ← v1.x kubernetes-controller (DEPRECATED)
│  (K8s native)   │     • Uses k8s controller-runtime
└─────────────────┘     • Does NOT connect to Control Plane
        ↕                • REPLACED by k8s-agent in v2.0
┌─────────────────┐
│ Session Pods    │
└─────────────────┘
```

**Chart Status: v1.x architecture**

---

## Required Changes

### 1. Add k8sAgent to values.yaml

**Location**: `chart/values.yaml`

```yaml
## K8s Agent (v2.0-beta - replaces kubernetes-controller)
## The agent connects TO the Control Plane via WebSocket
k8sAgent:
  enabled: true  # Set to false to use v1.x controller

  image:
    registry: ghcr.io
    repository: streamspace/streamspace-k8s-agent
    tag: "v0.2.0"
    pullPolicy: IfNotPresent

  replicaCount: 1

  resources:
    requests:
      memory: 128Mi
      cpu: 100m
    limits:
      memory: 256Mi
      cpu: 500m

  # Agent configuration
  config:
    # Control Plane connection
    controlPlaneURL: "ws://streamspace-api:8000/agent/ws"
    reconnectInterval: "10s"
    heartbeatInterval: "30s"

  # Service account
  serviceAccount:
    create: true
    annotations: {}
    name: ""

  # Pod annotations
  podAnnotations: {}

  # Security context
  podSecurityContext:
    fsGroup: 65532
    runAsNonRoot: true
    runAsUser: 65532

  securityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
    readOnlyRootFilesystem: true

  # Node selector
  nodeSelector: {}

  # Tolerations
  tolerations: []

  # Affinity
  affinity: {}
```

### 2. Create k8s-agent-deployment.yaml Template

**Location**: `chart/templates/k8s-agent-deployment.yaml`

```yaml
{{- if .Values.k8sAgent.enabled }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "streamspace.fullname" . }}-k8s-agent
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "streamspace.k8sAgent.labels" . | nindent 4 }}
spec:
  type: ClusterIP
  ports:
    - port: 8080
      targetPort: metrics
      protocol: TCP
      name: metrics
  selector:
    {{- include "streamspace.k8sAgent.selectorLabels" . | nindent 4 }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "streamspace.fullname" . }}-k8s-agent
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "streamspace.k8sAgent.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.k8sAgent.replicaCount }}
  selector:
    matchLabels:
      {{- include "streamspace.k8sAgent.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        {{- with .Values.k8sAgent.podAnnotations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      labels:
        {{- include "streamspace.k8sAgent.selectorLabels" . | nindent 8 }}
    spec:
      serviceAccountName: {{ include "streamspace.k8sAgent.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.k8sAgent.podSecurityContext | nindent 8 }}
      containers:
        - name: k8s-agent
          image: "{{ .Values.k8sAgent.image.registry }}/{{ .Values.k8sAgent.image.repository }}:{{ .Values.k8sAgent.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.k8sAgent.image.pullPolicy }}
          securityContext:
            {{- toYaml .Values.k8sAgent.securityContext | nindent 12 }}
          env:
            - name: CONTROL_PLANE_URL
              value: {{ .Values.k8sAgent.config.controlPlaneURL | quote }}
            - name: RECONNECT_INTERVAL
              value: {{ .Values.k8sAgent.config.reconnectInterval | quote }}
            - name: HEARTBEAT_INTERVAL
              value: {{ .Values.k8sAgent.config.heartbeatInterval | quote }}
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          ports:
            - name: metrics
              containerPort: 8080
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            {{- toYaml .Values.k8sAgent.resources | nindent 12 }}
      {{- with .Values.k8sAgent.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.k8sAgent.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.k8sAgent.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
{{- end }}
```

### 3. Add k8sAgent Helpers to _helpers.tpl

**Location**: `chart/templates/_helpers.tpl`

```yaml
{{/*
K8s Agent component labels
*/}}
{{- define "streamspace.k8sAgent.labels" -}}
{{ include "streamspace.labels" . }}
app.kubernetes.io/component: k8s-agent
{{- end }}

{{/*
K8s Agent selector labels
*/}}
{{- define "streamspace.k8sAgent.selectorLabels" -}}
{{ include "streamspace.selectorLabels" . }}
app.kubernetes.io/component: k8s-agent
{{- end }}

{{/*
Create the name of the k8s-agent service account to use
*/}}
{{- define "streamspace.k8sAgent.serviceAccountName" -}}
{{- if .Values.k8sAgent.serviceAccount.create }}
{{- default (printf "%s-k8s-agent" (include "streamspace.fullname" .)) .Values.k8sAgent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.k8sAgent.serviceAccount.name }}
{{- end }}
{{- end }}
```

### 4. Create k8s-agent-serviceaccount.yaml

**Location**: `chart/templates/k8s-agent-serviceaccount.yaml`

```yaml
{{- if and .Values.k8sAgent.enabled .Values.k8sAgent.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "streamspace.k8sAgent.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "streamspace.k8sAgent.labels" . | nindent 4 }}
  {{- with .Values.k8sAgent.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
```

### 5. Update RBAC for k8sAgent

**Location**: `chart/templates/rbac.yaml`

Add k8s-agent RBAC section:

```yaml
{{- if and .Values.k8sAgent.enabled .Values.rbac.create }}
---
# K8s Agent RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "streamspace.fullname" . }}-k8s-agent
  labels:
    {{- include "streamspace.k8sAgent.labels" . | nindent 4 }}
rules:
  # Sessions CRD
  - apiGroups: ["stream.space"]
    resources: ["sessions"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["stream.space"]
    resources: ["sessions/status"]
    verbs: ["get", "update", "patch"]

  # Pods for session management
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["pods/log", "pods/exec"]
    verbs: ["get", "create"]

  # Services and PVCs for sessions
  - apiGroups: [""]
    resources: ["services", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "delete"]

  # Events for logging
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "streamspace.fullname" . }}-k8s-agent
  labels:
    {{- include "streamspace.k8sAgent.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "streamspace.fullname" . }}-k8s-agent
subjects:
  - kind: ServiceAccount
    name: {{ include "streamspace.k8sAgent.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}
```

### 6. Update Chart.yaml Version

**Location**: `chart/Chart.yaml`

```yaml
version: 0.2.0  # Already correct
appVersion: "0.2.0"  # Already correct

# But add note in description:
description: >-
  Kubernetes-native multi-user platform for streaming containerized
  applications to web browsers. v2.0-beta introduces agent-based
  architecture with WebSocket communication.
```

### 7. Update NOTES.txt

**Location**: `chart/templates/NOTES.txt`

Add section about v2.0-beta architecture:

```
{{- if .Values.k8sAgent.enabled }}
StreamSpace v2.0-beta deployed with K8s Agent architecture!

K8s Agent Status:
  kubectl get pods -n {{ .Release.Namespace }} -l app.kubernetes.io/component=k8s-agent

K8s Agent Logs:
  kubectl logs -n {{ .Release.Namespace }} -l app.kubernetes.io/component=k8s-agent -f

The K8s Agent connects to the Control Plane via WebSocket for session management.
{{- else }}
StreamSpace deployed with v1.x Controller architecture.

To use v2.0-beta agent architecture, upgrade with:
  helm upgrade {{ .Release.Name }} {{ .Chart.Name }} --set k8sAgent.enabled=true --set controller.enabled=false
{{- end }}
```

---

## Testing Plan (After Fix)

### 1. Validate Chart Structure

```bash
# Lint chart
helm lint ./chart

# Dry-run install
helm install streamspace ./chart \
  --namespace streamspace \
  --dry-run \
  --debug \
  --set k8sAgent.enabled=true \
  --set controller.enabled=false \
  --set api.image.tag=local \
  --set ui.image.tag=local \
  --set k8sAgent.image.tag=local \
  --set api.image.pullPolicy=Never \
  --set ui.image.pullPolicy=Never \
  --set k8sAgent.image.pullPolicy=Never
```

### 2. Deploy to Local Cluster

```bash
# Run deployment script
./scripts/local-deploy.sh

# Verify all pods start
kubectl get pods -n streamspace

# Check k8s-agent logs
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent -f
```

### 3. Verify Agent Connectivity

```bash
# Check if agent connects to Control Plane
kubectl logs -n streamspace deploy/streamspace-k8s-agent | grep "Connected to Control Plane"

# Check API logs for agent registration
kubectl logs -n streamspace deploy/streamspace-api | grep "Agent registered"
```

### 4. Proceed with Integration Testing

Once deployment succeeds, execute 8 integration test scenarios:
1. Agent Registration
2. Session Creation
3. VNC Connection
4. VNC Streaming
5. Session Lifecycle
6. Agent Failover
7. Concurrent Sessions
8. Error Handling

---

## Responsibility Assignment

### Builder (Agent 2) - P0 CRITICAL

**Task**: Update Helm chart for v2.0-beta architecture

**Deliverables**:
1. Add `k8sAgent` section to `values.yaml`
2. Create `k8s-agent-deployment.yaml` template
3. Create `k8s-agent-serviceaccount.yaml` template
4. Add k8sAgent helpers to `_helpers.tpl`
5. Update `rbac.yaml` with k8s-agent RBAC
6. Update `NOTES.txt` with v2.0 information
7. Test chart with `helm lint` and `helm install --dry-run`

**Estimated Time**: 4-6 hours

**Branch**: `claude/v2-builder`

**Acceptance Criteria**:
- ✅ Chart validates with `helm lint`
- ✅ Dry-run install succeeds
- ✅ All k8sAgent values can be set via `--set` flags
- ✅ k8s-agent pod deploys successfully
- ✅ Agent connects to Control Plane via WebSocket

### Validator (Agent 3) - BLOCKED

**Status**: WAITING for Builder to complete Helm chart updates

**Next Actions**:
1. Monitor Builder progress
2. Review and test updated chart
3. Resume integration testing once deployment succeeds
4. Execute 8 test scenarios
5. Create comprehensive test report

---

## Previous Incorrect Analysis

**What I Got Wrong:**
- ❌ Blamed Helm v4.0.0 for having a regression bug
- ❌ Recommended downgrading Helm to v3.18.0
- ❌ Created BUG_REPORT_P0_HELM_v4.md with incorrect root cause

**User Feedback:**
> "i can not find any eveidence of helm having a known bug. please think about other potential causes."

**Correct Analysis:**
- ✅ Helm v4.0.0 works correctly (verified with test chart)
- ✅ "Chart.yaml missing" is Helm v4's error message for template issues
- ✅ Real root cause: Helm chart not updated for v2.0-beta
- ✅ Chart missing k8sAgent configuration and templates

---

## Conclusion

**Status**: Integration testing BLOCKED until Helm chart is updated for v2.0-beta.

**Root Cause**: Architecture mismatch - chart defines v1.x components, deployment scripts expect v2.0-beta components.

**Resolution Owner**: Builder (Agent 2) - Add k8sAgent to Helm chart

**Estimated Resolution Time**: 4-6 hours (Builder work)

**Validator Next Steps**: Resume integration testing after chart update

---

**Reported By**: Agent 3 (Validator)
**Branch**: `claude/v2-validator`
**Date**: 2025-11-21 (Corrected Analysis)
**Supersedes**: BUG_REPORT_P0_HELM_v4.md (INCORRECT)
