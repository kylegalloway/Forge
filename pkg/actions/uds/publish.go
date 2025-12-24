package uds

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions/common"
	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// PublishHandler handles Publish actions for UDSBundleJob resources
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

// Execute performs a Publish action for the given UDSBundleJob
//
//nolint:staticcheck // SA1019: UDSBundleJob v1alpha1 must be supported until v0.10.0
func (handler *PublishHandler) Execute(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*common.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Publish action", "name", bundle.Name, "namespace", bundle.Namespace)

	// Record publish started
	handler.metrics.RecordBundlePublishStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate publish configuration
	if bundle.Spec.Publish == nil || bundle.Spec.Publish.Destination.Type == "" {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("publish destination is required for Publish action")
	}

	// Create Kubernetes Job to publish the bundle
	job, err := handler.createPublishJob(ctx, bundle)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "publish")

	klog.InfoS("Bundle publish job created", "name", bundle.Name, "job", job.Name)

	result := &common.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle publish job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createPublishJob creates a Kubernetes Job to publish a UDS bundle
func (handler *PublishHandler) createPublishJob(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-publish", bundle.Name)
	namespace := bundle.Namespace

	// Build UDS CLI publish command
	udsCmd, err := handler.buildPublishCommand(bundle)
	if err != nil {
		return nil, err
	}

	// Determine timeout - use Publish.Timeout if specified, otherwise use default
	activeDeadlineSeconds := int64(constants.DefaultPublishTimeout)
	if bundle.Spec.Publish.Timeout != "" {
		timeout, parseErr := time.ParseDuration(bundle.Spec.Publish.Timeout)
		if parseErr != nil {
			klog.V(4).InfoS("Invalid publish timeout format, using default", "timeout", bundle.Spec.Publish.Timeout, "error", parseErr)
		} else {
			activeDeadlineSeconds = int64(timeout.Seconds())
		}
	}

	// Job configuration
	backoffLimit := int32(0) // Don't retry failed publishes

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge-uds",
				constants.LabelPackage: bundle.Name,
				constants.LabelAction:  "publish",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bundle, udsv1alpha1.SchemeGroupVersion.WithKind("UDSBundleJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: common.Ptr(int32(3600)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  "forge-uds",
						constants.LabelPackage: bundle.Name,
						constants.LabelAction:  "publish",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: bundle.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    "uds-publish",
							Image:   constants.UDSCLIImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{udsCmd},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
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
							Env:       handler.buildEnvVars(bundle),
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

	// Add credential volumes if needed
	handler.addCredentialVolumes(bundle, job)

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
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

// buildPublishCommand builds the UDS CLI publish command
func (handler *PublishHandler) buildPublishCommand(bundle *udsv1alpha1.UDSBundleJob) (string, error) {
	dest := bundle.Spec.Publish.Destination

	switch dest.Type {
	case udsv1alpha1.BundleDestinationTypeOCI:
		if dest.OCI == nil {
			return "", fmt.Errorf("OCI destination configuration is required")
		}
		// UDS publish command for OCI registry
		ociRef := fmt.Sprintf("%s/%s:%s", dest.OCI.Registry, dest.OCI.Repository, dest.OCI.Tag)
		return fmt.Sprintf("uds publish /workspace/uds-bundle-*.tar.zst %s", ociRef), nil

	case udsv1alpha1.BundleDestinationTypeS3:
		if dest.S3 == nil {
			return "", fmt.Errorf("S3 destination configuration is required")
		}
		// For S3, we'll use AWS CLI to upload
		s3Path := fmt.Sprintf("s3://%s/%s", dest.S3.Bucket, dest.S3.Key)
		return fmt.Sprintf("aws s3 cp /workspace/uds-bundle-*.tar.zst %s", s3Path), nil

	case udsv1alpha1.BundleDestinationTypeLocal:
		// Local destination - just echo success
		return "echo 'Bundle artifact stored locally in /workspace'", nil

	default:
		return "", fmt.Errorf("unsupported destination type: %s", dest.Type)
	}
}

// buildEnvVars builds environment variables for the publish job
func (handler *PublishHandler) buildEnvVars(bundle *udsv1alpha1.UDSBundleJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	dest := bundle.Spec.Publish.Destination

	// S3 configuration
	if dest.Type == udsv1alpha1.BundleDestinationTypeS3 && dest.S3 != nil {
		if dest.S3.Region != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_REGION",
				Value: dest.S3.Region,
			})
		}
		if dest.S3.Endpoint != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_ENDPOINT_URL",
				Value: dest.S3.Endpoint,
			})
		}

		// Add AWS credentials from secret if provided
		if dest.S3.CredentialsSecretRef != nil { // pragma: allowlist secret
			envVars = append(envVars,
				corev1.EnvVar{
					Name: "AWS_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
							LocalObjectReference: corev1.LocalObjectReference{
								Name: dest.S3.CredentialsSecretRef.Name, // pragma: allowlist secret
							},
							Key: "aws_access_key_id",
						},
					},
				},
				corev1.EnvVar{
					Name: "AWS_SECRET_ACCESS_KEY", // pragma: allowlist secret
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
							LocalObjectReference: corev1.LocalObjectReference{
								Name: dest.S3.CredentialsSecretRef.Name, // pragma: allowlist secret
							},
							Key: "aws_secret_access_key", // pragma: allowlist secret
						},
					},
				},
			)
		}
	}

	return envVars
}

// addCredentialVolumes adds credential volumes for OCI registries
func (handler *PublishHandler) addCredentialVolumes(bundle *udsv1alpha1.UDSBundleJob, job *batchv1.Job) {
	dest := bundle.Spec.Publish.Destination

	// Add docker-config volume for OCI registries
	if dest.Type == udsv1alpha1.BundleDestinationTypeOCI && dest.OCI != nil && dest.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		// Ensure the job has at least one container before accessing Containers[0]
		if len(job.Spec.Template.Spec.Containers) == 0 {
			klog.ErrorS(nil, "Job has no containers, cannot add credential volumes", "job", job.Name)
			return
		}

		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
					SecretName: dest.OCI.CredentialsSecretRef.Name, // pragma: allowlist secret
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})

		// Add volume mount to container
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "docker-config",
				MountPath: "/.docker",
				ReadOnly:  true,
			},
		)

		// Set DOCKER_CONFIG env var
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: "/.docker",
			},
		)
	}
}

// getResources returns resource requirements for the publish job
func (handler *PublishHandler) getResources(bundle *udsv1alpha1.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for publish jobs
	// Standardized with Zarf Publish (both upload artifacts to registries/storage)
	// Lower than Create/Deploy since primarily network I/O with minimal processing
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
