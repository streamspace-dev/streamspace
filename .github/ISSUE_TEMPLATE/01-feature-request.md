---
name: "✨ Feature Request"
about: "Propose a new feature aligned to wave planning"
title: "[FEATURE] "
labels: ["enhancement", "needs-triage"]
assignees: []
---

<!-- ============================================
     PLEASE FILL SECTIONS MARKED WITH (REQUIRED)
     Leave empty sections you don't need
     ============================================ -->

## Summary (REQUIRED)
<!-- Clear, one-sentence description of the feature -->


## Problem Statement (REQUIRED)
<!-- Why is this feature needed? What problem does it solve? -->


## Proposed Solution
<!-- How should this be implemented? What's the high-level approach? -->


## Scope & Components (REQUIRED)
<!-- Which components are affected? (e.g., api, ui, agents, infrastructure) -->
- Component:
- Affected Files/Modules:


## Acceptance Criteria (REQUIRED)
<!-- What must be true for this feature to be considered complete? -->
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3


## Definition of Ready (DoR) Checklist
- [ ] **Clear acceptance criteria** (no ambiguity)
- [ ] **Component mapping** (know what needs to change)
- [ ] **Size estimated** (XS/S/M/L/XL - use `/assign-size` comment)
- [ ] **Dependencies identified** (does it block or depend on other work?)
- [ ] **Agent assigned** (architect, builder, validator, scribe, security)
- [ ] **Wave planned** (which 2-3 day wave? use `/assign-wave` comment)

## Implementation Notes
<!-- Any technical guidance, existing patterns to follow, or gotchas? -->


## Testing Strategy
<!-- How should this be tested? Unit, integration, E2E? -->


## Documentation Impact
<!-- What docs need to update? CHANGELOG.md, README.md, etc. -->


## References
<!-- Links to related issues, ADRs, design docs -->
- Related:
- ADR: 
- Design Docs:


---

### For Maintainers

**To assign this issue:**
```bash
gh issue edit <number> --add-label "agent:builder"           # or validator, scribe, architect
gh issue edit <number> --add-label "size:m"                  # or xs, s, l, xl
gh issue edit <number> --add-label "wave:27"                 # current wave
gh issue edit <number> --add-label "component:backend"       # or ui, infrastructure, etc.
```

**Definition of Ready (DoR) Gate:** This issue must have agent + size + wave assigned before work begins.
