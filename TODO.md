# TODO

---

## 🔴 Critical Issues (Security/Correctness)

**All critical issues resolved! 🎉**

---

## 🟡 High Priority (Consistency/Maintainability)

### 1. Handler Signature Mismatch Between Zarf and UDS

**Location**: `pkg/actions/zarf/*.go` vs `pkg/actions/uds/*.go`
**Issue**:

- Zarf: `Execute(ctx, pkg *ZarfPackageJob, artifactPVCName string)`
- UDS: `Execute(ctx, bundle *UDSBundleJob)`

**Impact**: Makes generic handler interfaces impossible, increases maintenance burden.
**Action**: Unify signatures or document architectural decision for divergence.

---

## 🟢 Medium Priority (Code Quality)

### 2. Significant Code Duplication in Action Handlers

**Location**: `pkg/actions/zarf/*.go` and `pkg/actions/uds/*.go`
**Issue**: ~2000 lines across 6 handlers share only 610 lines via `pkg/actions/common/`.
**Duplicated patterns**:

- Job creation boilerplate
- Init container building
- Resource requirement defaults
- Volume mounting logic

**Action**: Consolidate Job building logic - expand JobBuilder pattern to reduce duplication.

### 3. v1alpha1 → v1alpha2 Migration Incomplete

**Location**: `pkg/apis/uds/v1alpha2/`, CRD definitions, controllers
**Missing pieces**:

- Conversion webhook between v1alpha1 and v1alpha2
- CRD conversion rules in Helm charts
- v1alpha2 controller (only v1alpha1 is currently watched)
- Automatic migration path for existing resources

**Action**: Implement full migration path or deprecate v1alpha1 completely.

### 4. Examples Use Deprecated v1alpha1 API

**Location**: `examples/samples/uds/`
**Issue**: Most examples still use `UDSBundleJob` (v1alpha1) instead of `UDSPackageJob` (v1alpha2).
**Files**:

- `01-git-to-oci/udsbundlejob.yaml`
- `02-local-to-s3/udsbundlejob.yaml`
- `03-git-build-deploy/udsbundlejob.yaml`

**Action**: Update examples to use v1alpha2 as primary, keep v1alpha1 as legacy reference.

### 5. Reconciliation Methods Lack Inline Comments

**Location**: `pkg/controller/controller.go:183-277`, `pkg/controller/uds_controller.go:186-260`
**Issue**: 95-line reconciliation methods with minimal documentation explaining:

- Compound action handling (BuildPublish, BuildDeploy)
- Job monitoring chain orchestration
- Status update structure

**Action**: Add inline comments documenting control flow and state transitions.

---

## 📊 Testing Gaps

### 6. Multi-Action Job Chaining Not Comprehensively Tested

**Location**: Test files for controllers and handlers
**Issue**: Complex orchestration (Build → Publish → Deploy) requires Job completion monitoring and status tracking, but test coverage is minimal.
**Action**: Add integration tests for:

- BuildPublish action (Build completes, then Publish starts)
- BuildDeploy action
- Full BuildPublishDeploy chain

### 7. Error Path Coverage Limited

**Location**: `pkg/actions/zarf/*_test.go`, `pkg/actions/uds/*_test.go`
**Missing tests**:

- Job creation failures
- Status update failures
- Race conditions in job monitoring
- Multiple completed jobs edge cases

**Action**: Add test cases for error scenarios and edge cases.

---

## 📝 Documentation Issues

### 8. README References Non-Existent Documentation Files

**Location**: `README.md`
**Issue**: References `docs/development/SERVICEACCOUNT_REFERENCE.md` which doesn't exist.
**Action**: Update README to reference correct documentation structure or create missing file.

---

## 🔧 Tool & Dependency Updates

### 9. Create Tool Version Tracking Document

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

### 10. Update Development Tools to Latest Stable Versions

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

### 11. Update GitHub Actions to Latest Stable

**Action**: Review `.github/workflows/*.yaml` and update action versions.

**Common actions to check**:

- `actions/checkout@v*`
- `actions/setup-go@v*`
- `docker/build-push-action@v*`
- `golangci/golangci-lint-action@v*`

**Reference**: https://github.com/actions/* for official actions

### 12. Update Go Dependencies

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

---

## Future Enhancements (Long-term, Non-critical)

- **Support adopting existing resources** for deploy actions
- **Support for additional destination types** (Artifactory, Nexus)
- **Configurable resource requirements** via CRD (not just hardcoded defaults)
- **Job retry logic** for transient failures
- **Progress reporting** for long-running operations
- **Multi-architecture** bundle builds
- **Webhook TLS certificate rotation** without downtime
