#!/bin/bash
# verify_environment.sh - Verify the test environment is correctly set up

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== StreamSpace Environment Verification ==="
echo ""

CHECKS_PASSED=0
CHECKS_FAILED=0

function run_check() {
  local name="$1"
  local command="$2"

  echo -n "Checking $name... "

  if eval "$command" &> /dev/null; then
    echo -e "${GREEN}✓${NC}"
    CHECKS_PASSED=$((CHECKS_PASSED + 1))
    return 0
  else
    echo -e "${RED}✗${NC}"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
    return 1
  fi
}

# Check environment variables
echo "--- Environment Variables ---"
run_check "TOKEN variable" "[ -n \"\$TOKEN\" ]"
run_check "API_BASE_URL variable" "[ -n \"\$API_BASE_URL\" ]"
echo ""

# Check Kubernetes
echo "--- Kubernetes Cluster ---"
run_check "kubectl connection" "kubectl cluster-info"
run_check "streamspace namespace" "kubectl get namespace streamspace"
echo ""

# Check pods
echo "--- StreamSpace Pods ---"
run_check "API pod" "kubectl get pods -n streamspace -l app=streamspace-api -o jsonpath='{.items[0].status.phase}' | grep -q Running"
run_check "K8s Agent pod" "kubectl get pods -n streamspace -l app=streamspace-k8s-agent -o jsonpath='{.items[0].status.phase}' | grep -q Running"
run_check "PostgreSQL pod" "kubectl get pods -n streamspace -l app=postgres -o jsonpath='{.items[0].status.phase}' | grep -q Running"
echo ""

# Check API connectivity
echo "--- API Connectivity ---"
API_URL="${API_BASE_URL:-http://localhost:8000}"
run_check "API health endpoint" "curl -s $API_URL/health | grep -q ok || curl -s $API_URL/health | grep -q healthy"
run_check "API authentication" "curl -s -H \"Authorization: Bearer \$TOKEN\" $API_URL/api/v1/sessions | jq -e . > /dev/null"
echo ""

# Check CRDs
echo "--- Custom Resource Definitions ---"
run_check "Session CRD" "kubectl get crd sessions.stream.space"
run_check "Template CRD" "kubectl get crd templates.stream.space"
echo ""

# Summary
echo "=== Verification Summary ==="
echo ""
echo "Checks passed: $CHECKS_PASSED"
echo "Checks failed: $CHECKS_FAILED"
echo ""

if [ $CHECKS_FAILED -eq 0 ]; then
  echo -e "${GREEN}✓ Environment is ready for testing!${NC}"
  echo ""
  echo "You can now run integration tests:"
  echo "  cd $SCRIPT_DIR/phase1"
  echo "  ./test_1.1a_basic_session_creation.sh"
  exit 0
else
  echo -e "${RED}✗ Environment has issues that need to be resolved${NC}"
  echo ""
  echo "Troubleshooting steps:"
  echo "1. Ensure you've run: source $SCRIPT_DIR/.env"
  echo "2. Check pod logs: kubectl logs -n streamspace -l app=streamspace-api"
  echo "3. Re-run setup: ./setup_environment.sh"
  exit 1
fi
