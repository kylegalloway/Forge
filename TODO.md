# TODO

---

## 🔴 Critical Issues (Security/Correctness)

**All critical issues resolved! 🎉**

---

## 🟡 High Priority (Consistency/Maintainability)

**All high-priority issues resolved! 🎉**

---

## 🟢 Medium Priority (Code Quality)

### 1. ~~Significant Code Duplication in Action Handlers~~ ✅ COMPLETE

**Location**: `pkg/actions/zarf/*.go` and `pkg/actions/uds/*.go`
**Status**: **COMPLETE** (2025-12-27)

**What was done**:
- ✅ Added 8 consolidation helpers to `pkg/actions/job_builder.go`:
  - `BuildResourceRequirements()`, `PublishResourceRequirements()`, `DeployResourceRequirements()`
  - `NonRootSecurityContextWithUID()` and `NonRootPodSecurityContextWithUID()`
  - `ParseTimeoutWithDefault()` for timeout string parsing
  - `AddKubeconfigVolume()` and `AddArtifactPVCVolume()` for common volume mounting
- ✅ Refactored all 6 handlers to use the new helpers:
  - Zarf: build.go, publish.go, deploy.go
  - UDS: create.go, publish.go, deploy.go
- ✅ Eliminated ~300+ lines of duplicated code across handlers
- ✅ All tests passing (actions/zarf: 80.5%, actions/uds: 81.2%)

**Results**:
- Better maintainability - changes to security contexts, resources, or timeouts now happen in one place
- Consistent resource requirements across Zarf and UDS handlers
- Reduced risk of bugs from copy-paste errors
- Cleaner, more readable handler code

---

## 📊 Testing Gaps

### 3. Multi-Action Job Chaining Not Comprehensively Tested

**Location**: Test files for controllers and handlers
**Issue**: Complex orchestration (Build → Publish → Deploy) requires Job completion monitoring and status tracking, but test coverage is minimal.
**Action**: Add integration tests for:

- BuildPublish action (Build completes, then Publish starts)
- BuildDeploy action
- Full BuildPublishDeploy chain

### 4. Error Path Coverage Limited

**Location**: `pkg/actions/zarf/*_test.go`, `pkg/actions/uds/*_test.go`
**Missing tests**:

- Job creation failures
- Status update failures
- Race conditions in job monitoring
- Multiple completed jobs edge cases

**Action**: Add test cases for error scenarios and edge cases.

---

## 🔧 Tool & Dependency Updates

### 5. Create Tool Version Tracking Document

**Action**: Create `docs/development/TOOL_VERSIONS.md` to document all tools and their current/target versions.
**Should include**:

- Language runtime (Go version)
- Build tools (golangci-lint, controller-gen, gofmt, goimports)
- Kubernetes tools (kubectl, helm, kind)
- CI/CD tools (GitHub Actions versions)
- Container tools (Docker/Podman versions)
- Development tools (pre-commit, yamllint, markdownlint-cli2)
- Go dependencies (major libraries like controller-runtime, client-go, etc.)

**Format**: Table with columns: Tool, Current Version, Latest Stable, EOL Date, Update Priority

### 6. Update Development Tools to Latest Stable Versions

**Research needed**: Use [endoflife.date](https://endoflife.date/) and/or direct tool documentation to identify latest stable versions.

**Tools to update**:

- **golangci-lint** - Currently on v2 format (recent upgrade from v1)
  - Check: `golangci-lint version` vs https://endoflife.date/golangci-lint
- **controller-gen** - Used for CRD generation
  - Check: Go module version vs https://github.com/kubernetes-sigs/controller-tools/releases
- **kubectl** - Kubernetes CLI
  - Check: https://endoflife.date/kubectl
- **helm** - Package manager
  - Check: https://endoflife.date/helm
- **kind** - Local cluster tool
  - Check: https://endoflife.date/kind or https://github.com/kubernetes-sigs/kind/releases
- **pre-commit** - Git hook manager
  - Check: https://github.com/pre-commit/pre-commit/releases
- **Go runtime** - Language version
  - Check: https://endoflife.date/go

**Strategy**: Target latest stable/LTS versions, avoid bleeding-edge releases. Prioritize security-supported versions.

### 7. Update GitHub Actions to Latest Stable

**Action**: Review `.github/workflows/*.yaml` and update action versions.

**Common actions to check**:

- `actions/checkout@v*`
- `actions/setup-go@v*`
- `docker/build-push-action@v*`
- `golangci/golangci-lint-action@v*`

**Reference**: https://github.com/actions/* for official actions

### 8. Update Go Dependencies

**Action**: Review and update major Go modules in `go.mod`.

**Key dependencies to check**:

- `sigs.k8s.io/controller-runtime`
- `k8s.io/client-go`
- `k8s.io/api`
- `k8s.io/apimachinery`
- OpenTelemetry libraries
- Zarf/UDS CLI container image versions (constants.ZarfCLIImage, constants.UDSCLIImage)

**Process**:

1. Run `go list -u -m all` to see available updates
2. Check compatibility matrices for Kubernetes client-go versions
3. Update incrementally, run tests after each major update
4. Update vendor directory if used

---

## Architecture Observations (For Consideration)

### Controller Complexity Growing (Optional Refactor)

**Current state**:

- Zarf Controller: 356 lines with 5 handler methods + job monitoring
- UDS Controller: 417 lines with different structure (watch in separate function)

**Issue**: Different approaches make consistency harder to maintain.
**Consideration**: Create unified base controller class to share common patterns.

---

## ✅ Strengths (No Action Required)

The following are working well:

- ✅ All tests passing
- ✅ Test coverage at target levels (52% overall, 82.7%+ critical packages)
- ✅ Clean handler pattern with good separation of concerns
- ✅ Consistent use of telemetry and tracing
- ✅ Attestation system fully implemented with comprehensive tests
- ✅ Clear separation between examples and tests
- ✅ Good dependency injection throughout
- ✅ UDS Policy validation fully implemented
- ✅ ServiceAccount enforcement in all Zarf handlers
- ✅ UDS S3 and OCI sources implemented with shared builder pattern
- ✅ Controller watch loops with consistent retry logic
- ✅ Constants package with comprehensive documentation
- ✅ Magic strings replaced with typed constants throughout
- ✅ UDS examples updated to v1alpha2 with correct annotation keys
- ✅ SERVICEACCOUNT_REFERENCE.md exists and is comprehensive
- ✅ Handler signature divergence documented in ARCHITECTURE.md

---

## Future Enhancements (Long-term, Non-critical)

- **Support adopting existing resources** for deploy actions
- **Support for additional destination types** (Artifactory, Nexus)
- **Configurable resource requirements** via CRD (not just hardcoded defaults)
- **Job retry logic** for transient failures
- **Progress reporting** for long-running operations
- **Multi-architecture** bundle builds
- **Webhook TLS certificate rotation** without downtime
