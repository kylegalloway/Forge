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

Triggered on version tags (v*).

**Actions:**
- Builds multi-arch binaries (Linux/Darwin, AMD64/ARM64)
- Builds and pushes Docker images to GHCR
- Creates GitHub release with binaries attached
- Generates release notes automatically

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
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### Required Secrets

For full functionality, configure these GitHub repository secrets:

- **GITHUB_TOKEN**: Automatically provided by GitHub Actions
- **CODECOV_TOKEN**: (Optional) For Codecov upload

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
