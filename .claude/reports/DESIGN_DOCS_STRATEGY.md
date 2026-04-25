# Design & Governance Documentation Strategy

**Date:** 2025-11-26
**Author:** Agent 1 (Architect)
**Status:** Approved

---

## Overview

StreamSpace maintains comprehensive design and governance documentation in a **separate private GitHub repository** to support professional software development practices while keeping the main public repository focused on user-facing content.

**Design Docs Location:** `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/`
**Private GitHub Repo:** `streamspace-dev/streamspace-design-and-governance` (to be created)
**Main Repo:** `streamspace-dev/streamspace` (public)

---

## Rationale

### Why Separate Repository?

1. **Access Control:** Design docs may contain sensitive information (security analysis, competitive strategy,未来 roadmap details) that should only be accessible to core team members.

2. **Clean Public Repo:** Main repository remains focused on:
   - User-facing documentation (README, FEATURES, DEPLOYMENT)
   - Getting started guides
   - API reference
   - Contribution guidelines
   - Technical architecture (high-level)

3. **Comprehensive Planning:** Design repo contains detailed planning artifacts:
   - Product vision and competitive positioning
   - Stakeholder analysis and requirements
   - System design deep-dives
   - ADRs (Architecture Decision Records)
   - Security threat models
   - Risk registers and mitigation plans
   - Operational runbooks (SLOs, backup/DR, incident response)
   - Test strategies and quality plans

4. **Professional Development Process:** Supports enterprise-grade software development:
   - Formal design reviews
   - RFC (Request for Comments) process
   - Change management
   - Compliance documentation (SOC2 prep)

---

## Repository Structure

### Design Docs Repo (Private)

**Location:** `streamspace-dev/streamspace-design-and-governance`
**Access:** Core team only (private repository)

```
streamspace-design-and-governance/
├── README.md                                 # Overview and navigation
├── 00-product-vision/                        # Product vision, goals, metrics
│   ├── product-vision.md
│   ├── success-metrics.md
│   └── competitive-positioning.md
├── 01-stakeholders-and-requirements/         # Stakeholders, personas, use cases
│   ├── stakeholders.md
│   ├── personas.md
│   ├── use-cases.md
│   └── requirements.md
├── 02-architecture/                          # Architecture and ADRs
│   ├── current-architecture.md
│   ├── future-architecture.md
│   ├── integration-map.md
│   ├── adr-001-vnc-token-auth.md
│   ├── adr-002-cache-layer.md
│   ├── adr-003-agent-heartbeat-contract.md
│   ├── adr-log.md
│   └── adr-template.md
├── 03-system-design/                         # Component-level designs
│   ├── control-plane.md
│   ├── agents.md
│   ├── api-design.md
│   ├── api-contracts.md
│   ├── data-model.md
│   ├── data-model-erd.md
│   ├── data-flow-diagram.md
│   ├── sequence-diagrams.md
│   ├── authz-and-rbac.md
│   ├── websocket-hardening.md
│   ├── websocket-hardening-checklist.md
│   ├── webhook-contracts.md
│   └── cache-strategy.md
├── 04-ux/                                    # User flows and UX principles
│   ├── user-flows.md
│   └── ux-principles.md
├── 05-delivery-plan/                         # Roadmap and delivery
│   ├── roadmap.md
│   ├── release-strategy.md
│   ├── release-checklist.md
│   ├── work-breakdown.md
│   ├── definition-of-ready-done.md
│   └── staffing-plan.md
├── 06-operations-and-sre/                    # Operations and SRE
│   ├── deployment-architecture.md
│   ├── slo.md
│   ├── observability-dashboards.md
│   ├── backup-and-dr.md
│   ├── incident-response.md
│   └── capacity-planning.md
├── 07-security-and-compliance/               # Security and compliance
│   ├── threat-model.md
│   ├── security-controls.md
│   ├── compliance-plan.md
│   └── privacy-and-audit.md
├── 08-quality-and-testing/                   # Quality and testing
│   ├── test-strategy.md
│   └── automation-coverage.md
└── 09-risk-and-governance/                   # Risk and governance
    ├── risk-register.md
    ├── communication-and-cadence.md
    ├── rfc-process.md
    ├── change-management.md
    ├── contribution-and-branching.md
    ├── contribution-quickstart.md
    ├── code-observations.md
    └── issue-drafts.md
```

---

### Main Repo (Public)

**Location:** `streamspace-dev/streamspace`
**Access:** Public

```
streamspace/
├── README.md                                 # Project overview (links to design docs)
├── FEATURES.md                               # Feature status
├── ROADMAP.md                                # Public roadmap (high-level)
├── CONTRIBUTING.md                           # Contribution guidelines
├── CHANGELOG.md                              # Version history
├── LICENSE                                   # License
├── DEPLOYMENT.md                             # Quick deployment guide
├── docs/                                     # User-facing documentation
│   ├── ARCHITECTURE.md                       # High-level architecture
│   ├── V2_DEPLOYMENT_GUIDE.md                # Detailed deployment
│   ├── V2_BETA_RELEASE_NOTES.md              # Release notes
│   ├── BACKUP_AND_DR_GUIDE.md                # Backup/DR procedures
│   ├── OBSERVABILITY.md                      # Monitoring setup
│   ├── TROUBLESHOOTING.md                    # Common issues
│   └── design/                               # Selected design docs (ADRs only)
│       └── architecture/
│           ├── adr-001-vnc-token-auth.md     # Copy from design repo
│           ├── adr-002-cache-layer.md        # Copy from design repo
│           ├── adr-003-agent-heartbeat-contract.md
│           └── adr-log.md
├── api/                                      # Control Plane API
├── agents/                                   # Execution Agents
├── ui/                                       # Web UI
├── manifests/                                # Kubernetes manifests
│   └── observability/                        # Grafana dashboards, alerts
├── chart/                                    # Helm chart
└── .claude/                                  # Multi-agent coordination
    ├── multi-agent/
    │   └── MULTI_AGENT_PLAN.md
    └── reports/                              # Agent reports (ephemeral)
```

---

## Synchronization Strategy

### ADRs (Architecture Decision Records)

**Strategy:** Copy ADRs from design repo to main repo for visibility

**Workflow:**
1. ADRs are created and maintained in design repo: `02-architecture/adr-*.md`
2. When ADR is "Accepted", copy to main repo: `docs/design/architecture/adr-*.md`
3. Update `adr-log.md` in both repos
4. Main repo ADRs are read-only copies (source of truth is design repo)

**Rationale:** ADRs document architectural decisions that affect contributors and users. Making them visible in public repo improves transparency while keeping full design context private.

---

### Other Design Docs

**Strategy:** Reference design docs via private repo links (team access only)

**Workflow:**
1. Design docs remain in private repo only
2. Main repo docs may reference design docs via links: `See streamspace-design-and-governance/03-system-design/api-contracts.md for details`
3. Public-facing summaries in main repo docs where appropriate

**Rationale:** Detailed design docs (threat models, competitive analysis, roadmap details) should remain private. Public docs provide sufficient information for users and contributors without exposing sensitive content.

---

### User-Facing Documentation

**Strategy:** Maintain in main repo (public)

**Content:**
- Deployment guides (`docs/V2_DEPLOYMENT_GUIDE.md`, `DEPLOYMENT.md`)
- Release notes (`docs/V2_BETA_RELEASE_NOTES.md`)
- Backup/DR guide (`docs/BACKUP_AND_DR_GUIDE.md`)
- Troubleshooting (`docs/TROUBLESHOOTING.md`)
- API reference (future: `docs/API_REFERENCE.md`)

**Workflow:**
1. Create/update documentation in main repo directly
2. May reference design repo for detailed context (team access)

**Rationale:** User-facing docs should be easily accessible without requiring access to private design repo.

---

## Access Control

### Design Docs Repo (Private)

**Access:** Core team members only
- Maintainers: Full read/write access
- Contributors: Request access if needed for specific work

**GitHub Settings:**
- Repository visibility: **Private**
- Team: `streamspace-dev/core-team` (Read/Write)
- Branch protection: `main` requires 1 approval for design doc changes

---

### Main Repo (Public)

**Access:** Public
- Anyone can read
- Contributors can submit PRs
- Maintainers approve/merge

**GitHub Settings:**
- Repository visibility: **Public**
- Branch protection: `main` requires 1-2 approvals

---

## Contributing to Design Docs

### For Core Team Members

1. **Clone Design Repo:**
   ```bash
   git clone git@github.com:streamspace-dev/streamspace-design-and-governance.git
   cd streamspace-design-and-governance
   ```

2. **Create Feature Branch:**
   ```bash
   git checkout -b design/your-feature-name
   ```

3. **Make Changes:**
   - Update existing design docs
   - Add new ADRs using `02-architecture/adr-template.md`
   - Update ADR log: `02-architecture/adr-log.md`

4. **Submit PR:**
   ```bash
   git add .
   git commit -m "design: Your design doc changes"
   git push origin design/your-feature-name
   gh pr create --title "design: Your feature" --body "Description"
   ```

5. **Review & Merge:**
   - Request review from team members
   - Merge to `main` after approval

6. **Sync ADRs to Main Repo (if applicable):**
   ```bash
   # If ADR is "Accepted", copy to main repo
   cd ../streamspace
   cp ../streamspace-design-and-governance/02-architecture/adr-NNN-*.md docs/design/architecture/
   git add docs/design/architecture/
   git commit -m "docs: Add ADR-NNN to public docs"
   ```

---

### For External Contributors

**Process:**
1. External contributors work on main repo (public)
2. If design context needed, core team member provides summary
3. Core team updates design docs separately based on implementation

---

## RFC (Request for Comments) Process

For major design changes, use RFC process defined in design repo:

1. **Create RFC:**
   - File: `09-risk-and-governance/rfcs/rfc-NNN-title.md`
   - Use template from `09-risk-and-governance/rfc-process.md`

2. **Circulate for Feedback:**
   - Post in team Slack/Discord
   - Request reviews from stakeholders

3. **Iterate:**
   - Address feedback
   - Update RFC document

4. **Decision:**
   - RFC approved → Create ADR in `02-architecture/`
   - RFC rejected → Document decision in RFC

5. **Implementation:**
   - Create GitHub issues in main repo
   - Link issues to RFC/ADR

---

## Maintenance

### Regular Reviews

**Quarterly:**
- Review ADRs for accuracy (mark "Superseded" if replaced)
- Update roadmap in design repo
- Sync public roadmap in main repo (high-level only)

**Semi-Annually:**
- Review threat model and security controls
- Update compliance documentation
- Review SLOs and adjust targets

**Annually:**
- Full design docs review
- Archive obsolete documents
- Update product vision and competitive analysis

---

### Design Docs Ownership

**Owner:** Agent 1 (Architect) + Core Team
- Architect coordinates design doc updates
- Scribe (Agent 4) assists with documentation quality
- Core team members contribute domain-specific docs

---

## GitHub Repository Setup

### Create Private Design Docs Repo

**Action Required:**

1. **Create Repo:**
   ```bash
   # Via GitHub UI or gh CLI
   gh repo create streamspace-dev/streamspace-design-and-governance \
     --private \
     --description "Design and governance documentation for StreamSpace" \
     --clone
   ```

2. **Initialize Repo:**
   ```bash
   cd streamspace-design-and-governance
   # Copy existing design docs
   cp -r /Users/s0v3r1gn/streamspace/streamspace-design-and-governance/* .
   git add .
   git commit -m "Initial commit: Design and governance docs"
   git push origin main
   ```

3. **Configure Access:**
   - Add `streamspace-dev/core-team` with Write access
   - Enable branch protection on `main`

4. **Update Main Repo README:**
   - Add link to design docs repo (for team members)
   - Note: Design docs are private (team access only)

---

## Links in Main Repo

**Update `README.md`:**

```markdown
## Documentation

### User Documentation
- [Deployment Guide](docs/V2_DEPLOYMENT_GUIDE.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Backup & DR Guide](docs/BACKUP_AND_DR_GUIDE.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

### Design Documentation (Core Team)
- [Design & Governance Docs](https://github.com/streamspace-dev/streamspace-design-and-governance) (Private - Core team access)
- [Architecture Decision Records](docs/design/architecture/) (Public ADRs)

### Contributing
- [Contribution Guidelines](CONTRIBUTING.md)
- [Roadmap](ROADMAP.md)
- [Features](FEATURES.md)
```

---

## Summary

**Design Docs:** Private repo (`streamspace-design-and-governance`) for comprehensive planning
**Main Repo:** Public repo (`streamspace`) for user-facing content
**ADRs:** Copied from design repo to main repo for visibility
**Access:** Core team has access to design docs; public has access to main repo
**Synchronization:** Manual sync of ADRs; design docs referenced via private links

This strategy balances transparency (public main repo) with confidentiality (private design docs) while maintaining professional development practices.

---

**Next Actions:**
1. ✅ Design docs strategy documented
2. ⏳ Create private GitHub repo: `streamspace-dev/streamspace-design-and-governance`
3. ⏳ Push existing design docs to private repo
4. ⏳ Update main repo README with links
5. ⏳ Copy ADRs to main repo `docs/design/architecture/`

**Status:** ✅ COMPLETE (pending repo creation)
**Owner:** Architect (Agent 1) + Scribe (Agent 4)
