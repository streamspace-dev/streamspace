# Vendor Assessment Template

**Version**: v1.0
**Last Updated**: 2025-11-26
**Owner**: Security + Procurement
**Status**: Template Document
**Usage**: Third-party integration evaluation

---

## Introduction

This template provides a standardized framework for assessing third-party vendors and integrations (SSO providers, storage backends, monitoring services, etc.) to ensure they meet StreamSpace's security, privacy, and operational requirements.

**Use This Template When**:
- Integrating new SSO provider (Okta, Auth0, Azure AD)
- Adding storage backend (S3, Azure Blob, GCS)
- Onboarding monitoring service (Datadog, New Relic)
- Evaluating API/service dependencies
- Any vendor with access to customer data

---

## Assessment Process

### Step 1: Initial Screening (30 minutes)

**Purpose**: Quick go/no-go decision before detailed assessment

**Questions**:
1. Does vendor have SOC 2 Type II certification? (Y/N)
2. Does vendor support enterprise SSO? (Y/N)
3. Is vendor financially stable (> 2 years in business)? (Y/N)
4. Does vendor have acceptable data processing agreement (DPA)? (Y/N)

**Decision**:
- **All Yes** → Proceed to detailed assessment
- **Any No** → Escalate to security team for review

---

### Step 2: Detailed Assessment (2-4 hours)

**Complete all sections below**

---

### Step 3: Risk Scoring (15 minutes)

**Calculate risk score** using scoring matrix (see below)

---

### Step 4: Approval (1 week)

**Approval Required**:
- **Low Risk** (Score 0-30): Engineering Lead
- **Medium Risk** (Score 31-60): Security + Engineering
- **High Risk** (Score 61-100): Executive approval

---

## Vendor Information

| Field | Response |
|-------|----------|
| **Vendor Name** | |
| **Website** | |
| **Service/Product** | |
| **Primary Contact** | (Name, email, phone) |
| **Contract Term** | (e.g., 1 year, month-to-month) |
| **Annual Cost** | (USD) |
| **Data Classification** | (Public, Internal, Confidential, Restricted) |
| **Assessment Date** | |
| **Assessor** | (Name, role) |

---

## Security Assessment

### 1. Certifications & Compliance

| Certification | Status | Expiry Date | Notes |
|---------------|--------|-------------|-------|
| **SOC 2 Type I** | ☐ Yes ☐ No | | (Copy of report received?) |
| **SOC 2 Type II** | ☐ Yes ☐ No | | (Preferred) |
| **ISO 27001** | ☐ Yes ☐ No | | |
| **HIPAA Compliance** | ☐ Yes ☐ No ☐ N/A | | (If handling PHI) |
| **PCI DSS** | ☐ Yes ☐ No ☐ N/A | | (If handling payments) |
| **GDPR Compliance** | ☐ Yes ☐ No | | (EU customers) |

**Score**:
- SOC 2 Type II: +20 points
- SOC 2 Type I: +10 points
- ISO 27001: +10 points
- HIPAA (if applicable): +10 points
- GDPR compliance: +5 points

**Minimum Requirement**: SOC 2 Type I for vendors handling customer data

---

### 2. Data Security

| Question | Response | Score |
|----------|----------|-------|
| **Encryption at Rest** | ☐ AES-256 ☐ Other: ___ ☐ None | AES-256: +10, Other: +5, None: -20 |
| **Encryption in Transit** | ☐ TLS 1.3 ☐ TLS 1.2 ☐ Other: ___ | TLS 1.3: +10, TLS 1.2: +5, Other: 0 |
| **Data Residency** | ☐ US ☐ EU ☐ Asia ☐ Multi-region | (Customer requirement dependent) |
| **Data Retention** | (Days after account deletion) | < 30 days: +5, > 90 days: -5 |
| **Data Backup** | ☐ Yes ☐ No | Yes: +5, No: -10 |
| **Disaster Recovery** | ☐ Documented ☐ Tested ☐ None | Tested: +10, Documented: +5, None: -10 |
| **Access Controls** | ☐ MFA enforced ☐ MFA optional ☐ None | Enforced: +10, Optional: +5, None: -15 |
| **Audit Logging** | ☐ Yes ☐ No | Yes: +5, No: -5 |

**Data Classification Requirements**:
- **Restricted**: SOC 2 Type II + AES-256 + MFA enforced (required)
- **Confidential**: SOC 2 Type I + TLS 1.2+ + MFA optional (minimum)
- **Internal/Public**: Basic security (TLS, access controls)

---

### 3. Availability & Performance

| Question | Response | Score |
|----------|----------|-------|
| **SLA Uptime** | ☐ 99.9% ☐ 99.5% ☐ 99.0% ☐ None | 99.9%: +10, 99.5%: +5, 99.0%: +0, None: -10 |
| **SLA Credits** | ☐ Yes ☐ No | Yes: +5, No: 0 |
| **Incident Response** | (SLA for P0 incidents) | < 1h: +10, < 4h: +5, None: -5 |
| **Planned Maintenance** | ☐ < 4h/month ☐ < 8h/month ☐ > 8h/month | < 4h: +5, < 8h: +0, > 8h: -5 |
| **Historical Uptime** | (Last 12 months, from status page) | (Validate against SLA) |
| **Load Balancing** | ☐ Multi-region ☐ Single region ☐ None | Multi: +10, Single: +5, None: 0 |

**Minimum Requirement**: 99.5% uptime SLA for critical vendors (SSO, database)

---

### 4. Vendor Stability

| Question | Response | Score |
|----------|----------|-------|
| **Years in Business** | | > 5 years: +10, 2-5: +5, < 2: -5 |
| **Funding Status** | ☐ Profitable ☐ Funded ☐ Unknown | Profitable: +10, Funded: +5, Unknown: -10 |
| **Customer Count** | | > 1000: +10, 100-1000: +5, < 100: 0 |
| **Notable Customers** | (List Fortune 500 customers) | Fortune 500: +5 per customer (max +20) |
| **Acquisition Risk** | ☐ Low ☐ Medium ☐ High | Low: +5, Medium: 0, High: -10 |
| **Open Source** | ☐ Yes ☐ No | Yes: +10 (reduced vendor lock-in) |

**Red Flags**:
- Company < 1 year old + unfunded
- No public customer references
- Frequent leadership changes (check LinkedIn)

---

### 5. Privacy & Data Processing

| Question | Response | Notes |
|----------|----------|-------|
| **Data Processing Agreement (DPA)** | ☐ Standard ☐ Custom ☐ None | (Attach DPA) |
| **GDPR Sub-Processor** | ☐ Yes ☐ No ☐ N/A | (If EU customers) |
| **Data Sharing** | ☐ No third parties ☐ Disclosed ☐ Undisclosed | (Review privacy policy) |
| **Data Access** | ☐ Need-to-know ☐ Broad access | |
| **Data Anonymization** | ☐ Yes ☐ No ☐ N/A | (For analytics vendors) |
| **Right to Delete** | ☐ < 30 days ☐ < 90 days ☐ > 90 days | (GDPR requirement) |

**Privacy Policy Review**:
- ☐ Privacy policy reviewed (link: ___)
- ☐ No concerning clauses (data selling, broad sharing)
- ☐ GDPR/CCPA compliant

---

### 6. Integration & API Security

| Question | Response | Score |
|----------|----------|-------|
| **Authentication** | ☐ OAuth 2.0 ☐ API Key ☐ Basic Auth | OAuth: +10, API Key: +5, Basic: -5 |
| **API Key Rotation** | ☐ Supported ☐ Not supported | Supported: +5, Not: -5 |
| **Rate Limiting** | ☐ Yes ☐ No | Yes: +5, No: 0 |
| **Webhook Signatures** | ☐ HMAC ☐ None | HMAC: +10, None: -10 |
| **IP Whitelisting** | ☐ Supported ☐ Not supported | Supported: +5, Not: 0 |
| **API Versioning** | ☐ Versioned ☐ Unversioned | Versioned: +5, Unversioned: -5 |
| **API Documentation** | ☐ Excellent ☐ Good ☐ Poor | Excellent: +5, Good: 0, Poor: -5 |

**Security Scan**:
- ☐ Performed API security scan (e.g., OWASP ZAP)
- ☐ No critical vulnerabilities found
- ☐ TLS configuration validated (SSL Labs: A+ rating)

---

### 7. Incident Response & Breach Notification

| Question | Response | Score |
|----------|----------|-------|
| **Breach Notification** | ☐ < 24h ☐ < 72h ☐ None | < 24h: +10, < 72h: +5, None: -20 |
| **Incident History** | (Public breaches in last 3 years) | No breaches: +10, 1 breach: -10, 2+: -20 |
| **Incident Response Plan** | ☐ Public ☐ Available on request ☐ None | Public: +10, Request: +5, None: -10 |
| **Vulnerability Disclosure** | ☐ Bug bounty ☐ security@vendor ☐ None | Bug bounty: +10, Email: +5, None: -5 |

**Incident History Review**:
- Search: `"[Vendor Name]" data breach` (Google, HaveIBeenPwned)
- Review: Incident timeline, root cause, remediation
- Red flag: Breach not disclosed publicly

---

## Operational Assessment

### 8. Support & SLA

| Question | Response |
|----------|----------|
| **Support Channels** | ☐ 24/7 phone ☐ Email ☐ Chat ☐ Ticket | (Required channels for P0) |
| **Support SLA** | (P0 response time) | |
| **Account Manager** | ☐ Dedicated ☐ Shared ☐ None | |
| **Escalation Path** | ☐ Documented ☐ Undocumented | |
| **Status Page** | (URL) | |

---

### 9. Contract & Legal

| Question | Response | Notes |
|----------|----------|-------|
| **Contract Term** | | (Lock-in period?) |
| **Auto-Renewal** | ☐ Yes ☐ No | (Cancellation notice period?) |
| **Termination Clause** | ☐ < 30 days ☐ 30-90 days ☐ > 90 days | |
| **Data Export** | ☐ API ☐ Manual ☐ None | (Exit strategy) |
| **Liability Cap** | | (Contract value multiple?) |
| **Indemnification** | ☐ Mutual ☐ Vendor only ☐ None | |

**Legal Review**:
- ☐ Contract reviewed by legal team
- ☐ No concerning IP clauses
- ☐ Data ownership clear (customer owns data)

---

## Risk Scoring

### Risk Score Calculation

**Total Score** = Sum of all section scores

| Risk Level | Score Range | Approval Required | Notes |
|------------|-------------|-------------------|-------|
| **Low** | 80-100 | Engineering Lead | Recommended vendor |
| **Medium** | 50-79 | Security + Engineering | Acceptable with conditions |
| **High** | 30-49 | Executive | Risk mitigation plan required |
| **Critical** | < 30 | Executive + Board | Not recommended |

### Calculated Score

| Section | Score | Weight | Weighted Score |
|---------|-------|--------|----------------|
| Certifications & Compliance | | 25% | |
| Data Security | | 20% | |
| Availability & Performance | | 15% | |
| Vendor Stability | | 15% | |
| Privacy & Data Processing | | 10% | |
| Integration & API Security | | 10% | |
| Incident Response | | 5% | |
| **TOTAL** | | **100%** | |

---

## Risk Mitigation Plan

**For Medium/High Risk vendors**, document mitigation strategies:

| Risk | Mitigation | Owner | Target Date |
|------|------------|-------|-------------|
| Example: No SOC 2 | Request SOC 2 audit completion timeline | Security | Q2 2026 |
| Example: No MFA | Enforce IP whitelisting | Engineering | Immediate |
| | | | |

---

## Decision

### Recommendation

☐ **Approve** - Proceed with integration
☐ **Approve with Conditions** - (List conditions)
☐ **Reject** - (Reason)
☐ **Defer** - (Pending additional info)

### Approvers

| Role | Name | Date | Signature |
|------|------|------|-----------|
| **Engineering Lead** | | | |
| **Security Lead** | | | (Required for Medium+ risk) |
| **Executive** | | | (Required for High+ risk) |

---

## Post-Assessment Actions

### Onboarding Checklist

- [ ] Contract signed
- [ ] DPA executed
- [ ] Security questionnaire completed
- [ ] API keys generated and stored in vault (1Password, Vault)
- [ ] IP whitelist configured (if applicable)
- [ ] Monitoring/alerting configured (vendor status page)
- [ ] Runbook created (vendor-specific operations)
- [ ] Team trained (integration usage, incident procedures)
- [ ] Annual review scheduled (calendar invite)

### Ongoing Monitoring

- [ ] **Quarterly**: Review vendor status page (uptime, incidents)
- [ ] **Annually**: Re-assess vendor (updated SOC 2, contract renewal)
- [ ] **Continuous**: Monitor for breaches (Google Alerts, HaveIBeenPwned)

---

## Example Assessments

### Example 1: Okta (SSO Provider)

| Section | Score | Notes |
|---------|-------|-------|
| **Certifications** | 45/50 | SOC 2 Type II, ISO 27001, HIPAA, GDPR |
| **Data Security** | 65/70 | AES-256, TLS 1.3, MFA enforced, audit logs |
| **Availability** | 60/60 | 99.99% uptime SLA, multi-region, < 1h P0 response |
| **Vendor Stability** | 50/50 | 13 years, profitable, 18K+ customers (Fortune 500) |
| **Privacy** | 30/30 | Standard DPA, GDPR sub-processor, no data sharing |
| **API Security** | 45/50 | OAuth 2.0, API key rotation, versioned, excellent docs |
| **Incident Response** | 20/20 | < 24h breach notification, no breaches (3 years), bug bounty |
| **TOTAL** | **315/330** | **95/100 (Normalized)** |

**Risk Level**: ✅ **Low**
**Recommendation**: **Approved**

---

### Example 2: Acme Storage Inc. (Hypothetical Startup)

| Section | Score | Notes |
|---------|-------|-------|
| **Certifications** | 0/50 | No SOC 2, no ISO 27001 ❌ |
| **Data Security** | 45/70 | AES-256, TLS 1.2, MFA optional, no audit logs |
| **Availability** | 25/60 | 99.5% SLA, single region, 4h P0 response |
| **Vendor Stability** | 10/50 | 1 year old, funded (Series A), 50 customers |
| **Privacy** | 15/30 | Custom DPA (legal review needed), data shared with analytics |
| **API Security** | 25/50 | API key only, no rotation, unversioned API, poor docs |
| **Incident Response** | 5/20 | 72h breach notification, no public incident plan |
| **TOTAL** | **125/330** | **38/100 (Normalized)** |

**Risk Level**: ⚠️ **High**
**Recommendation**: **Approve with Conditions**

**Conditions**:
1. SOC 2 audit completion within 12 months (contractual requirement)
2. IP whitelist enforced (compensating control for no MFA enforcement)
3. Quarterly security reviews until SOC 2 complete
4. Annual re-assessment with option to terminate if conditions not met

---

## Templates & Tools

### Vendor Questionnaire (Email Template)

```
Subject: Security Questionnaire - [Your Company] Integration

Hi [Vendor Contact],

We're evaluating [Vendor Product] for integration with StreamSpace.
As part of our security review, please complete the attached questionnaire.

**Required Documents**:
1. SOC 2 Type II report (or Type I if Type II unavailable)
2. Data Processing Agreement (DPA)
3. Privacy Policy
4. Incident Response Plan (if available)

**Questions**:
- See attached questionnaire (vendor-assessment-questionnaire.xlsx)

**Timeline**: Please respond within 5 business days.

Thank you,
[Your Name]
[Your Title]
```

### Annual Re-Assessment Checklist

```markdown
## Annual Vendor Re-Assessment: [Vendor Name]

**Last Assessment**: [Date]
**Next Assessment Due**: [Date + 1 year]

### Review Checklist

- [ ] SOC 2 report renewed (check expiry date)
- [ ] No security breaches in past year (Google search + HaveIBeenPwned)
- [ ] Uptime met SLA (review status page)
- [ ] Contract renewal terms acceptable
- [ ] Pricing remains competitive (benchmark against alternatives)
- [ ] Integration still necessary (review usage metrics)
- [ ] New features/changes evaluated (security impact)

### Decision

☐ Continue (no changes)
☐ Continue (with contract renegotiation)
☐ Sunset (migration plan required)
```

---

## References

- **Vendor Security Alliance (VSA)**: https://www.vendorsecurityalliance.org/
- **CAIQ (Consensus Assessments Initiative Questionnaire)**: https://cloudsecurityalliance.org/research/caiq/
- **NIST Cybersecurity Framework**: https://www.nist.gov/cyberframework
- **SOC 2 Trust Service Criteria**: https://us.aicpa.org/content/dam/aicpa/interestareas/frc/assuranceadvisoryservices/downloadabledocuments/trust-services-criteria.pdf

---

**Version History**:
- **v1.0** (2025-11-26): Initial vendor assessment template
- **Next Review**: After 5 vendor assessments (validate template effectiveness)
