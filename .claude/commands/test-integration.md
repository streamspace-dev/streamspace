# Run Integration Tests

Run integration tests for v2.0-beta features.

!cd tests/integration && go test -v $ARGUMENTS

Focus areas:
- Multi-pod API deployment (Redis-backed AgentHub)
- Agent failover scenarios (K8s Agent leader election)
- VNC streaming E2E (Control Plane → Agent → Container)
- Cross-platform operations (K8s + Docker agents)
- Performance testing (session throughput, latency)

After tests complete:
1. Summarize results (pass/fail by scenario)
2. Report performance metrics
3. Document any issues found
4. Create detailed report in `.claude/reports/INTEGRATION_TEST_*.md` format

If tests fail:
- Analyze failure logs
- Check infrastructure (K8s cluster, Docker daemon, Redis, PostgreSQL)
- Verify network connectivity
- Suggest fixes or environment corrections
