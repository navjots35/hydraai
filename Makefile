# Variables
VERSION ?= latest
REGISTRY ?= hydraai
IMAGE_NAME = hydra-route
FULL_IMAGE = $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Go variables
GOBASE = $(shell pwd)
GOBIN = $(GOBASE)/bin
GOFILES = $(wildcard *.go)

# Build variables
LDFLAGS = -w -s -extldflags "-static"
BUILD_FLAGS = -a -installsuffix cgo

# Kubernetes variables
NAMESPACE = hydra-route-system
KUBECONFIG ?= ~/.kube/config

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean build test ## Clean, build and test

.PHONY: build
build: ## Build the binary
	@echo "Building hydra-route..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags='$(LDFLAGS)' \
		$(BUILD_FLAGS) \
		-o $(GOBIN)/hydra-route \
		./cmd/hydra-route
	@echo "Binary built at $(GOBIN)/hydra-route"

.PHONY: build-local
build-local: ## Build the binary for local OS
	@echo "Building hydra-route for local OS..."
	@go build -o $(GOBIN)/hydra-route ./cmd/hydra-route
	@echo "Local binary built at $(GOBIN)/hydra-route"

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

.PHONY: lint
lint: ## Run linter
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@gofmt -s -w .
	@go mod tidy

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(GOBIN)
	@rm -f coverage.out coverage.html
	@go clean

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image $(FULL_IMAGE)..."
	@docker build -t $(FULL_IMAGE) .
	@echo "Docker image built: $(FULL_IMAGE)"

.PHONY: docker-push
docker-push: docker-build ## Build and push Docker image
	@echo "Pushing Docker image $(FULL_IMAGE)..."
	@docker push $(FULL_IMAGE)
	@echo "Docker image pushed: $(FULL_IMAGE)"

.PHONY: docker-run
docker-run: ## Run Docker container locally
	@echo "Running Docker container..."
	@docker run --rm -it \
		-v $(KUBECONFIG):/root/.kube/config:ro \
		$(FULL_IMAGE) \
		--config=/etc/hydra-route/config.yaml \
		--dry-run=true

.PHONY: k8s-namespace
k8s-namespace: ## Create Kubernetes namespace
	@echo "Creating namespace $(NAMESPACE)..."
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

.PHONY: k8s-deploy
k8s-deploy: k8s-namespace ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f deploy/kubernetes/rbac.yaml
	@kubectl apply -f deploy/kubernetes/deployment.yaml
	@echo "Deployment applied. Checking status..."
	@kubectl wait --for=condition=ready pod -l app=hydra-route-controller -n $(NAMESPACE) --timeout=300s

.PHONY: k8s-undeploy
k8s-undeploy: ## Remove from Kubernetes
	@echo "Removing from Kubernetes..."
	@kubectl delete -f deploy/kubernetes/deployment.yaml --ignore-not-found=true
	@kubectl delete -f deploy/kubernetes/rbac.yaml --ignore-not-found=true
	@kubectl delete namespace $(NAMESPACE) --ignore-not-found=true

.PHONY: k8s-logs
k8s-logs: ## Show logs from Kubernetes deployment
	@kubectl logs -n $(NAMESPACE) deployment/hydra-route-controller -f

.PHONY: k8s-status
k8s-status: ## Show Kubernetes deployment status
	@echo "Deployment status:"
	@kubectl get all -n $(NAMESPACE)
	@echo "\nController logs (last 20 lines):"
	@kubectl logs -n $(NAMESPACE) deployment/hydra-route-controller --tail=20

.PHONY: k8s-config
k8s-config: ## Show current configuration
	@echo "Current configuration:"
	@kubectl get configmap -n $(NAMESPACE) hydra-route-config -o yaml

.PHONY: k8s-port-forward
k8s-port-forward: ## Port forward metrics endpoint
	@echo "Port forwarding metrics endpoint to localhost:8080..."
	@kubectl port-forward -n $(NAMESPACE) service/hydra-route-controller-metrics 8080:8080

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest

.PHONY: generate-config
generate-config: ## Generate sample configuration
	@echo "Generating sample configuration..."
	@mkdir -p config
	@cp config/default-config.yaml config/sample-config.yaml
	@echo "Sample configuration generated at config/sample-config.yaml"

.PHONY: run-local
run-local: build-local ## Run locally with dry-run enabled
	@echo "Running hydra-route locally..."
	@$(GOBIN)/hydra-route \
		--config=config/default-config.yaml \
		--dry-run=true \
		--log-level=debug

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

.PHONY: security-scan
security-scan: ## Run security scan
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	@gosec ./...

.PHONY: release
release: clean lint test docker-push ## Full release pipeline
	@echo "Release completed for version $(VERSION)"

.PHONY: dev-setup
dev-setup: install-tools deps ## Setup development environment
	@echo "Development environment setup complete"

# Example targets for different environments
.PHONY: deploy-dev
deploy-dev: ## Deploy to development environment
	@$(MAKE) k8s-deploy NAMESPACE=hydra-route-dev

.PHONY: deploy-staging
deploy-staging: ## Deploy to staging environment
	@$(MAKE) k8s-deploy NAMESPACE=hydra-route-staging

.PHONY: deploy-prod
deploy-prod: ## Deploy to production environment
	@$(MAKE) k8s-deploy NAMESPACE=hydra-route-prod VERSION=stable 