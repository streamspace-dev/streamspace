#!/bin/bash
# List all StreamSpace sessions with details

set -e

NAMESPACE="${NAMESPACE:-streamspace}"

echo "================================================"
echo "StreamSpace Sessions in namespace: $NAMESPACE"
echo "================================================"
echo ""

# Get sessions
kubectl get sessions -n "$NAMESPACE" -o custom-columns=\
NAME:.metadata.name,\
USER:.spec.user,\
TEMPLATE:.spec.template,\
STATE:.spec.state,\
PHASE:.status.phase,\
URL:.status.url,\
AGE:.metadata.creationTimestamp

echo ""
echo "Summary:"
TOTAL=$(kubectl get sessions -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
RUNNING=$(kubectl get sessions -n "$NAMESPACE" -o json 2>/dev/null | jq -r '.items[] | select(.spec.state=="running") | .metadata.name' | wc -l || echo "0")
HIBERNATED=$(kubectl get sessions -n "$NAMESPACE" -o json 2>/dev/null | jq -r '.items[] | select(.spec.state=="hibernated") | .metadata.name' | wc -l || echo "0")

echo "  Total sessions: $TOTAL"
echo "  Running: $RUNNING"
echo "  Hibernated: $HIBERNATED"
echo ""
