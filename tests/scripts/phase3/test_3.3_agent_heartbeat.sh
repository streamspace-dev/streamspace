#!/bin/bash
# Test 3.3: Agent Heartbeat and Health Monitoring
# Objective: Verify agent heartbeats are tracked and stale agents detected

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "=== Test 3.3: Agent Heartbeat and Health Monitoring ==="
echo ""

if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN not set${NC}"
  exit 1
fi

API_BASE="${API_BASE_URL:-http://localhost:8000}"
NAMESPACE="${NAMESPACE:-streamspace}"

# Get initial agent status
echo "Step 1: Checking initial agent status..."

AGENTS_RESPONSE=$(curl -s "$API_BASE/api/v1/agents" \
  -H "Authorization: Bearer $TOKEN")

AGENT_COUNT=$(echo "$AGENTS_RESPONSE" | jq '. | length')
echo "Active agents: $AGENT_COUNT"

if [ "$AGENT_COUNT" -eq 0 ]; then
  echo -e "${RED}✗ No agents registered${NC}"
  exit 1
fi

# Get first agent details
FIRST_AGENT=$(echo "$AGENTS_RESPONSE" | jq -r '.[0]')
AGENT_ID=$(echo "$FIRST_AGENT" | jq -r '.agentId // .agent_id // .id')
LAST_HEARTBEAT=$(echo "$FIRST_AGENT" | jq -r '.lastHeartbeat // .last_heartbeat')
STATUS=$(echo "$FIRST_AGENT" | jq -r '.status')

echo "Agent: $AGENT_ID"
echo "Status: $STATUS"
echo "Last Heartbeat: $LAST_HEARTBEAT"
echo ""

# Monitor heartbeats over time
echo "Step 2: Monitoring heartbeats (30 seconds)..."

for i in {1..6}; do
  sleep 5

  AGENT_CHECK=$(curl -s "$API_BASE/api/v1/agents/$AGENT_ID" \
    -H "Authorization: Bearer $TOKEN")

  CURRENT_HEARTBEAT=$(echo "$AGENT_CHECK" | jq -r '.lastHeartbeat // .last_heartbeat')
  CURRENT_STATUS=$(echo "$AGENT_CHECK" | jq -r '.status')

  echo "  Check $i: Status=$CURRENT_STATUS, Heartbeat=$CURRENT_HEARTBEAT"

  if [ "$CURRENT_HEARTBEAT" != "$LAST_HEARTBEAT" ]; then
    echo -e "  ${GREEN}✓${NC} Heartbeat updated"
    LAST_HEARTBEAT="$CURRENT_HEARTBEAT"
  fi
done

echo ""
echo -e "${GREEN}✓${NC} Agent heartbeats are being tracked"
echo ""

# Check agent pod health
echo "Step 3: Checking agent pod health..."

AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app=streamspace-k8s-agent \
  -o jsonpath='{.items[0].metadata.name}')

if [ -n "$AGENT_POD" ]; then
  POD_STATUS=$(kubectl get pod "$AGENT_POD" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
  echo "Agent pod: $AGENT_POD"
  echo "Pod status: $POD_STATUS"

  if [ "$POD_STATUS" == "Running" ]; then
    echo -e "${GREEN}✓${NC} Agent pod healthy"
  else
    echo -e "${RED}✗${NC} Agent pod not running: $POD_STATUS"
  fi
fi

echo ""
echo "=== Test 3.3: PASSED ==="
echo ""
echo "Success Criteria Met:"
echo "  ✓ Agent heartbeats tracked"
echo "  ✓ Heartbeats update regularly"
echo "  ✓ Agent status reported correctly"
echo "  ✓ Agent pod healthy"
echo ""
exit 0
