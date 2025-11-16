#!/usr/bin/env bash
#
# local-deploy.sh - Deploy StreamSpace to local Kubernetes cluster
#
# This script deploys StreamSpace to a local Kubernetes cluster (e.g., Docker Desktop K8s).
# It uses the locally built images and configures the environment for testing.
#

set -euo pipefail

# Colors for output
COLOR_RESET='\033[0m'
COLOR_BOLD='\033[1m'
COLOR_GREEN='\033[32m'
COLOR_YELLOW='\033[33m'
COLOR_BLUE='\033[34m'
COLOR_RED='\033[31m'

# Project configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAMESPACE="${NAMESPACE:-streamspace}"
RELEASE_NAME="${RELEASE_NAME:-streamspace}"
VERSION="${VERSION:-local}"

# Helm chart location - use absolute path
CHART_PATH="$(cd "${PROJECT_ROOT}/chart" && pwd)"

# Helper functions
log() {
    echo -e "${COLOR_BOLD}==>${COLOR_RESET} $*"
}

log_success() {
    echo -e "${COLOR_GREEN}✓${COLOR_RESET} $*"
}

log_error() {
    echo -e "${COLOR_RED}✗${COLOR_RESET} $*" >&2
}

log_info() {
    echo -e "${COLOR_BLUE}→${COLOR_RESET} $*"
}

log_warning() {
    echo -e "${COLOR_YELLOW}⚠${COLOR_RESET} $*"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi

    if ! command -v helm &> /dev/null; then
        log_error "Helm is not installed or not in PATH"
        exit 1
    fi

    # Check Helm version
    local helm_version=$(helm version --short 2>/dev/null || echo "unknown")
    log_info "Helm version: ${helm_version}"

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        log_info "Make sure your kubeconfig is properly configured"
        exit 1
    fi

    local context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    log_success "Connected to cluster: ${context}"
}

# Check if images exist locally
check_images() {
    log "Checking for locally built images..."

    local missing_images=0

    for image in "streamspace/streamspace-controller" "streamspace/streamspace-api" "streamspace/streamspace-ui"; do
        if docker images "${image}:${VERSION}" --format "{{.Repository}}:{{.Tag}}" | grep -q "${image}:${VERSION}"; then
            log_success "Found ${image}:${VERSION}"
        else
            log_error "Missing ${image}:${VERSION}"
            missing_images=$((missing_images + 1))
        fi
    done

    if [ $missing_images -gt 0 ]; then
        log_error "Missing ${missing_images} image(s). Run './scripts/local-build.sh' first."
        exit 1
    fi
}

# Create namespace
create_namespace() {
    log "Creating namespace: ${NAMESPACE}"

    if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        log_warning "Namespace ${NAMESPACE} already exists"
    else
        kubectl create namespace "${NAMESPACE}"
        log_success "Namespace created"
    fi
}

# Apply CRDs manually (before Helm install)
apply_crds() {
    log "Applying Custom Resource Definitions..."

    kubectl apply -f "${CHART_PATH}/crds/" 2>/dev/null || {
        log_warning "CRD directory not found in chart, will rely on Helm CRD installation"
        return 0
    }

    log_success "CRDs applied"
}

# Install/Upgrade with Helm
deploy_helm() {
    log "Deploying StreamSpace with Helm..."

    # Debug output
    log_info "Chart path: ${CHART_PATH}"
    log_info "Chart.yaml exists: $(test -f "${CHART_PATH}/Chart.yaml" && echo "YES" || echo "NO")"
    log_info "Chart directory contents:"
    ls -la "${CHART_PATH}/" 2>&1 | head -10

    # Validate chart with helm lint (make it non-fatal as helm v3.19.0 has known issues)
    log_info "Validating chart with helm lint..."
    if helm lint "${CHART_PATH}" 2>&1 | tee /tmp/helm-lint.log; then
        log_success "Chart validation passed"
    else
        log_warning "Helm lint reported errors (this may be a Helm v3.19.0 issue)"
        log_info "Attempting alternative validation with 'helm template'..."
        if helm template test-release "${CHART_PATH}" \
            --set controller.image.tag="${VERSION}" \
            --set api.image.tag="${VERSION}" \
            --set ui.image.tag="${VERSION}" > /dev/null 2>&1; then
            log_success "Chart templates are valid - proceeding with deployment"
        else
            log_error "Chart validation failed with both 'helm lint' and 'helm template'"
            log_info "If using Helm v3.19.0, try downgrading to v3.18.0 or earlier"
            log_info "Or skip validation with: SKIP_LINT=true ./scripts/local-deploy.sh"
            if [ "${SKIP_LINT:-false}" != "true" ]; then
                exit 1
            fi
            log_warning "Skipping validation due to SKIP_LINT=true"
        fi
    fi

    # Test if Helm can package the chart
    log_info "Testing chart packaging..."
    local temp_dir=$(mktemp -d)
    if helm package "${CHART_PATH}" -d "${temp_dir}" &> /dev/null; then
        log_success "Chart packaging test passed"
        rm -rf "${temp_dir}"
    else
        log_warning "Chart packaging test failed (may not be critical)"
        rm -rf "${temp_dir}"
    fi

    # Check if release exists
    if helm status "${RELEASE_NAME}" -n "${NAMESPACE}" &> /dev/null; then
        log_info "Release exists, upgrading..."
        helm upgrade "${RELEASE_NAME}" "${CHART_PATH}" \
            --namespace "${NAMESPACE}" \
            --set controller.image.tag="${VERSION}" \
            --set controller.image.pullPolicy=Never \
            --set api.image.tag="${VERSION}" \
            --set api.image.pullPolicy=Never \
            --set ui.image.tag="${VERSION}" \
            --set ui.image.pullPolicy=Never \
            --set postgresql.enabled=true \
            --set postgresql.auth.password=streamspace \
            --wait \
            --timeout 5m
    else
        log_info "Installing fresh release..."
        log_info "Running: helm install ${RELEASE_NAME} ${CHART_PATH}"
        helm install "${RELEASE_NAME}" "${CHART_PATH}" \
            --namespace "${NAMESPACE}" \
            --create-namespace \
            --set controller.image.tag="${VERSION}" \
            --set controller.image.pullPolicy=Never \
            --set api.image.tag="${VERSION}" \
            --set api.image.pullPolicy=Never \
            --set ui.image.tag="${VERSION}" \
            --set ui.image.pullPolicy=Never \
            --set postgresql.enabled=true \
            --set postgresql.auth.password=streamspace \
            --debug \
            --wait \
            --timeout 5m
    fi

    log_success "Helm deployment complete"
}

# Wait for pods to be ready
wait_for_pods() {
    log "Waiting for pods to be ready..."

    local timeout=300  # 5 minutes
    local elapsed=0
    local interval=5

    while [ $elapsed -lt $timeout ]; do
        local ready_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | \
            jq -r '.items[] | select(.status.phase == "Running") | .metadata.name' | wc -l)
        local total_pods=$(kubectl get pods -n "${NAMESPACE}" --no-headers | wc -l)

        if [ "$ready_pods" -eq "$total_pods" ] && [ "$total_pods" -gt 0 ]; then
            log_success "All pods are ready"
            return 0
        fi

        log_info "Waiting... (${ready_pods}/${total_pods} pods ready)"
        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_warning "Timeout waiting for pods to be ready"
    log_info "Check pod status with: kubectl get pods -n ${NAMESPACE}"
}

# Show deployment status
show_status() {
    log "Deployment Status:"
    echo ""

    log_info "Pods:"
    kubectl get pods -n "${NAMESPACE}" -o wide
    echo ""

    log_info "Services:"
    kubectl get svc -n "${NAMESPACE}"
    echo ""

    log_info "Helm Release:"
    helm status "${RELEASE_NAME}" -n "${NAMESPACE}"
}

# Show access instructions
show_access_info() {
    echo ""
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Access Instructions${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""

    log_info "Port-forward UI (in a separate terminal):"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-ui 3000:80"
    echo "  Then access: http://localhost:3000"
    echo ""

    log_info "Port-forward API (in a separate terminal):"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-api 8000:8000"
    echo "  Then access: http://localhost:8000"
    echo ""

    log_info "View logs:"
    echo "  Controller: kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=controller -f"
    echo "  API:        kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=api -f"
    echo "  UI:         kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=ui -f"
    echo ""

    log_info "When finished testing:"
    echo "  ./scripts/local-teardown.sh"
    echo ""
}

# Main execution
main() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  StreamSpace Local Deployment${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_BLUE}Namespace:${COLOR_RESET}     ${NAMESPACE}"
    echo -e "${COLOR_BLUE}Release:${COLOR_RESET}       ${RELEASE_NAME}"
    echo -e "${COLOR_BLUE}Version:${COLOR_RESET}       ${VERSION}"
    echo ""

    check_prerequisites
    check_images
    create_namespace
    apply_crds
    deploy_helm
    wait_for_pods
    show_status
    show_access_info

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Deployment complete!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
}

# Run main function
main "$@"
