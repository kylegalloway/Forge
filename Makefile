# Build targets
CTRL_TARGET ?= controller
WBHK_TARGET ?= webhook
# Image name to use for building/pushing image targets
CTRL_IMG ?= forge-controller:latest
WBHK_IMG ?= forge-webhook:latest
# Kubernetes namespace for deployment
NAMESPACE ?= forge-system

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

##@ Tooling

##@ go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo 'Downloading $(2)' ;\
GOBIN=$(shell go env GOPATH)/bin GOFLAGS=$(GOFLAGS) go install $(2) ;\
}
endef

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.20.0)

# Tooling binaries
CONTROLLER_GEN = $(shell go env GOPATH)/bin/controller-gen

# Kind cluster name for local development
KIND_CLUSTER_NAME ?= forge-dev

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd:crdVersions=v1,allowDangerousTypes=true paths=./pkg/apis/... output:crd:artifacts:config=chart/forge/crds

.PHONY: generate
generate: ## Generate API deepcopy code.
	go generate ./pkg/apis/...

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

.PHONY: e2e-test
e2e-test: ## Run E2E tests (creates Kind cluster, deploys Forge, runs tests, cleans up).
	@echo "Creating Kind cluster..."
	$(MAKE) kind-create
	@echo "Building and deploying Forge..."
	$(MAKE) kind-deploy
	@echo "Waiting for controller to be ready..."
	sleep 10
	@echo "Running E2E tests..."
	cd tests/e2e && ./run-all-tests.sh
	@echo "Cleaning up Kind cluster..."
	$(MAKE) kind-delete
	@echo "E2E tests complete!"

.PHONY: e2e-test-keep
e2e-test-keep: ## Run E2E tests and keep Kind cluster for debugging.
	@echo "Creating Kind cluster..."
	$(MAKE) kind-create
	@echo "Building and deploying Forge..."
	$(MAKE) kind-deploy
	@echo "Waiting for controller to be ready..."
	sleep 10
	@echo "Running E2E tests..."
	cd tests/e2e && ./run-all-tests.sh
	@echo "E2E tests complete! Kind cluster is still running."
	@echo "To cleanup: make kind-delete"

.PHONY: e2e-test-existing
e2e-test-existing: ## Run E2E tests against existing cluster (assumes Forge is already deployed).
	@echo "Running E2E tests against existing cluster..."
	cd tests/e2e && ./run-all-tests.sh
	@echo "E2E tests complete!"

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

##@ Build

.PHONY: build
build: build-controller build-webhook build-kubectl-forge ## Build controller, webhook, and kubectl plugin binaries.

.PHONY: build-controller
build-controller: fmt vet ## Build controller binary.
	go build -o bin/$(CTRL_TARGET) cmd/$(CTRL_TARGET)/main.go

.PHONY: build-webhook
build-webhook: fmt vet ## Build webhook binary.
	go build -o bin/$(WBHK_TARGET) cmd/$(WBHK_TARGET)/main.go

.PHONY: build-kubectl-forge
build-kubectl-forge: fmt vet ## Build kubectl-forge plugin binary.
	go build -o bin/kubectl-forge cmd/kubectl-forge/*.go

.PHONY: run
run: fmt vet ## Run controller from your host.
	go run cmd/controller/main.go -kubeconfig=${HOME}/.kube/config -v=2

.PHONY: docker-build
docker-build: ## Build container images with docker.
	docker build --target $(CTRL_TARGET) -t $(CTRL_IMG) .
	docker build --target $(WBHK_TARGET) -t $(WBHK_IMG) .

.PHONY: podman-build
podman-build: ## Build container images with podman.
	podman build --target $(CTRL_TARGET) --iidfile $(CTRL_TARGET).iid .
	podman tag "$$(cat $(CTRL_TARGET).iid)" $(CTRL_IMG)
	rm -f $(CTRL_TARGET).iid
	podman build --target $(WBHK_TARGET) --iidfile $(WBHK_TARGET).iid .
	podman tag "$$(cat $(WBHK_TARGET).iid)" $(WBHK_IMG)
	rm -f $(WBHK_TARGET).iid

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
	helm upgrade --install forge chart/forge \
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

##@ Status

.PHONY: status
status: ## Show status of controller and samples.
	@echo "=== Controller Status ==="
	@kubectl get pods -n $(NAMESPACE) -l app=forge-controller 2>/dev/null || echo "Controller not deployed"
	@echo ""
	@echo "=== Webhook Status ==="
	@kubectl get pods -n $(NAMESPACE) -l app=forge-webhook 2>/dev/null || echo "Controller not deployed"
	@echo ""
	@echo "=== ZarfPackageJob Resources ==="
	@kubectl get zarfpackagejobs --all-namespaces 2>/dev/null || echo "No ZarfPackageJob resources found"
	@echo ""
	@echo "=== UDSBundleJob Resources ==="
	@kubectl get udsbundlejobs --all-namespaces 2>/dev/null || echo "No UDSBundleJob resources found"
	@echo ""
	@echo "=== Jobs ==="
	@kubectl get jobs --all-namespaces -l app=forge 2>/dev/null || echo "No jobs found"

.PHONY: dev-controller-logs
dev-controller-logs: ## Tail logs from controller
	@echo "=== Controller Logs ==="
	@kubectl logs -n $(NAMESPACE) -l app=forge-controller --tail=30 --prefix=true 2>/dev/null || echo "No controller logs"
	@echo ""

.PHONY: dev-webhook-logs
dev-webhook-logs: ## Tail logs from webhook
	@echo "=== Webhook Logs ==="
	@kubectl logs -n $(NAMESPACE) -l app=forge-webhook --tail=30 --prefix=true 2>/dev/null || echo "No webhook logs"
	@echo ""

.PHONY: dev-job-logs
dev-job-logs: ## Tail logs from the latest job
	@echo "=== Latest Job Logs ==="
	@JOB=$$(kubectl get jobs -l app=forge --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -n "$$JOB" ]; then \
		kubectl logs job/$$JOB 2>/dev/null || echo "Job not yet started"; \
	else \
		echo "No jobs found"; \
	fi

.PHONY: dev-logs
dev-logs: dev-controller-logs dev-webhook-logs dev-job-logs ## Show all logs

##@ Cleanup

.PHONY: clean
clean: ## Clean up built binaries and temporary files.
	rm -rf bin/
	rm -f cover*.out

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
kind-load: podman-build ## Build and load the controller and webhook images into kind.
	@echo "Loading image $(CTRL_TARGET) into kind cluster $(KIND_CLUSTER_NAME)..."
	podman save $(CTRL_IMG) -o /tmp/forge-$(CTRL_TARGET).tar
	kind load image-archive /tmp/forge-$(CTRL_TARGET).tar --name $(KIND_CLUSTER_NAME)
	rm /tmp/forge-$(CTRL_TARGET).tar
	@echo "Loading image $(WBHK_TARGET) into kind cluster $(KIND_CLUSTER_NAME)..."
	podman save $(WBHK_IMG) -o /tmp/forge-$(WBHK_TARGET).tar
	kind load image-archive /tmp/forge-$(WBHK_TARGET).tar --name $(KIND_CLUSTER_NAME)
	rm /tmp/forge-$(WBHK_TARGET).tar

.PHONY: kind-load-docker
kind-load-docker: docker-build ## Build and load the controller image into kind using Docker.
	@echo "Loading image $(CTRL_IMG) into kind cluster $(KIND_CLUSTER_NAME) with Docker..."
	kind load docker-image $(CTRL_IMG) --name $(KIND_CLUSTER_NAME)
	@echo "Loading image $(WBHK_IMG) into kind cluster $(KIND_CLUSTER_NAME) with Docker..."
	kind load docker-image $(WBHK_IMG) --name $(KIND_CLUSTER_NAME)

.PHONY: kind-images
kind-images: ## List images in kind cluster.
	podman exec -it $(KIND_CLUSTER_NAME)-control-plane crictl images | grep forge

.PHONY: kind-deploy
kind-deploy: kind-load ## Build, load image to kind, and deploy controller with Helm.
	@echo "Deploying with Helm..."
	@helm upgrade --install forge chart/forge \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set controller.image.repository=localhost/forge-controller \
		--set controller.image.tag=latest \
		--set webhook.image.repository=localhost/forge-webhook \
		--set webhook.image.tag=latest \
		--set observability.deployStack=false \
		--wait
	@echo "Deployment complete. Checking status..."
	@sleep 3
	@$(MAKE) status

.PHONY: kind-redeploy
kind-redeploy: ## Uninstall old helm chart and deploy a new one.
	@echo "Uninstalling old Helm chart..."
	@$(MAKE) uninstall
	@sleep 5
	@echo "Deploying with Helm..."
	@$(MAKE) kind-deploy

.PHONY: kind-setup
kind-setup: kind-create kind-deploy ## Complete setup: create kind cluster and deploy controller and webhook.
	@echo ""
	@echo "==============================================="
	@echo "Kind cluster setup complete!"
	@echo "Cluster name: $(KIND_CLUSTER_NAME)"
	@echo "==============================================="
	@echo ""
	@echo ""
	@echo "Check status:"
	@echo "  make status"
	@echo ""

##@ Test/Samples

.PHONY: kind-zarf-cli
kind-zarf-cli: ## Build and load the Zarf CLI image into kind.
	@echo "Building and loading Zarf CLI image into kind cluster '$(KIND_CLUSTER_NAME)'..."
	podman build -t localhost/zarf:v0.68.1 images/zarf-cli/
	podman save localhost/zarf:v0.68.1 -o /tmp/zarf-cli.tar
	kind load image-archive /tmp/zarf-cli.tar --name $(KIND_CLUSTER_NAME)
	rm /tmp/zarf-cli.tar

##@ Release

.PHONY: release-patch
release-patch: ## Release a new patch version (0.0.X).
	@./scripts/release.sh patch

.PHONY: release-minor
release-minor: ## Release a new minor version (0.X.0).
	@./scripts/release.sh minor

.PHONY: release-major
release-major: ## Release a new major version (X.0.0).
	@./scripts/release.sh major
