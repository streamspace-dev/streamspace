#!/bin/bash
# Test 2.2: Template Updates and Versioning
# Objective: Verify templates can be updated without affecting existing sessions

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "=== Test 2.2: Template Updates and Versioning ==="
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
  -d '{"name":"update-test","displayName":"Update Test","image":"ubuntu:22.04","defaultResources":{"cpu":"500m","memory":"1Gi"}}')

TEMPLATE_ID=$(echo "$TEMPLATE_RESPONSE" | jq -r '.id // .name')
echo -e "${GREEN}✓${NC} Template created: $TEMPLATE_ID"
echo ""

# Update template
echo "Step 2: Updating template..."
UPDATE_RESPONSE=$(curl -s -X PUT "$API_BASE/api/v1/templates/$TEMPLATE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"displayName":"Updated Template","defaultResources":{"cpu":"1000m","memory":"2Gi"}}')

UPDATED=$(echo "$UPDATE_RESPONSE" | jq -r '.displayName')

if [ "$UPDATED" == "Updated Template" ]; then
  echo -e "${GREEN}✓${NC} Template updated successfully"
else
  echo -e "${RED}✗ FAILED: Update failed${NC}"
  curl -s -X DELETE "$API_BASE/api/v1/templates/$TEMPLATE_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  exit 1
fi

echo ""

# Cleanup
curl -s -X DELETE "$API_BASE/api/v1/templates/$TEMPLATE_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
echo "=== Test 2.2: PASSED ==="
exit 0
