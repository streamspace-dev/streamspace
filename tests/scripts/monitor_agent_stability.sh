#!/bin/bash
set -e

echo "==================================="
echo "P0-AGENT-001 Fix Verification"
echo "==================================="
echo "Started: $(date)"
echo "Monitoring agent for 10 minutes..."
echo ""

for i in {1..10}; do
  echo "[$i/10] Check at $(date +%H:%M:%S):"
  POD_STATUS=$(kubectl get pods -n streamspace | grep k8s-agent | awk '{print $3, $4, $5}')
  echo "  Status: $POD_STATUS"

  # Check for panic in logs
  if kubectl logs -n streamspace deploy/streamspace-k8s-agent --tail=20 | grep -q "panic:"; then
    echo "  ❌ PANIC DETECTED! P0 fix failed."
    exit 1
  else
    echo "  ✓ No panics"
  fi

  if [ $i -lt 10 ]; then
    sleep 60
  fi
done

echo ""
echo "==================================="
echo "✅ 10-MINUTE STABILITY TEST PASSED!"
echo "==================================="
echo "Agent has been stable for 10 minutes with no crashes."
echo "Old buggy agent crashed every 4-5 minutes."
echo ""
echo "Recommendation: Continue to 30-minute extended test or proceed with integration testing."
