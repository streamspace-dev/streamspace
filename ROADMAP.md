# StreamSpace Development Roadmap

**Current Version**: v1.0.0-beta
**Last Updated**: 2025-11-19

---

## Current State

StreamSpace has a functional core platform but several areas require significant work before production readiness.

### Implementation Summary

| Component | Status | Completeness |
|-----------|--------|--------------|
| Kubernetes Controller | Complete | 100% |
| API Backend | Complete | 95% |
| Web UI | Complete | 95% |
| Database Schema | Complete | 100% |
| Helm Chart | Complete | 95% |
| Plugin System | Partial | 40% (framework only) |
| Docker Controller | Stub | 5% |
| Test Coverage | Incomplete | 15-20% |
| VNC Migration | Not Started | 0% |

---

## Completed Work

### Core Platform

- **Kubernetes Controller** (5,282 lines)
  - Session reconciler with full lifecycle management
  - Hibernation controller with idle detection
  - Template reconciler
  - ApplicationInstall reconciler
  - Prometheus metrics (40+ metric types)

- **API Backend** (61,289 lines)
  - 70+ API handler files
  - 87 database tables
  - 15+ middleware layers
  - Authentication: Local, SAML 2.0, OIDC OAuth2, MFA
  - WebSocket support for real-time updates
  - Webhook system (16 event types)
  - Integration support (Slack, Teams, Discord, PagerDuty, email)

- **Web UI** (25,629 lines)
  - 27 pages (14 user, 12 admin + login)
  - 27 React components
  - Real-time WebSocket integration
  - Material-UI design system

- **Infrastructure**
  - CRD definitions (Session, Template, ApplicationInstall)
  - Helm chart with 19 templates
  - Kubernetes manifests for deployment
  - Monitoring configuration (Prometheus, Grafana)

---

## Priority Work Items

### Priority 1: Test Coverage (High)

**Current**: ~15-20%
**Target**: 80%+

The existing test infrastructure needs significant expansion:

#### Controller Tests
- **Existing**: 4 test files (529 lines)
- **Needs**: Error handling, edge cases, concurrent operations
- **Blocker**: Requires envtest setup for local execution

#### API Tests
- **Existing**: 11 test files (~2,700 lines)
- **Needs**: 63+ untested handler files, database layer tests
- **Blocker**: Some tests have build errors (method name mismatches)

#### UI Tests
- **Existing**: 2 test files (SessionCard, SecuritySettings)
- **Needs**: 48+ untested components, all pages
- **Ready**: Vitest configured with 80% threshold

#### Integration Tests
- **Existing**: 5 test files with 23 test functions
- **Status**: Complete and passing

**Estimated effort**: 6-8 weeks with dedicated testing focus

### Priority 2: Plugin Implementations (High)

**Current**: Framework complete, 28 plugins are stubs
**Target**: Working implementations

The plugin system has a complete framework but individual plugins contain only TODOs:

```
plugins/
├── streamspace-calendar/        # TODO: Extract from scheduling handler
├── streamspace-multi-monitor/   # TODO: 3 items
├── streamspace-compliance/      # TODO: Stub
├── streamspace-dlp/             # TODO: Stub
├── streamspace-analytics/       # TODO: Stub
├── streamspace-slack/           # TODO: Extract from integrations
├── streamspace-teams/           # TODO: Extract from integrations
├── streamspace-discord/         # TODO: Extract from integrations
└── ... (20 more stubs)
```

**Work required**:
1. Extract existing handler logic into plugin modules
2. Implement plugin configuration UI
3. Add plugin-specific tests
4. Document each plugin

**Estimated effort**: 4-6 weeks to convert top 10 plugins

### Priority 3: Docker Controller (Medium)

**Current**: 102-line skeleton
**Target**: Functional parity with Kubernetes controller

The Docker controller exists as a framework only:
- NATS event subscription set up
- No actual Docker operations implemented
- Packages `pkg/docker` and `pkg/events` are stubs

**Work required**:
1. Implement container lifecycle management
2. Volume management for user storage
3. Network configuration
4. Event publishing back to API
5. Integration testing

**Estimated effort**: 4-6 weeks for MVP

### Priority 4: VNC Independence (Medium)

**Current**: Using LinuxServer.io images with KasmVNC
**Target**: StreamSpace-native images with TigerVNC + noVNC

**Work required**:
1. Create base container images (Ubuntu, Alpine, Debian)
2. Integrate TigerVNC server
3. Configure noVNC client
4. Rebuild all 200+ application templates
5. Set up image build pipeline
6. Security scanning and signing

**Estimated effort**: 4-6 months

---

## Backlog

### Nice to Have

- Multi-cluster federation
- WebRTC streaming (lower latency)
- GPU acceleration support
- Advanced caching with Redis
- Machine learning-based idle detection

### Known Issues

- Some API handlers have TODO comments for minor enhancements
- Plugin configuration endpoints have incomplete implementations
- SMS/Email MFA deliberately disabled (security concerns)

---

## Release Plan

### v1.0.0-beta (Current)

What's included:
- Functional Kubernetes platform
- Complete authentication stack
- 87 database tables
- 70+ API handlers
- 50+ UI components
- Helm chart for deployment

Known limitations:
- 15-20% test coverage
- Plugin stubs only
- Docker controller not functional
- Using external VNC images

### v1.0.0 (Stable Release)

Requirements before stable:
- [ ] Test coverage reaches 70%+
- [ ] Top 10 plugins implemented
- [ ] All critical API handler TODOs resolved
- [ ] Documentation audit complete
- [ ] Security audit complete

### v1.1.0 (Docker Support)

- [ ] Functional Docker controller
- [ ] Docker Compose deployment option
- [ ] Local volume management
- [ ] Integration tests for Docker platform

### v2.0.0 (VNC Independence)

- [ ] StreamSpace-native container images
- [ ] TigerVNC + noVNC stack
- [ ] Image build pipeline
- [ ] All templates migrated
- [ ] Performance optimization

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### High-Impact Contribution Areas

1. **Write tests** - Any test coverage helps
2. **Convert plugin stubs** - Pick a plugin and implement it
3. **Docker controller** - Help build multi-platform support
4. **Documentation** - Fix inaccuracies, add examples

### Getting Started

```bash
# Clone and explore
git clone https://github.com/JoshuaAFerguson/streamspace.git
cd streamspace

# Run existing tests
cd k8s-controller && make test
cd ../api && go test ./... -v
cd ../ui && npm test
```

---

## Timeline Estimates

| Milestone | Target | Dependencies |
|-----------|--------|--------------|
| 70% test coverage | 8 weeks | Testing infrastructure fixes |
| Top 10 plugins | 10 weeks | Plugin framework validation |
| Stable v1.0.0 | 12 weeks | Test coverage, plugin work |
| Docker support | 16 weeks | Docker controller completion |
| VNC independence | 6 months | Image build infrastructure |

These are rough estimates and depend on contributor availability.

---

## References

- [FEATURES.md](FEATURES.md) - Detailed feature status
- [TEST_COVERAGE_REPORT.md](tests/reports/TEST_COVERAGE_REPORT.md) - Test coverage analysis
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture

---

**Last Updated**: 2025-11-19
