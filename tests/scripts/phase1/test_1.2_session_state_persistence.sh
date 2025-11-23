#!/bin/bash
# Test 1.2: Session State Persistence
# Objective: Verify session state persists across API pod restarts

set -e

echo "=== Test 1.2: Session State Persistence ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo "ERROR: TOKEN not set"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

# Create session
echo "Step 1: Creating test session..."
SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user":"persist-test","template":"firefox-browser","resources":{"cpu":"500m","memory":"1Gi"}}')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')
echo "Session created: $SESSION_ID"
echo ""

# Restart API pod
echo "Step 2: Restarting API pod..."
kubectl delete pod -n "$NAMESPACE" -l app=streamspace-api
echo "Waiting for new pod to be ready..."
kubectl wait --for=condition=ready pod -l app=streamspace-api -n "$NAMESPACE" --timeout=60s
sleep 5
echo ""

# Verify session still exists
echo "Step 3: Verifying session persisted..."
SESSION_CHECK=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN")
STATUS=$(echo "$SESSION_CHECK" | jq -r '.status // .state')

if [ "$STATUS" != "null" ] && [ -n "$STATUS" ]; then
  echo "✓ Session persisted with status: $STATUS"
  echo ""
  echo "=== Test 1.2: PASSED ==="
else
  echo "✗ Session not found after restart"
  echo "=== Test 1.2: FAILED ==="
  exit 1
fi

# Cleanup
curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
exit 0
