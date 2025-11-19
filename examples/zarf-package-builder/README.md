# Zarf Package Builder for ScriptRunner

Build Zarf packages from multiple sources and publish to various destinations using ScriptRunner.

## Overview

[Zarf](https://zarf.dev) packages applications for air-gapped Kubernetes deployments. This ScriptRunner integration provides a production-ready build pipeline with:

- **Multi-Source Support**: Build from Git repos, OCI registries, Helm charts, or local directories
- **Multi-Destination Publishing**: Output to local storage, S3 buckets, or OCI registries
- **Fully Automated**: One ScriptRunner resource triggers the entire build-publish workflow
- **Cloud-Native**: Runs as Kubernetes Jobs with resource limits and security hardening

### Supported Input Sources

| Source Type | Description | Example |
|-------------|-------------|---------|
| **git** | Clone Git repository with zarf.yaml | `https://github.com/org/repo` |
| **oci** | Pull existing OCI package | `ghcr.io/org/package:v1.0.0` |
| **helm** | Package Helm chart as Zarf | `https://charts.bitnami.com/bitnami` |
| **local** | Use local directory (testing) | `/workspace/package` |

### Supported Output Destinations

| Output Type | Description | Use Case |
|-------------|-------------|----------|
| **local** | Save to pod filesystem | Quick testing, ephemeral builds |
| **s3** | Upload to AWS S3 bucket | Artifact storage, distribution |
| **oci** | Publish to OCI registry | Registry-based distribution |

## Use Cases

- **CI/CD Integration**: Automated package builds on every commit
- **Multi-Registry Mirroring**: Pull from one OCI registry, push to another
- **Helm-to-Zarf Conversion**: Package Helm charts for air-gap deployment
- **Scheduled Rebuilds**: Nightly builds with latest base images
- **S3 Artifact Distribution**: Build once, distribute via S3 presigned URLs
- **Air-Gap Preparation**: Fully automated package creation and storage

## Quick Start

### 1. Build the Zarf Builder Image

```bash
cd examples/zarf-package-builder
make build
```

### 2. Load into Kind (for local testing)

```bash
make load
```

### 3. Create a ScriptRunner to Build a Package

**Example: Build from Git repository**

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: build-zarf-package
  namespace: default
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "git"
    SOURCE_LOCATION: "https://github.com/defenseunicorns/zarf"
    SOURCE_REF: "main"
    OUTPUT_TYPE: "local"
    OUTPUT_PATH: "/tmp/packages"
```

**Example: Package Helm chart to S3**

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: helm-to-s3
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "helm"
    SOURCE_LOCATION: "https://charts.bitnami.com/bitnami"
    SOURCE_REF: "15.0.0"
    PACKAGE_NAME: "postgresql"
    OUTPUT_TYPE: "s3"
    OUTPUT_PATH: "my-zarf-packages"
    AWS_REGION: "us-east-1"
```

Apply:
```bash
kubectl apply -f - <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: build-zarf-package
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "git"
    SOURCE_LOCATION: "https://github.com/your-org/your-zarf-package"
    SOURCE_REF: "v1.0.0"
    OUTPUT_TYPE: "local"
    OUTPUT_PATH: "/tmp/packages"
EOF
```

### 4. Check Build Status

```bash
# View ScriptRunner status
kubectl get scriptrunners build-zarf-package

# View Job status
kubectl get jobs -l scriptrunner.io/name=build-zarf-package

# View build logs
kubectl logs -l scriptrunner.io/name=build-zarf-package
```

## Scripts

### build-package.sh

Builds a Zarf package from multiple input sources and publishes to various destinations.

**Required Inputs:**
- `INPUT_SOURCE_TYPE` - Input source type: `git`, `oci`, `helm`, or `local`
- `INPUT_SOURCE_LOCATION` - Source location (URL or path depending on type)
- `INPUT_OUTPUT_TYPE` - Output destination type: `local`, `s3`, or `oci`

**Source-Specific Inputs:**

**Git sources:**
- `INPUT_SOURCE_REF` - Git branch/tag (default: main)

**OCI sources:**
- `INPUT_SOURCE_REF` - OCI tag (default: latest)

**Helm sources:**
- `INPUT_SOURCE_REF` - Helm chart version
- `INPUT_PACKAGE_NAME` - Chart name (required)
- `INPUT_HELM_REPO_NAME` - Repository name (optional)

**Output-Specific Inputs:**

**Local output:**
- `INPUT_OUTPUT_PATH` - Directory path (default: /output)

**S3 output:**
- `INPUT_OUTPUT_PATH` - S3 bucket name
- `INPUT_AWS_REGION` - AWS region (default: us-east-1)
- `INPUT_S3_KEY_PREFIX` - S3 key prefix (optional)

**OCI output:**
- `INPUT_OUTPUT_REGISTRY` - OCI registry URL (e.g., `ghcr.io/org/repo`)

**Optional Inputs:**
- `INPUT_BUILD_ARGS` - Additional Zarf build arguments (e.g., `--set VAR=value`)
- `INPUT_ZARF_ARCHITECTURE` - Target architecture (default: auto-detect)

**Examples:**

```yaml
# Git to local
inputs:
  SOURCE_TYPE: "git"
  SOURCE_LOCATION: "https://github.com/org/package"
  SOURCE_REF: "v2.1.0"
  OUTPUT_TYPE: "local"
  OUTPUT_PATH: "/tmp/packages"

# Helm to S3
inputs:
  SOURCE_TYPE: "helm"
  SOURCE_LOCATION: "https://charts.bitnami.com/bitnami"
  SOURCE_REF: "15.0.0"
  PACKAGE_NAME: "postgresql"
  OUTPUT_TYPE: "s3"
  OUTPUT_PATH: "my-bucket"
  AWS_REGION: "us-west-2"

# OCI mirroring
inputs:
  SOURCE_TYPE: "oci"
  SOURCE_LOCATION: "ghcr.io/defenseunicorns/packages/init"
  SOURCE_REF: "v0.32.6-amd64"
  OUTPUT_TYPE: "oci"
  OUTPUT_REGISTRY: "registry.example.com/zarf/init"
```

**Output:**
- Zarf package file: `zarf-package-<name>-<arch>-<version>.tar.zst`
- SHA256 checksum (for local/S3 outputs)
- OCI artifact reference (for OCI output)

### validate-package.sh

Validates a Zarf package file.

**Required Inputs:**
- `INPUT_PACKAGE_PATH` - Path to the Zarf package (.tar.zst file)

**Example:**
```yaml
inputs:
  PACKAGE_PATH: "/packages/zarf-package-example-amd64-v1.0.0.tar.zst"
```

**Checks:**
- File format (Zstandard compression)
- Package structure (components, metadata)
- Integrity verification

## Advanced Usage

### Building with Persistent Storage

To persist built packages, mount a PersistentVolume:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: zarf-packages-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: build-with-storage
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_REPO: "https://github.com/org/package"
    OUTPUT_PATH: "/packages"
  # Note: This requires controller modification to support volumes
  # volumes:
  #   - name: packages
  #     persistentVolumeClaim:
  #       claimName: zarf-packages-pvc
  # volumeMounts:
  #   - name: packages
  #     mountPath: /packages
```

### OCI Registry Mirroring

Pull from one registry, push to another:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: mirror-to-internal-registry
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "oci"
    SOURCE_LOCATION: "ghcr.io/defenseunicorns/packages/init"
    SOURCE_REF: "v0.32.6-amd64"
    OUTPUT_TYPE: "oci"
    OUTPUT_REGISTRY: "registry.internal.example.com/zarf/init"
```

### Helm Chart to S3 Distribution

Package a Helm chart and upload to S3 for air-gap distribution:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: helm-to-s3-distribution
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "helm"
    SOURCE_LOCATION: "https://kubernetes-sigs.github.io/metrics-server/"
    SOURCE_REF: "3.11.0"
    PACKAGE_NAME: "metrics-server"
    OUTPUT_TYPE: "s3"
    OUTPUT_PATH: "airgap-packages"
    AWS_REGION: "us-east-1"
    S3_KEY_PREFIX: "production/metrics-server/"
```

### Scheduled Builds with CronJob

Create a CronJob to build packages on a schedule:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-zarf-build
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
          - name: create-scriptrunner
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              kubectl apply -f - <<EOF
              apiVersion: scriptrunner.io/v1alpha1
              kind: ScriptRunner
              metadata:
                name: nightly-build-$(date +%Y%m%d)
              spec:
                image: scriptrunner-zarf-builder:latest
                scriptRef: /scripts/build-package.sh
                inputs:
                  SOURCE_TYPE: "git"
                  SOURCE_LOCATION: "https://github.com/org/package"
                  SOURCE_REF: "main"
                  OUTPUT_TYPE: "local"
                  OUTPUT_PATH: "/tmp/packages"
              EOF
```

### Multi-Stage Pipeline

Build, validate, and publish in sequence:

```yaml
# 1. Build from Git to local storage
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: zarf-build
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "git"
    SOURCE_LOCATION: "https://github.com/org/package"
    SOURCE_REF: "v1.0.0"
    OUTPUT_TYPE: "local"
    OUTPUT_PATH: "/output"
---
# 2. Validate (after build completes)
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: zarf-validate
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/validate-package.sh
  inputs:
    PACKAGE_PATH: "/output/zarf-package-example-amd64.tar.zst"
---
# 3. Publish to OCI registry (after validation)
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: zarf-publish
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "local"
    SOURCE_LOCATION: "/output"
    OUTPUT_TYPE: "oci"
    OUTPUT_REGISTRY: "ghcr.io/org/packages/example"
```

## Container Image

The Zarf builder image includes:

- **Base**: Alpine 3.19 (minimal, secure)
- **Zarf CLI**: v0.66.0 (latest stable)
- **Helm CLI**: v3.14.0 (for Helm chart packaging)
- **AWS CLI**: Latest (for S3 uploads)
- **Tools**: bash, curl, git, jq, tar, gzip, ca-certificates, python3
- **Scripts**: Pre-installed in `/scripts/`
  - `build-package.sh` - Multi-source/destination builder
  - `validate-package.sh` - Package validator

### Customizing

To use specific versions:

```dockerfile
# In Dockerfile
ARG ZARF_VERSION=v0.66.0
ARG HELM_VERSION=v3.14.0
```

Rebuild:
```bash
make build
```

### Size Optimization

The image is optimized for air-gap scenarios:
- Alpine base: ~5 MB
- Zarf CLI: ~70 MB
- Helm CLI: ~50 MB
- AWS CLI: ~50 MB
- Total: ~180 MB compressed

## Security Considerations

### Input Source Validation

The admission webhook validates all inputs:

1. **Git repositories**: URL must match approved domain patterns
2. **OCI registries**: Only pull from whitelisted registries
3. **Helm repositories**: Validate chart repository URLs
4. **Command injection**: All inputs sanitized for shell metacharacters

### Credentials Management

Different sources require different credentials:

**Git (private repos):**
```yaml
# Use Kubernetes Secret with SSH key
env:
  - name: GIT_SSH_COMMAND
    value: "ssh -i /secrets/deploy-key"
```

**S3 uploads:**
```yaml
# Use IAM roles (preferred) or AWS credentials
env:
  - name: AWS_ROLE_ARN
    value: "arn:aws:iam::123456789:role/zarf-builder"
# OR
  - name: AWS_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: aws-creds
        key: access-key-id
```

**OCI registries:**
```yaml
# Use imagePullSecrets or explicit auth
env:
  - name: REGISTRY_USERNAME
    valueFrom:
      secretKeyRef:
        name: registry-creds
        key: username
  - name: REGISTRY_PASSWORD
    valueFrom:
      secretKeyRef:
        name: registry-creds
        key: password
```

### Network Access

Building Zarf packages requires:
- **Git access**: HTTPS port 443
- **Container registry access**: For pulling images referenced in zarf.yaml
- **Helm repository access**: HTTPS for chart downloads
- **S3 access**: HTTPS for uploads (if using S3 output)

Ensure NetworkPolicies allow:
```yaml
egress:
- to:
  - namespaceSelector: {}
  ports:
  - protocol: TCP
    port: 443  # HTTPS
  - protocol: TCP
    port: 5000  # Private registries (if needed)
```

### Resource Limits

Zarf builds can be resource-intensive, especially with container images:

```yaml
spec:
  resources:
    limits:
      cpu: "4"
      memory: "8Gi"
      ephemeral-storage: "20Gi"  # Important for large packages
    requests:
      cpu: "2"
      memory: "4Gi"
      ephemeral-storage: "10Gi"
```

### Air-Gap Considerations

For true air-gap builds:
1. Pre-pull all base images into the cluster
2. Use local OCI registry for dependencies
3. Bundle Helm charts with the builder image
4. Output to local storage, then manually transfer

## Troubleshooting

### Build Fails with "Permission Denied"

**Cause**: Scripts not executable in container

**Fix**: Verify Dockerfile sets execute permissions:
```dockerfile
RUN chmod +x /scripts/*.sh
```

### Source Fetch Errors

**Git clone fails:**
- Private repository: Configure SSH keys or personal access tokens
- Network policy: Check NetworkPolicy allows HTTPS egress to Git hosts
- Invalid ref: Verify branch/tag exists with `git ls-remote`

**OCI pull fails:**
- Authentication: Configure registry credentials via imagePullSecrets
- Invalid reference: Verify package exists with `zarf package list oci://registry/package`
- Network: Check egress rules for registry domain

**Helm fetch fails:**
- Chart not found: Verify chart name and version with `helm search repo`
- Repository not added: Ensure Helm repo URL is accessible
- Version mismatch: Check available versions with `helm search repo <chart> --versions`

### Output Destination Errors

**S3 upload fails:**
- Credentials: Verify AWS credentials with `aws sts get-caller-identity`
- Bucket access: Check bucket exists and has correct permissions
- Region mismatch: Ensure AWS_REGION matches bucket region

**OCI publish fails:**
- Registry authentication: Use `zarf tools registry login` to test
- Insufficient permissions: Verify push access to registry/repository
- Network: Check egress rules for registry domain

### Package Too Large

**Cause**: Zarf packages include container images

**Fix**:
1. Increase ephemeral storage limits: `ephemeral-storage: "20Gi"`
2. Use external volumes for build workspace
3. Clean up intermediate files in build script
4. Consider multi-component packages for modularity

### "zarf: command not found"

**Cause**: Image not built correctly

**Fix**: Rebuild image:
```bash
make build
make load  # if using kind
```

### Helm Chart Missing zarf.yaml

**Expected Behavior**: The build script auto-generates zarf.yaml for Helm charts

**Verify**: Check build logs for "Auto-generating zarf.yaml for Helm chart"

**Manual Override**: Pre-create zarf.yaml if you need custom component configuration

## Examples

See [config/samples/scriptrunner_zarf_build.yaml](../../config/samples/scriptrunner_zarf_build.yaml) for comprehensive examples including:

### Example 1: Git Repository to Local Storage

Most common use case - build a package from a Git repo:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: git-to-local-build
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "git"
    SOURCE_LOCATION: "https://github.com/defenseunicorns/zarf"
    SOURCE_REF: "v0.35.0"
    OUTPUT_TYPE: "local"
    OUTPUT_PATH: "/tmp/packages"
```

### Example 2: Helm Chart to S3 Bucket

Package a Helm chart and upload to S3 for distribution:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: helm-chart-to-s3
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "helm"
    SOURCE_LOCATION: "https://charts.bitnami.com/bitnami"
    SOURCE_REF: "15.0.0"
    PACKAGE_NAME: "postgresql"
    OUTPUT_TYPE: "s3"
    OUTPUT_PATH: "my-zarf-packages"
    AWS_REGION: "us-east-1"
```

### Example 3: Git Repository to OCI Registry

Build and publish directly to an OCI registry:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: git-to-oci-registry
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "git"
    SOURCE_LOCATION: "https://github.com/your-org/your-package"
    SOURCE_REF: "v1.2.3"
    OUTPUT_TYPE: "oci"
    OUTPUT_REGISTRY: "ghcr.io/your-org/zarf-packages/your-package"
```

### Example 4: OCI Registry Mirroring

Pull from one registry and push to another (air-gap preparation):

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: oci-registry-mirror
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "oci"
    SOURCE_LOCATION: "ghcr.io/defenseunicorns/packages/init"
    SOURCE_REF: "v0.32.6-amd64"
    OUTPUT_TYPE: "oci"
    OUTPUT_REGISTRY: "registry.internal.mil/zarf/init"
```

### Example 5: Local Testing

Quick local build for testing (uses embedded test package):

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: local-test-build
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_TYPE: "local"
    SOURCE_LOCATION: "/workspace/test-package"
    OUTPUT_TYPE: "local"
    OUTPUT_PATH: "/tmp/test-output"
```

### Example 6: Validating a Package

Validate package structure and integrity:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: validate-package
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/validate-package.sh
  inputs:
    PACKAGE_PATH: "/packages/zarf-package-example-amd64-v1.0.0.tar.zst"
```

## Integration with ScriptRunner Production Features

### Admission Webhook

The webhook validates:
- Image registry (builder image must be from approved registry)
- Script reference (`/scripts/build-package.sh` must be in whitelist)
- Input sanitization (repository URLs checked for injection)

### Resource Quotas

Building packages consumes resources:
- CPU for compression
- Memory for image processing
- Storage for output packages

Ensure user namespaces have adequate quotas:
```bash
./scripts/onboard-user.sh builder-user \
  --cpu-limit 20 \
  --memory-limit 40Gi
```

### Network Policies

Allow egress for Git and container registries:
```yaml
# In user namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-zarf-build
spec:
  podSelector:
    matchLabels:
      app: scriptrunner
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS for Git and registries
```

## References

- [Zarf Documentation](https://zarf.dev)
- [Zarf GitHub](https://github.com/defenseunicorns/zarf)
- [ScriptRunner Documentation](../../README.md)
- [Air-Gap Kubernetes Deployments](https://kubernetes.io/blog/2023/10/12/installing-k8s-offline-with-zarf/)
