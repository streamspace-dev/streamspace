# Test Coverage Report

Generate comprehensive test coverage report across all components.

**Use this when**: Checking test coverage progress, before release, or after adding tests.

## Usage

Run without arguments: `/coverage-report`

Or specify component: `/coverage-report api` or `/coverage-report ui`

## What This Does

Runs tests with coverage for all components:

1. **API (Go)**:
   - `go test -coverprofile=coverage.out ./...`
   - Generates HTML report
   - Shows per-package coverage

2. **K8s Agent (Go)**:
   - `go test -coverprofile=coverage.out ./...`
   - Agent-specific coverage

3. **Docker Agent (Go)**:
   - `go test -coverprofile=coverage.out ./...`
   - Docker agent coverage

4. **UI (TypeScript/React)**:
   - `npm test -- --coverage`
   - Component coverage
   - Integration test coverage

## Output Format

Creates report in `.claude/reports/TEST_COVERAGE_<date>.md`:

```markdown
# Test Coverage Report - 2025-11-23

## Summary

| Component | Coverage | Change | Status |
|-----------|----------|--------|--------|
| API | 47.2% | +5.2% ⬆️ | 🟡 Below Target |
| K8s Agent | 23.4% | +23.4% ⬆️ | 🔴 Needs Work |
| Docker Agent | 0.0% | 0.0% — | 🔴 No Tests |
| UI | 32.1% | -1.2% ⬇️ | 🔴 Needs Work |
| **Overall** | **34.2%** | **+6.9%** | 🔴 **Below 70% Target** |

## Detailed Breakdown

### API (47.2%)

#### High Coverage (>70%)
- ✅ `api/internal/db` - 89.3% (database layer)
- ✅ `api/internal/models` - 78.1% (data models)

#### Medium Coverage (40-70%)
- 🟡 `api/internal/handlers` - 56.2% (API handlers)
- 🟡 `api/internal/websocket` - 45.8% (WebSocket hub)

#### Low Coverage (<40%)
- 🔴 `api/internal/services` - 12.3% (business logic)
- 🔴 `api/internal/middleware` - 8.7% (middleware)

#### No Coverage (0%)
- ❌ `api/internal/auth` - 0.0% (auth handlers)
- ❌ `api/internal/sync` - 0.0% (CRD sync)

### K8s Agent (23.4%)

#### Coverage by Package
- 🟡 `agents/k8s-agent/internal/k8s` - 45.2%
- 🔴 `agents/k8s-agent/internal/vnc` - 18.9%
- 🔴 `agents/k8s-agent/internal/handlers` - 12.1%
- ❌ `agents/k8s-agent/internal/leader` - 0.0%

### Docker Agent (0.0%)

⚠️ **NO TESTS EXIST**

- Total lines: 2,100+
- Tested lines: 0
- Blocking Issue: #201

### UI (32.1%)

#### Component Coverage
- ✅ `src/components/Sessions` - 71.2%
- 🟡 `src/components/Agents` - 48.3%
- 🔴 `src/components/Admin` - 15.7%
- ❌ `src/services/api` - 0.0%

## Coverage Trends

```
Week 1: 25.3%
Week 2: 27.3% (+2.0%)
Week 3: 34.2% (+6.9%)

Target: 70%
Gap: -35.8%
```

## Priority Recommendations

### P0 CRITICAL (Must Add Tests)
1. **Docker Agent** - 0% coverage, 2100+ lines untested
2. **API Auth** - 0% coverage, security risk
3. **K8s Leader Election** - 0% coverage, HA feature untested

### P1 HIGH (Should Add Tests)
4. **API Services** - 12% coverage, core business logic
5. **WebSocket Hub** - 46% coverage, critical for agent communication
6. **UI API Service** - 0% coverage, all external calls untested

### P2 MEDIUM (Nice to Have)
7. **UI Admin Components** - 16% coverage
8. **K8s VNC Handlers** - 19% coverage

## Uncovered Critical Paths

### Security Risks (No Test Coverage)
- `/api/v1/login` endpoint (auth bypass possible)
- `/api/v1/admin/*` endpoints (privilege escalation)
- WebSocket authentication (unauthorized access)

### Reliability Risks (Low Coverage)
- Session lifecycle (45% coverage, edge cases untested)
- Agent failover (HA logic mostly untested)
- VNC streaming (connection handling untested)

## Action Plan

To reach 70% coverage:

1. **Immediate** (Next 2 Days):
   - Add Docker Agent tests (0% → 60%) - Issue #201
   - Add API auth tests (0% → 80%)
   - Add WebSocket auth tests

2. **Short Term** (Next Week):
   - Add service layer tests (12% → 70%)
   - Add leader election tests (0% → 80%)
   - Add UI API service tests (0% → 60%)

3. **Medium Term** (Next 2 Weeks):
   - Improve handler tests (56% → 80%)
   - Improve component tests (32% → 70%)
   - Add integration tests

**Estimated Effort**: 40-60 hours to reach 70% coverage

## Files Generated

- `coverage.out` - Go coverage data
- `coverage.html` - HTML coverage report (open in browser)
- `coverage/` - Per-package coverage reports
- `.claude/reports/TEST_COVERAGE_<date>.md` - This report

---
🤖 Generated via `/coverage-report` command
```

## Interactive Features

After generating report:

1. **Show uncovered lines**: Open HTML report in browser
2. **Generate test stubs**: Create test files for 0% coverage packages
3. **Create tracking issues**: Auto-create issues for critical gaps
4. **Update milestone**: Track coverage as release requirement

## Integration with CI/CD

The report can be:
- Posted as PR comment
- Tracked in GitHub Issues
- Required for release approval
- Monitored in dashboards
