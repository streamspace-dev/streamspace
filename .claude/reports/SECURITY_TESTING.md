# Security Testing Guide

This document provides comprehensive guidance for security testing of the StreamSpace platform.

**Last Updated**: 2025-11-14
**Version**: 1.0.0

---

## Table of Contents

- [Overview](#overview)
- [Pre-Deployment Security Testing](#pre-deployment-security-testing)
- [Automated Security Scanning](#automated-security-scanning)
- [Manual Security Testing](#manual-security-testing)
- [Penetration Testing](#penetration-testing)
- [Compliance Testing](#compliance-testing)
- [Security Test Cases](#security-test-cases)
- [Tools and Resources](#tools-and-resources)

---

## Overview

StreamSpace implements multiple layers of security controls. This guide outlines how to test each layer to ensure proper configuration and effectiveness.

### Security Testing Principles

1. **Defense in Depth**: Test all security layers (network, application, container, Kubernetes)
2. **Continuous Testing**: Integrate security tests into CI/CD pipeline
3. **Shift Left**: Test security early in development lifecycle
4. **Automated + Manual**: Combine automated scanning with manual testing
5. **Responsible Disclosure**: Report vulnerabilities through proper channels

---

## Pre-Deployment Security Testing

Before deploying StreamSpace to production, complete this security testing checklist:

### 1. Configuration Review

#### JWT Secret
```bash
# Verify JWT_SECRET is set and strong
echo $JWT_SECRET | wc -c  # Should be >= 32 characters

# Test with weak secret (should fail)
JWT_SECRET="weak" ./api
# Expected: "SECURITY ERROR: JWT_SECRET must be at least 32 characters long"

# Test with no secret (should fail)
unset JWT_SECRET
./api
# Expected: "SECURITY ERROR: JWT_SECRET environment variable must be set"
```

#### CORS Configuration
```bash
# Verify CORS is properly configured
echo $CORS_ALLOWED_ORIGINS

# Test: Should contain specific origins, not "*"
# Good: https://streamspace.example.com,https://app.example.com
# Bad: *

# Test CORS from unauthorized origin
curl -H "Origin: https://evil.com" \
  -H "Access-Control-Request-Method: POST" \
  -X OPTIONS http://localhost:8000/api/v1/sessions
# Expected: No Access-Control-Allow-Origin header in response
```

#### Database Security
```bash
# Verify SSL/TLS is enabled
echo $DB_SSL_MODE
# Expected: "require", "verify-ca", or "verify-full" (NOT "disable")

# Test database connection
psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=$DB_SSL_MODE"
```

#### Webhook Authentication
```bash
# Verify webhook secret is set
echo $WEBHOOK_SECRET | wc -c  # Should be >= 32 characters

# Test webhook without signature (should fail)
curl -X POST http://localhost:8000/webhooks/repository/sync \
  -H "Content-Type: application/json" \
  -d '{"event":"push"}'
# Expected: 401 Unauthorized
```

### 2. Pod Security Standards

```bash
# Verify namespace has Pod Security Standards labels
kubectl get namespace streamspace -o yaml | grep pod-security
# Expected:
#   pod-security.kubernetes.io/enforce: restricted
#   pod-security.kubernetes.io/audit: restricted
#   pod-security.kubernetes.io/warn: restricted

# Test: Try to create privileged pod (should fail)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-privileged
  namespace: streamspace
spec:
  containers:
  - name: test
    image: nginx
    securityContext:
      privileged: true
EOF
# Expected: Error creating pod (Pod Security Standards violation)

# Cleanup
kubectl delete pod test-privileged -n streamspace --ignore-not-found
```

### 3. Network Policies

```bash
# Verify network policies exist
kubectl get networkpolicies -n streamspace

# Test: Verify default deny policy
kubectl get networkpolicy default-deny-all -n streamspace -o yaml

# Test: Try unauthorized pod-to-pod communication
# Create test pods
kubectl run test-source -n streamspace --image=busybox --command -- sleep 3600
kubectl run test-dest -n streamspace --image=nginx

# Get destination pod IP
DEST_IP=$(kubectl get pod test-dest -n streamspace -o jsonpath='{.status.podIP}')

# Try to connect (should fail due to network policy)
kubectl exec test-source -n streamspace -- wget -T 5 -O- http://$DEST_IP
# Expected: Timeout or connection refused

# Cleanup
kubectl delete pod test-source test-dest -n streamspace
```

### 4. RBAC Testing

```bash
# Verify controller has minimal permissions
kubectl get role streamspace-controller -n streamspace -o yaml

# Test: Controller should NOT have cluster-wide permissions
kubectl get clusterrole | grep streamspace
# Expected: No cluster roles for streamspace

# Test: Create test service account with no permissions
kubectl create sa test-user -n streamspace
kubectl auth can-i create sessions --as=system:serviceaccount:streamspace:test-user -n streamspace
# Expected: no

# Cleanup
kubectl delete sa test-user -n streamspace
```

---

## Automated Security Scanning

StreamSpace uses GitHub Actions for automated security scanning. The workflow runs on:
- Every push to main
- Every pull request
- Daily at 2 AM UTC
- Manual trigger via workflow_dispatch

### Container Image Scanning (Trivy)

```bash
# Run Trivy locally
docker build -t streamspace-api:test ./api
trivy image --severity CRITICAL,HIGH streamspace-api:test

# Example output analysis:
# - CRITICAL: 0 (MUST be 0 for production)
# - HIGH: < 5 (acceptable for non-production)
# - MEDIUM: < 20 (monitor and prioritize)
```

### Go Dependency Scanning

```bash
# Run govulncheck
cd api
govulncheck ./...

# Run Nancy
go list -json -deps ./... | docker run --rm -i sonatypecommunity/nancy:latest sleuth
```

### npm Dependency Scanning

```bash
# Run npm audit
cd ui
npm audit --audit-level=moderate

# Fix vulnerabilities
npm audit fix

# Check for unfixable vulnerabilities
npm audit --audit-level=high
```

### Secret Scanning (Gitleaks)

```bash
# Run Gitleaks locally
gitleaks detect --source . --verbose

# Scan specific file
gitleaks detect --source . --no-git --verbose -f path/to/file

# Common secrets to watch for:
# - API keys: AWS, GitHub, etc.
# - Database credentials
# - JWT secrets
# - Private keys
# - OAuth client secrets
```

### SAST (Semgrep)

```bash
# Run Semgrep locally
semgrep --config=auto .

# Run with specific rulesets
semgrep --config=p/owasp-top-ten .
semgrep --config=p/kubernetes .
semgrep --config=p/golang .

# Generate SARIF report
semgrep --config=auto --sarif --output=semgrep-results.sarif .
```

### CodeQL Analysis

```bash
# CodeQL runs automatically in CI/CD
# To run locally, install CodeQL CLI:
# https://github.com/github/codeql-cli-binaries/releases

# Create database
codeql database create codeql-db --language=go

# Run analysis
codeql database analyze codeql-db \
  --format=sarif-latest \
  --output=codeql-results.sarif \
  -- codeql/go-queries:security-and-quality
```

### Kubernetes Manifest Scanning

```bash
# Run Kubesec
docker run --rm -v $(pwd):/manifests kubesec/kubesec:latest scan /manifests/config/session-pod.yaml

# Run Checkov
checkov -d manifests/ --framework kubernetes

# Run kube-bench (for cluster configuration)
kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml
kubectl logs -f job/kube-bench
```

---

## Manual Security Testing

### Authentication & Authorization

#### Test 1: JWT Token Validation
```bash
# Get valid token
TOKEN=$(curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}' \
  | jq -r '.token')

# Test: Valid token should work
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/sessions
# Expected: 200 OK with session list

# Test: No token should fail
curl http://localhost:8000/api/v1/sessions
# Expected: 401 Unauthorized

# Test: Invalid token should fail
curl -H "Authorization: Bearer invalid.token.here" \
  http://localhost:8000/api/v1/sessions
# Expected: 401 Unauthorized

# Test: Expired token should fail (wait for expiration or modify token)
# Expected: 401 Unauthorized with "token expired" message
```

#### Test 2: RBAC Enforcement
```bash
# Login as regular user
USER_TOKEN=$(curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"pass"}' \
  | jq -r '.token')

# Test: Regular user accessing cluster endpoints (should fail)
curl -H "Authorization: Bearer $USER_TOKEN" \
  http://localhost:8000/api/v1/cluster/nodes
# Expected: 403 Forbidden

# Login as admin
ADMIN_TOKEN=$(curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"adminpass"}' \
  | jq -r '.token')

# Test: Admin accessing cluster endpoints (should work)
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8000/api/v1/cluster/nodes
# Expected: 200 OK with node list
```

### CSRF Protection

#### Test 3: CSRF Token Validation
```bash
# Get CSRF token
CSRF_TOKEN=$(curl http://localhost:8000/api/v1/csrf-token | jq -r '.token')

# Test: POST without CSRF token (should fail)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8000/api/v1/sessions \
  -d '{"template":"firefox-browser"}'
# Expected: 403 Forbidden - CSRF validation failed

# Test: POST with CSRF token (should work)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  http://localhost:8000/api/v1/sessions \
  -d '{"template":"firefox-browser"}'
# Expected: 201 Created

# Test: GET requests should not require CSRF token
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/sessions
# Expected: 200 OK (no CSRF token needed for GET)
```

### Rate Limiting

#### Test 4: Rate Limit Enforcement
```bash
# Test general rate limit (100 req/sec)
for i in {1..150}; do
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8000/health &
done
wait
# Expected: First 100-200 requests succeed (200), remaining get 429 Too Many Requests

# Test auth rate limit (5 req/sec)
for i in {1..20}; do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -X POST http://localhost:8000/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"test","password":"test"}' &
done
wait
# Expected: First 5-10 requests complete, remaining get 429
```

### Input Validation

#### Test 5: SQL Injection Prevention
```bash
# Test SQL injection in username
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin OR 1=1--","password":"anything"}'
# Expected: 400 Bad Request or 401 Unauthorized (NOT 200 OK)

# Test SQL injection in query parameters
curl "http://localhost:8000/api/v1/sessions?user=admin'%20OR%20'1'='1" \
  -H "Authorization: Bearer $TOKEN"
# Expected: 400 Bad Request (input validation failure)
```

#### Test 6: XSS Prevention
```bash
# Test XSS in session name
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8000/api/v1/sessions \
  -d '{"template":"firefox","name":"<script>alert(1)</script>"}'
# Expected: Script tags should be sanitized/escaped in response

# Test XSS in template description
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8000/api/v1/templates \
  -d '{"name":"test","description":"<img src=x onerror=alert(1)>"}'
# Expected: HTML sanitized in response
```

#### Test 7: Path Traversal Prevention
```bash
# Test path traversal in file paths
curl "http://localhost:8000/api/v1/files?path=../../../etc/passwd" \
  -H "Authorization: Bearer $TOKEN"
# Expected: 400 Bad Request (path traversal detected)

# Test encoded path traversal
curl "http://localhost:8000/api/v1/files?path=%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd" \
  -H "Authorization: Bearer $TOKEN"
# Expected: 400 Bad Request
```

#### Test 8: Command Injection Prevention
```bash
# Test command injection in container image
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8000/api/v1/sessions \
  -d '{"template":"firefox","image":"nginx; rm -rf /"}'
# Expected: 400 Bad Request (invalid image format)
```

### Security Headers

#### Test 9: Security Headers Present
```bash
# Test security headers
curl -I http://localhost:8000/health

# Expected headers:
# Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
# X-Content-Type-Options: nosniff
# X-Frame-Options: DENY
# Content-Security-Policy: default-src 'self'; ...
# Referrer-Policy: strict-origin-when-cross-origin
# Permissions-Policy: geolocation=(), microphone=(), camera=()
```

### TLS/HTTPS

#### Test 10: TLS Configuration
```bash
# Test HTTPS redirect
curl -I http://streamspace.local
# Expected: 301/302 redirect to https://streamspace.local

# Test HSTS header
curl -I https://streamspace.local
# Expected: Strict-Transport-Security header present

# Test TLS version (should be TLS 1.2+)
openssl s_client -connect streamspace.local:443 -tls1_1
# Expected: Connection should fail (TLS 1.1 not supported)

openssl s_client -connect streamspace.local:443 -tls1_2
# Expected: Connection succeeds

# Test weak ciphers (should fail)
nmap --script ssl-enum-ciphers -p 443 streamspace.local
# Expected: No weak ciphers (RC4, DES, MD5, etc.)
```

### Resource Quotas

#### Test 11: Quota Enforcement
```bash
# Get user quota
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/quota
# Expected: JSON with limits and current usage

# Test: Exceed session count limit
# Create sessions until quota exceeded
for i in {1..10}; do
  curl -X POST -H "Authorization: Bearer $TOKEN" \
    -H "X-CSRF-Token: $CSRF_TOKEN" \
    -H "Content-Type: application/json" \
    http://localhost:8000/api/v1/sessions \
    -d "{\"template\":\"firefox\",\"name\":\"session-$i\"}"
done
# Expected: First N sessions succeed, then 403 Forbidden with "quota exceeded"

# Test: Exceed resource limits
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8000/api/v1/sessions \
  -d '{"template":"firefox","resources":{"cpu":"100000m","memory":"1000Gi"}}'
# Expected: 400 Bad Request - resource quota exceeded
```

---

## Penetration Testing

### OWASP Top 10 Testing

Refer to [OWASP Testing Guide](https://owasp.org/www-project-web-security-testing-guide/) for detailed methodologies.

#### A01:2021 - Broken Access Control
- Test horizontal privilege escalation (user accessing another user's sessions)
- Test vertical privilege escalation (user accessing admin endpoints)
- Test IDOR (Insecure Direct Object References)
- Test forced browsing to admin endpoints

#### A02:2021 - Cryptographic Failures
- Test password storage (should use bcrypt/argon2)
- Test token storage (should use secure hashing)
- Test TLS configuration (ciphers, versions)
- Test sensitive data in transit and at rest

#### A03:2021 - Injection
- Test SQL injection (all input fields)
- Test command injection (container images, file paths)
- Test LDAP injection (if using LDAP)
- Test XSS (all user-controlled inputs)

#### A04:2021 - Insecure Design
- Review architecture for security flaws
- Test for missing security controls
- Review threat model and attack surface

#### A05:2021 - Security Misconfiguration
- Test default credentials
- Test verbose error messages
- Test directory listing
- Test unnecessary services exposed

#### A06:2021 - Vulnerable Components
- Run dependency scanning (npm audit, govulncheck)
- Check for outdated container base images
- Review third-party library versions

#### A07:2021 - Authentication Failures
- Test brute force protection
- Test password complexity requirements
- Test session timeout
- Test concurrent session limits

#### A08:2021 - Software and Data Integrity
- Test webhook signature validation
- Test container image verification
- Test dependency integrity checks

#### A09:2021 - Security Logging Failures
- Verify audit logging is enabled
- Test log tampering prevention
- Verify sensitive data is not logged
- Test log aggregation and monitoring

#### A10:2021 - Server-Side Request Forgery
- Test SSRF in webhook URLs
- Test SSRF in repository URLs
- Test internal network access restrictions

### Tools for Penetration Testing

```bash
# OWASP ZAP (web application scanner)
docker run -t owasp/zap2docker-stable zap-baseline.py \
  -t http://streamspace.local

# Burp Suite (manual testing)
# Configure browser to proxy through Burp Suite
# Intercept and modify requests to test security controls

# Nikto (web server scanner)
nikto -h http://streamspace.local

# SQLMap (SQL injection testing)
sqlmap -u "http://streamspace.local/api/v1/sessions?user=test" \
  --cookie="token=$TOKEN"

# Nuclei (vulnerability scanner)
nuclei -u http://streamspace.local -t cves/ -t vulnerabilities/
```

---

## Compliance Testing

### CIS Kubernetes Benchmark

```bash
# Run kube-bench
kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml
kubectl logs -f job/kube-bench

# Review results and remediate failures
```

### PCI DSS (if handling payment data)

- 3.4: Encryption of cardholder data in transit and at rest
- 6.5: Secure coding practices (OWASP Top 10)
- 8.3: Multi-factor authentication for remote access
- 10.2: Audit trail for all system access

### GDPR (if handling EU personal data)

- Right to erasure (user data deletion)
- Data encryption in transit and at rest
- Audit logging of personal data access
- Data breach notification procedures

### SOC 2 Type II

- Access controls (RBAC, MFA)
- Change management (CI/CD, code review)
- Security monitoring (audit logs, alerts)
- Incident response procedures

---

## Security Test Cases

### Test Case Template

```
TC-SEC-001: JWT Token Expiration
Priority: High
Type: Functional Security

Steps:
1. Login and obtain JWT token
2. Wait for token expiration (default 24 hours)
3. Attempt to use expired token

Expected Result:
- API returns 401 Unauthorized
- Error message: "Token expired"
- User is redirected to login

Actual Result:
[To be filled during testing]

Status: [Pass/Fail]
Notes:
[Any additional observations]
```

### Critical Test Cases

1. **TC-SEC-001**: JWT token expiration and renewal
2. **TC-SEC-002**: RBAC enforcement for admin endpoints
3. **TC-SEC-003**: CSRF token validation on state-changing operations
4. **TC-SEC-004**: Rate limiting on authentication endpoints
5. **TC-SEC-005**: SQL injection in all input fields
6. **TC-SEC-006**: XSS in user-generated content
7. **TC-SEC-007**: Path traversal in file operations
8. **TC-SEC-008**: Command injection in container operations
9. **TC-SEC-009**: TLS/HTTPS enforcement
10. **TC-SEC-010**: Resource quota enforcement
11. **TC-SEC-011**: Pod Security Standards compliance
12. **TC-SEC-012**: Network policy isolation
13. **TC-SEC-013**: Webhook signature validation
14. **TC-SEC-014**: Audit logging completeness
15. **TC-SEC-015**: Secret management (no hardcoded secrets)

---

## Tools and Resources

### Open Source Security Tools

- **Trivy**: Container image vulnerability scanning
- **Gitleaks**: Secret detection in git repositories
- **Semgrep**: SAST (Static Application Security Testing)
- **Checkov**: Infrastructure-as-Code security scanning
- **OWASP ZAP**: Web application security scanner
- **Nuclei**: Vulnerability scanner
- **kube-bench**: CIS Kubernetes Benchmark testing

### Commercial Tools (Optional)

- **Snyk**: Dependency vulnerability scanning
- **Burp Suite Pro**: Advanced web application testing
- **Nessus**: Network vulnerability scanning
- **Qualys**: Cloud security posture management

### Learning Resources

- [OWASP Testing Guide](https://owasp.org/www-project-web-security-testing-guide/)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/security-best-practices/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

---

## Continuous Security Testing

### Integration with CI/CD

Security testing is automated in GitHub Actions (`.github/workflows/security-scan.yml`):

1. **On Every Commit**: Fast security checks
   - Linting (golangci-lint, ESLint)
   - Secret scanning (Gitleaks)
   - Dependency scanning (npm audit, govulncheck)

2. **On Pull Request**: Comprehensive scanning
   - All commit checks +
   - Container image scanning (Trivy)
   - SAST (Semgrep, CodeQL)
   - Kubernetes manifest scanning (Kubesec, Checkov)

3. **Daily Schedule**: Deep analysis
   - All PR checks +
   - Dependency review
   - License compliance
   - Security advisory checks

### Security Gates

Pull requests must pass all security checks before merging:

- ✅ No CRITICAL vulnerabilities
- ✅ No secrets detected
- ✅ No high-severity SAST findings
- ✅ All security tests pass
- ✅ Code review by security team (for sensitive changes)

---

## Reporting Security Issues

If you discover a security vulnerability:

1. **DO NOT** open a public GitHub issue
2. **DO** report via GitHub Security Advisories: https://github.com/JoshuaAFerguson/streamspace/security/advisories/new
3. **OR** email: security@streamspace.io
4. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

Expected response time:
- Acknowledgment: 48 hours
- Status update: 7 days
- Fix timeline: Based on severity

---

**For Questions**: Contact the security team at security@streamspace.io

**Last Updated**: 2025-11-14
