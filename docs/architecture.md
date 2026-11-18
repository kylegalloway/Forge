# Forge Architecture

## Overview

Forge is a Kubernetes operator that manages `ZarfPackageJob` and `UDSBundleJob` CRDs. When a user creates one of these resources, Forge validates it, builds Kubernetes Jobs to execute the requested operations (Build/Create → Publish → Deploy), monitors those jobs, and updates the CRD status with results.

---

## System Context

```mermaid
C4Context
    title Forge Operator — System Context

    Person(admin, "Platform Admin", "Configures ServiceAccounts with permission annotations")
    Person(dev, "Developer", "Submits ZarfPackageJob / UDSBundleJob manifests")

    System(forge, "Forge Operator", "Builds, publishes, and deploys Zarf packages and UDS bundles via Kubernetes Jobs")

    System_Ext(k8s, "Kubernetes API Server", "Stores CRDs, Jobs, Secrets")
    System_Ext(git, "Git Repositories", "Package source code")
    System_Ext(registry, "OCI Registries", "Built package artifacts")
    System_Ext(s3, "S3 / Object Storage", "Package artifacts & credentials")
    System_Ext(cluster, "Target Clusters", "Zarf/UDS deployment targets")

    Rel(admin, k8s, "Creates ServiceAccounts with forge.dev/* annotations")
    Rel(dev, k8s, "kubectl apply ZarfPackageJob / UDSBundleJob")
    Rel(forge, k8s, "Watches CRDs, creates/monitors Jobs, updates status")
    Rel(forge, git, "Clone source (via init containers)")
    Rel(forge, registry, "Push/pull OCI artifacts")
    Rel(forge, s3, "Push/pull S3 artifacts")
    Rel(forge, cluster, "Deploy packages (in-cluster or external)")
```

---

## Container Diagram

```mermaid
C4Container
    title Forge Operator — Containers

    System_Boundary(forge, "Forge Operator (forge-system namespace)") {
        Container(controller, "Controller", "Go binary", "Reconciles CRDs; creates, monitors, and chains Kubernetes Jobs")
        Container(webhook, "Webhook Server", "Go binary / TLS :8443", "Validates CRDs against ServiceAccount permission annotations before admission")
        Container(plugin, "kubectl-forge", "Go CLI plugin", "Debugging: logs, pods, cancel, retry, diagnose, download")
    }

    System_Ext(k8s_api, "Kubernetes API Server")
    System_Ext(etcd, "etcd", "CRD + Job state persistence")
    System_Ext(prometheus, "Prometheus", "Scrapes /metrics :8080")

    Rel(controller, k8s_api, "List/Watch/Update CRDs & Jobs")
    Rel(webhook, k8s_api, "AdmissionReview validation")
    Rel(controller, prometheus, "Exposes OpenTelemetry metrics")
    Rel(plugin, k8s_api, "kubectl extension queries")
```

---

## Component Diagram

```mermaid
graph TB
    subgraph "cmd/controller"
        MAIN["main.go<br/>─────────────<br/>• parse flags<br/>• build k8s clients<br/>• init telemetry<br/>• leader election (HA)<br/>• health/metrics servers<br/>• run Zarf + UDS controllers"]
    end

    subgraph "pkg/controller"
        GC["GenericController[T]<br/>─────────────<br/>• Informer (CRD + Job)<br/>• Work queue<br/>• Reconcile loop<br/>• ConcurrencyLimiter"]
        GM["GenericJobMonitor[T]<br/>─────────────<br/>• 10s poll interval<br/>• label selector scan<br/>• status → CRD<br/>• trigger next action"]
        COND["conditions.go<br/>─────────────<br/>• SetCondition<br/>• Ready / Reconciling<br/>• BuildSucceeded<br/>• PublishSucceeded<br/>• DeploySucceeded"]
    end

    subgraph "pkg/actions"
        JB["JobBuilder (fluent API)<br/>─────────────<br/>• WithContainerImage<br/>• WithInitContainers<br/>• WithWorkspaceVolume<br/>• WithArtifactPVC<br/>• WithSecurity contexts<br/>• Build() → batchv1.Job"]
        AH["ActionHandler[T] interface<br/>Execute(ctx, T) → ActionResult"]
    end

    subgraph "pkg/actions/zarf"
        ZBH["BuildHandler"]
        ZPH["PublishHandler"]
        ZDH["DeployHandler"]
    end

    subgraph "pkg/actions/uds"
        UCH["CreateHandler"]
        UPH["PublishHandler"]
        UDH["DeployHandler"]
        URH["RunnerHandler (PreTasks)"]
    end

    subgraph "pkg/webhook"
        WV["ZarfPackageJobValidator<br/>UDSBundleJobValidator<br/>─────────────<br/>• ServiceAccount lookup<br/>• Annotation-based RBAC<br/>• Glob pattern matching<br/>• ExtraArgs injection scan"]
    end

    subgraph "pkg/sources"
        GIT["GitSource<br/>git clone init container"]
        S3S["S3Source<br/>aws s3 cp init container"]
        OCIS["OCISource<br/>skopeo pull init container"]
        LOCS["LocalSource<br/>hostPath (dev only)"]
    end

    subgraph "pkg/destinations"
        S3D["S3Destination<br/>publish → s3://"]
        OCID["OCIDestination<br/>publish → oci://"]
        LOCD["LocalDestination<br/>publish → tarball://"]
    end

    subgraph "pkg/apis"
        ZPJ["ZarfPackageJob<br/>forge.dev/v1alpha3<br/>─────────────<br/>Spec: action, source,<br/>build/publish/deploy cfg,<br/>SA, resources, mounts<br/>─────────────<br/>Status: phase, conditions,<br/>buildStatus, publishStatus,<br/>deployStatus"]
        UBJ["UDSBundleJob<br/>forge.dev/v1alpha3<br/>─────────────<br/>Spec: action, source,<br/>create/publish/deploy cfg,<br/>preTasks, SA, resources<br/>─────────────<br/>Status: phase, conditions,<br/>createStatus, publishStatus,<br/>deployStatus"]
        CMN["pkg/apis/common<br/>─────────────<br/>PackageResource interface<br/>VolumeSizes<br/>ExtraMount<br/>SecretReference<br/>AWSCredentialRef<br/>ExternalClusterConfig"]
    end

    subgraph "pkg/constants"
        CONST["Phase constants<br/>Condition types/reasons<br/>Label & annotation keys<br/>Volume names & paths<br/>Default images & UIDs<br/>Timeouts & intervals"]
    end

    subgraph "pkg/telemetry + pkg/logging"
        TEL["OpenTelemetry<br/>Prometheus export<br/>Structured logging<br/>Correlation IDs"]
    end

    MAIN --> GC
    GC --> GM
    GC --> COND
    GC --> ZBH
    GC --> ZPH
    GC --> ZDH
    GC --> UCH
    GC --> UPH
    GC --> UDH
    ZBH --> JB
    ZPH --> JB
    ZDH --> JB
    UCH --> JB
    UPH --> JB
    UDH --> JB
    JB --> GIT
    JB --> S3S
    JB --> OCIS
    JB --> LOCS
    ZPH --> S3D
    ZPH --> OCID
    ZPH --> LOCD
    UPH --> S3D
    UPH --> OCID
    WV -.->|validate on CREATE/UPDATE| ZPJ
    WV -.->|validate on CREATE/UPDATE| UBJ
    GC --> ZPJ
    GC --> UBJ
    ZBH --> AH
    ZPH --> AH
    ZDH --> AH
    UCH --> AH
    UPH --> AH
    UDH --> AH
    ZPJ --> CMN
    UBJ --> CMN
    GC --> CONST
    JB --> CONST
    GC --> TEL
    GM --> TEL
```

---

## End-to-End Request Flow

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant K8s as K8s API Server
    participant WH as Webhook Server
    participant SA as ServiceAccount
    participant CTRL as Controller
    participant JB as JobBuilder
    participant JOB as Kubernetes Job
    participant MON as JobMonitor

    Dev->>K8s: kubectl apply ZarfPackageJob (action: BuildPublish)
    K8s->>WH: AdmissionReview (CREATE)
    WH->>SA: Get ServiceAccount + annotations
    SA-->>WH: forge.dev/allowed-actions: BuildPublish<br/>forge.dev/allowed-source-repos: github.com/myorg/*
    WH->>WH: Validate action, sources, destinations,<br/>extraArgs (injection scan)
    WH-->>K8s: Allowed
    K8s-->>Dev: ZarfPackageJob created

    loop Informer Watch
        K8s->>CTRL: ZarfPackageJob event
    end

    CTRL->>CTRL: Enqueue reconcile key

    rect rgb(220, 240, 255)
        Note over CTRL,JOB: BUILD PHASE
        CTRL->>JB: BuildHandler.Execute()
        JB->>JB: Build batchv1.Job spec<br/>(init: git-clone → main: zarf build)
        JB->>K8s: Create Job (my-build-build)<br/>labels: app=forge, package=my-build, action=build
        K8s-->>JOB: Schedule pod
        JOB->>JOB: Init: git clone repo → /workspace
        JOB->>JOB: Main: zarf package create . --confirm<br/>Output → /artifacts/package.tar.zst (PVC)
        JOB-->>K8s: Job succeeded
    end

    loop Every 10s
        MON->>K8s: List Jobs (label: app=forge)
        K8s-->>MON: Jobs + statuses
        MON->>K8s: Update ZarfPackageJob.Status<br/>buildStatus.phase=Completed, BuildSucceeded=True
        MON->>CTRL: Trigger next action: Publish
    end

    rect rgb(220, 255, 220)
        Note over CTRL,JOB: PUBLISH PHASE
        CTRL->>JB: PublishHandler.Execute()
        JB->>JB: Build Job spec<br/>(main: zarf package publish oci://registry/...)
        JB->>K8s: Create Job (my-build-publish)<br/>Attach artifact PVC (read-only)
        K8s-->>JOB: Schedule pod
        JOB->>JOB: zarf package publish /artifacts/package.tar.zst
        JOB-->>K8s: Job succeeded
    end

    loop Every 10s
        MON->>K8s: Update ZarfPackageJob.Status<br/>publishStatus.phase=Completed<br/>PublishSucceeded=True, Ready=True
    end

    K8s-->>Dev: kubectl get zpj → phase: Completed
```

---

## CRD Type Hierarchy

```mermaid
classDiagram
    class PackageResource {
        <<interface>>
        +GetName() string
        +GetNamespace() string
        +GetServiceAccountName() string
        +GetAction() string
        +GetUseArtifactPVC() bool
        +GetRetainArtifactPVC() bool
        +GetDebugMode() bool
        +GetDebugActions() []string
        +GetVolumeSizes() *VolumeSizes
    }

    class ZarfPackageJob {
        Spec: ZarfPackageJobSpec
        Status: OperationStatus
    }

    class ZarfPackageJobSpec {
        +ServiceAccountName string
        +Action string
        +Source PackageSource
        +Build *BuildConfig
        +Publish *PublishConfig
        +Deploy *DeployConfig
        +Resources *ResourceRequirements
        +NodeSelector map[string]string
        +Affinity *Affinity
        +Tolerations []Toleration
        +UseArtifactPVC bool
        +RetainArtifactPVC bool
        +ExtraMounts []ExtraMount
        +VolumeSizes *VolumeSizes
        +DebugMode bool
        +DebugActions []string
    }

    class UDSBundleJob {
        Spec: UDSBundleJobSpec
        Status: OperationStatus
    }

    class UDSBundleJobSpec {
        +ServiceAccountName string
        +Action string
        +Source PackageSource
        +Create *CreateConfig
        +Publish *PublishConfig
        +Deploy *DeployConfig
        +PreTasks []RunnerPreTask
        +Resources *ResourceRequirements
        +UseArtifactPVC bool
        +RetainArtifactPVC bool
        +ExtraMounts []ExtraMount
        +VolumeSizes *VolumeSizes
        +DebugMode bool
        +DebugActions []string
    }

    class OperationStatus {
        +Phase string
        +Message string
        +Conditions []Condition
        +BuildStatus *ActionStatus
        +PublishStatus *ActionStatus
        +DeployStatus *ActionStatus
        +ObservedGeneration int64
    }

    class ActionStatus {
        +Phase string
        +Message string
        +JobName string
        +StartTime *Time
        +CompletionTime *Time
        +RetryCount int
    }

    class PackageSource {
        +Type string
        +Git *GitSource
        +S3 *S3Source
        +OCI *OCISource
        +Local *LocalSource
    }

    class VolumeSizes {
        +Workspace *Quantity
        +Output *Quantity
        +Tmp *Quantity
        +Home *Quantity
        +ArtifactStorage *Quantity
    }

    PackageResource <|.. ZarfPackageJob
    PackageResource <|.. UDSBundleJob
    ZarfPackageJob "1" --> "1" ZarfPackageJobSpec
    ZarfPackageJob "1" --> "1" OperationStatus
    UDSBundleJob "1" --> "1" UDSBundleJobSpec
    UDSBundleJob "1" --> "1" OperationStatus
    OperationStatus "1" --> "0..3" ActionStatus
    ZarfPackageJobSpec "1" --> "1" PackageSource
    UDSBundleJobSpec "1" --> "1" PackageSource
    ZarfPackageJobSpec "1" --> "0..1" VolumeSizes
    UDSBundleJobSpec "1" --> "0..1" VolumeSizes
```

---

## Job Volume Layout

```mermaid
graph LR
    subgraph "Kubernetes Job Pod"
        subgraph "Init Containers"
            IC1["source-fetch<br/>(git-clone / s3-cp / skopeo-pull)"]
            IC2["kubeconfig-init<br/>(SA token → kubeconfig)<br/>deploy only"]
        end

        subgraph "Main Container (Zarf or UDS CLI)"
            CMD["zarf package create|publish|deploy<br/>uds create|publish|deploy"]
        end
    end

    subgraph "Volumes"
        WS["workspace EmptyDir<br/>/workspace (10Gi)"]
        OUT["output EmptyDir<br/>/output (10Gi)"]
        TMP["tmp EmptyDir<br/>/tmp (1Gi)"]
        HOME["home EmptyDir<br/>/home/zarf or /home/uds (1Gi)"]
        ART["artifacts PVC<br/>/artifacts<br/>(multi-action workflows)"]
        GIT_CRED["git-creds Secret<br/>/etc/git-secret"]
        REG_CRED["registry-creds Secret<br/>/home/*/\.docker"]
        AWS_CRED["aws-creds Secret<br/>env vars or ~/.aws/credentials"]
        KUBE["kubeconfig Secret or Projected<br/>/etc/kubeconfig"]
        EXTRA["ExtraMounts<br/>ConfigMap or Secret<br/>user-defined paths"]
    end

    IC1 -->|writes source| WS
    IC2 -->|writes| KUBE
    WS -->|source code| CMD
    ART -->|artifact input/output| CMD
    OUT -->|build output| CMD
    TMP --> CMD
    HOME --> CMD
    GIT_CRED --> IC1
    REG_CRED --> CMD
    AWS_CRED --> CMD
    KUBE --> CMD
    EXTRA --> CMD
```

---

## Webhook Permission Model

```mermaid
graph TD
    subgraph "Platform Admin configures"
        SA["ServiceAccount<br/>builder-sa"]
        ANNO["Annotations<br/>─────────────<br/>forge.dev/allowed-actions: Build,Publish,BuildPublish<br/>forge.dev/allowed-source-repos: github.com/myorg/*<br/>forge.dev/allowed-source-registries: ghcr.io/myorg/*<br/>forge.dev/allowed-source-buckets: my-bucket/*<br/>forge.dev/allowed-publish-registries: ghcr.io/myorg/*<br/>forge.dev/allowed-publish-buckets: my-bucket/*<br/>forge.dev/allowed-deploy-targets: InCluster"]
    end

    subgraph "Developer submits"
        ZPJ2["ZarfPackageJob<br/>serviceAccountName: builder-sa<br/>action: BuildPublish<br/>source.git.url: github.com/myorg/repo<br/>publish.destination: ghcr.io/myorg/pkg"]
    end

    subgraph "Webhook validates"
        CHECK1{"action in<br/>allowed-actions?"}
        CHECK2{"source URL matches<br/>allowed-source-repos?"}
        CHECK3{"publish destination matches<br/>allowed-publish-registries?"}
        CHECK4{"extraArgs safe from<br/>injection?"}
        ALLOW["ALLOW"]
        DENY["DENY 403"]
    end

    SA --> ANNO
    ZPJ2 -->|"kubectl apply"| CHECK1
    ANNO --> CHECK1
    CHECK1 -->|yes| CHECK2
    CHECK1 -->|no| DENY
    CHECK2 -->|yes| CHECK3
    CHECK2 -->|no| DENY
    CHECK3 -->|yes| CHECK4
    CHECK3 -->|no| DENY
    CHECK4 -->|safe| ALLOW
    CHECK4 -->|injection detected| DENY
```

---

## Source and Destination Plugin Matrix

```mermaid
graph LR
    subgraph "Sources (pkg/sources)"
        GIT2["GitSource<br/>─────────<br/>SSH key<br/>Token/password<br/>OAuth<br/>Custom hosts"]
        S3SRC["S3Source<br/>─────────<br/>AWS IAM (EnvVar/File/Node)<br/>MinIO compatible<br/>Custom endpoint"]
        OCISRC["OCISource<br/>─────────<br/>Registry credentials<br/>skopeo pull"]
        LOCSRC["LocalSource<br/>─────────<br/>hostPath mount<br/>dev mode only"]
    end

    subgraph "JobBuilder"
        JB2["WithInitContainers()"]
    end

    subgraph "Destinations (pkg/destinations)"
        S3DST["S3Destination<br/>─────────<br/>publish oci://s3://…<br/>AWS creds: EnvVar/File/Node"]
        OCIDST["OCIDestination<br/>─────────<br/>publish oci://registry/…<br/>Docker config secret"]
        LOCDST["LocalDestination<br/>─────────<br/>publish tarball://…<br/>ephemeral storage"]
    end

    GIT2 -->|GetInitContainer()| JB2
    S3SRC -->|GetInitContainer()| JB2
    OCISRC -->|GetInitContainer()| JB2
    LOCSRC -->|GetInitContainer()| JB2

    JB2 --> S3DST
    JB2 --> OCIDST
    JB2 --> LOCDST
```

---

## Action Chain State Machine

```mermaid
stateDiagram-v2
    [*] --> Pending : CRD created

    state "Pending" as Pending
    state "Build / Create Running" as BuildRunning
    state "Build / Create Completed" as BuildDone
    state "Publish Running" as PublishRunning
    state "Publish Completed" as PublishDone
    state "Deploy Running" as DeployRunning
    state "Completed" as Completed
    state "Failed" as Failed

    Pending --> BuildRunning : action includes Build/Create
    Pending --> PublishRunning : action is Publish-only
    Pending --> DeployRunning : action is Deploy-only

    BuildRunning --> BuildDone : Job succeeded
    BuildRunning --> Failed : Job failed (backoff exhausted)

    BuildDone --> PublishRunning : action includes Publish (BuildPublish / BuildPublishDeploy)
    BuildDone --> Completed : action is Build/Create only

    PublishRunning --> PublishDone : Job succeeded
    PublishRunning --> Failed : Job failed

    PublishDone --> DeployRunning : action includes Deploy (BuildPublishDeploy / PublishDeploy)
    PublishDone --> Completed : action is BuildPublish / Publish only

    DeployRunning --> Completed : Job succeeded
    DeployRunning --> Failed : Job failed

    Completed --> [*]
    Failed --> [*]
```

---

## Deployment Architecture (Helm Chart)

```mermaid
graph TB
    subgraph "forge-system namespace"
        subgraph "Controller Deployment (2+ replicas)"
            CTRL2["forge-controller pod<br/>─────────────<br/>cmd/controller<br/>:8080 /metrics<br/>:8081 /healthz /readyz"]
        end

        subgraph "Webhook Deployment (2+ replicas)"
            WH2["forge-webhook pod<br/>─────────────<br/>cmd/webhook<br/>:8443 TLS (ValidatingWebhook)<br/>:8081 /healthz /readyz"]
        end

        SVC1["webhook-svc<br/>ClusterIP :443 → :8443"]
        SVC2["metrics-svc<br/>ClusterIP :8080"]
        VWC["ValidatingWebhookConfiguration<br/>forge.dev/*<br/>CABundle injected"]
        LE["Lease (leader election)<br/>forge-controller-leader"]
    end

    subgraph "User Namespaces"
        ZPJ3["ZarfPackageJob"]
        UBJ3["UDSBundleJob"]
        SA3["ServiceAccount + Annotations"]
        JOBS["Kubernetes Jobs<br/>(created per action)"]
        PVC["Artifact PVC<br/>(multi-action workflows)"]
    end

    subgraph "Cluster-wide RBAC"
        CR["ClusterRole<br/>get/watch/list/update CRDs<br/>create/get/watch/list Jobs<br/>get ServiceAccounts"]
        CRB["ClusterRoleBinding"]
    end

    PROMETHEUS["Prometheus<br/>(external)"]

    VWC --> WH2
    WH2 --> SVC1
    CTRL2 --> LE
    CTRL2 -->|creates| JOBS
    CTRL2 -->|watches| ZPJ3
    CTRL2 -->|watches| UBJ3
    CTRL2 --> SVC2
    SVC2 --> PROMETHEUS
    JOBS --> PVC
    CR --> CRB
    CRB --> CTRL2
    CRB --> WH2
    SA3 -.->|permissions checked by| WH2
```
