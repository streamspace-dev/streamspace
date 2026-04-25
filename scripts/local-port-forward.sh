#!/usr/bin/env bash
#
# local-port-forward.sh - Start port forwards for StreamSpace v2.0 services
#
# This script automatically creates port forwards for all StreamSpace services
# in the background, making them accessible on localhost.
#
# Services:
#   - UI:  http://localhost:3000  -> streamspace-ui:80
#   - API: http://localhost:8000  -> streamspace-api:8000
#
# v2.0 Architecture Notes:
#   - VNC traffic now flows through the API's /api/v1/vnc/{sessionId} endpoint
#   - K8s Agent communicates with API via WebSocket
#   - No additional port-forwards needed for v2.0 architecture
#
# Port forwards run in the background with output redirected to log files.
# Use local-stop-port-forward.sh to stop all port forwards.
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
LOG_DIR="${PROJECT_ROOT}/.port-forward-logs"
PID_DIR="${PROJECT_ROOT}/.port-forward-pids"

# Port mappings
UI_LOCAL_PORT=3000
UI_REMOTE_PORT=80
API_LOCAL_PORT=8000
API_REMOTE_PORT=8000
NATS_LOCAL_PORT=4222
NATS_REMOTE_PORT=4222
NATS_MONITOR_LOCAL_PORT=8222
NATS_MONITOR_REMOTE_PORT=8222
REDIS_LOCAL_PORT=6379
REDIS_REMOTE_PORT=6379

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
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    if ! kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        log_error "Namespace ${NAMESPACE} does not exist"
        exit 1
    fi
}

# Create directories
create_directories() {
    mkdir -p "${LOG_DIR}"
    mkdir -p "${PID_DIR}"
}

# Check if port is already in use
check_port() {
    local port=$1
    if lsof -Pi ":${port}" -sTCP:LISTEN -t &> /dev/null; then
        return 0  # Port is in use
    else
        return 1  # Port is free
    fi
}

# Start port forward
start_port_forward() {
    local service=$1
    local local_port=$2
    local remote_port=$3
    local name=$4

    log_info "Starting port forward: ${name}"

    # Check if service exists
    if ! kubectl get svc "${service}" -n "${NAMESPACE}" &> /dev/null; then
        log_error "Service ${service} not found in namespace ${NAMESPACE}"
        return 1
    fi

    # Check if port is already in use
    if check_port "${local_port}"; then
        log_warning "Port ${local_port} already in use, skipping ${name}"
        log_info "To free the port: ./scripts/local-stop-port-forward.sh"
        return 1
    fi

    # Wait for service to have endpoints
    log_info "Waiting for ${service} to be ready..."
    local timeout=60
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        if kubectl get endpoints "${service}" -n "${NAMESPACE}" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | grep -q .; then
            break
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    if [ $elapsed -ge $timeout ]; then
        log_error "${service} has no ready endpoints after ${timeout}s"
        return 1
    fi

    # Start port forward in background
    local log_file="${LOG_DIR}/${name}.log"
    local pid_file="${PID_DIR}/${name}.pid"

    kubectl port-forward -n "${NAMESPACE}" "svc/${service}" "${local_port}:${remote_port}" \
        > "${log_file}" 2>&1 &

    local pid=$!
    echo "${pid}" > "${pid_file}"

    # Wait a moment and check if it's still running
    sleep 2
    if kill -0 "${pid}" 2>/dev/null; then
        log_success "${name} forwarded: localhost:${local_port} -> ${service}:${remote_port} (PID: ${pid})"
        return 0
    else
        log_error "Port forward failed to start for ${name}"
        rm -f "${pid_file}"
        return 1
    fi
}

# Check running port forwards
check_running() {
    log "Checking running port forwards..."
    echo ""

    local running=0

    for pid_file in "${PID_DIR}"/*.pid; do
        if [ -f "${pid_file}" ]; then
            local pid=$(cat "${pid_file}")
            local name=$(basename "${pid_file}" .pid)

            if kill -0 "${pid}" 2>/dev/null; then
                log_success "${name} (PID: ${pid})"
                running=$((running + 1))
            else
                log_warning "${name} (PID: ${pid}) - NOT RUNNING"
                rm -f "${pid_file}"
            fi
        fi
    done

    if [ $running -eq 0 ]; then
        log_warning "No port forwards currently running"
    fi
    echo ""
}

# Show access URLs
show_access_urls() {
    echo ""
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Access StreamSpace${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""

    log_info "Web UI:"
    echo "  ${COLOR_GREEN}http://localhost:${UI_LOCAL_PORT}${COLOR_RESET}"
    echo ""

    log_info "API Backend:"
    echo "  ${COLOR_GREEN}http://localhost:${API_LOCAL_PORT}${COLOR_RESET}"
    echo "  Health: ${COLOR_BLUE}http://localhost:${API_LOCAL_PORT}/health${COLOR_RESET}"
    echo ""

    # Show NATS info if available
    if [ -f "${PID_DIR}/nats.pid" ] || kubectl get svc "streamspace-nats" -n "${NAMESPACE}" &> /dev/null 2>&1; then
        log_info "NATS Message Queue:"
        echo "  Client:  ${COLOR_GREEN}nats://localhost:${NATS_LOCAL_PORT}${COLOR_RESET}"
        echo "  Monitor: ${COLOR_BLUE}http://localhost:${NATS_MONITOR_LOCAL_PORT}${COLOR_RESET}"
        echo ""
    fi

    # Show Redis info if available
    if [ -f "${PID_DIR}/redis.pid" ] || kubectl get svc "streamspace-redis" -n "${NAMESPACE}" &> /dev/null 2>&1; then
        log_info "Redis Cache:"
        echo "  ${COLOR_GREEN}localhost:${REDIS_LOCAL_PORT}${COLOR_RESET}"
        echo ""
    fi

    log_info "Logs:"
    echo "  UI:  tail -f ${LOG_DIR}/ui.log"
    echo "  API: tail -f ${LOG_DIR}/api.log"
    echo ""

    log_info "To stop port forwards:"
    echo "  ./scripts/local-stop-port-forward.sh"
    echo ""
}

# Cleanup on exit
cleanup() {
    echo ""
    log_warning "Received interrupt signal"
    log_info "Port forwards will continue running in background"
    log_info "Use ./scripts/local-stop-port-forward.sh to stop them"
    exit 0
}

# Main execution
main() {
    # Handle Ctrl+C gracefully
    trap cleanup SIGINT SIGTERM

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  Start Port Forwards${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_BLUE}Namespace:${COLOR_RESET}     ${NAMESPACE}"
    echo ""

    check_prerequisites
    create_directories

    # Check for existing port forwards
    if [ -d "${PID_DIR}" ] && [ -n "$(ls -A "${PID_DIR}" 2>/dev/null)" ]; then
        check_running
    fi

    # Start port forwards
    log "Starting port forwards..."
    echo ""

    local success=0

    if start_port_forward "streamspace-ui" "${UI_LOCAL_PORT}" "${UI_REMOTE_PORT}" "ui"; then
        success=$((success + 1))
    fi

    if start_port_forward "streamspace-api" "${API_LOCAL_PORT}" "${API_REMOTE_PORT}" "api"; then
        success=$((success + 1))
    fi

    # Optional NATS port forwards (if NATS is deployed)
    if kubectl get svc "streamspace-nats" -n "${NAMESPACE}" &> /dev/null; then
        if start_port_forward "streamspace-nats" "${NATS_LOCAL_PORT}" "${NATS_REMOTE_PORT}" "nats"; then
            success=$((success + 1))
        fi
        if start_port_forward "streamspace-nats" "${NATS_MONITOR_LOCAL_PORT}" "${NATS_MONITOR_REMOTE_PORT}" "nats-monitor"; then
            success=$((success + 1))
        fi
    fi

    # Optional Redis port forwards (if Redis is deployed)
    if kubectl get svc "streamspace-redis" -n "${NAMESPACE}" &> /dev/null; then
        if start_port_forward "streamspace-redis" "${REDIS_LOCAL_PORT}" "${REDIS_REMOTE_PORT}" "redis"; then
            success=$((success + 1))
        fi
    fi

    echo ""
    if [ $success -gt 0 ]; then
        show_access_urls

        echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
        log_success "Started ${success} port forward(s)"
        echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
        echo ""

        log_info "Port forwards are running in background"
        log_info "They will persist until you stop them or restart your terminal"
    else
        log_error "Failed to start any port forwards"
        exit 1
    fi
}

# Run main function
main "$@"
