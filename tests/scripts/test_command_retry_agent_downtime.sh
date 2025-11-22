#!/bin/bash

set -e

echo "=== Integration Test 3.2: Command Retry During Agent Downtime ==="
echo "Objective: Validate commands queued during agent downtime are processed after reconnection"
echo ""

# Configuration
AGENT_RESTART_TIMEOUT=60  # seconds to wait for agent to restart and reconnect
COMMAND_PROCESSING_TIMEOUT=30  # seconds to wait for queued command to be processed

# Step 1: Get JWT token
echo "[1/8] Authenticating..."
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Failed to get JWT token"
  exit 1
fi
echo "✅ Token obtained"
echo ""

# Step 2: Create session
echo "[2/8] Creating test session..."
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "512Mi", "cpu": "250m"},
    "persistentHome": false
  }')

SESSION_ID=$(echo "$RESPONSE" | jq -r '.name')
if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
  echo "❌ Failed to create session"
  echo "Response: $RESPONSE"
  exit 1
fi
echo "✅ Session created: $SESSION_ID"
echo ""

# Step 3: Wait for session pod to be running
echo "[3/8] Waiting for session pod to be running (max 60 seconds)..."
START_TIME=$(date +%s)

for attempt in {1..60}; do
  POD_STATUS=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
  if [ "$POD_STATUS" = "Running" ]; then
    END_TIME=$(date +%s)
    ELAPSED=$((END_TIME - START_TIME))
    echo ""
    echo "✅ Session pod running (startup time: ${ELAPSED}s)"
    POD_NAME=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" -o jsonpath='{.items[0].metadata.name}')
    echo "  Pod: $POD_NAME"
    break
  fi
  echo -n "."
  sleep 1
done

if [ "$POD_STATUS" != "Running" ]; then
  echo ""
  echo "❌ Session pod failed to reach Running state"
  exit 1
fi
echo ""

# Step 4: Capture current agent pod name
echo "[4/8] Capturing current agent state..."
AGENT_POD=$(kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
echo "  Current agent pod: $AGENT_POD"
echo ""

# Step 5: Kill agent pod (simulate downtime)
echo "[5/8] Killing agent pod (simulating downtime)..."
kubectl delete pod -n streamspace "$AGENT_POD" >/dev/null 2>&1
echo "  ✅ Agent pod deleted: $AGENT_POD"
echo ""

# Wait a few seconds for pod to terminate
echo "  Waiting for agent pod to terminate (5 seconds)..."
sleep 5
echo ""

# Step 6: Send termination command while agent is down
echo "[6/8] Sending session termination command while agent is down..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
  -H "Authorization: Bearer $TOKEN")

echo "  HTTP Response: $HTTP_CODE"

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "204" ]; then
  echo "  ✅ API accepted termination command (HTTP $HTTP_CODE)"
else
  echo "  ⚠️  Unexpected HTTP code: $HTTP_CODE (expected 200/202/204)"
fi
echo ""

# Step 7: Verify command stored in database
echo "[7/8] Verifying command queued in database..."
COMMAND_CHECK=$(kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -t -c "SELECT COUNT(*) FROM agent_commands WHERE session_id = '${SESSION_ID}' AND status IN ('pending', 'completed');" 2>/dev/null | tr -d ' ')

if [ "$COMMAND_CHECK" -gt 0 ]; then
  echo "  ✅ Command found in agent_commands table (count: $COMMAND_CHECK)"

  # Show command details
  echo ""
  echo "  Command details:"
  kubectl exec -n streamspace streamspace-postgres-0 -- \
    psql -U streamspace -d streamspace \
    -c "SELECT command_id, session_id, action, status, created_at FROM agent_commands WHERE session_id = '${SESSION_ID}' ORDER BY created_at DESC LIMIT 1;"
else
  echo "  ⚠️  No command found in agent_commands table (may have processed already or API bypass)"
fi
echo ""

# Step 8: Wait for agent to restart and reconnect
echo "[8/8] Waiting for agent to restart and reconnect (max ${AGENT_RESTART_TIMEOUT}s)..."
RESTART_START=$(date +%s)
AGENT_RECONNECTED=false

for attempt in $(seq 1 $AGENT_RESTART_TIMEOUT); do
  # Get new agent pod (will be different after restart)
  NEW_AGENT_POD=$(kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$NEW_AGENT_POD" ] && [ "$NEW_AGENT_POD" != "$AGENT_POD" ]; then
    # New pod exists, check if it's running and connected
    POD_STATUS=$(kubectl get pod -n streamspace "$NEW_AGENT_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$POD_STATUS" = "Running" ]; then
      # Check logs for WebSocket connection
      sleep 2  # Give agent time to connect
      NEW_AGENT_LOGS=$(kubectl logs -n streamspace "$NEW_AGENT_POD" --tail=30 2>/dev/null || echo "")

      if echo "$NEW_AGENT_LOGS" | grep -q "WebSocket connected"; then
        RESTART_END=$(date +%s)
        RESTART_TIME=$((RESTART_END - RESTART_START))
        echo ""
        echo "✅ Agent restarted and reconnected in ${RESTART_TIME}s"
        echo "  New agent pod: $NEW_AGENT_POD"
        AGENT_RECONNECTED=true
        break
      fi
    fi
  fi

  echo -n "."
  sleep 1
done

if [ "$AGENT_RECONNECTED" = false ]; then
  echo ""
  echo "❌ Agent failed to restart and reconnect within ${AGENT_RESTART_TIMEOUT}s"
  exit 1
fi
echo ""

# Step 9: Wait for queued command to be processed
echo "Waiting for queued command to be processed (max ${COMMAND_PROCESSING_TIMEOUT}s)..."
PROCESS_START=$(date +%s)
COMMAND_PROCESSED=false

for attempt in $(seq 1 $COMMAND_PROCESSING_TIMEOUT); do
  # Check if session pod has been deleted
  POD_COUNT=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')

  if [ "$POD_COUNT" = "0" ]; then
    PROCESS_END=$(date +%s)
    PROCESS_TIME=$((PROCESS_END - PROCESS_START))
    echo ""
    echo "✅ Session pod deleted (command processed in ${PROCESS_TIME}s)"
    COMMAND_PROCESSED=true
    break
  fi

  echo -n "."
  sleep 1
done

if [ "$COMMAND_PROCESSED" = false ]; then
  echo ""
  echo "⚠️  Session pod still running after ${COMMAND_PROCESSING_TIMEOUT}s"
  echo "  This may indicate command was not processed or is still processing"
fi
echo ""

# Step 10: Verify command status in database
echo "Verifying final command status..."
FINAL_COMMAND_STATUS=$(kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -t -c "SELECT status FROM agent_commands WHERE session_id = '${SESSION_ID}' ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d ' ')

if [ -n "$FINAL_COMMAND_STATUS" ]; then
  echo "  Command status: $FINAL_COMMAND_STATUS"
  if [ "$FINAL_COMMAND_STATUS" = "completed" ]; then
    echo "  ✅ Command marked as completed"
  elif [ "$FINAL_COMMAND_STATUS" = "pending" ]; then
    echo "  ⚠️  Command still pending (may be processing)"
  else
    echo "  ⚠️  Command status: $FINAL_COMMAND_STATUS"
  fi
else
  echo "  ℹ️  Command not found in database (may have been cleaned up or API bypass used)"
fi
echo ""

# Summary
echo "=== Test 3.2: Command Retry During Agent Downtime - Summary ==="
echo ""
echo "Results:"
echo "  Session created: $SESSION_ID"
echo "  Session pod startup time: ${ELAPSED}s"
echo "  Agent pod killed: $AGENT_POD"
echo "  Termination command sent: HTTP $HTTP_CODE"
echo "  Command queued: $([ "$COMMAND_CHECK" -gt 0 ] && echo 'Yes' || echo 'Unknown')"
echo "  Agent restart time: ${RESTART_TIME}s"
echo "  Command processed: $([ "$COMMAND_PROCESSED" = true ] && echo 'Yes' || echo 'Pending')"
echo "  Final command status: ${FINAL_COMMAND_STATUS:-'Unknown'}"
echo ""

# Overall test result
if [ "$AGENT_RECONNECTED" = true ] && [ "$COMMAND_PROCESSED" = true ]; then
  echo "✅ TEST PASSED: Command queued during downtime and processed after reconnection"
  exit 0
elif [ "$AGENT_RECONNECTED" = true ]; then
  echo "⚠️  TEST PARTIAL: Agent reconnected but command processing status unclear"
  exit 1
else
  echo "❌ TEST FAILED: Agent failed to reconnect"
  exit 1
fi
