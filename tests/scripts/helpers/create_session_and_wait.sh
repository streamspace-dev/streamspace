#!/bin/bash
# create_session_and_wait.sh - Create a session and wait for it to reach Running state
# Usage: ./create_session_and_wait.sh <token> <username> <template> <cpu> <memory> [timeout_seconds]

set -e

TOKEN="$1"
USERNAME="$2"
TEMPLATE="$3"
CPU="${4:-1000m}"
MEMORY="${5:-2Gi}"
TIMEOUT="${6:-300}"
API_BASE="${API_BASE_URL:-http://localhost:8000}"

if [ -z "$TOKEN" ] || [ -z "$USERNAME" ] || [ -z "$TEMPLATE" ]; then
  echo "ERROR: Missing required arguments"
  echo "Usage: $0 <token> <username> <template> [cpu] [memory] [timeout_seconds]"
  exit 1
fi

echo "Creating session for user=$USERNAME, template=$TEMPLATE..."

START_TIME=$(date +%s)

SESSION_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"user\": \"$USERNAME\",
    \"template\": \"$TEMPLATE\",
    \"resources\": {
      \"cpu\": \"$CPU\",
      \"memory\": \"$MEMORY\"
    }
  }")

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.sessionId // .session_id // .id')

if [ "$SESSION_ID" == "null" ] || [ -z "$SESSION_ID" ]; then
  echo "ERROR: Failed to create session"
  echo "Response: $SESSION_RESPONSE"
  exit 1
fi

echo "Session created: $SESSION_ID"
echo "Waiting for session to reach Running state (timeout: ${TIMEOUT}s)..."

ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
  STATUS_RESPONSE=$(curl -s "$API_BASE/api/v1/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $TOKEN")

  STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.status // .state')

  if [ "$STATUS" == "Running" ] || [ "$STATUS" == "running" ]; then
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    echo "SUCCESS: Session reached Running state in ${DURATION}s"
    echo "$SESSION_ID"
    exit 0
  elif [ "$STATUS" == "Failed" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "Error" ]; then
    echo "ERROR: Session failed to start"
    echo "Response: $STATUS_RESPONSE"
    exit 1
  fi

  echo "  Status: $STATUS (${ELAPSED}s elapsed)"
  sleep 5
  ELAPSED=$((ELAPSED + 5))
done

echo "ERROR: Timeout waiting for session to reach Running state"
exit 1
