# Tool Version Tracking

This document tracks all tools, dependencies, and their versions used in the Forge project. It helps identify update opportunities and plan migration strategies.

**Last Updated**: 2025-12-27

---

## Language Runtime

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|----------------|-------|
| Go | 1.25.0 | 1.25.5 | Supported | 🟡 Medium | Update to latest patch version (1.25.5) for security fixes |

### Go Version Support

Go follows an N-2 support policy, maintaining the two most recent major versions (currently 1.25 and 1.24). Each version is supported for approximately 14 months. Update to patch versions promptly for security fixes.

**References**:
- [Go Release History](https://go.dev/doc/devel/release)
- [Go endoflife.date](https://endoflife.date/go)

---

## Build Tools

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|----------------|-------|
| controller-gen | v0.20.0 | v0.19.0 | N/A | ✅ Up to date | Current version is newer than latest stable |
| golangci-lint (CI) | v2.7.2 | v2.7.2 | N/A | ✅ Up to date | Already on latest version (Dec 7, 2025) |
| golangci-lint (pre-commit) | v2.7.2 | v2.7.2 | N/A | ✅ Up to date | Matches CI version |
| gofmt | (Go built-in) | 1.25.5 | Matches Go | 🟡 Medium | Update with Go version |
| goimports | latest | latest | N/A | ✅ Up to date | Using latest from golang.org/x/tools |

### Build Tool Notes

- **controller-gen**: Using v0.20.0 while latest stable release is v0.19.0 (Aug 28, 2025) - no action needed, ahead of curve
- **golangci-lint**: Already on latest v2.7.2 (released Dec 7, 2025). Supports same Go versions as official Go team (2 latest minors)

**References**:
- [controller-tools releases](https://github.com/kubernetes-sigs/controller-tools/releases)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases)
- [golangci-lint changelog](https://golangci-lint.run/docs/product/changelog/)

---

## Kubernetes Tools

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|----------------|-------|
| kubectl | N/A (user-installed) | v1.35.0 | Feb 28, 2026 (v1.32) | 🟢 Low | Consider documenting recommended version |
| Helm | N/A (user-installed) | v4.0.0 / v3.19.2 | N/A | 🟢 Low | Helm 4 released Nov 2025, v3 still maintained |
| Kind | N/A (user-installed) | v0.31.0 | N/A | 🟢 Low | Recommend v0.31.0 for CI |

### Kubernetes Version Compatibility

The project currently uses Kubernetes v1.35 client libraries (**k8s.io/client-go v0.35.0**). Kubernetes follows an N-2 support policy, maintaining the 3 most recent minor versions. Each version is supported for approximately 14 months.

**Current Supported Kubernetes Versions**:
- v1.35 (latest, released Dec 17, 2025 - "Timbernetes")
- v1.34
- v1.33

**Note**: Our project is already on the latest Kubernetes v1.35 libraries! This is excellent for security and feature support.

**References**:
- [Kubernetes releases](https://kubernetes.io/releases/)
- [Kubernetes endoflife.date](https://endoflife.date/kubernetes)
- [Helm releases](https://github.com/helm/helm/releases)
- [Kind releases](https://github.com/kubernetes-sigs/kind/releases)

---

## CI/CD Tools (GitHub Actions)

| Action | Current Version | Latest Stable | Update Priority | Notes |
|--------|----------------|---------------|----------------|-------|
| actions/checkout | v6, v4 (mixed) | v6.0.0 | 🟡 Medium | Update remaining v4 references to v6 |
| actions/setup-go | v6 | v6 | ✅ Up to date | Already on latest |
| actions/upload-artifact | v4 | v4 | ✅ Up to date | Already on latest |
| actions/cache | v4 | v4 | ✅ Up to date | Already on latest |
| actions/setup-python | v5 | v5 | ✅ Up to date | Already on latest |
| docker/build-push-action | v6, v3 (mixed) | v6 | 🔴 High | Update v3 references to v6 (ci.yaml:114, 148) |
| docker/setup-buildx-action | v3 | v3 | ✅ Up to date | Already on latest |
| docker/login-action | v3 | v3 | ✅ Up to date | Already on latest |
| codecov/codecov-action | v4 | v4 | ✅ Up to date | Already on latest |
| golangci/golangci-lint-action | v9 | v9 | ✅ Up to date | Already on latest |
| aquasecurity/trivy-action | master | (unpinned) | 🟡 Medium | Consider pinning to specific version tag |
| github/codeql-action/upload-sarif | v4 | v4 | ✅ Up to date | Already on latest |
| securego/gosec | v2.21.4 | v2.21.4 | ✅ Up to date | Already on latest (pinned) |

### CI/CD Update Notes

**High Priority**:
- Update `docker/build-push-action` from v3 to v6 in security job (ci.yaml:114)
- Update `actions/checkout` from v4 to v6 in security job (ci.yaml:148)

**Medium Priority**:
- Update remaining `actions/checkout@v4` references to v6 throughout workflows
- Consider pinning `aquasecurity/trivy-action` to specific version instead of `@master`

**References**:
- [actions/checkout releases](https://github.com/actions/checkout/releases) - v6.0.0 released Nov 20, 2025
- [actions/setup-go releases](https://github.com/actions/setup-go/releases) - v6 with intelligent caching
- [docker/build-push-action releases](https://github.com/docker/build-push-action/releases) - v6 latest

---

## Container Tools

| Tool | Current Version | Latest Stable | Update Priority | Notes |
|------|----------------|---------------|----------------|-------|
| Podman | (user-installed) | Latest | 🟢 Low | Auto-detected by Makefile |
| Docker | (user-installed) | Latest | 🟢 Low | Auto-detected by Makefile |

### Container Runtime Notes

The project auto-detects the container runtime (podman or docker) via Makefile. No specific version constraints are enforced. Both are actively maintained and users should use recent stable versions.

---

## Development Tools

| Tool | Current Version | Latest Stable | EOL Date | Update Priority | Notes |
|------|----------------|---------------|----------|----------------|-------|
| pre-commit | (pip install) | v4.5.1 | N/A | 🟡 Medium | Update docs to reference v4.5.1 (Dec 16, 2025) |
| yamllint | v1.35.1 | v1.35.1 | N/A | ✅ Up to date | Already on latest |
| markdownlint-cli2 | v0.19.0 | v0.19.0 | N/A | ✅ Up to date | Already on latest |
| hadolint | v2.12.0 | v2.12.0 | N/A | ✅ Up to date | Already on latest |
| shellcheck-py | v0.10.0.1 | v0.10.0.1 | N/A | ✅ Up to date | Already on latest |
| detect-secrets | v1.5.0 | v1.5.0 | N/A | ✅ Up to date | Already on latest |
| gitlint | v0.19.1 | v0.19.1 | N/A | ✅ Up to date | Already on latest |
| pre-commit-hooks | v5.0.0 | v5.0.0 | N/A | ✅ Up to date | Already on latest |
| dnephin/pre-commit-golang | v0.5.1 | v0.5.1 | N/A | ✅ Up to date | Already on latest |

### Development Tool Notes

Most development tools are already on latest stable versions. The pre-commit framework itself should be documented as v4.5.1 (latest release Dec 16, 2025).

**References**:
- [pre-commit releases](https://github.com/pre-commit/pre-commit/releases)
- [pre-commit.com](https://pre-commit.com/)

---

## Go Dependencies

### Major Direct Dependencies

| Package | Current Version | Latest Stable | Update Priority | Notes |
|---------|----------------|---------------|----------------|-------|
| k8s.io/client-go | v0.35.0 | v0.35.0 | ✅ Up to date | Released Dec 17, 2025 with K8s 1.35 |
| k8s.io/api | v0.35.0 | v0.35.0 | ✅ Up to date | Matches client-go |
| k8s.io/apimachinery | v0.35.0 | v0.35.0 | ✅ Up to date | Matches client-go |
| k8s.io/klog/v2 | v2.130.1 | v2.130.1 | ✅ Up to date | Already on latest |
| sigs.k8s.io/controller-runtime | (not direct) | v0.22.4 | 🟡 Medium | May need to add as direct dependency |
| sigs.k8s.io/controller-tools | v0.20.0 | v0.19.0 | ✅ Up to date | Ahead of latest stable release |
| go.opentelemetry.io/otel | v1.38.0 | v1.38.0 | ✅ Up to date | Already on latest |
| go.opentelemetry.io/otel/metric | v1.38.0 | v1.38.0 | ✅ Up to date | Matches core OTEL |
| go.opentelemetry.io/otel/sdk | v1.38.0 | v1.38.0 | ✅ Up to date | Matches core OTEL |
| go.opentelemetry.io/otel/exporters/prometheus | v0.60.0 | v0.60.0 | ✅ Up to date | Matches OTEL SDK |
| github.com/prometheus/client_golang | v1.23.2 | v1.23.2 | ✅ Up to date | Already on latest |
| github.com/google/go-containerregistry | v0.20.7 | v0.20.7 | ✅ Up to date | Already on latest |
| gopkg.in/yaml.v3 | v3.0.1 | v3.0.1 | ✅ Up to date | Already on latest |

### Kubernetes Compatibility Matrix

**Excellent news**: The project is already on Kubernetes v1.35 libraries (latest as of Dec 17, 2025)!

- **k8s.io/client-go v0.35.0** → K8s 1.35 compatible
- **k8s.io/api v0.35.0** → K8s 1.35 compatible
- **k8s.io/apimachinery v0.35.0** → K8s 1.35 compatible

**Note**: controller-runtime v0.22.4 (latest) supports client-go v0.34, while we're using v0.35. This minor mismatch is typically fine but should be monitored. Consider testing or waiting for controller-runtime v0.23+ for full v0.35 support.

### Dependency Update Process

1. Run `go list -u -m all` to see available updates
2. Check Kubernetes compatibility matrices
3. Update incrementally, starting with patch versions
4. Run full test suite after each major update
5. Run `go mod tidy` to clean up

**References**:
- [controller-runtime releases](https://github.com/kubernetes-sigs/controller-runtime/releases)
- [client-go releases](https://github.com/kubernetes/client-go/releases)
- [Kubernetes compatibility matrix](https://github.com/kubernetes/client-go#compatibility-matrix)

---

## Deployment Images

### CLI Container Images

| Image | Current Version | Latest Stable | Update Priority | Notes |
|-------|----------------|---------------|----------------|-------|
| Zarf CLI | v0.66.0 | v0.68.1 | 🔴 High | Update to v0.68.1 (released Dec 18, 2025) |
| UDS CLI | latest | v0.27.13+ | 🔴 High | Pin to specific version instead of :latest |

### Deployment Image Notes

**High Priority Updates**:

1. **Zarf CLI**: Update from `v0.66.0` to `v0.68.1` (latest, Dec 18, 2025)
   - Location: `pkg/constants/config.go:42`
   - Current: `localhost/zarf:v0.66.0`
   - Target: `localhost/zarf:v0.68.1`

2. **UDS CLI**: Pin to specific version
   - Location: `pkg/constants/config.go:45`
   - Current: `ghcr.io/defenseunicorns/uds-cli:latest`
   - Target: `ghcr.io/defenseunicorns/uds-cli:v0.27.13` (or latest specific version)

**Impact**: Updating these images requires:
1. Update `pkg/constants/config.go`
2. Update Makefile reference (line 309: `kind-zarf-cli` target)
3. Full integration test run
4. Update documentation referencing these versions

**References**:
- [Zarf releases](https://github.com/zarf-dev/zarf/releases) - v0.68.1 latest
- [UDS CLI releases](https://github.com/defenseunicorns/uds-cli/releases) - v0.27.13+

---

## Update Strategy

### Priority Levels

- 🔴 **High**: Security fixes, major version updates with important features, known compatibility issues
- 🟡 **Medium**: Minor version updates, patch releases with bug fixes
- 🟢 **Low**: Optional updates, non-critical improvements, user-managed tools
- ✅ **Up to date**: Already on latest version

### Recommended Update Order

#### Batch 1: CLI Container Images (High Priority)

**Risk**: Medium | **Value**: High | **Effort**: Low

1. Update Zarf CLI from v0.66.0 to v0.68.1
2. Pin UDS CLI to specific version (v0.27.13+)
3. Update Makefile `kind-zarf-cli` target
4. Run integration tests

#### Batch 2: GitHub Actions (High Priority)

**Risk**: Low | **Value**: High | **Effort**: Low

1. Update `docker/build-push-action` from v3 to v6 (ci.yaml:114)
2. Update `actions/checkout` from v4 to v6 (ci.yaml:148)
3. Standardize all `actions/checkout` to v6
4. Run CI pipeline to verify

#### Batch 3: Go Runtime (Medium Priority)

**Risk**: Low | **Value**: Medium | **Effort**: Low

1. Update Go from 1.25.0 to 1.25.5 (security patches)
2. Update all CI workflows to match
3. Run full test suite
4. Update documentation

#### Batch 4: Pre-commit Framework (Low Priority)

**Risk**: Low | **Value**: Low | **Effort**: Low

1. Update documentation to reference pre-commit v4.5.1
2. Update CI if needed
3. Test pre-commit hooks locally

### Update Workflow

1. **Prepare**
   - Review changelogs and release notes
   - Check for breaking changes
   - Identify dependencies between updates

2. **Test**
   - Update in development environment
   - Run full test suite (`make test`)
   - Run integration tests (`make integration-test`)
   - Run E2E tests (`make e2e-test`)

3. **Deploy**
   - Create feature branch
   - Update versions
   - Update documentation
   - Run pre-commit hooks
   - Create PR with testing evidence

4. **Monitor**
   - Watch for issues in CI/CD
   - Monitor cluster stability
   - Check metrics and logs

---

## Summary of Action Items

### Immediate (This Week)

1. ✅ **Complete**: Create this TOOL_VERSIONS.md document
2. 🔴 **High**: Update Zarf CLI to v0.68.1 in `pkg/constants/config.go`
3. 🔴 **High**: Pin UDS CLI to v0.27.13 (or latest) in `pkg/constants/config.go`
4. 🔴 **High**: Update GitHub Actions docker/build-push-action to v6
5. 🔴 **High**: Update GitHub Actions checkout to v6 consistently

### Short-term (This Month)

6. 🟡 **Medium**: Update Go runtime to 1.25.5
7. 🟡 **Medium**: Update pre-commit documentation to v4.5.1
8. 🟡 **Medium**: Consider adding controller-runtime as direct dependency

### Long-term (Next Quarter)

9. 🟢 **Low**: Document recommended kubectl/helm/kind versions
10. 🟢 **Low**: Review and update Go module dependencies
11. 🟢 **Low**: Consider Helm 4 compatibility testing

---

## Monitoring Version Updates

### Automated Tools

- **Dependabot**: Already enabled for Go dependencies
- **go list -u -m all**: Check for Go dependency updates
- **GitHub Actions version updates**: Monitor via Dependabot

### Regular Maintenance Schedule

- **Weekly**: Check for security advisories
- **Monthly**: Review major dependency updates
- **Quarterly**: Full dependency audit and update cycle
- **As needed**: Respond to CVEs and critical patches

### Update This Document

Remember to update this document whenever:
- Dependency versions change
- New tools are added to the project
- EOL dates are announced
- Major version updates are completed

---

## Version Testing Checklist

Before updating any tool version, ensure:

- [ ] Reviewed changelog and breaking changes
- [ ] Updated documentation to reflect changes
- [ ] Run `make fmt vet` successfully
- [ ] Run `make test` successfully
- [ ] Run `make integration-test` successfully
- [ ] Run `make e2e-test` successfully
- [ ] Tested in Kind cluster locally
- [ ] Updated this TOOL_VERSIONS.md document
- [ ] Created tracking issue if update requires multiple PRs

---

## Additional Resources

- [Go releases](https://go.dev/doc/devel/release)
- [Kubernetes releases](https://kubernetes.io/releases/)
- [endoflife.date](https://endoflife.date/) - Track EOL dates for many tools
- [GitHub Actions changelog](https://github.blog/changelog/)
- [Kubernetes compatibility matrix](https://github.com/kubernetes/client-go#compatibility-matrix)
- [OpenTelemetry Go releases](https://github.com/open-telemetry/opentelemetry-go/releases)
- [Prometheus client_golang releases](https://github.com/prometheus/client_golang/releases)
