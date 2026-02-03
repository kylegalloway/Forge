// Package common provides shared interfaces used across Zarf and UDS APIs.
//
//nolint:revive // Package name "common" is intentionally generic for cross-cutting concerns
package common

// PackageResource defines the common interface for both ZarfPackageJob and UDSBundleJob.
// This abstraction allows unified controller and handler implementations using Go generics.
type PackageResource interface {
	// GetName returns the resource name
	GetName() string

	// GetNamespace returns the resource namespace
	GetNamespace() string

	// GetServiceAccountName returns the ServiceAccount to use for jobs
	GetServiceAccountName() string

	// GetAction returns the action to perform (e.g., "Build", "Create", "Publish")
	GetAction() string

	// GetUseArtifactPVC returns whether to create and attach a PVC for artifacts
	// Returns true by default if not specified
	GetUseArtifactPVC() bool

	// GetRetainArtifactPVC returns whether to retain the PVC after job completion
	// Returns true by default if not specified
	GetRetainArtifactPVC() bool

	// GetDebugMode returns whether debug mode is enabled for this job
	// When true, pods run in debug mode and have extended TTL
	GetDebugMode() bool

	// GetDebugActions returns the list of actions to run in debug mode
	// If empty and debugMode is true, all actions run in debug mode
	// If non-empty, only listed actions run in debug mode
	GetDebugActions() []string

	// GetVolumeSizes returns the configured volume sizes, or nil if not set
	GetVolumeSizes() *VolumeSizes
}
