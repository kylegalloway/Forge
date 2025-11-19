# Container runtime (podman or docker)
CONTAINER_RUNTIME ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || echo docker)

# Image name and tag
IMG ?= scriptrunner-scripts:latest

# Kind cluster name
KIND_CLUSTER_NAME ?= scriptrunner-dev

.PHONY: build
build: ## Build the scripts container image
	$(CONTAINER_RUNTIME) build -t $(IMG) .

.PHONY: load
load: build ## Build and load into kind cluster
	kind load docker-image $(IMG) --name $(KIND_CLUSTER_NAME)

.PHONY: push
push: ## Push the image to a registry
	$(CONTAINER_RUNTIME) push $(IMG)

.PHONY: test
test: ## Test the scripts locally
	@echo "Testing process-data.sh..."
	@$(CONTAINER_RUNTIME) run --rm $(IMG) /scripts/process-data.sh process 5
	@echo ""
	@echo "Testing report-status.py..."
	@$(CONTAINER_RUNTIME) run --rm -e INPUT_service=test $(IMG) /scripts/report-status.py json

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
