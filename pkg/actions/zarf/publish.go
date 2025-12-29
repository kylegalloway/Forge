package zarf

import (
	"context"
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
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
func (handler *PublishHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string, artifactPVCName string) (*actions.ActionResult, error) {
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
		Phase:     "Running",
		Message:   fmt.Sprintf("Publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a Zarf package
func (handler *PublishHandler) createPublishJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string, artifactPVCName string) (*batchv1.Job, error) {
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

	// Determine timeout
	timeoutStr := ""
	if pkg.Spec.Publish != nil {
		timeoutStr = pkg.Spec.Publish.Timeout
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultPublishTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, pkg.Namespace).
		WithOwnerReference(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")).
		WithLabels(map[string]string{
			"app":                  "forge",
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  "publish",
		}).
		WithContainerImage(constants.ZarfCLIImage).
		WithContainerName(constants.ContainerNameZarfPublish).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{publishCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithResources(handler.getResources(pkg)).
		WithBackoffLimit(0).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Add source credential volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		builder.WithDockerConfigSecret(pkg.Spec.Source.OCI.CredentialsSecretRef.Name) // pragma: allowlist secret
	}

	// Build the job spec so we can apply destination-specific configuration
	job := builder.Build()

	// Apply destination-specific configuration (volumes, env vars, etc.)
	jobConfig, err := destHandler.GetJobConfiguration(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	if jobConfig != nil {
		if len(job.Spec.Template.Spec.Containers) == 0 {
			return nil, fmt.Errorf("job has no containers, cannot apply job configuration")
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, jobConfig.Volumes...)
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, jobConfig.VolumeMounts...)
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, jobConfig.Env...)
		job.Spec.Template.Spec.Containers[0].EnvFrom = append(job.Spec.Template.Spec.Containers[0].EnvFrom, jobConfig.EnvFrom...)
	}

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(pkg.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Job already exists, reusing", "name", pkg.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(pkg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildInitContainers creates init containers for artifact retrieval
func (handler *PublishHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackageJob) ([]corev1.Container, error) {
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
func (handler *PublishHandler) getResources(pkg *zarfv1alpha1.ZarfPackageJob) corev1.ResourceRequirements {
	// If custom resources specified, use them
	if pkg.Spec.Resources != nil {
		return *pkg.Spec.Resources
	}

	// Default resources for publish jobs
	// Standardized with UDS Publish (both upload artifacts to registries/storage)
	return actions.PublishResourceRequirements()
}
