# Network Policies for ScriptRunner

This directory contains NetworkPolicy resources to secure ScriptRunner deployments.

## Overview

Network policies implement **deny-by-default** network security:
- By default, Kubernetes allows all pod-to-pod traffic
- NetworkPolicies restrict traffic to only what's necessary
- Reduces attack surface and limits lateral movement

## Prerequisites

**CNI Plugin with NetworkPolicy Support Required:**

Supported CNI plugins:
- ✅ Calico
- ✅ Cilium
- ✅ Weave Net
- ✅ Antrea
- ✅ Kube-router
- ❌ Flannel (requires additional components)
- ❌ Basic kubenet (no support)

Check your CNI:
```bash
kubectl get pods -n kube-system | grep -E 'calico|cilium|weave|antrea'
```

If you don't have a supported CNI, NetworkPolicies will be **accepted but not enforced**.

## Policies

### Controller Namespace (`scriptrunner-system`)

**File:** `controller-network-policy.yaml`

**Policies:**
1. `deny-all-ingress` - Deny all ingress to controller pods
2. `controller-allow-api` - Allow controller egress to Kubernetes API and DNS
3. `webhook-allow-api-ingress` - Allow webhook ingress from API server on port 8443
4. `webhook-allow-api-egress` - Allow webhook egress to Kubernetes API and DNS

**Apply:**
```bash
kubectl apply -f config/network/controller-network-policy.yaml
```

**Verify:**
```bash
# Check policies exist
kubectl get networkpolicy -n scriptrunner-system

# Test controller can create Jobs
kubectl apply -f config/samples/scriptrunner_v1alpha1_scriptrunner.yaml
kubectl get jobs -A

# Check logs for errors
kubectl logs -n scriptrunner-system deployment/scriptrunner-controller
kubectl logs -n scriptrunner-system deployment/scriptrunner-webhook
```

### User Namespaces

**Template File:** `../namespace-templates/network-policy.yaml`

**Policies:**
1. `deny-all-ingress` - Deny all ingress to job pods
2. `allow-dns` - Allow egress to DNS (kube-system/kube-dns)
3. `allow-kubernetes-api` - Allow egress to Kubernetes API server

**Additional Policies (commented out, enable as needed):**
- `allow-metrics-service` - Access to Prometheus/metrics
- `allow-external-https` - HTTPS to internet (use with caution)
- `allow-database-access` - Access to specific database subnets

**Auto-applied by onboarding script:**
```bash
./scripts/onboard-user.sh alice
```

**Manual application:**
```bash
# Replace {{ .Username }} with actual username
sed 's/{{ .Username }}/alice/g' config/namespace-templates/network-policy.yaml | kubectl apply -f -
```

**Verify:**
```bash
# Check policies
kubectl get networkpolicy -n user-alice

# Test DNS works
kubectl run test --rm -it --image=alpine -n user-alice -- nslookup kubernetes

# Test external HTTPS is blocked (if not explicitly allowed)
kubectl run test --rm -it --image=curlimages/curl -n user-alice -- curl -I https://google.com
# Should timeout or fail
```

## Default Behavior

With the default policies, job pods can:
- ✅ Query DNS (kube-system CoreDNS)
- ✅ Access Kubernetes API (ports 443, 6443)
- ❌ Access other pods in the same namespace
- ❌ Access pods in other namespaces
- ❌ Access external IPs/internet
- ❌ Receive any incoming connections

## Customization Guide

### Allow Access to Internal Services

**Scenario:** Job pods need to send metrics to Prometheus

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-prometheus
  namespace: user-alice
spec:
  podSelector:
    matchLabels:
      app: scriptrunner
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    - podSelector:
        matchLabels:
          app: prometheus
    ports:
    - protocol: TCP
      port: 9090
```

### Allow Access to External APIs

**Scenario:** Job pods need to call external REST APIs

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-https
  namespace: user-alice
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 169.254.169.254/32  # Block AWS metadata
        - 10.0.0.0/8          # Block RFC1918
        - 172.16.0.0/12
        - 192.168.0.0/16
    ports:
    - protocol: TCP
      port: 443
```

### Allow Access to Databases

**Scenario:** Job pods need to query a PostgreSQL database

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-postgres
  namespace: user-alice
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - ipBlock:
        cidr: 10.100.5.0/24  # Database subnet
    ports:
    - protocol: TCP
      port: 5432
```

## Testing

### 1. Test DNS Resolution
```bash
kubectl run test --rm -it --image=alpine -n user-alice -- nslookup kubernetes.default.svc.cluster.local
```

**Expected:** Success (DNS allowed)

### 2. Test Blocked Internet Access
```bash
kubectl run test --rm -it --image=curlimages/curl -n user-alice -- curl -m 5 https://google.com
```

**Expected:** Timeout (external HTTPS blocked by default)

### 3. Test Kubernetes API Access
```bash
kubectl run test --rm -it --image=bitnami/kubectl -n user-alice -- kubectl get pods
```

**Expected:** Success or RBAC error (network allows it, RBAC may deny)

### 4. Test Blocked Pod-to-Pod Access
```bash
# Terminal 1: Start a simple HTTP server
kubectl run server --image=nginx -n user-alice

# Terminal 2: Try to access it
kubectl run test --rm -it --image=curlimages/curl -n user-alice -- curl http://server
```

**Expected:** Timeout (ingress denied by deny-all-ingress policy)

### 5. Verify Controller Still Works
```bash
# Create a ScriptRunner
kubectl apply -f config/samples/scriptrunner_v1alpha1_scriptrunner.yaml

# Check Job was created
kubectl get jobs -A

# Check controller logs
kubectl logs -n scriptrunner-system deployment/scriptrunner-controller
```

**Expected:** Job created successfully

## Troubleshooting

### NetworkPolicies Not Enforced

**Symptom:** Policies exist but traffic isn't blocked

**Cause:** CNI doesn't support NetworkPolicy

**Fix:**
```bash
# Check CNI
kubectl get pods -n kube-system

# If using Flannel, switch to Calico or Cilium
# Or install Calico on top of Flannel:
kubectl apply -f https://docs.projectcalico.org/manifests/canal.yaml
```

### Controller Can't Create Jobs

**Symptom:** Controller logs show API errors

**Diagnosis:**
```bash
kubectl logs -n scriptrunner-system deployment/scriptrunner-controller
```

**Fix:** Verify controller egress allows Kubernetes API
```bash
kubectl get networkpolicy controller-allow-api -n scriptrunner-system -o yaml
```

### Webhook Rejecting All Requests

**Symptom:** All ScriptRunner creations fail with webhook timeout

**Diagnosis:**
```bash
kubectl logs -n scriptrunner-system deployment/scriptrunner-webhook
```

**Fix:** Verify webhook ingress allows API server
```bash
kubectl get networkpolicy webhook-allow-api-ingress -n scriptrunner-system -o yaml
```

### Job Pods Can't Resolve DNS

**Symptom:** Jobs fail with "Name or service not known"

**Fix:** Verify DNS policy
```bash
kubectl get networkpolicy allow-dns -n user-alice -o yaml

# Check kube-dns label
kubectl get pods -n kube-system -l k8s-app=kube-dns --show-labels
```

### Jobs Need Internet Access

**Scenario:** Jobs legitimately need external HTTPS

**Solution:** Uncomment `allow-external-https` in network-policy.yaml or add custom policy

**Security Note:** Be cautious - this allows exfiltration

## Security Considerations

### Defense-in-Depth Layers

1. **NetworkPolicy** - Restricts network traffic
2. **RBAC** - Restricts API access
3. **Pod Security Standards** - Restricts pod capabilities
4. **ResourceQuota** - Restricts resource consumption
5. **Admission Webhook** - Validates inputs and scripts

All layers work together. NetworkPolicy failure doesn't compromise RBAC or other layers.

### Threat Scenarios

| Threat | Mitigation |
|--------|------------|
| Compromised job pod scans internal network | Egress blocked to internal IPs |
| Compromised job pod exfiltrates data | Egress blocked to internet (by default) |
| Compromised job pod attacks other tenants | Ingress/egress blocked between namespaces |
| Compromised job pod attacks controller | Ingress to controller namespace denied |
| Compromised job pod accesses cloud metadata | AWS/GCP metadata IPs blocked in examples |
| Lateral movement after pod compromise | Pod can only access DNS and API server |

### Known Limitations

1. **NetworkPolicy is not firewall-grade**: Determined attacker with cluster-admin can disable
2. **No layer 7 filtering**: Can't block based on HTTP headers, only IPs/ports
3. **DNS allowed to all pods**: Can't distinguish between legitimate and malicious DNS queries
4. **Kubernetes API access required**: Jobs may not need it, but blocking it breaks some patterns
5. **CNI-dependent**: Policies not enforced without compatible CNI

### Best Practices

1. **Start with deny-all, add only required access**
2. **Use specific selectors (app: scriptrunner) over empty selectors where possible**
3. **Block cloud metadata services (169.254.169.254)**
4. **Log NetworkPolicy violations** (if CNI supports it - Cilium, Calico Enterprise)
5. **Audit policies regularly**
6. **Test in staging before production**
7. **Document all allowed traffic in comments**

## References

- [Kubernetes NetworkPolicy Documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [NetworkPolicy Recipes](https://github.com/ahmetb/kubernetes-network-policy-recipes)
- [Calico NetworkPolicy Tutorial](https://docs.projectcalico.org/security/tutorials/kubernetes-policy-basic)
- [Cilium NetworkPolicy](https://docs.cilium.io/en/stable/policy/)

## Support Matrix

| CNI Plugin | NetworkPolicy Support | Notes |
|------------|----------------------|-------|
| Calico | ✅ Full | Recommended, supports logging |
| Cilium | ✅ Full | Advanced features, eBPF-based |
| Weave Net | ✅ Full | Good performance |
| Antrea | ✅ Full | VMware-backed, good for vSphere |
| Kube-router | ✅ Full | Lightweight |
| Flannel | ❌ No | Can add Calico on top (Canal) |
| AWS VPC CNI | ⚠️ Partial | Requires security groups for pods |
| Azure CNI | ⚠️ Partial | Requires Azure Network Policies |
| GKE CNI | ✅ Full | Dataplane V2 (Cilium-based) |
