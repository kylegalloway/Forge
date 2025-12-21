# Forge Kind Setup Guide (Developer Workflow)

> **Audience**: Developers contributing to Forge who need to build and test local changes
>
> **For End Users**: If you just want to install Forge, see [USER_GUIDE.md](USER_GUIDE.md) for installation from the Helm repository

Complete guide for running Forge in a local Kind cluster with custom-built images.

## Overview

This guide covers local development using:

- **Custom-built images** from your local source code
- **Kind (Kubernetes in Docker)** for a disposable test cluster
- **Makefile targets** for rapid iteration

For production deployment with published images, see [USER_GUIDE.md](USER_GUIDE.md).

> **Note**: This guide focuses on deploying Forge itself. For metrics and observability, you can optionally install your own Prometheus/Grafana stack separately - see the Troubleshooting section for details.

## Prerequisites

- **kind** - `brew install kind` (or from <https://kind.sigs.k8s.io/>)
- **kubectl** - `brew install kubectl`
- **helm** - `brew install helm`
- **docker** or **podman** - For building images
- **make** - For build commands

## Quick Start (Copy-Paste Ready)

> **Note:** These commands use Docker by default. If you're using Podman, see the commented alternatives below.

```bash
# 1. Create Kind cluster
kind create cluster --name forge-demo

# 2. Build Forge controller and webhook images
make podman-build IMG=forge-controller:demo TARGET=controller
make podman-build IMG=forge-webhook:demo TARGET=webhook

# 3. Load images into Kind
# For Docker:
kind load docker-image forge-controller:demo --name forge-demo
kind load docker-image forge-webhook:demo --name forge-demo

# For Podman:
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar
podman save localhost/forge-webhook:demo -o /tmp/forge-webhook.tar
kind load image-archive /tmp/forge-webhook.tar --name forge-demo
rm /tmp/forge-webhook.tar

# 4. Build and load Zarf CLI image
# Zarf doesn't publish container images - build from included Dockerfile

# For Docker:
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/
kind load docker-image localhost/zarf:v0.66.0 --name forge-demo

# For Podman:
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-demo
rm /tmp/zarf-cli.tar

# 5. Install Forge
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.repository=localhost/forge-controller \
  --set controller.image.tag=demo \
  --set webhook.image.repository=localhost/forge-webhook \
  --set webhook.image.tag=demo

# 6. Wait for Forge to be ready
kubectl wait --for=condition=Ready pods --all -n forge-system --timeout=300s
```

**Or use the Makefile shortcut:**

```bash
# Does everything above in one command
make kind-setup
```

## Step-by-Step Instructions

### 1. Create Kind Cluster

Create a simple Kind cluster:

```bash
kind create cluster --name forge-demo
```

Verify cluster is running:

```bash
kubectl cluster-info
kubectl get nodes
```

### 2. Build Forge Images

Build the controller and webhook images locally:

```bash
# From the forge project root
make podman-build IMG=forge-controller:demo TARGET=controller
make podman-build IMG=forge-webhook:demo TARGET=webhook
```

Expected output:

```text
Building controller image: forge-controller:demo
[+] Building 45.2s (18/18) FINISHED
 => [internal] load build definition from Dockerfile
 => => transferring dockerfile: 1.23kB
 => [internal] load .dockerignore
 => [builder 6/6] RUN CGO_ENABLED=0 GOOS=linux go build -a -o controller cmd/controller/main.go
 => [builder 7/7] RUN CGO_ENABLED=0 GOOS=linux go build -a -o webhook cmd/webhook/main.go
 => exporting to image
 => => exporting layers
 => => writing image sha256:abc123...
 => => naming to docker.io/library/forge-controller:demo
 => => naming to docker.io/library/forge-webhook:demo

Successfully built forge-controller:demo and forge-webhook:demo
```

This builds the images and tags them as `forge-controller:demo` and `forge-webhook:demo`.

### 3. Load Images into Kind

Kind clusters can't pull from your local Docker/Podman registry, so you need to load images explicitly:

**For Docker users:**

```bash
kind load docker-image forge-controller:demo --name forge-demo
kind load docker-image forge-webhook:demo --name forge-demo
```

Expected output:

```text
Image: "forge-controller:demo" with ID "sha256:abc123..." not yet present on node "forge-demo-control-plane", loading...
Image: "forge-webhook:demo" with ID "sha256:def456..." not yet present on node "forge-demo-control-plane", loading...
```

**For Podman users:**

```bash
# Export to tar, load into Kind, cleanup
podman save localhost/forge-controller:demo -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-demo
rm /tmp/forge-controller.tar

podman save localhost/forge-webhook:demo -o /tmp/forge-webhook.tar
kind load image-archive /tmp/forge-webhook.tar --name forge-demo
rm /tmp/forge-webhook.tar
```

Verify the images are loaded:

```bash
# Check images in Kind cluster (works with both Docker and Podman)
docker exec -it forge-demo-control-plane crictl images | grep -E 'forge|zarf'
```

Expected output:

```text
forge-controller            demo        abc123def456   100MB
forge-webhook               demo        def456abc123   95MB
localhost/zarf              v0.66.0     789abc012def   45MB
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

### 4. Install Forge

Install Forge using your locally-built images:

```bash
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.repository=forge-controller \
  --set controller.image.tag=demo \
  --set webhook.image.repository=forge-webhook \
  --set webhook.image.tag=demo
```

**What gets deployed:**

- Forge controller (1 replica)
- Forge webhook (2 replicas)
- Metrics Service (for external Prometheus to scrape)

**What does NOT get deployed:**

- Grafana (install separately if needed)
- Prometheus (install separately if needed)
- OTEL Collector (install separately if needed)

### 5. Verify Installation

Check all pods are running:

```bash
kubectl get pods -n forge-system
```

Expected output in `forge-system`:

```text
NAME                                    READY   STATUS    RESTARTS   AGE
forge-controller-xxxxx                  1/1     Running   0          1m
forge-webhook-xxxxx                     1/1     Running   0          1m
forge-webhook-yyyyy                     1/1     Running   0          1m
```

Check that the CRDs are installed:

```bash
kubectl get crd | grep forge
```

Expected output:

```text
udsbundlejobs.forge.dev          2024-12-18T10:00:00Z
zarfpackagejobs.forge.dev        2024-12-18T10:00:00Z
```

### 6. Test Forge

Create a sample ZarfPackageJob:

```bash
# Create the required ServiceAccount with policy annotations
kubectl apply -f examples/service-accounts/simple-test-sa.yaml

# Apply a sample Zarf package job (update OCI credentials in the YAML first)
kubectl apply -f examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml
```

Expected output:

```text
serviceaccount/simple-test-sa created
zarfpackagejob.forge.dev/git-to-oci-example created
```

**Note:** Before applying, edit `examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml` to update the OCI registry credentials. See the example's README for details.

Watch the job:

```bash
kubectl get zarfpackagejobs -A -w
```

Expected output:

```text
NAMESPACE   NAME               PHASE      AGE
default     hello-forge-test   Pending    2s
default     hello-forge-test   Running    5s
default     hello-forge-test   Succeeded  35s
```

View controller logs:

```bash
kubectl logs -n forge-system -l app=forge-controller -f
```

Expected output:

```text
{"level":"info","ts":1703001234.567,"msg":"Reconciling ZarfPackageJob","name":"hello-forge-test","namespace":"default"}
{"level":"info","ts":1703001235.123,"msg":"Creating build job","name":"hello-forge-test","namespace":"default"}
{"level":"info","ts":1703001265.890,"msg":"Job completed successfully","name":"hello-forge-test","namespace":"default"}
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

### 7. View Metrics (Optional)

Forge exposes Prometheus metrics on the controller's metrics port. To view them:

```bash
# Port-forward to the metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080

# In another terminal, view the metrics
curl http://localhost:8080/metrics
```

**If you want to visualize metrics:**

You can install Prometheus and Grafana separately using kube-prometheus-stack. See the "Optional: Install Monitoring Stack" section below for instructions.

## Cleanup

Delete the Kind cluster (removes everything):

```bash
kind delete cluster --name forge-demo
```

## Troubleshooting

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

podman save localhost/forge-webhook:demo -o /tmp/forge-webhook.tar
kind load image-archive /tmp/forge-webhook.tar --name forge-demo
rm /tmp/forge-webhook.tar
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

## Optional: Install Monitoring Stack

If you want to visualize Forge metrics with Prometheus and Grafana:

### 1. Install kube-prometheus-stack

```bash
# Add Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install with relaxed settings for Kind
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.resources.requests.cpu=100m \
  --set prometheus.prometheusSpec.resources.requests.memory=512Mi \
  --set prometheus.prometheusSpec.resources.limits.cpu=500m \
  --set prometheus.prometheusSpec.resources.limits.memory=1Gi \
  --timeout 10m \
  --wait
```

### 2. Access Grafana

```bash
# Port-forward to Grafana
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80

# Get the admin password
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d

# Open http://localhost:3000 in your browser
# Username: admin
# Password: (from command above)
```

### 3. Access Prometheus

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090

# Open http://localhost:9090
# Query for Forge metrics like: forge_jobs_total
```

**Note:** Prometheus will automatically discover Forge metrics via the Pod annotations in the Helm chart.

## Advanced Configuration

### Custom Resource Limits

Edit the resource limits when installing:

```bash
helm upgrade forge ./chart/forge \
  --namespace forge-system \
  --set controller.resources.limits.cpu=2 \
  --set controller.resources.limits.memory=1Gi \
  --set controller.resources.requests.cpu=500m \
  --set controller.resources.requests.memory=512Mi
```

### Multiple Replicas

For testing HA:

```bash
helm upgrade forge ./chart/forge \
  --namespace forge-system \
  --set controller.replicaCount=2
```

### Enable Network Policies

```bash
helm upgrade forge ./chart/forge \
  --namespace forge-system \
  --set networkPolicies.enabled=true
```

## Additional Resources

- **User Guide**: [USER_GUIDE.md](USER_GUIDE.md)
- **Main README**: [../../README.md](../../README.md)
- **Helm Chart Docs**: [../../chart/README.md](../../chart/README.md)
- **Kind Documentation**: <https://kind.sigs.k8s.io/>
- **Prometheus Operator**: <https://github.com/prometheus-operator/prometheus-operator>
- **kube-prometheus-stack**: <https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack>
