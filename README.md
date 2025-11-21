# Forge - Kubernetes-Native Zarf Package Operations

**Forge** where Zarf packages are built, published, and deployed with declarative ops and actual security.

> **Status**: Under active development. API subject to change. Not yet deployed anywhere.

## What is Forge?

Forge is a Kubernetes controller that brings Zarf package operations into the declarative Kubernetes world. Instead of running arbitrary scripts (security nightmare), Forge provides purpose-built operations with fine-grained RBAC controls.

### What it does

- **Build** Zarf packages from Git repos, S3, or OCI registries
- **Publish** artifacts to S3 or OCI registries
- **Deploy** packages to in-cluster or external Kubernetes clusters
- **Enforce policies** on who can do what with which resources

### What it doesn't do

- Run arbitrary scripts (use a CronJob for that)
- Give you root access disguised as "flexibility"
- Trust users by default

## Quick Example

```yaml
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
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
      url: https://github.com/defenseunicorns/zarf
      ref: v0.66.0
      path: examples/big-bang

  # Where to deploy it
  deploy:
    target: InCluster
    namespace: bigbang
    timeout: 60m
```

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

Forge uses ServiceAccount annotations for fine-grained access control. See [SERVICEACCOUNT_REFERENCE.md](docs/SERVICEACCOUNT_REFERENCE.md) for complete reference.

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
    forge.zarf.dev/allowed-actions: "Build,Publish"

    # Which Git repos can be used
    forge.zarf.dev/allowed-source-repos: "https://github.com/myorg/*"

    # Where packages can be published
    forge.zarf.dev/allowed-publish-registries: "ghcr.io/myorg/dev/*"
```

The admission webhook validates all operations against these policies before creation.

## Installation

### Cluster-Wide Deployment (Default)

For platform teams managing multi-tenant environments with full cluster access:

```bash
# Install CRDs (requires cluster-admin)
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml
kubectl apply -f config/crd/uds.io_udsbundles.yaml

# Install Forge controller with ClusterRole
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/manager/deployment.yaml

# Install admission webhook (for policy enforcement)
kubectl apply -f webhook/deploy/
```

**Watches**: All namespaces
**Permissions**: Cluster-wide (ClusterRole)
**Use Case**: Platform teams, multi-tenant management

### Namespace-Scoped Deployment (Restricted)

For restricted environments where ClusterRole permissions aren't available:

```bash
# Install CRDs (requires cluster-admin - one-time setup)
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml
kubectl apply -f config/crd/uds.io_udsbundles.yaml

# Create namespace
kubectl create namespace forge-system

# Install Forge controller with Role (namespace-only)
kubectl apply -f config/namespace-scoped/rbac.yaml
kubectl apply -f config/namespace-scoped/deployment.yaml
```

**Watches**: Single namespace (forge-system)
**Permissions**: Namespace-only (Role)
**Use Case**: Restricted clusters, individual teams, compliance-heavy environments

**Important**: In namespace-scoped mode, all resources (ZarfPackages, ServiceAccounts, Secrets) must be created in the `forge-system` namespace.

📖 **Full Guide**: See [NAMESPACE_SCOPED_DEPLOYMENT.md](docs/NAMESPACE_SCOPED_DEPLOYMENT.md) for detailed instructions, migration paths, and multi-tenant patterns.

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

- **User Guide**: [USER_GUIDE.md](docs/USER_GUIDE.md) - Complete usage examples
- **ServiceAccount Reference**: [SERVICEACCOUNT_REFERENCE.md](docs/SERVICEACCOUNT_REFERENCE.md) - Policy annotations
- **Namespace-Scoped**: [NAMESPACE_SCOPED_DEPLOYMENT.md](docs/NAMESPACE_SCOPED_DEPLOYMENT.md) - Restricted deployment mode
- **Runbook**: [RUNBOOK.md](docs/RUNBOOK.md) - Operations and incident response
- **Troubleshooting**: [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) - Common issues and solutions
- **Production Checklist**: [PRODUCTION_CHECKLIST.md](docs/PRODUCTION_CHECKLIST.md) - Production readiness tracking

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow.

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
│   ├── apis/zarf/v1alpha1/     # ZarfPackage CRD types
│   ├── apis/uds/v1alpha1/      # UDSBundle CRD types
│   ├── controller/              # Main controller
│   ├── actions/                 # Action handlers
│   ├── sources/                 # Source handlers (Git, S3, OCI)
│   ├── destinations/            # Destination handlers
│   ├── credentials/             # Credential management
│   ├── policy/                  # Policy engine
│   ├── telemetry/               # OpenTelemetry integration
│   ├── leaderelection/          # HA leader election
│   └── webhook/                 # Admission webhook
├── config/
│   ├── crd/                     # CRD manifests
│   ├── manager/                 # Controller deployment (cluster-wide)
│   ├── rbac/                    # RBAC manifests (ClusterRole)
│   ├── namespace-scoped/        # Namespace-scoped deployment (Role)
│   ├── network/                 # Network policies
│   ├── samples/                 # Example ZarfPackages
│   ├── prometheus/              # Alerts
│   ├── grafana/                 # Dashboards
│   └── otel-collector/          # OTel Collector config
├── docs/                        # Documentation
└── cmd/                         # Entrypoints (controller, webhook)
```

## Roadmap

See [PRODUCTION_CHECKLIST.md](docs/PRODUCTION_CHECKLIST.md) for detailed progress tracking.

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

- [x] Unit and integration tests
- [x] Comprehensive documentation
- [x] Operational runbooks

**Phase 7-9: Launch** (Pending ⏸️)

- [ ] CI/CD pipeline
- [ ] Security audit
- [ ] Production deployment

**Current Status**: 93/115 items complete (81%)

## Why "Forge"?

Because packages aren't "run" - they're **forged** through multiple operations (build, publish, deploy). Like a blacksmith's forge where raw materials become finished products through controlled, repeatable processes.

Also, "ScriptRunner" sounded like a toy. Forge sounds like where serious ops get done.

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
