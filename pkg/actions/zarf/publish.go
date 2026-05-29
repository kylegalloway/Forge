package zarf

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
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/destinations"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// PublishHandler handles Publish actions for ZarfPackageJob resources
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

// Execute performs a Publish action for the given ZarfPackageJob
func (handler *PublishHandler) Execute(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	artifactPath := opts.ArtifactPath
	artifactPVCName := opts.ArtifactPVCName
	klog.InfoS("Executing Zarf Package Publish action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	handler.metrics.RecordPackagePublishStarted(ctx, pkg.Namespace, pkg.Name)

	if pkg.Spec.Publish == nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("publish configuration is required for Publish action")
	}

	// For multi-action jobs, use glob pattern in the PVC
	if artifactPVCName != "" {
		artifactPath = constants.VolumeMountPathArtifacts + "/*.tar.zst"
	}

	destParams, err := destinations.DestinationParamsFromZarf(pkg)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to build destination params: %w", err)
	}

	publishCmd, err := destinations.GetPublishCommand(destParams, artifactPath)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to get publish command: %w", err)
	}

	jobConfig, err := destinations.GetJobConfiguration(destParams)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to get job configuration: %w", err)
	}

	// Init containers only for standalone publish (not chained)
	var initContainers []corev1.Container
	if artifactPVCName == "" {
		initContainers, err = buildZarfInitContainers(pkg)
		if err != nil {
			handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
			return nil, fmt.Errorf("failed to build init containers: %w", err)
		}
	}

	timeoutStr := ""
	var maxRetries *int32
	if pkg.Spec.Publish != nil {
		timeoutStr = pkg.Spec.Publish.Timeout
		if pkg.Spec.Publish.Retry != nil {
			maxRetries = pkg.Spec.Publish.Retry.MaxRetries
		}
	}

	var publishActionExtraMounts []apiscommon.ExtraMount
	if pkg.Spec.Publish != nil {
		publishActionExtraMounts = pkg.Spec.Publish.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(pkg.Spec.ExtraMounts, publishActionExtraMounts)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	// Source credential volumes only for standalone publish
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume
	if artifactPVCName == "" {
		ociCredSecret, s3CredVol, gitCredVol = zarfSourceCredentialVolumes(pkg.Spec.Source)
	}

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-publish", pkg.Name),
		Namespace:     pkg.Namespace,
		CLIImage:      constants.ZarfCLIImage,
		ContainerUID:  constants.DefaultZarfUID,
		ContainerName: constants.ContainerNameZarfPublish,
		Args:          []string{publishCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueZarf,
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  constants.ActionPublish,
		},
		OwnerRef:            pkg,
		OwnerGVK:            zarfv1alpha3.SchemeGroupVersion.WithKind("ZarfPackageJob"),
		NodeSelector:        pkg.Spec.NodeSelector,
		Affinity:            pkg.Spec.Affinity,
		Tolerations:         pkg.Spec.Tolerations,
		Resources:           getResources(pkg.Spec.Resources, actions.PublishResourceRequirements),
		MaxRetries:          maxRetries,
		Timeout:             actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultPublishTimeout),
		VolumeSizes:         pkg.Spec.VolumeSizes,
		ArtifactPVCName:     artifactPVCName,
		InitContainers:      initContainers,
		SourceOCICredSecret: ociCredSecret,
		SourceS3CredVol:     s3CredVol,
		SourceGitCredVol:    gitCredVol,
		DestJobConfig:       jobConfig,
		ExtraMounts:         extraMounts,
		ServiceAccountName:  pkg.Spec.ServiceAccountName,
		DebugMode:           actions.ShouldDebugAction(pkg.GetDebugMode() || constants.DebugMode, pkg.GetDebugActions(), constants.ActionPublish),
		KubeClient:          handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordPackagePublishFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create publish job: %w", err)
	}

	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "publish")
	klog.InfoS("Zarf package publish job created", "name", pkg.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Publish job %s created", job.Name)), nil
}
