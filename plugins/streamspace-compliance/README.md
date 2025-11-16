# StreamSpace Compliance & Regulatory Framework Plugin

Comprehensive compliance management for GDPR, HIPAA, SOC2, ISO27001, PCI-DSS, FedRAMP, and custom regulatory frameworks.

## Features

### Compliance Frameworks
- **Pre-built Frameworks**
  - GDPR (General Data Protection Regulation)
  - HIPAA (Health Insurance Portability and Accountability Act)
  - SOC2 (System and Organization Controls 2)
  - ISO27001 (Information Security Management)
  - PCI-DSS (Payment Card Industry Data Security Standard)
  - FedRAMP (Federal Risk and Authorization Management Program)
  - Custom frameworks

- **Framework Controls**
  - Automated compliance checks
  - Control status tracking (compliant/non-compliant/unknown)
  - Evidence collection
  - Check scheduling

### Compliance Policies
- **Policy Types**
  - Data retention policies
  - Data classification policies
  - Access control policies
  - Audit requirement policies
  - Violation action policies

- **Enforcement Levels**
  - Advisory (log only)
  - Warning (notify but allow)
  - Blocking (prevent action)

- **Policy Scope**
  - Per-user policies
  - Team-based policies
  - Role-based policies
  - Organization-wide policies

### Violation Management
- **Automatic Detection**
  - Policy violation detection
  - Severity classification (low/medium/high/critical)
  - Real-time alerting

- **Violation Actions**
  - User notifications
  - Admin notifications
  - Automatic ticket creation
  - User suspension (critical violations)
  - Session termination
  - Escalation emails

- **Resolution Workflow**
  - Violation acknowledgment
  - Remediation tracking
  - Resolution documentation

### Compliance Reporting
- **Report Types**
  - Summary reports
  - Detailed control reports
  - Attestation reports
  - Violation trend reports

- **Automated Generation**
  - Scheduled monthly/quarterly reports
  - On-demand report generation
  - PDF export capability

- **Compliance Dashboard**
  - Real-time compliance status
  - Violation trends
  - Framework compliance rates
  - Recent violations

## Installation

### Via Plugin Marketplace

1. Navigate to **Admin → Plugins**
2. Search for "Compliance & Regulatory Framework"
3. Click **Install**
4. Configure frameworks and policies
5. Click **Enable**

### Manual Installation

```bash
cp -r streamspace-compliance /path/to/streamspace/plugins/
systemctl restart streamspace-api
```

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "defaultFrameworks": ["GDPR", "SOC2"],
  "autoEnforcement": true,
  "defaultEnforcementLevel": "warning"
}
```

### Full Configuration

```json
{
  "enabled": true,
  "defaultFrameworks": ["GDPR", "HIPAA", "SOC2", "ISO27001"],
  "autoEnforcement": true,
  "defaultEnforcementLevel": "warning",
  "dataRetentionDays": {
    "sessionData": 90,
    "recordings": 365,
    "auditLogs": 2555,
    "backups": 180
  },
  "violationActions": {
    "notifyUser": true,
    "notifyAdmin": true,
    "createTicket": true,
    "suspendOnCritical": false
  },
  "reportingSchedule": "0 0 1 * *",
  "escalationEmails": [
    "compliance@company.com",
    "security@company.com"
  ],
  "enableAutomaticChecks": true,
  "checkInterval": 24
}
```

## Usage

### Enable a Framework

```bash
POST /api/plugins/compliance/frameworks
{
  "name": "GDPR",
  "displayName": "GDPR Compliance",
  "version": "2018",
  "enabled": true,
  "controls": [
    {
      "id": "gdpr-art-5",
      "name": "Data Minimization",
      "category": "data_protection",
      "automated": true,
      "checkInterval": 24
    }
  ]
}
```

### Create a Compliance Policy

```bash
POST /api/plugins/compliance/policies
{
  "name": "Healthcare Data Protection",
  "frameworkId": 2,
  "appliesTo": {
    "allUsers": true
  },
  "enforcementLevel": "blocking",
  "dataRetention": {
    "enabled": true,
    "sessionDataDays": 365,
    "recordingDays": 2555,
    "auditLogDays": 2555,
    "autoPurge": true
  },
  "accessControls": {
    "requireMFA": true,
    "sessionTimeout": 15,
    "maxConcurrentSessions": 1
  }
}
```

### List Violations

```bash
GET /api/plugins/compliance/violations?severity=high&status=open
```

### Generate Compliance Report

```bash
POST /api/plugins/compliance/reports
{
  "frameworkId": 1,
  "reportType": "detailed",
  "startDate": "2025-01-01",
  "endDate": "2025-01-31"
}
```

### View Compliance Dashboard

```bash
GET /api/plugins/compliance/dashboard
```

**Response:**
```json
{
  "totalPolicies": 15,
  "activePolicies": 12,
  "totalOpenViolations": 3,
  "violationsBySeverity": {
    "critical": 0,
    "high": 1,
    "medium": 2,
    "low": 0
  },
  "recentViolations": [...]
}
```

## Pre-Built Frameworks

### GDPR (General Data Protection Regulation)

**Key Controls:**
- Data minimization
- Purpose limitation
- Storage limitation
- Right to erasure (right to be forgotten)
- Data portability
- Privacy by design

**Data Retention:**
- User data: 90 days after account deletion
- Audit logs: 7 years
- Consent records: Lifetime

### HIPAA (Health Insurance Portability and Accountability Act)

**Key Controls:**
- PHI access controls
- Audit trails (all PHI access)
- Encryption at rest and in transit
- Minimum necessary access
- Business associate agreements

**Data Retention:**
- Medical records: 6 years
- Audit logs: 6 years
- Security incidents: 6 years

### SOC2 (Type II)

**Key Controls:**
- Security (access controls, encryption)
- Availability (uptime monitoring)
- Processing integrity (data accuracy)
- Confidentiality (data protection)
- Privacy (PII handling)

**Audit Requirements:**
- Continuous monitoring
- Quarterly internal audits
- Annual external audits

### ISO27001

**Key Controls:**
- Information security policies
- Asset management
- Access control
- Cryptography
- Physical security
- Operations security
- Communications security
- Incident management

**Control Domains:** 14 domains, 114 controls

## Policy Examples

### Data Retention Policy

```json
{
  "name": "Standard Data Retention",
  "frameworkId": 1,
  "dataRetention": {
    "enabled": true,
    "sessionDataDays": 90,
    "recordingDays": 365,
    "auditLogDays": 2555,
    "backupDays": 180,
    "autoPurge": true,
    "purgeSchedule": "0 2 * * *"
  }
}
```

### MFA Enforcement Policy

```json
{
  "name": "Require MFA for All Users",
  "frameworkId": 3,
  "appliesTo": {"allUsers": true},
  "enforcementLevel": "blocking",
  "accessControls": {
    "requireMFA": true,
    "sessionTimeout": 30,
    "maxConcurrentSessions": 3
  },
  "violationActions": {
    "notifyUser": true,
    "blockAction": true
  }
}
```

### Sensitive Data Access Policy

```json
{
  "name": "Restricted Data Access Control",
  "frameworkId": 2,
  "appliesTo": {
    "roles": ["admin", "compliance_officer"]
  },
  "enforcementLevel": "blocking",
  "auditRequirements": {
    "logAllAccess": true,
    "requireJustification": true,
    "alertOnSuspicious": true
  },
  "accessControls": {
    "requireMFA": true,
    "allowedIPRanges": ["10.0.0.0/8"],
    "requireApproval": true
  }
}
```

## Automated Compliance Checks

The plugin runs automated checks for various controls:

### Access Control Checks
- Verify MFA is enabled for required users
- Check session timeout configurations
- Validate IP allowlists

### Data Protection Checks
- Verify encryption at rest
- Check data classification labels
- Validate retention policy enforcement

### Audit Checks
- Verify audit logging is enabled
- Check log retention periods
- Validate log integrity

## Violation Types

- **access_control_violation** - Unauthorized access attempt
- **data_retention_violation** - Data retained beyond policy
- **mfa_violation** - MFA not used when required
- **ip_restriction_violation** - Access from blocked IP
- **session_timeout_violation** - Session exceeded max duration
- **data_export_violation** - Unauthorized data export
- **classification_violation** - Improper data classification

## Escalation Workflow

1. **Violation Detected** → Automatic violation record created
2. **Severity Assessment** → Classified as low/medium/high/critical
3. **User Notification** → User notified of violation
4. **Admin Notification** → Admins alerted
5. **Ticket Creation** → Support ticket auto-created
6. **Escalation** → Critical violations escalated via email
7. **Enforcement** → Actions taken based on policy (block/suspend)
8. **Resolution** → Violation acknowledged and remediated
9. **Closure** → Violation closed with documentation

## Compliance Dashboard

Access via **Admin → Compliance** to view:

- Overall compliance status
- Active frameworks and controls
- Open violations by severity
- Compliance trends over time
- Recent policy changes
- Upcoming compliance checks
- Data retention statistics

## Best Practices

1. **Start with One Framework** - Enable one framework (e.g., SOC2) first
2. **Test in Advisory Mode** - Use "advisory" enforcement while testing
3. **Regular Reports** - Generate monthly compliance reports
4. **Review Violations Weekly** - Address violations promptly
5. **Update Policies Annually** - Review and update policies yearly
6. **Train Users** - Educate users on compliance requirements
7. **Document Everything** - Maintain evidence for audits
8. **Automate Checks** - Enable automated compliance checks
9. **Monitor Trends** - Watch for violation patterns
10. **External Audits** - Schedule annual third-party audits

## Troubleshooting

### Policies not enforcing

**Problem:** Violations occurring but no enforcement

**Solution:**
- Check `autoEnforcement` is `true`
- Verify enforcement level is not "advisory"
- Review policy scope matches users
- Check plugin is enabled

### Automated checks not running

**Problem:** Scheduled compliance checks not executing

**Solution:**
- Verify scheduler jobs are enabled
- Check `enableAutomaticChecks` is `true`
- Review job logs for errors
- Validate cron expressions

### Reports failing to generate

**Problem:** Compliance report generation fails

**Solution:**
- Check database connectivity
- Verify framework has controls defined
- Ensure date range is valid
- Check user has admin role

## Support

- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Docs: https://docs.streamspace.io/plugins/compliance
- Compliance: https://docs.streamspace.io/compliance

## License

MIT License

## Version History

- **1.0.0** (2025-01-15)
  - Initial release
  - GDPR, HIPAA, SOC2, ISO27001 frameworks
  - Policy management and enforcement
  - Violation tracking and resolution
  - Automated compliance checks
  - Compliance reporting and dashboard
