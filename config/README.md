# Config Directory

This directory contains reference configurations and alternative deployment options for Forge.

## 📦 Primary Deployment Method: Helm

**For standard deployments, use the Helm chart:** [chart/forge/](../chart/forge/)

```bash
# Install with Helm
helm install forge ../chart/forge --namespace forge-system --create-namespace
```

See [chart/README.md](../chart/README.md) for complete Helm documentation.

## 📁 Directory Contents

### Core Resources

- **[crd/](crd/)** - Custom Resource Definitions
  - `forge.dev_zarfpackagejobs.yaml` - ZarfPackageJob CRD
  - **Note**: Also available in `chart/forge/crds/` (Helm installs automatically)

- **[samples/](samples/)** - Example ZarfPackageJob resources
  - Reference examples for common use cases
  - ServiceAccount policy examples
  - Use these to understand how to create your own jobs

### Alternative Deployments

- **[namespace-scoped/](namespace-scoped/)** - Namespace-scoped deployment
  - For restricted environments without ClusterRole permissions
  - Controller watches only `forge-system` namespace
  - See [docs/NAMESPACE_SCOPED_DEPLOYMENT.md](../docs/NAMESPACE_SCOPED_DEPLOYMENT.md)

### Optional Configurations

- **[namespace-templates/](namespace-templates/)** - Multi-tenant namespace templates
  - User namespace creation templates
  - RBAC templates for tenant isolation
  - Network policy templates

- **[network/](network/)** - Network policy examples
  - Controller network policies
  - Job network policies
  - Use as reference for security hardening

### Reference Materials

- **[grafana/](grafana/)** - Grafana dashboards
  - `forge-dashboard.json` - Pre-built Forge monitoring dashboard
  - Import into your Grafana instance
  - **Note**: Dashboard also available in `chart/forge/dashboards/`

## 🚀 Deployment Comparison

### Helm (Recommended)

**Pros:**
- ✅ Easy configuration via values files
- ✅ Conditional observability stack deployment
- ✅ Built-in upgrade/rollback support
- ✅ Production-ready defaults
- ✅ Two deployment scenarios (mature/new cluster)

**Use when:**
- Standard Kubernetes cluster
- Want observability stack options
- Need easy configuration management

### Namespace-Scoped

**Pros:**
- ✅ Minimal permissions required
- ✅ Isolated to single namespace
- ✅ Compliance-friendly

**Use when:**
- Restricted cluster access (no ClusterRole)
- Single team/namespace usage
- Compliance requires namespace isolation

See [docs/NAMESPACE_SCOPED_DEPLOYMENT.md](../docs/NAMESPACE_SCOPED_DEPLOYMENT.md) for details.

## 📖 Documentation

- **Helm Deployment**: [chart/README.md](../chart/README.md)
- **Quick Start**: [chart/QUICKSTART.md](../chart/QUICKSTART.md)
- **Full Guide**: [DEPLOYMENT.md](../DEPLOYMENT.md)
- **User Guide**: [docs/USER_GUIDE.md](../docs/USER_GUIDE.md)

## 🗑️ What's Not Here Anymore

The following were removed as they're now handled by the Helm chart:

- ❌ `config/manager/` - Controller deployment (use Helm)
- ❌ `config/rbac/` - RBAC resources (use Helm)
- ❌ `config/metrics/` - Metrics service/ServiceMonitor (use Helm)
- ❌ `config/otel-collector/` - OTEL collector deployment (use Helm)
- ❌ `config/prometheus/` - Prometheus alerts (use Helm)

All of these are available as Helm templates in `chart/forge/templates/`.

## 🔄 Migration from Old Configs

If you were using raw manifests before:

```bash
# Old way (deprecated)
kubectl apply -f config/manager/
kubectl apply -f config/rbac/

# New way (use Helm)
helm install forge ../chart/forge --namespace forge-system --create-namespace
```

Git history preserves all old manifests if you need them for reference.
