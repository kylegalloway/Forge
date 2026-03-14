# Forge Operator Guide

This guide is for the engineer responsible for deploying and administering Forge. It covers installing Forge for your own team and for other dev teams, configuring RBAC, managing credentials, and tuning resource usage.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Installing Forge](#installing-forge)
3. [RBAC Design](#rbac-design)
4. [Setting Up Dev Teams](#setting-up-dev-teams)
5. [Credential Management](#credential-management)
6. [Example Workflows](#example-workflows)
7. [Volume Sizing and Resource Limits](#volume-sizing-and-resource-limits)
8. [Multi-Cluster Deployments](#multi-cluster-deployments)
9. [Verification and Troubleshooting](#verification-and-troubleshooting)

---

## Architecture Overview

Forge has two server-side components: a **controller** that reconciles `ZarfPackageJob` and `UDSBundleJob` resources, and a **validating webhook** that enforces policy at admission time. Both run in `forge-system`.

When a job resource is created, the controller spawns a Kubernetes `Job` that runs the CLI image (`zarfpackagejob` or `udsbundlejob`) to execute the requested operation. The webhook validates the job against annotations on the referenced `ServiceAccount` before it is admitted.

**Typical workflows:**

1. **BuildPublish** — clone a Git repo (or pull a pre-built package from OCI), build a Zarf package, and publish it to S3 or OCI.
2. **Deploy** — pull a published package from S3 or OCI and deploy it to a cluster.

---

## Installing Forge

### Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- `kubectl` connected to the target cluster
- Cluster-admin permissions (required to install CRDs and ClusterRoles)

### Add the Helm Repository

```bash
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update
```

### Cluster-Wide Installation (Recommended for Platform Teams)

This is the default. Forge watches all namespaces and can manage jobs in any namespace.

```bash
helm upgrade --install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set webhook.replicaCount=3 \
  --set networkPolicies.enabled=true
```

With `replicaCount > 1`, the chart automatically enables leader election, configures a PodDisruptionBudget, and spreads replicas across nodes via preferred pod anti-affinity.

### Namespace-Scoped Installation (Per-Team Isolation)

For environments where teams each need their own isolated Forge instance, or where ClusterRole permissions are unavailable:

```bash
helm upgrade --install forge-team-a forge/forge \
  --namespace team-a \
  --create-namespace \
  --set controller.namespaceScope=true \
  --set controller.replicaCount=1 \
  --set networkPolicies.enabled=true
```

In namespace-scoped mode, all `ZarfPackageJob`/`UDSBundleJob` resources, `ServiceAccount` policies, `Secret` credentials, and spawned `Job` objects must reside in the same namespace as the Forge installation. The controller uses a `Role`/`RoleBinding` instead of `ClusterRole`/`ClusterRoleBinding`.

**Note:** CRDs are still cluster-scoped and require cluster-admin to install.

### Concurrency Limits

To prevent runaway job creation from overloading a cluster:

```bash
helm upgrade forge forge/forge \
  --namespace forge-system \
  --set controller.concurrency.maxJobsPerNamespace=10 \
  --set controller.concurrency.maxJobsGlobal=50
```

Jobs that exceed the limit enter `Queued` phase and are dispatched automatically when capacity is available.

---

## RBAC Design

Forge has two distinct RBAC layers:

1. **Controller/Webhook RBAC** — what the Forge system itself can do in Kubernetes (managed by Helm, not normally modified).
2. **User Policy RBAC** — what each team or service account is allowed to request Forge to do (managed via `ServiceAccount` annotations).

### Controller RBAC (What Forge Needs)

The Helm chart creates a `ClusterRole` (or `Role` in namespace-scoped mode) with the minimum permissions Forge requires:

| Resource | Verbs | Purpose |
|---|---|---|
| `zarfpackagejobs`, `udsbundlejobs` | `get`, `list`, `watch` | Reconcile job resources |
| `zarfpackagejobs/status`, `udsbundlejobs/status` | `get`, `update`, `patch` | Update job phase and status |
| `serviceaccounts` | `get`, `list`, `watch` | Read policy annotations |
| `secrets` | `get`, `list` | Validate credential secrets exist |
| `jobs` (batch) | `create`, `get`, `list`, `watch` | Spawn CLI jobs |
| `pods` | `get`, `list`, `watch` | Track job pod status |
| `persistentvolumeclaims` | `create`, `get`, `list`, `watch`, `delete` | Manage artifact PVCs |
| `events` | `create`, `patch` | Emit audit events |
| `leases` (coordination.k8s.io) | `create`, `get`, `list`, `update` | Leader election |

Forge **does not** have `delete` on `Jobs` (TTL handles cleanup), `create/delete` on `Secrets`, or any permissions on `Deployments`, `Nodes`, or `ClusterRoles`.

To audit what is installed:

```bash
kubectl get clusterrole forge-controller-role -o yaml
kubectl describe clusterrolebinding forge-controller-rolebinding
```

### User Policy RBAC (ServiceAccount Annotations)

Each `ZarfPackageJob` or `UDSBundleJob` must reference a `ServiceAccount`. The webhook validates the job spec against annotations on that `ServiceAccount`. **If an annotation is absent, the corresponding permission is denied.**

| Annotation | Controls |
|---|---|
| `forge.dev/allowed-actions` | Which job actions may be used (`Build`, `Publish`, `Deploy`, `BuildPublish`, etc., or `*`) |
| `forge.dev/allowed-source-repos` | Git repo URL glob patterns |
| `forge.dev/allowed-source-buckets` | S3 bucket name glob patterns |
| `forge.dev/allowed-source-registries` | OCI registry URL glob patterns |
| `forge.dev/allowed-publish-buckets` | S3 bucket name glob patterns for publish destinations |
| `forge.dev/allowed-publish-registries` | OCI registry URL glob patterns for publish destinations |
| `forge.dev/allowed-deploy-targets` | `InCluster`, `ExternalCluster`, or `*` |
| `forge.dev/allow-local-sources` | `"true"` to permit local filesystem sources (dev only) |

**ServiceAccounts must be in the same namespace as the job resource** (or `forge-system` in namespace-scoped mode).

---

## Setting Up Dev Teams

The recommended pattern is one `ServiceAccount` per team (or per pipeline role), scoped to what that team actually needs.

### Cluster-Wide: Teams in Their Own Namespaces

```bash
kubectl create namespace team-platform
kubectl create namespace team-app-a
```

Each team creates their jobs in their own namespace and references a `ServiceAccount` in that same namespace.

```yaml
# team-platform namespace: full build/publish pipeline with S3 and OCI
apiVersion: v1
kind: ServiceAccount
metadata:
  name: platform-pipeline
  namespace: team-platform
  annotations:
    # Allow both individual and combined actions
    forge.dev/allowed-actions: "Build,Publish,BuildPublish,Create,CreatePublish"
    # Lock to repos in your org
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    # OCI source (for pulling pre-built packages to re-publish)
    forge.dev/allowed-source-registries: "ghcr.io/myorg/*"
    # Publish destinations
    forge.dev/allowed-publish-buckets: "myorg-artifacts-*"
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"

---
# team-app-a namespace: deploy-only, from OCI or S3, in-cluster only
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-a-deployer
  namespace: team-app-a
  annotations:
    forge.dev/allowed-actions: "Deploy"
    forge.dev/allowed-source-buckets: "myorg-artifacts-*"
    forge.dev/allowed-source-registries: "ghcr.io/myorg/releases/*"
    forge.dev/allowed-deploy-targets: "InCluster"
```

### Namespace-Scoped: Per-Team Forge Instances

Each team gets their own Forge controller scoped to their namespace. All resources (jobs, secrets, service accounts) live in that namespace.

```bash
# Install a Forge instance for team-a
helm upgrade --install forge-team-a forge/forge \
  --namespace team-a \
  --create-namespace \
  --set controller.namespaceScope=true

# Install a Forge instance for team-b
helm upgrade --install forge-team-b forge/forge \
  --namespace team-b \
  --create-namespace \
  --set controller.namespaceScope=true
```

In each team's namespace, create their `ServiceAccount` with policy annotations, their credential `Secret`s, and their job resources. The teams are fully isolated — each Forge controller only sees resources in its own namespace.

---

## Credential Management

All credentials are stored as Kubernetes `Secret`s in the same namespace as the job resource. Forge reads secrets at job creation time to validate they exist, and mounts them into the spawned `Job` pod at runtime.

### Git Credentials

```yaml
# Token-based (GitHub, GitLab.com)
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
  namespace: team-platform
type: Opaque
stringData:
  token: "ghp_your_token_here"

# For self-hosted Git servers requiring a username
# stringData:
#   username: "ci-user"
#   token: "your-token-or-password"

# SSH key (works with any Git server)
# stringData:
#   ssh-key: |
#     -----BEGIN OPENSSH PRIVATE KEY-----  # pragma: allowlist secret
#     ...
#     -----END OPENSSH PRIVATE KEY-----
```

Reference in a job: `source.git.credentialRef.name: git-credentials`

### OCI Registry Credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
  namespace: team-platform
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |
    {
      "auths": {
        "ghcr.io": {
          "username": "your-username",
          "password": "your-token"  # pragma: allowlist secret
        }
      }
    }
```

A single secret can include multiple registries. Reference in a job:

- For OCI source: `source.oci.credentialRef.name: registry-credentials`
- For build-time image pulls: `build.registryCredentialRef.name: registry-credentials`
- For OCI publish destination: `publish.destination.oci.credentialRef.name: registry-credentials`

### S3 / AWS Credentials

Forge supports three credential modes for S3:

```yaml
# EnvVar (default): secret contains 'access-key-id' and 'secret-access-key'
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
  namespace: team-platform
type: Opaque
stringData:
  access-key-id: "AKIAIOSFODNN7EXAMPLE"  # pragma: allowlist secret
  secret-access-key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"  # pragma: allowlist secret
```

Reference: `credentialRef.type: EnvVar` (or omit `type`, EnvVar is the default).

For IRSA / instance profiles (EKS, EC2), use `type: Node` and omit the secret name — no secret is needed.

For file-based credentials (multiple profiles, session tokens):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials-file
  namespace: team-platform
type: Opaque
stringData:
  credentials: |
    [default]
    aws_access_key_id = AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

Reference: `credentialRef.type: File`, `credentialRef.name: s3-credentials-file`.

### External Cluster Kubeconfig

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prod-cluster-kubeconfig
  namespace: team-platform
type: Opaque
stringData:
  kubeconfig: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        server: https://prod-cluster.example.com:6443
        certificate-authority-data: <base64-ca>
      name: prod
    contexts:
    - context:
        cluster: prod
        user: forge-deployer
      name: prod
    current-context: prod
    users:
    - name: forge-deployer
      user:
        token: "<service-account-token>"
```

Reference: `deploy.externalCluster.secretRef.name: prod-cluster-kubeconfig`

---

## Example Workflows

### BuildPublish: Git Source → OCI and S3 Destinations

This example builds a Zarf package from a private Git repo, publishes it to both an OCI registry and an S3 bucket, and demonstrates credential usage, volume sizing, resource limits, and retry configuration.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: platform-pipeline
  namespace: team-platform
  annotations:
    forge.dev/allowed-actions: "Build,Publish,BuildPublish"
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.dev/allowed-publish-buckets: "myorg-artifacts-*"
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"

---
# Build from Git and publish to OCI registry
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: my-app-build-publish
  namespace: team-platform
spec:
  serviceAccountName: platform-pipeline
  action: BuildPublish

  source:
    type: Git
    git:
      url: https://github.com/myorg/my-app-packages
      ref: v2.1.0
      path: packages/my-app
      credentialRef:
        name: git-credentials

  build:
    timeout: 2h
    architecture: amd64
    # Credentials for private images referenced in zarf.yaml during 'zarf package create'
    registryCredentialRef:
      name: registry-credentials
    variables:
      VERSION: "2.1.0"
      REGISTRY: "ghcr.io/myorg"
    retry:
      maxRetries: 2
      initialBackoff: 1m
      maxBackoff: 10m
      retryableErrors:
        - "*timeout*"
        - "*rate limit*"

  publish:
    destination:
      type: OCI
      oci:
        registry: ghcr.io
        repository: myorg/packages/my-app
        tag: "2.1.0"
        credentialRef:
          name: registry-credentials
    timeout: 30m
    retry:
      maxRetries: 3
      initialBackoff: 30s

  # Resource limits for the build pod — large packages may need more
  resources:
    requests:
      cpu: "1"
      memory: 2Gi
    limits:
      cpu: "4"
      memory: 8Gi

  # EmptyDir volume sizes — increase workspace/output for large packages
  # Defaults: workspace=10Gi, output=10Gi, tmp=1Gi, home=1Gi
  volumeSizes:
    workspace: 30Gi   # source checkout and build intermediates
    output: 20Gi      # built package before publish
    tmp: 2Gi
    home: 1Gi

  # Schedule on nodes with fast local storage
  nodeSelector:
    disktype: ssd

  # Clean up the artifact PVC after the job completes
  retainArtifactPVC: false
```

To publish to S3 instead of (or in addition to) OCI, change the `publish.destination`:

```yaml
  publish:
    destination:
      type: S3
      s3:
        bucket: myorg-artifacts-prod
        keyPrefix: zarf-packages/my-app/
        region: us-west-2
        credentialRef:
          type: EnvVar
          name: s3-credentials
    timeout: 30m
```

### Deploy: OCI or S3 Source → Cluster

This example deploys a previously published package. It covers both in-cluster and external cluster targets.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-a-deployer
  namespace: team-app-a
  annotations:
    forge.dev/allowed-actions: "Deploy"
    forge.dev/allowed-source-registries: "ghcr.io/myorg/packages/*"
    forge.dev/allowed-source-buckets: "myorg-artifacts-*"
    forge.dev/allowed-deploy-targets: "InCluster,ExternalCluster"

---
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: my-app-deploy-prod
  namespace: team-app-a
spec:
  serviceAccountName: app-a-deployer
  action: Deploy

  # Pull from OCI registry (swap for S3 source if needed)
  source:
    type: OCI
    oci:
      reference: ghcr.io/myorg/packages/my-app:2.1.0
      credentialRef:
        name: registry-credentials

  # To pull from S3 instead:
  # source:
  #   type: S3
  #   s3:
  #     bucket: myorg-artifacts-prod
  #     key: zarf-packages/my-app/zarf-package-my-app-amd64-2.1.0.tar.zst
  #     region: us-west-2
  #     credentialRef:
  #       name: s3-credentials   # type: EnvVar is the default

  deploy:
    # InCluster deploys to the cluster where Forge runs — no kubeconfig needed
    target: InCluster
    namespace: my-app
    variables:
      ENVIRONMENT: production
      REPLICAS: "3"
    timeout: 45m
    adoptionPolicy: Error   # Fail if resources already exist without Forge ownership
    retry:
      maxRetries: 2
      initialBackoff: 1m

    # For external cluster deployment, replace target and add externalCluster:
    # target: ExternalCluster
    # externalCluster:
    #   secretRef:
    #     name: prod-cluster-kubeconfig
    #   key: kubeconfig
    #   context: prod-context

  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: "1"
      memory: 2Gi
```

---

## Volume Sizing and Resource Limits

### Volume Sizes

Forge uses `emptyDir` volumes for job pod ephemeral storage. The `volumeSizes` field controls the `sizeLimit` for each volume. The PVC (`artifactStorage`) is used to pass the built package between chained actions (e.g., Build → Publish).

| Volume | Default | Used For |
|---|---|---|
| `workspace` | 10Gi | Source checkout, build intermediates |
| `output` | 10Gi | Built package tarball before upload |
| `tmp` | 1Gi | Temporary files |
| `home` | 1Gi | Tool caches, `.zarf`, `.uds` |
| `artifactStorage` (PVC) | 10Gi | Artifact handoff between chained actions |

**Sizing guidance:**

- Small packages (< 500 MB): defaults are fine.
- Medium packages (500 MB – 3 GB): set `workspace: 20Gi`, `output: 15Gi`.
- Large packages (3 GB+): set `workspace: 50Gi`, `output: 40Gi`, and increase `artifactStorage` if using chained actions.
- If building many image layers from scratch, add extra headroom to `workspace`.

Example for large package:

```yaml
spec:
  volumeSizes:
    workspace: 50Gi
    output: 40Gi
    tmp: 4Gi
    home: 2Gi
  # Also size the artifact PVC if chaining Build → Publish
    artifactStorage: 20Gi
```

### Resource Requests and Limits

Resource requests affect scheduling; limits cap consumption. Over-constraining CPU will slow builds significantly — prefer generous CPU limits.

| Workload | Requests | Limits |
|---|---|---|
| Small build/publish | `cpu: 500m`, `memory: 1Gi` | `cpu: 2`, `memory: 4Gi` |
| Large build/publish | `cpu: 1`, `memory: 2Gi` | `cpu: 4`, `memory: 8Gi` |
| Deploy only | `cpu: 250m`, `memory: 512Mi` | `cpu: 1`, `memory: 2Gi` |

```yaml
spec:
  resources:
    requests:
      cpu: "1"
      memory: 2Gi
    limits:
      cpu: "4"
      memory: 8Gi
```

To restrict all jobs in a namespace regardless of per-job settings, apply a `LimitRange`:

```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: forge-job-limits
  namespace: team-platform
spec:
  limits:
  - type: Container
    default:
      cpu: "2"
      memory: 4Gi
    defaultRequest:
      cpu: 500m
      memory: 1Gi
    max:
      cpu: "8"
      memory: 16Gi
```

To cap total resource consumption by a namespace:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: forge-quota
  namespace: team-platform
spec:
  hard:
    requests.cpu: "10"
    requests.memory: 20Gi
    limits.cpu: "40"
    limits.memory: 80Gi
    count/zarfpackagejobs.forge.dev: "50"
    count/jobs.batch: "20"
```

---

## Multi-Cluster Deployments

### Deploying Forge to Multiple Clusters

If you manage multiple clusters, install Forge in each. The CRDs and Helm values are identical; only the RBAC mode and concurrency settings may differ between clusters.

```bash
# Cluster A (platform/build cluster)
kubectl config use-context cluster-a
helm upgrade --install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=3 \
  --set webhook.replicaCount=3

# Cluster B (app/deploy target — Forge installed here so teams can deploy locally)
kubectl config use-context cluster-b
helm upgrade --install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=2
```

### Deploying FROM One Cluster TO Another

A job running in Cluster A can deploy a package to Cluster B using the `ExternalCluster` target. Store Cluster B's kubeconfig as a `Secret` in the job's namespace on Cluster A:

```bash
# Extract a deploy-only service account token from Cluster B
kubectl config use-context cluster-b
kubectl create serviceaccount forge-deployer -n kube-system
kubectl create clusterrolebinding forge-deployer \
  --clusterrole=cluster-admin \
  --serviceaccount=kube-system:forge-deployer
TOKEN=$(kubectl create token forge-deployer -n kube-system --duration=0s)

# Create the kubeconfig secret on Cluster A
kubectl config use-context cluster-a
kubectl create secret generic cluster-b-kubeconfig \
  --namespace team-platform \
  --from-literal=kubeconfig="$(kubectl config view --minify --raw \
    --context=cluster-b | \
    sed "s/token:.*/token: ${TOKEN}/")"
```

Then reference it in the job:

```yaml
deploy:
  target: ExternalCluster
  externalCluster:
    secretRef:
      name: cluster-b-kubeconfig
    key: kubeconfig
```

Ensure the `ServiceAccount` for the job allows `ExternalCluster` as a deploy target:

```yaml
forge.dev/allowed-deploy-targets: "ExternalCluster"
```

---

## Verification and Troubleshooting

### After Installation

```bash
# All Forge components healthy
kubectl get pods -n forge-system

# CRDs installed
kubectl get crd | grep forge.dev

# Webhook registered
kubectl get validatingwebhookconfiguration forge-webhook

# Controller logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller -f
```

### Watch a Job

```bash
# Submit and watch
kubectl apply -f my-job.yaml
kubectl get zarfpackagejobs -n team-platform -w

# Describe for events and status detail
kubectl describe zarfpackagejob my-app-build-publish -n team-platform

# Follow the spawned job's pod logs
kubectl logs -n team-platform -l forge.dev/job=my-app-build-publish -f
```

### Common Failures

**Webhook admission rejected:**

The webhook enforces `ServiceAccount` annotations. The error message will name the specific policy violation (e.g., action not allowed, source repo not permitted). Check that the `ServiceAccount` annotations match the job spec.

```bash
kubectl describe serviceaccount platform-pipeline -n team-platform
```

**Secret not found at admission:**

Forge validates that referenced secrets exist before admitting the job. Create all credential secrets before submitting the job.

**Job pod OOMKilled or evicted:**

Increase `resources.limits.memory` in the job spec, and check `volumeSizes` — an `emptyDir` that exceeds its `sizeLimit` will cause the pod to be evicted.

**Build runs out of disk space:**

Increase `volumeSizes.workspace` and/or `volumeSizes.output`. For chained actions, the artifact PVC also needs to be large enough to hold the completed package tarball.

**Job stuck in `Queued`:**

The global or per-namespace concurrency limit has been reached. Either wait for running jobs to complete, or raise the limits:

```bash
helm upgrade forge forge/forge \
  --namespace forge-system \
  --set controller.concurrency.maxJobsPerNamespace=20
```

**Controller not processing jobs in another namespace (namespace-scoped mode):**

In namespace-scoped mode, all resources must be in the Forge controller's namespace. Move the job resource, ServiceAccount, and secrets to that namespace.
