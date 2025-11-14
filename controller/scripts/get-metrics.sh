#!/bin/bash
# Get StreamSpace Prometheus metrics

set -e

NAMESPACE="${NAMESPACE:-streamspace}"
SERVICE="${SERVICE:-streamspace-controller-metrics}"

echo "================================================"
echo "StreamSpace Metrics"
echo "================================================"
echo ""

# Port forward in background
echo "Setting up port forward to metrics service..."
kubectl port-forward -n "$NAMESPACE" "svc/$SERVICE" 8080:8080 >/dev/null 2>&1 &
PF_PID=$!

# Cleanup on exit
trap "kill $PF_PID 2>/dev/null || true" EXIT

# Wait for port forward
sleep 2

echo "Fetching metrics from http://localhost:8080/metrics"
echo ""

# Get custom metrics
echo "=== Session Metrics ==="
curl -s http://localhost:8080/metrics 2>/dev/null | grep "^streamspace_sessions" || echo "(no session metrics yet)"
echo ""

echo "=== Reconciliation Metrics ==="
curl -s http://localhost:8080/metrics 2>/dev/null | grep "^streamspace_session_reconciliation" || echo "(no reconciliation metrics yet)"
echo ""

echo "=== Template Metrics ==="
curl -s http://localhost:8080/metrics 2>/dev/null | grep "^streamspace_template" || echo "(no template metrics yet)"
echo ""

echo "================================================"
echo "Full metrics available at: http://localhost:8080/metrics"
echo "Keep this script running to maintain port forward"
echo "Press Ctrl+C to exit"
echo "================================================"
echo ""

# Keep port forward alive
wait $PF_PID
