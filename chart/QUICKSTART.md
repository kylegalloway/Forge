# Forge Helm Chart - Quick Start Guide

This guide will help you quickly deploy Forge to your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.8+
- kubectl configured to access your cluster

## Choose Your Scenario

### Scenario 1: I have an existing monitoring setup 🔧

**Use this if you already have:**
- Prometheus deployed
- Grafana deployed
- OpenTelemetry Collector deployed

```bash
# 1. Edit values-mature-cluster.yaml and update your endpoints
vim chart/forge/values-mature-cluster.yaml

# 2. Install Forge
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system \
  --create-namespace

# 3. Verify installation
kubectl get pods -n forge-system
```

### Scenario 2: I need monitoring tools installed 🚀

**Use this if you need:**
- Complete observability stack
- Prometheus, Grafana, and OTEL Collector

```bash
# 1. Edit values-new-cluster.yaml and set a strong Grafana password
vim chart/forge/values-new-cluster.yaml
# Change: observability.grafana.config.adminPassword

# 2. (Optional) Configure ingress for Grafana
# Edit the ingress section in values-new-cluster.yaml

# 3. Install Forge with full stack
helm upgrade --install forge ./chart/forge \
  -f chart/forge/values-new-cluster.yaml \
  --namespace forge-system \
  --create-namespace

# 4. Verify installation
kubectl get pods -n forge-system

# 5. Access Grafana
kubectl port-forward -n forge-system svc/grafana 3000:80
# Visit http://localhost:3000
```

## Verify Installation

```bash
# Check all pods are running
kubectl get pods -n forge-system

# Expected output:
# NAME                                READY   STATUS    RESTARTS   AGE
# forge-controller-xxxxx              1/1     Running   0          1m
# forge-otel-collector-xxxxx          1/1     Running   0          1m  (if deployStack: true)
```

## Test the Controller

```bash
# Create a sample ZarfPackageJob
kubectl apply -f config/samples/v1alpha1/build-only-git.yaml

# Watch the job
kubectl get zarfpackagejobs -A -w

# Check the job details
kubectl describe zarfpackagejob <job-name> -n <namespace>

# View controller logs
kubectl logs -n forge-system -l app=forge-controller -f
```

## Access Monitoring Tools

### Controller Metrics

```bash
kubectl port-forward -n forge-system svc/forge-controller-metrics 8080:8080
# Visit http://localhost:8080/metrics
```

### Prometheus (if deployed)

```bash
kubectl port-forward -n forge-system svc/prometheus-server 9090:9090
# Visit http://localhost:9090
```

### Grafana (if deployed)

```bash
kubectl port-forward -n forge-system svc/grafana 3000:80
# Visit http://localhost:3000
# Username: admin
# Password: (the one you set in values-new-cluster.yaml)
```

### OTEL Collector (if deployed)

```bash
# Check OTEL Collector health
kubectl port-forward -n forge-system svc/forge-otel-collector 13133:13133
curl http://localhost:13133
```

## Common Configuration Options

### Scale the Controller

```bash
helm upgrade forge ./chart/forge \
  --set controller.replicaCount=3 \
  --namespace forge-system
```

### Enable Network Policies

```bash
helm upgrade forge ./chart/forge \
  --set networkPolicies.enabled=true \
  --namespace forge-system
```

### Disable ServiceMonitor

```bash
helm upgrade forge ./chart/forge \
  --set metrics.serviceMonitor.enabled=false \
  --namespace forge-system
```

## Troubleshooting

### Controller Pod Not Starting

```bash
# Describe the pod
kubectl describe pod -n forge-system -l app=forge-controller

# Check for image pull issues
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

### Metrics Not Showing in Prometheus

```bash
# Verify ServiceMonitor exists
kubectl get servicemonitor -n forge-system

# Check ServiceMonitor labels
kubectl get servicemonitor -n forge-system forge-controller -o yaml

# Ensure labels match your Prometheus selector
```

### OTEL Collector Not Receiving Data

```bash
# Check controller environment variables
kubectl get deployment -n forge-system forge-controller -o yaml | grep OTEL

# Verify OTEL service exists
kubectl get svc -n forge-system forge-otel-collector

# Check OTEL logs
kubectl logs -n forge-system -l app=otel-collector
```

## Upgrading

```bash
# Upgrade with new values
helm upgrade forge ./chart/forge \
  -f chart/forge/values-mature-cluster.yaml \
  --namespace forge-system

# Verify upgrade
helm list -n forge-system
kubectl rollout status deployment/forge-controller -n forge-system
```

## Uninstalling

```bash
# Remove the Helm release
helm uninstall forge --namespace forge-system

# (Optional) Remove the namespace
kubectl delete namespace forge-system

# (Optional) Remove CRDs
kubectl delete crd zarfpackagejobs.forge.dev
```

## Next Steps

1. **Review the full documentation**: [chart/README.md](README.md)
2. **Configure alerts**: Edit PrometheusRule settings in values.yaml
3. **Import dashboard**: Load `config/grafana/forge-dashboard.json` into Grafana
4. **Set up ingress**: Configure ingress for external Grafana access
5. **Enable network policies**: Lock down pod communication
6. **Configure backups**: Set up backup for Prometheus and Grafana data

## Getting Help

- **Documentation**: [chart/README.md](README.md)
- **Issues**: https://github.com/kylegalloway/forge/issues
- **Logs**: `kubectl logs -n forge-system -l app=forge-controller`
