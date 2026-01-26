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
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/destinations"
	"github.com/kylegalloway/forge/pkg/sources"
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
func (handler *PublishHandler) Execute(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, artifactPath, artifactPVCName string) (*actions.ActionResult, error) {

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
		Phase:     constants.PhaseRunning,
		Message:   fmt.Sprintf("Bundle publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a UDS bundle
func (handler *PublishHandler) createPublishJob(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, artifactPath, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-publish", bundle.Name)

	// Build UDS CLI publish command
	udsCmd, err := handler.buildPublishCommand(bundle, artifactPath)
	if err != nil {
		return nil, err
	}

	// Get job configuration (volumes, env vars) from shared destination adapters
	jobConfig, err := destinations.GetUDSJobConfiguration(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	// Build init containers for artifact retrieval (for standalone publish)
	initContainers, err := handler.buildInitContainers(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Get timeout and retry policy from publish config
	timeoutStr := ""
	var retryPolicy *udsv1alpha3.RetryPolicy
	if bundle.Spec.Publish != nil {
		timeoutStr = bundle.Spec.Publish.Timeout
		retryPolicy = bundle.Spec.Publish.Retry
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultPublishTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, bundle.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(bundle, udsv1alpha3.SchemeGroupVersion.WithKind("UDSBundleJob")).
		WithLabels(map[string]string{
			constants.LabelApp:     constants.LabelAppValueUDS,
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  constants.ActionPublish,
		}).
		WithContainerImage(constants.UDSCLIImage).
		WithContainerName(constants.ContainerNameUDSPublish).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{udsCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithUserConfig(constants.DefaultUDSUID).
		WithResources(handler.getResources(bundle)).
		WithNodeSelector(bundle.Spec.NodeSelector).
		WithAffinity(bundle.Spec.Affinity).
		WithTolerations(bundle.Spec.Tolerations).
		WithUDSRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName).
		WithServiceAccountName(bundle.Spec.ServiceAccountName).
		WithDebugMode(constants.DebugMode)

	// Apply destination-specific configuration (volumes, env vars) from shared adapters
	if jobConfig != nil {
		for _, vol := range jobConfig.Volumes {
			builder.WithCustomVolume(vol)
		}
		for _, mount := range jobConfig.VolumeMounts {
			builder.WithCustomVolumeMount(mount)
		}
		for _, env := range jobConfig.Env {
			builder.WithCustomEnvVar(env)
		}
	}

	// Add docker-config volume if OCI source with credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeOCI && bundle.Spec.Source.OCI != nil && bundle.Spec.Source.OCI.CredentialRef != nil { // pragma: allowlist secret
		builder.WithDockerConfigSecret(bundle.Spec.Source.OCI.CredentialRef.Name) // pragma: allowlist secret
	}

	// Add S3 credentials volume if S3 source with file credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeS3 && bundle.Spec.Source.S3 != nil {
		if vol := sources.GetS3CredentialVolume(bundle.Spec.Source.S3.CredentialRef); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Add git credentials volume if Git source with credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeGit && bundle.Spec.Source.Git != nil {
		if vol := sources.GetGitCredentialVolume(bundle.Spec.Source.Git.CredentialRef, bundle.Spec.Source.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Create or get the job
	job, err := builder.CreateOrGet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildPublishCommand builds the UDS CLI publish command using shared destination adapters
func (handler *PublishHandler) buildPublishCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPath string) (string, error) {
	// Determine bundle path - use artifactPath if provided (multi-action workflow),
	// otherwise search workspace for bundle (standalone publish)
	bundlePath := artifactPath
	if bundlePath == "" {
		bundlePath = constants.VolumeMountPathWorkspace + "/uds-bundle-*.tar.zst"
	}

	// Use shared destination adapter to generate the publish command
	return destinations.GetUDSPublishCommand(bundle, bundlePath)
}

// buildInitContainers creates init containers for artifact retrieval (for standalone publish)
func (handler *PublishHandler) buildInitContainers(bundle *udsv1alpha3.UDSBundleJob) ([]corev1.Container, error) {
	container, err := sources.GetUDSInitContainer(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// getResources returns resource requirements for the publish job
func (handler *PublishHandler) getResources(bundle *udsv1alpha3.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for publish jobs
	// Standardized with Zarf Publish (both upload artifacts to registries/storage)
	return actions.PublishResourceRequirements()
}
