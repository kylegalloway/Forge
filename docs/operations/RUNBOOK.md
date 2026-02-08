# Forge Operational Runbook

Operational procedures for running and maintaining Forge in production.

## Deployment Modes

Forge supports two deployment modes. Commands in this runbook are provided for both modes where they differ:

**Cluster-Wide (Default)**:

- Controller watches all namespaces
- Uses ClusterRole permissions
- Resources can be in any namespace
- Suitable for platform teams

**Namespace-Scoped (Restricted)**:

- Controller watches only `forge-system` namespace
- Uses Role permissions (namespace-only)
- All resources must be in `forge-system`
- Suitable for restricted clusters, individual teams

📖 See [NAMESPACE_SCOPED_DEPLOYMENT.md](./NAMESPACE_SCOPED_DEPLOYMENT.md) for deployment mode details.

## Table of Contents

- [Daily Operations](#daily-operations)
- [Monitoring](#monitoring)
- [Incident Response](#incident-response)
- [Maintenance](#maintenance)
- [Disaster Recovery](#disaster-recovery)
- [Scaling](#scaling)

---

## Daily Operations

### Health Checks

Run these checks daily or as part of automated monitoring:

**Cluster-Wide Deployment:**

```bash
# Controller health
kubectl get pods -n forge-system -l app=forge-controller
kubectl get deployment -n forge-system forge-controller

# Webhook health
kubectl get pods -n forge-system -l app=forge-webhook

# CRDs present
kubectl get crd zarfpackagejobs.forge.dev

# Recent errors in logs (last hour)
kubectl logs -n forge-system -l app=forge-controller --since=1h | grep -i error

# Leader election status (HA deployments)
kubectl get lease -n forge-system forge-controller-lock -o yaml
```

Expected output (healthy system):

```text
# Controller pods - should show Running
NAME                                READY   STATUS    RESTARTS   AGE
forge-controller-6d4f8c7b9-xxxxx    1/1     Running   0          5d

# Deployment - should show READY 1/1
NAME               READY   UP-TO-DATE   AVAILABLE   AGE
forge-controller   1/1     1            1           5d

# Webhook pods - should show all Running
NAME                            READY   STATUS    RESTARTS   AGE
forge-webhook-5b7c9d8f-xxxxx    1/1     Running   0          5d
forge-webhook-5b7c9d8f-yyyyy    1/1     Running   0          5d

# CRD - should exist
NAME                          CREATED AT
zarfpackagejobs.forge.dev     2025-12-14T10:00:00Z

# Errors - should be empty or only non-critical
(no output is ideal)

# Leader election - shows current leader
spec:
  holderIdentity: forge-controller-6d4f8c7b9-xxxxx
  leaseDurationSeconds: 15
  renewTime: "2025-12-19T10:15:30.123456Z"
```

**Namespace-Scoped Deployment:**

```bash
# Controller health (same namespace where deployed)
kubectl get pods -n forge-system -l app=forge-controller
kubectl get deployment -n forge-system forge-controller

# Verify watching correct namespace
kubectl logs -n forge-system -l app=forge-controller | grep "Watching namespace"
# Should show: "Watching namespace: forge-system"

# List resources (all in forge-system)
kubectl get zarfpackagejobs,serviceaccounts,secrets -n forge-system

# Recent errors
kubectl logs -n forge-system -l app=forge-controller --since=1h | grep -i error
```

### Metrics to Monitor

**Controller Metrics** (http://controller:8080/metrics):

- `forge_zarf_packages_created_total` - Total packages created
- `forge_zarf_packages_active` - Currently active packages
- `forge_jobs_failed_total` - Failed jobs (should be low)
- `forge_reconcile_errors_total` - Reconciliation errors
- `forge_builds_failed_total` - Failed builds

**Concurrency & Backpressure Metrics** (when concurrency limits are enabled):

- `forge_jobs_concurrent_active` - Currently active jobs (gauge)
- `forge_controller_queued_jobs` - Jobs waiting for capacity (gauge)
- `forge_controller_backpressure_events` - Times jobs were queued due to limits (counter)

**Alert if:**

- Job failure rate > 10% over 15 minutes
- Reconcile errors > 5 per minute
- Controller pod restarts
- Webhook validation failures > 20%
- Queued jobs > 50 for > 5 minutes (backpressure buildup)

### Common Tasks

#### Onboard New Team/User

**Cluster-Wide Deployment:**

1. Create ServiceAccount with appropriate permissions in team namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: team-dev-sa
  namespace: team-dev
  annotations:
    forge.dev/allowed-actions: "Build,Publish"
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

1. Create namespace ResourceQuota (if needed):

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: forge-quota
  namespace: team-dev
spec:
  hard:
    count/zarfpackagejobs.forge.dev: "50"
    count/jobs.batch: "20"
```

1. Document the ServiceAccount capabilities and share with team

**Namespace-Scoped Deployment:**

1. Create ServiceAccount in forge-system namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: team-dev-sa
  namespace: forge-system  # Must be in forge-system
  annotations:
    forge.dev/allowed-actions: "Build,Publish"
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

1. Apply ResourceQuota to forge-system namespace:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: forge-quota
  namespace: forge-system
spec:
  hard:
    count/zarfpackagejobs.forge.dev: "100"
    count/jobs.batch: "50"
```

1. Instruct team that all job resources (ZarfPackageJobs and UDSBundleJobs) must be created in `forge-system` namespace

#### Review Package Activity

**Cluster-Wide Deployment:**

```bash
# List all packages across all namespaces
kubectl get zarfpackagejobs -A

# Packages by phase
kubectl get zarfpackagejobs -A -o json | jq -r '.items[] | select(.status.phase=="Failed") | "\(.metadata.namespace)/\(.metadata.name)"'

# Recent builds
kubectl get zarfpackagejobs -A --sort-by=.metadata.creationTimestamp | tail -10
```

**Namespace-Scoped Deployment:**

```bash
# List all packages (only in forge-system)
kubectl get zarfpackagejobs -n forge-system

# Packages by phase
kubectl get zarfpackagejobs -n forge-system -o json | jq -r '.items[] | select(.status.phase=="Failed") | .metadata.name'

# Recent builds
kubectl get zarfpackagejobs -n forge-system --sort-by=.metadata.creationTimestamp | tail -10
```

#### Clean Up Completed Packages

```bash
# List completed packages older than 7 days
kubectl get zarfpackagejobs -A -o json | jq -r '.items[] | select(.status.phase=="Completed" and (now - (.metadata.creationTimestamp | fromdateiso8601) > 604800)) | "\(.metadata.namespace)/\(.metadata.name)"'

# Delete them (careful!)
# kubectl delete ZarfPackageJob -n <namespace> <name>
```

---

## Monitoring

### Prometheus Queries

**Package Creation Rate:**

```promql
rate(forge_zarf_packages_created_total[5m])
```

**Job Failure Rate:**

```promql
rate(forge_jobs_failed_total[5m]) / rate(forge_jobs_created_total[5m])
```

**Active Packages:**

```promql
forge_zarf_packages_active
```

**Build Duration (p95):**

```promql
histogram_quantile(0.95, rate(forge_action_duration_bucket{action="build"}[5m]))
```

**Reconcile Errors:**

```promql
rate(forge_reconcile_errors_total[5m])
```

### Grafana Dashboards

Key panels to include:

1. **Overview**
   - Total packages (counter)
   - Active packages (gauge)
   - Job success rate (percentage)

2. **Performance**
   - Action duration histogram
   - Reconcile duration
   - Jobs per minute

3. **Errors**
   - Failed jobs by action type
   - Policy violations
   - Reconcile errors

4. **Resources**
   - Controller CPU/memory
   - Job pod resource usage
   - API server request latency

### Alerts

**Critical Alerts:**

1. **Controller Down**

   ```promql
   up{job="forge-controller"} == 0
   ```

   **Action:** Check controller pods, review logs, restart if needed

2. **High Job Failure Rate**

   ```promql
   rate(forge_jobs_failed_total[15m]) / rate(forge_jobs_created_total[15m]) > 0.1
   ```

   **Action:** Review failed job logs, check for common errors

3. **Webhook Down**

   ```promql
   up{job="forge-webhook"} == 0
   ```

   **Action:** Check webhook pods, verify certificates

**Warning Alerts:**

1. **Elevated Reconcile Errors**

   ```promql
   rate(forge_reconcile_errors_total[5m]) > 5
   ```

   **Action:** Review controller logs

2. **High Policy Violation Rate**

   ```promql
   rate(forge_webhook_validations_total{status="denied"}[15m]) > 10
   ```

   **Action:** May indicate misconfigured ServiceAccounts or user education needed

---

## Incident Response

### Controller Crash Loop

**Symptoms:**

- Controller pod continuously restarting
- Metrics unavailable
- ZarfPackageJobs not being reconciled

**Investigation:**

```bash
# Check pod status
kubectl describe pod -n forge-system -l app=forge-controller

# Review logs from previous runs
kubectl logs -n forge-system -l app=forge-controller --previous --tail=100

# Check events
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

**Common Causes & Solutions:**

1. **RBAC Issue**
   - Verify ClusterRole and ClusterRoleBinding exist
   - Apply: `kubectl apply -f config/rbac/rbac.yaml`

2. **Resource Limits**
   - Increase memory/CPU in deployment
   - Check metrics for OOM kills

3. **Corrupted State**
   - Delete and recreate controller pod
   - Check for malformed ZarfPackageJobs

### Mass Job Failures

**Symptoms:**

- Multiple jobs failing simultaneously
- Specific error pattern across jobs

**Investigation:**

```bash
# Find failed jobs
kubectl get jobs -A -o json | jq -r '.items[] | select(.status.failed > 0) | "\(.metadata.namespace)/\(.metadata.name)"'

# Check common errors
kubectl get jobs -A -o json | jq -r '.items[] | select(.status.failed > 0) | .status.conditions[].message' | sort | uniq -c
```

**Common Causes:**

1. **Registry/Repository Down**
   - Verify external service availability
   - Check network policies

2. **Credential Expiration**
   - Rotate credentials in Secrets
   - Verify Secret references

3. **Resource Exhaustion**
   - Check cluster node capacity
   - Review resource quotas

### Webhook Validation Failures

**Symptoms:**

- All ZarfPackageJob creations failing
- Webhook timeout errors

**Quick Fix:**

```bash
# Temporarily disable webhook (EMERGENCY ONLY)
kubectl delete validatingwebhookconfiguration forge-webhook

# Fix webhook
kubectl get pods -n forge-system -l app=forge-webhook
kubectl logs -n forge-system -l app=forge-webhook

# Re-enable webhook
kubectl apply -f webhook/deploy/webhook-configuration.yaml
```

---

## Maintenance

### Upgrading Forge

**Pre-upgrade Checklist:**

1. [ ] Review CHANGELOG for breaking changes
2. [ ] Backup CRDs and important ZarfPackageJobs
3. [ ] Test upgrade in staging environment
4. [ ] Schedule maintenance window
5. [ ] Notify users

**Upgrade Procedure:**

```bash
# 1. Backup current state
kubectl get zarfpackagejobs -A -o yaml > zarfpackagejobs-backup.yaml
kubectl get crd zarfpackagejobs.forge.dev -o yaml > crd-backup.yaml

# 2. Update CRDs (if changed)
kubectl apply -f config/crd/

# 3. Update controller
kubectl set image deployment/forge-controller -n forge-system \
  controller=forge-controller:NEW_VERSION

# 4. Monitor rollout
kubectl rollout status deployment/forge-controller -n forge-system

# 5. Update webhook (if changed)
kubectl set image deployment/forge-webhook -n forge-system \
  webhook=forge-webhook:NEW_VERSION

# 6. Verify health
kubectl get pods -n forge-system
kubectl logs -n forge-system -l app=forge-controller --tail=50
```

**Rollback Procedure:**

```bash
# Rollback controller
kubectl rollout undo deployment/forge-controller -n forge-system

# Rollback webhook
kubectl rollout undo deployment/forge-webhook -n forge-system

# Verify
kubectl rollout status deployment/forge-controller -n forge-system
```

### Certificate Rotation

Certificates are managed by cert-manager and auto-renew. If manual rotation needed:

```bash
# Delete current certificate
kubectl delete certificate -n forge-system forge-webhook-cert

# Cert-manager will recreate
kubectl get certificate -n forge-system -w

# Restart webhook to pick up new cert
kubectl rollout restart deployment/forge-webhook -n forge-system
```

### Log Rotation

Controller and webhook logs are managed by Kubernetes. For external log aggregation:

1. Configure log shipping to your SIEM/log aggregator
2. Retention: 30 days minimum recommended
3. Alert on log volume anomalies

### Credential Rotation

**Git Credentials:**

```bash
# Update Secret
kubectl create secret generic github-token \
  --from-literal=token=NEW_TOKEN \
  --dry-run=client -o yaml | kubectl apply -f -

# No pod restart needed - next job will use new credentials
```

**S3 Credentials:**

```bash
# Update Secret
kubectl create secret generic s3-creds \
  --from-literal=access-key-id=NEW_KEY \
  --from-literal=secret-access-key=NEW_SECRET \
  --dry-run=client -o yaml | kubectl apply -f -
```

**OCI Registry Credentials:**

```bash
# Update dockerconfigjson secret
kubectl create secret docker-registry oci-creds \
  --docker-server=ghcr.io \
  --docker-username=user \
  --docker-password=NEW_TOKEN \
  --dry-run=client -o yaml | kubectl apply -f -
```

---

## Disaster Recovery

### Backup Strategy

**What to Backup:**

1. **CRDs** (critical)

   ```bash
   kubectl get crd zarfpackagejobs.forge.dev -o yaml > zarfpackagejobs-crd.yaml
   ```

2. **ServiceAccounts with policies** (critical)

   ```bash
   kubectl get sa -A -o yaml | grep -A 50 "forge.dev/" > serviceaccounts-backup.yaml
   ```

3. **Active ZarfPackageJobs** (important)

   ```bash
   kubectl get zarfpackagejobs -A -o yaml > zarfpackagejobs-backup.yaml
   ```

4. **Configuration** (important)
   - Controller deployment
   - Webhook deployment
   - RBAC configs
   - Network policies

**Backup Frequency:**

- CRDs: On each upgrade
- ServiceAccounts: Daily
- ZarfPackageJobs: Continuous (if needed)
- Configuration: On each change

### Recovery Procedures

**Complete Cluster Loss:**

```bash
# 1. Install CRDs
kubectl apply -f zarfpackagejobs-crd.yaml

# 2. Install Forge
kubectl create namespace forge-system
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/manager/deployment.yaml

# 3. Restore ServiceAccounts
kubectl apply -f serviceaccounts-backup.yaml

# 4. Restore ZarfPackageJobs (if needed)
kubectl apply -f zarfpackagejobs-backup.yaml

# 5. Verify
kubectl get pods -n forge-system
kubectl get zarfpackagejobs -A
```

**Controller Data Loss:**

Forge is stateless - all state is in Kubernetes. Recovery:

1. Redeploy controller
2. Controller will reconcile existing ZarfPackageJobs

**RTO/RPO:**

- RTO (Recovery Time Objective): < 15 minutes
- RPO (Recovery Point Objective): 0 (no data loss, all state in etcd)

---

## Scaling

### Horizontal Scaling

**When to scale:**

- More than 100 active ZarfPackageJobs
- Reconciliation lag > 5 seconds
- CPU usage consistently > 70%

**How to scale:**

Leader election is enabled by default. Increasing `replicaCount` automatically adds PodDisruptionBudget and pod anti-affinity:

```bash
helm upgrade forge forge/forge \
  --set controller.replicaCount=3 \
  --namespace forge-system
```

This gives you:

- Leader election for controller coordination
- PodDisruptionBudget (minAvailable: 1)
- Pod anti-affinity across nodes
- Configurable work queue parallelism (`controller.workers`)

**Verify HA status:**

```bash
# Check leader election lease
kubectl get lease -n forge-system forge-controller-lock -o yaml

# Check PDB
kubectl get pdb -n forge-system

# Check pod spread
kubectl get pods -n forge-system -l app=forge-controller -o wide
```

**Tune concurrency limits** to prevent cluster overload:

```bash
helm upgrade forge forge/forge \
  --set controller.concurrency.maxJobsPerNamespace=5 \
  --set controller.concurrency.maxJobsGlobal=20 \
  --namespace forge-system
```

### Vertical Scaling

**Increase resources:**

```yaml
resources:
  requests:
    cpu: 500m      # from 250m
    memory: 512Mi  # from 256Mi
  limits:
    cpu: 2000m     # from 1000m
    memory: 1Gi    # from 512Mi
```

**Apply:**

```bash
kubectl patch deployment forge-controller -n forge-system --patch '
spec:
  template:
    spec:
      containers:
      - name: controller

        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 1Gi
'
```

### Performance Tuning

**Worker Goroutines:**

- Default: 2 workers processing the reconciliation queue
- Increase for high-throughput environments: `--set controller.workers=4`
- More workers = faster reconciliation but higher API server load

**Concurrency Limits:**

- Per-namespace: `controller.concurrency.maxJobsPerNamespace` (0 = unlimited)
- Global: `controller.concurrency.maxJobsGlobal` (0 = unlimited)
- Queued jobs are automatically dispatched when capacity frees up

**Leader Election Timing:**

- Lease duration: `leaderElection.leaseDuration` (default: 15s)
- Renew deadline: `leaderElection.renewDeadline` (default: 10s)
- Retry period: `leaderElection.retryPeriod` (default: 2s)
- Faster values = quicker failover but more API server traffic

**Job Cleanup:**

- TTL: 1 hour (3600s)
- Adjust in code if shorter/longer retention needed

**Resource Quotas:**

- Limit ZarfPackageJobs per namespace
- Limit concurrent Jobs per namespace
- Example in `config/namespace-templates/`

---

## On-Call Procedures

### Pager Alerts

**P0 - Critical (Immediate Response):**

- Controller down
- Webhook down
- Job failure rate > 50%

**P1 - High (15 minute response):**

- Job failure rate > 20%
- High reconcile errors
- Certificate expiration < 7 days

**P2 - Medium (1 hour response):**

- Elevated error rates
- Performance degradation
- Resource constraints

### First Steps for Any Alert

1. Check current system status:

   ```bash
   kubectl get pods -n forge-system
   kubectl get zarfpackagejobs -A | grep -v Completed
   ```

2. Review recent logs:

   ```bash
   kubectl logs -n forge-system -l app=forge-controller --tail=100
   ```

3. Check metrics dashboard

4. Follow specific runbook section based on alert type

### Escalation

**Level 1:** On-call engineer (you)
**Level 2:** Forge maintainers
**Level 3:** Platform team lead

**When to escalate:**

- Issue not resolved in 30 minutes
- Requires code changes
- Impacts multiple teams/critical workloads

---

*Last Updated: 2025-11-20*
*Version: 1.0.0*
