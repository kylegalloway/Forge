# Release Process

This guide explains how to create new releases of Forge using the automated release script.

## Prerequisites

- Write access to the Forge repository
- GPG key configured for signing commits and tags
- Kubernetes tools: `helm`, `kubectl`
- On the `main` branch with no uncommitted changes

## Quick Release

The simplest way to release is using the Makefile targets:

```bash
# Patch release (0.0.X) - bug fixes, documentation updates
make release-patch

# Minor release (0.X.0) - new features, backward compatible
make release-minor

# Major release (X.0.0) - breaking changes
make release-major
```

Expected output:

```text
==> 🚀 Forge Release Automation

==> Current version: 0.1.2
==> New version (patch bump): 0.1.3

Proceed with release 0.1.3? [y/N]: y

==> Step 1: Updating Chart.yaml
✓ Chart.yaml updated
==> Step 2: Updating documentation
✓ Updated README.md
✓ Updated docs/getting-started/USER_GUIDE.md
✓ Updated docs/getting-started/KIND_TESTING_PUBLIC_IMAGES.md

==> Step 3: Creating commit
✓ Changes committed

==> Step 4: Creating tags
✓ Created tag v0.1.3
✓ Updated latest tag

==> Step 5: Pushing to origin
✓ Pushed main branch and tags

==> Step 6: Packaging Helm chart
✓ Chart packaged: forge-0.1.3.tgz

==> Step 7: Publishing to gh-pages
✓ Helm index updated
✓ Published to gh-pages

==> 🎉 Release Complete!

Version 0.1.3 has been released!
```

## Manual Release (Using Script Directly)

You can also call the script directly:

```bash
./scripts/release.sh patch
./scripts/release.sh minor
./scripts/release.sh major
```

## What the Script Does

The automated release script performs these steps:

### 1. **Validation**

- Verifies you're on the `main` branch
- Checks for uncommitted changes
- Confirms version bump type

### 2. **Version Calculation**

- Reads current version from `chart/forge/Chart.yaml`
- Calculates new version based on semver rules:
  - **Patch**: 0.1.2 → 0.1.3 (bug fixes, docs)
  - **Minor**: 0.1.2 → 0.2.0 (new features)
  - **Major**: 0.1.2 → 1.0.0 (breaking changes)

### 3. **File Updates**

Updates version references in:

- `chart/forge/Chart.yaml` (version and appVersion)
- `README.md`
- `docs/getting-started/USER_GUIDE.md`
- `docs/getting-started/KIND_TESTING_PUBLIC_IMAGES.md`
- `docs/getting-started/KIND_SETUP.md`

Replaces patterns:

- `version: X.Y.Z`
- `--version X.Y.Z`
- `vX.Y.Z`
- `:vX.Y.Z` (image tags)

### 4. **Git Commit**

Creates a signed commit with:

- Random cultural reference (Star Wars, LOTR, movies, etc.)
- Chaotic, funny tone following project conventions
- Emoji (📦 for version bumps)
- Detailed body explaining the changes

Example commit message:

```text
📦 Gandalf: You shall not pass... without a version bump 0.1.3

Bumped patch version from 0.1.2 to 0.1.3.
Chart.yaml updated with new version and appVersion. All documentation
references updated to reflect the new release. The prophecy foretold
this day would come, and here we are with a shiny new version number
ready to conquer the Kubernetes landscape.
```

### 5. **Git Tags**

- Creates signed tag: `v0.1.3`
- Updates `latest` tag (force push)
- Pushes both tags to origin

### 6. **Helm Chart Packaging**

- Packages chart with `helm package`
- Creates `forge-X.Y.Z.tgz`

### 7. **GitHub Pages Update**

- Switches to `gh-pages` branch
- Copies packaged chart
- Updates `index.yaml` with `helm repo index`
- Commits and pushes to gh-pages
- Returns to `main` branch

### 8. **Cleanup**

- Removes temporary files
- Restores any stashed changes
- Prints success summary

## Post-Release Steps

After the script completes:

### 1. **GitHub Actions**

The release tag triggers CI/CD workflows that:

- Build multi-arch container images
- Push to GHCR at:
  - `ghcr.io/kylegalloway/forge/forge-controller:vX.Y.Z`
  - `ghcr.io/kylegalloway/forge/forge-webhook:vX.Y.Z`
  - `ghcr.io/kylegalloway/forge/zarfpackagejob:vX.Y.Z`
  - `ghcr.io/kylegalloway/forge/udsbundlejob:vX.Y.Z`
- Update `:latest` tags

### 2. **Helm Repository**

Chart becomes immediately available:

```bash
helm repo update
helm search repo forge/forge
# Should show new version
```

### 3. **Verification**

Test the new release:

```bash
# Create test cluster
kind create cluster --name forge-test

# Install new version
helm install forge forge/forge --version X.Y.Z

# Verify
kubectl get pods -n forge-system
```

## Troubleshooting

### Script Fails: "Must be on main branch"

```bash
git checkout main
```

### Script Fails: "Uncommitted changes detected"

```bash
git status
git add .
git commit -S -m "Your message"
# Then retry release
```

### Script Fails: GPG Signing Error

Ensure GPG is configured:

```bash
git config --global user.signingkey YOUR_KEY_ID
git config --global commit.gpgsign true
```

### Need to Abort Mid-Release

If the script fails partway through:

```bash
# Check current state
git status
git log --oneline -3
git tag -l

# If commit was made but not pushed
git reset --soft HEAD~1  # Undo commit, keep changes
git restore --staged .   # Unstage files

# If tags were created locally but not pushed
git tag -d vX.Y.Z
git tag -d latest

# Clean up gh-pages if needed
git checkout gh-pages
git reset --hard origin/gh-pages
git checkout main
```

### Manual Rollback of Released Version

If you need to rollback a pushed release:

```bash
# Delete remote tags
git push origin :vX.Y.Z
git push origin :latest --force

# Restore previous latest tag
git tag -d latest
git tag -s latest -m "Latest stable release (vX.Y.Z-old)"
git push origin latest --force

# Revert gh-pages
git checkout gh-pages
git revert <commit-hash>
git push origin gh-pages
git checkout main

# Users will need to helm repo update
```

## Version Guidelines

### When to Bump Patch (0.0.X)

- Bug fixes
- Documentation updates
- Minor improvements
- Dependency updates
- Security patches (non-breaking)

### When to Bump Minor (0.X.0)

- New features (backward compatible)
- New CRD fields (optional)
- New configuration options
- Deprecation warnings (not removals)

### When to Bump Major (X.0.0)

- Breaking API changes
- Removed CRD fields
- Changed behavior (incompatible)
- Kubernetes version requirement changes
- Removed features

## Release Checklist

Before running the release script:

- [ ] All tests passing (`make test`)
- [ ] Pre-commit hooks passing (`pre-commit run --all-files`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if exists)
- [ ] No known critical bugs
- [ ] On `main` branch
- [ ] Branch is up to date with origin
- [ ] GPG key configured

After running the release script:

- [ ] Verify tags pushed: `git ls-remote --tags origin`
- [ ] Check GitHub Actions passed
- [ ] Verify images on GHCR
- [ ] Test helm install with new version
- [ ] Update release notes on GitHub (optional)
- [ ] Announce release (optional)

## Advanced Usage

### Dry Run (Not Implemented Yet)

Currently, the script requires confirmation before proceeding. To add a dry-run mode, modify the script to accept a `--dry-run` flag.

### Custom Commit Messages

To customize the commit message, edit `scripts/release.sh`:

- Modify the `get_cultural_reference()` function to add new references
- Edit `generate_commit_message()` to change the format

### Skip Confirmation

To skip the interactive confirmation (use in CI):

```bash
yes | ./scripts/release.sh patch
```

## Continuous Integration

The release script can be integrated into CI/CD:

```yaml
# Example GitHub Action (not included)
name: Release
on:
  workflow_dispatch:
    inputs:
      bump_type:
        description: 'Version bump type'
        required: true
        type: choice
        options:
          - patch
          - minor
          - major

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Import GPG key
        # ... GPG setup
      - name: Run release script
        run: yes | ./scripts/release.sh ${{ inputs.bump_type }}
```

## See Also

- [Git Conventions](./.claude/rules/git-conventions.md) - Commit message standards
- [Testing Guide](./TESTING.md) - Run tests before release
- [Hosting Setup](../operations/HOSTING_SETUP.md) - Container registry and Helm repository
