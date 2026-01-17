package uds

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// PublishHandler handles Publish actions for UDSBundleJob resources
type PublishHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewPublishHandler creates a new PublishHandler
func NewPublishHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *PublishHandler {
	return &PublishHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Publish action for the given UDSBundleJob
//
// The artifactPath and artifactPVCName parameters enable multi-action job support (CreatePublish, etc.)
// When artifactPVCName is set, the PVC is mounted and artifactPath specifies where to find the bundle.
// When empty, assumes standalone publish with bundle source in workspace or from spec.
//
//nolint:staticcheck // SA1019: UDSBundleJob v1alpha1 must be supported until v0.10.0
func (handler *PublishHandler) Execute(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPath, artifactPVCName string) (*actions.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Publish action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPath", artifactPath, "artifactPVC", artifactPVCName)

	// Record publish started
	handler.metrics.RecordBundlePublishStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate publish configuration
	if bundle.Spec.Publish == nil || bundle.Spec.Publish.Destination.Type == "" {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("publish destination is required for Publish action")
	}

	// Create Kubernetes Job to publish the bundle
	job, err := handler.createPublishJob(ctx, bundle, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "publish")

	klog.InfoS("Bundle publish job created", "name", bundle.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a UDS bundle
func (handler *PublishHandler) createPublishJob(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPath, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-publish", bundle.Name)

	// Build UDS CLI publish command
	udsCmd, err := handler.buildPublishCommand(bundle, artifactPath)
	if err != nil {
		return nil, err
	}

	// Build env vars
	envVars := handler.buildEnvVars(bundle)

	// Get retry policy from publish config
	var retryPolicy *udsv1alpha2.RetryPolicy
	if bundle.Spec.Publish != nil {
		retryPolicy = bundle.Spec.Publish.Retry
	}

	// Use default timeout for publish operations
	activeDeadlineSeconds := int64(constants.DefaultPublishTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, bundle.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(bundle, udsv1alpha2.SchemeGroupVersion.WithKind("UDSBundleJob")).
		WithLabels(map[string]string{
			"app":                  "forge",
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  "publish",
		}).
		WithContainerImage(constants.UDSCLIImage).
		WithContainerName(constants.ContainerNameUDSPublish).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{udsCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithResources(handler.getResources(bundle)).
		WithNodeSelector(bundle.Spec.NodeSelector).
		WithAffinity(bundle.Spec.Affinity).
		WithTolerations(bundle.Spec.Tolerations).
		WithUDSRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Build the job spec
	job := builder.Build()

	// Set ServiceAccount and SecurityContexts
	job.Spec.Template.Spec.ServiceAccountName = bundle.Spec.ServiceAccountName
	job.Spec.Template.Spec.SecurityContext = actions.NonRootPodSecurityContextWithUID(constants.DefaultUDSUID)
	if len(job.Spec.Template.Spec.Containers) > 0 {
		job.Spec.Template.Spec.Containers[0].SecurityContext = actions.NonRootSecurityContextWithUID(constants.DefaultUDSUID)
		// Set env vars (including those with ValueFrom for secrets)
		job.Spec.Template.Spec.Containers[0].Env = envVars
	}

	// Add credential volumes if needed
	handler.addCredentialVolumes(bundle, job)

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(bundle.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Job already exists, reusing", "name", bundle.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(bundle.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildPublishCommand builds the UDS CLI publish command
func (handler *PublishHandler) buildPublishCommand(bundle *udsv1alpha2.UDSBundleJob, artifactPath string) (string, error) {
	dest := bundle.Spec.Publish.Destination

	// Determine bundle path - use artifactPath if provided (multi-action workflow),
	// otherwise search workspace for bundle (standalone publish)
	var bundlePath string
	if artifactPath != "" {
		bundlePath = artifactPath
	} else {
		bundlePath = constants.VolumeMountPathWorkspace + "/uds-bundle-*.tar.zst"
	}

	switch dest.Type {
	case udsv1alpha2.DestinationTypeOCI:
		if dest.OCI == nil {
			return "", fmt.Errorf("OCI destination configuration is required")
		}
		// UDS publish command for OCI registry
		ociRef := fmt.Sprintf("%s/%s:%s", dest.OCI.Registry, dest.OCI.Repository, dest.OCI.Tag)
		return fmt.Sprintf("uds publish %s %s", bundlePath, ociRef), nil

	case udsv1alpha2.DestinationTypeS3:
		if dest.S3 == nil {
			return "", fmt.Errorf("S3 destination configuration is required")
		}
		// For S3, we'll use AWS CLI to upload
		s3Path := fmt.Sprintf("s3://%s/%s", dest.S3.Bucket, dest.S3.Key)
		return fmt.Sprintf("aws s3 cp %s %s", bundlePath, s3Path), nil

	case udsv1alpha2.DestinationTypeLocal:
		// Local destination - just echo success
		return fmt.Sprintf("echo 'Bundle artifact stored locally at %s'", bundlePath), nil

	default:
		return "", fmt.Errorf("unsupported destination type: %s", dest.Type)
	}
}

// buildEnvVars builds environment variables for the publish job
func (handler *PublishHandler) buildEnvVars(bundle *udsv1alpha2.UDSBundleJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	dest := bundle.Spec.Publish.Destination

	// S3 configuration
	if dest.Type == udsv1alpha2.DestinationTypeS3 && dest.S3 != nil {
		if dest.S3.Region != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_REGION",
				Value: dest.S3.Region,
			})
		}
		if dest.S3.Endpoint != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_ENDPOINT_URL",
				Value: dest.S3.Endpoint,
			})
		}

		// Add AWS credentials from secret if provided
		// Uses same key names as S3 sources: 'access-key-id' and 'secret-access-key'
		if dest.S3.CredentialsSecretRef != nil { // pragma: allowlist secret
			envVars = append(envVars,
				corev1.EnvVar{
					Name: "AWS_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
							LocalObjectReference: corev1.LocalObjectReference{
								Name: dest.S3.CredentialsSecretRef.Name, // pragma: allowlist secret
							},
							Key: "access-key-id",
						},
					},
				},
				corev1.EnvVar{
					Name: "AWS_SECRET_ACCESS_KEY", // pragma: allowlist secret
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
							LocalObjectReference: corev1.LocalObjectReference{
								Name: dest.S3.CredentialsSecretRef.Name, // pragma: allowlist secret
							},
							Key: "secret-access-key", // pragma: allowlist secret
						},
					},
				},
			)
		}
	}

	return envVars
}

// addCredentialVolumes adds credential volumes for OCI registries
func (handler *PublishHandler) addCredentialVolumes(bundle *udsv1alpha2.UDSBundleJob, job *batchv1.Job) {
	dest := bundle.Spec.Publish.Destination

	// Add docker-config volume for OCI registries
	if dest.Type == udsv1alpha2.DestinationTypeOCI && dest.OCI != nil && dest.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		// Ensure the job has at least one container before accessing Containers[0]
		if len(job.Spec.Template.Spec.Containers) == 0 {
			klog.ErrorS(nil, "Job has no containers, cannot add credential volumes", "job", job.Name)
			return
		}

		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
					SecretName: dest.OCI.CredentialsSecretRef.Name, // pragma: allowlist secret
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})

		// Add volume mount to container
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "docker-config",
				MountPath: "/.docker",
				ReadOnly:  true,
			},
		)

		// Set DOCKER_CONFIG env var
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: "/.docker",
			},
		)
	}
}

// getResources returns resource requirements for the publish job
func (handler *PublishHandler) getResources(bundle *udsv1alpha2.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for publish jobs
	// Standardized with Zarf Publish (both upload artifacts to registries/storage)
	return actions.PublishResourceRequirements()
}
