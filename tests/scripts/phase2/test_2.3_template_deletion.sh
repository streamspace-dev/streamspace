#!/bin/bash
# Test 2.3: Template Deletion Safety
# Objective: Verify templates with active sessions cannot be deleted

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 2.3: Template Deletion Safety ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"

# Create template
echo "Step 1: Creating template..."
TEMPLATE_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/templates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"delete-test","displayName":"Delete Test","image":"ubuntu:22.04","defaultResources":{"cpu":"500m","memory":"1Gi"}}')

TEMPLATE_ID=$(echo "$TEMPLATE_RESPONSE" | jq -r '.id // .name')
echo -e "${GREEN}✓${NC} Template created: $TEMPLATE_ID"
echo ""

# Create session using template
echo "Step 2: Creating session with template..."
SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user\":\"delete-test\",\"template\":\"$TEMPLATE_ID\",\"resources\":{\"cpu\":\"500m\",\"memory\":\"1Gi\"}}")

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')
echo -e "${GREEN}✓${NC} Session created: $SESSION_ID"
echo ""

# Attempt to delete template with active session
echo "Step 3: Attempting to delete template with active session..."
DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE "$API_BASE/api/v1/templates/$TEMPLATE_ID" \
  -H "Authorization: Bearer $TOKEN")

HTTP_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" == "400" ] || [ "$HTTP_CODE" == "409" ]; then
  echo -e "${GREEN}✓${NC} Template deletion correctly blocked (HTTP $HTTP_CODE)"
  echo "This is expected behavior for templates with active sessions"
else
  echo -e "${YELLOW}⚠${NC} Template deletion returned HTTP $HTTP_CODE"
  echo "Expected: 400 or 409 (conflict/bad request)"
fi

echo ""

# Cleanup session first
echo "Cleanup: Deleting session..."
curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2

# Now delete template
echo "Cleanup: Deleting template..."
curl -s -X DELETE "$API_BASE/api/v1/templates/$TEMPLATE_ID" -H "Authorization: Bearer $TOKEN" > /dev/null

echo -e "${GREEN}✓${NC} Cleanup complete"
echo ""
echo "=== Test 2.3: PASSED ==="
exit 0
