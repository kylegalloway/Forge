// Package constants defines shared constants used across Forge components.
package constants

const (
	// AnnotationAllowedActions is the annotation for allowed actions
	AnnotationAllowedActions = "forge.dev/allowed-actions"
	// AnnotationAllowedSourceRepos is the annotation for allowed source repositories
	AnnotationAllowedSourceRepos = "forge.dev/allowed-source-repos"
	// AnnotationAllowedSourceBuckets is the annotation for allowed source buckets
	AnnotationAllowedSourceBuckets = "forge.dev/allowed-source-buckets"
	// AnnotationAllowedSourceRegistries is the annotation for allowed source registries
	AnnotationAllowedSourceRegistries = "forge.dev/allowed-source-registries"
	// AnnotationAllowedPublishBuckets is the annotation for allowed publish buckets
	AnnotationAllowedPublishBuckets = "forge.dev/allowed-publish-buckets"
	// AnnotationAllowedPublishRegistries is the annotation for allowed publish registries
	AnnotationAllowedPublishRegistries = "forge.dev/allowed-publish-registries"
	// AnnotationAllowedDeployTargets is the annotation for allowed deploy targets
	AnnotationAllowedDeployTargets = "forge.dev/allowed-deploy-targets"
	// AnnotationAllowLocalSources is the annotation to allow local sources (dev mode)
	AnnotationAllowLocalSources = "forge.dev/allow-local-sources"
)
