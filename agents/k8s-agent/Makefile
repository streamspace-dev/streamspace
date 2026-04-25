.PHONY: help build test clean docker-build docker-push run fmt vet lint install deploy

# Configuration
BINARY_NAME := k8s-agent
DOCKER_REGISTRY := ghcr.io
DOCKER_ORG := streamspace
VERSION := v2.0.0
IMAGE_NAME := $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$(BINARY_NAME)

# Git information
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "$(VERSION)")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -X main.Version=$(GIT_TAG) \
           -X main.Commit=$(GIT_COMMIT) \
           -X main.BuildDate=$(BUILD_DATE)

# Colors
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

##@ General

help: ## Display this help message
	@echo "$(COLOR_BOLD)StreamSpace K8s Agent Makefile$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

fmt: ## Format Go code
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_GREEN)✓ Vet complete$(COLOR_RESET)"

lint: ## Run golangci-lint
	@echo "$(COLOR_GREEN)Running linters...$(COLOR_RESET)"
	@golangci-lint run || echo "$(COLOR_YELLOW)⚠ Install golangci-lint for linting$(COLOR_RESET)"

##@ Building

build: fmt vet ## Build the agent binary
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME)...$(COLOR_RESET)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		.
	@echo "$(COLOR_GREEN)✓ Built: bin/$(BINARY_NAME)$(COLOR_RESET)"

build-local: ## Build for local OS/arch
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME) for local platform...$(COLOR_RESET)"
	@go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		.
	@echo "$(COLOR_GREEN)✓ Built: bin/$(BINARY_NAME)$(COLOR_RESET)"

##@ Testing

test: ## Run unit tests
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"
	@go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'

test-coverage: test ## Run tests and display coverage
	@go tool cover -html=coverage.out

##@ Docker

docker-build: ## Build Docker image
	@echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Version: $(GIT_TAG) | Commit: $(GIT_COMMIT)$(COLOR_RESET)"
	@docker build \
		--build-arg VERSION=$(GIT_TAG) \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):$(GIT_TAG) \
		-t $(IMAGE_NAME):latest \
		.
	@echo "$(COLOR_GREEN)✓ Built $(IMAGE_NAME):$(GIT_TAG)$(COLOR_RESET)"

docker-push: ## Push Docker image
	@echo "$(COLOR_GREEN)Pushing Docker image...$(COLOR_RESET)"
	@docker push $(IMAGE_NAME):$(VERSION)
	@docker push $(IMAGE_NAME):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(IMAGE_NAME):$(VERSION)$(COLOR_RESET)"

docker-build-multiarch: ## Build multi-architecture image (amd64, arm64)
	@echo "$(COLOR_GREEN)Building multi-architecture image...$(COLOR_RESET)"
	@docker buildx build --platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(GIT_TAG) \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--push \
		.
	@echo "$(COLOR_GREEN)✓ Multi-arch image built and pushed$(COLOR_RESET)"

##@ Running

run: ## Run the agent locally (requires kubeconfig and Control Plane URL)
	@echo "$(COLOR_GREEN)Running $(BINARY_NAME) locally...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Required environment variables:$(COLOR_RESET)"
	@echo "  AGENT_ID=k8s-local-dev"
	@echo "  CONTROL_PLANE_URL=ws://localhost:8000"
	@echo ""
	@go run . \
		--agent-id=$${AGENT_ID:-k8s-local-dev} \
		--control-plane-url=$${CONTROL_PLANE_URL:-ws://localhost:8000} \
		--platform=kubernetes \
		--namespace=streamspace

run-debug: ## Run with debug logging
	@echo "$(COLOR_GREEN)Running $(BINARY_NAME) with debug logging...$(COLOR_RESET)"
	@LOG_LEVEL=debug go run . \
		--agent-id=$${AGENT_ID:-k8s-local-dev} \
		--control-plane-url=$${CONTROL_PLANE_URL:-ws://localhost:8000} \
		--platform=kubernetes \
		--namespace=streamspace

##@ Kubernetes Deployment

deploy: ## Deploy agent to Kubernetes cluster
	@echo "$(COLOR_GREEN)Deploying K8s Agent...$(COLOR_RESET)"
	@kubectl apply -f k8s/rbac.yaml
	@kubectl apply -f k8s/configmap.yaml
	@kubectl apply -f k8s/deployment.yaml
	@echo "$(COLOR_GREEN)✓ K8s Agent deployed$(COLOR_RESET)"

undeploy: ## Remove agent from Kubernetes cluster
	@echo "$(COLOR_YELLOW)Removing K8s Agent...$(COLOR_RESET)"
	@kubectl delete -f k8s/deployment.yaml --ignore-not-found=true
	@kubectl delete -f k8s/configmap.yaml --ignore-not-found=true
	@kubectl delete -f k8s/rbac.yaml --ignore-not-found=true
	@echo "$(COLOR_GREEN)✓ K8s Agent removed$(COLOR_RESET)"

status: ## Check agent deployment status
	@echo "$(COLOR_BOLD)K8s Agent Status$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)Pods:$(COLOR_RESET)"
	@kubectl get pods -n streamspace -l component=k8s-agent
	@echo ""
	@echo "$(COLOR_BLUE)Logs (last 20 lines):$(COLOR_RESET)"
	@kubectl logs -n streamspace -l component=k8s-agent --tail=20

logs: ## View agent logs
	@kubectl logs -n streamspace -l component=k8s-agent -f

##@ Utilities

clean: ## Clean build artifacts
	@echo "$(COLOR_GREEN)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf bin/
	@rm -f coverage.out
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"

version: ## Display version information
	@echo "$(COLOR_BOLD)K8s Agent $(VERSION)$(COLOR_RESET)"
	@echo ""
	@echo "Git Tag:    $(GIT_TAG)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Image:      $(IMAGE_NAME):$(VERSION)"

deps: ## Download Go dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	@go mod download
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"

tidy: ## Tidy Go modules
	@echo "$(COLOR_GREEN)Tidying Go modules...$(COLOR_RESET)"
	@go mod tidy
	@echo "$(COLOR_GREEN)✓ Modules tidied$(COLOR_RESET)"

##@ CI/CD

ci-build: fmt vet test build ## Run CI build (format, vet, test, build)
	@echo "$(COLOR_GREEN)✓ CI build complete$(COLOR_RESET)"

ci-docker: docker-build ## Build Docker image for CI
	@echo "$(COLOR_GREEN)✓ CI Docker build complete$(COLOR_RESET)"

.DEFAULT_GOAL := help
