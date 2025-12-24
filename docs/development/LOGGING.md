# Logging Conventions

This document defines the logging standards and conventions used throughout the Forge codebase.

## Log Levels

Forge uses structured logging via [klog](https://github.com/kubernetes/klog) with verbosity levels to control log output detail.

### Verbosity Level Conventions

| Level | Purpose | When to Use | Examples |
|-------|---------|-------------|----------|
| **Default (V(0))** | Essential operational information | Resource lifecycle events, critical operations, errors | "Executing Zarf Package Build action", "Build job created", errors |
| **V(2)** | Informational details | Reusing resources, skipping operations, non-critical decisions | "Job already exists, reusing", "Attestation not requested, skipping" |
| **V(4)** | Debug information | Detailed troubleshooting, error details, validation failures | "Invalid timeout format, using default", "Failed to read file" |

**Note**: V(6) and higher are reserved for future use (e.g., trace-level debugging).

## Standard Log Message Patterns

### Resource Actions

All action handler log messages MUST include the resource type for clarity:

```go
// Zarf Package actions
klog.InfoS("Executing Zarf Package Build action", "name", pkg.Name, "namespace", pkg.Namespace)
klog.InfoS("Zarf package build job created", "name", pkg.Name, "job", job.Name)

// UDS Bundle actions
klog.InfoS("Executing UDS Bundle Create action", "name", bundle.Name, "namespace", bundle.Namespace)
klog.InfoS("Bundle create job created", "name", bundle.Name, "job", job.Name)
```

### Structured Fields

Use klog's structured logging with key-value pairs:

```go
// Good: Structured with context
klog.InfoS("Job created", "name", jobName, "namespace", namespace, "action", "build")

// Bad: Unstructured string concatenation
klog.Infof("Created job %s in namespace %s for action %s", jobName, namespace, "build")
```

### Standard Field Names

Use consistent field names across the codebase:

| Field | Usage |
|-------|-------|
| `name` | Resource name (package, bundle, job) |
| `namespace` | Kubernetes namespace |
| `action` | Action type (build, publish, deploy, create) |
| `job` | Kubernetes Job name |
| `error` | Error details |
| `timeout` | Timeout value |
| `status` | Status information |

### Error Logging

Errors should be logged with context before being returned:

```go
// Log the error with context
klog.ErrorS(err, "Failed to create build job", "name", pkg.Name, "namespace", pkg.Namespace)

// Return wrapped error
return nil, fmt.Errorf("failed to create build job: %w", err)
```

## Running with Verbose Logging

### Development Mode

Enable debug logging for development:

```bash
# Run controller with debug logging
./controller -v=4

# Run with informational logging
./controller -v=2
```

### Helm Deployment

Set verbosity via Helm values:

```yaml
controller:
  verbosity: 4  # Debug mode
```

### kubectl Logs

Filter logs by verbosity when viewing:

```bash
# View all logs
kubectl logs -n forge-system deploy/forge-controller

# View only errors and essential info (default V(0))
kubectl logs -n forge-system deploy/forge-controller | grep -v "V(2)\|V(4)"

# View with informational details (V(2) and below)
kubectl logs -n forge-system deploy/forge-controller --v=2

# View with debug details (V(4) and below)
kubectl logs -n forge-system deploy/forge-controller --v=4
```

## Best Practices

### DO

- ✅ Use structured logging with key-value pairs
- ✅ Include relevant context (name, namespace, action)
- ✅ Use consistent field names across the codebase
- ✅ Log errors with `ErrorS` before returning them
- ✅ Use appropriate verbosity levels
- ✅ Include resource type in action messages

### DON'T

- ❌ Use unstructured string formatting (`Infof`, `Errorf`)
- ❌ Log sensitive information (credentials, secrets)
- ❌ Log at V(0) for debug information
- ❌ Duplicate error information (log OR return, not both without wrapping)
- ❌ Use inconsistent field names
- ❌ Omit resource type from action messages

## Examples

### Good Logging Examples

```go
// Action execution
klog.InfoS("Executing Zarf Package Deploy action",
    "name", pkg.Name,
    "namespace", pkg.Namespace,
    "artifactPVC", artifactPVCName)

// Informational detail (V(2))
klog.V(2).InfoS("Job already exists, reusing",
    "name", bundle.Name,
    "job", jobName)

// Debug detail (V(4))
klog.V(4).InfoS("Invalid timeout format, using default",
    "timeout", bundle.Spec.Deploy.Timeout,
    "error", parseErr)

// Error with context
klog.ErrorS(err, "Failed to create job",
    "name", pkg.Name,
    "namespace", pkg.Namespace)
```

### Bad Logging Examples

```go
// Bad: No resource type
klog.InfoS("Executing Build action", "name", pkg.Name)

// Bad: Unstructured
klog.Infof("Executing build for package %s in namespace %s", pkg.Name, pkg.Namespace)

// Bad: Inconsistent field names
klog.InfoS("Job created", "packageName", pkg.Name, "ns", pkg.Namespace)

// Bad: Wrong verbosity
klog.V(4).InfoS("Build job created", "job", job.Name) // Should be V(0)
```

## Troubleshooting

### Enabling Debug Logs for Specific Operations

To troubleshoot specific issues, increase verbosity temporarily:

```bash
# Debug timeout parsing issues
kubectl set env -n forge-system deployment/forge-controller KLOG_VERBOSITY=4
kubectl logs -n forge-system -l app=forge-controller -f | grep timeout

# Reset to default
kubectl set env -n forge-system deployment/forge-controller KLOG_VERBOSITY-
```

### Common Log Patterns

Search for common operations in logs:

```bash
# Find all action executions
kubectl logs -n forge-system -l app=forge-controller | grep "Executing.*action"

# Find job reuse events
kubectl logs -n forge-system -l app=forge-controller | grep "already exists, reusing"

# Find timeout-related issues
kubectl logs -n forge-system -l app=forge-controller | grep -i timeout
```

## Future Considerations

- **V(6) - Trace Level**: Reserved for extremely detailed debugging (e.g., request/response payloads, state transitions)
- **Structured Logging Migration**: Potential migration to slog (Go 1.21+) for enhanced structured logging capabilities
- **Log Aggregation**: Integration with logging systems (Loki, Elasticsearch) for centralized log management
