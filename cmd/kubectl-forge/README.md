# kubectl-forge

kubectl plugin for Forge - Kubernetes job orchestrator for Zarf packages and UDS bundles.

## Installation

### Build from source

```bash
make build-kubectl-forge
```

The binary will be created at `bin/kubectl-forge`.

### Install to PATH

To use as a kubectl plugin, copy the binary to your PATH:

```bash
# Copy to local bin directory
cp bin/kubectl-forge /usr/local/bin/kubectl-forge

# Or create a symlink
ln -s $(pwd)/bin/kubectl-forge /usr/local/bin/kubectl-forge
```

Once installed, you can use it as `kubectl forge` (without the dash).

## Commands

### list

List all Forge jobs in the cluster:

```bash
# List jobs in current namespace
kubectl forge list

# List jobs in all namespaces
kubectl forge list --all-namespaces

# List only Zarf package jobs
kubectl forge list --type zarf

# List only UDS bundle jobs
kubectl forge list --type uds
```

**Output:**
```txt
NAME                  TYPE   ACTION   PHASE       AGE
my-package-build      zarf   Build    Completed   5m
my-bundle-create      uds    Create   Running     2m
```

### download

Download artifacts from a completed job:

```bash
# Download artifacts from a job to current directory
kubectl forge download my-package-build

# Download to a specific directory
kubectl forge download my-package-build --output-dir ./artifacts

# Download all artifacts (including intermediate build files)
kubectl forge download my-package-build --all

# Specify namespace
kubectl forge download my-package-build -n my-namespace
```

**How it works:**
1. Finds the Kubernetes Job by name
2. Locates the artifact PVC associated with the job
3. Creates a temporary pod to mount the PVC
4. Downloads artifacts to your local machine
5. Cleans up the temporary pod

**Downloaded files:**
- By default: Final artifacts only (`.tar.zst`, `.tar.gz`, SBOM files)
- With `--all`: All files including build cache

### debug

Debug a failed or running job by exec'ing into the pod:

```bash
# Debug a failed job (exec into the pod)
kubectl forge debug my-package-build --failed

# Debug using a specific container (for multi-container pods)
kubectl forge debug my-package-build --container zarf-build

# Use bash instead of default sh
kubectl forge debug my-package-build --shell /bin/bash

# Create a new debug pod with access to the workspace
kubectl forge debug my-package-build --copy-workspace

# Use a custom debug image
kubectl forge debug my-package-build --debug-image ubuntu:22.04

# Keep the debug pod after exit for later inspection
kubectl forge debug my-package-build --copy-workspace --preserve-pod
```

**Features:**
- Exec into existing job pods (running or failed)
- Create new debug pods with access to job volumes
- Automatic cleanup of debug pods (unless `--preserve-pod` is used)
- Custom shells and debug images

**Common debugging workflows:**

1. **Quick debug of failed job:**
   ```bash
   kubectl forge debug my-job --failed
   ```
   Opens a shell in the failed pod to inspect logs and state.

2. **Create debug pod with workspace access:**
   ```bash
   kubectl forge debug my-job --copy-workspace --debug-image ubuntu:22.04
   ```
   Creates a new pod with the same volumes mounted, running Ubuntu for better debugging tools.

3. **Preserve debug pod for team inspection:**
   ```bash
   kubectl forge debug my-job --copy-workspace --preserve-pod
   ```
   Creates a debug pod that stays around after you exit, so others can inspect it.

## Examples

### Complete workflow: Build, debug, download

```bash
# 1. Create a ZarfPackageJob
kubectl apply -f zarfpackagejob.yaml

# 2. List jobs to check status
kubectl forge list

# 3. If job failed, debug it
kubectl forge debug my-package-build --failed

# 4. If job succeeded, download the artifacts
kubectl forge download my-package-build --output-dir ./artifacts
```

### Multi-namespace workflow

```bash
# List all jobs across all namespaces
kubectl forge list --all-namespaces

# Download from specific namespace
kubectl forge download my-job -n production

# Debug job in specific namespace
kubectl forge debug my-job -n dev --failed
```

## Troubleshooting

### "No artifact PVC found"

The job may not have completed, or may not produce artifacts. Check:
```bash
kubectl get job <job-name> -o yaml
kubectl get pvc | grep artifact
```

### "No failed pods found"

The pod may have been deleted by TTL or manual cleanup. To prevent this:
1. Set `spec.ttlSecondsAfterFinished` to a higher value in the job spec
2. Use `--preserve-pod` when creating debug pods

### "Timeout waiting for pod to start"

The debug pod may be failing to schedule. Check:
```bash
kubectl get pods | grep debug
kubectl describe pod <debug-pod-name>
```

Common issues:
- Insufficient cluster resources
- Image pull errors
- PVC mount failures

## Architecture

The kubectl-forge plugin consists of:

- **CLI layer** (`cmd/kubectl-forge/`): Cobra-based command-line interface
  - `main.go`: Root command and app entry point
  - `download.go`: Download command implementation
  - `debug.go`: Debug command implementation
  - `list.go`: List command implementation

- **Client library** (`pkg/kubectl/`): Kubernetes client operations
  - `client.go`: Core Kubernetes operations
    - Job/Pod discovery
    - PVC mounting and file transfer
    - Interactive exec
    - Debug pod creation

The plugin uses standard Kubernetes client-go libraries and follows kubectl plugin conventions.

## Development

### Build and test

```bash
# Build
make build-kubectl-forge

# Test the binary
./bin/kubectl-forge --help
./bin/kubectl-forge list --help
```

### Add new commands

1. Create new command file in `cmd/kubectl-forge/`
2. Implement command using Cobra
3. Add command to root in `main.go`
4. Add client functions in `pkg/kubectl/client.go` if needed

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `k8s.io/client-go` - Kubernetes client
- `k8s.io/cli-runtime` - kubectl plugin utilities
