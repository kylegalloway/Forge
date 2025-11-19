#!/bin/bash
set -e

# ScriptRunner Development Environment Setup Script
# This script sets up a complete local development environment using kind

CLUSTER_NAME="${KIND_CLUSTER_NAME:-scriptrunner-dev}"
NAMESPACE="scriptrunner-system"

echo "=================================="
echo "ScriptRunner Dev Environment Setup"
echo "=================================="
echo ""

# Check prerequisites
echo "Checking prerequisites..."

command -v go >/dev/null 2>&1 || { echo "Error: go is not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "Error: kubectl is not installed"; exit 1; }
command -v kind >/dev/null 2>&1 || { echo "Error: kind is not installed"; exit 1; }

if command -v podman >/dev/null 2>&1; then
    CONTAINER_RUNTIME="podman"
    echo "✓ Using podman as container runtime"
elif command -v docker >/dev/null 2>&1; then
    CONTAINER_RUNTIME="docker"
    echo "✓ Using docker as container runtime"
else
    echo "Error: Neither podman nor docker is installed"
    exit 1
fi

echo "✓ All prerequisites found"
echo ""

# Check if cluster exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "Kind cluster '${CLUSTER_NAME}' already exists"
    read -p "Do you want to delete and recreate it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Deleting existing cluster..."
        kind delete cluster --name "${CLUSTER_NAME}"
    else
        echo "Using existing cluster"
    fi
fi

# Create cluster if it doesn't exist
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "Creating kind cluster '${CLUSTER_NAME}'..."
    kind create cluster --name "${CLUSTER_NAME}"
    echo "✓ Cluster created"
else
    echo "✓ Cluster ready"
fi
echo ""

# Set kubectl context
echo "Setting kubectl context..."
kubectl config use-context "kind-${CLUSTER_NAME}"
echo "✓ Context set"
echo ""

# Build and load controller image
echo "Building controller image..."
make container-build CONTAINER_RUNTIME="${CONTAINER_RUNTIME}"
echo "✓ Image built"
echo ""

echo "Loading image into kind cluster..."
kind load docker-image scriptrunner-controller:latest --name "${CLUSTER_NAME}"
echo "✓ Image loaded"
echo ""

# Install CRD
echo "Installing CRD..."
make install-crd
echo "✓ CRD installed"
echo ""

# Deploy controller
echo "Deploying controller..."
make deploy
echo "✓ Controller deployed"
echo ""

# Wait for controller to be ready
echo "Waiting for controller to be ready..."
kubectl wait --for=condition=available --timeout=60s deployment/scriptrunner-controller -n "${NAMESPACE}" 2>/dev/null || {
    echo "Warning: Controller deployment not ready yet. This might be normal if the image is large."
}
echo ""

# Show status
echo "=================================="
echo "Setup Complete!"
echo "=================================="
echo ""
echo "Cluster: ${CLUSTER_NAME}"
echo "Namespace: ${NAMESPACE}"
echo ""
echo "Quick commands:"
echo "  make status          - Show controller and resource status"
echo "  make apply-sample    - Create a sample ScriptRunner"
echo "  make dev-logs        - Show controller and job logs"
echo "  make kind-redeploy   - Rebuild and redeploy after code changes"
echo ""
echo "To get started, try:"
echo "  make apply-sample"
echo "  make dev-logs"
echo ""
