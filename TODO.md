# TODO

---

## 🔴 Critical Issues (Security/Correctness)

### 1. UDS Policy Validation Not Implemented

**Location**: `pkg/controller/uds_controller.go:217`
**Issue**: `validatePolicy()` always returns `nil` - UDS bundles bypass all policy enforcement while Zarf packages are validated.
**Impact**: Security gap - UDS operations not subject to RBAC controls via ServiceAccount annotations.
**Action**: Implement proper policy validation using the same engine as Zarf controller.

### 2. ServiceAccountName Missing in Zarf Build/Publish Jobs

**Location**: `pkg/actions/zarf/build.go`, `pkg/actions/zarf/publish.go`
**Issue**: Deploy handler sets `job.Spec.Template.Spec.ServiceAccountName` correctly, but Build and Publish handlers don't.
**Impact**: RBAC controls bypassed - jobs run under `default` service account instead of specified one.
**Action**: Add `job.Spec.Template.Spec.ServiceAccountName = pkg.Spec.ServiceAccountName` to both handlers.

### 3. UDS Source Adapters Incomplete (S3, OCI)

**Location**: `pkg/sources/uds_adapters.go:40-52`
**Issue**: S3 and OCI source handlers return `nil, nil` (silent failure) with TODO comments.
**Impact**: Jobs silently fail at runtime instead of failing fast at creation.
**Action**: Either implement adapters or return proper errors (don't silently ignore).

---

## 🟡 High Priority (Consistency/Maintainability)

### 4. Handler Signature Mismatch Between Zarf and UDS

**Location**: `pkg/actions/zarf/*.go` vs `pkg/actions/uds/*.go`
**Issue**:

- Zarf: `Execute(ctx, pkg *ZarfPackageJob, artifactPVCName string)`
- UDS: `Execute(ctx, bundle *UDSBundleJob)`

**Impact**: Makes generic handler interfaces impossible, increases maintenance burden.
**Action**: Unify signatures or document architectural decision for divergence.

### 5. Controller Watch Reconnection Logic Divergence

**Location**: `pkg/controller/controller.go` vs `pkg/controller/uds_controller.go`
**Issue**:

- Zarf: No sleep on watcher reconnect (tight loop risk)
- UDS: 5-second sleep on watch errors (more resilient)

**Impact**: Zarf controller could hammer API server during outages.
**Action**: Add backoff/sleep to Zarf controller watch loop for consistency.

### 6. Missing Package Documentation in Constants

**Location**: `pkg/constants/*.go` (4 files)
**Issue**: No package-level godoc comments in:

- `actions.go`
- `api.go`
- `labels.go`
- `config.go`

**Action**: Add package-level documentation explaining purpose and usage patterns.

### 7. Magic Strings Not Using Constants

**Location**: Throughout codebase (job creation, labels, volume names)
**Issue**: Hardcoded strings like `"workspace"`, `"output"`, `"artifacts"`, `"zarf-build"` instead of constants.
**Action**: Create constants for:

- Volume and mount names
- Container names
- Job label selectors (`"app=forge"`, `"app=forge-uds"`)

---

## 🟢 Medium Priority (Code Quality)

### 8. Significant Code Duplication in Action Handlers

**Location**: `pkg/actions/zarf/*.go` and `pkg/actions/uds/*.go`
**Issue**: ~2000 lines across 6 handlers share only 610 lines via `pkg/actions/common/`.
**Duplicated patterns**:

- Job creation boilerplate
- Init container building
- Resource requirement defaults
- Volume mounting logic

**Action**: Consolidate Job building logic - expand JobBuilder pattern to reduce duplication.

### 9. v1alpha1 → v1alpha2 Migration Incomplete

**Location**: `pkg/apis/uds/v1alpha2/`, CRD definitions, controllers
**Missing pieces**:

- Conversion webhook between v1alpha1 and v1alpha2
- CRD conversion rules in Helm charts
- v1alpha2 controller (only v1alpha1 is currently watched)
- Automatic migration path for existing resources

**Action**: Implement full migration path or deprecate v1alpha1 completely.

### 10. Examples Use Deprecated v1alpha1 API

**Location**: `examples/samples/uds/`
**Issue**: Most examples still use `UDSBundleJob` (v1alpha1) instead of `UDSPackageJob` (v1alpha2).
**Files**:

- `01-git-to-oci/udsbundlejob.yaml`
- `02-local-to-s3/udsbundlejob.yaml`
- `03-git-build-deploy/udsbundlejob.yaml`

**Action**: Update examples to use v1alpha2 as primary, keep v1alpha1 as legacy reference.

### 11. Inconsistent Error Handling in Source Adapters

**Location**: `pkg/sources/uds_adapters.go`
**Issue**: Unimplemented sources return `nil, nil`; unknown sources return errors. Inconsistent pattern.
**Action**: Standardize error handling - all unsupported/unimplemented sources should return errors.

### 12. Reconciliation Methods Lack Inline Comments

**Location**: `pkg/controller/controller.go:183-277`, `pkg/controller/uds_controller.go:186-260`
**Issue**: 95-line reconciliation methods with minimal documentation explaining:

- Compound action handling (BuildPublish, BuildDeploy)
- Job monitoring chain orchestration
- Status update structure

**Action**: Add inline comments documenting control flow and state transitions.

---

## 📊 Testing Gaps

### 13. Multi-Action Job Chaining Not Comprehensively Tested

**Location**: Test files for controllers and handlers
**Issue**: Complex orchestration (Build → Publish → Deploy) requires Job completion monitoring and status tracking, but test coverage is minimal.
**Action**: Add integration tests for:

- BuildPublish action (Build completes, then Publish starts)
- BuildDeploy action
- Full BuildPublishDeploy chain

### 14. Error Path Coverage Limited

**Location**: `pkg/actions/zarf/*_test.go`, `pkg/actions/uds/*_test.go`
**Missing tests**:

- Job creation failures
- Status update failures
- Race conditions in job monitoring
- Multiple completed jobs edge cases

**Action**: Add test cases for error scenarios and edge cases.

---

## 📝 Documentation Issues

### 15. README References Non-Existent Documentation Files

**Location**: `README.md`
**Issue**: References `docs/development/SERVICEACCOUNT_REFERENCE.md` which doesn't exist.
**Action**: Update README to reference correct documentation structure or create missing file.

---

## Architecture Observations (For Consideration)

### 16. Controller Complexity Growing (Optional Refactor)

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

---

## Future Enhancements (Long-term, Non-critical)

- **Support adopting existing resources** for deploy actions
- **Support for additional destination types** (Artifactory, Nexus)
- **Configurable resource requirements** via CRD (not just hardcoded defaults)
- **Job retry logic** for transient failures
- **Progress reporting** for long-running operations
- **Multi-architecture** bundle builds
- **Webhook TLS certificate rotation** without downtime
