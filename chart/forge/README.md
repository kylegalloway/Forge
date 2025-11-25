# Forge Helm Chart

Helm chart for deploying Forge - a Kubernetes controller for managing Zarf package build and deployment jobs.

## Installation

### Mature Cluster (with existing monitoring)

```bash
helm install forge . -f values-mature-cluster.yaml --namespace forge-system --create-namespace
```

### New Cluster (deploy full observability stack)

```bash
helm install forge . -f values-new-cluster.yaml --namespace forge-system --create-namespace
```

### Default Installation

```bash
helm install forge . --namespace forge-system --create-namespace
```

## Configuration

See [values.yaml](values.yaml) for all configuration options.

### Key Configuration Areas

- **Controller**: Deployment, resources, security settings
- **Observability**: OTEL Collector, Prometheus, Grafana deployment options
- **Metrics**: ServiceMonitor and metrics service configuration
- **RBAC**: Service account and permissions
- **Network Policies**: Pod communication restrictions
- **Alerts**: PrometheusRule configuration

## Documentation

For complete documentation, see the [Chart README](../README.md).

## Deployment Scenarios

### 1. Mature Cluster

Use `values-mature-cluster.yaml` when you have:
- Existing Prometheus setup
- Existing Grafana instance
- Existing OTEL Collector

This deploys only the Forge controller and connects to your existing infrastructure.

### 2. New Cluster

Use `values-new-cluster.yaml` when you need:
- Full observability stack
- Prometheus for metrics
- Grafana for visualization
- OTEL Collector for telemetry

This deploys everything needed for complete observability.

## Upgrading

```bash
helm upgrade forge . -f values-mature-cluster.yaml --namespace forge-system
```

## Uninstalling

```bash
helm uninstall forge --namespace forge-system
```

**Note:** CRDs are not automatically removed.

## Support

For issues and questions, visit: https://github.com/kylegalloway/forge/issues
