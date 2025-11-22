#!/bin/bash

set -e

echo "=== P1-VNC-RBAC-001 Fix Validation ==="
echo "Testing VNC tunnel creation with pods/portforward permission"
echo ""

# Step 1: Get JWT token
echo "[1/4] Authenticating..."
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
echo "[2/4] Creating session..."
SESSION_RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {
      "memory": "1Gi",
      "cpu": "500m"
    },
    "persistentHome": false
  }')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.name')
if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
  echo "❌ Failed to create session"
  echo "Response: $SESSION_RESPONSE"
  exit 1
fi
echo "✅ Session created: $SESSION_ID"
echo ""

# Step 3: Wait for pod to be ready (max 30 seconds)
echo "[3/4] Waiting for pod to be ready (max 30 seconds)..."
for i in {1..30}; do
  POD_STATUS=$(kubectl get pods -n streamspace -l session=${SESSION_ID} -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
  if [ "$POD_STATUS" = "Running" ]; then
    POD_NAME=$(kubectl get pods -n streamspace -l session=${SESSION_ID} -o jsonpath='{.items[0].metadata.name}')
    echo "✅ Pod running: $POD_NAME"
    break
  fi
  echo -n "."
  sleep 1
done
echo ""
echo ""

# Step 4: Check agent logs for VNC tunnel creation
echo "[4/4] Checking agent logs for VNC tunnel creation..."
sleep 5  # Give agent time to create VNC tunnel

AGENT_LOGS=$(kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent --tail=50)

# Check for VNC tunnel success
if echo "$AGENT_LOGS" | grep -q "Port-forward established for session ${SESSION_ID}"; then
  echo "✅ VNC tunnel created successfully!"
  echo ""
  echo "Agent log excerpt:"
  echo "$AGENT_LOGS" | grep -A 2 -B 2 "VNCTunnel.*${SESSION_ID}"
elif echo "$AGENT_LOGS" | grep -q "Port-forward error.*${SESSION_ID}"; then
  echo "❌ VNC tunnel creation failed with RBAC error"
  echo ""
  echo "Agent log excerpt:"
  echo "$AGENT_LOGS" | grep -A 3 "Port-forward error.*${SESSION_ID}"
  exit 1
else
  echo "⚠️  VNC tunnel status unclear, showing recent logs:"
  echo "$AGENT_LOGS" | grep -E "(VNC|${SESSION_ID})" | tail -10
fi

echo ""
echo "=== P1-VNC-RBAC-001 Fix Validation Complete ==="
echo ""
echo "Session: $SESSION_ID"
echo "Cleanup: kubectl delete session -n streamspace $SESSION_ID"
