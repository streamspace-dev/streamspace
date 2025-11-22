#!/bin/bash
set -e

echo "========================================="
echo "  Testing Session Termination Fix (P1)"
echo "========================================="
echo ""

# Get JWT token
echo "1. Getting JWT token..."
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ Failed to get token"
  exit 1
fi

echo "✓ Got token"
echo ""

# Test DELETE endpoint
echo "2. Testing DELETE /api/v1/sessions/admin-firefox-browser-7e367bc3"
RESPONSE=$(curl -s -X DELETE "http://localhost:8000/api/v1/sessions/admin-firefox-browser-7e367bc3" \
  -H "Authorization: Bearer $TOKEN")

BODY="$RESPONSE"

echo "Response:"
echo "$BODY" | jq .
echo ""

# Check if command was dispatched
if echo "$BODY" | jq -e '.commandId' > /dev/null 2>&1; then
  COMMAND_ID=$(echo "$BODY" | jq -r '.commandId')
  echo "✓ Command ID returned: $COMMAND_ID"
  echo ""

  echo "3. Waiting 5 seconds for agent to process command..."
  sleep 5

  # Check agent logs for stop_session command
  echo "4. Checking agent logs for stop_session command..."
  kubectl logs -n streamspace deploy/streamspace-k8s-agent --tail=50 | grep -A5 "stop_session" || echo "No stop_session logs yet"
  echo ""

  # Check if pod still exists
  echo "5. Checking pod status..."
  kubectl get pods -n streamspace | grep "admin-firefox-browser-7e367bc3" || echo "Pod deleted (expected!)"
  echo ""

  # Check session CRD state
  echo "6. Checking session CRD state..."
  kubectl get session admin-firefox-browser-7e367bc3 -n streamspace -o jsonpath='{.spec.state}' 2>/dev/null && echo "" || echo "Session CRD deleted (expected!)"
  echo ""

  echo "========================================="
  echo "✅ Session termination test complete!"
  echo "========================================="
else
  echo "❌ No commandId in response - fix may not be working"
  exit 1
fi
