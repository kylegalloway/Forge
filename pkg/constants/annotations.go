// Package constants defines ServiceAccount annotation keys for policy enforcement.
//
// These annotations are applied to ServiceAccounts and validated by:
//   - Admission webhooks during ZarfPackageJob/UDSPackageJob creation
//   - Controllers before executing Job operations
//
// The policy engine (pkg/policy) uses these annotations to enforce RBAC-like controls:
//   - Which actions are permitted (build, publish, deploy, create)
//   - Which source repositories can be cloned (Git URLs, S3 buckets, OCI registries)
//   - Which destination registries can receive published artifacts
//   - Which Kubernetes clusters can be deployment targets
//
// Example ServiceAccount with policy annotations:
//
//	apiVersion: v1
//	kind: ServiceAccount
//	metadata:
//	  name: zarf-builder
//	  annotations:
//	    forge.dev/allowed-actions: "build,publish"
//	    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
//	    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"
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
