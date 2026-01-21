// Package zarf provides handlers for ZarfPackageJob actions (Build, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a Zarf package or UDS bundle.
package zarf

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// BuildHandler handles Build actions for ZarfPackageJob resources
type BuildHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewBuildHandler creates a new BuildHandler
func NewBuildHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *BuildHandler {
	return &BuildHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Build action for the given ZarfPackageJob
//
// The artifactPVCName parameter enables multi-action job support (BuildPublish, BuildDeploy, etc.)
// by providing a shared PersistentVolumeClaim for artifacts. When set, build outputs are stored
// in the PVC so subsequent actions (Publish/Deploy) can access them without re-building.
//
// This differs from UDS handlers which don't accept artifactPVCName because UDS multi-action
// jobs don't currently implement artifact sharing - each action runs independently.
//
// For the rationale behind this signature divergence, see:
// docs/development/ARCHITECTURE.md#handler-signature-divergence
func (handler *BuildHandler) Execute(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, artifactPVCName string) (*actions.ActionResult, error) {

	klog.InfoS("Executing Zarf Package Build action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	// Record build started
	handler.metrics.RecordPackageBuildStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate source is provided
	if pkg.Spec.Source.Type == "" {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("source type is required for Build action")
	}

	// Create Kubernetes Job to build the package
	job, err := handler.createBuildJob(ctx, pkg, artifactPVCName)
	if err != nil {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create build job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "build")

	klog.InfoS("Zarf package build job created", "name", pkg.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     constants.PhaseRunning,
		Message:   fmt.Sprintf("Build job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBuildJob creates a Kubernetes Job to build a Zarf package
func (handler *BuildHandler) createBuildJob(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-build", pkg.Name)

	// Build zarf command based on source type and artifact PVC
	zarfCmd, workingDir := handler.buildZarfCommand(pkg, artifactPVCName)

	// Build init containers
	initContainers, err := handler.buildInitContainers(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Determine timeout and retry policy
	timeoutStr := ""
	var retryPolicy *zarfv1alpha3.RetryPolicy
	if pkg.Spec.Build != nil {
		timeoutStr = pkg.Spec.Build.Timeout
		retryPolicy = pkg.Spec.Build.Retry
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultBuildTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, pkg.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(pkg, zarfv1alpha3.SchemeGroupVersion.WithKind("ZarfPackageJob")).
		WithLabels(map[string]string{
			constants.LabelApp:     constants.LabelAppValueZarf,
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  constants.ActionBuild,
		}).
		WithContainerImage(constants.ZarfCLIImage).
		WithContainerName(constants.ContainerNameZarfBuild).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{zarfCmd}).
		WithWorkingDir(workingDir).
		WithHomeDir(constants.HomePathZarf).
		WithResources(handler.getResources(pkg)).
		WithNodeSelector(pkg.Spec.NodeSelector).
		WithAffinity(pkg.Spec.Affinity).
		WithTolerations(pkg.Spec.Tolerations).
		WithZarfRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Add docker-config volume if OCI source with credentials
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

	// Add registry credentials for pulling images during build
	if pkg.Spec.Build != nil && pkg.Spec.Build.RegistryCredentialRef != nil { // pragma: allowlist secret
		builder.WithRegistryCredentials(pkg.Spec.Build.RegistryCredentialRef.Name, constants.VolumeMountPathDockerConfig) // pragma: allowlist secret
	}

	// Create or get the job
	job, err := builder.CreateOrGet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildZarfCommand builds the zarf CLI command based on package source
func (handler *BuildHandler) buildZarfCommand(_ *zarfv1alpha3.ZarfPackageJob, artifactPVCName string) (string, string) {
	workingDir := constants.VolumeMountPathWorkspace

	// Build command - output to /artifacts if PVC exists, otherwise /output
	var cmd string
	if artifactPVCName != "" {
		// Multi-action job: output to shared PVC directory
		// Zarf will generate filename based on package metadata
		cmd = "zarf package create . --confirm --output-directory " + constants.VolumeMountPathArtifacts
	} else {
		// Standalone build: output to EmptyDir
		cmd = "zarf package create . --confirm --output-directory " + constants.VolumeMountPathOutput
	}

	return cmd, workingDir
}

// buildInitContainers creates init containers for source artifact retrieval
func (handler *BuildHandler) buildInitContainers(pkg *zarfv1alpha3.ZarfPackageJob) ([]corev1.Container, error) {
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
func (handler *BuildHandler) getResources(pkg *zarfv1alpha3.ZarfPackageJob) corev1.ResourceRequirements {
	// If custom resources specified, use them
	if pkg.Spec.Resources != nil {
		return *pkg.Spec.Resources
	}

	// Default resources for build jobs
	// Standardized with UDS Create (both create artifacts)
	return actions.BuildResourceRequirements()
}
