package actions

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"github.com/kylegalloway/forge/pkg/apis/common"
	"github.com/kylegalloway/forge/pkg/destinations"
)

// JobParams holds all the per-action parameters needed to build a Kubernetes Job.
// The six action handlers (zarf/build, uds/create, zarf/publish, uds/publish,
// zarf/deploy, uds/deploy) populate this struct and call BuildActionJob, which
// owns the full JobBuilder chain.
type JobParams struct {
	// Job identity
	JobName   string
	Namespace string

	// Container config
	CLIImage      string
	ContainerUID  int64
	ContainerName string

	// Job command
	Command    []string // e.g. []string{"/bin/sh", "-c"}
	Args       []string // the actual shell command
	WorkingDir string

	// Labels and owner
	Labels   map[string]string
	OwnerRef metav1.Object
	OwnerGVK schema.GroupVersionKind

	// Pod scheduling
	NodeSelector map[string]string
	Affinity     *corev1.Affinity
	Tolerations  []corev1.Toleration

	// Resources
	Resources corev1.ResourceRequirements

	// Retry/timeout
	MaxRetries *int32
	Timeout    int64 // active deadline in seconds

	// Volumes
	VolumeSizes     *common.VolumeSizes
	ArtifactPVCName string

	// Init containers built by the caller
	InitContainers []corev1.Container

	// Source credential volumes (populated only for standalone publish/deploy)
	SourceOCICredSecret string // secret name for docker-config volume
	SourceS3CredVol     *corev1.Volume
	SourceGitCredVol    *corev1.Volume

	// Registry credentials for image pulls during build/create
	RegistryCredSecret    string // pragma: allowlist secret
	RegistryCredMountPath string

	// Destination job config (volumes, mounts, env vars)
	DestJobConfig *destinations.JobConfig

	// Extra mounts (already merged spec-level + action-level)
	ExtraMounts []common.ExtraMount

	// Service account
	ServiceAccountName string

	// Debug mode
	DebugMode bool

	// Extra environment variables (e.g. ZARF_CONFIRM, ZARF_NAMESPACE, UDS_TIMEOUT)
	EnvVars []corev1.EnvVar

	// Deploy kubeconfig — exactly one of the two variants below is used
	KubeconfigSecretName   string // pragma: allowlist secret
	KubeconfigKey          string
	UseInClusterKubeconfig bool

	// Kubernetes client used to create the Job
	KubeClient kubernetes.Interface
}

// BuildActionJob constructs and creates (or retrieves an existing) Kubernetes Job
// from the supplied JobParams. All job-construction logic is centralized here;
// the six action handlers are thin adapters that populate JobParams and call this.
func BuildActionJob(ctx context.Context, p JobParams) (*batchv1.Job, error) {
	builder := NewJobBuilder(p.JobName, p.Namespace).
		WithKubeClient(p.KubeClient).
		WithOwnerReference(p.OwnerRef, p.OwnerGVK).
		WithLabels(p.Labels).
		WithContainerImage(p.CLIImage).
		WithContainerName(p.ContainerName).
		WithCommand(p.Command).
		WithArgs(p.Args).
		WithWorkingDir(p.WorkingDir).
		WithUserConfig(p.ContainerUID).
		WithResources(p.Resources).
		WithNodeSelector(p.NodeSelector).
		WithAffinity(p.Affinity).
		WithTolerations(p.Tolerations).
		WithBackoffLimit(backoffLimit(p.MaxRetries)).
		WithActiveDeadlineSeconds(p.Timeout).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(p.InitContainers).
		WithWorkspaceVolume(p.VolumeSizes).
		WithArtifactPVC(p.ArtifactPVCName).
		WithServiceAccountName(p.ServiceAccountName).
		WithDebugMode(p.DebugMode)

	// Source credential volumes
	if p.SourceOCICredSecret != "" {
		builder.WithDockerConfigSecret(p.SourceOCICredSecret)
	}
	if p.SourceS3CredVol != nil {
		builder.WithCustomVolume(*p.SourceS3CredVol)
	}
	if p.SourceGitCredVol != nil {
		builder.WithCustomVolume(*p.SourceGitCredVol)
	}

	// Registry credentials for build/create
	if p.RegistryCredSecret != "" { // pragma: allowlist secret
		builder.WithRegistryCredentials(p.RegistryCredSecret, p.RegistryCredMountPath) // pragma: allowlist secret
	}

	// Destination-specific volumes, mounts, and env vars
	if p.DestJobConfig != nil {
		for _, vol := range p.DestJobConfig.Volumes {
			builder.WithCustomVolume(vol)
		}
		for _, mount := range p.DestJobConfig.VolumeMounts {
			builder.WithCustomVolumeMount(mount)
		}
		for _, env := range p.DestJobConfig.Env {
			builder.WithCustomEnvVar(env)
		}
	}

	// Caller-supplied extra env vars
	for _, env := range p.EnvVars {
		builder.WithEnvVar(env.Name, env.Value)
	}

	// Extra mounts (spec-level + action-level, already merged by caller)
	builder.WithExtraMounts(p.ExtraMounts)

	// Kubeconfig for deploy actions
	if p.UseInClusterKubeconfig {
		builder.WithInClusterKubeconfig()
	} else if p.KubeconfigSecretName != "" { // pragma: allowlist secret
		builder.WithKubeconfigVolume(p.KubeconfigSecretName, p.KubeconfigKey) // pragma: allowlist secret
	}

	return builder.CreateOrGet(ctx)
}

// backoffLimit converts a *int32 MaxRetries pointer (from a RetryPolicy) to the
// int32 value used by Job.Spec.BackoffLimit.  Nil means no retries (0).
func backoffLimit(maxRetries *int32) int32 {
	if maxRetries == nil {
		return 0
	}
	return *maxRetries
}

// ActionResultFromJob builds a standard ActionResult from a newly created Job.
func ActionResultFromJob(job *batchv1.Job, message string) *ActionResult {
	return &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   message,
		StartTime: metav1.Now(),
		Completed: false,
	}
}
