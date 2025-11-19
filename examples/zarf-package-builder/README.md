# Zarf Package Builder for ScriptRunner

This example demonstrates using ScriptRunner to build Zarf packages in Kubernetes Jobs.

## Overview

[Zarf](https://zarf.dev) is a tool for packaging and deploying applications to air-gapped Kubernetes clusters. This ScriptRunner example provides:

- **Zarf Builder Container**: Alpine-based image with Zarf CLI pre-installed
- **Build Script**: Automated Zarf package building from Git repositories
- **Validation Script**: Package integrity and structure validation
- **ScriptRunner Integration**: Run Zarf builds as Kubernetes Jobs

## Use Cases

- **CI/CD Integration**: Build Zarf packages as part of your deployment pipeline
- **Multi-Architecture Builds**: Build packages for different architectures
- **Scheduled Builds**: Periodic package rebuilds with fresh base images
- **Air-Gap Preparation**: Automated packaging for disconnected environments

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
    SOURCE_REPO: "https://github.com/defenseunicorns/zarf"
    SOURCE_REF: "main"
    OUTPUT_PATH: "/tmp/packages"
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
    SOURCE_REPO: "https://github.com/your-org/your-zarf-package"
    SOURCE_REF: "v1.0.0"
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

Builds a Zarf package from a Git repository.

**Required Inputs:**
- `INPUT_SOURCE_REPO` - Git repository URL containing zarf.yaml

**Optional Inputs:**
- `INPUT_SOURCE_REF` - Git ref (branch/tag) to build from (default: main)
- `INPUT_OUTPUT_PATH` - Where to store the built package (default: /output)
- `INPUT_PACKAGE_NAME` - Name override for the package

**Example:**
```yaml
inputs:
  SOURCE_REPO: "https://github.com/org/my-zarf-package"
  SOURCE_REF: "v2.1.0"
  OUTPUT_PATH: "/workspace/dist"
```

**Output:**
- Zarf package file: `zarf-package-<name>-<arch>-<version>.tar.zst`
- SHA256 checksum

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
                  SOURCE_REPO: "https://github.com/org/package"
                  SOURCE_REF: "main"
              EOF
```

### Multi-Stage Pipeline

Build, validate, and deploy in sequence:

```yaml
# 1. Build
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: zarf-build
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_REPO: "https://github.com/org/package"
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
```

## Container Image

The Zarf builder image includes:

- **Base**: Alpine 3.19 (minimal, secure)
- **Zarf CLI**: v0.66.0 (latest stable)
- **Tools**: bash, curl, git, jq, tar, gzip
- **Scripts**: Pre-installed in `/scripts/`

### Customizing

To use a specific Zarf version:

```dockerfile
# In Dockerfile
ARG ZARF_VERSION=v0.66.0
```

Rebuild:
```bash
make build
```

## Security Considerations

### Image Source Control

The builder image clones Git repositories. Ensure:

1. **Trust source repositories**: Only build from trusted Git sources
2. **Use commit SHAs**: Reference specific commits instead of branches
3. **Private repos**: Use deploy keys or tokens for authentication

### Network Access

Building Zarf packages requires:
- Git access (HTTPS port 443)
- Container registry access (for pulling images referenced in zarf.yaml)

Ensure NetworkPolicies allow:
```yaml
egress:
- to:
  - namespaceSelector: {}
  ports:
  - protocol: TCP
    port: 443
```

### Resource Limits

Zarf builds can be resource-intensive:

```yaml
spec:
  resources:
    limits:
      cpu: "4"
      memory: "8Gi"
    requests:
      cpu: "2"
      memory: "4Gi"
```

## Troubleshooting

### Build Fails with "Permission Denied"

**Cause**: Scripts not executable in container

**Fix**: Verify Dockerfile sets execute permissions:
```dockerfile
RUN chmod +x /scripts/*.sh
```

### Git Clone Fails

**Cause**: Private repository or network policy

**Fix**:
1. Use public repos or configure credentials
2. Check NetworkPolicy allows HTTPS egress

### Package Too Large

**Cause**: Zarf packages include container images

**Fix**:
1. Increase storage limits
2. Use multi-stage builds to separate build artifacts
3. Clean up intermediate files

### "zarf: command not found"

**Cause**: Image not built correctly

**Fix**: Rebuild image:
```bash
make build
make load  # if using kind
```

## Examples

### Building ScriptRunner's Own Zarf Package

Build the ScriptRunner package itself:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: build-scriptrunner-package
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/build-package.sh
  inputs:
    SOURCE_REPO: "https://github.com/kylegalloway/scriptrunner"
    SOURCE_REF: "main"
```

This will use the `zarf.yaml` at the repository root to build a complete ScriptRunner deployment package.

### Validating a Pre-Built Package

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: validate-package
spec:
  image: scriptrunner-zarf-builder:latest
  scriptRef: /scripts/validate-package.sh
  inputs:
    PACKAGE_PATH: "/packages/zarf-package-scriptrunner-amd64-v0.3.0.tar.zst"
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
