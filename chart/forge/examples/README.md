# Forge Helm Chart Examples

This directory contains example values files for different deployment scenarios.

## Available Examples

### [values-kind.yaml](values-kind.yaml)
Configuration for local development using Kind (Kubernetes in Docker).

**Key Features:**
- Lightweight resource limits
- Uses separate kube-prometheus-stack installation
- Local image registry (localhost/forge-controller)
- OTEL collector only (no Prometheus/Grafana deployment)

**Usage:**
```bash
helm upgrade --install forge ../.. -f values-kind.yaml
```

### [values-new-cluster.yaml](values-new-cluster.yaml)
Configuration for deploying into a new cluster with no existing monitoring infrastructure.

**Key Features:**
- Deploys full observability stack (OTEL, Prometheus, Grafana)
- Production-grade resource limits
- Persistent storage enabled
- Network policies enabled
- Ingress configuration example

**Usage:**
```bash
helm upgrade --install forge ../.. -f values-new-cluster.yaml
```

### [values-mature-cluster.yaml](values-mature-cluster.yaml)
Configuration for deploying into a mature cluster with existing monitoring infrastructure.

**Key Features:**
- No observability stack deployment
- Uses external OTEL collector endpoint
- Uses external Prometheus and Grafana
- Minimal chart footprint

**Usage:**
```bash
helm upgrade --install forge ../.. -f values-mature-cluster.yaml
```

## Using Examples

You can use these examples in three ways:

1. **Direct file reference:**
   ```bash
   helm upgrade --install forge ../.. -f examples/values-kind.yaml
   ```

2. **Override specific values:**
   ```bash
   helm upgrade --install forge ../.. \
     -f examples/values-kind.yaml \
     --set controller.replicaCount=2
   ```

3. **Combine multiple files:**
   ```bash
   helm upgrade --install forge ../.. \
     -f examples/values-kind.yaml \
     -f my-custom-overrides.yaml
   ```

## Default Values

The main [../values.yaml](../values.yaml) file contains all configuration options with:
- Comprehensive inline comments
- Scenario-specific guidance (Kind/Production)
- Examples for each setting
- References to these example files

For most use cases, you should be able to use the main values.yaml file with `--set` overrides rather than maintaining separate files.
