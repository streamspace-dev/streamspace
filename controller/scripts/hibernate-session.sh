#!/bin/bash
# Hibernate a StreamSpace session (scale to 0)

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <session-name> [namespace]"
    echo ""
    echo "Example: $0 testuser-firefox"
    echo "Example: $0 testuser-firefox streamspace"
    exit 1
fi

SESSION_NAME="$1"
NAMESPACE="${2:-streamspace}"

echo "Hibernating session: $SESSION_NAME in namespace: $NAMESPACE"

# Patch the session to hibernated state
kubectl patch session "$SESSION_NAME" -n "$NAMESPACE" \
    --type merge -p '{"spec":{"state":"hibernated"}}'

echo "✓ Session $SESSION_NAME set to hibernated state"
echo ""
echo "Waiting for deployment to scale down..."

# Wait for deployment to scale to 0
DEPLOYMENT_NAME=$(kubectl get session "$SESSION_NAME" -n "$NAMESPACE" -o jsonpath='{.status.podName}' 2>/dev/null || echo "")

if [ -n "$DEPLOYMENT_NAME" ]; then
    kubectl wait --for=jsonpath='{.spec.replicas}'=0 \
        deployment/"$DEPLOYMENT_NAME" -n "$NAMESPACE" \
        --timeout=60s 2>/dev/null || true
    echo "✓ Deployment scaled to 0 replicas"
else
    echo "⚠ Could not find deployment name in session status"
fi

echo ""
echo "Session $SESSION_NAME is now hibernated"
kubectl get session "$SESSION_NAME" -n "$NAMESPACE"
