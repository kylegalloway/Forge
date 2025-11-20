# Grafana Dashboard for Forge

This directory contains the Grafana dashboard for monitoring Forge controller.

## Dashboard Overview

The dashboard provides real-time visibility into:

- **Active Forges**: Current number of active Forge resources
- **Job Creation Rate**: Jobs created per minute (aggregated across all Forges)
- **Error Rate**: Percentage of reconciliations that failed
- **Job Creation by Forge**: Per-Forge job creation rates
- **Reconcile Errors by Type**: Breakdown of errors (conversion, job creation, status update)
- **Reconcile Duration**: p50 and p95 latency of reconciliation loops

## Installation

### Option 1: Import via Grafana UI

1. Open Grafana
2. Navigate to Dashboards → Import
3. Upload `forge-dashboard.json`
4. Select your Prometheus data source
5. Click Import

### Option 2: Deploy as ConfigMap (GitOps)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: forge-grafana-dashboard
  namespace: monitoring
  labels:
    grafana_dashboard: "1"
data:
  forge-controller.json: |
    <paste contents of forge-dashboard.json>
```

Grafana will automatically discover and load dashboards with the `grafana_dashboard: "1"` label.

### Option 3: Provision via Grafana Sidecar

If using the Grafana Helm chart with sidecar dashboards:

```bash
kubectl create configmap forge-dashboard \
  --from-file=forge-dashboard.json \
  --namespace monitoring \
  --dry-run=client -o yaml | \
  kubectl label --local -f - grafana_dashboard=1 -o yaml | \
  kubectl apply -f -
```

## Panels Explained

### Active Forges
- **Metric**: `forge_resources_active`
- **Type**: Stat (single value)
- **Shows**: Current number of Forge resources being managed
- **Use**: Capacity planning, understanding load

### Job Creation Rate
- **Metric**: `sum(rate(forge_jobs_created_total[5m])) * 60`
- **Type**: Stat (single value)
- **Shows**: Jobs created per minute (5-minute average)
- **Use**: Traffic monitoring, scaling decisions

### Error Rate
- **Metric**: `sum(rate(forge_reconcile_errors_total[5m])) / sum(rate(forge_resources_created_total[5m]))`
- **Type**: Stat with thresholds
- **Shows**: Percentage of reconciliations that failed
- **Thresholds**:
  - Green: < 5%
  - Yellow: 5-10%
  - Red: > 10%
- **Use**: SLO monitoring, alerting trigger

### Job Creation Rate by Forge
- **Metric**: `rate(forge_jobs_created_total[5m])`
- **Type**: Time series
- **Shows**: Job creation rate for each Forge (by namespace/name)
- **Use**: Identifying hot Forges, troubleshooting specific resources

### Reconcile Errors by Type
- **Metric**: `rate(forge_reconcile_errors_total[5m])`
- **Type**: Stacked time series
- **Shows**: Error rate breakdown by type (conversion_error, job_creation_error, status_update_error)
- **Use**: Targeted troubleshooting, understanding failure modes

### Reconcile Duration
- **Metrics**:
  - `histogram_quantile(0.95, sum(rate(forge_reconcile_duration_seconds_bucket[5m])) by (le))`
  - `histogram_quantile(0.50, sum(rate(forge_reconcile_duration_seconds_bucket[5m])) by (le))`
- **Type**: Time series (bars)
- **Shows**: p50 and p95 reconciliation latency
- **Use**: Performance monitoring, detecting slowdowns

## Customization

### Time Range
Default: Last 1 hour with 10-second refresh
To change: Use the time picker in the top right

### Adding Panels
Common additions:
- Pod resource usage (CPU, memory from cAdvisor)
- API server request rate (from kube-apiserver metrics)
- Namespace-specific views (add `namespace` template variable)

### Alerts
This dashboard doesn't include alerts. Configure alerts via:
- Grafana Alerting (built into panels)
- Prometheus AlertManager (see `../prometheus/alerts/`)

## Troubleshooting

### No Data
- Verify Prometheus data source is configured
- Check ServiceMonitor is deployed: `kubectl get servicemonitor -n forge-system`
- Verify Prometheus is scraping: Check Prometheus UI → Status → Targets
- Confirm controller is exposing metrics: `kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080` then `curl localhost:8080/metrics`

### Missing Metrics
- Controller may not have processed any Forges yet
- Metrics are only created when events occur (counters increment, histograms observe)
- Wait for Forge activity or create a test resource

### Dashboard Shows Zero Values
- Check time range (metrics may not exist in selected window)
- Verify rate() interval (5m) has sufficient data points
- Adjust time range to "Last 5 minutes" for immediate feedback

## References

- [Prometheus Metrics](../../pkg/metrics/metrics.go)
- [ServiceMonitor](../metrics/servicemonitor.yaml)
- [Production Checklist](../../docs/PRODUCTION_CHECKLIST.md#batch-4-observability--monitoring)
