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
CONTROLLER_IMAGE="streamspace/streamspace-controller"
API_IMAGE="streamspace/streamspace-api"
UI_IMAGE="streamspace/streamspace-ui"

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

# Build controller image
build_controller() {
    log "Building controller image..."
    log_info "Image: ${CONTROLLER_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/controller"

    docker build ${BUILD_ARGS} \
        -t "${CONTROLLER_IMAGE}:${VERSION}" \
        -t "${CONTROLLER_IMAGE}:latest" \
        -f "${PROJECT_ROOT}/controller/Dockerfile" \
        "${PROJECT_ROOT}/controller/"

    log_success "Controller image built successfully"
}

# Build API image
build_api() {
    log "Building API image..."
    log_info "Image: ${API_IMAGE}:${VERSION}"
    log_info "Context: ${PROJECT_ROOT}/api"

    docker build ${BUILD_ARGS} \
        -t "${API_IMAGE}:${VERSION}" \
        -t "${API_IMAGE}:latest" \
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
        -f "${PROJECT_ROOT}/ui/Dockerfile" \
        "${PROJECT_ROOT}/ui/"

    log_success "UI image built successfully"
}

# List built images
list_images() {
    log "Built images:"
    echo ""
    docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}" | \
        grep -E "REPOSITORY|streamspace/streamspace-(controller|api|ui)" || true
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
        # Build all components
        build_controller
        build_api
        build_ui
    else
        # Build specific components
        for component in "$@"; do
            case "$component" in
                controller)
                    build_controller
                    ;;
                api)
                    build_api
                    ;;
                ui)
                    build_ui
                    ;;
                *)
                    log_error "Unknown component: $component"
                    log_info "Valid components: controller, api, ui"
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
    log_info "Next steps:"
    echo "  1. Deploy to local cluster: ./scripts/local-deploy.sh"
    echo "  2. Access the UI via port-forward or ingress"
    echo "  3. Teardown when done: ./scripts/local-teardown.sh"
    echo ""
}

# Run main function
main "$@"
