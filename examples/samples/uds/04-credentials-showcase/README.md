# Credentials Showcase - UDS Bundle Jobs

This example demonstrates all credential types supported by Forge for UDSBundleJobs.

## Files

- `secrets.yaml` - All secret definitions with placeholder values
- `udsbundlejobs.yaml` - Example jobs showing different credential combinations

## Credential Types

| Type | Secret Format | Keys | Used For |
|------|---------------|------|----------|
| Git (token) | Opaque | `token` | Cloning private repositories |
| Git (SSH) | Opaque | `ssh-key` | Cloning private repositories |
| OCI Registry | `kubernetes.io/dockerconfigjson` | `.dockerconfigjson` | Pulling/pushing bundles and packages |
| S3 | Opaque | `access-key-id`, `secret-access-key` | Pulling/pushing from S3 |
| Kubeconfig | Opaque | `kubeconfig` | Deploying to external clusters |

## Examples

1. **Git -> OCI**: Create bundle from private Git, publish to OCI registry
2. **Git -> S3**: Create bundle from private Git, publish to S3 bucket
3. **OCI -> OCI**: Promote bundle between registries
4. **S3 -> External Deploy**: Deploy S3-stored bundle to external cluster
5. **Full Pipeline**: Git -> Create -> OCI -> External Deploy
6. **MinIO Workflow**: S3-compatible storage for air-gapped environments
7. **In-Cluster Deploy**: Deploy without external kubeconfig
8. **Multi-Registry Create**: Bundle creation pulling packages from multiple registries

## Usage

```bash
# Create the namespace
kubectl create namespace forge-jobs

# Edit secrets.yaml with real credentials
vim secrets.yaml

# Apply secrets and service account
kubectl apply -f secrets.yaml

# Run an example job
kubectl apply -f udsbundlejobs.yaml
```

## UDS-Specific Considerations

### Multi-Registry Access

UDS bundles often reference Zarf packages from multiple OCI registries. When creating a bundle, Forge needs credentials for **all** registries referenced in your `uds-bundle.yaml`:

```yaml
# Example uds-bundle.yaml referencing multiple registries
packages:
  - name: init
    repository: ghcr.io/defenseunicorns/packages/init
    ref: v0.32.0
  - name: loki
    repository: registry1.dso.mil/ironbank/opensource/grafana/loki
    ref: 2.9.3
```

Use a multi-registry secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oci-multi-registry-credentials
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |
    {
      "auths": {
        "ghcr.io": {"username": "...", "password": "..."},
        "registry1.dso.mil": {"username": "...", "password": "..."}
      }
    }
```

### Resource Requirements

UDS bundle creation can be resource-intensive, especially for large bundles with many packages. Consider setting appropriate resource requests:

```yaml
resources:
  requests:
    cpu: "2"
    memory: 8Gi
    ephemeral-storage: 50Gi
  limits:
    cpu: "8"
    memory: 32Gi
    ephemeral-storage: 100Gi
```

## Secret Examples

### Git Token

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-token-credentials
type: Opaque
stringData:
  token: "ghp_your_github_token_here"
```

### OCI Registry (Multi-Registry)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oci-credentials
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |  # pragma: allowlist secret
    {
      "auths": {
        "ghcr.io": {"username": "user", "password": "token"},
        "registry1.dso.mil": {"username": "user", "password": "token"}
      }
    }
```

### S3

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
type: Opaque
stringData:  # pragma: allowlist secret
  access-key-id: "AKIAIOSFODNN7EXAMPLE"
  secret-access-key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

### Kubeconfig

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cluster-kubeconfig
type: Opaque
stringData:
  kubeconfig: |
    apiVersion: v1
    kind: Config
    # ... full kubeconfig content
```
