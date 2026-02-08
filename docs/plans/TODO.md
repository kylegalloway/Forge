# TODO

## Manually added

* **kubectl-forge plugin not working** - Investigate and fix the kubectl forge CLI plugin (unknown root cause)

## Production Readiness (High Priority)

### Critical Operational Concerns

* **Secrets management hardening** - Git/registry credentials stored as Kubernetes Secrets visible to namespace admins:
  * Integration with external secret managers (Vault, AWS Secrets Manager, GCP Secret Manager)
  * External Secrets Operator support for dynamic credential injection
  * Credential rotation automation
  * Audit logging for secret access
  * Support for Workload Identity/IRSA instead of static credentials
  * Encrypted secret storage at rest

### Security Hardening

* **RBAC least privilege audit** - Current ClusterRole permissions may be overly broad:
  * Audit all verbs in controller/webhook ClusterRoles
  * Remove unused permissions (if any)
  * Add RBAC escalation prevention (restrict access to ServiceAccount creation)
  * Document permission requirements per action type
  * Support for custom RBAC roles per tenant

### Scalability & Performance

* **Job queue management and throttling** - No limit on concurrent jobs, can overwhelm cluster scheduler:
  * Configurable max concurrent jobs per namespace
  * Global cluster-wide job quota
  * Priority queue for jobs (high/normal/low priority)
  * Fair scheduling across teams/namespaces
  * Backpressure when queue is full
  * Job queueing metrics and visibility

* **Controller performance optimization** - Single controller handles all namespaces, potential bottleneck at scale:
  * Leader election for multi-replica active-active mode
  * Namespace sharding across multiple controller instances
  * Watch filtering to reduce API server load
  * Reconciliation batching for high-churn scenarios
  * Cache tuning and optimization
  * Performance benchmarking suite (100+ concurrent jobs)

### GitOps & Integration

* **CI/CD pipeline integration** - No native integrations with popular CI/CD systems:
  * GitHub Actions workflow examples
  * GitLab CI/CD templates
  * Jenkins shared library
  * Tekton Task definitions
  * Argo Workflows templates
  * Webhook triggers for job creation (Git push → auto-build)

### Developer Experience

* **Job artifact retrieval** - Partial (CLI complete via kubectl-forge, metadata pending):
  * ✅ `kubectl forge download <job-name>` CLI command
  * ❌ Artifact metadata API (size, checksum, build time)

* **Interactive job debugging** - Partial (CLI complete via kubectl-forge, advanced features pending):
  * ✅ `kubectl forge debug <job-name>` to exec into failed pod
  * ✅ Create debug pods with workspace access (`--copy-workspace`)
  * ✅ Preserve debug pods for inspection (`--preserve-pod`)
  * ✅ Debug mode flag in CRD spec (`spec.debugMode`, `spec.debugActions`)
  * ❌ Live log streaming in CLI

## Future Enhancements (Long-term)

### Medium Value

* **Additional destination types** - Artifactory and Nexus registry support (currently supports OCI, S3, Local)
* **Multi-architecture bundle builds** - Cross-platform builds for arm64/amd64 in single job execution
* **Priority classes** - Priority classes configurable per-job for pod scheduling priority

### Operational Excellence

* **Disaster recovery and backup** - No mechanism to backup/restore job history and artifacts:
  * Automated backup of ZarfPackageJob/UDSBundleJob resources to S3/Git
  * Artifact backup to object storage (deduplicated)
  * Point-in-time recovery for job definitions
  * Cross-region disaster recovery replication
  * Backup encryption and retention policies
  * Restore testing automation

* **Cost optimization and resource efficiency** - No visibility into job costs or resource waste:
  * Cost attribution labels on Jobs/Pods (team, project, cost-center)
  * Spot/preemptible instance support for non-critical builds
  * Automatic job scheduling during off-peak hours
  * Resource usage recommendations (right-size jobs based on history)
  * Idle resource detection (warn if cluster underutilized)
  * FinOps dashboard (cost per job, cost trends, budget alerts)

* **Chaos engineering and resilience testing** - No automated testing of failure scenarios:
  * Chaos mesh integration (kill random job pods, network failures)
  * Automated fault injection tests
  * Recovery time measurement
  * Degradation testing (webhook down, controller down, API server slow)
  * Load testing framework (simulate 100+ concurrent jobs)
  * Resilience scorecard

### Testing & Quality

* **Integration test suite expansion** - Limited end-to-end testing coverage:
  * ~~Full lifecycle tests (Build → Publish → Deploy in real cluster)~~
  * ~~Multi-action job testing~~
  * ~~Policy enforcement integration tests~~
  * ~~Webhook validation test coverage~~
  * Upgrade/downgrade testing (n-1 version compatibility)
  * Cross-platform testing (different Kubernetes versions)

* **Test environment automation** - Manual KIND cluster setup for testing:
  * Automated test environment provisioning (KIND/k3d)
  * Test data generators (realistic ZarfPackageJobs)
  * Snapshot testing for CRD schemas
  * Golden file testing for generated manifests
  * Test cleanup automation
  * Parallel test execution

### Low Priority (Polish)

* **Webhook TLS certificate rotation** - Automated cert-manager integration for seamless certificate rotation without webhook downtime (partially covered in Production Readiness - secrets management)
* **Structured event streaming** - CloudEvents format for integration with external systems:
  * Emit CloudEvents for job state transitions
  * Kafka/NATS streaming integration
  * Event replay capability
  * Event schema registry
  * Event-driven automation (trigger downstream systems)
* **Enhanced user notifications** - No built-in notification system for job completion:
  * Slack/Microsoft Teams integration
  * Email notifications
  * Webhook callbacks on job state changes
  * Customizable notification templates
  * Notification preferences per team/user
* **Web UI dashboard** - All operations require kubectl/API calls:
  * Browser-based job creation wizard
  * Real-time job status dashboard
  * Log viewer with search/filtering
  * Artifact browser and download
  * Policy management UI
  * RBAC permission visualizer
