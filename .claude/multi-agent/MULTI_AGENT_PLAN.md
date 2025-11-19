# StreamSpace Multi-Agent Development Plan

> **Coordination Hub for Phase 5.5: Feature Completion**

**Created**: 2025-11-19
**Last Updated**: 2025-11-19
**Current Phase**: Phase 5.5 - Feature Completion (BEFORE Phase 6)
**Target Version**: v1.1.0

---

## IMPORTANT: Priority Change

**Phase 6 (VNC Independence) is ON HOLD** until existing features are completed and functional.

Research revealed **40+ incomplete features** across API handlers, controllers, UI components, and plugins that must be addressed before introducing major architectural changes.

---

## Overview

This document serves as the central coordination hub for the multi-agent development of StreamSpace. Current focus is **Phase 5.5: Feature Completion** - ensuring all existing features are fully implemented and functional before proceeding to Phase 6.

All agents should read this document frequently and update it with their progress.

### Agents

| Agent | Role | Responsibilities |
|-------|------|------------------|
| **Agent 1: Architect** | Strategic Leader | Research, architecture design, planning, coordination |
| **Agent 2: Builder** | Implementation | Code implementation, feature development |
| **Agent 3: Validator** | Quality Assurance | Testing, validation, security audits |
| **Agent 4: Scribe** | Documentation | Documentation, guides, migration docs |

---

## External Repositories

StreamSpace uses separate repositories for templates and plugins:

| Repository | URL | Contents |
|------------|-----|----------|
| **Templates** | https://github.com/JoshuaAFerguson/streamspace-templates | 195 templates across 50 categories |
| **Plugins** | https://github.com/JoshuaAFerguson/streamspace-plugins | 27 official plugins |

---

## Current Status

### Phase 5.5 Goals (Feature Completion)

**Primary Objective**: Complete all partially implemented features and fix broken functionality before Phase 6.

**Key Deliverables**:
1. Fix critical plugin runtime loading
2. Complete all stub API handlers
3. Implement missing controller functionality
4. Fix UI components with missing handlers
5. Address security vulnerabilities

### Progress Summary

| Task Area | Status | Assigned To | Progress |
|-----------|--------|-------------|----------|
| **Critical Issues** | In Progress | Builder | 0% |
| Plugin Runtime Loading | Not Started | Builder | 0% |
| Webhook Secret Panic Fix | Not Started | Builder | 0% |
| **High Priority** | Not Started | Builder | 0% |
| Plugin Enable/Config | Not Started | Builder | 0% |
| MFA SMS/Email | Not Started | Builder | 0% |
| **Medium Priority** | Not Started | Builder | 0% |
| Multi-Monitor Plugin | Not Started | Builder | 0% |
| Calendar Plugin | Not Started | Builder | 0% |
| Session Status Conditions | Not Started | Builder | 0% |
| **UI Fixes** | Not Started | Builder | 0% |
| Marketplace Install Button | Not Started | Builder | 0% |
| Favorites API | Not Started | Builder | 0% |
| **Testing** | Not Started | Validator | 0% |
| **Documentation** | Not Started | Scribe | 0% |

---

## Active Tasks

### Task 1: Feature Completion Research (COMPLETE)
- **Assigned To:** Architect
- **Status:** Complete
- **Priority:** Critical
- **Dependencies:** None
- **Notes:**
  - Identified 40+ incomplete features across codebase
  - Found critical plugin runtime issues
  - Documented security vulnerabilities
  - Created priority list for completion
- **Last Updated:** 2025-11-19 - Architect

---

## Task Backlog (Phase 5.5: Feature Completion)

### CRITICAL Priority (Must Fix Immediately)

1. **Plugin Runtime Loading** (Builder)
   - **File:** `/home/user/streamspace/api/internal/plugins/runtime.go:1043`
   - **Issue:** `LoadHandler()` returns "not yet implemented" error
   - **Impact:** Plugins cannot be dynamically loaded from disk
   - **Acceptance Criteria:** Plugins load successfully at runtime

2. **Webhook Secret Generation Panic** (Builder)
   - **File:** `/home/user/streamspace/api/internal/handlers/integrations.go:896`
   - **Issue:** `panic()` instead of graceful error handling
   - **Impact:** API crashes if random generation fails
   - **Acceptance Criteria:** Return proper error response, no panics

### HIGH Priority (Core Functionality Broken)

3. **Plugin Enable Runtime Loading** (Builder)
   - **File:** `/home/user/streamspace/api/internal/handlers/plugin_marketplace.go:455-476`
   - **Issue:** `EnablePlugin()` only updates database, doesn't load into runtime
   - **Impact:** Enabled plugins don't actually run
   - **Acceptance Criteria:** Enabled plugins are loaded and functional

4. **Plugin Config Update** (Builder)
   - **File:** `/home/user/streamspace/api/internal/handlers/plugin_marketplace.go:620-641`
   - **Issue:** Returns success without updating database or reloading
   - **Impact:** Plugin configuration changes are ignored
   - **Acceptance Criteria:** Config updates persist and reload plugins

5. **SAML Return URL Validation** (Builder)
   - **File:** SAML handler
   - **Issue:** Open redirect vulnerability - no whitelist validation
   - **Impact:** Security vulnerability
   - **Acceptance Criteria:** Validate return URLs against whitelist

### MEDIUM Priority (Features Incomplete)

6. **MFA SMS/Email Implementation** (Builder)
   - **File:** `/home/user/streamspace/api/internal/handlers/security.go:283-315`
   - **Issue:** SMS/Email return 501 Not Implemented
   - **Impact:** Users cannot use SMS/Email for 2FA
   - **Acceptance Criteria:** SMS/Email MFA works end-to-end (or remove from UI)

7. **Multi-Monitor Plugin** (Builder)
   - **File:** `/home/user/streamspace/plugins/streamspace-multi-monitor/multi_monitor_plugin.go:20-28`
   - **Issue:** `OnLoad()` returns nil without doing anything
   - **Impact:** Multi-monitor feature completely non-functional
   - **Acceptance Criteria:** Plugin registers endpoints and creates tables

8. **Calendar Plugin** (Builder)
   - **File:** `/home/user/streamspace/plugins/streamspace-calendar/calendar_plugin.go:20-30`
   - **Issue:** `OnLoad()` returns nil without doing anything
   - **Impact:** Calendar integration completely non-functional
   - **Acceptance Criteria:** OAuth handlers and sync jobs functional

9. **Session Status Conditions** (Builder)
   - **Files:** `/home/user/streamspace/k8s-controller/controllers/session_controller.go:314,435,493`
   - **Issue:** TODOs for setting Status.Conditions on errors
   - **Impact:** API users can't track failure reasons
   - **Acceptance Criteria:** Proper conditions set for all error states

10. **Batch Operations Error Collection** (Builder)
    - **File:** `/home/user/streamspace/api/internal/handlers/batch.go:632-851`
    - **Issue:** Errors not collected in error array
    - **Impact:** Users can't see what failed in batch operations
    - **Acceptance Criteria:** All errors included in response

11. **Docker Controller Template Lookup** (Builder)
    - **File:** `/home/user/streamspace/docker-controller/pkg/events/subscriber.go:118`
    - **Issue:** Hardcodes Firefox image instead of looking up template
    - **Impact:** Docker sessions ignore template settings
    - **Acceptance Criteria:** Actually look up template configuration

### UI Fixes (User-Facing Issues)

12. **Marketplace Install Button** (Builder)
    - **File:** `/home/user/streamspace/ui/src/pages/Catalog.tsx:185-187`
    - **Issue:** Install button has no onClick handler
    - **Impact:** Users cannot install marketplace templates
    - **Acceptance Criteria:** Install functionality works

13. **Dashboard Favorites API** (Builder)
    - **File:** `/home/user/streamspace/ui/src/pages/Dashboard.tsx:78-94`
    - **Issue:** Uses localStorage instead of backend API
    - **Impact:** Favorites not synced across devices
    - **Acceptance Criteria:** API endpoint for user favorites

14. **Demo Mode Security** (Builder)
    - **File:** `/home/user/streamspace/ui/src/pages/Login.tsx:103-123`
    - **Issue:** Hardcoded auth allows ANY username
    - **Impact:** Security risk if enabled in production
    - **Acceptance Criteria:** Guard with environment variable

15. **Remove Debug Console.log** (Builder)
    - **File:** `/home/user/streamspace/ui/src/pages/Scheduling.tsx:157`
    - **Issue:** Debug console.log in production
    - **Acceptance Criteria:** Remove debug statements

### LOW Priority (Enhancements)

16. **Hibernation Scheduling** (Builder)
    - **File:** `/home/user/streamspace/k8s-controller/controllers/hibernation_controller.go:286-289`
    - **Issue:** Scheduled hibernation not implemented
    - **Impact:** Cannot hibernate at specific times

17. **Wake-on-Access** (Builder)
    - **File:** `/home/user/streamspace/k8s-controller/controllers/hibernation_controller.go:291-293`
    - **Issue:** Sessions don't auto-wake on request
    - **Impact:** Manual wake required

18. **Hibernation Notifications** (Builder)
    - **File:** `/home/user/streamspace/k8s-controller/controllers/hibernation_controller.go:295-297`
    - **Issue:** No warnings before hibernation
    - **Impact:** Users lose unsaved work

19. **Template Watching** (Builder)
    - **File:** `/home/user/streamspace/k8s-controller/controllers/session_controller.go:1272`
    - **Issue:** Sessions not updated when template changes
    - **Impact:** Manual session updates required

---

## Phase 6 Backlog (ON HOLD)

Phase 6 tasks will resume after Phase 5.5 is complete:

- VNC Stack Research (Completed research, 105+ files identified)
- TigerVNC + noVNC Integration
- StreamSpace-native Container Images (200+)
- Remove Kasm/LinuxServer.io dependencies

---

## Design Decisions

### Decision Log

*(Design decisions will be documented here as they are made)*

---

## Agent Communication Log

### 2025-11-19

#### Architect - Priority Change (10:30)

**MAJOR PIVOT**: User feedback indicates many features are not yet fully implemented. Shifting focus from Phase 6 to Phase 5.5 (Feature Completion).

#### Architect - Research Complete (10:00)

Completed comprehensive research on incomplete features. Key findings:

1. **40+ Incomplete Features Identified**
   - 2 Critical (API crashes, core plugin feature broken)
   - 3 High priority (security vulnerabilities, broken functionality)
   - 11 Medium priority (plugins, controllers incomplete)
   - 4 UI fixes needed

2. **Critical Issues**
   - Plugin runtime loading returns "not yet implemented"
   - Webhook secret generation can panic and crash API
   - SAML has open redirect vulnerability

3. **External Repositories Reviewed**
   - streamspace-templates: 195 templates, 50 categories
   - streamspace-plugins: 27 official plugins

4. **Phase 6 Research (Completed for Reference)**
   - 105+ files with VNC/Kasm references
   - WebSocket proxy exists for status/metrics, NOT for VNC
   - Direct Kubernetes ingress used for VNC access

**Recommendation**: Complete Phase 5.5 before Phase 6. The plugin system is fundamentally broken and must be fixed first.

---

## Architect → Builder - Assignment Ready

Builder, please start with **Critical Issues** in Week 2:

1. **Plugin Runtime Loading** (`api/internal/plugins/runtime.go:1043`)
   - Implement `LoadHandler()` to actually load plugins from disk
   - This is blocking all plugin functionality

2. **Webhook Secret Panic** (`api/internal/handlers/integrations.go:896`)
   - Replace `panic()` with proper error return
   - Simple fix but critical for stability

After Critical, proceed to High Priority items. See Task Backlog for full details with file paths and acceptance criteria.

---

## Architect → Validator - Test Plan Needed

Validator, please prepare test plans for:

1. **Plugin System Tests**
   - Plugin installation and loading
   - Plugin enable/disable
   - Plugin configuration updates

2. **Security Tests**
   - SAML return URL validation
   - CSRF protection
   - Demo mode disabled in production

3. **Integration Tests**
   - Multi-monitor plugin
   - Calendar plugin
   - Batch operations

---

## Architect → Scribe - Documentation Planning

Scribe, please prepare documentation outlines for:

1. **Plugin Development Guide Updates**
   - Runtime loading implementation
   - Configuration management

2. **Security Hardening Guide**
   - SAML configuration
   - MFA setup

3. **Feature Completion Notes**
   - What was fixed
   - Breaking changes (if any)

Wait for implementation to stabilize before writing final docs.

---

## Research Findings

### Phase 5.5: Incomplete Features Analysis (COMPLETE)

#### Summary Statistics
- **Total incomplete features found:** 40+
- **Critical issues:** 2
- **High priority issues:** 3
- **Medium priority issues:** 11
- **UI fixes needed:** 4
- **Low priority enhancements:** 4

#### Critical Issues Found

1. **Plugin Runtime Loading** - Core plugin feature not implemented
2. **Webhook Secret Panic** - API can crash on random generation failure

#### Security Vulnerabilities

1. **SAML Return URL** - Open redirect vulnerability
2. **Demo Mode** - Hardcoded auth in Login.tsx
3. **CSRF Validation** - Only token-based, missing Origin/Referer

#### Broken Core Features

1. **Plugin System** - Enable/Config updates don't work
2. **MFA SMS/Email** - Returns 501 Not Implemented
3. **Multi-Monitor Plugin** - Completely non-functional
4. **Calendar Plugin** - Completely non-functional

#### UI Issues

1. **Marketplace Install** - Button does nothing
2. **Dashboard Favorites** - Uses localStorage, not persisted
3. **Debug Code** - Console.log in production

### Phase 6 Research (FOR REFERENCE)

#### VNC Implementation
- **Status**: Research complete
- **Files affected**: 105+ files contain VNC/Kasm references
- **Current port**: 3000 (LinuxServer.io convention)
- **Target port**: 5900 (standard VNC)

#### Container Images
- **Current source**: LinuxServer.io (lscr.io)
- **Image count**: 195 templates across 50 categories
- **Target**: StreamSpace-native images with TigerVNC + noVNC

#### WebSocket Proxy
- **Location**: `/home/user/streamspace/api/internal/websocket/`
- **Current use**: Status updates, metrics, notifications (NOT VNC)
- **Note**: Direct Kubernetes ingress routes to container VNC, no WebSocket proxy for VNC yet

---

## Technical Specifications

### Proposed VNC Stack

```
┌─────────────────────────────────────┐
│  Web Browser (User)                 │
└──────────────┬──────────────────────┘
               │ HTTPS + WebSocket
               ↓
┌─────────────────────────────────────┐
│  noVNC Web Client (JavaScript)      │
│  - Canvas rendering                 │
│  - WebSocket transport              │
│  - Input handling                   │
└──────────────┬──────────────────────┘
               │ RFB Protocol
               ↓
┌─────────────────────────────────────┐
│  WebSocket Proxy (Go)               │
│  - TLS termination                  │
│  - Authentication                   │
│  - Connection routing               │
└──────────────┬──────────────────────┘
               │ TCP
               ↓
┌─────────────────────────────────────┐
│  TigerVNC Server (Container)        │
│  - Xvfb (Virtual framebuffer)       │
│  - Window manager (XFCE/i3)         │
│  - Application                      │
└─────────────────────────────────────┘
```

### Component Specifications

#### TigerVNC Server
- **License**: GPL-2.0 (100% open source)
- **Port**: 5900 (standard VNC)
- **Features**: High performance, clipboard support, resize
- **Platform**: Linux with Xvfb

#### noVNC Client
- **License**: MPL-2.0 (100% open source)
- **Features**: HTML5 canvas, touch support, mobile-friendly
- **Customization**: Full UI control, branding

#### WebSocket Proxy
- **Language**: Go (part of API backend)
- **Features**: Authentication, rate limiting, monitoring
- **Protocol**: WebSocket to TCP translation

---

## Implementation Guidelines

### Code Patterns

#### Good: VNC-Agnostic Pattern
```go
type VNCConfig struct {
    Port        int    `json:"port"`
    Protocol    string `json:"protocol"`  // "vnc", "rfb", "websocket"
    Encryption  bool   `json:"encryption"`
}

func (t *Template) GetVNCPort() int {
    if t.Spec.VNC.Port != 0 {
        return t.Spec.VNC.Port
    }
    return 5900  // Standard VNC port
}
```

#### Bad: Kasm-Specific Pattern
```go
// DON'T DO THIS
type KasmVNCConfig struct {
    KasmPort int `json:"kasmPort"`
}
```

### Template Definition

#### Good: Generic VNC Config
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
spec:
  vnc:  # Generic VNC config
    enabled: true
    port: 5900
    protocol: rfb
    websocket: true
```

#### Bad: Kasm-Specific Config
```yaml
# DON'T DO THIS
spec:
  kasmvnc:  # Kasm-specific
    enabled: true
    kasmPort: 3000
```

---

## Timeline (Phase 5.5: Feature Completion)

### Week 1 (Current) - Research & Planning
- [x] Read project documentation
- [x] Research incomplete features
- [x] Analyze external repositories
- [x] Create priority list
- [x] Update MULTI_AGENT_PLAN.md

### Week 2 - Critical & High Priority Fixes
- [ ] Fix Plugin Runtime Loading (Critical)
- [ ] Fix Webhook Secret Panic (Critical)
- [ ] Fix Plugin Enable/Config (High)
- [ ] Fix SAML Return URL Validation (High)

### Week 3 - Medium Priority (Plugin System)
- [ ] Implement Multi-Monitor Plugin
- [ ] Implement Calendar Plugin
- [ ] Complete Session Status Conditions
- [ ] Fix Batch Operations Error Collection

### Week 4 - Medium Priority (Controllers)
- [ ] Fix Docker Controller Template Lookup
- [ ] Implement MFA SMS/Email (or remove from UI)

### Week 5 - UI Fixes
- [ ] Fix Marketplace Install Button
- [ ] Implement Dashboard Favorites API
- [ ] Fix Demo Mode Security
- [ ] Remove Debug Console.log

### Week 6 - Testing & Validation
- [ ] Complete test coverage for all fixes
- [ ] Security audit of fixes
- [ ] Integration testing

### Week 7 - Documentation & Polish
- [ ] Update documentation for completed features
- [ ] Create user guides for new functionality
- [ ] Prepare for Phase 6

### Week 8+ - Phase 6 (VNC Independence)
- [ ] Resume VNC migration work
- [ ] Build StreamSpace-native container images
- [ ] Complete open-source independence

---

## Risk Assessment

### High Risks

1. **Performance Degradation**
   - Risk: TigerVNC may have different performance characteristics
   - Mitigation: Extensive benchmarking before migration

2. **Breaking Changes**
   - Risk: Existing sessions may fail after migration
   - Mitigation: Feature flag for gradual rollout, rollback plan

3. **Image Build Complexity**
   - Risk: Building 200+ images is resource-intensive
   - Mitigation: Tiered approach, automated CI/CD

### Medium Risks

4. **noVNC Customization**
   - Risk: UI may differ from current experience
   - Mitigation: Extensive UI testing, user feedback

5. **Authentication Integration**
   - Risk: VNC password handling may differ
   - Mitigation: Abstract authentication layer

---

## Success Criteria

### Phase 5.5 Complete When:

1. [ ] All Critical issues resolved (Plugin runtime, Webhook panic)
2. [ ] All High priority issues resolved (Plugin enable/config, SAML validation)
3. [ ] Plugin system fully functional (install, enable, configure, load)
4. [ ] No API panics or crashes
5. [ ] Security vulnerabilities addressed (SAML, demo mode, CSRF)
6. [ ] UI components have working handlers (Install button, Favorites)
7. [ ] All Medium priority issues addressed
8. [ ] Test coverage for all fixes
9. [ ] Documentation updated

### Phase 6 Complete When (Future):

1. [ ] Zero mentions of "Kasm", "kasmvnc", or "LinuxServer.io" in codebase
2. [ ] All container images built and maintained by StreamSpace
3. [ ] No external dependencies on proprietary software
4. [ ] Documentation explains 100% open source stack
5. [ ] Migration path documented for existing users
6. [ ] Performance equal to or better than LinuxServer.io images
7. [ ] All existing tests pass with new VNC stack
8. [ ] Security audit completed successfully

---

## References

### Internal Documentation
- [ROADMAP.md](../../ROADMAP.md) - Development roadmap
- [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) - System architecture
- [FEATURES.md](../../FEATURES.md) - Complete feature list
- [CLAUDE.md](../../CLAUDE.md) - AI assistant guide

### External Resources
- [TigerVNC Documentation](https://tigervnc.org/)
- [noVNC Repository](https://github.com/novnc/noVNC)
- [VNC Protocol (RFB)](https://github.com/rfbproto/rfbproto)

---

## Notes for Agents

### For Architect
- Update this document after every major decision
- Provide clear specifications to Builder
- Define acceptance criteria for Validator

### For Builder
- Check this document before starting work
- Update task status as you progress
- Report blockers immediately

### For Validator
- Create test plans based on specifications
- Document test results
- Report issues with severity levels

### For Scribe
- Wait for implementation to stabilize
- Document as features are completed
- Include diagrams and examples

---

**Remember**: This document is the source of truth. Update it frequently!
