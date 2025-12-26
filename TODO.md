# TODO

## Recently Completed (2025-12-25)

### Testing Infrastructure Overhaul

* ✅ **UDS Controller Test Coverage** - Achieved parity with Zarf controller (894 lines)
  * Added HealthzHandler/ReadyzHandler methods
  * Added 4 comprehensive HTTP response tests
  * Coverage: 712 → 894 lines (99% parity)

* ✅ **YAML Validation Tests** - Fixed and working
  * Updated paths: `.config/*` → `chart/forge/crds/`, `examples/samples/`, `examples/policies/`
  * Now validates 30+ real YAML files
  * Tests passing: `make test-validation` ✅

* ✅ **E2E Test Suite** - Completely rebuilt
  * Created `tests/e2e/` with 3 portable tests (Kind + prod clusters)
  * Test 01: Simple build (Git → package)
  * Test 02: Simple deploy (package → cluster)
  * Test 03: Health check (controller endpoints)
  * Added Makefile targets: `make e2e-test`, `make e2e-test-keep`, `make e2e-test-existing`
  * Automated runner script with color output and timeouts

* ✅ **OTLP Tests** - Documented setup instructions
  * Added comprehensive setup guide in `pkg/telemetry/otel_test.go`
  * Explained OTLP exists but not currently used (Forge uses Prometheus directly)
  * Test can be enabled after collector setup

* ✅ **Job State Bouncing** - Documented expected behavior
  * Created troubleshooting guide: `docs/troubleshooting/JOB_STATE_BOUNCING.md`
  * Explained multi-action workflow status transitions are intentional
  * Provided diagnostic commands for identifying real issues

* ✅ **Examples Organization** - Cleaned and clarified
  * Removed obsolete `examples/test-packages/` (unused, 3 files)
  * Removed obsolete `scripts/test-e2e.sh` (replaced by make targets)
  * Updated `examples/README.md` with "Examples vs Tests" section
  * Created `EXAMPLES_AUDIT.md` documenting all decisions
  * Repository ~1000 lines lighter

---

## Current Status

**No outstanding TODOs or critical issues.**

All testing infrastructure has been reviewed, fixed, or documented. The codebase is in good shape with:

* ✅ All tests passing
* ✅ Test coverage at target levels
* ✅ Clear separation between examples and tests
* ✅ Comprehensive documentation

---

## Future Enhancements (Long-term, Non-critical)

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
