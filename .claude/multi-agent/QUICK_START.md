# Multi-Agent Quick Start

**Goal**: Run 4 parallel agents for StreamSpace development.

## 1. Workspaces

Ensure you have 4 terminals open in these directories:

1. **Architect**: `streamspace/` (Coordination)
2. **Builder**: `streamspace-builder/` (Implementation)
3. **Validator**: `streamspace-validator/` (Testing)
4. **Scribe**: `streamspace-scribe/` (Documentation)

## 2. Initialization Prompts

**Terminal 1: Architect**

```text
Act as Agent 1 (Architect). Read .claude/multi-agent/agent1-architect-instructions.md.
Task: Coordinate v2.0-beta. Check .claude/multi-agent/MULTI_AGENT_PLAN.md.
```

**Terminal 2: Builder**

```text
Act as Agent 2 (Builder). Read .claude/multi-agent/agent2-builder-instructions.md.
Task: Fix bugs and implement features. Check GitHub Issues.
```

**Terminal 3: Validator**

```text
Act as Agent 3 (Validator). Read .claude/multi-agent/agent3-validator-instructions.md.
Task: Test API handlers and report bugs.
```

**Terminal 4: Scribe**

```text
Act as Agent 4 (Scribe). Read .claude/multi-agent/agent4-scribe-instructions.md.
Task: Update CHANGELOG and documentation.
```

## 3. Integration Cycle

1. **Architect**: Run `/integrate-agents` to merge work.
2. **Architect**: Update `MULTI_AGENT_PLAN.md`.
3. **Agents**: Pull latest changes (`git pull`).
