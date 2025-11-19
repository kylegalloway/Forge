# ScriptRunner Admission Webhook

This directory contains a validating admission webhook for ScriptRunner that enforces production security policies.

## Purpose

The webhook provides:

1. **Script Whitelisting**: Only approved `scriptRef` values are allowed
2. **Image Validation**: Only images from approved registries
3. **Input Validation**: Validate input keys and sanitize values
4. **Block Inline Scripts**: Prevent `script` field in production
5. **Set Defaults**: Auto-populate image, resource limits, etc.
6. **Audit Logging**: Log all ScriptRunner creation attempts

## Quick Start

> **Note**: This is a skeleton implementation. For production use, you should implement the full validation logic based on your requirements.

### Prerequisites

- Kubernetes cluster with webhook support
- cert-manager for TLS certificates

### Deploy

```bash
# Install cert-manager (if not already installed)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Deploy the webhook
kubectl apply -f deploy/

# Verify
kubectl get validatingwebhookconfiguration scriptrunner-webhook
```

### Test

```bash
# This should be allowed (approved script)
kubectl apply -f - <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: test-allowed
spec:
  image: your-registry.io/scriptrunner-scripts:v1.0.0
  scriptRef: /scripts/process-data.sh
  inputs:
    test: "value"
EOF

# This should be denied (inline script)
kubectl apply -f - <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: test-denied
spec:
  script: "echo 'not allowed'"
EOF
```

## Configuration

Edit `deploy/webhook-config.yaml` to configure:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: scriptrunner-system
data:
  config.yaml: |
    # Approved scripts that users can run
    approvedScripts:
      - /scripts/process-data.sh
      - /scripts/validate-inputs.sh
      - /scripts/report-status.py

    # Approved image registry prefix
    approvedImagePrefix: your-registry.io/scriptrunner-scripts:

    # Max values to prevent abuse
    maxInputs: 20
    maxInputValueLength: 1000
    maxScriptArgs: 10
    maxScriptArgLength: 200

    # Whether to allow inline scripts (false for production)
    allowInlineScripts: false

    # Default values
    defaults:
      image: your-registry.io/scriptrunner-scripts:v1.0.0
      resources:
        requests:
          cpu: 250m
          memory: 256Mi
        limits:
          cpu: 1000m
          memory: 1Gi
      activeDeadlineSeconds: 600  # 10 minutes
      ttlSecondsAfterFinished: 3600  # 1 hour
```

## Implementation Guide

### Step 1: Implement Validation Logic

Edit `pkg/webhook/validator.go` to add your validation:

```go
func (v *Validator) validateScriptRunner(sr *scriptrunnerv1alpha1.ScriptRunner) error {
    // Check scriptRef is approved
    if !v.isApprovedScript(sr.Spec.ScriptRef) {
        return fmt.Errorf("scriptRef '%s' not in approved list", sr.Spec.ScriptRef)
    }

    // Check image is from approved registry
    if !strings.HasPrefix(sr.Spec.Image, v.config.ApprovedImagePrefix) {
        return fmt.Errorf("image must be from %s", v.config.ApprovedImagePrefix)
    }

    // Validate inputs
    if len(sr.Spec.Inputs) > v.config.MaxInputs {
        return fmt.Errorf("too many inputs (max %d)", v.config.MaxInputs)
    }

    for key, value := range sr.Spec.Inputs {
        if !isValidInputKey(key) {
            return fmt.Errorf("invalid input key: %s", key)
        }
        if len(value) > v.config.MaxInputValueLength {
            return fmt.Errorf("input value too long: %s", key)
        }
    }

    // Block inline scripts if configured
    if !v.config.AllowInlineScripts && sr.Spec.Script != "" {
        return errors.New("inline scripts not allowed in this environment")
    }

    return nil
}
```

### Step 2: Add Default Values

Implement mutation webhook to set defaults:

```go
func (v *Validator) setDefaults(sr *scriptrunnerv1alpha1.ScriptRunner) {
    // Set default image if not specified
    if sr.Spec.Image == "" {
        sr.Spec.Image = v.config.Defaults.Image
    }

    // Add default labels
    if sr.Labels == nil {
        sr.Labels = make(map[string]string)
    }
    sr.Labels["managed-by"] = "scriptrunner-webhook"
}
```

### Step 3: Deploy Webhook

```bash
# Build webhook image
podman build -t your-registry.io/scriptrunner-webhook:v1.0.0 .

# Push to registry
podman push your-registry.io/scriptrunner-webhook:v1.0.0

# Deploy
kubectl apply -f deploy/
```

## Webhook Types

### Validating Webhook

Validates ScriptRunner objects before they are created:

- Checks scriptRef against whitelist
- Validates image registry
- Enforces input constraints
- Blocks inline scripts (optional)

### Mutating Webhook

Modifies ScriptRunner objects before creation:

- Sets default image
- Adds default labels
- Sets resource limits
- Configures TTL

## Security Considerations

1. **TLS Required**: Webhook must use TLS (cert-manager recommended)
2. **Fail-Safe**: Configure webhook to fail closed (reject on error)
3. **Audit Logging**: Log all validation decisions
4. **Rate Limiting**: Prevent DoS attacks on webhook
5. **Namespace Scoped**: Apply webhook rules based on namespace labels

## Testing

```bash
# Unit tests
go test ./pkg/webhook/...

# Integration tests
go test ./test/integration/...

# Manual testing
kubectl apply -f test/fixtures/valid-scriptrunner.yaml
kubectl apply -f test/fixtures/invalid-scriptrunner.yaml
```

## Troubleshooting

### Webhook not being called

```bash
# Check webhook configuration
kubectl get validatingwebhookconfiguration scriptrunner-webhook -o yaml

# Check certificate
kubectl get certificate -n scriptrunner-system

# Check webhook logs
kubectl logs -n scriptrunner-system -l app=scriptrunner-webhook
```

### Certificate issues

```bash
# Recreate certificate
kubectl delete certificate -n scriptrunner-system scriptrunner-webhook-cert
kubectl apply -f deploy/certificate.yaml

# Wait for cert-manager to issue
kubectl wait --for=condition=Ready certificate/scriptrunner-webhook-cert -n scriptrunner-system
```

## Production Deployment

For production use:

1. **High Availability**: Run multiple webhook replicas
2. **Monitoring**: Export metrics for webhook requests
3. **Alerting**: Alert on webhook failures
4. **Resource Limits**: Set appropriate CPU/memory limits
5. **Pod Disruption Budget**: Ensure webhook availability

See [PRODUCTION.md](../docs/PRODUCTION.md) for complete production deployment guide.

## References

- [Kubernetes Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [controller-runtime Webhook](https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
