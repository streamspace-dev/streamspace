# Architecture Migration Note

## k8s-controller Directory Removed

As of v2.0, the `k8s-controller/` directory has been **removed** from the codebase.

### What Changed

**Before (v1.x)**: Kubernetes CRD-based controller architecture

- Directory: `k8s-controller/`
- Pattern: Kubebuilder-based controller watching Session/Template CRDs
- Communication: Direct Kubernetes API

**After (v2.0+)**: WebSocket agent architecture  

- Directory: `agents/k8s-agent/`
- Pattern: Agent connects to Control Plane (API) via WebSocket
- Communication: WebSocket command channel + VNC proxy tunneling

### Impacted Documentation

The following documentation files contain **historical references** to `k8s-controller`:

- DEPLOYMENT.md
- ROADMAP.md  
- ANALYSIS_REPORT.md
- CHANGELOG.md
- docs/TESTING_GUIDE.md
- docs/MULTI_CONTROLLER_ARCHITECTURE.md
- docs/architecture/NATS_EVENT_ARCHITECTURE.md
- docs/CODEBASE_AUDIT_REPORT.md
- docs/PHASE_5_5_RELEASE_NOTES.md
- docs/CRD_FIELD_COMPARISON.md
- docs/V1_ROADMAP_SUMMARY.md
- docs/TEMPLATE_CRD_ANALYSIS.md

These references are **historical/archival** and describe the v1.x architecture. They have been left intact for reference purposes.

### For New Development

**Use**: `agents/k8s-agent/` for Kubernetes platform implementation  
**Architecture**: Agent-based with WebSocket communication to Control Plane  
**See**: README.md, CLAUDE.md for current v2.0 architecture

---

*This note added: 2025-11-21*
