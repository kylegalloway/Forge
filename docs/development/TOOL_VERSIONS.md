# Tool Version Tracking

This document tracks all tools, dependencies, and their versions used in the Forge project.

**Last Updated:** 2025-12-26

---

## Language Runtime

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|-----------------|-------|
| Go | 1.24.0 | 1.25.5 | 2027-02 (approx) | 🟡 Medium | Go 1.24.11 available with security fixes. Go 1.25.5 is latest major. Both are supported. |

---

## Build Tools

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|-----------------|-------|
| golangci-lint (CI) | v2.7.2 | v2.7.2 | Active | ✅ Up to date | Latest version |
| golangci-lint (pre-commit) | v1.64.8 | v2.7.2 | Deprecated | 🔴 High | Update to v2.x in pre-commit config |
| controller-gen | v0.20.0 | v0.20.0 | Active | ✅ Up to date | Latest version, supports k8s v0.34 |
| gofmt | (bundled with Go) | (bundled with Go) | - | - | Follows Go version |
| goimports | (installed via go install) | Latest | - | 🟢 Low | Auto-updated |

---

## Kubernetes Tools

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|-----------------|-------|
| k8s.io/api | v0.28.0 | v0.35.x | 2024-10 (v0.28) | 🔴 High | Several versions behind, security risk |
| k8s.io/client-go | v0.28.0 | v0.35.x | 2024-10 (v0.28) | 🔴 High | Kubernetes v1.28 EOL, must upgrade |
| k8s.io/apimachinery | v0.28.0 | v0.35.x | 2024-10 (v0.28) | 🔴 High | Must match client-go version |
| kubectl | (user-installed) | v1.35.x | - | 🟢 Low | User responsibility |
| helm | (user-installed) | Latest | - | 🟢 Low | User responsibility |
| kind | (user-installed) | Latest | - | 🟢 Low | User responsibility |

**Note:** Kubernetes v1.28 reached end-of-life in October 2024. Upgrading to v1.35 (released Dec 2025) is critical.

---

## Observability & Telemetry

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|-----------------|-------|
| go.opentelemetry.io/otel | v1.38.0 | v1.38.0 | Active | ✅ Up to date | Latest stable release |
| go.opentelemetry.io/otel/metric | v1.38.0 | v1.38.0 | Active | ✅ Up to date | Matches core OTEL version |
| go.opentelemetry.io/otel/sdk | v1.38.0 | v1.38.0 | Active | ✅ Up to date | Matches core OTEL version |
| go.opentelemetry.io/otel/exporters/prometheus | v0.60.0 | v0.60.0 | Active | ✅ Up to date | Matches OTEL release |
| go.opentelemetry.io/otel/exporters/otlp/* | v1.38.0 | v1.38.0 | Active | ✅ Up to date | Matches core OTEL version |
| prometheus/client_golang | v1.23.2 | Latest | - | 🟢 Low | Check for updates periodically |

---

## GitHub Actions

| Action | Current Version | Latest Stable | Update Priority | Notes |
|--------|----------------|---------------|-----------------|-------|
| actions/checkout | v4 | v6 | 🔴 High | 2 major versions behind |
| actions/setup-go | v5 | v6 | 🔴 High | Requires runner v2.327.1+, supports node24 |
| actions/upload-artifact | v4 | v4 | ✅ Up to date | Latest version |
| docker/build-push-action | v6 | v6 | ✅ Up to date | Latest version |
| docker/setup-buildx-action | v3 | v3 | ✅ Up to date | Check for v4 |
| docker/setup-qemu-action | v3 | v3 | ✅ Up to date | Check for v4 |
| docker/login-action | v3 | v3 | ✅ Up to date | Check for v4 |
| docker/metadata-action | v5 | v5 | ✅ Up to date | Latest version |
| golangci/golangci-lint-action | v9 | v9 | ✅ Up to date | Using golangci-lint v2.7.2 |
| codecov/codecov-action | v4 | v4 | ✅ Up to date | Latest version |
| aquasecurity/trivy-action | master | (unpinned) | 🔴 High | Should pin to specific version |
| github/codeql-action/upload-sarif | v3, v4 (mixed) | v4 | 🟡 Medium | Standardize to v4 everywhere |
| securego/gosec | master | (unpinned) | 🔴 High | Should pin to specific version |
| sigstore/cosign-installer | v3.3.0 | v3.x | 🟢 Low | Check for v3.4+ |
| anchore/sbom-action/download-syft | v0.15.0 | Latest | 🟢 Low | Check for updates |
| azure/setup-helm | v4 | v4 | ✅ Up to date | Latest version |
| softprops/action-gh-release | v2 | v2 | ✅ Up to date | Latest version |

**Security Note:** Using `@master` for security tools (trivy, gosec) is risky. Pin to specific versions for reproducibility.

---

## Pre-commit Hooks

| Hook | Current Version | Latest Stable | Update Priority | Notes |
|------|----------------|---------------|-----------------|-------|
| pre-commit/pre-commit-hooks | v5.0.0 | v5.x | ✅ Up to date | Check for v5.1+ |
| dnephin/pre-commit-golang | v0.5.1 | v0.5.x | ✅ Up to date | Latest version |
| golangci/golangci-lint | v1.64.8 | v2.7.2 | 🔴 High | Upgrade to v2.x |
| Yelp/detect-secrets | v1.5.0 | v1.5.x | ✅ Up to date | Latest stable |
| DavidAnson/markdownlint-cli2 | v0.19.0 | v0.19.x | ✅ Up to date | Check for updates |
| adrienverge/yamllint | v1.35.1 | v1.35.x | ✅ Up to date | Latest version |
| hadolint/hadolint | v2.12.0 | v2.12.x | ✅ Up to date | Check for updates |
| shellcheck-py/shellcheck-py | v0.10.0.1 | v0.10.x | ✅ Up to date | Latest version |
| jorisroovers/gitlint | v0.19.1 | v0.19.x | ✅ Up to date | Latest version |

---

## Go Module Dependencies (Critical)

| Module | Current Version | Latest Stable | Update Priority | Notes |
|--------|----------------|---------------|-----------------|-------|
| github.com/google/go-containerregistry | v0.20.7 | Latest | 🟢 Low | Check for updates |
| github.com/prometheus/client_golang | v1.23.2 | Latest | 🟢 Low | Check for updates |
| gopkg.in/yaml.v3 | v3.0.1 | v3.0.x | ✅ Up to date | Latest stable |
| k8s.io/klog/v2 | v2.100.1 | v2.130.x | 🟡 Medium | Check for updates |
| sigs.k8s.io/controller-runtime | (not in go.mod) | v0.19.x | 🟡 Medium | Consider adding if needed |

---

## Container Images (Runtime Dependencies)

| Image | Current Version | Latest Stable | Update Priority | Notes |
|-------|----------------|---------------|-----------------|-------|
| Zarf CLI | localhost/zarf:v0.66.0 | Latest | 🟡 Medium | Check defenseunicorns/zarf releases |
| UDS CLI | ghcr.io/defenseunicorns/uds-cli:latest | latest (floating) | 🔴 High | Pin to specific version for reproducibility |

**Security Note:** Using `:latest` tag for UDS CLI is not recommended for production. Pin to specific version.

---

## Update Strategy

### Priority Levels

- 🔴 **High Priority**: Security risk, EOL software, or multiple versions behind
- 🟡 **Medium Priority**: Minor versions behind, good to update but not critical
- 🟢 **Low Priority**: Up to date or user-managed tools
- ✅ **Up to date**: Already on latest stable version

### Recommended Update Order

1. **Kubernetes dependencies** (k8s.io/*) - v0.28.0 → v0.35.x
   - Critical: v0.28 is past EOL
   - Test compatibility with existing CRDs and controllers

2. **GitHub Actions security tools** (trivy, gosec)
   - Pin specific versions instead of `@master`

3. **GitHub Actions** (checkout, setup-go)
   - Update to v6 versions
   - Ensure runner compatibility (v2.327.1+)

4. **golangci-lint** in pre-commit config
   - v1.64.8 → v2.7.2
   - Update .config/golangci.yaml for v2 format

5. **Go runtime** (optional)
   - 1.24.0 → 1.24.11 (security patches)
   - Or → 1.25.5 (latest major, requires testing)

6. **UDS CLI image**
   - Pin to specific version instead of `:latest`

---

## Testing After Updates

After updating dependencies, run:

```bash
# Unit tests
make test

# Linting
make fmt vet
pre-commit run --all-files

# Integration tests
make integration-test-keep

# E2E tests
make e2e-test-keep
```

---

## References

- [Go Release Policy](https://go.dev/doc/devel/release)
- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/)
- [OpenTelemetry Go Releases](https://github.com/open-telemetry/opentelemetry-go/releases)
- [golangci-lint Releases](https://github.com/golangci/golangci-lint/releases)
- [GitHub Actions Changelog](https://github.blog/changelog/)
- [EndOfLife.date](https://endoflife.date/) - EOL tracking for various tools
