#!/bin/bash
echo "Waiting for session to reach running state..."
for i in {1..12}; do
  sleep 5
  STATE=$(kubectl get session admin-firefox-browser-52bfac7e -n streamspace -o jsonpath='{.spec.state}' 2>/dev/null || echo "not found")
  echo "[$i/12] Session state: $STATE"
  if [ "$STATE" = "running" ]; then
    echo "✓ Session is running!"
    break
  fi
done

echo ""
kubectl get pods -n streamspace | grep admin-firefox-browser-52bfac7e || echo "Pod not found yet"
