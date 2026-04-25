<div align="center">

# StreamSpace Roadmap

**Last Updated**: 2026-04-25

</div>

---

> [!NOTE]
> Detailed in-flight work is tracked on the [GitHub project board](https://github.com/orgs/streamspace-dev/projects/2) and the [Milestones page](https://github.com/streamspace-dev/streamspace/milestones). This file is the high-level shape.

## Where the project is right now

The v2.0 architecture (Control Plane + multi-platform Agents) shipped. In April 2026 we paused to clean up artifacts from the previous multi-clone dev workflow, retire the dual VNC/Selkies streaming code path in favor of **Selkies-GStreamer (WebRTC) only**, and stand up an actual image build pipeline in [`streamspace-templates`](https://github.com/streamspace-dev/streamspace-templates).

The control plane and agents compile and run end-to-end. The next milestone reconstructs the golden path — *user logs in → picks Chrome → Chrome streams in their browser within 15 s* — and then iterates on whatever it surfaces.

## Released

### v2.0

- Multi-platform Control Plane + Agent architecture
- Kubernetes Agent (session lifecycle, leader election)
- Docker Agent (container lifecycle, HA backends)
- Multi-pod API with Redis-backed AgentHub
- Multi-tenancy: org-scoped access, JWT claims, cross-tenant prevention
- 3 Grafana dashboards, 12 Prometheus alert rules
- OpenAPI 3.0 spec at `/api/docs`
- Security baseline: rate limiting, input validation, security headers, MFA, SSO (SAML/OIDC/OAuth2)

## Active rebuild — April 2026

Tracking on the [project board](https://github.com/orgs/streamspace-dev/projects/2).

| Status | Item |
|---|---|
| ✅ | Strip 99 K lines of obsolete dev artifacts (`.claude/reports/`, multi-agent workflow files, plugins moved to sibling repo) |
| ✅ | Remove the VNC code path; rename agent ↔ control-plane fields to protocol-agnostic naming |
| ✅ | Stand up the image build pipeline in `streamspace-templates` (multi-arch, cosign-signed, SBOM-attested) |
| ✅ | Publish first custom image: `ghcr.io/streamspace-dev/chrome-selkies` |
| 🔄 | Reconcile docs (this repo's `docs/` + the wiki) against the post-cleanup reality |
| 📝 | Golden-path test: login → Chrome template → streaming in 15 s |
| 📝 | Selkies-native template catalog (replacing the inherited LinuxServer set, ~195 templates) |
| 📝 | Bug-fix sprint against whatever the golden-path surfaces |
| 📝 | Migrate `github.com/docker/docker` → `github.com/moby/moby/client` (the upstream module path moved) |

## Next: v2.1

- Performance: Redis cache audit, frontend code splitting, route-level lazy loading
- UX: accessibility (WCAG 2.1 AA), virtual scrolling on large tables, bulk session operations
- Plugin marketplace polish (discovery, install UX, signature verification)
- Multi-cloud onboarding (validated paths for EKS / GKE / AKS)
- Hardware-accelerated streaming (NVENC, VA-API) end-to-end through the chrome-selkies image

## Future: v3.0

- Federation — multi-cluster sessions
- Pluggable streaming protocol — the architecture leaves the door open even though only Selkies ships today
- GPU scheduling: per-session GPU allocation with quotas
- Session recording with on-demand playback (currently a stub plugin)

## How to contribute

High-impact areas right now:

1. **Image pipeline** — pick an app from the [streamspace-templates catalog](https://github.com/streamspace-dev/streamspace-templates) and ship a Selkies-native build under `images/`.
2. **Bug triage** — work through the open [issues](https://github.com/streamspace-dev/streamspace/issues), close the ones that the recent rebuild already resolved.
3. **Docs** — the [wiki](https://github.com/streamspace-dev/streamspace.wiki) is the user-facing entry point; getting-started and template-catalog pages need updating to match the new pipeline.
4. **Plugins** — pick a stub from [`streamspace-plugins`](https://github.com/streamspace-dev/streamspace-plugins) and turn it into a working implementation.

Start with [`CONTRIBUTING.md`](CONTRIBUTING.md).
