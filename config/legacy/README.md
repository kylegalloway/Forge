# Legacy Kubernetes Manifests

⚠️ **DEPRECATED**: These raw Kubernetes manifests are provided for backwards compatibility only.

## Recommended Approach

**Use Helm for deployment instead:**

```bash
helm install forge ../../chart/forge --namespace forge-system --create-namespace
```

See [chart/README.md](../../chart/README.md) for complete Helm deployment documentation.

## Why Use Helm?

- ✅ Easier configuration management
- ✅ Support for multiple deployment scenarios (mature vs new clusters)
- ✅ Conditional deployment of observability stack
- ✅ Built-in upgrade and rollback support
- ✅ Values-based configuration (no manual manifest editing)
- ✅ Production-ready defaults with security best practices

## Legacy Deployment (Not Recommended)

If you must use raw manifests:

```bash
# Install CRDs
kubectl apply -f ../crd/

# Install RBAC and controller
kubectl apply -f rbac/
kubectl apply -f manager/
```

**Limitations:**
- Manual configuration required
- No support for observability stack deployment
- No easy upgrade path
- Missing advanced features available in Helm chart

## Migration to Helm

To migrate from legacy manifests to Helm:

```bash
# 1. Uninstall legacy deployment
kubectl delete -f manager/
kubectl delete -f rbac/
# Keep CRDs installed

# 2. Install with Helm
helm install forge ../../chart/forge \
  --namespace forge-system \
  --create-namespace

# CRDs are included in Helm chart's crds/ directory
```

## Contents

- `manager/` - Controller deployment manifests
- `rbac/` - RBAC resources (ServiceAccount, ClusterRole, ClusterRoleBinding)

These manifests were moved here when Helm deployment became the primary installation method.
