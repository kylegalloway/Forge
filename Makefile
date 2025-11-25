# Image URL to use all building/pushing image targets
IMG ?= forge-controller:latest
# Kubernetes namespace for deployment
NAMESPACE ?= forge-system
# Container runtime (docker or podman)
CONTAINER_RUNTIME ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || echo docker)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Kind cluster name for local development
KIND_CLUSTER_NAME ?= forge-dev

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: test-coverage
test-coverage: test ## Run tests and show coverage report.
	go tool cover -html=cover.out

.PHONY: test-validation
test-validation: ## Run YAML validation tests.
	go test ./pkg/validation -v

.PHONY: test-unit
test-unit: ## Run unit tests only (no integration tests).
	go test ./pkg/... -short

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

##@ Build

.PHONY: build
build: fmt vet ## Build controller binary.
	go build -o bin/controller cmd/controller/main.go

.PHONY: run
run: fmt vet ## Run controller from your host.
	go run cmd/controller/main.go -kubeconfig=${HOME}/.kube/config -v=2

.PHONY: container-build
container-build: ## Build container image with the controller.
	$(CONTAINER_RUNTIME) build -t ${IMG} .

.PHONY: container-push
container-push: ## Push container image with the controller.
	$(CONTAINER_RUNTIME) push ${IMG}

# Legacy aliases for backwards compatibility
.PHONY: docker-build
docker-build: container-build ## Alias for container-build (legacy).

.PHONY: docker-push
docker-push: container-push ## Alias for container-push (legacy).

##@ Deployment

# Helm values file (can be overridden)
HELM_VALUES ?= chart/forge/values.yaml

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart.
	helm lint chart/forge

.PHONY: helm-template
helm-template: ## Generate manifests from Helm chart (dry-run).
	helm template forge chart/forge -f $(HELM_VALUES) --namespace $(NAMESPACE)

.PHONY: install
install: ## Install Forge using Helm with default values.
	helm install forge chart/forge \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--wait

.PHONY: install-mature
install-mature: ## Install Forge for mature cluster (existing monitoring).
	helm install forge chart/forge \
		-f chart/forge/values-mature-cluster.yaml \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--wait

.PHONY: install-new
install-new: ## Install Forge with full observability stack for new cluster.
	helm install forge chart/forge \
		-f chart/forge/values-new-cluster.yaml \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--wait

.PHONY: upgrade
upgrade: ## Upgrade Forge installation using Helm.
	helm upgrade forge chart/forge \
		--namespace $(NAMESPACE) \
		--wait

.PHONY: uninstall
uninstall: ## Uninstall Forge using Helm.
	helm uninstall forge --namespace $(NAMESPACE) || true

# Note: Legacy raw manifest targets removed. Use Helm for all deployments.
# CRDs are included in the Helm chart's crds/ directory and installed automatically.

##@ Samples

.PHONY: apply-sample
apply-sample: ## Apply sample ZarfPackageJob resource.
	kubectl apply -f examples/samples/v1alpha1/build-only-git.yaml

.PHONY: delete-samples
delete-samples: ## Delete sample resources.
	kubectl delete -f examples/samples/ --ignore-not-found=true

##@ Status

.PHONY: status
status: ## Show status of controller and samples.
	@echo "=== Controller Status ==="
	@kubectl get pods -n $(NAMESPACE) -l app=forge-controller 2>/dev/null || echo "Controller not deployed"
	@echo ""
	@echo "=== ZarfPackageJob Resources ==="
	@kubectl get forges --all-namespaces 2>/dev/null || echo "No ZarfPackageJob resources found"
	@echo ""
	@echo "=== Jobs ==="
	@kubectl get jobs --all-namespaces -l app=forge 2>/dev/null || echo "No jobs found"

.PHONY: logs
logs: ## Show controller logs.
	kubectl logs -n $(NAMESPACE) -l app=forge-controller --tail=50 -f

##@ Cleanup

.PHONY: clean
clean: ## Clean up built binaries and temporary files.
	rm -rf bin/
	rm -f cover.out

##@ Testing Scripts

.PHONY: dev-setup
dev-setup: ## Run automated development environment setup script.
	@./scripts/dev-setup.sh

.PHONY: quick-test
quick-test: ## Run quick smoke test to verify controller works.
	@./scripts/quick-test.sh

.PHONY: e2e-test
e2e-test: ## Run comprehensive end-to-end test suite.
	@./scripts/test-e2e.sh

.PHONY: integration-test
integration-test: ## Run full integration test with Kind cluster.
	@./scripts/test-integration-kind.sh

.PHONY: integration-test-keep
integration-test-keep: ## Run integration test and keep cluster on success.
	@CLEANUP_ON_SUCCESS=false ./scripts/test-integration-kind.sh

.PHONY: integration-test-registry
integration-test-registry: ## Run integration test with Gitea registry for publish workflows.
	@./scripts/test-integration-registry.sh

.PHONY: integration-test-registry-keep
integration-test-registry-keep: ## Run registry integration test and keep cluster on success.
	@CLEANUP_ON_SUCCESS=false ./scripts/test-integration-registry.sh

##@ Local Development (Kind)

.PHONY: kind-create
kind-create: ## Create a kind cluster for local development.
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "Kind cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		echo "Creating kind cluster '$(KIND_CLUSTER_NAME)'..."; \
		kind create cluster --name $(KIND_CLUSTER_NAME); \
	fi

.PHONY: kind-delete
kind-delete: ## Delete the kind cluster.
	kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-load
kind-load: container-build ## Build and load the controller image into kind.
	@echo "Loading image $(IMG) into kind cluster $(KIND_CLUSTER_NAME)..."
	kind load docker-image $(IMG) --name $(KIND_CLUSTER_NAME)

.PHONY: kind-deploy
kind-deploy: kind-load ## Build, load image to kind, and deploy controller with Helm.
	@echo "Deploying with Helm..."
	@helm install forge chart/forge \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set controller.image.repository=forge-controller \
		--set controller.image.tag=latest \
		--set observability.deployStack=false \
		--wait
	@echo "Deployment complete. Checking status..."
	@sleep 3
	@$(MAKE) status

.PHONY: kind-redeploy
kind-redeploy: ## Rebuild, reload, and restart controller in kind (for iterative development).
	@echo "Rebuilding controller..."
	@$(MAKE) container-build
	@echo "Loading new image into kind..."
	@$(MAKE) kind-load
	@echo "Restarting controller pods..."
	@kubectl delete pods -n $(NAMESPACE) -l app=forge-controller --ignore-not-found=true
	@echo "Waiting for new pods to start..."
	@sleep 5
	@$(MAKE) status

.PHONY: kind-setup
kind-setup: kind-create kind-deploy ## Complete setup: create kind cluster and deploy controller.
	@echo ""
	@echo "==============================================="
	@echo "Kind cluster setup complete!"
	@echo "Cluster name: $(KIND_CLUSTER_NAME)"
	@echo "==============================================="
	@echo ""
	@echo "Try creating a sample ZarfPackageJob:"
	@echo "  make apply-sample"
	@echo ""
	@echo "Check status:"
	@echo "  make status"
	@echo ""

.PHONY: dev-logs
dev-logs: ## Tail logs from controller and latest job.
	@echo "=== Controller Logs ==="
	@kubectl logs -n $(NAMESPACE) -l app=forge-controller --tail=20 --prefix=true 2>/dev/null || echo "No controller logs"
	@echo ""
	@echo "=== Latest Job Logs ==="
	@JOB=$$(kubectl get jobs -l app=forge --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -n "$$JOB" ]; then \
		kubectl logs job/$$JOB 2>/dev/null || echo "Job not yet started"; \
	else \
		echo "No jobs found"; \
	fi
