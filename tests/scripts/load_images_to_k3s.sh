#!/bin/bash
set -e

echo "════════════════════════════════════════════════════════════"
echo "  Loading Local Docker Images into k3s"
echo "════════════════════════════════════════════════════════════"
echo ""

IMAGES=(
  "streamspace/streamspace-api:local"
  "streamspace/streamspace-ui:local"
  "streamspace/streamspace-k8s-agent:local"
)

for IMAGE in "${IMAGES[@]}"; do
  echo "→ Loading $IMAGE..."
  docker save "$IMAGE" | sudo k3s ctr images import -
  if [ $? -eq 0 ]; then
    echo "✓ Successfully loaded $IMAGE"
  else
    echo "✗ Failed to load $IMAGE"
    exit 1
  fi
  echo ""
done

echo "════════════════════════════════════════════════════════════"
echo "✓ All images loaded into k3s successfully!"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Next steps:"
echo "1. Run: cd /Users/s0v3r1gn/streamspace/streamspace-validator"
echo "2. Run: ./scripts/local-deploy.sh"
echo "3. Wait for pods to restart with new images"
echo ""
