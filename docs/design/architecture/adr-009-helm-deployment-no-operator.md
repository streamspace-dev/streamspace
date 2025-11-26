# ADR-009: Helm Chart Deployment (No Kubernetes Operator for v2.0)
- **Status**: Accepted
- **Date**: 2025-11-26
- **Owners**: Agent 1 (Architect)
- **Implementation**: chart/ (Helm chart)

## Context

StreamSpace uses Kubernetes Custom Resource Definitions (CRDs):
- `Session` (stream.space/v1alpha1)
- `Template` (stream.space/v1alpha1)
- `TemplateRepository` (stream.space/v1alpha1)
- `Connection` (stream.space/v1alpha1)

Typically, CRDs require custom controllers (Kubernetes Operators) to watch and reconcile resources. However, v2.0-beta has CRDs but **no Operator**.

**Question**: Should we build a Kubernetes Operator for v2.0, or is Helm chart deployment sufficient?

## Decision

**Deploy via Helm chart only; do NOT build Kubernetes Operator for v2.0.**

### Rationale

v2.0 architecture uses **Database as Source of Truth** (ADR-006):
- Database is canonical (not K8s)
- Agents create CRDs (not Control Plane)
- API reads from database (not K8s)
- No reconciliation loop needed

**CRDs are "projections" of database state** - They exist for K8s-native tooling (`kubectl get sessions`) but are not authoritative.

### Architecture Without Operator

```
┌──────────────┐
│  PostgreSQL  │ ← Source of truth
└──────┬───────┘
       │
       │ All reads/writes
       ↓
┌──────────────┐
│  API Server  │ ← No K8s reconciliation
└──────┬───────┘
       │
       │ Commands via WebSocket
       ↓
┌──────────────┐
│    Agents    │ ← Create/manage CRDs
└──────┬───────┘
       │
       │ Create CRDs as needed
       ↓
┌──────────────┐
│   K8s CRDs   │ ← Projections (not canonical)
└──────────────┘
```

**No Operator Needed Because:**
1. **No Reconciliation**: Database is source of truth (not CRDs)
2. **Agent Manages**: Agents create/update/delete CRDs directly
3. **No Drift Detection**: Don't care if CRDs manually modified (database wins)
4. **Simpler**: Fewer components, easier deployment

## Alternatives Considered

### Alternative A: Helm + Operator (Typical K8s Pattern) ❌

**Pros:**
- Standard K8s pattern (CRDs + Operator)
- Reconciliation loop handles drift
- Declarative desired state

**Cons:**
- Extra complexity (Operator code, deployment, RBAC)
- Not needed (database is source of truth)
- Would conflict with agent-managed CRDs

**Verdict:** Rejected - Unnecessary for v2.0 architecture

### Alternative B: Helm Only (v2.0 Choice) ✅

**Pros:**
- Simpler (no Operator code)
- Fewer RBAC permissions
- Easier to understand
- Aligns with database-first architecture

**Cons:**
- CRDs may become stale (no reconciliation)
- Manual cleanup if agent crashes

**Verdict:** Accepted - Sufficient for v2.0

### Alternative C: Operator-Only (No Helm) ❌

**Pros:**
- Fully K8s-native

**Cons:**
- Harder for users (Operator development complex)
- Doesn't fit v2.0 architecture (database-first)

**Verdict:** Rejected

## Implementation

### Helm Chart Structure

```
chart/
├── Chart.yaml              # Helm chart metadata
├── values.yaml             # Default values
├── crds/                   # CRD definitions (installed first)
│   ├── stream.space_sessions.yaml
│   ├── stream.space_templates.yaml
│   ├── stream.space_templaterepositories.yaml
│   └── stream.space_connections.yaml
├── templates/              # K8s manifests
│   ├── api-deployment.yaml
│   ├── ui-deployment.yaml
│   ├── k8s-agent-deployment.yaml
│   ├── postgresql.yaml
│   ├── redis.yaml
│   ├── rbac.yaml
│   └── ...
└── README.md
```

### CRD Lifecycle

**Who Creates CRDs?**
- **Helm**: Installs CRD definitions (`chart/crds/`)
- **Agent**: Creates CRD instances (Session, Template)

**Who Manages CRD Instances?**
- **Agent**: Creates Session CRDs when provisioning
- **Agent**: Deletes Session CRDs when terminating
- **No Operator**: No reconciliation loop

**What if CRD orphaned (agent crashes)?**
- **Current**: Manual cleanup (v2.0)
- **Future**: Cleanup job or reconciliation (v2.1+)

### Deployment

**Install StreamSpace**:
```bash
helm install streamspace ./chart \
  --set postgresql.enabled=true \
  --set redis.enabled=true \
  --set api.replicas=2
```

**What Helm Does:**
1. Install CRD definitions (chart/crds/)
2. Create namespace
3. Deploy PostgreSQL, Redis
4. Deploy API, UI, K8s Agent
5. Create RBAC (ServiceAccount, Role, RoleBinding)
6. Create Ingress

**What Helm Does NOT Do:**
- Does NOT run Operator (none exists)
- Does NOT reconcile CRDs (no controller)
- Does NOT watch CRDs (agents handle lifecycle)

## Consequences

### Positive ✅

1. **Simpler Deployment**
   - Fewer components (no Operator)
   - Faster installation
   - Easier troubleshooting

2. **Fewer RBAC Permissions**
   - No Operator = no cluster-wide permissions
   - Agents need minimal RBAC (namespace-scoped)

3. **Easier to Understand**
   - Clear responsibility: Database canonical, agents manage CRDs
   - No complex reconciliation logic

4. **Multi-Platform Ready**
   - Helm chart works for K8s
   - Docker deployment doesn't need K8s (no Operator dependency)

### Negative ⚠️

1. **No Reconciliation**
   - If agent crashes, orphaned CRDs not cleaned up
   - Manual intervention required (or cleanup job)

2. **CRDs May Be Stale**
   - Database updated, CRDs not synced
   - `kubectl get sessions` may show stale data

3. **No Drift Detection**
   - Manual CRD changes not detected/reverted
   - User must not manually edit CRDs

### Mitigation Strategies

1. **Cleanup Job** (v2.1):
   - CronJob runs daily: Delete orphaned CRDs
   - Query database, compare with K8s CRDs
   - Delete CRDs not in database

2. **Documentation**:
   - Warn users: Don't manually edit CRDs
   - Database is source of truth
   - Use API, not `kubectl`, for session management

3. **Future Operator** (v3.0?):
   - If reconciliation becomes essential
   - Operator syncs database → K8s CRDs
   - Optional (not required)

## When Would We Need an Operator?

**Scenarios where Operator would help:**
1. **Automated Cleanup**: Reconcile database ↔ CRDs, delete orphans
2. **Drift Detection**: Revert manual CRD changes
3. **Multi-Cluster**: Sync CRDs across clusters
4. **GitOps**: Declarative CRD management

**Current Assessment (v2.0):**
- None of above are blockers
- Manual cleanup acceptable for v2.0
- Defer Operator to v3.0 if needed

## Comparison: With vs Without Operator

| Aspect | With Operator | Without Operator (v2.0) |
|--------|---------------|-------------------------|
| **Complexity** | High (Operator code) | Low (Helm only) |
| **RBAC** | Cluster-wide | Namespace-scoped |
| **Reconciliation** | Automatic | Manual (future: cleanup job) |
| **Drift Detection** | Yes | No (database wins) |
| **Deployment** | Operator + Helm | Helm only |
| **Multi-Platform** | K8s only | K8s + Docker |
| **Maintenance** | Operator upgrades | CRD cleanup |

**Verdict**: Without Operator is simpler and sufficient for v2.0.

## Future Considerations

### v2.1: Cleanup Job
```yaml
# chart/templates/cleanup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: streamspace-cleanup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cleanup
            image: streamspace/cleanup:v2.1
            command:
            - /cleanup.sh
            # Query database, delete orphaned CRDs
```

### v3.0: Optional Operator
- Reconcile database → CRDs (one-way sync)
- Detect drift, revert manual changes
- Optional feature (Helm chart flag: `operator.enabled=true`)

## References

- **Helm Chart**: chart/ (current implementation)
- **CRDs**: chart/crds/ (CRD definitions)
- **Related ADRs**:
  - ADR-006: Database as Source of Truth (why no Operator needed)
  - ADR-005: WebSocket Command Dispatch (agent communication)
- **Design Docs**:
  - 03-system-design/control-plane.md (architecture)

## Approval

- **Status**: Accepted (current implementation)
- **Approved By**: Agent 1 (Architect)
- **Release**: v2.0-beta
- **Review**: v2.1 (evaluate need for cleanup job)
