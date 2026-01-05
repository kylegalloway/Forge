# Podinfo Example

This example demonstrates building a Zarf package from the Podinfo Helm chart.

[Podinfo](https://github.com/stefanprodan/podinfo) is a tiny web application made with Go that showcases best practices of running microservices in Kubernetes.

## What This Example Does

- Packages the Podinfo Helm chart (version 6.7.0) into a Zarf package
- Includes the Podinfo container image
- Can be built and optionally deployed to a Kubernetes cluster

## Files

- `zarf.yaml`: Zarf package definition that references the Podinfo Helm chart
- `zarfpackagejob.yaml`: Example ZarfPackageJob for building this package
- `serviceaccount.yaml`: ServiceAccount with appropriate Forge policies

## Quick Start

### 1. Create ServiceAccount

```bash
kubectl apply -f serviceaccount.yaml
```

### 2. Build the Package

```bash
kubectl apply -f zarfpackagejob.yaml
```

### 3. Watch Progress

```bash
kubectl get zarfpackagejobs -n default -w
```

## Expected Output

The job will:
1. Clone this Forge repository
2. Navigate to `examples/samples/zarf/04-podinfo`
3. Run `zarf package create` to build the package
4. Create `zarf-package-podinfo-<arch>-6.7.0.tar.zst`

## What Gets Packaged

- **Helm Chart**: Podinfo chart from https://stefanprodan.github.io/podinfo
- **Container Image**: `ghcr.io/stefanprodan/podinfo:6.7.0`

This creates an air-gapped package that can be deployed to disconnected environments.
