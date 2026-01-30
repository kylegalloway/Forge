package zarf

import (
	"context"
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	"github.com/kylegalloway/forge/pkg/apis/common"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/destinations"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// PublishHandler handles Publish actions for ZarfPackageJob resources
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

// Execute performs a Publish action for the given ZarfPackageJob
func (handler *PublishHandler) Execute(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string, artifactPVCName string) (*actions.ActionResult, error) {
	klog.InfoS("Executing Zarf Package Publish action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	// Record publish started
	handler.metrics.RecordPackagePublishStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate publish destination is provided
	if pkg.Spec.Publish == nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("publish configuration is required for Publish action")
	}

	// Create Kubernetes Job to publish the package
	job, err := handler.createPublishJob(ctx, pkg, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "publish")

	klog.InfoS("Zarf package publish job created", "name", pkg.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     constants.PhaseRunning,
		Message:   fmt.Sprintf("Publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a Zarf package
func (handler *PublishHandler) createPublishJob(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-publish", pkg.Name)

	// If multi-action job, update artifactPath to use glob pattern for PVC location
	if artifactPVCName != "" {
		artifactPath = constants.VolumeMountPathArtifacts + "/*.tar.zst"
	}

	// Create destination handler
	destHandler, err := destinations.New(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination handler: %w", err)
	}

	// Get publish command
	publishCmd, err := destHandler.GetPublishCommand(pkg, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get publish command: %w", err)
	}

	// Build init containers for artifact retrieval
	initContainers, err := handler.buildInitContainers(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Determine timeout and retry policy
	timeoutStr := ""
	var retryPolicy *zarfv1alpha3.RetryPolicy
	if pkg.Spec.Publish != nil {
		timeoutStr = pkg.Spec.Publish.Timeout
		retryPolicy = pkg.Spec.Publish.Retry
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultPublishTimeout)

	// Get destination job configuration (volumes, env vars, etc.)
	jobConfig, err := destHandler.GetJobConfiguration(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, pkg.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(pkg, zarfv1alpha3.SchemeGroupVersion.WithKind("ZarfPackageJob")).
		WithLabels(map[string]string{
			constants.LabelApp:     constants.LabelAppValueZarf,
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  constants.ActionPublish,
		}).
		WithContainerImage(constants.ZarfCLIImage).
		WithContainerName(constants.ContainerNameZarfPublish).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{publishCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithUserConfig(constants.DefaultZarfUID).
		WithResources(handler.getResources(pkg)).
		WithNodeSelector(pkg.Spec.NodeSelector).
		WithAffinity(pkg.Spec.Affinity).
		WithTolerations(pkg.Spec.Tolerations).
		WithZarfRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume(pkg.Spec.VolumeSizes).
		WithArtifactPVC(artifactPVCName).
		WithServiceAccountName(pkg.Spec.ServiceAccountName).
		WithDebugMode(actions.ShouldDebugAction(pkg.GetDebugMode() || constants.DebugMode, pkg.GetDebugActions(), constants.ActionPublish))

	// Add source credential volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha3.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialRef != nil { // pragma: allowlist secret
		builder.WithDockerConfigSecret(pkg.Spec.Source.OCI.CredentialRef.Name) // pragma: allowlist secret
	}

	// Add S3 credentials volume if S3 source with file credentials
	if pkg.Spec.Source.Type == zarfv1alpha3.SourceTypeS3 && pkg.Spec.Source.S3 != nil {
		if vol := sources.GetS3CredentialVolume(pkg.Spec.Source.S3.CredentialRef); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Add git credentials volume if Git source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha3.SourceTypeGit && pkg.Spec.Source.Git != nil {
		if vol := sources.GetGitCredentialVolume(pkg.Spec.Source.Git.CredentialRef, pkg.Spec.Source.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Apply destination-specific configuration (volumes, env vars, etc.)
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

	// Add extra mounts (merged: spec-level + publish-level)
	var publishExtraMounts []common.ExtraMount
	if pkg.Spec.Publish != nil {
		publishExtraMounts = pkg.Spec.Publish.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(pkg.Spec.ExtraMounts, publishExtraMounts)
	if err != nil {
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}
	builder.WithExtraMounts(extraMounts)

	// Create or get the job
	job, err := builder.CreateOrGet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildInitContainers creates init containers for artifact retrieval
func (handler *PublishHandler) buildInitContainers(pkg *zarfv1alpha3.ZarfPackageJob) ([]corev1.Container, error) {
	sourceHandler, err := sources.New(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to create source handler: %w", err)
	}

	container, err := sourceHandler.GetInitContainer(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// getResources returns resource requirements for the job pod
// Uses spec.resources if provided, otherwise falls back to sensible defaults
func (handler *PublishHandler) getResources(pkg *zarfv1alpha3.ZarfPackageJob) corev1.ResourceRequirements {
	// If custom resources specified, use them
	if pkg.Spec.Resources != nil {
		return *pkg.Spec.Resources
	}

	// Default resources for publish jobs
	// Standardized with UDS Publish (both upload artifacts to registries/storage)
	return actions.PublishResourceRequirements()
}
