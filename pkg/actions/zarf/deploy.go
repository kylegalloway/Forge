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

	// If multi-action job, update artifactPath to use glob pattern for PVC location
	if artifactPVCName != "" {
		artifactPath = constants.VolumeMountPathArtifacts + "/*.tar.zst"
	}

	// Build deploy command based on target
	deployCmd := handler.buildDeployCommand(pkg, artifactPath)

	// Build init containers for artifact retrieval (if needed)
	initContainers, err := handler.buildInitContainers(pkg, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Parse timeout
	timeoutStr := ""
	if pkg.Spec.Deploy != nil {
		timeoutStr = pkg.Spec.Deploy.Timeout
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout)

	// Build env vars
	envVars := handler.buildEnvVars(pkg)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, pkg.Namespace).
		WithOwnerReference(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob")).
		WithLabels(map[string]string{
			"app":                  "forge",
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  "deploy",
		}).
		WithContainerImage(constants.ZarfCLIImage).
		WithContainerName(constants.ContainerNameZarfDeploy).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{deployCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithResources(handler.getResources(pkg)).
		WithBackoffLimit(0).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Add env vars
	for _, envVar := range envVars {
		builder.WithEnvVar(envVar.Name, envVar.Value)
	}

	// Add source credential volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil { // pragma: allowlist secret
		builder.WithDockerConfigSecret(pkg.Spec.Source.OCI.CredentialsSecretRef.Name) // pragma: allowlist secret
	}

	// Build the job spec so we can apply additional configuration
	job := builder.Build()

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

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(pkg.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Job already exists, reusing", "name", pkg.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(pkg.Namespace).Create(ctx, job, metav1.CreateOptions{})
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
