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
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// PublishHandler handles Publish actions for ZarfPackage resources
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

// Execute performs a Publish action for the given ZarfPackage
func (h *PublishHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage, artifactPath string) (*ActionResult, error) {
	klog.InfoS("Executing Publish action", "name", pkg.Name, "namespace", pkg.Namespace)

	// Validate publish destination is provided
	if pkg.Spec.Publish == nil {
		return nil, fmt.Errorf("publish configuration is required for Publish action")
	}

	// Create Kubernetes Job to publish the package
	job, err := h.createPublishJob(ctx, pkg, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	klog.InfoS("Publish job created", "name", pkg.Name, "job", job.Name)

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a Zarf package
func (h *PublishHandler) createPublishJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage, artifactPath string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-publish", pkg.Name)
	namespace := pkg.Namespace

	// Build publish command based on destination type
	publishCmd := h.buildPublishCommand(pkg, artifactPath)

	// Build init containers for artifact retrieval (if needed)
	initContainers := h.buildInitContainers(pkg, artifactPath)

	// Job configuration
	backoffLimit := int32(0)
	activeDeadlineSeconds := int64(1800) // 30 minutes timeout

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                    "forge",
				"forge.zarf.dev/package": pkg.Name,
				"forge.zarf.dev/action":  "publish",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackage")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    "forge",
						"forge.zarf.dev/package": pkg.Name,
						"forge.zarf.dev/action":  "publish",
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("250m"),
									corev1.ResourceMemory: mustParseQuantity("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("1000m"),
									corev1.ResourceMemory: mustParseQuantity("2Gi"),
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

	// Add volume mounts and env vars for credentials if needed
	h.addCredentialVolumes(pkg, job)

	// Create the job
	createdJob, err := h.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildPublishCommand builds the zarf/uds publish command based on destination
func (h *PublishHandler) buildPublishCommand(pkg *zarfv1alpha1.ZarfPackage, artifactPath string) string {
	dest := pkg.Spec.Publish.Destination

	switch dest.Type {
	case zarfv1alpha1.DestinationTypeOCI:
		// zarf package publish <artifact> oci://<registry>/<repository>:<tag>
		ociRef := fmt.Sprintf("oci://%s/%s:%s",
			dest.OCI.Registry,
			dest.OCI.Repository,
			dest.OCI.Tag,
		)
		return fmt.Sprintf("zarf package publish %s %s", artifactPath, ociRef)

	case zarfv1alpha1.DestinationTypeS3:
		// For S3, we'll use AWS CLI to upload
		s3Path := fmt.Sprintf("s3://%s/%s", dest.S3.Bucket, dest.S3.KeyPrefix)
		return fmt.Sprintf("aws s3 cp %s %s --region %s", artifactPath, s3Path, dest.S3.Region)

	case zarfv1alpha1.DestinationTypeLocal:
		// Local publish (dev mode) - just copy to local path
		return fmt.Sprintf("cp %s %s", artifactPath, dest.Local.Path)

	default:
		return fmt.Sprintf("echo 'Unknown destination type: %s'", dest.Type)
	}
}

// buildInitContainers creates init containers for artifact retrieval
func (h *PublishHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackage, artifactPath string) []corev1.Container {
	var initContainers []corev1.Container

	// If artifactPath is not local, we need to fetch it first
	// This would be the case when Publish is a standalone action (not part of BuildPublish)
	// For now, we assume the artifact is already available in the workspace
	// TODO: Implement artifact fetching from source if needed

	return initContainers
}

// addCredentialVolumes adds Secret volumes and mounts for credentials
func (h *PublishHandler) addCredentialVolumes(pkg *zarfv1alpha1.ZarfPackage, job *batchv1.Job) {
	dest := pkg.Spec.Publish.Destination

	switch dest.Type {
	case zarfv1alpha1.DestinationTypeOCI:
		if dest.OCI != nil && dest.OCI.CredentialsSecretRef != nil {
			// Mount Docker config secret
			secretName := dest.OCI.CredentialsSecretRef.Name
			job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "registry-creds",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			})

			// Mount to standard Docker config location
			job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				job.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "registry-creds",
					MountPath: "/home/zarf/.docker",
					ReadOnly:  true,
				},
			)
		}

	case zarfv1alpha1.DestinationTypeS3:
		if dest.S3 != nil && dest.S3.CredentialsSecretRef != nil {
			// Mount AWS credentials as env vars from secret
			secretName := dest.S3.CredentialsSecretRef.Name
			job.Spec.Template.Spec.Containers[0].EnvFrom = append(
				job.Spec.Template.Spec.Containers[0].EnvFrom,
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			)
		}
	}
}
