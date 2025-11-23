# Recommended Claude Code Tools for StreamSpace

**Created**: 2025-11-23
**For**: StreamSpace v2.0+ Development
**Based on**: Research of best practices and community tools

---

## Overview

This document provides curated recommendations for **Slash Commands**, **Agent Skills**, **Subagents**, and **Plugins** specifically tailored for StreamSpace's multi-platform container streaming development.

**Project Context**:
- **Tech Stack**: Go (API + Agents), React/TypeScript (UI), Kubernetes, Docker
- **Architecture**: Control Plane + Multi-platform Agents (K8s + Docker)
- **Testing Needs**: Unit, Integration, E2E (critical gap identified)
- **Multi-Agent Workflow**: Architect, Builder, Validator, Scribe

---

## 🎯 Recommended Slash Commands

### Essential Development Commands

#### 1. Testing & Quality Assurance

**`/test-go` - Run Go Tests with Coverage**
```markdown
# .claude/commands/test-go.md

Run Go tests for the specified package or all packages if none specified.

!cd api && go test $ARGUMENTS -v -coverprofile=coverage.out -covermode=atomic

After running tests:
1. Show test results summary
2. Calculate coverage percentage
3. Identify untested packages
4. Suggest areas needing tests

If tests fail, analyze failures and suggest fixes.
```

**`/test-ui` - Run React Tests**
```markdown
# .claude/commands/test-ui.md

Run UI tests with coverage reporting.

!cd ui && npm test -- --coverage --run $ARGUMENTS

After running tests:
1. Show test results (passed/failed)
2. Report coverage percentages
3. Identify components without tests
4. Suggest test improvements

If tests fail, fix import errors and component issues.
```

**`/test-integration` - Run Integration Tests**
```markdown
# .claude/commands/test-integration.md

Run integration tests for v2.0-beta features.

!cd tests/integration && go test -v $ARGUMENTS

Focus on:
- Multi-pod API deployment
- Agent failover scenarios
- VNC streaming E2E
- Cross-platform operations

Report results in .claude/reports/INTEGRATION_TEST_*.md format.
```

**`/verify-all` - Complete Pre-Commit Verification**
```markdown
# .claude/commands/verify-all.md
model: haiku

Run all verification checks before committing:

!cd api && go test ./... && go vet ./... && golint ./...
!cd ui && npm run lint && npm test -- --run
!cd agents/k8s-agent && go test ./...
!cd agents/docker-agent && go test ./...

Success criteria:
- ✅ All tests passing
- ✅ No linting errors
- ✅ No type errors
- ✅ Build succeeds

If any check fails, fix issues before allowing commit.
```

---

#### 2. Git & Version Control

**`/commit-smart` - Generate Semantic Commit**
```markdown
# .claude/commands/commit-smart.md

Analyze staged changes and create a semantic commit message.

!git diff --staged

Generate commit message following this format:
- Type: feat, fix, docs, test, refactor, chore
- Scope: api, k8s-agent, docker-agent, ui, etc.
- Description: Clear, concise summary
- Body: Bullet points for significant changes
- Footer: References to issues, breaking changes

Include StreamSpace footer:
🤖 Generated with [Claude Code](https://claude.com/claude-code)
Co-Authored-By: Claude <noreply@anthropic.com>

DO NOT commit automatically - show message for review first.
```

**`/pr-description` - Generate PR Description**
```markdown
# .claude/commands/pr-description.md

Generate comprehensive PR description from branch commits.

!git log main..HEAD --oneline
!git diff main...HEAD --stat

Create PR description with:
## Summary
- High-level overview of changes

## Changes
- Detailed bullet points by component

## Testing
- Test coverage changes
- Integration tests added
- Manual testing performed

## Checklist
- [ ] Tests passing
- [ ] Documentation updated
- [ ] No breaking changes (or documented)

Include relevant issue references.
```

---

#### 3. Kubernetes Operations

**`/k8s-deploy` - Deploy to Kubernetes**
```markdown
# .claude/commands/k8s-deploy.md

Deploy StreamSpace to Kubernetes cluster.

Verify cluster connectivity:
!kubectl cluster-info

Deploy components:
!kubectl apply -f manifests/

Check deployment status:
!kubectl get pods -n streamspace
!kubectl get services -n streamspace

Verify:
- All pods running
- Services accessible
- Agents connected to API

If issues found, troubleshoot and fix.
```

**`/k8s-logs` - Fetch Component Logs**
```markdown
# .claude/commands/k8s-logs.md

Fetch logs from StreamSpace components.

$ARGUMENTS should specify: api, k8s-agent, docker-agent, postgres, or redis

!kubectl logs -n streamspace -l app.kubernetes.io/component=$ARGUMENTS --tail=100

Analyze logs for:
- Errors or warnings
- Performance issues
- Connection problems
- Authentication failures

Suggest fixes for any issues found.
```

**`/k8s-debug` - Debug Kubernetes Issues**
```markdown
# .claude/commands/k8s-debug.md

Debug Kubernetes deployment issues.

!kubectl get all -n streamspace
!kubectl describe pods -n streamspace | grep -A 10 "Events:"
!kubectl get events -n streamspace --sort-by='.lastTimestamp'

Common issues to check:
- Image pull failures
- CrashLoopBackOff
- Resource constraints
- ConfigMap/Secret missing
- RBAC permission errors

Provide step-by-step troubleshooting.
```

---

#### 4. Docker Operations

**`/docker-build` - Build Docker Images**
```markdown
# .claude/commands/docker-build.md

Build Docker images for StreamSpace components.

Component: $ARGUMENTS (api, k8s-agent, docker-agent, ui)

!docker build -t streamspace/$ARGUMENTS:latest -f $ARGUMENTS/Dockerfile .

Verify build:
!docker images streamspace/$ARGUMENTS

Optionally test locally:
!docker run --rm streamspace/$ARGUMENTS:latest --version
```

**`/docker-test` - Test Docker Agent Locally**
```markdown
# .claude/commands/docker-test.md

Test Docker Agent locally without Kubernetes.

Start test environment:
!docker-compose -f docker-compose.test.yml up -d

Verify agent connection:
!docker logs streamspace-docker-agent --tail=50

Test session creation:
- Create session via API
- Verify container created
- Test VNC access
- Verify cleanup

Stop environment:
!docker-compose -f docker-compose.test.yml down
```

---

#### 5. Multi-Agent Workflow

**`/integrate-agents` - Integrate Agent Work**
```markdown
# .claude/commands/integrate-agents.md

Integrate work from Builder, Validator, and Scribe branches.

!git fetch origin claude/v2-builder claude/v2-validator claude/v2-scribe

Show what's new:
!git log --oneline origin/claude/v2-scribe ^HEAD
!git log --oneline origin/claude/v2-builder ^HEAD
!git log --oneline origin/claude/v2-validator ^HEAD

Merge in order:
!git merge origin/claude/v2-scribe --no-edit
!git merge origin/claude/v2-builder --no-edit
!git merge origin/claude/v2-validator --no-edit

Update MULTI_AGENT_PLAN.md with:
- Integration summary
- Changes integrated
- Metrics (files changed, tests added)
- Next steps

Commit and push integration.
```

**`/wave-summary` - Create Wave Summary**
```markdown
# .claude/commands/wave-summary.md

Create integration wave summary for MULTI_AGENT_PLAN.md.

!git log --stat HEAD~5..HEAD

Generate summary with:
## Integration Wave N - [Title] (YYYY-MM-DD)

### Builder (Agent 2)
- Commits integrated
- Files changed
- Key features delivered

### Validator (Agent 3)
- Tests created
- Coverage improvements
- Validation results

### Scribe (Agent 4)
- Documentation updates
- Reports created

**Achievements**:
- Key milestones
- Metrics
- Impact

Format in Markdown for MULTI_AGENT_PLAN.md.
```

---

### StreamSpace-Specific Commands

#### 6. Agent Development

**`/test-agent-lifecycle` - Test Agent Lifecycle**
```markdown
# .claude/commands/test-agent-lifecycle.md

Test complete agent lifecycle (K8s or Docker).

Agent type: $ARGUMENTS (k8s or docker)

Test sequence:
1. Agent registration (WebSocket connect)
2. Heartbeat mechanism (30s interval)
3. Session creation command
4. Session status updates
5. VNC tunnel creation
6. Session termination
7. Agent deregistration

Verify:
- WebSocket connection stable
- Commands processed correctly
- Database state accurate
- Resource cleanup complete

Report results in .claude/reports/ format.
```

**`/test-ha-failover` - Test HA Failover**
```markdown
# .claude/commands/test-ha-failover.md

Test High Availability failover scenarios.

!kubectl scale deployment/streamspace-k8s-agent -n streamspace --replicas=3

Create test sessions:
!for i in {1..5}; do curl -X POST http://localhost:8000/api/v1/sessions ...; done

Simulate failover:
!kubectl delete pod -n streamspace -l app.kubernetes.io/component=k8s-agent | head -1

Verify:
- New leader elected (< 30s)
- All sessions still running
- Zero data loss
- Commands processed by new leader

Document results in .claude/reports/INTEGRATION_TEST_HA_*.md
```

---

#### 7. VNC & Streaming

**`/test-vnc-e2e` - Test VNC Streaming E2E**
```markdown
# .claude/commands/test-vnc-e2e.md

Test VNC streaming end-to-end flow.

Platform: $ARGUMENTS (k8s or docker)

Test flow:
1. Create session with VNC template
2. Verify VNC tunnel created (agent → pod/container)
3. Test Control Plane VNC proxy connection
4. Simulate WebSocket data flow
5. Verify bidirectional streaming
6. Test connection cleanup

Check:
- VNC port accessible (5900)
- Proxy routing working
- No connection leaks
- Clean termination

Report in .claude/reports/INTEGRATION_TEST_VNC_*.md
```

---

#### 8. Code Quality

**`/fix-imports` - Fix Go/TypeScript Imports**
```markdown
# .claude/commands/fix-imports.md

Fix import errors in Go or TypeScript files.

Language: $ARGUMENTS (go or ts)

For Go:
!goimports -w .
!go mod tidy

For TypeScript:
- Scan for missing imports
- Add required import statements
- Remove unused imports
- Organize alphabetically

Verify no compilation errors after fixes.
```

**`/security-audit` - Run Security Audit**
```markdown
# .claude/commands/security-audit.md

Run security audit on codebase.

For Go:
!gosec ./...
!go list -m all | nancy sleuth

For UI:
!npm audit
!npm audit fix --dry-run

Check for:
- Known vulnerabilities
- Hardcoded secrets
- Insecure dependencies
- SQL injection risks
- XSS vulnerabilities

Report findings with severity levels.
```

---

## 🤖 Recommended Subagents

### 1. Test Generator Agent

**`.claude/agents/test-generator.md`**
```markdown
You are a Test Generator agent for StreamSpace.

Your role: Generate comprehensive tests for Go and TypeScript code.

When invoked with a file path:
1. Read the source file
2. Analyze functions/methods/components
3. Generate test file with:
   - Unit tests for all public functions
   - Edge cases and error scenarios
   - Mock dependencies
   - Table-driven tests (for Go)
   - React Testing Library (for UI)

Follow StreamSpace conventions:
- Go: testify/assert, table-driven tests
- UI: Vitest, React Testing Library, @testing-library/user-event

Ensure:
- 80%+ coverage target
- All error paths tested
- Mock external dependencies

Output test file ready to run.
```

---

### 2. PR Reviewer Agent

**`.claude/agents/pr-reviewer.md`**
```markdown
You are a PR Review agent for StreamSpace.

Your role: Review pull requests for code quality, tests, and documentation.

Review checklist:
1. **Code Quality**:
   - Follows Go/TypeScript best practices
   - No code smells or anti-patterns
   - Proper error handling
   - Resource cleanup (defers, cleanup)

2. **Testing**:
   - Tests included for new code
   - Existing tests still pass
   - Coverage not decreased
   - Integration tests for new features

3. **Security**:
   - No hardcoded secrets
   - Input validation
   - SQL injection prevention
   - XSS prevention (UI)

4. **Documentation**:
   - CHANGELOG.md updated
   - README.md updated if needed
   - Code comments for complex logic
   - API documentation current

5. **StreamSpace-Specific**:
   - Follows multi-agent workflow
   - Reports in .claude/reports/
   - Proper git commit format
   - Issue references included

Provide actionable feedback with line numbers.
```

---

### 3. Integration Test Agent

**`.claude/agents/integration-tester.md`**
```markdown
You are an Integration Test agent for StreamSpace v2.0-beta.

Your role: Create and execute integration tests for complex scenarios.

Focus areas:
1. **Multi-Pod API** (Redis-backed AgentHub)
2. **HA Leader Election** (K8s Agent)
3. **VNC Streaming** (E2E flow)
4. **Cross-Platform** (K8s + Docker agents)
5. **Performance** (throughput, latency)

Test creation process:
1. Define test scenario
2. Create test infrastructure (Kind, Docker Compose)
3. Write test code (Go integration tests)
4. Execute tests
5. Collect metrics
6. Generate report in .claude/reports/

Report format:
- Test scenario description
- Test steps executed
- Results (pass/fail)
- Performance metrics
- Issues found
- Recommendations

All reports follow: INTEGRATION_TEST_*.md naming.
```

---

### 4. Documentation Agent

**`.claude/agents/docs-writer.md`**
```markdown
You are a Documentation agent for StreamSpace.

Your role: Create and maintain high-quality documentation.

Documentation types:
1. **API Documentation**: OpenAPI specs, endpoint docs
2. **Architecture**: System design, diagrams
3. **Deployment**: Installation, configuration guides
4. **Developer**: Contributing, testing, workflows
5. **User**: Feature guides, tutorials

When updating docs:
1. Check existing docs first
2. Maintain consistent format
3. Include code examples
4. Add diagrams (mermaid)
5. Update table of contents
6. Cross-reference related docs

StreamSpace standards:
- Essential docs in project root
- Permanent docs in docs/
- Agent reports in .claude/reports/
- Multi-agent coordination in .claude/multi-agent/

Output docs ready to commit.
```

---

## 🎯 Recommended Agent Skills

### 1. Kubernetes Operations Skill

Install from: [Kubernetes MCP Server](https://github.com/blankcut/kubernetes-claude)

**Purpose**: Interact with Kubernetes clusters directly

**Capabilities**:
- List pods, services, deployments
- Get logs from containers
- Describe resources
- Apply manifests
- Check cluster status

**Use Case**: Debugging StreamSpace K8s deployments, checking agent status

---

### 2. Docker Operations Skill

**Purpose**: Manage Docker containers and images

**Capabilities**:
- Build images
- Run containers
- Inspect container logs
- Manage networks/volumes
- Docker Compose operations

**Use Case**: Testing Docker Agent locally, building images

---

### 3. Database Query Skill

**Purpose**: Query PostgreSQL database directly

**Capabilities**:
- Run SELECT queries
- Inspect schema
- Check data integrity
- Analyze query performance

**Use Case**: Debugging session state, verifying agent commands, checking database migrations

---

### 4. Testing & Coverage Skill

**Purpose**: Automated test generation and coverage analysis

**Capabilities**:
- Generate unit tests
- Calculate coverage
- Identify untested code
- Suggest test cases

**Use Case**: Addressing test coverage gaps identified in analysis

---

## 🔌 Recommended Plugins

### 1. [Claude Code Plugins Plus](https://github.com/jeremylongshore/claude-code-plugins-plus)

**Description**: 243 plugins (175 with Agent Skills), 100% compliant with 2025 schema

**Recommended for StreamSpace**:
- Testing plugins
- Git workflow plugins
- Code quality plugins
- Documentation plugins

**Installation**:
```bash
/plugin install github:jeremylongshore/claude-code-plugins-plus
```

---

### 2. [Claude Code Tresor](https://github.com/alirezarezvani/claude-code-tresor)

**Description**: Expert agents, autonomous skills, slash commands

**Recommended for StreamSpace**:
- React/TypeScript development
- Go development
- Testing workflows
- CI/CD automation

---

### 3. [Awesome Claude Code](https://github.com/hesreallyhim/awesome-claude-code)

**Description**: Curated collection of commands, files, workflows

**Explore for**:
- Custom command examples
- CLAUDE.md templates
- Workflow automation

---

## 📚 Best Practices for StreamSpace

### 1. Use CLAUDE.md Effectively

Create comprehensive project context in `CLAUDE.md`:
- Project architecture (Control Plane + Agents)
- Tech stack conventions (Go, React, K8s, Docker)
- Testing philosophy (unit, integration, E2E)
- Multi-agent workflow
- Directory structure
- Common commands

**Reference**: [CLAUDE.md Best Practices](https://www.anthropic.com/engineering/claude-code-best-practices)

---

### 2. Multi-Agent Coordination

Use slash commands to coordinate agents:
- `/integrate-agents` - Pull and merge agent work
- `/wave-summary` - Document integration
- `/agent-status` - Check agent progress

**Reference**: Existing MULTI_AGENT_PLAN.md workflow

---

### 3. Test-Driven Development

Use TDD with Claude:
1. `/generate-tests` - Create test file first
2. Implement feature to pass tests
3. `/verify-all` - Run all checks
4. Iterate until green

**Reference**: [Claude Code TDD](https://www.anthropic.com/engineering/claude-code-best-practices)

---

### 4. Security First

Always run security checks:
- `/security-audit` before PRs
- Never commit secrets
- Use sandboxed environments
- Require confirmations for destructive ops

**Reference**: [Docker Container Security](https://medium.com/@dan.avila7/running-claude-code-agents-in-docker-containers-for-complete-isolation-63036a2ef6f4)

---

### 5. Context Management

Keep context clean:
- Use `/clear` between tasks
- Reference specific files with @
- Use retrieval over dumping logs
- Periodic context pruning

**Reference**: [Claude Agent SDK Best Practices](https://skywork.ai/blog/claude-agent-sdk-best-practices-ai-agents-2025/)

---

## 🚀 Implementation Priority

### Phase 1: Essential Commands (Week 1)
1. `/test-go`, `/test-ui`, `/test-integration`
2. `/verify-all`
3. `/commit-smart`, `/pr-description`
4. `/k8s-logs`, `/k8s-debug`

### Phase 2: Agents (Week 2)
1. Test Generator Agent
2. PR Reviewer Agent
3. Integration Test Agent

### Phase 3: Advanced (Week 3-4)
1. Install recommended plugins
2. Add specialized skills
3. Custom StreamSpace commands
4. Documentation agent

---

## 📖 References

### Official Documentation
- [Claude Code Slash Commands](https://docs.claude.com/en/docs/claude-code/slash-commands)
- [Claude Agent SDK](https://docs.claude.com/en/api/agent-sdk/overview)
- [Agent Skills](https://www.anthropic.com/news/skills)

### Community Resources
- [Awesome Claude Code](https://github.com/hesreallyhim/awesome-claude-code)
- [Claude Command Suite](https://github.com/qdhenry/Claude-Command-Suite)
- [Claude Code Best Practices](https://www.anthropic.com/engineering/claude-code-best-practices)
- [Docker Container Setup](https://medium.com/@dan.avila7/running-claude-code-agents-in-docker-containers-for-complete-isolation-63036a2ef6f4)

### StreamSpace-Specific
- Test Coverage Analysis: `.claude/reports/TEST_COVERAGE_ANALYSIS_2025-11-23.md`
- Multi-Agent Plan: `.claude/multi-agent/MULTI_AGENT_PLAN.md`
- GitHub Issues: #200-207 (testing work)

---

**End of Recommendations**
