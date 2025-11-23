#!/bin/bash
# Test 4.4: Concurrent Session Capacity
# Objective: Determine maximum concurrent sessions the system can handle

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 4.4: Concurrent Session Capacity ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"
MAX_SESSIONS=10  # Conservative limit for local testing

echo "Configuration:"
echo "  Max sessions to create: $MAX_SESSIONS"
echo "  WARNING: This test creates significant load"
echo ""

read -p "Continue? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Test cancelled"
  exit 0
fi

declare -a SESSION_IDS
SUCCESSFUL=0
FAILED=0

echo ""
echo "Creating concurrent sessions..."
echo ""

# Create sessions
for i in $(seq 1 $MAX_SESSIONS); do
  echo "Creating session $i/$MAX_SESSIONS..."

  SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"capacity$i\",\"template\":\"firefox-browser\",\"resources\":{\"cpu\":\"500m\",\"memory\":\"1Gi\"}}")

  SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

  if [ "$SESSION_ID" != "null" ] && [ -n "$SESSION_ID" ]; then
    SESSION_IDS+=("$SESSION_ID")
    SUCCESSFUL=$((SUCCESSFUL + 1))
    echo "  ✓ Session created: $SESSION_ID"
  else
    FAILED=$((FAILED + 1))
    echo "  ✗ Failed to create session"
  fi

  sleep 3  # Don't overwhelm the system
done

echo ""
echo "=== Results ==="
echo ""
echo "Sessions created: $SUCCESSFUL"
echo "Failures: $FAILED"
echo ""

# Check system resources
echo "System resource usage:"
echo ""

if kubectl top nodes &>/dev/null; then
  echo "Node resources:"
  kubectl top nodes
  echo ""
fi

echo "Pod count:"
kubectl get pods -n "$NAMESPACE" --no-headers | wc -l

echo ""

# Cleanup
echo "Cleanup: Deleting all test sessions..."
for SESSION_ID in "${SESSION_IDS[@]}"; do
  curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1 &
done

echo "Waiting for cleanup to complete..."
wait

echo -e "${GREEN}✓${NC} Cleanup complete"
echo ""

echo "=== Test 4.4: COMPLETED ==="
echo ""
echo "Capacity Results:"
echo "  Concurrent sessions created: $SUCCESSFUL"
echo "  System handled load: $([ $SUCCESSFUL -eq $MAX_SESSIONS ] && echo 'YES' || echo 'PARTIAL')"
echo ""
echo "Note: For production capacity planning, consider:"
echo "  - Node resources"
echo "  - Database connections"
echo "  - Network bandwidth"
echo "  - Storage IOPS"
echo ""
exit 0
