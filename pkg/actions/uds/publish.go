package uds

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	"github.com/kylegalloway/forge/pkg/actions/validation"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/destinations"
	"github.com/kylegalloway/forge/pkg/sources"
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
// opts.ArtifactPVCName enables multi-action job support (CreatePublish, etc.).
// When set, the PVC is mounted and opts.ArtifactPath specifies where to find the bundle.
// When empty, assumes standalone publish with bundle source in workspace or from spec.
func (handler *PublishHandler) Execute(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	artifactPath := opts.ArtifactPath
	artifactPVCName := opts.ArtifactPVCName
	klog.InfoS("Executing UDS Bundle Publish action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPath", artifactPath, "artifactPVC", artifactPVCName)

	handler.metrics.RecordBundlePublishStarted(ctx, bundle.Namespace, bundle.Name)

	if bundle.Spec.Publish == nil || bundle.Spec.Publish.Destination.Type == "" {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("publish destination is required for Publish action")
	}

	udsCmd, err := handler.buildPublishCommand(bundle, artifactPath)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to build publish command: %w", err)
	}

	jobConfig, err := destinations.GetUDSJobConfiguration(bundle)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	// Init containers only for standalone publish (not chained)
	var initContainers []corev1.Container
	if artifactPVCName == "" {
		initContainers, err = handler.buildInitContainers(bundle)
		if err != nil {
			handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
			return nil, fmt.Errorf("failed to build init containers: %w", err)
		}
	}

	timeoutStr := ""
	var maxRetries *int32
	if bundle.Spec.Publish != nil {
		timeoutStr = bundle.Spec.Publish.Timeout
		if bundle.Spec.Publish.Retry != nil {
			maxRetries = bundle.Spec.Publish.Retry.MaxRetries
		}
	}

	var publishActionExtraMounts []apiscommon.ExtraMount
	if bundle.Spec.Publish != nil {
		publishActionExtraMounts = bundle.Spec.Publish.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(bundle.Spec.ExtraMounts, publishActionExtraMounts)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	// Source credential volumes only for standalone publish
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume
	if artifactPVCName == "" {
		ociCredSecret, s3CredVol, gitCredVol = udsSourceCredentialVolumes(bundle.Spec.Source)
	}

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-publish", bundle.Name),
		Namespace:     bundle.Namespace,
		CLIImage:      constants.UDSCLIImage,
		ContainerUID:  constants.DefaultUDSUID,
		ContainerName: constants.ContainerNameUDSPublish,
		Args:          []string{udsCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueUDS,
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  constants.ActionPublish,
		},
		OwnerRef:            bundle,
		OwnerGVK:            udsv1alpha3.SchemeGroupVersion.WithKind("UDSBundleJob"),
		NodeSelector:        bundle.Spec.NodeSelector,
		Affinity:            bundle.Spec.Affinity,
		Tolerations:         bundle.Spec.Tolerations,
		Resources:           getUDSResources(bundle.Spec.Resources, actions.PublishResourceRequirements),
		MaxRetries:          maxRetries,
		Timeout:             actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultPublishTimeout),
		VolumeSizes:         bundle.Spec.VolumeSizes,
		ArtifactPVCName:     artifactPVCName,
		InitContainers:      initContainers,
		SourceOCICredSecret: ociCredSecret,
		SourceS3CredVol:     s3CredVol,
		SourceGitCredVol:    gitCredVol,
		DestJobConfig:       jobConfig,
		ExtraMounts:         extraMounts,
		ServiceAccountName:  bundle.Spec.ServiceAccountName,
		DebugMode:           actions.ShouldDebugAction(bundle.GetDebugMode() || constants.DebugMode, bundle.GetDebugActions(), constants.ActionPublish),
		KubeClient:          handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "publish")
	klog.InfoS("Bundle publish job created", "name", bundle.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Bundle publish job %s created", job.Name)), nil
}

// buildPublishCommand builds the UDS CLI publish command
func (handler *PublishHandler) buildPublishCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPath string) (string, error) {
	bundlePath := artifactPath
	if bundlePath == "" {
		bundlePath = constants.VolumeMountPathWorkspace + "/*.tar.zst"
	}

	return destinations.GetUDSPublishCommand(bundle, bundlePath)
}

// buildInitContainers creates init containers for artifact retrieval (for standalone publish)
func (handler *PublishHandler) buildInitContainers(bundle *udsv1alpha3.UDSBundleJob) ([]corev1.Container, error) {
	container, err := sources.GetUDSInitContainer(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to get init container: %w", err)
	}

	if container == nil {
		return nil, nil
	}

	return []corev1.Container{*container}, nil
}

// getResources returns resource requirements for the publish job.
// Kept as a receiver method for backward compatibility with tests.
func (handler *PublishHandler) getResources(bundle *udsv1alpha3.UDSBundleJob) corev1.ResourceRequirements {
	return getUDSResources(bundle.Spec.Resources, actions.PublishResourceRequirements)
}
