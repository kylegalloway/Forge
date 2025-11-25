package attestation

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

// ControllerIntegration provides attestation integration for the controller
// This is a reference implementation showing how to integrate attestation
// generation into the controller's reconciliation loop
type ControllerIntegration struct {
	// Generator generates attestations
	Generator *Generator

	// Storage stores attestations
	Storage Storage

	// Enabled indicates if attestation is enabled globally
	Enabled bool
}

// NewControllerIntegration creates a new controller integration
func NewControllerIntegration(gen *Generator, storage Storage) *ControllerIntegration {
	return &ControllerIntegration{
		Generator: gen,
		Storage:   storage,
		Enabled:   true,
	}
}

// OnBuildComplete is called when a build operation completes
// This should be integrated into the controller after a successful build
func (ci *ControllerIntegration) OnBuildComplete(ctx context.Context, opts BuildCompletionOptions) error {
	if !ci.Enabled {
		return nil
	}

	// Check if attestation is requested
	if !ShouldGenerateAttestation(opts.Annotations) {
		klog.V(2).InfoS("Attestation not requested, skipping",
			"zarfPackageJob", opts.ZarfPackageJob,
		)
		return nil
	}

	klog.InfoS("Generating build attestation",
		"zarfPackageJob", opts.ZarfPackageJob,
		"namespace", opts.Namespace,
	)

	// Generate attestation
	attestationOpts := BuildAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: opts.ZarfPackageJob,
			Namespace:      opts.Namespace,
			ServiceAccount: opts.ServiceAccount,
			Source:         opts.Source,
			StartTime:      opts.StartTime,
			EndTime:        opts.EndTime,
			Status:         "Completed",
			JobName:        opts.JobName,
			PodName:        opts.PodName,
			InvocationID:   GenerateInvocationID(opts.Namespace, opts.ZarfPackageJob, opts.StartTime),
		},
		ArtifactPath:   opts.ArtifactPath,
		ArtifactDigest: opts.ArtifactDigest,
	}

	bundle, err := ci.Generator.GenerateForBuild(attestationOpts)
	if err != nil {
		return fmt.Errorf("failed to generate build attestation: %w", err)
	}

	// Validate attestation
	if err := ValidateAttestationBundle(bundle); err != nil {
		return fmt.Errorf("attestation validation failed: %w", err)
	}

	// Store attestation
	storeOpts := StoreOptions{
		ZarfPackageJob: opts.ZarfPackageJob,
		Namespace:      opts.Namespace,
		Operation:      "Build",
		ArtifactDigest: opts.ArtifactDigest,
	}

	if err := ci.Storage.Store(ctx, bundle, storeOpts); err != nil {
		return fmt.Errorf("failed to store attestation: %w", err)
	}

	klog.InfoS("Build attestation generated and stored",
		"zarfPackageJob", opts.ZarfPackageJob,
	)

	return nil
}

// OnPublishComplete is called when a publish operation completes
func (ci *ControllerIntegration) OnPublishComplete(ctx context.Context, opts PublishCompletionOptions) error {
	if !ci.Enabled {
		return nil
	}

	if !ShouldGenerateAttestation(opts.Annotations) {
		return nil
	}

	klog.InfoS("Generating publish attestation",
		"zarfPackageJob", opts.ZarfPackageJob,
		"location", opts.PublishedLocation,
	)

	attestationOpts := PublishAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: opts.ZarfPackageJob,
			Namespace:      opts.Namespace,
			ServiceAccount: opts.ServiceAccount,
			Source:         opts.Source,
			StartTime:      opts.StartTime,
			EndTime:        opts.EndTime,
			Status:         "Completed",
			JobName:        opts.JobName,
			InvocationID:   GenerateInvocationID(opts.Namespace, opts.ZarfPackageJob, opts.StartTime),
		},
		Destination:       opts.Destination,
		PublishedLocation: opts.PublishedLocation,
		PublishedDigest:   opts.PublishedDigest,
	}

	bundle, err := ci.Generator.GenerateForPublish(attestationOpts)
	if err != nil {
		return fmt.Errorf("failed to generate publish attestation: %w", err)
	}

	storeOpts := StoreOptions{
		ZarfPackageJob: opts.ZarfPackageJob,
		Namespace:      opts.Namespace,
		Operation:      "Publish",
		ArtifactDigest: opts.PublishedDigest,
	}

	if err := ci.Storage.Store(ctx, bundle, storeOpts); err != nil {
		return fmt.Errorf("failed to store attestation: %w", err)
	}

	klog.InfoS("Publish attestation generated and stored",
		"zarfPackageJob", opts.ZarfPackageJob,
	)

	return nil
}

// OnDeployComplete is called when a deploy operation completes
func (ci *ControllerIntegration) OnDeployComplete(ctx context.Context, opts DeployCompletionOptions) error {
	if !ci.Enabled {
		return nil
	}

	if !ShouldGenerateAttestation(opts.Annotations) {
		return nil
	}

	klog.InfoS("Generating deploy attestation",
		"zarfPackageJob", opts.ZarfPackageJob,
		"target", opts.DeployTarget.Type,
	)

	attestationOpts := DeployAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: opts.ZarfPackageJob,
			Namespace:      opts.Namespace,
			ServiceAccount: opts.ServiceAccount,
			Source:         opts.Source,
			StartTime:      opts.StartTime,
			EndTime:        opts.EndTime,
			Status:         "Completed",
			JobName:        opts.JobName,
			InvocationID:   GenerateInvocationID(opts.Namespace, opts.ZarfPackageJob, opts.StartTime),
		},
		DeployTarget:  opts.DeployTarget,
		PackageName:   opts.PackageName,
		PackageDigest: opts.PackageDigest,
	}

	bundle, err := ci.Generator.GenerateForDeploy(attestationOpts)
	if err != nil {
		return fmt.Errorf("failed to generate deploy attestation: %w", err)
	}

	storeOpts := StoreOptions{
		ZarfPackageJob: opts.ZarfPackageJob,
		Namespace:      opts.Namespace,
		Operation:      "Deploy",
		ArtifactDigest: opts.PackageDigest,
	}

	if err := ci.Storage.Store(ctx, bundle, storeOpts); err != nil {
		return fmt.Errorf("failed to store attestation: %w", err)
	}

	klog.InfoS("Deploy attestation generated and stored",
		"zarfPackageJob", opts.ZarfPackageJob,
	)

	return nil
}

// BuildCompletionOptions contains information about a completed build
type BuildCompletionOptions struct {
	ZarfPackageJob string
	Namespace      string
	ServiceAccount string
	Annotations    map[string]string
	Source         *SourceInfo
	StartTime      time.Time
	EndTime        time.Time
	JobName        string
	PodName        string
	ArtifactPath   string
	ArtifactDigest string
}

// PublishCompletionOptions contains information about a completed publish
type PublishCompletionOptions struct {
	ZarfPackageJob    string
	Namespace         string
	ServiceAccount    string
	Annotations       map[string]string
	Source            *SourceInfo
	Destination       *DestinationInfo
	StartTime         time.Time
	EndTime           time.Time
	JobName           string
	PublishedLocation string
	PublishedDigest   string
}

// DeployCompletionOptions contains information about a completed deploy
type DeployCompletionOptions struct {
	ZarfPackageJob string
	Namespace      string
	ServiceAccount string
	Annotations    map[string]string
	Source         *SourceInfo
	DeployTarget   *DeployTargetInfo
	StartTime      time.Time
	EndTime        time.Time
	JobName        string
	PackageName    string
	PackageDigest  string
}

// Example controller integration:
//
// In the controller's reconciliation loop, after a successful operation:
//
//	// After build completes
//	if attestationEnabled {
//		err := attestationIntegration.OnBuildComplete(ctx, attestation.BuildCompletionOptions{
//			ZarfPackageJob: zpj.Name,
//			Namespace:      zpj.Namespace,
//			ServiceAccount: zpj.Spec.ServiceAccountName,
//			Annotations:    zpj.Annotations,
//			Source:         extractSourceInfo(zpj),
//			StartTime:      startTime,
//			EndTime:        time.Now(),
//			JobName:        job.Name,
//			ArtifactPath:   artifactPath,
//			ArtifactDigest: artifactDigest,
//		})
//		if err != nil {
//			// Handle error - don't fail the operation, just log
//			klog.ErrorS(err, "Failed to generate attestation")
//		}
//	}
