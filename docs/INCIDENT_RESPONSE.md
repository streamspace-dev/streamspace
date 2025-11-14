# Security Incident Response Plan

**Document Version**: 1.0
**Last Updated**: 2025-11-14
**Owner**: Security Team

---

## Purpose

This document outlines the procedures for responding to security incidents affecting the StreamSpace platform.

---

## Incident Classification

### Severity Levels

| Severity | Description | Response Time | Examples |
|----------|-------------|---------------|----------|
| **P0 - Critical** | Active breach, data exposed | Immediate (< 15 min) | Database compromised, active attacker |
| **P1 - High** | Potential breach, service degraded | < 1 hour | Suspicious admin access, DDoS attack |
| **P2 - Medium** | Security control bypassed | < 4 hours | Failed auth spike, rate limit exceeded |
| **P3 - Low** | Minor vulnerability, no active exploit | < 24 hours | Outdated dependency, config issue |

---

## Incident Response Phases

### 1. Detection and Analysis

**Indicators of Compromise (IoCs)**:
- Failed authentication spike (>100/min from single IP)
- Authorization failures for sensitive endpoints
- Unexpected privilege escalation attempts
- Unusual outbound network connections
- Falco security alerts
- Security scan failures in CI/CD
- Suspicious API access patterns

**Detection Sources**:
- Grafana security dashboard alerts
- Falco runtime security alerts
- Audit logs (structured JSON logs)
- Prometheus metrics anomalies
- GitHub security advisories
- Vulnerability scan reports

**Initial Response** (First 15 minutes):
1. **Acknowledge** the incident
   ```bash
   # Log incident start
   echo "[$(date)] INCIDENT STARTED: ${INCIDENT_ID}" >> /var/log/security/incidents.log
   ```

2. **Assess** severity using classification matrix

3. **Assemble** incident response team
   - Incident Commander
   - Security Engineer
   - Platform Engineer
   - Communications Lead

4. **Contain** if critical (P0)
   - Isolate affected systems
   - Block malicious IPs
   - Disable compromised accounts

### 2. Containment

#### Short-term Containment (Immediate)

**Block malicious IP**:
```bash
# Add NetworkPolicy to block IP
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: block-malicious-ip
  namespace: streamspace
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - ${MALICIOUS_IP}/32
EOF
```

**Disable compromised user account**:
```bash
# Revoke user access
kubectl exec -n streamspace deployment/streamspace-api -- \
  psql -U streamspace -c "UPDATE users SET active = false WHERE username = '${COMPROMISED_USER}';"
```

**Isolate affected pods**:
```bash
# Scale down compromised deployment
kubectl scale deployment/${COMPROMISED_DEPLOYMENT} -n streamspace --replicas=0

# Or delete specific pod
kubectl delete pod/${COMPROMISED_POD} -n streamspace
```

#### Long-term Containment

**Rotate compromised credentials**:
```bash
# Run secrets rotation script
./scripts/security/rotate-secrets.sh

# Force logout all users (rotate JWT secret)
kubectl create secret generic streamspace-api-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 32)" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deployment/streamspace-api -n streamspace
```

**Apply emergency patches**:
```bash
# Deploy security hotfix
kubectl set image deployment/streamspace-api \
  api=ghcr.io/streamspace/api:security-patch-${VERSION} -n streamspace
```

### 3. Eradication

**Remove threat**:
```bash
# Scan for malware
kubectl exec -n streamspace ${POD_NAME} -- clamscan -r /

# Remove malicious files
kubectl exec -n streamspace ${POD_NAME} -- rm -f /path/to/malicious/file

# Rebuild compromised containers
docker build --no-cache -t streamspace-api:clean ./api
```

**Patch vulnerabilities**:
```bash
# Update dependencies
cd api && go get -u ./...
cd ui && npm audit fix

# Rebuild and redeploy
./scripts/deploy.sh
```

**Close security gaps**:
- Apply missing security controls
- Update firewall rules
- Enhance monitoring
- Improve alerting

### 4. Recovery

**Restore services**:
```bash
# Verify system integrity
./scripts/verify-integrity.sh

# Restore from backup (if needed)
kubectl apply -f backups/streamspace-backup-${TIMESTAMP}.yaml

# Scale services back up
kubectl scale deployment/streamspace-api -n streamspace --replicas=3

# Verify health
kubectl get pods -n streamspace
curl https://streamspace.local/health
```

**Validate security controls**:
```bash
# Run security tests
./scripts/security-tests.sh

# Verify authentication works
curl -X POST https://streamspace.local/api/v1/auth/login \
  -d '{"username":"test","password":"test"}'

# Check rate limiting
for i in {1..200}; do curl https://streamspace.local/health; done

# Verify HTTPS enforcement
curl -I http://streamspace.local  # Should redirect to HTTPS
```

**Monitor for recurrence**:
- Watch Grafana security dashboard for 24-48 hours
- Review audit logs for suspicious activity
- Check Falco alerts for similar patterns

### 5. Post-Incident Review

**Conduct within 72 hours of resolution**.

**Post-Incident Report Template**:

```markdown
# Incident Report: ${INCIDENT_ID}

## Executive Summary
- **Incident**: [Brief description]
- **Severity**: [P0/P1/P2/P3]
- **Duration**: [Start time - End time]
- **Impact**: [Users affected, data exposed, downtime]

## Timeline
| Time | Event |
|------|-------|
| 10:00 | Initial detection via Falco alert |
| 10:05 | Incident declared, team assembled |
| 10:15 | Malicious IP blocked |
| 10:30 | Compromised accounts disabled |
| 11:00 | Root cause identified |
| 12:00 | Patch applied |
| 13:00 | Services restored |
| 14:00 | Monitoring confirmed no recurrence |

## Root Cause Analysis
**What happened**: [Detailed technical explanation]

**Why it happened**: [Contributing factors]

**Attack vector**: [How attacker gained access]

## Response Effectiveness
**What went well**:
- Detection within 5 minutes via Falco
- Rapid containment (15 minutes)
- Clear communication

**What needs improvement**:
- Runbook was outdated
- Backup restoration took longer than expected
- Need better alerting for this scenario

## Action Items
| Action | Owner | Due Date | Priority |
|--------|-------|----------|----------|
| Update firewall rules | SecEng | 2025-11-20 | P0 |
| Improve Falco rules | Platform | 2025-11-25 | P1 |
| Update incident runbook | SecTeam | 2025-11-22 | P1 |
| Conduct tabletop exercise | All | 2025-12-01 | P2 |

## Lessons Learned
1. Defense in depth worked - multiple controls detected the incident
2. Automation helped with rapid response
3. Need better backup/restore procedures
4. Team coordination was effective

## Recommendations
- Implement [specific security control]
- Enhance monitoring for [specific indicator]
- Update training on [specific procedure]
```

---

## Communication Plan

### Internal Communication

**Slack Channels**:
- `#security-incidents` - Real-time incident coordination
- `#engineering` - Technical updates
- `#leadership` - Executive briefings

**Status Updates**:
- Every 30 minutes during active incident (P0/P1)
- Every 2 hours for P2
- Daily for P3

**Template**:
```
🚨 INCIDENT UPDATE - ${INCIDENT_ID}

Severity: P${SEVERITY}
Status: [Investigating/Contained/Resolved]
Impact: [Description]
Next Update: [Time]

Actions Taken:
- [Action 1]
- [Action 2]

Next Steps:
- [Step 1]
- [Step 2]
```

### External Communication

**When to notify users**:
- Data breach (any personal information exposed)
- Service outage > 30 minutes
- Security vulnerability that requires user action

**Status Page** (https://status.streamspace.io):
```
We are investigating reports of [issue].
Last updated: [timestamp]
Impact: [Describe user impact]
```

**Security Advisory** (for vulnerabilities):
```markdown
# Security Advisory: ${CVE_ID}

**Severity**: [Critical/High/Medium/Low]
**Affected Versions**: [Version range]
**Fixed in**: [Version]

## Summary
[Brief description of vulnerability]

## Impact
[What attackers could do]

## Mitigation
[Steps users should take]

## Timeline
- [Date]: Vulnerability discovered
- [Date]: Patch released
- [Date]: Public disclosure

## Credit
[Researcher who discovered it]
```

---

## Incident Response Toolkit

### Essential Commands

**View audit logs**:
```bash
# Recent authentication failures
kubectl logs -n streamspace deployment/streamspace-api | grep "auth_failure"

# User activity for specific user
kubectl logs -n streamspace deployment/streamspace-api | grep "username:${USER}"

# Failed authorization attempts
kubectl logs -n streamspace deployment/streamspace-api | grep "authz_denied"
```

**Check security metrics**:
```bash
# Failed auth rate
kubectl exec -n streamspace deployment/prometheus -- \
  promtool query instant 'rate(streamspace_auth_failures_total[5m])'

# Active sessions
kubectl exec -n streamspace deployment/prometheus -- \
  promtool query instant 'streamspace_active_sessions'
```

**Network analysis**:
```bash
# View active connections
kubectl exec -n streamspace ${POD} -- netstat -tunap

# Check for suspicious DNS queries
kubectl exec -n streamspace ${POD} -- cat /etc/resolv.conf
kubectl logs -n kube-system -l k8s-app=kube-dns
```

**Forensics**:
```bash
# Capture pod state before termination
kubectl get pod ${POD} -n streamspace -o yaml > evidence/pod-${POD}.yaml
kubectl logs ${POD} -n streamspace > evidence/logs-${POD}.log
kubectl exec -n streamspace ${POD} -- ps aux > evidence/processes-${POD}.txt

# Create pod snapshot
kubectl debug ${POD} -n streamspace --image=busybox --copy-to=debug-${POD}
```

---

## Runbooks

### Runbook 1: Suspected Account Compromise

**Symptoms**: Unusual activity from user account, failed MFA, login from unusual location

**Steps**:
1. **Disable account immediately**
   ```bash
   psql -c "UPDATE users SET active=false WHERE username='${USER}';"
   ```

2. **Revoke all active sessions**
   ```bash
   psql -c "DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE username='${USER}');"
   ```

3. **Review audit logs**
   ```bash
   grep "username:${USER}" /var/log/streamspace/audit.log | tail -100
   ```

4. **Check for data exfiltration**
   ```bash
   grep "username:${USER}" /var/log/streamspace/audit.log | grep -E "(download|export)"
   ```

5. **Force password reset**
   ```bash
   psql -c "UPDATE users SET password_reset_required=true WHERE username='${USER}';"
   ```

6. **Notify user**
   - Send security alert email
   - Provide password reset instructions

### Runbook 2: DDoS Attack

**Symptoms**: High request rate, service degradation, rate limit alerts

**Steps**:
1. **Identify attack source**
   ```bash
   kubectl logs -n streamspace deployment/streamspace-api | \
     grep "rate_limit_exceeded" | \
     awk '{print $NF}' | sort | uniq -c | sort -nr | head -20
   ```

2. **Block attacking IPs**
   ```bash
   for ip in ${ATTACK_IPS}; do
     kubectl apply -f - <<EOF
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: block-$ip
   spec:
     podSelector: {}
     policyTypes: [Ingress]
     ingress:
     - from:
       - ipBlock:
           cidr: 0.0.0.0/0
           except: [$ip/32]
   EOF
   done
   ```

3. **Enable aggressive rate limiting**
   ```bash
   kubectl set env deployment/streamspace-api \
     RATE_LIMIT_REQUESTS_PER_SECOND=10 \
     RATE_LIMIT_BURST=20
   ```

4. **Scale up capacity** (if legitimate traffic)
   ```bash
   kubectl scale deployment/streamspace-api --replicas=10
   ```

5. **Enable Cloudflare DDoS protection** (if applicable)

---

## Contact Information

### Security Team
- **Security Lead**: security-lead@streamspace.io
- **On-Call Engineer**: oncall@streamspace.io
- **Security Hotline**: +1-xxx-xxx-xxxx (24/7)

### Escalation Path
1. On-Call Engineer → Security Lead (15 min)
2. Security Lead → Engineering Manager (30 min)
3. Engineering Manager → CTO (1 hour)
4. CTO → CEO (2 hours for critical incidents)

### External Resources
- **CERT**: cert@cert.org
- **FBI IC3**: https://www.ic3.gov
- **Cloud Provider Support**: support@cloudprovider.com

---

## Training and Exercises

### Tabletop Exercises (Quarterly)

**Scenario 1: Ransomware**
- Attacker encrypts session pods
- Ransom demand received
- Test backup/restore procedures

**Scenario 2: Insider Threat**
- Admin account misuse
- Data exfiltration attempt
- Test detection and response

**Scenario 3: Supply Chain Attack**
- Compromised dependency discovered
- Malicious code in production
- Test patch deployment process

### Incident Response Drills (Monthly)

- Practice containment procedures
- Test communication plan
- Verify runbook accuracy
- Time response activities

---

**Document Maintenance**: Review and update quarterly or after each major incident.

