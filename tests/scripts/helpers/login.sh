#!/bin/bash
# login.sh - Authenticate and retrieve JWT token
# Usage: ./login.sh <username> <password>

set -e

USERNAME="${1:-admin}"
PASSWORD="${2:-admin}"
API_BASE="${API_BASE_URL:-http://localhost:8000}"

echo "Authenticating as $USERNAME..."

RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$RESPONSE" | jq -r '.token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "ERROR: Authentication failed"
  echo "Response: $RESPONSE"
  exit 1
fi

echo "$TOKEN"
