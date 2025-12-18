# GitHub Actions Workflows

This directory contains GitHub Actions workflows for CI/CD automation.

## Workflows

### CI Pipeline (`ci.yaml`)

Runs on every push and pull request to main/develop branches.

**Jobs:**

- **lint**: Runs golangci-lint, go fmt, go vet
- **test**: Runs unit tests with race detection and coverage reporting
- **build**: Builds controller and webhook binaries
- **docker**: Builds Docker images for both components
- **security**: Runs Trivy and gosec security scans

**Artifacts:**

- Test coverage reports (uploaded to Codecov)
- Build binaries
- Security scan results (uploaded to GitHub Security tab)

### Pre-commit (`pre-commit.yaml`)

Runs pre-commit hooks on pull requests.

**Checks:**

- Go formatting
- YAML linting
- Markdown linting
- Shell script linting
- Security checks
- General file checks

### Release (`release.yaml`)

Triggered on version tags (v*) or manual workflow dispatch.

**Actions:**

- Runs full test suite
- Builds multi-arch binaries (Linux/Darwin, AMD64/ARM64) with reproducible builds
- Generates SBOMs (SPDX and CycloneDX formats)
- Builds and pushes multi-arch Docker images (amd64/arm64) to GHCR
- Signs images with Cosign (keyless OIDC)
- Attests SBOMs to images
- Scans images for vulnerabilities with Trivy
- Packages Helm chart with versioning
- Creates Helm repository index
- Publishes to GitHub Pages for Helm repository
- Creates GitHub release with:
  - Binaries for all platforms
  - SBOMs
  - Helm chart package
  - Comprehensive release notes with verification commands

**SLSA Level:** Build Level 3 (isolated, signed, non-falsifiable, hermetic)

### Attest (`attest.yaml`)

Triggered after successful CI runs on main branch or manual workflow dispatch.

**Purpose:** Build and attest development/main branch images for testing

**Actions:**

- Builds and pushes Docker images to GHCR (main branch builds)
- Generates SLSA provenance
- Creates SBOMs
- Signs images with Cosign
- Attests SBOMs to images
- Scans for vulnerabilities

**Note:** Version tag releases are handled exclusively by `release.yaml`

## Usage

### Running Locally

Install pre-commit:

```bash
brew install pre-commit
pre-commit install
```

Run all hooks:

```bash
pre-commit run --all-files
```

Run specific hook:

```bash
pre-commit run go-fmt --all-files
```

### Triggering Release

Create and push a version tag:

```bash
git tag -s v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

Or trigger manually via workflow dispatch:

```bash
gh workflow run release.yaml -f tag=v1.0.0
```

### Using the Helm Chart

After release, users can install via Helm repository:

```bash
# Add Helm repository (hosted on GitHub Pages)
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install
helm install forge forge/forge --version 1.0.0
```

Or install directly from GitHub release:

```bash
helm install forge https://github.com/<your-org>/forge/releases/download/v1.0.0/forge-1.0.0.tgz
```

### Verifying Releases

Verify image signatures:

```bash
cosign verify \
  --certificate-identity-regexp="https://github.com/<your-org>/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/<your-org>/forge-controller:v1.0.0
```

Download and verify SBOM:

```bash
cosign download sbom ghcr.io/<your-org>/forge-controller:v1.0.0 | jq
```

### Required Secrets

For full functionality, configure these GitHub repository secrets:

- **GITHUB_TOKEN**: Automatically provided by GitHub Actions (used for
  GHCR, releases, and Pages)
- **CODECOV_TOKEN**: (Optional) For Codecov upload

**Note:** No additional secrets are required for image signing. Cosign
uses GitHub's OIDC provider for keyless signing.

### Badge Integration

Add workflow badges to README:

```markdown
![CI](https://github.com/kylegalloway/forge/workflows/CI/badge.svg)
![Pre-commit](https://github.com/kylegalloway/forge/workflows/Pre-commit/badge.svg)
```

## Workflow Configuration

### Modifying Workflows

When updating workflows:

1. Test changes in a feature branch first
2. Use `act` for local testing (optional)
3. Review workflow logs in Actions tab
4. Monitor for rate limits or quota issues

### Adding New Jobs

To add a new job to CI:

```yaml
new-job:
  name: New Job
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Your step
      run: |
        your-commands-here
```

### Caching

Workflows use GitHub Actions cache for:

- Go modules cache
- Docker layer cache
- Pre-commit environments

This significantly speeds up builds.

## Troubleshooting

### Workflow Failures

**Go version mismatch:**

- Update `go-version` in all workflows to match go.mod

**Docker build fails:**

- Check Dockerfile syntax
- Verify base image availability
- Review Docker build logs

**Pre-commit hooks fail:**

- Run `pre-commit run --all-files` locally
- Fix reported issues
- Commit and push again

**Security scan failures:**

- Review Trivy/gosec reports in Security tab
- Update dependencies if needed
- Add exceptions only if false positives

### Performance

If workflows are slow:

- Check cache hit rates
- Review job parallelization
- Consider self-hosted runners for large repos

## Best Practices

1. **Keep workflows fast**: Parallelize independent jobs
2. **Use caching**: Cache dependencies and build artifacts
3. **Fail fast**: Set `fail-fast: true` in matrix builds
4. **Secure secrets**: Never log or expose secrets
5. **Version pins**: Use specific versions for actions (`@v4` not `@main`)
6. **Conditional execution**: Use `if:` conditions to skip unnecessary work
7. **Artifact retention**: Set appropriate expiry times for artifacts

## Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Workflow Syntax](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions)
- [Action Marketplace](https://github.com/marketplace?type=actions)
- [golangci-lint](https://golangci-lint.run/)
- [pre-commit](https://pre-commit.com/)
