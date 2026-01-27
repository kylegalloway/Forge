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
	// ActionBuild is the Zarf action name for building packages (used in job labels).
	ActionBuild = "build"

	// ActionPublish is the action name for publishing packages to registries (used by both Zarf and UDS, in job labels).
	ActionPublish = "publish"

	// ActionDeploy is the action name for deploying packages to clusters (used by both Zarf and UDS, in job labels).
	ActionDeploy = "deploy"

	// ActionCreate is the UDS action name for creating bundles (used in job labels).
	ActionCreate = "create"
)

// Spec action constants (PascalCase) - these match the CRD enum values in spec.action.
// Used for comparing against user-specified actions in the resource spec.
const (
	// SpecActionBuild is the Zarf spec action for building packages.
	SpecActionBuild = "Build"

	// SpecActionPublish is the spec action for publishing packages.
	SpecActionPublish = "Publish"

	// SpecActionDeploy is the spec action for deploying packages.
	SpecActionDeploy = "Deploy"

	// SpecActionCreate is the UDS spec action for creating bundles.
	SpecActionCreate = "Create"

	// SpecActionBuildPublish is the Zarf compound action: build then publish.
	SpecActionBuildPublish = "BuildPublish"

	// SpecActionBuildDeploy is the Zarf compound action: build then deploy.
	SpecActionBuildDeploy = "BuildDeploy"

	// SpecActionBuildPublishDeploy is the Zarf compound action: build, publish, then deploy.
	SpecActionBuildPublishDeploy = "BuildPublishDeploy"

	// SpecActionCreatePublish is the UDS compound action: create then publish.
	SpecActionCreatePublish = "CreatePublish"

	// SpecActionCreateDeploy is the UDS compound action: create then deploy.
	SpecActionCreateDeploy = "CreateDeploy"

	// SpecActionCreatePublishDeploy is the UDS compound action: create, publish, then deploy.
	SpecActionCreatePublishDeploy = "CreatePublishDeploy"

	// SpecActionPublishDeploy is the compound action: publish then deploy (both Zarf and UDS).
	SpecActionPublishDeploy = "PublishDeploy"
)
