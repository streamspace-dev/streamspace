#!/bin/bash

set -e

echo "=== StreamSpace Session Termination Test ==="
echo "Session: admin-firefox-browser-d020bb30 (older session)"
echo ""

# Step 1: Get JWT token
echo "[1/5] Authenticating..."
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Failed to get JWT token"
  exit 1
fi
echo "✅ Token obtained: ${TOKEN:0:20}..."
echo ""

# Step 2: Get session details before termination
echo "[2/5] Getting session details before termination..."
SESSION_ID="admin-firefox-browser-d020bb30"
SESSION_BEFORE=$(curl -s -X GET "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
  -H "Authorization: Bearer $TOKEN")
echo "Session state: $(echo $SESSION_BEFORE | jq -r '.state')"
echo "Session user: $(echo $SESSION_BEFORE | jq -r '.user')"
echo ""

# Step 3: Check pod status before termination
echo "[3/5] Checking pod status before termination..."
POD_NAME=$(kubectl get pods -n streamspace -l session=${SESSION_ID} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD_NAME" ]; then
  echo "✅ Pod exists: $POD_NAME"
  kubectl get pod -n streamspace $POD_NAME -o wide
else
  echo "⚠️  Pod not found (may have been cleaned up)"
fi
echo ""

# Step 4: Terminate session
echo "[4/5] Terminating session..."
TERMINATE_RESPONSE=$(curl -s -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
  -H "Authorization: Bearer $TOKEN" \
  -w "\nHTTP_CODE:%{http_code}")

HTTP_CODE=$(echo "$TERMINATE_RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
RESPONSE_BODY=$(echo "$TERMINATE_RESPONSE" | grep -v "HTTP_CODE")

echo "HTTP Status: $HTTP_CODE"
echo "Response: $RESPONSE_BODY"

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "204" ]; then
  echo "✅ Termination request accepted"
else
  echo "❌ Termination failed with HTTP $HTTP_CODE"
  exit 1
fi
echo ""

# Step 5: Verify cleanup
echo "[5/5] Verifying cleanup (waiting 15 seconds)..."
sleep 15

# Check session CRD
echo "Checking Session CRD..."
SESSION_EXISTS=$(kubectl get session -n streamspace ${SESSION_ID} 2>/dev/null || echo "not found")
if echo "$SESSION_EXISTS" | grep -q "not found"; then
  echo "✅ Session CRD deleted"
else
  echo "⚠️  Session CRD still exists"
  kubectl get session -n streamspace ${SESSION_ID}
fi

# Check pod
echo ""
echo "Checking pod..."
POD_EXISTS=$(kubectl get pods -n streamspace -l session=${SESSION_ID} 2>/dev/null || echo "")
if [ -z "$POD_EXISTS" ]; then
  echo "✅ Pod deleted"
else
  echo "⚠️  Pod still exists:"
  kubectl get pods -n streamspace -l session=${SESSION_ID}
fi

# Check deployment
echo ""
echo "Checking deployment..."
DEPLOY_EXISTS=$(kubectl get deployment -n streamspace ${SESSION_ID} 2>/dev/null || echo "")
if [ -z "$DEPLOY_EXISTS" ]; then
  echo "✅ Deployment deleted"
else
  echo "⚠️  Deployment still exists:"
  kubectl get deployment -n streamspace ${SESSION_ID}
fi

# Check service
echo ""
echo "Checking service..."
SVC_EXISTS=$(kubectl get service -n streamspace ${SESSION_ID} 2>/dev/null || echo "")
if [ -z "$SVC_EXISTS" ]; then
  echo "✅ Service deleted"
else
  echo "⚠️  Service still exists:"
  kubectl get service -n streamspace ${SESSION_ID}
fi

echo ""
echo "=== Session Termination Test Complete ==="
