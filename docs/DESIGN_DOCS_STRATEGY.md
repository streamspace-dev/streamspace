# Design Documentation Strategy

**Version:** 1.0
**Last Updated:** 2025-11-26
**Owner:** Architecture Team
**Status:** Active

---

## Overview

StreamSpace maintains design and governance documentation across two repositories with different visibility levels:

1. **Private Repository** (`streamspace-design-and-governance`) - Comprehensive internal documentation
2. **Public Repository** (`streamspace`) - Selected documentation for community and contributors

This strategy balances transparency with security by keeping sensitive planning and vendor assessments private while publishing helpful architectural decisions and standards publicly.

---

## Repository Structure

### Private Repository: streamspace-design-and-governance

**URL:** https://github.com/streamspace-dev/streamspace-design-and-governance (Private)
**Purpose:** Comprehensive design and governance documentation (internal only)
**Access:** StreamSpace core team and authorized contributors

**Directory Structure:**
```
streamspace-design-and-governance/
├── 00-product-vision/               # Product strategy and vision
├── 01-stakeholders-and-requirements/ # Stakeholder maps, requirements
├── 02-architecture/                 # Architecture decisions (ADRs)
├── 03-system-design/                # Detailed system design specs
├── 04-ux/                           # UX design, wireframes, mockups
├── 05-delivery-plan/                # Release planning, timelines
├── 06-operations-and-sre/           # Operations runbooks, SRE guides
├── 07-security-and-compliance/      # Security assessments, compliance
├── 08-quality-and-testing/          # QA strategy, test plans
├── 09-risk-and-governance/          # Risk register, governance docs
└── README.md                        # Repository overview
```

**Total Documents:** 79 markdown files (~15,000 lines)

**Content Types:**
- Product vision and strategy
- Stakeholder requirements and analysis
- Architecture Decision Records (ADRs)
- System design specifications
- UX mockups and wireframes
- Delivery timelines and milestones
- Operations and SRE runbooks
- Security assessments and compliance mappings
- Risk register and governance policies
- Quality assurance and test strategies

---

### Public Repository: streamspace/docs/design

**URL:** https://github.com/streamspace-dev/streamspace/tree/main/docs/design (Public)
**Purpose:** Community-facing design documentation for contributors
**Access:** Public (open source)

**Directory Structure:**
```
streamspace/docs/design/
├── README.md                        # Documentation index
├── architecture/                    # ADRs and architecture diagrams
│   ├── adr-log.md                  # ADR index
│   ├── adr-template.md             # ADR template
│   ├── adr-001-vnc-token-auth.md   # Individual ADRs
│   ├── adr-002-cache-layer.md
│   ├── adr-003-agent-heartbeat-contract.md
│   ├── adr-004-multi-tenancy-org-scoping.md
│   ├── adr-005-websocket-command-dispatch.md
│   ├── adr-006-database-source-of-truth.md
│   ├── adr-007-agent-outbound-websocket.md
│   ├── adr-008-vnc-proxy-control-plane.md
│   ├── adr-009-helm-deployment-no-operator.md
│   └── c4-diagrams.md              # C4 architecture diagrams
├── ux/                              # UX documentation
│   ├── information-architecture.md  # Site map and navigation
│   └── component-library.md         # UI component catalog
├── operations/                      # Operations guides
│   └── load-balancing-and-scaling.md
├── compliance/                      # Compliance documentation
│   └── industry-compliance.md       # SOC 2, HIPAA, FedRAMP
├── product/                         # Product management
│   └── product-lifecycle.md         # API versioning, deprecation
├── coding-standards.md              # Coding conventions
├── acceptance-criteria-guide.md     # Feature definition standards
├── retrospective-template.md        # Sprint retrospective format
└── vendor-assessment.md             # Third-party risk evaluation
```

**Total Documents:** 26 files (~8,600 lines)

---

## Documentation Sync Strategy

### What Gets Published to Public Repo

**Published (Public):**
- ✅ Architecture Decision Records (ADRs) - Technical decisions
- ✅ C4 Architecture Diagrams - System visualization
- ✅ Coding Standards - Development conventions
- ✅ Component Library - UI component documentation
- ✅ Information Architecture - Public UI structure
- ✅ Acceptance Criteria Guide - Feature definition standards
- ✅ Load Balancing & Scaling - Production operations (non-sensitive)
- ✅ Compliance Framework (SOC 2, HIPAA) - Control mappings only
- ✅ Product Lifecycle - API versioning and deprecation policies
- ✅ Vendor Assessment Template - Assessment framework only

**Rationale:** These documents help community contributors understand architecture, contribute code following standards, and understand production requirements.

---

### What Stays Private

**Private Only (Not Published):**
- 🔒 Product Vision & Strategy - Competitive roadmap
- 🔒 Stakeholder Requirements - Customer-specific requirements
- 🔒 Detailed System Design - Implementation specifics
- 🔒 UX Wireframes & Mockups - Pre-release design work
- 🔒 Delivery Timelines - Release dates, milestones
- 🔒 Security Assessments - Vulnerability assessments, penetration test results
- 🔒 Vendor Evaluations - Specific vendor scores and contracts
- 🔒 Risk Register - Detailed risk analysis and mitigations
- 🔒 Compliance Evidence - Actual compliance audit artifacts
- 🔒 Internal Operations Runbooks - Sensitive operational procedures

**Rationale:** These documents contain sensitive competitive information, customer data, security details, or contractual information that should remain confidential.

---

## Sync Process

### Manual Sync (Current)

**When to Sync:**
- After creating/updating ADRs
- After major design document updates
- Before major releases (v2.0, v2.1, etc.)
- Quarterly documentation review

**How to Sync:**

1. **Review Private Repo Changes:**
   ```bash
   cd /Users/s0v3r1gn/streamspace/streamspace-design-and-governance
   git log --since="1 week ago" --oneline
   ```

2. **Identify Public-Safe Content:**
   - ADRs (all are public-safe)
   - Updated coding standards
   - New architecture diagrams
   - Compliance framework updates (exclude evidence)

3. **Copy to Public Repo:**
   ```bash
   # Example: Sync ADRs
   cp /Users/s0v3r1gn/streamspace/streamspace-design-and-governance/02-architecture/adr-*.md \
      /Users/s0v3r1gn/streamspace/streamspace/docs/design/architecture/

   # Example: Sync C4 diagrams
   cp /Users/s0v3r1gn/streamspace/streamspace-design-and-governance/02-architecture/c4-diagrams.md \
      /Users/s0v3r1gn/streamspace/streamspace/docs/design/architecture/
   ```

4. **Sanitize if Needed:**
   - Remove internal-only sections (e.g., "Internal Notes")
   - Redact specific vendor names if under NDA
   - Remove customer-specific examples

5. **Commit to Public Repo:**
   ```bash
   cd /Users/s0v3r1gn/streamspace/streamspace
   git add docs/design/
   git commit -m "docs: Sync design documentation from private repo"
   git push origin main
   ```

---

### Automated Sync (Future - Recommended)

**GitHub Actions Workflow** (`.github/workflows/sync-design-docs.yml`):

```yaml
name: Sync Design Docs from Private Repo

on:
  workflow_dispatch: # Manual trigger
  schedule:
    - cron: '0 0 * * 0' # Weekly on Sunday

jobs:
  sync-docs:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout public repo
        uses: actions/checkout@v4
        with:
          repository: streamspace-dev/streamspace
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Checkout private repo
        uses: actions/checkout@v4
        with:
          repository: streamspace-dev/streamspace-design-and-governance
          token: ${{ secrets.PRIVATE_REPO_TOKEN }}
          path: private-docs

      - name: Sync ADRs
        run: |
          rsync -av --delete \
            private-docs/02-architecture/adr-*.md \
            docs/design/architecture/

      - name: Sync C4 Diagrams
        run: |
          rsync -av --delete \
            private-docs/02-architecture/c4-diagrams.md \
            docs/design/architecture/

      - name: Sync Coding Standards
        run: |
          rsync -av --delete \
            private-docs/08-quality-and-testing/coding-standards.md \
            docs/design/

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v5
        with:
          commit-message: "docs: Sync design documentation from private repo"
          title: "Automated Design Docs Sync"
          body: |
            Automated sync of design documentation from private repository.

            **Synced:**
            - ADRs
            - C4 Diagrams
            - Coding Standards

            **Review:** Verify no sensitive information leaked.
          branch: automated-docs-sync
```

**Benefits:**
- Consistent weekly sync
- Pull request review for safety
- Automated conflict detection

---

## Document Lifecycle

### Creating New Design Documents

**In Private Repo:**
1. Create document in appropriate directory (e.g., `02-architecture/adr-010-new-decision.md`)
2. Follow template (ADRs use `adr-template.md`)
3. Commit and push to private repo
4. Mark as "Public" or "Private" in document metadata

**Publishing to Public Repo:**
1. Review document for sensitive information
2. If public-safe, sync to public repo (`docs/design/`)
3. Create PR in public repo for review
4. Merge after approval

---

### Updating Existing Documents

**Private Repo (Source of Truth):**
1. Update document in private repo
2. Commit with clear changelog entry
3. Push to private repo

**Public Repo (Selective Sync):**
1. If document is public-facing, sync changes
2. Review diff for new sensitive information
3. Create PR in public repo
4. Merge after approval

---

### Deprecating Documents

**Private Repo:**
- Keep all documents (historical record)
- Mark as "Deprecated" or "Superseded"

**Public Repo:**
- Keep deprecated ADRs (with "Superseded" notice)
- Remove deprecated design docs if no longer relevant
- Update index (README.md) to reflect deprecation

---

## Security & Compliance

### Preventing Information Leakage

**Pre-Sync Checklist:**
- [ ] No customer names or identifiable information
- [ ] No specific vendor pricing or contracts
- [ ] No security vulnerability details (beyond fixed CVEs)
- [ ] No internal server names, IPs, or credentials
- [ ] No unreleased feature details (if under NDA)
- [ ] No compliance audit evidence (certificates, reports)

**Review Process:**
- All public syncs require PR review
- Security team reviews compliance docs
- Product team reviews feature roadmaps

---

### Access Control

**Private Repo Access:**
- Core team: Read + Write
- External contractors: Read (case-by-case)
- Community: No access

**Public Repo Access:**
- Anyone: Read
- Contributors: Read + PR
- Maintainers: Read + Write

---

## Maintenance

### Quarterly Review (Every 3 Months)

**Tasks:**
1. Review all ADRs for accuracy (implementation vs. documented)
2. Update "Last Reviewed" dates
3. Archive obsolete documents
4. Sync new public-safe content
5. Update documentation index

**Checklist:**
- [ ] ADRs accurate (status reflects reality)
- [ ] Coding standards current
- [ ] Compliance mappings up-to-date
- [ ] C4 diagrams reflect current architecture
- [ ] Dead links fixed
- [ ] Mermaid diagrams render correctly

---

### Annual Review (Yearly)

**Tasks:**
1. Comprehensive audit of all documentation
2. Assess ROI of private vs. public split
3. Review security of private repo
4. Update sync strategy if needed
5. Archive old design iterations

---

## Metrics

### Documentation Health

**Private Repo:**
- Total documents: 79
- Total lines: ~15,000
- Last updated: 2025-11-26
- Stale documents (>6 months): 0

**Public Repo:**
- Total documents: 26
- Total lines: ~8,600
- Last synced: 2025-11-26
- Coverage: ~33% of private repo (by document count)

**Sync Frequency:**
- Current: Manual (ad-hoc)
- Target: Weekly automated sync

---

## Tools & Automation

### Current Tools

**Manual:**
- `rsync` for file copying
- `git` for version control
- GitHub CLI (`gh`) for PR creation

**Editor:**
- VS Code with Markdown linters
- Mermaid preview for diagrams

---

### Recommended Tools (Future)

**Automation:**
- GitHub Actions for automated sync
- Pre-commit hooks for sensitive data detection
- Markdown link checker (CI/CD)

**Collaboration:**
- GitHub Discussions for design RFC process
- GitHub Projects for tracking documentation work

**Monitoring:**
- GitHub Insights for documentation activity
- Custom dashboard for "Last Updated" tracking

---

## FAQ

### Why maintain two repositories?

**Answer:** Balance transparency with security. Public repo helps community contributors, private repo protects competitive and sensitive information.

### How often should we sync?

**Answer:** Weekly automated sync recommended, or after major design changes (new ADRs, architecture updates).

### What if we accidentally leak sensitive info?

**Answer:**
1. Immediately revert commit in public repo
2. Force push to remove from history (if caught early)
3. Rotate any leaked credentials
4. Conduct security review of sync process

### Can we automate the sync?

**Answer:** Yes, GitHub Actions can automate with careful filtering and PR review process. Recommended for v2.1+.

### Who approves public syncs?

**Answer:**
- ADRs: Architecture team (1 approval)
- Compliance docs: Security team (1 approval)
- Operations docs: SRE team (1 approval)
- All docs: General maintainer review

---

## References

**Related Documents:**
- [Documentation Index](design/README.md) - Public docs navigation
- [ADR Log](design/architecture/adr-log.md) - All architecture decisions
- [MULTI_AGENT_PLAN.md](.claude/multi-agent/MULTI_AGENT_PLAN.md) - Multi-agent coordination

**External Resources:**
- GitHub: https://github.com/streamspace-dev/streamspace (Public)
- GitHub: https://github.com/streamspace-dev/streamspace-design-and-governance (Private)
- Notion (if used): [Design workspace link]

---

## Contact

**Questions about design docs strategy?**
- GitHub Issues: Tag with `documentation` label
- Team Channel: #architecture (Slack/Discord)
- Email: architecture@streamspace.dev

**Maintainers:**
- Architecture: Agent 1 (Architect) + Architecture Team
- Operations: SRE Team
- Security: Security Team
- Product: Product Management

---

**Version History:**
- **v1.0** (2025-11-26): Initial design docs strategy documented
- **Next Review:** Q1 2026 (post v2.0 GA)

---

## Quick Commands

### Sync ADRs to Public Repo
```bash
rsync -av --delete \
  /Users/s0v3r1gn/streamspace/streamspace-design-and-governance/02-architecture/adr-*.md \
  /Users/s0v3r1gn/streamspace/streamspace/docs/design/architecture/
```

### Check for Sensitive Strings (Pre-Sync)
```bash
grep -r "PRIVATE\|CONFIDENTIAL\|INTERNAL ONLY" docs/design/
grep -r "password\|api_key\|secret" docs/design/
```

### Create Sync PR
```bash
cd /Users/s0v3r1gn/streamspace/streamspace
git checkout -b sync-design-docs-$(date +%Y%m%d)
git add docs/design/
git commit -m "docs: Sync design documentation from private repo"
git push origin sync-design-docs-$(date +%Y%m%d)
gh pr create --title "Sync Design Docs" --body "Weekly sync from private repo"
```

### Find Stale Docs (>6 months)
```bash
find docs/design -name "*.md" -mtime +180 -exec ls -lh {} \;
```

---

**Last Updated:** 2025-11-26
**Status:** ✅ Active - Private repo created, sync strategy documented
