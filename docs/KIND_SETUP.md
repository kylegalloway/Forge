# Forge Kind Setup Guide

Complete guide for running Forge in a local Kind cluster with full observability.

## Prerequisites

- **kind** - `brew install kind` (or from https://kind.sigs.k8s.io/)
- **kubectl** - `brew install kubectl`
- **helm** - `brew install helm`
- **docker** or **podman** - For building images
- **make** - For build commands

## Quick Start (Copy-Paste Ready)

```bash
# 1. Create Kind cluster with port mappings for Grafana
cat <<EOF | kind create cluster --name forge-demo --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 3000
    protocol: TCP
EOF

# 2. Install kube-prometheus-stack (Prometheus + Grafana)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.service.type=NodePort \
  --set grafana.service.nodePort=30000 \
  --timeout 10m \
  --wait

# 3. Build Forge controller image
make container-build IMG=forge-controller:demo

# 4. Load image into Kind
# For Docker users:
kind load docker-image forge-controller:demo --name forge-demo

# For Podman users:
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar && \
kind load image-archive /tmp/forge-controller.tar --name forge-demo && \
rm /tmp/forge-controller.tar

# 5. Install Forge
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-kind.yaml \
  --namespace forge-system \
  --create-namespace

# 6. Wait for everything to be ready
kubectl wait --for=condition=Ready pods --all -n forge-system --timeout=300s
kubectl wait --for=condition=Ready pods --all -n monitoring --timeout=300s

# 7. Access Grafana
open http://localhost:3000
# Username: admin
# Password: prom-operator (default for kube-prometheus-stack)
```

## Step-by-Step Instructions

### 1. Create Kind Cluster

The Kind cluster needs port mappings to access Grafana from your host:

```bash
cat <<EOF | kind create cluster --name forge-demo --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 3000
    protocol: TCP
EOF
```

Verify cluster is running:
```bash
kubectl cluster-info
kubectl get nodes
```

### 2. Install Monitoring Stack

Install kube-prometheus-stack which provides Prometheus, Grafana, and Alertmanager:

```bash
# Add Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install with NodePort for Grafana
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.service.type=NodePort \
  --set grafana.service.nodePort=30000 \
  --timeout 10m \
  --wait
```

**Why these settings?**
- `serviceMonitorSelectorNilUsesHelmValues=false` - Allows Prometheus to discover ServiceMonitors in any namespace
- `grafana.service.type=NodePort` - Exposes Grafana on port 30000 (mapped to host port 3000)
- `--timeout 10m` - kube-prometheus-stack deploys many resources and needs more than the default 5m timeout

Verify monitoring stack is running:
```bash
kubectl get pods -n monitoring
```

### 3. Build Forge Images

Build the controller image locally:

```bash
# From the forge project root
make container-build IMG=forge-controller:demo
```

This builds the image and tags it as `forge-controller:demo`.

### 4. Load Images into Kind

Kind clusters can't pull from your local Docker/Podman registry, so you need to load images explicitly:

**For Docker users:**
```bash
kind load docker-image forge-controller:demo --name forge-demo
```

**For Podman users:**
```bash
# Export to tar, load into Kind, cleanup
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar
```

Verify the image is loaded:
```bash
docker exec -it forge-demo-control-plane crictl images | grep forge
```

### 5. Install Forge

Install Forge using the Kind-specific values file:

```bash
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-kind.yaml \
  --namespace forge-system \
  --create-namespace
```

**What gets deployed:**
- Forge controller (1 replica)
- OTEL Collector
- ServiceMonitor (for Prometheus integration)
- Metrics Service

**What does NOT get deployed:**
- Grafana (provided by kube-prometheus-stack)
- Prometheus (provided by kube-prometheus-stack)

### 6. Verify Installation

Check all pods are running:
```bash
kubectl get pods -n forge-system
kubectl get pods -n monitoring
```

Expected output in `forge-system`:
```
NAME                                  READY   STATUS    RESTARTS   AGE
forge-controller-xxxxx                1/1     Running   0          1m
forge-otel-collector-xxxxx            1/1     Running   0          1m
```

Check ServiceMonitor is created:
```bash
kubectl get servicemonitor -n forge-system
```

### 7. Access Grafana

Grafana is exposed on NodePort 30000, which maps to host port 3000:

```bash
# Open in browser
open http://localhost:3000

# Or use port-forward as alternative
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

**Default credentials:**
- Username: `admin`
- Password: `prom-operator` (default for kube-prometheus-stack) <!-- pragma: allowlist secret -->

To get the actual password:
```bash
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d
```

### 8. Import Forge Dashboard

Once in Grafana:

1. Click **Dashboards** → **Import**
2. Upload `config/grafana/forge-dashboard.json` from the Forge repo
3. Select **Prometheus** as the data source
4. Click **Import**

You should now see the Forge Operations dashboard with metrics from your controller.

### 9. Test Forge

Create a sample ZarfPackageJob:

```bash
kubectl apply -f examples/samples/build-only-git.yaml
```

Watch the job:
```bash
kubectl get zarfpackagejobs -A -w
```

View controller logs:
```bash
kubectl logs -n forge-system -l app=forge-controller -f
```

Check metrics in Grafana:
- Navigate to the Forge dashboard
- You should see metrics for jobs created, actions performed, etc.

## Cleanup

Delete the Kind cluster (removes everything):

```bash
kind delete cluster --name forge-demo
```

## Troubleshooting

### Helm Install Shows "failed" but Pods Are Running

If `helm list -n monitoring` shows status `failed` but all pods are running:

```bash
# Check if pods are actually healthy
kubectl get pods -n monitoring

# If everything is Running, the install actually succeeded
# The "failed" status is from --wait timeout, not deployment failure
# Fix by marking release as deployed:
helm upgrade kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --reuse-values
```

This happens when the chart takes longer than the timeout to deploy all resources. The pods are fine, Helm just gave up waiting.

### Grafana Not Accessible

Check if the service is exposed:
```bash
kubectl get svc -n monitoring kube-prometheus-stack-grafana
```

Should show `NodePort` with port `30000`.

If not, patch it:
```bash
kubectl patch svc -n monitoring kube-prometheus-stack-grafana -p '{"spec":{"type":"NodePort","ports":[{"port":80,"nodePort":30000}]}}'
```

### Metrics Not Showing in Prometheus

Verify ServiceMonitor exists:
```bash
kubectl get servicemonitor -n forge-system -o yaml
```

Check Prometheus targets:
```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
# Visit http://localhost:9090/targets
# Look for forge-system/forge-controller
```

If not found, check Prometheus configuration:
```bash
kubectl get prometheus -n monitoring -o yaml | grep serviceMonitorSelector -A 10
```

### OTEL Collector Issues

Check OTEL Collector logs:
```bash
kubectl logs -n forge-system -l app=otel-collector
```

Verify service exists:
```bash
kubectl get svc -n forge-system forge-otel-collector
```

Test connectivity from controller:
```bash
kubectl run -it --rm debug --image=busybox -n forge-system -- \
  nc -zv forge-otel-collector 4317
```

### Image Not Found

If pods show `ImagePullBackOff`:

```bash
# Check images in Kind
docker exec -it forge-demo-control-plane crictl images | grep forge

# Reload image
kind load docker-image forge-controller:demo --name forge-demo
```

### Port Already in Use

If port 3000 is already in use on your host:

```bash
# Find what's using it
lsof -i :3000

# Kill the process or use different port in cluster config
# Edit extraPortMappings hostPort to something like 3001
```

## Docker vs Podman

### Using Docker (Default)

Works out of the box with Kind:
```bash
make container-build IMG=forge-controller:demo
kind load docker-image forge-controller:demo --name forge-demo
```

### Using Podman

Requires extra steps:
```bash
# Set Podman as builder
make container-build IMG=forge-controller:demo DOCKER=podman

# Load into Kind via tar
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar
```

Alternatively, alias podman to docker:
```bash
alias docker=podman
```

## Advanced Configuration

### Custom Resource Limits

Edit `chart/forge/values-kind.yaml`:

```yaml
controller:
  resources:
    limits:
      cpu: "2"
      memory: 1Gi
    requests:
      cpu: 500m
      memory: 512Mi
```

### Multiple Replicas

For testing HA:

```bash
helm upgrade forge ./chart/forge \
  -f chart/forge/values-kind.yaml \
  --set controller.replicaCount=2 \
  --namespace forge-system
```

### Enable Network Policies

```bash
helm upgrade forge ./chart/forge \
  -f chart/forge/values-kind.yaml \
  --set networkPolicies.enabled=true \
  --namespace forge-system
```

## Additional Resources

- **User Guide**: [USER_GUIDE.md](USER_GUIDE.md)
- **Main README**: [../README.md](../README.md)
- **Helm Chart Docs**: [../chart/README.md](../chart/README.md)
- **Kind Documentation**: https://kind.sigs.k8s.io/
- **kube-prometheus-stack**: https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack
