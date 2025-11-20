package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// AnnotationAllowedActions is the annotation for allowed actions
	AnnotationAllowedActions = "forge.zarf.dev/allowed-actions"
	// AnnotationAllowedSourceRepos is the annotation for allowed source repositories
	AnnotationAllowedSourceRepos = "forge.zarf.dev/allowed-source-repos"
	// AnnotationAllowedSourceBuckets is the annotation for allowed source buckets
	AnnotationAllowedSourceBuckets = "forge.zarf.dev/allowed-source-buckets"
	// AnnotationAllowedSourceRegistries is the annotation for allowed source registries
	AnnotationAllowedSourceRegistries = "forge.zarf.dev/allowed-source-registries"
	// AnnotationAllowedPublishBuckets is the annotation for allowed publish buckets
	AnnotationAllowedPublishBuckets = "forge.zarf.dev/allowed-publish-buckets"
	// AnnotationAllowedPublishRegistries is the annotation for allowed publish registries
	AnnotationAllowedPublishRegistries = "forge.zarf.dev/allowed-publish-registries"
	// AnnotationAllowedDeployTargets is the annotation for allowed deploy targets
	AnnotationAllowedDeployTargets = "forge.zarf.dev/allowed-deploy-targets"
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
func (e *Engine) Validate(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage) error {
	// 1. Fetch ServiceAccount
	saName := pkg.Spec.ServiceAccountName
	if saName == "" {
		return fmt.Errorf("serviceAccountName is required")
	}

	sa, err := e.kubeClient.CoreV1().ServiceAccounts(pkg.Namespace).Get(ctx, saName, metav1.GetOptions{})
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
		return fmt.Errorf("action %s is not allowed by ServiceAccount %s", pkg.Spec.Action, saName)
	}

	// 3. Check Allowed Sources
	if !e.isSourceAllowed(pkg.Spec.Source, annotations) {
		return fmt.Errorf("source type %s is not allowed by ServiceAccount %s", pkg.Spec.Source.Type, saName)
	}

	// 4. Check Allowed Destinations
	if pkg.Spec.Publish != nil {
		if !e.isDestinationAllowed(pkg.Spec.Publish.Destination, annotations) {
			return fmt.Errorf("destination type %s is not allowed by ServiceAccount %s", pkg.Spec.Publish.Destination.Type, saName)
		}
	}

	// 5. Check Allowed Deploy Targets
	if pkg.Spec.Deploy != nil {
		allowedTargets := parseList(annotations[AnnotationAllowedDeployTargets])
		if !isDeployTargetAllowed(pkg.Spec.Deploy.Target, allowedTargets) {
			return fmt.Errorf("deploy target %s is not allowed by ServiceAccount %s", pkg.Spec.Deploy.Target, saName)
		}
	}

	// 6. Check embedded RBACPolicy (if present)
	// This allows the package itself to further restrict usage, e.g. "Only Alice can use this package"
	// But the Controller doesn't know who "Alice" is.
	// So maybe RBACPolicy is for the Webhook to check against the requesting user?
	// For now, we'll skip UserInfo checks in the Controller.

	return nil
}

func (e *Engine) isSourceAllowed(source zarfv1alpha1.PackageSource, annotations map[string]string) bool {
	// If no restrictions are defined, is it allowed?
	// "Security by default" implies denied. But let's assume if annotation is missing, it's denied?
	// Or maybe if annotation is missing, it's allowed?
	// The commit message said "Denies by default".

	switch source.Type {
	case zarfv1alpha1.SourceTypeGit:
		allowedRepos := parseList(annotations[AnnotationAllowedSourceRepos])
		if len(allowedRepos) == 0 {
			return false
		}
		if source.Git != nil {
			return matchAny(allowedRepos, source.Git.URL)
		}
	case zarfv1alpha1.SourceTypeS3:
		allowedBuckets := parseList(annotations[AnnotationAllowedSourceBuckets])
		if len(allowedBuckets) == 0 {
			return false
		}
		if source.S3 != nil {
			return matchAny(allowedBuckets, source.S3.Bucket)
		}
	case zarfv1alpha1.SourceTypeOCI:
		allowedRegistries := parseList(annotations[AnnotationAllowedSourceRegistries])
		if len(allowedRegistries) == 0 {
			return false
		}
		if source.OCI != nil {
			return matchAny(allowedRegistries, source.OCI.Image)
		}
	case zarfv1alpha1.SourceTypeLocal:
		// Local sources usually require special permission or dev mode
		// For now, let's assume they are not allowed unless explicitly handled?
		// Or maybe we check a "allow-local" annotation?
		return false
	}
	return false
}

func (e *Engine) isDestinationAllowed(dest zarfv1alpha1.PublishDestination, annotations map[string]string) bool {
	switch dest.Type {
	case zarfv1alpha1.DestinationTypeS3:
		allowedBuckets := parseList(annotations[AnnotationAllowedPublishBuckets])
		if len(allowedBuckets) == 0 {
			return false
		}
		if dest.S3 != nil {
			return matchAny(allowedBuckets, dest.S3.Bucket)
		}
	case zarfv1alpha1.DestinationTypeOCI:
		allowedRegistries := parseList(annotations[AnnotationAllowedPublishRegistries])
		if len(allowedRegistries) == 0 {
			return false
		}
		if dest.OCI != nil {
			return matchAny(allowedRegistries, dest.OCI.Registry)
		}
	case zarfv1alpha1.DestinationTypeLocal:
		return false
	}
	return false
}

func isActionAllowed(action zarfv1alpha1.Action, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if string(action) == a || a == "*" {
			return true
		}
	}
	return false
}

func isDeployTargetAllowed(target zarfv1alpha1.DeployTargetType, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, t := range allowed {
		if string(target) == t || t == "*" {
			return true
		}
	}
	return false
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}

func matchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, value); matched {
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
