# Prometheus Alerts for Forge

This directory contains PrometheusRule CRDs defining alerts for Forge.

## Alert Overview

### Controller Health Alerts

**ForgeControllerDown** (Critical)

- **Condition**: Controller metrics endpoint is unreachable for 5 minutes
- **Impact**: No Forge resources will be reconciled
- **Action**: Check controller pod status, restart if necessary
- **False positives**: Network issues, pod restarts

**ForgeNoActivity** (Warning)

- **Condition**: No new Forges created in 30 minutes, but active count > 0
- **Impact**: Controller may be stuck in a watch loop
- **Action**: Check controller logs, restart controller
- **False positives**: Low activity periods, all Forges already processed

### Error Rate Alerts

**ForgeHighErrorRate** (Warning)

- **Condition**: > 10% of reconciliations failing for 10 minutes
- **Impact**: Some Forges may not create Jobs
- **Action**: Check controller logs for error patterns, review recent Forge changes
- **False positives**: Brief spike in invalid resources

**ForgeCriticalErrorRate** (Critical)

- **Condition**: > 50% of reconciliations failing for 5 minutes
- **Impact**: Controller is effectively broken
- **Action**: Immediate investigation required, consider rollback
- **False positives**: Deployment in progress, API server issues

**ForgeJobCreationFailures** (Warning)

- **Condition**: Job creation errors > 0.1/sec for 10 minutes
- **Impact**: Forges exist but Jobs aren't being created
- **Action**: Check RBAC permissions, API server health, resource quotas
- **False positives**: Namespace quota exhaustion (expected behavior)

### Performance Alerts

**ForgeSlowReconciliation** (Warning)

- **Condition**: p95 reconciliation latency > 5 seconds for 15 minutes
- **Impact**: Slow Job creation, degraded user experience
- **Action**: Check controller resource usage, API server latency, consider scaling
- **False positives**: Large Forge specs, complex validation

### Webhook Alerts

**ForgeWebhookDown** (Warning)

- **Condition**: Webhook metrics endpoint unreachable for 5 minutes
- **Impact**: New Forges cannot be validated (may be rejected by fail-closed policy)
- **Action**: Check webhook pod status, certificate validity
- **False positives**: Webhook deployment rollout

**ForgeWebhookHighRejectionRate** (Info)

- **Condition**: Webhook rejecting > 1 request/second for 10 minutes
- **Impact**: Users may be submitting invalid Forges
- **Action**: Review webhook logs, educate users on validation rules
- **False positives**: Load testing, CI/CD pipeline issues

### Capacity Alerts

**ForgeHighResourceCount** (Info)

- **Condition**: > 1000 active Forges for 10 minutes
- **Impact**: Informational, may need capacity planning
- **Action**: Review trends, consider controller scaling, audit old Forges
- **False positives**: Expected in large clusters

**ForgeVeryHighResourceCount** (Warning)

- **Condition**: > 5000 active Forges for 10 minutes
- **Impact**: Controller may struggle, watch loop overhead
- **Action**: Scale controller, implement namespace sharding, review TTLs
- **False positives**: None, this is genuinely high

## Installation

### Deploy PrometheusRule

```bash
kubectl apply -f forge-alerts.yaml
```

The PrometheusRule CRD requires Prometheus Operator to be installed.

### Verify Alerts Loaded

```bash
# Check PrometheusRule exists
kubectl get prometheusrule -n forge-system

# Check Prometheus has loaded rules
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Open http://localhost:9090/alerts
```

## Alert Routing

Configure AlertManager to route these alerts appropriately:

```yaml
route:
  routes:
  - match:
      component: controller
    receiver: forge-team
    group_by: ['alertname', 'severity']
    group_wait: 30s
    group_interval: 5m
    repeat_interval: 4h

receivers:
- name: forge-team
  slack_configs:
  - api_url: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
    channel: '#forge-alerts'
    title: '{{ range .Alerts }}{{ .Labels.severity }}: {{ .Annotations.summary }}{{ end }}'
    text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

## Silencing Alerts

### During Maintenance

```bash
# Silence controller alerts for 2 hours
amtool silence add \
  --author="SRE Team" \
  --duration=2h \
  --comment="Planned controller upgrade" \
  component=controller
```

### For Known Issues

```bash
# Silence specific alert
amtool silence add \
  --author="SRE Team" \
  --duration=24h \
  --comment="Known issue, ticket #123" \
  alertname=ForgeHighErrorRate
```

## Tuning Alerts

### Adjusting Thresholds

Common adjustments based on your environment:

**For high-traffic clusters:**

```yaml
# Increase error rate threshold
ForgeHighErrorRate:
  expr: ... > 0.20  # Was 0.10

# Increase resource count thresholds
ForgeHighResourceCount:
  expr: ... > 5000  # Was 1000
```

**For low-latency requirements:**

```yaml
# Decrease reconciliation latency threshold
ForgeSlowReconciliation:
  expr: ... > 2  # Was 5 seconds
```

### Adjusting For Duration

**For stable environments:**

```yaml
# Require longer sustained state before alerting
ForgeHighErrorRate:
  for: 30m  # Was 10m
```

**For critical systems:**

```yaml
# Alert faster
ForgeCriticalErrorRate:
  for: 2m  # Was 5m
```

## Runbook Links

Each alert includes a `runbook_url` annotation pointing to troubleshooting documentation.

Create runbooks at:

```text
docs/runbooks/
├── controller-down.md
├── high-error-rate.md
├── critical-error-rate.md
├── slow-reconciliation.md
├── job-creation-failures.md
├── no-activity.md
├── webhook-rejections.md
├── webhook-down.md
├── high-resource-count.md
└── very-high-resource-count.md
```

## Testing Alerts

### Manually Trigger Alerts

```bash
# Stop controller to trigger ForgeControllerDown
kubectl scale deployment forge-controller -n forge-system --replicas=0

# Wait 5 minutes, verify alert fires
# Restore
kubectl scale deployment forge-controller -n forge-system --replicas=1
```

### Alert Rule Testing

Use promtool to validate alert syntax:

```bash
promtool check rules forge-alerts.yaml
```

## Integration with On-Call

Recommended severity levels:

- **Critical**: Page on-call immediately (controller down, >50% error rate)
- **Warning**: Alert during business hours, page if unresolved after 2 hours
- **Info**: Log to channel, review during weekly ops meeting

Example PagerDuty routing:

```yaml
- match:
    severity: critical
  receiver: pagerduty-critical
  continue: true

- match:
    severity: warning
  receiver: pagerduty-warning
  continue: true

- match:
    severity: info
  receiver: slack-info
```

## References

- [Prometheus Metrics](../../../pkg/metrics/metrics.go)
- [Grafana Dashboard](../../grafana/forge-dashboard.json)
- [Production Checklist](../../../docs/PRODUCTION_CHECKLIST.md)
- [Prometheus Operator Docs](https://prometheus-operator.dev/)
