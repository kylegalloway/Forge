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

## Quick Start (Recommended)

**Using the Makefile (easiest):**

```bash
# Complete setup in one command
# Creates cluster + builds images + deploys Forge
make kind-setup

# Verify deployment
make status
```

**Important Notes:**
- Default cluster name: `forge-dev` (customize with `KIND_CLUSTER_NAME=my-cluster make kind-setup`)
- Uses Podman by default (automatically falls back to Docker if Podman not available)
- Builds images tagged as `localhost/forge-controller:latest` and `localhost/forge-webhook:latest`
- Does NOT include Zarf CLI image (see "Adding Zarf CLI" below)

### Adding Zarf CLI Image

Most Forge operations need the Zarf CLI container image. Add it after setup:

```bash
# Build and load Zarf Package Job image into the cluster
make kind-zarfpackagejob
```

### Manual Setup (for reference)

If you prefer to run steps individually or need custom configuration:

```bash
# 1. Create Kind cluster
kind create cluster --name forge-dev

# 2. Build Forge controller and webhook images
make podman-build

# 3. Load images into Kind
# For Podman (default):
make kind-load

# For Docker:
make kind-load-docker

# 4. Build and load Zarf Package Job image
make kind-zarfpackagejob

# 5. Deploy with Helm
make install

# 6. Wait for Forge to be ready
kubectl wait --for=condition=Ready pods --all -n forge-system --timeout=300s
```

## Makefile Commands Reference

The Forge Makefile provides convenient targets for Kind development workflows:

### Setup & Deployment

| Command | Description |
|---------|-------------|
| `make kind-setup` | **Complete setup**: Creates cluster, builds images, loads into Kind, and deploys with Helm |
| `make kind-create` | Create Kind cluster (default name: `forge-dev`) |
| `make kind-deploy` | Build images, load into Kind, and install with Helm |
| `make kind-redeploy` | Uninstall Forge, rebuild images, reload, and redeploy (useful for testing changes) |
| `make install` | Install Forge using Helm with default values |
| `make upgrade` | Upgrade existing Forge installation |
| `make uninstall` | Uninstall Forge from the cluster |

### Image Management

| Command | Description |
|---------|-------------|
| `make podman-build` | Build controller and webhook images using Podman |
| `make docker-build` | Build controller and webhook images using Docker |
| `make kind-load` | Build with Podman and load into Kind cluster (via tar archive) |
| `make kind-load-docker` | Build with Docker and load into Kind cluster (direct) |
| `make kind-zarfpackagejob` | Build and load Zarf Package Job image into Kind cluster |
| `make kind-images` | List Forge images in the Kind cluster |

### Observability

| Command | Description |
|---------|-------------|
| `make status` | Show controller, webhook, and ZarfPackageJob status |
| `make dev-controller-logs` | View controller logs (last 30 lines) |
| `make dev-webhook-logs` | View webhook logs (last 30 lines) |
| `make dev-job-logs` | View logs from latest Forge job |
| `make dev-logs` | Show all logs (controller + webhook + latest job) |

### Cleanup Commands

| Command | Description |
|---------|-------------|
| `make kind-delete` | Delete the Kind cluster |
| `make clean` | Remove built binaries and temporary files |

### Customization Examples

```bash
# Use custom cluster name
KIND_CLUSTER_NAME=my-test-cluster make kind-setup

# Use custom image tags
CTRL_IMG=forge-controller:v1.2.3 WBHK_IMG=forge-webhook:v1.2.3 make kind-deploy

# Change namespace
NAMESPACE=my-namespace make install

# Use Docker instead of Podman
make docker-build
make kind-load-docker
```

### Typical Development Workflows

**Initial Setup:**
```bash
make kind-setup           # Full setup
make kind-zarfpackagejob   # Add Zarf Package Job for jobs
make status               # Verify everything is running
```

**Iterative Development:**
```bash
# Make code changes, then:
make kind-redeploy        # Rebuild, reload, redeploy
make dev-logs             # Check logs for errors

# Or individual steps for finer control:
make podman-build         # Build new images
make kind-load            # Load into cluster
kubectl delete pods -n forge-system -l app=forge-controller  # Restart pods
make dev-controller-logs  # Check controller logs
```

**Testing Changes:**
```bash
kubectl apply -f examples/samples/zarf/build-only-git.yaml
make dev-job-logs         # Watch job execution
kubectl get zarfpackagejobs -w  # Watch status changes
```

**Cleanup:**
```bash
make kind-delete          # Remove cluster
make clean                # Clean build artifacts
```

## Step-by-Step Instructions

> **Note**: These manual instructions explain what happens under the hood. For most cases, use `make kind-setup` instead (see Quick Start above).

### 1. Create Kind Cluster

**Using Make:**
```bash
make kind-create
```

**Manual command:**
```bash
kind create cluster --name forge-dev
```

Verify cluster is running:

```bash
kubectl cluster-info
kubectl get nodes
```

### 2. Build Forge Images

**Using Make (recommended):**
```bash
# Builds both controller and webhook images
make podman-build
```

**Manual commands:**
```bash
# Build controller image
podman build --target controller --iidfile controller.iid .
podman tag "$(cat controller.iid)" localhost/forge-controller:latest
rm -f controller.iid

# Build webhook image
podman build --target webhook --iidfile webhook.iid .
podman tag "$(cat webhook.iid)" localhost/forge-webhook:latest
rm -f webhook.iid
```

Expected output:

```text
[+] Building 45.2s (18/18) FINISHED
 => [internal] load build definition from Dockerfile
 => [builder 6/6] RUN CGO_ENABLED=0 GOOS=linux go build -a -o controller cmd/controller/main.go
 => [builder 7/7] RUN CGO_ENABLED=0 GOOS=linux go build -a -o webhook cmd/webhook/main.go
 => exporting to image
 => => exporting layers
 => => writing image sha256:abc123...

Successfully built localhost/forge-controller:latest and localhost/forge-webhook:latest
```

### 3. Load Images into Kind

Kind clusters can't pull from your local Docker/Podman registry, so you need to load images explicitly.

**Using Make (recommended):**

```bash
# For Podman (default):
make kind-load

# For Docker:
make kind-load-docker
```

**Manual commands for Podman:**

```bash
# Export to tar, load into Kind, cleanup
podman save localhost/forge-controller:latest -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-dev
rm /tmp/forge-controller.tar

podman save localhost/forge-webhook:latest -o /tmp/forge-webhook.tar
kind load image-archive /tmp/forge-webhook.tar --name forge-dev
rm /tmp/forge-webhook.tar
```

**Manual commands for Docker:**

```bash
kind load docker-image localhost/forge-controller:latest --name forge-dev
kind load docker-image localhost/forge-webhook:latest --name forge-dev
```

**Verify images are loaded:**

```bash
# Check images in Kind cluster
docker exec -it forge-dev-control-plane crictl images | grep -E 'forge|zarf'
```

Expected output:

```text
localhost/forge-controller  latest      abc123def456   100MB
localhost/forge-webhook     latest      def456abc123   95MB
localhost/zarf              v0.68.1     789abc012def   45MB
```

### 3a. Build and Load Zarf Package Job Image

Zarf doesn't publish container images - only binaries. Forge includes a Dockerfile that packages the official Zarf CLI binary into a container image for use in Job pods.

**Using Make (recommended):**

```bash
make kind-zarfpackagejob
```

**Manual commands:**

```bash
# For Podman:
podman build -t localhost/zarfpackagejob:v0.11.5 images/zarfpackagejob/
podman save localhost/zarfpackagejob:v0.11.5 -o /tmp/zarfpackagejob.tar
kind load image-archive /tmp/zarfpackagejob.tar --name forge-dev
rm /tmp/zarfpackagejob.tar

# For Docker:
docker build -t localhost/zarfpackagejob:v0.11.5 images/zarfpackagejob/
kind load docker-image localhost/zarfpackagejob:v0.11.5 --name forge-dev
```

Without this image, Zarf build/deploy jobs will fail with `ImagePullBackOff`.

### 4. Install Forge

**Using Make (recommended):**

```bash
make install
```

**Manual command:**

```bash
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.repository=localhost/forge-controller \
  --set controller.image.tag=latest \
  --set webhook.image.repository=localhost/forge-webhook \
  --set webhook.image.tag=latest \
  --wait
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

**Using Make:**

```bash
make kind-delete    # Delete the Kind cluster
make clean          # Remove build artifacts
```

**Manual commands:**

```bash
kind delete cluster --name forge-dev
rm -rf bin/ cover*.out
```

## Troubleshooting

### Image Not Found

If pods show `ImagePullBackOff`, the images aren't loaded in the Kind cluster.

**Fix with Make:**

```bash
# Check what images are loaded
make kind-images

# Reload Forge images
make kind-load

# Reload Zarf Package Job image
make kind-zarfpackagejob
```

**Manual diagnosis and fix:**

```bash
# Check images in Kind cluster
docker exec -it forge-dev-control-plane crictl images | grep -E 'forge|zarf'

# Reload Forge images - For Podman:
make kind-load

# Reload Forge images - For Docker:
make kind-load-docker

# Reload Zarf CLI image
make kind-zarfpackagejob
```

### Pods Not Starting

Check pod status and events:

```bash
make status                                    # Quick overview
kubectl get pods -n forge-system               # Detailed pod status
kubectl describe pod <pod-name> -n forge-system  # Events and errors
make dev-controller-logs                       # View controller logs
make dev-webhook-logs                          # View webhook logs
```

### Jobs Failing

Check job logs and status:

```bash
kubectl get jobs -A                            # List all jobs
make dev-job-logs                              # View latest job logs
kubectl logs job/<job-name> -n <namespace>     # Specific job logs
kubectl describe job/<job-name> -n <namespace> # Job events
```

### Complete Reset

If things are completely broken, start fresh:

```bash
make kind-delete      # Delete cluster
make clean            # Clean artifacts
make kind-setup       # Recreate everything
make kind-zarfpackagejob    # Add Zarf Package Job image
make status           # Verify
```

## Docker vs Podman

The Makefile provides separate targets for Docker and Podman workflows.

### Using Podman (Default)

Podman requires saving images to tar archives before loading into Kind:

**Using Make (handles tar creation/cleanup automatically):**

```bash
make podman-build     # Build with Podman
make kind-load        # Save to tar, load into Kind, cleanup
make kind-zarfpackagejob    # Same for Zarf Package Job
```

**What it does behind the scenes:**

```bash
# For each image:
podman save localhost/forge-controller:latest -o /tmp/forge-controller.tar
kind load image-archive /tmp/forge-controller.tar --name forge-dev
rm /tmp/forge-controller.tar
```

### Using Docker

Docker can load images directly into Kind without tar archives:

**Using Make:**

```bash
make docker-build        # Build with Docker
make kind-load-docker    # Load directly into Kind
```

**Manual commands:**

```bash
# Docker loads images directly
kind load docker-image localhost/forge-controller:latest --name forge-dev
kind load docker-image localhost/forge-webhook:latest --name forge-dev

# Zarf CLI
docker build -t localhost/zarfpackagejob:v0.11.5 images/zarfpackagejob/
kind load docker-image localhost/zarf:v0.68.1 --name forge-dev
```

### Switching Between Docker and Podman

```bash
# Use Docker explicitly
make docker-build
make kind-load-docker

# Use Podman explicitly
make podman-build
make kind-load

# Override for specific commands
CONTAINER_RUNTIME=docker make kind-setup
```

**Tip:** If you prefer Docker syntax, you can alias podman:

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
