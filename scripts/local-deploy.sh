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

    for image in "streamspace/streamspace-kubernetes-controller" "streamspace/streamspace-api" "streamspace/streamspace-ui"; do
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

    # Workaround for Helm v3.19.0: Package chart first, then install from .tgz
    # This avoids the directory loading bug in v3.19.0
    local helm_version=$(helm version --short 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
    local use_package_workaround=false

    if [[ "${helm_version}" == "v3.19."* ]] || [[ "${FORCE_PACKAGE:-false}" == "true" ]]; then
        log_warning "Detected Helm ${helm_version} - using package workaround for chart loading bug"
        use_package_workaround=true
    fi

    # Try validation only if not using package workaround
    if [ "${use_package_workaround}" = false ] && [ "${SKIP_LINT:-false}" != "true" ]; then
        log_info "Validating chart with helm lint..."
        if helm lint "${CHART_PATH}" 2>&1 | tee /tmp/helm-lint.log; then
            log_success "Chart validation passed"
        else
            log_warning "Helm lint reported errors (this may be a Helm v3.19.0 issue)"
            log_info "Will use package workaround for installation"
            use_package_workaround=true
        fi
    fi

    # Prepare chart for installation
    local chart_ref="${CHART_PATH}"
    local temp_dir=""

    if [ "${use_package_workaround}" = true ]; then
        log_info "Packaging chart to work around Helm v3.19.0 directory loading bug..."
        temp_dir=$(mktemp -d)

        if helm package "${CHART_PATH}" -d "${temp_dir}" 2>&1 | tee /tmp/helm-package.log; then
            # Find the packaged chart file
            local chart_package=$(find "${temp_dir}" -name "streamspace-*.tgz" | head -1)
            if [ -n "${chart_package}" ]; then
                chart_ref="${chart_package}"
                log_success "Chart packaged successfully: $(basename ${chart_package})"
            else
                log_error "Chart packaging failed - package file not found"
                log_info "Package output:"
                cat /tmp/helm-package.log
                rm -rf "${temp_dir}"
                exit 1
            fi
        else
            log_error "Chart packaging failed"
            log_info "This is a critical Helm v3.19.0 bug. Please downgrade Helm to v3.18.0 or earlier."
            log_info "See docs/DEPLOYMENT_TROUBLESHOOTING.md for detailed instructions."
            rm -rf "${temp_dir}"
            exit 1
        fi
    fi

    # Check if release exists
    if helm status "${RELEASE_NAME}" -n "${NAMESPACE}" &> /dev/null; then
        log_info "Release exists, upgrading..."
        log_info "Running: helm upgrade ${RELEASE_NAME} ${chart_ref}"
        helm upgrade "${RELEASE_NAME}" "${chart_ref}" \
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
        log_info "Running: helm install ${RELEASE_NAME} ${chart_ref}"
        helm install "${RELEASE_NAME}" "${chart_ref}" \
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

    # Clean up temporary directory if we created one
    if [ -n "${temp_dir}" ] && [ -d "${temp_dir}" ]; then
        rm -rf "${temp_dir}"
        log_info "Cleaned up temporary package directory"
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

# Start port forwards
start_port_forwards() {
    if [ "${AUTO_PORT_FORWARD:-true}" = "true" ]; then
        echo ""
        log "Starting port forwards automatically..."

        if [ -f "${PROJECT_ROOT}/scripts/local-port-forward.sh" ]; then
            "${PROJECT_ROOT}/scripts/local-port-forward.sh"
            return 0
        else
            log_warning "Port forward script not found, skipping"
            show_access_info
        fi
    else
        show_access_info
    fi
}

# Show access instructions
show_access_info() {
    echo ""
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Access Instructions${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""

    log_info "Start automatic port forwards:"
    echo "  ./scripts/local-port-forward.sh"
    echo ""

    log_info "Or manually port-forward (in separate terminals):"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-ui 3000:80"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-api 8000:8000"
    echo ""

    log_info "View logs:"
    echo "  Controller: kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=controller -f"
    echo "  API:        kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=api -f"
    echo "  UI:         kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=ui -f"
    echo ""

    log_info "When finished testing:"
    echo "  ./scripts/local-stop-port-forward.sh  # Stop port forwards"
    echo "  ./scripts/local-teardown.sh           # Full teardown"
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
    start_port_forwards

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Deployment complete!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
}

# Run main function
main "$@"
