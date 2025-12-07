package uds

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
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
func (handler *CreateHandler) Execute(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*ActionResult, error) {

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

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle create job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBundleJob creates a Kubernetes Job to create a UDS bundle
func (handler *CreateHandler) createBundleJob(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-create", bundle.Name)
	namespace := bundle.Namespace

	// Build UDS CLI command
	udsCmd, workingDir, err := handler.buildUDSCommand(bundle)
	if err != nil {
		return nil, err
	}

	// Build init containers for source retrieval
	initContainers := handler.buildInitContainers(bundle)

	// Job configuration
	backoffLimit := int32(0)             // Don't retry failed creates
	activeDeadlineSeconds := int64(7200) // 2 hour timeout (bundles can be large)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                    "forge-uds",
				"forge.forge.dev/bundle": bundle.Name,
				"forge.forge.dev/action": "create",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bundle, udsv1alpha1.SchemeGroupVersion.WithKind("UDSBundleJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)), // Clean up after 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    "forge-uds",
						"forge.forge.dev/bundle": bundle.Name,
						"forge.forge.dev/action": "create",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: bundle.Spec.ServiceAccountName,
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Name:       "uds-create",
							Image:      UDSCLIImage,
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
								RunAsNonRoot:             ptr(true),
								RunAsUser:                ptr(int64(65532)),
								AllowPrivilegeEscalation: ptr(false),
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
					Volumes: []corev1.Volume{
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
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr(true),
						RunAsUser:    ptr(int64(65532)),
						FSGroup:      ptr(int64(65532)),
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
func (handler *CreateHandler) buildUDSCommand(_ *udsv1alpha1.UDSBundleJob) (string, string, error) {
	workingDir := "/workspace"

	// UDS bundle create command
	// Assumes uds-bundle.yaml is in the workspace root
	cmd := "uds create . --confirm --output-directory /output"

	return cmd, workingDir, nil
}

// buildInitContainers creates init containers for source retrieval
func (handler *CreateHandler) buildInitContainers(bundle *udsv1alpha1.UDSBundleJob) []corev1.Container {
	// Convert UDSBundleJob source to a format compatible with existing source handlers
	// For now, we'll need to adapt the sources package to handle UDS bundles
	// This is a placeholder that will need source package updates

	// TODO: Implement source handler conversion from UDS types to Zarf types
	// or create UDS-specific source handlers

	// For Git sources, we can use the existing Git source handler
	if bundle.Spec.Source.Type == udsv1alpha1.BundleSourceTypeGit && bundle.Spec.Source.Git != nil {
		container := &corev1.Container{
			Name:    "fetch-source",
			Image:   "alpine/git:latest",
			Command: []string{"/bin/sh", "-c"},
			Args: []string{
				fmt.Sprintf("git clone --depth 1 --branch %s %s /workspace && cd /workspace && ls -la",
					bundle.Spec.Source.Git.Ref,
					bundle.Spec.Source.Git.URL),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             ptr(true),
				RunAsUser:                ptr(int64(65532)),
				AllowPrivilegeEscalation: ptr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		}
		return []corev1.Container{*container}
	}

	// For other source types (S3, OCI), return empty for now
	// These will be implemented as part of full source handler integration
	return nil
}

// getResources returns resource requirements for the bundle create job
func (handler *CreateHandler) getResources(bundle *udsv1alpha1.UDSBundleJob) corev1.ResourceRequirements {
	// Use user-provided resources if specified
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for UDS bundle creation (higher than Zarf due to bundle size)
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("500m"),
			corev1.ResourceMemory: mustParseQuantity("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("2"),
			corev1.ResourceMemory: mustParseQuantity("4Gi"),
		},
	}
}
