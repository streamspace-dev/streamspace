# StreamSpace Enterprise Features

**Version**: 1.0.0
**Last Updated**: 2025-11-15

This document provides comprehensive documentation for StreamSpace's enterprise-grade features designed for production deployments.

---

## Table of Contents

- [Overview](#overview)
- [1. Integration Hub](#1-integration-hub)
- [2. Security & Authentication](#2-security--authentication)
- [3. Session Scheduling](#3-session-scheduling)
- [4. Load Balancing & Auto-scaling](#4-load-balancing--auto-scaling)
- [5. Compliance & Governance](#5-compliance--governance)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)

---

## Overview

StreamSpace enterprise features provide production-ready capabilities for organizations requiring:

- **Integration**: Webhooks and external service integrations
- **Security**: Multi-factor authentication and IP whitelisting
- **Automation**: Scheduled sessions and calendar integration
- **Scalability**: Intelligent load balancing and auto-scaling
- **Compliance**: GDPR, HIPAA, and SOC2 compliance frameworks

All features are accessible through both the web UI and REST API, with role-based access control (RBAC) for administrative functions.

---

## 1. Integration Hub

**Purpose**: Connect StreamSpace to external systems via webhooks and service integrations.

### Features

#### Webhook Management
- Create custom webhooks with event subscriptions
- Automatic retry with exponential backoff
- Delivery tracking and status monitoring
- Secret-based signature verification

#### Supported Events
- `session.created` - New session started
- `session.updated` - Session configuration changed
- `session.deleted` - Session terminated
- `session.hibernated` - Session auto-hibernated
- `session.awakened` - Session resumed from hibernation
- `user.created` - New user account
- `user.updated` - User details changed
- `quota.exceeded` - Resource quota limit reached
- `plugin.installed` - New plugin activated
- `template.created` - New template added
- `security.alert` - Security event triggered
- `compliance.violation` - Policy violation detected
- `scaling.triggered` - Auto-scaling event
- `node.unhealthy` - Cluster node failure
- `backup.completed` - Backup operation finished
- `backup.failed` - Backup operation failed
- `cost.threshold` - Cost limit approaching

#### External Integrations
- **Slack**: Send notifications to channels
- **Microsoft Teams**: Post to team channels
- **Discord**: Webhook notifications
- **PagerDuty**: Incident management
- **Email**: SMTP notifications

### Usage

#### Web UI (Admin)
Navigate to **Admin → Integrations** to:
1. Create webhooks with event selection
2. Configure external integrations
3. View delivery history
4. Test webhook endpoints

#### API Examples

**Create Webhook**:
```bash
curl -X POST https://streamspace.local/api/webhooks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Alerts",
    "url": "https://alerts.company.com/webhook",
    "secret": "your-secret-key",
    "events": ["session.created", "quota.exceeded"],
    "enabled": true,
    "retry_policy": {
      "max_attempts": 3,
      "backoff_seconds": 60
    }
  }'
```

**List Webhook Deliveries**:
```bash
curl https://streamspace.local/api/webhooks/1/deliveries \
  -H "Authorization: Bearer $TOKEN"
```

### Webhook Payload Format

```json
{
  "event": "session.created",
  "timestamp": "2025-11-15T10:30:00Z",
  "data": {
    "session_id": "user1-firefox",
    "user": "user1",
    "template": "firefox-browser",
    "state": "running"
  },
  "signature": "sha256=..."
}
```

---

## 2. Security & Authentication

**Purpose**: Enterprise-grade security with MFA and network access controls.

### Features

#### Multi-Factor Authentication (MFA)
- **TOTP (Authenticator Apps)**: Google Authenticator, Authy, 1Password
- **SMS**: Text message verification codes
- **Email**: Email-based verification codes
- **Backup Codes**: 10 single-use recovery codes

#### IP Whitelisting
- Allow specific IP addresses or CIDR ranges
- Per-user or global whitelisting
- Temporary access grants with expiration
- Audit logging for access attempts

#### Security Alerts
- Failed login attempts
- Unusual access patterns
- MFA setup changes
- IP whitelist violations
- Privilege escalation attempts

### Usage

#### Web UI (User)
Navigate to **Security** to:
1. Set up MFA methods
2. Manage IP whitelist entries
3. View security alerts

#### MFA Setup Flow

**1. Initiate Setup**:
```bash
curl -X POST https://streamspace.local/api/security/mfa/setup \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"type": "totp"}'
```

**Response**:
```json
{
  "mfa_id": 123,
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code_url": "otpauth://totp/StreamSpace:user@example.com?secret=..."
}
```

**2. Verify Setup**:
```bash
curl -X POST https://streamspace.local/api/security/mfa/123/verify \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"code": "123456"}'
```

**Response**:
```json
{
  "verified": true,
  "backup_codes": [
    "ABC123-456789",
    "DEF456-123789",
    ...
  ]
}
```

#### IP Whitelist Management

**Add IP Address**:
```bash
curl -X POST https://streamspace.local/api/security/ip-whitelist \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "ip_address": "192.168.1.100",
    "description": "Office network",
    "enabled": true
  }'
```

**Add CIDR Range**:
```bash
curl -X POST https://streamspace.local/api/security/ip-whitelist \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "ip_address": "10.0.0.0/24",
    "description": "VPN subnet",
    "enabled": true
  }'
```

---

## 3. Session Scheduling

**Purpose**: Automated session management with calendar integration.

### Features

#### Schedule Types
- **One-time**: Single execution at specified time
- **Daily**: Repeat every day at specified time
- **Weekly**: Repeat on selected days of week
- **Monthly**: Repeat on specific day of month
- **Cron**: Custom cron expression

#### Schedule Options
- **Timezone Support**: Schedule in any timezone
- **Auto-termination**: Automatically end sessions after duration
- **Pre-warming**: Start sessions before scheduled time
- **Template Selection**: Choose any available template
- **Resource Configuration**: Set CPU/memory limits

#### Calendar Integration
- **Google Calendar**: Sync sessions to Google Calendar
- **Outlook Calendar**: Sync to Microsoft Outlook
- **iCal Export**: Download .ics file for any calendar app
- **Two-way Sync**: Calendar events create sessions

### Usage

#### Web UI (User)
Navigate to **Scheduling** to:
1. Create scheduled sessions
2. Connect calendar accounts
3. Export to iCal format
4. View upcoming sessions

#### API Examples

**Create Daily Schedule**:
```bash
curl -X POST https://streamspace.local/api/scheduling/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Morning standup workspace",
    "template_id": "vscode-dev",
    "schedule": {
      "type": "daily",
      "time_of_day": "09:00"
    },
    "timezone": "America/New_York",
    "auto_terminate": true,
    "terminate_after": 480,
    "enabled": true
  }'
```

**Create Weekly Schedule**:
```bash
curl -X POST https://streamspace.local/api/scheduling/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Weekly team review",
    "template_id": "firefox-browser",
    "schedule": {
      "type": "weekly",
      "days_of_week": [1, 3, 5],
      "time_of_day": "14:00"
    },
    "timezone": "UTC",
    "pre_warm": true,
    "pre_warm_minutes": 5,
    "enabled": true
  }'
```

**Connect Google Calendar**:
```bash
curl -X POST https://streamspace.local/api/scheduling/calendar/connect \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"provider": "google"}'
```

**Export to iCal**:
```bash
curl https://streamspace.local/api/scheduling/ical \
  -H "Authorization: Bearer $TOKEN" \
  -o streamspace-schedule.ics
```

---

## 4. Load Balancing & Auto-scaling

**Purpose**: Intelligent workload distribution and automatic capacity management.

### Features

#### Load Balancing Strategies
1. **Round Robin**: Distribute evenly across nodes
2. **Least Connections**: Route to node with fewest sessions
3. **Resource-Based**: Consider CPU/memory availability
4. **Affinity-Based**: Keep user sessions on same node
5. **Custom**: Define custom placement rules

#### Auto-scaling Policies
- **Horizontal Scaling**: Add/remove worker nodes
- **Vertical Scaling**: Adjust node resources
- **Metric-Based**: CPU, memory, or custom metrics
- **Schedule-Based**: Scale for known peak times
- **Predictive**: Machine learning-based forecasting

#### Node Monitoring
- Real-time CPU/memory usage
- Active session counts
- Health status tracking
- Resource capacity planning
- Historical performance data

### Usage

#### Web UI (Admin)
Navigate to **Admin → Scaling** to:
1. View cluster node status
2. Configure load balancing policies
3. Set up auto-scaling rules
4. Trigger manual scaling operations
5. Review scaling history

#### API Examples

**Get Node Status**:
```bash
curl https://streamspace.local/api/scaling/nodes \
  -H "Authorization: Bearer $TOKEN"
```

**Response**:
```json
{
  "nodes": [
    {
      "node_name": "worker-1",
      "cpu_percent": 45.2,
      "memory_percent": 62.8,
      "active_sessions": 12,
      "health_status": "healthy",
      "capacity": {
        "max_sessions": 50,
        "cpu_cores": 8,
        "memory_gb": 32
      }
    }
  ],
  "cluster_summary": {
    "total_nodes": 3,
    "healthy_nodes": 3,
    "total_sessions": 35,
    "avg_cpu": 42.5,
    "avg_memory": 58.3
  }
}
```

**Create Load Balancing Policy**:
```bash
curl -X POST https://streamspace.local/api/scaling/load-balancing \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Production LB",
    "strategy": "resource-based",
    "weight_cpu": 0.6,
    "weight_memory": 0.4,
    "enabled": true
  }'
```

**Create Auto-scaling Policy**:
```bash
curl -X POST https://streamspace.local/api/scaling/autoscaling \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Scale on CPU",
    "scaling_mode": "horizontal",
    "metric_type": "cpu",
    "metric_threshold": 75,
    "min_replicas": 2,
    "max_replicas": 10,
    "cooldown_seconds": 300,
    "enabled": true
  }'
```

**Trigger Manual Scaling**:
```bash
curl -X POST https://streamspace.local/api/scaling/autoscaling/1/trigger \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "action": "scale_up",
    "target_replicas": 5
  }'
```

---

## 5. Compliance & Governance

**Purpose**: Meet regulatory requirements and enforce organizational policies.

### Features

#### Compliance Frameworks
- **GDPR**: EU data protection regulation
- **HIPAA**: Healthcare data security
- **SOC2**: Service organization controls
- **PCI-DSS**: Payment card industry standards
- **ISO 27001**: Information security management
- **Custom**: Define organization-specific frameworks

#### Policy Enforcement
- Data retention policies
- Access control policies
- Encryption requirements
- Audit logging requirements
- Session recording policies
- Data export restrictions

#### Violation Tracking
- Automatic policy violation detection
- Severity classification (low/medium/high/critical)
- Remediation workflows
- Compliance officer notifications
- Audit trail maintenance

#### Reporting
- Compliance dashboard
- Violation reports
- Audit logs export
- Framework-specific reports
- Executive summaries

### Usage

#### Web UI (Admin)
Navigate to **Admin → Compliance** to:
1. View compliance dashboard
2. Configure frameworks and policies
3. Track and resolve violations
4. Generate audit reports

#### API Examples

**Get Compliance Dashboard**:
```bash
curl https://streamspace.local/api/compliance/dashboard \
  -H "Authorization: Bearer $TOKEN"
```

**Response**:
```json
{
  "overall_score": 87.5,
  "frameworks": [
    {
      "framework": "GDPR",
      "compliance_score": 92.3,
      "policies_total": 15,
      "policies_compliant": 14,
      "open_violations": 2
    }
  ],
  "recent_violations": [
    {
      "id": 101,
      "policy_id": 5,
      "severity": "medium",
      "status": "acknowledged",
      "created_at": "2025-11-15T08:00:00Z"
    }
  ],
  "compliance_trends": {
    "last_30_days": 88.1,
    "last_90_days": 85.7
  }
}
```

**List Compliance Violations**:
```bash
curl https://streamspace.local/api/compliance/violations?severity=high&status=open \
  -H "Authorization: Bearer $TOKEN"
```

**Resolve Violation**:
```bash
curl -X PATCH https://streamspace.local/api/compliance/violations/101/resolve \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "resolution": "Implemented encryption for data at rest",
    "remediation_steps": [
      "Enabled encryption on all volumes",
      "Updated security policies",
      "Notified affected users"
    ]
  }'
```

**Generate Compliance Report**:
```bash
curl -X POST https://streamspace.local/api/compliance/reports \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "framework": "GDPR",
    "report_type": "full_audit",
    "start_date": "2025-10-01",
    "end_date": "2025-10-31",
    "format": "pdf"
  }'
```

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────┐
│                  Web UI (React)                     │
│  - Admin Panels    - User Dashboards                │
│  - Configuration   - Monitoring Views                │
└──────────────────┬──────────────────────────────────┘
                   │ HTTPS/WSS
┌──────────────────▼──────────────────────────────────┐
│              API Backend (Go/Gin)                    │
│  ┌──────────┬──────────┬──────────┬──────────┐      │
│  │Integration│Security  │Scheduling│ Scaling  │      │
│  │  Handler  │ Handler  │ Handler  │ Handler  │      │
│  └──────────┴──────────┴──────────┴──────────┘      │
└──────────────────┬──────────────────────────────────┘
                   │
    ┌──────────────┼──────────────────┐
    │              │                  │
┌───▼─────┐  ┌────▼────┐   ┌────────▼──────┐
│PostgreSQL│  │ Redis   │   │  Kubernetes   │
│  (State) │  │ (Cache) │   │  (Sessions)   │
└──────────┘  └─────────┘   └───────────────┘
```

### Data Flow

1. **User Action** → Web UI component
2. **API Request** → API client with JWT auth
3. **Handler Processing** → Business logic execution
4. **State Persistence** → PostgreSQL database
5. **Cache Update** → Redis for performance
6. **Kubernetes Sync** → CRD updates
7. **Response** → JSON to client
8. **UI Update** → React state update

### Security Layers

1. **Authentication**: JWT tokens via OIDC/SAML
2. **Authorization**: RBAC with user/admin roles
3. **Transport**: TLS 1.3 encryption
4. **Storage**: Encrypted data at rest
5. **Audit**: All actions logged
6. **MFA**: Additional verification layer
7. **IP Filtering**: Network-level access control

---

## Quick Start

### Enable Enterprise Features

**1. Set Environment Variables**:
```bash
# In api/.env
ENABLE_ENTERPRISE_FEATURES=true
WEBHOOK_SIGNING_SECRET=your-secret-key
MFA_ISSUER=StreamSpace
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=notifications@company.com
SMTP_PASSWORD=app-password
```

**2. Database Migrations**:
```bash
cd api
./bin/migrate up
```

**3. Restart Services**:
```bash
kubectl rollout restart deployment/streamspace-api -n streamspace
kubectl rollout restart deployment/streamspace-ui -n streamspace
```

**4. Verify Access**:
- Login as admin user
- Navigate to **Admin** menu
- Verify new menu items appear:
  - Integrations
  - Scaling
  - Compliance

### Configuration

**Helm Values** (`chart/values.yaml`):
```yaml
enterprise:
  enabled: true

  integrations:
    webhooks:
      enabled: true
      maxRetries: 3

  security:
    mfa:
      enabled: true
      issuer: "StreamSpace"
    ipWhitelist:
      enabled: true

  scheduling:
    enabled: true
    calendar:
      google:
        clientId: "your-client-id"
        clientSecret: "your-secret"

  scaling:
    loadBalancing:
      enabled: true
      strategy: "resource-based"
    autoScaling:
      enabled: true

  compliance:
    enabled: true
    frameworks:
      - "GDPR"
      - "SOC2"
```

---

## API Reference

Complete API documentation available at:
- **OpenAPI Spec**: `/api/docs/openapi.yaml`
- **Swagger UI**: `https://streamspace.local/api/docs`
- **Postman Collection**: `/api/docs/streamspace.postman_collection.json`

### Base URL
```
https://streamspace.local/api
```

### Authentication
All API requests require authentication:
```bash
Authorization: Bearer <JWT_TOKEN>
```

### Rate Limiting
- **Standard**: 100 requests/minute
- **Admin**: 1000 requests/minute
- **Webhooks**: 10 requests/second

### Error Handling
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": {
      "field": "email",
      "issue": "Must be valid email address"
    }
  }
}
```

---

## Support

**Documentation**: https://docs.streamspace.io
**API Reference**: https://api.streamspace.io/docs
**Community**: https://community.streamspace.io
**Enterprise Support**: enterprise@streamspace.io

---

**Copyright © 2025 StreamSpace. All rights reserved.**
