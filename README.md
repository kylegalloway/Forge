# ScriptRunner - Kubernetes Controller

A Kubernetes controller that allows you to run scripts in isolated Jobs using Custom Resources. This controller watches for `ScriptRunner` custom resources and automatically creates Kubernetes Jobs to execute scripts with provided inputs.

> **Quick Start**: New to ScriptRunner? Check out [QUICKSTART.md](QUICKSTART.md) for a 5-minute getting started guide!

## Overview

ScriptRunner is designed for high-throughput script execution in Kubernetes. Instead of the controller processing tasks directly, it creates Jobs for each ScriptRunner resource, allowing for better parallelization and resource management.

### Key Features

- **Custom Resource-based**: Define script executions declaratively using Kubernetes CRs
- **Job-based Execution**: Each ScriptRunner creates a Job, enabling high throughput and parallel processing
- **Flexible Input**: Pass key-value pairs as environment variables to your scripts
- **Customizable**: Override default container images and scripts per resource
- **Owner References**: Jobs are owned by their ScriptRunner resources for automatic cleanup

## Architecture

```
ScriptRunner CR → Controller watches → Creates Job → Pod runs script
```

1. User creates a `ScriptRunner` custom resource with inputs
2. Controller detects the new resource and creates a Kubernetes Job
3. Job spawns a Pod that runs the script with inputs as environment variables
4. Controller updates the ScriptRunner status with Job information

## Project Structure

```
.
├── cmd/
│   └── controller/
│       └── main.go                 # Controller entry point
├── pkg/
│   ├── apis/
│   │   └── scriptrunner/
│   │       └── v1alpha1/           # API types and registration
│   └── controller/
│       ├── simple_controller.go    # Controller implementation
│       └── controller.go.example   # Example with generated clients
├── config/
│   ├── crd/                        # Custom Resource Definition
│   ├── rbac/                       # RBAC manifests
│   ├── manager/                    # Controller deployment
│   └── samples/                    # Sample ScriptRunner resources
├── Dockerfile                      # Multi-stage build for controller
└── Makefile                        # Build and deployment targets
```

## Prerequisites

- Go 1.21 or later
- Kubernetes cluster (v1.28+) or [kind](https://kind.sigs.k8s.io/) for local development
- kubectl configured to access your cluster
- Podman or Docker (for building the controller image)

> **Note**: The project uses podman by default if available, falling back to docker. All commands work with either container runtime.

## Quick Start

### Local Development with Kind (Recommended)

The fastest way to get started is using [kind](https://kind.sigs.k8s.io/):

```bash
# Complete setup: create cluster, build, and deploy
make kind-setup

# Create a sample ScriptRunner
make apply-sample

# Check status
make status

# View logs
make dev-logs
```

That's it! See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed development workflows.

### Manual Setup (Existing Cluster)

If you have an existing Kubernetes cluster:

#### 1. Install the CRD

```bash
make install-crd
```

#### 2. Build and Load the Controller Image

```bash
# Build the container image (uses podman if available, otherwise docker)
make container-build

# If using kind, load the image
kind load docker-image scriptrunner-controller:latest --name scriptrunner-dev

# If using minikube
minikube image load scriptrunner-controller:latest
```

#### 3. Deploy the Controller

```bash
make deploy
```

#### 4. Create a ScriptRunner Resource

```bash
# Apply the sample
make apply-sample

# Or create your own
kubectl apply -f - <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-script
  namespace: default
spec:
  inputs:
    message: "Hello World"
    user: "Alice"
EOF
```

#### 5. Check the Results

```bash
# View ScriptRunner resources
kubectl get scriptrunners

# View created Jobs
kubectl get jobs -l app=scriptrunner

# View Job logs
kubectl logs -l app=scriptrunner
```

## Usage Examples

### Basic Example with Default Script

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: example-scriptrunner
  namespace: default
spec:
  inputs:
    message: "Hello from ScriptRunner!"
    environment: "production"
    value: "42"
```

The default script will print all environment variables with the `INPUT_` prefix.

### Custom Script Example

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: custom-script
  namespace: default
spec:
  inputs:
    name: "John Doe"
    task: "data-processing"
    count: "100"
  image: "alpine:3.18"
  script: |
    #!/bin/sh
    echo "Processing task: $INPUT_task"
    echo "User: $INPUT_name"
    for i in $(seq 1 ${INPUT_count}); do
      echo "Processing item $i"
    done
    echo "Complete!"
```

### Environment Variables Available in Scripts

Scripts have access to:

- `INPUT_<key>`: Each key from `spec.inputs` is available as `INPUT_<KEY>`
- `SCRIPTRUNNER_NAME`: Name of the ScriptRunner resource
- `SCRIPTRUNNER_NAMESPACE`: Namespace of the ScriptRunner resource

## Development

### Running Locally

```bash
# Run the controller locally (outside the cluster)
make run
```

### Building

```bash
# Build the controller binary
make build

# Build the Docker image
make docker-build
```

### Testing

```bash
# Run tests
make test

# Format code
make fmt

# Run linter
make vet
```

## Makefile Targets

### Development
- `make build` - Build the controller binary
- `make run` - Run controller locally (outside cluster)
- `make test` - Run tests
- `make fmt` - Format code
- `make vet` - Run go vet

### Container Images
- `make container-build` - Build container image (podman/docker)
- `make container-push` - Push container image
- `make docker-build` - Legacy alias for container-build
- `make docker-push` - Legacy alias for container-push

### Deployment
- `make install-crd` - Install CRDs
- `make deploy` - Deploy controller
- `make install` - Install CRDs and deploy controller
- `make uninstall` - Remove everything

### Kind (Local Development)
- `make kind-setup` - Complete kind setup (create cluster + deploy)
- `make kind-create` - Create kind cluster
- `make kind-delete` - Delete kind cluster
- `make kind-load` - Build and load image into kind
- `make kind-deploy` - Build, load, and deploy to kind
- `make kind-redeploy` - Quick rebuild and restart (for iteration)

### Samples & Status
- `make apply-sample` - Apply sample ScriptRunner
- `make apply-custom-sample` - Apply custom script sample
- `make delete-samples` - Delete sample resources
- `make status` - Show status of controller and resources
- `make logs` - Show controller logs (follow mode)
- `make dev-logs` - Show controller and latest job logs

### Cleanup
- `make clean` - Clean built binaries
- `make help` - Display all available targets

## Configuration

### Controller Configuration

The controller can be configured using command-line flags:

- `-kubeconfig`: Path to kubeconfig file (for out-of-cluster operation)
- `-master`: Kubernetes API server address
- `-namespace`: Namespace to watch (empty = all namespaces)
- `-v`: Log verbosity level

### ScriptRunner Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `inputs` | map[string]string | No | {} | Key-value pairs passed as environment variables |
| `image` | string | No | `busybox:latest` | Container image to use |
| `script` | string | No | Default echo script | Shell script to execute |

### ScriptRunner Status

| Field | Description |
|-------|-------------|
| `phase` | Current phase (e.g., "JobCreated") |
| `jobName` | Name of the created Job |
| `message` | Additional status information |
| `lastUpdateTime` | Last status update timestamp |

## Advanced Usage

### Using with Code-Generated Clients

The project includes an example (`pkg/controller/controller.go.example`) showing how to use Kubernetes code-generated clients for better type safety and performance. To use this approach:

1. Set up code-generation tools
2. Generate clientset, informers, and listers
3. Update main.go to use the generated controller

### Customizing the Default Script

Edit `pkg/controller/simple_controller.go` and modify the `DefaultScript` constant:

```go
const DefaultScript = `#!/bin/sh
# Your custom default script here
`
```

### High Throughput Scenarios

For high-throughput scenarios with many ScriptRunner resources:

1. Increase controller replicas in `config/manager/deployment.yaml`
2. Adjust resource limits based on your needs
3. Consider namespace-scoped controllers for better isolation
4. Monitor Job completion and cleanup old Jobs regularly

## Cleanup

```bash
# Delete sample resources
make delete-samples

# Uninstall controller and CRDs
make uninstall
```

## Troubleshooting

### Controller not starting

```bash
# Check controller logs
make logs

# Check RBAC permissions
kubectl auth can-i create jobs --as=system:serviceaccount:scriptrunner-system:scriptrunner-controller
```

### Jobs not being created

```bash
# Check ScriptRunner status
kubectl describe scriptrunner <name>

# Check controller logs
make logs
```

### Pods failing

```bash
# Check Job status
kubectl get jobs -l app=scriptrunner

# Check pod logs
kubectl logs -l app=scriptrunner
```

## Development Scripts

The `scripts/` directory contains helpful utilities for development:

- **[scripts/dev-setup.sh](scripts/dev-setup.sh)** - Automated setup of complete dev environment
  ```bash
  ./scripts/dev-setup.sh
  ```

- **[scripts/quick-test.sh](scripts/quick-test.sh)** - Quick smoke test to verify controller works
  ```bash
  ./scripts/quick-test.sh [namespace]
  ```

- **[scripts/test-e2e.sh](scripts/test-e2e.sh)** - Comprehensive end-to-end test suite
  ```bash
  ./scripts/test-e2e.sh [namespace]
  ```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for detailed information on:

- Setting up your development environment
- Local testing with kind
- Modifying the CRD
- Modifying controller logic
- Testing workflows
- Code style guidelines
- Submitting pull requests

## License

See LICENSE file for details.

## Resources

- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [sample-controller](https://github.com/kubernetes/sample-controller)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
