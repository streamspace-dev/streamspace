#!/bin/bash
# Test 1.1b: Session Startup Time
# Objective: Measure time from session creation to Running state

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_DIR="$SCRIPT_DIR/../helpers"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 1.1b: Session Startup Time ==="
echo ""

# Check prerequisites
if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  echo "Run: source ../env"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"
TARGET_TIME=60  # Target: session reaches Running in < 60s

echo "Configuration:"
echo "  API: $API_BASE"
echo "  Target startup time: < ${TARGET_TIME}s"
echo ""

# Record start time
START_TIME=$(date +%s)

echo "Creating session and measuring startup time..."
echo ""

# Create session
SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "perftest",
    "template": "firefox-browser",
    "resources": {
      "cpu": "1000m",
      "memory": "2Gi"
    }
  }')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

if [ "$SESSION_ID" == "null" ] || [ -z "$SESSION_ID" ]; then
  echo -e "${RED}✗ FAILED: Could not create session${NC}"
  echo "Response: $SESSION_RESPONSE"
  exit 1
fi

echo "Session created: $SESSION_ID"
echo "Polling for Running status..."
echo ""

# Poll until Running
TIMEOUT=180
ELAPSED=0
STATUS="Unknown"

while [ $ELAPSED -lt $TIMEOUT ]; do
  STATUS_RESPONSE=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN")

  STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.status // .state')

  CURRENT_TIME=$(date +%s)
  DURATION=$((CURRENT_TIME - START_TIME))

  if [ "$STATUS" == "Running" ] || [ "$STATUS" == "running" ]; then
    END_TIME=$(date +%s)
    FINAL_DURATION=$((END_TIME - START_TIME))

    echo -e "${GREEN}✓ Session reached Running state${NC}"
    echo ""
    echo "Timing Results:"
    echo "  Startup Time: ${FINAL_DURATION}s"
    echo "  Target Time: < ${TARGET_TIME}s"

    if [ $FINAL_DURATION -lt $TARGET_TIME ]; then
      echo -e "  ${GREEN}✓ PASSED: Within target time${NC}"
      RESULT="PASSED"
    else
      echo -e "  ${YELLOW}⚠ MARGINAL: Exceeded target by $((FINAL_DURATION - TARGET_TIME))s${NC}"
      RESULT="MARGINAL"
    fi

    # Get additional metrics
    echo ""
    echo "Additional Metrics:"

    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_ID" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [ -n "$POD_NAME" ]; then
      POD_START=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.startTime}')
      POD_READY=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')

      echo "  Pod Name: $POD_NAME"
      echo "  Pod Start Time: $POD_START"
      echo "  Pod Ready: $POD_READY"

      # Get container ready time
      CONTAINER_READY=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[0].ready}')
      echo "  Container Ready: $CONTAINER_READY"
    fi

    # Cleanup
    echo ""
    echo "Cleanup: Deleting test session..."
    curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
      -H "Authorization: Bearer $TOKEN" > /dev/null

    echo ""
    echo "=== Test 1.1b: $RESULT ==="
    echo ""

    if [ "$RESULT" == "PASSED" ]; then
      exit 0
    else
      exit 0  # Still exit 0 for marginal, but could be changed to exit 1 if strict
    fi
  elif [ "$STATUS" == "Failed" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "Error" ]; then
    echo -e "${RED}✗ FAILED: Session failed to start${NC}"
    echo "Final status: $STATUS"
    echo "Response: $STATUS_RESPONSE"

    # Show pod logs for debugging
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_ID" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$POD_NAME" ]; then
      echo ""
      echo "Pod logs:"
      kubectl logs "$POD_NAME" -n "$NAMESPACE" --tail=50 || true
    fi

    exit 1
  fi

  echo "  Status: $STATUS (${DURATION}s elapsed)"
  sleep 5
  ELAPSED=$((ELAPSED + 5))
done

echo -e "${RED}✗ FAILED: Timeout waiting for Running state${NC}"
echo "Final status: $STATUS"
echo ""

# Cleanup on failure
curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

exit 1
