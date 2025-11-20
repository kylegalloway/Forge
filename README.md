# Forge - Kubernetes-Native Zarf Package Operations

**Forge** where Zarf packages are built, published, and deployed with declarative ops and actual security.

> **Status**: Under active development. API subject to change. Not yet deployed anywhere.

## What is Forge?

Forge is a Kubernetes controller that brings Zarf package operations into the declarative Kubernetes world. Instead of running arbitrary scripts (security nightmare), Forge provides purpose-built operations with fine-grained RBAC controls.

### What it does:
- **Build** Zarf packages from Git repos, S3, or OCI registries
- **Publish** artifacts to S3 or OCI registries
- **Deploy** packages to in-cluster or external Kubernetes clusters
- **Enforce policies** on who can do what with which resources

### What it doesn't do:
- Run arbitrary scripts (use a CronJob for that)
- Give you root access disguised as "flexibility"
- Trust users by default

## Quick Example

```yaml
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
metadata:
  name: build-and-deploy
spec:
  # What to do
  action: BuildDeploy

  # Where to get it
  source:
    type: Git
    git:
      url: https://github.com/defenseunicorns/zarf
      ref: v0.32.0
      path: examples/big-bang

  # Where to deploy it
  deploy:
    target: InCluster
    namespace: bigbang
    timeout: 60m

  # Who can use this
  rbacPolicy:
    allowedUsers:
      - group:platform-team
    allowedActions:
      - BuildDeploy
    allowedDeployTargets:
      - InCluster
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
| `Local` | Dev/testing ONLY | None | Must set `devMode: true` |

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

## RBAC Policy Enforcement

Every `ZarfPackage` can define its own RBAC policy:

```yaml
spec:
  rbacPolicy:
    # Who can use this
    allowedUsers:
      - user:alice@example.com
      - group:developers

    # What actions they can take
    allowedActions:
      - Build
      - Publish

    # What sources they can use
    allowedSourceRepos:
      - github.com/myorg/*

    # Where they can publish
    allowedPublishRegistries:
      - ghcr.io/myorg/*

    # Where they can deploy (if allowed)
    allowedDeployTargets:
      - InCluster
```

**Policy Hierarchy:**
1. **Cluster-level policy** (platform team) - most restrictive
2. **Namespace-level policy** (admin-managed) - overrides resource policy
3. **Resource-level policy** (user self-service) - least restrictive

The admission webhook enforces all three levels.

## Installation

```bash
# Install CRD
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml

# Install Forge controller
kubectl apply -f config/manager/deployment.yaml
kubectl apply -f config/rbac/rbac.yaml

# Install admission webhook (for policy enforcement)
kubectl apply -f webhook/deploy/
```

## Observability

Forge includes production-grade observability:

### Metrics (OpenTelemetry + Prometheus)
- Package operations (build/publish/deploy) with status and duration
- Policy decisions (allowed/denied) with user and reason
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
2. **RBAC Policies** - Fine-grained access control
3. **Network Policies** - Limits what jobs can access
4. **Pod Security** - Non-root, dropped capabilities, read-only filesystem
5. **Credential Management** - Secrets never in env vars

### What Users CAN'T Do

- Run arbitrary images (only Zarf CLI image)
- Execute arbitrary commands (only build/publish/deploy)
- Access unapproved repositories or registries
- Deploy to production without explicit RBAC grant
- Bypass policy enforcement

### What Users CAN Do

- Build packages from approved Git repos
- Publish to approved registries
- Deploy to approved clusters
- Use pre-built packages
- Self-service within policy boundaries

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow.

### Project Structure

```
forge/
├── pkg/
│   ├── apis/zarf/v1alpha1/     # ZarfPackage CRD types
│   ├── controller/              # Main controller (TODO)
│   ├── actions/                 # Action handlers (TODO)
│   ├── sources/                 # Source handlers (TODO)
│   ├── policy/                  # Policy engine (TODO)
│   ├── telemetry/               # OpenTelemetry integration
│   └── webhook/                 # Admission webhook
├── config/
│   ├── crd/                     # CRD manifests
│   ├── manager/                 # Controller deployment
│   ├── rbac/                    # RBAC manifests
│   ├── samples/                 # Example ZarfPackages
│   ├── prometheus/              # Alerts
│   ├── grafana/                 # Dashboards
│   └── otel-collector/          # OTel Collector config
└── docs/
    ├── REFACTOR_PLAN.md         # Architectural refactor plan
    └── PRODUCTION_CHECKLIST.md  # Production readiness tracker
```

## Roadmap

See [REFACTOR_PLAN.md](docs/REFACTOR_PLAN.md) for detailed implementation plan.

**Phase 1: Core Operations** (Current)
- [x] API design (ZarfPackage CRD)
- [x] Sample manifests
- [x] CRD YAML with validation
- [ ] Controller implementation
- [ ] Build/Publish/Deploy action handlers
- [ ] Source handlers (Git, S3, OCI)

**Phase 2: Policy Enforcement**
- [ ] Policy evaluation engine
- [ ] Webhook policy checks
- [ ] User/group authorization
- [ ] Audit logging

**Phase 3: Production Ready**
- [ ] Comprehensive testing
- [ ] Documentation
- [ ] Migration tooling (if needed)
- [ ] Production deployment

## Why "Forge"?

Because packages aren't "run" - they're **forged** through multiple operations (build, publish, deploy). Like a blacksmith's forge where raw materials become finished products through controlled, repeatable processes.

Also, "ScriptRunner" sounded like a toy. Forge sounds like where serious ops get done.

## License

[Add license here]

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- **Issues**: [GitHub Issues](https://github.com/kylegalloway/forge/issues)
- **Docs**: Check the `docs/` directory
- **Questions**: Open a discussion

---

*Forge: Where Zarf packages are made, not run.*
