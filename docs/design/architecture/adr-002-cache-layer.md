# ADR-002: Cache Layer for Control Plane Reads
- **Status**: Accepted
- **Date**: 2025-11-20
- **Owners**: Agent 2 (Builder)
- **Implementation**: api/internal/cache/cache.go

## Context
Hot read paths (session lists, templates, org metadata) hit PostgreSQL. A Redis cache exists (api/internal/cache) but usage is ad-hoc. Need a consistent cache policy with invalidation and fallbacks.

## Decision (proposed)
- Use Redis as primary cache for read-heavy, low-staleness-tolerance objects: template lists, org settings, feature flags, user/org lookup, session summary counts.
- Keep cache optional (`Enabled` flag); code must operate correctly when disabled.
- Standardize envelopes and keys (reuse cache/keys helpers); enforce TTL defaults (e.g., 60s for templates, 15s for session summaries, 5m for org metadata) with per-key overrides.
- Invalidate on write paths via Delete/DeletePattern; avoid cache write from side effects outside services.
- Add metrics for hit/miss/error and circuit-break cache on repeated failures (fail open, log/metric only).

## Rationale
- Reduces DB load for UI dashboards and list endpoints.
- Aligns with existing Redis client and middleware; minimal new infra.
- Explicit TTL + invalidation reduces stale state risk vs implicit caching.

## Consequences
- Staleness windows must be acceptable; design UI to tolerate slight lag or force-refresh on critical actions.
- Additional complexity in services to manage invalidations.
- Need observability to avoid silent cache poisoning.

## Implementation Status
- ✅ Infrastructure implemented (api/internal/cache/cache.go) - v2.0-beta
- ✅ Redis client with fail-open behavior
- ✅ Cache enabled via CACHE_ENABLED environment variable
- ✅ Graceful degradation when Redis unavailable
- ⚠️ TODO: Standardize keys/TTLs across services (Issue #214 - v2.0-beta.2)
- ⚠️ TODO: Implement invalidation hooks on writes
- ⚠️ TODO: Add cache metrics (hit/miss/error rates)

## Rollout Plan
- Phase 1 (v2.0-beta): ✅ Cache infrastructure and fail-open behavior
- Phase 2 (v2.0-beta.2): Issue #214 - Standardized keys, TTLs, invalidation
- Phase 3 (v2.1): Cache metrics and alerting

## References
- Implementation: api/internal/cache/cache.go
- Design doc: 03-system-design/cache-strategy.md
- Future work: Issue #214 (Cache strategy with keys/TTLs/metrics)
