# ScriptRunner Namespace Templates

Templates for creating isolated, multi-tenant namespaces for ScriptRunner users.

## Quick Start

```bash
# Onboard a new user with default quotas
./scripts/onboard-user.sh alice

# Onboard with custom quotas
./scripts/onboard-user.sh bob \
  --max-scriptrunners 100 \
  --max-jobs 50 \
  --cpu-limit 20 \
  --memory-limit 40Gi

# Dry run to see what would be created
./scripts/onboard-user.sh charlie --dry-run
```

## What Gets Created

Each user namespace includes:

### 1. Namespace with Pod Security Standards
- **Pod Security Standards**: Restricted profile enforced
- **Labels**: Tenant identification and webhook enablement
- **Isolation**: Users cannot access other namespaces

### 2. ResourceQuota
Limits total resource consumption:
- ScriptRunner resources (default: 50)
- Concurrent Jobs (default: 20)
- Pods (default: 20)
- CPU requests/limits (default: 5/10 cores)
- Memory requests/limits (default: 10Gi/20Gi)
- Storage (default: 10Gi)
- PVCs (default: 5)

### 3. LimitRange
Sets default and maximum limits for containers:
- **Pod limits**: 50m-4 CPU, 64Mi-8Gi memory
- **Container limits**: 50m-2 CPU, 64Mi-4Gi memory
- **Default resources**: 250m CPU, 256Mi memory (if not specified)
- **PVC limits**: 1Gi-5Gi storage

### 4. RBAC Role
User permissions within their namespace:
- **ScriptRunners**: create, get, list, watch, delete
- **Jobs**: get, list, watch (read-only)
- **Pods**: get, list, watch, logs (read-only)
- **Status**: get, list, watch

### 5. RoleBinding
Binds the user to their namespace role.

## Default Resource Limits

| Resource | Default Value | Configurable |
|----------|---------------|--------------|
| Max ScriptRunners | 50 | `--max-scriptrunners` |
| Max Jobs | 20 | `--max-jobs` |
| Max Pods | 20 | (matches max-jobs) |
| CPU Request | 5 cores | `--cpu-request` |
| CPU Limit | 10 cores | `--cpu-limit` |
| Memory Request | 10Gi | `--memory-request` |
| Memory Limit | 20Gi | `--memory-limit` |
| Storage Request | 10Gi | (auto) |
| Max PVCs | 5 | (auto) |

### Per-Container Defaults

| Resource | Default | Max |
|----------|---------|-----|
| CPU Request | 250m | 2 cores |
| CPU Limit | 1 core | 2 cores |
| Memory Request | 256Mi | 4Gi |
| Memory Limit | 1Gi | 4Gi |

## Manual Template Usage

If you prefer to create namespaces manually:

```bash
# 1. Copy the template
cp config/namespace-templates/user-namespace.yaml /tmp/alice-namespace.yaml

# 2. Replace template variables
sed -i 's/{{ .Username }}/alice/g' /tmp/alice-namespace.yaml
sed -i 's/{{ .MaxScriptRunners | default 50 }}/100/g' /tmp/alice-namespace.yaml
# ... (replace other variables as needed)

# 3. Apply
kubectl apply -f /tmp/alice-namespace.yaml
```

## Customization

### Adjust Quotas for Specific Users

High-usage users (e.g., CI/CD):
```bash
./scripts/onboard-user.sh ci-pipeline \
  --max-scriptrunners 500 \
  --max-jobs 100 \
  --cpu-limit 50 \
  --memory-limit 100Gi
```

Low-usage users (e.g., testing):
```bash
./scripts/onboard-user.sh test-user \
  --max-scriptrunners 10 \
  --max-jobs 5 \
  --cpu-limit 2 \
  --memory-limit 4Gi
```

### Enable Network Policies

Uncomment the NetworkPolicy section in `user-namespace.yaml` to restrict network access.

Default policy allows:
- DNS lookups (kube-system)
- Kubernetes API access
- Add custom egress rules as needed

### Modify Default Limits

Edit the template variables in `user-namespace.yaml` or override in the onboarding script.

## Monitoring Quota Usage

```bash
# View quota status
kubectl describe resourcequota scriptrunner-quota -n user-alice

# View all user namespaces
kubectl get namespaces -l scriptrunner.io/managed=true

# View quota across all users
kubectl get resourcequota --all-namespaces -l scriptrunner.io/managed=true
```

## Offboarding a User

```bash
# Delete the user's namespace (removes all resources)
kubectl delete namespace user-alice

# Or use a script
./scripts/offboard-user.sh alice
```

## Best Practices

1. **Start Conservative**: Begin with default quotas, increase based on usage patterns
2. **Monitor Usage**: Set up alerts when users approach quota limits
3. **Regular Reviews**: Review quotas quarterly, adjust based on needs
4. **Separate Environments**: Use different quotas for dev/staging/prod
5. **Document Limits**: Communicate quotas to users during onboarding
6. **Quota Alerts**: Configure alerts when users hit 80% of quota
7. **Audit Trail**: Log all namespace creations and modifications

## Troubleshooting

### User Cannot Create ScriptRunners

Check quota:
```bash
kubectl describe resourcequota scriptrunner-quota -n user-alice
```

Look for:
- `count/scriptrunners.scriptrunner.io` exceeded
- `count/jobs.batch` exceeded
- CPU/memory limits exceeded

### Pods Not Starting

Check LimitRange:
```bash
kubectl describe limitrange scriptrunner-limits -n user-alice
```

Ensure ScriptRunner resource requests fit within container limits.

### Permission Denied

Check RoleBinding:
```bash
kubectl get rolebinding scriptrunner-user-binding -n user-alice -o yaml
```

Verify user is listed in subjects.

## Integration with Admission Webhook

Namespaces with `scriptrunner.io/webhook: enabled` label will have ScriptRunner resources validated by the admission webhook.

To disable webhook for a namespace:
```bash
kubectl label namespace user-alice scriptrunner.io/webhook=disabled --overwrite
```

## See Also

- [PRODUCTION.md](../../docs/PRODUCTION.md) - Complete production deployment guide
- [USER_GUIDE.md](../../docs/USER_GUIDE.md) - End-user documentation
- [PRODUCTION_CHECKLIST.md](../../docs/PRODUCTION_CHECKLIST.md) - Production readiness tracker
