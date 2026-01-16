# Forge - Kubernetes-Native Zarf Package Operations

**Forge** where Zarf packages are built, published, and deployed with declarative ops and actual security.

> **Status**: Production-ready foundation complete (81% complete). Core functionality tested and documented. Ready for deployment in scoped environments.

## What is Forge?

Forge is a Kubernetes controller that brings Zarf package and UDS bundle operations into the declarative Kubernetes world. Instead of running arbitrary scripts (security nightmare), Forge provides purpose-built operations with fine-grained RBAC controls.

### What it does

- **Build** Zarf packages or UDS bundles from Git repos, S3, or OCI registries
- **Publish** artifacts to S3 or OCI registries
- **Deploy** packages/bundles to in-cluster or external Kubernetes clusters
- **Enforce policies** on who can do what with which resources

## Quick Example

```yaml
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: build-and-deploy
  namespace: default  # cluster-wide: any namespace; namespace-scoped: forge-system only
spec:
  # Who can use this (ServiceAccount with policy annotations)
  serviceAccountName: platform-sa

  # What to do
  action: BuildDeploy

  # Where to get it
  source:
    type: Git
    git:
      url: https://github.com/stefanprodan/podinfo
      ref: 6.7.0
      path: charts/podinfo

  # Where to deploy it
  deploy:
    target: InCluster
    namespace: bigbang
    timeout: 60m
```

> **Note**: For UDS bundles, use `UDSBundleJob` with the v1alpha2 API. The v1alpha1 `UDSBundleJob` API has been removed.

## Architecture

### Actions (What You Can Do)

| Action | Description | Input | Output |
|--------|-------------|-------|--------|
| `Build` | Build Zarf package from source | Git repo or local path | Package artifact |
| `Publish` | Publish artifact to registry | Artifact | Published location |
| `Deploy` | Deploy package to cluster | Artifact | Deployed resources |
| `BuildPublish` | Build + immediately publish | Source | Published location |
| `BuildDeploy` | Build + immediately deploy | Source | Deployed resources |
| `PublishDeploy` | Publish pre-built + deploy | Artifact | Deployed resources |
| `BuildPublishDeploy` | Full pipeline | Source | Deployed resources |

### Source Types (Where Packages Come From)

| Type | Use Cases | Auth Required | Restrictions |
|------|-----------|---------------|--------------|
| `Git` | Source code repos | SSH key or token | HTTPS only, approved repos |
| `S3` | Pre-built artifacts | AWS credentials | Approved buckets |
| `OCI` | OCI registries | Registry credentials | Approved registries |
| `Local` | Dev/testing ONLY | None | Must set `allow-local-sources: true` |

### Destinations (Where Artifacts Go)

| Type | Use Cases | Auth Required |
|------|-----------|---------------|
| `S3` | Artifact storage | AWS credentials |
| `OCI` | Container registries | Registry credentials |
| `Local` | Testing only | None (dev mode) |

### Deploy Targets

| Type | Description | Auth Required |
|------|-------------|---------------|
| `InCluster` | Same cluster as Forge | ServiceAccount |
| `ExternalCluster` | Different cluster | Kubeconfig secret |

## Policy Enforcement

Forge uses ServiceAccount annotations for fine-grained access control. See [SERVICEACCOUNT_REFERENCE.md](docs/development/SERVICEACCOUNT_REFERENCE.md) for complete reference.

**Quick Example:**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dev-team-sa
  namespace: dev-team  # cluster-wide mode
  # namespace: forge-system  # namespace-scoped mode (all SAs must be here)
  annotations:
    # What actions are allowed
    forge.dev/allowed-actions: "Build,Publish"

    # Which Git repos can be used
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"

    # Where packages can be published
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/dev/*"
```

The admission webhook validates all operations against these policies before creation.

## Installation

### For Users (Recommended)

Install Forge from the published Helm repository and container images:

```bash
# Add the Forge Helm repository
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install Forge (uses published images from ghcr.io)
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --version 0.7.1
```

**Container Images**:
- `ghcr.io/kylegalloway/forge/forge-controller:v0.7.1`
- `ghcr.io/kylegalloway/forge/forge-webhook:v0.7.1`
- `ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1` (used by ZarfPackageJobs)

**Custom Configuration**:

```bash
helm install forge forge/forge \
  --version 0.7.1 \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3 \
  --set networkPolicies.enabled=true
```

📖 **User Guide**: See [docs/getting-started/USER_GUIDE.md](docs/getting-started/USER_GUIDE.md) for complete usage examples
📖 **Deployment Guide**: See [docs/getting-started/DEPLOYMENT.md](docs/getting-started/DEPLOYMENT.md) for deployment scenarios and configurations
📖 **Helm Chart Docs**: See [chart/README.md](chart/README.md) for all configuration options

### For Testing

Want to try Forge locally without building from source? Use Kind with published images:

```bash
# Create Kind cluster
kind create cluster --name forge-test

# Add Helm repo and install Forge
helm repo add forge https://kylegalloway.github.io/forge
helm install forge forge/forge --namespace forge-system --create-namespace --wait

# Pull and load Zarf CLI image (Kind can't pull from registries)
docker pull ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1
kind load docker-image ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1 --name forge-test
```

📖 **Testing Guide**: See [docs/development/KIND_TESTING_PUBLIC_IMAGES.md](docs/development/KIND_TESTING_PUBLIC_IMAGES.md) for complete testing setup

### For Developers

For local development with custom builds and iteration:

**Prerequisites**:

- Docker or Podman
- Kind (Kubernetes in Docker)
- Helm
- kubectl

**Quick Start**:

```bash
# Complete setup: create Kind cluster, build image, deploy
make kind-setup

# Apply sample resources
make apply-sample

# View logs from controller and jobs
make dev-logs
```

**Development Cycle**:

```bash
# Make code changes...

# Rebuild, reload, and restart (preserves cluster)
make kind-redeploy

# Run tests
make test

# Cleanup
make kind-delete
```

📖 **Developer Guide**: See [docs/development/KIND_SETUP.md](docs/development/KIND_SETUP.md) for complete local development setup
📖 **Contributing**: See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow and testing

## Observability

Forge includes production-grade observability:

### Metrics (OpenTelemetry + Prometheus)

- Package operations (build/publish/deploy) with status and duration
- Job lifecycle metrics (created, completed, failed)
- Policy decisions (allowed/denied) with reason
- Controller health and performance

### Tracing (OpenTelemetry)

- Distributed traces for complete workflows
- Span per action (build, publish, deploy)
- Context propagation across operations

### Dashboards & Alerts

- Grafana dashboard for Forge operations
- Prometheus alerts for failures and policy violations
- OTel Collector for multi-backend export

## Security Model

### Defense in Depth

1. **Admission Webhook** - Validates resources before creation
2. **ServiceAccount Policies** - Fine-grained access control via annotations
3. **Network Policies** - Limits what jobs can access
4. **Pod Security** - Non-root, dropped capabilities, read-only filesystem
5. **Credential Management** - Secrets never in env vars

### What Users CAN'T Do

- Run arbitrary images (only Zarf CLI image)
- Execute arbitrary commands (only build/publish/deploy)
- Access unapproved repositories or registries
- Deploy to production without explicit policy grant
- Bypass policy enforcement

### What Users CAN Do

- Build packages from approved Git repos
- Publish to approved registries
- Deploy to approved clusters
- Use pre-built packages
- Self-service within policy boundaries

## Documentation

- **User Guide**: [USER_GUIDE.md](docs/getting-started/USER_GUIDE.md) - Complete usage examples
- **kubectl-forge Plugin**: [kubectl-forge README](cmd/kubectl-forge/README.md) - CLI tool for listing, downloading, and debugging Forge jobs
- **ServiceAccount Reference**: [SERVICEACCOUNT_REFERENCE.md](docs/development/SERVICEACCOUNT_REFERENCE.md) - Policy annotations
- **Namespace-Scoped**: [NAMESPACE_SCOPED_DEPLOYMENT.md](docs/operations/NAMESPACE_SCOPED_DEPLOYMENT.md) - Restricted deployment mode
- **Testing Guide**: [TESTING.md](docs/development/TESTING.md) - Unit, E2E, and integration testing
- **Runbook**: [RUNBOOK.md](docs/operations/RUNBOOK.md) - Operations and incident response
- **Troubleshooting**: [TROUBLESHOOTING.md](docs/operations/TROUBLESHOOTING.md) - Common issues and solutions
- **Production Checklist**: [PRODUCTION_CHECKLIST.md](docs/operations/PRODUCTION_CHECKLIST.md) - Production readiness tracking

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow.

### Testing

Forge includes comprehensive test coverage across all major components:

```bash
# Run all tests with coverage report
make test

# View HTML coverage report
make test-coverage

# Run YAML validation tests
make test-validation

# Run unit tests only (no integration)
make test-unit

# Run E2E tests (requires running cluster)
make e2e-test

# Run full integration test with Kind (creates cluster, deploys, tests, cleans up)
make integration-test

# Run integration test and keep cluster for debugging
make integration-test-keep

# Run integration test with Gitea registry (tests publish workflows)
make integration-test-registry

# Run registry integration test and keep cluster for debugging
make integration-test-registry-keep
```

**Current Coverage**: 48.3% overall (82%+ for critical packages)

**Test Suites:**

- **Controller Tests** (62.1%): Event handling, status updates, reconciliation
- **Action Handler Tests** (82.4%): Build, publish, and deploy operations
- **Policy Engine Tests** (84.0%): ServiceAccount policy validation and webhook enforcement
- **Source/Destination Tests** (92.5-100%): Git, S3, OCI, Local handlers
- **Credential Tests** (100%): Secret extraction and mounting
- **Telemetry Tests** (69.2%): Metrics, tracing, and OTel integration
- **E2E Tests**: Policy enforcement, multi-action workflows, status field population
- **Integration Tests (Kind)**: Full cluster deployment and workflow validation
- **Registry Integration Tests**: Publish workflows with Gitea OCI registry
- **YAML Validation**: All config files (CRDs, RBAC, samples) validated for correctness

### CI/CD

Forge includes comprehensive CI/CD pipelines for both GitHub Actions and GitLab CI.

**Pre-commit Hooks:**

```bash
# Install pre-commit
brew install pre-commit
pre-commit install

# Run all hooks
pre-commit run --all-files
```

**GitHub Actions:**

- **CI**: Lint, test, build, security scans on every push/PR
- **Pre-commit**: Runs all hooks on pull requests
- **Release**: Builds multi-arch binaries and Docker images on version tags

**GitLab CI:**

- Complete pipeline with lint, test, build, security, and release stages
- Multi-arch Docker image builds
- Coverage reporting and artifact management

See [.github/workflows/README.md](.github/workflows/README.md) for details.

### Project Structure

```text
forge/
├── pkg/
│   ├── apis/
│   │   ├── zarf/v1alpha1/       # ZarfPackageJob CRD types
│   │   └── uds/v1alpha2/        # UDSBundleJob CRD types
│   ├── controller/              # Main controller
│   ├── actions/
│   │   ├── common/              # Shared action code (~610 lines)
│   │   ├── zarf/                # Zarf action handlers
│   │   └── uds/                 # UDS action handlers
│   ├── sources/                 # Source handlers (Git, S3, OCI)
│   ├── destinations/            # Destination handlers
│   ├── credentials/             # Credential management
│   ├── policy/                  # Policy engine
│   ├── telemetry/               # OpenTelemetry integration
│   ├── leaderelection/          # HA leader election
│   └── webhook/                 # Admission webhook
├── chart/forge/                 # Helm chart for deployment
│   ├── templates/
│   │   ├── controller/          # Controller manifests
│   │   └── webhook/             # Webhook manifests
│   ├── crds/                    # CRD definitions
│   ├── dashboards/              # Grafana dashboards
│   └── values*.yaml             # Configuration options
├── examples/
│   └── samples/
│       ├── zarf/                # ZarfPackageJob examples
│       └── uds/                 # UDSBundleJob examples
├── docs/                        # Documentation
└── cmd/
    ├── controller/              # Controller entrypoint
    ├── webhook/                 # Webhook entrypoint
    └── kubectl-forge/           # kubectl plugin for Forge jobs
```

## Roadmap

See [PRODUCTION_CHECKLIST.md](docs/operations/PRODUCTION_CHECKLIST.md) for detailed progress tracking.

**Phase 1-3: Foundation** (Completed ✅)

- [x] API design and CRDs
- [x] Controller implementation
- [x] Policy engine and webhook
- [x] OpenTelemetry observability

**Phase 4: Production Hardening** (Completed ✅)

- [x] Leader election for HA
- [x] Network policies
- [x] Namespace-scoped deployment mode
- [x] Image security

**Phase 5-6: Testing & Documentation** (Completed ✅)

- [x] Controller unit tests (62.1% coverage)
- [x] Action handler tests (82.4% coverage)
- [x] Policy engine tests (84.0% coverage)
- [x] Source/destination tests (92.5-100% coverage)
- [x] Telemetry tests (69.2% coverage)
- [x] E2E test suite with policy enforcement
- [x] Kind-based integration test framework
- [x] YAML validation tests
- [x] Comprehensive documentation
- [x] Operational runbooks

**Phase 7: CI/CD Pipeline** (Completed ✅)

- [x] GitHub Actions (CI, pre-commit, release pipelines)
- [x] GitLab CI (complete pipeline with artifacts)
- [x] Automated builds on PR
- [x] Security scans (Trivy CVE, gosec)
- [x] Multi-arch builds (amd64, arm64)
- [x] Image signing support
- [x] Coverage reporting (Codecov)

**Current Status**: 103/127 items complete (81%)

**Note**: Phase 7 (Supply Chain Security & Attestation) added SLSA provenance, signing, and SBOM generation for Forge images. Package attestation framework implemented and ready for integration.

## Why "Forge"?

Because packages aren't "run" - they're **forged** through multiple operations (build, publish, deploy). Like a blacksmith's forge where raw materials become finished products through controlled, repeatable processes.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- **Issues**: [GitHub Issues](https://github.com/kylegalloway/forge/issues)
- **Docs**: Check the `docs/` directory
- **Questions**: Open a discussion

---

*Forge: Where Zarf packages are made, not run.*
