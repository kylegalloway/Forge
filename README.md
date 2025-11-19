# ScriptRunner - Production-Ready Kubernetes Script Execution Platform

A secure, multi-tenant Kubernetes controller for running scripts in isolated Jobs. ScriptRunner provides enterprise-grade script execution with comprehensive security, multi-tenancy, and observability features.

> **Quick Start**: New to ScriptRunner? Check out [QUICKSTART.md](QUICKSTART.md) for a 5-minute getting started guide!

## Overview

ScriptRunner enables declarative script execution in Kubernetes with production-ready security controls. Users define script executions as custom resources, and the controller creates isolated Jobs with comprehensive security hardening, resource quotas, and network policies.

### Key Features

**Core Functionality:**
- **Custom Resource-based**: Define script executions declaratively using Kubernetes CRs
- **Job-based Execution**: Each ScriptRunner creates a Job, enabling high throughput and parallel processing
- **Flexible Input**: Pass key-value pairs as environment variables with automatic validation
- **Pre-built Scripts**: Reference versioned scripts in container images (scriptRef)
- **Owner References**: Automatic cleanup with TTL-based Job deletion

**Security & Multi-Tenancy (Production-Ready):**
- **Admission Webhook**: Validates scripts, images, and inputs before execution
- **Multi-tenant Isolation**: Namespace-based tenancy with ResourceQuotas and RBAC
- **Network Policies**: Default-deny networking with explicit DNS and API access
- **Image Security**: Registry whitelisting, vulnerability scanning, and pull policy enforcement
- **Pod Security Standards**: Restricted profile with non-root users and dropped capabilities

**Observability:**
- **Health Checks**: Liveness and readiness probes on controller and webhook
- **Structured Logging**: klog-based logging with InfoS/ErrorS patterns
- **Metrics Ready**: Prometheus annotations and placeholder endpoints

## Architecture

### Basic Flow

```
User → Admission Webhook → ScriptRunner CR → Controller → Job → Pod
         (validates)         (namespace)      (watches) (creates) (executes)
```

1. **User creates ScriptRunner** in their namespace with inputs
2. **Admission Webhook validates**:
   - Image from approved registry
   - Script in whitelist (if using scriptRef)
   - Inputs sanitized (no command injection patterns)
   - Mutations applied (default values)
3. **Controller watches** and detects new ScriptRunner
4. **Job created** with security context and resource limits
5. **Pod executes script** in isolated environment with NetworkPolicy
6. **Controller updates status** with Job information and completion state

### Security Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Admission Webhook Layer                     │
│  ✓ Registry whitelist  ✓ Input sanitization                 │
│  ✓ Script whitelist    ✓ Image validation                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   Namespace Isolation                        │
│  ✓ RBAC (user-scoped)   ✓ ResourceQuota                     │
│  ✓ LimitRange           ✓ NetworkPolicy (default-deny)      │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    Job Pod Security                          │
│  ✓ Non-root user        ✓ Read-only filesystem              │
│  ✓ Dropped capabilities ✓ Seccomp profile                   │
│  ✓ Resource limits      ✓ TTL cleanup (1 hour)              │
└─────────────────────────────────────────────────────────────┘
```

## Project Structure

```
.
├── cmd/
│   ├── controller/
│   │   └── main.go                        # Controller entry point
│   └── webhook/
│       └── main.go                        # Admission webhook server
├── pkg/
│   ├── apis/scriptrunner/v1alpha1/       # API types and registration
│   ├── controller/
│   │   └── simple_controller.go          # Controller with smart ImagePullPolicy
│   └── webhook/
│       └── validator.go                   # Validation logic
├── config/
│   ├── crd/                               # Custom Resource Definition
│   ├── rbac/                              # Controller RBAC
│   ├── manager/                           # Controller deployment
│   ├── samples/                           # Sample ScriptRunner resources
│   ├── namespace-templates/               # Multi-tenancy templates
│   │   ├── user-namespace.yaml            # ResourceQuota, LimitRange, PSS
│   │   ├── user-rbac.yaml                 # User roles and bindings
│   │   └── network-policy.yaml            # Default-deny networking
│   └── network/
│       ├── controller-network-policy.yaml # Controller network policies
│       └── README.md                      # NetworkPolicy documentation
├── webhook/
│   ├── deploy/                            # Webhook deployment manifests
│   │   ├── webhook-deployment.yaml        # HA deployment (2 replicas)
│   │   ├── webhook-configuration.yaml     # ValidatingWebhook + MutatingWebhook
│   │   ├── webhook-config.yaml            # ConfigMap (registry whitelist, etc.)
│   │   └── certificate.yaml               # cert-manager Certificate + Issuer
│   ├── Dockerfile                         # Webhook container build
│   └── README.md                          # Webhook documentation
├── scripts/
│   ├── onboard-user.sh                    # Automated user onboarding
│   ├── dev-setup.sh                       # Dev environment setup
│   ├── quick-test.sh                      # Smoke tests
│   └── test-e2e.sh                        # E2E test suite
├── docs/
│   ├── PRODUCTION_CHECKLIST.md            # 160-item production tracker
│   ├── PRODUCTION.md                      # Production deployment guide
│   ├── RBAC_SECURITY.md                   # RBAC security model
│   ├── IMAGE_SECURITY.md                  # Container image security
│   ├── USER_GUIDE.md                      # End-user documentation
│   └── CLIENT_VALIDATION.md               # IDE integration and validation
├── examples/
│   └── prebuilt-scripts-image/            # Example container with scripts
├── Dockerfile                             # Controller multi-stage build
└── Makefile                               # Build and deployment targets
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

### Custom Inline Script Example

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

### Pre-built Script Reference Example

Instead of inline scripts, you can reference scripts built into your container image:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: prebuilt-script
  namespace: default
spec:
  # Custom image with pre-built scripts
  image: scriptrunner-scripts:latest

  # Reference a script in the container
  scriptRef: /scripts/process-data.sh

  # Pass arguments to the script
  scriptArgs:
    - "batch-process"
    - "20"

  # Input variables still work
  inputs:
    environment: "production"
    source: "database"
```

See [examples/prebuilt-scripts-image/](examples/prebuilt-scripts-image/) for a complete example of building a container with pre-built scripts.

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
| `script` | string | No | Default echo script | Inline shell script to execute (mutually exclusive with `scriptRef`) |
| `scriptRef` | string | No | - | Path to pre-built script in container (mutually exclusive with `script`) |
| `scriptArgs` | []string | No | [] | Arguments to pass when using `scriptRef` |

### ScriptRunner Status

| Field | Description |
|-------|-------------|
| `phase` | Current phase (e.g., "JobCreated") |
| `jobName` | Name of the created Job |
| `message` | Additional status information |
| `lastUpdateTime` | Last status update timestamp |

## Advanced Usage

### Using Pre-built Scripts

ScriptRunner supports two ways to provide scripts:

1. **Inline Scripts** (`script` field): Define the script directly in the ScriptRunner YAML
2. **Pre-built Scripts** (`scriptRef` field): Reference scripts built into your container image

#### Why Use Pre-built Scripts?

- **Reusability**: Share scripts across multiple ScriptRunners
- **Version Control**: Scripts are versioned with container images
- **Testing**: Test scripts independently before deploying
- **Performance**: No need to inline large scripts in YAML
- **Security**: Scripts can be reviewed and scanned during image build
- **Complex Dependencies**: Support multiple languages and libraries

#### Creating a Container with Pre-built Scripts

See the complete example in [examples/prebuilt-scripts-image/](examples/prebuilt-scripts-image/):

```dockerfile
FROM alpine:3.18
RUN apk add --no-cache bash curl jq python3
COPY scripts/ /scripts/
RUN chmod +x /scripts/*.sh
ENV PATH="/scripts:${PATH}"
```

Build and load into kind:
```bash
cd examples/prebuilt-scripts-image
make build
make load  # Loads into kind cluster
```

#### Using Pre-built Scripts

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-prebuilt-script
spec:
  image: scriptrunner-scripts:latest
  scriptRef: /scripts/process-data.sh
  scriptArgs:
    - "arg1"
    - "arg2"
  inputs:
    key: "value"
```

**Script Execution:**
- If `scriptRef` is specified, it's executed directly with `scriptArgs` as arguments
- INPUT_ environment variables are still available
- If `script` is specified instead, it's passed to `/bin/sh -c`
- Only one of `script` or `scriptRef` can be used

See the [examples](config/samples/) directory for more scriptRef examples.

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

## Production Deployment

ScriptRunner is designed for production use with comprehensive security and multi-tenancy features.

### Documentation

- **[Production Readiness Checklist](docs/PRODUCTION_CHECKLIST.md)** - Comprehensive tracker with:
  - 10 batches from Foundation to Launch Readiness
  - 160 checklist items with priorities and dependencies
  - Current progress: **56/160 items (35%)** complete
  - ✅ Batch 1: Foundation (Health checks, security, docs)
  - ✅ Batch 2: Validation & Multi-Tenancy (Webhook, RBAC, namespaces)
  - ✅ Batch 3: Network & Image Security (NetworkPolicy, image scanning)

- **[Production Guide](docs/PRODUCTION.md)** - Complete deployment guide covering security, multi-tenancy, and monitoring

- **[RBAC Security Model](docs/RBAC_SECURITY.md)** - Comprehensive RBAC documentation:
  - Security architecture and boundaries
  - User roles and privilege escalation prevention
  - Threat model with 6 attack scenarios
  - Audit and monitoring recommendations

- **[Image Security](docs/IMAGE_SECURITY.md)** - Container image security guide:
  - Vulnerability scanning (Trivy, Clair, cloud providers)
  - Image signing (cosign, notary)
  - Registry whitelisting and authentication
  - Smart ImagePullPolicy enforcement

- **[Network Policies](config/network/README.md)** - Network isolation guide:
  - Default-deny networking with explicit allow rules
  - CNI compatibility matrix
  - Testing and troubleshooting

- **[User Guide](docs/USER_GUIDE.md)** - Documentation for end users

- **[Webhook Implementation](webhook/README.md)** - Admission webhook for validation and defaults

### Quick Production Setup

```bash
# 1. Deploy controller with security hardening
make install

# 2. Deploy admission webhook (validates scripts and images)
kubectl apply -f webhook/deploy/

# 3. Onboard a user with quotas, RBAC, and network policies
./scripts/onboard-user.sh alice \
  --max-scriptrunners 50 \
  --max-jobs 20 \
  --cpu-limit 10 \
  --memory-limit 20Gi

# 4. User creates ScriptRunner in their namespace
kubectl apply -f my-script.yaml -n user-alice
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

## Client-Side Validation

For better developer experience, ScriptRunner provides client-side validation options:

### Quick Validation with kubectl

```bash
# Validate before applying
kubectl apply --dry-run=server -f my-scriptrunner.yaml
```

### IDE Integration (Recommended)

Get autocomplete and real-time validation in VS Code, IntelliJ, and other editors:

```bash
# Generate JSON schema
./scripts/generate-json-schema.sh

# Configure VS Code (one-time)
cp .vscode/settings.json.example .vscode/settings.json
```

Now your IDE will provide:
- Autocomplete for field names
- Inline documentation
- Real-time validation
- Type checking

See [CLIENT_VALIDATION.md](docs/CLIENT_VALIDATION.md) for complete setup options including:
- JSON Schema generation and hosting
- Pre-commit hooks
- kubeconform for offline validation
- Custom CLI tools

