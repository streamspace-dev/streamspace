# StreamSpace Plugin Migration Plan

**Goal**: Extract non-essential features from core to plugins for a leaner, more modular platform
**Status**: In Progress
**Created**: 2025-11-16
**Impact**: No running instances - full refactoring possible

---

## Executive Summary

This plan migrates **7 major feature areas** from StreamSpace core to plugins, reducing core database tables from 82+ to ~40-50 and making the platform more modular and maintainable.

### Migration Phases

1. **Phase 1**: External Integrations (Slack, Teams, Discord, PagerDuty, Email) - **EASIEST**
2. **Phase 2**: Billing System - **LOW RISK**
3. **Phase 3**: Compliance Framework - **HIGH VALUE**
4. **Phase 4**: DLP (Data Loss Prevention) - **SPECIALIZED**
5. **Phase 5**: Node Management - **INFRASTRUCTURE**
6. **Phase 6**: Session Recording - **STORAGE INTENSIVE**
7. **Phase 7**: Advanced Features (Multi-monitor, Workflows, Calendar) - **NICE-TO-HAVE**

---

## Plugin Details

### 1. External Integrations → Multiple Plugins

**Current Location**: `api/internal/handlers/integrations.go`
**Database Tables**:
- `integrations` (provider config)
- `integration_deliveries` (delivery tracking)

**Extract to**:
- `streamspace-slack` - Slack notifications
- `streamspace-teams` - Microsoft Teams notifications
- `streamspace-discord` - Discord notifications
- `streamspace-pagerduty` - PagerDuty incident management
- `streamspace-email-smtp` - SMTP email integration

**Plugin Architecture**:
```javascript
// Each integration as separate plugin
module.exports = {
  async onLoad() {
    // Validate config (API key, webhook URL, etc.)
    this.validateConfig();

    // Register event handlers
    streamspace.events.on('session.created', this.onSessionCreated.bind(this));
    streamspace.events.on('user.created', this.onUserCreated.bind(this));
  },

  async onSessionCreated(session) {
    // Send notification to Slack/Teams/Discord/etc
    await this.sendNotification({
      title: 'Session Created',
      message: `${session.user} created ${session.template}`,
      session: session
    });
  },

  async sendNotification(data) {
    // Provider-specific implementation
  }
};
```

**Database Migration**:
- **Keep**: `integrations` table (generic integration storage for plugins)
- **Remove**: Provider-specific logic from core handlers
- **Plugin Storage**: Each plugin uses `streamspace.storage.*` for state

**API Changes**:
- Core keeps generic `/api/integrations` endpoints (CRUD)
- Plugins register webhooks via plugin API
- Notification delivery handled by plugins

**Benefits**:
- Users install only needed integrations
- Easy to add new providers as community plugins
- Reduces core dependencies

---

### 2. Billing System → `streamspace-billing`

**Current Location**: `api/internal/handlers/billing.go`
**Database Tables**:
- `billing_costs`
- `billing_invoices`
- `billing_payment_methods`
- `billing_usage_tracking`
- `billing_pricing`

**Plugin Type**: Extension
**Category**: Enterprise
**Permissions**: `admin`, `read:billing`, `write:billing`

**Plugin Features**:
- Cost tracking and forecasting
- Invoice generation (PDF/CSV export)
- Payment method management
- Usage reports
- Custom pricing rules

**Configuration Schema**:
```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "currency": {
        "type": "string",
        "enum": ["USD", "EUR", "GBP"],
        "default": "USD"
      },
      "billingCycle": {
        "type": "string",
        "enum": ["monthly", "quarterly", "annual"],
        "default": "monthly"
      },
      "pricing": {
        "type": "object",
        "properties": {
          "cpuHourRate": { "type": "number", "default": 0.01 },
          "memoryGBRate": { "type": "number", "default": 0.005 },
          "storageGBRate": { "type": "number", "default": 0.10 }
        }
      },
      "invoiceGeneration": {
        "type": "object",
        "properties": {
          "autoGenerate": { "type": "boolean", "default": true },
          "dayOfMonth": { "type": "number", "minimum": 1, "maximum": 28, "default": 1 }
        }
      }
    }
  }
}
```

**Database Migration**:
- **Move**: All billing tables to plugin-managed schema
- **Plugin Init**: Create tables on first install
- **Data Export**: Provide migration script for existing billing data

**API Endpoints** (moved to plugin):
- `GET /api/plugins/billing/costs/*`
- `GET /api/plugins/billing/invoices/*`
- `POST /api/plugins/billing/invoices/generate`
- `GET /api/plugins/billing/usage/*`
- `GET /api/plugins/billing/pricing`

**UI Components**:
- Admin billing dashboard (plugin-registered widget)
- Invoice list/detail views
- Usage charts and analytics
- Pricing configuration page

---

### 3. Compliance Framework → `streamspace-compliance`

**Current Location**: `api/internal/handlers/compliance.go`
**Database Tables**:
- `compliance_frameworks` (SOC2, HIPAA, GDPR, ISO27001)
- `compliance_controls`
- `compliance_policies`
- `compliance_violations`
- `compliance_reports`
- `dlp_policies` (Data Loss Prevention)
- `dlp_violations`

**Plugin Type**: Extension
**Category**: Security & Compliance
**Permissions**: `admin`, `compliance:read`, `compliance:write`

**Plugin Features**:
- Multiple compliance frameworks (SOC2, HIPAA, GDPR, ISO27001)
- Automated compliance checks
- Violation tracking and remediation
- Compliance dashboards and reports
- Policy management
- Data retention policies
- Access control policies
- Audit requirements

**Configuration Schema**:
```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "enabledFrameworks": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["SOC2", "HIPAA", "GDPR", "ISO27001"]
        },
        "default": ["SOC2"]
      },
      "autoCheck": {
        "type": "boolean",
        "default": true,
        "description": "Automatically check compliance status"
      },
      "checkInterval": {
        "type": "number",
        "minimum": 1,
        "maximum": 168,
        "default": 24,
        "description": "Hours between compliance checks"
      },
      "violationActions": {
        "type": "object",
        "properties": {
          "notifyAdmins": { "type": "boolean", "default": true },
          "blockActions": { "type": "boolean", "default": false },
          "createTickets": { "type": "boolean", "default": false }
        }
      },
      "dataRetention": {
        "type": "object",
        "properties": {
          "auditLogDays": { "type": "number", "default": 365 },
          "sessionDataDays": { "type": "number", "default": 90 },
          "recordingDays": { "type": "number", "default": 30 }
        }
      }
    }
  }
}
```

**Database Migration**:
- **Move**: All compliance and DLP tables to plugin
- **Conditional**: Only create tables if plugin installed
- **Impact**: Huge reduction in core database complexity

**Event Hooks**:
```javascript
module.exports = {
  async onSessionCreated(session) {
    // Check data classification policies
    await this.checkDataClassification(session);
  },

  async onUserLogin(user) {
    // Check access control policies (IP restrictions, MFA, etc.)
    await this.checkAccessControl(user);
  },

  async onFileUpload(file, session) {
    // DLP scanning for sensitive data patterns
    await this.scanForSensitiveData(file);
  }
};
```

**Benefits**:
- Only regulated industries install compliance features
- Reduces core overhead for non-regulated users
- Easy to add new frameworks via updates
- Framework-specific customization possible

---

### 4. DLP → `streamspace-dlp`

**Current Location**: Embedded in `compliance.go`
**Database Tables**:
- `dlp_policies`
- `dlp_violations`
- `dlp_patterns`

**Note**: DLP is currently part of compliance handler. This could be:
- **Option A**: Part of `streamspace-compliance` plugin
- **Option B**: Separate `streamspace-dlp` plugin
- **Recommendation**: **Option A** - keep in compliance plugin (tightly coupled)

**If separate plugin**:
- Pattern-based data scanning
- Violation tracking
- Integration with compliance plugin
- Custom pattern rules (SSN, credit cards, API keys, etc.)

---

### 5. Node Management → `streamspace-node-manager`

**Current Location**: `api/internal/handlers/nodes.go`
**Database Tables**:
- `node_configs`
- `node_selection_policies`
- `scaling_policies`
- `scaling_history`

**Plugin Type**: Extension
**Category**: Infrastructure
**Permissions**: `admin`, `infrastructure:read`, `infrastructure:write`

**Plugin Features**:
- Kubernetes node listing and health
- Node labeling and taints
- Auto-scaling policies
- Load balancing configuration
- Node selection algorithms
- Scaling history and analytics

**Configuration Schema**:
```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "autoScaling": {
        "type": "object",
        "properties": {
          "enabled": { "type": "boolean", "default": false },
          "minNodes": { "type": "number", "minimum": 1, "default": 1 },
          "maxNodes": { "type": "number", "minimum": 1, "default": 10 },
          "scaleUpThreshold": { "type": "number", "default": 80 },
          "scaleDownThreshold": { "type": "number", "default": 20 }
        }
      },
      "nodeSelection": {
        "type": "string",
        "enum": ["least-sessions", "most-resources", "random", "weighted"],
        "default": "least-sessions"
      },
      "healthCheck": {
        "type": "object",
        "properties": {
          "enabled": { "type": "boolean", "default": true },
          "interval": { "type": "number", "default": 60 }
        }
      }
    }
  }
}
```

**Benefits**:
- Users with single-node clusters don't need this
- Advanced cluster operators get powerful tools
- Integration possible with external tools (Rancher, k9s, etc.)

---

### 6. Session Recording → `streamspace-session-recorder`

**Current Location**: `api/internal/handlers/sessions.go` (recording endpoints)
**Database Tables**:
- `session_recordings`
- `session_recording_policies`
- `session_recording_access_log`

**Plugin Type**: Extension
**Category**: Security & Compliance
**Permissions**: `admin`, `recording:read`, `recording:write`

**Plugin Features**:
- Start/stop session recording
- Recording policies (auto-record certain users/sessions)
- Access logging (who viewed recordings)
- Storage management
- Playback interface
- Recording retention policies

**Configuration Schema**:
```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "storage": {
        "type": "object",
        "properties": {
          "backend": {
            "type": "string",
            "enum": ["local", "s3", "gcs", "azure"],
            "default": "local"
          },
          "path": { "type": "string", "default": "/recordings" },
          "compression": { "type": "boolean", "default": true }
        }
      },
      "retention": {
        "type": "object",
        "properties": {
          "enabled": { "type": "boolean", "default": true },
          "days": { "type": "number", "default": 30 },
          "autoPurge": { "type": "boolean", "default": true }
        }
      },
      "policies": {
        "type": "object",
        "properties": {
          "autoRecord": { "type": "boolean", "default": false },
          "recordByRole": { "type": "array", "items": { "type": "string" } },
          "notifyOnAccess": { "type": "boolean", "default": true }
        }
      }
    }
  }
}
```

**Privacy Considerations**:
- Clear user consent mechanisms
- Access logging for accountability
- Retention policy compliance
- Secure storage encryption

---

### 7. Advanced Features → Multiple Plugins

#### 7a. `streamspace-multi-monitor`
- Multiple display support
- Monitor configuration and presets
- Independent display streams

#### 7b. `streamspace-workflows`
- Workflow automation
- Trigger-based actions
- Integration with external automation tools

#### 7c. `streamspace-calendar`
- Google Calendar integration
- Outlook Calendar integration
- iCal export
- Scheduled session automation

---

## Plugin API Enhancements Needed

### 1. Event System Enhancement
```javascript
// Current: Plugin lifecycle hooks only
// Needed: Full event system

streamspace.events.on('session.created', handler);
streamspace.events.on('session.started', handler);
streamspace.events.on('session.stopped', handler);
streamspace.events.on('session.hibernated', handler);
streamspace.events.on('session.woken', handler);
streamspace.events.on('session.deleted', handler);
streamspace.events.on('user.created', handler);
streamspace.events.on('user.login', handler);
streamspace.events.on('user.logout', handler);
streamspace.events.on('file.uploaded', handler);
streamspace.events.on('quota.exceeded', handler);
```

### 2. Database Access for Plugins
```javascript
// Plugins need ability to create/manage their own tables
streamspace.database.exec(sql, params);
streamspace.database.query(sql, params);
streamspace.database.transaction(callback);

// Schema migration support
streamspace.database.migrate(migrationSQL);
```

### 3. Admin UI Registration
```javascript
// Plugins need to register admin pages
streamspace.ui.registerAdminPage('billing-dashboard', {
  title: 'Billing',
  icon: 'dollar-sign',
  component: './pages/BillingDashboard.jsx',
  path: '/admin/billing',
  permissions: ['admin', 'billing:read']
});

// Register admin widgets
streamspace.ui.registerAdminWidget('compliance-status', {
  title: 'Compliance Status',
  component: './widgets/ComplianceStatus.jsx',
  position: 'top',
  width: 'half'
});
```

### 4. API Endpoint Registration
```javascript
// Plugins already can register endpoints, but enhance:
streamspace.api.registerEndpoint({
  method: 'GET',
  path: '/api/plugins/billing/invoices',
  handler: async (req, res) => { /* ... */ },
  permissions: ['billing:read'],
  rateLimitpattern: 'standard', // Use platform rate limiting
  validation: invoiceQuerySchema // JSON schema validation
});
```

### 5. Configuration UI Generation
```javascript
// Auto-generate configuration UI from schema
// Already supported via configSchema in manifest
```

### 6. Inter-Plugin Communication
```javascript
// Plugins can depend on other plugins
streamspace.plugins.get('compliance');
streamspace.plugins.isEnabled('billing');
streamspace.plugins.call('billing', 'calculateCost', session);
```

### 7. Scheduled Jobs
```javascript
// Plugins can schedule periodic tasks
streamspace.scheduler.schedule('0 0 * * *', async () => {
  // Daily compliance check
  await this.runComplianceCheck();
});
```

---

## Implementation Order

### Phase 1: Infrastructure (Week 1)
1. ✅ Enhance plugin API with required features
2. ✅ Add database access to plugins
3. ✅ Add event system
4. ✅ Add admin UI registration
5. ✅ Add scheduler support

### Phase 2: Easy Wins (Week 1-2)
1. Extract Slack integration → `streamspace-slack`
2. Extract Teams integration → `streamspace-teams`
3. Extract Discord integration → `streamspace-discord`
4. Extract PagerDuty integration → `streamspace-pagerduty`
5. Extract Email SMTP → `streamspace-email-smtp`

### Phase 3: Medium Complexity (Week 2-3)
1. Extract Billing → `streamspace-billing`
2. Extract Node Management → `streamspace-node-manager`
3. Extract Calendar → `streamspace-calendar`

### Phase 4: High Complexity (Week 3-4)
1. Extract Compliance + DLP → `streamspace-compliance`
2. Extract Session Recording → `streamspace-session-recorder`
3. Extract Workflows → `streamspace-workflows`
4. Extract Multi-Monitor → `streamspace-multi-monitor`

### Phase 5: Cleanup (Week 4)
1. Remove extracted code from core
2. Update database schema
3. Update documentation
4. Create migration guides
5. Test core without plugins
6. Test each plugin independently

---

## Database Impact

### Before (Core Tables):
- Sessions: ~15 tables
- Users/Groups: ~10 tables
- Templates: ~8 tables
- Authentication: ~12 tables
- Webhooks: ~3 tables
- **Integrations: ~5 tables** ← TO PLUGIN
- **Billing: ~8 tables** ← TO PLUGIN
- **Compliance: ~10 tables** ← TO PLUGIN
- **DLP: ~5 tables** ← TO PLUGIN
- **Nodes/Scaling: ~6 tables** ← TO PLUGIN
- **Recording: ~4 tables** ← TO PLUGIN
- Other: ~16 tables

**Total**: 82+ tables

### After (Core Tables):
- Sessions: ~15 tables
- Users/Groups: ~10 tables
- Templates: ~8 tables
- Authentication: ~12 tables
- Webhooks: ~3 tables
- Plugins: ~6 tables
- Other: ~16 tables

**Total**: ~40-50 tables (**40% reduction**)

### Plugin Tables:
- `streamspace-billing`: ~8 tables
- `streamspace-compliance`: ~15 tables (including DLP)
- `streamspace-node-manager`: ~6 tables
- `streamspace-session-recorder`: ~4 tables
- Integrations: Use plugin storage (no dedicated tables)

---

## Testing Strategy

### Plugin Testing
1. **Unit Tests**: Each plugin has comprehensive unit tests
2. **Integration Tests**: Test plugin with core API
3. **E2E Tests**: Full user workflows with plugins
4. **Isolation Tests**: Core works without plugins
5. **Dependency Tests**: Plugins with dependencies work correctly

### Migration Testing
1. **Schema Migration**: Test table creation on plugin install
2. **Data Migration**: Test moving existing data to plugins
3. **Rollback**: Test disabling/uninstalling plugins
4. **Performance**: Ensure plugin overhead is minimal

---

## Documentation Updates

### User Documentation
- [ ] Plugin installation guide
- [ ] Plugin configuration guide
- [ ] Migration guide (for users upgrading from all-in-one)
- [ ] Per-plugin documentation

### Developer Documentation
- [ ] Enhanced Plugin API reference
- [ ] Plugin development guide updates
- [ ] Database access guide for plugins
- [ ] Event system documentation
- [ ] Inter-plugin communication guide

### Admin Documentation
- [ ] Plugin management guide
- [ ] Performance impact guide
- [ ] Security considerations per plugin

---

## Success Criteria

### Core Platform
- ✅ Core works independently without any plugins
- ✅ Core database reduced to ~40-50 tables
- ✅ Core Docker image size reduced by 30%+
- ✅ Core startup time reduced by 20%+
- ✅ All existing tests pass with plugins disabled

### Plugins
- ✅ Each plugin installs/uninstalls cleanly
- ✅ Plugins don't interfere with each other
- ✅ Plugin configuration UI auto-generated from schema
- ✅ Plugins can be enabled/disabled at runtime
- ✅ Plugin data properly isolated

### Developer Experience
- ✅ Plugin API comprehensive and well-documented
- ✅ Example plugins for each category
- ✅ Plugin development guide updated
- ✅ Plugin testing framework available

---

## Risks & Mitigation

### Risk 1: Plugin API Limitations
- **Mitigation**: Implement comprehensive plugin API first (Phase 1)
- **Testing**: Build one prototype plugin to validate API

### Risk 2: Performance Overhead
- **Mitigation**: Benchmark each plugin, optimize hot paths
- **Testing**: Performance tests with 0, 5, 10 plugins installed

### Risk 3: Complexity for Users
- **Mitigation**: Create plugin "bundles" (Enterprise bundle, etc.)
- **Documentation**: Clear plugin selection guide

### Risk 4: Breaking Changes
- **Mitigation**: No running instances to worry about
- **Forward Plan**: Provide upgrade path for future versions

---

## Next Steps

1. **Immediate**: Implement Plugin API enhancements
2. **Week 1**: Extract Slack/Teams/Discord/PagerDuty integrations
3. **Week 2**: Extract Billing and Node Management
4. **Week 3**: Extract Compliance and Recording
5. **Week 4**: Cleanup and documentation

---

**Status**: Ready to begin implementation
**Owner**: Development Team
**Timeline**: 4 weeks for full migration
**Impact**: Leaner core, modular architecture, better maintainability
