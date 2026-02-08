# Forge Deployment Guide

This guide covers deploying Forge to Kubernetes clusters using Helm.

## Overview

Forge is deployed as a Kubernetes controller with an admission webhook. It exposes metrics endpoints for integration with your existing monitoring infrastructure.

## Prerequisites

- Kubernetes 1.24 or later
- Helm 3.8 or later
- kubectl configured and connected to your cluster
- (Optional) Prometheus for metrics collection
- (Optional) Grafana for visualization

## Installation Methods

### For Users (Recommended)

Install from the published Helm repository with pre-built container images.

### For Developers

Install from local source code for testing changes. See [CONTRIBUTING.md](CONTRIBUTING.md).

### For Air-Gapped Environments (Zarf)

Deploy to disconnected clusters using the included Zarf package. See [Air-Gapped Deployment](#air-gapped-deployment-zarf) below.

---

## Quick Start (Users)

### 1. Add Helm Repository

```bash
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update
```

### 2. Install Forge

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace
```

**Container Images Used**:

- `ghcr.io/kylegalloway/forge/forge-controller:v0.11.17`
- `ghcr.io/kylegalloway/forge/forge-webhook:v0.11.17`
- `ghcr.io/kylegalloway/forge/zarfpackagejob:v0.11.17` (used by ZarfPackageJobs)
- `ghcr.io/kylegalloway/forge/udsbundlejob:v0.11.17` (used by UDSBundleJobs)

**CLI Image Configuration**:

CLI images can be customized via Helm values or environment variables:

```bash
# Via Helm values
helm install forge forge/forge \
  --set zarfCLI.image.repository=my-registry.io/zarfpackagejob \
  --set zarfCLI.image.tag=v0.69.0 \
  --set udsCLI.image.repository=my-registry.io/udsbundlejob \
  --set udsCLI.image.tag=v0.27.21
```

Or via controller environment variables: `FORGE_ZARF_CLI_IMAGE`, `FORGE_UDS_CLI_IMAGE`

---

## Quick Start (Developers)

For local development with custom-built images:

```bash
# Complete setup: create Kind cluster, build, deploy
make kind-setup

# Iterative development
make kind-redeploy

# Cleanup
make kind-delete
```

See [docs/getting-started/KIND_SETUP.md](docs/getting-started/KIND_SETUP.md) for detailed developer workflow.

## What Gets Deployed

When you install Forge, you get:

- ✅ Forge Controller (manages ZarfPackageJob and UDSBundleJob resources)
- ✅ Forge Webhook (validates resources at admission time)
- ✅ Metrics Service (exposes metrics for external Prometheus to scrape)
- ✅ CRDs (ZarfPackageJob, UDSBundleJob)
- ✅ RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)

Forge **does not** deploy:

- ❌ Grafana (install separately if needed)
- ❌ Prometheus (install separately if needed)
- ❌ OTEL Collector (install separately if needed)

## Configuration

### Basic Installation

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace
```

### Custom Configuration

Override values using `--set`:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3 \
  --set controller.resources.limits.memory=1Gi
```

### High Availability

For production environments with multiple controller replicas. Leader election is enabled by default, and pod anti-affinity + PodDisruptionBudget are automatically configured when `replicaCount > 1`:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set webhook.replicaCount=3
```

This automatically provides:

- **Leader election** for controller coordination (enabled by default)
- **PodDisruptionBudget** ensuring at least 1 replica during disruptions
- **Pod anti-affinity** spreading replicas across nodes
- **Configurable workers** for reconciliation throughput (`controller.workers`)

Leader election timing can be tuned if needed:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set leaderElection.leaseDuration=20s \
  --set leaderElection.renewDeadline=15s \
  --set leaderElection.retryPeriod=3s
```

### Job Concurrency Limits

Control how many jobs can run simultaneously to prevent cluster overload:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.concurrency.maxJobsPerNamespace=5 \
  --set controller.concurrency.maxJobsGlobal=20
```

Jobs exceeding the limit enter a `Queued` phase with backpressure and are automatically dispatched when capacity becomes available. Metrics are exposed for monitoring queue depth and backpressure events.

### Enable Network Policies

For enhanced security:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set networkPolicies.enabled=true
```

## Verification

### Check Installation

```bash
# List all resources
kubectl get all -n forge-system

# Check controller status
kubectl get deployment -n forge-system forge-controller

# View controller logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller -f

# Check CRD installation
kubectl get crd | grep forge.dev
```

### Test the Controller

```bash
# Create a test job
kubectl apply -f examples/samples/zarf/build-only-git.yaml

# Watch the job
kubectl get zarfpackagejobs -A -w

# Describe the job
kubectl describe zarfpackagejob <name> -n <namespace>
```

### Verify Metrics

```bash
# Check metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

## Integrating with Monitoring

### Prometheus Integration

Forge exposes metrics on port 8080 with Prometheus annotations. There are several ways to integrate:

**Option 1: Automatic Discovery via Pod Annotations**

If your Prometheus is configured to discover pods with `prometheus.io/scrape` annotations, metrics will be scraped automatically. This is the default configuration.

**Option 2: ServiceMonitor (Prometheus Operator)**

Create a ServiceMonitor resource:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: forge-controller
  namespace: forge-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: forge
      app.kubernetes.io/component: controller
  endpoints:
  - port: metrics
    path: /metrics
```

**Option 3: Manual Scrape Configuration**

Add to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'forge'
    static_configs:
      - targets: ['forge-controller-metrics.forge-system.svc:8080']
```

### Grafana Dashboards

Create dashboards in your existing Grafana instance to visualize Forge metrics:

- `forge_jobs_total` - Total jobs created
- `forge_jobs_active` - Currently active jobs
- `go_goroutines` - Controller goroutines
- `go_memstats_alloc_bytes` - Memory usage

## Upgrading

### Upgrade Release

```bash
helm upgrade forge forge/forge \
  --namespace forge-system
```

### Verify Upgrade

```bash
helm list -n forge-system
kubectl rollout status deployment/forge-controller -n forge-system
kubectl rollout status deployment/forge-webhook -n forge-system
```

## Uninstalling

### Remove Release

```bash
helm uninstall forge --namespace forge-system
```

### Clean Up (Optional)

```bash
# Remove namespace
kubectl delete namespace forge-system

# Remove CRDs (will delete all ZarfPackageJob resources!)
kubectl delete crd zarfpackagejobs.forge.dev udsbundlejobs.forge.dev
```

## Air-Gapped Deployment (Zarf)

Forge includes a `zarf.yaml` package definition for deploying to air-gapped or disconnected Kubernetes clusters. This is ideal for environments without internet access.

### Zarf Prerequisites

- Zarf CLI installed ([zarf.dev](https://zarf.dev))
- Cluster already initialized with `zarf init`
- Access to a workstation with internet (for package creation)

### Creating the Zarf Package

On a connected workstation:

```bash
# Clone the Forge repository
git clone https://github.com/kylegalloway/forge.git
cd forge

# Create the Zarf package
zarf package create . --confirm
```

This creates `zarf-package-forge-<arch>-<version>.tar.zst` containing:

- **forge** component: Controller, webhook, Helm chart, and CRDs
- **zarfpackagejob** component: Zarf Package Job image for running ZarfPackageJobs
- **image-scanning** component (optional): Trivy and Grype for vulnerability scanning

### Deploying to Air-Gapped Cluster

Transfer the package to the air-gapped environment and deploy:

```bash
# Deploy Forge (includes all required images)
zarf package deploy zarf-package-forge-*.tar.zst --confirm

# Deploy with optional image scanning tools
zarf package deploy zarf-package-forge-*.tar.zst \
  --components=forge,zarfpackagejob,image-scanning \
  --confirm
```

### Package Components

| Component | Required | Description |
|-----------|----------|-------------|
| `forge` | Yes | Controller, webhook, Helm chart |
| `zarfpackagejob` | Yes | Zarf Package Job image for build/deploy jobs |
| `image-scanning` | No | Trivy and Grype scanners |

### Verifying Deployment

```bash
# Check Forge is running
kubectl get pods -n forge-system

# Verify CRDs are installed
kubectl get crd | grep forge.dev

# Test creating a job
kubectl apply -f examples/samples/zarf/04-podinfo/zarfpackagejob.yaml
```

---

## Security Hardening

### Network Policies

Enable network policies for enhanced security:

```bash
helm upgrade forge forge/forge \
  --set networkPolicies.enabled=true \
  --namespace forge-system
```

### Pod Security Standards

The chart enforces restricted Pod Security Standards by default. The namespace is labeled:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/audit: restricted
pod-security.kubernetes.io/warn: restricted
```

### RBAC

The controller uses a dedicated ServiceAccount with minimal permissions:

- Read/write ZarfPackageJob and UDSBundleJob resources
- Create Jobs
- Read ServiceAccounts and Secrets (for validation)
- Create Events
- Manage Leases (for leader election)

## Advanced Configuration

### Custom Resource Limits

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.resources.limits.cpu=2 \
  --set controller.resources.limits.memory=2Gi \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi
```

### Custom Pod Affinity

When `controller.replicaCount > 1`, the chart automatically configures preferred pod anti-affinity to spread replicas across nodes. To override with custom affinity rules (e.g., zone spreading or required anti-affinity), set `controller.affinity`:

```yaml
# custom-values.yaml
controller:
  replicaCount: 3
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/component: controller
        topologyKey: topology.kubernetes.io/zone
```

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  -f custom-values.yaml
```

Setting `controller.affinity` replaces the automatic anti-affinity entirely, giving you full control.

## Troubleshooting

### Common Issues

#### Controller Not Starting

```bash
kubectl describe pod -n forge-system -l app.kubernetes.io/component=controller
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

#### Webhook Validation Failures

```bash
# Check webhook status
kubectl get validatingwebhookconfiguration forge-webhook -o yaml

# Check webhook logs
kubectl logs -n forge-system -l app.kubernetes.io/component=webhook
```

#### Metrics Not Scraped

```bash
# Check metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics

# If using ServiceMonitor, check Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Visit http://localhost:9090/targets
```

## Additional Resources

- **User Guide**: [docs/getting-started/USER_GUIDE.md](docs/getting-started/USER_GUIDE.md) - Complete usage examples
- **Developer Guide**: [docs/getting-started/KIND_SETUP.md](docs/getting-started/KIND_SETUP.md) - Local development setup
- **Helm Chart Documentation**: [chart/README.md](chart/README.md) - Complete chart documentation
- **Helm Chart Values**: [chart/forge/values.yaml](chart/forge/values.yaml) - All configuration options

## Support

- **Issues**: <https://github.com/kylegalloway/forge/issues>
- **Discussions**: <https://github.com/kylegalloway/forge/discussions>
