# Testing Forge with Kind Using Public Images

> **Audience**: Users who want to evaluate Forge using published container images
>
> **For Developers**: If you're developing Forge and need to build from source, see [KIND_SETUP.md](KIND_SETUP.md)

This guide shows how to test Forge in a local Kind cluster using pre-built images from GitHub Container Registry (GHCR). No source code compilation required.

## Overview

This testing approach:
- Uses **published images** from GHCR (no builds needed)
- Deploys to **Kind** for a disposable test cluster
- Takes **~5 minutes** from zero to running test job
- Requires only **kind**, **kubectl**, **helm**, and **podman** (or docker)

## Prerequisites

### Required Tools

```bash
# macOS (using Homebrew)
brew install kind kubectl helm podman

# Or install individually:
# - kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation
# - kubectl: https://kubernetes.io/docs/tasks/tools/
# - helm: https://helm.sh/docs/intro/install/
# - podman: https://podman.io/getting-started/installation
```

**Note**: This guide uses Podman. If you prefer Docker, the commands are nearly identical - just replace `podman` with `docker`.

### Verify Installation

```bash
kind version
kubectl version --client
helm version
podman version
```

Expected output:

```text
kind v0.20.0 go1.20.4 linux/amd64
Client Version: v1.28.2
version.BuildInfo{Version:"v3.13.0", GitCommit:"...", GoVersion:"go1.20.8"}
podman version 4.7.2
```

## Quick Start

### 1. Create Kind Cluster

```bash
kind create cluster --name forge-test
```

Expected output:

```text
Creating cluster "forge-test" ...
 ✓ Ensuring node image (kindest/node:v1.27.3) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-forge-test"
You can now use your cluster with:

kubectl cluster-info --context kind-forge-test
```

Verify the cluster is ready:

```bash
kubectl cluster-info
kubectl get nodes
```

Expected output:

```text
Kubernetes control plane is running at https://127.0.0.1:xxxxx
CoreDNS is running at https://127.0.0.1:xxxxx/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

NAME                       STATUS   ROLES           AGE   VERSION
forge-test-control-plane   Ready    control-plane   30s   v1.27.3
```

### 2. Add Forge Helm Repository

```bash
helm repo add forge https://kylegalloway.github.io/forge
helm repo update
```

Expected output:

```text
"forge" has been added to your repositories
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "forge" chart repository
Update Complete. ⎈Happy Helming!⎈
```

Verify the chart is available:

```bash
helm search repo forge/forge
```

Expected output:

```text
NAME            CHART VERSION   APP VERSION     DESCRIPTION
forge/forge     0.1.2           v0.1.2          A Helm chart for deploying Forge - a Kubernetes...
```

### 3. Install Forge

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --wait
```

Expected output:

```text
NAME: forge
LAST DEPLOYED: Thu Dec 19 23:53:30 2025
NAMESPACE: forge-system
STATUS: deployed
REVISION: 1
NOTES:
Thank you for installing Forge!

Your release is named forge.

To learn more about the release, try:

  $ helm status forge -n forge-system
  $ helm get all forge -n forge-system
```

This installs Forge using the latest published images from `ghcr.io/kylegalloway/forge`.

**Images used:**
- Controller: `ghcr.io/kylegalloway/forge/forge-controller:latest`
- Webhook: `ghcr.io/kylegalloway/forge/forge-webhook:latest`

**To install a specific version:**

```bash
# List available versions
helm search repo forge/forge --versions

# Install specific version
helm install forge forge/forge \
  --version 0.1.1 \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.tag=v0.1.1 \
  --set webhook.image.tag=v0.1.1 \
  --wait
```

### 4. Verify Installation

Check that all pods are running:

```bash
kubectl get pods -n forge-system
```

Expected output:

```text
NAME                                READY   STATUS    RESTARTS   AGE
forge-controller-797956fdb9-xxxxx   1/1     Running   0          1m
forge-webhook-7fcbdc87fb-xxxxx      1/1     Running   0          1m
forge-webhook-7fcbdc87fb-yyyyy      1/1     Running   0          1m
```

Check that the webhook TLS certificate was generated successfully:

```bash
kubectl get secret forge-webhook-tls -n forge-system
```

Expected output:

```text
NAME                TYPE     DATA   AGE
forge-webhook-tls   Opaque   3      1m
```

Verify the CRDs are installed:

```bash
kubectl get crd | grep forge.dev
```

Expected output:

```text
udspackagejobs.forge.dev      2025-12-19T10:00:00Z
zarfpackagejobs.forge.dev     2025-12-19T10:00:00Z
```

### 5. Build and Load Zarf CLI Image

Forge requires a containerized Zarf CLI for build and deploy operations. Unlike the Forge controller and webhook, the Zarf CLI image must be built locally because the Zarf project doesn't publish container images.

**Why build is needed**: Zarf only publishes binaries, not container images. Forge includes a Dockerfile that downloads the official Zarf binary and packages it into a container.

**Clone the Forge repo (if you haven't already):**

```bash
git clone https://github.com/kylegalloway/forge.git
cd forge
```

**Build and load the image:**

```bash
# Using Docker
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/
kind load docker-image localhost/zarf:v0.66.0 --name forge-test

# OR using Podman
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-test
rm /tmp/zarf-cli.tar
```

Expected output during build:

```text
[+] Building 45.2s (10/10) FINISHED
 => [1/5] FROM docker.io/library/alpine:3.20
 => [2/5] RUN apk add --no-cache ca-certificates git curl bash
 => [3/5] RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')...
 => [4/5] RUN adduser -D -u 1000 zarf...
 => [5/5] RUN zarf version
 => exporting to image
 => => naming to localhost/zarf:v0.66.0

Image: "localhost/zarf:v0.66.0" with ID "sha256:..." not yet present on node "forge-test-control-plane", loading...
```

Verify the image is loaded:

```bash
docker exec -it forge-test-control-plane crictl images | grep zarf
```

Expected output:

```text
localhost/zarf    v0.66.0    e8c96af1c3cbd    45MB
```

### 6. Run a Test Job

Create a ServiceAccount with policy annotations and a test ZarfPackageJob:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-builder
  namespace: default
  annotations:
    forge.dev/allowed-actions: "Build,Publish,Deploy"
    forge.dev/allowed-source-repos: "https://github.com/*"
    forge.dev/allowed-registries: "ghcr.io/*,registry1.dso.mil/*"
    forge.dev/allowed-namespaces: "default,zarf"
---
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: hello-forge
  namespace: default
spec:
  serviceAccountName: test-builder
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/defenseunicorns/zarf
      ref: v0.32.0
      path: examples/dos-games
EOF
```

Expected output:

```text
serviceaccount/test-builder created
zarfpackagejob.forge.dev/hello-forge created
```

Watch the job progress:

```bash
kubectl get zarfpackagejobs -n default -w
```

Expected progression: `Pending` → `Running` → `Succeeded`

Expected output:

```text
NAME          PHASE      AGE
hello-forge   Pending    2s
hello-forge   Running    5s
hello-forge   Succeeded  45s
```

View the build logs:

```bash
# Find the job pod
kubectl get pods -n default

# View logs (replace <pod-name> with actual name)
kubectl logs -n default <pod-name> -f
```

Expected output:

```text
NAME                      READY   STATUS      RESTARTS   AGE
hello-forge-build-xxxxx   0/1     Completed   0          1m

# Logs will show:
📦 ZARF-PACKAGE CREATE zarf-package-dos-games-amd64-0.0.1.tar.zst
✔  Package created successfully
```

Check job status:

```bash
kubectl describe zarfpackagejob hello-forge -n default
```

Expected output (showing relevant sections):

```text
Name:         hello-forge
Namespace:    default
API Version:  forge.dev/v1alpha1
Kind:         ZarfPackageJob
Spec:
  Action:  build
  Service Account Name:  test-builder
Status:
  Completion Time:  2025-12-19T10:01:30Z
  Phase:           Succeeded
  Start Time:      2025-12-19T10:00:45Z
```

### 7. Cleanup

Delete the Kind cluster when finished:

```bash
kind delete cluster --name forge-test
```

Expected output:

```text
Deleting cluster "forge-test" ...
Deleted nodes: ["forge-test-control-plane"]
```

## Testing Scenarios

### Build Only

Test building a Zarf package from Git:

```yaml
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: build-test
  namespace: default
spec:
  serviceAccountName: test-builder
  action: build
  source:
    git:
      url: https://github.com/defenseunicorns/zarf
      ref: v0.32.0
      path: examples/dos-games
  buildOptions:
    output: /workspace/output
```

### Build and Publish

Test building and publishing to an OCI registry:

```yaml
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: build-publish-test
  namespace: default
spec:
  serviceAccountName: test-builder
  action: build-and-publish
  source:
    git:
      url: https://github.com/defenseunicorns/zarf
      ref: v0.32.0
      path: examples/dos-games
  destination:
    oci:
      registry: ghcr.io
      repository: myorg/packages
      credentialsSecret: registry-creds  # pragma: allowlist secret
```

**Note**: You'll need to create a Secret with registry credentials:

```bash
kubectl create secret generic registry-creds \
  --from-literal=username=myusername \
  --from-literal=password=mytoken \
  -n default
```

### Deploy Only

Test deploying a package from an OCI registry:

```yaml
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: deploy-test
  namespace: default
spec:
  serviceAccountName: test-builder
  action: deploy
  source:
    oci:
      registry: ghcr.io
      repository: myorg/packages
      tag: latest
  deployOptions:
    namespace: default
```

### UDS Package Job

Test UDS bundle operations:

```yaml
apiVersion: forge.dev/v1alpha1
kind: UDSPackageJob
metadata:
  name: uds-test
  namespace: default
spec:
  serviceAccountName: test-builder
  action: create
  source:
    git:
      url: https://github.com/defenseunicorns/uds-bundle-software-factory
      ref: main
  createOptions:
    output: /workspace/output
```

## Configuration Options

### Custom Resource Limits

Adjust resources for testing in constrained environments:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=512Mi \
  --set controller.resources.requests.cpu=100m \
  --set controller.resources.requests.memory=128Mi
```

### High Availability Mode

Test with multiple replicas:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3
```

### Network Policies

Test with network policies enabled:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set networkPolicies.enabled=true
```

### Namespace-Scoped Mode

Test namespace-scoped RBAC (all ZarfPackageJobs must be in `forge-system`):

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set rbac.clusterWide=false
```

## Monitoring and Observability

### View Metrics

Forge exposes Prometheus metrics:

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080 &

# View metrics
curl http://localhost:8080/metrics | grep forge_

# Stop port-forward
kill %1
```

Expected output:

```text
Forwarding from 127.0.0.1:8080 -> 8080
Forwarding from [::1]:8080 -> 8080

# go_* metrics
forge_jobs_total{action="build",status="succeeded"} 5
forge_jobs_total{action="publish",status="succeeded"} 2
forge_controller_reconcile_duration_seconds_bucket{le="0.5"} 45
```

### Install Prometheus and Grafana (Optional)

For metric visualization:

```bash
# Add Prometheus Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-prometheus-stack
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --wait

# Access Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80 &

# Get admin password
kubectl get secret -n monitoring prometheus-grafana \
  -o jsonpath="{.data.admin-password}" | base64 -d && echo

# Open http://localhost:3000
# Username: admin
# Password: (from command above)
```

Expected output:

```text
Forwarding from 127.0.0.1:3000 -> 3000

prom-operator
```

Access Prometheus directly:

```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &
# Open http://localhost:9090
# Query: forge_jobs_total
```

## Troubleshooting

### Forge Images Not Pulling

If pods show `ImagePullBackOff` for controller or webhook:

```bash
# Check if images are public
podman pull ghcr.io/kylegalloway/forge/forge-controller:latest
podman pull ghcr.io/kylegalloway/forge/forge-webhook:latest

# If images don't exist or are private, see KIND_SETUP.md for building from source
```

### Zarf CLI Image Not Found

If job pods show `ImagePullBackOff` for `localhost/zarf:v0.66.0`:

```bash
# Verify image is in Kind cluster
docker exec -it forge-test-control-plane crictl images | grep zarf

# If missing, build and load it
cd /path/to/forge  # Your Forge repo clone

# Using Docker
docker build -t localhost/zarf:v0.66.0 images/zarf-cli/
kind load docker-image localhost/zarf:v0.66.0 --name forge-test

# OR using Podman
podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
kind load image-archive /tmp/zarf-cli.tar --name forge-test
rm /tmp/zarf-cli.tar
```

### Webhook TLS Certificate Errors

If you see webhook admission errors:

```bash
# Check webhook certificates
kubectl get secret forge-webhook-tls -n forge-system

# Check cert generator job completed successfully
kubectl get events -n forge-system --sort-by='.lastTimestamp' | grep cert-generator
```

Expected output (successful):

```text
NAME                TYPE     DATA   AGE
forge-webhook-tls   Opaque   3      5m

# Events should show:
Normal   SuccessfulCreate   job/forge-cert-generator   Created pod: forge-cert-generator-xxxxx
Normal   Completed          job/forge-cert-generator   Job completed
```

If the secret is missing or cert-generator job failed:

```bash
# Reinstall with auto-generated certs
helm upgrade forge forge/forge \
  --namespace forge-system \
  --reuse-values \
  --set webhook.tls.autoGenerate=true
```

**Note**: Forge uses the latest webhook cert generator image (`registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20231226-1a7112e06`) which runs properly as non-root. If you see `apk` permission errors, the chart may be using an outdated image.

### Job Execution Failures

If ZarfPackageJobs fail to execute:

```bash
# View controller logs
kubectl logs -n forge-system -l app=forge-controller --tail=100

# Check job details
kubectl describe zarfpackagejob <job-name> -n <namespace>

# View job pod logs
kubectl get pods -n <namespace>
kubectl logs -n <namespace> <pod-name>

# Common issues:
# - ServiceAccount missing required annotations
# - Git repository not in allowed-repos
# - Insufficient resources for job pod
```

### Policy Validation Failures

If jobs are rejected by the webhook:

```bash
# View webhook logs
kubectl logs -n forge-system -l app=forge-webhook --tail=50

# Check ServiceAccount annotations
kubectl get sa <service-account> -n <namespace> -o yaml

# Verify annotations match job requirements:
# - forge.dev/allowed-actions: must include the action (build/publish/deploy)
# - forge.dev/allowed-repos: must match source Git URL
# - forge.dev/allowed-registries: must match destination registry
```

### Kind Cluster Issues

If Kind cluster behaves unexpectedly:

```bash
# Check cluster status
kubectl get nodes
kubectl get pods -A

# Reset cluster
kind delete cluster --name forge-test
kind create cluster --name forge-test

# Reinstall Forge
helm install forge forge/forge --namespace forge-system --create-namespace --wait
```

## Next Steps

After testing Forge with Kind:

1. **Production Deployment**: See [USER_GUIDE.md](USER_GUIDE.md) for deploying to production clusters
2. **Policy Configuration**: See [SERVICEACCOUNT_REFERENCE.md](../development/SERVICEACCOUNT_REFERENCE.md) for detailed RBAC setup
3. **Operations**: See [RUNBOOK.md](../operations/RUNBOOK.md) for operational procedures
4. **Troubleshooting**: See [TROUBLESHOOTING.md](../operations/TROUBLESHOOTING.md) for detailed debugging

## Additional Resources

- **User Guide**: [USER_GUIDE.md](USER_GUIDE.md) - Complete usage examples
- **Developer Guide**: [KIND_SETUP.md](KIND_SETUP.md) - Local development with source builds
- **Helm Chart**: [chart/README.md](../../chart/README.md) - All configuration options
- **Forge Releases**: [GitHub Releases](https://github.com/kylegalloway/forge/releases)
- **Kind Documentation**: <https://kind.sigs.k8s.io/>
- **Zarf Documentation**: <https://zarf.dev/>
