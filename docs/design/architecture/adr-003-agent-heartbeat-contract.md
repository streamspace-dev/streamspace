# ADR-003: Agent Heartbeat Contract
- **Status**: In Progress
- **Date**: 2025-11-26
- **Owners**: Agent 2 (Builder), Agent 3 (Validator)
- **Implementation**: Issue #215 (v2.0-beta.2)

## Context
Agents maintain WebSocket connections and send heartbeats (see api/internal/websocket/agent_hub.go, handlers/agent_websocket.go, handlers/agents.go). The protocol and timeouts are implicit; tests suggest 10–30s intervals and stale detection at ~45–60s. Need a formal contract for compatibility across agent versions and control plane HA.

## Decision (proposed)
- Heartbeat message schema (JSON): `{ type: "heartbeat", agent_id, platform, region, status, capacity: { sessions:int, cpu?:string, memory?:string }, active_sessions:int, timestamp }`.
- Interval: agents send every 10s (configurable); control plane tolerates up to 3x interval before marking offline.
- Persistence: on heartbeat, update DB `agents.status=online`, `last_heartbeat=now()`, refresh Redis state if present.
- Staleness: if no heartbeat for >30s (or 3x interval), mark agent `degraded`; >60s mark `offline` and stop scheduling.
- Compatibility: agents include `protocol_version`; control plane negotiates supported features (e.g., capacity fields) and logs mismatches.

## Rationale
- Explicit schema/timeouts reduce flakiness and clarify HA behavior.
- Supports mixed versions by making interval configurable and exposing protocol version.
- Enables better alerting and scheduling decisions based on status/degradation.

## Consequences
- Agents must be updated to include protocol_version and adhere to interval; control plane must handle older agents gracefully.
- Scheduling logic must respect status transitions; may pause work on degraded/offline agents.

## Implementation Status
- ✅ Basic heartbeat implemented (30s interval) - v2.0-beta
- ✅ AgentHub tracks last_heartbeat timestamp
- ✅ Agent status updates on heartbeat
- ⚠️ TODO: Formal heartbeat schema with protocol_version (Issue #215)
- ⚠️ TODO: Capacity reporting (active_sessions, max_sessions, cpu, memory)
- ⚠️ TODO: Status transitions (online/degraded/offline) with thresholds
- ⚠️ TODO: Metrics and alerts for stale agents

## Rollout Plan
- Phase 1 (v2.0-beta): ✅ Basic heartbeat mechanism (30s interval)
- Phase 2 (v2.0-beta.2): Issue #215 - Formal contract, capacity reporting, status transitions
- Phase 3 (v2.1): Protocol versioning for backward compatibility

## References
- Current implementation: api/internal/websocket/agent_hub.go
- Agent code: agents/k8s-agent/main.go, agents/docker-agent/main.go
- Future work: Issue #215 (Agent heartbeat contract)
