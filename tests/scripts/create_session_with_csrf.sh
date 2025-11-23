#!/bin/bash
set -e

# Login and save cookies
COOKIES="/tmp/cookies.txt"
rm -f $COOKIES

# Login and get token
RESPONSE=$(curl -s -c $COOKIES -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}')

TOKEN=$(echo "$RESPONSE" | jq -r '.token')
echo "Token obtained: ${TOKEN:0:50}..."

# Get CSRF token from cookies
CSRF_TOKEN=$(grep csrf $COOKIES | awk '{print $7}' || echo "")
echo "CSRF Token: $CSRF_TOKEN"

# Create session with cookies and CSRF token
curl -s -b $COOKIES -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq .
