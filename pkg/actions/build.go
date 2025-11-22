// Package actions provides handlers for ZarfPackageJob actions (Build, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a Zarf package or UDS bundle.
package actions

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// ZarfCLIImage is the default Zarf CLI container image
	ZarfCLIImage = "ghcr.io/defenseunicorns/zarf:v0.66.0"
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
func (h *BuildHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob) (*ActionResult, error) {

	klog.InfoS("Executing Build action", "name", pkg.Name, "namespace", pkg.Namespace)

	// Validate source is provided
	if pkg.Spec.Source.Type == "" {
		return nil, fmt.Errorf("source type is required for Build action")
	}

	// Create Kubernetes Job to build the package
	job, err := h.createBuildJob(ctx, pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to create build job: %w", err)
	}

	klog.InfoS("Build job created", "name", pkg.Name, "job", job.Name)

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Build job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBuildJob creates a Kubernetes Job to build a Zarf package
func (h *BuildHandler) createBuildJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-build", pkg.Name)
	namespace := pkg.Namespace

	// Build zarf command based on source type
	zarfCmd, workingDir, err := h.buildZarfCommand(pkg)
	if err != nil {
		return nil, err
	}

	// Build init containers
	initContainers, err := h.buildInitContainers(pkg)
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
				"app":                     "forge",
				"forge.forge.dev/package": pkg.Name,
				"forge.forge.dev/action":  "build",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)), // Clean up after 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                     "forge",
						"forge.forge.dev/package": pkg.Name,
						"forge.forge.dev/action":  "build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       "zarf-build",
							Image:      ZarfCLIImage,
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("500m"),
									corev1.ResourceMemory: mustParseQuantity("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("2000m"),
									corev1.ResourceMemory: mustParseQuantity("4Gi"),
								},
							},
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

	// Add docker-config volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pkg.Spec.Source.OCI.CredentialsSecretRef.Name,
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

	// Create the job
	createdJob, err := h.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildZarfCommand builds the zarf CLI command based on package source
func (h *BuildHandler) buildZarfCommand(_ *zarfv1alpha1.ZarfPackageJob) (string, string, error) {
	workingDir := "/workspace"

	// Basic zarf package create command
	cmd := "zarf package create . --confirm --output-directory /output"

	return cmd, workingDir, nil
}

// buildInitContainers creates init containers for source artifact retrieval
func (h *BuildHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackageJob) ([]corev1.Container, error) {
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
