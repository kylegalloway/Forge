# Forge Admission Webhook

The Forge admission webhook validates `ZarfPackageJob` resources before they are created or updated in the cluster.

## Implementation

The webhook is fully implemented and validates:

- ServiceAccount existence and policy annotations
- Allowed actions
- Source patterns (Git repos, S3 buckets, OCI registries)
- Publish destinations
- Deploy targets

See [pkg/webhook/zarfpackage_validator.go](../pkg/webhook/zarfpackage_validator.go) for implementation details.

## Deployment

The webhook is deployed as part of the Forge Helm chart.

Enable/disable via `values.yaml`:
```yaml
webhook:
  enabled: true  # Set to false to disable webhook
  replicaCount: 2
```

## Build

Build webhook image:
```bash
docker build -f Dockerfile.webhook -t ghcr.io/kylegalloway/forge-webhook:latest .
```

## TLS Certificates

The webhook requires TLS certificates. Options:

1. **Auto-generate** (development):
   - Set `webhook.tls.autoGenerate: true` in values.yaml
   - Chart will generate self-signed certs

2. **cert-manager** (production):
   - Install cert-manager
   - Create Certificate resource
   - Chart will reference cert secret

3. **Manual** (production):
   - Generate certs manually
   - Create secret: `kubectl create secret tls forge-webhook-tls --cert=tls.crt --key=tls.key`  # pragma: allowlist secret
   - Set `webhook.tls.secretName: forge-webhook-tls`

## Testing

Test webhook validation:
```bash
# Should succeed
kubectl apply -f examples/samples/build-example.yaml

# Should fail (no ServiceAccount)
kubectl apply -f examples/samples/invalid-no-sa.yaml
```

## Code Location

- Entrypoint: [cmd/webhook/main.go](../cmd/webhook/main.go)
- Validator: [pkg/webhook/zarfpackage_validator.go](../pkg/webhook/zarfpackage_validator.go)
- Policy Engine: [pkg/policy/engine.go](../pkg/policy/engine.go)
- Dockerfile: [Dockerfile.webhook](../Dockerfile.webhook) (root)
