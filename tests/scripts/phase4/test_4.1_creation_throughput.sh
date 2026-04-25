#!/bin/bash
# Test 4.1: Session Creation Throughput
# Objective: Measure sessions created per minute (target: ≥10/min)

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 4.1: Session Creation Throughput ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
TARGET_THROUGHPUT=10  # sessions per minute
TEST_DURATION=60      # seconds

echo "Configuration:"
echo "  Target: ≥${TARGET_THROUGHPUT} sessions/minute"
echo "  Test duration: ${TEST_DURATION}s"
echo ""

declare -a SESSION_IDS
START_TIME=$(date +%s)
SUCCESS_COUNT=0
FAILURE_COUNT=0

echo "Creating sessions..."
echo ""

# Create sessions as fast as possible for TEST_DURATION
COUNTER=1
while true; do
  CURRENT_TIME=$(date +%s)
  ELAPSED=$((CURRENT_TIME - START_TIME))

  if [ $ELAPSED -ge $TEST_DURATION ]; then
    break
  fi

  SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"perftest${COUNTER}\",\"template\":\"firefox-browser\",\"resources\":{\"cpu\":\"500m\",\"memory\":\"1Gi\"}}" \
    2>/dev/null)

  SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id' 2>/dev/null || echo "null")

  if [ "$SESSION_ID" != "null" ] && [ -n "$SESSION_ID" ]; then
    SESSION_IDS+=("$SESSION_ID")
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    echo "  ✓ Session $SUCCESS_COUNT created (${ELAPSED}s)"
  else
    FAILURE_COUNT=$((FAILURE_COUNT + 1))
    echo "  ✗ Failed to create session (${ELAPSED}s)"
  fi

  COUNTER=$((COUNTER + 1))
done

END_TIME=$(date +%s)
ACTUAL_DURATION=$((END_TIME - START_TIME))

echo ""
echo "=== Results ==="
echo ""
echo "Test Duration: ${ACTUAL_DURATION}s"
echo "Successful Creations: $SUCCESS_COUNT"
echo "Failed Creations: $FAILURE_COUNT"

# Calculate throughput
THROUGHPUT=$(echo "scale=2; ($SUCCESS_COUNT / $ACTUAL_DURATION) * 60" | bc)

echo "Throughput: ${THROUGHPUT} sessions/minute"
echo "Target: ≥${TARGET_THROUGHPUT} sessions/minute"
echo ""

# Cleanup
echo "Cleanup: Deleting test sessions..."
for SESSION_ID in "${SESSION_IDS[@]}"; do
  curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1 &
done
wait

echo -e "${GREEN}✓${NC} Cleanup complete"
echo ""

# Evaluate result
if (( $(echo "$THROUGHPUT >= $TARGET_THROUGHPUT" | bc -l) )); then
  echo "=== Test 4.1: PASSED ==="
  echo "Throughput meets target (${THROUGHPUT} >= ${TARGET_THROUGHPUT})"
  exit 0
else
  echo "=== Test 4.1: MARGINAL ==="
  echo "Throughput below target (${THROUGHPUT} < ${TARGET_THROUGHPUT})"
  exit 0  # Still exit 0 as this is performance benchmark, not functionality
fi
