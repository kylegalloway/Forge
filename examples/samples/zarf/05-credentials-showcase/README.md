# Credentials Showcase - Zarf Package Jobs

This example demonstrates all credential types supported by Forge for ZarfPackageJobs.

## Files

- `secrets.yaml` - All secret definitions with placeholder values
- `zarfpackagejobs.yaml` - Example jobs showing different credential combinations

## Credential Types

| Type | Secret Format | Keys | Used For |
|------|---------------|------|----------|
| Git (token) | Opaque | `token` | Cloning private repositories |
| Git (SSH) | Opaque | `ssh-key` | Cloning private repositories |
| OCI Registry | `kubernetes.io/dockerconfigjson` | `.dockerconfigjson` | Pulling/pushing packages |
| S3 | Opaque | `access-key-id`, `secret-access-key` | Pulling/pushing from S3 |
| Kubeconfig | Opaque | `kubeconfig` | Deploying to external clusters |

## Examples

1. **Git -> OCI**: Build from private Git, publish to OCI registry
2. **Git -> S3**: Build from private Git, publish to S3 bucket
3. **OCI -> OCI**: Promote package between registries
4. **S3 -> External Deploy**: Deploy S3-stored package to external cluster
5. **Full Pipeline**: Git -> Build -> OCI -> External Deploy
6. **MinIO Workflow**: S3-compatible storage for air-gapped environments
7. **In-Cluster Deploy**: Deploy without external kubeconfig

## Usage

```bash
# Create the namespace
kubectl create namespace forge-jobs

# Edit secrets.yaml with real credentials
vim secrets.yaml

# Apply secrets and service account
kubectl apply -f secrets.yaml

# Run an example job
kubectl apply -f zarfpackagejobs.yaml
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

### OCI Registry

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oci-credentials
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |  # pragma: allowlist secret
    {"auths":{"ghcr.io":{"username":"user","password":"token"}}}
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
