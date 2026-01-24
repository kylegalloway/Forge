// Package constants provides image constants for init containers.
//
// These images are used for source fetching operations:
//   - ImageGitClone: Alpine-based Git for cloning repositories
//   - ImageAWSCLI: Amazon AWS CLI for S3 downloads
//   - ImageCrane: Google crane tool for OCI registry operations
//
// All images use pinned versions for security and reproducibility.
// Images can be overridden via environment variables.
package constants

import "os"

const (
	// DefaultImageGitClone is the default image for Git clone init containers.
	DefaultImageGitClone = "alpine/git:v2.43.0"

	// DefaultImageAWSCLI is the default image for AWS CLI init containers.
	DefaultImageAWSCLI = "amazon/aws-cli:2.15.0"

	// DefaultImageCrane is the default image for OCI pull init containers.
	DefaultImageCrane = "gcr.io/go-containerregistry/crane:v0.19.0"

	// DefaultImageBusybox is the default image for utility init containers.
	DefaultImageBusybox = "busybox:1.36"
)

// ImageGitClone is the container image for Git clone init containers.
// It can be overridden via the FORGE_GIT_CLONE_IMAGE environment variable.
var ImageGitClone = getImageEnvOrDefault("FORGE_GIT_CLONE_IMAGE", DefaultImageGitClone)

// ImageAWSCLI is the container image for AWS CLI init containers.
// It can be overridden via the FORGE_AWS_CLI_IMAGE environment variable.
var ImageAWSCLI = getImageEnvOrDefault("FORGE_AWS_CLI_IMAGE", DefaultImageAWSCLI)

// ImageCrane is the container image for OCI pull init containers.
// It can be overridden via the FORGE_CRANE_IMAGE environment variable.
var ImageCrane = getImageEnvOrDefault("FORGE_CRANE_IMAGE", DefaultImageCrane)

// ImageBusybox is the container image for utility init containers.
// It can be overridden via the FORGE_BUSYBOX_IMAGE environment variable.
var ImageBusybox = getImageEnvOrDefault("FORGE_BUSYBOX_IMAGE", DefaultImageBusybox)

// getImageEnvOrDefault returns the value of the environment variable or the default value if not set.
func getImageEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
