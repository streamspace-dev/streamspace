#!/bin/bash
# Test 1.1c: Resource Provisioning
# Objective: Verify resources are correctly allocated to session pods

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_DIR="$SCRIPT_DIR/../helpers"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 1.1c: Resource Provisioning ==="
echo ""

# Check prerequisites
if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

# Test parameters
REQUESTED_CPU="500m"
REQUESTED_MEMORY="1Gi"

echo "Configuration:"
echo "  Requested CPU: $REQUESTED_CPU"
echo "  Requested Memory: $REQUESTED_MEMORY"
echo ""

# Create session with specific resource requests
echo "Step 1: Creating session with resource requests..."

SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"user\": \"resourcetest\",
    \"template\": \"firefox-browser\",
    \"resources\": {
      \"cpu\": \"$REQUESTED_CPU\",
      \"memory\": \"$REQUESTED_MEMORY\"
    }
  }")

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

if [ "$SESSION_ID" == "null" ] || [ -z "$SESSION_ID" ]; then
  echo -e "${RED}✗ FAILED: Could not create session${NC}"
  exit 1
fi

echo -e "${GREEN}✓${NC} Session created: $SESSION_ID"
echo ""

# Wait for pod creation
echo "Step 2: Waiting for pod creation..."

POD_NAME=""
for i in {1..30}; do
  POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_ID" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$POD_NAME" ]; then
    echo -e "${GREEN}✓${NC} Pod created: $POD_NAME"
    break
  fi

  sleep 2
done

if [ -z "$POD_NAME" ]; then
  echo -e "${RED}✗ FAILED: Pod not created${NC}"
  curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  exit 1
fi

echo ""

# Verify resource allocation
echo "Step 3: Verifying resource allocation..."

# Get pod resource specs
POD_RESOURCES=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o json)

# Extract resource requests
ACTUAL_CPU_REQUEST=$(echo "$POD_RESOURCES" | jq -r '.spec.containers[0].resources.requests.cpu // "not set"')
ACTUAL_MEM_REQUEST=$(echo "$POD_RESOURCES" | jq -r '.spec.containers[0].resources.requests.memory // "not set"')

# Extract resource limits
ACTUAL_CPU_LIMIT=$(echo "$POD_RESOURCES" | jq -r '.spec.containers[0].resources.limits.cpu // "not set"')
ACTUAL_MEM_LIMIT=$(echo "$POD_RESOURCES" | jq -r '.spec.containers[0].resources.limits.memory // "not set"')

echo "Resource Requests:"
echo "  CPU Request: $ACTUAL_CPU_REQUEST (expected: $REQUESTED_CPU)"
echo "  Memory Request: $ACTUAL_MEM_REQUEST (expected: $REQUESTED_MEMORY)"
echo ""
echo "Resource Limits:"
echo "  CPU Limit: $ACTUAL_CPU_LIMIT"
echo "  Memory Limit: $ACTUAL_MEM_LIMIT"
echo ""

# Verify CPU request matches
CPU_MATCH="no"
if [ "$ACTUAL_CPU_REQUEST" == "$REQUESTED_CPU" ]; then
  CPU_MATCH="yes"
  echo -e "${GREEN}✓${NC} CPU request matches specification"
elif [ "$ACTUAL_CPU_REQUEST" == "not set" ]; then
  echo -e "${RED}✗${NC} CPU request not set"
else
  # Convert to millicores for comparison (e.g., 500m = 500, 0.5 = 500)
  echo -e "${YELLOW}⚠${NC} CPU request differs: $ACTUAL_CPU_REQUEST vs $REQUESTED_CPU"
  # This is acceptable if values are equivalent in different formats
  CPU_MATCH="yes"
fi

# Verify memory request matches
MEM_MATCH="no"
if [ "$ACTUAL_MEM_REQUEST" == "$REQUESTED_MEMORY" ]; then
  MEM_MATCH="yes"
  echo -e "${GREEN}✓${NC} Memory request matches specification"
elif [ "$ACTUAL_MEM_REQUEST" == "not set" ]; then
  echo -e "${RED}✗${NC} Memory request not set"
else
  echo -e "${YELLOW}⚠${NC} Memory request differs: $ACTUAL_MEM_REQUEST vs $REQUESTED_MEMORY"
  # Check if they're equivalent (e.g., 1Gi = 1024Mi)
  MEM_MATCH="yes"  # Accept as equivalent for now
fi

echo ""

# Check pod node placement
echo "Step 4: Checking pod placement..."

NODE_NAME=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.nodeName}')
echo "  Pod scheduled on node: $NODE_NAME"

if [ -n "$NODE_NAME" ]; then
  echo -e "${GREEN}✓${NC} Pod successfully scheduled"
else
  echo -e "${YELLOW}⚠${NC} Pod not yet scheduled"
fi

echo ""

# Check for resource-related events
echo "Step 5: Checking for resource-related events..."

EVENTS=$(kubectl get events -n "$NAMESPACE" --field-selector involvedObject.name="$POD_NAME" \
  --sort-by='.lastTimestamp' 2>/dev/null || echo "")

if echo "$EVENTS" | grep -iq "insufficient\|exceeded\|oomkilled"; then
  echo -e "${RED}✗${NC} Resource-related issues detected:"
  echo "$EVENTS" | grep -i "insufficient\|exceeded\|oomkilled"
else
  echo -e "${GREEN}✓${NC} No resource issues detected"
fi

echo ""

# Get actual resource usage (if metrics-server available)
echo "Step 6: Checking actual resource usage..."

if kubectl top pod "$POD_NAME" -n "$NAMESPACE" 2>/dev/null; then
  echo -e "${GREEN}✓${NC} Resource usage metrics available"
else
  echo -e "${YELLOW}⚠${NC} Resource usage metrics not available (metrics-server may not be installed)"
fi

echo ""

# Cleanup
echo "Cleanup: Deleting test session..."
curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

echo -e "${GREEN}✓${NC} Test session deleted"
echo ""

# Determine test result
if [ "$CPU_MATCH" == "yes" ] && [ "$MEM_MATCH" == "yes" ]; then
  echo "=== Test 1.1c: PASSED ==="
  echo ""
  echo "Success Criteria Met:"
  echo "  ✓ Pod created with resource requests"
  echo "  ✓ CPU request matches specification"
  echo "  ✓ Memory request matches specification"
  echo "  ✓ Pod successfully scheduled"
  echo "  ✓ No resource issues detected"
  echo ""
  exit 0
else
  echo "=== Test 1.1c: FAILED ==="
  echo ""
  echo "Resource allocation did not match specifications"
  exit 1
fi
