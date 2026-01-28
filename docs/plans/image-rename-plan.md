# Plan: Rename CLI Images and Align Versioning

## Overview

Rename the CLI container images from `zarf-cli`/`uds-cli` to `zarfpackagejob`/`udsbundlejob` and align their versioning with the controller/webhook version (currently v0.11.1) instead of the underlying tool versions.

## Changes Summary

| Current | New |
|---------|-----|
| `ghcr.io/.../zarf-cli:v0.69.0` | `ghcr.io/.../zarfpackagejob:v0.11.1` |
| `ghcr.io/.../uds-cli:v0.27.21` | `ghcr.io/.../udsbundlejob:v0.11.1` |

## IMPORTANT: What NOT to Change

The following references to `zarf-cli` and `uds-cli` are legitimate references to the actual tools and MUST be preserved:

1. **Dockerfile download URLs** - These download the actual CLI binaries:
   - `https://github.com/zarf-dev/zarf/releases/download/...`
   - `https://github.com/defenseunicorns/uds-cli/releases/download/...`

2. **Upstream project references**:
   - `ghcr.io/defenseunicorns/uds-cli` (official upstream image)
   - `ghcr.io/defenseunicorns/uds-cli/podinfo` (OCI artifacts)
   - `defenseunicorns/uds-cli` (GitHub repo references)
   - `zarf-dev/zarf` (GitHub repo references)

3. **Documentation about the tools themselves**:
   - Links to official releases
   - Explanations of what zarf/uds-cli tools do
   - Version ARGs in Dockerfiles (ZARF_VERSION, UDS_VERSION)

**Only change**: Our Forge image names (`ghcr.io/kylegalloway/forge/zarf-cli` and `ghcr.io/kylegalloway/forge/uds-cli`)

## Implementation Steps

### 1. Rename Image Directories

```bash
git mv images/zarf-cli images/zarfpackagejob
git mv images/uds-cli images/udsbundlejob
```

Update comments in Dockerfiles:
- `images/zarfpackagejob/Dockerfile` - line 4 comment
- `images/udsbundlejob/Dockerfile` - line 4 comment

### 2. Update Core Code

**`pkg/constants/config.go`** (line 49, 52):
```go
DefaultZarfCLIImage = "ghcr.io/kylegalloway/forge/zarfpackagejob:v0.11.1"
DefaultUDSCLIImage = "ghcr.io/kylegalloway/forge/udsbundlejob:v0.11.1"
```

### 3. Update Helm Chart

**`chart/forge/values.yaml`** (lines 165-176):
- Change `zarfCLI` section repository to `ghcr.io/kylegalloway/forge/zarfpackagejob`
- Change `udsCLI` section repository to `ghcr.io/kylegalloway/forge/udsbundlejob`
- Change both tags to `v0.11.1`

### 4. Update Zarf Package Config

**`zarf.yaml`** (lines 29-41):
- Rename component `zarf-cli` → `zarfpackagejob`
- Rename component `uds-cli` → `udsbundlejob`
- Update image references and tags to v0.11.1

### 5. Update Release Workflow

**`.github/workflows/release.yaml`**:

a) Update prepare job outputs (lines 33-36):
- `zarf_cli_repo` → `zarfpackagejob_repo`
- `uds_cli_repo` → `udsbundlejob_repo`
- Remove `zarf_version` and `uds_version` outputs

b) Update repo name generation (lines 46-47):
```yaml
echo "zarfpackagejob=ghcr.io/${{ github.repository }}/zarfpackagejob"
echo "udsbundlejob=ghcr.io/${{ github.repository }}/udsbundlejob"
```

c) Remove CLI version extraction step (lines 60-66)

d) Update build matrix (lines 176-189):
- Change `zarf-cli` → `zarfpackagejob`
- Change `uds-cli` → `udsbundlejob`
- Change contexts to `./images/zarfpackagejob` and `./images/udsbundlejob`
- Change `tags_type: cli` → `tags_type: semver` for both
- Remove `cli_version_key` entries

e) Update all subsequent references in:
- outputs (lines 191-198)
- repo-name step (lines 218-226)
- digests loading (lines 349-355, 522-528)
- sign-images matrix (lines 390-393)
- scan-images matrix (lines 390-393)
- release notes generation (lines 547-555, 639-646)

### 6. Update CI Workflow

**`.github/workflows/ci.yaml`** (lines 174-180):
- Change context to `./images/zarfpackagejob`
- Change file to `./images/zarfpackagejob/Dockerfile`
- Change tags to `zarfpackagejob:${{ github.sha }}`

### 7. Update Makefile

**`Makefile`** (lines 310-316):
- Rename target `kind-zarf-cli` → `kind-zarfpackagejob`
- Update build context path
- Update image tag references

### 8. Update Release Script

**`scripts/release.sh`**:
- Line 168-169: Update image names in commit message
- Lines 446-449: Update image names in summary output
- Add logic to update CLI image tags in `values.yaml` (like controller/webhook)

### 9. Rewrite update-cli-versions.sh

**`scripts/update-cli-versions.sh`** → **`scripts/update-tool-versions.sh`**:

The script's purpose changes from updating image tags to only updating the underlying tool versions in Dockerfiles. Rename and simplify:

**Keep** (fetching latest upstream tool versions):
- `get_latest_github_release "zarf-dev/zarf"` - Get latest Zarf version
- `get_latest_github_release "defenseunicorns/uds-cli"` - Get latest UDS version
- Update `images/zarfpackagejob/Dockerfile` ARG ZARF_VERSION
- Update `images/udsbundlejob/Dockerfile` ARG UDS_VERSION

**Remove** (no longer tracking tool versions for image tags):
- config.go updates (now uses release version)
- values.yaml updates (now uses release version)
- zarf.yaml updates (now uses release version)

### 10. Update Documentation Files

Update Forge image references from `zarf-cli`/`uds-cli` to `zarfpackagejob`/`udsbundlejob`.

**Pattern to change**: `ghcr.io/kylegalloway/forge/zarf-cli` and `ghcr.io/kylegalloway/forge/uds-cli`
**Pattern to preserve**: `ghcr.io/defenseunicorns/uds-cli`, `defenseunicorns/uds-cli`, tool download URLs

| File | Changes | Preserve |
|------|---------|----------|
| `README.md` | Forge image refs | - |
| `chart/README.md` | Helm image examples | - |
| `docs/getting-started/DEPLOYMENT.md` | Forge image list, Helm examples | - |
| `docs/getting-started/USER_GUIDE.md` | Forge image refs | - |
| `docs/operations/TROUBLESHOOTING.md` | Error examples, pull/build commands | - |
| `docs/development/RELEASING.md` | Forge image list | - |
| `docs/development/KIND_TESTING_PUBLIC_IMAGES.md` | Forge image refs, commands | - |
| `docs/development/KIND_SETUP.md` | Forge refs, make targets | - |
| `docs/development/GITEA_TESTING.md` | Make target ref | - |
| `docs/development/TOOL_VERSIONS.md` | Make target refs | `defenseunicorns/uds-cli` refs |
| `docs/development/ATTESTATION_VERIFICATION.md` | Forge image examples | - |
| `images/zarfpackagejob/README.md` | Forge image refs | Tool download URLs, `zarf-dev/zarf` |
| `images/udsbundlejob/README.md` | Forge image refs | Tool download URLs, `defenseunicorns/uds-cli` |
| `CHANGELOG.md` | "zarf-cli and uds-cli container images" → "zarfpackagejob and udsbundlejob container images" | - |

### 11. Update Test Files

**`tests/e2e/05-uds-deploy/test-kubeconfig-auth.yaml`** (line 14):
- Change `localhost/uds-cli:v0.27.21` → `localhost/udsbundlejob:v0.11.1`

## Verification

1. **Build test**: Run `go build ./...` to verify constants compile
2. **Helm lint**: Run `helm lint chart/forge`
3. **Docker build**: Test building images from new paths:
   ```bash
   docker build -t zarfpackagejob:test images/zarfpackagejob/
   docker build -t udsbundlejob:test images/udsbundlejob/
   ```
4. **Grep verification**: Confirm no remaining Forge image references to old names:
   ```bash
   # Should return NO results (our image refs should all be renamed)
   grep -r "kylegalloway/forge/zarf-cli" .
   grep -r "kylegalloway/forge/uds-cli" .

   # These are OK to still exist (tool references, not our images):
   # - defenseunicorns/uds-cli (upstream)
   # - zarf-dev/zarf (upstream)
   # - images/zarfpackagejob/Dockerfile ARG ZARF_VERSION
   # - images/udsbundlejob/Dockerfile ARG UDS_VERSION
   ```

## Files Modified (Complete List)

### Renamed
- `images/zarf-cli/` → `images/zarfpackagejob/`
- `images/uds-cli/` → `images/udsbundlejob/`
- `scripts/update-cli-versions.sh` → `scripts/update-tool-versions.sh`

### Code/Config
- `pkg/constants/config.go`
- `chart/forge/values.yaml`
- `zarf.yaml`
- `Makefile`

### Workflows
- `.github/workflows/release.yaml`
- `.github/workflows/ci.yaml`

### Scripts
- `scripts/release.sh`
- `scripts/update-tool-versions.sh` (rewritten)

### Documentation (14 files)
- `README.md`
- `chart/README.md`
- `CHANGELOG.md`
- `docs/getting-started/DEPLOYMENT.md`
- `docs/getting-started/USER_GUIDE.md`
- `docs/operations/TROUBLESHOOTING.md`
- `docs/development/RELEASING.md`
- `docs/development/KIND_TESTING_PUBLIC_IMAGES.md`
- `docs/development/KIND_SETUP.md`
- `docs/development/GITEA_TESTING.md`
- `docs/development/TOOL_VERSIONS.md`
- `docs/development/ATTESTATION_VERIFICATION.md`
- `images/zarfpackagejob/README.md`
- `images/udsbundlejob/README.md`

### Tests
- `tests/e2e/05-uds-deploy/test-kubeconfig-auth.yaml`
