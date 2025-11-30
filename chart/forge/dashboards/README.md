# Forge Grafana Dashboard

This directory contains Grafana dashboards for monitoring Forge operations.

## forge-dashboard.json

The main Forge Controller dashboard provides visibility into:

- Active Forges
- Job creation rate
- Error rates
- Reconcile duration metrics

### Current Status

**⚠️ Dashboard metrics are not yet implemented in the controller.**

The dashboard is designed for future custom metrics that will be added to the Forge controller. Currently, the controller only exposes standard Go runtime metrics (goroutines, memory, GC stats) via OpenTelemetry.

### Expected Metrics (Not Yet Implemented)

The dashboard expects the following custom metrics:

- `forge_resources_active` - Number of active Forge resources
- `forge_jobs_created_total` - Total count of jobs created by action type
- `forge_reconcile_errors_total` - Total reconciliation errors by type
- `forge_reconcile_duration_seconds` - Histogram of reconciliation durations
- `forge_resources_created_total` - Total resources created

### Available Metrics (Current)

The Forge controller currently exposes these standard Go metrics through OTEL:

- `forge_go_goroutines` - Number of goroutines
- `forge_go_memstats_*` - Go memory statistics
- `forge_go_gc_duration_seconds` - Garbage collection duration
- `forge_process_*` - Process-level metrics (CPU, memory, file descriptors)

You can query these metrics in Prometheus by navigating to the Prometheus UI and searching for metrics prefixed with `forge_`.

## Importing the Dashboard

Even though custom metrics aren't yet available, you can still import the dashboard to preview its layout:

1. Log into Grafana (default: http://localhost:3000 when using Kind setup)
2. Navigate to **Dashboards** → **Import**
3. Click **Upload JSON file**
4. Select `chart/forge/dashboards/forge-dashboard.json`
5. Select your Prometheus data source
6. Click **Import**

All panels will show "No data" until custom metrics are implemented in the controller.

## Future Work

Custom metrics will be added to the controller in a future update. Once implemented, this dashboard will automatically start displaying data without requiring any changes to the dashboard JSON itself.
