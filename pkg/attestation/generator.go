package attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"
)

const (
	// BuildTypeForgeZarfPackage is the build type for Forge Zarf packages
	BuildTypeForgeZarfPackage = "https://forge.dev/ZarfPackage@v1alpha1"

	// ForgeBuilderID identifies the Forge controller as the builder
	ForgeBuilderID = "https://forge.dev/controller@v1alpha1"
)

// Generator generates attestations for Forge operations
type Generator struct {
	// ControllerName is the name of the Forge controller
	ControllerName string

	// ControllerNamespace is the namespace of the Forge controller
	ControllerNamespace string

	// ControllerVersion is the version of Forge
	ControllerVersion string

	// PodName is the name of the controller pod generating attestations
	PodName string
}

// NewGenerator creates a new attestation generator
func NewGenerator(name, namespace, version string) *Generator {
	return &Generator{
		ControllerName:      name,
		ControllerNamespace: namespace,
		ControllerVersion:   version,
		PodName:             os.Getenv("POD_NAME"),
	}
}

// GenerateForBuild generates an attestation for a Build operation
func (generator *Generator) GenerateForBuild(opts BuildAttestationOptions) (*AttestationBundle, error) {
	klog.InfoS("Generating build attestation", "zarfPackageJob", opts.ZarfPackageJob, "namespace", opts.Namespace)

	// Create subject for the built artifact
	subjects := []Subject{}
	if opts.ArtifactPath != "" && opts.ArtifactDigest != "" {
		subjects = append(subjects, Subject{
			Name: opts.ArtifactPath,
			Digest: map[string]string{
				"sha256": opts.ArtifactDigest,
			},
		})
	}

	// Generate SLSA provenance
	provenance := generator.generateSLSAProvenance(opts.CommonOptions)

	// Create attestation statement
	statement := Statement{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       subjects,
		PredicateType: PredicateTypeSLSAProvenance,
		Predicate:     provenance,
	}

	// Also generate Forge operation predicate for additional metadata
	forgePredicate := generator.generateForgeOperationPredicate("Build", opts.CommonOptions)

	// Create bundle with both attestations
	bundle := &AttestationBundle{
		Statement: statement,
	}

	klog.InfoS("Build attestation generated", "subjects", len(subjects), "buildType", provenance.BuildDefinition.BuildType)

	// Store the Forge predicate as metadata (in a real implementation, this might be a separate statement)
	_ = forgePredicate // TODO: Store or attach this metadata

	return bundle, nil
}

// GenerateForPublish generates an attestation for a Publish operation
func (generator *Generator) GenerateForPublish(opts PublishAttestationOptions) (*AttestationBundle, error) {
	klog.InfoS("Generating publish attestation", "zarfPackageJob", opts.ZarfPackageJob, "namespace", opts.Namespace)

	// Create subject for the published artifact
	subjects := []Subject{}
	if opts.PublishedDigest != "" {
		subjects = append(subjects, Subject{
			Name: opts.PublishedLocation,
			Digest: map[string]string{
				"sha256": opts.PublishedDigest,
			},
		})
	}

	// Generate Forge operation predicate
	forgePredicate := generator.generateForgeOperationPredicate("Publish", opts.CommonOptions)
	if opts.Destination != nil {
		forgePredicate.Destination = opts.Destination
	}

	// Create attestation statement with Forge operation predicate
	statement := Statement{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       subjects,
		PredicateType: PredicateTypeForgeOperation,
		Predicate:     forgePredicate,
	}

	bundle := &AttestationBundle{
		Statement: statement,
	}

	klog.InfoS("Publish attestation generated", "location", opts.PublishedLocation)

	return bundle, nil
}

// GenerateForDeploy generates an attestation for a Deploy operation
func (generator *Generator) GenerateForDeploy(opts DeployAttestationOptions) (*AttestationBundle, error) {
	klog.InfoS("Generating deploy attestation", "zarfPackageJob", opts.ZarfPackageJob, "namespace", opts.Namespace)

	// Create subject for the deployed package
	subjects := []Subject{}
	if opts.PackageDigest != "" {
		subjects = append(subjects, Subject{
			Name: opts.PackageName,
			Digest: map[string]string{
				"sha256": opts.PackageDigest,
			},
		})
	}

	// Generate Forge operation predicate
	forgePredicate := generator.generateForgeOperationPredicate("Deploy", opts.CommonOptions)
	if opts.DeployTarget != nil {
		forgePredicate.DeployTarget = opts.DeployTarget
	}

	// Create attestation statement
	statement := Statement{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       subjects,
		PredicateType: PredicateTypeForgeOperation,
		Predicate:     forgePredicate,
	}

	bundle := &AttestationBundle{
		Statement: statement,
	}

	klog.InfoS("Deploy attestation generated", "target", opts.DeployTarget)

	return bundle, nil
}

// generateSLSAProvenance generates SLSA provenance metadata
func (generator *Generator) generateSLSAProvenance(opts CommonOptions) SLSAProvenance {
	// Build definition
	externalParams := map[string]interface{}{
		"zarfPackageJob": opts.ZarfPackageJob,
		"namespace":      opts.Namespace,
		"serviceAccount": opts.ServiceAccount,
	}

	// Add source information to external parameters
	if opts.Source != nil {
		externalParams["source"] = opts.Source
	}

	buildDef := BuildDefinition{
		BuildType:          BuildTypeForgeZarfPackage,
		ExternalParameters: externalParams,
		InternalParameters: map[string]interface{}{
			"jobName": opts.JobName,
		},
		ResolvedDependencies: []ResolvedDependency{},
	}

	// Add source as a resolved dependency
	if opts.Source != nil && opts.Source.Git != nil {
		buildDef.ResolvedDependencies = append(buildDef.ResolvedDependencies, ResolvedDependency{
			URI:  opts.Source.Git.URL,
			Name: "source-repository",
			Digest: map[string]string{
				"gitCommit": opts.Source.Git.CommitSHA,
			},
		})
	}

	// Run details
	builder := Builder{
		ID: ForgeBuilderID,
		Version: map[string]string{
			"forge": generator.ControllerVersion,
		},
	}

	metadata := BuildMetadata{
		InvocationID: opts.InvocationID,
		StartedOn:    &opts.StartTime,
		FinishedOn:   &opts.EndTime,
	}

	runDetails := RunDetails{
		Builder:  builder,
		Metadata: metadata,
	}

	return SLSAProvenance{
		BuildDefinition: buildDef,
		RunDetails:      runDetails,
	}
}

// generateForgeOperationPredicate generates a Forge operation predicate
func (generator *Generator) generateForgeOperationPredicate(operation string, opts CommonOptions) ForgeOperationPredicate {
	return ForgeOperationPredicate{
		Operation:      operation,
		ZarfPackageJob: opts.ZarfPackageJob,
		Namespace:      opts.Namespace,
		ServiceAccount: opts.ServiceAccount,
		Source:         opts.Source,
		StartTime:      opts.StartTime,
		EndTime:        opts.EndTime,
		Status:         opts.Status,
		JobName:        opts.JobName,
		PodName:        opts.PodName,
		Controller: ControllerInfo{
			Name:      generator.ControllerName,
			Namespace: generator.ControllerNamespace,
			Version:   generator.ControllerVersion,
			PodName:   generator.PodName,
		},
	}
}

// ComputeDigest computes a SHA256 digest of a file
func ComputeDigest(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// AttestationOptions contains common options for attestation generation
type CommonOptions struct {
	// ZarfPackageJob is the name of the ZarfPackageJob
	ZarfPackageJob string

	// Namespace is the Kubernetes namespace
	Namespace string

	// ServiceAccount is the ServiceAccount used
	ServiceAccount string

	// Source is the package source
	Source *SourceInfo

	// StartTime is when the operation started
	StartTime time.Time

	// EndTime is when the operation completed
	EndTime time.Time

	// Status is the operation status
	Status string

	// JobName is the Kubernetes Job name
	JobName string

	// PodName is the Kubernetes Pod name
	PodName string

	// InvocationID is a unique identifier for this operation
	InvocationID string
}

// BuildAttestationOptions contains options for build attestation
type BuildAttestationOptions struct {
	CommonOptions

	// ArtifactPath is the path to the built artifact
	ArtifactPath string

	// ArtifactDigest is the SHA256 digest of the artifact
	ArtifactDigest string
}

// PublishAttestationOptions contains options for publish attestation
type PublishAttestationOptions struct {
	CommonOptions

	// Destination is where the artifact was published
	Destination *DestinationInfo

	// PublishedLocation is the full location where published
	PublishedLocation string

	// PublishedDigest is the digest of the published artifact
	PublishedDigest string
}

// DeployAttestationOptions contains options for deploy attestation
type DeployAttestationOptions struct {
	CommonOptions

	// DeployTarget is where the package was deployed
	DeployTarget *DeployTargetInfo

	// PackageName is the name of the deployed package
	PackageName string

	// PackageDigest is the digest of the deployed package
	PackageDigest string
}
