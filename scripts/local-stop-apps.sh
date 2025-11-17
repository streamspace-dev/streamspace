#!/usr/bin/env bash
#
# local-stop-apps.sh - Stop StreamSpace application containers (preserves database)
#
# This script safely stops the StreamSpace application components by scaling
# their deployments to 0 replicas. The database and all data are preserved.
#
# Use this when you want to:
#   1. Pull latest code changes
#   2. Rebuild Docker images
#   3. Redeploy with updated images
#
# Workflow:
#   1. ./scripts/local-stop-apps.sh       # Stop apps (this script)
#   2. git pull                           # Get latest code
#   3. ./scripts/local-build.sh           # Rebuild images
#   4. ./scripts/local-deploy-kubectl.sh  # Deploy updated apps
#
# The database remains running and all data is preserved throughout.
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

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        log_info "Make sure your kubeconfig is properly configured"
        exit 1
    fi

    if ! kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        log_error "Namespace ${NAMESPACE} does not exist"
        log_info "Nothing to stop - namespace not found"
        exit 1
    fi

    local context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    log_success "Connected to cluster: ${context}"
}

# Show current status
show_status_before() {
    log "Current deployment status:"
    echo ""

    log_info "Application Deployments:"
    kubectl get deployments -n "${NAMESPACE}" \
        -l 'app.kubernetes.io/name=streamspace,app.kubernetes.io/component in (controller,api,ui)' \
        -o wide 2>/dev/null || log_warning "No application deployments found"
    echo ""

    log_info "Database StatefulSet (will NOT be stopped):"
    kubectl get statefulsets -n "${NAMESPACE}" \
        -l 'app.kubernetes.io/component=database' \
        -o wide 2>/dev/null || log_warning "No database found"
    echo ""

    log_info "Running Pods:"
    kubectl get pods -n "${NAMESPACE}" -o wide 2>/dev/null || log_warning "No pods found"
    echo ""
}

# Stop application deployments
stop_applications() {
    log "Stopping application containers..."
    echo ""

    local deployments=(
        "streamspace-controller"
        "streamspace-api"
        "streamspace-ui"
    )

    local stopped=0
    for deployment in "${deployments[@]}"; do
        if kubectl get deployment "${deployment}" -n "${NAMESPACE}" &> /dev/null; then
            log_info "Scaling ${deployment} to 0 replicas..."
            kubectl scale deployment "${deployment}" -n "${NAMESPACE}" --replicas=0
            stopped=$((stopped + 1))
            log_success "${deployment} stopped"
        else
            log_warning "${deployment} not found"
        fi
    done

    echo ""
    if [ $stopped -gt 0 ]; then
        log_success "Stopped ${stopped} application deployment(s)"
    else
        log_warning "No application deployments found to stop"
    fi
}

# Wait for pods to terminate
wait_for_termination() {
    log "Waiting for application pods to terminate..."

    local timeout=60  # 1 minute
    local elapsed=0
    local interval=2

    while [ $elapsed -lt $timeout ]; do
        local app_pods=$(kubectl get pods -n "${NAMESPACE}" \
            -l 'app.kubernetes.io/component in (controller,api,ui)' \
            --field-selector=status.phase!=Succeeded,status.phase!=Failed \
            --no-headers 2>/dev/null | wc -l || echo "0")

        if [ "$app_pods" -eq 0 ]; then
            log_success "All application pods terminated"
            return 0
        fi

        log_info "Waiting... (${app_pods} pod(s) still terminating)"
        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_warning "Some pods are still terminating (timeout reached)"
    log_info "They will continue terminating in the background"
}

# Show final status
show_status_after() {
    echo ""
    log "Final status:"
    echo ""

    log_info "Application Deployments (should show 0/0 ready):"
    kubectl get deployments -n "${NAMESPACE}" \
        -l 'app.kubernetes.io/name=streamspace,app.kubernetes.io/component in (controller,api,ui)' \
        2>/dev/null || log_warning "No application deployments found"
    echo ""

    log_info "Database StatefulSet (should still be running):"
    kubectl get statefulsets -n "${NAMESPACE}" \
        -l 'app.kubernetes.io/component=database' \
        2>/dev/null || log_warning "No database found"
    echo ""

    log_info "Remaining Pods (should only be database):"
    kubectl get pods -n "${NAMESPACE}" 2>/dev/null || log_warning "No pods found"
    echo ""

    log_info "Persistent Volume Claims (all preserved):"
    kubectl get pvc -n "${NAMESPACE}" 2>/dev/null || log_warning "No PVCs found"
    echo ""
}

# Show next steps
show_next_steps() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Next Steps${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""

    log_info "To update and restart StreamSpace:"
    echo ""
    echo "  1. Pull latest code:"
    echo "     ${COLOR_BLUE}git pull${COLOR_RESET}"
    echo ""
    echo "  2. Rebuild Docker images:"
    echo "     ${COLOR_BLUE}./scripts/local-build.sh${COLOR_RESET}"
    echo ""
    echo "  3. Deploy updated applications:"
    echo "     ${COLOR_BLUE}./scripts/local-deploy-kubectl.sh${COLOR_RESET}"
    echo ""
    echo "     ${COLOR_YELLOW}NOTE:${COLOR_RESET} The deploy script will detect existing resources"
    echo "           and update them with the new images."
    echo ""

    log_info "To manually restart without rebuilding:"
    echo "     ${COLOR_BLUE}kubectl scale deployment streamspace-controller -n ${NAMESPACE} --replicas=1${COLOR_RESET}"
    echo "     ${COLOR_BLUE}kubectl scale deployment streamspace-api -n ${NAMESPACE} --replicas=1${COLOR_RESET}"
    echo "     ${COLOR_BLUE}kubectl scale deployment streamspace-ui -n ${NAMESPACE} --replicas=1${COLOR_RESET}"
    echo ""

    log_info "Database status:"
    echo "     The PostgreSQL database is still running and all data is preserved."
    echo ""
}

# Main execution
main() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Stop StreamSpace Applications${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  (Database will NOT be stopped)${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_BLUE}Namespace:${COLOR_RESET}     ${NAMESPACE}"
    echo ""

    check_prerequisites
    show_status_before
    stop_applications
    wait_for_termination
    show_status_after
    show_next_steps

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Applications stopped successfully!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
}

# Run main function
main "$@"
