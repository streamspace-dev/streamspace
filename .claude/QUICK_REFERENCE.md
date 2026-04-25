# Multi-Agent Orchestration - Quick Reference

**Status:** v1.0.0 REFACTOR-READY | v2.0 Architecture Refactor In Progress

## Current Agent Branches

```
Architect:  claude/audit-streamspace-codebase-011L9FVvX77mjeHy4j1Guj9B
Builder:    claude/setup-agent2-builder-01H8U2FdjPrj3ee4Hi3oZoWz
Validator:  claude/setup-agent3-validator-01GL2ZjZMHXQAKNbjQVwy9xA
Scribe:     claude/setup-agent4-scribe-019staDXKAJaGuCWQWwsfVtL
```

## Starting Agents (Every Session)

**All Agents Read First:**
```bash
# Check current status
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | head -100

# Check your role
cat .claude/multi-agent/agent[X]-[role]-instructions.md
```

**Agent-Specific Start Commands:**

**Architect:**
```
Act as Agent 1 (Architect) for StreamSpace v2.0 refactor.
Read: .claude/multi-agent/agent1-architect-instructions.md
Read: .claude/multi-agent/MULTI_AGENT_PLAN.md
Current focus: Coordinate v2.0 multi-platform refactor.
```

**Builder:**
```
Act as Agent 2 (Builder) for StreamSpace.
Read: .claude/multi-agent/agent2-builder-instructions.md
Read: .claude/multi-agent/MULTI_AGENT_PLAN.md
Check for assigned tasks in plan.
```

**Validator:**
```
Act as Agent 3 (Validator) for StreamSpace.
Read: .claude/multi-agent/agent3-validator-instructions.md
Read: .claude/multi-agent/MULTI_AGENT_PLAN.md
Continue API handler tests (non-blocking).
```

**Scribe:**
```
Act as Agent 4 (Scribe) for StreamSpace.
Read: .claude/multi-agent/agent4-scribe-instructions.md
Read: .claude/multi-agent/MULTI_AGENT_PLAN.md
Document refactor progress.
```

## Current Focus: v2.0 Multi-Platform Refactor

### What We're Building

**From:** Kubernetes-native (single cluster)
**To:** Multi-platform Control Plane + Agents (K8s, Docker, VM, Cloud)

### Implementation Phases

```
✅ Phase 1: Design & Documentation (complete)
🔄 Phase 2: Agent Registration API (Builder working)
⏳ Phase 3: WebSocket Command Channel
⏳ Phase 4: VNC Proxy/Tunnel
⏳ Phase 5: K8s Agent Conversion
⏳ Phase 6: K8s Agent VNC Tunneling
⏳ Phase 7: Docker Agent
⏳ Phase 8: UI Updates (Admin UI focus)
✅ Phase 9: Database Schema (complete)
⏳ Phase 10: Testing & Migration
```

**See:** `docs/REFACTOR_ARCHITECTURE_V2.md`

## Common Commands

### Check Current Status

```bash
# What's happening now?
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | grep -A 10 "Current Status"

# What phase are we on?
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | grep -A 5 "IN PROGRESS"

# What's assigned to Builder?
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | grep -B 5 -A 30 "Assigned To: Builder"
```

### Check Tasks

```bash
# All tasks
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | grep -A 5 "### Task:"

# Recent updates
tail -100 .claude/multi-agent/MULTI_AGENT_PLAN.md
```

### View Agent Activity

```bash
# Recent commits
git log --oneline --graph --all | head -20

# What changed on Builder branch?
git log --oneline claude/setup-agent2-builder-01H8U2FdjPrj3ee4Hi3oZoWz | head -10

# Compare branches
git diff claude/audit-streamspace-codebase-011L9FVvX77mjeHy4j1Guj9B..claude/setup-agent2-builder-01H8U2FdjPrj3ee4Hi3oZoWz
```

## v2.0 Refactor Quick Commands

### Check Architecture Docs

```bash
# Main architecture
cat docs/REFACTOR_ARCHITECTURE_V2.md | head -200

# Database schema
grep -A 30 "v2.0 Architecture" api/internal/db/database.go

# Models
cat api/internal/models/agent.go | head -100
```

### Check Implementation Progress

```bash
# Agent Registration API (Phase 2)
ls -la api/internal/handlers/agents*

# Database tables
psql streamspace -c "\d agents"
psql streamspace -c "\d agent_commands"

# Test coverage
find . -name "*agent*test*"
```

### Architect Integration Commands

```bash
# Pull Builder work
git fetch origin claude/setup-agent2-builder-01H8U2FdjPrj3ee4Hi3oZoWz
git merge --no-ff origin/claude/setup-agent2-builder-01H8U2FdjPrj3ee4Hi3oZoWz

# Pull Validator work
git fetch origin claude/setup-agent3-validator-01GL2ZjZMHXQAKNbjQVwy9xA
git merge --no-ff origin/claude/setup-agent3-validator-01GL2ZjZMHXQAKNbjQVwy9xA

# Pull Scribe work
git fetch origin claude/setup-agent4-scribe-019staDXKAJaGuCWQWwsfVtL
git merge --no-ff origin/claude/setup-agent4-scribe-019staDXKAJaGuCWQWwsfVtL

# Update plan and push
git add .claude/multi-agent/MULTI_AGENT_PLAN.md
git commit -m "feat(architect): Integrate agent work"
git push origin claude/audit-streamspace-codebase-011L9FVvX77mjeHy4j1Guj9B
```

## Task Status Format

```markdown
### Task: [Name]
- **Assigned To:** [Agent]
- **Status:** [Pending | In Progress | Complete | Blocked]
- **Priority:** [P0 | P1 | P2]
- **Duration:** [estimate]
- **Dependencies:** [List or "None"]
- **Notes:**
  - [Implementation details]
  - [Progress updates]
  - [Blockers]
- **Last Updated:** [Date] - [Agent]
```

## Message Format (in MULTI_AGENT_PLAN.md)

```markdown
## [From Agent] → [To Agent] - [Timestamp]
[Message content with clear action items]

**Deliverables:**
- Item 1
- Item 2

**Status:** [What's done]
**Next:** [What's next]
```

## Typical v2.0 Workflow

1. **Architect** defines phase and assigns to Builder
2. **Builder** implements API/backend/UI changes
3. **Builder** writes unit tests
4. **Builder** notifies Architect when complete
5. **Validator** tests integration (parallel work)
6. **Architect** reviews and merges to coordination branch
7. **Scribe** documents changes
8. **Repeat for next phase**

## Key Files to Monitor

### For All Agents
- `.claude/multi-agent/MULTI_AGENT_PLAN.md` - **SOURCE OF TRUTH**
- `.claude/multi-agent/agent[X]-instructions.md` - Your role guide
- `docs/REFACTOR_ARCHITECTURE_V2.md` - v2.0 architecture
- `CHANGELOG.md` - Version history

### For Builder
- `api/internal/models/agent.go` - v2.0 models
- `api/internal/db/database.go` - Database schema
- `api/internal/handlers/agents.go` - Agent management API
- Existing patterns in `api/internal/handlers/*.go`
- Test patterns in `api/internal/handlers/*_test.go`

### For Validator
- `docs/TESTING_GUIDE.md` - Testing patterns
- Test files to create/update
- API handler tests (59 remaining)

### For Scribe
- `CHANGELOG.md` - Update with each phase
- Architecture docs to update
- Implementation guides

## Emergency Commands

### Agent Lost Context

```bash
# Re-read your role
cat .claude/multi-agent/agent[X]-[role]-instructions.md

# Re-read current status
cat .claude/multi-agent/MULTI_AGENT_PLAN.md | head -200

# Check what you were working on
git log --oneline -20
```

### Check What Changed Since Last Session

```bash
# Recent commits on your branch
git log --oneline -10

# What files changed?
git diff HEAD~5

# What's new in the plan?
git diff HEAD~1 .claude/multi-agent/MULTI_AGENT_PLAN.md
```

### Builder Checklist (Before Notifying Architect)

- [ ] Implementation complete
- [ ] Unit tests written (>70% coverage)
- [ ] All tests passing (`go test ./...` or `npm test`)
- [ ] Code follows existing patterns
- [ ] Documentation comments added
- [ ] Updated MULTI_AGENT_PLAN.md with completion status
- [ ] Committed and pushed to branch
- [ ] No merge conflicts with main branch

## Integration Checklist (Architect Only)

- [ ] Pull all agent branches
- [ ] Review changes (read commits, check code quality)
- [ ] Merge in order: Scribe → Builder → Validator
- [ ] Resolve any conflicts
- [ ] Run tests to verify integration
- [ ] Update MULTI_AGENT_PLAN.md with integration summary
- [ ] Commit and push to coordination branch
- [ ] Notify agents of integration completion

## Remember

### All Agents
- ✅ Read MULTI_AGENT_PLAN.md at session start
- ✅ Update status when completing tasks
- ✅ Leave clear messages for other agents
- ✅ Commit frequently with descriptive messages
- ✅ Push to your branch regularly

### Builder
- ✅ Follow existing code patterns
- ✅ Write unit tests alongside code
- ✅ Run tests before pushing
- ✅ Update MULTI_AGENT_PLAN.md with progress

### Validator
- ✅ Test immediately when Builder completes
- ✅ Report bugs clearly with reproduction steps
- ✅ Continue API handler tests (non-blocking)

### Scribe
- ✅ Document as changes are merged
- ✅ Update CHANGELOG.md with each phase
- ✅ Keep architecture docs current

### Architect
- ✅ Coordinate all agents
- ✅ Don't implement code (assign to Builder)
- ✅ Integrate completed work regularly
- ✅ Maintain MULTI_AGENT_PLAN.md as source of truth

## Current Priorities

**Phase 2: Agent Registration API** (Builder working)
- Duration: 3-5 days
- Files: `api/internal/handlers/agents.go`, tests
- 5 HTTP endpoints for agent management
- Unit tests >70% coverage

**Next Up:**
- Phase 3: WebSocket Command Channel
- Phase 4: VNC Proxy/Tunnel
- Phase 8: UI Updates (Admin UI)

## Success Metrics

**v1.0.0 Achieved:**
- ✅ 82%+ completion
- ✅ 11,131 lines tests, 464 cases
- ✅ 6,700+ lines documentation
- ✅ 7/7 admin features complete
- ✅ REFACTOR-READY status

**v2.0 Target:**
- Multi-platform support (K8s, Docker, VM, Cloud)
- Control Plane + Agent architecture
- VNC tunneling through Control Plane
- WebSocket-based agent communication
- Comprehensive admin UI for agents

## Need Help?

1. **Check MULTI_AGENT_PLAN.md** - Current status and tasks
2. **Read your agent instructions** - Role-specific guidance
3. **Review architecture docs** - `docs/REFACTOR_ARCHITECTURE_V2.md`
4. **Check existing patterns** - Look at similar files in codebase
5. **Ask Architect** - Coordination questions

---

**Last Updated:** 2025-11-21
**Status:** v2.0 Phase 2 In Progress
**Builder Task:** Agent Registration API (5 endpoints + tests)
