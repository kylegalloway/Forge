# Production Deployment Guide

This guide covers the requirements and best practices for running ScriptRunner in production, allowing external users to execute your pre-built scripts with their own inputs.

## Table of Contents

- [Security Model](#security-model)
- [Multi-Tenancy Setup](#multi-tenancy-setup)
- [Validation and Constraints](#validation-and-constraints)
- [Resource Management](#resource-management)
- [Monitoring and Observability](#monitoring-and-observability)
- [Script Image Management](#script-image-management)
- [User Access Patterns](#user-access-patterns)
- [Example Production Setup](#example-production-setup)

## Security Model

### Key Security Concerns

When allowing users to run scripts, you must control:

1. **What scripts can be executed** (whitelist approved scripts)
2. **What inputs are allowed** (validate and sanitize)
3. **Resource consumption** (prevent DoS via resource limits)
4. **Access control** (who can create ScriptRunners)
5. **Network access** (what the scripts can reach)
6. **Data access** (what data scripts can access)

### Recommended Architecture

```
User → RBAC → Admission Webhook → ScriptRunner → Job (with restricted script image)
         ↓           ↓                              ↓
    Namespace  Validation                    Pod Security
    Isolation  + Defaults                     Standards
```

## Multi-Tenancy Setup

### Namespace Isolation

Each user/team should have their own namespace:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: user-alice
  labels:
    scriptrunner.io/tenant: alice
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: scriptrunner-quota
  namespace: user-alice
spec:
  hard:
    # Limit number of ScriptRunners
    count/scriptrunners.scriptrunner.io: "10"
    # Limit Jobs
    count/jobs.batch: "20"
    # Limit pods
    pods: "20"
    # CPU/Memory limits
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: scriptrunner-limits
  namespace: user-alice
spec:
  limits:
  - max:
      cpu: "2"
      memory: "4Gi"
    min:
      cpu: "100m"
      memory: "128Mi"
    default:
      cpu: "500m"
      memory: "512Mi"
    defaultRequest:
      cpu: "250m"
      memory: "256Mi"
    type: Container
```

### RBAC Per Namespace

Users should only be able to create ScriptRunners, not modify the controller or CRD:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scriptrunner-user
  namespace: user-alice
rules:
# Allow managing ScriptRunners
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners"]
  verbs: ["create", "get", "list", "watch", "delete"]
# Allow reading ScriptRunner status
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners/status"]
  verbs: ["get", "list", "watch"]
# Allow viewing Jobs and Pods (for debugging)
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: alice-scriptrunner-user
  namespace: user-alice
subjects:
- kind: User
  name: alice
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: scriptrunner-user
  apiGroup: rbac.authorization.k8s.io
```

## Validation and Constraints

### Admission Webhook (Recommended)

Create a validating admission webhook to enforce policies:

**Key Validations:**
1. **Script whitelist**: Only allow approved `scriptRef` values
2. **Image whitelist**: Only allow your approved images
3. **Input validation**: Validate input keys and values
4. **Prevent inline scripts**: Block `script` field in production
5. **Set defaults**: Auto-populate image, resource limits, etc.

**Example Webhook Logic:**

```go
// Validate scriptRef is in approved list
approvedScripts := map[string]bool{
    "/scripts/process-data.sh":     true,
    "/scripts/validate-inputs.sh":  true,
    "/scripts/report-status.py":    true,
}

if !approvedScripts[scriptRunner.Spec.ScriptRef] {
    return admission.Denied("scriptRef not in approved list")
}

// Validate image is from your registry
allowedImagePrefix := "your-registry.io/scriptrunner-scripts:"
if !strings.HasPrefix(scriptRunner.Spec.Image, allowedImagePrefix) {
    return admission.Denied("image must be from approved registry")
}

// Validate inputs
for key := range scriptRunner.Spec.Inputs {
    if !isValidInputKey(key) {
        return admission.Denied(fmt.Sprintf("invalid input key: %s", key))
    }
}

// Block inline scripts in production
if scriptRunner.Spec.Script != "" {
    return admission.Denied("inline scripts not allowed in production")
}
```

See [webhook/](../webhook/) for a complete webhook implementation example.

### CRD-level Validation

Add validation to the CRD itself:

```yaml
spec:
  type: object
  required: ["scriptRef", "image"]  # Make these required
  properties:
    scriptRef:
      type: string
      pattern: '^/scripts/[a-z0-9-]+\.(sh|py)$'  # Enforce path format
    image:
      type: string
      pattern: '^your-registry\.io/scriptrunner-scripts:v[0-9]+\.[0-9]+\.[0-9]+$'
    inputs:
      type: object
      maxProperties: 20  # Limit number of inputs
      additionalProperties:
        type: string
        maxLength: 1000  # Limit input value size
    scriptArgs:
      type: array
      maxItems: 10  # Limit number of arguments
      items:
        type: string
        maxLength: 200
  # Prevent inline scripts
  not:
    required: ["script"]
```

## Resource Management

### Controller Resource Limits

Update the controller deployment to handle production load:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scriptrunner-controller
  namespace: scriptrunner-system
spec:
  replicas: 3  # Multiple replicas for HA
  template:
    spec:
      containers:
      - name: controller
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: "2"
            memory: 2Gi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
```

### Job Resource Limits

Modify controller to set default resource limits on Jobs:

```go
// In createJob function
job := &batchv1.Job{
    Spec: batchv1.JobSpec{
        // Cleanup completed jobs automatically
        TTLSecondsAfterFinished: pointer.Int32(3600), // 1 hour

        // Limit job duration
        ActiveDeadlineSeconds: pointer.Int64(600), // 10 minutes max

        Template: corev1.PodTemplateSpec{
            Spec: corev1.PodSpec{
                Containers: []corev1.Container{
                    {
                        Resources: corev1.ResourceRequirements{
                            Requests: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("250m"),
                                corev1.ResourceMemory: resource.MustParse("256Mi"),
                            },
                            Limits: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("1000m"),
                                corev1.ResourceMemory: resource.MustParse("1Gi"),
                            },
                        },
                        // Security context
                        SecurityContext: &corev1.SecurityContext{
                            RunAsNonRoot:             pointer.Bool(true),
                            RunAsUser:                pointer.Int64(1000),
                            AllowPrivilegeEscalation: pointer.Bool(false),
                            ReadOnlyRootFilesystem:   pointer.Bool(true),
                            Capabilities: &corev1.Capabilities{
                                Drop: []corev1.Capability{"ALL"},
                            },
                        },
                    },
                },
            },
        },
    },
}
```

### Network Policies

Restrict network access for Job pods:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: scriptrunner-jobs
  namespace: user-alice
spec:
  podSelector:
    matchLabels:
      app: scriptrunner
  policyTypes:
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow specific external services only
  - to:
    - podSelector:
        matchLabels:
          app: approved-api
    ports:
    - protocol: TCP
      port: 443
```

## Monitoring and Observability

### Metrics to Track

Add Prometheus metrics to the controller:

```go
var (
    scriptRunnersCreated = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "scriptrunner_created_total",
            Help: "Total number of ScriptRunners created",
        },
        []string{"namespace", "script"},
    )

    jobsCreated = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "scriptrunner_jobs_created_total",
            Help: "Total number of Jobs created",
        },
        []string{"namespace", "script"},
    )

    jobDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "scriptrunner_job_duration_seconds",
            Help:    "Duration of Job execution",
            Buckets: prometheus.DefBuckets,
        },
        []string{"namespace", "script", "status"},
    )

    activeScriptRunners = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "scriptrunner_active",
            Help: "Number of active ScriptRunners",
        },
        []string{"namespace"},
    )
)
```

### Logging

Structure logs for easy querying:

```go
import "k8s.io/klog/v2"

klog.InfoS("Creating Job",
    "scriptrunner", scriptRunner.Name,
    "namespace", scriptRunner.Namespace,
    "scriptRef", scriptRunner.Spec.ScriptRef,
    "image", scriptRunner.Spec.Image,
    "user", getUserFromContext(ctx),
)
```

### Alerts

Example Prometheus alerts:

```yaml
groups:
- name: scriptrunner
  rules:
  - alert: HighScriptRunnerFailureRate
    expr: |
      rate(scriptrunner_jobs_created_total{status="failed"}[5m])
      /
      rate(scriptrunner_jobs_created_total[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High ScriptRunner failure rate"

  - alert: ScriptRunnerControllerDown
    expr: up{job="scriptrunner-controller"} == 0
    for: 5m
    annotations:
      summary: "ScriptRunner controller is down"
```

## Script Image Management

### Versioned Script Images

Use semantic versioning for script images:

```bash
# Build and tag
podman build -t your-registry.io/scriptrunner-scripts:v1.2.3 .
podman tag your-registry.io/scriptrunner-scripts:v1.2.3 \
           your-registry.io/scriptrunner-scripts:v1.2
podman tag your-registry.io/scriptrunner-scripts:v1.2.3 \
           your-registry.io/scriptrunner-scripts:v1

# Push all tags
podman push your-registry.io/scriptrunner-scripts:v1.2.3
podman push your-registry.io/scriptrunner-scripts:v1.2
podman push your-registry.io/scriptrunner-scripts:v1
```

### Image Security

**Scan images for vulnerabilities:**

```bash
# Using Trivy
trivy image your-registry.io/scriptrunner-scripts:v1.2.3

# Using Snyk
snyk container test your-registry.io/scriptrunner-scripts:v1.2.3
```

**Sign images:**

```bash
# Using cosign
cosign sign your-registry.io/scriptrunner-scripts:v1.2.3
```

**Enforce image signatures in admission webhook:**

```go
// Verify image signature before allowing ScriptRunner
if !verifyImageSignature(scriptRunner.Spec.Image) {
    return admission.Denied("image signature verification failed")
}
```

### Script Changelog

Maintain a changelog for script versions:

```markdown
# Script Changelog

## v1.2.3 (2024-01-15)
- process-data.sh: Fixed handling of special characters in inputs
- validate-inputs.sh: Added support for custom validation rules

## v1.2.2 (2024-01-10)
- report-status.py: Added CSV output format
```

## User Access Patterns

### Pattern 1: Direct kubectl Access

Users create ScriptRunners via kubectl:

```bash
# User creates ScriptRunner
kubectl apply -f - <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
  namespace: user-alice
spec:
  image: your-registry.io/scriptrunner-scripts:v1.2.3
  scriptRef: /scripts/process-data.sh
  inputs:
    environment: "production"
    count: "100"
EOF

# Check status
kubectl get scriptrunner my-task -n user-alice

# View logs
kubectl logs -n user-alice -l scriptrunner.io/name=my-task
```

### Pattern 2: API/Portal

Build a web portal or API that creates ScriptRunners on behalf of users:

```go
// API endpoint
func handleRunScript(w http.ResponseWriter, r *http.Request) {
    // Authenticate user
    user := authenticateUser(r)

    // Parse request
    var req ScriptRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Validate request
    if err := validateRequest(req); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }

    // Create ScriptRunner in user's namespace
    sr := &scriptrunnerv1alpha1.ScriptRunner{
        ObjectMeta: metav1.ObjectMeta{
            Name:      generateName(),
            Namespace: fmt.Sprintf("user-%s", user.Name),
            Labels: map[string]string{
                "user": user.Name,
            },
        },
        Spec: scriptrunnerv1alpha1.ScriptRunnerSpec{
            Image:      approvedImage,
            ScriptRef:  req.Script,
            ScriptArgs: req.Args,
            Inputs:     req.Inputs,
        },
    }

    _, err := client.Create(ctx, sr)
    // ... handle response
}
```

### Pattern 3: GitOps

Users commit ScriptRunners to Git, ArgoCD/Flux applies them:

```yaml
# user-alice/scripts/daily-report.yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: daily-report
  namespace: user-alice
spec:
  image: your-registry.io/scriptrunner-scripts:v1.2.3
  scriptRef: /scripts/report-status.py
  scriptArgs: ["json"]
  inputs:
    report_date: "2024-01-15"
```

## Example Production Setup

### Complete Namespace Template

```yaml
---
# Namespace
apiVersion: v1
kind: Namespace
metadata:
  name: user-{{ .Username }}
  labels:
    scriptrunner.io/tenant: {{ .Username }}
    pod-security.kubernetes.io/enforce: restricted
---
# Resource Quota
apiVersion: v1
kind: ResourceQuota
metadata:
  name: scriptrunner-quota
  namespace: user-{{ .Username }}
spec:
  hard:
    count/scriptrunners.scriptrunner.io: "{{ .MaxScriptRunners }}"
    count/jobs.batch: "{{ .MaxJobs }}"
    pods: "{{ .MaxPods }}"
    requests.cpu: "{{ .CPURequest }}"
    requests.memory: "{{ .MemoryRequest }}"
    limits.cpu: "{{ .CPULimit }}"
    limits.memory: "{{ .MemoryLimit }}"
---
# Limit Range
apiVersion: v1
kind: LimitRange
metadata:
  name: scriptrunner-limits
  namespace: user-{{ .Username }}
spec:
  limits:
  - max:
      cpu: "2"
      memory: "4Gi"
    default:
      cpu: "500m"
      memory: "512Mi"
    defaultRequest:
      cpu: "250m"
      memory: "256Mi"
    type: Container
---
# RBAC Role
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scriptrunner-user
  namespace: user-{{ .Username }}
rules:
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners/status"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
---
# RBAC RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Username }}-scriptrunner-user
  namespace: user-{{ .Username }}
subjects:
- kind: User
  name: {{ .Username }}
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: scriptrunner-user
  apiGroup: rbac.authorization.k8s.io
---
# Network Policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: scriptrunner-jobs
  namespace: user-{{ .Username }}
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
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
```

### User Onboarding Script

```bash
#!/bin/bash
# onboard-user.sh

USERNAME=$1
MAX_SCRIPTRUNNERS=${2:-10}

# Create namespace and apply resources
kubectl apply -f - <<EOF
$(cat namespace-template.yaml | \
  sed "s/{{ .Username }}/$USERNAME/g" | \
  sed "s/{{ .MaxScriptRunners }}/$MAX_SCRIPTRUNNERS/g" | \
  sed "s/{{ .MaxJobs }}/20/g" | \
  sed "s/{{ .MaxPods }}/20/g" | \
  sed "s/{{ .CPURequest }}/4/g" | \
  sed "s/{{ .MemoryRequest }}/8Gi/g" | \
  sed "s/{{ .CPULimit }}/8/g" | \
  sed "s/{{ .MemoryLimit }}/16Gi/g")
EOF

echo "User $USERNAME onboarded successfully"
echo "Namespace: user-$USERNAME"
echo "Max ScriptRunners: $MAX_SCRIPTRUNNERS"
```

## Production Readiness Tracking

For a comprehensive, prioritized production checklist with batch organization and time estimates, see:

**[PRODUCTION_CHECKLIST.md](PRODUCTION_CHECKLIST.md)** - Complete production readiness tracker with:
- 10 organized batches (Foundation, Validation, Observability, etc.)
- 160 total checklist items across all production concerns
- Clear dependencies and parallel work identification
- Priority levels (P0 Must Have, P1 Should Have, P2 Nice to Have)
- Time estimates and execution plan (10-12 weeks to production)
- Current progress tracking (Batch 1 Foundation: ✅ Complete)

**Quick Status:**
- ✅ **Batch 1 (Foundation)**: Complete - Secure controller with health checks, resource limits, and structured logging
- ⏸️ **Batch 2 (Validation & Multi-Tenancy)**: Next - Admission webhooks and namespace isolation
- ⏸️ **Batch 4 (Observability)**: Critical - Prometheus metrics and alerting

**Current Overall Progress: 42/160 items (26%)**

## Next Steps

Based on the production checklist, the recommended order is:

**Immediate (Week 1-2): Batch 2 - Validation & Multi-Tenancy**
1. Implement admission webhook (see [webhook/](../webhook/))
2. Set up namespace templates with quotas
3. Configure user RBAC

**Near-term (Week 3-5): Batch 4 - Observability**
4. Implement Prometheus metrics exporter
5. Configure monitoring dashboards and alerting
6. Centralize log aggregation

**Before Launch (Week 6-12):**
7. Complete security audit (Batch 8)
8. Perform load testing (Batch 6)
9. Document operational procedures (Batch 7)
10. Execute launch readiness checklist (Batch 10)

## Security Considerations

**Never trust user input:**
- Validate all inputs at webhook level
- Sanitize input values
- Limit input sizes
- Whitelist allowed characters

**Principle of least privilege:**
- Users can only create/delete their own ScriptRunners
- Job pods run as non-root
- ReadOnlyRootFilesystem enforced
- Drop all capabilities

**Defense in depth:**
- Namespace isolation
- Network policies
- Resource quotas
- Admission webhooks
- Pod security standards
- Image verification

## Support

For production deployment questions:
- Review the [webhook implementation](../webhook/)
- Check [examples](../examples/)
- See [CONTRIBUTING.md](../CONTRIBUTING.md) for development workflows
