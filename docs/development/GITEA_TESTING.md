# Testing Forge with Private Git Repositories (Gitea in Kind)

This guide walks through setting up a local Gitea instance in a Kind cluster to test Forge's git credential mounting functionality. This is useful for:

- Testing private repository authentication
- Debugging credential mounting issues
- Validating fixes for git-related bugs
- Integration testing without external dependencies

## Prerequisites

- Kind installed
- Podman or Docker installed
- kubectl configured
- Helm installed
- Forge source code cloned locally

## Quick Start

```bash
# From the forge repository root
make kind-setup          # Create cluster and deploy Forge
make kind-zarf-cli       # Load zarf CLI image
# Then follow the steps below to set up Gitea
```

## Step 1: Create Kind Cluster and Deploy Forge

If you don't have a cluster running:

```bash
# Create cluster with Forge deployed
make kind-setup

# Verify deployment
make status
```

Expected output:
```txt
=== Controller Status ===
NAME                                READY   STATUS    RESTARTS   AGE
forge-controller-xxxxx              1/1     Running   0          30s

=== Webhook Status ===
NAME                             READY   STATUS    RESTARTS   AGE
forge-webhook-xxxxx              1/1     Running   0          30s
```

## Step 2: Deploy Gitea

Create the Gitea namespace and deployment:

```bash
kubectl create namespace gitea
```

Apply the Gitea deployment:

```bash
cat <<'EOF' | kubectl apply -f -
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: gitea-data
  namespace: gitea
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitea
  namespace: gitea
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitea
  template:
    metadata:
      labels:
        app: gitea
    spec:
      containers:
      - name: gitea
        image: gitea/gitea:latest
        ports:
        - containerPort: 3000
          name: http
        - containerPort: 22
          name: ssh
        env:
        - name: GITEA__database__DB_TYPE
          value: sqlite3
        - name: GITEA__server__ROOT_URL
          value: http://gitea.gitea.svc.cluster.local:3000/
        - name: GITEA__server__HTTP_PORT
          value: "3000"
        - name: GITEA__security__INSTALL_LOCK
          value: "true"
        - name: GITEA__service__DISABLE_REGISTRATION
          value: "false"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: gitea-data
---
apiVersion: v1
kind: Service
metadata:
  name: gitea
  namespace: gitea
spec:
  type: ClusterIP
  ports:
  - port: 3000
    targetPort: 3000
    name: http
  - port: 22
    targetPort: 22
    name: ssh
  selector:
    app: gitea
EOF
```

Wait for Gitea to be ready:

```bash
kubectl wait --for=condition=available deployment/gitea -n gitea --timeout=120s
```

## Step 3: Create Gitea User

Create an admin user in Gitea:

```bash
GITEA_POD=$(kubectl get pods -n gitea -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n gitea $GITEA_POD -- su git -c \
  "gitea admin user create --admin --username testuser --password testpass123 --email test@test.com"
```

Expected output:

```txt
New user 'testuser' has been successfully created!
```

## Step 4: Create Private Repository

Create a private repository via the Gitea API:

```bash
kubectl run -n gitea curl-create-repo --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/user/repos" \
  -H "Content-Type: application/json" \
  -u "testuser:testpass123" \
  -d '{"name": "private-zarf-repo", "private": true, "auto_init": true}'
```

## Step 5: Add Zarf Package Files

Add a `zarf.yaml` file:

```bash
ZARF_YAML=$(cat <<'ZARFEOF'
kind: ZarfPackageConfig
metadata:
  name: test-private-package
  description: Test package from private repo
  version: 1.0.0

components:
  - name: test-component
    required: true
    manifests:
      - name: test-configmap
        namespace: default
        files:
          - configmap.yaml
ZARFEOF
)

ZARF_YAML_BASE64=$(echo "$ZARF_YAML" | base64 | tr -d '\n')

kubectl run -n gitea curl-add-zarf --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/repos/testuser/private-zarf-repo/contents/zarf.yaml" \
  -H "Content-Type: application/json" \
  -u "testuser:testpass123" \
  -d "{\"content\": \"$ZARF_YAML_BASE64\", \"message\": \"Add zarf.yaml\"}"
```

Add a `configmap.yaml` file:

```bash
CONFIGMAP_YAML=$(cat <<'CMEOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
data:
  message: "Hello from private repo"
CMEOF
)

CONFIGMAP_YAML_BASE64=$(echo "$CONFIGMAP_YAML" | base64 | tr -d '\n')

kubectl run -n gitea curl-add-cm --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/repos/testuser/private-zarf-repo/contents/configmap.yaml" \
  -H "Content-Type: application/json" \
  -u "testuser:testpass123" \
  -d "{\"content\": \"$CONFIGMAP_YAML_BASE64\", \"message\": \"Add configmap.yaml\"}"
```

## Step 6: Create Forge Resources

Create the namespace, credentials, and service account:

```bash
kubectl create namespace forge-jobs 2>/dev/null || true

cat <<'EOF' | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: gitea-creds
  namespace: forge-jobs
type: Opaque
stringData:
  username: "testuser"
  token: "testpass123"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: private-repo-sa
  namespace: forge-jobs
  annotations:
    forge.dev/allowed-actions: "Build"
    forge.dev/allowed-source-repos: "http://gitea.gitea.svc.cluster.local:3000/*"
EOF
```

## Step 7: Create and Test ZarfPackageJob

Create the job:

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: private-gitea-build
  namespace: forge-jobs
spec:
  serviceAccountName: private-repo-sa
  action: Build
  source:
    type: Git
    git:
      url: http://gitea.gitea.svc.cluster.local:3000/testuser/private-zarf-repo.git
      ref: main
      credentialRef:
        name: gitea-creds
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 1
      memory: 1Gi
EOF
```

Monitor the job:

```bash
# Watch pod status
kubectl get pods -n forge-jobs -l app=forge -w

# Check job status
kubectl get zarfpackagejob -n forge-jobs private-gitea-build
```

## Step 8: Verify Success

Check the init container logs:

```bash
POD=$(kubectl get pods -n forge-jobs -l forge.dev/package=private-gitea-build -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n forge-jobs $POD -c git-clone
```

Expected output:

```txt
Cloning into '/workspace'...
```

Check the main container logs:

```bash
kubectl logs -n forge-jobs $POD -c zarf-build
```

Expected output:
```txt
2026-01-21 23:55:34 INF assembling package path=.
2026-01-21 23:55:34 INF composed components successfully
2026-01-21 23:55:34 INF skipping package signing (no signing key material configured)
2026-01-21 23:55:34 INF writing package to disk path=/artifacts/zarf-package-test-private-package-arm64-1.0.0.tar.zst
```

## Troubleshooting

### Init Container Fails with "could not read Username"

**Symptom:**
```txt
fatal: could not read Username for 'http://...': No such device or address
```

**Cause:** Credential file protocol mismatch. The URL uses `http://` but credentials were written with `https://`.

**Solution:** Ensure you're running the fixed version of Forge that uses `extractGitScheme()`.

### Init Container Fails with "Authentication failed"

**Symptom:**
```txt
remote: Failed to authenticate user
fatal: Authentication failed for '...'
```

**Cause:** Using `oauth2:token` format with a server that requires `username:password` format.

**Solution:** Add `username` field to your git credentials secret:
```yaml
stringData:
  username: "your-username"
  token: "your-password-or-token"
```

### Init Container Fails with "Repository not found"

**Symptom:**
```txt
fatal: repository '...' not found
```

**Cause:** Repository doesn't exist or credentials are incorrect.

**Solution:**
1. Verify the repository exists:
   ```bash
   kubectl run -n gitea test --rm -i --restart=Never --image=curlimages/curl -- \
     curl -s -u "testuser:testpass123" \
     "http://gitea.gitea.svc.cluster.local:3000/api/v1/repos/testuser/private-zarf-repo"
   ```
2. Check credentials are correct in the secret

### Pod Stuck in Init:0/1

**Symptom:** Pod never progresses past init container.

**Debugging Steps:**
```bash
# Describe the pod
kubectl describe pod -n forge-jobs <pod-name>

# Check init container logs
kubectl logs -n forge-jobs <pod-name> -c git-clone

# Check events
kubectl get events -n forge-jobs --sort-by='.lastTimestamp'
```

### Webhook Rejects Job Creation

**Symptom:**
```txt
The ZarfPackageJob "..." is invalid: spec.source.git.url: Invalid value: "http://...":
spec.source.git.url in body should match '^https://.*'
```

**Cause:** CRD validation requires HTTPS URLs by default.

**Solution:** For testing with HTTP, update the CRD validation pattern in `pkg/apis/zarf/v1alpha3/types.go`:
```go
// +kubebuilder:validation:Pattern=`^https?://.*`  // Changed from ^https://.*
URL string `json:"url"`
```

Then regenerate and redeploy:
```bash
make manifests
make kind-redeploy
```

## Debugging Commands

### View All Forge Resources

```bash
make status
```

### View Controller Logs

```bash
kubectl logs -n forge-system -l app.kubernetes.io/component=controller -f
```

### View Webhook Logs

```bash
kubectl logs -n forge-system -l app.kubernetes.io/component=webhook -f
```

### Describe a Failing Pod

```bash
kubectl describe pod -n forge-jobs <pod-name>
```

### Check Secret Contents

```bash
kubectl get secret -n forge-jobs gitea-creds -o jsonpath='{.data}' | jq -r 'to_entries[] | "\(.key): \(.value | @base64d)"'
```

### Test Git Clone Manually

```bash
kubectl run -n gitea test-clone --rm -i --restart=Never --image=alpine/git -- \
  clone --depth 1 http://testuser:testpass123@gitea.gitea.svc.cluster.local:3000/testuser/private-zarf-repo.git /tmp/test
```

### Exec into Gitea Pod

```bash
GITEA_POD=$(kubectl get pods -n gitea -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n gitea $GITEA_POD -- /bin/sh
```

## Cleanup

Remove test resources:

```bash
# Delete ZarfPackageJob
kubectl delete zarfpackagejob -n forge-jobs --all

# Delete forge-jobs namespace
kubectl delete namespace forge-jobs

# Delete Gitea
kubectl delete namespace gitea

# Delete Kind cluster (optional)
make kind-delete
```

## Full Automated Test Script

Save this as `test-gitea-credentials.sh`:

```bash
#!/bin/bash
set -e

echo "=== Setting up Gitea test environment ==="

# Create Gitea namespace and deployment
kubectl create namespace gitea 2>/dev/null || true

cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: gitea-data
  namespace: gitea
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitea
  namespace: gitea
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitea
  template:
    metadata:
      labels:
        app: gitea
    spec:
      containers:
      - name: gitea
        image: gitea/gitea:latest
        ports:
        - containerPort: 3000
        env:
        - {name: GITEA__database__DB_TYPE, value: sqlite3}
        - {name: GITEA__server__ROOT_URL, value: "http://gitea.gitea.svc.cluster.local:3000/"}
        - {name: GITEA__security__INSTALL_LOCK, value: "true"}
        volumeMounts:
        - {name: data, mountPath: /data}
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: gitea-data
---
apiVersion: v1
kind: Service
metadata:
  name: gitea
  namespace: gitea
spec:
  ports:
  - {port: 3000, targetPort: 3000}
  selector:
    app: gitea
EOF

echo "Waiting for Gitea..."
kubectl wait --for=condition=available deployment/gitea -n gitea --timeout=120s
sleep 5

echo "Creating Gitea user..."
GITEA_POD=$(kubectl get pods -n gitea -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n gitea $GITEA_POD -- su git -c \
  "gitea admin user create --admin --username testuser --password testpass123 --email test@test.com" 2>/dev/null || true

echo "Creating private repository..."
kubectl run -n gitea curl-repo --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/user/repos" \
  -H "Content-Type: application/json" -u "testuser:testpass123" \
  -d '{"name":"test-repo","private":true,"auto_init":true}' > /dev/null

echo "Adding zarf.yaml..."
CONTENT=$(echo 'kind: ZarfPackageConfig
metadata:
  name: test-pkg
  version: 1.0.0
components:
  - name: test
    required: true
    manifests:
      - name: cm
        files: [cm.yaml]' | base64 | tr -d '\n')

kubectl run -n gitea curl-zarf --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/repos/testuser/test-repo/contents/zarf.yaml" \
  -H "Content-Type: application/json" -u "testuser:testpass123" \
  -d "{\"content\":\"$CONTENT\",\"message\":\"add zarf.yaml\"}" > /dev/null

echo "Adding cm.yaml..."
CONTENT=$(echo 'apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value' | base64 | tr -d '\n')

kubectl run -n gitea curl-cm --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST "http://gitea.gitea.svc.cluster.local:3000/api/v1/repos/testuser/test-repo/contents/cm.yaml" \
  -H "Content-Type: application/json" -u "testuser:testpass123" \
  -d "{\"content\":\"$CONTENT\",\"message\":\"add cm.yaml\"}" > /dev/null

echo "Creating Forge resources..."
kubectl create namespace forge-jobs 2>/dev/null || true

cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: git-creds
  namespace: forge-jobs
stringData:
  username: testuser
  token: testpass123
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: forge-jobs
  annotations:
    forge.dev/allowed-actions: "Build"
    forge.dev/allowed-source-repos: "http://gitea.gitea.svc.cluster.local:3000/*"
---
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: git-creds-test
  namespace: forge-jobs
spec:
  serviceAccountName: test-sa
  action: Build
  source:
    type: Git
    git:
      url: http://gitea.gitea.svc.cluster.local:3000/testuser/test-repo.git
      ref: main
      credentialRef:
        name: git-creds
EOF

echo "Waiting for job to complete..."
sleep 10

POD=$(kubectl get pods -n forge-jobs -l forge.dev/package=git-creds-test -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$POD" ]; then
  echo "ERROR: No pod found"
  exit 1
fi

STATUS=$(kubectl get pod -n forge-jobs $POD -o jsonpath='{.status.phase}')
if [ "$STATUS" == "Succeeded" ]; then
  echo "SUCCESS: Job completed successfully!"
  kubectl logs -n forge-jobs $POD -c zarf-build | tail -5
else
  echo "FAILURE: Job did not succeed (status: $STATUS)"
  kubectl logs -n forge-jobs $POD -c git-clone
  exit 1
fi
```

Make it executable and run:
```bash
chmod +x test-gitea-credentials.sh
./test-gitea-credentials.sh
```
