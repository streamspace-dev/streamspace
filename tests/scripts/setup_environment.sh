#!/bin/bash
# setup_environment.sh - Set up local environment for integration testing
# This script verifies prerequisites and deploys StreamSpace to local k3s cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== StreamSpace v2.0-beta.1 Integration Test Environment Setup ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

function check_prerequisite() {
  local cmd="$1"
  local name="$2"

  if command -v "$cmd" &> /dev/null; then
    echo -e "${GREEN}✓${NC} $name found: $(command -v $cmd)"
    return 0
  else
    echo -e "${RED}✗${NC} $name not found"
    return 1
  fi
}

function check_helm_version() {
  local version=$(helm version --short 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")

  if [[ "$version" == "unknown" ]]; then
    echo -e "${RED}✗${NC} Could not determine Helm version"
    return 1
  fi

  # Check if version is 4.0.x (not supported)
  if [[ "$version" =~ ^v4\.0\. ]]; then
    echo -e "${RED}✗${NC} Helm $version is not supported (v4.0.x has known issues)"
    echo "   Please downgrade to v3.x or upgrade to v4.1+"
    return 1
  fi

  echo -e "${GREEN}✓${NC} Helm $version (compatible)"
  return 0
}

echo "Step 1: Checking prerequisites..."
echo ""

PREREQS_OK=true
check_prerequisite "kubectl" "kubectl" || PREREQS_OK=false
check_prerequisite "helm" "Helm" || PREREQS_OK=false
check_prerequisite "docker" "Docker" || PREREQS_OK=false
check_prerequisite "jq" "jq" || PREREQS_OK=false
check_prerequisite "curl" "curl" || PREREQS_OK=false
check_helm_version || PREREQS_OK=false

echo ""

if [ "$PREREQS_OK" != "true" ]; then
  echo -e "${RED}ERROR: Missing prerequisites. Please install missing tools and try again.${NC}"
  exit 1
fi

echo -e "${GREEN}All prerequisites met!${NC}"
echo ""

# Check k3s cluster
echo "Step 2: Verifying Kubernetes cluster..."
echo ""

if ! kubectl cluster-info &> /dev/null; then
  echo -e "${RED}ERROR: Cannot connect to Kubernetes cluster${NC}"
  echo "Please ensure k3s or Docker Desktop Kubernetes is running"
  exit 1
fi

CLUSTER_VERSION=$(kubectl version --short 2>/dev/null | grep "Server Version" || echo "unknown")
echo -e "${GREEN}✓${NC} Cluster connection successful"
echo "  $CLUSTER_VERSION"
echo ""

# Build local images
echo "Step 3: Building local images..."
echo ""
echo "This may take 5-10 minutes depending on your system..."
echo ""

cd "$PROJECT_ROOT"

if [ -f "./scripts/local-build.sh" ]; then
  echo "Running local-build.sh..."
  ./scripts/local-build.sh
  echo -e "${GREEN}✓${NC} Images built successfully"
else
  echo -e "${YELLOW}⚠${NC} local-build.sh not found, attempting manual build..."

  # Build API
  echo "Building streamspace-api..."
  docker build -t streamspace-api:local -f api/Dockerfile .

  # Build K8s Agent
  echo "Building streamspace-k8s-agent..."
  docker build -t streamspace-k8s-agent:local -f agents/k8s-agent/Dockerfile .

  # Build UI
  echo "Building streamspace-ui..."
  docker build -t streamspace-ui:local -f ui/Dockerfile .

  # Import to k3s
  if command -v k3s &> /dev/null; then
    echo "Importing images to k3s..."
    docker save streamspace-api:local | sudo k3s ctr images import -
    docker save streamspace-k8s-agent:local | sudo k3s ctr images import -
    docker save streamspace-ui:local | sudo k3s ctr images import -
  fi

  echo -e "${GREEN}✓${NC} Images built successfully"
fi

echo ""

# Deploy with Helm
echo "Step 4: Deploying StreamSpace..."
echo ""

# Check if already deployed
if helm list -n streamspace 2>/dev/null | grep -q streamspace; then
  echo -e "${YELLOW}StreamSpace already deployed, upgrading...${NC}"
  helm upgrade streamspace ./chart -n streamspace \
    --set api.image.tag=local \
    --set agent.k8s.image.tag=local \
    --set ui.image.tag=local \
    --wait --timeout=5m
else
  echo "Installing StreamSpace..."
  helm install streamspace ./chart -n streamspace --create-namespace \
    --set api.image.tag=local \
    --set agent.k8s.image.tag=local \
    --set ui.image.tag=local \
    --wait --timeout=5m
fi

echo -e "${GREEN}✓${NC} StreamSpace deployed successfully"
echo ""

# Wait for pods to be ready
echo "Step 5: Waiting for pods to be ready..."
echo ""

kubectl wait --for=condition=ready pod -l app=streamspace-api -n streamspace --timeout=120s
kubectl wait --for=condition=ready pod -l app=streamspace-k8s-agent -n streamspace --timeout=120s

echo -e "${GREEN}✓${NC} All pods are ready"
echo ""

# Setup port forwarding
echo "Step 6: Setting up port forwarding..."
echo ""

# Kill any existing port forwards
pkill -f "kubectl port-forward.*streamspace" || true
sleep 2

# Start new port forward in background
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000 &
PF_PID=$!

sleep 3

if ps -p $PF_PID > /dev/null; then
  echo -e "${GREEN}✓${NC} Port forwarding active (PID: $PF_PID)"
  echo "  API accessible at: http://localhost:8000"
else
  echo -e "${YELLOW}⚠${NC} Port forwarding may have failed, please check manually"
fi

echo ""

# Get authentication token
echo "Step 7: Getting authentication token..."
echo ""

# Wait for API to be responsive
RETRIES=0
MAX_RETRIES=30
while [ $RETRIES -lt $MAX_RETRIES ]; do
  if curl -s http://localhost:8000/health &> /dev/null; then
    break
  fi
  echo "  Waiting for API to be ready... ($RETRIES/$MAX_RETRIES)"
  sleep 2
  RETRIES=$((RETRIES + 1))
done

if [ $RETRIES -eq $MAX_RETRIES ]; then
  echo -e "${RED}ERROR: API did not become ready${NC}"
  exit 1
fi

# Attempt login
TOKEN_RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' || echo "{}")

TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.token')

if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
  echo -e "${GREEN}✓${NC} Authentication successful"
  echo ""
  echo "Export this token for use in tests:"
  echo ""
  echo -e "${YELLOW}export TOKEN=\"$TOKEN\"${NC}"
  echo -e "${YELLOW}export API_BASE_URL=\"http://localhost:8000\"${NC}"
  echo ""

  # Save to file for convenience
  cat > "$SCRIPT_DIR/.env" <<EOF
# StreamSpace Integration Test Environment
# Generated: $(date)
export TOKEN="$TOKEN"
export API_BASE_URL="http://localhost:8000"
export NAMESPACE="streamspace"
EOF

  echo "Environment variables saved to: $SCRIPT_DIR/.env"
  echo "Source this file before running tests: source $SCRIPT_DIR/.env"
  echo ""
else
  echo -e "${YELLOW}⚠${NC} Could not authenticate automatically"
  echo "You may need to manually obtain a token"
  echo ""
fi

echo "=== Environment Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Source the environment file: source $SCRIPT_DIR/.env"
echo "2. Verify setup: ./verify_environment.sh"
echo "3. Run tests: cd phase1 && ./test_1.1a_basic_session_creation.sh"
echo ""
echo "To tear down: helm uninstall streamspace -n streamspace"
echo ""
