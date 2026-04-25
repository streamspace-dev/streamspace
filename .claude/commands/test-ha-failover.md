# Test HA Failover

Test High Availability failover scenarios.

## Test Multi-Pod API Failover

### Setup
!kubectl scale deployment/streamspace-api -n streamspace --replicas=3

Verify Redis enabled:
!kubectl get configmap -n streamspace streamspace-config -o yaml | grep redis

### Create Test Sessions
Create 5-10 active sessions distributed across API pods:
!for i in {1..5}; do curl -X POST http://localhost:8000/api/v1/sessions -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"512Mi","cpu":"250m"}}'; done

### Simulate API Pod Failure
!kubectl delete pod -n streamspace -l app.kubernetes.io/component=api | head -1

### Verify Failover
- Check session survival (all should still be running)
- Verify agent connections redistributed
- Test new session creation via different pod
- Confirm zero data loss

---

## Test K8s Agent Leader Election

### Setup
!kubectl scale deployment/streamspace-k8s-agent -n streamspace --replicas=3

Verify HA enabled:
!kubectl get deployment streamspace-k8s-agent -n streamspace -o yaml | grep ENABLE_HA

### Create Test Sessions
Create 5-10 sessions (leader will process):
!for i in {1..5}; do curl -X POST http://localhost:8000/api/v1/sessions ...; done

### Identify Current Leader
!kubectl logs -n streamspace -l app=streamspace-k8s-agent | grep "Elected as leader"

### Simulate Leader Failure
!kubectl delete pod -n streamspace [leader-pod-name]

### Measure Failover Time
Start timer, wait for:
- New leader election
- Command processing resumed
- Session creation working

Target: < 30 seconds

### Verify Zero Session Loss
- All sessions still running
- No pod restarts
- Database state consistent

---

## Test Docker Agent HA (if applicable)

Test file-based, Redis-based, or Swarm-based leader election depending on configuration.

---

## Report Results

Create report in `.claude/reports/INTEGRATION_TEST_HA_FAILOVER_YYYY-MM-DD.md` with:

### Test Results
- Setup configuration
- Number of replicas tested
- Number of sessions created
- Failover trigger method
- Failover time measured
- Session survival rate
- Any data loss detected

### Metrics
- Leader election time
- Session survival: X/Y (percentage)
- Command processing delay
- Recovery time

### Issues Found
- List any issues encountered
- Severity levels
- Suggested fixes

### Conclusion
- ✅ HA working as expected
- 🟡 Issues found (document)
- ❌ Critical failures (escalate)
