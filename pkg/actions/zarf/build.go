// Package zarf provides handlers for ZarfPackageJob actions (Build, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a Zarf package or UDS bundle.
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
	"github.com/kylegalloway/forge/pkg/sources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// BuildHandler handles Build actions for ZarfPackageJob resources
type BuildHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewBuildHandler creates a new BuildHandler
func NewBuildHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *BuildHandler {
	return &BuildHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Build action for the given ZarfPackageJob
//
// The opts.ArtifactPVCName field enables multi-action job support (BuildPublish, BuildDeploy, etc.)
// by providing a shared PersistentVolumeClaim for artifacts. When set, build outputs are stored
// in the PVC so subsequent actions (Publish/Deploy) can access them without re-building.
func (handler *BuildHandler) Execute(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	artifactPVCName := opts.ArtifactPVCName
	klog.InfoS("Executing Zarf Package Build action", "name", pkg.Name, "namespace", pkg.Namespace, "artifactPVC", artifactPVCName)

	handler.metrics.RecordPackageBuildStarted(ctx, pkg.Namespace, pkg.Name)

	if pkg.Spec.Source.Type == "" {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("source type is required for Build action")
	}

	zarfCmd, err := handler.buildZarfCommand(pkg, artifactPVCName)
	if err != nil {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to build zarf command: %w", err)
	}

	initContainers, err := buildZarfInitContainers(pkg)
	if err != nil {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to build init containers: %w", err)
	}

	timeoutStr := ""
	var maxRetries *int32
	if pkg.Spec.Build != nil {
		timeoutStr = pkg.Spec.Build.Timeout
		if pkg.Spec.Build.Retry != nil {
			maxRetries = pkg.Spec.Build.Retry.MaxRetries
		}
	}

	var regCredSecret, regCredMount string                                    // pragma: allowlist secret
	if pkg.Spec.Build != nil && pkg.Spec.Build.RegistryCredentialRef != nil { // pragma: allowlist secret
		regCredSecret = pkg.Spec.Build.RegistryCredentialRef.Name // pragma: allowlist secret
		regCredMount = constants.VolumeMountPathDockerConfig
	}

	var buildActionExtraMounts []apiscommon.ExtraMount
	if pkg.Spec.Build != nil {
		buildActionExtraMounts = pkg.Spec.Build.ExtraMounts
	}
	extraMounts, err := validation.MergeExtraMounts(pkg.Spec.ExtraMounts, buildActionExtraMounts)
	if err != nil {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("invalid extraMounts: %w", err)
	}

	ociCredSecret, s3CredVol, gitCredVol := zarfSourceCredentialVolumes(pkg.Spec.Source)

	params := actions.JobParams{
		JobName:       fmt.Sprintf("%s-build", pkg.Name),
		Namespace:     pkg.Namespace,
		CLIImage:      constants.ZarfCLIImage,
		ContainerUID:  constants.DefaultZarfUID,
		ContainerName: constants.ContainerNameZarfBuild,
		Args:          []string{zarfCmd},
		Labels: map[string]string{
			constants.LabelApp:     constants.LabelAppValueZarf,
			"resource-type":        "zarfpackagejob",
			constants.LabelPackage: pkg.Name,
			constants.LabelAction:  constants.ActionBuild,
		},
		OwnerRef:              pkg,
		OwnerGVK:              zarfv1alpha3.SchemeGroupVersion.WithKind("ZarfPackageJob"),
		NodeSelector:          pkg.Spec.NodeSelector,
		Affinity:              pkg.Spec.Affinity,
		Tolerations:           pkg.Spec.Tolerations,
		Resources:             getResources(pkg.Spec.Resources, actions.BuildResourceRequirements),
		MaxRetries:            maxRetries,
		Timeout:               actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultBuildTimeout),
		VolumeSizes:           pkg.Spec.VolumeSizes,
		ArtifactPVCName:       artifactPVCName,
		InitContainers:        initContainers,
		SourceOCICredSecret:   ociCredSecret,
		SourceS3CredVol:       s3CredVol,
		SourceGitCredVol:      gitCredVol,
		RegistryCredSecret:    regCredSecret, // pragma: allowlist secret
		RegistryCredMountPath: regCredMount,
		ExtraMounts:           extraMounts,
		ServiceAccountName:    pkg.Spec.ServiceAccountName,
		DebugMode:             actions.ShouldDebugAction(pkg.GetDebugMode() || constants.DebugMode, pkg.GetDebugActions(), constants.ActionBuild),
		KubeClient:            handler.kubeClient,
	}

	job, err := actions.BuildActionJob(ctx, params)
	if err != nil {
		handler.metrics.RecordPackageBuildFailed(ctx, pkg.Namespace, pkg.Name)
		return nil, fmt.Errorf("failed to create build job: %w", err)
	}

	handler.metrics.RecordJobCreated(ctx, pkg.Namespace, pkg.Name, "build")
	klog.InfoS("Zarf package build job created", "name", pkg.Name, "job", job.Name)

	return actions.ActionResultFromJob(job, fmt.Sprintf("Build job %s created", job.Name)), nil
}

// buildZarfCommand builds the zarf CLI command based on package source.
// The working directory is always the workspace volume mount; that default is
// absorbed by BuildActionJob and does not need to be returned here.
func (handler *BuildHandler) buildZarfCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPVCName string) (string, error) {
	var cmd string
	if artifactPVCName != "" {
		cmd = "zarf package create . --confirm --output-directory " + constants.VolumeMountPathArtifacts
	} else {
		cmd = "zarf package create . --confirm --output-directory " + constants.VolumeMountPathOutput
	}

	if pkg.Spec.Build != nil {
		build := pkg.Spec.Build

		if build.Flavor != "" {
			cmd = fmt.Sprintf("%s --flavor %s", cmd, build.Flavor)
		}
		if build.Architecture != "" {
			cmd = fmt.Sprintf("%s --architecture %s", cmd, build.Architecture)
		}
		if build.SkipSBOM {
			cmd = fmt.Sprintf("%s --skip-sbom", cmd)
		}

		for key, value := range build.Variables {
			cmd = fmt.Sprintf("%s --set %s=%s", cmd, key, value)
		}

		if len(build.ExtraArgs) > 0 {
			var err error
			cmd, err = validation.AppendExtraArgs(cmd, build.ExtraArgs)
			if err != nil {
				return "", fmt.Errorf("invalid extraArgs: %w", err)
			}
		}
	}

	return cmd, nil
}

// buildZarfInitContainers creates init containers for Zarf source artifact retrieval
func buildZarfInitContainers(pkg *zarfv1alpha3.ZarfPackageJob) ([]corev1.Container, error) {
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

// zarfSourceCredentialVolumes returns the source credential volumes for a Zarf package source.
// Returns (ociCredSecret, s3CredVol, gitCredVol).
func zarfSourceCredentialVolumes(src zarfv1alpha3.PackageSource) (string, *corev1.Volume, *corev1.Volume) {
	var ociCredSecret string
	var s3CredVol, gitCredVol *corev1.Volume

	if src.Type == zarfv1alpha3.SourceTypeOCI && src.OCI != nil && src.OCI.CredentialRef != nil { // pragma: allowlist secret
		ociCredSecret = src.OCI.CredentialRef.Name // pragma: allowlist secret
	}

	if src.Type == zarfv1alpha3.SourceTypeS3 && src.S3 != nil {
		if vol := sources.GetS3CredentialVolume(src.S3.CredentialRef); vol != nil { // pragma: allowlist secret
			s3CredVol = vol
		}
	}

	if src.Type == zarfv1alpha3.SourceTypeGit && src.Git != nil {
		if vol := sources.GetGitCredentialVolume(src.Git.CredentialRef, src.Git.DisableCloneCredentials); vol != nil { // pragma: allowlist secret
			gitCredVol = vol
		}
	}

	return ociCredSecret, s3CredVol, gitCredVol
}

// getResources returns resource requirements, using spec-provided resources if set,
// or falling back to the provided default function.
func getResources(specResources *corev1.ResourceRequirements, defaultFn func() corev1.ResourceRequirements) corev1.ResourceRequirements {
	if specResources != nil {
		return *specResources
	}
	return defaultFn()
}
