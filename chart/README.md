# Forge Helm Chart

This directory contains the Helm chart for deploying Forge - a Kubernetes controller for managing Zarf package build and deployment jobs.

## Overview

The Forge Helm chart deploys:

- **Forge Controller** - Manages ZarfPackageJob and UDSBundleJob resources
- **Forge Webhook** - Validates resources at admission time
- **Metrics Endpoints** - Exposes metrics for scraping by external Prometheus

Forge **does not** bundle Grafana, Prometheus, or OTEL Collector. It exposes metrics endpoints that your existing monitoring infrastructure can scrape.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- (Optional) Prometheus for metrics collection
- (Optional) Grafana for visualization

## Installation

### For Users (Published Chart)

Install from the public Helm repository:

```bash
# Add the Forge Helm repository
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install latest version
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace

# Or install specific version
helm install forge forge/forge \
  --version 0.6.0 \
  --namespace forge-system \
  --create-namespace
```

**Available Images**:

- Controller: `ghcr.io/kylegalloway/forge/forge-controller:v0.6.0`
- Webhook: `ghcr.io/kylegalloway/forge/forge-webhook:v0.6.0`
- Zarf Package Job: `ghcr.io/kylegalloway/forge/zarfpackagejob:v0.11.1` (used by ZarfPackageJobs)

### For Developers (Local Chart)

Install from local source code for development:

```bash
# Build images locally
make container-build

# Install with local images
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.repository=forge-controller \
  --set controller.image.tag=demo \
  --set webhook.image.repository=forge-webhook \
  --set webhook.image.tag=demo
```

See [docs/getting-started/KIND_SETUP.md](../docs/getting-started/KIND_SETUP.md) for complete developer workflow.

## Chart Structure

```text
chart/forge/
├── Chart.yaml                          # Chart metadata
├── values.yaml                         # Default values
├── crds/                               # Custom Resource Definitions
│   ├── forge.dev_zarfpackagejobs.yaml
│   └── forge.dev_udsbundlejobs.yaml   # v1alpha2 API
└── templates/                          # Kubernetes manifests templates
    ├── _helpers.tpl                    # Template helpers
    ├── NOTES.txt                       # Post-install notes
    ├── namespace.yaml                  # Namespace
    ├── controller/                     # Controller resources
    │   ├── deployment.yaml
    │   ├── serviceaccount.yaml
    │   ├── rbac.yaml
    │   └── service.yaml
    ├── webhook/                        # Webhook resources
    │   ├── deployment.yaml
    │   ├── service.yaml
    │   ├── rbac.yaml
    │   └── validatingwebhookconfiguration.yaml
    └── networkpolicy.yaml              # Network policies
```

## Configuration

### Key Configuration Options

#### Controller Settings

```yaml
controller:
  replicaCount: 1              # Number of controller replicas
  image:
    repository: ghcr.io/kylegalloway/forge/forge-controller
    tag: "v0.6.0"
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

#### Webhook Settings

```yaml
webhook:
  enabled: true                # Enable webhook deployment
  replicaCount: 2              # Number of webhook replicas (recommend 2+ for HA)
  image:
    repository: ghcr.io/kylegalloway/forge/forge-webhook
    tag: "v0.6.0"
  tls:
    autoGenerate: true         # Auto-generate self-signed certs
```

#### CLI Images

Forge uses containerized CLI tools for build and deploy operations. These can be customized:

**Zarf CLI** - Used by ZarfPackageJobs:

```yaml
zarfCLI:
  image:
    repository: ghcr.io/kylegalloway/forge/zarfpackagejob
    tag: v0.69.0
    pullPolicy: IfNotPresent
```

**UDS CLI** - Used by UDSBundleJobs:

```yaml
udsCLI:
  image:
    repository: ghcr.io/defenseunicorns/uds-cli
    tag: v0.27.21
    pullPolicy: IfNotPresent
```

The CLI images are passed to the controller via environment variables (`FORGE_ZARF_CLI_IMAGE`, `FORGE_UDS_CLI_IMAGE`), allowing runtime configuration without rebuilding the controller.

#### Metrics

```yaml
controller:
  podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
  metrics:
    enabled: true
    port: 8080
    path: /metrics
```

The controller exposes Prometheus metrics on port 8080. Configure your external Prometheus to scrape this endpoint using Pod annotations or ServiceMonitors.

#### Network Policy Configuration

```yaml
networkPolicies:
  enabled: false               # Enable network policies for security
```

## Installation Examples

### Default Installation

```bash
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace
```

### With Custom Values

```bash
helm upgrade --install forge forge/forge \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3 \
  --namespace forge-system \
  --create-namespace
```

### Upgrade an Existing Installation

```bash
helm upgrade forge forge/forge \
  --namespace forge-system
```

### Uninstall

```bash
helm uninstall forge --namespace forge-system
```

**Note:** CRDs are not automatically removed. To remove them:

```bash
kubectl delete crd zarfpackagejobs.forge.dev udsbundlejobs.forge.dev
```

## Monitoring and Observability

### Metrics Endpoints

The controller exposes metrics on port 8080:

```bash
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Integrating with Prometheus

**Option 1: Pod Annotations (Automatic)**

The default `values.yaml` includes Prometheus scraping annotations on the controller pods. If your Prometheus is configured to discover pods with these annotations, metrics will be scraped automatically.

**Option 2: ServiceMonitor (Prometheus Operator)**

If you're using Prometheus Operator, you can create a ServiceMonitor:

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

Add a scrape job to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'forge-controller'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - forge-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        target_label: __address__
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
```

### Grafana Dashboards

While Forge doesn't bundle Grafana, you can visualize Forge metrics in your existing Grafana instance by:

1. Configuring Prometheus as a data source
2. Creating dashboards for Forge metrics
3. Importing community dashboards (if available)

Example metrics available:

- `forge_jobs_total` - Total jobs created
- `forge_jobs_active` - Currently active jobs
- `go_goroutines` - Controller goroutines
- `go_memstats_alloc_bytes` - Memory usage

## Security Considerations

### Pod Security Standards

The chart enforces restricted Pod Security Standards:

```yaml
podSecurityStandards:
  enforce: restricted
  audit: restricted
  warn: restricted
```

### Security Contexts

All containers run with:

- Non-root user (UID 65532 for controller, 65533 for webhook)
- Read-only root filesystem
- Dropped capabilities
- No privilege escalation

### Network Policies

Enable network policies for defense in depth:

```yaml
networkPolicies:
  enabled: true
```

This restricts:

- Ingress to metrics and health endpoints only
- Egress to Kubernetes API and DNS only

### RBAC

The chart follows least-privilege principles:

- Controller only has permissions for required resources
- ServiceAccount is dedicated to Forge
- ClusterRole is scoped to necessary operations

## Troubleshooting

### Controller Not Starting

```bash
# Check pod status
kubectl get pods -n forge-system -l app.kubernetes.io/component=controller

# Check logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller

# Check events
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

### Webhook Validation Failures

```bash
# Check webhook pod status
kubectl get pods -n forge-system -l app.kubernetes.io/component=webhook

# Check webhook logs
kubectl logs -n forge-system -l app.kubernetes.io/component=webhook

# Verify webhook configuration
kubectl get validatingwebhookconfiguration forge-webhook -o yaml
```

### Metrics Not Being Scraped

```bash
# Verify metrics endpoint is accessible
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics

# Check Prometheus targets (if using Prometheus Operator)
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Visit http://localhost:9090/targets

# Verify ServiceMonitor (if using one)
kubectl get servicemonitor -n forge-system
```

### CRD Installation Issues

CRDs are installed automatically from the `crds/` directory. If there are issues:

```bash
# Manually install CRDs
kubectl apply -f chart/forge/crds/forge.dev_zarfpackagejobs.yaml
kubectl apply -f chart/forge/crds/forge.dev_udsbundlejobs.yaml

# Verify CRDs are installed
kubectl get crd | grep forge.dev
```

## Values Reference

See [values.yaml](forge/values.yaml) for complete configuration options with inline documentation.

Key sections:

- `controller` - Controller deployment configuration
- `webhook` - Webhook deployment configuration
- `zarfCLI` - Zarf CLI image configuration for jobs
- `serviceAccount` - ServiceAccount configuration
- `rbac` - RBAC configuration
- `networkPolicies` - Network policy configuration
- `podSecurityStandards` - Pod security settings

## Contributing

See the main project [README](../README.md) for contribution guidelines.

## License

See the main project [LICENSE](../LICENSE) file.
