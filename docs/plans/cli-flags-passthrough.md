# CLI Flags Passthrough for ZarfPackageJob/UDSBundleJob

**Status:** Complete

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

- [x] Add `flavor`, `architecture`, `skipSBOM` to `BuildConfig`
- [x] Add `adoptExistingResources`, `skipWebhooks`, `retries` to `DeployConfig`
- [x] Update build.go to use new fields
- [x] Update deploy.go to use new fields
- [x] Regenerate CRDs
- [x] Add tests

### Phase 2: ExtraArgs (Escape Hatch)

- [x] Add `extraArgs []string` to all config types (Build, Deploy, Publish, Create)
- [x] Add `ValidateExtraArgs()` function with shell injection protection
- [x] Add webhook validation for extraArgs
- [x] Update all handlers to append extraArgs
- [x] Add tests for validation and command building
- [x] Document security implications (in code comments)

### Phase 3: UDS Parity

- [x] Add equivalent fields to UDS CreateConfig
- [x] Add equivalent fields to UDS DeployConfig
- [x] Add equivalent fields to UDS PublishConfig
- [x] Update UDS handlers
- [x] Regenerate CRDs

### Phase 4: Documentation

- [x] CRD fields are self-documenting with godoc comments
- [x] Example in plan file shows common flag usage
- [x] Structured flags are validated, extraArgs are validated for injection only
- [x] Security validation documented in pkg/actions/common/validation.go

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
