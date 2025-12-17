package actions

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// DeployHandler handles Deploy actions for ZarfPackageJob resources
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

// Execute performs a Deploy action for the given ZarfPackageJob
func (handler *DeployHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (*ActionResult, error) {
	klog.InfoS("Executing Deploy action", "name", pkg.Name, "namespace", pkg.Namespace)

	// Record deploy started
	handler.metrics.RecordDeployStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate deploy config is provided
	if pkg.Spec.Deploy == nil {
		handler.metrics.RecordDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("deploy configuration is required for Deploy action")
	}

	// Create Kubernetes Job to deploy the package
	job, err := handler.createDeployJob(ctx, pkg, artifactPath)
	if err != nil {
		handler.metrics.RecordDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "deploy")

	klog.InfoS("Deploy job created", "name", pkg.Name, "job", job.Name)

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Deploy job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createDeployJob creates a Kubernetes Job to deploy a Zarf package
func (handler *DeployHandler) createDeployJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-deploy", pkg.Name)
	namespace := pkg.Namespace

	// Build deploy command based on target
	deployCmd := handler.buildDeployCommand(pkg, artifactPath)

	// Build init containers for artifact retrieval (if needed)
	initContainers, err := handler.buildInitContainers(pkg, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Parse timeout (default 30m)
	activeDeadlineSeconds := int64(1800) // Default 30 minutes
	if pkg.Spec.Deploy.Timeout != "" {
		timeout, parseErr := time.ParseDuration(pkg.Spec.Deploy.Timeout)
		if parseErr != nil {
			klog.V(4).InfoS("Invalid timeout format, using default", "timeout", pkg.Spec.Deploy.Timeout, "error", parseErr)
		} else {
			activeDeadlineSeconds = int64(timeout.Seconds())
		}
	}

	// Job configuration
	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                     "forge",
				"forge.forge.dev/package": pkg.Name,
				"forge.forge.dev/action":  "deploy",
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
						"app":                     "forge",
						"forge.forge.dev/package": pkg.Name,
						"forge.forge.dev/action":  "deploy",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       "zarf-deploy",
							Image:      ZarfCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{deployCmd},
							WorkingDir: "/workspace",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
							Env: handler.buildEnvVars(pkg),
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

	// Add ServiceAccount for in-cluster or external cluster access
	handler.addServiceAccount(pkg, job)

	// Add kubeconfig volume for external cluster deploys
	if pkg.Spec.Deploy.Target == zarfv1alpha1.DeployTargetExternalCluster {
		handler.addKubeconfigVolume(pkg, job)
	}

	// Add source credential volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "source-docker-config",
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

// buildDeployCommand builds the zarf deploy command
func (handler *DeployHandler) buildDeployCommand(pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) string {
	deploy := pkg.Spec.Deploy

	// Base command
	cmd := fmt.Sprintf("zarf package deploy %s --confirm", artifactPath)

	// Add components if specified
	if len(deploy.Components) > 0 {
		for _, comp := range deploy.Components {
			cmd = fmt.Sprintf("%s --components=%s", cmd, comp)
		}
	}

	// Add variables if specified
	for key, value := range deploy.SetVariables {
		cmd = fmt.Sprintf("%s --set %s=%s", cmd, key, value)
	}

	// External cluster needs kubeconfig
	if deploy.Target == zarfv1alpha1.DeployTargetExternalCluster {
		cmd = fmt.Sprintf("export KUBECONFIG=/kubeconfig/config && %s", cmd)
		if deploy.ExternalCluster != nil && deploy.ExternalCluster.Context != "" {
			cmd = fmt.Sprintf("%s --kubeconfig-context=%s", cmd, deploy.ExternalCluster.Context)
		}
	}

	return cmd
}

// buildEnvVars builds environment variables for deploy job
func (handler *DeployHandler) buildEnvVars(pkg *zarfv1alpha1.ZarfPackageJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "ZARF_CONFIRM",
			Value: "true",
		},
	}

	// Add namespace
	if pkg.Spec.Deploy.Namespace != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ZARF_NAMESPACE",
			Value: pkg.Spec.Deploy.Namespace,
		})
	}

	return envVars
}

// buildInitContainers creates init containers for artifact retrieval
func (handler *DeployHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackageJob, _ string) ([]corev1.Container, error) {
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
func (handler *DeployHandler) getResources(pkg *zarfv1alpha1.ZarfPackageJob) corev1.ResourceRequirements {
	// If custom resources specified, use them
	if pkg.Spec.Resources != nil {
		return *pkg.Spec.Resources
	}

	// Default resources for deploy jobs
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("500m"),
			corev1.ResourceMemory: mustParseQuantity("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity("2000m"),
			corev1.ResourceMemory: mustParseQuantity("4Gi"),
		},
	}
}

// addServiceAccount adds the ServiceAccount to the job pod
func (handler *DeployHandler) addServiceAccount(pkg *zarfv1alpha1.ZarfPackageJob, job *batchv1.Job) {
	// Use the ZarfPackageJob's ServiceAccount
	job.Spec.Template.Spec.ServiceAccountName = pkg.Spec.ServiceAccountName
}

// addKubeconfigVolume adds kubeconfig Secret volume for external cluster deployments
func (handler *DeployHandler) addKubeconfigVolume(pkg *zarfv1alpha1.ZarfPackageJob, job *batchv1.Job) {
	if pkg.Spec.Deploy.ExternalCluster == nil || pkg.Spec.Deploy.ExternalCluster.KubeconfigSecretRef.Name == "" {
		return
	}

	// Ensure the job has at least one container before accessing Containers[0]
	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add kubeconfig volume", "job", job.Name)
		return
	}

	secretName := pkg.Spec.Deploy.ExternalCluster.KubeconfigSecretRef.Name

	// Add volume
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	})

	// Add volume mount
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: "/kubeconfig",
			ReadOnly:  true,
		},
	)
}
