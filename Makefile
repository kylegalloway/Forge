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

.PHONY: install-crd
install-crd: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/manager/
	kubectl apply -f config/rbac/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/rbac/ --ignore-not-found=true
	kubectl delete -f config/manager/ --ignore-not-found=true

.PHONY: install
install: install-crd deploy ## Install CRDs and deploy controller.

.PHONY: uninstall
uninstall: undeploy uninstall-crd ## Undeploy controller and uninstall CRDs.

##@ Samples

.PHONY: apply-sample
apply-sample: ## Apply sample ZarfPackageJob resource.
	kubectl apply -f config/samples/forge_v1alpha1_forge.yaml

.PHONY: apply-custom-sample
apply-custom-sample: ## Apply custom script sample.
	kubectl apply -f config/samples/forge_custom_script.yaml

.PHONY: delete-samples
delete-samples: ## Delete sample resources.
	kubectl delete -f config/samples/ --ignore-not-found=true

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
kind-deploy: kind-load install ## Build, load image to kind, and deploy controller.
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
