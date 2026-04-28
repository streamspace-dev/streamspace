#!/usr/bin/env bash
#
# docker-dev.sh - Start StreamSpace development environment with Docker Compose
#
# This script starts the complete development environment using docker-compose,
# including PostgreSQL, NATS with JetStream, and optionally the API and Docker agent.
#
# Usage:
#   ./scripts/docker-dev.sh              # Start core services (postgres, nats)
#   ./scripts/docker-dev.sh --with-api   # Include API service
#   ./scripts/docker-dev.sh --with-docker # Include Docker agent
#   ./scripts/docker-dev.sh --all        # Start all services
#   ./scripts/docker-dev.sh --logs       # Start and follow logs
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
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.yml"

# Default options
PROFILES=""
FOLLOW_LOGS=false

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

# Show usage
usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Start StreamSpace development environment with Docker Compose.

Options:
    --with-api      Include the API service
    --with-docker   Include the Docker agent (profile: docker)
    --with-dev      Include development tools like pgAdmin (profile: dev)
    --with-monitor  Include monitoring stack (profile: monitoring)
    --all           Start all services including all profiles
    --logs          Follow logs after starting
    -h, --help      Show this help message

Examples:
    $(basename "$0")                    # Start core services (postgres, nats)
    $(basename "$0") --with-api         # Start with API
    $(basename "$0") --with-docker      # Start with Docker agent
    $(basename "$0") --all --logs       # Start all and follow logs

Services:
    Core (always started):
      - postgres         PostgreSQL database
      - nats            NATS message broker with JetStream

    API (--with-api):
      - api             StreamSpace API backend

    Docker Profile (--with-docker):
      - docker-agent  Docker platform controller

    Dev Profile (--with-dev):
      - pgadmin         PostgreSQL admin interface

    Monitoring Profile (--with-monitor):
      - prometheus      Metrics collection
      - grafana         Dashboards

EOF
    exit 0
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --with-api)
                # API is part of default services, no profile needed
                shift
                ;;
            --with-docker)
                PROFILES="${PROFILES} --profile docker"
                shift
                ;;
            --with-dev)
                PROFILES="${PROFILES} --profile dev"
                shift
                ;;
            --with-monitor|--with-monitoring)
                PROFILES="${PROFILES} --profile monitoring"
                shift
                ;;
            --all)
                PROFILES="--profile docker --profile dev --profile monitoring"
                shift
                ;;
            --logs)
                FOLLOW_LOGS=true
                shift
                ;;
            -h|--help)
                usage
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                ;;
        esac
    done
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

    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi

    if [ ! -f "$COMPOSE_FILE" ]; then
        log_error "docker-compose.yml not found at: $COMPOSE_FILE"
        exit 1
    fi

    log_success "Prerequisites satisfied"
}

# Determine docker compose command
get_compose_cmd() {
    if docker compose version &> /dev/null 2>&1; then
        echo "docker compose"
    else
        echo "docker-compose"
    fi
}

# Start services
start_services() {
    local compose_cmd
    compose_cmd=$(get_compose_cmd)

    log "Starting development environment..."
    log_info "Compose file: $COMPOSE_FILE"

    if [ -n "$PROFILES" ]; then
        log_info "Profiles: $PROFILES"
    fi

    cd "$PROJECT_ROOT"

    # Start services
    # shellcheck disable=SC2086
    $compose_cmd -f "$COMPOSE_FILE" $PROFILES up -d

    log_success "Services started"
}

# Show service status
show_status() {
    local compose_cmd
    compose_cmd=$(get_compose_cmd)

    echo ""
    log "Service status:"
    cd "$PROJECT_ROOT"
    $compose_cmd -f "$COMPOSE_FILE" ps
}

# Show connection info
show_connection_info() {
    echo ""
    log "Connection Information:"
    echo ""
    echo -e "${COLOR_BLUE}PostgreSQL:${COLOR_RESET}"
    echo "  Host:     localhost:5432"
    echo "  User:     streamspace"
    echo "  Password: streamspace"
    echo "  Database: streamspace"
    echo ""
    echo -e "${COLOR_BLUE}NATS:${COLOR_RESET}"
    echo "  Client:   nats://localhost:4222"
    echo "  Monitor:  http://localhost:8222"
    echo "  Cluster:  localhost:6222"
    echo ""

    if [[ "$PROFILES" == *"dev"* ]]; then
        echo -e "${COLOR_BLUE}pgAdmin:${COLOR_RESET}"
        echo "  URL:      http://localhost:5050"
        echo "  Email:    admin@streamspace.local"
        echo "  Password: admin"
        echo ""
    fi

    if [[ "$PROFILES" == *"monitoring"* ]]; then
        echo -e "${COLOR_BLUE}Prometheus:${COLOR_RESET}"
        echo "  URL:      http://localhost:9090"
        echo ""
        echo -e "${COLOR_BLUE}Grafana:${COLOR_RESET}"
        echo "  URL:      http://localhost:3000"
        echo "  User:     admin"
        echo "  Password: admin"
        echo ""
    fi
}

# Follow logs
follow_logs() {
    local compose_cmd
    compose_cmd=$(get_compose_cmd)

    log "Following logs (Ctrl+C to stop)..."
    cd "$PROJECT_ROOT"
    # shellcheck disable=SC2086
    $compose_cmd -f "$COMPOSE_FILE" $PROFILES logs -f
}

# Main execution
main() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  StreamSpace Development Environment${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""

    parse_args "$@"
    check_prerequisites
    start_services
    show_status
    show_connection_info

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Development environment is ready!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    log_info "Quick commands:"
    echo "  Stop:     ./scripts/docker-dev-stop.sh"
    echo "  Logs:     docker compose logs -f"
    echo "  Status:   docker compose ps"
    echo ""

    if [ "$FOLLOW_LOGS" = true ]; then
        follow_logs
    fi
}

# Run main function
main "$@"
