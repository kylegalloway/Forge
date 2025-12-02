#!/bin/bash
set -e

CLUSTER_NAME="${CLUSTER_NAME:-forge-demo}"
VERSION=$(cat .forge-version 2>/dev/null || echo "demo")
ZARF_VERSION="${ZARF_VERSION:-v0.66.0}"

echo "🎪 Setting up Kind cluster: ${CLUSTER_NAME}"
echo "📦 Using Forge version: ${VERSION}"
echo "🐋 Container runtime: podman"

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo ""
  echo "⚠️  Kind cluster '${CLUSTER_NAME}' already exists!"
  echo ""
  echo "Options:"
  echo "  1) Delete existing cluster and create new one"
  echo "  2) Keep existing cluster and skip to image loading"
  echo "  3) Exit script"
  echo ""
  read -p "Enter your choice (1/2/3): " choice

  case $choice in
    1)
      echo "🗑️  Deleting existing cluster..."
      kind delete cluster --name ${CLUSTER_NAME}
      echo "✅ Cluster deleted"
      ;;
    2)
      echo "⏩ Keeping existing cluster, skipping creation..."
      SKIP_CLUSTER_CREATE=true
      ;;
    3)
      echo "👋 Exiting..."
      exit 0
      ;;
    *)
      echo "❌ Invalid choice. Exiting..."
      exit 1
      ;;
  esac
fi

# Create Kind cluster with port mappings for Grafana
if [ "$SKIP_CLUSTER_CREATE" != "true" ]; then
  echo ""
  echo "Creating Kind cluster with increased memory for build jobs..."
cat <<EOF | kind create cluster --name ${CLUSTER_NAME} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  # Increase memory for the control plane to handle Zarf build jobs
  # Default is 4GB, we need at least 6GB for builds
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        "max-mutating-requests-inflight": "100"
        "max-requests-inflight": "200"
  extraPortMappings:
  - containerPort: 30000
    hostPort: 3000
    protocol: TCP
EOF

# Install kube-prometheus-stack (Prometheus + Grafana)
echo ""
echo "📊 Installing kube-prometheus-stack (Prometheus + Grafana)..."
echo "ℹ️  Using minimal resource requests to leave memory for build jobs..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo update

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.service.type=NodePort \
  --set grafana.service.nodePort=30000 \
  --set prometheus.prometheusSpec.resources.requests.cpu=50m \
  --set prometheus.prometheusSpec.resources.requests.memory=256Mi \
  --set prometheus.prometheusSpec.resources.limits.cpu=200m \
  --set prometheus.prometheusSpec.resources.limits.memory=512Mi \
  --set grafana.resources.requests.cpu=50m \
  --set grafana.resources.requests.memory=128Mi \
  --set grafana.resources.limits.cpu=200m \
  --set grafana.resources.limits.memory=256Mi \
  --set prometheusOperator.resources.requests.cpu=50m \
  --set prometheusOperator.resources.requests.memory=128Mi \
  --set prometheusOperator.resources.limits.cpu=200m \
  --set prometheusOperator.resources.limits.memory=256Mi \
  --set alertmanager.enabled=false \
  --set kubeStateMetrics.resources.requests.cpu=10m \
  --set kubeStateMetrics.resources.requests.memory=32Mi \
  --set nodeExporter.resources.requests.cpu=10m \
  --set nodeExporter.resources.requests.memory=32Mi \
  --timeout 10m \
  --wait
else
  echo "⏩ Skipping kube-prometheus-stack installation (already exists or cluster already configured)"
fi

# Load images into Kind (always using podman)
echo ""
echo "📥 Loading images into Kind cluster..."
echo "Using Podman - exporting images to tar..."

podman save localhost/forge-controller:${VERSION} -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name ${CLUSTER_NAME}
rm /tmp/forge-controller.tar

podman save localhost/zarf:${ZARF_VERSION} -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name ${CLUSTER_NAME}
rm /tmp/zarf-cli.tar

# Verify images are loaded
echo ""
echo "Verifying images in Kind cluster..."
podman exec ${CLUSTER_NAME}-control-plane crictl images | grep -E 'forge-controller|zarf' || echo "Warning: Images not found in cluster"

# Install Forge with Helm using Kind values file
echo ""
echo "🔥 Installing Forge with Helm..."
helm upgrade --install forge ./chart/forge \
  -f chart/forge/examples/values-kind.yaml \
  --set controller.image.tag=${VERSION} \
  --namespace forge-system \
  --create-namespace \
  --wait \
  --timeout 5m

echo ""
echo "✅ Kind cluster setup complete!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Cluster Information"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Cluster name: ${CLUSTER_NAME}"
echo "Controller version: ${VERSION}"
echo "Zarf version: ${ZARF_VERSION}"
echo ""
echo "🌐 Grafana: http://localhost:3000"
echo "   Username: admin"
echo "   Password: (run command below)"
echo ""
echo "   kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath=\"{.data.admin-password}\" | base64 -d"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "⚠️  Resource Constraints:"
echo "   Build jobs request 512Mi memory (limit 2Gi). The monitoring stack has"
echo "   been configured with minimal resources to leave room for build jobs."
echo "   If you still see 'Insufficient memory' errors, you may need to:"
echo "   - Increase Docker/Podman VM memory to 6GB+"
echo "   - Stop other running containers"
echo "   - Reduce monitoring component resources further"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📝 Next steps:"
echo "   1. Wait for all pods: kubectl wait --for=condition=Ready pods --all -n forge-system --timeout=300s"
echo "   2. Run tests: ./scripts/local-dev/3-test-api.sh"
echo "   3. Access Grafana: ./scripts/local-dev/4-grafana-metrics.sh"
echo ""
