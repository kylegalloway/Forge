# ScriptRunner User Guide

This guide is for users who want to run pre-built scripts using ScriptRunner.

## Getting Started

### What is ScriptRunner?

ScriptRunner allows you to execute pre-built scripts in isolated Kubernetes Jobs by simply creating a YAML resource. You provide inputs, and the script runs with those inputs in a secure, controlled environment.

### Prerequisites

- Access to a Kubernetes cluster with ScriptRunner installed
- kubectl configured with appropriate permissions
- Know which namespace you're using (e.g., `user-alice`)

### Your First ScriptRunner

Create a file `my-first-run.yaml`:

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-first-run
  namespace: user-alice  # Your namespace
spec:
  # Script to run (ask your admin for available scripts)
  scriptRef: /scripts/process-data.sh

  # Your inputs (will be available as environment variables)
  inputs:
    environment: "staging"
    count: "10"
```

Apply it:

```bash
kubectl apply -f my-first-run.yaml
```

Check the status:

```bash
# View the ScriptRunner
kubectl get scriptrunner my-first-run -n user-alice

# View the Job that was created
kubectl get jobs -n user-alice -l scriptrunner.io/name=my-first-run

# View the logs
kubectl logs -n user-alice -l scriptrunner.io/name=my-first-run
```

## Available Scripts

Contact your administrator for the list of approved scripts. Common examples:

| Script | Purpose | Required Inputs | Optional Inputs |
|--------|---------|-----------------|-----------------|
| `/scripts/process-data.sh` | Process data batches | `environment` | `count`, `batch_id` |
| `/scripts/validate-inputs.sh` | Validate configuration | `environment`, `version` | `service_name` |
| `/scripts/report-status.py` | Generate reports | - | `format` (json/text) |

## Providing Inputs

Inputs are passed as environment variables to your script, prefixed with `INPUT_`:

**YAML:**
```yaml
spec:
  inputs:
    environment: "production"
    user_id: "12345"
    count: "100"
```

**Script receives:**
```bash
INPUT_environment="production"
INPUT_user_id="12345"
INPUT_count="100"
```

### Input Best Practices

1. **Use descriptive keys**: `environment` not `env`
2. **All values are strings**: Convert in your script if needed
3. **Check limits**: Your admin may limit number/size of inputs
4. **Avoid secrets**: Don't put passwords in inputs

### Passing Arguments to Scripts

Some scripts accept command-line arguments:

```yaml
spec:
  scriptRef: /scripts/process-data.sh
  scriptArgs:
    - "batch-process"  # First argument
    - "20"             # Second argument
  inputs:
    environment: "production"
```

The script receives:
- Arguments as `$1`, `$2`, etc.
- Inputs as `$INPUT_environment`, etc.

## Checking Results

### View ScriptRunner Status

```bash
kubectl get scriptrunner my-run -n user-alice -o yaml
```

Look for the `status` section:

```yaml
status:
  phase: JobCreated
  jobName: my-run-job-1234567890
  message: Job created successfully
  lastUpdateTime: "2024-01-15T10:30:00Z"
```

### View Job Logs

```bash
# Quick way - view logs by label
kubectl logs -n user-alice -l scriptrunner.io/name=my-run

# Or find the job name and view logs
JOB=$(kubectl get scriptrunner my-run -n user-alice -o jsonpath='{.status.jobName}')
kubectl logs -n user-alice job/$JOB
```

### View Job Status

```bash
kubectl get jobs -n user-alice
```

Status meanings:
- **Active**: Job is currently running
- **Succeeded**: Job completed successfully
- **Failed**: Job failed (check logs)

## Common Patterns

### Running the Same Script Multiple Times

Each ScriptRunner must have a unique name:

```yaml
metadata:
  name: daily-report-2024-01-15
spec:
  scriptRef: /scripts/report-status.py
  inputs:
    date: "2024-01-15"
---
metadata:
  name: daily-report-2024-01-16
spec:
  scriptRef: /scripts/report-status.py
  inputs:
    date: "2024-01-16"
```

### Scheduled Runs

Use a CronJob to create ScriptRunners on a schedule:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
  namespace: user-alice
spec:
  schedule: "0 9 * * *"  # 9 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: scriptrunner-creator
          containers:
          - name: create-scriptrunner
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              kubectl apply -f - <<EOF
              apiVersion: scriptrunner.io/v1alpha1
              kind: ScriptRunner
              metadata:
                name: daily-report-$(date +%Y%m%d)
                namespace: user-alice
              spec:
                scriptRef: /scripts/report-status.py
                inputs:
                  date: "$(date +%Y-%m-%d)"
              EOF
          restartPolicy: OnFailure
```

### Parameterized Runs

Create ScriptRunners programmatically:

```bash
#!/bin/bash
# run-for-each-env.sh

for ENV in dev staging production; do
  cat <<EOF | kubectl apply -f -
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: validate-${ENV}
  namespace: user-alice
spec:
  scriptRef: /scripts/validate-inputs.sh
  inputs:
    environment: "${ENV}"
    version: "1.2.3"
EOF
done
```

## Troubleshooting

### ScriptRunner not creating a Job

**Check the ScriptRunner status:**
```bash
kubectl describe scriptrunner my-run -n user-alice
```

Look for error messages in the Events section.

**Common causes:**
- Invalid `scriptRef` (not in approved list)
- Invalid `image` (not from approved registry)
- Too many inputs or inputs too large
- Inline `script` used (may be blocked in production)

### Job fails immediately

**Check the Job description:**
```bash
kubectl describe job <job-name> -n user-alice
```

**Common causes:**
- Image pull errors (check image name)
- Resource limits exceeded
- Script exits with error code

### Cannot see logs

**Check if Pod exists:**
```bash
kubectl get pods -n user-alice -l scriptrunner.io/name=my-run
```

**If Pod is not found:**
- Job may not have started yet
- Job may have been cleaned up (check TTL settings)

**View previous Pod logs:**
```bash
kubectl logs -n user-alice <pod-name> --previous
```

### "Forbidden" errors

**You may not have permission to:**
- Create ScriptRunners in this namespace
- View logs
- Access certain scripts

Contact your administrator to verify permissions.

## Resource Limits

Your namespace likely has resource quotas:

```bash
# Check your quota
kubectl get resourcequota -n user-alice
```

This may limit:
- Number of ScriptRunners
- Number of concurrent Jobs
- Total CPU/memory usage

If you hit limits:
1. Delete old ScriptRunners: `kubectl delete scriptrunner old-run -n user-alice`
2. Wait for Jobs to complete
3. Contact admin to increase quota if needed

## Best Practices

### Naming

Use descriptive, unique names:

```yaml
# Good
name: process-orders-2024-01-15-batch-1

# Bad
name: test
name: run1
```

### Labels

Add labels for organization:

```yaml
metadata:
  name: my-run
  labels:
    app: my-app
    environment: production
    batch-id: "12345"
```

Query by label:

```bash
kubectl get scriptrunner -n user-alice -l app=my-app
kubectl get scriptrunner -n user-alice -l environment=production
```

### Cleanup

Delete old ScriptRunners when done:

```bash
# Delete specific ScriptRunner
kubectl delete scriptrunner old-run -n user-alice

# Delete all ScriptRunners with a label
kubectl delete scriptrunner -n user-alice -l app=my-app

# Delete ScriptRunners older than 7 days
kubectl get scriptrunner -n user-alice --sort-by=.metadata.creationTimestamp | \
  head -n -7 | \
  awk '{print $1}' | \
  xargs -r kubectl delete scriptrunner -n user-alice
```

Jobs are usually cleaned up automatically based on TTL settings.

## Getting Help

### Check Script Documentation

Ask your administrator for documentation on available scripts, including:
- What the script does
- Required inputs
- Optional inputs
- Expected output
- Examples

### View Resource Definitions

```bash
# See full ScriptRunner spec
kubectl explain scriptrunner.spec

# See specific fields
kubectl explain scriptrunner.spec.inputs
kubectl explain scriptrunner.spec.scriptRef
```

### Support

Contact your platform team or administrator with:
- ScriptRunner name and namespace
- What you're trying to accomplish
- Error messages from `kubectl describe`
- Relevant logs

## Examples

### Example 1: Data Processing

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: process-january-data
  namespace: user-alice
  labels:
    app: data-pipeline
    month: january
spec:
  scriptRef: /scripts/process-data.sh
  scriptArgs:
    - "monthly-rollup"
    - "100"
  inputs:
    environment: "production"
    month: "2024-01"
    source: "database"
    destination: "s3://my-bucket/processed/"
```

### Example 2: Validation

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: validate-deployment-config
  namespace: user-alice
spec:
  scriptRef: /scripts/validate-inputs.sh
  inputs:
    environment: "staging"
    version: "2.1.0"
    service_name: "api-gateway"
```

### Example 3: Reporting

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: monthly-report-jan-2024
  namespace: user-alice
spec:
  scriptRef: /scripts/report-status.py
  scriptArgs:
    - "json"
  inputs:
    report_type: "monthly"
    month: "2024-01"
    services: "api,web,worker"
    format: "detailed"
```

## Quick Reference

```bash
# Create ScriptRunner
kubectl apply -f my-run.yaml

# View ScriptRunners
kubectl get scriptrunner -n user-alice

# Describe ScriptRunner
kubectl describe scriptrunner my-run -n user-alice

# View logs
kubectl logs -n user-alice -l scriptrunner.io/name=my-run

# View Job
kubectl get jobs -n user-alice

# Delete ScriptRunner
kubectl delete scriptrunner my-run -n user-alice

# Check quota
kubectl get resourcequota -n user-alice
```

## Advanced Topics

### Using with CI/CD

Include ScriptRunner creation in your CI/CD pipeline:

```yaml
# .gitlab-ci.yml
run-validation:
  stage: test
  script:
    - |
      kubectl apply -f - <<EOF
      apiVersion: scriptrunner.io/v1alpha1
      kind: ScriptRunner
      metadata:
        name: validate-$CI_COMMIT_SHORT_SHA
        namespace: user-alice
      spec:
        scriptRef: /scripts/validate-inputs.sh
        inputs:
          environment: staging
          version: $CI_COMMIT_TAG
      EOF
    - kubectl wait --for=condition=complete --timeout=300s job -l scriptrunner.io/name=validate-$CI_COMMIT_SHORT_SHA -n user-alice
```

### GitOps Integration

Manage ScriptRunners in Git with ArgoCD/Flux:

```yaml
# config/scriptrunners/daily-report.yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: daily-report
  namespace: user-alice
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  scriptRef: /scripts/report-status.py
  inputs:
    frequency: "daily"
```

Commit to Git, ArgoCD syncs automatically.

## Summary

ScriptRunner makes it easy to run pre-built scripts with your own inputs:

1. Choose an approved script
2. Provide your inputs in YAML
3. Apply with kubectl
4. Check logs for results

For questions or issues, contact your platform team.
