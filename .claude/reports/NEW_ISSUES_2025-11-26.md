# New Issues Created - 2025-11-26

**Date:** 2025-11-26
**Created By:** Agent 1 (Architect)
**Context:** Gap analysis after Wave 27 planning and Gemini test improvements
**Status:** ✅ Complete

---

## Summary

Created 3 new issues to address gaps identified during session work:
1. **Issue #220:** Security vulnerabilities (P0 - Critical)
2. **Issue #221:** Documentation CI/CD automation (P2 - Future)
3. **Issue #222:** Design docs sync automation (P2 - Future)

---

## Issue #220: Dependabot Security Vulnerabilities (P0)

**URL:** https://github.com/streamspace-dev/streamspace/issues/220
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Labels:** security, P0, component:backend
**Assignee:** TBD (Builder or Security Team)

### Overview

GitHub Dependabot has identified 15 security vulnerabilities in Go dependencies, including 2 critical and 2 high severity issues that must be addressed before v2.0-beta.1 release.

### Critical Vulnerabilities

1. **golang.org/x/crypto SSH Authorization Bypass**
   - Severity: Critical
   - Description: Misuse of ServerConfig.PublicKeyCallback may cause authorization bypass
   - Impact: High (if SSH features used)
   - Action: Update to latest version

2. **Authz Zero Length Regression**
   - Severity: Critical
   - Description: Authorization bypass vulnerability
   - Impact: Unknown (needs investigation)
   - Action: Identify affected package and update

### High Severity Vulnerabilities

3. **golang.org/x/crypto DoS via Slow Key Exchange**
   - Severity: High
   - Description: Vulnerable to Denial of Service
   - Action: Update golang.org/x/crypto

4. **jwt-go Excessive Memory Allocation**
   - Severity: High
   - Description: Header parsing vulnerability
   - Impact: Medium (jwt-go used for API auth)
   - Action: Migrate to golang-jwt/jwt (jwt-go unmaintained)

### Medium & Low Vulnerabilities (10+1)

- golang.org/x/crypto/ssh/agent panics (3 instances)
- golang.org/x/crypto/ssh unbounded memory (2 instances)
- golang.org/x/net XSS vulnerability
- golang.org/x/net HTTP proxy bypass
- net/http excessive headers
- Docker builder cache poisoning
- Moby firewalld isolation issue (low)

### Recommended Timeline

**Immediate (before v2.0-beta.1):**
- Update golang.org/x/crypto
- Migrate from jwt-go to golang-jwt/jwt
- Update golang.org/x/net

**Short Term (v2.0-beta.2):**
- Update Docker/Moby dependencies
- Review all Go dependencies

**Long Term (v2.1+):**
- Add vulnerability scanning to CI/CD
- Automated security alerts
- Document SLA for vulnerability remediation

### Why This Issue Was Created

**Source:** GitHub Dependabot alerts (visible in every push notification)

**Reason:** 15 vulnerabilities discovered, with 2 critical and 2 high severity issues that could impact authentication and security. These should be addressed before v2.0-beta.1 release.

**Alignment:**
- Compliance: docs/design/compliance/industry-compliance.md requires vulnerability remediation SLA
- Security: Critical for SOC 2 readiness (76% ready)
- Production: Needed for secure v2.0-beta.1 release

---

## Issue #221: Documentation CI/CD Automation (P2)

**URL:** https://github.com/streamspace-dev/streamspace/issues/221
**Priority:** P2 - Medium
**Milestone:** Future (v2.1+)
**Labels:** enhancement, P2, component:infrastructure
**Assignee:** Builder (Agent 2) - when ready

### Overview

Automate documentation quality checks in CI/CD to catch broken links, malformed ADRs, and documentation drift before merge.

### Motivation

As documented in SESSION_HANDOFF_2025-11-26.md (Recommendation #9), we need automated checks for:
- **Broken Markdown links** (internal and external)
- **ADR format compliance** (Status, Date, Owner fields required)
- **Mermaid diagram syntax validation**
- **Stale documentation detection** (>6 months without review)

### Proposed Solution

GitHub Actions workflow: `.github/workflows/docs-check.yml`

```yaml
name: Documentation Check

on:
  pull_request:
    paths:
      - 'docs/**'
      - '.claude/reports/**'

jobs:
  validate-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Check Markdown links
        uses: gaurav-nelson/github-action-markdown-link-check@v1

      - name: Validate ADR format
        run: |
          for adr in docs/design/architecture/adr-*.md; do
            echo "Checking $adr"
            grep -q "^- \*\*Status\*\*:" "$adr" || exit 1
            grep -q "^- \*\*Date\*\*:" "$adr" || exit 1
          done

      - name: Check for broken Mermaid diagrams
        run: |
          grep -n "```mermaid" docs/**/*.md | while read match; do
            echo "Found Mermaid diagram: $match"
          done
```

### Benefits

- **Catch issues early:** Broken links detected in PRs before merge
- **Enforce standards:** ADRs must follow template format
- **Prevent drift:** Detect stale documentation automatically
- **Save time:** Automated checks vs. manual review

### Implementation Phases

**Phase 1 (Minimum Viable):**
- Markdown link checker only
- Block PR merge on broken links

**Phase 2 (Enhanced):**
- ADR format validation
- Check for required sections

**Phase 3 (Advanced - Optional):**
- Mermaid diagram syntax checking
- Stale documentation warnings

### Acceptance Criteria

- [ ] GitHub Actions workflow created
- [ ] Markdown link checker enabled
- [ ] ADR format validation implemented
- [ ] Workflow runs on all documentation PRs
- [ ] Green checkmark required to merge

### Why This Issue Was Created

**Source:** SESSION_HANDOFF_2025-11-26.md (Recommendation #9)

**Reason:** With 26 documentation files (~8,600 lines) now on main, we need automated quality checks to prevent documentation debt and broken links.

**Alignment:**
- DESIGN_DOCS_STRATEGY.md - Maintenance section recommends quarterly reviews
- Best practices - Automated validation catches issues early

**Priority:** P2 (not blocking v2.0 releases, but valuable for long-term quality)

---

## Issue #222: Design Docs Sync Automation (P2)

**URL:** https://github.com/streamspace-dev/streamspace/issues/222
**Priority:** P2 - Medium
**Milestone:** Future (v2.1+)
**Labels:** enhancement, P2, component:infrastructure
**Assignee:** Builder (Agent 2) - when ready

### Overview

Automate weekly sync of design documentation from private repo (`streamspace-design-governance`) to public repo (`streamspace/docs/design`) using GitHub Actions.

### Motivation

Currently documented in docs/DESIGN_DOCS_STRATEGY.md, manual sync process:
1. Review changes in private repo
2. Identify public-safe content
3. Run rsync commands to copy files
4. Review for sensitive information
5. Commit and push to public repo

**Problem:** Manual process is error-prone and easy to forget.

**Solution:** Automated weekly sync with PR review for safety.

### Proposed Solution

GitHub Actions workflow in **private repo**: `.github/workflows/sync-to-public.yml`

```yaml
name: Sync Design Docs to Public Repo

on:
  workflow_dispatch: # Manual trigger
  schedule:
    - cron: '0 0 * * 0' # Weekly on Sunday

jobs:
  sync-docs:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout private repo
        uses: actions/checkout@v4

      - name: Checkout public repo
        uses: actions/checkout@v4
        with:
          repository: streamspace-dev/streamspace
          token: ${{ secrets.PUBLIC_REPO_TOKEN }}
          path: public-repo

      - name: Sync ADRs
        run: |
          rsync -av --delete \
            02-architecture/adr-*.md \
            public-repo/docs/design/architecture/

      - name: Sync C4 Diagrams
        run: |
          rsync -av --delete \
            02-architecture/c4-diagrams.md \
            public-repo/docs/design/architecture/

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v5
        with:
          token: ${{ secrets.PUBLIC_REPO_TOKEN }}
          commit-message: "docs: Sync design documentation from private repo"
          title: "Automated Design Docs Sync"
          body: |
            Automated weekly sync of design documentation.

            **Review:** Verify no sensitive information leaked.
          branch: automated-docs-sync
          path: public-repo
```

### What Gets Synced (Public)

- ✅ ADRs (all architecture decisions)
- ✅ C4 diagrams (system architecture)
- ✅ Coding standards
- ✅ Compliance frameworks (controls only, not evidence)

### What Stays Private (NOT Synced)

- 🔒 Stakeholder requirements (customer-specific)
- 🔒 Security assessments (vulnerability details)
- 🔒 Vendor evaluations (contract details)
- 🔒 Risk register (internal risk analysis)
- 🔒 Compliance audit evidence (SOC 2 reports, etc.)

### Security Considerations

- **PR review required:** Automated PR creation, manual merge approval
- **Token security:** GitHub PAT stored as secret in private repo
- **Audit trail:** All syncs tracked in public repo commit history
- **Rollback:** Easy to revert if sensitive info accidentally synced

### Prerequisites

1. Create GitHub Personal Access Token (PAT) with `repo` scope
2. Add as secret in private repo: `PUBLIC_REPO_TOKEN`
3. Test manual workflow trigger before enabling schedule
4. Document sync process in DESIGN_DOCS_STRATEGY.md

### Benefits

- **Consistency:** Public docs stay current with private repo
- **Less manual work:** Weekly automated sync saves time
- **Safety:** PR review prevents accidental leaks
- **Traceability:** Sync commits show what changed and when

### Acceptance Criteria

- [ ] GitHub Actions workflow created in private repo
- [ ] Workflow syncs ADRs, C4 diagrams, coding standards
- [ ] Creates PR in public repo (not auto-merge)
- [ ] Weekly schedule configured (Sunday midnight)
- [ ] Manual trigger available for ad-hoc syncs
- [ ] Documentation updated in DESIGN_DOCS_STRATEGY.md

### Why This Issue Was Created

**Source:** docs/DESIGN_DOCS_STRATEGY.md (Manual sync process documented)

**Reason:** With 79 design docs in private repo and 26 in public, manual sync is time-consuming and error-prone. Automation ensures consistency.

**Alignment:**
- DESIGN_DOCS_STRATEGY.md - Recommends weekly sync
- Best practices - Automate repetitive manual tasks

**Priority:** P2 (nice to have, not urgent - manual sync works for now)

---

## Impact Assessment

### Immediate Impact (v2.0-beta.1)

**Issue #220 (Security):**
- ⚠️ **HIGH IMPACT** - Must be addressed before release
- 2 Critical vulnerabilities require immediate attention
- Timeline: 2-3 days (align with Wave 27 schedule)

**Issues #221 & #222 (Automation):**
- ℹ️ **NO IMPACT** - Future enhancements, not blocking

### Long-Term Impact (v2.1+)

**Documentation Quality:**
- Automated link checking prevents broken documentation
- ADR format validation enforces standards
- Weekly sync keeps public docs current

**Developer Efficiency:**
- Less manual work (sync automation)
- Faster issue detection (CI/CD checks)
- Better documentation quality overall

---

## Recommended Actions

### This Week (Wave 27)

1. **Address Issue #220 immediately**
   - Assign to Builder (Agent 2) or Security Team
   - Prioritize after Issues #211, #212 (security-related)
   - Update dependencies before v2.0-beta.1 release

2. **Defer Issues #221 & #222**
   - Add to v2.1 backlog
   - No action needed for v2.0-beta releases

### Next Week (Post Wave 27)

3. **Create v2.1 milestone**
   - Add Issues #221, #222 to v2.1 milestone
   - Include other automation improvements

4. **Document vulnerability SLA**
   - As recommended in compliance docs
   - Critical: 48h, High: 7 days

---

## Related Documentation

- **Session Handoff:** .claude/reports/SESSION_HANDOFF_2025-11-26.md
- **Design Strategy:** docs/DESIGN_DOCS_STRATEGY.md
- **Compliance:** docs/design/compliance/industry-compliance.md
- **Gemini Report:** .claude/reports/GEMINI_TEST_IMPROVEMENTS_2025-11-26.md

---

## Issue Creation Log

| Issue | Title | Priority | Created | URL |
|-------|-------|----------|---------|-----|
| #220 | Dependabot Security Vulnerabilities | P0 | 2025-11-26 | https://github.com/streamspace-dev/streamspace/issues/220 |
| #221 | Documentation CI/CD Automation | P2 | 2025-11-26 | https://github.com/streamspace-dev/streamspace/issues/221 |
| #222 | Design Docs Sync Automation | P2 | 2025-11-26 | https://github.com/streamspace-dev/streamspace/issues/222 |

**Total:** 3 new issues (1 P0, 2 P2)

---

## Summary

**Question:** "are there any additional issues that need to be opened?"

**Answer:** Yes, 3 issues created:

1. **Security vulnerabilities (P0)** - Critical, must address before v2.0-beta.1
2. **Documentation CI/CD (P2)** - Future automation, improves quality
3. **Design docs sync (P2)** - Future automation, reduces manual work

**Priority for Wave 27:** Only Issue #220 (Security) needs immediate attention. Issues #221 and #222 are future enhancements for v2.1+.

---

**Report Complete:** 2025-11-26
**Status:** ✅ All identified gaps now have issues
**Next Action:** Address Issue #220 before v2.0-beta.1 release
