// Package constants defines API group, version, and resource identifiers for Forge CRDs.
//
// These constants are used by:
//   - Dynamic Kubernetes clients for resource discovery
//   - Controllers to watch and reconcile custom resources
//   - Webhooks for admission control
//
// GroupVersionResource (GVR) values are constructed from these constants and used
// throughout the codebase to interact with ZarfPackageJob and UDSBundleJob resources
// via the Kubernetes dynamic client.
package constants

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	// APIGroup is the API group for all Forge resources
	APIGroup = "forge.dev"
	// APIVersion is the API version for all Forge resources
	APIVersion = "v1alpha1"

	// ZarfPackageJobResource is the resource name for ZarfPackageJob
	ZarfPackageJobResource = "zarfpackagejobs"
	// UDSBundleJobResource is the resource name for UDSBundleJob
	UDSBundleJobResource = "udsbundlejobs"
)

var (
	// ZarfPackageJobGVR is the GroupVersionResource for ZarfPackageJob
	ZarfPackageJobGVR = schema.GroupVersionResource{
		Group:    APIGroup,
		Version:  APIVersion,
		Resource: ZarfPackageJobResource,
	}

	// UDSBundleJobGVR is the GroupVersionResource for UDSBundleJob
	UDSBundleJobGVR = schema.GroupVersionResource{
		Group:    APIGroup,
		Version:  APIVersion,
		Resource: UDSBundleJobResource,
	}
)
