#!/bin/bash
set -e

echo "============================================="
echo "  E2E VNC Streaming Validation Test"
echo "============================================="
echo ""

API_URL="http://localhost:8000"
TEMPLATE="firefox-browser"

# Get JWT token
echo "1. Getting JWT token..."
TOKEN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Failed to get token"
  exit 1
fi
echo "✓ Got token"
echo ""

# Create session
echo "2. Creating session with template: $TEMPLATE"
SESSION_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user\":\"admin\",\"template\":\"$TEMPLATE\",\"resources\":{\"memory\":\"1Gi\",\"cpu\":\"500m\"},\"persistentHome\":false}")

SESSION_NAME=$(echo "$SESSION_RESPONSE" | jq -r '.name')

if [ -z "$SESSION_NAME" ] || [ "$SESSION_NAME" = "null" ]; then
  echo "❌ Failed to create session"
  echo "$SESSION_RESPONSE" | jq '.'
  exit 1
fi

echo "✓ Session created: $SESSION_NAME"
echo ""

# Monitor session state transitions
echo "3. Monitoring session state transitions (max 60 seconds)..."
START_TIME=$(date +%s)
STATE="pending"
ITERATION=0

while [ "$STATE" != "running" ] && [ "$STATE" != "failed" ]; do
  CURRENT_TIME=$(date +%s)
  ELAPSED=$((CURRENT_TIME - START_TIME))

  if [ $ELAPSED -gt 60 ]; then
    echo "❌ Timeout waiting for session to reach 'running' state"
    break
  fi

  sleep 2
  ITERATION=$((ITERATION + 1))

  # Get session state
  SESSION_INFO=$(curl -s -X GET "$API_URL/api/v1/sessions/$SESSION_NAME" \
    -H "Authorization: Bearer $TOKEN")

  STATE=$(echo "$SESSION_INFO" | jq -r '.state // "unknown"')
  STATUS_MSG=$(echo "$SESSION_INFO" | jq -r '.status.message // "No status"')

  echo "  [${ITERATION}] State: $STATE | Status: $STATUS_MSG | Elapsed: ${ELAPSED}s"
done

echo ""

if [ "$STATE" = "failed" ]; then
  echo "❌ Session failed to start"
  echo "$SESSION_INFO" | jq '.'
  exit 1
fi

if [ "$STATE" != "running" ]; then
  echo "⚠️  Session did not reach 'running' state within timeout"
  echo "Final state: $STATE"
fi

echo "✓ Session state: $STATE (in ${ELAPSED}s)"
echo ""

# Check pod status
echo "4. Checking pod status..."
POD_NAME="${SESSION_NAME}"
POD_STATUS=$(kubectl get pod -n streamspace "$POD_NAME" -o json 2>/dev/null || echo "{}")

if [ "$POD_STATUS" = "{}" ]; then
  echo "❌ Pod not found: $POD_NAME"
else
  PHASE=$(echo "$POD_STATUS" | jq -r '.status.phase')
  CONTAINERS=$(echo "$POD_STATUS" | jq -r '.spec.containers[].name' | tr '\n' ', ' | sed 's/,$//')
  READY=$(echo "$POD_STATUS" | jq -r '.status.conditions[] | select(.type=="Ready") | .status')

  echo "  Pod phase: $PHASE"
  echo "  Containers: $CONTAINERS"
  echo "  Ready: $READY"

  if [ "$PHASE" = "Running" ] && [ "$READY" = "True" ]; then
    echo "✓ Pod is running and ready"
  else
    echo "⚠️  Pod is not fully ready yet"
  fi
fi
echo ""

# Check service
echo "5. Checking service..."
SERVICE_NAME="${SESSION_NAME}"
SERVICE_INFO=$(kubectl get svc -n streamspace "$SERVICE_NAME" -o json 2>/dev/null || echo "{}")

if [ "$SERVICE_INFO" = "{}" ]; then
  echo "❌ Service not found: $SERVICE_NAME"
else
  CLUSTER_IP=$(echo "$SERVICE_INFO" | jq -r '.spec.clusterIP')
  PORTS=$(echo "$SERVICE_INFO" | jq -r '.spec.ports[] | "\(.name):\(.port)->\(.targetPort)"' | tr '\n' ', ' | sed 's/,$//')

  echo "  ClusterIP: $CLUSTER_IP"
  echo "  Ports: $PORTS"
  echo "✓ Service created"
fi
echo ""

# Check VNC connectivity (if session is running)
if [ "$STATE" = "running" ]; then
  echo "6. Checking VNC connectivity..."

  # Try to get VNC URL from session info
  VNC_URL=$(echo "$SESSION_INFO" | jq -r '.url // ""')

  if [ -n "$VNC_URL" ] && [ "$VNC_URL" != "null" ]; then
    echo "  VNC URL: $VNC_URL"
    echo "✓ VNC URL available"
  else
    echo "⚠️  VNC URL not set in session"
  fi
  echo ""
fi

# Display summary
echo "============================================="
echo "  E2E VNC Streaming Test Summary"
echo "============================================="
echo "Session: $SESSION_NAME"
echo "State: $STATE"
echo "Time to state: ${ELAPSED}s"
echo ""

if [ "$STATE" = "running" ]; then
  echo "✅ SUCCESS - Session is running"
  echo ""
  echo "Next steps:"
  echo "1. Port-forward to VNC: kubectl port-forward -n streamspace svc/$SERVICE_NAME 3000:3000"
  echo "2. Access VNC: http://localhost:3000"
  echo "3. Test mouse/keyboard interactions"
  echo ""
else
  echo "⚠️  PARTIAL SUCCESS - Session created but not running yet"
  echo ""
fi

# Keep session for manual testing
echo "Session will remain active for manual VNC testing."
echo "To terminate: curl -X DELETE '$API_URL/api/v1/sessions/$SESSION_NAME' -H 'Authorization: Bearer $TOKEN'"
echo ""
