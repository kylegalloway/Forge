# Image URL to use all building/pushing image targets
IMG ?= scriptrunner-controller:latest
# Kubernetes namespace for deployment
NAMESPACE ?= scriptrunner-system

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

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

.PHONY: docker-build
docker-build: ## Build docker image with the controller.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the controller.
	docker push ${IMG}

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
apply-sample: ## Apply sample ScriptRunner resource.
	kubectl apply -f config/samples/scriptrunner_v1alpha1_scriptrunner.yaml

.PHONY: apply-custom-sample
apply-custom-sample: ## Apply custom script sample.
	kubectl apply -f config/samples/scriptrunner_custom_script.yaml

.PHONY: delete-samples
delete-samples: ## Delete sample resources.
	kubectl delete -f config/samples/ --ignore-not-found=true

##@ Status

.PHONY: status
status: ## Show status of controller and samples.
	@echo "=== Controller Status ==="
	@kubectl get pods -n $(NAMESPACE) -l app=scriptrunner-controller 2>/dev/null || echo "Controller not deployed"
	@echo ""
	@echo "=== ScriptRunner Resources ==="
	@kubectl get scriptrunners --all-namespaces 2>/dev/null || echo "No ScriptRunner resources found"
	@echo ""
	@echo "=== Jobs ==="
	@kubectl get jobs --all-namespaces -l app=scriptrunner 2>/dev/null || echo "No jobs found"

.PHONY: logs
logs: ## Show controller logs.
	kubectl logs -n $(NAMESPACE) -l app=scriptrunner-controller --tail=50 -f

##@ Cleanup

.PHONY: clean
clean: ## Clean up built binaries and temporary files.
	rm -rf bin/
	rm -f cover.out
