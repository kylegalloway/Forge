package zarf

import (
	"context"
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions/common"

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
func (handler *PublishHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string, artifactPVCName string) (*common.ActionResult, error) {
	klog.InfoS("Executing Publish action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	// Record publish started
	handler.metrics.RecordPublishStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate publish destination is provided
	if pkg.Spec.Publish == nil {
		handler.metrics.RecordPublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("publish configuration is required for Publish action")
	}

	// Create Kubernetes Job to publish the package
	job, err := handler.createPublishJob(ctx, pkg, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordPublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "publish")

	klog.InfoS("Publish job created", "name", pkg.Name, "job", job.Name)

	result := &common.ActionResult{
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
	namespace := pkg.Namespace

	// If multi-action job, update artifactPath to use glob pattern for PVC location
	if artifactPVCName != "" {
		// Use glob pattern to find the zarf package created by build job
		artifactPath = "/artifacts/*.tar.zst"
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

	// Job configuration
	backoffLimit := int32(0)
	activeDeadlineSeconds := int64(1800) // 30 minutes timeout

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge",
				constants.LabelPackage: pkg.Name,
				constants.LabelAction:  "publish",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  "forge",
						constants.LabelPackage: pkg.Name,
						constants.LabelAction:  "publish",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       "zarf-publish",
							Image:      ZarfCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{publishCmd},
							WorkingDir: "/workspace",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             ptr(true),
								RunAsUser:                ptr(int64(1000)),
								AllowPrivilegeEscalation: ptr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Resources: handler.getResources(pkg),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr(true),
						RunAsUser:    ptr(int64(1000)),
						FSGroup:      ptr(int64(1000)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	// Add artifact PVC if multi-action job
	if artifactPVCName != "" {
		artifactVolume := corev1.Volume{
			Name: "artifacts",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: artifactPVCName,
				},
			},
		}
		artifactMount := corev1.VolumeMount{
			Name:      "artifacts",
			MountPath: "/artifacts",
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, artifactVolume)
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[0].VolumeMounts,
			artifactMount,
		)
	}

	// Apply destination-specific configuration (volumes, env vars, etc.)
	jobConfig, err := destHandler.GetJobConfiguration(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	if jobConfig != nil {
		// Ensure the job has at least one container before accessing Containers[0]
		if len(job.Spec.Template.Spec.Containers) == 0 {
			return nil, fmt.Errorf("job has no containers, cannot apply job configuration")
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, jobConfig.Volumes...)
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, jobConfig.VolumeMounts...)
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, jobConfig.Env...)
		job.Spec.Template.Spec.Containers[0].EnvFrom = append(job.Spec.Template.Spec.Containers[0].EnvFrom, jobConfig.EnvFrom...)
	}

	// Add source credential volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "source-docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
					SecretName: pkg.Spec.Source.OCI.CredentialsSecretRef.Name, // pragma: allowlist secret
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})
	}

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// Job already exists, return it
		klog.V(2).InfoS("Job already exists, reusing", "name", pkg.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
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

	// Default resources for publish jobs (slightly less than build)
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("250m"),
			corev1.ResourceMemory: mustParseQuantity("512Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("1000m"),
			corev1.ResourceMemory: mustParseQuantity("2Gi"),
		},
	}
}
