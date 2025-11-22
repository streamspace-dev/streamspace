#!/bin/bash
set -e

echo "========================================="
echo "  Testing Session Creation with P1 Fixes"
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

# Create session
echo "2. Creating session..."
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}')

echo "$RESPONSE" | jq .
echo ""

SESSION_ID=$(echo "$RESPONSE" | jq -r '.name // .id // empty')

if [ -z "$SESSION_ID" ]; then
  echo "❌ Failed to create session"
  exit 1
fi

echo "✓ Session created: $SESSION_ID"
echo ""

# Wait for session to start
echo "3. Waiting 10 seconds for session to start..."
sleep 10

# Check if agent_id is populated in database
echo "4. Checking agent_id in database..."
kubectl exec -n streamspace statefulset/streamspace-postgres -- psql -U streamspace -d streamspace -c "SELECT id, agent_id, state FROM sessions WHERE id = '$SESSION_ID';" 2>/dev/null || echo "Could not query database"
echo ""

echo "========================================="
echo "✅ Session creation test complete!"
echo "Session ID: $SESSION_ID"
echo "========================================="
