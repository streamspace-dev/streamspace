# StreamSpace Plugin Architecture Analysis

## Overview

StreamSpace uses a comprehensive plugin system to extend the platform's functionality. This document identifies which features are **intentionally stubbed** in the core API because they are provided by optional plugins, not bugs.

---

## Intentional Core API Stubs

### Compliance Features (stubs.go, lines 1016-1098)

**Status**: Intentional stubs awaiting `streamspace-compliance` plugin

These endpoints return empty/stub data until the compliance plugin is installed:

- `ListComplianceFrameworks()` - Returns empty array
- `CreateComplianceFramework()` - Returns 501 Not Implemented
- `ListCompliancePolicies()` - Returns empty array
- `CreateCompliancePolicy()` - Returns 501 Not Implemented
- `ListViolations()` - Returns empty array
- `RecordViolation()` - Returns 501 Not Implemented
- `ResolveViolation()` - Returns 501 Not Implemented
- `GetComplianceDashboard()` - Returns zero metrics

**Plugin that provides real implementation**: `streamspace-compliance`

**File**: `/home/user/streamspace/plugins/streamspace-compliance/manifest.json`

---

## Complete Plugin Ecosystem

### 1. Security & Compliance Plugins

#### streamspace-compliance
- **Category**: Security
- **Type**: system
- **Purpose**: GDPR, HIPAA, SOC2, ISO27001, PCI-DSS, FedRAMP compliance management
- **Overrides**: The stub endpoints above
- **Features**:
  - Compliance framework management
  - Policy creation and enforcement
  - Violation tracking and resolution
  - Automated compliance checks
  - Compliance reporting and escalation
  - Data retention policies
- **Database Tables**: 6 tables for compliance data
- **API Endpoints**: 9 endpoints for framework/policy/violation management
- **UI Pages**: 5 admin pages for compliance dashboard, frameworks, policies, violations, reports

#### streamspace-dlp
- **Category**: Security
- **Type**: system
- **Purpose**: Data Loss Prevention (DLP)
- **Features**:
  - Clipboard controls
  - File transfer restrictions
  - Screen capture controls
  - Printing restrictions
  - USB device blocking
  - Network access controls

#### streamspace-audit-advanced
- **Category**: Security
- **Type**: system
- **Purpose**: Enhanced audit logging
- **Features**:
  - Advanced audit search
  - Export capabilities
  - Retention policies
  - Compliance reports

---

### 2. Session Management Plugins

#### streamspace-recording
- **Category**: Session Management
- **Type**: system
- **Purpose**: Session recording and playback
- **Features**:
  - Multiple format support (WebM, MP4, VNC)
  - Retention policies
  - Compliance-driven recording
  - Encryption support
- **Database Tables**: 2 tables (session_recordings, recording_playback)
- **API Endpoints**: 4 endpoints for recording/playback/download
- **Retention**: Default 365 days (configurable)

#### streamspace-snapshots
- **Category**: Session Management
- **Type**: system
- **Purpose**: Session snapshots and restore
- **Features**:
  - Create/manage/restore snapshots
  - Scheduling support
  - Sharing capabilities
  - Compression and encryption
  - Retention policies (default 90 days)
- **Database Tables**: 2 tables (session_snapshots, snapshot_schedules)
- **Max Snapshots**: Default 10 per session (configurable)

#### streamspace-multi-monitor
- **Category**: Advanced Features
- **Type**: system
- **Purpose**: Multi-monitor support
- **Features**:
  - Up to 16 monitors per session (configurable, max 8 default)
  - Multiple display layouts (horizontal, vertical, grid, custom)
  - Independent display streams
  - Custom layout support
- **API Endpoints**: 7 endpoints for monitor configuration and stream management

---

### 3. Automation & Workflow Plugins

#### streamspace-workflows
- **Category**: Automation
- **Type**: system
- **Purpose**: Workflow automation
- **Features**:
  - Event-driven workflows
  - Triggers and actions
  - Conditional logic
  - Workflow history tracking
  - Custom script support (optional)
- **Database Tables**: 3 tables (workflows, workflow_executions, workflow_actions)
- **Max Workflows**: Default 50 per user (configurable)

---

### 4. Business & Billing Plugins

#### streamspace-billing
- **Category**: Business
- **Type**: system
- **Purpose**: Usage tracking and billing
- **Features**:
  - Usage tracking (CPU, memory, storage)
  - Multiple billing modes (usage, subscription, hybrid)
  - Stripe integration for payments
  - Invoice generation and management
  - Subscription plan management
  - Cost calculation and reporting
  - Usage alerts and quotas
  - Auto-suspend on overage (optional)
- **Database Tables**: 5 tables (billing_usage_records, invoices, subscriptions, payments, credits)
- **Pricing Models**:
  - CPU: $0.05/core/hour
  - Memory: $0.01/GB/hour
  - Storage: $0.10/GB/month
- **UI**: Billing dashboard for users, admin billing management

#### streamspace-analytics-advanced
- **Category**: Analytics
- **Type**: system
- **Purpose**: Advanced analytics and reporting
- **Features**:
  - Usage trends analysis
  - Session metrics
  - User engagement tracking
  - Resource utilization analysis
  - Cost analysis
  - Custom reports

---

### 5. Monitoring & Observability Plugins

#### streamspace-datadog
- **Category**: Monitoring
- **Type**: system
- **Purpose**: Datadog integration
- **Features**:
  - Metrics export to Datadog
  - Trace collection
  - Log aggregation
  - APM integration

#### streamspace-newrelic
- **Category**: Monitoring
- **Type**: system
- **Purpose**: New Relic monitoring
- **Features**:
  - Performance metrics
  - Distributed tracing
  - Event tracking
  - Full-stack observability

#### streamspace-sentry
- **Category**: Monitoring
- **Type**: system
- **Purpose**: Error tracking with Sentry
- **Features**:
  - Error/exception tracking
  - Performance issue monitoring
  - Error alerting

#### streamspace-elastic-apm
- **Category**: Monitoring
- **Type**: system
- **Purpose**: Elastic APM integration
- **Features**:
  - Application Performance Monitoring
  - Distributed tracing
  - Performance metrics

#### streamspace-honeycomb
- **Category**: Monitoring
- **Type**: system
- **Purpose**: High-definition observability
- **Features**:
  - Deep system analysis
  - Debugging support
  - Trace collection

---

### 6. Notification & Integration Plugins

#### streamspace-slack
- **Category**: Integrations
- **Type**: webhook
- **Purpose**: Slack notifications
- **Features**:
  - Session event notifications
  - User event notifications
  - Custom channel routing
  - Configurable event triggers

#### streamspace-teams
- **Category**: Integrations
- **Type**: webhook
- **Purpose**: Microsoft Teams notifications
- **Features**:
  - Teams channel notifications
  - Event-driven messaging

#### streamspace-discord
- **Category**: Integrations
- **Type**: webhook
- **Purpose**: Discord notifications
- **Features**:
  - Discord channel notifications
  - Customizable messages

#### streamspace-pagerduty
- **Category**: Integrations
- **Type**: webhook
- **Purpose**: Incident alerting
- **Features**:
  - PagerDuty incident creation
  - Severity configuration
  - Alert routing

#### streamspace-email
- **Category**: Integrations
- **Type**: integration
- **Purpose**: Email notifications via SMTP
- **Features**:
  - Email notifications for events
  - HTML/text format support
  - Template support

---

### 7. Authentication Plugins

#### streamspace-auth-saml
- **Category**: Authentication
- **Type**: system
- **Purpose**: SAML 2.0 SSO
- **Supported Providers**:
  - Okta
  - OneLogin
  - Azure AD
  - Google Workspace
  - JumpCloud
  - Auth0
  - Custom SAML IdP
- **Features**:
  - IdP-initiated and SP-initiated login
  - Request signing
  - Force re-authentication
  - Attribute mapping
  - Auto-user provisioning
  - Role assignment
- **API Endpoints**: 5 endpoints (metadata, ACS, SLO, login, logout)

#### streamspace-auth-oauth
- **Category**: Authentication
- **Type**: system
- **Purpose**: OAuth2 / OIDC SSO
- **Supported Providers**:
  - Google
  - GitHub
  - GitLab
  - Okta
  - Azure AD
  - Auth0
  - Keycloak
  - Custom OIDC providers
- **Features**:
  - Modern OAuth2/OIDC flows
  - Multiple provider support
  - Auto-user provisioning
  - Custom claim mapping

---

### 8. Storage Plugins

#### streamspace-storage-s3
- **Category**: Storage
- **Type**: system
- **Purpose**: AWS S3 and S3-compatible storage
- **Supported Providers**:
  - AWS S3
  - MinIO
  - DigitalOcean Spaces
  - Wasabi
  - Custom S3-compatible services
- **Features**:
  - Recording storage
  - Snapshot storage
  - General file uploads
  - Encryption support (AES256, KMS)
  - SSL/TLS support
  - Path-style URLs for MinIO

#### streamspace-storage-azure
- **Category**: Storage
- **Type**: system
- **Purpose**: Azure Blob Storage
- **Features**:
  - Recording storage
  - Snapshot storage
  - Blob container management

#### streamspace-storage-gcs
- **Category**: Storage
- **Type**: system
- **Purpose**: Google Cloud Storage
- **Features**:
  - Recording storage
  - Snapshot storage
  - GCS bucket management

---

### 9. Integration & Scheduling Plugins

#### streamspace-calendar
- **Category**: Integrations
- **Type**: integration
- **Purpose**: Calendar integration (Google Calendar, Outlook)
- **Features**:
  - Google Calendar OAuth integration
  - Outlook Calendar OAuth integration
  - Automated session scheduling
  - iCalendar (.ics) export
  - Auto-sync at configurable intervals
  - Automatic event creation for scheduled sessions

---

## Summary Table: Plugin-Based vs Core Features

| Feature | Category | Status | Plugin | Notes |
|---------|----------|--------|--------|-------|
| Session recording | Session Mgmt | Plugin | `streamspace-recording` | Multiple formats, retention policies |
| Session snapshots | Session Mgmt | Plugin | `streamspace-snapshots` | Compression, encryption, scheduling |
| Multi-monitor support | Advanced | Plugin | `streamspace-multi-monitor` | Up to 16 monitors, custom layouts |
| Compliance frameworks | Security | Plugin (Stub) | `streamspace-compliance` | GDPR, HIPAA, SOC2, ISO27001 |
| DLP (Data Loss Prevention) | Security | Plugin | `streamspace-dlp` | Clipboard, file transfer, printing controls |
| Advanced audit logging | Security | Plugin | `streamspace-audit-advanced` | Search, export, retention, reports |
| Billing & usage tracking | Business | Plugin | `streamspace-billing` | Stripe, usage-based pricing, invoicing |
| Advanced analytics | Analytics | Plugin | `streamspace-analytics-advanced` | Trends, cost analysis, custom reports |
| Workflow automation | Automation | Plugin | `streamspace-workflows` | Event-driven, triggers, actions |
| Slack integration | Integrations | Plugin | `streamspace-slack` | Event notifications |
| Teams integration | Integrations | Plugin | `streamspace-teams` | Event notifications |
| Discord integration | Integrations | Plugin | `streamspace-discord` | Event notifications |
| PagerDuty integration | Integrations | Plugin | `streamspace-pagerduty` | Incident alerting |
| Email notifications | Integrations | Plugin | `streamspace-email` | SMTP-based notifications |
| Calendar integration | Integrations | Plugin | `streamspace-calendar` | Google Calendar, Outlook |
| SAML authentication | Auth | Plugin | `streamspace-auth-saml` | Enterprise SSO (Okta, Azure AD, etc.) |
| OAuth2/OIDC auth | Auth | Plugin | `streamspace-auth-oauth` | Modern SSO (Google, GitHub, etc.) |
| S3 storage backend | Storage | Plugin | `streamspace-storage-s3` | AWS S3, MinIO, Wasabi |
| Azure storage backend | Storage | Plugin | `streamspace-storage-azure` | Azure Blob Storage |
| GCS storage backend | Storage | Plugin | `streamspace-storage-gcs` | Google Cloud Storage |
| Datadog monitoring | Monitoring | Plugin | `streamspace-datadog` | Metrics, traces, logs |
| New Relic monitoring | Monitoring | Plugin | `streamspace-newrelic` | APM, distributed tracing |
| Sentry error tracking | Monitoring | Plugin | `streamspace-sentry` | Error tracking, performance |
| Elastic APM | Monitoring | Plugin | `streamspace-elastic-apm` | Performance monitoring |
| Honeycomb observability | Monitoring | Plugin | `streamspace-honeycomb` | High-definition observability |

---

## Plugin Installation & Management

### How Plugins Override Stubs

When a plugin is installed (e.g., `streamspace-compliance`), it:
1. Registers real API endpoint handlers that override the stub implementations
2. Creates necessary database tables
3. Registers UI components and pages
4. Subscribes to system events (webhooks)
5. Registers scheduler jobs for background tasks

### Core Features (Not Pluggable)

The following features are **core** to StreamSpace and are NOT plugin-based:

- Session lifecycle management (create, run, hibernate, terminate)
- User management and authentication (basic local auth)
- Template management and catalog
- Kubernetes integration and resource management
- WebSocket proxy for VNC connections
- Pod/deployment/service management
- PVC provisioning and management
- Ingress and networking
- Metrics and basic monitoring (no external service)
- Basic CRUD operations for sessions and templates
- Plugin system itself (plugin registry, discovery, install/uninstall)

---

## Key Design Principles

1. **Stubs Return Helpful Messages**: Compliance stub endpoints return clear error messages directing users to install the plugin
2. **No Core Functionality Locked Behind Plugins**: All essential platform features are in core
3. **Optional but Powerful**: Plugins add enterprise features without bloating the core
4. **Plugin Override Pattern**: Plugins can override stub endpoints with real implementations
5. **Database Isolation**: Each plugin can define its own database tables
6. **Event-Driven Architecture**: Plugins react to system events (session created, user logged in, etc.)

---

## Important Notes for Issue Tracking

When reviewing issues or TODOs:

1. **NOT a bug**: Compliance features returning empty data or 501 errors until plugin is installed
2. **NOT a bug**: Recording features not available until plugin is installed
3. **NOT a bug**: Billing endpoints not available until plugin is installed
4. **NOT a bug**: SAML/OAuth auth endpoints not available until plugins are installed
5. **NOT a bug**: Storage endpoints returning errors until storage plugin is configured

These are **intentional design patterns** - the stubs exist to provide graceful degradation and clear guidance to users.

---

## Plugin Manifest Schema

All plugins follow a consistent manifest structure defining:
- Plugin metadata (name, version, description, author)
- Type (extension, webhook, integration, system, theme)
- Permissions required
- Configuration schema (auto-generates UI forms)
- Database tables to create
- API endpoints to register
- UI pages/components to register
- Event subscriptions (webhooks)
- Scheduler jobs
- Lifecycle hooks (onLoad, onUnload)

