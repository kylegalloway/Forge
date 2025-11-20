# ScriptRunner Quick Start Guide

## 5-Minute Setup (Kind)

```bash
# 1. Clone and enter directory
git clone https://github.com/kylegalloway/scriptrunner.git
cd scriptrunner

# 2. One-command setup (creates cluster, builds, deploys)
make kind-setup

# 3. Create your first ScriptRunner
make apply-sample

# 4. See it in action
make dev-logs
```

Done! 🎉

## What Just Happened?

1. Created a local Kubernetes cluster using kind
2. Built and deployed the ScriptRunner controller
3. Created a ScriptRunner custom resource
4. Controller detected it and created a Job
5. Job ran a pod with your script and inputs

## Common Commands

```bash
# View all resources
make status

# Create a custom script
kubectl apply -f config/samples/scriptrunner_custom_script.yaml

# Watch logs live
make logs

# Rebuild after code changes
make kind-redeploy

# Run tests
make quick-test
make e2e-test

# Clean up everything
make kind-delete
```

## Your First Custom ScriptRunner

Create a file `my-script.yaml`:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-first-script
spec:
  inputs:
    name: "Your Name"
    action: "test"
  image: "alpine:3.18"
  script: |
    #!/bin/sh
    echo "Hello, $INPUT_name!"
    echo "Running action: $INPUT_action"
    echo "ScriptRunner: $SCRIPTRUNNER_NAME"
```

Apply it:

```bash
kubectl apply -f my-script.yaml
```

Check results:

```bash
# Get ScriptRunner status
kubectl get scriptrunner my-first-script

# Get Job name
kubectl get scriptrunner my-first-script -o jsonpath='{.status.jobName}'

# View logs
kubectl logs job/<job-name>
```

## Development Workflow

### Making Changes to the Controller

```bash
# 1. Edit controller code
vim pkg/controller/controller.go

# 2. Rebuild and redeploy
make kind-redeploy

# 3. Test your changes
make quick-test

# 4. Try a real example
make apply-sample
make dev-logs
```

### Making Changes to the CRD

```bash
# 1. Edit the CRD types
vim pkg/apis/scriptrunner/v1alpha1/types.go

# 2. Edit the CRD manifest
vim config/crd/scriptrunner.io_scriptrunners.yaml

# 3. Update controller to use new fields
vim pkg/controller/controller.go

# 4. Reinstall CRD and redeploy
make install-crd
make kind-redeploy

# 5. Test with updated sample
kubectl apply -f my-updated-sample.yaml
```

## Container Runtime

The project automatically detects and uses:
- **podman** (preferred if available)
- **docker** (fallback)

Override if needed:

```bash
# Force using docker
make container-build CONTAINER_RUNTIME=docker

# Force using podman
make container-build CONTAINER_RUNTIME=podman
```

## Troubleshooting

### Controller won't start

```bash
# Check controller logs
make logs

# Verify image loaded
docker exec -it scriptrunner-dev-control-plane crictl images | grep scriptrunner
```

### Jobs not created

```bash
# Check ScriptRunner status
kubectl describe scriptrunner <name>

# Check RBAC
kubectl auth can-i create jobs --as=system:serviceaccount:scriptrunner-system:scriptrunner-controller
```

### Start fresh

```bash
# Delete cluster and recreate
make kind-delete
make kind-setup
```

## Next Steps

- Read [CONTRIBUTING.md](CONTRIBUTING.md) for detailed development guide
- Read [README.md](README.md) for complete documentation
- Check [config/samples/](config/samples/) for more examples
- Run `make help` to see all available commands

## Environment Variables in Scripts

Your scripts automatically get:

- `INPUT_<key>` - Each input becomes INPUT_KEYNAME
- `SCRIPTRUNNER_NAME` - Name of the ScriptRunner resource
- `SCRIPTRUNNER_NAMESPACE` - Namespace of the resource

Example:

```yaml
spec:
  inputs:
    user: "alice"
    count: "10"
```

In your script:
```bash
echo "User: $INPUT_user"      # Prints: User: alice
echo "Count: $INPUT_count"    # Prints: Count: 10
```

## Useful Kubectl Commands

```bash
# List all ScriptRunners
kubectl get scriptrunners

# Watch ScriptRunners (updates live)
kubectl get scriptrunners -w

# Describe a ScriptRunner (shows events)
kubectl describe scriptrunner <name>

# Get ScriptRunner YAML
kubectl get scriptrunner <name> -o yaml

# Get status only
kubectl get scriptrunner <name> -o jsonpath='{.status}'

# List all Jobs created by ScriptRunner
kubectl get jobs -l app=scriptrunner

# Delete all ScriptRunners
kubectl delete scriptrunners --all
```

## Tips

1. **Use `kind-redeploy` for fast iteration** - rebuilds and restarts in seconds
2. **Check `dev-logs` for quick debugging** - shows controller and latest job
3. **Run `quick-test` before committing** - catches basic issues
4. **Run `e2e-test` for comprehensive validation** - tests all functionality
5. **Use `make status` to see everything at once** - controller, resources, jobs

## Help

```bash
# Show all make targets
make help

# Get kubectl help
kubectl explain scriptrunner
kubectl explain scriptrunner.spec
kubectl explain scriptrunner.status
```

Happy ScriptRunning! 🚀
