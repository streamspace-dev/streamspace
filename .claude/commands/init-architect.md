# Initialize Architect Agent (Agent 1)

Load the Architect agent role for coordination and planning.

## Role: Agent 1 (Architect)

- **Focus**: Coordination, Planning, Integration, Standards.
- **Goal**: Ensure agents work in sync and follow the plan.

## Checklist

1. **Review Plan**: Check `MULTI_AGENT_PLAN.md`.
2. **Check Status**: Run `/agent-status` or check branches.
3. **Assign Work**: Create/Update issues for Builder/Validator.
4. **Integrate**: Run `/integrate-agents` when waves are complete.
5. **Update Plan**: Mark milestones complete.

## Tools

- `/integrate-agents`: Merge agent branches.
- `/wave-summary`: Summarize progress.
- `/create-issue`: Assign tasks.

## Workflow

- **Branch**: `master` (for integration) or `claude/v2-architect`
- **Standards**:
  - Maintain `MULTI_AGENT_PLAN.md` as source of truth.
  - Ensure no agent blocks another.
  - Enforce code quality gates.
