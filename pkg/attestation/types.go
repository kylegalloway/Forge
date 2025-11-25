package attestation

import (
	"time"
)

// PredicateType defines the type of attestation predicate
type PredicateType string

const (
	// PredicateTypeSLSAProvenance is the SLSA provenance predicate type
	// https://slsa.dev/provenance/v1
	PredicateTypeSLSAProvenance PredicateType = "https://slsa.dev/provenance/v1"

	// PredicateTypeInTotoBuild is the in-toto build predicate type
	PredicateTypeInTotoBuild PredicateType = "https://in-toto.io/attestation/build/v0.1"

	// PredicateTypeForgeOperation is a custom predicate for Forge operations
	PredicateTypeForgeOperation PredicateType = "https://forge.dev/attestation/operation/v1alpha1"
)

// Statement represents an in-toto attestation statement
// https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md
type Statement struct {
	// Type is always "https://in-toto.io/Statement/v1"
	Type string `json:"_type"`

	// Subject identifies the artifact(s) this statement applies to
	Subject []Subject `json:"subject"`

	// PredicateType identifies the type of predicate
	PredicateType PredicateType `json:"predicateType"`

	// Predicate contains the attestation metadata
	Predicate interface{} `json:"predicate"`
}

// Subject identifies an artifact by its digest
type Subject struct {
	// Name is a human-readable identifier (e.g., package name)
	Name string `json:"name"`

	// Digest is a collection of cryptographic digests
	Digest map[string]string `json:"digest"`
}

// SLSAProvenance represents SLSA provenance metadata
// https://slsa.dev/provenance/v1
type SLSAProvenance struct {
	// BuildDefinition describes how the build was performed
	BuildDefinition BuildDefinition `json:"buildDefinition"`

	// RunDetails provides information about the build execution
	RunDetails RunDetails `json:"runDetails"`
}

// BuildDefinition describes the build
type BuildDefinition struct {
	// BuildType identifies the build system
	BuildType string `json:"buildType"`

	// ExternalParameters are the top-level inputs to the build
	ExternalParameters map[string]interface{} `json:"externalParameters"`

	// InternalParameters are parameters set by the build system
	InternalParameters map[string]interface{} `json:"internalParameters,omitempty"`

	// ResolvedDependencies are the resolved build dependencies
	ResolvedDependencies []ResolvedDependency `json:"resolvedDependencies,omitempty"`
}

// ResolvedDependency represents a resolved build dependency
type ResolvedDependency struct {
	// URI uniquely identifies the dependency
	URI string `json:"uri"`

	// Digest is the cryptographic digest of the dependency
	Digest map[string]string `json:"digest,omitempty"`

	// Name is a human-readable name
	Name string `json:"name,omitempty"`

	// DownloadLocation is where the dependency was obtained
	DownloadLocation string `json:"downloadLocation,omitempty"`
}

// RunDetails provides information about the build execution
type RunDetails struct {
	// Builder identifies the build system that executed the build
	Builder Builder `json:"builder"`

	// Metadata contains additional metadata about the build
	Metadata BuildMetadata `json:"metadata"`

	// Byproducts are additional artifacts produced during the build
	Byproducts []Subject `json:"byproducts,omitempty"`
}

// Builder identifies the build system
type Builder struct {
	// ID is the unique identifier for the builder
	ID string `json:"id"`

	// Version is the version of the builder
	Version map[string]string `json:"version,omitempty"`

	// BuilderDependencies are dependencies of the builder itself
	BuilderDependencies []ResolvedDependency `json:"builderDependencies,omitempty"`
}

// BuildMetadata contains additional build metadata
type BuildMetadata struct {
	// InvocationID is a unique identifier for this build execution
	InvocationID string `json:"invocationId"`

	// StartedOn is when the build started
	StartedOn *time.Time `json:"startedOn,omitempty"`

	// FinishedOn is when the build finished
	FinishedOn *time.Time `json:"finishedOn,omitempty"`
}

// ForgeOperationPredicate is a custom predicate for Forge operations
type ForgeOperationPredicate struct {
	// Operation is the type of operation (Build, Publish, Deploy)
	Operation string `json:"operation"`

	// ZarfPackageJob is the name of the ZarfPackageJob resource
	ZarfPackageJob string `json:"zarfPackageJob"`

	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace"`

	// ServiceAccount is the ServiceAccount used
	ServiceAccount string `json:"serviceAccount"`

	// Source describes where the package came from
	Source *SourceInfo `json:"source,omitempty"`

	// Destination describes where the package was published
	Destination *DestinationInfo `json:"destination,omitempty"`

	// DeployTarget describes where the package was deployed
	DeployTarget *DeployTargetInfo `json:"deployTarget,omitempty"`

	// StartTime is when the operation started
	StartTime time.Time `json:"startTime"`

	// EndTime is when the operation completed
	EndTime time.Time `json:"endTime"`

	// Status is the final status (Completed, Failed)
	Status string `json:"status"`

	// JobName is the Kubernetes Job that executed the operation
	JobName string `json:"jobName"`

	// PodName is the Kubernetes Pod that executed the operation
	PodName string `json:"podName,omitempty"`

	// Controller identifies the Forge controller instance
	Controller ControllerInfo `json:"controller"`
}

// SourceInfo describes the package source
type SourceInfo struct {
	// Type is the source type (Git, S3, OCI, Local)
	Type string `json:"type"`

	// Git contains Git source information
	Git *GitSourceInfo `json:"git,omitempty"`

	// S3 contains S3 source information
	S3 *S3SourceInfo `json:"s3,omitempty"`

	// OCI contains OCI source information
	OCI *OCISourceInfo `json:"oci,omitempty"`
}

// GitSourceInfo contains Git-specific source information
type GitSourceInfo struct {
	// URL is the Git repository URL
	URL string `json:"url"`

	// Ref is the Git reference (branch, tag, commit)
	Ref string `json:"ref"`

	// CommitSHA is the resolved commit SHA
	CommitSHA string `json:"commitSha,omitempty"`

	// Path is the path within the repository
	Path string `json:"path,omitempty"`
}

// S3SourceInfo contains S3-specific source information
type S3SourceInfo struct {
	// Bucket is the S3 bucket name
	Bucket string `json:"bucket"`

	// Key is the S3 object key
	Key string `json:"key"`

	// Region is the AWS region
	Region string `json:"region,omitempty"`

	// VersionID is the S3 object version
	VersionID string `json:"versionId,omitempty"`
}

// OCISourceInfo contains OCI-specific source information
type OCISourceInfo struct {
	// Registry is the OCI registry
	Registry string `json:"registry"`

	// Repository is the repository path
	Repository string `json:"repository"`

	// Tag is the image tag
	Tag string `json:"tag,omitempty"`

	// Digest is the image digest
	Digest string `json:"digest,omitempty"`
}

// DestinationInfo describes the publish destination
type DestinationInfo struct {
	// Type is the destination type (S3, OCI, Local)
	Type string `json:"type"`

	// S3 contains S3 destination information
	S3 *S3DestinationInfo `json:"s3,omitempty"`

	// OCI contains OCI destination information
	OCI *OCIDestinationInfo `json:"oci,omitempty"`
}

// S3DestinationInfo contains S3 destination information
type S3DestinationInfo struct {
	// Bucket is the S3 bucket name
	Bucket string `json:"bucket"`

	// Key is the S3 object key where the package was published
	Key string `json:"key"`

	// Region is the AWS region
	Region string `json:"region,omitempty"`

	// VersionID is the S3 object version
	VersionID string `json:"versionId,omitempty"`
}

// OCIDestinationInfo contains OCI destination information
type OCIDestinationInfo struct {
	// Registry is the OCI registry
	Registry string `json:"registry"`

	// Repository is the repository path
	Repository string `json:"repository"`

	// Tag is the image tag
	Tag string `json:"tag,omitempty"`

	// Digest is the image digest
	Digest string `json:"digest"`
}

// DeployTargetInfo describes the deploy target
type DeployTargetInfo struct {
	// Type is the target type (InCluster, ExternalCluster)
	Type string `json:"type"`

	// Namespace is the target namespace
	Namespace string `json:"namespace"`

	// ClusterName is a human-readable cluster identifier
	ClusterName string `json:"clusterName,omitempty"`

	// ClusterEndpoint is the Kubernetes API endpoint
	ClusterEndpoint string `json:"clusterEndpoint,omitempty"`
}

// ControllerInfo identifies the Forge controller
type ControllerInfo struct {
	// Name is the controller deployment name
	Name string `json:"name"`

	// Namespace is the controller namespace
	Namespace string `json:"namespace"`

	// Version is the Forge version
	Version string `json:"version"`

	// PodName is the controller pod that created this attestation
	PodName string `json:"podName,omitempty"`
}

// AttestationBundle contains an attestation and optional signature
type AttestationBundle struct {
	// Statement is the attestation statement
	Statement Statement `json:"statement"`

	// Signature is the digital signature (optional, added by signing process)
	Signature *Signature `json:"signature,omitempty"`

	// Envelope is the DSSE envelope (if using DSSE)
	Envelope *DSSEEnvelope `json:"envelope,omitempty"`
}

// Signature represents a digital signature
type Signature struct {
	// KeyID identifies the signing key
	KeyID string `json:"keyid"`

	// Signature is the base64-encoded signature
	Signature string `json:"sig"`
}

// DSSEEnvelope is a Dead Simple Signing Envelope
// https://github.com/secure-systems-lab/dsse
type DSSEEnvelope struct {
	// PayloadType identifies the payload type
	PayloadType string `json:"payloadType"`

	// Payload is the base64-encoded payload
	Payload string `json:"payload"`

	// Signatures contains one or more signatures
	Signatures []Signature `json:"signatures"`
}
