package zarf

import (
	"context"
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
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
func (handler *DeployHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string, artifactPVCName string) (*actions.ActionResult, error) {
	klog.InfoS("Executing Zarf Package Deploy action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	// Record deploy started
	handler.metrics.RecordPackageDeployStarted(ctx, pkg.Namespace, pkg.Name)

	// Validate deploy config is provided
	if pkg.Spec.Deploy == nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("deploy configuration is required for Deploy action")
	}

	// Create Kubernetes Job to deploy the package
	job, err := handler.createDeployJob(ctx, pkg, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "deploy")

	klog.InfoS("Zarf package deploy job created", "name", pkg.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Deploy job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createDeployJob creates a Kubernetes Job to deploy a Zarf package
func (handler *DeployHandler) createDeployJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-deploy", pkg.Name)
	namespace := pkg.Namespace

	// If multi-action job, update artifactPath to use glob pattern for PVC location
	if artifactPVCName != "" {
		// Use glob pattern to find the zarf package created by build job
		artifactPath = constants.VolumeMountPathArtifacts + "/*.tar.zst"
	}

	// Build deploy command based on target
	deployCmd := handler.buildDeployCommand(pkg, artifactPath)

	// Build init containers for artifact retrieval (if needed)
	initContainers, err := handler.buildInitContainers(pkg, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Parse timeout (default 30m)
	timeoutStr := ""
	if pkg.Spec.Deploy != nil {
		timeoutStr = pkg.Spec.Deploy.Timeout
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout)

	// Job configuration
	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge",
				"resource-type":        "zarfpackagejob",
				constants.LabelPackage: pkg.Name,
				constants.LabelAction:  "deploy",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: actions.Ptr(int32(3600)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  "forge",
						"resource-type":        "zarfpackagejob",
						constants.LabelPackage: pkg.Name,
						constants.LabelAction:  "deploy",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       constants.ContainerNameZarfDeploy,
							Image:      constants.ZarfCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{deployCmd},
							WorkingDir: constants.VolumeMountPathWorkspace,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      constants.VolumeNameWorkspace,
									MountPath: constants.VolumeMountPathWorkspace,
								},
							},
							Env:             handler.buildEnvVars(pkg),
							SecurityContext: actions.NonRootSecurityContextWithUID(constants.DefaultZarfUID),
							Resources:       handler.getResources(pkg),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: constants.VolumeNameWorkspace,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					SecurityContext: actions.NonRootPodSecurityContextWithUID(constants.DefaultZarfUID),
				},
			},
		},
	}

	// Add artifact PVC if multi-action job
	actions.AddArtifactPVCVolume(job, artifactPVCName)

	// Add ServiceAccount for in-cluster or external cluster access
	handler.addServiceAccount(pkg, job)

	// Add kubeconfig volume for external cluster deploys
	if pkg.Spec.Deploy.Target == zarfv1alpha1.DeployTargetExternalCluster {
		kubeconfigSecretName := ""
		if pkg.Spec.Deploy.ExternalCluster != nil && pkg.Spec.Deploy.ExternalCluster.KubeconfigSecretRef.Name != "" { // pragma: allowlist secret
			kubeconfigSecretName = pkg.Spec.Deploy.ExternalCluster.KubeconfigSecretRef.Name // pragma: allowlist secret
		}
		actions.AddKubeconfigVolume(job, kubeconfigSecretName)
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
	return actions.DeployResourceRequirements()
}

// addServiceAccount adds the ServiceAccount to the job pod
func (handler *DeployHandler) addServiceAccount(pkg *zarfv1alpha1.ZarfPackageJob, job *batchv1.Job) {
	// Use the ZarfPackageJob's ServiceAccount
	job.Spec.Template.Spec.ServiceAccountName = pkg.Spec.ServiceAccountName
}
