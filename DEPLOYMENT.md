# Forge Deployment Guide

This guide covers deploying Forge to Kubernetes clusters using Helm.

## Overview

Forge can be deployed in two primary scenarios:

1. **Mature Cluster** - Into a cluster with existing observability infrastructure
2. **New Cluster** - With a complete observability stack included

## Prerequisites

- Kubernetes 1.24 or later
- Helm 3.8 or later
- kubectl configured and connected to your cluster
- (Optional) Prometheus Operator CRDs for ServiceMonitor support

## Quick Start

### For Mature Clusters (Existing Monitoring)

If you already have Prometheus, Grafana, and OTEL Collector:

```bash
# Edit the values file with your endpoints
vi chart/forge/values-mature-cluster.yaml

# Install Forge
helm install forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system \
  --create-namespace
```

### For New Clusters (Full Stack)

If you need monitoring tools deployed:

```bash
# Set a strong Grafana password
vi chart/forge/values-new-cluster.yaml

# Install Forge with full observability stack
helm install forge ./chart/forge \
  -f chart/forge/values-new-cluster.yaml \
  --namespace forge-system \
  --create-namespace
```

## Deployment Architectures

### Architecture 1: Mature Cluster Integration

```
┌─────────────────────────────────────────┐
│         Existing Infrastructure         │
│  ┌─────────┐  ┌──────────┐  ┌────────┐ │
│  │Prometheus│  │ Grafana  │  │  OTEL  │ │
│  └────▲────┘  └────▲─────┘  └───▲────┘ │
│       │            │              │      │
└───────┼────────────┼──────────────┼──────┘
        │            │              │
        │         Metrics        Telemetry
        │            │              │
  ┌─────┴────────────┴──────────────┴─────┐
  │      Forge Controller (New)            │
  │  ┌──────────────────────────────┐     │
  │  │ - ServiceMonitor             │     │
  │  │ - PrometheusRule (Alerts)    │     │
  │  │ - Metrics Endpoint           │     │
  │  └──────────────────────────────┘     │
  └────────────────────────────────────────┘
```

**What gets deployed:**
- ✅ Forge Controller
- ✅ ServiceMonitor (connects to existing Prometheus)
- ✅ PrometheusRule (alerts)
- ✅ Metrics Service

**What you configure:**
- OTEL Collector endpoint
- Prometheus URL
- Grafana URL
- ServiceMonitor labels (to match your Prometheus)

### Architecture 2: New Cluster Full Stack

```
┌────────────────────────────────────────────┐
│           New Full Stack                   │
│                                            │
│  ┌──────────────────────────────────────┐ │
│  │         Grafana                      │ │
│  │  - Dashboards                        │ │
│  │  - Data Source: Prometheus           │ │
│  └──────────────▲───────────────────────┘ │
│                 │                          │
│  ┌──────────────┴───────────────────────┐ │
│  │         Prometheus                   │ │
│  │  - ServiceMonitor Discovery          │ │
│  │  - PrometheusRule (Alerts)           │ │
│  │  - Persistent Storage                │ │
│  └──────────────▲───────────────────────┘ │
│                 │                          │
│                 │ Scrape                   │
│                 │                          │
│  ┌──────────────┴───────────────────────┐ │
│  │      OTEL Collector                  │ │
│  │  - OTLP Receiver                     │ │
│  │  - Prometheus Exporter               │ │
│  │  - Batch Processing                  │ │
│  └──────────────▲───────────────────────┘ │
│                 │                          │
│                 │ Telemetry                │
│                 │                          │
│  ┌──────────────┴───────────────────────┐ │
│  │      Forge Controller                │ │
│  │  - Metrics Endpoint                  │ │
│  │  - OTLP Export                       │ │
│  │  - Health Checks                     │ │
│  └──────────────────────────────────────┘ │
└────────────────────────────────────────────┘
```

**What gets deployed:**
- ✅ Forge Controller
- ✅ OpenTelemetry Collector
- ✅ Prometheus (with persistent storage)
- ✅ Grafana (with pre-loaded dashboards)
- ✅ ServiceMonitors
- ✅ PrometheusRules

## Configuration Guide

### Mature Cluster Configuration

Edit `chart/forge/values-mature-cluster.yaml`:

```yaml
observability:
  deployStack: false

  otelCollector:
    enabled: false
    external:
      endpoint: "otel-collector.monitoring.svc.cluster.local:4317"
      tls:
        enabled: true
        insecure: false

  prometheus:
    enabled: false
    external:
      url: "http://prometheus.monitoring.svc.cluster.local:9090"

  grafana:
    enabled: false
    external:
      url: "http://grafana.monitoring.svc.cluster.local"

metrics:
  serviceMonitor:
    enabled: true
    additionalLabels:
      # Match these to your Prometheus selector
      prometheus: kube-prometheus
      release: prometheus-operator
```

### New Cluster Configuration

Edit `chart/forge/values-new-cluster.yaml`:

```yaml
observability:
  deployStack: true

  grafana:
    config:
      # IMPORTANT: Set a strong password!
      adminPassword: "YOUR-STRONG-PASSWORD-HERE"

      # Enable ingress for external access
      ingress:
        enabled: true
        ingressClassName: nginx
        hosts:
          - grafana.yourdomain.com
        tls:
          - secretName: grafana-tls
            hosts:
              - grafana.yourdomain.com

  prometheus:
    config:
      persistence:
        enabled: true
        size: 100Gi
        storageClass: "fast-ssd"  # Your storage class
      retention: 30d

controller:
  replicaCount: 2  # Scale for HA
```

## Installation Commands

### Basic Installation

```bash
helm install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace
```

### With Custom Values

```bash
helm install forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --set controller.replicaCount=3 \
  --namespace forge-system \
  --create-namespace
```

### Dry Run (Preview)

```bash
helm install forge ./chart/forge \
  -f chart/forge/values-new-cluster.yaml \
  --namespace forge-system \
  --dry-run --debug
```

## Verification

### Check Installation

```bash
# List all resources
kubectl get all -n forge-system

# Check controller status
kubectl get deployment -n forge-system forge-controller

# View controller logs
kubectl logs -n forge-system -l app=forge-controller -f

# Check CRD installation
kubectl get crd zarfpackagejobs.forge.dev
```

### Test the Controller

```bash
# Create a test job
kubectl apply -f config/samples/v1alpha1/build-only-git.yaml

# Watch the job
kubectl get zarfpackagejobs -A

# Describe the job
kubectl describe zarfpackagejob <name> -n <namespace>
```

### Verify Observability

```bash
# Check metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
curl http://localhost:8080/metrics

# Access Prometheus (if deployed)
kubectl port-forward -n forge-system svc/prometheus-server 9090:9090

# Access Grafana (if deployed)
kubectl port-forward -n forge-system svc/grafana 3000:80
```

## Upgrading

### Upgrade Release

```bash
helm upgrade forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system
```

### Verify Upgrade

```bash
helm list -n forge-system
kubectl rollout status deployment/forge-controller -n forge-system
```

## Uninstalling

### Remove Release

```bash
helm uninstall forge --namespace forge-system
```

### Clean Up (Optional)

```bash
# Remove namespace
kubectl delete namespace forge-system

# Remove CRDs (will delete all ZarfPackageJob resources!)
kubectl delete crd zarfpackagejobs.forge.dev
```

## Security Hardening

### Enable Network Policies

```bash
helm upgrade forge ./chart/forge \
  --set networkPolicies.enabled=true \
  --namespace forge-system
```

### Pod Security Standards

The chart enforces restricted Pod Security Standards by default. The namespace is labeled:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/audit: restricted
pod-security.kubernetes.io/warn: restricted
```

### RBAC

The controller uses a dedicated ServiceAccount with minimal permissions:
- Read/write ZarfPackageJob resources
- Create Jobs
- Read ServiceAccounts and Secrets (for validation)
- Create Events
- Manage Leases (for leader election)

## Advanced Configuration

### High Availability

```yaml
controller:
  replicaCount: 3
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: forge-controller
          topologyKey: kubernetes.io/hostname
```

### Resource Limits

```yaml
controller:
  resources:
    limits:
      cpu: "4"
      memory: 2Gi
    requests:
      cpu: 1000m
      memory: 1Gi
```

### Custom OTEL Configuration

```yaml
observability:
  otelCollector:
    config:
      exporters:
        # Add custom exporters
        otlp/datadog:
          endpoint: https://api.datadoghq.com
          headers:
            DD-API-KEY: ${DD_API_KEY}
```

## Troubleshooting

### Common Issues

#### Controller Not Starting

```bash
kubectl describe pod -n forge-system -l app=forge-controller
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

#### Metrics Not Scraped

```bash
# Check ServiceMonitor
kubectl get servicemonitor -n forge-system forge-controller -o yaml

# Verify labels match Prometheus selector
kubectl get prometheus -A -o yaml | grep serviceMonitorSelector -A 10
```

#### OTEL Connection Issues

```bash
# Test OTEL endpoint from controller
kubectl run -it --rm debug --image=busybox -n forge-system -- \
  nc -zv forge-otel-collector 4317
```

## Additional Resources

- **Quick Start Guide**: [chart/QUICKSTART.md](chart/QUICKSTART.md)
- **Full Chart Documentation**: [chart/README.md](chart/README.md)
- **Helm Chart Values**: [chart/forge/values.yaml](chart/forge/values.yaml)
- **Grafana Dashboard**: [config/grafana/forge-dashboard.json](config/grafana/forge-dashboard.json)

## Support

- **Issues**: https://github.com/kylegalloway/forge/issues
- **Discussions**: https://github.com/kylegalloway/forge/discussions
