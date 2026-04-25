# Wave 28 Agent Assignments

**Date:** 2025-11-26
**Created By:** Agent 1 (Architect)
**Wave Duration:** 2025-11-26 → 2025-11-29 (3-4 days)
**Status:** 🔴 ACTIVE - P0 Blockers for v2.0-beta.1

---

## Executive Summary

Wave 27 integration is complete. Wave 28 focuses exclusively on **P0 blockers** preventing the v2.0-beta.1 release:

1. **Issue #220:** Security vulnerabilities (15 Dependabot alerts)
2. **Issue #200:** UI test failures (19 test files failing)

Both issues can be worked in **parallel** and must be complete before release.

---

## Agent Assignments

### Builder (Agent 2) - Issue #220: Security Vulnerabilities 🚨

**Priority:** P0 - CRITICAL
**Timeline:** 2-3 days
**Branch:** `claude/v2-builder`
**GitHub Issue:** https://github.com/streamspace-dev/streamspace/issues/220

#### Task Overview

Fix 15 security vulnerabilities identified by GitHub Dependabot:
- **2 Critical** severity (SSH auth bypass, Authz zero length)
- **2 High** severity (DoS, JWT excessive memory)
- **10 Moderate** severity (various crypto/network issues)
- **1 Low** severity (Docker/Moby firewall)

#### Critical Vulnerabilities

1. **golang.org/x/crypto - SSH Authorization Bypass**
   - Severity: Critical
   - CVE: Misuse of ServerConfig.PublicKeyCallback
   - Fix: Update to latest version

2. **Authz Zero Length Regression**
   - Severity: Critical
   - Fix: Identify affected package and update

3. **golang.org/x/crypto - DoS via Slow Key Exchange**
   - Severity: High
   - Fix: Update golang.org/x/crypto

4. **jwt-go - Excessive Memory Allocation**
   - Severity: High
   - Impact: jwt-go is UNMAINTAINED
   - Fix: Migrate to golang-jwt/jwt (maintained fork)

#### Recommended Approach

**Day 1: Critical/High Fixes**
1. Update `golang.org/x/crypto` to latest
   ```bash
   go get -u golang.org/x/crypto@latest
   ```

2. Migrate from `jwt-go` to `golang-jwt/jwt`
   ```bash
   # Find all imports
   grep -r "github.com/dgrijalva/jwt-go" .

   # Replace with
   go get github.com/golang-jwt/jwt/v5
   # Update all imports
   # Update code for API changes
   ```

3. Update `golang.org/x/net` to latest
   ```bash
   go get -u golang.org/x/net@latest
   ```

4. Run full test suite
   ```bash
   go test ./api/... -v
   ```

**Day 2: Moderate/Low Fixes**
5. Update Docker/Moby dependencies
6. Review all other Go dependencies
7. Run security scan

**Day 3: Verification & PR**
8. Full test suite (backend + UI)
9. Manual security testing
10. Create PR with changes

#### Acceptance Criteria

- [ ] All Critical vulnerabilities resolved (2/2)
- [ ] All High vulnerabilities resolved (2/2)
- [ ] jwt-go → golang-jwt/jwt migration complete
- [ ] All backend tests passing
- [ ] No new vulnerabilities introduced
- [ ] Security scan: 0 Critical/High issues
- [ ] Report delivered: `.claude/reports/SECURITY_VULNERABILITIES_FIXED_ISSUE_220.md`

#### Resources

- **Issue Details:** https://github.com/streamspace-dev/streamspace/issues/220
- **Wave 28 Context:** Comment on issue with detailed plan
- **Dependabot Alerts:** https://github.com/streamspace-dev/streamspace/security/dependabot
- **Related Work:** Issue #211, #212 (multi-tenancy - uses JWT heavily)

#### Deliverable

**Report:** `.claude/reports/SECURITY_VULNERABILITIES_FIXED_ISSUE_220.md`

Should include:
- List of all vulnerabilities fixed
- Before/after dependency versions
- JWT migration notes (breaking changes, code updates)
- Test results (all passing)
- Security scan results (0 Critical/High)
- Recommendations for future vulnerability management

---

### Validator (Agent 3) - Issue #200: UI Test Fixes 🚨

**Priority:** P0 - CRITICAL
**Timeline:** 2-3 days
**Branch:** `claude/v2-validator`
**GitHub Issue:** https://github.com/streamspace-dev/streamspace/issues/200

#### Task Overview

Complete UI test suite fixes started in Wave 27:
- **Current Status:** 60% complete (128 passing, 101 failing)
- **Remaining Work:** Fix 19 failing test files
- **Target:** 100% passing (277+ tests)

#### Current Test Status

**Passing (2 files):** ✅
- Some basic component tests

**Failing (19 files):** ❌

Admin Pages (15 files):
- `APIKeys.test.tsx`
- `AuditLogs.test.tsx`
- `Settings.test.tsx`
- `RBAC.test.tsx`
- `Security.test.tsx`
- `Sharing.test.tsx`
- `Users.test.tsx`
- `Recordings.test.tsx`
- `Applications.test.tsx`
- `Catalog.test.tsx`
- `Configuration.test.tsx`
- `License.test.tsx`
- `Monitoring.test.tsx`
- `SessionTemplates.test.tsx`
- `Sessions.test.tsx`

Component Tests (4 files):
- Various component test files

#### Root Causes (Identified)

1. **Deprecated Component APIs**
   - Tests use old props that no longer exist
   - Example: `onHibernate` → `onStateChange`
   - Fix: Update prop names to match current API

2. **Mock Data Mismatches**
   - Component structure changed, tests not updated
   - Missing required fields in mock objects
   - Fix: Update mock data structure

3. **Async Timing Issues**
   - `waitFor` timeouts in dialog/modal tests
   - Race conditions in state updates
   - Fix: Increase timeouts, add proper async handling

4. **Missing User Context**
   - Some tests lack authentication context
   - User/org data not properly mocked
   - Fix: Add user context to test setup

#### Recommended Approach

**Day 1: Admin Page Tests (8-10 files)**
1. Start with simplest files (APIKeys, AuditLogs)
2. Fix component prop references
3. Update mock data structure
4. Add missing user/auth context
5. Run tests incrementally
6. Fix one file at a time, verify before moving on

**Day 2: Complex Components (5-7 files)**
7. Fix dialog/modal tests (Settings, RBAC, Security)
8. Resolve async timing issues
9. Mock WebSocket connections properly
10. Fix form validation tests

**Day 3: Final Cleanup (2-4 files)**
11. Fix remaining edge case tests
12. Run full suite repeatedly
13. Ensure consistent passing
14. Create final validation report

#### Example Fix Pattern

**Before (Failing):**
```tsx
it('calls onHibernate when button clicked', () => {
  const onHibernate = vi.fn();
  render(<SessionCard session={mockSession} onHibernate={onHibernate} />);

  fireEvent.click(screen.getByRole('button', { name: /hibernate/i }));
  expect(onHibernate).toHaveBeenCalledWith(mockSession.id);
});
```

**After (Passing):**
```tsx
it('calls onStateChange with hibernated when button clicked', () => {
  const onStateChange = vi.fn();
  render(<SessionCard session={mockSession} onStateChange={onStateChange} />);

  fireEvent.click(screen.getByRole('button', { name: /hibernate/i }));
  expect(onStateChange).toHaveBeenCalledWith(mockSession.name, 'hibernated');
});
```

#### Acceptance Criteria

- [ ] All UI test files passing (21/21)
- [ ] Test results: 277+ passing, 0 failing
- [ ] No skipped tests (or documented why)
- [ ] Full test suite runs in < 60 seconds
- [ ] CI/CD green checkmark
- [ ] Report delivered: `.claude/reports/UI_TEST_FIXES_COMPLETE_ISSUE_200.md`

#### Resources

**Previous Work:**
- `.claude/reports/GEMINI_TEST_IMPROVEMENTS_2025-11-26.md` - What Gemini fixed
- `.claude/reports/TEST_FIX_REPORT_ISSUE_200.md` - Your Wave 27 progress
- `.claude/reports/WAVE_27_INTEGRATION_COMPLETE_2025-11-26.md` - Integration status

**Example Files:**
- `ui/src/components/SessionCard.test.tsx` - Example of prop updates by Gemini
- `ui/src/pages/admin/Settings.test.tsx` - Example of form validation fixes

**Test Commands:**
```bash
# Run all tests
cd ui && npm test -- --run

# Run specific test file
npm test -- --run src/pages/admin/APIKeys.test.tsx

# Run in watch mode
npm test
```

#### Deliverable

**Report:** `.claude/reports/UI_TEST_FIXES_COMPLETE_ISSUE_200.md`

Should include:
- List of all test files fixed
- Summary of changes made (prop updates, mock fixes, etc.)
- Before/after test results
- Any remaining issues or edge cases
- Recommendations for maintaining test quality

---

### Scribe (Agent 4) - STANDBY 📝

**Priority:** Low (supporting role)
**Timeline:** As needed
**Branch:** `claude/v2-scribe`
**Status:** ⏸️ Available for documentation support

#### Potential Tasks (If Time Permits)

1. **Update CHANGELOG.md**
   - Wave 27 changes (multi-tenancy, observability, DR guide)
   - Wave 28 changes (security fixes, test improvements)

2. **Refine v2.0-beta.1 Release Notes**
   - Highlight new features (multi-tenancy, observability)
   - Document breaking changes (if any from JWT migration)
   - List all issues resolved

3. **Document Vulnerability Remediation Process**
   - Based on Issue #220 work
   - SLA for vulnerability fixes (Critical: 48h, High: 7d)
   - Security scanning in CI/CD

4. **Update FEATURES.md**
   - Multi-tenancy capabilities
   - Observability dashboards
   - Disaster recovery procedures

#### Notes

- **Priority:** Only proceed if Builder/Validator request documentation
- **Do not block** release-critical work
- **Coordinate** with Architect before starting any tasks

---

### Architect (Agent 1) - Coordination 🏗️

**Status:** 🟢 ACTIVE
**Role:** Wave coordination and integration

#### Tasks Completed ✅

1. ✅ Assigned Issue #220 to Builder (agent:builder label)
2. ✅ Assigned Issue #200 to Validator (agent:validator label)
3. ✅ Added Wave 28 context comments to both issues
4. ✅ Updated MULTI_AGENT_PLAN.md with Wave 28 assignments
5. ✅ Created WAVE_28_ASSIGNMENTS report

#### Ongoing Tasks ⏳

6. ⏳ Monitor daily progress on both issues
7. ⏳ Answer questions and unblock agents as needed
8. ⏳ Integrate agent branches when ready
9. ⏳ Prepare v2.0-beta.1 release (after blockers resolved)

#### Release Preparation Checklist

After both P0 blockers resolved:

**Pre-Release:**
- [ ] All tests passing (backend + UI)
- [ ] Security scan clean (0 Critical/High)
- [ ] Manual testing complete
- [ ] CHANGELOG.md updated
- [ ] Release notes drafted
- [ ] Version bump (v2.0-beta.1)

**Release:**
- [ ] Create git tag: `v2.0-beta.1`
- [ ] Build Docker images
- [ ] Push images to registry
- [ ] Update Helm chart version
- [ ] Publish release notes on GitHub

**Post-Release:**
- [ ] Deploy to staging
- [ ] Smoke tests
- [ ] Monitor dashboards
- [ ] Notify team

---

## Parallel Work Strategy

Both P0 issues can proceed **in parallel**:

```
Day 1:
├─ Builder: golang.org/x/crypto updates, JWT migration
└─ Validator: Fix 8-10 admin page tests

Day 2:
├─ Builder: Moderate/Low severity fixes, testing
└─ Validator: Fix complex components, async issues

Day 3:
├─ Builder: Security scan, PR creation, report
└─ Validator: Final cleanup, full suite verification, report

Integration:
└─ Architect: Merge both branches, final testing, release prep
```

**No dependencies** between the two issues - can work independently.

---

## Success Metrics

### Wave 28 Goals

| Goal | Target | Current | Status |
|------|--------|---------|--------|
| Security vulnerabilities | 0 Critical/High | 2 Critical, 2 High | 🔴 TO DO |
| UI test files passing | 21/21 | 2/21 | 🔴 TO DO |
| Backend tests | All passing | ✅ 9/9 passing | ✅ DONE |
| Integration | Clean merge | N/A | ⏳ PENDING |
| v2.0-beta.1 release | Ready | Blocked | 🔴 BLOCKED |

### Definition of Done (Wave 28)

**Builder:**
- [ ] Issue #220 closed
- [ ] 0 Critical vulnerabilities
- [ ] 0 High vulnerabilities
- [ ] All backend tests passing
- [ ] Security scan report delivered

**Validator:**
- [ ] Issue #200 closed
- [ ] All UI tests passing (277+ tests)
- [ ] CI/CD green checkmark
- [ ] Test fixes report delivered

**Architect:**
- [ ] Both agent branches merged
- [ ] All tests passing (backend + UI)
- [ ] Ready for v2.0-beta.1 release

---

## Communication Plan

### Daily Check-ins

**Time:** End of day (EOD)
**Format:** Comment on assigned issue with progress update

**Template:**
```markdown
## Daily Progress Update - Day X

**Completed:**
- [ ] Task 1
- [ ] Task 2

**In Progress:**
- [ ] Task 3

**Blockers:**
- None / [describe blocker]

**Tomorrow:**
- [ ] Task 4
- [ ] Task 5

**ETA:** On track / 1 day delay / etc.
```

### Blockers & Questions

- **For technical blockers:** Comment on issue, tag @Architect
- **For urgent issues:** Escalate immediately
- **For clarifications:** Ask in issue comments

### Integration

- **When ready:** Comment on issue: "Ready for integration"
- **Architect will:** Review, merge, run tests, create integration report

---

## Risk Assessment

### Risk 1: JWT Migration Breaking Changes ⚠️

**Likelihood:** Medium
**Impact:** High (could break authentication)

**Mitigation:**
- Comprehensive testing of all auth flows
- Review all JWT usage in codebase
- Update tests to match new API
- Manual testing of login/logout/token refresh

**Owner:** Builder (Agent 2)

---

### Risk 2: UI Tests Still Failing After Fixes ⚠️

**Likelihood:** Low
**Impact:** High (blocks release)

**Mitigation:**
- Fix incrementally, verify each file
- Run full suite multiple times before declaring done
- Document any remaining issues clearly
- Escalate early if stuck

**Owner:** Validator (Agent 3)

---

### Risk 3: New Vulnerabilities Introduced 🚨

**Likelihood:** Low
**Impact:** Critical (new blockers)

**Mitigation:**
- Run security scan after all updates
- Test thoroughly before merging
- Review dependency update changelogs
- Rollback if new issues found

**Owner:** Builder (Agent 2)

---

## Related Documents

- **Wave 27 Integration:** `.claude/reports/WAVE_27_INTEGRATION_COMPLETE_2025-11-26.md`
- **Agent Updates Summary:** `.claude/reports/AGENT_UPDATES_SUMMARY_2025-11-26.md`
- **New Issues Report:** `.claude/reports/NEW_ISSUES_2025-11-26.md`
- **Multi-Agent Plan:** `.claude/multi-agent/MULTI_AGENT_PLAN.md`

---

## Timeline

```
2025-11-26 (Day 1):
├─ 14:00 - Wave 28 kickoff
├─ 14:00-18:00 - Builder: Critical vulnerability fixes
└─ 14:00-18:00 - Validator: Admin page test fixes

2025-11-27 (Day 2):
├─ 09:00-18:00 - Builder: Moderate/Low fixes, testing
└─ 09:00-18:00 - Validator: Complex component fixes

2025-11-28 (Day 3):
├─ 09:00-15:00 - Builder: Security scan, PR, report
├─ 09:00-15:00 - Validator: Final cleanup, report
└─ 15:00-18:00 - Architect: Integration

2025-11-29 (Day 4 - Buffer):
└─ 09:00-18:00 - Final testing, release prep
```

**Target Release:** 2025-11-29 EOD or 2025-12-02 (Monday)

---

**Report Complete:** 2025-11-26 14:00
**Status:** ✅ Assignments complete, agents ready to start
**Next Action:** Builder and Validator begin work on assigned issues

---

**Good luck, team! Let's ship v2.0-beta.1! 🚀**
