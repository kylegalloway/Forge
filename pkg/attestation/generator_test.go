package attestation

import (
	"testing"
	"time"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("forge-controller", "forge-system", "v0.1.1")

	if gen.ControllerName != "forge-controller" {
		t.Errorf("expected controller name 'forge-controller', got '%s'", gen.ControllerName)
	}

	if gen.ControllerNamespace != "forge-system" {
		t.Errorf("expected controller namespace 'forge-system', got '%s'", gen.ControllerNamespace)
	}

	if gen.ControllerVersion != "v0.1.1" {
		t.Errorf("expected controller version 'v0.1.1', got '%s'", gen.ControllerVersion)
	}
}

func TestGenerateForBuild(t *testing.T) {
	gen := NewGenerator("forge-controller", "forge-system", "v0.1.1")

	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	opts := BuildAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: "test-build",
			Namespace:      "default",
			ServiceAccount: "test-sa",
			Source: &SourceInfo{
				Type: "Git",
				Git: &GitSourceInfo{
					URL:       "https://github.com/defenseunicorns/zarf",
					Ref:       "main",
					CommitSHA: "abc123def456", // pragma: allowlist secret
					Path:      "examples/dos-games",
				},
			},
			StartTime:    startTime,
			EndTime:      endTime,
			Status:       "Completed",
			JobName:      "test-build-job",
			PodName:      "test-build-pod",
			InvocationID: "inv-12345",
		},
		ArtifactPath:   "zarf-package-dos-games-amd64.tar.zst",
		ArtifactDigest: "sha256:abcdef1234567890",
	}

	bundle, err := gen.GenerateForBuild(opts)
	if err != nil {
		t.Fatalf("GenerateForBuild failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("expected bundle, got nil")
	}

	// Verify statement type
	if bundle.Statement.Type != "https://in-toto.io/Statement/v1" {
		t.Errorf("expected statement type 'https://in-toto.io/Statement/v1', got '%s'", bundle.Statement.Type)
	}

	// Verify predicate type
	if bundle.Statement.PredicateType != PredicateTypeSLSAProvenance {
		t.Errorf("expected predicate type '%s', got '%s'", PredicateTypeSLSAProvenance, bundle.Statement.PredicateType)
	}

	// Verify subject
	if len(bundle.Statement.Subject) != 1 {
		t.Errorf("expected 1 subject, got %d", len(bundle.Statement.Subject))
	}

	if len(bundle.Statement.Subject) > 0 {
		subject := bundle.Statement.Subject[0]
		if subject.Name != opts.ArtifactPath {
			t.Errorf("expected subject name '%s', got '%s'", opts.ArtifactPath, subject.Name)
		}

		if subject.Digest["sha256"] != opts.ArtifactDigest {
			t.Errorf("expected digest '%s', got '%s'", opts.ArtifactDigest, subject.Digest["sha256"])
		}
	}

	// Verify predicate is SLSA provenance
	provenance, ok := bundle.Statement.Predicate.(SLSAProvenance)
	if !ok {
		t.Fatalf("expected predicate to be SLSAProvenance, got %T", bundle.Statement.Predicate)
	}

	// Verify build definition
	if provenance.BuildDefinition.BuildType != BuildTypeForgeZarfPackage {
		t.Errorf("expected build type '%s', got '%s'", BuildTypeForgeZarfPackage, provenance.BuildDefinition.BuildType)
	}

	// Verify external parameters
	externalParams := provenance.BuildDefinition.ExternalParameters
	if externalParams["zarfPackageJob"] != opts.ZarfPackageJob {
		t.Errorf("expected zarfPackageJob '%s', got '%v'", opts.ZarfPackageJob, externalParams["zarfPackageJob"])
	}

	// Verify builder
	if provenance.RunDetails.Builder.ID != ForgeBuilderID {
		t.Errorf("expected builder ID '%s', got '%s'", ForgeBuilderID, provenance.RunDetails.Builder.ID)
	}

	// Verify metadata
	if provenance.RunDetails.Metadata.InvocationID != opts.InvocationID {
		t.Errorf("expected invocation ID '%s', got '%s'", opts.InvocationID, provenance.RunDetails.Metadata.InvocationID)
	}

	if provenance.RunDetails.Metadata.StartedOn == nil {
		t.Error("expected StartedOn to be set")
	}

	if provenance.RunDetails.Metadata.FinishedOn == nil {
		t.Error("expected FinishedOn to be set")
	}
}

func TestGenerateForPublish(t *testing.T) {
	gen := NewGenerator("forge-controller", "forge-system", "v0.1.1")

	startTime := time.Now().Add(-3 * time.Minute)
	endTime := time.Now()

	opts := PublishAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: "test-publish",
			Namespace:      "default",
			ServiceAccount: "test-sa",
			StartTime:      startTime,
			EndTime:        endTime,
			Status:         "Completed",
			JobName:        "test-publish-job",
			InvocationID:   "inv-67890",
		},
		Destination: &DestinationInfo{
			Type: "OCI",
			OCI: &OCIDestinationInfo{
				Registry:   "ghcr.io",
				Repository: "myorg/packages",
				Tag:        "v1.0.0",
				Digest:     "sha256:fedcba0987654321",
			},
		},
		PublishedLocation: "ghcr.io/myorg/packages:v1.0.0",
		PublishedDigest:   "sha256:fedcba0987654321",
	}

	bundle, err := gen.GenerateForPublish(opts)
	if err != nil {
		t.Fatalf("GenerateForPublish failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("expected bundle, got nil")
	}

	// Verify predicate type
	if bundle.Statement.PredicateType != PredicateTypeForgeOperation {
		t.Errorf("expected predicate type '%s', got '%s'", PredicateTypeForgeOperation, bundle.Statement.PredicateType)
	}

	// Verify subject
	if len(bundle.Statement.Subject) != 1 {
		t.Errorf("expected 1 subject, got %d", len(bundle.Statement.Subject))
	}

	// Verify predicate is Forge operation
	forgePredicate, ok := bundle.Statement.Predicate.(ForgeOperationPredicate)
	if !ok {
		t.Fatalf("expected predicate to be ForgeOperationPredicate, got %T", bundle.Statement.Predicate)
	}

	// Verify operation type
	if forgePredicate.Operation != "Publish" {
		t.Errorf("expected operation 'Publish', got '%s'", forgePredicate.Operation)
	}

	// Verify destination
	if forgePredicate.Destination == nil {
		t.Fatal("expected destination to be set")
	}

	if forgePredicate.Destination.Type != "OCI" {
		t.Errorf("expected destination type 'OCI', got '%s'", forgePredicate.Destination.Type)
	}

	if forgePredicate.Destination.OCI.Registry != opts.Destination.OCI.Registry {
		t.Errorf("expected registry '%s', got '%s'", opts.Destination.OCI.Registry, forgePredicate.Destination.OCI.Registry)
	}
}

func TestGenerateForDeploy(t *testing.T) {
	gen := NewGenerator("forge-controller", "forge-system", "v0.1.1")

	startTime := time.Now().Add(-10 * time.Minute)
	endTime := time.Now()

	opts := DeployAttestationOptions{
		CommonOptions: CommonOptions{
			ZarfPackageJob: "test-deploy",
			Namespace:      "default",
			ServiceAccount: "test-sa",
			StartTime:      startTime,
			EndTime:        endTime,
			Status:         "Completed",
			JobName:        "test-deploy-job",
			InvocationID:   "inv-11111",
		},
		DeployTarget: &DeployTargetInfo{
			Type:      "InCluster",
			Namespace: "production",
		},
		PackageName:   "dos-games",
		PackageDigest: "sha256:1234567890abcdef",
	}

	bundle, err := gen.GenerateForDeploy(opts)
	if err != nil {
		t.Fatalf("GenerateForDeploy failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("expected bundle, got nil")
	}

	// Verify predicate type
	if bundle.Statement.PredicateType != PredicateTypeForgeOperation {
		t.Errorf("expected predicate type '%s', got '%s'", PredicateTypeForgeOperation, bundle.Statement.PredicateType)
	}

	// Verify predicate is Forge operation
	forgePredicate, ok := bundle.Statement.Predicate.(ForgeOperationPredicate)
	if !ok {
		t.Fatalf("expected predicate to be ForgeOperationPredicate, got %T", bundle.Statement.Predicate)
	}

	// Verify operation type
	if forgePredicate.Operation != "Deploy" {
		t.Errorf("expected operation 'Deploy', got '%s'", forgePredicate.Operation)
	}

	// Verify deploy target
	if forgePredicate.DeployTarget == nil {
		t.Fatal("expected deploy target to be set")
	}

	if forgePredicate.DeployTarget.Type != "InCluster" {
		t.Errorf("expected deploy target type 'InCluster', got '%s'", forgePredicate.DeployTarget.Type)
	}

	if forgePredicate.DeployTarget.Namespace != "production" {
		t.Errorf("expected deploy namespace 'production', got '%s'", forgePredicate.DeployTarget.Namespace)
	}

	// Verify controller info
	if forgePredicate.Controller.Name != gen.ControllerName {
		t.Errorf("expected controller name '%s', got '%s'", gen.ControllerName, forgePredicate.Controller.Name)
	}

	if forgePredicate.Controller.Version != gen.ControllerVersion {
		t.Errorf("expected controller version '%s', got '%s'", gen.ControllerVersion, forgePredicate.Controller.Version)
	}
}

func TestGenerateSLSAProvenance(t *testing.T) {
	gen := NewGenerator("forge-controller", "forge-system", "v0.1.1")

	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	opts := CommonOptions{
		ZarfPackageJob: "test-job",
		Namespace:      "default",
		ServiceAccount: "test-sa",
		Source: &SourceInfo{
			Type: "Git",
			Git: &GitSourceInfo{
				URL:       "https://github.com/example/repo",
				Ref:       "v1.0.0",
				CommitSHA: "abc123",
			},
		},
		StartTime:    startTime,
		EndTime:      endTime,
		Status:       "Completed",
		JobName:      "test-job",
		InvocationID: "inv-test",
	}

	provenance := gen.generateSLSAProvenance(opts)

	// Verify build type
	if provenance.BuildDefinition.BuildType != BuildTypeForgeZarfPackage {
		t.Errorf("expected build type '%s', got '%s'", BuildTypeForgeZarfPackage, provenance.BuildDefinition.BuildType)
	}

	// Verify external parameters
	if provenance.BuildDefinition.ExternalParameters["zarfPackageJob"] != opts.ZarfPackageJob {
		t.Errorf("expected zarfPackageJob '%s', got '%v'", opts.ZarfPackageJob, provenance.BuildDefinition.ExternalParameters["zarfPackageJob"])
	}

	// Verify resolved dependencies
	if len(provenance.BuildDefinition.ResolvedDependencies) == 0 {
		t.Error("expected at least one resolved dependency")
	}

	if len(provenance.BuildDefinition.ResolvedDependencies) > 0 {
		dep := provenance.BuildDefinition.ResolvedDependencies[0]
		if dep.URI != opts.Source.Git.URL {
			t.Errorf("expected dependency URI '%s', got '%s'", opts.Source.Git.URL, dep.URI)
		}

		if dep.Digest["gitCommit"] != opts.Source.Git.CommitSHA {
			t.Errorf("expected git commit '%s', got '%s'", opts.Source.Git.CommitSHA, dep.Digest["gitCommit"])
		}
	}

	// Verify builder
	if provenance.RunDetails.Builder.ID != ForgeBuilderID {
		t.Errorf("expected builder ID '%s', got '%s'", ForgeBuilderID, provenance.RunDetails.Builder.ID)
	}

	if provenance.RunDetails.Builder.Version["forge"] != gen.ControllerVersion {
		t.Errorf("expected forge version '%s', got '%s'", gen.ControllerVersion, provenance.RunDetails.Builder.Version["forge"])
	}

	// Verify metadata timestamps
	if provenance.RunDetails.Metadata.StartedOn == nil {
		t.Error("expected StartedOn to be set")
	}

	if provenance.RunDetails.Metadata.FinishedOn == nil {
		t.Error("expected FinishedOn to be set")
	}

	if !provenance.RunDetails.Metadata.StartedOn.Equal(startTime) {
		t.Errorf("expected start time %v, got %v", startTime, provenance.RunDetails.Metadata.StartedOn)
	}
}
