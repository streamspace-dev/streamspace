# Security Hardening Guide

> **Status**: Implementation Complete
> **Version**: 1.1.0
> **Last Updated**: 2025-11-19

---

## Overview

This guide provides comprehensive security hardening recommendations for StreamSpace deployments. It covers authentication configuration, MFA setup, and security best practices.

## Table of Contents

- [Authentication Hardening](#authentication-hardening)
  - [SAML Configuration](#saml-configuration)
  - [OIDC Configuration](#oidc-configuration)
  - [Local Authentication](#local-authentication)
- [Multi-Factor Authentication](#multi-factor-authentication)
  - [TOTP Setup](#totp-setup)
  - [SMS Authentication](#sms-authentication)
  - [Email Authentication](#email-authentication)
- [Security Vulnerabilities & Fixes](#security-vulnerabilities--fixes)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Authentication Hardening

### SAML Configuration

StreamSpace supports SAML 2.0 authentication with multiple identity providers.

#### Supported Providers

- Okta
- Azure AD
- OneLogin
- Google Workspace
- PingIdentity
- ADFS

#### Basic SAML Setup

**1. Configure Identity Provider**

Create a new SAML application in your IdP with these settings:

| Setting | Value |
|---------|-------|
| ACS URL | `https://streamspace.example.com/api/v1/auth/saml/acs` |
| Entity ID | `https://streamspace.example.com/api/v1/auth/saml/metadata` |
| Name ID Format | `emailAddress` |

**2. Configure StreamSpace**

```yaml
# values.yaml
auth:
  saml:
    enabled: true
    idpMetadataUrl: "https://your-idp.com/metadata.xml"
    # OR
    idpMetadata: |
      <EntityDescriptor>...</EntityDescriptor>

    # Optional settings
    signRequests: true
    signatureAlgorithm: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"

    # Attribute mapping
    attributeMapping:
      email: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
      firstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
      lastName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
```

#### SAML Return URL Validation

> **SECURITY FIX REQUIRED**: The current SAML implementation has an open redirect vulnerability.

<!-- TODO: Update after Builder implements whitelist validation -->

**Issue**: Return URLs are not validated against a whitelist, allowing attackers to redirect users to malicious sites.

**Fix (Pending Implementation)**:

```yaml
# values.yaml
auth:
  saml:
    allowedReturnUrls:
      - "https://streamspace.example.com/*"
      - "https://app.streamspace.example.com/*"

    # Strict mode - only exact matches allowed
    strictReturnUrlValidation: true
```

**Validation Logic**:
```go
// Expected implementation
func validateReturnURL(returnURL string, allowedPatterns []string) error {
    parsedURL, err := url.Parse(returnURL)
    if err != nil {
        return ErrInvalidURL
    }

    for _, pattern := range allowedPatterns {
        if matchPattern(parsedURL, pattern) {
            return nil
        }
    }

    return ErrUnauthorizedRedirect
}
```

#### Provider-Specific Guides

##### Okta Setup

1. Create new SAML 2.0 application in Okta Admin Console
2. Set Single Sign-On URL: `https://streamspace.example.com/api/v1/auth/saml/acs`
3. Set Audience URI: `https://streamspace.example.com`
4. Configure attribute statements:
   - `email` -> `user.email`
   - `firstName` -> `user.firstName`
   - `lastName` -> `user.lastName`
5. Download IdP metadata XML
6. Configure StreamSpace with metadata

##### Azure AD Setup

1. Register new Enterprise Application in Azure Portal
2. Configure Single sign-on -> SAML
3. Set Reply URL and Identifier
4. Configure claims (email, name)
5. Download Federation Metadata XML

For detailed provider guides, see [SAML_GUIDE.md](SAML_GUIDE.md).

---

### OIDC Configuration

StreamSpace supports OpenID Connect for authentication.

```yaml
auth:
  oidc:
    enabled: true
    issuerUrl: "https://accounts.google.com"
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
    scopes:
      - openid
      - email
      - profile

    # Claim mapping
    claimMapping:
      email: "email"
      name: "name"
      groups: "groups"
```

---

### Local Authentication

For environments without SSO, StreamSpace provides local authentication.

**Password Requirements**:
```yaml
auth:
  local:
    enabled: true
    passwordPolicy:
      minLength: 12
      requireUppercase: true
      requireLowercase: true
      requireNumbers: true
      requireSpecial: true
      maxAge: 90  # days
      preventReuse: 5  # previous passwords
```

**Account Lockout**:
```yaml
auth:
  local:
    lockout:
      enabled: true
      maxAttempts: 5
      lockoutDuration: 15m
      resetAfter: 1h
```

---

## Multi-Factor Authentication

### TOTP Setup

Time-based One-Time Password (TOTP) is the recommended MFA method.

**Enable TOTP for User**:

1. Navigate to Settings -> Security -> Enable 2FA
2. Scan QR code with authenticator app (Google Authenticator, Authy, etc.)
3. Enter verification code
4. Save backup codes

**API Configuration**:
```yaml
auth:
  mfa:
    totp:
      enabled: true
      issuer: "StreamSpace"
      period: 30  # seconds
      digits: 6
      algorithm: "SHA1"
```

**Admin Enforcement**:
```yaml
auth:
  mfa:
    required: true  # Require MFA for all users
    requiredForRoles:
      - admin
      - operator
```

---

### SMS Authentication

> **STATUS**: Returns 501 Not Implemented
> **PENDING**: Builder implementation

<!-- TODO: Update after Builder implements SMS MFA -->

SMS-based MFA sends a verification code via text message.

**Configuration (Pending)**:
```yaml
auth:
  mfa:
    sms:
      enabled: true
      provider: "twilio"  # or "aws-sns"

      # Twilio configuration
      twilio:
        accountSid: "your-account-sid"
        authToken: "your-auth-token"
        fromNumber: "+1234567890"

      # Message template
      messageTemplate: "Your StreamSpace verification code is: {{code}}"
      codeExpiry: 5m
```

**Implementation Notes**:
- File: `/api/internal/handlers/security.go:283-315`
- Currently returns 501 Not Implemented
- Needs SMS provider integration (Twilio, AWS SNS)

---

### Email Authentication

> **STATUS**: Returns 501 Not Implemented
> **PENDING**: Builder implementation

<!-- TODO: Update after Builder implements Email MFA -->

Email-based MFA sends a verification code via email.

**Configuration (Pending)**:
```yaml
auth:
  mfa:
    email:
      enabled: true

      # Email template
      subject: "StreamSpace Verification Code"
      template: "mfa-verification"
      codeExpiry: 10m
```

**Implementation Notes**:
- File: `/api/internal/handlers/security.go:283-315`
- Currently returns 501 Not Implemented
- Needs email service integration

---

## Security Vulnerabilities & Fixes

### Phase 5.5 Security Fixes

The following security issues are being addressed in Phase 5.5:

#### 1. SAML Open Redirect (HIGH)

**Issue**: No whitelist validation for return URLs
**Impact**: Attackers can redirect users to malicious sites
**Status**: Pending fix
**Mitigation**: Validate return URLs against configured whitelist

#### 2. Demo Mode Security (MEDIUM)

**Issue**: Hardcoded authentication allows any username in demo mode
**Impact**: Security risk if enabled in production
**Status**: Pending fix
**Mitigation**: Guard with environment variable, disable in production

**Current Code** (`ui/src/pages/Login.tsx:103-123`):
```javascript
// VULNERABLE - Any username accepted
if (DEMO_MODE) {
  setAuthenticated(true);
  return;
}
```

**Fix (Expected)**:
```javascript
// Only allow demo mode if explicitly enabled
if (process.env.REACT_APP_DEMO_MODE === 'true' &&
    process.env.NODE_ENV !== 'production') {
  // Demo mode logic
}
```

#### 3. Webhook Secret Generation Panic (CRITICAL)

**Issue**: `panic()` instead of error handling
**Impact**: API crashes if random generation fails
**Status**: Pending fix
**Location**: `/api/internal/handlers/integrations.go:896`

---

## Best Practices

### 1. Authentication

- [ ] Use SSO (SAML/OIDC) instead of local authentication
- [ ] Enforce MFA for all users, especially admins
- [ ] Configure session timeouts appropriately
- [ ] Use HTTPS only (redirect HTTP to HTTPS)

### 2. Authorization

- [ ] Follow principle of least privilege
- [ ] Regularly audit user permissions
- [ ] Use role-based access control (RBAC)
- [ ] Log all authorization failures

### 3. Network Security

- [ ] Enable network policies to isolate sessions
- [ ] Use TLS 1.3 for all communications
- [ ] Configure ingress rate limiting
- [ ] Block unused ports

### 4. Secrets Management

- [ ] Rotate secrets regularly (see [SECURITY_IMPL_GUIDE.md](SECURITY_IMPL_GUIDE.md))
- [ ] Use external secrets management (Vault, AWS Secrets Manager)
- [ ] Never commit secrets to version control
- [ ] Audit secret access

### 5. Monitoring

- [ ] Enable audit logging
- [ ] Monitor failed authentication attempts
- [ ] Set up alerts for suspicious activity
- [ ] Review logs regularly

---

## Troubleshooting

### SAML Issues

#### "Invalid SAML Response"

1. Check clock sync between IdP and StreamSpace
2. Verify certificate hasn't expired
3. Check signature algorithm matches configuration

#### "User Not Found"

1. Verify attribute mapping is correct
2. Check if auto-provisioning is enabled
3. Verify email claim is present in SAML assertion

### MFA Issues

#### "TOTP Code Invalid"

1. Check time sync on user's device
2. Verify TOTP configuration (period, algorithm)
3. Try using backup code

#### "SMS Not Received"

1. Verify phone number format (E.164)
2. Check SMS provider credentials
3. Review provider logs for delivery issues

---

## Implementation Status

### Completed

- [x] TOTP MFA implementation
- [x] SAML basic integration
- [x] OIDC basic integration
- [x] Local authentication with password policy
- [x] Session management

### Pending (Phase 5.5)

- [ ] SAML return URL validation (security fix)
- [ ] SMS MFA implementation
- [ ] Email MFA implementation
- [ ] Demo mode security guard

---

## Related Documentation

- [SAML Guide](SAML_GUIDE.md) - Detailed SAML setup for each provider
- [MFA Setup Guide](guides/MFA_SETUP_GUIDE.md) - User guide for enabling MFA
- [Security Implementation Guide](SECURITY_IMPL_GUIDE.md) - Advanced security features

---

*This document will be updated as security fixes are implemented in Phase 5.5.*
