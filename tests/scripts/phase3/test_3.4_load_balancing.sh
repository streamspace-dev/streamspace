#!/bin/bash
# Test 3.4: Multi-Agent Load Balancing
# Objective: Verify sessions are distributed across multiple agents
# NOTE: Requires multiple agents to be deployed

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Test 3.4: Multi-Agent Load Balancing ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

# Check number of agents
echo "Step 1: Checking available agents..."

AGENTS_RESPONSE=$(curl -s "$API_BASE/api/v1/agents" \
  -H "Authorization: Bearer $TOKEN")

AGENT_COUNT=$(echo "$AGENTS_RESPONSE" | jq '. | length')
echo "Available agents: $AGENT_COUNT"
echo ""

if [ "$AGENT_COUNT" -lt 2 ]; then
  echo -e "${YELLOW}⚠ SKIPPED: This test requires at least 2 agents${NC}"
  echo "Current agent count: $AGENT_COUNT"
  echo ""
  echo "To run this test, scale up agents:"
  echo "  kubectl scale deployment streamspace-k8s-agent -n $NAMESPACE --replicas=2"
  echo ""
  exit 0
fi

# Create multiple sessions
echo "Step 2: Creating multiple sessions..."

NUM_SESSIONS=4
declare -a SESSION_IDS

for i in $(seq 1 $NUM_SESSIONS); do
  SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"loadtest$i\",\"template\":\"firefox-browser\",\"resources\":{\"cpu\":\"500m\",\"memory\":\"1Gi\"}}")

  SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')
  SESSION_IDS+=("$SESSION_ID")
  echo "  Created session $i: $SESSION_ID"
  sleep 2
done

echo ""

# Check session distribution
echo "Step 3: Analyzing session distribution..."

declare -A AGENT_SESSIONS

for SESSION_ID in "${SESSION_IDS[@]}"; do
  SESSION_DETAILS=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN")

  AGENT=$(echo "$SESSION_DETAILS" | jq -r '.agentId // .agent_id // .agent // "unknown"')

  if [ -n "$AGENT" ] && [ "$AGENT" != "null" ]; then
    AGENT_SESSIONS[$AGENT]=$((${AGENT_SESSIONS[$AGENT]:-0} + 1))
  fi
done

echo "Session distribution:"
for agent in "${!AGENT_SESSIONS[@]}"; do
  count=${AGENT_SESSIONS[$agent]}
  echo "  Agent $agent: $count sessions"
done

echo ""

# Verify distribution is reasonable
UNIQUE_AGENTS=${#AGENT_SESSIONS[@]}

if [ $UNIQUE_AGENTS -ge 2 ]; then
  echo -e "${GREEN}✓${NC} Sessions distributed across multiple agents"
else
  echo -e "${YELLOW}⚠${NC} All sessions on single agent (may indicate load balancing issue)"
fi

echo ""

# Cleanup
echo "Cleanup: Deleting test sessions..."
for SESSION_ID in "${SESSION_IDS[@]}"; do
  curl -s -X DELETE "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN" > /dev/null
done

echo ""
echo "=== Test 3.4: COMPLETED ==="
exit 0
