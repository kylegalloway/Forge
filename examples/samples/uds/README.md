# UDS Bundle Examples

This directory contains example `UDSBundleJob` resources demonstrating UDS bundle workflows with Forge.

## Examples

### 01-git-to-oci: Create from Git, Publish to OCI

Demonstrates creating a UDS bundle from a Git repository and publishing to an OCI registry.

**Bundle Content:**
- UDS bundle definition (uds-bundle.yaml)
- Local Zarf package with service manifest
- Located in `01-git-to-oci/` directory

**Workflow:**
1. Fetch bundle definition from Git repository
2. Create UDS bundle
3. Publish to OCI registry (GHCR)

**Usage:**
```bash
# Update the OCI credentials secret first
kubectl apply -f 01-git-to-oci/udsbundlejob.yaml
kubectl get udsbundlejobs git-to-oci-bundle-example -w
```

### 02-local-to-s3: Create from Local, Publish to S3

Demonstrates creating a UDS bundle from local filesystem and publishing to an S3 bucket.

**Bundle Content:**
- UDS bundle definition with multiple packages
- Database package (Postgres StatefulSet)
- API package (REST API deployment)
- Located in `02-local-to-s3/` directory

**Workflow:**
1. Create bundle from local filesystem (mounted via PVC)
2. Publish to S3 bucket

**Usage:**
```bash
# Update the S3 credentials secret first
kubectl apply -f 02-local-to-s3/udsbundlejob.yaml
kubectl get udsbundlejobs local-to-s3-bundle-example -w
```

### 03-git-build-deploy: Create from Git, Deploy to Cluster

Demonstrates creating a UDS bundle from a public Git repository and deploying it directly to the cluster.

**Bundle Content:**
- Headlamp Kubernetes UI packaged as a UDS bundle
- Zarf package with Helm chart from official Headlamp repository
- Located in `03-git-build-deploy/` directory

**Workflow:**
1. Fetch bundle definition from Git repository
2. Create UDS bundle (builds Zarf package, downloads Helm chart and images)
3. Deploy to cluster in `headlamp` namespace

**Usage:**
```bash
kubectl apply -f 03-git-build-deploy/udsbundlejob.yaml
kubectl get udsbundlejobs git-build-deploy-bundle-example -w
# Access Headlamp: kubectl port-forward -n headlamp svc/headlamp 8080:80
```

## Prerequisites

1. **Forge Controller Installed**: Deploy Forge to your Kubernetes cluster with UDS support
2. **ServiceAccount Permissions**: Each example includes a ServiceAccount with appropriate policy annotations
3. **Credentials**: Configure secrets for OCI registries or S3 buckets
4. **UDS CLI**: The controller uses the UDS CLI to create and publish bundles

## Monitoring

### Check Status

```bash
kubectl get udsbundlejobs
kubectl describe udsbundlejob <name>
```

### View Job Logs

```bash
kubectl get jobs -l app=forge-uds
kubectl logs job/<job-name>
```

### Check Metrics

```bash
kubectl port-forward -n forge-system svc/forge-controller 8080:8080
curl http://localhost:8080/metrics | grep bundle
```

## Related Documentation

- [UDS Documentation](https://uds.defenseunicorns.com/)
- [Forge Controller Documentation](../../../docs/)
- [Zarf Package Examples](../zarf/)
