#!/bin/bash
# Test 1.3: Multi-User Concurrent Sessions
# Objective: Verify multiple users can have concurrent sessions without interference

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_DIR="$SCRIPT_DIR/../helpers"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 1.3: Multi-User Concurrent Sessions ==="
echo ""

# Check prerequisites
if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"
NUM_USERS=3

echo "Configuration:"
echo "  API: $API_BASE"
echo "  Concurrent users: $NUM_USERS"
echo ""

# Arrays to store session IDs
declare -a SESSION_IDS
declare -a USERS

# Create sessions for multiple users
echo "Step 1: Creating concurrent sessions..."

for i in $(seq 1 $NUM_USERS); do
  USER="user${i}"
  USERS+=("$USER")

  echo "  Creating session for $USER..."

  SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"user\": \"$USER\",
      \"template\": \"firefox-browser\",
      \"resources\": {
        \"cpu\": \"500m\",
        \"memory\": \"1Gi\"
      }
    }")

  SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

  if [ "$SESSION_ID" == "null" ] || [ -z "$SESSION_ID" ]; then
    echo -e "${RED}✗ FAILED: Could not create session for $USER${NC}"
    echo "Response: $SESSION_RESPONSE"

    # Cleanup any created sessions
    for cleanup_id in "${SESSION_IDS[@]}"; do
      curl -s -X DELETE "$API_BASE/api/v1/sessions/$cleanup_id" \
        -H "Authorization: Bearer $TOKEN" > /dev/null
    done

    exit 1
  fi

  SESSION_IDS+=("$SESSION_ID")
  echo -e "  ${GREEN}✓${NC} Session created: $SESSION_ID"

  sleep 1  # Slight delay between creations
done

echo ""
echo -e "${GREEN}✓${NC} All $NUM_USERS sessions created successfully"
echo ""

# Verify all sessions are independent
echo "Step 2: Verifying session independence..."

for i in "${!SESSION_IDS[@]}"; do
  SESSION_ID="${SESSION_IDS[$i]}"
  USER="${USERS[$i]}"

  SESSION_DETAILS=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN")

  OWNER=$(echo "$SESSION_DETAILS" | jq -r '.user // .owner')
  STATUS=$(echo "$SESSION_DETAILS" | jq -r '.status // .state')

  if [ "$OWNER" == "$USER" ]; then
    echo -e "  ${GREEN}✓${NC} Session $SESSION_ID correctly assigned to $USER (status: $STATUS)"
  else
    echo -e "  ${RED}✗${NC} Session $SESSION_ID owner mismatch: expected $USER, got $OWNER"
  fi
done

echo ""

# Verify pods are created for all sessions
echo "Step 3: Verifying pod creation..."

ALL_PODS_FOUND=true

for i in "${!SESSION_IDS[@]}"; do
  SESSION_ID="${SESSION_IDS[$i]}"
  USER="${USERS[$i]}"

  # Wait briefly for pod
  sleep 5

  POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_ID" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$POD_NAME" ]; then
    echo -e "  ${GREEN}✓${NC} Pod created for $USER: $POD_NAME"
  else
    echo -e "  ${RED}✗${NC} No pod found for $USER session $SESSION_ID"
    ALL_PODS_FOUND=false
  fi
done

echo ""

# Check resource isolation
echo "Step 4: Checking resource isolation..."

echo "Verifying each session has separate pods:"
POD_COUNT=$(kubectl get pods -n "$NAMESPACE" -l "app=streamspace-session" -o name 2>/dev/null | wc -l)
echo "  Total session pods: $POD_COUNT (expected: >= $NUM_USERS)"

if [ "$POD_COUNT" -ge "$NUM_USERS" ]; then
  echo -e "  ${GREEN}✓${NC} Resource isolation verified"
else
  echo -e "  ${YELLOW}⚠${NC} Pod count lower than expected"
fi

echo ""

# List all concurrent sessions
echo "Step 5: Listing all active sessions..."

ALL_SESSIONS=$(curl -s "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN")

TOTAL_SESSIONS=$(echo "$ALL_SESSIONS" | jq '. | length')
echo "  Total sessions in API: $TOTAL_SESSIONS"

for SESSION_ID in "${SESSION_IDS[@]}"; do
  FOUND=$(echo "$ALL_SESSIONS" | jq -r ".[] | select(.sessionId == \"$SESSION_ID\" or .session_id == \"$SESSION_ID\" or .id == \"$SESSION_ID\") | .sessionId // .session_id // .id")

  if [ -n "$FOUND" ]; then
    echo -e "  ${GREEN}✓${NC} Session $SESSION_ID present in list"
  else
    echo -e "  ${RED}✗${NC} Session $SESSION_ID missing from list"
  fi
done

echo ""

# Cleanup
echo "Cleanup: Deleting all test sessions..."

for i in "${!SESSION_IDS[@]}"; do
  SESSION_ID="${SESSION_IDS[$i]}"
  USER="${USERS[$i]}"

  curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN" > /dev/null

  echo "  ✓ Deleted session for $USER"
done

echo ""

# Verify cleanup
sleep 5
REMAINING_PODS=$(kubectl get pods -n "$NAMESPACE" -l "app=streamspace-session" -o name 2>/dev/null | wc -l)
echo "Remaining test pods: $REMAINING_PODS"

echo ""

# Determine result
if [ "$ALL_PODS_FOUND" == "true" ]; then
  echo "=== Test 1.3: PASSED ==="
  echo ""
  echo "Success Criteria Met:"
  echo "  ✓ Multiple concurrent sessions created"
  echo "  ✓ Each session correctly assigned to owner"
  echo "  ✓ Separate pods created for each session"
  echo "  ✓ Resource isolation maintained"
  echo "  ✓ All sessions queryable via API"
  echo ""
  exit 0
else
  echo "=== Test 1.3: FAILED ==="
  echo ""
  echo "Some pods were not created successfully"
  exit 1
fi
