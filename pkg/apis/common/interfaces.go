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
}
