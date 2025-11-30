# Forge Examples

Example resources for Forge deployment and usage.

## Directory Structure

```
examples/
├── zarfpackagejobs/     # ZarfPackageJob CRD examples
├── service-accounts/     # ServiceAccount policy examples
└── test-packages/        # Minimal Zarf packages for testing
```

## ZarfPackageJob Examples

See [zarfpackagejobs/](zarfpackagejobs/) for complete workflow examples:

- **[hello-forge-test.yaml](zarfpackagejobs/hello-forge-test.yaml)** - Minimal test that succeeds in Kind
- **[build-only-git.yaml](zarfpackagejobs/build-only-git.yaml)** - Build package from Git source
- **[build-publish-deploy-git.yaml](zarfpackagejobs/build-publish-deploy-git.yaml)** - Full workflow example
- **[deploy-from-oci.yaml](zarfpackagejobs/deploy-from-oci.yaml)** - Deploy pre-built package
- **[publish-s3-to-oci.yaml](zarfpackagejobs/publish-s3-to-oci.yaml)** - Cross-registry publishing
- **[local-dev-testing.yaml](zarfpackagejobs/local-dev-testing.yaml)** - Development/testing workflow

## ServiceAccount Examples

See [service-accounts/](service-accounts/) for policy configuration:

- **[service-account-example.yaml](service-accounts/service-account-example.yaml)** - Policy annotations and RBAC examples

## Test Packages

See [test-packages/](test-packages/) for lightweight Zarf packages:

- **[hello-forge/](test-packages/hello-forge/)** - Minimal package for testing in resource-constrained environments

## Quick Start

```bash
# Create ServiceAccount with policies
kubectl apply -f service-accounts/service-account-example.yaml

# Run a test build
kubectl apply -f zarfpackagejobs/hello-forge-test.yaml

# Check status
kubectl get zarfpackagejobs -A

# View controller logs
kubectl logs -n forge-system -l app=forge-controller -f
```

## Documentation

- **[User Guide](../docs/getting-started/USER_GUIDE.md)** - Complete usage guide
- **[ServiceAccount Reference](../docs/development/SERVICEACCOUNT_REFERENCE.md)** - Policy configuration
- **[KIND Setup](../docs/getting-started/KIND_SETUP.md)** - Local testing with Kind
