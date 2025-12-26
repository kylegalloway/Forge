package uds

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions/common"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// CreateHandler handles Create actions for UDSPackageJob resources
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

// Execute performs a Create action for the given UDSPackageJob
//
// Unlike Zarf handlers, UDS handlers don't accept an artifactPVCName parameter because
// UDS multi-action jobs (CreatePublish, CreateDeploy, etc.) don't currently implement
// artifact sharing between actions. Each UDS action runs independently in its own Job
// with isolated storage. This design choice simplifies UDS job lifecycle at the cost
// of less efficiency for multi-action workflows.
//
// Future enhancement: Implement artifact PVC sharing for UDS to match Zarf's multi-action
// efficiency (see TODO.md). This would unify the handler signatures across both systems.
func (handler *CreateHandler) Execute(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob) (*common.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Create action", "name", bundle.Name, "namespace", bundle.Namespace)

	// Record create started
	handler.metrics.RecordBundleCreateStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate source is provided
	if bundle.Spec.Source.Type == "" {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("source type is required for Create action")
	}

	// Create Kubernetes Job to create the bundle
	job, err := handler.createBundleJob(ctx, bundle)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create bundle job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "create")

	klog.InfoS("Bundle create job created", "name", bundle.Name, "job", job.Name)

	result := &common.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle create job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBundleJob creates a Kubernetes Job to create a UDS bundle
func (handler *CreateHandler) createBundleJob(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-create", bundle.Name)
	namespace := bundle.Namespace

	// Build UDS CLI command
	udsCmd, workingDir, err := handler.buildUDSCommand(bundle)
	if err != nil {
		return nil, err
	}

	// Build init containers for source retrieval
	initContainers, err := handler.buildInitContainers(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Use default timeout for create operations
	activeDeadlineSeconds := int64(constants.DefaultCreateTimeout)

	// Job configuration
	backoffLimit := int32(0) // Don't retry failed creates

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge",
				"resource-type":        "udspackagejob",
				constants.LabelPackage: bundle.Name,
				constants.LabelAction:  "create",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bundle, udsv1alpha2.SchemeGroupVersion.WithKind("UDSPackageJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: common.Ptr(int32(3600)), // Clean up after 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  "forge",
						"resource-type":        "udspackagejob",
						constants.LabelPackage: bundle.Name,
						constants.LabelAction:  "create",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: bundle.Spec.ServiceAccountName,
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Name:       "uds-create",
							Image:      constants.UDSCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{udsCmd},
							WorkingDir: workingDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
								{
									Name:      "output",
									MountPath: "/output",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             common.Ptr(true),
								RunAsUser:                common.Ptr(int64(constants.DefaultUDSUID)),
								AllowPrivilegeEscalation: common.Ptr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Resources: handler.getResources(bundle),
						},
					},
					Volumes: handler.buildVolumes(bundle),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: common.Ptr(true),
						RunAsUser:    common.Ptr(int64(constants.DefaultUDSUID)),
						FSGroup:      common.Ptr(int64(constants.DefaultUDSUID)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// Job already exists, return it
		klog.V(2).InfoS("Job already exists, reusing", "name", bundle.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildUDSCommand builds the UDS CLI command for bundle creation
func (handler *CreateHandler) buildUDSCommand(_ *udsv1alpha2.UDSPackageJob) (string, string, error) {
	workingDir := "/workspace"

	// UDS bundle create command
	// Assumes uds-bundle.yaml is in the workspace root
	cmd := "uds create . --confirm --output-directory /output"

	return cmd, workingDir, nil
}

// buildInitContainers creates init containers for source retrieval
//
//nolint:staticcheck // SA1019: UDSPackageJob v1alpha1 must be supported until v0.10.0
func (handler *CreateHandler) buildInitContainers(bundle *udsv1alpha2.UDSPackageJob) ([]corev1.Container, error) {
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
func (handler *CreateHandler) buildVolumes(bundle *udsv1alpha2.UDSPackageJob) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "output",
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
func (handler *CreateHandler) getResources(bundle *udsv1alpha2.UDSPackageJob) corev1.ResourceRequirements {
	// Use user-provided resources if specified
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for UDS bundle creation
	// Standardized with Zarf Build (both create artifacts)
	// Higher resources account for bundling multiple packages, compression, and artifact creation
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    common.MustParseQuantity("500m"),
			corev1.ResourceMemory: common.MustParseQuantity("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    common.MustParseQuantity("2000m"),
			corev1.ResourceMemory: common.MustParseQuantity("4Gi"),
		},
	}
}
