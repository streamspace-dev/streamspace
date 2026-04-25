# StreamSpace Slash Commands Reference

**Last Updated**: 2025-11-23
**Total Commands**: 27

---

## 🎯 Agent Coordination (NEW)

### `/check-work`

#### Check for assigned work by role/priority

- Shows issues assigned to your agent
- Filters by priority (P0 → P1 → P2)
- Lists ready-for-testing items (Validator)
- Checks MULTI_AGENT_PLAN.md for wave assignments

**Use when**: Starting new session, looking for next task

---

### `/signal-ready`

#### Signal work ready for testing

- Builder → Validator handoff mechanism
- Commits and pushes your work
- Posts GitHub comment with testing instructions
- Adds `ready-for-testing` label

**Use when**: Bug fix/feature complete, ready for validation

**Example**: `/signal-ready 200`

---

### `/update-issue`

#### Update GitHub issue with progress

- Progress updates
- Report blockers
- Ask questions
- Share findings
- Change status/labels

**Use when**: Need to update issue without closing it

**Example**: `/update-issue 200`

---

### `/create-issue`

#### Create new GitHub issue

- Bugs discovered during work
- New tasks identified
- Feature requests
- Auto-labels and assigns milestone

**Use when**: Discover new bug/task during work

**Example**: `/create-issue`

---

### `/sync-integration`

#### Sync integration branch to your agent branch

- Merges `feature/streamspace-v2-agent-refactor` into your branch
- Shows what's new
- Handles conflicts
- Pushes updated branch

**Use when**: Need latest work from other agents

**Example**: `/sync-integration`

---

### `/agent-status`

#### Generate status report

- Work completed today/week
- Issues closed/in-progress
- Blockers
- Next steps
- Metrics (commits, coverage, files)

**Use when**: End of day, handoff to another agent, Architect requests status

**Example**: `/agent-status` or `/agent-status week`

---

## 🔨 Code Quality

### `/review-pr`

#### Automated PR review

- Uses `@pr-reviewer` subagent
- Code quality checks (Go, TypeScript)
- Security analysis (SQL injection, XSS, secrets)
- Performance review (N+1, caching)
- Test coverage validation

**Use when**: Reviewing PRs before merge

**Example**: `/review-pr 42`

---

### `/quick-fix`

#### Fast workflow for small bug fixes

- Interactive fix session
- Automated quality checks
- Auto-commit with semantic message
- Auto-push and issue update

**Use when**: Small fix (< 50 lines, single file)

**Example**: `/quick-fix 165`

---

### `/coverage-report`

#### Comprehensive test coverage analysis

- All components (API, Agents, UI)
- Per-package breakdown
- Coverage trends
- Priority recommendations
- Generates HTML report

**Use when**: Checking coverage progress, before release

**Example**: `/coverage-report` or `/coverage-report api`

---

### `/verify-all`

#### Complete pre-commit verification

- Go tests with coverage
- UI tests with coverage
- Linting (Go, TypeScript)
- Formatting checks
- Build validation
- Uses haiku model for speed

**Use when**: Before commits, before push, pre-integration

---

### `/commit-smart`

#### Generate semantic commit messages

- Analyzes staged changes
- Generates conventional commit format
- Includes issue references
- Co-authored footer

**Use when**: Ready to commit, want standardized message

---

### `/pr-description`

#### Auto-generate PR descriptions

- Analyzes branch changes
- Lists files changed
- Summarizes modifications
- Includes testing checklist

**Use when**: Creating pull request

---

## 🧪 Testing Commands

### `/test-go [package]`

#### Run Go tests with coverage

- Runs tests for specified package (or all)
- Generates coverage report
- Shows coverage percentage
- Identifies untested code

**Example**: `/test-go ./api/internal/handlers`

---

### `/test-ui`

#### Run UI tests with coverage

- Runs Jest/React Testing Library tests
- Generates coverage report
- Shows component coverage
- Identifies missing tests

---

### `/test-integration`

#### Run integration tests

- Full E2E test suite
- Database setup
- API + Agent + UI testing
- Generates test report

---

### `/test-agent-lifecycle`

#### Test agent lifecycle

- Agent registration
- Heartbeat mechanism
- Command processing
- Graceful shutdown

---

### `/test-ha-failover`

#### Test HA failover

- Multi-pod API failover
- Agent reconnection
- Leader election
- Session survival

---

### `/test-vnc-e2e`

#### Test VNC streaming E2E

- Session creation
- VNC tunnel establishment
- Port-forward validation
- Client connectivity

---

### `/test-e2e`

#### Run Playwright E2E tests

- Full browser automation
- UI interaction testing
- Cross-browser testing (Chromium, Firefox, WebKit)
- Visual regression testing

---

## ☸️ Kubernetes Commands

### `/k8s-deploy`

#### Deploy to Kubernetes

- Applies manifests
- Helm chart deployment
- Waits for rollout
- Validates deployment

---

### `/k8s-logs [component]`

#### Fetch component logs

- API logs
- Agent logs
- Database logs
- Filters and follows

**Example**: `/k8s-logs api` or `/k8s-logs k8s-agent`

---

### `/k8s-debug`

#### Debug Kubernetes issues

- Pod status
- Events
- Resource usage
- Network connectivity

---

## 🐳 Docker Commands

### `/docker-build`

#### Build all Docker images

- API image
- K8s Agent image
- Docker Agent image
- UI image
- Tags appropriately

---

### `/docker-test`

#### Test Docker Agent locally

- Runs Docker Agent in container
- Connects to local API
- Creates test sessions
- Validates container lifecycle

---

## 🔐 Security & Maintenance

### `/security-audit`

#### Run security scans

- Dependency vulnerability scan
- Secret detection
- SAST analysis
- Generates security report

---

### `/fix-imports`

#### Fix Go/TypeScript imports

- Organizes imports
- Removes unused imports
- Groups by type (stdlib, external, internal)
- Formats correctly

---

## 🏗️ Workflow Commands

### `/integrate-agents`

#### Integrate multi-agent work (Architect only)

- Fetches all agent branches
- Shows changes from each agent
- Merges in order (Scribe → Builder → Validator)
- Updates MULTI_AGENT_PLAN.md

**Use when**: Ready to integrate wave of work

---

### `/wave-summary`

#### Generate integration summary (Architect only)

- Summarizes wave changes
- Lists files changed per agent
- Calculates metrics
- Documents integration

**Use when**: After integration, documenting wave

---

## 🎭 Agent Initialization

### `/init-architect`

#### Initialize Architect agent (Agent 1)

- Loads coordination role
- Checks agent branches
- Reviews issues and milestones
- Prepares for integration work

---

### `/init-builder`

#### Initialize Builder agent (Agent 2)

- Loads implementation role
- Checks assigned issues
- Reviews MULTI_AGENT_PLAN priorities
- Ready for feature work

---

### `/init-validator`

#### Initialize Validator agent (Agent 3)

- Loads testing/validation role
- Checks ready-for-testing issues
- Reviews test coverage
- Prepares testing environment

---

### `/init-scribe`

#### Initialize Scribe agent (Agent 4)

- Loads documentation role
- Checks documentation needs
- Reviews feature completions
- Identifies docs gaps

---

## 📊 Command Usage Guide

### Agent Workflows

**Builder Workflow**:

1. `/check-work` - Find assigned issues
2. Work on fix/feature
3. `/verify-all` - Validate changes
4. `/signal-ready <issue>` - Notify Validator
5. `/agent-status` - Report progress

**Validator Workflow**:

1. `/check-work` - Find ready-for-testing items
2. `/test-*` commands - Run tests
3. `/coverage-report` - Check coverage
4. `/update-issue <issue>` - Report results
5. Create validation reports in `.claude/reports/`

**Scribe Workflow**:

1. `/check-work` - Find documentation needs
2. Update docs based on completed features
3. `/commit-smart` - Commit documentation
4. `/agent-status` - Report progress

**Architect Workflow**:

1. `/check-work` - Review all agent work
2. `/integrate-agents` - Merge agent branches
3. `/wave-summary` - Document integration
4. `/review-pr` - Review external PRs
5. Update MULTI_AGENT_PLAN.md

---

## 🎯 Quick Reference by Task

**Starting Work:**

- `/check-work` - What should I work on?
- `/sync-integration` - Get latest from other agents

**During Work:**

- `/update-issue` - Report progress/blockers
- `/create-issue` - Track new bugs/tasks

**Completing Work:**

- `/verify-all` - Validate quality
- `/signal-ready` - Hand off to Validator
- `/agent-status` - Report completion

**Testing:**

- `/test-go`, `/test-ui`, `/test-integration` - Run tests
- `/coverage-report` - Check coverage

**Code Review:**

- `/review-pr` - Review pull request
- `/security-audit` - Check security

**Deployment:**

- `/k8s-deploy` - Deploy to cluster
- `/docker-build` - Build images

---

## 📝 Notes

- All commands use native CLI tools (`gh`, `git`, `kubectl`) instead of MCP servers
- Commands generate reports in `.claude/reports/`
- Semantic commit messages follow conventional commits spec
- Test commands use appropriate models (haiku for speed)
- Coordination commands notify relevant agents

---

**For full command details, see**: `.claude/commands/<command-name>.md`
