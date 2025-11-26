# ADR-001: VNC Token Authentication Model
- **Status**: Accepted
- **Date**: 2025-11-18
- **Owners**: Agent 2 (Builder)
- **Implementation**: api/internal/handlers/vnc_proxy.go

## Context
VNC proxy uses WebSocket to tunnel to session containers. Tokens must authenticate the user/org and authorize session access with minimal replay risk. Current design needs formalization for production hardening and testability.

## Decision (implemented in v2.0-beta)
- Use signed short-lived JWT containing session_id, user_id, issued_at, expires_at
- Validate signature and expiry at VNC proxy endpoint before establishing WebSocket tunnel
- Default TTL: 1 hour (configurable via JWT_SECRET env var)
- Token issued via GET /api/v1/sessions/{id}/vnc endpoint
- Bind token to session; validate user has access to session before proxying

## Rationale
- JWT keeps validation stateless at proxy; reduces central DB lookups per connection.
- Short TTL limits replay window; binding to session/org prevents cross-tenant misuse.
- Simpler to instrument and test vs opaque DB lookups for every handshake.

## Consequences
- Need key rotation strategy; keys must be protected and rolled without downtime.
- Clock skew handling required; small allowable drift.
- Replay within TTL still possible if stolen; mitigate with TLS, short TTL, and optional nonce cache if needed.

## Implementation Status
- ✅ Implemented in v2.0-beta (2025-11-18)
- ✅ JWT validation in VNC proxy handler
- ✅ Token generation endpoint: GET /api/v1/sessions/{id}/vnc
- ✅ Configurable via JWT_SECRET environment variable
- ⚠️ TODO: Add org_id to JWT claims (Issue #212 - Wave 27)
- ⚠️ TODO: Add tests for expired tokens, tampered signatures

## References
- Implementation: api/internal/handlers/vnc_proxy.go
- Token generation: api/internal/api/handlers.go (GetSessionVNC)
- Related: Issue #212 (Org context in JWT claims)
