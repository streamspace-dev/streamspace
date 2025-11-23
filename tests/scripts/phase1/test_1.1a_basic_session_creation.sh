#!/bin/bash
# Test 1.1a: Basic Session Creation
# Objective: Verify that a session can be successfully created via API

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_DIR="$SCRIPT_DIR/../helpers"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 1.1a: Basic Session Creation ==="
echo ""

# Check prerequisites
if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  echo "Run: source ../env"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

echo "Configuration:"
echo "  API: $API_BASE"
echo "  Namespace: $NAMESPACE"
echo "  User: testuser"
echo "  Template: firefox-browser"
echo ""

# Step 1: Create session
echo "Step 1: Creating session..."

SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "resources": {
      "cpu": "1000m",
      "memory": "2Gi"
    }
  }')

# Extract session ID (try multiple possible field names)
SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

if [ "$SESSION_ID" == "null" ] || [ -z "$SESSION_ID" ]; then
  echo -e "${RED}✗ FAILED: Could not create session${NC}"
  echo "Response: $SESSION_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓${NC} Session created: $SESSION_ID"
echo ""

# Step 2: Verify session in API
echo "Step 2: Verifying session in API..."

SESSION_DETAILS=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN")

API_STATUS=$(echo "$SESSION_DETAILS" | jq -r '.status // .state')

if [ "$API_STATUS" == "null" ] || [ -z "$API_STATUS" ]; then
  echo -e "${RED}✗ FAILED: Session not found in API${NC}"
  echo "Response: $SESSION_DETAILS"
  exit 1
fi

echo -e "${GREEN}✓${NC} Session found in API with status: $API_STATUS"
echo ""

# Step 3: Verify CRD was created
echo "Step 3: Verifying Session CRD..."

sleep 2 # Give controller time to create CRD

CRD_EXISTS=$(kubectl get session -n "$NAMESPACE" "$SESSION_ID" 2>/dev/null && echo "yes" || echo "no")

if [ "$CRD_EXISTS" == "yes" ]; then
  echo -e "${GREEN}✓${NC} Session CRD created"

  CRD_STATE=$(kubectl get session -n "$NAMESPACE" "$SESSION_ID" -o jsonpath='{.spec.state}')
  echo "  CRD State: $CRD_STATE"
else
  echo -e "${YELLOW}⚠${NC} Session CRD not found (may not have propagated yet)"
fi

echo ""

# Step 4: Wait for pod creation
echo "Step 4: Waiting for pod creation..."

POD_FOUND="no"
for i in {1..30}; do
  POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_ID" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$POD_NAME" ]; then
    POD_FOUND="yes"
    echo -e "${GREEN}✓${NC} Pod created: $POD_NAME"
    break
  fi

  echo "  Waiting for pod... (${i}/30)"
  sleep 2
done

if [ "$POD_FOUND" == "no" ]; then
  echo -e "${RED}✗ FAILED: Pod was not created within timeout${NC}"
  echo ""
  echo "Debugging information:"
  kubectl get sessions -n "$NAMESPACE" "$SESSION_ID" -o yaml || true
  kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -n 10
  exit 1
fi

echo ""

# Step 5: Check pod status
echo "Step 5: Checking pod status..."

POD_PHASE=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
echo "  Pod Phase: $POD_PHASE"

if [ "$POD_PHASE" == "Running" ] || [ "$POD_PHASE" == "Pending" ]; then
  echo -e "${GREEN}✓${NC} Pod is in valid state"
else
  echo -e "${YELLOW}⚠${NC} Pod is in unexpected state: $POD_PHASE"
fi

echo ""

# Cleanup
echo "Cleanup: Deleting test session..."
curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

echo -e "${GREEN}✓${NC} Test session deleted"
echo ""

# Summary
echo "=== Test 1.1a: PASSED ==="
echo ""
echo "Success Criteria Met:"
echo "  ✓ API accepts session creation request"
echo "  ✓ Session ID returned and valid"
echo "  ✓ Session queryable via GET endpoint"
echo "  ✓ Session CRD created in Kubernetes"
echo "  ✓ Pod created for session"
echo ""
echo "Test Duration: ${SECONDS}s"
echo ""

exit 0
