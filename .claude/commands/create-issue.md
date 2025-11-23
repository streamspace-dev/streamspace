# Create GitHub Issue

Create a new GitHub issue for bugs, features, or tasks discovered during your work.

**Use this when**: You discover a new bug, identify needed work, or want to track a task.

## Usage

Run the command and it will guide you through issue creation.

Example: `/create-issue`

## Interactive Prompts

### 1. Issue Type

- **Bug**: Something broken or not working as expected
- **Feature**: New functionality needed
- **Task**: Work item (testing, documentation, refactoring)
- **Question**: Need clarification or discussion

### 2. Priority

- **P0 CRITICAL**: Blocks release, production down, security vulnerability
- **P1 HIGH**: Important for release, affects multiple users
- **P2 MEDIUM**: Nice to have, affects some users
- **P3 LOW**: Future work, minor issue

### 3. Basic Information

- **Title**: Clear, concise summary (will be auto-prefixed with type)
- **Description**: Detailed explanation
- **Component**: Which part of system (API, K8s Agent, Docker Agent, UI, etc.)
- **Agent assignment**: Which agent should handle this?

### 4. Additional Context

- **Steps to reproduce** (for bugs)
- **Expected behavior** (for bugs)
- **Actual behavior** (for bugs)
- **Acceptance criteria** (for features/tasks)
- **Related issues**: Dependencies or related work

### 5. Metadata

- **Milestone**: Which release? (v2.0-beta.1, v2.0-beta.2, v2.1.0, etc.)
- **Labels**: Auto-assigned based on type/priority/component/agent
- **Assignee**: (Optional) specific GitHub user

## Example Output

For a bug:

```text
Title: [BUG] WebSocket connection drops after 5 minutes

Labels: bug, P1, component:api, agent:builder

Description:
## Problem
WebSocket connections from K8s Agent to API drop after exactly 5 minutes.

## Steps to Reproduce
1. Start K8s Agent
2. Connect to API
3. Wait 5 minutes
4. Connection drops with error: "websocket: close 1006"

## Expected Behavior
WebSocket should stay connected indefinitely with heartbeats.

## Actual Behavior
Connection drops at 5:00 mark consistently.

## Impact
- Agent reconnects but loses 5-10 seconds
- In-flight commands may fail
- Session provisioning delayed

## Environment
- API: v2.0-beta
- K8s Agent: v2.0-beta
- Kubernetes: 1.28

## Possible Cause
Nginx/Load balancer timeout = 300s (5 minutes)

## Suggested Fix
1. Reduce heartbeat interval to 30s
2. Add WebSocket ping/pong
3. Configure load balancer for longer timeout

## Related Issues
- #156 (Agent heartbeat mechanism)

---
🤖 Created by Builder via `/create-issue` command
```

## Execution Steps

The agent will:

1. **Collect Information**: Ask for any missing details.
2. **Create Issue**: Use `gh issue create` with the formatted title and body.

   ```bash
   gh issue create --title "[TYPE] Title" --body "Description..." --label "bug,P1" --assignee "@me"
   ```

3. **Update Plan**: Add the new issue to `MULTI_AGENT_PLAN.md`.
4. **Create Report**: For P0/P1 issues, create a file in `.claude/reports/`.

## Validation

Before running the command, the agent will:

- Check for duplicate issues using `gh issue list --search "query"`
- Validate required fields
