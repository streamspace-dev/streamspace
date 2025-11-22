#!/bin/bash

echo "==================================="
echo "  Error Scenario Testing"
echo "==================================="
echo ""

# Get token
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

echo "✓ Got auth token"
echo ""

# Test 1: Invalid template
echo "Test 1: Invalid template name"
echo "---"
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"nonexistent-template","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}')
echo "$RESPONSE" | jq .
echo ""

# Test 2: Missing required fields
echo "Test 2: Missing required fields"
echo "---"
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"template":"firefox-browser"}')
echo "$RESPONSE" | jq .
echo ""

# Test 3: Invalid resource values
echo "Test 3: Invalid resource values"
echo "---"
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"invalid","cpu":"invalid"},"persistentHome":false}')
echo "$RESPONSE" | jq .
echo ""

# Test 4: Unauthorized access (no token)
echo "Test 4: Unauthorized access (no token)"
echo "---"
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}')
echo "$RESPONSE" | jq .
echo ""

echo "==================================="
echo "  Error Testing Complete"
echo "==================================="
