// Package constants provides resource type names for Forge custom resources.
//
// These constants identify the different custom resource types managed by Forge:
//   - ZarfPackageJob: Jobs for building, publishing, and deploying Zarf packages
//   - UDSBundleJob: Jobs for creating, publishing, and deploying UDS bundles
package constants

const (
	// ResourceTypeZarfPackageJob is the resource type name for ZarfPackageJob resources.
	ResourceTypeZarfPackageJob = "ZarfPackageJob"

	// ResourceTypeUDSBundleJob is the resource type name for UDSBundleJob resources.
	ResourceTypeUDSBundleJob = "UDSBundleJob"
)
