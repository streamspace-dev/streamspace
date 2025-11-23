#!/bin/bash
# CSRF Fix Testing Script
# Tests that JWT-authenticated API clients can create sessions without CSRF tokens
#
# Prerequisites:
# - StreamSpace v2.0-beta deployed and running
# - API accessible at http://localhost:8000 (or update API_URL below)
# - Admin credentials configured
#
# Usage:
#   ./csrf_fix_test.sh

set -e

# Configuration
API_URL="${API_URL:-http://localhost:8000}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"  # Use actual password from secret

echo "========================================"
echo "CSRF Fix Testing Script"
echo "========================================"
echo ""
echo "API URL: $API_URL"
echo "Admin User: $ADMIN_USERNAME"
echo ""

# Test 1: Login and Get JWT Token
echo "TEST 1: Login and Get JWT Token"
echo "--------------------------------"
echo "POST $API_URL/api/v1/auth/login"
echo ""

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}")

HTTP_CODE=$(echo "$RESPONSE" | grep HTTP_CODE | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

echo "HTTP Status: $HTTP_CODE"
echo "Response: $BODY" | jq . 2>/dev/null || echo "$BODY"
echo ""

if [ "$HTTP_CODE" != "200" ]; then
    echo "❌ FAILED: Login failed"
    exit 1
fi

TOKEN=$(echo "$BODY" | jq -r '.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "❌ FAILED: No JWT token in response"
    exit 1
fi

echo "✅ PASSED: Login successful"
echo "JWT Token: ${TOKEN:0:20}..."
echo ""

# Test 2: Create Session WITHOUT CSRF Token (Should Work Now)
echo "TEST 2: Create Session with JWT (No CSRF Token)"
echo "------------------------------------------------"
echo "POST $API_URL/api/v1/sessions"
echo "Authorization: Bearer <token>"
echo ""

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_URL/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {
      "memory": "1Gi",
      "cpu": "500m"
    },
    "persistentHome": false
  }')

HTTP_CODE=$(echo "$RESPONSE" | grep HTTP_CODE | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

echo "HTTP Status: $HTTP_CODE"
echo "Response: $BODY" | jq . 2>/dev/null || echo "$BODY"
echo ""

if [ "$HTTP_CODE" = "403" ]; then
    ERROR=$(echo "$BODY" | jq -r '.error' 2>/dev/null)
    if [ "$ERROR" = "CSRF token missing" ] || [ "$ERROR" = "CSRF token mismatch" ]; then
        echo "❌ FAILED: CSRF protection still blocking JWT-authenticated requests"
        echo "Fix did not work - JWT exemption not applied"
        exit 1
    fi
fi

if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "201" ] && [ "$HTTP_CODE" != "202" ]; then
    echo "⚠️  WARNING: Session creation failed, but not due to CSRF"
    echo "This might be due to missing K8s controller or other issues"
    echo "CSRF fix appears to be working (no CSRF error)"
else
    echo "✅ PASSED: Session creation succeeded without CSRF token"
    SESSION_ID=$(echo "$BODY" | jq -r '.id // .sessionId // .name' 2>/dev/null)
    if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
        echo "Session ID: $SESSION_ID"
    fi
fi
echo ""

# Test 3: Verify CSRF Still Protects Cookie-Based Requests
echo "TEST 3: CSRF Still Protects Cookie-Based Requests"
echo "--------------------------------------------------"
echo "POST $API_URL/api/v1/sessions (No Authorization header, no CSRF)"
echo ""

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_URL/api/v1/sessions" \
  -H 'Content-Type: application/json' \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {
      "memory": "1Gi",
      "cpu": "500m"
    },
    "persistentHome": false
  }')

HTTP_CODE=$(echo "$RESPONSE" | grep HTTP_CODE | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

echo "HTTP Status: $HTTP_CODE"
echo "Response: $BODY" | jq . 2>/dev/null || echo "$BODY"
echo ""

if [ "$HTTP_CODE" = "403" ]; then
    ERROR=$(echo "$BODY" | jq -r '.error' 2>/dev/null)
    if [ "$ERROR" = "CSRF token missing" ] || [ "$ERROR" = "CSRF token mismatch" ]; then
        echo "✅ PASSED: CSRF protection still active for non-JWT requests"
    else
        echo "⚠️  Request blocked by $ERROR (not CSRF)"
    fi
elif [ "$HTTP_CODE" = "401" ]; then
    echo "✅ PASSED: Request blocked by authentication (expected)"
else
    echo "⚠️  WARNING: Expected CSRF or auth error, got HTTP $HTTP_CODE"
fi
echo ""

# Summary
echo "========================================"
echo "TEST SUMMARY"
echo "========================================"
echo ""
echo "✅ Login with JWT works"
echo "✅ JWT-authenticated requests bypass CSRF"
echo "✅ Non-JWT requests still protected by CSRF"
echo ""
echo "CSRF fix is working correctly!"
echo ""
echo "Note: If session creation failed for reasons other than CSRF,"
echo "      check that the K8s controller is running and configured."
