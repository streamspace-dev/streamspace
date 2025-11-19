# Plugin-Based Features Checklist

This checklist helps identify which features are **intentionally plugin-based** and should NOT be marked as bugs when they appear stubbed in the core API.

## When You Encounter These Features...

### DO NOT mark as bug if feature:
- Returns empty list/array
- Returns `501 Not Implemented` status code
- Shows message: "install streamspace-[plugin-name] plugin"
- Has no UI components (not registered)
- Returns zero/default metrics
- Doesn't create database tables
- Returns stub/placeholder data

### DO mark as bug if feature:
- Crashes/panics
- Returns 500 Internal Server Error
- Is missing and should be core (not plugin-dependent)
- Breaks existing functionality
- Returns incorrect HTTP status codes (not 501)
- Returns error when plugin IS installed

---

## Checklist: Plugin-Based Features

Use this checklist when reviewing code, issues, or TODOs:

### Security & Compliance

- [ ] Compliance frameworks (GDPR, HIPAA, SOC2, ISO27001) 
  - Plugin: `streamspace-compliance`
  - Status: Stub returns empty array with helpful message
  - NOT a bug ✓

- [ ] Compliance policies 
  - Plugin: `streamspace-compliance`
  - Status: Stub returns 501 Not Implemented
  - NOT a bug ✓

- [ ] Compliance violations tracking 
  - Plugin: `streamspace-compliance`
  - Status: Stub returns empty array
  - NOT a bug ✓

- [ ] Compliance reports/dashboard 
  - Plugin: `streamspace-compliance`
  - Status: Stub returns zero metrics
  - NOT a bug ✓

- [ ] Data Loss Prevention (DLP) 
  - Plugin: `streamspace-dlp`
  - Status: Plugin provides all features
  - Install plugin first

- [ ] Advanced audit logging 
  - Plugin: `streamspace-audit-advanced`
  - Status: Plugin provides all features
  - Install plugin first

### Session Management

- [ ] Session recording 
  - Plugin: `streamspace-recording`
  - Status: Core has session lifecycle; recording is plugin
  - Install plugin for recording features

- [ ] Session snapshots 
  - Plugin: `streamspace-snapshots`
  - Status: Core has session lifecycle; snapshots are plugin
  - Install plugin for snapshot features

- [ ] Multi-monitor support 
  - Plugin: `streamspace-multi-monitor`
  - Status: Single monitor is core; multi-monitor is plugin
  - Install plugin for multi-monitor features

### Business

- [ ] Billing & usage tracking 
  - Plugin: `streamspace-billing`
  - Status: Core has usage APIs; billing is plugin
  - Install plugin for billing features

- [ ] Advanced analytics & reports 
  - Plugin: `streamspace-analytics-advanced`
  - Status: Basic metrics in core; advanced analytics are plugin
  - Install plugin for advanced features

- [ ] Cost analysis and forecasting 
  - Plugin: `streamspace-billing`
  - Status: Plugin feature
  - Install plugin first

### Automation

- [ ] Workflow automation 
  - Plugin: `streamspace-workflows`
  - Status: Plugin provides all workflow features
  - Install plugin first

- [ ] Event-triggered automation 
  - Plugin: `streamspace-workflows`
  - Status: Core has webhooks; workflows are plugin
  - Install plugin for workflow features

### Notifications & Integrations

- [ ] Slack notifications 
  - Plugin: `streamspace-slack`
  - Status: Plugin provides all features
  - Install plugin first

- [ ] Teams notifications 
  - Plugin: `streamspace-teams`
  - Status: Plugin provides all features
  - Install plugin first

- [ ] Discord notifications 
  - Plugin: `streamspace-discord`
  - Status: Plugin provides all features
  - Install plugin first

- [ ] PagerDuty alerting 
  - Plugin: `streamspace-pagerduty`
  - Status: Plugin provides all features
  - Install plugin first

- [ ] Email notifications 
  - Plugin: `streamspace-email`
  - Status: Plugin provides SMTP integration
  - Install plugin first

- [ ] Calendar integration 
  - Plugin: `streamspace-calendar`
  - Status: Plugin provides Google/Outlook integration
  - Install plugin first

### Authentication & Identity

- [ ] SAML 2.0 authentication 
  - Plugin: `streamspace-auth-saml`
  - Status: Core has local auth; SAML is plugin
  - Install plugin for SAML features

- [ ] OAuth2 / OIDC authentication 
  - Plugin: `streamspace-auth-oauth`
  - Status: Core has local auth; OAuth2/OIDC is plugin
  - Install plugin for OAuth2/OIDC features

- [ ] Okta SSO 
  - Plugin: `streamspace-auth-saml` or `streamspace-auth-oauth`
  - Status: Supported via plugins
  - Install appropriate plugin

- [ ] Azure AD integration 
  - Plugin: `streamspace-auth-saml` or `streamspace-auth-oauth`
  - Status: Supported via plugins
  - Install appropriate plugin

- [ ] Google Workspace SSO 
  - Plugin: `streamspace-auth-saml` or `streamspace-auth-oauth`
  - Status: Supported via plugins
  - Install appropriate plugin

### Storage Backends

- [ ] AWS S3 storage 
  - Plugin: `streamspace-storage-s3`
  - Status: Plugin provides S3 backend
  - Install plugin first

- [ ] Azure Blob Storage 
  - Plugin: `streamspace-storage-azure`
  - Status: Plugin provides Azure backend
  - Install plugin first

- [ ] Google Cloud Storage 
  - Plugin: `streamspace-storage-gcs`
  - Status: Plugin provides GCS backend
  - Install plugin first

### Monitoring & Observability

- [ ] Datadog integration 
  - Plugin: `streamspace-datadog`
  - Status: Plugin provides integration
  - Install plugin first

- [ ] New Relic monitoring 
  - Plugin: `streamspace-newrelic`
  - Status: Plugin provides integration
  - Install plugin first

- [ ] Sentry error tracking 
  - Plugin: `streamspace-sentry`
  - Status: Plugin provides integration
  - Install plugin first

- [ ] Elastic APM 
  - Plugin: `streamspace-elastic-apm`
  - Status: Plugin provides integration
  - Install plugin first

- [ ] Honeycomb observability 
  - Plugin: `streamspace-honeycomb`
  - Status: Plugin provides integration
  - Install plugin first

---

## Features That ARE Core (Not Plugins)

Do NOT expect these to be plugins - they should always work:

- [ ] Session CRUD operations (create, read, update, delete)
- [ ] Session lifecycle (running, hibernated, terminated states)
- [ ] User management (basic local authentication)
- [ ] Template management and discovery
- [ ] Kubernetes pod/deployment/service management
- [ ] PVC provisioning and management
- [ ] Ingress and networking configuration
- [ ] WebSocket proxy for VNC
- [ ] Basic monitoring (Prometheus metrics)
- [ ] Plugin system (install, uninstall, enable/disable)
- [ ] WebSocket connections for real-time updates
- [ ] Pod logging
- [ ] Cluster resource queries
- [ ] Session sharing
- [ ] Session scheduling

---

## Action Items

When you find a feature not working:

1. **Check if it's plugin-based** using this checklist
2. **If plugin-based**: ✓ NOT a bug - install the plugin
3. **If core feature**: → File an issue, it's a bug

### Installing Plugins

```bash
# Via kubectl
kubectl apply -f plugin-repository.yaml

# Then install plugins from Admin → Plugins UI
```

### Verifying Plugin Installation

```bash
# Check if plugin is loaded
kubectl logs -n streamspace deploy/streamspace-api | grep "plugin.*loaded"

# Check plugin registry
curl http://localhost:3000/api/v1/plugins/installed
```

---

## Related Documentation

- [Plugin Architecture Reference](./PLUGIN_ARCHITECTURE_REFERENCE.md)
- [Plugin Development Guide](../PLUGIN_DEVELOPMENT.md)
- [Plugin API Reference](./PLUGIN_API.md)

