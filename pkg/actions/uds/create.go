// Package uds provides handlers for UDSBundleJob actions (Create, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a UDS bundle.
package uds

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// CreateHandler handles Create actions for UDSBundleJob resources
type CreateHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewCreateHandler creates a new CreateHandler
func NewCreateHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *CreateHandler {
	return &CreateHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Create action for the given UDSBundleJob
//
// The artifactPVCName parameter enables multi-action job support (CreatePublish, CreateDeploy, etc.)
// by providing a shared PersistentVolumeClaim for artifacts. When set, create outputs are stored
// in the PVC so subsequent actions (Publish/Deploy) can access them without re-creating.
//
// This matches the Zarf Build handler pattern and enables efficient action chaining.
func (handler *CreateHandler) Execute(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, artifactPVCName string) (*actions.ActionResult, error) {
	klog.InfoS("Executing UDS Bundle Create action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPVC", artifactPVCName)

	handler.metrics.RecordBundleCreateStarted(ctx, bundle.Namespace, bundle.Name)

	if bundle.Spec.Source.Type == "" {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("source type is required for Create action")
	}

	udsCmd, err := handler.buildUDSCommand(bundle, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to build UDS command: %w", err)
	}

	initContainers, err := handler.buildInitContainers(bundle)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	timeoutStr := ""
	var maxRetries *int32
	if bundle.Spec.Create != nil {
		timeoutStr = bundle.Spec.Create.Timeout
		if bundle.Spec.Create.Retry != nil {
			maxRetries = bundle.Spec.Create.Retry.MaxRetries
		}
	}

	var regCredSecret, regCredMount string                                            // pragma: allowlist secret
	if bundle.Spec.Create != nil && bundle.Spec.Create.RegistryCredentialRef != nil { // pragma: allowlist secret
		regCredSecret = bundle.Spec.Create.RegistryCredentialRef.Name // pragma: allowlist secret
		regCredMount = constants.VolumeMountPathDockerConfigUDS
	}

	var createActionExtraMounts []common.ExtraMount
	if bundle.Spec.Create != nil {
		createActionExtraMounts = bundle.Spec.Create.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(bundle.Spec.ExtraMounts, createActionExtraMounts)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	ociCredSecret, s3CredVol, gitCredVol := udsSourceCredentialVolumes(bundle.Spec.Source)

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-create", bundle.Name),
		Namespace:     bundle.Namespace,
		CLIImage:      constants.UDSCLIImage,
		ContainerUID:  constants.DefaultUDSUID,
		ContainerName: constants.ContainerNameUDSCreate,
		Args:          []string{udsCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueUDS,
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  constants.ActionCreate,
		},
		OwnerRef:              bundle,
		OwnerGVK:              udsv1alpha3.SchemeGroupVersion.WithKind("UDSBundleJob"),
		NodeSelector:          bundle.Spec.NodeSelector,
		Affinity:              bundle.Spec.Affinity,
		Tolerations:           bundle.Spec.Tolerations,
		Resources:             getUDSResources(bundle.Spec.Resources, actions.BuildResourceRequirements),
		MaxRetries:            maxRetries,
		Timeout:               actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultCreateTimeout),
		VolumeSizes:           bundle.Spec.VolumeSizes,
		ArtifactPVCName:       artifactPVCName,
		InitContainers:        initContainers,
		SourceOCICredSecret:   ociCredSecret,
		SourceS3CredVol:       s3CredVol,
		SourceGitCredVol:      gitCredVol,
		RegistryCredSecret:    regCredSecret, // pragma: allowlist secret
		RegistryCredMountPath: regCredMount,
		ExtraMounts:           extraMounts,
		ServiceAccountName:    bundle.Spec.ServiceAccountName,
		DebugMode:             actions.ShouldDebugAction(bundle.GetDebugMode() || constants.DebugMode, bundle.GetDebugActions(), constants.ActionCreate),
		KubeClient:            handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create bundle job: %w", err)
	}

	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "create")
	klog.InfoS("Bundle create job created", "name", bundle.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Bundle create job %s created", job.Name)), nil
}

// buildUDSCommand builds the UDS CLI command for bundle creation.
// The working directory is always the workspace volume mount; that default is
// absorbed by BuildActionJob and does not need to be returned here.
func (handler *CreateHandler) buildUDSCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPVCName string) (string, error) {
	var outputDir string
	if artifactPVCName != "" {
		outputDir = constants.VolumeMountPathArtifacts
	} else {
		outputDir = constants.VolumeMountPathOutput
	}

	cmd := "uds create . --confirm"

	if bundle.Spec.Create != nil {
		create := bundle.Spec.Create

		if create.Flavor != "" {
			cmd = fmt.Sprintf("%s --flavor %s", cmd, create.Flavor)
		}
		if create.Architecture != "" {
			cmd = fmt.Sprintf("%s --architecture %s", cmd, create.Architecture)
		}
		if create.SkipSBOM {
			cmd = fmt.Sprintf("%s --skip-sbom", cmd)
		}

		if len(create.ExtraArgs) > 0 {
			var err error
			cmd, err = validation.AppendExtraArgs(cmd, create.ExtraArgs)
			if err != nil {
				return "", fmt.Errorf("invalid extraArgs: %w", err)
			}
		}
	}

	cmd = fmt.Sprintf("%s && mv uds-bundle-*.tar.zst %s/", cmd, outputDir)

	if bundle.Spec.Create != nil && len(bundle.Spec.Create.PreTasks) > 0 {
		preTaskCmd := buildPreTaskCommands(bundle.Spec.Create.PreTasks)
		cmd = preTaskCmd + " && " + cmd
	}

	return cmd, nil
}

// buildInitContainers creates init containers for source retrieval
func (handler *CreateHandler) buildInitContainers(bundle *udsv1alpha3.UDSBundleJob) ([]corev1.Container, error) {
	container, err := sources.GetUDSInitContainer(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// buildVolumes creates volumes for the create job.
// Kept for backward compatibility with tests that call it directly.
func (handler *CreateHandler) buildVolumes(bundle *udsv1alpha3.UDSBundleJob) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: constants.VolumeNameWorkspace,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: constants.VolumeNameOutput,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if bundle.Spec.Source.Type == udsv1alpha3.SourceTypeGit && bundle.Spec.Source.Git != nil {
		if vol := sources.GetGitCredentialVolume(bundle.Spec.Source.Git.CredentialRef, bundle.Spec.Source.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			volumes = append(volumes, *vol)
		}
	}

	return volumes
}

// getResources returns resource requirements for the bundle create job.
// Kept as a receiver method for backward compatibility with tests.
func (handler *CreateHandler) getResources(bundle *udsv1alpha3.UDSBundleJob) corev1.ResourceRequirements {
	return getUDSResources(bundle.Spec.Resources, actions.BuildResourceRequirements)
}

// udsSourceCredentialVolumes returns the source credential volumes for a UDS bundle source.
// Returns (ociCredSecret, s3CredVol, gitCredVol).
func udsSourceCredentialVolumes(src udsv1alpha3.PackageSource) (string, *corev1.Volume, *corev1.Volume) {
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume

	if src.Type == udsv1alpha3.SourceTypeOCI && src.OCI != nil && src.OCI.CredentialRef != nil { // pragma: allowlist secret
		ociCredSecret = src.OCI.CredentialRef.Name // pragma: allowlist secret
	}

	if src.Type == udsv1alpha3.SourceTypeS3 && src.S3 != nil {
		if vol := sources.GetS3CredentialVolume(src.S3.CredentialRef); vol != nil { // pragma: allowlist secret
			s3CredVol = vol
		}
	}

	if src.Type == udsv1alpha3.SourceTypeGit && src.Git != nil {
		if vol := sources.GetGitCredentialVolume(src.Git.CredentialRef, src.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			gitCredVol = vol
		}
	}

	return ociCredSecret, s3CredVol, gitCredVol
}

// getUDSResources returns resource requirements, using spec-provided resources if set,
// or falling back to the provided default function.
func getUDSResources(specResources *corev1.ResourceRequirements, defaultFn func() corev1.ResourceRequirements) corev1.ResourceRequirements {
	if specResources != nil {
		return *specResources
	}
	return defaultFn()
}
