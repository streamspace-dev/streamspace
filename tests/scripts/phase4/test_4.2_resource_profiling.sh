#!/bin/bash
# Test 4.2: Resource Usage Profiling
# Objective: Profile CPU/memory usage of API and agent components

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 4.2: Resource Usage Profiling ==="
echo ""

NAMESPACE="${NAMESPACE:-streamspace}"

# Check if metrics-server is available
if ! kubectl top nodes &>/dev/null; then
  echo -e "${YELLOW}⚠ SKIPPED: metrics-server not available${NC}"
  echo "Install metrics-server to enable resource profiling"
  exit 0
fi

echo "Step 1: Baseline resource usage (no load)..."
echo ""

echo "API Pods:"
kubectl top pods -n "$NAMESPACE" -l app=streamspace-api

echo ""
echo "Agent Pods:"
kubectl top pods -n "$NAMESPACE" -l app=streamspace-k8s-agent

echo ""
echo "PostgreSQL:"
kubectl top pods -n "$NAMESPACE" -l app=postgres

echo ""

# Create load
if [ -n "$TOKEN" ]; then
  echo "Step 2: Creating load (5 sessions)..."

  API_BASE="${API_BASE_URL:-http://localhost:8000}"
  declare -a SESSION_IDS

  for i in {1..5}; do
    SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user\":\"prof$i\",\"template\":\"firefox-browser\",\"resources\":{\"cpu\":\"500m\",\"memory\":\"1Gi\"}}")

    SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')
    if [ "$SESSION_ID" != "null" ]; then
      SESSION_IDS+=("$SESSION_ID")
    fi
    sleep 2
  done

  echo ""
  echo "Waiting for sessions to stabilize (30s)..."
  sleep 30

  echo ""
  echo "Step 3: Resource usage under load..."
  echo ""

  echo "API Pods:"
  kubectl top pods -n "$NAMESPACE" -l app=streamspace-api

  echo ""
  echo "Agent Pods:"
  kubectl top pods -n "$NAMESPACE" -l app=streamspace-k8s-agent

  echo ""
  echo "Session Pods:"
  kubectl top pods -n "$NAMESPACE" -l app=streamspace-session 2>/dev/null || echo "No session pods found"

  # Cleanup
  echo ""
  echo "Cleanup..."
  for SESSION_ID in "${SESSION_IDS[@]}"; do
    curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
      -H "Authorization: Bearer $TOKEN" > /dev/null
  done
fi

echo ""
echo "=== Test 4.2: COMPLETED ==="
echo ""
echo "Note: Review resource usage values above"
echo "Recommended limits for production:"
echo "  API: CPU 1000m, Memory 512Mi"
echo "  Agent: CPU 500m, Memory 256Mi"
echo ""
exit 0
