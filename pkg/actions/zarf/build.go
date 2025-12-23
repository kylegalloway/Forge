// Package actions provides handlers for ZarfPackageJob actions (Build, Publish, Deploy).
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

	"github.com/kylegalloway/forge/pkg/actions/common"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
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
func (handler *BuildHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPVCName string) (*common.ActionResult, error) {

	klog.InfoS("Executing Build action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	// Record build started
	handler.metrics.RecordBuildStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate source is provided
	if pkg.Spec.Source.Type == "" {
		handler.metrics.RecordBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("source type is required for Build action")
	}

	// Create Kubernetes Job to build the package
	job, err := handler.createBuildJob(ctx, pkg, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create build job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "build")

	klog.InfoS("Build job created", "name", pkg.Name, "job", job.Name)

	result := &common.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Build job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBuildJob creates a Kubernetes Job to build a Zarf package
func (handler *BuildHandler) createBuildJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-build", pkg.Name)
	namespace := pkg.Namespace

	// Build zarf command based on source type and artifact PVC
	zarfCmd, workingDir := handler.buildZarfCommand(pkg, artifactPVCName)

	// Build init containers
	initContainers, err := handler.buildInitContainers(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Job configuration
	backoffLimit := int32(0)             // Don't retry failed builds
	activeDeadlineSeconds := int64(3600) // 1 hour timeout

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge",
				constants.LabelPackage: pkg.Name,
				constants.LabelAction:  "build",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")),
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
						constants.LabelPackage: pkg.Name,
						constants.LabelAction:  "build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       "zarf-build",
							Image:      constants.ZarfCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{zarfCmd},
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
								RunAsUser:                common.Ptr(int64(constants.DefaultZarfUID)),
								AllowPrivilegeEscalation: common.Ptr(false),
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
						{
							Name: "output",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: common.Ptr(true),
						RunAsUser:    common.Ptr(int64(constants.DefaultZarfUID)),
						FSGroup:      common.Ptr(int64(constants.DefaultZarfUID)),
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
		// Ensure the job has at least one container before accessing Containers[0]
		if len(job.Spec.Template.Spec.Containers) == 0 {
			return nil, fmt.Errorf("job has no containers, cannot add artifact PVC volume")
		}
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

	// Add docker-config volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "docker-config",
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

// buildZarfCommand builds the zarf CLI command based on package source
func (handler *BuildHandler) buildZarfCommand(_ *zarfv1alpha1.ZarfPackageJob, artifactPVCName string) (string, string) {
	workingDir := "/workspace"

	// Build command - output to /artifacts if PVC exists, otherwise /output
	var cmd string
	if artifactPVCName != "" {
		// Multi-action job: output to shared PVC directory
		// Zarf will generate filename based on package metadata
		cmd = "zarf package create . --confirm --output-directory /artifacts"
	} else {
		// Standalone build: output to EmptyDir
		cmd = "zarf package create . --confirm --output-directory /output"
	}

	return cmd, workingDir
}

// buildInitContainers creates init containers for source artifact retrieval
func (handler *BuildHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackageJob) ([]corev1.Container, error) {
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
func (handler *BuildHandler) getResources(pkg *zarfv1alpha1.ZarfPackageJob) corev1.ResourceRequirements {
	// If custom resources specified, use them
	if pkg.Spec.Resources != nil {
		return *pkg.Spec.Resources
	}

	// Default resources for build jobs
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    common.MustParseQuantity("200m"),
			corev1.ResourceMemory: common.MustParseQuantity("512Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    common.MustParseQuantity("1000m"),
			corev1.ResourceMemory: common.MustParseQuantity("2Gi"),
		},
	}
}
