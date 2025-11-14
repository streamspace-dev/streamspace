#!/bin/bash
# Create a new StreamSpace session from a template

set -e

if [ $# -lt 3 ]; then
    echo "Usage: $0 <username> <template-name> <session-name> [namespace]"
    echo ""
    echo "Example: $0 alice firefox-browser alice-firefox"
    echo "Example: $0 bob vscode bob-vscode streamspace"
    echo ""
    echo "Available templates:"
    kubectl get templates -n "${4:-streamspace}" --no-headers 2>/dev/null | awk '{print "  - " $1}' || echo "  (none found)"
    exit 1
fi

USERNAME="$1"
TEMPLATE="$2"
SESSION_NAME="$3"
NAMESPACE="${4:-streamspace}"

echo "Creating session..."
echo "  User: $USERNAME"
echo "  Template: $TEMPLATE"
echo "  Session name: $SESSION_NAME"
echo "  Namespace: $NAMESPACE"
echo ""

# Check if template exists
if ! kubectl get template "$TEMPLATE" -n "$NAMESPACE" &>/dev/null; then
    echo "❌ Error: Template '$TEMPLATE' not found in namespace '$NAMESPACE'"
    echo ""
    echo "Available templates:"
    kubectl get templates -n "$NAMESPACE" --no-headers | awk '{print "  - " $1}'
    exit 1
fi

# Create the session
cat <<EOF | kubectl apply -f -
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: $SESSION_NAME
  namespace: $NAMESPACE
spec:
  user: $USERNAME
  template: $TEMPLATE
  state: running
  persistentHome: true
  idleTimeout: 30m
  maxSessionDuration: 8h
EOF

echo ""
echo "✓ Session created"
echo ""
echo "Waiting for session to be ready..."

# Wait for session to be running
sleep 2
for i in {1..30}; do
    PHASE=$(kubectl get session "$SESSION_NAME" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$PHASE" == "Running" ]; then
        echo "✓ Session is running"
        break
    fi
    echo "  Status: $PHASE (waiting...)"
    sleep 2
done

echo ""
kubectl get session "$SESSION_NAME" -n "$NAMESPACE"
echo ""

# Get the URL
URL=$(kubectl get session "$SESSION_NAME" -n "$NAMESPACE" -o jsonpath='{.status.url}' 2>/dev/null || echo "")
if [ -n "$URL" ]; then
    echo "🌐 Access your session at: $URL"
fi

echo ""
echo "📋 Quick commands:"
echo "  View logs:     kubectl logs -n $NAMESPACE -l session=$SESSION_NAME"
echo "  Hibernate:     ./scripts/hibernate-session.sh $SESSION_NAME"
echo "  Wake:          ./scripts/wake-session.sh $SESSION_NAME"
echo "  Delete:        kubectl delete session $SESSION_NAME -n $NAMESPACE"
