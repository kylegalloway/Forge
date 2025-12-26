// Package constants defines operational configuration values for Forge.
//
// This includes:
//   - Job monitoring intervals and timeout defaults
//   - Security context UIDs for Zarf and UDS containers
//   - Container images for CLI operations
//
// Timeout values are used as defaults when users don't specify custom timeouts
// in their ZarfPackageJob or UDSBundleJob specs. These can be overridden per-job
// via the Build.Timeout, Publish.Timeout, or Deploy.Timeout fields.
//
// UID constants ensure Jobs run with predictable non-root users for security:
//   - DefaultZarfUID (1000): Standard user UID for Zarf CLI containers
//   - DefaultUDSUID (65532): Higher UID for UDS CLI containers to avoid conflicts
package constants

import "time"

const (
	// JobMonitorInterval is how often to check Job statuses
	JobMonitorInterval = 10 * time.Second

	// Default timeout values (in seconds)
	DefaultBuildTimeout   = 3600 // 1 hour
	DefaultPublishTimeout = 1800 // 30 minutes
	DefaultDeployTimeout  = 1800 // 30 minutes
	DefaultCreateTimeout  = 3600 // 1 hour for UDS bundle creation

	// Security context UIDs
	DefaultZarfUID = 1000
	DefaultUDSUID  = 65532

	// CLI container images
	ZarfCLIImage = "localhost/zarf:v0.66.0"
	UDSCLIImage  = "ghcr.io/defenseunicorns/uds-cli:latest"

	// Volume names for Job containers
	VolumeNameWorkspace = "workspace"
	VolumeNameOutput    = "output"
	VolumeNameArtifacts = "artifacts"

	// Volume mount paths
	VolumeMountPathWorkspace = "/workspace"
	VolumeMountPathOutput    = "/output"
	VolumeMountPathArtifacts = "/artifacts"

	// Container names for Zarf Jobs
	ContainerNameZarfBuild   = "zarf-build"
	ContainerNameZarfPublish = "zarf-publish"
	ContainerNameZarfDeploy  = "zarf-deploy"

	// Container names for UDS Bundle Jobs
	ContainerNameUDSCreate  = "uds-create"
	ContainerNameUDSPublish = "uds-publish"
	ContainerNameUDSDeploy  = "uds-deploy"
)
