package uds

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/resources"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// DeployHandler handles Deploy actions for UDSBundleJob resources
type DeployHandler struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	metrics       *telemetry.Metrics
	tracer        *telemetry.Tracer
}

// NewDeployHandler creates a new DeployHandler
func NewDeployHandler(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *DeployHandler {
	return &DeployHandler{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		metrics:       metrics,
		tracer:        tracer,
	}
}

// Execute performs a Deploy action for the given UDSBundleJob
//
// The artifactPath and artifactPVCName parameters enable multi-action job support (CreateDeploy, etc.)
// When artifactPVCName is set, the PVC is mounted and artifactPath specifies where to find the bundle.
// When empty, assumes standalone deploy with bundle source in workspace or from spec.
func (handler *DeployHandler) Execute(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, artifactPath, artifactPVCName string) (*actions.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Deploy action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPath", artifactPath, "artifactPVC", artifactPVCName)

	// Record deploy started
	handler.metrics.RecordBundleDeployStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate deploy configuration
	if bundle.Spec.Deploy == nil || bundle.Spec.Deploy.Target == "" {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("deploy target is required for Deploy action")
	}

	// Validate and handle resource adoption configuration
	if err := handler.validateAdoptionConfig(ctx, bundle); err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("adoption configuration validation failed: %w", err)
	}

	// Create Kubernetes Job to deploy the bundle
	job, err := handler.createDeployJob(ctx, bundle, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "deploy")

	klog.InfoS("Bundle deploy job created", "name", bundle.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     constants.PhaseRunning,
		Message:   fmt.Sprintf("Bundle deploy job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createDeployJob creates a Kubernetes Job to deploy a UDS bundle
func (handler *DeployHandler) createDeployJob(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, artifactPath, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-deploy", bundle.Name)

	// Build UDS CLI deploy command
	udsCmd, err := handler.buildDeployCommand(bundle, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build deploy command: %w", err)
	}

	// Build env vars
	envVars := handler.buildEnvVars(bundle)

	// Build init containers for artifact retrieval (if needed)
	initContainers, err := handler.buildInitContainers(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	// Determine timeout and retry policy - use Deploy config if specified
	timeoutStr := ""
	var retryPolicy *udsv1alpha3.RetryPolicy
	if bundle.Spec.Deploy != nil {
		timeoutStr = bundle.Spec.Deploy.Timeout
		retryPolicy = bundle.Spec.Deploy.Retry
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, bundle.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(bundle, udsv1alpha3.SchemeGroupVersion.WithKind("UDSBundleJob")).
		WithLabels(map[string]string{
			constants.LabelApp:     constants.LabelAppValueUDS,
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  constants.ActionDeploy,
		}).
		WithContainerImage(constants.UDSCLIImage).
		WithContainerName(constants.ContainerNameUDSDeploy).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{udsCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithUserConfig(constants.DefaultUDSUID).
		WithResources(handler.getResources(bundle)).
		WithNodeSelector(bundle.Spec.NodeSelector).
		WithAffinity(bundle.Spec.Affinity).
		WithTolerations(bundle.Spec.Tolerations).
		WithUDSRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithInitContainers(initContainers).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName).
		WithServiceAccountName(bundle.Spec.ServiceAccountName).
		WithDebugMode(actions.ShouldDebugAction(bundle.GetDebugMode() || constants.DebugMode, bundle.GetDebugActions(), constants.ActionDeploy))

	// Add env vars
	for _, envVar := range envVars {
		builder.WithEnvVar(envVar.Name, envVar.Value)
	}

	// Add source credential volume if OCI source with credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeOCI && bundle.Spec.Source.OCI != nil && bundle.Spec.Source.OCI.CredentialRef != nil { // pragma: allowlist secret
		builder.WithDockerConfigSecret(bundle.Spec.Source.OCI.CredentialRef.Name) // pragma: allowlist secret
	}

	// Add S3 credentials volume if S3 source with file credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeS3 && bundle.Spec.Source.S3 != nil {
		if vol := sources.GetS3CredentialVolume(bundle.Spec.Source.S3.CredentialRef); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Add git credentials volume if Git source with credentials
	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeGit && bundle.Spec.Source.Git != nil {
		if vol := sources.GetGitCredentialVolume(bundle.Spec.Source.Git.CredentialRef, bundle.Spec.Source.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			builder.WithCustomVolume(*vol)
		}
	}

	// Add kubeconfig volume based on deploy target
	if bundle.Spec.Deploy.Target == udsv1alpha3.DeployTargetExternalCluster {
		// External cluster: mount kubeconfig from secret
		kubeconfigSecretName := ""
		kubeconfigKey := ""
		if bundle.Spec.Deploy.ExternalCluster != nil && bundle.Spec.Deploy.ExternalCluster.SecretRef.Name != "" { // pragma: allowlist secret
			kubeconfigSecretName = bundle.Spec.Deploy.ExternalCluster.SecretRef.Name // pragma: allowlist secret
			kubeconfigKey = bundle.Spec.Deploy.ExternalCluster.Key
		}
		builder.WithKubeconfigVolume(kubeconfigSecretName, kubeconfigKey)
	} else {
		// In-cluster: add projected volume for SA token with explicit control
		builder.WithProjectedServiceAccountVolume()
	}

	// Add extra mounts (merged: spec-level + deploy-level)
	var deployExtraMounts []common.ExtraMount
	if bundle.Spec.Deploy != nil {
		deployExtraMounts = bundle.Spec.Deploy.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(bundle.Spec.ExtraMounts, deployExtraMounts)
	if err != nil {
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}
	builder.WithExtraMounts(extraMounts)

	// Create or get the job
	job, err := builder.CreateOrGet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildDeployCommand builds the UDS CLI deploy command
func (handler *DeployHandler) buildDeployCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPath string) (string, error) {
	deploy := bundle.Spec.Deploy

	// Determine bundle path - use artifactPath if provided (multi-action workflow),
	// otherwise search workspace for bundle (standalone deploy)
	var bundlePath string
	if artifactPath != "" {
		bundlePath = artifactPath
	} else {
		bundlePath = constants.VolumeMountPathWorkspace + "/uds-bundle-*.tar.zst"
	}

	// Base command
	cmd := "uds deploy " + bundlePath + " --confirm"

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

	// Structured flags
	if deploy.Insecure {
		cmd += " --insecure"
	}
	if deploy.Retries != nil {
		cmd += fmt.Sprintf(" --retries=%d", *deploy.Retries)
	}

	// ExtraArgs (validated and shell-escaped)
	if len(deploy.ExtraArgs) > 0 {
		var err error
		cmd, err = validation.AppendExtraArgs(cmd, deploy.ExtraArgs)
		if err != nil {
			return "", fmt.Errorf("invalid extraArgs: %w", err)
		}
	}

	// Configure kubeconfig based on deploy target
	if deploy.Target == udsv1alpha3.DeployTargetExternalCluster {
		// External cluster: mount kubeconfig from secret
		cmd = fmt.Sprintf("export KUBECONFIG=%s/kubeconfig && ", constants.VolumeMountPathKubeconfig) + cmd
		// Add context flag if specified
		if deploy.ExternalCluster != nil && deploy.ExternalCluster.Context != "" {
			cmd += fmt.Sprintf(" --kubeconfig-context %s", deploy.ExternalCluster.Context)
		}
	} else {
		// In-cluster: generate kubeconfig from service account token
		cmd = constants.InClusterKubeconfigSetup + cmd
	}

	return cmd, nil
}

// buildEnvVars builds environment variables for the deploy job
func (handler *DeployHandler) buildEnvVars(bundle *udsv1alpha3.UDSBundleJob) []corev1.EnvVar {
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

// buildInitContainers creates init containers for artifact retrieval
func (handler *DeployHandler) buildInitContainers(bundle *udsv1alpha3.UDSBundleJob) ([]corev1.Container, error) {
	container, err := sources.GetUDSInitContainer(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// getResources returns resource requirements for the deploy job
func (handler *DeployHandler) getResources(bundle *udsv1alpha3.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for deploy jobs
	// Standardized with Zarf Deploy (both deploy packages to clusters)
	return actions.DeployResourceRequirements()
}

// validateAdoptionConfig validates resource adoption configuration
func (handler *DeployHandler) validateAdoptionConfig(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob) error {
	// If no adoption policy specified, nothing to validate
	if bundle.Spec.Deploy.AdoptionPolicy == nil {
		return nil
	}

	adoptionPolicy := *bundle.Spec.Deploy.AdoptionPolicy

	klog.V(4).InfoS("Validating adoption configuration", "policy", adoptionPolicy, "bundle", bundle.Name)

	// If policy is "Adopt", ResourceSelector must be provided
	if adoptionPolicy == udsv1alpha3.AdoptionPolicyAdopt {
		if bundle.Spec.Deploy.ResourceSelector == nil {
			return fmt.Errorf("resourceSelector is required when adoptionPolicy is 'Adopt'")
		}

		// Validate that at least one selector criterion is provided
		selector := bundle.Spec.Deploy.ResourceSelector
		if len(selector.MatchLabels) == 0 && len(selector.MatchNames) == 0 {
			return fmt.Errorf("resourceSelector must specify at least one of matchLabels or matchNames")
		}

		// Pre-deployment validation: Check for conflicting owners
		namespaces := selector.Namespaces
		if len(namespaces) == 0 {
			// Default to bundle namespace if not specified
			namespaces = []string{bundle.Spec.Deploy.Namespace}
		}

		// Discover existing resources to validate ownership
		discoverer := resources.NewDiscoverer(handler.dynamicClient)
		discovered, err := discoverer.DiscoverUDSResources(ctx, selector, namespaces)
		if err != nil {
			return fmt.Errorf("failed to discover resources: %w", err)
		}

		// Validate no conflicting owners
		validateOwnership := true
		if selector.ValidateOwnership != nil {
			validateOwnership = *selector.ValidateOwnership
		}

		if validateOwnership && len(discovered) > 0 {
			adopter := resources.NewAdopter(handler.dynamicClient)
			if err := adopter.ValidateNoConflictingOwners(discovered); err != nil {
				return fmt.Errorf("pre-deployment ownership validation failed: %w", err)
			}
		}

		klog.InfoS("Resource adoption validation passed", "bundle", bundle.Name, "resourcesFound", len(discovered))
	}

	return nil
}
