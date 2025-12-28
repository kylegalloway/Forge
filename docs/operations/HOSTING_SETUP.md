# Forge Hosting Setup Guide

This guide provides step-by-step instructions for setting up container image
hosting (GHCR) and Helm chart distribution for Forge.

## Current Status

### ✅ Already Automated

The repository already has comprehensive automation in place:

1. **Container Image Building & Publishing** (`.github/workflows/attest.yaml`)
   - Multi-arch builds (linux/amd64, linux/arm64)
   - Pushes to GHCR automatically on every push to main
   - Image names:
     - `ghcr.io/kylegalloway/forge/forge-controller`
     - `ghcr.io/kylegalloway/forge/forge-webhook`
   - Tagging strategy:
     - `latest` (on main branch)
     - `main` (on main branch)
     - `main-<sha>` (git commit SHA)
     - `v1.2.3` (on version tags)
     - `1.2` (on version tags)

2. **SLSA Build Provenance** (`.github/workflows/attest.yaml`)
   - Automatic SLSA provenance generation
   - **SLSA Build Level 3** compliance
   - Signed with GitHub OIDC (keyless Cosign)

3. **SBOM Generation** (`.github/workflows/attest.yaml`)
   - SPDX format
   - CycloneDX format
   - Attached to images as attestations

4. **Image Signing** (`.github/workflows/attest.yaml`)
   - Cosign signatures (keyless with GitHub OIDC)
   - Signatures stored in Rekor transparency log

5. **Vulnerability Scanning** (`.github/workflows/attest.yaml`)
   - Trivy scans on every build
   - Results uploaded to GitHub Security tab

### ⏸️ Needs Manual Setup

1. **Repository Settings**
   - Enable GitHub Container Registry package access
   - Configure package visibility (public/private)

2. **Helm Chart Distribution**
   - Create Helm chart release workflow
   - Set up GitHub Pages for chart repository

3. **Optional: Artifact Hub**
   - Submit chart to Artifact Hub for discoverability

## Manual Setup Instructions

### Step 1: Enable GHCR Package Access

The attest.yaml workflow uses `secrets.GITHUB_TOKEN` which automatically has
the necessary permissions. However, you need to ensure packages are accessible:

1. Go to `https://github.com/kylegalloway/Forge/settings/actions`
2. Under "Workflow permissions", ensure:
   - ✅ "Read and write permissions" is selected
   - ✅ "Allow GitHub Actions to create and approve pull requests" is checked (optional)

3. Verify package visibility:
   - After the next push to main, go to `https://github.com/kylegalloway?tab=packages`
   - Find the `forge/forge-controller` and `forge/forge-webhook` packages
   - Click on each package → "Package settings"
   - Under "Danger Zone", you can change visibility:
     - **Public**: Anyone can pull (recommended for open source)
     - **Private**: Only you and collaborators can pull

### Step 2: Verify Attest Workflow Success

The workflow should now succeed with the lowercase registry fix. Monitor the
next run:

```bash
# Watch the latest workflow run
gh run watch --repo kylegalloway/Forge

# Or check status manually
gh run list --repo kylegalloway/Forge --workflow=attest.yaml --limit 1
```

**Expected Success Indicators:**

- ✅ Multi-arch images built and pushed
- ✅ Images signed with Cosign
- ✅ SBOM attestations attached
- ✅ Trivy scans completed
- ✅ Results in GitHub Security tab

### Step 3: Pull and Verify Images

Once the workflow succeeds, verify you can pull the images:

```bash
# Pull controller image (latest)
docker pull ghcr.io/kylegalloway/forge/forge-controller:latest

# Pull webhook image
docker pull ghcr.io/kylegalloway/forge/forge-webhook:latest

# Verify signature (requires cosign)
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/Forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-controller:latest

# Download and view SBOM
cosign download sbom ghcr.io/kylegalloway/forge/forge-controller:latest | jq .
```

### Step 4: Create Helm Chart Release Workflow

Create `.github/workflows/helm-release.yaml`:

```yaml
name: Release Helm Chart

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Configure Git
        run: |
          git config user.name "$GITHUB_ACTOR"
          git config user.email "$GITHUB_ACTOR@users.noreply.github.com"

      - name: Install Helm
        uses: azure/setup-helm@v3
        with:
          version: 'latest'

      - name: Package Helm chart
        run: |
          # Extract version from tag (remove 'v' prefix)
          VERSION="${GITHUB_REF_NAME#v}"

          # Update Chart.yaml with version and appVersion
          sed -i "s/^version:.*/version: $VERSION/" chart/forge/Chart.yaml
          sed -i "s/^appVersion:.*/appVersion: $VERSION/" chart/forge/Chart.yaml

          # Package the chart
          helm package chart/forge --destination .helm-releases/

          # Create or update index
          BASE_URL="https://github.com/kylegalloway/Forge/releases"
          RELEASE_URL="${BASE_URL}/download/${GITHUB_REF_NAME}"

          if [ -f index.yaml ]; then
            helm repo index .helm-releases/ \
              --url "$RELEASE_URL" \
              --merge index.yaml
          else
            helm repo index .helm-releases/ --url "$RELEASE_URL"
          fi

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            .helm-releases/*.tgz
            .helm-releases/index.yaml
          generate_release_notes: true
          draft: false
          prerelease: false

      - name: Checkout gh-pages branch
        uses: actions/checkout@v4
        with:
          ref: gh-pages
          path: gh-pages

      - name: Update GitHub Pages
        run: |
          # Copy index to gh-pages
          mkdir -p gh-pages
          cp .helm-releases/index.yaml gh-pages/

          cd gh-pages
          git add index.yaml
          git commit -m "Update Helm chart index for $GITHUB_REF_NAME" || \
            echo "No changes to commit"
          git push origin gh-pages || echo "No changes to push"
```

### Step 5: Set Up GitHub Pages (One-Time)

1. Create the `gh-pages` branch:

   ```bash
   cd /Users/kylegalloway/src/forge

   # Create orphan branch
   git checkout --orphan gh-pages
   git reset --hard
   git commit --allow-empty -m "Initialize gh-pages"
   git push origin gh-pages

   # Return to main
   git checkout main
   ```

2. Enable GitHub Pages:
   - Go to `https://github.com/kylegalloway/Forge/settings/pages`
   - Source: Deploy from a branch
   - Branch: `gh-pages` / `/ (root)`
   - Click "Save"

3. Verify GitHub Pages is active:
   - Visit `https://kylegalloway.github.io/Forge/`
   - Should see a blank page or 404 (normal until first chart is published)

### Step 6: Create Your First Release

When you're ready to publish v0.1.0 (or any version):

```bash
cd /Users/kylegalloway/src/forge

# Tag the release
git tag -s v0.1.0 -m "Release v0.1.0

First stable release of Forge with:
- ZarfPackageJob and UDSBundleJob controllers
- Policy enforcement via ServiceAccount annotations
- SLSA provenance and SBOM generation
- Multi-arch container images"

# Push the tag (triggers helm-release.yaml and attest.yaml)
git push origin v0.1.0
```

### Step 7: Using the Published Helm Chart

After the release workflow completes, users can install Forge with:

```bash
# Add the Helm repository
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install Forge
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --wait

# Or with custom values
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  -f my-values.yaml \
  --wait
```

### Step 8: (Optional) Submit to Artifact Hub

For better discoverability, submit your chart to Artifact Hub:

1. Create `chart/forge/artifacthub-repo.yml`:

   ```yaml
   repositoryID: <your-repo-id>
   owners:
     - name: Kyle Galloway
       email: kyle@example.com
   ```

2. Visit <https://artifacthub.io/>
3. Sign in with GitHub
4. Click "Add Repository"
5. Fill in:
   - **Repository Type**: Helm charts
   - **Name**: Forge
   - **URL**: `https://kylegalloway.github.io/Forge`
   - **Description**: Kubernetes controller for declarative Zarf package operations
6. Submit

Artifact Hub will automatically sync your index.yaml every few hours.

## Verification Checklist

After completing setup, verify everything works:

- [ ] **GHCR Images Published**
  - `ghcr.io/kylegalloway/forge/forge-controller:latest` pulls successfully
  - `ghcr.io/kylegalloway/forge/forge-webhook:latest` pulls successfully

- [ ] **Images Signed**
  - `cosign verify` succeeds with GitHub OIDC identity
  - Signatures visible in Rekor transparency log

- [ ] **SBOM Attached**
  - `cosign download sbom` returns valid SPDX JSON
  - SBOM contains accurate dependency information

- [ ] **Vulnerability Scans**
  - Trivy results visible in GitHub Security tab
  - No critical vulnerabilities (or acknowledged if present)

- [ ] **Helm Chart Available**
  - `helm repo add forge https://kylegalloway.github.io/Forge` succeeds
  - `helm search repo forge` shows chart
  - `helm install` works in test cluster

- [ ] **GitHub Release Created**
  - Release page shows chart tarballs
  - Release notes auto-generated
  - Chart index.yaml accessible

## Troubleshooting

### Attest Workflow Fails with "parsing reference" Error

**Symptom**: `Error: signing [...]: parsing reference: could not parse reference`

**Cause**: Uppercase letters in repository name (OCI registries require lowercase)

**Fix**: Already applied in this commit - workflow now lowercases repository names

### Images Not Visible in Package Registry

**Symptom**: Workflow succeeds but packages don't appear

**Cause**: Package visibility restrictions or workflow permissions

**Fix**:

1. Check workflow permissions in repository settings
2. Visit `https://github.com/kylegalloway?tab=packages`
3. If package exists but is private, change visibility to public

### Helm Chart Not Found After Release

**Symptom**: `helm repo add` succeeds but `helm search` finds nothing

**Cause**: index.yaml not updated or GitHub Pages not serving correctly

**Fix**:

1. Verify `gh-pages` branch exists and has index.yaml
2. Check GitHub Pages is enabled and serving
3. Visit `https://kylegalloway.github.io/Forge/index.yaml` directly
4. Re-run helm-release workflow if needed

### Cosign Verification Fails

**Symptom**: `cosign verify` fails with certificate identity mismatch

**Cause**: Image built outside GitHub Actions or with different identity

**Fix**:

- Only verify images from official releases
- Check certificate identity matches: `https://github.com/kylegalloway/Forge/.github/workflows/attest.yaml@refs/heads/main`
- Use `cosign verify --certificate-identity-regexp` for flexibility

## Cost Summary

Current recommended setup costs: **$0/month**

- GitHub Actions: 2,000 minutes/month free (current usage ~5 min/push)
- GHCR: Unlimited storage and bandwidth for public packages
- GitHub Pages: Free for public repositories
- Rekor/Fulcio (Sigstore): Free public good infrastructure

**Scaling Considerations:**

- If you exceed 2,000 Actions minutes/month: ~$0.008/minute
- Private packages have different storage limits
- Enterprise features available for additional cost

## Next Steps

1. **Monitor First Workflow Run**: Watch the attest workflow with the lowercase fix
2. **Create First Release**: Tag v0.1.0 when ready (triggers Helm chart release)
3. **Test Installation**: Install Forge in a test cluster using Helm chart
4. **Update Documentation**: Add installation instructions to README.md
5. **Consider Artifact Hub**: Submit for better discoverability

## Additional Resources

- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview/)
- [Helm Chart Repository Guide](https://helm.sh/docs/topics/chart_repository/)
- [Artifact Hub](https://artifacthub.io/)
- [SLSA Framework](https://slsa.dev/)

---

**Last Updated:** 2025-12-17
