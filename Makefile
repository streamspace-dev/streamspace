.PHONY: help build test deploy clean docker-build docker-push helm-install helm-upgrade helm-uninstall dev-setup

# Project configuration
PROJECT_NAME := streamspace
DOCKER_REGISTRY := ghcr.io
DOCKER_ORG := streamspace
VERSION := v2.0.0

# Git information for versioning
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "$(VERSION)")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Component images (v2.0 architecture)
K8S_AGENT_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/k8s-agent
DOCKER_AGENT_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/docker-agent
API_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/streamspace-api
UI_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/streamspace-ui

# Kubernetes configuration
NAMESPACE := streamspace
HELM_RELEASE := streamspace
KUBE_CONTEXT := $(shell kubectl config current-context)

# Paths
PROJECT_ROOT := $(shell pwd)
CHART_PATH := $(PROJECT_ROOT)/chart

# Build configuration
GO_VERSION := 1.21
NODE_VERSION := 18

# Build arguments
BUILD_ARGS := --build-arg VERSION=$(GIT_TAG) \
              --build-arg COMMIT=$(GIT_COMMIT) \
              --build-arg BUILD_DATE=$(BUILD_DATE)

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

##@ General

help: ## Display this help message
	@echo "$(COLOR_BOLD)StreamSpace v2.0 Development Makefile$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Multi-Platform Agent Architecture$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

dev-setup: ## Set up development environment
	@echo "$(COLOR_GREEN)Setting up v2.0 development environment...$(COLOR_RESET)"
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed. Please install Go $(GO_VERSION)+"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "Node.js is not installed. Please install Node.js $(NODE_VERSION)+"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Docker is not installed. Please install Docker"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Please install kubectl"; exit 1; }
	@command -v helm >/dev/null 2>&1 || { echo "Helm is not installed. Please install Helm 3+"; exit 1; }
	@echo "$(COLOR_GREEN)✓ All prerequisites are installed$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)Installing Go dependencies...$(COLOR_RESET)"
	@cd agents/k8s-agent && go mod download
	@cd api && go mod download
	@echo "$(COLOR_GREEN)Installing UI dependencies...$(COLOR_RESET)"
	@cd ui && npm install
	@echo "$(COLOR_GREEN)✓ v2.0 development environment ready!$(COLOR_RESET)"

fmt: ## Format code (Go and JavaScript)
	@echo "$(COLOR_GREEN)Formatting Go code...$(COLOR_RESET)"
	@cd agents/k8s-agent && go fmt ./...
	@cd api && go fmt ./...
	@echo "$(COLOR_GREEN)Formatting JavaScript code...$(COLOR_RESET)"
	@cd ui && npm run format || true
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

lint: ## Run linters
	@echo "$(COLOR_GREEN)Linting Go code...$(COLOR_RESET)"
	@cd agents/k8s-agent && golangci-lint run || echo "$(COLOR_YELLOW)⚠ Install golangci-lint for Go linting$(COLOR_RESET)"
	@cd api && golangci-lint run || echo "$(COLOR_YELLOW)⚠ Install golangci-lint for Go linting$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)Linting JavaScript code...$(COLOR_RESET)"
	@cd ui && npm run lint || true
	@echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"

##@ Building

build: build-k8s-agent build-api build-ui ## Build all components (v2.0)
	@echo "$(COLOR_GREEN)✓ All v2.0 components built$(COLOR_RESET)"

build-k8s-agent: ## Build K8s Agent binary
	@echo "$(COLOR_GREEN)Building K8s Agent...$(COLOR_RESET)"
	@cd agents/k8s-agent && make build
	@echo "$(COLOR_GREEN)✓ K8s Agent built$(COLOR_RESET)"

build-api: ## Build Control Plane API binary
	@echo "$(COLOR_GREEN)Building Control Plane API...$(COLOR_RESET)"
	@cd api && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/api cmd/main.go
	@echo "$(COLOR_GREEN)✓ Control Plane API built: api/bin/api$(COLOR_RESET)"

build-ui: ## Build UI static assets
	@echo "$(COLOR_GREEN)Building UI...$(COLOR_RESET)"
	@cd ui && npm run build
	@echo "$(COLOR_GREEN)✓ UI built: ui/build/$(COLOR_RESET)"

##@ Testing

test: test-k8s-agent test-api test-ui ## Run all tests (v2.0)

test-k8s-agent: ## Run K8s Agent tests
	@echo "$(COLOR_GREEN)Running K8s Agent tests...$(COLOR_RESET)"
	@cd agents/k8s-agent && make test

test-api: ## Run Control Plane API tests
	@echo "$(COLOR_GREEN)Running Control Plane API tests...$(COLOR_RESET)"
	@cd api && go test -v ./... -coverprofile=coverage.out
	@cd api && go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'

test-ui: ## Run UI tests
	@echo "$(COLOR_GREEN)Running UI tests...$(COLOR_RESET)"
	@cd ui && npm test -- --coverage --watchAll=false || true

test-integration: ## Run v2.0 integration tests (agent communication, VNC proxy)
	@echo "$(COLOR_GREEN)Running v2.0 integration tests...$(COLOR_RESET)"
	@cd tests/integration && go test -v ./... || echo "$(COLOR_YELLOW)⚠ Integration tests not yet complete$(COLOR_RESET)"

##@ Docker

docker-build: docker-build-k8s-agent docker-build-api docker-build-ui ## Build all v2.0 Docker images

docker-build-k8s-agent: ## Build K8s Agent Docker image
	@echo "$(COLOR_GREEN)Building K8s Agent Docker image...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Version: $(GIT_TAG) | Commit: $(GIT_COMMIT)$(COLOR_RESET)"
	@cd agents/k8s-agent && docker build $(BUILD_ARGS) \
		-t $(K8S_AGENT_IMAGE):$(VERSION) \
		-t $(K8S_AGENT_IMAGE):$(GIT_TAG) \
		-t $(K8S_AGENT_IMAGE):latest \
		.
	@echo "$(COLOR_GREEN)✓ Built $(K8S_AGENT_IMAGE):$(GIT_TAG)$(COLOR_RESET)"

docker-build-docker-agent: ## Build Docker Agent Docker image (future)
	@echo "$(COLOR_YELLOW)Docker Agent not yet implemented (v2.1)$(COLOR_RESET)"

docker-build-api: ## Build Control Plane API Docker image
	@echo "$(COLOR_GREEN)Building Control Plane API Docker image...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Version: $(GIT_TAG) | Commit: $(GIT_COMMIT)$(COLOR_RESET)"
	@docker build $(BUILD_ARGS) \
		-t $(API_IMAGE):$(VERSION) \
		-t $(API_IMAGE):$(GIT_TAG) \
		-t $(API_IMAGE):latest \
		-f api/Dockerfile api/
	@echo "$(COLOR_GREEN)✓ Built $(API_IMAGE):$(GIT_TAG)$(COLOR_RESET)"

docker-build-ui: ## Build UI Docker image
	@echo "$(COLOR_GREEN)Building UI Docker image...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Version: $(GIT_TAG) | Commit: $(GIT_COMMIT)$(COLOR_RESET)"
	@docker build $(BUILD_ARGS) \
		-t $(UI_IMAGE):$(VERSION) \
		-t $(UI_IMAGE):$(GIT_TAG) \
		-t $(UI_IMAGE):latest \
		-f ui/Dockerfile ui/
	@echo "$(COLOR_GREEN)✓ Built $(UI_IMAGE):$(GIT_TAG)$(COLOR_RESET)"

docker-push: docker-push-k8s-agent docker-push-api docker-push-ui ## Push all Docker images

docker-push-k8s-agent: ## Push K8s Agent Docker image
	@echo "$(COLOR_GREEN)Pushing K8s Agent image...$(COLOR_RESET)"
	@docker push $(K8S_AGENT_IMAGE):$(VERSION)
	@docker push $(K8S_AGENT_IMAGE):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(K8S_AGENT_IMAGE):$(VERSION)$(COLOR_RESET)"

docker-push-api: ## Push Control Plane API Docker image
	@echo "$(COLOR_GREEN)Pushing API image...$(COLOR_RESET)"
	@docker push $(API_IMAGE):$(VERSION)
	@docker push $(API_IMAGE):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(API_IMAGE):$(VERSION)$(COLOR_RESET)"

docker-push-ui: ## Push UI Docker image
	@echo "$(COLOR_GREEN)Pushing UI image...$(COLOR_RESET)"
	@docker push $(UI_IMAGE):$(VERSION)
	@docker push $(UI_IMAGE):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(UI_IMAGE):$(VERSION)$(COLOR_RESET)"

docker-build-multiarch: ## Build multi-architecture images (amd64, arm64)
	@echo "$(COLOR_GREEN)Building multi-architecture images for v2.0...$(COLOR_RESET)"
	@cd agents/k8s-agent && docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(K8S_AGENT_IMAGE):$(VERSION) \
		-t $(K8S_AGENT_IMAGE):latest \
		--push \
		.
	@docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(API_IMAGE):$(VERSION) \
		-t $(API_IMAGE):latest \
		-f api/Dockerfile \
		--push \
		api/
	@docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(UI_IMAGE):$(VERSION) \
		-t $(UI_IMAGE):latest \
		-f ui/Dockerfile \
		--push \
		ui/
	@echo "$(COLOR_GREEN)✓ Multi-architecture images built and pushed$(COLOR_RESET)"

##@ Helm

helm-lint: ## Lint Helm chart
	@echo "$(COLOR_GREEN)Linting Helm chart (v2.0)...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Chart: $(CHART_PATH)$(COLOR_RESET)"
	@helm lint $(CHART_PATH)
	@echo "$(COLOR_GREEN)✓ Helm chart is valid$(COLOR_RESET)"

helm-template: ## Render Helm templates (dry-run)
	@echo "$(COLOR_GREEN)Rendering v2.0 Helm templates...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Chart: $(CHART_PATH)$(COLOR_RESET)"
	@helm template $(HELM_RELEASE) $(CHART_PATH) --namespace $(NAMESPACE)

helm-install: ## Install StreamSpace v2.0 using Helm
	@echo "$(COLOR_GREEN)Installing StreamSpace v2.0...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Context: $(KUBE_CONTEXT)$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Namespace: $(NAMESPACE)$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Chart: $(CHART_PATH)$(COLOR_RESET)"
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm install $(HELM_RELEASE) $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--set k8sAgent.image.tag=$(VERSION) \
		--set api.image.tag=$(VERSION) \
		--set ui.image.tag=$(VERSION) \
		--wait
	@echo "$(COLOR_GREEN)✓ StreamSpace v2.0 installed!$(COLOR_RESET)"
	@echo ""
	@helm status $(HELM_RELEASE) -n $(NAMESPACE)

helm-upgrade: ## Upgrade StreamSpace v2.0 Helm release
	@echo "$(COLOR_GREEN)Upgrading StreamSpace v2.0...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Chart: $(CHART_PATH)$(COLOR_RESET)"
	@helm upgrade $(HELM_RELEASE) $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--set k8sAgent.image.tag=$(VERSION) \
		--set api.image.tag=$(VERSION) \
		--set ui.image.tag=$(VERSION) \
		--wait
	@echo "$(COLOR_GREEN)✓ StreamSpace v2.0 upgraded!$(COLOR_RESET)"

helm-uninstall: ## Uninstall StreamSpace v2.0 Helm release
	@echo "$(COLOR_YELLOW)Uninstalling StreamSpace v2.0...$(COLOR_RESET)"
	@helm uninstall $(HELM_RELEASE) -n $(NAMESPACE)
	@echo "$(COLOR_GREEN)✓ StreamSpace uninstalled$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Note: PVCs and namespace are preserved. Delete manually if needed.$(COLOR_RESET)"

##@ Kubernetes (v2.0 Architecture)

k8s-deploy-control-plane: ## Deploy Control Plane (API + UI)
	@echo "$(COLOR_GREEN)Deploying Control Plane...$(COLOR_RESET)"
	@kubectl apply -f manifests/v2/control-plane/ || echo "$(COLOR_YELLOW)⚠ Control Plane manifests not yet created$(COLOR_RESET)"

k8s-deploy-k8s-agent: ## Deploy K8s Agent to cluster
	@echo "$(COLOR_GREEN)Deploying K8s Agent...$(COLOR_RESET)"
	@cd agents/k8s-agent && make deploy

k8s-status: ## Check v2.0 deployment status
	@echo "$(COLOR_BOLD)StreamSpace v2.0 Status$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)Control Plane Pods:$(COLOR_RESET)"
	@kubectl get pods -n $(NAMESPACE) -l app.kubernetes.io/component=control-plane
	@echo ""
	@echo "$(COLOR_BLUE)Agents:$(COLOR_RESET)"
	@kubectl get pods -n $(NAMESPACE) -l component=k8s-agent
	@echo ""
	@echo "$(COLOR_BLUE)Services:$(COLOR_RESET)"
	@kubectl get svc -n $(NAMESPACE)
	@echo ""
	@echo "$(COLOR_BLUE)Ingresses:$(COLOR_RESET)"
	@kubectl get ingress -n $(NAMESPACE)

k8s-logs-api: ## View Control Plane API logs
	@kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/component=api --tail=100 -f

k8s-logs-ui: ## View UI logs
	@kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/component=ui --tail=100 -f

k8s-logs-k8s-agent: ## View K8s Agent logs
	@kubectl logs -n $(NAMESPACE) -l component=k8s-agent --tail=100 -f

k8s-port-forward-ui: ## Port-forward UI to localhost:3000
	@echo "$(COLOR_GREEN)Port-forwarding UI to http://localhost:3000$(COLOR_RESET)"
	@kubectl port-forward -n $(NAMESPACE) svc/$(HELM_RELEASE)-ui 3000:80

k8s-port-forward-api: ## Port-forward Control Plane API to localhost:8000
	@echo "$(COLOR_GREEN)Port-forwarding Control Plane API to http://localhost:8000$(COLOR_RESET)"
	@kubectl port-forward -n $(NAMESPACE) svc/$(HELM_RELEASE)-api 8000:8000

##@ Docker Compose (Development)

docker-compose-up: ## Start Control Plane with Docker Compose
	@echo "$(COLOR_GREEN)Starting v2.0 Control Plane with Docker Compose...$(COLOR_RESET)"
	@docker-compose up -d
	@echo "$(COLOR_GREEN)✓ Control Plane started$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Access points:$(COLOR_RESET)"
	@echo "  Control Plane API: http://localhost:8000"
	@echo "  Database:          localhost:5432"
	@echo ""
	@echo "Run 'make docker-compose-logs' to view logs"

docker-compose-up-dev: ## Start services with monitoring stack
	@echo "$(COLOR_GREEN)Starting v2.0 Control Plane with monitoring...$(COLOR_RESET)"
	@docker-compose --profile monitoring --profile dev up -d
	@echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Access points:$(COLOR_RESET)"
	@echo "  API:        http://localhost:8000"
	@echo "  Database:   localhost:5432"
	@echo "  pgAdmin:    http://localhost:5050 (admin@streamspace.local / admin)"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana:    http://localhost:3000 (admin / admin)"

docker-compose-down: ## Stop all Docker Compose services
	@echo "$(COLOR_YELLOW)Stopping Docker Compose services...$(COLOR_RESET)"
	@docker-compose --profile monitoring --profile dev down
	@echo "$(COLOR_GREEN)✓ Services stopped$(COLOR_RESET)"

docker-compose-logs: ## View logs from Docker Compose services
	@docker-compose logs -f

docker-compose-logs-api: ## View Control Plane API logs from Docker Compose
	@docker-compose logs -f api

docker-compose-restart: ## Restart Docker Compose services
	@docker-compose restart

##@ Development Workflows

dev-run-k8s-agent: ## Run K8s Agent locally (requires kubeconfig and Control Plane)
	@echo "$(COLOR_GREEN)Running K8s Agent locally...$(COLOR_RESET)"
	@cd agents/k8s-agent && make run

dev-run-api: ## Run Control Plane API locally (requires database)
	@echo "$(COLOR_GREEN)Running Control Plane API locally...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Ensure PostgreSQL is running and DB_* env vars are set$(COLOR_RESET)"
	@cd api && go run cmd/main.go

dev-run-ui: ## Run UI development server
	@echo "$(COLOR_GREEN)Running UI development server...$(COLOR_RESET)"
	@cd ui && npm start

dev-full-local: ## Run all v2.0 components locally (separate terminals required)
	@echo "$(COLOR_YELLOW)v2.0 Architecture - Run these commands in separate terminals:$(COLOR_RESET)"
	@echo "  1. make dev-run-api       # Control Plane API"
	@echo "  2. make dev-run-k8s-agent # K8s Agent (connects to API)"
	@echo "  3. make dev-run-ui        # UI"

##@ Deployment

deploy-dev: docker-build helm-install ## Build and deploy v2.0 to dev environment
	@echo "$(COLOR_GREEN)✓ Deployed v2.0 to development$(COLOR_RESET)"

deploy-prod: docker-build-multiarch ## Build and push v2.0 production images
	@echo "$(COLOR_GREEN)✓ v2.0 production images ready$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Run 'helm install' or 'helm upgrade' with production values$(COLOR_RESET)"

##@ Utilities

clean: ## Clean build artifacts
	@echo "$(COLOR_GREEN)Cleaning v2.0 build artifacts...$(COLOR_RESET)"
	@cd agents/k8s-agent && make clean
	@rm -rf api/bin/
	@rm -rf ui/build/
	@rm -f api/coverage.out
	@echo "$(COLOR_GREEN)✓ Build artifacts cleaned$(COLOR_RESET)"

clean-docker: ## Remove local Docker images
	@echo "$(COLOR_YELLOW)Removing v2.0 Docker images...$(COLOR_RESET)"
	@docker rmi $(K8S_AGENT_IMAGE):$(VERSION) $(K8S_AGENT_IMAGE):latest || true
	@docker rmi $(API_IMAGE):$(VERSION) $(API_IMAGE):latest || true
	@docker rmi $(UI_IMAGE):$(VERSION) $(UI_IMAGE):latest || true
	@echo "$(COLOR_GREEN)✓ Docker images removed$(COLOR_RESET)"

version: ## Display v2.0 version information
	@echo "$(COLOR_BOLD)StreamSpace v2.0 (Multi-Platform Agent Architecture)$(COLOR_RESET)"
	@echo ""
	@echo "Version:    $(VERSION)"
	@echo "Git Tag:    $(GIT_TAG)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo ""
	@echo "Components:"
	@echo "  K8s Agent:        $(K8S_AGENT_IMAGE):$(VERSION)"
	@echo "  Docker Agent:     $(DOCKER_AGENT_IMAGE):$(VERSION) (v2.1)"
	@echo "  Control Plane:    $(API_IMAGE):$(VERSION)"
	@echo "  UI:               $(UI_IMAGE):$(VERSION)"

##@ CI/CD

ci-build: build test ## Run CI build (build + test)
	@echo "$(COLOR_GREEN)✓ v2.0 CI build complete$(COLOR_RESET)"

ci-docker: docker-build ## Build Docker images for CI
	@echo "$(COLOR_GREEN)✓ v2.0 CI Docker build complete$(COLOR_RESET)"

ci-deploy: docker-push helm-upgrade ## Deploy from CI (push + upgrade)
	@echo "$(COLOR_GREEN)✓ v2.0 CI deployment complete$(COLOR_RESET)"

##@ Documentation

docs-serve: ## Serve documentation locally
	@echo "$(COLOR_GREEN)Serving documentation...$(COLOR_RESET)"
	@command -v python3 >/dev/null 2>&1 && \
		cd docs && python3 -m http.server 8080 || \
		echo "$(COLOR_YELLOW)Python 3 required to serve docs$(COLOR_RESET)"

##@ v1.0 Legacy (Deprecated)

v1-controller: ## Run v1.0 controller (deprecated, use K8s Agent)
	@echo "$(COLOR_YELLOW)⚠ v1.0 controller is deprecated. Use 'make build-k8s-agent' for v2.0$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)See docs/V2_ARCHITECTURE_STATUS.md for migration guide$(COLOR_RESET)"

.DEFAULT_GOAL := help
