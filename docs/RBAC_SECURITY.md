# RBAC and Security Model

This document describes the ScriptRunner RBAC (Role-Based Access Control) security model and how it enforces multi-tenant isolation.

## Security Goals

1. **Namespace Isolation**: Users can only access their own namespace
2. **No Privilege Escalation**: Users cannot gain cluster-admin or modify RBAC
3. **Protected Controller**: Users cannot modify the controller, CRD, or webhooks
4. **Least Privilege**: Users get only the permissions they need
5. **Auditability**: All actions are logged and can be audited

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Cluster-Wide Resources                    │
│  (Only accessible by cluster-admin)                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  scriptrunner-system namespace                       │    │
│  │  - Controller Deployment                             │    │
│  │  - Webhook Deployment                                │    │
│  │  - ServiceAccounts (controller, webhook)             │    │
│  │  - ClusterRole/ClusterRoleBindings                   │    │
│  │  (Users have NO access)                              │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  CustomResourceDefinition                            │    │
│  │  - scriptrunners.scriptrunner.io                     │    │
│  │  (Users can CREATE resources, but cannot modify CRD)│    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  ValidatingWebhookConfiguration                      │    │
│  │  MutatingWebhookConfiguration                        │    │
│  │  (Users have NO access)                              │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    User Namespaces                           │
│  (Scoped RBAC - users only access their own)                 │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  user-alice namespace                                │    │
│  │  - ScriptRunner resources (CRUD)                     │    │
│  │  - Jobs (read-only, created by controller)           │    │
│  │  - Pods (read-only, for debugging)                   │    │
│  │  - ResourceQuota (enforced by k8s)                   │    │
│  │  - LimitRange (enforced by k8s)                      │    │
│  │  - Role: scriptrunner-user                           │    │
│  │  - RoleBinding: alice → scriptrunner-user            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  user-bob namespace                                  │    │
│  │  (Same structure, isolated from alice)               │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## RBAC Roles

### Controller ServiceAccount (Cluster-Wide)

**Location**: `scriptrunner-system` namespace
**Bound to**: ClusterRole `scriptrunner-controller-role`

**Permissions**:
```yaml
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners", "scriptrunners/status"]
  verbs: ["get", "list", "watch", "update", "patch"]

- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch"]

- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

**Why these permissions**:
- `watch` ScriptRunners to detect new resources
- `update/patch` ScriptRunner status to report job creation
- `create` Jobs to execute scripts
- `create/patch` Events for observability

**Notable exclusions**:
- NO `delete` on Jobs (cleanup via TTL)
- NO `update/patch` on Jobs (immutable after creation)
- NO `delete` on ScriptRunners (users can delete their own)

### Webhook ServiceAccount (Cluster-Wide)

**Location**: `scriptrunner-system` namespace
**Bound to**: ClusterRole `scriptrunner-webhook-role`

**Permissions**:
```yaml
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners"]
  verbs: ["get", "list"]
```

**Why these permissions**:
- `get/list` ScriptRunners to validate against existing resources
- NO write permissions (webhook only validates/mutates admission requests)

### User Role (Namespace-Scoped)

**Location**: Each user namespace (`user-<username>`)
**Bound to**: User via RoleBinding

**Permissions**:
```yaml
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]

- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners/status"]
  verbs: ["get", "list", "watch"]

- apiGroups: ["batch"]
  resources: ["jobs", "jobs/status"]
  verbs: ["get", "list", "watch"]

- apiGroups: [""]
  resources: ["pods", "pods/log", "events"]
  verbs: ["get", "list", "watch"]
```

**Why these permissions**:
- Full CRUD on ScriptRunners in their namespace
- Read-only on ScriptRunner status (controller owns this)
- Read-only on Jobs (controller creates these)
- Read-only on Pods and logs (for debugging script execution)
- Read-only on Events (for troubleshooting)

**Notable exclusions**:
- NO access to other namespaces
- NO ClusterRole or ClusterRoleBinding permissions
- NO ability to create/modify Roles or RoleBindings
- NO access to Secrets (except via ScriptRunner inputs)
- NO access to ConfigMaps (except those mounted by controller)
- NO access to ServiceAccounts
- NO access to controller namespace (scriptrunner-system)
- NO access to CRD definition
- NO access to webhook configurations

## Security Boundaries

### What Users CAN Do ✅

1. **Create ScriptRunners** in their namespace
   ```bash
   kubectl apply -f my-scriptrunner.yaml -n user-alice
   ```

2. **View their ScriptRunners**
   ```bash
   kubectl get scriptrunners -n user-alice
   ```

3. **Update/Delete their ScriptRunners**
   ```bash
   kubectl delete scriptrunner my-task -n user-alice
   ```

4. **View Jobs created by their ScriptRunners**
   ```bash
   kubectl get jobs -n user-alice
   ```

5. **View Pod logs for debugging**
   ```bash
   kubectl logs -n user-alice scriptrunner-my-task-abc123
   ```

6. **Check resource quota usage**
   ```bash
   kubectl describe resourcequota -n user-alice
   ```

### What Users CANNOT Do ❌

1. **Access other user namespaces**
   ```bash
   kubectl get scriptrunners -n user-bob  # DENIED
   ```

2. **Modify the controller**
   ```bash
   kubectl delete deployment scriptrunner-controller -n scriptrunner-system  # DENIED
   ```

3. **Modify the CRD**
   ```bash
   kubectl edit crd scriptrunners.scriptrunner.io  # DENIED
   ```

4. **Create or modify RBAC**
   ```bash
   kubectl create clusterrole evil-admin  # DENIED
   kubectl create rolebinding escalate -n user-alice  # DENIED
   ```

5. **Modify webhooks**
   ```bash
   kubectl delete validatingwebhookconfiguration scriptrunner-webhook  # DENIED
   ```

6. **Create Jobs directly** (only controller can)
   ```bash
   kubectl create job my-job --image=alpine -n user-alice  # Allowed by k8s, but not useful
   ```
   Note: Users CAN create Jobs, but the controller won't manage them

7. **Modify ResourceQuota or LimitRange**
   ```bash
   kubectl edit resourcequota scriptrunner-quota -n user-alice  # DENIED
   ```

8. **Access controller ServiceAccount tokens**
   ```bash
   kubectl get secret -n scriptrunner-system  # DENIED
   ```

## Security Enforcement Layers

ScriptRunner uses defense-in-depth with multiple security layers:

### Layer 1: Kubernetes RBAC
- **Enforcement**: Kubernetes API server
- **Controls**: Namespace isolation, resource access
- **Bypass difficulty**: Requires cluster-admin or RBAC misconfiguration

### Layer 2: Admission Webhook
- **Enforcement**: Validating/Mutating webhooks
- **Controls**: Script whitelist, image registry, input validation
- **Bypass difficulty**: Requires webhook deletion (needs cluster-admin)

### Layer 3: ResourceQuota
- **Enforcement**: Kubernetes quota admission controller
- **Controls**: Resource consumption limits
- **Bypass difficulty**: Requires ResourceQuota modification (needs cluster-admin)

### Layer 4: LimitRange
- **Enforcement**: Kubernetes limit admission controller
- **Controls**: Default and maximum resource requests/limits
- **Bypass difficulty**: Requires LimitRange modification (needs cluster-admin)

### Layer 5: Pod Security Standards
- **Enforcement**: Pod Security admission controller
- **Controls**: Privileged containers, host access, capabilities
- **Bypass difficulty**: Requires namespace label modification (needs cluster-admin)

### Layer 6: Network Policies (Optional)
- **Enforcement**: CNI plugin (Calico, Cilium, etc.)
- **Controls**: Network egress/ingress
- **Bypass difficulty**: Requires NetworkPolicy modification (needs cluster-admin)

## Privilege Escalation Prevention

### Scenario 1: User tries to create cluster-admin RoleBinding
```bash
kubectl create clusterrolebinding evil --clusterrole=cluster-admin --user=alice
```
**Result**: DENIED (no ClusterRoleBinding create permission)

### Scenario 2: User tries to modify their Role
```bash
kubectl edit role scriptrunner-user -n user-alice
```
**Result**: DENIED (no Role edit permission in their namespace)

### Scenario 3: User tries to create a new RoleBinding to escalate
```bash
kubectl create rolebinding admin --clusterrole=admin --user=alice -n user-alice
```
**Result**: DENIED (no RoleBinding create permission)

### Scenario 4: User tries to access controller ServiceAccount token
```bash
kubectl get secret scriptrunner-controller-token-xyz -n scriptrunner-system
```
**Result**: DENIED (no access to scriptrunner-system namespace)

### Scenario 5: User tries to disable webhook
```bash
kubectl delete validatingwebhookconfiguration scriptrunner-webhook
```
**Result**: DENIED (no cluster-wide delete permission)

### Scenario 6: User tries to modify CRD to remove validation
```bash
kubectl edit crd scriptrunners.scriptrunner.io
```
**Result**: DENIED (no CRD edit permission)

## Audit and Monitoring

### Recommended Audit Policy

Enable Kubernetes audit logging for these resources:

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all requests to ScriptRunner resources
- level: RequestResponse
  resources:
  - group: scriptrunner.io
    resources: ["scriptrunners"]

# Log RBAC changes
- level: RequestResponse
  resources:
  - group: rbac.authorization.k8s.io
    resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]

# Log webhook changes
- level: RequestResponse
  resources:
  - group: admissionregistration.k8s.io
    resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
```

### Monitoring Queries

**Detect privilege escalation attempts**:
```bash
kubectl logs -n scriptrunner-system deployment/scriptrunner-controller | grep "RBAC: access denied"
```

**View user activity**:
```bash
kubectl get events -n user-alice --sort-by='.lastTimestamp'
```

**Check quota violations**:
```bash
kubectl get events -A --field-selector reason=FailedCreate | grep quota
```

**View webhook denials**:
```bash
kubectl logs -n scriptrunner-system deployment/scriptrunner-webhook | grep "denied"
```

## Best Practices

### For Cluster Admins

1. **Never grant cluster-admin to users**: Use the provided namespace-scoped roles
2. **Audit RBAC regularly**: Review ClusterRoleBindings and RoleBindings
3. **Monitor webhook health**: Ensure webhooks are running and validating
4. **Enable audit logging**: Track all ScriptRunner and RBAC operations
5. **Rotate certificates**: Use cert-manager to auto-rotate webhook TLS certs
6. **Review quotas periodically**: Adjust based on user needs and cluster capacity

### For Users

1. **Use scriptRef, not inline scripts**: Inline scripts may be blocked in production
2. **Request quota increases properly**: Don't try to bypass quotas
3. **Report issues via proper channels**: Don't attempt to self-service cluster-admin tasks
4. **Use namespaced kubectl commands**: Always specify `-n user-<username>`
5. **Check quota before creating resources**:
   ```bash
   kubectl describe resourcequota -n user-alice
   ```

## Troubleshooting

### "Forbidden: User cannot create resource"

**Symptom**: User gets RBAC error when trying to create ScriptRunner

**Diagnosis**:
```bash
kubectl auth can-i create scriptrunners -n user-alice --as=alice
```

**Solutions**:
1. Verify RoleBinding exists: `kubectl get rolebinding -n user-alice`
2. Verify Role exists: `kubectl get role scriptrunner-user -n user-alice`
3. Check if user is bound correctly: `kubectl get rolebinding -n user-alice -o yaml`

### "Webhook denied the request"

**Symptom**: ScriptRunner creation blocked by webhook

**Diagnosis**: Check webhook logs
```bash
kubectl logs -n scriptrunner-system deployment/scriptrunner-webhook
```

**Common causes**:
1. Script not in whitelist (check webhook config)
2. Image not from approved registry
3. Suspicious patterns in inputs (`;`, `|`, `$()`, etc.)
4. Inline script blocked (use scriptRef instead)

### "Quota exceeded"

**Symptom**: Cannot create ScriptRunner due to quota

**Diagnosis**:
```bash
kubectl describe resourcequota scriptrunner-quota -n user-alice
```

**Solutions**:
1. Delete old ScriptRunners: `kubectl delete scriptrunner <name> -n user-alice`
2. Wait for TTL cleanup of Jobs (auto-deletes after 1 hour)
3. Request quota increase from cluster admin

## Security Considerations

### Threat Model

**Trusted**:
- Cluster administrators
- ScriptRunner controller
- Admission webhook
- Kubernetes control plane

**Untrusted**:
- ScriptRunner users
- Script execution (job pods)
- User-provided inputs
- Referenced container images

**Attack Scenarios**:
1. **Malicious user tries to escalate privileges** → Blocked by RBAC
2. **Malicious user tries to consume all cluster resources** → Blocked by ResourceQuota
3. **Malicious user tries to run unapproved script** → Blocked by webhook
4. **Malicious user tries to inject commands via inputs** → Blocked by webhook input validation
5. **Malicious user tries to access other namespaces** → Blocked by RBAC
6. **Compromised job pod tries to access API** → Limited by ServiceAccount permissions (none by default)
7. **Compromised job pod tries to access host** → Blocked by Pod Security Standards

### Known Limitations

1. **No fine-grained scriptRef permissions**: All users can use any whitelisted script
2. **No per-user quotas**: Quotas are per-namespace, not per-user within a namespace
3. **No network policies by default**: Must be configured separately
4. **Webhook can be bypassed if namespace labels are modified**: Requires cluster-admin
5. **Users can create arbitrary Jobs**: Jobs won't be managed by controller, but still consume quota

## References

- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [ResourceQuotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/)
- [LimitRanges](https://kubernetes.io/docs/concepts/policy/limit-range/)
- [Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
