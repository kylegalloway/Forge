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
  --version 0.6.0 \
  --namespace forge-system \
  --create-namespace
```

**Container Images Used**:

- `ghcr.io/kylegalloway/forge/forge-controller:v0.6.0`
- `ghcr.io/kylegalloway/forge/forge-webhook:v0.6.0`
- `ghcr.io/kylegalloway/forge/zarf-cli:v0.69.0` (used by ZarfPackageJobs)
- `ghcr.io/kylegalloway/forge/uds-cli:v0.27.21` (used by UDSBundleJobs)

**CLI Image Configuration**:

CLI images can be customized via Helm values or environment variables:

```bash
# Via Helm values
helm install forge forge/forge \
  --set zarfCLI.image.repository=my-registry.io/zarf-cli \
  --set zarfCLI.image.tag=v0.69.0 \
  --set udsCLI.image.repository=my-registry.io/uds-cli \
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
  --version 0.6.0 \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3 \
  --set controller.resources.limits.memory=1Gi
```

### High Availability

For production environments:

```bash
helm install forge forge/forge \
  --version 0.6.0 \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set webhook.replicaCount=3 \
  --set leaderElection.enabled=true
```

### Enable Network Policies

For enhanced security:

```bash
helm install forge forge/forge \
  --version 0.6.0 \
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
  --version 0.6.0 \
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

This creates `zarf-package-forge-<arch>-v0.6.0.tar.zst` containing:

- **forge** component: Controller, webhook, Helm chart, and CRDs
- **zarf-cli** component: Zarf CLI image for running ZarfPackageJobs
- **image-scanning** component (optional): Trivy and Grype for vulnerability scanning

### Deploying to Air-Gapped Cluster

Transfer the package to the air-gapped environment and deploy:

```bash
# Deploy Forge (includes all required images)
zarf package deploy zarf-package-forge-*.tar.zst --confirm

# Deploy with optional image scanning tools
zarf package deploy zarf-package-forge-*.tar.zst \
  --components=forge,zarf-cli,image-scanning \
  --confirm
```

### Package Components

| Component | Required | Description |
|-----------|----------|-------------|
| `forge` | Yes | Controller, webhook, Helm chart |
| `zarf-cli` | Yes | Zarf CLI image for build/deploy jobs |
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

### Pod Anti-Affinity (HA)

For high availability, spread controller replicas across nodes:

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set controller.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight=100 \
  --set controller.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels.app=forge-controller \
  --set controller.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey=kubernetes.io/hostname
```

Or use a custom values file:

```yaml
# custom-values.yaml
controller:
  replicaCount: 3
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/component: controller
          topologyKey: kubernetes.io/hostname
```

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  -f custom-values.yaml
```

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
