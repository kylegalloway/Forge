# Forge Examples

This directory contains example resources and configurations for Forge.

## 📋 ZarfPackageJob Examples

See [samples/](samples/) for complete ZarfPackageJob resource examples:

- **Build workflows** - Building Zarf packages from Git, S3, or OCI sources
- **Publish workflows** - Publishing packages to S3 or OCI registries
- **Deploy workflows** - Deploying packages to clusters
- **Multi-action workflows** - Combined build/publish/deploy operations
- **ServiceAccount examples** - Policy configuration examples

## 🚀 Quick Start

After installing Forge with Helm:

```bash
# Apply a simple build-only example
kubectl apply -f samples/v1alpha1/build-only-git.yaml

# Check the job status
kubectl get zarfpackagejobs -A

# View logs
kubectl logs -n forge-system -l app=forge-controller
```

## 📖 Documentation

For complete documentation on creating ZarfPackageJobs:

- [User Guide](../docs/USER_GUIDE.md) - Complete usage guide
- [ServiceAccount Reference](../docs/SERVICEACCOUNT_REFERENCE.md) - Policy configuration
- [Helm Chart](../chart/README.md) - Deployment configuration

## 🔐 ServiceAccount Policies

See [samples/service-account-example.yaml](samples/service-account-example.yaml) for examples of:

- Allowed actions configuration
- Repository whitelisting
- Registry access control
- Namespace restrictions
