#!/bin/bash
# generate_resource_report.sh - Generate resource usage report for a session
# Usage: ./generate_resource_report.sh <namespace> <session-name>

set -e

NAMESPACE="${1:-streamspace}"
SESSION_NAME="$2"

if [ -z "$SESSION_NAME" ]; then
  echo "ERROR: Missing session name"
  echo "Usage: $0 [namespace] <session-name>"
  exit 1
fi

echo "=== Resource Report for Session: $SESSION_NAME ==="
echo ""

# Get pod name
POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "session=$SESSION_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POD_NAME" ]; then
  echo "ERROR: No pod found for session $SESSION_NAME"
  exit 1
fi

echo "Pod: $POD_NAME"
echo ""

# Get resource requests and limits
echo "--- Resource Requests/Limits ---"
kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o json | jq -r '
  .spec.containers[0].resources |
  "Requests:",
  "  CPU: \(.requests.cpu // "not set")",
  "  Memory: \(.requests.memory // "not set")",
  "Limits:",
  "  CPU: \(.limits.cpu // "not set")",
  "  Memory: \(.limits.memory // "not set")"
'
echo ""

# Get actual resource usage
echo "--- Current Resource Usage ---"
kubectl top pod "$POD_NAME" -n "$NAMESPACE" 2>/dev/null || echo "Note: metrics-server not available"
echo ""

# Get pod events
echo "--- Recent Events ---"
kubectl get events -n "$NAMESPACE" --field-selector involvedObject.name="$POD_NAME" \
  --sort-by='.lastTimestamp' | tail -n 10
echo ""

# Get pod status
echo "--- Pod Status ---"
kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o json | jq -r '
  "Phase: \(.status.phase)",
  "Node: \(.spec.nodeName)",
  "Start Time: \(.status.startTime)",
  "Conditions:",
  (.status.conditions[] | "  \(.type): \(.status) (\(.reason // "N/A"))")
'
echo ""

echo "=== End of Report ==="
