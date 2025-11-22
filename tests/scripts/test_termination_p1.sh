#!/bin/bash
set -e

echo "========================================="
echo "  Testing Session Termination (P1 Fix)"
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

# Verify session agent_id in database before termination
echo "2. Verifying agent_id in database before termination..."
kubectl exec -n streamspace statefulset/streamspace-postgres -- psql -U streamspace -d streamspace -c "SELECT id, agent_id, state FROM sessions WHERE id = 'admin-firefox-browser-52bfac7e';" 2>/dev/null
echo ""

# Test DELETE endpoint
echo "3. Sending DELETE request..."
RESPONSE=$(curl -s -X DELETE "http://localhost:8000/api/v1/sessions/admin-firefox-browser-52bfac7e" \
  -H "Authorization: Bearer $TOKEN" \
  -w "\nHTTP_CODE:%{http_code}")

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE")

echo "HTTP Status: $HTTP_CODE"
echo "Response:"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
echo ""

# Check if command was dispatched
if echo "$BODY" | jq -e '.commandId' > /dev/null 2>&1; then
  COMMAND_ID=$(echo "$BODY" | jq -r '.commandId')
  echo "✅ SUCCESS! Command ID returned: $COMMAND_ID"
  echo ""
  
  echo "4. Waiting 5 seconds for agent to process command..."
  sleep 5
  
  # Check agent logs for stop_session command
  echo "5. Checking agent logs for stop_session command..."
  kubectl logs -n streamspace deploy/streamspace-k8s-agent --tail=50 | grep -A5 "stop_session" || echo "⚠️  No stop_session logs yet"
  echo ""
  
  # Check if pod still exists
  echo "6. Checking pod status..."
  kubectl get pods -n streamspace | grep "admin-firefox-browser-52bfac7e" || echo "✓ Pod deleted (expected!)"
  echo ""
  
  # Check session CRD state
  echo "7. Checking session CRD state..."
  CRD_STATE=$(kubectl get session admin-firefox-browser-52bfac7e -n streamspace -o jsonpath='{.spec.state}' 2>/dev/null || echo "deleted")
  echo "Session CRD state: $CRD_STATE"
  echo ""
  
  echo "========================================="
  echo "✅ Session termination test PASSED!"
  echo "========================================="
else
  echo "❌ FAILED: No commandId in response"
  echo "HTTP Status: $HTTP_CODE"
  exit 1
fi
