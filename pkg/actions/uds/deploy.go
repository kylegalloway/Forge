package uds

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// DeployHandler handles Deploy actions for UDSBundleJob resources
type DeployHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewDeployHandler creates a new DeployHandler
func NewDeployHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *DeployHandler {
	return &DeployHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Deploy action for the given UDSBundleJob
func (handler *DeployHandler) Execute(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Deploy action", "name", bundle.Name, "namespace", bundle.Namespace)

	// Record deploy started
	handler.metrics.RecordBundleDeployStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate deploy configuration
	if bundle.Spec.Deploy == nil || bundle.Spec.Deploy.Target == "" {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("deploy target is required for Deploy action")
	}

	// Create Kubernetes Job to deploy the bundle
	job, err := handler.createDeployJob(ctx, bundle)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "deploy")

	klog.InfoS("Bundle deploy job created", "name", bundle.Name, "job", job.Name)

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle deploy job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createDeployJob creates a Kubernetes Job to deploy a UDS bundle
func (handler *DeployHandler) createDeployJob(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-deploy", bundle.Name)
	namespace := bundle.Namespace

	// Build UDS CLI deploy command
	udsCmd := handler.buildDeployCommand(bundle)

	// Determine timeout (default 60m, configurable)
	timeout := "60m"
	if bundle.Spec.Deploy.Timeout != "" {
		timeout = bundle.Spec.Deploy.Timeout
	}

	// Convert timeout to seconds for ActiveDeadlineSeconds
	activeDeadlineSeconds, err := handler.parseTimeout(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}

	// Job configuration
	backoffLimit := int32(0) // Don't retry failed deploys

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                    "forge-uds",
				"forge.forge.dev/bundle": bundle.Name,
				"forge.forge.dev/action": "deploy",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bundle, udsv1alpha1.SchemeGroupVersion.WithKind("UDSBundleJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    "forge-uds",
						"forge.forge.dev/bundle": bundle.Name,
						"forge.forge.dev/action": "deploy",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: bundle.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    "uds-deploy",
							Image:   UDSCLIImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{udsCmd},
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

	// Add kubeconfig volume for external cluster deployment
	if bundle.Spec.Deploy.Target == udsv1alpha1.BundleDeployTargetExternalCluster {
		handler.addKubeconfigVolume(bundle, job)
	}

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

// buildDeployCommand builds the UDS CLI deploy command
func (handler *DeployHandler) buildDeployCommand(bundle *udsv1alpha1.UDSBundleJob) string {
	deploy := bundle.Spec.Deploy

	// Base command
	cmd := "uds deploy /workspace/uds-bundle-*.tar.zst --confirm"

	// Add namespace if specified
	if deploy.Namespace != "" {
		cmd += fmt.Sprintf(" --namespace %s", deploy.Namespace)
	}

	// Add component selection if specified
	if len(deploy.Components) > 0 {
		components := strings.Join(deploy.Components, ",")
		cmd += fmt.Sprintf(" --components %s", components)
	}

	// Add variables if specified
	for key, value := range deploy.Variables {
		cmd += fmt.Sprintf(" --set %s=%s", key, value)
	}

	// Add kubeconfig for external cluster
	if deploy.Target == udsv1alpha1.BundleDeployTargetExternalCluster {
		cmd = "export KUBECONFIG=/etc/kubeconfig/kubeconfig && " + cmd
	}

	return cmd
}

// buildEnvVars builds environment variables for the deploy job
func (handler *DeployHandler) buildEnvVars(bundle *udsv1alpha1.UDSBundleJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	// Add timeout as env var for UDS CLI
	if bundle.Spec.Deploy.Timeout != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "UDS_TIMEOUT",
			Value: bundle.Spec.Deploy.Timeout,
		})
	}

	return envVars
}

// addKubeconfigVolume adds kubeconfig volume and mount for external cluster deployment
func (handler *DeployHandler) addKubeconfigVolume(bundle *udsv1alpha1.UDSBundleJob, job *batchv1.Job) {
	if bundle.Spec.Deploy.Kubeconfig == nil || bundle.Spec.Deploy.Kubeconfig.SecretRef.Name == "" { // pragma: allowlist secret
		return
	}

	// Ensure the job has at least one container before accessing Containers[0]
	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add kubeconfig volume", "job", job.Name)
		return
	}

	kubeconfigKey := "kubeconfig"
	if bundle.Spec.Deploy.Kubeconfig.Key != "" {
		kubeconfigKey = bundle.Spec.Deploy.Kubeconfig.Key
	}

	// Add kubeconfig volume
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
				SecretName: bundle.Spec.Deploy.Kubeconfig.SecretRef.Name, // pragma: allowlist secret
				Items: []corev1.KeyToPath{
					{
						Key:  kubeconfigKey,
						Path: "kubeconfig",
					},
				},
			},
		},
	})

	// Add volume mount to container
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: "/etc/kubeconfig",
			ReadOnly:  true,
		},
	)
}

// parseTimeout converts a timeout string (e.g., "60m", "2h") to seconds
func (handler *DeployHandler) parseTimeout(timeout string) (int64, error) {
	// Simple parser for common timeout formats
	// Supports: "30m", "1h", "90m", "2h30m"

	var totalSeconds int64

	// Parse hours
	if strings.Contains(timeout, "h") {
		parts := strings.Split(timeout, "h")
		var hours int64
		_, err := fmt.Sscanf(parts[0], "%d", &hours)
		if err != nil {
			return 0, fmt.Errorf("invalid hours in timeout: %s", timeout)
		}
		totalSeconds += hours * 3600
		timeout = parts[1] // Remaining part after hours
	}

	// Parse minutes
	if strings.Contains(timeout, "m") {
		parts := strings.Split(timeout, "m")
		var minutes int64
		_, err := fmt.Sscanf(parts[0], "%d", &minutes)
		if err != nil {
			return 0, fmt.Errorf("invalid minutes in timeout: %s", timeout)
		}
		totalSeconds += minutes * 60
	}

	// Default to 60 minutes if parsing failed
	if totalSeconds == 0 {
		totalSeconds = 3600
	}

	return totalSeconds, nil
}

// getResources returns resource requirements for the deploy job
func (handler *DeployHandler) getResources(bundle *udsv1alpha1.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for deployment
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("200m"),
			corev1.ResourceMemory: mustParseQuantity("512Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("1"),
			corev1.ResourceMemory: mustParseQuantity("2Gi"),
		},
	}
}
