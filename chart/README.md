# Forge Helm Chart

This directory contains the Helm chart for deploying Forge - a Kubernetes controller for managing Zarf package build and deployment jobs.

## Overview

The Forge Helm chart supports two primary deployment scenarios:

1. **Mature Cluster** - Deploy into a cluster with existing observability infrastructure (Grafana, Prometheus, OTEL Collector)
2. **New Cluster** - Deploy with a full observability stack included

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- (Optional) Prometheus Operator CRDs if using ServiceMonitors

## Chart Structure

```text
chart/forge/
├── Chart.yaml                          # Chart metadata
├── values.yaml                         # Default values
├── values-mature-cluster.yaml          # Values for mature cluster deployment
├── values-new-cluster.yaml             # Values for new cluster deployment
├── crds/                               # Custom Resource Definitions
│   └── forge.dev_zarfpackagejobs.yaml
└── templates/                          # Kubernetes manifests templates
    ├── _helpers.tpl                    # Template helpers
    ├── NOTES.txt                       # Post-install notes
    ├── namespace.yaml                  # Namespace
    ├── controller-deployment.yaml      # Controller deployment
    ├── serviceaccount.yaml             # Service account
    ├── rbac.yaml                       # RBAC resources
    ├── metrics-service.yaml            # Metrics service
    ├── servicemonitor.yaml             # Prometheus ServiceMonitor
    ├── prometheusrule.yaml             # Prometheus alerts
    ├── networkpolicy.yaml              # Network policies
    ├── otel-collector-*.yaml           # OTEL Collector resources
    └── crd.yaml                        # CRD installation template
```

## Quick Start

### Scenario 1: Deploy to a Mature Cluster

If you already have Grafana, Prometheus, and OTEL Collector deployed:

```bash
# Install Forge without observability stack
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system \
  --create-namespace
```

**Important:** Edit `values-mature-cluster.yaml` to configure your existing observability endpoints:

```yaml
observability:
  deployStack: false
  otelCollector:
    external:
      endpoint: "otel-collector.monitoring.svc.cluster.local:4317"
  prometheus:
    external:
      url: "http://prometheus.monitoring.svc.cluster.local:9090"
  grafana:
    external:
      url: "http://grafana.monitoring.svc.cluster.local"
```

### Scenario 2: Deploy to a New Cluster

If you need a complete observability stack:

```bash
# Install Forge with full observability stack
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-new-cluster.yaml \
  --namespace forge-system \
  --create-namespace
```

**Important:** Before deploying, update the Grafana admin password in `values-new-cluster.yaml`:

```yaml
observability:
  grafana:
    config:
      adminPassword: "YOUR-STRONG-PASSWORD-HERE"
```

## Configuration

### Key Configuration Options

#### Controller Settings

```yaml
controller:
  replicaCount: 1              # Number of controller replicas
  image:
    repository: forge-controller
    tag: "latest"
  resources:
    limits:
      cpu: "1"
      memory: 512Mi
    requests:
      cpu: 250m
      memory: 256Mi
```

#### Observability Stack

```yaml
observability:
  deployStack: true            # Set to false for mature clusters

  otelCollector:
    enabled: true              # Deploy OTEL Collector
    external:
      endpoint: ""             # Use existing OTEL endpoint

  prometheus:
    enabled: true              # Deploy Prometheus
    config:
      retention: 15d
      persistence:
        enabled: true
        size: 50Gi

  grafana:
    enabled: true              # Deploy Grafana
    config:
      adminPassword: "changeme"
      ingress:
        enabled: false
        hosts:
          - grafana.example.com
```

#### Metrics and Monitoring

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true              # Create ServiceMonitor for Prometheus Operator
    additionalLabels:
      prometheus: kube-prometheus

alerts:
  enabled: true                # Create PrometheusRule with alerts
```

#### Network Policy Configuration

```yaml
networkPolicies:
  enabled: false               # Enable network policies for security
```

## Installation Examples

### Install with Custom Values

```bash
helm upgrade --install forge ./chart/forge \
  --set controller.replicaCount=2 \
  --set observability.deployStack=false \
  --namespace forge-system \
  --create-namespace
```

### Upgrade an Existing Installation

```bash
helm upgrade forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system
```

### Uninstall

```bash
helm uninstall forge --namespace forge-system
```

**Note:** CRDs are not automatically removed. To remove them:

```bash
kubectl delete crd zarfpackagejobs.forge.dev
```

## Deployment Scenarios in Detail

### Mature Cluster Deployment

In a mature cluster, you already have:

- Prometheus Operator with ServiceMonitor support
- Grafana with configured dashboards
- OTEL Collector for telemetry aggregation

**What gets deployed:**

- ✅ Forge Controller
- ✅ ServiceMonitor (points to existing Prometheus)
- ✅ PrometheusRule (alerts for existing Prometheus)
- ❌ OTEL Collector (uses existing)
- ❌ Prometheus (uses existing)
- ❌ Grafana (uses existing)

**Configuration checklist:**

1. Set `observability.deployStack: false`
2. Configure external OTEL endpoint
3. Configure external Prometheus URL
4. Configure external Grafana URL
5. Ensure ServiceMonitor labels match your Prometheus selector
6. Import Forge dashboard into your Grafana (from `chart/forge/dashboards/forge-dashboard.json`)

### New Cluster Deployment

In a new cluster, you need a complete monitoring setup.

**What gets deployed:**

- ✅ Forge Controller
- ✅ OpenTelemetry Collector
- ✅ Prometheus (via configuration or subchart)
- ✅ Grafana (via configuration or subchart)
- ✅ ServiceMonitors
- ✅ PrometheusRules
- ✅ Forge Dashboard (pre-loaded in Grafana)

**Configuration checklist:**

1. Set `observability.deployStack: true`
2. Configure storage classes for Prometheus and Grafana persistence
3. Set strong Grafana admin password
4. (Optional) Configure Grafana ingress for external access
5. (Optional) Configure TLS certificates
6. Review and adjust resource limits based on cluster capacity

## Monitoring and Observability

### Metrics Endpoints

The controller exposes metrics on port 8080:

```bash
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Prometheus Alerts

The chart includes pre-configured alerts for:

- Controller health and availability
- High error rates in reconciliation
- Slow reconciliation performance
- Job creation failures
- Webhook validation issues
- Resource capacity concerns

### Grafana Dashboard

A pre-built dashboard is available at `chart/forge/dashboards/forge-dashboard.json`. It includes:

- Controller health status
- Job creation and completion rates
- Error rates and types
- Reconciliation latency
- Resource usage metrics

### OTEL Collector Integration

The controller can export telemetry to OTEL Collector via:

- OTLP gRPC (port 4317)
- OTLP HTTP (port 4318)

The OTEL Collector then forwards to:

- Prometheus (for metrics)
- Jaeger (for traces, if configured)
- Other backends (Datadog, New Relic, Honeycomb, etc.)

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

- Non-root user (UID 65532 for controller, 10001 for OTEL)
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
- Egress to Kubernetes API, DNS, and OTEL endpoints only

### RBAC

The chart follows least-privilege principles:

- Controller only has permissions for required resources
- ServiceAccount is dedicated to Forge
- ClusterRole is scoped to necessary operations

## Troubleshooting

### Controller Not Starting

```bash
# Check pod status
kubectl get pods -n forge-system -l app=forge-controller

# Check logs
kubectl logs -n forge-system -l app=forge-controller

# Check events
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

### Metrics Not Being Scraped

```bash
# Verify ServiceMonitor exists
kubectl get servicemonitor -n forge-system

# Check ServiceMonitor labels match Prometheus selector
kubectl describe servicemonitor -n forge-system forge-controller

# Verify Prometheus can reach the metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

### OTEL Collector Connection Issues

```bash
# Check OTEL Collector status
kubectl get pods -n forge-system -l app=otel-collector

# Check OTEL Collector logs
kubectl logs -n forge-system -l app=otel-collector

# Verify OTEL service is reachable
kubectl get svc -n forge-system forge-otel-collector
```

### CRD Installation Issues

CRDs are installed automatically from the `crds/` directory. If there are issues:

```bash
# Manually install CRDs
kubectl apply -f chart/forge/crds/forge.dev_zarfpackagejobs.yaml

# Verify CRD is installed
kubectl get crd zarfpackagejobs.forge.dev
```

## Values Reference

See [values.yaml](forge/values.yaml) for complete configuration options with inline documentation.

## Contributing

See the main project [README](../README.md) for contribution guidelines.

## License

See the main project [LICENSE](../LICENSE) file.
