# Contributing to Forge

Thank you for your interest in contributing to Forge! This guide will help you set up a local development environment and understand the development workflow.

> **Note**: This guide is for contributors developing Forge itself. If you want to **use** Forge, see [docs/getting-started/USER_GUIDE.md](docs/getting-started/USER_GUIDE.md) for installation from the published Helm repository.

## Development Environment Setup

### Prerequisites

1. **Go** (1.25 or later)

   ```bash
   # macOS
   brew install go

   # Linux
   wget https://go.dev/dl/go1.25.5.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.25.5.linux-amd64.tar.gz
   ```

2. **Podman or Docker**

   ```bash
   # macOS - Podman
   brew install podman
   podman machine init
   podman machine start

   # Linux - Podman
   sudo apt-get install podman  # Debian/Ubuntu
   sudo dnf install podman      # Fedora/RHEL
   ```

3. **kubectl**

   ```bash
   brew install kubectl  # macOS
   # Or download from https://kubernetes.io/docs/tasks/tools/
   ```

4. **kind** (Kubernetes in Docker)

   ```bash
   brew install kind  # macOS
   # Or: curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
   ```

### Clone the Repository

```bash
git clone https://github.com/kylegalloway/forge.git
cd forge
```

### Install Git Hooks

Forge uses custom git hooks to enforce code quality and commit conventions:

```bash
# Install pre-commit framework (runs linters, formatters, security checks)
pip install pre-commit
pre-commit install

# Install custom hooks (commit message validation, signed commits)
cp scripts/hooks/* .git/hooks/
chmod +x .git/hooks/*
```

The hooks enforce:

| Hook | Purpose |
|------|---------|
| `pre-commit` | Runs linters, formatters, and security scanners via pre-commit framework |
| `prepare-commit-msg` | Requires signed commits (`git commit -S` or `git config commit.gpgSign true`) |
| `commit-msg` | Validates commit messages: emoji required, no boring prefixes, body required |

> **Note**: If you don't have GPG signing set up, see [GitHub's guide to signing commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).

## Local Testing with Kind

### Quick Start

```bash
# Create kind cluster, build, and deploy in one command
make kind-setup

# Verify deployment
kubectl get pods -n forge-system
kubectl get crd zarfpackagejobs.forge.dev
```

### Step-by-Step Setup

```bash
# 1. Create kind cluster
make kind-create

# 2. Build controller image
make container-build

# 3. Load image into kind
make kind-load

# 4. Deploy with Helm
make install

# 5. Check status
kubectl get pods -n forge-system
```

### Testing Your Changes

```bash
# Apply a sample ZarfPackageJob
kubectl apply -f examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml

# Check status
kubectl get zarfpackagejobs
kubectl get zpj  # shortname

# View logs
kubectl logs -n forge-system -l app=forge-controller -f
```

## Development Workflow

### Quick Iteration Cycle

```bash
# 1. Make code changes
vim pkg/controller/controller.go

# 2. Run tests (optional but recommended)
make test
make vet

# 3. Rebuild and redeploy
make kind-redeploy

# 4. Test with samples
kubectl apply -f examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml

# 5. Check logs and status
kubectl logs -n forge-system -l app=forge-controller -f
kubectl get zpj -w
```

## Modifying the CRD

### API Types

Edit [pkg/apis/zarf/v1alpha1/types.go](pkg/apis/zarf/v1alpha1/types.go) to modify the ZarfPackageJob schema.

Example: Adding a new field to `ZarfPackageJobSpec`:

```go
type ZarfPackageJobSpec struct {
    Action     Action         `json:"action"`
    Source     PackageSource  `json:"source"`
    Publish    *PublishConfig `json:"publish,omitempty"`
    Deploy     *DeployConfig  `json:"deploy,omitempty"`
    RBACPolicy *RBACPolicy    `json:"rbacPolicy,omitempty"`

    // Your new field
    Timeout string `json:"timeout,omitempty"`
}
```

### Update the CRD Manifest

Edit [config/crd/forge.dev_zarfpackagejobs.yaml](config/crd/forge.dev_zarfpackagejobs.yaml) to add validation:

```yaml
spec:
  properties:
    timeout:
      type: string
      description: Maximum duration for operations
      pattern: '^[0-9]+[smh]$'
```

### Redeploy with Helm

```bash
# Upgrade existing installation
make upgrade

# Or uninstall and reinstall
make uninstall
make install
```

## Modifying Controller Logic

The controller structure (once implemented) will follow this pattern:

```text
pkg/
├── controller/          # Main controller reconciliation loop
├── actions/            # Build, Publish, Deploy handlers
├── sources/            # Git, S3, OCI source handlers
├── policy/             # RBAC policy engine
└── telemetry/          # OpenTelemetry metrics/tracing
```

### Example: Adding a New Action Handler

```go
// pkg/actions/zarf/build.go
package zarf

func (h *BuildHandler) Execute(ctx context.Context, pkg *v1alpha1.ZarfPackageJob) error {
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(attribute.String("action", "build"))

    // Your build logic here

    return nil
}
```

## Testing Controller Changes

### Unit Tests

```bash
# Run all tests
make test

# Run tests for specific package
go test ./pkg/controller -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

```bash
# Apply test resource
kubectl apply -f examples/samples/zarf/01-git-to-oci/zarfpackagejob.yaml

# Wait for completion
kubectl wait --for=condition=BuildComplete --timeout=300s ZarfPackageJob/git-to-oci-example

# Check status
kubectl get zpj git-to-oci-example -o yaml

# Cleanup
kubectl delete zpj git-to-oci-example
```

## Code Style and Standards

### Formatting and Linting

```bash
# Format code
make fmt

# Run linter
make vet

# Both
make test
```

### Code Organization

- Controller logic: `pkg/controller/`
- Action handlers: `pkg/actions/`
- Source handlers: `pkg/sources/`
- API types: `pkg/apis/zarf/v1alpha1/`
- Kubernetes manifests: `config/`
- Sample resources: `config/samples/v1alpha1/`

### Comments

Document all exported functions and types using godoc format:

```go
// BuildPackage creates a Zarf package from the specified source.
// It returns the artifact location on success or an error if the build fails.
func BuildPackage(ctx context.Context, source PackageSource) (string, error) {
    // Implementation
}
```

### Error Handling

Always wrap errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to clone git repo %s: %w", url, err)
}
```

## Submitting Changes

### Before Submitting

1. Ensure git hooks are installed (see [Install Git Hooks](#install-git-hooks))

2. Run tests and linters (hooks run these automatically, but you can run manually)

   ```bash
   make test
   make fmt
   make vet
   ```

3. Test in kind cluster

   ```bash
   make kind-redeploy
   # Manual testing
   ```

4. Update documentation if needed

5. Commit with a signed, chaotic commit message
   - Must include an emoji in the title
   - Must have a body explaining the "why"
   - No boring prefixes like `fix:`, `feat:`, `chore:`
   - Cultural references encouraged (see `scripts/hooks/commit-msg` for details)

### Pull Request Process

1. Fork the repository
2. Create a feature branch

   ```bash
   git checkout -b feature/my-feature
   ```

3. Make your changes with clear commits
4. Push to your fork
5. Open a Pull Request with:
   - Clear description of changes
   - Testing performed
   - Related issues

## Troubleshooting

### Controller Won't Start

```bash
# Check logs
kubectl logs -n forge-system -l app=forge-controller

# Check CRD is installed
kubectl get crd zarfpackagejobs.forge.dev

# Check RBAC
kubectl describe clusterrole forge-controller
```

### Image Not Loading into Kind

```bash
# List images in kind
docker exec -it forge-dev-control-plane crictl images | grep forge

# Manually load
kind load docker-image forge-controller:latest --name forge-dev
```

### CRD Changes Not Taking Effect

```bash
# Reinstall CRD
kubectl delete crd zarfpackagejobs.forge.dev
make install-crd

# Verify version
kubectl get crd zarfpackagejobs.forge.dev -o yaml | grep -A 5 versions
```

## Resources

- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Writing Controllers](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/controllers.md)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Zarf Documentation](https://docs.zarf.dev/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues and PRs
- Review the `docs/` directory for detailed guides

Happy forging!
