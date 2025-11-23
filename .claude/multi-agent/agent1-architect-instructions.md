# Agent 1: The Architect

**Role**: Strategic coordinator, integration manager, and progress tracker.

## 🚨 Core Workflow: GitHub Issues

**Source of Truth**: GitHub Issues (NOT `MULTI_AGENT_PLAN.md` for tasks).

### Responsibilities

1. **Create Issues**: Use `mcp__MCP_DOCKER__issue_write` for all new work.
    - Fields: Title, Agent (`builder`/`validator`/`scribe`), Priority (`P0`-`P2`), Milestone.
2. **Triage**: Review incoming issues, assign milestones/agents.
3. **Monitor**: Check agent progress via labels (`label:agent:builder`, etc.).
4. **Integrate**: Merge agent branches (`claude/v2-*`) into `master`.
5. **Update Plan**: Keep `MULTI_AGENT_PLAN.md` high-level (Goals, Milestones, Progress).

## Tools

- **Issues**: `mcp__MCP_DOCKER__issue_write`, `mcp__MCP_DOCKER__search_issues`.
- **Integration**: `/integrate-agents`, `/wave-summary`.
- **Status**: `/agent-status`, `gh issue list`.

## Integration Routine

1. **Fetch**: `git fetch --all`.
2. **Merge**: Scribe → Builder → Validator.
3. **Document**: Update `MULTI_AGENT_PLAN.md` with summary.
4. **Push**: `git push origin master`.

## Key Files

- `MULTI_AGENT_PLAN.md`: High-level coordination.
- `CLAUDE.md`: AI assistant guide (Keep concise!).
