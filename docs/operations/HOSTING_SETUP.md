# Forge Hosting Setup Guide

This guide documents the current hosting setup for Forge's container images and Helm charts.

## Current Status: ✅ Fully Automated

**Good news:** Everything is already set up and working! The repository has complete automation for building, publishing, signing, and distributing Forge.

### What's Already Working

#### 1. Container Image Publishing

**Workflow**: `.github/workflows/release.yaml` and `.github/workflows/attest.yaml`

**Images Published**:
- `ghcr.io/kylegalloway/forge/forge-controller`
- `ghcr.io/kylegalloway/forge/forge-webhook`

**Platforms**:
- `linux/amd64`
- `linux/arm64`

**Tagging Strategy**:
- On version tags (e.g., `v0.4.3`):
  - `0.4.3` (semver full version)
  - `0.4` (semver major.minor)
  - `0` (semver major)
  - `sha-<commit>` (git commit)
- On main branch pushes:
  - `latest`
  - `main`
  - `main-<sha>`

#### 2. Security Attestations

**SLSA Build Level 3**:
- ✅ Isolated builds in GitHub Actions
- ✅ Signed provenance attached to images
- ✅ Non-falsifiable (GitHub OIDC tokens)
- ✅ Hermetic builds (reproducible)

**Image Signing**:
- ✅ Cosign signatures (keyless with GitHub OIDC)
- ✅ Signatures stored in Rekor transparency log
- ✅ Verifiable with standard Sigstore tools

**SBOM Generation**:
- ✅ SPDX format (attached as attestations)
- ✅ CycloneDX format (in release artifacts)
- ✅ Generated with Syft

**Vulnerability Scanning**:
- ✅ Trivy scans on every build
- ✅ Results uploaded to GitHub Security tab
- ✅ SARIF format for integration

#### 3. Helm Chart Distribution

**GitHub Pages**: `https://kylegalloway.github.io/Forge/`
- ✅ Helm repository index published
- ✅ Chart tarballs hosted
- ✅ Automatic updates on releases

**GitHub Releases**:
- ✅ Chart tarball attached to each release
- ✅ Binaries for multiple platforms
- ✅ SBOMs attached
- ✅ Auto-generated release notes

**Latest Release**: v0.4.3

## Using the Published Artifacts

### Install Forge via Helm

```bash
# Add the Helm repository
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install Forge
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --version 0.4.3

# Or install latest version
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace
```

### Pull Container Images

```bash
# Pull controller image
docker pull ghcr.io/kylegalloway/forge/forge-controller:0.4.3

# Pull webhook image
docker pull ghcr.io/kylegalloway/forge/forge-webhook:0.4.3

# Pull latest
docker pull ghcr.io/kylegalloway/forge/forge-controller:latest
```

### Verify Image Signatures

```bash
# Install cosign (if not already installed)
# macOS: brew install cosign
# Linux: See https://docs.sigstore.dev/cosign/installation/

# Verify controller image signature
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-controller:0.4.3

# Verify webhook image signature
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-webhook:0.4.3
```

### Download and Inspect SBOM

```bash
# Download SBOM for controller
cosign download sbom ghcr.io/kylegalloway/forge/forge-controller:0.4.3 > controller-sbom.json

# View SBOM with jq
cat controller-sbom.json | jq .

# Verify SBOM attestation
cosign verify-attestation \
  --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --type spdx \
  ghcr.io/kylegalloway/forge/forge-controller:0.4.3
```

## How Releases Work

### Automated Release Process

1. **Tag a Release**:
   ```bash
   git tag -s v0.6.0 -m "Release v0.6.0"
   git push origin v0.6.0
   ```

2. **Workflow Triggers**:
   - `.github/workflows/release.yaml` is triggered
   - Runs tests
   - Builds multi-arch binaries
   - Generates SBOMs
   - Builds and pushes container images
   - Signs images with Cosign
   - Attaches SBOM attestations
   - Scans images with Trivy
   - Packages Helm chart
   - Creates GitHub release
   - Updates GitHub Pages

3. **Artifacts Published**:
   - Container images pushed to GHCR
   - Helm chart pushed to GitHub Pages
   - Release created with:
     - Binaries (Linux, macOS, multiple architectures)
     - SBOMs (SPDX and CycloneDX)
     - Helm chart tarball
     - Helm repository index
     - Auto-generated release notes

4. **Users Can**:
   - Pull images from GHCR
   - Install Helm chart from GitHub Pages
   - Download binaries from GitHub Releases
   - Verify signatures with Cosign
   - Inspect SBOMs

### Main Branch Builds

On every push to `main` branch:
- `.github/workflows/attest.yaml` runs
- Builds and pushes images with tags:
  - `latest`
  - `main`
  - `main-<sha>`
- Signs images
- Attaches attestations
- Scans for vulnerabilities

## Repository Configuration

### Required Settings

All necessary settings are already configured:

✅ **GitHub Actions Permissions**:
- Repository Settings → Actions → General → Workflow permissions
- "Read and write permissions" enabled
- "Allow GitHub Actions to create and approve pull requests" enabled

✅ **GitHub Pages**:
- Repository Settings → Pages
- Source: Deploy from branch `gh-pages`
- Serving at: `https://kylegalloway.github.io/Forge/`

✅ **Package Visibility**:
- Packages are public (recommended for open source)
- Accessible at: `https://github.com/kylegalloway?tab=packages`

### Secrets

No custom secrets required! Everything uses the built-in `GITHUB_TOKEN` with OIDC for keyless signing.

## Optional: Artifact Hub Submission

The only thing NOT automated is Artifact Hub submission (optional, improves discoverability).

### Submit to Artifact Hub

1. Visit [Artifact Hub](https://artifacthub.io/)
2. Sign in with GitHub
3. Click "Add Repository"
4. Fill in:
   - **Repository Type**: Helm charts
   - **Name**: Forge
   - **URL**: `https://kylegalloway.github.io/Forge`
   - **Description**: Kubernetes controller for declarative Zarf package and UDS bundle operations
5. Submit

Artifact Hub will automatically sync your `index.yaml` every few hours.

### Add Artifact Hub Metadata (Optional)

Create `chart/forge/artifacthub-repo.yml`:

```yaml
repositoryID: <your-repo-id-from-artifact-hub>
owners:
  - name: Kyle Galloway
    email: kyle@example.com
```

This provides additional metadata for Artifact Hub listings.

## Verification Checklist

Use this checklist to verify everything is working after a release:

### After Creating a Release (e.g., v0.6.0)

- [ ] **Workflow Succeeded**
  ```bash
  gh run list --repo kylegalloway/Forge --workflow=release.yaml --limit 1
  ```

- [ ] **Images Published to GHCR**
  ```bash
  docker pull ghcr.io/kylegalloway/forge/forge-controller:0.6.0
  docker pull ghcr.io/kylegalloway/forge/forge-webhook:0.6.0
  ```

- [ ] **Images Signed**
  ```bash
  cosign verify \
    --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
    --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
    ghcr.io/kylegalloway/forge/forge-controller:0.6.0
  ```

- [ ] **SBOM Attached**
  ```bash
  cosign download sbom ghcr.io/kylegalloway/forge/forge-controller:0.6.0 | jq .
  ```

- [ ] **GitHub Release Created**
  ```bash
  gh release view v0.6.0 --repo kylegalloway/Forge
  ```

- [ ] **Helm Chart Available**
  ```bash
  helm repo add forge https://kylegalloway.github.io/Forge
  helm repo update
  helm search repo forge --versions | grep 0.6.0
  ```

- [ ] **Helm Chart Installable**
  ```bash
  # In a test cluster
  helm install forge-test forge/forge \
    --version 0.6.0 \
    --namespace forge-test \
    --create-namespace \
    --dry-run
  ```

- [ ] **Security Scans Completed**
  - Check GitHub Security tab for Trivy results
  - Review any vulnerabilities found

## Troubleshooting

### Workflow Failures

**Check workflow logs**:
```bash
# List recent workflow runs
gh run list --repo kylegalloway/Forge --workflow=release.yaml --limit 5

# View specific run
gh run view <run-id> --repo kylegalloway/Forge --log

# Watch live
gh run watch --repo kylegalloway/Forge
```

### Image Pull Failures

**Issue**: `Error response from daemon: manifest unknown`

**Cause**: Image tag doesn't exist or package is private

**Fix**:
1. Verify tag exists: `gh release list --repo kylegalloway/Forge`
2. Check package visibility at `https://github.com/kylegalloway?tab=packages`
3. If private, authenticate: `docker login ghcr.io -u kylegalloway`

### Cosign Verification Failures

**Issue**: `Error: no matching signatures`

**Causes**:
- Image not signed (only release tags are signed)
- Using wrong certificate identity
- Image built outside GitHub Actions

**Fix**:
```bash
# Verify you're checking a released version
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-controller:0.4.3  # Use specific version tag
```

### Helm Chart Not Found

**Issue**: `helm search repo forge` returns nothing

**Causes**:
- Helm repository not added
- Repository cache stale
- Chart not published yet

**Fix**:
```bash
# Add repository (if not already added)
helm repo add forge https://kylegalloway.github.io/Forge

# Force update
helm repo update

# Verify index is accessible
curl -sL https://kylegalloway.github.io/Forge/index.yaml | head -20

# Search for chart
helm search repo forge --versions
```

### gh-pages Update Failures

**Issue**: Chart not appearing on GitHub Pages after release

**Causes**:
- gh-pages branch push failed
- GitHub Pages not enabled
- Cache not refreshed

**Fix**:
```bash
# Check gh-pages branch
git fetch origin gh-pages
git log origin/gh-pages --oneline -5

# Verify index.yaml in gh-pages
git show gh-pages:index.yaml | head -20

# Check GitHub Pages settings
# Go to: https://github.com/kylegalloway/Forge/settings/pages

# Manually trigger Pages rebuild (visit any gh-pages file and edit/save)
```

## Monitoring and Maintenance

### Regular Checks

**Weekly**:
- [ ] Check GitHub Security tab for new vulnerabilities
- [ ] Review Trivy scan results for latest images
- [ ] Verify Helm charts are installable

**Monthly**:
- [ ] Review dependency updates (Dependabot PRs)
- [ ] Check Cosign signatures are still valid
- [ ] Audit SBOM contents for accuracy

**Per Release**:
- [ ] Run verification checklist
- [ ] Test Helm chart installation in clean cluster
- [ ] Verify image signatures
- [ ] Review release notes

### Updating Dependencies

When updating Go dependencies, controller code, or Dockerfile:

1. **Update dependencies**:
   ```bash
   go get -u ./...
   go mod tidy
   ```

2. **Run tests**:
   ```bash
   make test
   ```

3. **Commit and push to main**:
   - Workflow builds and publishes `latest` tag
   - Scans for vulnerabilities
   - Signs images

4. **Create release when ready**:
   ```bash
   git tag -s v0.6.0 -m "Release v0.6.0"
   git push origin v0.6.0
   ```

## Cost Summary

**Current hosting costs: $0/month**

- ✅ **GitHub Actions**: 2,000 minutes/month free (current usage ~10-15 min/release)
- ✅ **GHCR**: Unlimited storage and bandwidth for public packages
- ✅ **GitHub Pages**: Free for public repositories
- ✅ **Rekor/Fulcio (Sigstore)**: Free public good infrastructure
- ✅ **GitHub Releases**: Free for public repositories

**Scaling Considerations**:
- If you exceed 2,000 Actions minutes/month: ~$0.008/minute
- Private packages have storage limits (500 MB free, then $0.008/GB/day)
- Enterprise features available at additional cost

## Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────┐
│                         GitHub Repository                        │
│                    github.com/kylegalloway/Forge                │
└────────────┬─────────────────────────────────┬──────────────────┘
             │                                 │
    ┌────────▼────────┐               ┌────────▼────────┐
    │   Push to main  │               │  Push tag (v*)  │
    │  Trigger: attest│               │ Trigger: release│
    └────────┬────────┘               └────────┬────────┘
             │                                 │
    ┌────────▼────────┐               ┌────────▼────────────────┐
    │ attest.yaml     │               │ release.yaml            │
    │ • Build images  │               │ • Run tests             │
    │ • Tag: latest   │               │ • Build binaries        │
    │ • Sign & attest │               │ • Build images          │
    │ • Scan (Trivy)  │               │ • Sign & attest         │
    └────────┬────────┘               │ • Scan (Trivy)          │
             │                        │ • Package Helm chart    │
             ├────────────────────────┤ • Create GitHub Release │
             │                        │ • Update gh-pages       │
             │                        └────────┬────────────────┘
             │                                 │
    ┌────────▼─────────────────────────────────▼────────┐
    │         GitHub Container Registry (GHCR)          │
    │   • ghcr.io/kylegalloway/forge/forge-controller  │
    │   • ghcr.io/kylegalloway/forge/forge-webhook     │
    │   • Multi-arch (amd64, arm64)                     │
    │   • Signed with Cosign (OIDC)                     │
    │   • SBOM + SLSA attestations attached             │
    └───────────────────────────────────────────────────┘
                              │
                   ┌──────────┴──────────┐
                   │                     │
          ┌────────▼────────┐   ┌────────▼────────┐
          │  GitHub Pages   │   │ GitHub Releases │
          │  (gh-pages)     │   │ • Binaries      │
          │  • index.yaml   │   │ • SBOMs         │
          │  • Chart .tgz   │   │ • Chart .tgz    │
          └────────┬────────┘   └─────────────────┘
                   │
          ┌────────▼────────┐
          │  Helm Repository│
          │  kylegalloway.  │
          │  github.io/     │
          │  Forge          │
          └─────────────────┘
```

## Additional Resources

- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview/)
- [SLSA Framework](https://slsa.dev/)
- [Helm Chart Repository Guide](https://helm.sh/docs/topics/chart_repository/)
- [Artifact Hub](https://artifacthub.io/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)

---

**Last Updated:** 2026-01-04
