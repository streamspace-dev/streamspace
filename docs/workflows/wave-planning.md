---
title: Wave Planning & Execution Roadmap
description: 2-3 day development waves organized toward v2.0-beta.1 release
---

# Wave Planning & Roadmap

**Current Status**: Wave 27 IN PROGRESS (2025-11-26 → 2025-11-28)  
**Target Release**: v2.0-beta.1 (2025-12-14)  
**Total Timeline**: 18 days (~6 waves)

---

## Wave 27: Org Context & Security Hardening ⚡

**Status**: 🔴 **IN PROGRESS**  
**Timeline**: 2025-11-26 → 2025-11-28 (2 days)  
**Focus**: P0 multi-tenancy security fixes  
**Milestone**: [#223](https://github.com/streamspace-dev/streamspace/issues/223)

### Issues in This Wave

| # | Title | Agent | Size | Status |
|---|-------|-------|------|--------|
| #212 | Org context and RBAC plumbing for API and WebSockets | Builder | L | `wave:27` `P0` |
| #211 | WebSocket org scoping and auth guard | Builder | M | `wave:27` `P0` `status:blocked` |
| #208 | Docker Agent Test Suite (v2.0 P0) | Validator | L | `wave:27` `P0` |
| #200 | Fix Broken Test Suites - API, K8s Agent, UI | Validator | M | `wave:27` `P0` |

### Definition of Done (DoD) Checklist

**Builder Deliverables:**
- [ ] JWT org_id claims implemented (auth service)
- [ ] Auth middleware extracts org_id to Gin context
- [ ] All API handlers validate org scoping
- [ ] WebSocket handlers validate org authorization
- [ ] All new code tested (unit + integration)
- [ ] Test coverage >70% for auth components
- [ ] Ready for validation (ready-for-testing label added)

**Validator Deliverables:**
- [ ] Org isolation tests passing (cross-org rejection verified)
- [ ] WebSocket cross-org tests passing
- [ ] Docker agent test suite passing
- [ ] Broken test suites fixed
- [ ] Overall coverage >70%
- [ ] No P0 regressions
- [ ] Security audit complete (org context)

**Scribe Deliverables:**
- [ ] CHANGELOG.md updated (org context feature entry)
- [ ] docs/MULTI_TENANCY.md created
- [ ] SECURITY.md org isolation section added
- [ ] README.md updated with security note

**Architect Goals:**
- [ ] Daily standup conducted
- [ ] Blockers identified & resolved
- [ ] Master integration gates passing
- [ ] Wave completed on schedule
- [ ] Wave retrospective documented

### Daily Progress

**Monday 2025-11-26 (Start)**
- [ ] Wave issues assigned to agents
- [ ] Blocker mitigation plan established
- [ ] Builder starts on #212 (blocker for #211)
- [ ] Validator begins breaking test suite triage
- [ ] Scribe reviews design docs for multi-tenancy

**Tuesday 2025-11-27 (Mid-point)**
- [ ] #212 implementation ~80% complete
- [ ] #200 tests fixed
- [ ] #208 test suite foundation written
- [ ] Security implications documented
- [ ] Scribe has CHANGELOG draft

**Wednesday 2025-11-28 (Completion Target)**
- [ ] All issues reach ready-for-testing
- [ ] Validation complete
- [ ] Master merge gates passing
- [ ] Documentation final
- [ ] Retrospective: Proceed to Wave 28?

### Blocker Mitigation

**Identified Dependencies:**
- #211 **blocked by** #212 (org context must be implemented first)
- #208 depends on #200 (broken tests must be fixed)

**Escalation Path:**
1. Daily standup identifies blockers
2. Architect notifies affected agent
3. Re-prioritize wave if needed
4. Document decision in wave issue comments

**If Delayed:**
- Move #211 to Wave 28 (non-critical if #212 doesn't complete)
- Push #208 to Wave 28 if not critical for release
- Keep #200 (fixing broken tests is non-negotiable)

---

## Wave 28: Testing & v2.0-beta.1 Release Prep

**Status**: 🟡 **PLANNED** (starts 2025-11-29)  
**Timeline**: 2025-11-29 → 2025-12-01 (3 days)  
**Focus**: Test coverage, release documentation  
**Milestone**: [#224](https://github.com/streamspace-dev/streamspace/issues/224)

### Issues Planned

| # | Title | Agent | Size |
|---|-------|-------|------|
| #204 | API Handler & Middleware Coverage (4% → 40%) | Validator | L |
| #210 | Integration & E2E Test Suite (v2.0 P1) | Validator | L |
| #187 | Create OpenAPI/Swagger Specification | Scribe | M |
| #219 | Surface contribution workflow and DoR/DoD in repo | Scribe | S |
| #220 | [SECURITY] Address Dependabot Vulnerability Alerts | Validator | L |

### Expected Outcomes
- ✅ API test coverage >70%
- ✅ UI test suite >70%
- ✅ Integration tests passing
- ✅ OpenAPI spec generated
- ✅ CONTRIBUTING.md updated with DoD
- ✅ Security audit sign-off for Dependabot
- ✅ Release notes drafted

---

## Wave 29: Performance & Final Hardening

**Status**: 🔵 **PLANNED** (starts 2025-12-02)  
**Timeline**: 2025-12-02 → 2025-12-05 (3 days)  
**Focus**: Performance, stability, final validation  
**Milestone**: [#225](https://github.com/streamspace-dev/streamspace/issues/225)

### Issues Planned

| # | Title | Agent | Size |
|---|-------|-------|------|
| #214 | Implement cache strategy with keys/TTLs/metrics | Builder | M |
| #213 | Standardize API pagination and error envelopes | Builder | M |
| #169 | Add Load Testing with k6 | Validator | M |
| #205 | Integration Test Suite - HA, VNC, Multi-Platform | Validator | L |

### Expected Outcomes
- ✅ Load tests: <200ms p99 latency
- ✅ Cache strategy operational
- ✅ API error standardization complete
- ✅ HA tests passing
- ✅ All P0 issues resolved
- ✅ Ready for v2.0-beta.1 release (2025-12-14)

---

## Backlog Beyond v2.0-beta.1

### Wave 30+ (Post-Release)

**v2.1 Features** (lower priority):
- Plugin system enhancements (#185, #184, #186)
- Advanced filtering & sorting (#171)
- CLI tool (#193)
- VS Code extension (#194)

**Ongoing Improvements:**
- Performance optimization (#195)
- Cost attribution (#191)
- Feature flags (#192)

---

## Key Metrics

### Velocity Tracking

| Wave | Start Date | Issues | Planned | Completed | Velocity |
|------|-----------|--------|---------|-----------|----------|
| 27 | 2025-11-26 | 4 | TBD | - | - |
| 28 | 2025-11-29 | 5 | TBD | - | - |
| 29 | 2025-12-02 | 4 | TBD | - | - |

### Quality Metrics

| Metric | Target | Wave 27 | Wave 28 | Status |
|--------|--------|---------|---------|--------|
| Test Coverage | >70% | - | - | 🔄 |
| P0 Issues | 0 | 4 active | 1 planned | 🟡 |
| Cycle Time | <2 days/issue | - | - | 🔄 |
| Blocker Ratio | <10% | 1/4 = 25% | - | 🔴 |

---

## Integration & Merge Schedule

### Wave 27 Integration (2025-11-28 EOD)

```
Thursday 2025-11-28:

1. Final CI/CD validation
   - All tests passing on feature branches
   - Coverage reports reviewed
   - Security audit sign-off

2. Merge Order (prevents conflicts):
   a) Scribe branch → master (docs-only changes)
   b) Builder branch → master (implementation)
   c) Validator branch → master (tests)

3. Post-merge:
   - Close completed issues
   - Update milestone progress
   - Plan Wave 28 kickoff
   - Document retrospective
```

### Master Branch Gates

**Before merging to master, verify:**
- ✅ All tests passing locally AND on CI
- ✅ Code coverage maintained (no decrease)
- ✅ No linting errors
- ✅ Semantic commit messages
- ✅ CHANGELOG.md updated
- ✅ Security review complete (if P0)
- ✅ Architect approval

---

## Daily Standup Template

Use this in GitHub issue comments or Slack daily:

```markdown
### Wave 27 Standup - [Date]

**Builder (@builder)**
- Yesterday: Implemented JWT org_id claims; tests passing
- Today: Adding org_id extraction to middleware
- Blocker: None
- ETA for #212: Tomorrow EOD

**Validator (@validator)**
- Yesterday: Fixed 20 broken API tests
- Today: Writing org isolation cross-org rejection tests
- Blocker: Waiting for #212 (ready-for-testing)
- ETA for #200: Today

**Scribe (@scribe)**
- Yesterday: Reviewed multi-tenancy design docs
- Today: Drafting CHANGELOG entry; creating MULTI_TENANCY.md
- Blocker: None
- Ready: Can finalize docs once #212 complete

**Architect (@architect)**
- Yesterday: Set up Wave 27; assigned all issues
- Today: Monitoring progress; no blockers identified
- Action: Daily sync confirmed 4pm PT
- Next: Plan Wave 28 if 27 tracking to complete
```

---

## Adjustment Protocol

If Wave Falls Behind:

1. **Identify**: Daily standup exposes delays
2. **Assess**: How much time lost? Can we catch up?
3. **Decide**: 
   - Extend wave by 1 day? (shifts everything back)
   - De-scope issue? (move to Wave 28)
   - Add resources? (pull from Wave 28)
4. **Communicate**: Update wave issue with rationale
5. **Document**: Note in retrospective for next wave

**Recent Adjustments**: None yet (Wave 27 just started)

---

## References

- **Wave Planning Template**: [.github/ISSUE_TEMPLATE/03-wave-planning.md](.github/ISSUE_TEMPLATE/03-wave-planning.md)
- **GitHub Workflow**: [github-workflow.md](GITHUB_WORKFLOW.md)
- **Zencoder Rules**: [.zencoder/rules/](./zencoder/rules/)
- **Milestones**: [GitHub Milestones](https://github.com/streamspace-dev/streamspace/milestones)
- **Current Issues**: [Wave 27 Issues](https://github.com/streamspace-dev/streamspace/issues?q=label%3Awave%3A27)

---

**Last Updated**: 2025-11-26  
**Next Review**: 2025-11-27 (daily standup)  
**Owner**: @architect
