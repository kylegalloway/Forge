# Forge Examples

Example resources demonstrating Forge's Zarf package and UDS bundle operations.

## Directory Structure

```text
examples/
├── samples/              # Complete workflow examples
│   ├── zarf/            # ZarfPackageJob examples
│   └── uds/             # UDSBundleJob examples
├── policies/            # Policy examples by type
│   └── uds/             # UDS-specific policies
└── service-accounts/     # ServiceAccount policy examples
```

## Examples vs Tests

This directory contains **reference material** for users to learn Forge workflows. For **automated testing**, see:

**`examples/`** - Reference examples (this directory)
- Complex workflows with real-world packages
- Comprehensive documentation and prerequisites
- Includes credentials setup, monitoring, troubleshooting
- Educational value for understanding Forge capabilities

**[`tests/e2e/`](../tests/e2e/)** - Automated functional tests
- Simple, focused tests for CI/CD
- Work on both Kind and production clusters
- Used by `make e2e-test`
- No external dependencies (public repos only)

| Workflow Type | Where to Find It | Purpose |
|---------------|------------------|---------|
| Complex real-world workflows | `examples/samples/` | Learning and customization |
| Simple automated tests | `tests/e2e/` | CI/CD and verification |
| Policy configurations | `examples/policies/`, `examples/service-accounts/` | RBAC reference |

## Workflow Examples

### Zarf Package Operations

The [samples/zarf/](samples/zarf/) directory contains complete ZarfPackageJob examples:

- **[01-git-to-oci/](samples/zarf/01-git-to-oci/)** - Build from Git, publish to OCI registry
  - Demonstrates Git source integration
  - Shows OCI registry publishing
  - Includes ServiceAccount with appropriate policies

- **[02-local-to-s3/](samples/zarf/02-local-to-s3/)** - Build from local, publish to S3
  - Demonstrates local filesystem source
  - Shows S3 bucket publishing
  - Includes PVC configuration for local files

See [samples/zarf/README.md](samples/zarf/README.md) for detailed documentation.

### UDS Bundle Operations

The [samples/uds/](samples/uds/) directory contains complete UDSBundleJob examples:

- **[01-git-to-oci/](samples/uds/01-git-to-oci/)** - Create from Git, publish to OCI registry
  - Demonstrates UDS bundle creation from Git
  - Shows multi-package bundle structure
  - Includes OCI publishing workflow

- **[02-local-to-s3/](samples/uds/02-local-to-s3/)** - Create from local, publish to S3
  - Demonstrates local bundle creation
  - Shows complex multi-package bundles
  - Includes S3 publishing workflow

See [samples/uds/README.md](samples/uds/README.md) for detailed documentation.

## ServiceAccount Policy Examples

The [service-accounts/](service-accounts/) directory contains policy configuration examples:

- **[service-account-example.yaml](service-accounts/service-account-example.yaml)**
  - Multiple ServiceAccounts demonstrating different policy levels
  - Developer, production, and platform-team examples
  - Comprehensive annotation reference
  - Multi-namespace configuration

- **[simple-test-sa.yaml](service-accounts/simple-test-sa.yaml)**
  - Single ServiceAccount for quick testing
  - Permissive policies for development
  - Default namespace only
  - Ideal for Kind local development

See the [ServiceAccount Reference](../docs/development/SERVICEACCOUNT_REFERENCE.md) for complete policy documentation.

## Quick Start

### For Zarf Packages

```bash
# 1. Install Forge
helm install forge forge/forge --namespace forge-system --create-namespace

# 2. Create ServiceAccount with policies
kubectl apply -f examples/service-accounts/simple-test-sa.yaml

# 3. Run Git-to-OCI example (update OCI credentials first)
kubectl apply -f examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml

# 4. Check status
kubectl get zarfpackagejobs -w
kubectl logs -n forge-system -l app=forge-controller -f
```

### For UDS Bundles

```bash
# 1. Install Forge with UDS support
helm install forge forge/forge --namespace forge-system --create-namespace

# 2. Create ServiceAccount with bundle policies
kubectl apply -f examples/service-accounts/simple-test-sa.yaml

# 3. Run Git-to-OCI bundle example (update OCI credentials first)
kubectl apply -f examples/samples/uds/01-git-to-oci/udsbundlejob.yaml

# 4. Check status
kubectl get udsbundlejobs -w
kubectl logs -n forge-system -l app=forge-controller -f
```

## Prerequisites

Before running examples, ensure you have:

1. **Forge Installed**: Controller and webhook running in your cluster
2. **Credentials Configured**: Secrets for OCI registries or S3 buckets
3. **ServiceAccount Created**: With appropriate policy annotations
4. **Namespace Access**: Permissions to create jobs in the target namespace

See [docs/getting-started/KIND_SETUP.md](../docs/getting-started/KIND_SETUP.md) for local development setup.

## Documentation

- **[User Guide](../docs/getting-started/USER_GUIDE.md)** - Complete usage guide
- **[ServiceAccount Reference](../docs/development/SERVICEACCOUNT_REFERENCE.md)** - Policy configuration
- **[KIND Setup](../docs/getting-started/KIND_SETUP.md)** - Local testing with Kind
- **[Troubleshooting](../docs/operations/TROUBLESHOOTING.md)** - Common issues and solutions

## Workflow Coverage

| Workflow | Examples | Tests | Notes |
|----------|----------|-------|-------|
| Git → OCI | ✅ Zarf, UDS | ⏸️ | Reference examples (samples/) |
| Local → S3 | ✅ Zarf, UDS | ⏸️ | Reference examples (samples/) |
| BuildDeploy | ✅ Zarf, UDS | ⏸️ | Reference examples (samples/) |
| Build only | ⏸️ | ✅ | Automated test (tests/e2e/01) |
| Deploy only | ⏸️ | ✅ | Automated test (tests/e2e/02) |
| Health check | ⏸️ | ✅ | Automated test (tests/e2e/03) |
| Full pipeline | ⏸️ | ⏸️ | BuildPublishDeploy (planned) |

## Contributing Examples

When adding new examples:

1. Create numbered directory (e.g., `04-deploy-only-oci/`)
2. Include complete YAML files (resource + ServiceAccount + Secret)
3. Add descriptive README with workflow explanation
4. Update this main README with the new example
5. Test in Kind before committing

## Related Documentation

- **[User Guide](../docs/getting-started/USER_GUIDE.md)** - Complete usage guide
- **[Automated Tests](../tests/e2e/)** - Simple tests for CI/CD
- **[ServiceAccount Reference](../docs/development/SERVICEACCOUNT_REFERENCE.md)** - Policy configuration
- **[KIND Setup](../docs/getting-started/KIND_SETUP.md)** - Local testing with Kind
- **[Troubleshooting](../docs/operations/TROUBLESHOOTING.md)** - Common issues and solutions
