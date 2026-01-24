# CLI Flags Passthrough for ZarfPackageJob/UDSBundleJob

**Status:** Planning

## Problem

Users cannot pass arbitrary flags to the underlying `zarf` or `uds` CLI commands. Only a limited set of flags are exposed through the CRD spec.

### Currently Supported

| Action | Exposed Flags |
|--------|---------------|
| Zarf Build | `--set` (via `build.variables`) |
| Zarf Deploy | `--components`, `--set`, `--kubeconfig-context` |
| Zarf Publish | None |
| UDS Create | `--set` (via `create.variables`) |
| UDS Deploy | `--set` (via `deploy.variables`) |
| UDS Publish | None |

### Commonly Needed But Missing

| Flag | CLI | Use Case |
|------|-----|----------|
| `--flavor` | zarf create | Build specific flavor variant |
| `--architecture` | zarf create | Cross-architecture builds (arm64) |
| `--skip-sbom` | zarf create | Speed up builds when SBOM not needed |
| `--no-progress` | all | Cleaner logs in CI |
| `--adopt-existing-resources` | zarf deploy | Adopt pre-existing resources |
| `--skip-webhooks` | zarf deploy | Skip webhook validation |
| `--retries` | zarf deploy | Retry failed deployments |
| `--insecure` | uds deploy | Skip TLS verification |

## Proposed Solution

Add two mechanisms:

1. **Structured fields** for common, validated flags
2. **`extraArgs`** array for arbitrary flag passthrough

### API Changes

```go
// pkg/apis/zarf/v1alpha3/types.go

type BuildConfig struct {
    // ... existing fields ...

    // Flavor specifies which package flavor to build
    // +optional
    Flavor string `json:"flavor,omitempty"`

    // Architecture specifies target architecture (e.g., "arm64", "amd64")
    // +optional
    Architecture string `json:"architecture,omitempty"`

    // SkipSBOM disables SBOM generation for faster builds
    // +optional
    SkipSBOM bool `json:"skipSBOM,omitempty"`

    // ExtraArgs are additional CLI arguments passed to 'zarf package create'
    // Use for flags not explicitly supported in the API
    // Example: ["--max-package-size", "100"]
    // +optional
    ExtraArgs []string `json:"extraArgs,omitempty"`
}

type DeployConfig struct {
    // ... existing fields ...

    // AdoptExistingResources enables adoption of pre-existing resources
    // +optional
    AdoptExistingResources bool `json:"adoptExistingResources,omitempty"`

    // SkipWebhooks disables webhook validation during deploy
    // +optional
    SkipWebhooks bool `json:"skipWebhooks,omitempty"`

    // Retries specifies the number of retry attempts for failed deployments
    // +optional
    Retries *int `json:"retries,omitempty"`

    // ExtraArgs are additional CLI arguments passed to 'zarf package deploy'
    // +optional
    ExtraArgs []string `json:"extraArgs,omitempty"`
}

type PublishConfig struct {
    // ... existing fields ...

    // ExtraArgs are additional CLI arguments passed to 'zarf package publish'
    // +optional
    ExtraArgs []string `json:"extraArgs,omitempty"`
}
```

Similar changes for UDS types.

### Handler Changes

```go
// pkg/actions/zarf/build.go

func (handler *BuildHandler) buildZarfCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPVCName string) (string, string) {
    // ... existing code ...

    if pkg.Spec.Build != nil {
        build := pkg.Spec.Build

        // Structured flags
        if build.Flavor != "" {
            cmd = fmt.Sprintf("%s --flavor %s", cmd, build.Flavor)
        }
        if build.Architecture != "" {
            cmd = fmt.Sprintf("%s --architecture %s", cmd, build.Architecture)
        }
        if build.SkipSBOM {
            cmd = fmt.Sprintf("%s --skip-sbom", cmd)
        }

        // Variables (existing)
        for key, value := range build.Variables {
            cmd = fmt.Sprintf("%s --set %s=%s", cmd, key, value)
        }

        // Extra args (escape/validate as needed)
        for _, arg := range build.ExtraArgs {
            cmd = fmt.Sprintf("%s %s", cmd, shellescape(arg))
        }
    }

    return cmd, workingDir
}
```

### Security Considerations

The `extraArgs` field needs validation to prevent command injection:

```go
// pkg/actions/common/validation.go

// ValidateExtraArgs ensures extra args don't contain shell metacharacters
func ValidateExtraArgs(args []string) error {
    forbidden := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\n"}
    for _, arg := range args {
        for _, char := range forbidden {
            if strings.Contains(arg, char) {
                return fmt.Errorf("extraArgs contains forbidden character %q: %s", char, arg)
            }
        }
    }
    return nil
}
```

Also validate in the webhook:

```go
// pkg/webhook/zarfpackage_validator.go

func (v *ZarfPackageJobValidator) validateExtraArgs(spec *zarfv1alpha3.ZarfPackageJobSpec) error {
    if spec.Build != nil {
        if err := common.ValidateExtraArgs(spec.Build.ExtraArgs); err != nil {
            return fmt.Errorf("build.extraArgs: %w", err)
        }
    }
    // ... similar for deploy, publish
}
```

## Implementation Checklist

### Phase 1: Structured Flags (Common Use Cases)

- [ ] Add `flavor`, `architecture`, `skipSBOM` to `BuildConfig`
- [ ] Add `adoptExistingResources`, `skipWebhooks`, `retries` to `DeployConfig`
- [ ] Update build.go to use new fields
- [ ] Update deploy.go to use new fields
- [ ] Regenerate CRDs
- [ ] Add tests

### Phase 2: ExtraArgs (Escape Hatch)

- [ ] Add `extraArgs []string` to all config types (Build, Deploy, Publish, Create)
- [ ] Add `ValidateExtraArgs()` function with shell injection protection
- [ ] Add webhook validation for extraArgs
- [ ] Update all handlers to append extraArgs
- [ ] Add tests for validation and command building
- [ ] Document security implications

### Phase 3: UDS Parity

- [ ] Add equivalent fields to UDS CreateConfig
- [ ] Add equivalent fields to UDS DeployConfig
- [ ] Add equivalent fields to UDS PublishConfig
- [ ] Update UDS handlers
- [ ] Regenerate CRDs

### Phase 4: Documentation

- [ ] Update CRD documentation
- [ ] Add examples showing common flag usage
- [ ] Document which flags are validated vs passthrough
- [ ] Security documentation for extraArgs

## Example Usage

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: multi-arch-build
spec:
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package
      ref: main
  build:
    flavor: "slim"
    architecture: "arm64"
    skipSBOM: true
    variables:
      VERSION: "1.0.0"
    extraArgs:
      - "--max-package-size"
      - "100"
  publish:
    destination:
      type: OCI
      oci:
        registry: ghcr.io/example/packages
```

## Alternatives Considered

### 1. Only ExtraArgs (No Structured Fields)

**Pros:** Simpler API, maximum flexibility
**Cons:** No validation, no discoverability, easy to make mistakes

### 2. Only Structured Fields (No ExtraArgs)

**Pros:** Full validation, better UX
**Cons:** Requires API changes for every new flag, can't handle edge cases

### 3. Hybrid Approach (Recommended)

**Pros:** Best of both - common cases are validated, edge cases supported
**Cons:** More API surface, need to maintain both

## Decision

Implement the **hybrid approach**:
1. Structured fields for common, well-understood flags
2. `extraArgs` as an escape hatch with shell injection validation
