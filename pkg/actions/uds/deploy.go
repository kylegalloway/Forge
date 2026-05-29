package uds

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
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
// opts.ArtifactPVCName enables multi-action job support (CreateDeploy, etc.).
// When set, the PVC is mounted and opts.ArtifactPath specifies where to find the bundle.
// When empty, assumes standalone deploy with bundle source in workspace or from spec.
func (handler *DeployHandler) Execute(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	artifactPath := opts.ArtifactPath
	artifactPVCName := opts.ArtifactPVCName
	klog.InfoS("Executing UDS Bundle Deploy action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPath", artifactPath, "artifactPVC", artifactPVCName)

	handler.metrics.RecordBundleDeployStarted(ctx, bundle.Namespace, bundle.Name)

	if bundle.Spec.Deploy == nil || bundle.Spec.Deploy.Target == "" {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("deploy target is required for Deploy action")
	}

	if err := handler.validateAdoptionConfig(ctx, bundle); err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("adoption configuration validation failed: %w", err)
	}

	udsCmd, err := handler.buildDeployCommand(bundle, artifactPath)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to build deploy command: %w", err)
	}

	// Init containers only for standalone deploy (not chained)
	var initContainers []corev1.Container
	if artifactPVCName == "" {
		initContainers, err = handler.buildInitContainers(bundle)
		if err != nil {
			handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
			return nil, fmt.Errorf("failed to build init containers: %w", err)
		}
	}

	timeoutStr := ""
	var maxRetries *int32
	if bundle.Spec.Deploy != nil {
		timeoutStr = bundle.Spec.Deploy.Timeout
		if bundle.Spec.Deploy.Retry != nil {
			maxRetries = bundle.Spec.Deploy.Retry.MaxRetries
		}
	}

	var deployActionExtraMounts []apiscommon.ExtraMount
	if bundle.Spec.Deploy != nil {
		deployActionExtraMounts = bundle.Spec.Deploy.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(bundle.Spec.ExtraMounts, deployActionExtraMounts)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	// Source credential volumes only for standalone deploy
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume
	if artifactPVCName == "" {
		ociCredSecret, s3CredVol, gitCredVol = udsSourceCredentialVolumes(bundle.Spec.Source)
	}

	envVars := handler.buildEnvVars(bundle)

	// Kubeconfig setup
	var kubeconfigSecretName, kubeconfigKey string
	useInCluster := false
	if bundle.Spec.Deploy.Target == udsv1alpha3.DeployTargetExternalCluster {
		if bundle.Spec.Deploy.ExternalCluster != nil && bundle.Spec.Deploy.ExternalCluster.SecretRef.Name != "" { // pragma: allowlist secret
			kubeconfigSecretName = bundle.Spec.Deploy.ExternalCluster.SecretRef.Name // pragma: allowlist secret
			kubeconfigKey = bundle.Spec.Deploy.ExternalCluster.Key
		}
	} else {
		useInCluster = true
	}

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-deploy", bundle.Name),
		Namespace:     bundle.Namespace,
		CLIImage:      constants.UDSCLIImage,
		ContainerUID:  constants.DefaultUDSUID,
		ContainerName: constants.ContainerNameUDSDeploy,
		Args:          []string{udsCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueUDS,
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  constants.ActionDeploy,
		},
		OwnerRef:               bundle,
		OwnerGVK:               udsv1alpha3.SchemeGroupVersion.WithKind("UDSBundleJob"),
		NodeSelector:           bundle.Spec.NodeSelector,
		Affinity:               bundle.Spec.Affinity,
		Tolerations:            bundle.Spec.Tolerations,
		Resources:              getUDSResources(bundle.Spec.Resources, actions.DeployResourceRequirements),
		MaxRetries:             maxRetries,
		Timeout:                actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout),
		VolumeSizes:            bundle.Spec.VolumeSizes,
		ArtifactPVCName:        artifactPVCName,
		InitContainers:         initContainers,
		SourceOCICredSecret:    ociCredSecret,
		SourceS3CredVol:        s3CredVol,
		SourceGitCredVol:       gitCredVol,
		ExtraMounts:            extraMounts,
		ServiceAccountName:     bundle.Spec.ServiceAccountName,
		DebugMode:              actions.ShouldDebugAction(bundle.GetDebugMode() || constants.DebugMode, bundle.GetDebugActions(), constants.ActionDeploy),
		EnvVars:                envVars,
		KubeconfigSecretName:   kubeconfigSecretName, // pragma: allowlist secret
		KubeconfigKey:          kubeconfigKey,
		UseInClusterKubeconfig: useInCluster,
		KubeClient:             handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "deploy")
	klog.InfoS("Bundle deploy job created", "name", bundle.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Bundle deploy job %s created", job.Name)), nil
}

// buildDeployCommand builds the UDS CLI deploy command
func (handler *DeployHandler) buildDeployCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPath string) (string, error) {
	deploy := bundle.Spec.Deploy

	var bundlePath string
	if artifactPath != "" {
		bundlePath = artifactPath
	} else {
		bundlePath = constants.VolumeMountPathWorkspace + "/*.tar.zst"
	}

	cmd := "uds deploy " + bundlePath + " --confirm"

	if deploy.Namespace != "" {
		cmd += fmt.Sprintf(" --namespace %s", deploy.Namespace)
	}

	if len(deploy.Components) > 0 {
		components := strings.Join(deploy.Components, ",")
		cmd += fmt.Sprintf(" --components %s", components)
	}

	for key, value := range deploy.Variables {
		cmd += fmt.Sprintf(" --set %s=%s", key, value)
	}

	if deploy.Insecure {
		cmd += " --insecure"
	}
	if deploy.Retries != nil {
		cmd += fmt.Sprintf(" --retries=%d", *deploy.Retries)
	}

	if len(deploy.ExtraArgs) > 0 {
		var err error
		cmd, err = validation.AppendExtraArgs(cmd, deploy.ExtraArgs)
		if err != nil {
			return "", fmt.Errorf("invalid extraArgs: %w", err)
		}
	}

	if len(deploy.PreTasks) > 0 {
		preTaskCmd := buildPreTaskCommands(deploy.PreTasks)
		cmd = preTaskCmd + " && " + cmd
	}

	if deploy.Target == udsv1alpha3.DeployTargetExternalCluster {
		if deploy.ExternalCluster != nil && deploy.ExternalCluster.Context != "" {
			cmd += fmt.Sprintf(" --kubeconfig-context %s", deploy.ExternalCluster.Context)
		}
	}

	return cmd, nil
}

// buildEnvVars builds environment variables for the deploy job
func (handler *DeployHandler) buildEnvVars(bundle *udsv1alpha3.UDSBundleJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

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
	params, err := sources.SourceParamsFromUDS(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build source params: %w", err)
	}

	container, err := sources.GetInitContainer(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// getResources returns resource requirements for the deploy job.
// Kept as a receiver method for backward compatibility with tests.
func (handler *DeployHandler) getResources(bundle *udsv1alpha3.UDSBundleJob) corev1.ResourceRequirements {
	return getUDSResources(bundle.Spec.Resources, actions.DeployResourceRequirements)
}

// validateAdoptionConfig validates resource adoption configuration
func (handler *DeployHandler) validateAdoptionConfig(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob) error {
	if bundle.Spec.Deploy.AdoptionPolicy == nil {
		return nil
	}

	adoptionPolicy := *bundle.Spec.Deploy.AdoptionPolicy

	klog.V(4).InfoS("Validating adoption configuration", "policy", adoptionPolicy, "bundle", bundle.Name)

	if adoptionPolicy == udsv1alpha3.AdoptionPolicyAdopt {
		if bundle.Spec.Deploy.ResourceSelector == nil {
			return fmt.Errorf("resourceSelector is required when adoptionPolicy is 'Adopt'")
		}

		selector := bundle.Spec.Deploy.ResourceSelector
		if len(selector.MatchLabels) == 0 && len(selector.MatchNames) == 0 {
			return fmt.Errorf("resourceSelector must specify at least one of matchLabels or matchNames")
		}

		namespaces := selector.Namespaces
		if len(namespaces) == 0 {
			namespaces = []string{bundle.Spec.Deploy.Namespace}
		}

		discoverer := resources.NewDiscoverer(handler.dynamicClient)
		discovered, err := discoverer.DiscoverUDSResources(ctx, selector, namespaces)
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

		klog.InfoS("Resource adoption validation passed", "bundle", bundle.Name, "resourcesFound", len(discovered))
	}

	return nil
}
