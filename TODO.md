# TODO

---

## 🔴 Critical Issues (Security/Correctness)

**All critical issues resolved! 🎉**

---

## 🟡 High Priority (Consistency/Maintainability)

**All high-priority issues resolved! 🎉**

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

**All tool and dependency updates completed! 🎉** (2025-12-27)

See `docs/development/TOOL_VERSIONS.md` for comprehensive version tracking and update history.

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
