.PHONY: help build load test clean

# Detect container runtime
CONTAINER_RUNTIME ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
IMAGE_NAME := scriptrunner-zarf-builder
IMAGE_TAG := latest

help: ## Show this help message
	@echo "Zarf Package Builder - Makefile targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the Zarf builder container image
	$(CONTAINER_RUNTIME) build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "Built image: $(IMAGE_NAME):$(IMAGE_TAG)"

load: build ## Build and load image into kind cluster
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name scriptrunner-dev
	@echo "Loaded $(IMAGE_NAME):$(IMAGE_TAG) into kind cluster"

test: ## Test the Zarf builder image locally
	$(CONTAINER_RUNTIME) run --rm -it \
		-v $(PWD)/test:/workspace \
		$(IMAGE_NAME):$(IMAGE_TAG) \
		zarf version

shell: ## Open a shell in the builder container
	$(CONTAINER_RUNTIME) run --rm -it \
		-v $(PWD):/workspace \
		$(IMAGE_NAME):$(IMAGE_TAG) \
		/bin/bash

clean: ## Remove built image
	$(CONTAINER_RUNTIME) rmi $(IMAGE_NAME):$(IMAGE_TAG) || true

.DEFAULT_GOAL := help
