# Security Policy

## 🛡️ Security Status

**Current Status**: ⚠️ **PRE-PRODUCTION** - Not recommended for production use without addressing critical security issues.

StreamSpace is currently in active development (Phase 1). A comprehensive security review has been conducted, identifying 40 security issues across critical, high, medium, and low severity categories. See the full security audit report in this document.

**Last Security Review**: 2025-11-14

---

## 📋 Supported Versions

| Version | Supported          | Status |
| ------- | ------------------ | ------ |
| 0.1.x   | :white_check_mark: | Development - Security fixes only |
| < 0.1   | :x:                | Not supported |

**Note**: StreamSpace has not yet reached v1.0 production readiness. All versions are considered development releases.

---

## 🔒 Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in StreamSpace, please follow these steps:

### Preferred Method: Private Security Advisory

1. Go to the [Security Advisories page](https://github.com/JoshuaAFerguson/streamspace/security/advisories)
2. Click "Report a vulnerability"
3. Provide detailed information about the vulnerability
4. We will respond within **48 hours**

### Alternative: Email

Send an email to: **security@streamspace.io** (or repository maintainer email)

**Please include:**
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)
- Your contact information for follow-up

### What to Expect

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**:
  - Critical: 1-7 days
  - High: 7-30 days
  - Medium: 30-90 days
  - Low: Next release cycle

### Responsible Disclosure

Please give us a reasonable amount of time to fix the issue before public disclosure. We aim to:

1. Confirm the vulnerability within 48 hours
2. Develop and test a fix
3. Release a security patch
4. Publicly disclose the issue with credit to the reporter (if desired)

**We do not currently have a bug bounty program**, but we deeply appreciate security research and will acknowledge contributors in our security advisories and release notes.

---

## ⚠️ Known Security Issues

The following security issues have been identified and are being actively addressed:

### 🔴 Critical Severity (10 issues)

1. **Secrets in ConfigMaps** - Database credentials stored in plain text
2. **Unauthenticated API Routes** - Most endpoints lack authentication middleware
3. **Wide Open CORS** - Allows any origin with credentials
4. **Weak Default JWT Secret** - Hardcoded fallback secret
5. **SQL Injection Risk** - Insufficient validation on database connection strings
6. **No Rate Limiting** - API vulnerable to DoS attacks
7. **Elevated Pod Privileges** - Session pods can run with excessive permissions
8. **No CRD Input Validation** - Resource fields accept malformed input
9. **Webhook Authentication Missing** - Public webhooks without signature validation
10. **RBAC Over-Permissions** - Controller has excessive cluster permissions

### 🟠 High Severity (10 issues)

See full security audit report for complete list of high, medium, and low severity issues.

### Tracking

Active security issues are tracked in GitHub Issues with the `security` label:
- [View Open Security Issues](https://github.com/JoshuaAFerguson/streamspace/labels/security)

---

## 🎯 Security Roadmap

### Phase 1: Critical Fixes (Target: Week 1)
- [ ] Implement authentication middleware on all protected routes
- [ ] Fix CORS policy to whitelist specific origins
- [ ] Remove all default/hardcoded secrets
- [ ] Enable network policies by default
- [ ] Add input validation to CRDs
- [ ] Implement rate limiting
- [ ] Secure SAML cookies
- [ ] Add webhook authentication

### Phase 2: High Priority (Target: Week 2-3)
- [ ] Enable TLS on all ingress by default
- [ ] Implement Pod Security Standards
- [ ] Add comprehensive audit logging
- [ ] Enable ReadOnlyRootFilesystem
- [ ] Apply least-privilege RBAC
- [ ] Implement CSRF protection
- [ ] Add per-user resource quotas
- [ ] Container image vulnerability scanning in CI/CD

### Phase 3: Medium Priority (Target: Month 2)
- [ ] Hash session tokens before storage
- [ ] Encrypt database at rest
- [ ] Add request size limits
- [ ] Implement brute force protection
- [ ] Automated dependency vulnerability scanning
- [ ] Container image signing

### Phase 4: Continuous Improvement
- [ ] Regular penetration testing
- [ ] Security training for contributors
- [ ] Automated security testing in CI/CD
- [ ] Third-party security audit before v1.0

---

## 🏗️ Security Architecture

### Defense in Depth

StreamSpace implements multiple layers of security:

```
┌─────────────────────────────────────────┐
│  Network Layer                          │
│  - TLS/SSL encryption                   │
│  - Network policies                     │
│  - Ingress authentication               │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Application Layer                      │
│  - JWT authentication                   │
│  - RBAC authorization                   │
│  - Input validation                     │
│  - Rate limiting                        │
│  - CSRF protection                      │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Kubernetes Layer                       │
│  - Pod Security Standards               │
│  - RBAC policies                        │
│  - Network policies                     │
│  - Resource quotas                      │
│  - Secrets management                   │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Container Layer                        │
│  - Non-root user                        │
│  - Read-only root filesystem            │
│  - Dropped capabilities                 │
│  - Seccomp profiles                     │
└─────────────────────────────────────────┘
```

### Current Gaps

As of v0.1.0, several security layers are incomplete:
- Network policies disabled by default
- TLS not enforced
- Pod Security Standards not implemented
- Authentication middleware incomplete
- Rate limiting not implemented

**These gaps must be addressed before production deployment.**

---

## 🔐 Security Best Practices for Deployment

### 1. Secrets Management

**DO:**
- Use external secret management (HashiCorp Vault, AWS Secrets Manager, Sealed Secrets)
- Generate strong, random secrets during installation
- Rotate secrets regularly
- Mount secrets as files, not environment variables

**DON'T:**
- Use default passwords
- Store secrets in ConfigMaps
- Commit secrets to Git
- Use weak or predictable secrets

**Example: Generate Strong JWT Secret**
```bash
# Generate 256-bit random secret
openssl rand -base64 32

# Set during Helm installation
helm install streamspace ./chart \
  --set secrets.jwtSecret=$(openssl rand -base64 32) \
  --set secrets.postgresPassword=$(openssl rand -base64 32)
```

### 2. Network Security

**Enable TLS:**
```yaml
# values.yaml
ingress:
  tls:
    enabled: true
    certManager: true
    issuer: letsencrypt-prod
```

**Enable Network Policies:**
```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

**Restrict CORS:**
```yaml
api:
  cors:
    allowedOrigins:
      - https://streamspace.yourdomain.com
```

### 3. Authentication & Authorization

**Configure OIDC/SAML:**
```yaml
auth:
  oidc:
    enabled: true
    issuer: https://your-idp.com
    clientId: streamspace
    # clientSecret: provided via external secret
```

**Enable RBAC:**
```yaml
rbac:
  enabled: true
  strictMode: true
  defaultRole: user  # Not admin!
```

### 4. Pod Security

**Apply Pod Security Standards:**
```yaml
podSecurityStandards:
  enforce: restricted
  audit: restricted
  warn: restricted
```

**Container Security Context:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  seccompProfile:
    type: RuntimeDefault
```

### 5. Monitoring & Auditing

**Enable Audit Logging:**
```yaml
audit:
  enabled: true
  level: RequestResponse
  retention: 90d
```

**Configure Monitoring:**
```yaml
monitoring:
  prometheus:
    enabled: true
    serviceMonitor: true
  grafana:
    enabled: true
    dashboards: true
```

### 6. Database Security

**Enable TLS:**
```yaml
postgresql:
  tls:
    enabled: true
    certificatesSecret: postgres-tls
```

**Restrict Access:**
```yaml
postgresql:
  networkPolicy:
    enabled: true
    allowedNamespaces:
      - streamspace
```

### 7. Resource Limits

**Enforce Quotas:**
```yaml
resourceQuotas:
  enabled: true
  perUser:
    maxSessions: 5
    maxMemory: 16Gi
    maxCPU: 8000m
```

---

## 🧪 Security Testing

### Pre-Deployment Checklist

Before deploying StreamSpace to production, complete this security checklist:

- [ ] All secrets are generated and stored securely (no defaults)
- [ ] TLS is enabled on all ingress endpoints
- [ ] Network policies are enabled and tested
- [ ] CORS is configured with specific origins
- [ ] Authentication is enabled on all API routes
- [ ] RBAC follows least-privilege principle
- [ ] Pod Security Standards are enforced
- [ ] Rate limiting is configured
- [ ] Audit logging is enabled
- [ ] Database is encrypted at rest
- [ ] Container images are scanned for vulnerabilities
- [ ] All critical and high-severity issues are resolved
- [ ] Security testing has been performed

### Automated Security Scanning

**Container Image Scanning:**
```bash
# Scan all images with Trivy
trivy image --severity CRITICAL,HIGH streamspace/controller:v0.1.0
trivy image --severity CRITICAL,HIGH streamspace/api:v0.1.0
trivy image --severity CRITICAL,HIGH streamspace/ui:v0.1.0
```

**Kubernetes Manifest Scanning:**
```bash
# Scan manifests with kubesec
kubesec scan manifests/config/*.yaml

# Or with Checkov
checkov -d manifests/
```

**Dependency Scanning:**
```bash
# Go dependencies
go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth

# Node.js dependencies
npm audit --production
```

### Manual Security Testing

**Penetration Testing Focus Areas:**
1. Authentication bypass attempts
2. Authorization escalation
3. SQL injection in database queries
4. XSS in web UI
5. CSRF on state-changing operations
6. API rate limiting effectiveness
7. Session management
8. Secrets exposure
9. Container escape attempts
10. Network segmentation

---

## 📚 Security Resources

### Standards & Frameworks

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP Kubernetes Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Kubernetes_Security_Cheat_Sheet.html)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NSA/CISA Kubernetes Hardening Guide](https://media.defense.gov/2022/Aug/29/2003066362/-1/-1/0/CTR_KUBERNETES_HARDENING_GUIDANCE_1.2_20220829.PDF)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)

### Tools

- **Container Scanning**: [Trivy](https://github.com/aquasecurity/trivy), [Grype](https://github.com/anchore/grype)
- **Kubernetes Scanning**: [kubesec](https://github.com/controlplaneio/kubesec), [Checkov](https://github.com/bridgecrewio/checkov)
- **Dependency Scanning**: [Nancy](https://github.com/sonatype-nexus-community/nancy), [Snyk](https://snyk.io/)
- **Secret Detection**: [gitleaks](https://github.com/gitleaks/gitleaks), [TruffleHog](https://github.com/trufflesecurity/trufflehog)
- **Network Policy**: [Network Policy Editor](https://networkpolicy.io/)

### StreamSpace Documentation

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture
- [CONTROLLER_GUIDE.md](docs/CONTROLLER_GUIDE.md) - Controller implementation
- [CONTRIBUTING.md](CONTRIBUTING.md) - Security-aware development practices

---

## 🔄 Security Update Policy

### Release Cycle

- **Security Patches**: Released as soon as fixes are available
- **Version Format**: `vMAJOR.MINOR.PATCH-security.N`
- **Notification**: GitHub Security Advisories + Release Notes

### Supported Versions

We provide security updates for:
- Latest major version (v1.x when released)
- Previous major version for 6 months after new major release
- Development versions (v0.x) receive best-effort security fixes

### CVE Policy

- All security vulnerabilities will be assigned a CVE if applicable
- CVEs will be published to the [GitHub Advisory Database](https://github.com/advisories)
- Severity ratings follow [CVSS 3.1](https://www.first.org/cvss/)

---

## 🙏 Acknowledgments

We would like to thank the following for their contributions to StreamSpace security:

- Security researchers who responsibly disclose vulnerabilities
- Open source security tools and their maintainers
- The Kubernetes security community

**Want to contribute to StreamSpace security?** See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📞 Contact

- **Security Issues**: security@streamspace.io
- **General Questions**: GitHub Discussions
- **Bug Reports**: GitHub Issues (non-security bugs only)

---

**Last Updated**: 2025-11-14
**Next Security Review**: Scheduled for Phase 2 completion
