# Deployment Status - P0 Bug Fix

**Date**: 2025-11-21 20:48
**Branch**: claude/v2-validator
**Status**: READY FOR IMAGE LOADING

---

## Summary

Builder's P0 bug fix (commit 8a36616) has been:
- ✅ **Reviewed**: SQL query correctly calculates active_sessions with LEFT JOIN subquery
- ✅ **Merged**: Integrated into claude/v2-validator branch
- ✅ **Built**: All 3 images built successfully with P0 fix
- ⏳ **Pending**: Images need to be loaded into k3s (requires sudo)

---

## Current System State

### Deployed Version
Currently running **WITHOUT** P0 fix (still has active_sessions bug):
- API pods: `streamspace-api-5bd97c787c-*` (CSRF fix only)
- Agent pods: `streamspace-k8s-agent-75fb565575-*` (old version)
- UI pods: `streamspace-ui-55f9bc7848-*` (old version)

### Built Images (Ready to Deploy)
New images with P0 fix built and ready:
- `streamspace/streamspace-api:local` - 168MB (includes P0 fix)
- `streamspace/streamspace-ui:local` - 85.6MB
- `streamspace/streamspace-k8s-agent:local` - 87.5MB

### Why Deployment Failed
Helm upgrade attempted to deploy the new images, but k3s couldn't pull them because:
1. Images are local (not in a registry)
2. k3s needs images imported into its containerd image store
3. Import requires `sudo k3s ctr images import` which I can't execute

---

## What Needs to Happen Next

### Step 1: Load Images into k3s (User Action Required)

**Run this command** to load the new images:

```bash
/tmp/load_images_to_k3s.sh
```

This script will:
- Export each Docker image to a tar stream
- Import into k3s containerd with `sudo k3s ctr images import`
- Verify all 3 images loaded successfully

**Expected output**:
```
════════════════════════════════════════════════════════════
  Loading Local Docker Images into k3s
════════════════════════════════════════════════════════════

→ Loading streamspace/streamspace-api:local...
✓ Successfully loaded streamspace/streamspace-api:local

→ Loading streamspace/streamspace-ui:local...
✓ Successfully loaded streamspace/streamspace-ui:local

→ Loading streamspace/streamspace-k8s-agent:local...
✓ Successfully loaded streamspace/streamspace-k8s-agent:local

════════════════════════════════════════════════════════════
✓ All images loaded into k3s successfully!
════════════════════════════════════════════════════════════
```

### Step 2: Deploy with Helm (Automated After Step 1)

Once images are loaded, run:

```bash
cd /Users/s0v3r1gn/streamspace/streamspace-validator
./scripts/local-deploy.sh
```

This will:
- Upgrade the Helm release with new images
- Trigger rolling update of all deployments
- Wait for pods to become ready

**Expected result**: All pods restart with new images containing P0 fix.

### Step 3: Test Session Creation (Validator)

After deployment completes, test session creation:

```bash
# Get fresh JWT token
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

# Create session
curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq .
```

**Expected result**: HTTP 202 Accepted with session details (not "No agents available" error).

---

## Builder's P0 Fix Details

### Commit: 8a36616
**Title**: `fix(api): resolve P0 bug - calculate active_sessions with subquery`

### Changes
**File**: `api/internal/api/handlers.go` (lines 687-702)

**Before (Broken)**:
```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT agent_id FROM agents
    WHERE status = 'online' AND platform = $1
    ORDER BY active_sessions ASC    -- ❌ Column doesn't exist!
    LIMIT 1
`, h.platform).Scan(&agentID)
```

**After (Fixed)**:
```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT a.agent_id
    FROM agents a
    LEFT JOIN (
        SELECT agent_id, COUNT(*) as active_sessions
        FROM sessions
        WHERE status IN ('running', 'starting')
        GROUP BY agent_id
    ) s ON a.agent_id = s.agent_id
    WHERE a.status = 'online' AND a.platform = $1
    ORDER BY COALESCE(s.active_sessions, 0) ASC
    LIMIT 1
`, h.platform).Scan(&agentID)
```

**Why This Works**:
- LEFT JOIN includes agents with 0 sessions
- Subquery dynamically counts active sessions
- COALESCE converts NULL to 0 for proper sorting
- No schema changes required
- Provides accurate load balancing

---

## Rollback Status

The failed deployment was rolled back to the stable version:
- ✅ API rolled back successfully
- ✅ Agent rolled back successfully
- ✅ UI rolled back successfully
- ✅ All failed pods cleaned up
- ✅ System stable and running

**Current pod count**:
```
NAME                                     READY   STATUS    RESTARTS
streamspace-api-5bd97c787c-chd82         1/1     Running   0
streamspace-api-5bd97c787c-sfqtp         1/1     Running   0
streamspace-k8s-agent-75fb565575-pwqrv   1/1     Running   4
streamspace-postgres-0                   1/1     Running   1
streamspace-ui-55f9bc7848-4m8s4          1/1     Running   0
streamspace-ui-55f9bc7848-v4t6m          1/1     Running   0
```

---

## Next Steps Summary

1. **User**: Run `/tmp/load_images_to_k3s.sh` (requires sudo)
2. **User or Validator**: Run `./scripts/local-deploy.sh`
3. **Validator**: Test session creation end-to-end
4. **Validator**: Update V2_BETA_VALIDATION_SUMMARY.md with results

---

**Validator**: Claude Code
**Date**: 2025-11-21 20:48
**Branch**: `claude/v2-validator`
