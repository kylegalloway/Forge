# Structured Logging Implementation

**Status:** ✅ Complete
**Date:** January 5, 2026

## Overview

Implemented structured logging for the Forge Kubernetes operator with correlation ID support, context-aware metadata extraction, and automatic sensitive data redaction.

---

## Features

### 1. Structured Logging Package

Created `pkg/logging` with a structured logger that wraps klog/v2 and provides:

- **Component-based logging**: Each logger has a component identifier
- **Context-aware logging**: Automatically extracts metadata from context
- **Multiple log levels**: Info, Error, Warning, Debug (V4), Trace (V5)
- **Key-value pairs**: Structured logging with key-value arguments
- **Thread-safe**: Safe for concurrent use across goroutines

### 2. Correlation ID Support

Enables request tracing across the entire job lifecycle:

```go
// Generate correlation ID from job metadata
correlationID := logging.GenerateCorrelationID(namespace, jobName, action)
// Result: "default/my-job/Build"

// Add to context
ctx := logging.WithCorrelationID(ctx, correlationID)

// All log messages will include correlationID automatically
logger.Info(ctx, "Job started")
// Output: component="controller" correlationID="default/my-job/Build" msg="Job started"
```

**Benefits:**
- Trace a single job across multiple controller reconciliations
- Correlate logs from different components (controller, webhook, job pods)
- Debug multi-action jobs (BuildPublishDeploy) by following the same ID

### 3. Context Propagation

Automatically includes job metadata in all log messages:

```go
// Add context
ctx = logging.WithJobName(ctx, "my-job")
ctx = logging.WithNamespace(ctx, "production")
ctx = logging.WithAction(ctx, "Deploy")

// Log message
logger.Info(ctx, "Processing job", "replicas", 3)

// Output includes all context automatically:
// component="controller" namespace="production" job="my-job" action="Deploy"
// msg="Processing job" replicas=3
```

**Context keys:**
- `correlationID` - Unique identifier for request tracing
- `jobName` - Name of the ZarfPackageJob or UDSBundleJob
- `namespace` - Kubernetes namespace
- `action` - Job action (Build, Deploy, Publish, etc.)

### 4. Sensitive Data Redaction

Automatically redacts sensitive information from logs to prevent credential leakage:

**Redaction by key name:**
```go
logger.Info(ctx, "Config loaded",
    "password", "secretPassword123",
    "token", "Bearer abc123",
    "api_key", "sk-1234567890")

// Output:
// password="***REDACTED***" token="***REDACTED***" api_key="***REDACTED***"
```

**Redacted key names:**
- `password`, `secret`, `token`
- `apikey`, `api_key`, `api-key`
- `access_key`, `secret_key`
- `credentials`, `auth`, `authorization`, `bearer`
- `ssh_key`, `private_key`, `certificate`
- `kubeconfig`, `dockerconfigjson`

**Redaction by pattern matching:**
```go
logger.Info(ctx, "Environment",
    "vars", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE") // pragma: allowlist secret

// Output:
// vars="***REDACTED***"
```

**Detected patterns:**
- AWS credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- Generic secrets (`password:`, `token:`, `secret:`, `api_key:`)
- GitHub tokens (ghp_, ghs_)
- Docker auth fields

---

## Usage

### Basic Logging

```go
import "github.com/kylegalloway/forge/pkg/logging"

// Create logger
logger := logging.NewLogger("my-component")

// Log messages
ctx := context.Background()
logger.Info(ctx, "Starting operation", "count", 5)
logger.Error(ctx, err, "Operation failed", "reason", "timeout")
logger.Warning(ctx, "Resource limit reached", "limit", "10Gi")
logger.Debug(ctx, "Debug information", "details", debugData)  // V(4)
logger.Trace(ctx, "Trace information", "stack", stack)        // V(5)
```

### With Correlation IDs

```go
// Generate correlation ID
correlationID := logging.GenerateCorrelationID("default", "my-job", "Build")

// Add to context
ctx := logging.WithCorrelationID(context.Background(), correlationID)

// All subsequent logs will include the correlation ID
logger.Info(ctx, "Job started")
logger.Info(ctx, "Build phase complete")
logger.Info(ctx, "Job finished")

// Retrieve correlation ID from context
id := logging.GetCorrelationID(ctx)
```

### With Job Context

```go
// Build context with all job metadata
ctx := context.Background()
ctx = logging.WithCorrelationID(ctx, correlationID)
ctx = logging.WithJobName(ctx, pkg.Name)
ctx = logging.WithNamespace(ctx, pkg.Namespace)
ctx = logging.WithAction(ctx, pkg.Spec.Action)

// Log includes all metadata automatically
logger.Info(ctx, "Processing job")
```

---

## Integration

### Controller Integration

The controller main.go now uses structured logging:

```go
// Initialize logger
logger := logging.NewLogger("forge-controller")

// Use throughout application
logger.Info(ctx, "Starting Forge controllers")
logger.Info(ctx, "Watching namespace", "namespace", watchNamespace)
logger.Error(ctx, err, "Health server failed")
```

### Webhook Integration

The webhook server uses structured logging:

```go
// Initialize logger
logger := logging.NewLogger("forge-webhook")

// Add to webhook server
server := &WebhookServer{
    logger: logger,
}

// Use in validation
ws.logger.Info(ctx, "Validating ZarfPackageJob",
    "name", request.Name,
    "namespace", request.Namespace,
    "operation", request.Operation)
```

---

## Testing

Comprehensive test coverage with 8 test suites:

### Test Suites

1. **TestNewLogger** - Logger creation
2. **TestWithContext** - Context metadata extraction (6 subtests)
3. **TestRedactSensitiveData** - Sensitive data redaction (16 subtests)
4. **TestCorrelationIDHelpers** - Correlation ID helpers
5. **TestGenerateCorrelationID** - ID generation
6. **TestContextHelpers** - Context helper functions
7. **TestSensitivePatterns** - Pattern matching (9 subtests)
8. **TestSensitiveKeys** - Key-based redaction (24 subtests)

### Test Results

```bash
$ go test ./pkg/logging/... -v
PASS
ok      github.com/kylegalloway/forge/pkg/logging    0.813s
```

All tests passing ✅

---

## Files

### Created Files

- `pkg/logging/logger.go` (210 lines)
  - Logger struct and methods
  - Context helpers
  - Redaction logic
  - Sensitive data patterns

- `pkg/logging/logger_test.go` (400+ lines)
  - Comprehensive test coverage
  - 8 test suites
  - 55+ individual test cases

### Modified Files

- `cmd/controller/main.go`
  - Integrated structured logger
  - Replaced klog calls with logger methods
  - Added context propagation

- `cmd/webhook/main.go`
  - Integrated structured logger
  - Added logger to WebhookServer
  - Context-aware validation logging

---

## Example Output

### Without Structured Logging (Before)

```text
I0105 12:34:56.123456       1 controller.go:123] Starting controller
I0105 12:34:57.234567       1 controller.go:234] Processing job my-job
E0105 12:34:58.345678       1 controller.go:345] Job failed: timeout
```

### With Structured Logging (After)

```text
I0105 12:34:56.123456 component="forge-controller" msg="Starting controller"
I0105 12:34:57.234567 component="forge-controller" correlationID="default/my-job/Build"
  namespace="default" job="my-job" action="Build" msg="Processing job"
E0105 12:34:58.345678 component="forge-controller" correlationID="default/my-job/Build"
  namespace="default" job="my-job" action="Build" msg="Job failed" error="timeout"
```

**Benefits:**
- Searchable by component, namespace, job name
- Traceable via correlation ID
- Structured key-value pairs for easy parsing
- Automatic sensitive data redaction

---

## Best Practices

### 1. Always Use Context

```go
// Good
logger.Info(ctx, "Message", "key", "value")

// Avoid (context info won't be included)
klog.Info("Message")
```

### 2. Add Correlation IDs for Jobs

```go
// Generate at job start
correlationID := logging.GenerateCorrelationID(namespace, name, action)
ctx := logging.WithCorrelationID(ctx, correlationID)

// Pass context through entire job lifecycle
```

### 3. Use Appropriate Log Levels

- **Info**: Normal operation, important events
- **Error**: Errors that need attention
- **Warning**: Potential issues, degraded state
- **Debug (V4)**: Detailed debugging information
- **Trace (V5)**: Very detailed trace information

### 4. Redaction is Automatic

Don't manually redact - the logger does it automatically:

```go
// The logger handles this automatically
logger.Info(ctx, "Credentials loaded", "password", userPassword)
// Output: password="***REDACTED***"
```

### 5. Use Structured Key-Value Pairs

```go
// Good - structured
logger.Info(ctx, "Job completed",
    "duration", duration,
    "artifacts", artifactCount,
    "status", "success")

// Avoid - unstructured string formatting
logger.Info(ctx, fmt.Sprintf("Job completed in %v with %d artifacts", duration, artifactCount))
```

---

## Performance Considerations

- **Zero allocation** for log calls when log level is disabled
- **Context values** are extracted once per log call
- **Pattern matching** only runs on string values
- **Redaction** creates a copy of the slice, original data unchanged

---

## Security

### Prevents Credential Leakage

The redaction system protects against accidental logging of:

- ✅ Passwords and secrets
- ✅ API keys and tokens
- ✅ AWS credentials
- ✅ GitHub tokens
- ✅ Kubernetes configs
- ✅ Docker registry auth
- ✅ SSH keys and certificates

### Limitations

- Redaction is **best-effort** - always review logs before sharing
- Custom secret formats may not be detected
- Use secret management systems (Vault, etc.) for sensitive data
- Never log raw Secret data from Kubernetes

---

## Future Enhancements

Potential improvements for future iterations:

1. **JSON output format** - Structured JSON logs for log aggregation
2. **OpenTelemetry integration** - Export structured logs to OTLP
3. **Dynamic log levels** - Change log level without restart
4. **Log sampling** - Reduce high-volume log noise
5. **Custom redaction patterns** - User-configurable patterns
6. **Audit logging** - Separate audit trail for compliance

---

## References

- [klog/v2 Documentation](https://github.com/kubernetes/klog)
- [Structured Logging Guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md)
- [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
