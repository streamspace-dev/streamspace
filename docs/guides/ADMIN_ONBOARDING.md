# Admin User Onboarding Guide

Complete guide for configuring the initial admin user in StreamSpace.

**Last Updated**: 2025-11-16
**Version**: v1.0.0

---

## Table of Contents

- [Overview](#overview)
- [Multi-Layered Approach](#multi-layered-approach)
- [Method 1: Helm Chart (Production Recommended)](#method-1-helm-chart-production-recommended)
- [Method 2: Environment Variable (Manual Deployments)](#method-2-environment-variable-manual-deployments)
- [Method 3: Setup Wizard (First-Run & Recovery)](#method-3-setup-wizard-first-run--recovery)
- [Password Reset (Account Recovery)](#password-reset-account-recovery)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)

---

## Overview

StreamSpace provides **three** complementary methods for admin user onboarding, ensuring you can deploy in any environment - from production Kubernetes clusters to development docker-compose setups.

### The Admin User

- **Username**: `admin` (fixed)
- **Default Email**: `admin@streamspace.local` (configurable)
- **Role**: `admin` (full platform access)
- **Created**: Automatically during database migration

### Why Multiple Methods?

1. **Flexibility**: Works in Kubernetes, docker-compose, bare metal
2. **Security**: Production-grade secret management with Helm
3. **Recoverability**: Never get locked out of your admin account
4. **User-Friendly**: Setup wizard for non-technical users

---

## Multi-Layered Approach

StreamSpace uses a **cascading fallback system** to configure the admin password:

```
┌─────────────────────────────────────────────────────┐
│  Priority 1: Helm-Generated Kubernetes Secret      │
│  (Production deployments with Helm chart)           │
└────────────────┬────────────────────────────────────┘
                 │
                 ↓ If not available
┌─────────────────────────────────────────────────────┐
│  Priority 2: ADMIN_PASSWORD Environment Variable   │
│  (Manual deployments, docker-compose)               │
└────────────────┬────────────────────────────────────┘
                 │
                 ↓ If not set
┌─────────────────────────────────────────────────────┐
│  Priority 3: Setup Wizard at /api/v1/auth/setup   │
│  (First-run setup, account recovery)                │
└─────────────────────────────────────────────────────┘
```

### Decision Flow

The API backend checks these methods **in order** during startup:

1. **Check Kubernetes Secret**: If `streamspace-admin-credentials` exists → use it
2. **Check Environment Variable**: If `ADMIN_PASSWORD` is set → use it
3. **Check Database**: If admin has password → normal operation
4. **Enable Setup Wizard**: If none above → allow browser-based setup

---

## Method 1: Helm Chart (Production Recommended)

### How It Works

When you install StreamSpace via Helm, the chart:

1. Generates a secure **32-character random password**
2. Stores it in Kubernetes Secret: `streamspace-admin-credentials`
3. Injects it into the API deployment via environment variable
4. Displays retrieval command in Helm output

### Installation

```bash
# Install StreamSpace with Helm (auto-generates password)
helm install streamspace ./chart -n streamspace --create-namespace

# Helm will output credentials retrieval command
```

### Retrieve Auto-Generated Password

After installation, run the command from Helm output:

```bash
kubectl get secret streamspace-admin-credentials \
  -n streamspace \
  -o jsonpath='{.data.password}' | base64 -d && echo
```

**Example Output**:
```
Xy8k2P9mQw3nL5vR7tA1zB6cJ4hN8sG0
```

### Custom Password (Optional)

To provide your own password instead of auto-generation:

```yaml
# values.yaml
auth:
  admin:
    password: "YourSecurePassword123!"
    email: "admin@yourcompany.com"
```

```bash
helm install streamspace ./chart -n streamspace \
  --values values.yaml
```

### Use Existing Secret

If you have your own secret management:

```bash
# Create your secret first
kubectl create secret generic my-admin-secret \
  -n streamspace \
  --from-literal=username='admin' \
  --from-literal=password='YourSecurePassword123!' \
  --from-literal=email='admin@yourcompany.com'
```

```yaml
# values.yaml
auth:
  admin:
    existingSecret: "my-admin-secret"
```

### Security Features

✅ **Random 32-character password** (high entropy)
✅ **Stored in Kubernetes Secret** (encrypted at rest if configured)
✅ **helm.sh/resource-policy: keep** (survives uninstall)
✅ **Injected at runtime** (not in plain text in manifests)
✅ **One-time retrieval command** (displayed once in Helm output)

---

## Method 2: Environment Variable (Manual Deployments)

### Use Case

- Docker Compose deployments
- Manual Kubernetes deployments (without Helm)
- Development environments
- CI/CD pipelines

### How It Works

Set the `ADMIN_PASSWORD` environment variable:

- API backend reads it at startup
- Hashes with bcrypt (cost 10)
- Stores in database `users.password_hash`
- Clears from memory after hashing

### Docker Compose

```yaml
# docker-compose.yml
services:
  api:
    image: streamspace/streamspace-api:latest
    environment:
      - DB_HOST=postgres
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}  # <-- Admin password
    env_file:
      - .env  # Or read from .env file
```

```bash
# .env file
ADMIN_PASSWORD=YourSecurePassword123!
POSTGRES_PASSWORD=postgres_secure_password
```

```bash
# Start services
docker-compose up -d

# Check API logs for confirmation
docker-compose logs api | grep -i admin
# Should see: "✓ Admin password configured successfully from environment variable"
```

### Kubernetes (Manual Deployment)

```bash
# Create secret with admin password
kubectl create secret generic streamspace-admin-pass \
  -n streamspace \
  --from-literal=password='YourSecurePassword123!'

# Reference in deployment
kubectl set env deployment/streamspace-api \
  -n streamspace \
  --from=secret/streamspace-admin-pass \
  ADMIN_PASSWORD=password

# Or edit deployment YAML directly
kubectl edit deployment streamspace-api -n streamspace
```

```yaml
# Add to env section
env:
  - name: ADMIN_PASSWORD
    valueFrom:
      secretKeyRef:
        name: streamspace-admin-pass
        key: password
```

### Bare Metal / Systemd

```bash
# /etc/streamspace/api.env
ADMIN_PASSWORD=YourSecurePassword123!

# systemd service file
[Service]
EnvironmentFile=/etc/streamspace/api.env
ExecStart=/usr/local/bin/streamspace-api
```

### Security Features

✅ **Password strength validation** (minimum 8 characters)
✅ **Bcrypt hashing** (industry standard)
✅ **One-time read** (only on first startup when password is NULL)
✅ **Environment variable isolation** (not visible in `ps` on modern systems)
⚠️ **Manual secret management required** (use external secret stores in production)

---

## Method 3: Setup Wizard (First-Run & Recovery)

### Use Case

- First deployment where no password was configured
- Admin account lockout recovery
- Non-technical users who prefer web UI
- Air-gapped environments without Helm

### How It Works

If the admin user has **no password set**:

1. API backend logs warning message on startup
2. Setup wizard is **automatically enabled**
3. Visit `/setup` in your browser
4. Complete the form to set password
5. Setup wizard **auto-disables** after password is set

### Access the Setup Wizard

**If Ingress is Enabled**:
```
https://streamspace.yourdomain.com/setup
```

**If Port-Forwarding**:
```bash
kubectl port-forward -n streamspace svc/streamspace-ui 8080:80

# Visit: http://localhost:8080/setup
```

### Setup Wizard Form

Fill out the following fields:

- **Password**: Minimum 12 characters (NIST recommendation for admin)
- **Confirm Password**: Must match password
- **Email**: Valid email address for admin notifications

Click **Configure Admin Account** to complete setup.

### API Endpoints

#### Check Setup Status

```bash
GET /api/v1/auth/setup/status
```

**Response**:
```json
{
  "setupRequired": true,
  "adminExists": true,
  "hasPassword": false,
  "message": "Setup wizard is available - admin account needs password configuration"
}
```

#### Complete Setup

```bash
POST /api/v1/auth/setup
Content-Type: application/json

{
  "password": "SecurePassword123!Example",
  "passwordConfirm": "SecurePassword123!Example",
  "email": "admin@yourcompany.com"
}
```

**Success Response** (200 OK):
```json
{
  "message": "Admin account configured successfully - setup wizard is now disabled",
  "username": "admin",
  "email": "admin@yourcompany.com"
}
```

**Error Responses**:

- **403 Forbidden**: Setup wizard is disabled (admin already configured)
- **400 Bad Request**: Password mismatch or validation error
- **409 Conflict**: Another request completed setup first (race condition)

### Security Features

✅ **Only enabled when admin has no password** (can't override existing password)
✅ **Password strength validation** (12+ characters for admin accounts)
✅ **Password confirmation** (prevents typos)
✅ **Email validation** (RFC 5322 compliance)
✅ **Atomic database transaction** (prevents race conditions)
✅ **Single-use wizard** (auto-disables after success)
✅ **Input sanitization** (prevents SQL injection)

---

## Password Reset (Account Recovery)

### When to Use

- Admin account is locked out
- Password forgotten or lost
- Security breach requiring password rotation
- Helm secret lost or deleted

### How It Works

Use the `ADMIN_PASSWORD_RESET` environment variable:

1. Set the environment variable with new password
2. Restart API pods
3. Password is reset automatically on startup
4. Remove the environment variable
5. Restart API pods again (cleanup)

### Kubernetes (Helm Deployment)

```bash
# Step 1: Set password reset variable
kubectl set env deployment/streamspace-api \
  -n streamspace \
  ADMIN_PASSWORD_RESET='NewSecurePassword123!'

# Step 2: Restart is automatic, but you can force it
kubectl rollout restart deployment/streamspace-api -n streamspace

# Step 3: Check logs for confirmation
kubectl logs -n streamspace deploy/streamspace-api | grep -i reset
# Should see: "✓ Admin password RESET successfully!"

# Step 4: Remove the reset variable (IMPORTANT!)
kubectl set env deployment/streamspace-api \
  -n streamspace \
  ADMIN_PASSWORD_RESET-

# Step 5: Restart again to clear the variable
kubectl rollout restart deployment/streamspace-api -n streamspace
```

### Docker Compose

```bash
# Step 1: Add to docker-compose.yml
services:
  api:
    environment:
      - ADMIN_PASSWORD_RESET=NewSecurePassword123!

# Step 2: Restart service
docker-compose restart api

# Step 3: Check logs
docker-compose logs api | grep -i reset

# Step 4: Remove variable from docker-compose.yml

# Step 5: Restart again
docker-compose restart api
```

### Security Warning

⚠️ **Remove `ADMIN_PASSWORD_RESET` after use!**

- Leaving it set will reset password on every restart
- Could be exploited if environment is compromised
- API logs prominently warn about this

---

## Security Best Practices

### Production Deployments

1. **Use Helm Chart** (Method 1) for automated secret management
2. **Enable Kubernetes Secrets Encryption** at rest
3. **Use External Secret Management** (Vault, AWS Secrets Manager, etc.)
4. **Rotate Admin Password** regularly (every 90 days recommended)
5. **Enable MFA** for admin account after first login
6. **Restrict Admin Access** to specific IP ranges (IP whitelisting)
7. **Monitor Audit Logs** for admin account activity

### Password Requirements

For **admin accounts**, enforce stronger requirements:

- **Minimum 12 characters** (NIST 800-63B recommendation)
- **Mix of character types** (uppercase, lowercase, numbers, symbols)
- **No common passwords** (validated by setup wizard)
- **No password reuse** (compare against password history)

### Secret Management

**Good Practices**:
- ✅ Store passwords in Kubernetes Secrets
- ✅ Use `helm.sh/resource-policy: keep` for admin credentials
- ✅ Mount secrets as environment variables (not files in pod filesystem)
- ✅ Use RBAC to restrict secret access
- ✅ Enable secret encryption at rest in etcd

**Bad Practices**:
- ❌ Hardcoding passwords in Helm values.yaml
- ❌ Committing passwords to Git
- ❌ Sharing admin password via email/Slack
- ❌ Using weak passwords for convenience
- ❌ Leaving `ADMIN_PASSWORD_RESET` set permanently

---

## Troubleshooting

### Issue: "Setup wizard is not available"

**Symptoms**:
- Accessing `/setup` shows 403 Forbidden
- API logs: "Setup wizard is disabled - admin account is already configured"

**Cause**: Admin user already has a password set

**Solution**:
1. Use the login page: `/login`
2. If password is forgotten, use [Password Reset](#password-reset-account-recovery)
3. If Helm-deployed, [retrieve the auto-generated password](#retrieve-auto-generated-password)

---

### Issue: "Admin user not created yet"

**Symptoms**:
- Setup wizard returns: "Setup wizard is not available - admin user not created yet"
- API logs: "Admin user doesn't exist yet"

**Cause**: Database migration failed or didn't run

**Solution**:

```bash
# Check database migration logs
kubectl logs -n streamspace deploy/streamspace-api | grep -i migration

# Check if admin user exists
kubectl exec -n streamspace deploy/streamspace-postgres -- \
  psql -U streamspace -d streamspace -c "SELECT id, username, email, role FROM users WHERE id='admin';"

# If empty, manually create admin user (emergency only)
kubectl exec -n streamspace deploy/streamspace-postgres -- \
  psql -U streamspace -d streamspace -c \
  "INSERT INTO users (id, username, email, full_name, role, provider, active)
   VALUES ('admin', 'admin', 'admin@streamspace.local', 'Administrator', 'admin', 'local', true)
   ON CONFLICT (id) DO NOTHING;"
```

---

### Issue: Password reset not working

**Symptoms**:
- Set `ADMIN_PASSWORD_RESET` but password didn't change
- Can still log in with old password

**Cause**: Environment variable not read by API

**Solution**:

```bash
# Verify variable is set in deployment
kubectl get deployment streamspace-api -n streamspace -o jsonpath='{.spec.template.spec.containers[0].env[*].name}' | grep ADMIN_PASSWORD_RESET

# Check API pod has the variable
kubectl exec -n streamspace deploy/streamspace-api -- env | grep ADMIN_PASSWORD_RESET

# If missing, ensure you used `kubectl set env` correctly
kubectl set env deployment/streamspace-api \
  -n streamspace \
  ADMIN_PASSWORD_RESET='NewPassword' \
  --overwrite

# Force pod restart to pick up changes
kubectl delete pod -n streamspace -l app.kubernetes.io/component=api
```

---

### Issue: Helm secret not being used

**Symptoms**:
- Helm installed successfully
- But `ADMIN_PASSWORD` environment variable not set in API deployment

**Cause**: Secret not mounted correctly in deployment

**Solution**:

```bash
# Check if secret exists
kubectl get secret streamspace-admin-credentials -n streamspace

# If missing, Helm hook may have failed
helm get manifest streamspace -n streamspace | grep -A 10 "kind: Secret"

# Check API deployment has the env var
kubectl get deployment streamspace-api -n streamspace -o yaml | grep -A 5 ADMIN_PASSWORD

# If missing, upgrade Helm release to fix
helm upgrade streamspace ./chart -n streamspace
```

---

### Issue: "Password must be at least 12 characters"

**Symptoms**:
- Setup wizard rejects password
- Error: "Password must be at least 12 characters long"

**Cause**: Admin accounts enforce stricter password policy

**Solution**:
- Use a password with **12 or more characters**
- Consider using a **passphrase** (multiple words): `correct-horse-battery-staple-2025`
- Use a **password manager** to generate strong passwords

---

## Summary

StreamSpace provides **defense in depth** for admin onboarding:

| Method | Best For | Pros | Cons |
|--------|----------|------|------|
| **Helm Chart** | Production Kubernetes | Auto-generated, encrypted, managed | Requires Helm |
| **Environment Variable** | Docker Compose, CI/CD | Simple, flexible | Manual secret management |
| **Setup Wizard** | First-run, recovery | User-friendly, no CLI needed | Must secure `/setup` endpoint |

**Recommendation for Production**: Use Method 1 (Helm Chart) with external secret management (Vault, AWS Secrets Manager) for maximum security.

---

## Next Steps

After configuring the admin password:

1. **Log in to the Web UI** with username `admin` and your password
2. **Change the admin password** via User Settings (recommended)
3. **Enable MFA** for the admin account (Settings → Security → MFA)
4. **Create additional users** or configure SSO (SAML/OIDC)
5. **Set up IP whitelisting** to restrict admin access
6. **Review audit logs** regularly for security monitoring

For more information:
- [User Management Guide](USER_GROUP_MANAGEMENT.md)
- [Security Configuration](SECURITY.md)
- [SAML Setup Guide](SAML_SETUP.md)

---

**Version**: v1.0.0
**Last Updated**: 2025-11-16
**Maintained By**: StreamSpace Project
