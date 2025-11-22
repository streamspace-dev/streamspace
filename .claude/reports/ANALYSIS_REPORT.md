# Architecture Redesign Analysis Report

## Executive Summary

The transition to a platform-agnostic architecture requires significant refactoring of the `api` and `k8s-controller` components. The `ui` is relatively decoupled but still contains Kubernetes-specific terminology and assumptions that need to be abstracted.

The core challenge is moving from a **Kubernetes-Native** model (where the API talks directly to K8s) to an **Agent-Based** model (where the API talks to generic Controllers).

## Component Analysis

### 1. API Backend (`api/`)

**Current State**:

- Heavily coupled with Kubernetes via `k8s.io/client-go`.
- `internal/k8s/client.go` handles direct CRD operations.
- `internal/handlers/` assumes Session/Template CRDs exist in a cluster.
- `go.mod` has heavy K8s dependencies.

**Required Changes**:

- **Remove K8s Dependencies**: Strip `k8s.io/*` imports.
- **Abstract Data Model**: Replace CRD-based models with database-backed models for `Session` and `Template`.
- **Controller Management**: Implement a registry for Controllers (Agents) to register/connect.
- **Communication Layer**: Implement the secure WebSocket/gRPC server for Controllers to connect to.
- **Scheduler**: Implement a scheduler to decide which Controller should run a session (based on tags/resources).

### 2. Kubernetes Controller (`k8s-controller/`)

**Current State**:

- Standard Kubebuilder controller.
- Watches CRDs and reconciles Pods/PVCs.
- Logic is tightly bound to the "Operator pattern" (watch loop).

**Required Changes**:

- **Refactor to Agent**: Change from "watching CRDs" to "listening to Control Plane".
- **Command Execution**: Implement handlers for `StartSession`, `StopSession`, etc., triggered by the Control Plane.
- **State Sync**: Instead of updating CRD status, report status back to the Control Plane via API.
- **Rename**: Move to `controllers/k8s/` and rename to `streamspace-agent-k8s`.

### 3. Web UI (`ui/`)

**Current State**:

- Mostly consumes generic API endpoints.
- Some admin pages (`Nodes.tsx`) likely assume K8s nodes.
- Terminology like "Pod Name" is exposed in the UI.

**Required Changes**:

- **Terminology Update**: Rename "Pod" to "Instance" or "Container".
- **Admin Views**: Update "Nodes" view to show "Controllers" and their underlying resources.
- **Status Display**: Ensure status fields (Phase, URL) map correctly from the new generic model.

## Migration Strategy

1. **Phase 1: Control Plane Decoupling**
   - Create the new database schema for Sessions/Templates.
   - Update API to read/write to DB instead of K8s.
   - Implement the Controller Registration API.

2. **Phase 2: K8s Agent Adaptation**
   - Fork `k8s-controller` to `controllers/k8s`.
   - Replace the Manager/Reconciler loop with an Agent loop that connects to the new API.

3. **Phase 3: UI Updates**
   - Update the UI to reflect the new API response structures.
   - Remove K8s-specific jargon.

## Risk Assessment

- **Complexity**: High. This is a rewrite of the core orchestration logic.
- **Compatibility**: Breaking change. Existing deployments will need a migration path (likely re-creating sessions).
- **Performance**: Moving from K8s watch events to Agent reporting might introduce latency in status updates.

## Conclusion

The redesign is feasible but requires a structured approach. The "Control Plane" needs to become the source of truth, rather than Kubernetes. The K8s Controller will become just one of many possible backends.
