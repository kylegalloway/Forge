# OpenTelemetry Collector for Forge

The OpenTelemetry (OTel) Collector receives telemetry from the Forge controller and exports it to multiple backends. This provides vendor-neutral observability that works with Prometheus, Jaeger, Datadog, New Relic, Honeycomb, and more.

## Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                   Forge Controller                        │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐           │
│  │ OTel Metrics │  │ OTel Traces │  │ Prom Bridge  │           │
│  └──────┬───────┘  └──────┬──────┘  └──────┬───────┘           │
│         │                  │                 │                   │
└─────────┼──────────────────┼─────────────────┼───────────────────┘
          │                  │                 │
          │ OTLP/gRPC        │                 │ /metrics endpoint
          │ :4317            │                 │ :8080
          │                  │                 │
          ▼                  ▼                 │
┌─────────────────────────────────────────────┼───────────────────┐
│           OpenTelemetry Collector            │                   │
│                                              │                   │
│  ┌────────────┐  ┌─────────────┐           │                   │
│  │ OTLP       │  │ Prometheus  │◄──────────┘                   │
│  │ Receiver   │  │ Receiver    │ (scrapes /metrics)            │
│  └─────┬──────┘  └──────┬──────┘                               │
│        │                 │                                       │
│        │    ┌────────────▼───────────┐                          │
│        │    │   Processors           │                          │
│        └───►│ - Batch                │                          │
│             │ - Memory Limiter       │                          │
│             │ - Resource Attributes  │                          │
│             └────────────┬───────────┘                          │
│                          │                                       │
│             ┌────────────▼───────────┐                          │
│             │   Exporters            │                          │
│             │ - Prometheus (metrics) │                          │
│             │ - Jaeger (traces)      │                          │
│             │ - Datadog (optional)   │                          │
│             │ - New Relic (optional) │                          │
│             │ - Logging (debug)      │                          │
│             └────────────┬───────────┘                          │
└──────────────────────────┼──────────────────────────────────────┘
                           │
         ┌─────────────────┼──────────────────┐
         │                 │                  │
         ▼                 ▼                  ▼
   ┌──────────┐     ┌──────────┐      ┌──────────┐
   │Prometheus│     │  Jaeger  │      │ Datadog/ │
   │          │     │          │      │ NewRelic │
   └──────────┘     └──────────┘      └──────────┘
```

## Deployment

### 1. Deploy OTel Collector

```bash
# Deploy ConfigMap and Collector
kubectl apply -f config/otel-collector/otel-collector-config.yaml
kubectl apply -f config/otel-collector/otel-collector-deployment.yaml

# Verify deployment
kubectl get pods -n forge-system -l app=otel-collector
kubectl logs -n forge-system -l app=otel-collector
```

### 2. Update Controller to Send to OTel Collector

The controller is already configured to use OpenTelemetry. To enable OTLP export (instead of just Prometheus bridge), update the controller deployment:

```yaml
# config/manager/deployment.yaml
env:
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "http://otel-collector:4317"
- name: OTEL_EXPORTER_OTLP_INSECURE
  value: "true"
- name: OTEL_SERVICE_NAME
  value: "forge-controller"
```

### 3. Deploy Jaeger (for traces)

```bash
# Quick Jaeger all-in-one deployment
kubectl create -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: forge-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:1.53
        ports:
        - containerPort: 16686  # UI
        - containerPort: 4317   # OTLP gRPC
        - containerPort: 4318   # OTLP HTTP
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-collector
  namespace: forge-system
spec:
  ports:
  - name: otlp-grpc
    port: 4317
  - name: otlp-http
    port: 4318
  selector:
    app: jaeger
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-query
  namespace: forge-system
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 16686
  selector:
    app: jaeger
EOF

# Access Jaeger UI
kubectl port-forward -n forge-system svc/jaeger-query 16686:16686
# Open http://localhost:16686
```

## Configuration

### Receivers

**OTLP Receiver** (primary):

- Receives metrics and traces from controller via gRPC
- Endpoint: `0.0.0.0:4317` (gRPC) and `0.0.0.0:4318` (HTTP)

**Prometheus Receiver** (backup):

- Scrapes controller's `/metrics` endpoint
- Useful if controller uses Prometheus bridge instead of OTLP export

### Processors

**Batch Processor**:

- Batches telemetry to reduce network overhead
- Timeout: 10s, batch size: 1024

**Memory Limiter**:

- Prevents OOM by limiting memory usage
- Limit: 512 MiB, spike limit: 128 MiB

**Resource Processor**:

- Adds service metadata: `service.name`, `service.namespace`, `deployment.environment`

**Attributes Processor**:

- Maps Kubernetes attributes: `k8s.namespace.name`, `k8s.pod.name`

### Exporters

**Prometheus Exporter** (enabled):

- Exposes metrics at `:8889/metrics` for Prometheus to scrape
- Namespace: `forge`
- ServiceMonitor included for Prometheus Operator

**Jaeger Exporter** (enabled):

- Sends traces to Jaeger via OTLP
- Endpoint: `jaeger-collector:4317`

**Prometheus Remote Write** (enabled):

- Sends metrics to Prometheus via remote write API
- Endpoint: `http://prometheus-server:9090/api/v1/write`

**Logging Exporter** (enabled):

- Logs telemetry to stdout for debugging
- Sampling: 1/5 initially, 1/200 thereafter

**Other Exporters** (disabled, uncomment to enable):

- Datadog: Set `DD_API_KEY` environment variable
- New Relic: Set `NEW_RELIC_LICENSE_KEY`
- Honeycomb: Set `HONEYCOMB_API_KEY`

## Hybrid Observability Approach

Forge uses a **hybrid approach** combining:

### 1. In-App Metrics (OTel SDK in Go)

**What**: Controller internals, business logic
**How**: `pkg/telemetry` package with OTel SDK
**Examples**:

- Reconcile loop duration (p50, p95, p99)
- Error types (conversion_error, job_creation_error, status_update_error)
- Job creation attempts vs successes
- Webhook validation reasons

**Why**: Only the controller knows these internal details.

### 2. Kubernetes Metrics (kube-state-metrics)

**What**: Resource counts, object statuses
**How**: Deploy kube-state-metrics (separate deployment)
**Examples**:

- Total Forge resources (by namespace, by phase)
- Total Jobs (by condition: succeeded, failed, active)
- Pod counts, container restarts
- Resource quota usage

**Why**: Free metrics, no code changes, works for any CRD.

### 3. OTel Collector (This Component)

**What**: Aggregation, routing, transformation
**How**: Receives from #1 and #2, exports to backends
**Examples**:

- Route metrics to Prometheus, traces to Jaeger
- Add common labels (environment, cluster, region)
- Batch and compress telemetry
- Filter sensitive data

**Why**: Vendor-neutral, flexible, keeps controller simple.

## Enabling kube-state-metrics

Deploy kube-state-metrics to get Kubernetes resource metrics:

```bash
# Add Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-state-metrics
helm install kube-state-metrics prometheus-community/kube-state-metrics \
  --namespace forge-system \
  --set customResourceState.enabled=true \
  --set customResourceState.config.spec.resources[0].groupVersionKind.group=forge.io \
  --set customResourceState.config.spec.resources[0].groupVersionKind.version=v1alpha1 \
  --set customResourceState.config.spec.resources[0].groupVersionKind.kind=Forge \
  --set customResourceState.config.spec.resources[0].metrics[0].name=forge_info \
  --set customResourceState.config.spec.resources[0].metrics[0].help="Information about Forge" \
  --set customResourceState.config.spec.resources[0].metrics[0].each.type=Info \
  --set customResourceState.config.spec.resources[0].metrics[0].each.info.labelsFromPath.namespace=[metadata,namespace] \
  --set customResourceState.config.spec.resources[0].metrics[0].each.info.labelsFromPath.name=[metadata,name]
```

Or deploy manually:

```bash
kubectl apply -f https://github.com/kubernetes/kube-state-metrics/releases/download/v2.10.1/kube-state-metrics-deployment.yaml
```

kube-state-metrics will expose metrics like:

- `kube_forge_created` - Total Forges
- `kube_forge_status_phase{phase="JobCreated"}` - Forges by phase
- `kube_job_status_succeeded` - Jobs that succeeded
- `kube_job_status_failed` - Jobs that failed

## Backends

### Prometheus (Metrics)

The OTel Collector exposes a Prometheus endpoint at `:8889/metrics`. Configure Prometheus to scrape it:

```yaml
# prometheus.yaml
scrape_configs:
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['otel-collector:8889']
```

Or use the included ServiceMonitor (Prometheus Operator):

```bash
kubectl apply -f config/otel-collector/otel-collector-deployment.yaml
# ServiceMonitor is included in the deployment manifest
```

**Metrics Available**:

- `forge_resources_created` - Counter
- `forge_resources_active` - Gauge
- `forge_jobs_created` - Counter (by namespace, forge)
- `forge_jobs_completed` - Counter
- `forge_jobs_failed` - Counter
- `forge_job_duration_bucket` - Histogram
- `forge_reconcile_errors` - Counter (by error_type)
- `forge_reconcile_duration_bucket` - Histogram

### Jaeger (Traces)

Jaeger receives traces from the OTel Collector and provides a UI for exploring them.

**Trace Structure**:

```text
reconcile_forge (root span)
  └── create_job (child span)
```

**Attributes**:

- `forge.namespace` - Namespace
- `forge.name` - Forge name
- `job.name` - Created Job name
- `error` - Error message (if failed)
- `error.type` - Error type (conversion_error, job_creation_error, etc.)

**Events**:

- `job_created` - When Job is successfully created

**Access Jaeger UI**:

```bash
kubectl port-forward -n forge-system svc/jaeger-query 16686:16686
# Open http://localhost:16686
```

### Datadog (Metrics + Traces)

Enable Datadog exporter in `otel-collector-config.yaml`:

```yaml
exporters:
  otlp/datadog:
    endpoint: https://api.datadoghq.com
    headers:
      DD-API-KEY: ${DD_API_KEY}

service:
  pipelines:
    metrics:
      exporters: [otlp/datadog]
    traces:
      exporters: [otlp/datadog]
```

Set API key as environment variable:

```yaml
# otel-collector-deployment.yaml
env:
- name: DD_API_KEY
  valueFrom:
    secretKeyRef:
      name: datadog-secret
      key: api-key
```

### New Relic (Metrics + Traces)

Enable New Relic exporter:

```yaml
exporters:
  otlp/newrelic:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

### Honeycomb (Traces)

Enable Honeycomb exporter:

```yaml
exporters:
  otlp/honeycomb:
    endpoint: api.honeycomb.io:443
    headers:
      x-honeycomb-team: ${HONEYCOMB_API_KEY}
```

## Monitoring the Collector

### Health Check

```bash
# Check collector health
kubectl exec -n forge-system deploy/otel-collector -- \
  curl http://localhost:13133

# Should return HTTP 200
```

### Metrics

The collector exports its own metrics at `:8888/metrics`:

```bash
kubectl port-forward -n forge-system svc/otel-collector 8888:8888
curl http://localhost:8888/metrics
```

**Collector Metrics**:

- `otelcol_receiver_accepted_spans` - Traces received
- `otelcol_receiver_accepted_metric_points` - Metrics received
- `otelcol_exporter_sent_spans` - Traces exported
- `otelcol_exporter_sent_metric_points` - Metrics exported
- `otelcol_processor_batch_batch_send_size` - Batch sizes

### Logs

```bash
# View collector logs
kubectl logs -n forge-system -l app=otel-collector -f

# Should see lines like:
# 2025-01-19T00:00:00.000Z info MetricsExporter {"#metrics": 42}
# 2025-01-19T00:00:00.000Z info TracesExporter {"#spans": 15}
```

### zpages (Debug UI)

The collector includes zpages for debugging:

```bash
kubectl port-forward -n forge-system svc/otel-collector 55679:55679
# Open http://localhost:55679/debug/tracez
```

## Troubleshooting

### Controller not sending telemetry

**Symptom**: No metrics/traces in collector logs

**Check**:

```bash
# Verify controller can reach collector
kubectl exec -n forge-system deploy/forge-controller -- \
  nc -zv otel-collector 4317

# Check controller logs for OTLP errors
kubectl logs -n forge-system -l app=forge-controller | grep -i otlp
```

**Fix**: Ensure `OTEL_EXPORTER_OTLP_ENDPOINT` is set in controller deployment.

### Collector OOM

**Symptom**: Collector pod restarting, OOMKilled events

**Fix**: Increase memory limit or adjust `memory_limiter`:

```yaml
# otel-collector-config.yaml
processors:
  memory_limiter:
    limit_mib: 1024  # Increase from 512
```

### Prometheus not scraping

**Symptom**: Metrics not appearing in Prometheus

**Check**:

```bash
# Verify Prometheus can reach collector
kubectl exec -n monitoring deploy/prometheus-server -- \
  curl http://otel-collector.forge-system:8889/metrics

# Check ServiceMonitor
kubectl get servicemonitor -n forge-system otel-collector
```

### Jaeger not receiving traces

**Symptom**: No traces in Jaeger UI

**Check**:

```bash
# Verify collector can reach Jaeger
kubectl exec -n forge-system deploy/otel-collector -- \
  nc -zv jaeger-collector 4317

# Check collector logs
kubectl logs -n forge-system -l app=otel-collector | grep -i jaeger
```

**Fix**: Verify Jaeger endpoint in `otel-collector-config.yaml`.

## Performance Tuning

### High Throughput

For >1000 Forges/min:

```yaml
# otel-collector-config.yaml
processors:
  batch:
    timeout: 5s           # Reduce from 10s
    send_batch_size: 2048 # Increase from 1024

# otel-collector-deployment.yaml
resources:
  requests:
    cpu: 500m     # Increase from 200m
    memory: 512Mi # Increase from 256Mi
  limits:
    cpu: 2000m    # Increase from 1000m
    memory: 1Gi   # Increase from 512Mi
```

### Low Latency

For <100ms trace latency:

```yaml
processors:
  batch:
    timeout: 1s          # Reduce batching delay
    send_batch_size: 256 # Reduce batch size
```

### Cost Optimization

To reduce backend costs:

```yaml
# Sample traces (only send 10%)
processors:
  probabilistic_sampler:
    sampling_percentage: 10

# Filter low-value metrics
processors:
  filter/drop_noisy_metrics:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          - .*_bucket  # Drop histogram buckets
```

## Migration from Prometheus-only

If you're currently using Prometheus directly:

1. **Deploy OTel Collector** (this guide)
2. **Update Prometheus scrape config** to scrape OTel Collector instead of controller
3. **No controller changes needed** - Prometheus bridge still works
4. **Optional**: Enable OTLP export from controller for better performance

**Before**:

```text
Controller :8080/metrics → Prometheus
```

**After**:

```text
Controller :8080/metrics (Prometheus bridge) → OTel Collector :8889/metrics → Prometheus
Controller → OTLP :4317 → OTel Collector → Prometheus + Jaeger
```

## References

- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [OTel Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Prometheus Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusexporter)
- [Jaeger Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/jaegerexporter)
- [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)
