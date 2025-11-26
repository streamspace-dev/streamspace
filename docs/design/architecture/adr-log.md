# ADR Log

| ADR | Title | Status | Date | Notes |
| --- | --- | --- | --- | --- |
| ADR-001 | VNC proxy authentication model | Accepted | 2025-11-18 | Token format/expiry and validation path; see adr-001-vnc-token-auth.md |
| ADR-002 | Cache layer for control plane reads | Accepted | 2025-11-20 | See adr-002-cache-layer.md; Issue #214 tracks full implementation |
| ADR-003 | Agent heartbeat contract | In Progress | 2025-11-21 | See adr-003-agent-heartbeat-contract.md; Issue #215 tracks implementation |
| ADR-004 | Multi-tenancy via org-scoped RBAC | Accepted | 2025-11-20 | Critical security architecture; see adr-004-multi-tenancy-org-scoping.md |
| ADR-005 | WebSocket command dispatch (replace NATS) | Accepted | 2025-11-20 | Event bus simplification; see adr-005-websocket-command-dispatch.md |
| ADR-006 | Database as source of truth | Accepted | 2025-11-20 | Decouple from Kubernetes; see adr-006-database-source-of-truth.md |
| ADR-007 | Agent outbound WebSocket | Accepted | 2025-11-18 | Firewall-friendly architecture; see adr-007-agent-outbound-websocket.md |
| ADR-008 | VNC proxy via Control Plane | Accepted | 2025-11-18 | Centralized VNC access control; see adr-008-vnc-proxy-control-plane.md |
| ADR-009 | Helm chart deployment (no operator) | Accepted | 2025-11-26 | Simplified K8s deployment; see adr-009-helm-deployment-no-operator.md |
