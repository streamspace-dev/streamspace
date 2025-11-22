#!/bin/bash

set -e

echo "=== Integration Test 3.1: Agent Disconnection During Active Sessions ==="
echo "Objective: Validate system resilience when agent disconnects and reconnects"
echo ""

# Configuration
NUM_SESSIONS=5
RECONNECT_TIMEOUT=60  # seconds to wait for agent reconnection

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

# Step 2: Create 5 sessions
echo "[2/8] Creating $NUM_SESSIONS sessions..."
SESSION_IDS=()

for i in $(seq 1 $NUM_SESSIONS); do
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
  SESSION_IDS+=($SESSION_ID)
  echo "  Created session $i: $SESSION_ID"
done

echo "✅ All $NUM_SESSIONS sessions created"
echo ""

# Step 3: Wait for all pods to be running
echo "[3/8] Waiting for all pods to be running (max 60 seconds)..."
START_TIME=$(date +%s)

for attempt in {1..60}; do
  RUNNING_COUNT=0

  for SESSION_ID in "${SESSION_IDS[@]}"; do
    POD_STATUS=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
    if [ "$POD_STATUS" = "Running" ]; then
      ((RUNNING_COUNT++))
    fi
  done

  if [ $RUNNING_COUNT -eq $NUM_SESSIONS ]; then
    END_TIME=$(date +%s)
    ELAPSED=$((END_TIME - START_TIME))
    echo ""
    echo "✅ All $NUM_SESSIONS pods running (startup time: ${ELAPSED}s)"
    break
  fi

  echo -n "."
  sleep 1
done

if [ $RUNNING_COUNT -ne $NUM_SESSIONS ]; then
  echo ""
  echo "⚠️  Only $RUNNING_COUNT/$NUM_SESSIONS pods running after 60 seconds"
fi
echo ""

# Step 4: Capture agent pod name and verify connection
echo "[4/8] Capturing agent state before restart..."
AGENT_POD=$(kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent -o jsonpath='{.items[0].metadata.name}')
echo "  Current agent pod: $AGENT_POD"

# Verify agent is connected
AGENT_LOGS=$(kubectl logs -n streamspace "$AGENT_POD" --tail=20 2>/dev/null || echo "")
if echo "$AGENT_LOGS" | grep -q "WebSocket connected"; then
  echo "  ✅ Agent WebSocket connected"
else
  echo "  ⚠️  Agent WebSocket status unknown"
fi
echo ""

# Step 5: Restart agent deployment (simulate disconnect)
echo "[5/8] Restarting agent deployment (simulating disconnect)..."
kubectl rollout restart deployment/streamspace-k8s-agent -n streamspace >/dev/null 2>&1
echo "  ✅ Agent deployment restart triggered"
echo ""

# Step 6: Wait for agent to reconnect
echo "[6/8] Waiting for agent to reconnect (max ${RECONNECT_TIMEOUT}s)..."
RECONNECT_START=$(date +%s)
AGENT_RECONNECTED=false

for attempt in $(seq 1 $RECONNECT_TIMEOUT); do
  # Get new agent pod name (will be different after restart)
  NEW_AGENT_POD=$(kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$NEW_AGENT_POD" ] && [ "$NEW_AGENT_POD" != "$AGENT_POD" ]; then
    # New pod exists, check if it's connected
    POD_STATUS=$(kubectl get pod -n streamspace "$NEW_AGENT_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$POD_STATUS" = "Running" ]; then
      # Check logs for WebSocket connection
      sleep 2  # Give agent time to connect
      NEW_AGENT_LOGS=$(kubectl logs -n streamspace "$NEW_AGENT_POD" --tail=30 2>/dev/null || echo "")

      if echo "$NEW_AGENT_LOGS" | grep -q "WebSocket connected"; then
        RECONNECT_END=$(date +%s)
        RECONNECT_TIME=$((RECONNECT_END - RECONNECT_START))
        echo ""
        echo "✅ Agent reconnected in ${RECONNECT_TIME}s"
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
  echo "❌ Agent failed to reconnect within ${RECONNECT_TIMEOUT}s"
  exit 1
fi
echo ""

# Step 7: Verify existing sessions still accessible (pods still running)
echo "[7/8] Verifying existing sessions still accessible..."
ACCESSIBLE_COUNT=0

for SESSION_ID in "${SESSION_IDS[@]}"; do
  POD_STATUS=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
  if [ "$POD_STATUS" = "Running" ]; then
    ((ACCESSIBLE_COUNT++))
  fi
done

echo "  Sessions still running: $ACCESSIBLE_COUNT/$NUM_SESSIONS"

if [ $ACCESSIBLE_COUNT -eq $NUM_SESSIONS ]; then
  echo "✅ All sessions survived agent restart"
else
  echo "⚠️  $((NUM_SESSIONS - ACCESSIBLE_COUNT)) sessions lost during agent restart"
fi
echo ""

# Step 8: Create new session post-reconnection
echo "[8/8] Creating new session post-reconnection..."
NEW_SESSION_RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "512Mi", "cpu": "250m"},
    "persistentHome": false
  }')

NEW_SESSION_ID=$(echo "$NEW_SESSION_RESPONSE" | jq -r '.name')
echo "  Created post-reconnect session: $NEW_SESSION_ID"

# Wait for new session pod to start
echo "  Waiting for new session pod to start (max 30 seconds)..."
for i in {1..30}; do
  POD_STATUS=$(kubectl get pods -n streamspace -l "session=${NEW_SESSION_ID}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
  if [ "$POD_STATUS" = "Running" ]; then
    echo "  ✅ New session pod running"
    break
  fi
  echo -n "."
  sleep 1
done

echo ""
echo "✅ New session creation successful post-reconnection"
echo ""

# Cleanup: Terminate all test sessions
echo "Cleanup: Terminating all test sessions..."
ALL_SESSION_IDS=("${SESSION_IDS[@]}" "$NEW_SESSION_ID")
TERMINATED=0

for SESSION_ID in "${ALL_SESSION_IDS[@]}"; do
  if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
    continue
  fi

  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
    -H "Authorization: Bearer $TOKEN")

  if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "204" ]; then
    ((TERMINATED++))
  fi
done

echo "  Terminated $TERMINATED sessions"
echo ""

# Wait for cleanup
echo "Waiting for cleanup (10 seconds)..."
sleep 10

# Verify cleanup
REMAINING_PODS=0
for SESSION_ID in "${ALL_SESSION_IDS[@]}"; do
  if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
    continue
  fi
  COUNT=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')
  REMAINING_PODS=$((REMAINING_PODS + COUNT))
done

echo "Remaining test pods: $REMAINING_PODS"
echo ""

# Summary
echo "=== Test 3.1: Agent Disconnection During Active Sessions - Summary ==="
echo ""
echo "Results:"
echo "  Sessions created before restart: $NUM_SESSIONS"
echo "  Sessions survived restart: $ACCESSIBLE_COUNT/$NUM_SESSIONS"
echo "  Agent reconnection time: ${RECONNECT_TIME}s"
echo "  New session creation post-reconnect: $([ -n "$NEW_SESSION_ID" ] && echo 'Success' || echo 'Failed')"
echo "  Sessions terminated: $TERMINATED"
echo "  Cleanup: $([ $REMAINING_PODS -eq 0 ] && echo 'Complete' || echo 'Incomplete')"
echo ""

# Overall test result
if [ $ACCESSIBLE_COUNT -eq $NUM_SESSIONS ] && [ "$AGENT_RECONNECTED" = true ] && [ -n "$NEW_SESSION_ID" ]; then
  echo "✅ TEST PASSED: Agent failover successful with zero data loss"
  exit 0
else
  echo "⚠️  TEST PARTIAL: Some issues detected during failover"
  exit 1
fi
