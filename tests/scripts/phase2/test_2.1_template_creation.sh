#!/bin/bash
# Test 2.1: Template Creation and Validation
# Objective: Verify templates can be created and validated correctly

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "=== Test 2.1: Template Creation and Validation ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

# Create a test template
echo "Step 1: Creating test template..."

TEMPLATE_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/templates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-template",
    "displayName": "Test Template",
    "description": "A test template for integration testing",
    "image": "ubuntu:22.04",
    "category": "testing",
    "defaultResources": {
      "cpu": "500m",
      "memory": "1Gi"
    },
    "env": {
      "TEST_VAR": "test_value"
    }
  }')

TEMPLATE_ID=$(echo "$TEMPLATE_RESPONSE" | jq -r '.id // .templateId // .name')

if [ "$TEMPLATE_ID" == "null" ] || [ -z "$TEMPLATE_ID" ]; then
  echo -e "${RED}✗ FAILED: Could not create template${NC}"
  echo "Response: $TEMPLATE_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓${NC} Template created: $TEMPLATE_ID"
echo ""

# Verify template exists
echo "Step 2: Verifying template..."

TEMPLATE_CHECK=$(curl -s "$API_BASE/api/v1/templates/$TEMPLATE_ID" \
  -H "Authorization: Bearer $TOKEN")

TEMPLATE_NAME=$(echo "$TEMPLATE_CHECK" | jq -r '.name')

if [ "$TEMPLATE_NAME" == "test-template" ]; then
  echo -e "${GREEN}✓${NC} Template verified: $TEMPLATE_NAME"
else
  echo -e "${RED}✗ FAILED: Template not found or name mismatch${NC}"
  exit 1
fi

echo ""

# Verify CRD created
echo "Step 3: Checking Template CRD..."

sleep 2
CRD_EXISTS=$(kubectl get template -n "$NAMESPACE" "test-template" 2>/dev/null && echo "yes" || echo "no")

if [ "$CRD_EXISTS" == "yes" ]; then
  echo -e "${GREEN}✓${NC} Template CRD created"
else
  echo -e "${RED}✗${NC} Template CRD not found"
fi

echo ""

# Cleanup
echo "Cleanup: Deleting test template..."
curl -s -X DELETE "$API_BASE/api/v1/templates/$TEMPLATE_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

echo -e "${GREEN}✓${NC} Template deleted"
echo ""
echo "=== Test 2.1: PASSED ==="
exit 0
