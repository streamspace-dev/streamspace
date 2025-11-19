#!/bin/bash

# StreamSpace Integration Test Runner
# Usage: ./run-integration-tests.sh [options]
#
# Options:
#   -v          Verbose output
#   -short      Skip long-running tests
#   -cover      Generate coverage report
#   -filter     Run specific test pattern (e.g., -filter TestPlugin)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTS_DIR="$PROJECT_ROOT/tests/integration"
REPORTS_DIR="$PROJECT_ROOT/tests/reports"

# Default options
VERBOSE=""
SHORT=""
COVER=""
FILTER=""
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v)
            VERBOSE="-v"
            shift
            ;;
        -short)
            SHORT="-short"
            shift
            ;;
        -cover)
            COVER="-cover -coverprofile=$REPORTS_DIR/coverage_$TIMESTAMP.out"
            shift
            ;;
        -filter)
            FILTER="-run $2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create reports directory if it doesn't exist
mkdir -p "$REPORTS_DIR"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}StreamSpace Integration Test Runner${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "Timestamp: $TIMESTAMP"
echo "Tests Dir: $TESTS_DIR"
echo "Reports Dir: $REPORTS_DIR"
echo ""

# Check if API is running
echo -e "${YELLOW}Checking API availability...${NC}"
API_URL="${STREAMSPACE_API_URL:-http://localhost:8080}"
if curl -s -o /dev/null -w "%{http_code}" "$API_URL/health" | grep -q "200"; then
    echo -e "${GREEN}API is available at $API_URL${NC}"
else
    echo -e "${RED}Warning: API may not be running at $API_URL${NC}"
    echo "Set STREAMSPACE_API_URL environment variable if using different URL"
fi
echo ""

# Run tests
echo -e "${YELLOW}Running Integration Tests...${NC}"
echo ""

cd "$TESTS_DIR"

# Run with JSON output for parsing
go test $VERBOSE $SHORT $COVER $FILTER \
    -timeout 30m \
    -json \
    ./... 2>&1 | tee "$REPORTS_DIR/test_output_$TIMESTAMP.json" | \
    go tool test2json -p integration | \
    while IFS= read -r line; do
        # Parse JSON and format output
        action=$(echo "$line" | jq -r '.Action // empty')
        package=$(echo "$line" | jq -r '.Package // empty')
        test=$(echo "$line" | jq -r '.Test // empty')
        output=$(echo "$line" | jq -r '.Output // empty')

        if [ "$action" = "pass" ] && [ -n "$test" ]; then
            echo -e "${GREEN}PASS${NC}: $test"
        elif [ "$action" = "fail" ] && [ -n "$test" ]; then
            echo -e "${RED}FAIL${NC}: $test"
        elif [ -n "$output" ]; then
            echo -n "$output"
        fi
    done

# Generate summary
echo ""
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Test Summary${NC}"
echo -e "${YELLOW}========================================${NC}"

# Count results
TOTAL=$(grep -c '"Action":"run"' "$REPORTS_DIR/test_output_$TIMESTAMP.json" 2>/dev/null || echo "0")
PASSED=$(grep -c '"Action":"pass"' "$REPORTS_DIR/test_output_$TIMESTAMP.json" 2>/dev/null || echo "0")
FAILED=$(grep -c '"Action":"fail"' "$REPORTS_DIR/test_output_$TIMESTAMP.json" 2>/dev/null || echo "0")
SKIPPED=$(grep -c '"Action":"skip"' "$REPORTS_DIR/test_output_$TIMESTAMP.json" 2>/dev/null || echo "0")

echo "Total Tests: $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Skipped: ${YELLOW}$SKIPPED${NC}"

if [ "$FAILED" -gt 0 ]; then
    echo ""
    echo -e "${RED}Failed Tests:${NC}"
    grep '"Action":"fail"' "$REPORTS_DIR/test_output_$TIMESTAMP.json" | \
        jq -r '.Test' | sort -u
fi

# Generate coverage report if requested
if [ -n "$COVER" ]; then
    echo ""
    echo -e "${YELLOW}Coverage Report:${NC}"
    go tool cover -func="$REPORTS_DIR/coverage_$TIMESTAMP.out"
fi

echo ""
echo "Full output saved to: $REPORTS_DIR/test_output_$TIMESTAMP.json"

# Exit with failure if any tests failed
if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

exit 0
