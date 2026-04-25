# Agent Status Report

Generate a status report for your agent showing progress, blockers, and next steps.

**Use this when**: End of day, before handoff to another agent, or when Architect requests status.

## Usage

Run without arguments: `/agent-status`

Or specify date range: `/agent-status today` or `/agent-status week`

## What This Does

Generates comprehensive status report including:

1. **Work Completed** (from git commits today/this week)
2. **Issues Closed** (GitHub issues you closed)
3. **Issues In Progress** (Issues assigned to you, status updates)
4. **Blockers** (Issues blocking your work)
5. **Next Steps** (Planned work for next session)
6. **Metrics** (Lines changed, files modified, test coverage)

## Output Format

Creates report in `.claude/reports/AGENT_STATUS_<role>_<date>.md`:

```markdown
# Agent Status Report: Builder

**Date**: 2025-11-23
**Agent**: Builder (Agent 2)
**Branch**: claude/v2-builder

## 📊 Summary

- **Issues Closed**: 2 (#134, #135)
- **Issues In Progress**: 1 (#200)
- **Commits**: 8 commits
- **Files Changed**: 15 files (+456/-89 lines)
- **Tests Added**: 12 tests
- **Test Coverage**: 42% → 47% (+5%)

## ✅ Work Completed Today

### Issue #134: P1-MULTI-POD-001 (AgentHub Multi-Pod Support)
- ✅ Implemented Redis-backed AgentHub
- ✅ Added cross-pod command routing
- ✅ Deployed Redis to chart/
- ✅ Validated by Validator
- **Status**: CLOSED

### Issue #135: P1-SCHEMA-002 (Missing updated_at Column)
- ✅ Created migration 004
- ✅ Added trigger function
- ✅ Backfilled existing rows
- ✅ Validated by Validator
- **Status**: CLOSED

## 🔄 In Progress

### Issue #200: Fix Broken Test Suites (P0)
- ⏳ Fixed API handler test mocks (70% complete)
- ⏳ Investigating PostgreSQL array handling
- **Blocker**: Need test database setup clarification
- **ETA**: 4 hours

## 🚧 Blockers

1. **Issue #200**: Missing test database configuration
   - **Impact**: Cannot complete API handler test fixes
   - **Needs**: Architect decision on test DB approach
   - **Priority**: P0

## 📈 Metrics

### Commits (Last 24 Hours)
- 8 commits to `claude/v2-builder`
- Files changed: 15 (+456/-89)
- Average commit size: 68 lines

### Test Coverage
- Before: 42%
- After: 47%
- Change: +5%
- Tests added: 12

### Issues
- Closed: 2
- In Progress: 1
- Opened: 0

## 🎯 Next Steps

1. **Immediate** (Next Session):
   - Resolve Issue #200 blocker with Architect
   - Complete API handler test fixes
   - Run test suite validation

2. **Short Term** (Next 1-2 Days):
   - Issue #201: Create Docker Agent tests
   - Issue #163: Implement rate limiting

3. **Waiting On**:
   - Architect: Test DB configuration decision
   - Validator: Feedback on #200 partial fixes

## 💬 Notes

- Good progress on P1 fixes - both validated and closed
- Test infrastructure issues more extensive than expected
- May need to break Issue #200 into smaller tasks

## 🔗 References

- Branch: `claude/v2-builder`
- Reports: `.claude/reports/BUG_REPORT_P1_*.md`
- Next Integration: Wave 23 (estimated tomorrow)

---
🤖 Generated via `/agent-status` command
```

## Auto-Post to GitHub

The command can optionally:
1. Post summary as comment on milestone issue
2. Update agent coordination issue
3. Share in team discussion

## Use Cases

- **Daily Standup**: Quick status for Architect
- **Handoff**: Context for next agent session
- **Weekly Review**: Progress tracking
- **Blocker Escalation**: Highlight what's blocking you
