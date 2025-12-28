# TODO

---

## 🔴 Critical Issues (Security/Correctness)

**All critical issues resolved! 🎉**

---

## 🟡 High Priority (Consistency/Maintainability)

### UDS bundles shouldn't be called UDS packages. Update this naming across the board

---

## 🟢 Medium Priority (Code Quality)

### Documentation has too many outdated/unused docs at the top level and in docs/*. Clean them up/consolidate

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

**All tool and dependency updates completed! 🎉** (2025-12-27)

See `docs/development/TOOL_VERSIONS.md` for comprehensive version tracking and update history.

---

## Architecture Observations (For Consideration)

### Controller Implementation Divergence (Optional Refactor)

**Current state**:

- Zarf Controller: 416 lines, 11 methods
  - Event handling: `handleEvent` → `handleObject` → `handleZarfPackageJob` → `reconcilePackage`
  - Job monitoring integrated into `reconcilePackage`
- UDS Controller: 470 lines, 14 methods
  - Event handling: `handleWatchEvent` → `reconcile` → `validatePolicy`/`dispatchAction`
  - Separate methods: `executeCreate`, `executePublish`, `executeDeploy`

**Issue**: Inconsistent patterns for similar functionality increases maintenance burden and makes it harder to apply fixes consistently across both controllers.

**Shared patterns that could be extracted**:
- Health check handlers (identical implementations)
- Status update logic (similar patterns)
- Watch loop setup and error handling
- Policy validation flow

**Consideration**: Extract common controller behaviors into a shared base struct or interface to reduce duplication and ensure consistent behavior. This is non-critical but would improve long-term maintainability.

**Priority**: Low - Both controllers are stable and well-tested. Only pursue if making significant changes to controller logic.

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
