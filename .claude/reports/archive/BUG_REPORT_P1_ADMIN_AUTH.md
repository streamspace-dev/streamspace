# BUG REPORT: P1 - Admin Authentication Failure (Blocks Integration Testing)

**Date**: 2025-11-21
**Reporter**: Agent 3 (Validator)
**Severity**: P1 - HIGH (Blocks integration testing, but Control Plane operational)
**Status**: NEW - Requires investigation by Builder (Agent 2)
**Branch**: `claude/v2-validator`

---

## Executive Summary

The admin user credentials stored in the Kubernetes secret do not authenticate successfully against the API's `/api/v1/auth/login` endpoint. This blocks all integration testing that requires creating sessions via the REST API.

**Impact**: **Integration test scenarios 2-8 are blocked** - cannot create sessions via API to test the full Control Plane → Agent workflow.

---

## Bug Details

### Symptom

When attempting to login with the admin credentials from the Kubernetes secret, the API returns:

```json
{
  "error": "Invalid credentials"
}
```

### Steps to Reproduce

1. Get admin credentials from Kubernetes secret:
   ```bash
   USERNAME=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.username}' | base64 -d)
   PASSWORD=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.password}' | base64 -d)
   echo "Username: $USERNAME"
   echo "Password: $PASSWORD"
   ```
   **Result**:
   ```
   Username: admin
   Password: aYknE4dQMLA1dg3Dd0zNcpt7IiCw0X8z
   ```

2. Attempt to login via API:
   ```bash
   curl -s -X POST http://localhost:8000/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"aYknE4dQMLA1dg3Dd0zNcpt7IiCw0X8z"}'
   ```
   **Result**:
   ```json
   {
     "error": "Invalid credentials"
   }
   ```

3. Verify admin user exists in database:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT id, username, email, role, active FROM users WHERE username = 'admin';"
   ```
   **Result**:
   ```
   id    | username |          email          | role  | active
   ------+----------+-------------------------+-------+--------
   admin | admin    | admin@streamspace.local | admin | t
   (1 row)
   ```

**Observation**: Admin user exists, is active, has correct role, but password verification fails.

---

## Root Cause Analysis

The issue is likely one of the following:

### Hypothesis 1: Password Secret Mismatch

**Theory**: The password stored in the Kubernetes secret (`streamspace-admin-credentials`) does not match the password hash stored in the `users` table.

**Evidence**:
- The admin user was created (row exists in `users` table)
- The password in the Kubernetes secret appears to be a random 32-character alphanumeric string
- The API's `VerifyPassword` function (api/internal/auth/handlers.go:243) checks the password against the `password_hash` column

**Possible Cause**:
- The admin user creation script may have generated one password but stored a different one in the Kubernetes secret
- OR the admin user was created without a password initially, and the secret was generated later

**File to Investigate**: Helm chart post-install hooks or init container that creates the admin user

### Hypothesis 2: Password Hashing Algorithm Mismatch

**Theory**: The password hash in the database uses a different algorithm or configuration than what the API's `VerifyPassword` function expects.

**Evidence**:
- The API uses bcrypt for password hashing (standard Go `golang.org/x/crypto/bcrypt`)
- The `VerifyPassword` function should handle bcrypt hashes correctly

**Less Likely**: bcrypt is well-tested and standard

### Hypothesis 3: Admin User Created Without Password

**Theory**: The admin user might have been created without a password hash, expecting initialization via a different flow (e.g., first-time setup wizard).

**Evidence**:
- There's a `SetupHandler` in the API (api/cmd/main.go:314)
- Some systems require initial password setup via web UI

**Check**: Query the `password_hash` column:
```sql
SELECT username, password_hash IS NULL as no_password FROM users WHERE username = 'admin';
```

---

## Investigation Steps Required

### Step 1: Check Password Hash in Database

```bash
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT username, password_hash IS NULL as no_password, LENGTH(password_hash) as hash_length FROM users WHERE username = 'admin';"
```

**Expected**: If `no_password` is `t` (true), then the admin user has no password set.

### Step 2: Check Admin User Creation Code

**Files to Examine**:
- `chart/templates/hooks/create-admin-user.yaml` (if exists)
- `chart/templates/api-deployment.yaml` - init containers
- `api/cmd/main.go` - admin user creation logic
- Database initialization scripts

**What to Look For**:
- Where is the admin user created?
- Is the password from the Kubernetes secret used to create the user?
- Is there a mismatch between secret generation and user creation?

### Step 3: Check Secret Generation

**File**: `chart/templates/secrets.yaml`

**What to Look For**:
- How is the admin password generated?
- Is it the same password used when creating the admin user?

---

## Temporary Workarounds

### Workaround 1: Reset Admin Password Directly

If we can determine the correct password hashing mechanism, we could manually update the `password_hash` in the database:

```bash
# Generate a bcrypt hash of the password (requires Go or Python with bcrypt)
# Then update the database:
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "UPDATE users SET password_hash = '<bcrypt-hash-here>' WHERE username = 'admin';"
```

**Risk**: Requires knowing the exact bcrypt cost and salt configuration used by the API.

### Workaround 2: Create a New Test User

If admin user creation is broken, we could manually create a test user with a known password:

```bash
# Generate a bcrypt hash (example: password = "test123")
# Insert new user:
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "INSERT INTO users (id, username, email, password_hash, role, active, created_at, updated_at)
      VALUES ('test-user', 'testuser', 'test@streamspace.local', '<bcrypt-hash>', 'admin', true, NOW(), NOW());"
```

**Note**: This is a temporary workaround and doesn't fix the underlying admin user issue.

### Workaround 3: Bypass Authentication for Integration Testing

Modify the API to accept a test token or disable authentication for local testing. **NOT RECOMMENDED** for production.

---

## Impact Assessment

### Blocked Functionality

**ALL API-based integration test scenarios are blocked**:

1. ✅ **Agent Registration**: WORKS (does not require API authentication)
2. ❌ **Session Creation via API**: BLOCKED (requires authentication)
3. ❌ **VNC Connection**: BLOCKED (requires session to exist)
4. ❌ **VNC Streaming**: BLOCKED (requires VNC connection)
5. ❌ **Session Lifecycle**: BLOCKED (requires session)
6. ❌ **Agent Failover**: BLOCKED (requires session)
7. ❌ **Concurrent Sessions**: BLOCKED (requires sessions)
8. ❌ **Error Handling**: BLOCKED (requires sessions)

### Alternative Testing Approaches

Since API authentication is broken, we explored:

1. **Creating Session CRDs Directly via kubectl**:
   - ❌ **Does not work** in v2.0-beta architecture
   - In v2.0, there's no Kubernetes controller watching Session CRDs
   - Sessions MUST be created via the REST API
   - The API then sends WebSocket commands to agents to provision pods

2. **Direct Database Manipulation**:
   - Could potentially create session records in the database
   - But this wouldn't trigger the agent commands
   - Not a valid integration test

3. **Manual WebSocket Commands to Agent**:
   - Could manually craft WebSocket messages to the agent
   - But this bypasses the Control Plane logic
   - Not a valid integration test

**Conclusion**: There's no valid workaround. **Authentication must be fixed** to proceed with integration testing.

---

## Architectural Context: v2.0-beta Session Creation Flow

For context, here's how session creation works in v2.0-beta (discovered during investigation):

1. **User/API creates session via REST API**: `POST /api/v1/sessions`
   - Handler: `api/internal/api/handlers.go:376` (`CreateSession`)
   - Requires authentication (JWT token)

2. **API validates request and creates Session CRD**:
   - Uses Kubernetes API client to create Session CRD in cluster

3. **API sends WebSocket command to agent**:
   - Looks up which agent should handle the session (based on load balancing)
   - Sends command to agent via existing WebSocket connection

4. **Agent receives command and provisions pod**:
   - Agent creates Deployment/Pod in Kubernetes
   - Agent updates Session CRD with status (phase, podName, etc.)

5. **API polls Session CRD and returns session details to client**

**Key Insight**: In v2.0-beta, the Control Plane API is the ONLY way to create sessions. Directly creating Session CRDs via kubectl does NOT work because there's no controller watching them.

---

## Expected Behavior

1. Admin credentials in Kubernetes secret should successfully authenticate against the API
2. `POST /api/v1/auth/login` should return a JWT token:
   ```json
   {
     "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
     "expiresAt": "2025-11-21T18:00:00Z",
     "user": {
       "id": "admin",
       "username": "admin",
       "email": "admin@streamspace.local",
       "role": "admin",
       "active": true
     }
   }
   ```
3. JWT token can then be used to create sessions: `POST /api/v1/sessions` with `Authorization: Bearer <token>` header
4. Integration testing can proceed with automated session creation

---

## Fix Required (For Builder - Agent 2)

### Priority

**P1 - HIGH**: This is a **high-priority bug** blocking integration testing. However, it's P1 (not P0) because:
- The Control Plane is operational (API, UI, Database all working)
- K8s Agent is working (registration and heartbeats successful)
- The issue is specific to admin authentication, not a critical system failure

**P0 bugs** (like the K8s Agent crash) block ALL functionality. This bug blocks integration testing but the system is otherwise functional.

### Investigation Tasks

1. **Check password hash in database** (5 minutes)
2. **Trace admin user creation flow** (30-60 minutes):
   - Find where admin user is created (Helm hooks? Init container? API startup?)
   - Verify password from secret is used correctly
3. **Fix password mismatch** (15-30 minutes):
   - Ensure password in secret matches password_hash in database
   - May require updating admin user creation logic
4. **Test login** (5 minutes)
5. **Document fix** (10 minutes)

### Estimated Effort

- **Investigation**: 35-65 minutes
- **Fix**: 15-30 minutes
- **Testing**: 5-10 minutes
- **Total Time**: 55-105 minutes (roughly 1-2 hours)

---

## Testing After Fix

### Verify Admin Login Works

```bash
# 1. Get admin credentials
USERNAME=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.username}' | base64 -d)
PASSWORD=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.password}' | base64 -d)

# 2. Login via API
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.token')

echo "Token: $TOKEN"

# 3. Verify token is valid (not null or error)
if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
  echo "✅ Login successful!"
else
  echo "❌ Login failed!"
fi
```

### Verify Session Creation Works

```bash
# 4. List available templates
curl -s -X GET http://localhost:8000/api/v1/templates \
  -H "Authorization: Bearer $TOKEN" | jq '.templates[] | {name, displayName}' | head -5

# 5. Create a test session
SESSION_ID=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {
      "memory": "1Gi",
      "cpu": "500m"
    }
  }' | jq -r '.id')

echo "Session ID: $SESSION_ID"

# 6. Wait for pod to be provisioned
sleep 10

# 7. Check session status
kubectl get session $SESSION_ID -n streamspace -o jsonpath='{.status.phase}'

# Expected: "Running"

# 8. Check if pod was created
kubectl get pods -n streamspace | grep $SESSION_ID

# Expected: One pod with name containing session ID, status Running or Pending
```

---

## Success Criteria

After fix is applied, the following should be verified:

✅ **Admin Login Works**:
- `POST /api/v1/auth/login` returns 200 with valid JWT token
- Token is a valid JWT (can be decoded)
- Token contains correct user claims (username, role, etc.)

✅ **Authenticated Requests Work**:
- `GET /api/v1/templates` with Bearer token returns template list
- `POST /api/v1/sessions` with Bearer token creates session

✅ **Session Creation Triggers Agent**:
- Session CRD is created in Kubernetes
- Agent receives WebSocket command from Control Plane
- Agent provisions pod for session
- Session CRD status is updated with phase and pod name

✅ **Integration Testing Can Proceed**:
- Validator (Agent 3) can begin Test Scenario 2: Session Creation
- All subsequent test scenarios become unblocked

---

## Related Files

- **Auth Handler**: `api/internal/auth/handlers.go` (lines 236-285) - Login function
- **API Handler**: `api/internal/api/handlers.go` (lines 376+) - CreateSession function
- **Main**: `api/cmd/main.go` (lines 280-320) - Handler initialization
- **Helm Chart**: `chart/templates/secrets.yaml` - Secret generation
- **Database Schema**: Users table with `password_hash` column
- **Kubernetes Secret**: `streamspace-admin-credentials` in `streamspace` namespace

---

## Notes for Builder (Agent 2)

### Context from Integration Testing

During integration testing (Phase 10), we discovered:
1. ✅ K8s Agent successfully connects and registers with Control Plane
2. ✅ Heartbeats working (agent sends status every 30s)
3. ✅ WebSocket connection between agent and Control Plane is stable
4. ❌ **BLOCKED**: Cannot create sessions to test agent's pod provisioning because authentication is broken

**What We Need**:
- Admin login to work so we can get a JWT token
- JWT token to authenticate session creation requests
- Session creation via API so we can verify the full Control Plane → Agent workflow

### v2.0-beta Architecture Insights

During investigation, we confirmed that v2.0-beta has fundamentally different session management than v1.x:
- **v1.x**: Kubernetes controller watches Session CRDs and provisions pods
- **v2.0-beta**: Control Plane API sends WebSocket commands to agents to provision pods

This means:
- Creating Session CRDs via kubectl **does not work** in v2.0-beta
- Sessions **must** be created via REST API
- Authentication is **required** for all session operations

---

**Status**: REPORTED - Awaiting Builder (Agent 2) investigation and fix

**Next Steps**:
1. Builder investigates admin user creation flow
2. Builder fixes password mismatch between secret and database
3. Builder verifies admin login works
4. Validator resumes integration testing (Test Scenario 2: Session Creation)
