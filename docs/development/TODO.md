# TODO

---

## 📊 Testing Gaps

### Error Path Coverage

**Location**: `pkg/actions/zarf/*_test.go`, `pkg/actions/uds/*_test.go`
**Current state**: Handler validation errors are tested, but K8s API interaction failures are not.
**Missing tests**:
- Job creation failures (K8s API errors)
- Status update failures
- Race conditions in job monitoring
- Multiple completed jobs edge cases

**Action**: Add test cases for controller-level error scenarios and edge cases.

---

## Future Enhancements (Long-term)

### High Value

- **Support adopting existing resources** - Allow deploy actions to take ownership of pre-existing cluster resources instead of only creating new ones
- **Job retry logic with backoff** - Configurable retry attempts and exponential backoff for transient failures (currently uses fixed BackoffLimit)
- **Progress reporting** - Stream real-time progress updates from Jobs back to resource status (percentage complete, current step, etc.)

### Medium Value

- **Additional destination types** - Artifactory and Nexus registry support (currently supports OCI, S3, Local)
- **Multi-architecture bundle builds** - Cross-platform builds for arm64/amd64 in single job execution
- **Enhanced resource scheduling** - Node affinity, tolerations, and priority classes configurable per-job

### Low Priority (Polish)

- **Webhook TLS certificate rotation** - Automated cert-manager integration for seamless certificate rotation without webhook downtime
- **Structured event streaming** - CloudEvents format for integration with external systems
- **Cost attribution labels** - Automatic tagging of Jobs/Pods with cost center metadata for chargeback
