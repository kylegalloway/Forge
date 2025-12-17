# Hosting Plan for Forge

This document outlines the strategy for hosting Forge's Helm chart and container images.

## Container Image Hosting

### Recommended: GitHub Container Registry (GHCR)

**Rationale**: Free for public repositories, integrated with GitHub, supports OCI artifacts

**Setup Steps**:
1. Enable GitHub Actions in the repository
2. Create GitHub Personal Access Token (PAT) with `write:packages` scope
3. Add PAT as repository secret `GHCR_TOKEN`
4. Update `.github/workflows/release.yaml` to push to GHCR:
   ```yaml
   - name: Login to GHCR
     uses: docker/login-action@v3
     with:
       registry: ghcr.io
       username: ${{ github.actor }}
       password: ${{ secrets.GHCR_TOKEN }}

   - name: Build and push
     uses: docker/build-push-action@v5
     with:
       push: true
       tags: |
         ghcr.io/${{ github.repository }}:latest
         ghcr.io/${{ github.repository }}:${{ github.ref_name }}
   ```

**Image naming**: `ghcr.io/<org>/forge:<version>`

### Alternative: Docker Hub

If GHCR is not suitable, Docker Hub offers:
- Free tier for public images
- Widely used and trusted
- Good CDN performance

Setup requires Docker Hub account and `DOCKER_USERNAME`/`DOCKER_PASSWORD` secrets.

## Helm Chart Hosting

### Recommended: GitHub Pages + GitHub Releases

**Rationale**: Free, integrated with repository, automated with GitHub Actions

**Implementation**:

1. **Chart Packaging**: Create `.github/workflows/helm-release.yaml`:
   ```yaml
   name: Release Helm Chart

   on:
     push:
       tags:
         - 'v*'

   jobs:
     release:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4

         - name: Install Helm
           uses: azure/setup-helm@v3

         - name: Package chart
           run: |
             helm package chart/forge --version ${GITHUB_REF_NAME#v}

         - name: Create index
           run: |
             helm repo index . --url https://github.com/${{ github.repository }}/releases/download/${GITHUB_REF_NAME}

         - name: Upload to release
           uses: softprops/action-gh-release@v1
           with:
             files: |
               forge-*.tgz
               index.yaml
   ```

2. **Chart Repository**: Users add the repository:
   ```bash
   helm repo add forge https://raw.githubusercontent.com/<org>/forge/gh-pages
   helm repo update
   helm install forge forge/forge
   ```

### Alternative: Artifact Hub

Publish to [Artifact Hub](https://artifacthub.io/) for better discoverability:
- Submit repository to Artifact Hub
- Add `artifacthub-repo.yml` to chart directory
- Automatic indexing and UI

## Image Signing and Attestation

For production deployments, consider implementing:

1. **Cosign** for image signing:
   ```bash
   cosign sign ghcr.io/<org>/forge:v1.0.0
   ```

2. **SBOM generation** with Syft:
   ```bash
   syft ghcr.io/<org>/forge:v1.0.0 -o spdx-json
   ```

3. **Attestation publishing** using in-toto:
   ```bash
   cosign attest --predicate attestation.json ghcr.io/<org>/forge:v1.0.0
   ```

## Multi-Architecture Builds

Support multiple architectures in release workflow:
```yaml
- name: Set up QEMU
  uses: docker/setup-qemu-action@v3

- name: Build multi-arch images
  uses: docker/build-push-action@v5
  with:
    platforms: linux/amd64,linux/arm64
    push: true
    tags: ghcr.io/${{ github.repository }}:${{ github.ref_name }}
```

## Security Best Practices

1. **Scan images** with Trivy before publishing
2. **Use minimal base images** (currently using Alpine)
3. **Sign releases** with GPG keys
4. **Implement SBOMs** for supply chain transparency
5. **Version pinning** for all dependencies

## Bandwidth and Rate Limits

### GHCR Limits:
- No bandwidth charges for public images
- No rate limits for public repositories
- Good CDN distribution

### Docker Hub Limits (Free Tier):
- 200 image pulls per 6 hours (anonymous)
- Unlimited pulls for authenticated users
- 1 private repository

## Recommended Initial Setup

For a new open-source project, start with:
1. **GHCR** for container images (free, no limits)
2. **GitHub Releases** for Helm charts (simple, integrated)
3. **GitHub Actions** for CI/CD automation

This provides a complete, free hosting solution with good reliability and no vendor lock-in.

## Migration Path

If scaling beyond GitHub's offerings:
1. Consider **Artifact Registry** (GCP) or **ECR Public** (AWS)
2. Move Helm charts to **ChartMuseum** or **Harbor**
3. Implement CDN caching with **Cloudflare** for better global distribution

## Costs

Current recommended stack: **$0/month**
- GitHub Actions: 2,000 minutes/month free (sufficient for small projects)
- GHCR: Unlimited public image storage and pulls
- GitHub Pages: Free for public repos

Future scaling might require GitHub Actions paid tiers (~$4/month for additional minutes).
