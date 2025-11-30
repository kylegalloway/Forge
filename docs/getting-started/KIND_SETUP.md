# Forge Kind Setup Guide

Complete guide for running Forge in a local Kind cluster with full observability.

## Prerequisites

- **kind** - `brew install kind` (or from https://kind.sigs.k8s.io/)
- **kubectl** - `brew install kubectl`
- **helm** - `brew install helm`
- **docker** or **podman** - For building images
- **make** - For build commands

## Quick Start (Copy-Paste Ready)

> **Note:** These commands use Docker by default. If you're using Podman, see the commented alternatives in steps 4-5 below.

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

# 4. Load images into Kind
# Choose commands based on your container runtime (Docker or Podman)

# For Docker:
kind load docker-image forge-controller:demo --name forge-demo

# For Podman:
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar

# 5. Build and load Zarf CLI image
# Zarf doesn't publish container images - build from included Dockerfile

# For Docker:
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/
kind load docker-image localhost/zarf:v0.66.0 --name forge-demo

# For Podman:
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-demo
rm /tmp/zarf-cli.tar

# 6. Install Forge
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-kind.yaml \
  --namespace forge-system \
  --create-namespace

# 7. Wait for everything to be ready
kubectl wait --for=condition=Ready pods --all -n forge-system --timeout=300s
kubectl wait --for=condition=Ready pods --all -n monitoring --timeout=300s

# 8. Access Grafana
open http://localhost:3000
# Username: admin
# Password: Get it with the command below:
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d
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

# Install with NodePort for Grafana and relaxed probe settings for Kind
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.service.type=NodePort \
  --set grafana.service.nodePort=30000 \
  --set prometheus.prometheusSpec.resources.requests.cpu=100m \
  --set prometheus.prometheusSpec.resources.requests.memory=512Mi \
  --set prometheus.prometheusSpec.resources.limits.cpu=500m \
  --set prometheus.prometheusSpec.resources.limits.memory=1Gi \
  --timeout 10m \
  --wait
```

**Why these settings?**

- `serviceMonitorSelectorNilUsesHelmValues=false` - Allows Prometheus to discover ServiceMonitors in any namespace
- `grafana.service.type=NodePort` - Exposes Grafana on port 30000 (mapped to host port 3000)
- `resources.requests/limits` - Reduced resources suitable for Kind (prevents probe timeouts)
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

Verify the images are loaded:
```bash
# Check images in Kind cluster (works with both Docker and Podman)
docker exec -it forge-demo-control-plane crictl images | grep -E 'forge|zarf'
```

**Build and load Zarf CLI image:**

Zarf doesn't publish container images - only binaries. Forge includes a Dockerfile that packages the official Zarf CLI binary into a container image for use in Job pods.

```bash
# Build the image
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/

# Load into Kind
kind load docker-image localhost/zarf:v0.66.0 --name forge-demo
```

**For Podman users:**
```bash
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-demo
rm /tmp/zarf-cli.tar
```

This packages the Zarf CLI so build/deploy jobs can run. Without this, job pods will fail with ImagePullBackOff.

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
- ServiceMonitor (for Prometheus to scrape controller metrics)
- PrometheusRule (for alerts)
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

```text
NAME                                    READY   STATUS    RESTARTS   AGE
forge-controller-xxxxx                  1/1     Running   0          1m
forge-otel-collector-xxxxx              1/1     Running   0          1m
```

Check ServiceMonitor and PrometheusRule are created:
```bash
kubectl get servicemonitor -n forge-system
kubectl get prometheusrule -n forge-system
```

Expected output:

```text
NAME               AGE
forge-controller   1m

NAME           AGE
forge-alerts   1m
```

### 7. Access Grafana

Grafana is exposed on NodePort 30000, which maps to host port 3000:

```bash
# Open in browser
open http://localhost:3000

# Or use port-forward as alternative
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

**Credentials:**

- Username: `admin`
- Password: Randomly generated on install

Get the password:
```bash
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d
```

### 8. Import Forge Dashboard

Once in Grafana:

1. Click **Dashboards** → **Import**
2. Click **Upload JSON file**
3. Select `chart/forge/dashboards/forge-dashboard.json` from the Forge repo
4. Select **Prometheus** as the data source (should auto-detect)
5. Click **Import**

You should now see the Forge Operations dashboard with metrics from your controller.

**Alternative (using dashboard ID):**
If the dashboard has been published to grafana.com, you can import by ID instead of uploading the JSON file.

### 9. Test Forge

Create a sample ZarfPackageJob:

```bash
# Create the required ServiceAccount with policy annotations
kubectl apply -f examples/service-accounts/service-account-example.yaml

# Apply the hello-forge test job (lightweight, will succeed)
kubectl apply -f examples/zarfpackagejobs/hello-forge-test.yaml
```

**Note:** This uses a minimal test package specifically designed for resource-constrained environments. The build should complete successfully in 15-30 seconds.

Watch the job:
```bash
kubectl get zarfpackagejobs -A -w
```

View controller logs:
```bash
kubectl logs -n forge-system -l app=forge-controller -f
```

Verify the controller processed the job:
```bash
# Check the ZarfPackageJob resource
kubectl describe zarfpackagejob hello-forge-test -n default

# Check if the underlying Kubernetes Job was created
kubectl get jobs -n default

# Watch the job complete
kubectl get pods -n default -w
```

**Expected behavior:**

- The controller creates a Kubernetes Job for the ZarfPackageJob
- The Job clones the Git repository
- Zarf builds the minimal test package
- The build completes successfully (Status: Completed)
- You should see phase change from Pending → Running → Succeeded

Check the job logs to see the successful build:
```bash
# Find the pod name
kubectl get pods -n default

# View the build output
kubectl logs -n default <pod-name> -c zarf-build
```

### 10. View Metrics

**Current Status:** The Forge controller currently exposes standard Go runtime metrics (goroutines, memory, GC stats) through the OTEL collector. Custom Forge-specific metrics (like `forge_jobs_created_total`, `forge_resources_active`) are planned but not yet implemented.

Check available metrics:
```bash
# View metrics in Prometheus
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090

# Visit http://localhost:9090
# Query for: forge_go_goroutines
```

Access Grafana:
```bash
# Already accessible at http://localhost:3000 via NodePort
# Or use port-forward:
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

**Dashboard Note:** The included Grafana dashboard (`chart/forge/dashboards/forge-dashboard.json`) expects custom Forge metrics that are not yet implemented. You can still import the dashboard to see its structure, but panels will show "No data" until custom metrics are added to the controller.

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
# Check images in Kind cluster
docker exec -it forge-demo-control-plane crictl images | grep -E 'forge|zarf'

# Reload images (choose based on your container runtime)
# For Docker:
kind load docker-image forge-controller:demo --name forge-demo
kind load docker-image localhost/zarf:v0.66.0 --name forge-demo

# For Podman:
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar
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
# Build Forge controller
make container-build IMG=forge-controller:demo
kind load docker-image forge-controller:demo --name forge-demo

# Build Zarf CLI image
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/
kind load docker-image localhost/zarf:v0.66.0 --name forge-demo
```

### Using Podman

Requires extra steps (save/load via tar):
```bash
# Build Forge controller
make container-build IMG=forge-controller:demo DOCKER=podman

# Load Forge controller into Kind
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar

# Build and load Zarf CLI image
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-demo
rm /tmp/zarf-cli.tar
```

**Tip:** You can alias podman to docker if you prefer:
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
