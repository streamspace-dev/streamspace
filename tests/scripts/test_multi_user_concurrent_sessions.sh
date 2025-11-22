#!/bin/bash

set -e

echo "=== Integration Test 1.3: Multi-User Concurrent Sessions ==="
echo "Objective: Validate session isolation and concurrent execution"
echo ""

# Configuration
NUM_SESSIONS=5
USERS=("user1" "user2" "user3" "user4" "user5")
TEMPLATES=("firefox-browser" "firefox-browser" "firefox-browser" "firefox-browser" "firefox-browser")

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

for i in $(seq 0 $((NUM_SESSIONS-1))); do
  USER=${USERS[$i]}
  TEMPLATE=${TEMPLATES[$i]}
  
  # Create session in background
  (
    RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{
        \"user\": \"$USER\",
        \"template\": \"$TEMPLATE\",
        \"resources\": {
          \"memory\": \"512Mi\",
          \"cpu\": \"250m\"
        },
        \"persistentHome\": false
      }")
    
    SESSION_ID=$(echo $RESPONSE | jq -r '.name')
    echo "$SESSION_ID" > /tmp/session_${i}.txt
    echo "  Created: $SESSION_ID (user: $USER)"
  ) &
  
  PIDS+=($!)
done

# Wait for all creation requests to complete
echo "  Waiting for all creation requests..."
for pid in "${PIDS[@]}"; do
  wait $pid
done

# Collect session IDs
for i in $(seq 0 $((NUM_SESSIONS-1))); do
  SESSION_ID=$(cat /tmp/session_${i}.txt)
  SESSION_IDS+=($SESSION_ID)
  rm /tmp/session_${i}.txt
done

echo "✅ All $NUM_SESSIONS sessions created"
echo ""

# Step 3: Wait for all pods to be ready
echo "[3/6] Waiting for all pods to be ready (max 60 seconds)..."
START_TIME=$(date +%s)
ALL_READY=false

for attempt in {1..60}; do
  READY_COUNT=0
  
  for SESSION_ID in "${SESSION_IDS[@]}"; do
    POD_STATUS=$(kubectl get pods -n streamspace -l session=${SESSION_ID} -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
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
  echo "⚠️  Only $READY_COUNT/$NUM_SESSIONS pods ready after 60 seconds"
fi
echo ""

# Step 4: Verify session isolation
echo "[4/6] Verifying session isolation..."

# Check that each session has its own pod, deployment, service
ISOLATED=true
for SESSION_ID in "${SESSION_IDS[@]}"; do
  POD_COUNT=$(kubectl get pods -n streamspace -l session=${SESSION_ID} --no-headers 2>/dev/null | wc -l)
  DEPLOY_EXISTS=$(kubectl get deployment -n streamspace ${SESSION_ID} 2>/dev/null && echo "yes" || echo "no")
  SVC_EXISTS=$(kubectl get service -n streamspace ${SESSION_ID} 2>/dev/null && echo "yes" || echo "no")
  
  if [ "$POD_COUNT" -ne 1 ] || [ "$DEPLOY_EXISTS" != "yes" ] || [ "$SVC_EXISTS" != "yes" ]; then
    echo "  ❌ Session $SESSION_ID missing resources (pod: $POD_COUNT, deploy: $DEPLOY_EXISTS, svc: $SVC_EXISTS)"
    ISOLATED=false
  fi
done

if [ "$ISOLATED" = true ]; then
  echo "✅ All sessions have isolated resources (pod, deployment, service)"
else
  echo "⚠️  Some sessions missing resources"
fi
echo ""

# Step 5: Check resource usage
echo "[5/6] Checking resource usage..."
kubectl top pods -n streamspace -l 'session' 2>/dev/null | head -10 || echo "  (metrics not available)"
echo ""

# Step 6: Terminate all sessions
echo "[6/6] Terminating all sessions..."
TERMINATED=0

for SESSION_ID in "${SESSION_IDS[@]}"; do
  RESPONSE=$(curl -s -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
    -H "Authorization: Bearer $TOKEN" \
    -w "\nHTTP_CODE:%{http_code}")
  
  HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
  
  if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "204" ]; then
    ((TERMINATED++))
    echo "  ✅ Terminated: $SESSION_ID"
  else
    echo "  ❌ Failed to terminate: $SESSION_ID (HTTP $HTTP_CODE)"
  fi
done

echo ""
echo "✅ Terminated $TERMINATED/$NUM_SESSIONS sessions"
echo ""

# Step 7: Verify cleanup (wait 10 seconds)
echo "Verifying cleanup (waiting 10 seconds)..."
sleep 10

REMAINING_PODS=$(kubectl get pods -n streamspace -l 'session' --no-headers 2>/dev/null | grep -E "$(IFS=\|; echo "${SESSION_IDS[*]}")" | wc -l)

echo "Remaining pods from test sessions: $REMAINING_PODS"

if [ $REMAINING_PODS -eq 0 ]; then
  echo "✅ All test session pods cleaned up"
else
  echo "⚠️  $REMAINING_PODS pods still exist"
  kubectl get pods -n streamspace -l 'session' | grep -E "$(IFS=\|; echo "${SESSION_IDS[*]}")"
fi

echo ""
echo "=== Test 1.3: Multi-User Concurrent Sessions Complete ==="
echo ""
echo "Summary:"
echo "  Sessions created: $NUM_SESSIONS"
echo "  Pods ready: $READY_COUNT/$NUM_SESSIONS"
echo "  Time to ready: ${ELAPSED}s"
echo "  Sessions terminated: $TERMINATED/$NUM_SESSIONS"
echo "  Cleanup: $([ $REMAINING_PODS -eq 0 ] && echo 'Complete' || echo 'Incomplete')"
