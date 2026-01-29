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
//
// Container images can be overridden via environment variables:
//   - FORGE_ZARF_CLI_IMAGE: Override the Zarf CLI image
//   - FORGE_UDS_CLI_IMAGE: Override the UDS CLI image
package constants

import (
	"os"
	"time"
)

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

	// DefaultZarfCLIImage is the default container image for Zarf CLI operations.
	DefaultZarfCLIImage = "ghcr.io/kylegalloway/forge/zarfpackagejob:v0.11.1"

	// DefaultUDSCLIImage is the default container image for UDS CLI operations.
	DefaultUDSCLIImage = "ghcr.io/kylegalloway/forge/udsbundlejob:v0.11.1"

	// VolumeNameWorkspace is the name of the workspace volume.
	VolumeNameWorkspace = "workspace"

	// VolumeNameOutput is the name of the output volume.
	VolumeNameOutput = "output"

	// VolumeNameArtifacts is the name of the artifacts volume.
	VolumeNameArtifacts = "artifacts"

	// VolumeNameAWSCredentials is the name of the AWS credentials volume.
	VolumeNameAWSCredentials = "aws-credentials"

	// VolumeNameGitCredentials is the name of the git credentials volume.
	VolumeNameGitCredentials = "git-creds"

	// VolumeMountPathGitCredentials is the mount path for the git credentials volume.
	VolumeMountPathGitCredentials = "/etc/git-secret" // #nosec G101 -- This is a path constant, not a credential // pragma: allowlist secret

	// VolumeNameRegistryCredentials is the name of the registry credentials volume for image pulls.
	VolumeNameRegistryCredentials = "registry-creds"

	// VolumeMountPathDockerConfig is the mount path for docker config (registry credentials).
	VolumeMountPathDockerConfig = "/home/zarf/.docker"

	// VolumeMountPathDockerConfigUDS is the mount path for docker config for UDS containers.
	VolumeMountPathDockerConfigUDS = "/home/uds/.docker"

	// VolumeMountPathWorkspace is the mount path for the workspace volume.
	VolumeMountPathWorkspace = "/workspace"

	// VolumeMountPathOutput is the mount path for the output volume.
	VolumeMountPathOutput = "/output"

	// VolumeMountPathArtifacts is the mount path for the artifacts volume.
	VolumeMountPathArtifacts = "/artifacts"

	// VolumeMountPathKubeconfig is the mount path for the kubeconfig volume.
	VolumeMountPathKubeconfig = "/etc/kubeconfig"

	// HomePathZarf is the home directory for Zarf containers (UID 1000).
	HomePathZarf = "/home/zarf"

	// HomePathUDS is the home directory for UDS containers (UID 65532).
	HomePathUDS = "/home/uds"

	// HomePathTmp is the home directory for init containers where no real home exists.
	HomePathTmp = "/tmp"

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

	// DefaultArtifactPath is the default path where built artifacts are stored.
	DefaultArtifactPath = "/workspace/package.tar.zst"

	// InClusterKubeconfigSetup is a shell script snippet that generates a kubeconfig
	// from the pod's service account token for in-cluster deployments.
	// This is needed because Zarf/UDS CLIs don't auto-detect in-cluster config.
	// The CA certificate is base64-encoded and embedded directly (certificate-authority-data)
	// to avoid file path issues. The token is read and trimmed of whitespace.
	// The API server endpoint is determined from environment variables
	// (KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT).
	InClusterKubeconfigSetup = `SA_DIR=/var/run/secrets/kubernetes.io/serviceaccount && ` +
		`mkdir -p /tmp/.kube && ` +
		`TOKEN=$(cat ${SA_DIR}/token | tr -d '\n') && ` +
		`CA_DATA=$(base64 -w0 ${SA_DIR}/ca.crt 2>/dev/null || base64 ${SA_DIR}/ca.crt | tr -d '\n') && ` +
		`API_SERVER="${KUBERNETES_SERVICE_HOST:-kubernetes.default.svc}:${KUBERNETES_SERVICE_PORT:-443}" && ` +
		`printf 'apiVersion: v1\nkind: Config\nclusters:\n- cluster:\n    certificate-authority-data: %s\n    server: https://%s\n  name: in-cluster\ncontexts:\n- context:\n    cluster: in-cluster\n    namespace: default\n    user: service-account\n  name: in-cluster\ncurrent-context: in-cluster\nusers:\n- name: service-account\n  user:\n    token: %s\n' "$CA_DATA" "$API_SERVER" "$TOKEN" > /tmp/.kube/config && ` +
		`export KUBECONFIG=/tmp/.kube/config && `
)

// ZarfCLIImage is the container image for Zarf CLI operations.
// It can be overridden via the FORGE_ZARF_CLI_IMAGE environment variable.
var ZarfCLIImage = getEnvOrDefault("FORGE_ZARF_CLI_IMAGE", DefaultZarfCLIImage)

// UDSCLIImage is the container image for UDS CLI operations.
// It can be overridden via the FORGE_UDS_CLI_IMAGE environment variable.
var UDSCLIImage = getEnvOrDefault("FORGE_UDS_CLI_IMAGE", DefaultUDSCLIImage)

// DebugMode controls whether jobs run in debug mode (sleep instead of command).
// When enabled, job pods run "sleep infinity" instead of actual commands,
// allowing users to exec into pods for debugging.
var DebugMode = getEnvOrDefault("FORGE_DEBUG_MODE", "false") == "true"

// getEnvOrDefault returns the value of the environment variable or the default value if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
