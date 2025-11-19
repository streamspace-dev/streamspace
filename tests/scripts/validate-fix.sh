#!/bin/bash

# Quick Fix Validator
# Usage: ./validate-fix.sh <fix-name>
#
# This script runs targeted tests for specific Builder fixes.
# Use this for rapid validation during development.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTS_DIR="$PROJECT_ROOT/tests/integration"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

FIX_NAME=${1:-""}

if [ -z "$FIX_NAME" ]; then
    echo -e "${CYAN}StreamSpace Fix Validator${NC}"
    echo ""
    echo "Usage: $0 <fix-name>"
    echo ""
    echo "Available fixes to validate:"
    echo ""
    echo -e "${YELLOW}CRITICAL Priority:${NC}"
    echo "  session-name      - Session Name/ID Mismatch (TC-CORE-001)"
    echo "  template-name     - Template Name in Sessions (TC-CORE-002)"
    echo "  vnc-url           - VNC URL Empty (TC-CORE-004)"
    echo "  heartbeat         - Heartbeat Validation (TC-CORE-005)"
    echo "  plugin-runtime    - Plugin Runtime Loading (TC-002)"
    echo "  webhook-secret    - Webhook Secret Panic (TC-SEC-011)"
    echo ""
    echo -e "${YELLOW}HIGH Priority:${NC}"
    echo "  plugin-enable     - Plugin Enable/Config (TC-003, TC-005)"
    echo "  saml-redirect     - SAML Return URL (TC-SEC-001)"
    echo ""
    echo -e "${YELLOW}MEDIUM Priority:${NC}"
    echo "  batch-errors      - Batch Operations Errors (TC-INT-001-004)"
    echo ""
    echo -e "${YELLOW}ALL:${NC}"
    echo "  all               - Run all integration tests"
    echo "  core              - All Core Platform tests"
    echo "  security          - All Security tests"
    echo "  plugin            - All Plugin System tests"
    echo "  batch             - All Batch Operations tests"
    echo ""
    exit 0
fi

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}Validating Fix: $FIX_NAME${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

cd "$TESTS_DIR"

case $FIX_NAME in
    # CRITICAL fixes
    session-name)
        echo "Running: TestSessionNameInAPIResponse"
        go test -v -run TestSessionNameInAPIResponse -timeout 5m ./...
        ;;
    template-name)
        echo "Running: TestTemplateNameUsedInSessionCreation"
        go test -v -run TestTemplateNameUsedInSessionCreation -timeout 5m ./...
        ;;
    vnc-url)
        echo "Running: TestVNCURLAvailableOnConnection"
        go test -v -run TestVNCURLAvailableOnConnection -timeout 5m ./...
        ;;
    heartbeat)
        echo "Running: TestHeartbeatValidatesConnection"
        go test -v -run TestHeartbeatValidatesConnection -timeout 5m ./...
        ;;
    plugin-runtime)
        echo "Running: TestPluginRuntimeLoading"
        go test -v -run TestPluginRuntimeLoading -timeout 5m ./...
        ;;
    webhook-secret)
        echo "Running: TestWebhookSecretGeneration"
        go test -v -run TestWebhookSecretGeneration -timeout 5m ./...
        ;;

    # HIGH priority fixes
    plugin-enable)
        echo "Running: TestPluginEnable, TestPluginConfigUpdate"
        go test -v -run "TestPluginEnable|TestPluginConfigUpdate" -timeout 5m ./...
        ;;
    saml-redirect)
        echo "Running: TestSAMLReturnURLValidation"
        go test -v -run TestSAMLReturnURLValidation -timeout 5m ./...
        ;;

    # MEDIUM priority fixes
    batch-errors)
        echo "Running: All Batch Operations tests"
        go test -v -run "TestBatch" -timeout 10m ./...
        ;;

    # Category runs
    all)
        echo "Running: ALL integration tests"
        go test -v -timeout 30m ./...
        ;;
    core)
        echo "Running: Core Platform tests"
        go test -v -run "TestSession|TestTemplate|TestVNC|TestHeartbeat" -timeout 10m ./...
        ;;
    security)
        echo "Running: Security tests"
        go test -v -run "TestSAML|TestCSRF|TestDemo|TestWebhook|TestSQL|TestXSS" -timeout 10m ./...
        ;;
    plugin)
        echo "Running: Plugin System tests"
        go test -v -run "TestPlugin" -timeout 15m ./...
        ;;
    batch)
        echo "Running: Batch Operations tests"
        go test -v -run "TestBatch" -timeout 10m ./...
        ;;

    *)
        echo -e "${RED}Unknown fix: $FIX_NAME${NC}"
        echo "Run '$0' without arguments to see available options."
        exit 1
        ;;
esac

TEST_EXIT=$?

echo ""
if [ $TEST_EXIT -eq 0 ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Fix Validation: PASSED${NC}"
    echo -e "${GREEN}========================================${NC}"
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}Fix Validation: FAILED${NC}"
    echo -e "${RED}========================================${NC}"
fi

exit $TEST_EXIT
