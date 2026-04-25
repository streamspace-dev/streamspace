#!/usr/bin/env bash
#
# local-deploy-kubectl.sh - Deploy StreamSpace using kubectl (Helm-free)
#
# This script deploys StreamSpace without Helm, using raw Kubernetes manifests.
# Use this as a workaround for Helm v3.19.0 bugs or when Helm is unavailable.
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
VERSION="${VERSION:-local}"

# Image configuration
K8S_AGENT_IMAGE="${K8S_AGENT_IMAGE:-streamspace/streamspace-k8s-agent:${VERSION}}"
API_IMAGE="${API_IMAGE:-streamspace/streamspace-api:${VERSION}}"
UI_IMAGE="${UI_IMAGE:-streamspace/streamspace-ui:${VERSION}}"
POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:15-alpine}"
REDIS_IMAGE="${REDIS_IMAGE:-redis:7-alpine}"

# Optional services (can be set via environment or command line)
ENABLE_REDIS="${ENABLE_REDIS:-false}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-redis)
            ENABLE_REDIS=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Deploy StreamSpace using kubectl (Helm-free)"
            echo ""
            echo "Options:"
            echo "  --with-redis    Enable Redis cache for improved performance"
            echo "  --help, -h      Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  NAMESPACE       Kubernetes namespace (default: streamspace)"
            echo "  VERSION         Image version tag (default: local)"
            echo "  ENABLE_REDIS    Enable Redis (default: false)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

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

    local context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    log_success "Connected to cluster: ${context}"
}

# Check if images exist locally
check_images() {
    log "Checking for locally built images..."

    local missing_images=0

    for image in "${K8S_AGENT_IMAGE}" "${API_IMAGE}" "${UI_IMAGE}"; do
        # Extract repo and tag for checking
        local repo_tag="${image}"
        if docker images "${repo_tag}" --format "{{.Repository}}:{{.Tag}}" | grep -q "${repo_tag}"; then
            log_success "Found ${repo_tag}"
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

# Apply CRDs
apply_crds() {
    log "Applying Custom Resource Definitions..."

    kubectl apply -f "${PROJECT_ROOT}/chart/crds/"
    log_success "CRDs applied"
}

# Create secrets
create_secrets() {
    log "Creating secrets..."

    if kubectl get secret streamspace-secrets -n "${NAMESPACE}" &> /dev/null; then
        log_warning "Secret streamspace-secrets already exists"
    else
        kubectl create secret generic streamspace-secrets \
            -n "${NAMESPACE}" \
            --from-literal=postgres-password=streamspace \
            --from-literal=jwt-secret=$(openssl rand -base64 32) \
            --from-literal=api-key=$(openssl rand -hex 32)
        log_success "Secrets created"
    fi

    # Create admin credentials secret
    if kubectl get secret streamspace-admin-credentials -n "${NAMESPACE}" &> /dev/null; then
        log_warning "Secret streamspace-admin-credentials already exists"
    else
        kubectl create secret generic streamspace-admin-credentials \
            -n "${NAMESPACE}" \
            --from-literal=username=admin \
            --from-literal=password=Password12345 \
            --from-literal=email=admin@streamspace.local
        kubectl label secret streamspace-admin-credentials \
            -n "${NAMESPACE}" \
            app.kubernetes.io/name=streamspace \
            app.kubernetes.io/component=admin \
            app.kubernetes.io/managed-by=kubectl
        log_success "Admin credentials secret created"
    fi
}

# Deploy PostgreSQL
deploy_postgresql() {
    log "Deploying PostgreSQL..."

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: streamspace-postgres
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: database
spec:
  type: ClusterIP
  ports:
    - port: 5432
      targetPort: 5432
      protocol: TCP
      name: postgres
  selector:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: database
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: streamspace-postgres
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: database
spec:
  serviceName: streamspace-postgres
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: streamspace
      app.kubernetes.io/component: database
  template:
    metadata:
      labels:
        app.kubernetes.io/name: streamspace
        app.kubernetes.io/component: database
    spec:
      containers:
      - name: postgres
        image: ${POSTGRES_IMAGE}
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 5432
          name: postgres
        env:
        - name: POSTGRES_DB
          value: streamspace
        - name: POSTGRES_USER
          value: streamspace
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: streamspace-secrets
              key: postgres-password
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: 256Mi
            cpu: 100m
          limits:
            memory: 1Gi
            cpu: 500m
        livenessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - streamspace
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - streamspace
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
EOF

    log_success "PostgreSQL deployed"
}

# NATS MESSAGE BROKER REMOVED
# Agents now communicate via WebSocket instead of NATS pub/sub
# deploy_nats() {
#     log "Deploying NATS..."

#     cat <<EOF | kubectl apply -f -
# apiVersion: v1
# kind: Service
# metadata:
#   name: streamspace-nats
#   namespace: ${NAMESPACE}
#   labels:
#     app.kubernetes.io/name: streamspace
#     app.kubernetes.io/component: nats
# spec:
#   type: ClusterIP
#   ports:
#     - port: 4222
#       targetPort: 4222
#       protocol: TCP
#       name: client
#     - port: 8222
#       targetPort: 8222
#       protocol: TCP
#       name: monitoring
#   selector:
#     app.kubernetes.io/name: streamspace
#     app.kubernetes.io/component: nats
# ---
# apiVersion: apps/v1
# kind: Deployment
# metadata:
#   name: streamspace-nats
#   namespace: ${NAMESPACE}
#   labels:
#     app.kubernetes.io/name: streamspace
#     app.kubernetes.io/component: nats
# spec:
#   replicas: 1
#   selector:
#     matchLabels:
#       app.kubernetes.io/name: streamspace
#       app.kubernetes.io/component: nats
#   template:
#     metadata:
#       labels:
#         app.kubernetes.io/name: streamspace
#         app.kubernetes.io/component: nats
#     spec:
#       containers:
#       - name: nats
#         image: nats:2.10-alpine
#         imagePullPolicy: IfNotPresent
#         args:
#           - "--jetstream"
#           - "--store_dir=/data"
#           - "--http_port=8222"
#         ports:
#         - containerPort: 4222
#           name: client
#         - containerPort: 8222
#           name: monitoring
#         resources:
#           requests:
#             memory: 64Mi
#             cpu: 50m
#           limits:
#             memory: 256Mi
#             cpu: 200m
#         livenessProbe:
#           httpGet:
#             path: /healthz
#             port: monitoring
#           initialDelaySeconds: 10
#           periodSeconds: 10
#         readinessProbe:
#           httpGet:
#             path: /healthz
#             port: monitoring
#           initialDelaySeconds: 5
#           periodSeconds: 5
#         volumeMounts:
#         - name: data
#           mountPath: /data
#       volumes:
#       - name: data
#         emptyDir: {}
# EOF

#     log_success "NATS deployed"
# }

# Deploy Redis (optional)
deploy_redis() {
    if [ "${ENABLE_REDIS}" != "true" ]; then
        log_info "Redis disabled (use --with-redis to enable)"
        return 0
    fi

    log "Deploying Redis..."

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: streamspace-redis
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: redis
spec:
  type: ClusterIP
  ports:
    - port: 6379
      targetPort: 6379
      protocol: TCP
      name: redis
  selector:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: redis
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-redis
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: streamspace
      app.kubernetes.io/component: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: streamspace
        app.kubernetes.io/component: redis
    spec:
      containers:
      - name: redis
        image: ${REDIS_IMAGE}
        imagePullPolicy: IfNotPresent
        args:
          - redis-server
          - --maxmemory
          - 200mb
          - --maxmemory-policy
          - allkeys-lru
        ports:
        - containerPort: 6379
          name: redis
        resources:
          requests:
            memory: 64Mi
            cpu: 50m
          limits:
            memory: 256Mi
            cpu: 200m
        livenessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 5
          periodSeconds: 5
EOF

    log_success "Redis deployed"
}

# Deploy K8s Agent
deploy_agent() {
    log "Deploying K8s Agent..."

    # Create ServiceAccount and RBAC
    kubectl apply -f "${PROJECT_ROOT}/manifests/kubectl/rbac.yaml"

    # Create Agent Deployment
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-k8s-agent
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: k8s-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: streamspace
      app.kubernetes.io/component: k8s-agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: streamspace
        app.kubernetes.io/component: k8s-agent
    spec:
      serviceAccountName: streamspace-k8s-agent
      containers:
      - name: k8s-agent
        image: ${K8S_AGENT_IMAGE}
        imagePullPolicy: Never
        args:
          - --agent-id=k8s-agent-local
          - --control-plane-url=http://streamspace-api:8000
          - --platform=kubernetes
          - --namespace=${NAMESPACE}
        resources:
          requests:
            memory: 64Mi
            cpu: 50m
          limits:
            memory: 256Mi
            cpu: 200m
EOF

    log_success "K8s Agent deployed"
}

# Deploy API
deploy_api() {
    log "Deploying API Backend..."

    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-api
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: api
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: streamspace
      app.kubernetes.io/component: api
  template:
    metadata:
      labels:
        app.kubernetes.io/name: streamspace
        app.kubernetes.io/component: api
    spec:
      serviceAccountName: streamspace-api
      containers:
      - name: api
        image: ${API_IMAGE}
        imagePullPolicy: Never
        ports:
        - containerPort: 8000
          name: http
          protocol: TCP
        env:
        - name: GIN_MODE
          value: debug
        - name: DB_HOST
          value: streamspace-postgres
        - name: DB_PORT
          value: "5432"
        - name: DB_NAME
          value: streamspace
        - name: DB_USER
          value: streamspace
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: streamspace-secrets
              key: postgres-password
        - name: DB_SSLMODE
          value: disable
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: streamspace-secrets
              key: jwt-secret
        - name: ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: streamspace-admin-credentials
              key: password
              optional: true
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # - name: NATS_URL
        #   value: nats://streamspace-nats:4222  # NATS REMOVED
        - name: PLATFORM
          value: kubernetes
        - name: CACHE_ENABLED
          value: "${ENABLE_REDIS}"
        - name: REDIS_HOST
          value: streamspace-redis
        - name: REDIS_PORT
          value: "6379"
        resources:
          requests:
            memory: 256Mi
            cpu: 100m
          limits:
            memory: 1Gi
            cpu: 1000m
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: streamspace-api
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: api
spec:
  type: ClusterIP
  ports:
    - port: 8000
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: api
EOF

    log_success "API deployed"
}

# Deploy UI
deploy_ui() {
    log "Deploying Web UI..."

    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-ui
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: ui
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: streamspace
      app.kubernetes.io/component: ui
  template:
    metadata:
      labels:
        app.kubernetes.io/name: streamspace
        app.kubernetes.io/component: ui
    spec:
      containers:
      - name: ui
        image: ${UI_IMAGE}
        imagePullPolicy: Never
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        env:
        - name: API_URL
          value: http://streamspace-api:8000
        resources:
          requests:
            memory: 128Mi
            cpu: 50m
          limits:
            memory: 256Mi
            cpu: 200m
        livenessProbe:
          httpGet:
            path: /
            port: http
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: streamspace-ui
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: ui
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: ui
EOF

    log_success "UI deployed"
}

# Wait for pods to be ready
wait_for_pods() {
    log "Waiting for pods to be ready..."

    local timeout=300  # 5 minutes
    local elapsed=0
    local interval=5

    while [ $elapsed -lt $timeout ]; do
        local ready_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | \
            jq -r '.items[] | select(.status.phase == "Running") | .metadata.name' 2>/dev/null | wc -l || echo "0")
        local total_pods=$(kubectl get pods -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo "0")

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
    echo "  kubectl port-forward -n ${NAMESPACE} svc/streamspace-ui 3000:80"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/streamspace-api 8000:8000"
    # echo "  kubectl port-forward -n ${NAMESPACE} svc/streamspace-nats 4222:4222"  # NATS REMOVED
    if [ "${ENABLE_REDIS}" = "true" ]; then
        echo "  kubectl port-forward -n ${NAMESPACE} svc/streamspace-redis 6379:6379"
    fi
    echo ""

    log_info "Service URLs (after port-forward):"
    echo "  UI:   http://localhost:3000"
    echo "  API:  http://localhost:8000"
    # echo "  NATS: nats://localhost:4222 (monitor: http://localhost:8222)"  # NATS REMOVED
    if [ "${ENABLE_REDIS}" = "true" ]; then
        echo "  Redis: localhost:6379"
    fi
    echo ""

    log_info "View logs:"
    echo "  K8s Agent:  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=k8s-agent -f"
    echo "  API:        kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=api -f"
    echo "  UI:         kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=ui -f"
    echo "  Database:   kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=database -f"
    # echo "  NATS:       kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=nats -f"  # NATS REMOVED
    echo ""

    log_info "When finished testing:"
    echo "  ./scripts/local-stop-port-forward.sh  # Stop port forwards"
    echo "  kubectl delete namespace ${NAMESPACE}  # Delete everything"
    echo ""
}

# Main execution
main() {
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  StreamSpace kubectl Deployment${COLOR_RESET}"
    echo -e "${COLOR_BOLD}  (Helm-free alternative)${COLOR_RESET}"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    echo ""
    echo -e "${COLOR_BLUE}Namespace:${COLOR_RESET}     ${NAMESPACE}"
    echo -e "${COLOR_BLUE}Version:${COLOR_RESET}       ${VERSION}"
    echo -e "${COLOR_BLUE}Redis:${COLOR_RESET}         ${ENABLE_REDIS}"
    echo ""

    check_prerequisites
    check_images
    create_namespace
    apply_crds
    create_secrets
    deploy_postgresql
    # deploy_nats  # NATS REMOVED - agents use WebSocket
    deploy_redis
    deploy_agent
    deploy_api
    deploy_ui
    wait_for_pods
    show_status
    start_port_forwards

    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
    log_success "Deployment complete!"
    echo -e "${COLOR_BOLD}═══════════════════════════════════════════════════${COLOR_RESET}"
}

# Run main function
main "$@"
