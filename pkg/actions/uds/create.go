// Package uds provides handlers for UDSBundleJob actions (Create, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a UDS bundle.
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
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// CreateHandler handles Create actions for UDSBundleJob resources
type CreateHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewCreateHandler creates a new CreateHandler
func NewCreateHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *CreateHandler {
	return &CreateHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Create action for the given UDSBundleJob
//
// The artifactPVCName parameter enables multi-action job support (CreatePublish, CreateDeploy, etc.)
// by providing a shared PersistentVolumeClaim for artifacts. When set, create outputs are stored
// in the PVC so subsequent actions (Publish/Deploy) can access them without re-creating.
//
// This matches the Zarf Build handler pattern and enables efficient action chaining.
func (handler *CreateHandler) Execute(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPVCName string) (*actions.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Create action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPVC", artifactPVCName)

	// Record create started
	handler.metrics.RecordBundleCreateStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate source is provided
	if bundle.Spec.Source.Type == "" {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("source type is required for Create action")
	}

	// Create Kubernetes Job to create the bundle
	job, err := handler.createBundleJob(ctx, bundle, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create bundle job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "create")

	klog.InfoS("Bundle create job created", "name", bundle.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle create job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBundleJob creates a Kubernetes Job to create a UDS bundle
func (handler *CreateHandler) createBundleJob(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-create", bundle.Name)

	// Build UDS CLI command
	udsCmd, workingDir := handler.buildUDSCommand(bundle, artifactPVCName)

	// Build init containers for source retrieval
	initContainers, err := handler.buildInitContainers(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Use default timeout for create operations
	activeDeadlineSeconds := int64(constants.DefaultCreateTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, bundle.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(bundle, udsv1alpha2.SchemeGroupVersion.WithKind("UDSBundleJob")).
		WithLabels(map[string]string{
			"app":                  "forge",
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  "create",
		}).
		WithContainerImage(constants.UDSCLIImage).
		WithContainerName(constants.ContainerNameUDSCreate).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{udsCmd}).
		WithWorkingDir(workingDir).
		WithResources(handler.getResources(bundle)).
		WithBackoffLimit(0).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Add git credentials volume if needed
	if bundle.Spec.Source.Type == udsv1alpha2.SourceTypeGit &&
		bundle.Spec.Source.Git != nil &&
		bundle.Spec.Source.Git.CredentialsSecretRef != nil && // pragma: allowlist secret
		!bundle.Spec.Source.Git.DisableCloneCredentials {

		secretName := bundle.Spec.Source.Git.CredentialsSecretRef.Name // pragma: allowlist secret
		builder.WithCustomVolume(corev1.Volume{
			Name: "git-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	// Create or get the job
	job, err := builder.CreateOrGet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildUDSCommand builds the UDS CLI command for bundle creation
func (handler *CreateHandler) buildUDSCommand(_ *udsv1alpha2.UDSBundleJob, artifactPVCName string) (string, string) {
	workingDir := constants.VolumeMountPathWorkspace

	// Determine output directory based on whether we're using a PVC for multi-action workflows
	var outputDir string
	if artifactPVCName != "" {
		// Multi-action job: output to shared PVC directory for subsequent actions
		outputDir = constants.VolumeMountPathArtifacts
	} else {
		// Standalone create: output to EmptyDir volume
		outputDir = constants.VolumeMountPathOutput
	}

	// UDS bundle create command
	// Assumes uds-bundle.yaml is in the workspace root
	cmd := "uds create . --confirm --output-directory " + outputDir

	return cmd, workingDir
}

// buildInitContainers creates init containers for source retrieval
//
//nolint:staticcheck // SA1019: UDSBundleJob v1alpha1 must be supported until v0.10.0
func (handler *CreateHandler) buildInitContainers(bundle *udsv1alpha2.UDSBundleJob) ([]corev1.Container, error) {
	// Use shared source handler logic from pkg/sources
	container, err := sources.GetUDSInitContainer(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// buildVolumes creates volumes for the create job
func (handler *CreateHandler) buildVolumes(bundle *udsv1alpha2.UDSBundleJob) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: constants.VolumeNameWorkspace,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: constants.VolumeNameOutput,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Add git credentials volume if needed  # pragma: allowlist secret
	if bundle.Spec.Source.Type == udsv1alpha2.SourceTypeGit &&
		bundle.Spec.Source.Git != nil &&
		bundle.Spec.Source.Git.CredentialsSecretRef != nil && // pragma: allowlist secret
		!bundle.Spec.Source.Git.DisableCloneCredentials {

		secretName := bundle.Spec.Source.Git.CredentialsSecretRef.Name // pragma: allowlist secret
		volumes = append(volumes, corev1.Volume{
			Name: "git-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	return volumes
}

// getResources returns resource requirements for the bundle create job
func (handler *CreateHandler) getResources(bundle *udsv1alpha2.UDSBundleJob) corev1.ResourceRequirements {
	// Use user-provided resources if specified
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for UDS bundle creation
	// Standardized with Zarf Build (both create artifacts)
	return actions.BuildResourceRequirements()
}
