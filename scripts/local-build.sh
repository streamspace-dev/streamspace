#!/usr/bin/env bash
#
# local-build.sh - Build all StreamSpace Docker images locally
#
# This script builds the controller, API, and UI Docker images for local testing.
# Images are tagged with 'local' and 'latest' tags for easy identification.
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
VERSION="${VERSION:-local}"
GIT_COMMIT="${GIT_COMMIT:-$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# Image names (matching Helm chart expectations)
KUBERNETES_CONTROLLER_IMAGE="streamspace/streamspace-kubernetes-controller"
API_IMAGE="streamspace/streamspace-api"
UI_IMAGE="streamspace/streamspace-ui"
K8S_AGENT_IMAGE="streamspace/streamspace-k8s-agent"
DOCKER_CONTROLLER_IMAGE="streamspace/streamspace-docker-controller"

# GHCR image names (for local K8s deployment compatibility)
GHCR_API_IMAGE="ghcr.io/streamspace-dev/streamspace-api"
GHCR_UI_IMAGE="ghcr.io/streamspace-dev/streamspace-ui"
GHCR_K8S_AGENT_IMAGE="ghcr.io/streamspace-dev/streamspace-k8s-agent"

# Build arguments
BUILD_ARGS="--build-arg VERSION=${VERSION} --build-arg COMMIT=${GIT_COMMIT} --build-arg BUILD_DATE=${BUILD_DATE}"

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

    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi

    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running"
        exit 1
    fi

    log_success "Docker is available and running"
}

# Kubernetes controller removed in v2.0 (replaced by k8s-agent)
# Agent-based architecture replaces controller-based CRD approach

# Build API image
build_api() {
    log "Building API image..."
    log_info "Image: ${API_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/api"

    docker build ${BUILD_ARGS} \
        -t "${API_IMAGE}:${VERSION}" \
        -t "${API_IMAGE}:latest" \
        -t "${GHCR_API_IMAGE}:${VERSION}" \
        -t "${GHCR_API_IMAGE}:latest" \
        -f "${PROJECT_ROOT}/api/Dockerfile" \
        "${PROJECT_ROOT}/api/"

    log_success "API image built successfully"
}

# Build UI image
build_ui() {
    log "Building UI image..."
    log_info "Image: ${UI_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/ui"

    docker build ${BUILD_ARGS} \
        -t "${UI_IMAGE}:${VERSION}" \
        -t "${UI_IMAGE}:latest" \
        -t "${GHCR_UI_IMAGE}:${VERSION}" \
        -t "${GHCR_UI_IMAGE}:latest" \
        -f "${PROJECT_ROOT}/ui/Dockerfile" \
        "${PROJECT_ROOT}/ui/"

    log_success "UI image built successfully"
}

# Build K8s Agent image (v2.0)
build_k8s_agent() {
    log "Building K8s Agent image (v2.0)..."
    log_info "Image: ${K8S_AGENT_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/agents/k8s-agent"

    # Check if k8s-agent directory exists
    if [ ! -d "${PROJECT_ROOT}/agents/k8s-agent" ]; then
        log_warning "K8s Agent directory not found, skipping"
        return 0
    fi

    docker build ${BUILD_ARGS} \
        -t "${K8S_AGENT_IMAGE}:${VERSION}" \
        -t "${K8S_AGENT_IMAGE}:latest" \
        -t "${GHCR_K8S_AGENT_IMAGE}:${VERSION}" \
        -t "${GHCR_K8S_AGENT_IMAGE}:latest" \
        -f "${PROJECT_ROOT}/agents/k8s-agent/Dockerfile" \
        "${PROJECT_ROOT}/agents/k8s-agent/"

    log_success "K8s Agent image built successfully"
}

# Build Docker controller image
build_docker_controller() {
    log "Building Docker controller image..."
    log_info "Image: ${DOCKER_CONTROLLER_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/docker-controller"

    # Check if docker-controller directory exists
    if [ ! -d "${PROJECT_ROOT}/docker-controller" ]; then
        log_warning "Docker controller directory not found, skipping (deferred to v2.1)"
        return 0
    fi

    docker build ${BUILD_ARGS} \
        -t "${DOCKER_CONTROLLER_IMAGE}:${VERSION}" \
        -t "${DOCKER_CONTROLLER_IMAGE}:latest" \
        -f "${PROJECT_ROOT}/docker-controller/Dockerfile" \
        "${PROJECT_ROOT}/docker-controller/"

    log_success "Docker controller image built successfully"
}

# List built images
list_images() {
    log "Built images:"
    echo ""
    docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}" | \
        grep -E "REPOSITORY|streamspace/streamspace-(kubernetes-controller|api|ui|k8s-agent|docker-controller)" || true
    echo ""
}

# Main execution
main() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  StreamSpace Local Build${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_BLUE}Version:${COLOR_RESET}    ${VERSION}"
    echo -e "${COLOR_BLUE}Commit:${COLOR_RESET}     ${GIT_COMMIT}"
    echo -e "${COLOR_BLUE}Build Date:${COLOR_RESET} ${BUILD_DATE}"
    echo ""

    check_prerequisites

    # Allow building individual components
    if [ $# -eq 0 ]; then
        # v2.0-beta components only
        build_api
        build_ui
        build_k8s_agent
    else
        # Build specific components
        for component in "$@"; do
            case "$component" in
                controller|kubernetes-controller)
                    log_error "Kubernetes controller has been replaced by k8s-agent in v2.0"
                    log_info "The controller-based architecture is deprecated"
                    exit 1
                    ;;
                api)
                    build_api
                    ;;
                ui)
                    build_ui
                    ;;
                k8s-agent|agent)
                    build_k8s_agent
                    ;;
                docker-controller)
                    build_docker_controller
                    ;;
                *)
                    log_error "Unknown component: $component"
                    log_info "Valid components: controller, api, ui, k8s-agent, docker-controller"
                    exit 1
                    ;;
            esac
        done
    fi

    list_images

    echo ""
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "All images built successfully!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    log_info "v2.0-beta Components Built:"
    echo "  ✓ API Server (Control Plane with VNC proxy)"
    echo "  ✓ UI (Web interface)"
    echo "  ✓ K8s Agent (Session management via WebSocket)"
    echo ""

    log_info "Deferred to v2.1:"
    echo "  • Docker Agent (multi-platform support)"
    echo ""
    log_info "Next steps:"
    echo "  1. Deploy to local cluster: ./scripts/local-deploy.sh"
    echo "  2. Access the UI via port-forward or ingress"
    echo "  3. Teardown when done: ./scripts/local-teardown.sh"
    echo ""
}

# Run main function
main "$@"
