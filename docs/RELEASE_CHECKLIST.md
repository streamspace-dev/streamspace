# StreamSpace Release Checklist

**Document Version**: 1.0
**Last Updated**: 2025-11-26

---

This checklist ensures consistent, safe releases for StreamSpace. Complete all applicable sections before promoting to production.

## Pre-Release

### Code Quality

- [ ] All tests pass (unit, integration, E2E)
- [ ] Test coverage meets milestone targets
- [ ] No critical/high issues in security scans (dependencies, containers)
- [ ] Code review completed and approved
- [ ] All linked issues addressed

### Documentation

- [ ] CHANGELOG.md updated with user-facing changes
- [ ] Release notes drafted (for major releases)
- [ ] Runbooks updated if operational changes
- [ ] API documentation current (if endpoints changed)

### Database

- [ ] Migration scripts reviewed for safety
- [ ] Rollback/downgrade plan documented
- [ ] Migration tested in staging environment
- [ ] Backup taken before migration (production)

### Backup & DR Verification

- [ ] Database backup completed within last 24 hours
- [ ] Database backup integrity verified (pg_restore --list)
- [ ] Storage snapshots exist and are recent
- [ ] Secrets backup current
- [ ] Restore procedure validated within last quarter

```bash
# Quick backup validation
aws s3 ls s3://streamspace-backups/postgres/ | tail -1  # Check latest
kubectl get volumesnapshot -n streamspace               # Check snapshots
```

---

## Staging Deployment

### Deploy

- [ ] Deploy to staging environment
- [ ] Feature flags configured appropriately
- [ ] Database migrations applied successfully

### Smoke Tests

- [ ] Create session: `kubectl apply -f tests/manifests/test-session.yaml`
- [ ] Session reaches Running state within SLA (< 60s)
- [ ] VNC connection works (browser access)
- [ ] Hibernate and resume session
- [ ] Stop and terminate session
- [ ] Webhook delivery to test sink (if applicable)

### Observability

- [ ] No errors in API logs: `kubectl logs -n streamspace deploy/streamspace-api | grep -i error`
- [ ] No errors in Agent logs: `kubectl logs -n streamspace deploy/streamspace-k8s-agent | grep -i error`
- [ ] Metrics flowing to dashboards
- [ ] Audit events recorded for core actions

### Security

- [ ] CSP/HSTS/rate limiting enabled
- [ ] Authentication works (login/logout)
- [ ] Authorization enforced (test role-based access)
- [ ] No sensitive data in logs

---

## Canary/Production Deployment

### Pre-Deploy

- [ ] Notify stakeholders of deployment window
- [ ] Confirm rollback artifacts available (previous image tags)
- [ ] On-call engineer identified and available
- [ ] Database backup verified (< 1 hour old for production)

### Deploy Canary

- [ ] Deploy to canary (1 pod or 10% traffic)
- [ ] Monitor for 15-30 minutes:
  - [ ] Error rate stable or improved
  - [ ] Latency p99 within SLA
  - [ ] Session start time within SLA
  - [ ] Agent heartbeats healthy

```bash
# Monitor canary
kubectl logs -f -n streamspace deploy/streamspace-api --since=5m | grep -i error
```

### Production Rollout

- [ ] Scale canary to full deployment
- [ ] Verify all pods healthy: `kubectl get pods -n streamspace`
- [ ] Monitor dashboards for 30 minutes
- [ ] Spot check: create test session in production

### Multi-Tenancy Verification (if applicable)

- [ ] Verify organization scoping on sessions
- [ ] Check for cross-tenant data leakage (manual spot check)
- [ ] WebSocket auth enforcing org boundaries

---

## Post-Release

### Verification

- [ ] All pods running and healthy
- [ ] No increase in error rates
- [ ] User-reported issues triaged
- [ ] Monitoring shows normal patterns

### Documentation

- [ ] Update project board/status
- [ ] Close linked GitHub issues
- [ ] Tag release in git: `git tag v2.0.x && git push --tags`
- [ ] Publish release notes (GitHub Releases)

### Backup Verification (Post-Deploy)

- [ ] Trigger post-deploy backup if significant DB changes
- [ ] Verify backup completed successfully
- [ ] Update DR documentation if architecture changed

### Lessons Learned

- [ ] Capture any issues encountered
- [ ] Document workarounds applied
- [ ] Create follow-up issues for improvements
- [ ] Schedule retrospective (for major releases)

---

## Rollback Procedure

If issues are detected:

```bash
# 1. Rollback to previous version
kubectl set image deployment/streamspace-api \
  api=ghcr.io/streamspace/api:PREVIOUS_TAG -n streamspace

# 2. Wait for rollout
kubectl rollout status deployment/streamspace-api -n streamspace

# 3. Verify health
kubectl get pods -n streamspace
curl -s https://streamspace.example.com/health | jq .

# 4. If DB migration needs rollback
kubectl exec -n streamspace deploy/streamspace-api -- \
  ./migrate -dir=migrations -rollback 1
```

---

## Quarterly DR Drill Reminder

Every quarter, complete a DR drill:

- [ ] Database restore test (to isolated environment)
- [ ] Storage snapshot restore test
- [ ] Secrets restore test
- [ ] Document RTO achieved
- [ ] Update runbooks with lessons learned

See [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) for full DR procedures.

---

## Sign-Off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Release Engineer | | | |
| QA Lead | | | |
| Security (if applicable) | | | |
| On-Call Engineer | | | |
