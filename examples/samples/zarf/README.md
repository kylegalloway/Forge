# Zarf Package Examples

This directory contains example `ZarfPackageJob` resources demonstrating build and publish workflows with Forge.

## Examples

### 01-git-to-oci: Build from Git, Publish to OCI

Demonstrates fetching a Zarf package from a Git repository, building it, and publishing to an OCI registry.

**Package Content:**
- Simple Helm chart with ConfigMap
- Located in `01-git-to-oci/` directory

**Workflow:**
1. Fetch package definition from Git
2. Build Zarf package
3. Publish to OCI registry (GHCR)

**Usage:**
```bash
# Update the OCI credentials secret first
kubectl apply -f 01-git-to-oci/zarfpackagejob.yaml
kubectl get zarfpackagejobs git-to-oci-example -w
```

### 02-local-to-s3: Build from Local, Publish to S3

Demonstrates building a Zarf package from local filesystem and publishing to an S3 bucket.

**Package Content:**
- Nginx deployment manifest
- Located in `02-local-to-s3/` directory

**Workflow:**
1. Build package from local filesystem (mounted via PVC)
2. Publish to S3 bucket

**Usage:**
```bash
# Update the S3 credentials secret first
kubectl apply -f 02-local-to-s3/zarfpackagejob.yaml
kubectl get zarfpackagejobs local-to-s3-example -w
```

### 03-git-build-deploy: Build from Git, Deploy to Cluster

Demonstrates building a Zarf package from a public Git repository and deploying it directly to the cluster.

**Package Content:**
- Headlamp Kubernetes UI
- Helm chart from official Headlamp repository
- Located in `03-git-build-deploy/` directory

**Workflow:**
1. Fetch package definition from Git repository
2. Build Zarf package (downloads Helm chart and container images)
3. Deploy to cluster in `headlamp` namespace

**Usage:**
```bash
kubectl apply -f 03-git-build-deploy/zarfpackagejob.yaml
kubectl get zarfpackagejobs git-build-deploy-example -w
# Access Headlamp: kubectl port-forward -n headlamp svc/headlamp 8080:80
```

## Prerequisites

1. **Forge Controller Installed**: Deploy Forge to your Kubernetes cluster
2. **ServiceAccount Permissions**: Each example includes a ServiceAccount with appropriate policy annotations
3. **Credentials**: Configure secrets for OCI registries or S3 buckets

## Monitoring

### Check Status

```bash
kubectl get zarfpackagejobs
kubectl describe zarfpackagejob <name>
```

### View Job Logs

```bash
kubectl get jobs -l forge.dev/package=<package-name>
kubectl logs job/<job-name>
```

### Check Metrics

```bash
kubectl port-forward -n forge-system svc/forge-controller 8080:8080
curl http://localhost:8080/metrics | grep zarf
```

## Related Documentation

- [Zarf Documentation](https://zarf.dev/)
- [Forge Controller Documentation](../../../docs/)
- [UDS Bundle Examples](../uds/)
