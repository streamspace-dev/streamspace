# Test Plan: Security

**Test Plan ID**: TP-002
**Author**: Agent 3 (Validator)
**Created**: 2025-11-19
**Status**: Active
**Priority**: High

---

## Objective

Validate security controls in StreamSpace including authentication flows, authorization checks, input validation, and protection against common web vulnerabilities.

---

## Scope

### In Scope
- SAML return URL validation (open redirect prevention)
- CSRF protection mechanisms
- Demo mode security controls
- Session token validation
- Input validation for security-sensitive endpoints
- Rate limiting effectiveness

### Out of Scope
- Penetration testing (requires specialized tools)
- Third-party library vulnerabilities
- Infrastructure security

---

## Test Environment

### Prerequisites
- StreamSpace API running
- SAML IdP configured (or mock)
- Test accounts with various roles
- Security scanning tools available

### Test Data
- Valid and invalid return URLs
- CSRF tokens (valid and forged)
- Demo mode credentials
- Malicious input payloads

---

## Test Cases

### TC-SEC-001: SAML Return URL Validation

**Priority**: High (Security Vulnerability)
**Type**: Security
**Related Issue**: SAML Return URL - Open redirect vulnerability

**Preconditions**:
- SAML authentication configured
- Valid IdP endpoint

**Steps**:
1. Initiate SAML login with valid return URL
2. Complete authentication
3. Verify redirect to valid URL
4. Initiate SAML login with external domain return URL (e.g., `https://evil.com`)
5. Complete authentication
6. Verify redirect is blocked or goes to default

**Test URLs**:
```
# Valid (should work)
/api/v1/auth/saml/login?returnUrl=/dashboard
/api/v1/auth/saml/login?returnUrl=/sessions

# Invalid (should be blocked)
/api/v1/auth/saml/login?returnUrl=https://evil.com
/api/v1/auth/saml/login?returnUrl=//evil.com/path
/api/v1/auth/saml/login?returnUrl=javascript:alert(1)
/api/v1/auth/saml/login?returnUrl=data:text/html,<script>
```

**Expected Results**:
- Valid internal URLs: Redirect successful
- External domains: Redirect blocked, go to default page
- Malicious URLs: Blocked with error logged
- No open redirect possible

**Test File**: `tests/security/saml_redirect_test.go`

---

### TC-SEC-002: CSRF Token Validation

**Priority**: High
**Type**: Security

**Preconditions**:
- Authenticated session
- CSRF protection enabled

**Steps**:
1. Login and obtain CSRF token from cookie/header
2. Make state-changing request with valid token
3. Verify request succeeds
4. Make request with missing CSRF token
5. Make request with invalid/forged CSRF token
6. Make request with expired CSRF token

**Test Requests**:
```http
# Valid request
POST /api/v1/sessions
X-CSRF-Token: valid-token-here
Cookie: session=...; csrf=...

# Missing token (should fail)
POST /api/v1/sessions
Cookie: session=...

# Forged token (should fail)
POST /api/v1/sessions
X-CSRF-Token: forged-token
Cookie: session=...
```

**Expected Results**:
- Valid token: Request succeeds (200/201)
- Missing token: Request fails (403 Forbidden)
- Invalid token: Request fails (403 Forbidden)
- Expired token: Request fails (403 Forbidden)

**Test File**: `tests/security/csrf_validation_test.go`

---

### TC-SEC-003: CSRF Origin/Referer Check

**Priority**: Medium
**Type**: Security

**Preconditions**:
- API server running
- CSRF protection enabled

**Steps**:
1. Send request with matching Origin header
2. Send request with different Origin header
3. Send request with no Origin but valid Referer
4. Send request with no Origin and no Referer
5. Send request with mismatched Referer

**Expected Results**:
- Matching Origin: Request allowed
- Different Origin: Request blocked (403)
- Valid Referer fallback: Request allowed
- No Origin/Referer: Request blocked or allowed based on config
- Mismatched Referer: Request blocked

**Test File**: `tests/security/csrf_origin_test.go`

---

### TC-SEC-004: Demo Mode Security - Disabled by Default

**Priority**: High
**Type**: Security
**Related Issue**: Demo Mode - Hardcoded auth allows ANY username

**Preconditions**:
- Production environment (DEMO_MODE not set)

**Steps**:
1. Verify DEMO_MODE environment variable not set
2. Attempt login with demo credentials
3. Verify authentication fails
4. Attempt login with "demo" button if visible
5. Verify demo login not available

**Expected Results**:
- Demo mode inaccessible in production
- Demo credentials rejected
- No "demo login" option in UI
- Environment variable required to enable

**Test File**: `tests/security/demo_mode_test.go`

---

### TC-SEC-005: Demo Mode Security - When Enabled

**Priority**: Medium
**Type**: Security

**Preconditions**:
- Development environment with DEMO_MODE=true

**Steps**:
1. Set DEMO_MODE=true environment variable
2. Verify demo login available
3. Login with any username
4. Verify limited permissions
5. Verify cannot access admin functions
6. Verify cannot modify real data

**Expected Results**:
- Demo mode works when enabled
- Demo users have restricted permissions
- Cannot access admin panel
- Cannot modify persistent data
- Clear warning displayed in UI

**Test File**: `tests/security/demo_mode_enabled_test.go`

---

### TC-SEC-006: Session Token Security

**Priority**: High
**Type**: Security

**Preconditions**:
- API server running

**Steps**:
1. Login and receive session token
2. Verify token is httpOnly cookie
3. Verify token has Secure flag (HTTPS)
4. Verify token has SameSite attribute
5. Verify token expires appropriately
6. Logout and verify token invalidated
7. Attempt to use old token

**Expected Results**:
- Token in httpOnly cookie (not accessible to JS)
- Secure flag set (HTTPS only)
- SameSite=Lax or Strict
- Token expires per configuration
- Logout invalidates token server-side
- Old tokens rejected

**Test File**: `tests/security/session_token_test.go`

---

### TC-SEC-007: SQL Injection Prevention

**Priority**: High
**Type**: Security

**Preconditions**:
- API server running
- Database connected

**Steps**:
1. Send session creation with SQL injection in name
2. Send search query with SQL injection
3. Send filter parameter with SQL injection
4. Verify all queries use parameterized statements

**Test Payloads**:
```
name: "'; DROP TABLE sessions;--"
name: "' OR '1'='1"
search: "test' UNION SELECT * FROM users--"
filter: "1; DELETE FROM sessions"
```

**Expected Results**:
- All inputs properly escaped
- No SQL errors in response
- Database unchanged by malicious input
- Parameterized queries used throughout

**Test File**: `tests/security/sql_injection_test.go`

---

### TC-SEC-008: XSS Prevention

**Priority**: High
**Type**: Security

**Preconditions**:
- API server running

**Steps**:
1. Create session with XSS payload in name
2. Create session with XSS in description
3. Retrieve sessions via API
4. Verify output is properly encoded

**Test Payloads**:
```
name: "<script>alert('XSS')</script>"
name: "<img src=x onerror=alert('XSS')>"
description: "Test<svg onload=alert(1)>"
```

**Expected Results**:
- Input accepted or rejected based on validation
- Output properly encoded when rendered
- No script execution possible
- Content-Type headers correct

**Test File**: `tests/security/xss_prevention_test.go`

---

### TC-SEC-009: Rate Limiting

**Priority**: Medium
**Type**: Security

**Preconditions**:
- Rate limiting enabled

**Steps**:
1. Make 100 requests to login endpoint in 10 seconds
2. Verify rate limit triggered
3. Wait for rate limit window to reset
4. Verify requests allowed again
5. Test rate limit on API endpoints

**Expected Results**:
- Rate limit triggers after threshold
- Returns 429 Too Many Requests
- Retry-After header present
- Limits reset after window
- Different limits for different endpoints

**Test File**: `tests/security/rate_limiting_test.go`

---

### TC-SEC-010: Authorization Checks

**Priority**: High
**Type**: Security

**Preconditions**:
- Users with different roles (admin, user)

**Steps**:
1. Login as regular user
2. Attempt to access admin endpoints
3. Attempt to modify other user's sessions
4. Attempt to view other user's data
5. Login as admin
6. Verify admin can access admin endpoints

**Expected Results**:
- Users cannot access admin endpoints (403)
- Users cannot modify others' resources (403)
- Users cannot view others' private data (403)
- Admins have appropriate access
- All authorization logged

**Test File**: `tests/security/authorization_test.go`

---

### TC-SEC-011: Webhook Secret Handling

**Priority**: Critical
**Type**: Security
**Related Issue**: Webhook Secret Generation Panic

**Preconditions**:
- API server running

**Steps**:
1. Create webhook without providing secret
2. Verify secret is auto-generated
3. Verify secret meets complexity requirements
4. Verify no panic on generation failure
5. Test webhook with correct secret
6. Test webhook with incorrect secret

**Expected Results**:
- Secret generated successfully
- 32+ characters, cryptographically random
- No panic on error (graceful failure)
- Webhook validates secret correctly
- Invalid secrets rejected (401)

**Test File**: `tests/security/webhook_secret_test.go`

---

## Test Data Requirements

### SAML Configuration

```yaml
# tests/fixtures/security/saml-config.yaml
idp:
  entityId: https://idp.test.local
  ssoUrl: https://idp.test.local/sso
  certificate: |
    -----BEGIN CERTIFICATE-----
    ... test certificate ...
    -----END CERTIFICATE-----
sp:
  entityId: https://streamspace.test.local
  acsUrl: https://streamspace.test.local/api/v1/auth/saml/acs
  allowedReturnUrls:
    - /dashboard
    - /sessions
    - /settings
```

### Security Test Users

```sql
-- Admin user
INSERT INTO users (username, email, role)
VALUES ('admin-test', 'admin@test.local', 'admin');

-- Regular user
INSERT INTO users (username, email, role)
VALUES ('user-test', 'user@test.local', 'user');
```

---

## Success Criteria

### Must Pass (Security Critical)
- TC-SEC-001: SAML Return URL Validation
- TC-SEC-002: CSRF Token Validation
- TC-SEC-004: Demo Mode Disabled by Default
- TC-SEC-007: SQL Injection Prevention
- TC-SEC-010: Authorization Checks
- TC-SEC-011: Webhook Secret Handling

### Should Pass
- TC-SEC-003: CSRF Origin/Referer Check
- TC-SEC-006: Session Token Security
- TC-SEC-008: XSS Prevention
- TC-SEC-009: Rate Limiting

### Nice to Have
- TC-SEC-005: Demo Mode When Enabled

---

## Risks

1. **SAML Testing Complexity**: Requires mock IdP or test environment
2. **False Positives**: Security tests may flag intentional behaviors
3. **Environment Differences**: Some tests require specific configuration

---

## Dependencies

- Builder completes SAML Return URL Validation fix
- Builder completes Demo Mode Security fix
- Builder completes Webhook Secret Panic fix

---

## Schedule

| Phase | Timeline | Status |
|-------|----------|--------|
| Test plan creation | Week 1 | Complete |
| Test implementation | Week 2 | Pending (after Builder fixes) |
| Test execution | Week 3 | Pending |
| Security audit | Week 4 | Pending |
| Report generation | Week 4-5 | Pending |

---

## Reporting

Results will be reported in:
- `tests/reports/security-test-report.md`
- Updates to `MULTI_AGENT_PLAN.md` Agent Communication Log

Security issues will be reported with:
- CVE-style severity (Critical/High/Medium/Low)
- Attack vector description
- Proof of concept (sanitized)
- Remediation recommendations
