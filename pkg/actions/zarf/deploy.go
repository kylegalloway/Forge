package zarf

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/resources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// DeployHandler handles Deploy actions for ZarfPackageJob resources
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

// Execute performs a Deploy action for the given ZarfPackageJob
func (handler *DeployHandler) Execute(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string, artifactPVCName string) (*actions.ActionResult, error) {
	klog.InfoS("Executing Zarf Package Deploy action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	handler.metrics.RecordPackageDeployStarted(ctx, pkg.Namespace, pkg.Name)

	if pkg.Spec.Deploy == nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("deploy configuration is required for Deploy action")
	}

	if err := handler.validateAdoptionConfig(ctx, pkg); err != nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("adoption configuration validation failed: %w", err)
	}

	// For multi-action jobs, use glob pattern in the PVC
	if artifactPVCName != "" {
		artifactPath = constants.VolumeMountPathArtifacts + "/*.tar.zst"
	}

	deployCmd, err := handler.buildDeployCommand(pkg, artifactPath)
	if err != nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to build deploy command: %w", err)
	}

	// Init containers only for standalone deploy (not chained)
	var initContainers []corev1.Container
	if artifactPVCName == "" {
		initContainers, err = buildZarfInitContainers(pkg)
		if err != nil {
			handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
			return nil, fmt.Errorf("failed to build init containers: %w", err)
		}
	}

	timeoutStr := ""
	var maxRetries *int32
	if pkg.Spec.Deploy != nil {
		timeoutStr = pkg.Spec.Deploy.Timeout
		if pkg.Spec.Deploy.Retry != nil {
			maxRetries = pkg.Spec.Deploy.Retry.MaxRetries
		}
	}

	var deployActionExtraMounts []common.ExtraMount
	if pkg.Spec.Deploy != nil {
		deployActionExtraMounts = pkg.Spec.Deploy.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(pkg.Spec.ExtraMounts, deployActionExtraMounts)
	if err != nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	// Source credential volumes only for standalone deploy
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume
	if artifactPVCName == "" {
		ociCredSecret, s3CredVol, gitCredVol = zarfSourceCredentialVolumes(pkg.Spec.Source)
	}

	envVars := handler.buildEnvVars(pkg)

	// Kubeconfig setup
	var kubeconfigSecretName, kubeconfigKey string
	useInCluster := false
	if pkg.Spec.Deploy.Target == zarfv1alpha3.DeployTargetExternalCluster {
		if pkg.Spec.Deploy.ExternalCluster != nil && pkg.Spec.Deploy.ExternalCluster.SecretRef.Name != "" { // pragma: allowlist secret
			kubeconfigSecretName = pkg.Spec.Deploy.ExternalCluster.SecretRef.Name // pragma: allowlist secret
			kubeconfigKey = pkg.Spec.Deploy.ExternalCluster.Key
		}
	} else {
		useInCluster = true
	}

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-deploy", pkg.Name),
		Namespace:     pkg.Namespace,
		CLIImage:      constants.ZarfCLIImage,
		ContainerUID:  constants.DefaultZarfUID,
		ContainerName: constants.ContainerNameZarfDeploy,
		Args:          []string{deployCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueZarf,
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  constants.ActionDeploy,
		},
		OwnerRef:               pkg,
		OwnerGVK:               zarfv1alpha3.SchemeGroupVersion.WithKind("ZarfPackageJob"),
		NodeSelector:           pkg.Spec.NodeSelector,
		Affinity:               pkg.Spec.Affinity,
		Tolerations:            pkg.Spec.Tolerations,
		Resources:              getResources(pkg.Spec.Resources, actions.DeployResourceRequirements),
		MaxRetries:             maxRetries,
		Timeout:                actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout),
		VolumeSizes:            pkg.Spec.VolumeSizes,
		ArtifactPVCName:        artifactPVCName,
		InitContainers:         initContainers,
		SourceOCICredSecret:    ociCredSecret,
		SourceS3CredVol:        s3CredVol,
		SourceGitCredVol:       gitCredVol,
		ExtraMounts:            extraMounts,
		ServiceAccountName:     pkg.Spec.ServiceAccountName,
		DebugMode:              actions.ShouldDebugAction(pkg.GetDebugMode() || constants.DebugMode, pkg.GetDebugActions(), constants.ActionDeploy),
		EnvVars:                envVars,
		KubeconfigSecretName:   kubeconfigSecretName, // pragma: allowlist secret
		KubeconfigKey:          kubeconfigKey,
		UseInClusterKubeconfig: useInCluster,
		KubeClient:             handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordPackageDeployFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "deploy")
	klog.InfoS("Zarf package deploy job created", "name", pkg.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Deploy job %s created", job.Name)), nil
}

// buildDeployCommand builds the zarf deploy command
func (handler *DeployHandler) buildDeployCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string) (string, error) {
	deploy := pkg.Spec.Deploy

	packagePath := artifactPath
	if packagePath == "" {
		packagePath = constants.VolumeMountPathWorkspace + "/*.tar.zst"
	}

	cmd := fmt.Sprintf("zarf package deploy %s --confirm", packagePath)

	if len(deploy.Components) > 0 {
		for _, comp := range deploy.Components {
			cmd = fmt.Sprintf("%s --components=%s", cmd, comp)
		}
	}

	for key, value := range deploy.Variables {
		cmd = fmt.Sprintf("%s --set %s=%s", cmd, key, value)
	}

	if deploy.AdoptExistingResources {
		cmd = fmt.Sprintf("%s --adopt-existing-resources", cmd)
	}
	if deploy.SkipWebhooks {
		cmd = fmt.Sprintf("%s --skip-webhooks", cmd)
	}
	if deploy.Retries != nil {
		cmd = fmt.Sprintf("%s --retries=%d", cmd, *deploy.Retries)
	}

	if len(deploy.ExtraArgs) > 0 {
		var err error
		cmd, err = validation.AppendExtraArgs(cmd, deploy.ExtraArgs)
		if err != nil {
			return "", fmt.Errorf("invalid extraArgs: %w", err)
		}
	}

	if deploy.Target == zarfv1alpha3.DeployTargetExternalCluster {
		if deploy.ExternalCluster != nil && deploy.ExternalCluster.Context != "" {
			cmd = fmt.Sprintf("%s --kubeconfig-context=%s", cmd, deploy.ExternalCluster.Context)
		}
	}

	return cmd, nil
}

// buildEnvVars builds environment variables for deploy job
func (handler *DeployHandler) buildEnvVars(pkg *zarfv1alpha3.ZarfPackageJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "ZARF_CONFIRM",
			Value: "true",
		},
	}

	if pkg.Spec.Deploy.Namespace != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ZARF_NAMESPACE",
			Value: pkg.Spec.Deploy.Namespace,
		})
	}

	return envVars
}

// validateAdoptionConfig validates resource adoption configuration
func (handler *DeployHandler) validateAdoptionConfig(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob) error {
	if pkg.Spec.Deploy.AdoptionPolicy == nil {
		return nil
	}

	adoptionPolicy := *pkg.Spec.Deploy.AdoptionPolicy

	klog.V(4).InfoS("Validating adoption configuration", "policy", adoptionPolicy, "package", pkg.Name)

	if adoptionPolicy == zarfv1alpha3.AdoptionPolicyAdopt {
		if pkg.Spec.Deploy.ResourceSelector == nil {
			return fmt.Errorf("resourceSelector is required when adoptionPolicy is 'Adopt'")
		}

		selector := pkg.Spec.Deploy.ResourceSelector
		if len(selector.MatchLabels) == 0 && len(selector.MatchNames) == 0 {
			return fmt.Errorf("resourceSelector must specify at least one of matchLabels or matchNames")
		}

		namespaces := selector.Namespaces
		if len(namespaces) == 0 {
			namespaces = []string{pkg.Spec.Deploy.Namespace}
		}

		discoverer := resources.NewDiscoverer(handler.dynamicClient)
		discovered, err := discoverer.DiscoverZarfResources(ctx, selector, namespaces)
		if err != nil {
			return fmt.Errorf("failed to discover resources: %w", err)
		}

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

		klog.InfoS("Resource adoption validation passed", "package", pkg.Name, "resourcesFound", len(discovered))
	}

	return nil
}
