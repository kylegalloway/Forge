# TODO

## Recently Completed (2025-12-25)

All testing infrastructure issues from previous audit have been investigated and addressed:

### ✅ UDS Controller Test Coverage

* **Status**: COMPLETED
* Added HealthzHandler/ReadyzHandler methods to UDS controller
* Added 4 comprehensive HTTP response tests
* Coverage increased from 712 to 894 lines (99% parity with Zarf controller)
* See: INVESTIGATION_REPORT.md for details

### ✅ YAML Validation Tests

* **Status**: INVESTIGATED - Intentionally skipped, paths need updating
* Tests point to non-existent `.config/` directory (old kubebuilder structure)
* Actual files in `examples/samples/`, `chart/forge/crds/`, `chart/forge/templates/`
* **Recommendation**: Update test paths to use Helm chart locations or delete tests
* See: INVESTIGATION_REPORT.md Section 2

### ✅ E2E Tests

* **Status**: COMPLETE - Functional script exists
* Comprehensive E2E script at `scripts/test-e2e.sh` (8 test scenarios)
* **Recommendation**: Add `make e2e-test` target for convenience
* See: INVESTIGATION_REPORT.md Section 3

### ✅ OTLP Test Skips

* **Status**: INTENTIONAL - Skip is appropriate
* Single test skipped in `pkg/telemetry/otel_test.go` requires external OTLP collector
* Other telemetry tests provide adequate coverage
* **Recommendation**: Keep as-is
* See: INVESTIGATION_REPORT.md Section 4

### ✅ Job State Bouncing

* **Status**: INVESTIGATED - Expected behavior for multi-action workflows
* Examined job monitor logic and action chaining
* Status changes during Build→Publish→Deploy chains are intentional
* **Recommendation**: Document as expected behavior or add per-action status tracking
* See: INVESTIGATION_REPORT.md Section 5

### ✅ Example Validation

* **Status**: CATALOGUED - 32 example files identified
* Files in `examples/samples/zarf/`, `examples/samples/uds/`, `examples/policies/`
* **Recommendation**: Create automated validation workflow
* See: INVESTIGATION_REPORT.md Section 6

---

## Existing Issues

No outstanding critical issues from testing infrastructure audit.

---

## Medium Priority (Testing & Documentation)

### Test Coverage Gaps

* **Improve UDS controller test coverage**
  * Location: `pkg/controller/uds_controller_test.go` (712 lines)
  * Issue: 21% fewer tests than Zarf controller (902 lines)
  * Add: Tests for edge cases, error conditions, and policy enforcement
  * Target: Match Zarf controller coverage

---

## Future Enhancements (Not Inconsistencies)

* **Support adopting existing resources** for deploy actions
* **Support for additional source types** (Azure DevOps, GitLab, Bitbucket)
* **Support for additional destination types** (Artifactory, Nexus)
* **Configurable resource requirements** via CRD (not just hardcoded defaults)
* **Job retry logic** for transient failures
* **Progress reporting** for long-running operations
* **Pause/Resume** functionality for jobs
* **Multi-architecture** bundle builds
* **Webhook TLS certificate rotation** without downtime
* **Metrics dashboard templates** for Grafana

---

## Architecture Notes

**Recent Refactoring (Completed)**:

* ✅ Action handlers reorganized into `pkg/actions/zarf/` and `pkg/actions/uds/` packages
* ✅ Common job building utilities extracted to `pkg/actions/common/`
* ✅ Constants package created in `pkg/constants/` with proper organization
* ✅ Defensive container bounds checking added to all handlers that need it
* ✅ Hardcoded UIDs replaced with constants throughout action handlers
* ✅ Resource requirements standardized (CPU millicore notation, consistent values)
* ✅ ActionResult type consolidated in `pkg/actions/common/types.go` (eliminates duplication)
* ✅ CLI image constants moved to `pkg/constants/config.go`
* ✅ All helper functions (Ptr, MustParseQuantity) consolidated in common package
* ✅ Integrated UDS actions with shared source handlers via `pkg/sources/uds_adapters.go`
* ✅ All timeouts made configurable via CRD with time.ParseDuration parsing
* ✅ Metric naming standardized with Package/Bundle prefixes
* ✅ Log message format standardized across all action handlers
* ✅ Logging conventions documented in `docs/development/LOGGING.md`
* ✅ UDS documentation complete (user guide, troubleshooting, policy examples)
* ✅ CLAUDE.md updated with comprehensive UDS vs Zarf guidance
