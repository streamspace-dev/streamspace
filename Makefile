.PHONY: help build test deploy clean docker-build docker-push helm-install helm-upgrade helm-uninstall dev-setup

# Project configuration
PROJECT_NAME := streamspace
DOCKER_REGISTRY := ghcr.io
DOCKER_ORG := streamspace
VERSION := v0.2.0

# Git information for versioning
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "$(VERSION)")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Component images
CONTROLLER_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/streamspace-controller
API_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/streamspace-api
UI_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/streamspace-ui

# Kubernetes configuration
NAMESPACE := streamspace
HELM_RELEASE := streamspace
KUBE_CONTEXT := $(shell kubectl config current-context)

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
	@echo "$(COLOR_BOLD)StreamSpace Development Makefile$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

dev-setup: ## Set up development environment
	@echo "$(COLOR_GREEN)Setting up development environment...$(COLOR_RESET)"
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed. Please install Go $(GO_VERSION)+"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "Node.js is not installed. Please install Node.js $(NODE_VERSION)+"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Docker is not installed. Please install Docker"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Please install kubectl"; exit 1; }
	@command -v helm >/dev/null 2>&1 || { echo "Helm is not installed. Please install Helm 3+"; exit 1; }
	@echo "$(COLOR_GREEN)✓ All prerequisites are installed$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)Installing Go dependencies...$(COLOR_RESET)"
	@cd controller && go mod download
	@echo "$(COLOR_GREEN)Installing UI dependencies...$(COLOR_RESET)"
	@cd ui && npm install
	@echo "$(COLOR_GREEN)✓ Development environment ready!$(COLOR_RESET)"

fmt: ## Format code (Go and JavaScript)
	@echo "$(COLOR_GREEN)Formatting Go code...$(COLOR_RESET)"
	@cd controller && go fmt ./...
	@cd api && go fmt ./...
	@echo "$(COLOR_GREEN)Formatting JavaScript code...$(COLOR_RESET)"
	@cd ui && npm run format || true
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

lint: ## Run linters
	@echo "$(COLOR_GREEN)Linting Go code...$(COLOR_RESET)"
	@cd controller && golangci-lint run || echo "$(COLOR_YELLOW)⚠ Install golangci-lint for Go linting$(COLOR_RESET)"
	@cd api && golangci-lint run || echo "$(COLOR_YELLOW)⚠ Install golangci-lint for Go linting$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)Linting JavaScript code...$(COLOR_RESET)"
	@cd ui && npm run lint || true
	@echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"

##@ Building

build: build-controller build-api build-ui ## Build all components

build-controller: ## Build controller binary
	@echo "$(COLOR_GREEN)Building controller...$(COLOR_RESET)"
	@cd controller && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/manager cmd/main.go
	@echo "$(COLOR_GREEN)✓ Controller built: controller/bin/manager$(COLOR_RESET)"

build-api: ## Build API binary
	@echo "$(COLOR_GREEN)Building API...$(COLOR_RESET)"
	@cd api && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/api cmd/main.go
	@echo "$(COLOR_GREEN)✓ API built: api/bin/api$(COLOR_RESET)"

build-ui: ## Build UI static assets
	@echo "$(COLOR_GREEN)Building UI...$(COLOR_RESET)"
	@cd ui && npm run build
	@echo "$(COLOR_GREEN)✓ UI built: ui/build/$(COLOR_RESET)"

##@ Testing

test: test-controller test-api test-ui ## Run all tests

test-controller: ## Run controller tests
	@echo "$(COLOR_GREEN)Running controller tests...$(COLOR_RESET)"
	@cd controller && go test -v ./... -coverprofile=coverage.out
	@cd controller && go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'

test-api: ## Run API tests
	@echo "$(COLOR_GREEN)Running API tests...$(COLOR_RESET)"
	@cd api && go test -v ./... -coverprofile=coverage.out
	@cd api && go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'

test-ui: ## Run UI tests
	@echo "$(COLOR_GREEN)Running UI tests...$(COLOR_RESET)"
	@cd ui && npm test -- --coverage --watchAll=false || true

test-integration: ## Run integration tests
	@echo "$(COLOR_GREEN)Running integration tests...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Integration tests not yet implemented$(COLOR_RESET)"

##@ Docker

docker-build: docker-build-controller docker-build-api docker-build-ui ## Build all Docker images

docker-build-controller: ## Build controller Docker image
	@echo "$(COLOR_GREEN)Building controller Docker image...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Version: $(GIT_TAG) | Commit: $(GIT_COMMIT)$(COLOR_RESET)"
	@docker build $(BUILD_ARGS) \
		-t $(CONTROLLER_IMAGE):$(VERSION) \
		-t $(CONTROLLER_IMAGE):$(GIT_TAG) \
		-t $(CONTROLLER_IMAGE):latest \
		-f controller/Dockerfile controller/
	@echo "$(COLOR_GREEN)✓ Built $(CONTROLLER_IMAGE):$(GIT_TAG)$(COLOR_RESET)"

docker-build-api: ## Build API Docker image
	@echo "$(COLOR_GREEN)Building API Docker image...$(COLOR_RESET)"
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

docker-push: docker-push-controller docker-push-api docker-push-ui ## Push all Docker images

docker-push-controller: ## Push controller Docker image
	@echo "$(COLOR_GREEN)Pushing controller image...$(COLOR_RESET)"
	@docker push $(CONTROLLER_IMAGE):$(VERSION)
	@docker push $(CONTROLLER_IMAGE):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(CONTROLLER_IMAGE):$(VERSION)$(COLOR_RESET)"

docker-push-api: ## Push API Docker image
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
	@echo "$(COLOR_GREEN)Building multi-architecture images...$(COLOR_RESET)"
	@docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(CONTROLLER_IMAGE):$(VERSION) \
		-t $(CONTROLLER_IMAGE):latest \
		-f controller/Dockerfile \
		--push \
		controller/
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
	@echo "$(COLOR_GREEN)Linting Helm chart...$(COLOR_RESET)"
	@helm lint chart/
	@echo "$(COLOR_GREEN)✓ Helm chart is valid$(COLOR_RESET)"

helm-template: ## Render Helm templates (dry-run)
	@echo "$(COLOR_GREEN)Rendering Helm templates...$(COLOR_RESET)"
	@helm template $(HELM_RELEASE) chart/ --namespace $(NAMESPACE)

helm-install: ## Install StreamSpace using Helm
	@echo "$(COLOR_GREEN)Installing StreamSpace...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Context: $(KUBE_CONTEXT)$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Namespace: $(NAMESPACE)$(COLOR_RESET)"
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm install $(HELM_RELEASE) chart/ \
		--namespace $(NAMESPACE) \
		--set controller.image.tag=$(VERSION) \
		--set api.image.tag=$(VERSION) \
		--set ui.image.tag=$(VERSION) \
		--wait
	@echo "$(COLOR_GREEN)✓ StreamSpace installed!$(COLOR_RESET)"
	@echo ""
	@helm status $(HELM_RELEASE) -n $(NAMESPACE)

helm-upgrade: ## Upgrade StreamSpace Helm release
	@echo "$(COLOR_GREEN)Upgrading StreamSpace...$(COLOR_RESET)"
	@helm upgrade $(HELM_RELEASE) chart/ \
		--namespace $(NAMESPACE) \
		--set controller.image.tag=$(VERSION) \
		--set api.image.tag=$(VERSION) \
		--set ui.image.tag=$(VERSION) \
		--wait
	@echo "$(COLOR_GREEN)✓ StreamSpace upgraded!$(COLOR_RESET)"

helm-uninstall: ## Uninstall StreamSpace Helm release
	@echo "$(COLOR_YELLOW)Uninstalling StreamSpace...$(COLOR_RESET)"
	@helm uninstall $(HELM_RELEASE) -n $(NAMESPACE)
	@echo "$(COLOR_GREEN)✓ StreamSpace uninstalled$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Note: PVCs and namespace are preserved. Delete manually if needed.$(COLOR_RESET)"

##@ Kubernetes

k8s-apply-crds: ## Apply CRDs to cluster
	@echo "$(COLOR_GREEN)Applying CRDs...$(COLOR_RESET)"
	@kubectl apply -f manifests/crds/session.yaml
	@kubectl apply -f manifests/crds/template.yaml
	@echo "$(COLOR_GREEN)✓ CRDs applied$(COLOR_RESET)"

k8s-apply-templates: ## Apply application templates
	@echo "$(COLOR_GREEN)Applying templates...$(COLOR_RESET)"
	@kubectl apply -f manifests/templates/ -n $(NAMESPACE)
	@echo "$(COLOR_GREEN)✓ Templates applied$(COLOR_RESET)"

k8s-status: ## Check deployment status
	@echo "$(COLOR_BOLD)StreamSpace Status$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)Pods:$(COLOR_RESET)"
	@kubectl get pods -n $(NAMESPACE)
	@echo ""
	@echo "$(COLOR_BLUE)Services:$(COLOR_RESET)"
	@kubectl get svc -n $(NAMESPACE)
	@echo ""
	@echo "$(COLOR_BLUE)Ingresses:$(COLOR_RESET)"
	@kubectl get ingress -n $(NAMESPACE)
	@echo ""
	@echo "$(COLOR_BLUE)Sessions:$(COLOR_RESET)"
	@kubectl get sessions -n $(NAMESPACE)
	@echo ""
	@echo "$(COLOR_BLUE)Templates:$(COLOR_RESET)"
	@kubectl get templates -n $(NAMESPACE)

k8s-logs-controller: ## View controller logs
	@kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/component=controller --tail=100 -f

k8s-logs-api: ## View API logs
	@kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/component=api --tail=100 -f

k8s-logs-ui: ## View UI logs
	@kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/component=ui --tail=100 -f

k8s-port-forward-ui: ## Port-forward UI to localhost:3000
	@echo "$(COLOR_GREEN)Port-forwarding UI to http://localhost:3000$(COLOR_RESET)"
	@kubectl port-forward -n $(NAMESPACE) svc/$(HELM_RELEASE)-ui 3000:80

k8s-port-forward-api: ## Port-forward API to localhost:8000
	@echo "$(COLOR_GREEN)Port-forwarding API to http://localhost:8000$(COLOR_RESET)"
	@kubectl port-forward -n $(NAMESPACE) svc/$(HELM_RELEASE)-api 8000:8000

##@ Docker Compose

docker-compose-up: ## Start all services with Docker Compose
	@echo "$(COLOR_GREEN)Starting services with Docker Compose...$(COLOR_RESET)"
	@docker-compose up -d
	@echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Access points:$(COLOR_RESET)"
	@echo "  API:      http://localhost:8000"
	@echo "  Database: localhost:5432"
	@echo ""
	@echo "Run 'make docker-compose-logs' to view logs"

docker-compose-up-dev: ## Start services with monitoring stack
	@echo "$(COLOR_GREEN)Starting services with monitoring...$(COLOR_RESET)"
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

docker-compose-logs-api: ## View API logs from Docker Compose
	@docker-compose logs -f api

docker-compose-restart: ## Restart Docker Compose services
	@docker-compose restart

##@ Development Workflows

dev-run-controller: ## Run controller locally (requires kubeconfig)
	@echo "$(COLOR_GREEN)Running controller locally...$(COLOR_RESET)"
	@cd controller && go run cmd/main.go

dev-run-api: ## Run API locally (requires database)
	@echo "$(COLOR_GREEN)Running API locally...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Ensure PostgreSQL is running and DB_* env vars are set$(COLOR_RESET)"
	@cd api && go run cmd/main.go

dev-run-ui: ## Run UI development server
	@echo "$(COLOR_GREEN)Running UI development server...$(COLOR_RESET)"
	@cd ui && npm start

dev-full-local: ## Run all components locally (separate terminals required)
	@echo "$(COLOR_YELLOW)Run these commands in separate terminals:$(COLOR_RESET)"
	@echo "  make dev-run-controller"
	@echo "  make dev-run-api"
	@echo "  make dev-run-ui"

##@ Deployment

deploy-dev: docker-build helm-install ## Build and deploy to dev environment
	@echo "$(COLOR_GREEN)✓ Deployed to development$(COLOR_RESET)"

deploy-prod: docker-build-multiarch ## Build and push production images
	@echo "$(COLOR_GREEN)✓ Production images ready$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Run 'helm install' or 'helm upgrade' with production values$(COLOR_RESET)"

##@ Utilities

generate-templates: ## Generate 200+ application templates
	@echo "$(COLOR_GREEN)Generating templates from LinuxServer.io catalog...$(COLOR_RESET)"
	@python3 scripts/generate-templates.py
	@echo "$(COLOR_GREEN)✓ Templates generated in manifests/templates/$(COLOR_RESET)"

clean: ## Clean build artifacts
	@echo "$(COLOR_GREEN)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf controller/bin/
	@rm -rf api/bin/
	@rm -rf ui/build/
	@rm -f controller/coverage.out
	@rm -f api/coverage.out
	@echo "$(COLOR_GREEN)✓ Build artifacts cleaned$(COLOR_RESET)"

clean-docker: ## Remove local Docker images
	@echo "$(COLOR_YELLOW)Removing local Docker images...$(COLOR_RESET)"
	@docker rmi $(CONTROLLER_IMAGE):$(VERSION) $(CONTROLLER_IMAGE):latest || true
	@docker rmi $(API_IMAGE):$(VERSION) $(API_IMAGE):latest || true
	@docker rmi $(UI_IMAGE):$(VERSION) $(UI_IMAGE):latest || true
	@echo "$(COLOR_GREEN)✓ Docker images removed$(COLOR_RESET)"

version: ## Display project version
	@echo "$(COLOR_BOLD)StreamSpace $(VERSION)$(COLOR_RESET)"
	@echo ""
	@echo "Components:"
	@echo "  Controller: $(CONTROLLER_IMAGE):$(VERSION)"
	@echo "  API:        $(API_IMAGE):$(VERSION)"
	@echo "  UI:         $(UI_IMAGE):$(VERSION)"

##@ CI/CD

ci-build: build test ## Run CI build (build + test)
	@echo "$(COLOR_GREEN)✓ CI build complete$(COLOR_RESET)"

ci-docker: docker-build ## Build Docker images for CI
	@echo "$(COLOR_GREEN)✓ CI Docker build complete$(COLOR_RESET)"

ci-deploy: docker-push helm-upgrade ## Deploy from CI (push + upgrade)
	@echo "$(COLOR_GREEN)✓ CI deployment complete$(COLOR_RESET)"

##@ Documentation

docs-serve: ## Serve documentation locally
	@echo "$(COLOR_GREEN)Serving documentation...$(COLOR_RESET)"
	@command -v python3 >/dev/null 2>&1 && \
		cd docs && python3 -m http.server 8080 || \
		echo "$(COLOR_YELLOW)Python 3 required to serve docs$(COLOR_RESET)"

docs-generate: ## Generate documentation
	@echo "$(COLOR_YELLOW)Documentation generation not yet implemented$(COLOR_RESET)"

.DEFAULT_GOAL := help
