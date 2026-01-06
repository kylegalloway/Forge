# TODO

## Production Readiness (High Priority)

### Critical Operational Concerns

* **Default resource limits on jobs** - Jobs currently run without resource constraints, risking cluster resource starvation. Need configurable defaults:
  * Memory requests/limits (default: 1Gi request, 4Gi limit)
  * CPU requests/limits (default: 500m request, 2 CPU limit)
  * Ephemeral storage limits for large package builds
  * Per-action overrides (build vs publish vs deploy may have different needs)
  * Node selector and affinity rules for workload isolation
  * Timeout configuration (default: 2 hours, configurable per-job)

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

* **Artifact storage alternatives** - PVC-based storage doesn't scale well across clusters:
  * S3-backed artifact storage (presigned URLs for job access)
  * OCI registry as artifact store (publish intermediate builds)
  * Distributed storage support (Ceph, MinIO, Longhorn)
  * Artifact compression and deduplication
  * Multi-region artifact replication
  * Artifact garbage collection based on LRU policy

### GitOps & Integration

* **ArgoCD integration hardening** - Current integration requires manual ignoreDifferences configuration:
  * ArgoCD Application/ApplicationSet examples in docs
  * Pre-configured sync policies for Forge resources
  * ArgoCD Health Assessment for custom resources (show job status in ArgoCD UI)
  * ArgoCD Notification integration (alert on job failures)
  * Argo Workflows integration (trigger ZarfPackageJobs from workflows)
  * FluxCD reconciliation support

* **Multi-cluster management** - Managing Zarf CLI image across clusters is manual and error-prone:
  * Cluster fleet management for Zarf CLI image distribution
  * Automated image sync CronJob (pull from registry, load to nodes)
  * Multi-cluster deployment via ClusterAPI/vCluster
  * Cross-cluster artifact sharing
  * Centralized policy management across clusters
  * Cluster inventory and version tracking

* **CI/CD pipeline integration** - No native integrations with popular CI/CD systems:
  * GitHub Actions workflow examples
  * GitLab CI/CD templates
  * Jenkins shared library
  * Tekton Task definitions
  * Argo Workflows templates
  * Webhook triggers for job creation (Git push → auto-build)

### Developer Experience

* **Job artifact retrieval** - No easy way to download built packages from completed jobs:
  * `kubectl forge download <job-name>` CLI command
  * HTTP endpoint to stream artifacts from PVC
  * Automatic artifact upload to S3/registry after build
  * Artifact retention policy (keep last N successful builds)
  * Artifact metadata API (size, checksum, build time)
  * Browser-based artifact explorer UI

* **Job templates and reusable workflows** - Users recreate similar jobs repeatedly:
  * ZarfPackageTemplate CRD for reusable job definitions
  * Parameterized templates (substitute values at runtime)
  * Template library/marketplace
  * Inheritance and composition (extend base templates)
  * Template validation and linting
  * Version control for templates

* **Interactive job debugging** - When jobs fail, debugging requires manual kubectl commands:
  * `kubectl forge debug <job-name>` to exec into failed pod
  * Automatic pod preservation on failure (keep failed pods for debugging)
  * Job retry with modifications (change env vars, add debug flags)
  * Live log streaming in CLI
  * Job execution replay (re-run with same inputs)
  * Debug mode flag (skip cleanup, verbose logging)

## Future Enhancements (Long-term)

### Medium Value

* **Additional destination types** - Artifactory and Nexus registry support (currently supports OCI, S3, Local)
* **Multi-architecture bundle builds** - Cross-platform builds for arm64/amd64 in single job execution
* **Enhanced resource scheduling** - Node affinity, tolerations, and priority classes configurable per-job (partially covered in Production Readiness)

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
  * Full lifecycle tests (Build → Publish → Deploy in real cluster)
  * Multi-action job testing
  * Policy enforcement integration tests
  * Webhook validation test coverage
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
