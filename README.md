# ScriptRunner - Kubernetes Controller

A Kubernetes controller that allows you to run scripts in isolated Jobs using Custom Resources. This controller watches for `ScriptRunner` custom resources and automatically creates Kubernetes Jobs to execute scripts with provided inputs.

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
- Kubernetes cluster (v1.28+)
- kubectl configured to access your cluster
- Docker (for building the controller image)

## Quick Start

### 1. Install the CRD

```bash
make install-crd
```

### 2. Build and Load the Controller Image

```bash
# Build the Docker image
make docker-build

# If using kind or minikube, load the image
kind load docker-image scriptrunner-controller:latest
# OR
minikube image load scriptrunner-controller:latest
```

### 3. Deploy the Controller

```bash
make deploy
```

### 4. Create a ScriptRunner Resource

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

### 5. Check the Results

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

- `make build` - Build the controller binary
- `make docker-build` - Build the Docker image
- `make install-crd` - Install CRDs
- `make deploy` - Deploy the controller
- `make install` - Install CRDs and deploy controller
- `make uninstall` - Remove everything
- `make apply-sample` - Apply sample ScriptRunner
- `make status` - Show status of controller and resources
- `make logs` - Show controller logs
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

## Contributing

This is a learning/experimental project. Feel free to fork and modify for your needs.

## License

See LICENSE file for details.

## Resources

- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [sample-controller](https://github.com/kubernetes/sample-controller)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
