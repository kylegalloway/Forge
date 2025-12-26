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
	// ActionBuild is the Zarf action name for building packages.
	ActionBuild = "build"

	// ActionPublish is the action name for publishing packages to registries (used by both Zarf and UDS).
	ActionPublish = "publish"

	// ActionDeploy is the action name for deploying packages to clusters (used by both Zarf and UDS).
	ActionDeploy = "deploy"

	// ActionCreate is the UDS action name for creating bundles.
	ActionCreate = "create"
)
