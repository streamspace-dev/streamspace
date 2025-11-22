#!/bin/bash

TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

echo "Creating test session..."
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "resources": {
      "memory": "512Mi",
      "cpu": "250m"
    },
    "persistentHome": false
  }')

echo "Full API response:"
echo "$RESPONSE" | jq '.'
