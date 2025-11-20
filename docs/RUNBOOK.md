# Forge Operational Runbook

Operational procedures for running and maintaining Forge in production.

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

```bash
# Controller health
kubectl get pods -n forge-system -l app=forge-controller
kubectl get deployment -n forge-system forge-controller

# Webhook health
kubectl get pods -n forge-system -l app=forge-webhook

# CRDs present
kubectl get crd zarfpackages.zarf.dev
kubectl get crd udsbundles.uds.io

# Recent errors in logs (last hour)
kubectl logs -n forge-system -l app=forge-controller --since=1h | grep -i error
```

### Metrics to Monitor

**Controller Metrics** (http://controller:8080/metrics):

- `forge_zarf_packages_created_total` - Total packages created
- `forge_zarf_packages_active` - Currently active packages
- `forge_jobs_failed_total` - Failed jobs (should be low)
- `forge_reconcile_errors_total` - Reconciliation errors
- `forge_builds_failed_total` - Failed builds

**Alert if:**
- Job failure rate > 10% over 15 minutes
- Reconcile errors > 5 per minute
- Controller pod restarts
- Webhook validation failures > 20%

### Common Tasks

#### Onboard New Team/User

1. Create ServiceAccount with appropriate permissions:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: team-dev-sa
  namespace: team-dev
  annotations:
    forge.zarf.dev/allowed-actions: "Build,Publish"
    forge.zarf.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.zarf.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

2. Create namespace ResourceQuota (if needed):
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: forge-quota
  namespace: team-dev
spec:
  hard:
    count/zarfpackages.zarf.dev: "50"
    count/jobs.batch: "20"
```

3. Document the ServiceAccount capabilities and share with team

#### Review Package Activity

```bash
# List all packages
kubectl get zarfpackages -A

# Packages by phase
kubectl get zarfpackages -A -o json | jq -r '.items[] | select(.status.phase=="Failed") | "\(.metadata.namespace)/\(.metadata.name)"'

# Recent builds
kubectl get zarfpackages -A --sort-by=.metadata.creationTimestamp | tail -10
```

#### Clean Up Completed Packages

```bash
# List completed packages older than 7 days
kubectl get zarfpackages -A -o json | jq -r '.items[] | select(.status.phase=="Completed" and (now - (.metadata.creationTimestamp | fromdateiso8601) > 604800)) | "\(.metadata.namespace)/\(.metadata.name)"'

# Delete them (careful!)
# kubectl delete zarfpackage -n <namespace> <name>
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
- ZarfPackages not being reconciled

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
   - Check for malformed ZarfPackages

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
- All ZarfPackage creations failing
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
2. [ ] Backup CRDs and important ZarfPackages
3. [ ] Test upgrade in staging environment
4. [ ] Schedule maintenance window
5. [ ] Notify users

**Upgrade Procedure:**

```bash
# 1. Backup current state
kubectl get zarfpackages -A -o yaml > zarfpackages-backup.yaml
kubectl get crd zarfpackages.zarf.dev -o yaml > crd-backup.yaml

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
   kubectl get crd zarfpackages.zarf.dev -o yaml > zarfpackages-crd.yaml
   kubectl get crd udsbundles.uds.io -o yaml > udsbundles-crd.yaml
   ```

2. **ServiceAccounts with policies** (critical)
   ```bash
   kubectl get sa -A -o yaml | grep -A 50 "forge.zarf.dev/" > serviceaccounts-backup.yaml
   ```

3. **Active ZarfPackages** (important)
   ```bash
   kubectl get zarfpackages -A -o yaml > zarfpackages-backup.yaml
   ```

4. **Configuration** (important)
   - Controller deployment
   - Webhook deployment
   - RBAC configs
   - Network policies

**Backup Frequency:**
- CRDs: On each upgrade
- ServiceAccounts: Daily
- ZarfPackages: Continuous (if needed)
- Configuration: On each change

### Recovery Procedures

**Complete Cluster Loss:**

```bash
# 1. Install CRDs
kubectl apply -f zarfpackages-crd.yaml
kubectl apply -f udsbundles-crd.yaml

# 2. Install Forge
kubectl create namespace forge-system
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/manager/deployment.yaml

# 3. Restore ServiceAccounts
kubectl apply -f serviceaccounts-backup.yaml

# 4. Restore ZarfPackages (if needed)
kubectl apply -f zarfpackages-backup.yaml

# 5. Verify
kubectl get pods -n forge-system
kubectl get zarfpackages -A
```

**Controller Data Loss:**

Forge is stateless - all state is in Kubernetes. Recovery:
1. Redeploy controller
2. Controller will reconcile existing ZarfPackages

**RTO/RPO:**
- RTO (Recovery Time Objective): < 15 minutes
- RPO (Recovery Point Objective): 0 (no data loss, all state in etcd)

---

## Scaling

### Horizontal Scaling

**When to scale:**
- More than 100 active ZarfPackages
- Reconciliation lag > 5 seconds
- CPU usage consistently > 70%

**How to scale:**

Currently single replica (no leader election yet). For HA:

```yaml
# Phase 4: Production Hardening required first
spec:
  replicas: 3
  # Add leader election
  # Add anti-affinity
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

**Reconciliation Rate:**
- Default: Every 10 minutes for each ZarfPackage
- Immediate on spec changes
- Adjust via controller flags if needed

**Job Cleanup:**
- TTL: 1 hour (3600s)
- Adjust in code if shorter/longer retention needed

**Resource Quotas:**
- Limit ZarfPackages per namespace
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
   kubectl get zarfpackages -A | grep -v Completed
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
