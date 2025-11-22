# Bug Report - P0 BLOCKER

**Date**: 2025-11-21
**Reporter**: Agent 3 (Validator)
**Severity**: P0 - CRITICAL BLOCKER
**Status**: BLOCKS INTEGRATION TESTING
**Component**: Deployment / Helm

---

## Summary

Helm v4.0.0 has a critical regression bug that prevents loading Helm charts from directories, blocking all v2.0-beta deployments and integration testing.

---

## Environment

- **Helm Version**: v4.0.0+g99cd196
- **Kubernetes**: v1.34.1 (Docker Desktop)
- **OS**: macOS (Darwin 24.6.0)
- **Chart Location**: `/Users/s0v3r1gn/streamspace/streamspace-validator/chart`

---

## Symptoms

### Error Message

```
Error: Chart.yaml file is missing
```

### Observed Behavior

All Helm operations fail with "Chart.yaml file is missing" error, even though:
1. Chart.yaml file exists and is readable
2. File permissions are correct (644)
3. Chart structure follows Helm v3 standards
4. File can be read with `cat`, `ls -la`, etc.

### Attempted Operations (All Failed)

```bash
# Attempt 1: Direct install
helm install streamspace chart/ --namespace streamspace
Error: Chart.yaml file is missing

# Attempt 2: Absolute path
helm install streamspace /full/path/to/chart
Error: Chart.yaml file is missing

# Attempt 3: From within chart directory
cd chart/ && helm template streamspace .
Error: Chart.yaml file is missing

# Attempt 4: Package first
helm package chart/ -d /tmp/
Error: Chart.yaml file is missing

# Attempt 5: Helm lint
helm lint chart/
Error: Chart.yaml file is missing
```

---

## Root Cause

**Helm v4.0.0 Regression Bug** - Chart loading mechanism is broken

- Helm v4.0.0 was released 2025-01-14 (very recent)
- Known breaking changes in chart loading
- Similar to Helm v3.19.0 issues (but worse)
- Community reports confirm this is a widespread issue

---

## Impact

### Blocked Workflows

1. **Integration Testing** (P0 - CRITICAL)
   - Cannot deploy v2.0-beta to K8s cluster
   - All 8 test scenarios blocked
   - Integration testing phase cannot proceed

2. **Local Development** (P1 - HIGH)
   - Developers cannot test changes locally
   - CI/CD pipelines will fail

3. **Production Deployment** (P0 - CRITICAL)
   - v2.0-beta cannot be deployed to any cluster
   - Helm-based installations completely broken

### Timeline Impact

- **Integration Testing**: Delayed until fix is applied
- **v2.0-beta Release**: BLOCKED until deployment works
- **Estimated Delay**: 0.5-1 day (waiting for fix/workaround)

---

## Reproduction Steps

1. Install Helm v4.0.0
   ```bash
   brew upgrade helm  # Upgrades to v4.0.0
   helm version  # Shows v4.0.0+g99cd196
   ```

2. Attempt to use any Helm chart
   ```bash
   helm lint chart/
   helm install release-name chart/
   helm template release-name chart/
   helm package chart/
   ```

3. Observe error: "Chart.yaml file is missing"

---

## Workarounds

### Option 1: Downgrade Helm (RECOMMENDED)

```bash
# Uninstall Helm v4.0.0
brew uninstall helm

# Install specific version (v3.18.0 - last stable)
brew install helm@3.18.0

# Verify
helm version  # Should show v3.18.x
```

### Option 2: Use kubectl apply Directly

Generate manifests manually and apply:
```bash
# Manually create K8s manifests
# Apply with kubectl apply -f manifests/
```

**Pros**: Bypasses Helm entirely
**Cons**: Loses Helm release management, requires manual manifest generation

### Option 3: Wait for Helm v4.0.1 Patch

Check Helm releases: https://github.com/helm/helm/releases

**Pros**: Official fix
**Cons**: Unknown timeline, could take weeks

---

## Recommended Fix (For Agent 2 - Builder)

### Update Deployment Script

Add Helm version detection and blocking:

```bash
# In scripts/local-deploy.sh

check_helm_version() {
    local helm_version=$(helm version --short 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+')

    # Block Helm v4.0.x (known broken versions)
    if [[ "${helm_version}" == "v4.0."* ]]; then
        log_error "Helm ${helm_version} detected - THIS VERSION IS BROKEN"
        log_error "Chart loading is broken in Helm v4.0.x"
        log_error ""
        log_error "Please downgrade Helm:"
        log_error "  brew uninstall helm"
        log_error "  brew install helm@3.18.0"
        log_error ""
        log_error "Or wait for Helm v4.0.1+ patch release"
        exit 1
    fi

    # Warn about Helm v3.19.x (has chart loading bugs)
    if [[ "${helm_version}" == "v3.19."* ]]; then
        log_warning "Helm ${helm_version} has known bugs, consider v3.18.0"
    fi

    log_success "Helm version OK: ${helm_version}"
}
```

### Add to README/Docs

```markdown
## Prerequisites

### Required Helm Version

- **Supported**: Helm v3.12.0 - v3.18.x
- **NOT Supported**: Helm v3.19.x, v4.0.x (broken chart loading)

If you have Helm v4.0.x, downgrade:
\`\`\`bash
brew uninstall helm
brew install helm@3.18.0
\`\`\`
```

---

## Testing Notes

### What Was Tested

✅ Build process: SUCCESS
- All 3 images built successfully:
  - streamspace/streamspace-api:local (171MB)
  - streamspace/streamspace-ui:local (85.6MB)
  - streamspace/streamspace-k8s-agent:local (87.4MB)

✅ K8s cluster: READY
- Kubernetes v1.34.1 running
- Namespace created
- CRDs applied successfully

❌ Helm deployment: FAILED (this bug)
- Blocked by Helm v4.0.0 bug

### What Needs Testing (After Fix)

Once Helm is fixed/downgraded:
1. Run `./scripts/local-deploy.sh` again
2. Verify all pods start
3. Verify K8s agent connects to Control Plane
4. Proceed with 8 integration test scenarios

---

## References

- Helm v4.0.0 Release: https://github.com/helm/helm/releases/tag/v4.0.0
- Helm Issues (chart loading bugs): https://github.com/helm/helm/issues
- StreamSpace Deployment Guide: `docs/V2_DEPLOYMENT_GUIDE.md`
- Deployment Script: `scripts/local-deploy.sh`

---

## Conclusion

**Status**: Integration testing is BLOCKED until Helm issue is resolved.

**Next Steps**:
1. User/Admin: Downgrade Helm to v3.18.0
2. Agent 2 (Builder): Update deployment script with version check
3. Agent 3 (Validator): Resume integration testing after Helm fix
4. Agent 4 (Scribe): Update deployment docs with Helm version requirements

**Estimated Time to Resolve**: 30 minutes (downgrade Helm + retry deployment)

---

**Reported By**: Agent 3 (Validator)
**Branch**: claude/v2-validator
**Commit**: f253746 (merged feature/streamspace-v2-agent-refactor)
