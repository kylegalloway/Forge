# Forge Helm Chart

Helm chart for deploying Forge - a Kubernetes controller for managing Zarf package build and deployment jobs.

## Installation

### Default Installation

```bash
helm upgrade --install forge . --namespace forge-system --create-namespace
```

### From Local Source (Developers)

```bash
# From the project root
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.image.repository=forge-controller \
  --set controller.image.tag=demo \
  --set webhook.image.repository=forge-webhook \
  --set webhook.image.tag=demo
```

## Configuration

See [values.yaml](values.yaml) for all configuration options.

### Key Configuration Areas

- **Controller**: Deployment, resources, security settings
- **Webhook**: Admission webhook configuration
- **Metrics**: Metrics endpoint configuration (for external Prometheus)
- **RBAC**: Service account and permissions
- **Network Policies**: Pod communication restrictions

## What Gets Deployed

- Forge Controller (manages ZarfPackageJob/UDSBundleJob resources)
- Forge Webhook (validates resources at admission time)
- Metrics Service (exposes metrics for external Prometheus)
- CRDs (ZarfPackageJob, UDSBundleJob)
- RBAC resources (ServiceAccount, ClusterRole, ClusterRoleBinding)

**What does NOT get deployed:**

- Grafana (install separately if needed)
- Prometheus (install separately if needed)
- OTEL Collector (install separately if needed)

## Monitoring Integration

Forge exposes Prometheus metrics on port 8080. The controller pods include annotations for automatic Prometheus discovery:

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

Configure your external Prometheus to scrape these endpoints.

## Documentation

For complete documentation, see:

- [Project README](../../README.md) - Project overview
- [Chart Documentation](../README.md) - Complete chart documentation
- [User Guide](../../docs/getting-started/USER_GUIDE.md) - Usage examples
- [Developer Guide](../../docs/getting-started/KIND_SETUP.md) - Local development setup

## Upgrading

```bash
helm upgrade forge . --namespace forge-system
```

## Uninstalling

```bash
helm uninstall forge --namespace forge-system
```

**Note:** CRDs are not automatically removed.

## Support

For issues and questions, visit: https://github.com/kylegalloway/forge/issues
