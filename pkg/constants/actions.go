// Package constants provides centralized string constants for Forge components.
//
// This package eliminates magic strings scattered throughout the codebase by providing
// typed constants for:
//   - Action names (build, publish, deploy, create)
//   - ServiceAccount annotation keys for policy enforcement
//   - Job and Pod label keys for resource identification
//   - API group versions for dynamic client operations
//   - Container images and configuration values
//
// Using these constants ensures consistency across controllers, handlers, webhooks,
// and monitoring systems.
package constants

const (
	// Zarf action names used in Job labels and monitoring
	ActionBuild = "build"

	// Shared action names used by both Zarf and UDS
	ActionPublish = "publish"
	ActionDeploy  = "deploy"

	// UDS bundle action names used in Job labels and monitoring
	ActionCreate = "create"
)
