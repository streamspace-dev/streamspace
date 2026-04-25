---
name: "📊 Wave Planning"
about: "Plan a 2-3 day development wave"
title: "Wave XX: [Focus Area]"
labels: ["agent:architect", "planning"]
assignees: []
---

<!-- ============================================
     WAVE PLANNING TEMPLATE
     One of these issues created per 2-3 day cycle
     ============================================ -->

## Wave Overview
**Timeline**: YYYY-MM-DD → YYYY-MM-DD (X days)
**Focus**: [Brief description of wave goals]
**Target Milestone**: [e.g., v2.0-beta.1]


## Issues in This Wave

<!-- List all issues being worked this wave with checkbox for completion -->
- [ ] #XXX: [Title]
- [ ] #XXX: [Title]
- [ ] #XXX: [Title]


## Wave Goals (DoD: Definition of Done)

### Builder Goals (Implementation)
- [ ] Feature 1 implemented
- [ ] Feature 2 implemented
- [ ] All new code tested (unit + integration)
- [ ] All commits pass `make fmt lint test`
- [ ] Branches pushed and ready for review

### Validator Goals (Testing & QA)
- [ ] All tests passing on master
- [ ] Code coverage maintained/improved (target: >70%)
- [ ] Security audit complete for P0 issues
- [ ] No regressions found
- [ ] Issues marked `ready-for-testing` all validated

### Scribe Goals (Documentation)
- [ ] CHANGELOG.md updated
- [ ] README.md reflects accurate status
- [ ] Related docs/ files created/updated
- [ ] Breaking changes documented
- [ ] API changes documented

### Architect Goals (Coordination)
- [ ] Wave started on schedule
- [ ] All issues triaged and assigned
- [ ] Daily standup conducted
- [ ] Blockers identified and escalated
- [ ] Master branch integration gates passing
- [ ] Wave completed on schedule


## Daily Standup Template

**Monday (Day 1)**
- Builder: "Working on issue #212..."
- Validator: "Starting test suite #210..."
- Scribe: "Drafting docs for #212..."
- Blockers: None

**Tuesday (Day 2)**
- Builder: "Issue #212 complete, ready for testing..."
- Validator: "Validating #212, found issue #226..."
- Scribe: "CHANGELOG updated..."
- Blockers: #212 needs security review before #211 starts

**Wednesday (Day 3)**
- Builder: "Issue #211 complete..."
- Validator: "All tests passing, coverage 75%..."
- Scribe: "Documentation complete, PR ready..."
- Blockers: None


## Blocker Management

<!-- Update this section as blockers are discovered -->

### Current Blockers
None


### Resolved This Wave
- [Date] Blocker: [Description] → Solution: [How resolved]


## Integration Plan

**Master Branch Merge Order** (prevents conflicts):
1. Scribe branch → master (docs-only)
2. Builder branch → master (implementation)
3. Validator branch → master (tests)

**CI/CD Gates Before Merge**:
- ✅ All tests passing
- ✅ Coverage maintained
- ✅ No linting errors
- ✅ Security audit approved (if P0)


## Wave Retrospective (Completed Waves Only)

### What Went Well
- 

### What Could Improve
- 

### Action Items for Next Wave
- 


---

### Architect Notes
- Created: [Date]
- Status: PLANNED / IN PROGRESS / COMPLETED
- Next Wave: [Link to next wave issue]
