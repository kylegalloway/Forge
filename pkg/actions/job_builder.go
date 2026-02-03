package actions

import (
	"context"
	"fmt"
	"slices"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
)

// JobBuilder provides a fluent interface for building Kubernetes Jobs
// with common patterns used across Zarf and UDS action handlers.
type JobBuilder struct {
	job                      *batchv1.Job
	initContainers           []corev1.Container
	volumes                  []corev1.Volume
	volumeMounts             []corev1.VolumeMount
	envVars                  []corev1.EnvVar
	containerImage           string
	containerName            string
	command                  []string
	args                     []string
	workingDir               string
	homeDir                  string
	resources                corev1.ResourceRequirements
	nodeSelector             map[string]string
	affinity                 *corev1.Affinity
	tolerations              []corev1.Toleration
	artifactPVCName          string
	kubeClient               kubernetes.Interface
	podSecurityContext       *corev1.PodSecurityContext
	containerSecurityContext *corev1.SecurityContext
	serviceAccountName       string
	debugMode                bool
	volumeSizes              *common.VolumeSizes
	inClusterKubeconfig      bool
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
func (b *JobBuilder) WithZarfRetryPolicy(policy *zarfv1alpha3.RetryPolicy) *JobBuilder {
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
func (b *JobBuilder) WithUDSRetryPolicy(policy *udsv1alpha3.RetryPolicy) *JobBuilder {
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
// Volumes have size limits for Kyverno/PSS compliance: 10Gi for workspace, 10Gi for output.
// Custom sizes can be provided via volumeSizes; nil or unset fields use the defaults.
func (b *JobBuilder) WithWorkspaceVolume(volumeSizes *common.VolumeSizes) *JobBuilder {
	b.volumeSizes = volumeSizes

	workspaceSizeLimit := resource.MustParse("10Gi")
	outputSizeLimit := resource.MustParse("10Gi")
	if volumeSizes != nil {
		if volumeSizes.Workspace != nil {
			workspaceSizeLimit = *volumeSizes.Workspace
		}
		if volumeSizes.Output != nil {
			outputSizeLimit = *volumeSizes.Output
		}
	}

	b.volumes = append(b.volumes,
		corev1.Volume{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &workspaceSizeLimit,
				},
			},
		},
		corev1.Volume{
			Name: "output",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &outputSizeLimit,
				},
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
// Note: This only adds the volume, not a mount. Used for init containers that define their own mounts.
// For main container registry credentials, use WithRegistryCredentials instead.
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

// WithRegistryCredentials adds a registry credentials volume and mounts it to the main container.
// This is used for pulling images during build/create operations.
// The mountPath should be the docker config directory for the container user (e.g., /home/zarf/.docker).
func (b *JobBuilder) WithRegistryCredentials(secretName, mountPath string) *JobBuilder {
	if secretName == "" {
		return b
	}
	b.volumes = append(b.volumes, corev1.Volume{
		Name: constants.VolumeNameRegistryCredentials,
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
	b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
		Name:      constants.VolumeNameRegistryCredentials,
		MountPath: mountPath,
		ReadOnly:  true,
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

// WithExtraMounts adds user-specified ConfigMap/Secret volumes and mounts.
func (b *JobBuilder) WithExtraMounts(mounts []common.ExtraMount) *JobBuilder {
	for i, mount := range mounts {
		volName := fmt.Sprintf("extra-mount-%d", i)

		var volSource corev1.VolumeSource
		if mount.ConfigMapRef != nil {
			volSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mount.ConfigMapRef.Name,
					},
				},
			}
		} else if mount.SecretRef != nil { // pragma: allowlist secret
			volSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mount.SecretRef.Name, // pragma: allowlist secret
				},
			}
		}

		b.volumes = append(b.volumes, corev1.Volume{
			Name:         volName,
			VolumeSource: volSource,
		})

		readOnly := true
		if mount.ReadOnly != nil {
			readOnly = *mount.ReadOnly
		}

		vm := corev1.VolumeMount{
			Name:      volName,
			MountPath: mount.MountPath,
			ReadOnly:  readOnly,
		}
		if mount.SubPath != "" {
			vm.SubPath = mount.SubPath
		}

		b.volumeMounts = append(b.volumeMounts, vm)
	}
	return b
}

// WithEnvVar adds an environment variable with a simple name/value.
func (b *JobBuilder) WithEnvVar(name, value string) *JobBuilder {
	b.envVars = append(b.envVars, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return b
}

// WithHomeDir sets the HOME environment variable for the container.
//
// Deprecated: Use WithUserConfig instead, which also sets security contexts
// and adds a writable home directory volume.
func (b *JobBuilder) WithHomeDir(home string) *JobBuilder {
	b.homeDir = home
	return b.WithEnvVar("HOME", home)
}

// WithUserConfig configures the container to run as a specific user.
// This sets the appropriate home directory, security contexts, and environment.
// For Zarf containers, use constants.DefaultZarfUID (1000).
// For UDS containers, use constants.DefaultUDSUID (65532).
// This also adds a writable emptyDir volume for the home directory.
func (b *JobBuilder) WithUserConfig(uid int64) *JobBuilder {
	// Determine home directory based on UID
	var homePath string
	switch uid {
	case constants.DefaultZarfUID:
		homePath = constants.HomePathZarf
	case constants.DefaultUDSUID:
		homePath = constants.HomePathUDS
	default:
		homePath = fmt.Sprintf("/home/%d", uid)
	}

	b.homeDir = homePath
	b.podSecurityContext = NonRootPodSecurityContextWithUID(uid)
	b.containerSecurityContext = NonRootSecurityContextWithUID(uid)

	return b.WithEnvVar("HOME", homePath)
}

// WithCustomEnvVar adds a full environment variable (supports ValueFrom for secrets).
func (b *JobBuilder) WithCustomEnvVar(env corev1.EnvVar) *JobBuilder {
	b.envVars = append(b.envVars, env)
	return b
}

// WithKubeconfigVolume adds a kubeconfig secret volume and mount for external cluster deployment.
// If secretName is empty, nothing is added. If key is empty, defaults to "kubeconfig".
// Also sets the KUBECONFIG environment variable so the deploy command doesn't need to.
func (b *JobBuilder) WithKubeconfigVolume(secretName, key string) *JobBuilder {
	if secretName == "" {
		return b
	}

	if key == "" {
		key = "kubeconfig"
	}

	// Add volume
	b.volumes = append(b.volumes, corev1.Volume{
		Name: constants.VolumeNameKubeconfig,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{
						Key:  key,
						Path: "kubeconfig",
					},
				},
			},
		},
	})

	// Add volume mount
	b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
		Name:      constants.VolumeNameKubeconfig,
		MountPath: constants.VolumeMountPathKubeconfig,
		ReadOnly:  true,
	})

	// Set KUBECONFIG env var so the deploy command doesn't need to export it
	b.envVars = append(b.envVars, corev1.EnvVar{
		Name:  "KUBECONFIG",
		Value: constants.VolumeMountPathKubeconfig + "/kubeconfig",
	})

	return b
}

// WithInClusterKubeconfig configures in-cluster kubeconfig generation via an init container.
// This adds:
//   - A projected service account volume for the SA token
//   - An emptyDir volume for the generated kubeconfig
//   - An init container that generates the kubeconfig from the SA token
//   - The KUBECONFIG environment variable pointing to the generated file
//
// This approach works correctly with debug mode since the init container runs
// before the main container, making the kubeconfig available when a user execs
// into a debug pod.
func (b *JobBuilder) WithInClusterKubeconfig() *JobBuilder {
	b.inClusterKubeconfig = true
	return b
}

// WithPodSecurityContext sets the pod-level security context.
func (b *JobBuilder) WithPodSecurityContext(ctx *corev1.PodSecurityContext) *JobBuilder {
	b.podSecurityContext = ctx
	return b
}

// WithContainerSecurityContext sets the container-level security context.
func (b *JobBuilder) WithContainerSecurityContext(ctx *corev1.SecurityContext) *JobBuilder {
	b.containerSecurityContext = ctx
	return b
}

// WithServiceAccountName sets the service account name for the pod.
func (b *JobBuilder) WithServiceAccountName(name string) *JobBuilder {
	b.serviceAccountName = name
	return b
}

// WithDebugMode enables debug mode for the job.
// When enabled, the job waits for a completion marker file (/tmp/debug-complete)
// instead of running the actual command, allowing users to exec into the pod for debugging.
// Touch /tmp/debug-complete inside the pod to signal completion and continue to the next action.
func (b *JobBuilder) WithDebugMode(enabled bool) *JobBuilder {
	b.debugMode = enabled
	return b
}

// ShouldDebugAction determines if a specific action should run in debug mode.
// It checks both the global debugMode flag and the debugActions list.
// If debugActions is empty and debugMode is true, all actions are debugged.
// If debugActions is non-empty, only listed actions are debugged (regardless of debugMode).
func ShouldDebugAction(debugMode bool, debugActions []string, currentAction string) bool {
	// If debugActions is specified, it takes precedence
	if len(debugActions) > 0 {
		return slices.Contains(debugActions, currentAction)
	}
	// Fall back to global debugMode
	return debugMode
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
		ReadOnlyRootFilesystem:   Ptr(true),
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
	// Use custom container security context if provided, otherwise use default
	containerSecCtx := b.containerSecurityContext
	if containerSecCtx == nil {
		containerSecCtx = NonRootSecurityContext()
	}

	// Add /tmp emptyDir volume for containers with readOnlyRootFilesystem
	// This allows tools to write temporary files (required for Kyverno/PSS compliance)
	tmpSizeLimit := resource.MustParse("1Gi")
	if b.volumeSizes != nil && b.volumeSizes.Tmp != nil {
		tmpSizeLimit = *b.volumeSizes.Tmp
	}
	b.volumes = append(b.volumes, corev1.Volume{
		Name: "tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: &tmpSizeLimit,
			},
		},
	})
	b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	})

	// Add in-cluster kubeconfig init container if configured
	if b.inClusterKubeconfig {
		// Add projected service account volume for the SA token
		expirationSeconds := int64(3600) // 1 hour token expiration
		b.volumes = append(b.volumes, corev1.Volume{
			Name: "kube-api-access",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Path:              "token",
								ExpirationSeconds: &expirationSeconds,
							},
						},
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kube-root-ca.crt",
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "ca.crt",
										Path: "ca.crt",
									},
								},
							},
						},
						{
							DownwardAPI: &corev1.DownwardAPIProjection{
								Items: []corev1.DownwardAPIVolumeFile{
									{
										Path: "namespace",
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
						},
					},
				},
			},
		})

		// Add kubeconfig emptyDir volume (shared between init container and main container)
		b.volumes = append(b.volumes, corev1.Volume{
			Name: constants.VolumeNameKubeconfig,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		// Mount kubeconfig volume in main container (read-only)
		b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
			Name:      constants.VolumeNameKubeconfig,
			MountPath: constants.VolumeMountPathKubeconfig,
			ReadOnly:  true,
		})

		// Set KUBECONFIG env var for main container
		b.envVars = append(b.envVars, corev1.EnvVar{
			Name:  "KUBECONFIG",
			Value: constants.VolumeMountPathKubeconfig + "/kubeconfig",
		})

		// Create init container that generates the kubeconfig
		kubeconfigInit := corev1.Container{
			Name:    "kubeconfig-init",
			Image:   b.containerImage,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{constants.KubeconfigInitScript},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "kube-api-access",
					MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					ReadOnly:  true,
				},
				{
					Name:      constants.VolumeNameKubeconfig,
					MountPath: "/kubeconfig",
				},
			},
			SecurityContext: containerSecCtx,
		}

		// Prepend kubeconfig init container before any source retrieval init containers
		b.initContainers = append([]corev1.Container{kubeconfigInit}, b.initContainers...)
	}

	// Add home directory emptyDir volume if configured via WithUserConfig or WithHomeDir
	// This allows tools to write config files to their home directory (e.g., .cache, .config)
	if b.homeDir != "" {
		homeSizeLimit := resource.MustParse("1Gi")
		if b.volumeSizes != nil && b.volumeSizes.Home != nil {
			homeSizeLimit = *b.volumeSizes.Home
		}
		b.volumes = append(b.volumes, corev1.Volume{
			Name: "home",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &homeSizeLimit,
				},
			},
		})
		b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
			Name:      "home",
			MountPath: b.homeDir,
		})
	}

	// Determine container args - use debug command if debug mode is enabled
	containerArgs := b.args
	if b.debugMode {
		// Debug script that:
		// 1. Shows instructions for the operator
		// 2. Waits for /tmp/debug-complete marker file
		// 3. Exits 0 when marker is created, allowing job to complete and chaining to continue
		debugScript := `echo "=========================================="
echo "DEBUG MODE ENABLED"
echo "=========================================="
echo ""
echo "Pod is ready for debugging. The original command was:"
echo "  %s"
echo ""
echo "To inspect the environment, exec into this pod:"
echo "  kubectl exec -it $HOSTNAME -n %s -- /bin/sh"
echo ""
echo "When done debugging, signal completion to continue:"
echo "  touch /tmp/debug-complete"
echo ""
echo "Waiting for /tmp/debug-complete..."
while [ ! -f /tmp/debug-complete ]; do sleep 2; done
echo "Debug marker found, exiting successfully."
exit 0`
		// Format the script with the original command for reference
		originalCmd := ""
		if len(b.args) > 0 {
			originalCmd = b.args[0]
		}
		containerArgs = []string{fmt.Sprintf(debugScript, originalCmd, b.job.Namespace)}
		// Set extended TTL for debug pods (1 hour) to allow time for inspection
		debugTTL := int32(3600)
		b.job.Spec.TTLSecondsAfterFinished = &debugTTL
		klog.InfoS("Debug mode enabled, job will wait for completion marker",
			"job", b.job.Name, "originalArgs", b.args, "ttlSecondsAfterFinished", debugTTL,
			"completionMarker", "/tmp/debug-complete")
	}

	// Build the main container
	container := corev1.Container{
		Name:            b.containerName,
		Image:           b.containerImage,
		Command:         b.command,
		Args:            containerArgs,
		WorkingDir:      b.workingDir,
		VolumeMounts:    b.volumeMounts,
		Env:             b.envVars,
		SecurityContext: containerSecCtx,
		Resources:       b.resources,
	}

	// Use custom pod security context if provided, otherwise use default
	podSecCtx := b.podSecurityContext
	if podSecCtx == nil {
		podSecCtx = NonRootPodSecurityContext()
	}

	// Determine automountServiceAccountToken based on whether a service account is specified
	// Only mount the token if a service account is explicitly provided (needed for RBAC)
	// Otherwise, disable it for security (Kyverno/PSS compliance)
	automountToken := b.serviceAccountName != ""

	// Build the pod template
	b.job.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: b.job.Labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			ServiceAccountName:           b.serviceAccountName,
			AutomountServiceAccountToken: &automountToken,
			InitContainers:               b.initContainers,
			Containers:                   []corev1.Container{container},
			Volumes:                      b.volumes,
			SecurityContext:              podSecCtx,
			NodeSelector:                 b.nodeSelector,
			Affinity:                     b.affinity,
			Tolerations:                  b.tolerations,
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
// The kubeconfigKey parameter specifies which key in the secret contains the kubeconfig data.
// If kubeconfigKey is empty, defaults to "kubeconfig".
// Used by deploy handlers to mount external cluster kubeconfig into deploy jobs.
func AddKubeconfigVolume(job *batchv1.Job, kubeconfigSecretName, kubeconfigKey string) {
	if kubeconfigSecretName == "" {
		return
	}

	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add kubeconfig volume", "job", job.Name)
		return
	}

	// Default key if not specified
	if kubeconfigKey == "" {
		kubeconfigKey = "kubeconfig"
	}

	// Add volume - mounts specific key from secret
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: kubeconfigSecretName,
				Items: []corev1.KeyToPath{
					{
						Key:  kubeconfigKey,
						Path: "kubeconfig",
					},
				},
			},
		},
	})

	// Add volume mount to standardized path
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: constants.VolumeMountPathKubeconfig,
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
