# Contributing to ScriptRunner

Thank you for your interest in contributing to ScriptRunner! This guide will help you set up a local development environment and understand the development workflow.

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Local Testing with Kind](#local-testing-with-kind)
- [Development Workflow](#development-workflow)
- [Modifying the CRD](#modifying-the-crd)
- [Modifying Controller Logic](#modifying-controller-logic)
- [Testing Your Changes](#testing-your-changes)
- [Code Style and Standards](#code-style-and-standards)
- [Submitting Changes](#submitting-changes)
- [Troubleshooting](#troubleshooting)

## Development Environment Setup

### Prerequisites

Install the following tools:

1. **Go** (1.21 or later)
   ```bash
   # macOS
   brew install go

   # Linux - download from https://go.dev/dl/
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   ```

2. **Podman or Docker**
   ```bash
   # macOS - Podman
   brew install podman
   podman machine init
   podman machine start

   # macOS - Docker Desktop
   brew install --cask docker

   # Linux - Podman
   sudo apt-get install podman  # Debian/Ubuntu
   sudo dnf install podman      # Fedora/RHEL
   ```

3. **kubectl**
   ```bash
   # macOS
   brew install kubectl

   # Linux
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
   ```

4. **kind** (Kubernetes in Docker)
   ```bash
   # macOS
   brew install kind

   # Linux
   curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
   chmod +x ./kind
   sudo mv ./kind /usr/local/bin/kind
   ```

### Clone the Repository

```bash
git clone https://github.com/kylegalloway/scriptrunner.git
cd scriptrunner
```

### Verify Your Setup

```bash
# Check Go version
go version

# Check container runtime (podman or docker)
podman --version
# or
docker --version

# Check kubectl
kubectl version --client

# Check kind
kind --version
```

## Local Testing with Kind

Kind (Kubernetes in Docker) provides a lightweight Kubernetes cluster perfect for local development.

### One-Command Setup

The easiest way to get started:

```bash
make kind-setup
```

This single command will:
1. Create a kind cluster named `scriptrunner-dev`
2. Build the controller container image
3. Load the image into the kind cluster
4. Install the CRD
5. Deploy the controller with RBAC
6. Show deployment status

### Step-by-Step Setup

If you prefer manual control:

```bash
# 1. Create kind cluster
make kind-create

# 2. Build controller image
make container-build

# 3. Load image into kind
make kind-load

# 4. Install CRD
make install-crd

# 5. Deploy controller
make deploy

# 6. Check status
make status
```

### Testing Your Setup

```bash
# Apply a sample ScriptRunner
make apply-sample

# Check if the Job was created
kubectl get scriptrunners
kubectl get jobs -l app=scriptrunner

# View logs
make dev-logs
```

Expected output:
```
=== Controller Logs ===
[controller logs showing ScriptRunner detected and Job created]

=== Latest Job Logs ===
Starting script execution...
Inputs received:
INPUT_environment=production
INPUT_message=Hello from ScriptRunner!
INPUT_value=42
Script completed successfully!
```

### Cleaning Up

```bash
# Delete the kind cluster
make kind-delete

# Or just undeploy the controller
make uninstall
```

## Development Workflow

### Quick Iteration Cycle

For rapid development, use the `kind-redeploy` target:

```bash
# Make changes to controller code
vim pkg/controller/simple_controller.go

# Rebuild, reload, and restart in one command
make kind-redeploy
```

This command:
1. Rebuilds the controller binary and image
2. Loads the new image into kind
3. Deletes existing controller pods
4. Waits for new pods to start
5. Shows the current status

### Development Loop Example

```bash
# 1. Make code changes
vim pkg/controller/simple_controller.go

# 2. Test locally (optional but fast)
make test
make vet

# 3. Deploy to kind
make kind-redeploy

# 4. Test with a sample
kubectl apply -f config/samples/scriptrunner_v1alpha1_scriptrunner.yaml

# 5. Check logs
make dev-logs

# 6. Verify behavior
kubectl get scriptrunners
kubectl get jobs
kubectl logs job/<job-name>

# 7. Repeat!
```

## Modifying the CRD

The Custom Resource Definition (CRD) defines the schema for ScriptRunner resources.

### CRD Files to Modify

1. **API Types**: [pkg/apis/scriptrunner/v1alpha1/types.go](pkg/apis/scriptrunner/v1alpha1/types.go)
2. **CRD Manifest**: [config/crd/scriptrunner.io_scriptrunners.yaml](config/crd/scriptrunner.io_scriptrunners.yaml)

### Example: Adding a New Field

Let's add a `timeout` field to the ScriptRunner spec.

#### Step 1: Update the Go Types

Edit `pkg/apis/scriptrunner/v1alpha1/types.go`:

```go
// ScriptRunnerSpec is the spec for a ScriptRunner resource
type ScriptRunnerSpec struct {
	// Inputs are key-value pairs passed to the script
	Inputs map[string]string `json:"inputs,omitempty"`

	// Image is the container image to use for the job (optional, defaults to hardcoded value)
	Image string `json:"image,omitempty"`

	// Script is the shell script to run (optional, defaults to hardcoded value)
	Script string `json:"script,omitempty"`

	// Timeout is the maximum duration for the job in seconds (optional)
	Timeout *int32 `json:"timeout,omitempty"`
}
```

#### Step 2: Update the DeepCopy Functions

Edit `pkg/apis/scriptrunner/v1alpha1/zz_generated.deepcopy.go`:

```go
// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScriptRunnerSpec) DeepCopyInto(out *ScriptRunnerSpec) {
	*out = *in
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(int32)
		**out = **in
	}
	return
}
```

#### Step 3: Update the CRD YAML

Edit `config/crd/scriptrunner.io_scriptrunners.yaml`:

```yaml
spec:
  type: object
  properties:
    inputs:
      type: object
      additionalProperties:
        type: string
      description: Key-value pairs passed as environment variables to the script
    image:
      type: string
      description: Container image to use for the job (defaults to busybox:latest)
    script:
      type: string
      description: Shell script to execute (defaults to a simple echo script)
    timeout:
      type: integer
      format: int32
      description: Maximum duration for the job in seconds
      minimum: 1
      maximum: 86400
```

#### Step 4: Update Controller Logic

Edit `pkg/controller/simple_controller.go` to use the timeout:

```go
func (c *SimpleController) createJob(scriptRunner *scriptrunnerv1alpha1.ScriptRunner, jobName string) *batchv1.Job {
	// ... existing code ...

	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			// ... existing metadata ...
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			// Add timeout if specified
			ActiveDeadlineSeconds: scriptRunner.Spec.Timeout,
			Template: corev1.PodTemplateSpec{
				// ... existing template ...
			},
		},
	}

	return job
}
```

#### Step 5: Test Your Changes

```bash
# Rebuild and deploy
make kind-redeploy

# Update CRD
make install-crd

# Test with a sample that uses the new field
cat <<EOF | kubectl apply -f -
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: test-timeout
spec:
  timeout: 30
  inputs:
    test: "value"
EOF

# Verify the Job has the timeout set
kubectl get job -o yaml | grep activeDeadlineSeconds
```

## Modifying Controller Logic

The controller logic is in [pkg/controller/simple_controller.go](pkg/controller/simple_controller.go).

### Key Components

1. **SimpleController struct** - Main controller with clients and state
2. **Run()** - Main loop that watches ScriptRunner resources
3. **handleScriptRunner()** - Processes each ScriptRunner resource
4. **createJob()** - Creates a Kubernetes Job from a ScriptRunner
5. **updateStatus()** - Updates the ScriptRunner status

### Example: Adding Job Completion Tracking

Let's add logic to update the status when a Job completes.

#### Step 1: Add Job Informer/Watcher

This would require switching to the informer-based controller in `controller.go.example`, but for the simple controller, you could poll Job status:

```go
func (c *SimpleController) handleScriptRunner(ctx context.Context, obj *unstructured.Unstructured) error {
	// ... existing code to create job ...

	// After creating the job, optionally check its status
	if scriptRunner.Status.JobName != "" {
		job, err := c.kubeclientset.BatchV1().Jobs(namespace).Get(ctx, scriptRunner.Status.JobName, metav1.GetOptions{})
		if err == nil {
			// Update status based on job conditions
			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					c.updateStatus(ctx, obj, scriptRunner.Status.JobName, "Completed", "Job completed successfully")
				} else if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					c.updateStatus(ctx, obj, scriptRunner.Status.JobName, "Failed", "Job failed")
				}
			}
		}
	}

	return nil
}
```

#### Step 2: Test Your Changes

```bash
# Make your changes
vim pkg/controller/simple_controller.go

# Run tests
make test

# Lint code
make fmt
make vet

# Deploy to kind
make kind-redeploy

# Create a test resource
make apply-sample

# Watch the status updates
kubectl get scriptrunners -w
```

### Adding Logging

Use the `klog` package for structured logging:

```go
import "k8s.io/klog/v2"

// Info level
klog.Infof("Processing ScriptRunner %s/%s", namespace, name)

// Verbose level (controlled by -v flag)
klog.V(4).Infof("Detailed debug info: %+v", scriptRunner)

// Error level
klog.Errorf("Failed to create job: %v", err)
```

Test different verbosity levels:

```bash
# Run locally with high verbosity
go run cmd/controller/main.go -kubeconfig=$HOME/.kube/config -v=4
```

## Testing Your Changes

### Unit Tests

```bash
# Run all tests
make test

# Run tests for a specific package
go test ./pkg/controller -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

Create a test ScriptRunner and verify behavior:

```bash
# Apply test resource
cat <<EOF | kubectl apply -f -
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: integration-test
spec:
  inputs:
    test_key: "test_value"
  image: "alpine:3.18"
  script: |
    #!/bin/sh
    echo "Testing: \$INPUT_test_key"
    exit 0
EOF

# Wait for job to complete
kubectl wait --for=condition=complete --timeout=60s job -l scriptrunner.io/name=integration-test

# Check job logs
kubectl logs job/integration-test-job-*

# Verify ScriptRunner status
kubectl get scriptrunner integration-test -o jsonpath='{.status}'

# Cleanup
kubectl delete scriptrunner integration-test
```

### Manual Testing Checklist

Before submitting changes, test:

- [ ] Controller starts successfully
- [ ] ScriptRunner resources are detected
- [ ] Jobs are created with correct configuration
- [ ] Environment variables are passed correctly
- [ ] Status is updated appropriately
- [ ] Jobs are cleaned up when ScriptRunner is deleted (owner references)
- [ ] Multiple concurrent ScriptRunners work correctly
- [ ] Invalid inputs are handled gracefully

## Code Style and Standards

### Go Code Style

```bash
# Format all code
make fmt

# Run linter
make vet

# Both together
make test
```

### Code Organization

- Keep controller logic in `pkg/controller/`
- API types in `pkg/apis/scriptrunner/v1alpha1/`
- Kubernetes manifests in `config/`
- Samples in `config/samples/`

### Comments

- Document all exported functions and types
- Use godoc format
- Explain "why" not "what" in implementation comments

Example:
```go
// createJob creates a Kubernetes Job from a ScriptRunner resource.
// The Job will have owner references set to ensure it's cleaned up
// when the ScriptRunner is deleted.
func (c *SimpleController) createJob(scriptRunner *scriptrunnerv1alpha1.ScriptRunner, jobName string) *batchv1.Job {
	// Use default image if not specified to maintain backwards compatibility
	image := scriptRunner.Spec.Image
	if image == "" {
		image = DefaultImage
	}
	// ... rest of implementation
}
```

### Error Handling

Always handle errors appropriately:

```go
// Good
job, err := c.kubeclientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
if err != nil {
	if errors.IsAlreadyExists(err) {
		klog.Infof("Job %s already exists", jobName)
		return nil
	}
	return fmt.Errorf("failed to create job: %w", err)
}

// Bad - silently ignoring errors
job, _ := c.kubeclientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
```

## Submitting Changes

### Before Submitting

1. **Test your changes**
   ```bash
   make test
   make kind-redeploy
   # Manual testing
   ```

2. **Format and lint**
   ```bash
   make fmt
   make vet
   ```

3. **Update documentation**
   - Update README.md if adding features
   - Update this CONTRIBUTING.md if changing workflows
   - Add examples to `config/samples/` if applicable

4. **Commit with clear messages**
   ```bash
   git add .
   git commit -m "Add timeout field to ScriptRunner spec

   - Added timeout field to ScriptRunnerSpec
   - Updated CRD with validation
   - Modified controller to use activeDeadlineSeconds
   - Added sample with timeout configuration"
   ```

### Pull Request Process

1. Fork the repository
2. Create a feature branch
   ```bash
   git checkout -b feature/my-new-feature
   ```
3. Make your changes
4. Push to your fork
   ```bash
   git push origin feature/my-new-feature
   ```
5. Open a Pull Request with:
   - Clear description of changes
   - Motivation/use case
   - Testing performed
   - Screenshots/logs if applicable

## Troubleshooting

### Controller Won't Start

```bash
# Check controller logs
make logs

# Check if CRD is installed
kubectl get crd scriptrunners.scriptrunner.io

# Check RBAC
kubectl auth can-i create jobs --as=system:serviceaccount:scriptrunner-system:scriptrunner-controller -n default
```

### Image Not Loading into Kind

```bash
# List images in kind
docker exec -it scriptrunner-dev-control-plane crictl images | grep scriptrunner

# Manually load image
kind load docker-image scriptrunner-controller:latest --name scriptrunner-dev

# Check if image pull policy is correct in deployment
kubectl get deployment -n scriptrunner-system scriptrunner-controller -o yaml | grep imagePullPolicy
```

### Jobs Not Being Created

```bash
# Check ScriptRunner status
kubectl describe scriptrunner <name>

# Check controller has permissions
kubectl describe clusterrolebinding scriptrunner-controller-rolebinding

# Watch controller logs in real-time
kubectl logs -n scriptrunner-system -l app=scriptrunner-controller -f

# Check for API errors
kubectl get events --sort-by='.lastTimestamp'
```

### CRD Changes Not Taking Effect

```bash
# Reinstall CRD
kubectl delete crd scriptrunners.scriptrunner.io
make install-crd

# Check CRD version
kubectl get crd scriptrunners.scriptrunner.io -o yaml | grep version -A 5
```

### Podman Issues on macOS

```bash
# Start podman machine
podman machine start

# Check connection
podman ps

# If kind doesn't work with podman, set KIND_EXPERIMENTAL_PROVIDER
export KIND_EXPERIMENTAL_PROVIDER=podman
make kind-create
```

### Code-Generated Clients

If you want to use the code-generated clients (shown in `controller.go.example`):

1. Install code-generator tools
   ```bash
   go install k8s.io/code-generator/cmd/...@latest
   ```

2. Run code generation
   ```bash
   ./hack/update-codegen.sh  # You'll need to create this script
   ```

3. Update `cmd/controller/main.go` to use the generated informers

For most development, the simple dynamic client controller is sufficient.

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues and PRs
- Review the [Kubernetes sample-controller](https://github.com/kubernetes/sample-controller) for patterns
- Read the [controller-runtime book](https://book.kubebuilder.io/) for advanced patterns

## Resources

- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Writing Controllers](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/controllers.md)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [client-go Examples](https://github.com/kubernetes/client-go/tree/master/examples)

Happy contributing!

## Working with Pre-built Scripts

ScriptRunner supports two modes for scripts:

1. **Inline scripts** - Defined directly in the ScriptRunner YAML (`spec.script`)
2. **Pre-built scripts** - Referenced from container images (`spec.scriptRef`)

### Creating a Container with Pre-built Scripts

For complex scripts or when you need reusability, build a container image with your scripts:

**Step 1: Create a Dockerfile**

```dockerfile
FROM alpine:3.18

# Install dependencies your scripts need
RUN apk add --no-cache bash curl jq python3

# Copy scripts
COPY scripts/ /scripts/

# Make executable
RUN chmod +x /scripts/*.sh /scripts/*.py

# Add to PATH (optional)
ENV PATH="/scripts:${PATH}"
```

**Step 2: Build and Load**

```bash
# Build the image
podman build -t my-scripts:v1.0 .

# Load into kind for testing
kind load docker-image my-scripts:v1.0 --name scriptrunner-dev
```

**Step 3: Use in ScriptRunner**

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: use-prebuilt-script
spec:
  image: my-scripts:v1.0
  scriptRef: /scripts/my-script.sh
  scriptArgs:
    - "arg1"
    - "arg2"
  inputs:
    key: "value"
```

### Benefits of Pre-built Scripts

- **Reusability**: One script, many ScriptRunners
- **Version Control**: Tag images to version your scripts
- **Testing**: Test scripts in containers before deploying
- **Dependencies**: Install complex dependencies in the image
- **Performance**: Large scripts don't inflate YAML size
- **Multiple Languages**: Python, Ruby, compiled binaries, etc.
- **Security**: Scripts are part of the image build and scanning process

### Complete Example

See [examples/prebuilt-scripts-image/](examples/prebuilt-scripts-image/) for a working example including:

- Bash scripts with argument handling
- Python scripts for complex logic
- Input validation scripts
- Makefile for building and testing
- Sample ScriptRunner manifests

### Testing Pre-built Scripts

Test your scripts locally before using with ScriptRunner:

```bash
# Test directly in the container
podman run --rm my-scripts:v1.0 /scripts/my-script.sh arg1 arg2

# Test with environment variables
podman run --rm \
  -e INPUT_key=value \
  -e SCRIPTRUNNER_NAME=test \
  my-scripts:v1.0 /scripts/my-script.sh
```

### scriptRef vs script

| Aspect | `script` (inline) | `scriptRef` (pre-built) |
|--------|------------------|------------------------|
| Definition | Inline in YAML | Built into container |
| Versioning | Via ScriptRunner resource | Via container tag |
| Size | Limited by YAML size | No practical limit |
| Testing | Must deploy to test | Can test locally |
| Reusability | Copy/paste YAML | Reference same image |
| Languages | Shell scripts | Any executable |
| Dependencies | Limited to image | Full control |

Choose `script` for simple, one-off tasks. Choose `scriptRef` for production workloads with complex logic.

