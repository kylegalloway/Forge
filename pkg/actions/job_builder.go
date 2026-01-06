package actions

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
)

// JobBuilder provides a fluent interface for building Kubernetes Jobs
// with common patterns used across Zarf and UDS action handlers.
type JobBuilder struct {
	job             *batchv1.Job
	initContainers  []corev1.Container
	volumes         []corev1.Volume
	volumeMounts    []corev1.VolumeMount
	envVars         []corev1.EnvVar
	containerImage  string
	containerName   string
	command         []string
	args            []string
	workingDir      string
	resources       corev1.ResourceRequirements
	nodeSelector    map[string]string
	affinity        *corev1.Affinity
	tolerations     []corev1.Toleration
	artifactPVCName string
	kubeClient      kubernetes.Interface
}

// NewJobBuilder creates a new JobBuilder with basic metadata.
func NewJobBuilder(name, namespace string) *JobBuilder {
	return &JobBuilder{
		job: &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    make(map[string]string),
			},
			Spec: batchv1.JobSpec{},
		},
		volumes:      []corev1.Volume{},
		volumeMounts: []corev1.VolumeMount{},
		envVars:      []corev1.EnvVar{},
	}
}

// WithKubeClient sets the Kubernetes client for Job creation.
func (b *JobBuilder) WithKubeClient(client kubernetes.Interface) *JobBuilder {
	b.kubeClient = client
	return b
}

// WithOwnerReference adds an owner reference to the Job.
func (b *JobBuilder) WithOwnerReference(owner metav1.Object, gvk schema.GroupVersionKind) *JobBuilder {
	b.job.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(owner, gvk),
	}
	return b
}

// WithLabels adds labels to the Job and Pod template.
func (b *JobBuilder) WithLabels(labels map[string]string) *JobBuilder {
	for k, v := range labels {
		b.job.Labels[k] = v
	}
	return b
}

// WithContainerImage sets the container image.
func (b *JobBuilder) WithContainerImage(image string) *JobBuilder {
	b.containerImage = image
	return b
}

// WithContainerName sets the container name.
func (b *JobBuilder) WithContainerName(name string) *JobBuilder {
	b.containerName = name
	return b
}

// WithCommand sets the container command.
func (b *JobBuilder) WithCommand(command []string) *JobBuilder {
	b.command = command
	return b
}

// WithArgs sets the container arguments.
func (b *JobBuilder) WithArgs(args []string) *JobBuilder {
	b.args = args
	return b
}

// WithWorkingDir sets the container working directory.
func (b *JobBuilder) WithWorkingDir(dir string) *JobBuilder {
	b.workingDir = dir
	return b
}

// WithResources sets the container resource requirements.
func (b *JobBuilder) WithResources(resources corev1.ResourceRequirements) *JobBuilder {
	b.resources = resources
	return b
}

// WithNodeSelector sets node selection constraints for the job pod.
func (b *JobBuilder) WithNodeSelector(nodeSelector map[string]string) *JobBuilder {
	b.nodeSelector = nodeSelector
	return b
}

// WithAffinity sets pod scheduling affinity rules.
func (b *JobBuilder) WithAffinity(affinity *corev1.Affinity) *JobBuilder {
	b.affinity = affinity
	return b
}

// WithTolerations sets pod tolerations.
func (b *JobBuilder) WithTolerations(tolerations []corev1.Toleration) *JobBuilder {
	b.tolerations = tolerations
	return b
}

// WithBackoffLimit sets the Job backoff limit.
func (b *JobBuilder) WithBackoffLimit(limit int32) *JobBuilder {
	b.job.Spec.BackoffLimit = &limit
	return b
}

// WithZarfRetryPolicy configures retry behavior for Zarf actions.
// If policy is nil or MaxRetries is nil, defaults to BackoffLimit=0 (no retries).
func (b *JobBuilder) WithZarfRetryPolicy(policy *zarfv1alpha1.RetryPolicy) *JobBuilder {
	if policy == nil || policy.MaxRetries == nil {
		// Default behavior: no retries
		limit := int32(0)
		b.job.Spec.BackoffLimit = &limit
		return b
	}

	// Set Kubernetes BackoffLimit to MaxRetries
	b.job.Spec.BackoffLimit = policy.MaxRetries
	return b
}

// WithUDSRetryPolicy configures retry behavior for UDS actions.
// If policy is nil or MaxRetries is nil, defaults to BackoffLimit=0 (no retries).
func (b *JobBuilder) WithUDSRetryPolicy(policy *udsv1alpha2.RetryPolicy) *JobBuilder {
	if policy == nil || policy.MaxRetries == nil {
		// Default behavior: no retries
		limit := int32(0)
		b.job.Spec.BackoffLimit = &limit
		return b
	}

	// Set Kubernetes BackoffLimit to MaxRetries
	b.job.Spec.BackoffLimit = policy.MaxRetries
	return b
}

// WithActiveDeadlineSeconds sets the Job active deadline.
func (b *JobBuilder) WithActiveDeadlineSeconds(seconds int64) *JobBuilder {
	b.job.Spec.ActiveDeadlineSeconds = &seconds
	return b
}

// WithTTLSecondsAfterFinished sets the Job TTL after completion.
func (b *JobBuilder) WithTTLSecondsAfterFinished(seconds int32) *JobBuilder {
	b.job.Spec.TTLSecondsAfterFinished = &seconds
	return b
}

// WithInitContainers adds init containers to the Pod spec.
func (b *JobBuilder) WithInitContainers(containers []corev1.Container) *JobBuilder {
	b.initContainers = containers
	return b
}

// WithWorkspaceVolume adds workspace and output EmptyDir volumes.
func (b *JobBuilder) WithWorkspaceVolume() *JobBuilder {
	b.volumes = append(b.volumes,
		corev1.Volume{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "output",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)
	b.volumeMounts = append(b.volumeMounts,
		corev1.VolumeMount{
			Name:      "workspace",
			MountPath: "/workspace",
		},
		corev1.VolumeMount{
			Name:      "output",
			MountPath: "/output",
		},
	)
	return b
}

// WithArtifactPVC adds a PVC volume for artifact storage.
func (b *JobBuilder) WithArtifactPVC(pvcName string) *JobBuilder {
	if pvcName == "" {
		return b
	}
	b.artifactPVCName = pvcName
	b.volumes = append(b.volumes, corev1.Volume{
		Name: "artifacts",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	})
	b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
		Name:      "artifacts",
		MountPath: "/artifacts",
	})
	return b
}

// WithDockerConfigSecret adds a docker config secret volume.
func (b *JobBuilder) WithDockerConfigSecret(secretName string) *JobBuilder {
	if secretName == "" {
		return b
	}
	b.volumes = append(b.volumes, corev1.Volume{
		Name: "docker-config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{
						Key:  ".dockerconfigjson",
						Path: "config.json",
					},
				},
			},
		},
	})
	return b
}

// WithCustomVolume adds a custom volume.
func (b *JobBuilder) WithCustomVolume(volume corev1.Volume) *JobBuilder {
	b.volumes = append(b.volumes, volume)
	return b
}

// WithCustomVolumeMount adds a custom volume mount.
func (b *JobBuilder) WithCustomVolumeMount(mount corev1.VolumeMount) *JobBuilder {
	b.volumeMounts = append(b.volumeMounts, mount)
	return b
}

// WithEnvVar adds an environment variable.
func (b *JobBuilder) WithEnvVar(name, value string) *JobBuilder {
	b.envVars = append(b.envVars, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return b
}

// NonRootSecurityContext returns a standard non-root security context with UID 1000.
// For custom UIDs, use NonRootSecurityContextWithUID.
func NonRootSecurityContext() *corev1.SecurityContext {
	return NonRootSecurityContextWithUID(1000)
}

// NonRootSecurityContextWithUID returns a non-root security context with specified UID.
// Used to create secure container contexts for Zarf (UID 1000) and UDS (UID 65532) jobs.
func NonRootSecurityContextWithUID(uid int64) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             Ptr(true),
		RunAsUser:                Ptr(uid),
		AllowPrivilegeEscalation: Ptr(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// NonRootPodSecurityContext returns a standard non-root pod security context with UID 1000.
// For custom UIDs, use NonRootPodSecurityContextWithUID.
func NonRootPodSecurityContext() *corev1.PodSecurityContext {
	return NonRootPodSecurityContextWithUID(1000)
}

// NonRootPodSecurityContextWithUID returns a non-root pod security context with specified UID.
// Used to create secure pod contexts for Zarf (UID 1000) and UDS (UID 65532) jobs.
func NonRootPodSecurityContextWithUID(uid int64) *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: Ptr(true),
		RunAsUser:    Ptr(uid),
		FSGroup:      Ptr(uid),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// Build constructs the final Job specification.
func (b *JobBuilder) Build() *batchv1.Job {
	// Build the main container
	container := corev1.Container{
		Name:            b.containerName,
		Image:           b.containerImage,
		Command:         b.command,
		Args:            b.args,
		WorkingDir:      b.workingDir,
		VolumeMounts:    b.volumeMounts,
		Env:             b.envVars,
		SecurityContext: NonRootSecurityContext(),
		Resources:       b.resources,
	}

	// Build the pod template
	b.job.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: b.job.Labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:   corev1.RestartPolicyNever,
			InitContainers:  b.initContainers,
			Containers:      []corev1.Container{container},
			Volumes:         b.volumes,
			SecurityContext: NonRootPodSecurityContext(),
			NodeSelector:    b.nodeSelector,
			Affinity:        b.affinity,
			Tolerations:     b.tolerations,
		},
	}

	return b.job
}

// CreateOrGet creates the Job if it doesn't exist, or returns the existing Job.
func (b *JobBuilder) CreateOrGet(ctx context.Context) (*batchv1.Job, error) {
	if b.kubeClient == nil {
		return nil, fmt.Errorf("kubeClient not set")
	}

	job := b.Build()

	// Check if job already exists
	existingJob, err := b.kubeClient.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Job already exists, reusing", "name", job.Name, "namespace", job.Namespace)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := b.kubeClient.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// DefaultResourceRequirements returns default resource requirements for Jobs.
func DefaultResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    MustParseQuantity("100m"),
			corev1.ResourceMemory: MustParseQuantity("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    MustParseQuantity("1000m"),
			corev1.ResourceMemory: MustParseQuantity("512Mi"),
		},
	}
}

// BuildResourceRequirements returns resource requirements for Build/Create operations.
// Used by Zarf build and UDS create actions which involve compiling and bundling packages.
// Includes higher ephemeral storage limits for large package builds.
func BuildResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("500m"),
			corev1.ResourceMemory:           MustParseQuantity("1Gi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("10Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("2000m"),
			corev1.ResourceMemory:           MustParseQuantity("4Gi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("20Gi"),
		},
	}
}

// PublishResourceRequirements returns resource requirements for Publish operations.
// Used by both Zarf and UDS publish actions which upload artifacts to registries/S3.
// Includes moderate ephemeral storage for reading artifacts.
func PublishResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("200m"),
			corev1.ResourceMemory:           MustParseQuantity("512Mi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("5Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("1000m"),
			corev1.ResourceMemory:           MustParseQuantity("2Gi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("10Gi"),
		},
	}
}

// DeployResourceRequirements returns resource requirements for Deploy operations.
// Used by both Zarf and UDS deploy actions which install packages to clusters.
// Includes ephemeral storage for extracting and processing packages.
func DeployResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("500m"),
			corev1.ResourceMemory:           MustParseQuantity("1Gi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("10Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              MustParseQuantity("2000m"),
			corev1.ResourceMemory:           MustParseQuantity("4Gi"),
			corev1.ResourceEphemeralStorage: MustParseQuantity("20Gi"),
		},
	}
}

// ParseTimeoutWithDefault parses a timeout string (e.g., "30m", "1h") and returns seconds.
// If parsing fails or timeout is empty, returns the default value.
// Used by action handlers to convert timeout strings from specs to Job activeDeadlineSeconds.
func ParseTimeoutWithDefault(timeoutStr string, defaultSeconds int64) int64 {
	if timeoutStr == "" {
		return defaultSeconds
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		klog.V(4).InfoS("Invalid timeout format, using default",
			"timeout", timeoutStr,
			"default", defaultSeconds,
			"error", err)
		return defaultSeconds
	}

	return int64(timeout.Seconds())
}

// AddKubeconfigVolume adds a kubeconfig secret volume to a Job.
// Returns early if kubeconfigSecretName is empty or Job has no containers.
// Used by deploy handlers to mount external cluster kubeconfig into deploy jobs.
func AddKubeconfigVolume(job *batchv1.Job, kubeconfigSecretName string) {
	if kubeconfigSecretName == "" {
		return
	}

	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add kubeconfig volume", "job", job.Name)
		return
	}

	// Add volume - mounts entire secret
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: kubeconfigSecretName,
			},
		},
	})

	// Add volume mount
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: "/kubeconfig",
			ReadOnly:  true,
		},
	)
}

// AddArtifactPVCVolume adds an artifact PVC volume to a Job.
// Returns early if pvcName is empty or Job has no containers.
// Used by multi-action jobs to share artifacts between build/publish/deploy phases.
func AddArtifactPVCVolume(job *batchv1.Job, pvcName string) {
	if pvcName == "" {
		return
	}

	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add artifact PVC volume", "job", job.Name)
		return
	}

	// Add volume
	artifactVolume := corev1.Volume{
		Name: "artifacts",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, artifactVolume)

	// Add volume mount
	artifactMount := corev1.VolumeMount{
		Name:      "artifacts",
		MountPath: "/artifacts",
	}
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		artifactMount,
	)
}
