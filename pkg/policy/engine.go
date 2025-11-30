// Package policy enforces access control policies for ZarfPackageJob operations based on ServiceAccount permissions.
package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// AnnotationAllowedActions is the annotation for allowed actions
	AnnotationAllowedActions = "forge.forge.dev/allowed-actions"
	// AnnotationAllowedSourceRepos is the annotation for allowed source repositories
	AnnotationAllowedSourceRepos = "forge.forge.dev/allowed-source-repos"
	// AnnotationAllowedSourceBuckets is the annotation for allowed source buckets
	AnnotationAllowedSourceBuckets = "forge.forge.dev/allowed-source-buckets"
	// AnnotationAllowedSourceRegistries is the annotation for allowed source registries
	AnnotationAllowedSourceRegistries = "forge.forge.dev/allowed-source-registries"
	// AnnotationAllowedPublishBuckets is the annotation for allowed publish buckets
	AnnotationAllowedPublishBuckets = "forge.forge.dev/allowed-publish-buckets"
	// AnnotationAllowedPublishRegistries is the annotation for allowed publish registries
	AnnotationAllowedPublishRegistries = "forge.forge.dev/allowed-publish-registries"
	// AnnotationAllowedDeployTargets is the annotation for allowed deploy targets
	AnnotationAllowedDeployTargets = "forge.forge.dev/allowed-deploy-targets"
	// AnnotationAllowLocalSources is the annotation to allow local sources (dev mode)
	AnnotationAllowLocalSources = "forge.forge.dev/allow-local-sources"
)

// Engine handles policy validation
type Engine struct {
	kubeClient kubernetes.Interface
}

// NewEngine creates a new policy engine
func NewEngine(kubeClient kubernetes.Interface) *Engine {
	return &Engine{
		kubeClient: kubeClient,
	}
}

// Validate checks if the operation is allowed based on the ServiceAccount permissions
func (engine *Engine) Validate(ctx context.Context, pkg *zarfv1alpha1.ZarfPackageJob) error {
	// 1. Fetch ServiceAccount
	saName := pkg.Spec.ServiceAccountName
	if saName == "" {
		return fmt.Errorf("serviceAccountName is required")
	}

	sa, err := engine.kubeClient.CoreV1().ServiceAccounts(pkg.Namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ServiceAccount %s: %w", saName, err)
	}

	annotations := sa.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// 2. Check Allowed Actions
	allowedActions := parseList(annotations[AnnotationAllowedActions])
	if !isActionAllowed(pkg.Spec.Action, allowedActions) {
		return fmt.Errorf("action %s is not allowed (allowed actions: %v) for ServiceAccount %s",
			pkg.Spec.Action, allowedActions, saName)
	}

	// Log successful policy validation
	klog.InfoS("Policy validation passed",
		"package", pkg.Name,
		"namespace", pkg.Namespace,
		"action", pkg.Spec.Action,
		"serviceAccount", saName)

	// 3. Check Allowed Sources
	if err := engine.validateSource(pkg.Spec.Source, annotations, saName); err != nil {
		return err
	}

	// 4. Check Allowed Destinations
	if pkg.Spec.Publish != nil {
		if err := engine.validateDestination(pkg.Spec.Publish.Destination, annotations, saName); err != nil {
			return err
		}
	}

	// 5. Check Allowed Deploy Targets
	if pkg.Spec.Deploy != nil {
		allowedTargets := parseList(annotations[AnnotationAllowedDeployTargets])
		if !isDeployTargetAllowed(pkg.Spec.Deploy.Target, allowedTargets) {
			return fmt.Errorf("deploy target %s is not allowed (allowed targets: %v) for ServiceAccount %s",
				pkg.Spec.Deploy.Target, allowedTargets, saName)
		}
	}

	return nil
}

// validateSource checks if the source is allowed
func (engine *Engine) validateSource(source zarfv1alpha1.PackageSource, annotations map[string]string, saName string) error {
	// If no restrictions are defined, is it allowed?
	// "Security by default" implies denied. But let's assume if annotation is missing, it's denied?
	// Or maybe if annotation is missing, it's allowed?
	// The commit message said "Denies by default".

	switch source.Type {
	case zarfv1alpha1.SourceTypeGit:
		allowedRepos := parseList(annotations[AnnotationAllowedSourceRepos])
		if len(allowedRepos) == 0 {
			return fmt.Errorf("no allowed source repos defined (annotation %s is required)", AnnotationAllowedSourceRepos)
		}
		if source.Git == nil {
			return fmt.Errorf("source type is Git but Git config is nil")
		}
		if !matchAny(allowedRepos, source.Git.URL) {
			return fmt.Errorf("git repo %s is not allowed (allowed repos: %v) for ServiceAccount %s",
				source.Git.URL, allowedRepos, saName)
		}
	case zarfv1alpha1.SourceTypeS3:
		allowedBuckets := parseList(annotations[AnnotationAllowedSourceBuckets])
		if len(allowedBuckets) == 0 {
			return fmt.Errorf("no allowed source buckets defined (annotation %s is required)", AnnotationAllowedSourceBuckets)
		}
		if source.S3 == nil {
			return fmt.Errorf("source type is S3 but S3 config is nil")
		}
		if !matchAny(allowedBuckets, source.S3.Bucket) {
			return fmt.Errorf("S3 bucket %s is not allowed (allowed buckets: %v) for ServiceAccount %s",
				source.S3.Bucket, allowedBuckets, saName)
		}
	case zarfv1alpha1.SourceTypeOCI:
		allowedRegistries := parseList(annotations[AnnotationAllowedSourceRegistries])
		if len(allowedRegistries) == 0 {
			return fmt.Errorf("no allowed source registries defined (annotation %s is required)", AnnotationAllowedSourceRegistries)
		}
		if source.OCI == nil {
			return fmt.Errorf("source type is OCI but OCI config is nil")
		}
		if !matchAny(allowedRegistries, source.OCI.Image) {
			return fmt.Errorf("OCI image %s is not allowed (allowed registries: %v) for ServiceAccount %s",
				source.OCI.Image, allowedRegistries, saName)
		}
	case zarfv1alpha1.SourceTypeLocal:
		// Local sources require explicit permission (dev mode only)
		if annotations[AnnotationAllowLocalSources] != "true" {
			return fmt.Errorf("local sources are not allowed (set annotation %s: true for dev mode)", AnnotationAllowLocalSources)
		}
	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}
	return nil
}

// validateDestination checks if the destination is allowed
func (engine *Engine) validateDestination(dest zarfv1alpha1.PublishDestination, annotations map[string]string, saName string) error {
	switch dest.Type {
	case zarfv1alpha1.DestinationTypeS3:
		allowedBuckets := parseList(annotations[AnnotationAllowedPublishBuckets])
		if len(allowedBuckets) == 0 {
			return fmt.Errorf("no allowed publish buckets defined (annotation %s is required)", AnnotationAllowedPublishBuckets)
		}
		if dest.S3 == nil {
			return fmt.Errorf("destination type is S3 but S3 config is nil")
		}
		if !matchAny(allowedBuckets, dest.S3.Bucket) {
			return fmt.Errorf("S3 bucket %s is not allowed (allowed buckets: %v) for ServiceAccount %s",
				dest.S3.Bucket, allowedBuckets, saName)
		}
	case zarfv1alpha1.DestinationTypeOCI:
		allowedRegistries := parseList(annotations[AnnotationAllowedPublishRegistries])
		if len(allowedRegistries) == 0 {
			return fmt.Errorf("no allowed publish registries defined (annotation %s is required)", AnnotationAllowedPublishRegistries)
		}
		if dest.OCI == nil {
			return fmt.Errorf("destination type is OCI but OCI config is nil")
		}
		if !matchAny(allowedRegistries, dest.OCI.Registry) {
			return fmt.Errorf("OCI registry %s is not allowed (allowed registries: %v) for ServiceAccount %s",
				dest.OCI.Registry, allowedRegistries, saName)
		}
	case zarfv1alpha1.DestinationTypeLocal:
		// Local destinations require explicit permission (dev mode only)
		if annotations[AnnotationAllowLocalSources] != "true" {
			return fmt.Errorf("local destinations are not allowed (set annotation %s: true for dev mode)", AnnotationAllowLocalSources)
		}
	default:
		return fmt.Errorf("unknown destination type: %s", dest.Type)
	}
	return nil
}

func isActionAllowed(action zarfv1alpha1.Action, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, allowedAction := range allowed {
		if string(action) == allowedAction || allowedAction == "*" {
			return true
		}
	}
	return false
}

func isDeployTargetAllowed(target zarfv1alpha1.DeployTargetType, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, allowedTarget := range allowed {
		if string(target) == allowedTarget || allowedTarget == "*" {
			return true
		}
	}
	return false
}

func parseList(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var result []string
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

func matchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, value)
		if err != nil {
			// Invalid pattern, log and skip it
			klog.V(4).InfoS("Invalid glob pattern", "pattern", pattern, "error", err)
			continue
		}
		if matched {
			return true
		}
		// Also handle simple prefix matching if glob fails or is not enough
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}
