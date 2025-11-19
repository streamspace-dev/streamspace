#!/usr/bin/env bash
#
# local-teardown.sh - Teardown StreamSpace and cleanup Docker artifacts
#
# This script completely removes StreamSpace from your local Kubernetes cluster
# and cleans up all Docker images and containers created during testing.
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

# Confirm teardown
confirm_teardown() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  StreamSpace Local Teardown${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_YELLOW}This will:${COLOR_RESET}"
    echo "  • Uninstall Helm release: ${RELEASE_NAME}"
    echo "  • Delete namespace: ${NAMESPACE} (and all resources)"
    echo "  • Remove CRDs (Custom Resource Definitions)"
    echo "  • Delete local Docker images"
    echo "  • Clean up Docker build cache"
    echo ""

    if [ "${AUTO_CONFIRM:-}" != "true" ]; then
        read -p "Continue? (yes/no): " -r
        echo
        if [[ ! $REPLY =~ ^[Yy](es)?$ ]]; then
            log_info "Teardown cancelled"
            exit 0
        fi
    fi
}

# Uninstall Helm release
uninstall_helm() {
    log "Uninstalling Helm release..."

    if helm status "${RELEASE_NAME}" -n "${NAMESPACE}" &> /dev/null; then
        helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" --wait
        log_success "Helm release uninstalled"
    else
        log_warning "Helm release ${RELEASE_NAME} not found in namespace ${NAMESPACE}"
    fi
}

# Delete namespace
delete_namespace() {
    log "Deleting namespace: ${NAMESPACE}"

    if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        kubectl delete namespace "${NAMESPACE}" --wait=false
        log_success "Namespace deletion initiated (may take a few moments)"

        # Wait for namespace deletion (with timeout)
        log_info "Waiting for namespace deletion to complete..."
        local timeout=120  # 2 minutes
        local elapsed=0
        local interval=5

        while kubectl get namespace "${NAMESPACE}" &> /dev/null && [ $elapsed -lt $timeout ]; do
            sleep $interval
            elapsed=$((elapsed + interval))
        done

        if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
            log_warning "Namespace deletion taking longer than expected"
            log_info "You may need to manually check: kubectl get namespace ${NAMESPACE}"
        else
            log_success "Namespace deleted"
        fi
    else
        log_warning "Namespace ${NAMESPACE} not found"
    fi
}

# Delete CRDs
delete_crds() {
    log "Deleting Custom Resource Definitions..."

    local crds=(
        "sessions.stream.streamspace.io"
        "templates.stream.streamspace.io"
        "templaterepositories.stream.streamspace.io"
        "connections.stream.streamspace.io"
    )

    local deleted=0
    for crd in "${crds[@]}"; do
        if kubectl get crd "${crd}" &> /dev/null; then
            kubectl delete crd "${crd}" --wait=false
            deleted=$((deleted + 1))
        fi
    done

    if [ $deleted -gt 0 ]; then
        log_success "Deleted ${deleted} CRD(s)"
        log_info "Waiting for CRD deletion to finalize..."
        sleep 5
    else
        log_warning "No StreamSpace CRDs found"
    fi
}

# Clean Docker images
clean_docker_images() {
    log "Cleaning Docker images..."

    # Remove StreamSpace images
    local images=(
        "streamspace/streamspace-kubernetes-controller:${VERSION}"
        "streamspace/streamspace-kubernetes-controller:latest"
        "streamspace/streamspace-api:${VERSION}"
        "streamspace/streamspace-api:latest"
        "streamspace/streamspace-ui:${VERSION}"
        "streamspace/streamspace-ui:latest"
        "streamspace/streamspace-docker-controller:${VERSION}"
        "streamspace/streamspace-docker-controller:latest"
    )

    local removed=0
    for image in "${images[@]}"; do
        if docker images -q "${image}" 2>/dev/null | grep -q .; then
            docker rmi "${image}" &> /dev/null || true
            removed=$((removed + 1))
        fi
    done

    if [ $removed -gt 0 ]; then
        log_success "Removed ${removed} Docker image(s)"
    else
        log_warning "No StreamSpace Docker images found"
    fi
}

# Clean dangling images
clean_dangling_images() {
    log "Cleaning dangling images..."

    local dangling_count=$(docker images -f "dangling=true" -q | wc -l)

    if [ "$dangling_count" -gt 0 ]; then
        docker image prune -f &> /dev/null
        log_success "Removed ${dangling_count} dangling image(s)"
    else
        log_info "No dangling images to clean"
    fi
}

# Clean build cache
clean_build_cache() {
    log "Cleaning Docker build cache..."

    # Only clean buildx cache if user confirms (can be large)
    if [ "${CLEAN_CACHE:-}" = "true" ]; then
        docker builder prune -af &> /dev/null || true
        log_success "Build cache cleaned"
    else
        log_info "Skipping build cache cleanup (set CLEAN_CACHE=true to enable)"
    fi
}

# Clean stopped containers
clean_containers() {
    log "Cleaning stopped containers..."

    local stopped_count=$(docker ps -aq -f status=exited | wc -l)

    if [ "$stopped_count" -gt 0 ]; then
        docker container prune -f &> /dev/null
        log_success "Removed ${stopped_count} stopped container(s)"
    else
        log_info "No stopped containers to clean"
    fi
}

# Show remaining resources
show_remaining() {
    echo ""
    log "Remaining StreamSpace resources:"
    echo ""

    # Check for any remaining pods
    local remaining_pods=$(kubectl get pods -A -l app.kubernetes.io/name=streamspace 2>/dev/null | tail -n +2 | wc -l)
    if [ "$remaining_pods" -gt 0 ]; then
        log_warning "Found ${remaining_pods} remaining pod(s)"
        kubectl get pods -A -l app.kubernetes.io/name=streamspace
    else
        log_success "No remaining pods"
    fi

    # Check for any remaining images
    local remaining_images=$(docker images | grep -c "streamspace/streamspace-" || echo "0")
    if [ "$remaining_images" -gt 0 ]; then
        log_warning "Found ${remaining_images} remaining image(s)"
        docker images | grep "streamspace/streamspace-" || true
    else
        log_success "No remaining Docker images"
    fi

    # Check for Docker Compose development containers
    local compose_containers=$(docker ps -a --filter "name=streamspace" --format "{{.Names}}" | wc -l)
    if [ "$compose_containers" -gt 0 ]; then
        log_warning "Found ${compose_containers} Docker Compose container(s)"
        log_info "Stop with: ./scripts/docker-dev-stop.sh"
    fi
}

# Show Docker disk usage
show_docker_usage() {
    echo ""
    log_info "Docker disk usage:"
    docker system df
}

# Main execution
main() {
    confirm_teardown

    echo ""
    log "Starting teardown process..."
    echo ""

    # Check if kubectl is available
    if command -v kubectl &> /dev/null; then
        uninstall_helm
        delete_namespace
        delete_crds
    else
        log_warning "kubectl not found, skipping Kubernetes cleanup"
    fi

    # Check if docker is available
    if command -v docker &> /dev/null; then
        clean_docker_images
        clean_dangling_images
        clean_containers
        clean_build_cache
    else
        log_warning "docker not found, skipping Docker cleanup"
    fi

    show_remaining
    show_docker_usage

    echo ""
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Teardown complete!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    log_info "To rebuild and redeploy:"
    echo "  1. ./scripts/local-build.sh"
    echo "  2. ./scripts/local-deploy.sh"
    echo ""
}

# Run main function
main "$@"
