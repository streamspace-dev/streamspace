#!/bin/bash
# Wake a hibernated StreamSpace session (scale to 1)

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

echo "Waking session: $SESSION_NAME in namespace: $NAMESPACE"

# Patch the session to running state
kubectl patch session "$SESSION_NAME" -n "$NAMESPACE" \
    --type merge -p '{"spec":{"state":"running"}}'

echo "✓ Session $SESSION_NAME set to running state"
echo ""
echo "Waiting for pod to be ready..."

# Wait for deployment to scale to 1
DEPLOYMENT_NAME=$(kubectl get session "$SESSION_NAME" -n "$NAMESPACE" -o jsonpath='{.status.podName}' 2>/dev/null || echo "")

if [ -n "$DEPLOYMENT_NAME" ]; then
    kubectl wait --for=jsonpath='{.spec.replicas}'=1 \
        deployment/"$DEPLOYMENT_NAME" -n "$NAMESPACE" \
        --timeout=30s 2>/dev/null || true

    echo "Waiting for pod to be ready..."
    kubectl wait --for=condition=ready pod \
        -l session="$SESSION_NAME" -n "$NAMESPACE" \
        --timeout=120s 2>/dev/null || true
    echo "✓ Pod is ready"
else
    echo "⚠ Could not find deployment name in session status"
fi

echo ""
echo "Session $SESSION_NAME is now running"
kubectl get session "$SESSION_NAME" -n "$NAMESPACE"
echo ""

# Get the URL
URL=$(kubectl get session "$SESSION_NAME" -n "$NAMESPACE" -o jsonpath='{.status.url}' 2>/dev/null || echo "")
if [ -n "$URL" ]; then
    echo "Access at: $URL"
fi
