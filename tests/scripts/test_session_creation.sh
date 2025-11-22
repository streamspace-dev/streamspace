#!/bin/bash
set -e

echo "Setting up port-forward..."
pkill -f "port-forward.*8000" 2>/dev/null || true
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &
PF_PID=$!
sleep 3

echo "Getting JWT token..."
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ Failed to get token"
  kill $PF_PID 2>/dev/null || true
  exit 1
fi

echo "✓ Got token: ${TOKEN:0:20}..."
echo ""
echo "Testing session creation..."
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}')

echo "$RESPONSE" | jq .

# Check if successful
if echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
  echo ""
  echo "❌ Session creation failed"
  kill $PF_PID 2>/dev/null || true
  exit 1
else
  echo ""
  echo "✅ Session creation succeeded!"
  kill $PF_PID 2>/dev/null || true
  exit 0
fi
