# TODO

---

## 🔴 Critical Issues (Security/Correctness)

**All critical issues resolved! 🎉**

---

## 🟡 High Priority (Consistency/Maintainability)

### 2. Fix the gitlab-ci yaml to mostly match the github actions

**Location**: .gitlab-ci.yml
**Issue**: Gitlab CI instructions haven't been updated.
**Action**: Update the gitlab-ci instructions

---

## 🟢 Medium Priority (Code Quality)

**All medium-priority issues resolved! 🎉**

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

### ✅ 5. Create Tool Version Tracking Document (COMPLETED 2025-12-27)

**Status**: COMPLETED

**Deliverable**: Created comprehensive `docs/development/TOOL_VERSIONS.md` with:
- Language runtime tracking (Go 1.25.0 → 1.25.5)
- Build tools version matrix (golangci-lint v2.7.2, controller-gen v0.20.0)
- Kubernetes tools documentation (kubectl v1.35, helm v4.0.0, kind v0.31.0)
- CI/CD tools audit (GitHub Actions versions)
- Development tools status (pre-commit v4.5.1, all hooks latest)
- Go dependencies matrix (k8s.io/* v0.35.0, OTEL v1.38.0)
- Update priority system (High/Medium/Low/Up-to-date)
- Detailed update strategy and testing checklist

**Key Finding**: Project is in excellent shape! Most tools already on latest versions.

### ✅ 6. Update Development Tools to Latest Stable Versions (COMPLETED 2025-12-27)

**Status**: COMPLETED

**Updates Applied**:
- ✅ **Go runtime**: 1.25.0 → 1.25.5 (latest patch with security fixes)
  - Updated `go.mod`
  - Ran `go mod tidy`
  - All tests passing
- ✅ **Zarf CLI Image**: v0.66.0 → v0.68.1 (latest, Dec 18, 2025)
  - Updated `pkg/constants/config.go:42`
  - Updated `Makefile:309` (kind-zarf-cli target)
- ✅ **UDS CLI Image**: `:latest` → `v0.27.13` (pinned for reproducibility)
  - Updated `pkg/constants/config.go:45`

**Already on Latest** (no updates needed):
- ✅ golangci-lint v2.7.2 (latest, Dec 7, 2025)
- ✅ controller-gen v0.20.0 (ahead of latest stable v0.19.0)
- ✅ All pre-commit hooks on latest versions
- ✅ kubectl, helm, kind (latest versions documented)

### ✅ 7. Update GitHub Actions to Latest Stable (COMPLETED 2025-12-27)

**Status**: COMPLETED

**Updates Applied**:
- ✅ **actions/checkout**: Updated v4 → v6 in ci.yaml
  - Security job (line 148)
  - Docker job (line 114)

**Already on Latest** (verified across all workflows):
- ✅ actions/setup-go@v6
- ✅ docker/build-push-action@v6
- ✅ golangci/golangci-lint-action@v9
- ✅ codecov/codecov-action@v4
- ✅ actions/upload-artifact@v4
- ✅ actions/cache@v4
- ✅ docker/setup-buildx-action@v3
- ✅ docker/login-action@v3
- ✅ All other actions on latest versions

**Note**: aquasecurity/trivy-action@master remains unpinned (medium priority for future improvement).

### ✅ 8. Update Go Dependencies (COMPLETED - Already Up-to-Date!)

**Status**: COMPLETED - No updates needed!

**Current State** (verified 2025-12-27):
- ✅ **k8s.io/client-go** v0.35.0 (latest, Dec 17, 2025 with K8s 1.35)
- ✅ **k8s.io/api** v0.35.0
- ✅ **k8s.io/apimachinery** v0.35.0
- ✅ **k8s.io/klog/v2** v2.130.1
- ✅ **go.opentelemetry.io/otel** v1.38.0 (all OTEL packages)
- ✅ **github.com/prometheus/client_golang** v1.23.2
- ✅ **github.com/google/go-containerregistry** v0.20.7
- ✅ **gopkg.in/yaml.v3** v3.0.1

**Findings**:
- Kubernetes dependencies already on latest v1.35 libraries! 🎉
- All OpenTelemetry packages on latest v1.38.0
- All major dependencies up to date
- Zero updates required

**Note**: controller-runtime v0.22.4 supports client-go v0.34 while we're using v0.35. Minor mismatch is acceptable, monitor for future controller-runtime v0.23+ release.

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
