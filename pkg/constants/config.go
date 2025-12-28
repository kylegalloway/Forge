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
	// JobMonitorInterval is how often to check Job statuses.
	JobMonitorInterval = 10 * time.Second

	// DefaultBuildTimeout is the default timeout for Zarf build operations (1 hour).
	DefaultBuildTimeout = 3600

	// DefaultPublishTimeout is the default timeout for publish operations (30 minutes).
	DefaultPublishTimeout = 1800

	// DefaultDeployTimeout is the default timeout for deploy operations (30 minutes).
	DefaultDeployTimeout = 1800

	// DefaultCreateTimeout is the default timeout for UDS bundle creation (1 hour).
	DefaultCreateTimeout = 3600

	// DefaultZarfUID is the UID for Zarf CLI containers (1000).
	DefaultZarfUID = 1000

	// DefaultUDSUID is the UID for UDS CLI containers (65532).
	DefaultUDSUID = 65532

	// ZarfCLIImage is the container image for Zarf CLI operations.
	ZarfCLIImage = "localhost/zarf:v0.68.1"

	// UDSCLIImage is the container image for UDS CLI operations.
	UDSCLIImage = "ghcr.io/defenseunicorns/uds-cli:v0.27.13"

	// VolumeNameWorkspace is the name of the workspace volume.
	VolumeNameWorkspace = "workspace"

	// VolumeNameOutput is the name of the output volume.
	VolumeNameOutput = "output"

	// VolumeNameArtifacts is the name of the artifacts volume.
	VolumeNameArtifacts = "artifacts"

	// VolumeMountPathWorkspace is the mount path for the workspace volume.
	VolumeMountPathWorkspace = "/workspace"

	// VolumeMountPathOutput is the mount path for the output volume.
	VolumeMountPathOutput = "/output"

	// VolumeMountPathArtifacts is the mount path for the artifacts volume.
	VolumeMountPathArtifacts = "/artifacts"

	// ContainerNameZarfBuild is the container name for Zarf build Jobs.
	ContainerNameZarfBuild = "zarf-build"

	// ContainerNameZarfPublish is the container name for Zarf publish Jobs.
	ContainerNameZarfPublish = "zarf-publish"

	// ContainerNameZarfDeploy is the container name for Zarf deploy Jobs.
	ContainerNameZarfDeploy = "zarf-deploy"

	// ContainerNameUDSCreate is the container name for UDS bundle create Jobs.
	ContainerNameUDSCreate = "uds-create"

	// ContainerNameUDSPublish is the container name for UDS bundle publish Jobs.
	ContainerNameUDSPublish = "uds-publish"

	// ContainerNameUDSDeploy is the container name for UDS bundle deploy Jobs.
	ContainerNameUDSDeploy = "uds-deploy"
)
