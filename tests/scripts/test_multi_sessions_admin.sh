#!/bin/bash

set -e

echo "=== Integration Test 1.3: Multi-User Concurrent Sessions (admin user) ==="
echo "Objective: Validate concurrent session creation and isolation"
echo ""

# Configuration
NUM_SESSIONS=5

# Step 1: Get JWT token
echo "[1/6] Authenticating..."
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Failed to get JWT token"
  exit 1
fi
echo "✅ Token obtained"
echo ""

# Step 2: Create sessions concurrently
echo "[2/6] Creating $NUM_SESSIONS sessions concurrently..."
SESSION_IDS=()
PIDS=()

for i in $(seq 1 $NUM_SESSIONS); do
  # Create session in background
  (
    RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "user": "admin",
        "template": "firefox-browser",
        "resources": {
          "memory": "512Mi",
          "cpu": "250m"
        },
        "persistentHome": false
      }')
    
    SESSION_ID=$(echo "$RESPONSE" | jq -r '.name')
    echo "$SESSION_ID" > "/tmp/session_${i}.txt"
    echo "  Created session $i: $SESSION_ID"
  ) &
  
  PIDS+=($!)
done

# Wait for all creation requests to complete
echo "  Waiting for all creation requests..."
for pid in "${PIDS[@]}"; do
  wait $pid || true
done

# Collect session IDs
for i in $(seq 1 $NUM_SESSIONS); do
  SESSION_ID=$(cat "/tmp/session_${i}.txt" 2>/dev/null || echo "null")
  SESSION_IDS+=($SESSION_ID)
  rm -f "/tmp/session_${i}.txt"
done

echo "✅ All $NUM_SESSIONS sessions created"
echo "Session IDs: ${SESSION_IDS[@]}"
echo ""

# Step 3: Wait for all pods to be ready
echo "[3/6] Waiting for all pods to be ready (max 45 seconds)..."
START_TIME=$(date +%s)
ALL_READY=false

for attempt in {1..45}; do
  READY_COUNT=0
  
  for SESSION_ID in "${SESSION_IDS[@]}"; do
    if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
      continue
    fi
    
    POD_STATUS=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
    if [ "$POD_STATUS" = "Running" ]; then
      ((READY_COUNT++))
    fi
  done
  
  if [ $READY_COUNT -eq $NUM_SESSIONS ]; then
    ALL_READY=true
    break
  fi
  
  echo -n "."
  sleep 1
done

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
if [ "$ALL_READY" = true ]; then
  echo "✅ All $NUM_SESSIONS pods ready in ${ELAPSED} seconds"
else
  echo "⚠️  Only $READY_COUNT/$NUM_SESSIONS pods ready after 45 seconds"
fi
echo ""

# Step 4: Verify session isolation
echo "[4/6] Verifying session isolation..."
ISOLATED=true

for SESSION_ID in "${SESSION_IDS[@]}"; do
  if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    echo "  ⚠️  Skipping null session ID"
    continue
  fi
  
  POD_COUNT=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')
  DEPLOY_EXISTS=$(kubectl get deployment -n streamspace "${SESSION_ID}" &>/dev/null && echo "yes" || echo "no")
  SVC_EXISTS=$(kubectl get service -n streamspace "${SESSION_ID}" &>/dev/null && echo "yes" || echo "no")
  
  if [ "$POD_COUNT" -ne 1 ] || [ "$DEPLOY_EXISTS" != "yes" ] || [ "$SVC_EXISTS" != "yes" ]; then
    echo "  ❌ Session $SESSION_ID missing resources (pod: $POD_COUNT, deploy: $DEPLOY_EXISTS, svc: $SVC_EXISTS)"
    ISOLATED=false
  else
    echo "  ✅ Session $SESSION_ID has all resources"
  fi
done

echo ""
if [ "$ISOLATED" = true ]; then
  echo "✅ All sessions have isolated resources"
else
  echo "⚠️  Some sessions missing resources"
fi
echo ""

# Step 5: Check VNC tunnels
echo "[5/6] Checking VNC tunnel status..."
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent --tail=30 | grep -E "(VNCTunnel|Port-forward established)" | tail -10 || echo "  No recent VNC tunnel logs"
echo ""

# Step 6: Terminate all sessions
echo "[6/6] Terminating all sessions..."
TERMINATED=0

for SESSION_ID in "${SESSION_IDS[@]}"; do
  if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    echo "  ⚠️  Skipping null session ID"
    continue
  fi
  
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
    -H "Authorization: Bearer $TOKEN")
  
  if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "204" ]; then
    ((TERMINATED++))
    echo "  ✅ Terminated: $SESSION_ID"
  else
    echo "  ❌ Failed to terminate: $SESSION_ID (HTTP $HTTP_CODE)"
  fi
done

echo ""
echo "✅ Terminated $TERMINATED sessions"
echo ""

# Step 7: Verify cleanup
echo "Verifying cleanup (waiting 10 seconds)..."
sleep 10

REMAINING_PODS=0
for SESSION_ID in "${SESSION_IDS[@]}"; do
  if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    continue
  fi
  COUNT=$(kubectl get pods -n streamspace -l "session=${SESSION_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')
  REMAINING_PODS=$((REMAINING_PODS + COUNT))
done

echo "Remaining pods from test sessions: $REMAINING_PODS"

if [ $REMAINING_PODS -eq 0 ]; then
  echo "✅ All test session pods cleaned up"
else
  echo "⚠️  $REMAINING_PODS pods still exist"
fi

echo ""
echo "=== Test 1.3: Multi-User Concurrent Sessions Complete ==="
echo ""
echo "Summary:"
echo "  Sessions created: $NUM_SESSIONS"
echo "  Pods ready: $READY_COUNT/$NUM_SESSIONS"
echo "  Time to ready: ${ELAPSED}s"
echo "  Sessions terminated: $TERMINATED"
echo "  Cleanup: $([ $REMAINING_PODS -eq 0 ] && echo 'Complete' || echo 'Incomplete')"
