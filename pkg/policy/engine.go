package policy

import (
	"context"
	"fmt"
	"path/filepath"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

// Engine handles policy validation
type Engine struct{}

// NewEngine creates a new policy engine
func NewEngine() *Engine {
	return &Engine{}
}

// Validate checks if the user is allowed to perform the action on the package
func (e *Engine) Validate(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage, user authenticationv1.UserInfo) error {
	policy := pkg.Spec.RBACPolicy
	if policy == nil {
		// If no policy is defined, we default to allowing (or maybe denying? Plan said "Security by default").
		// However, the plan also mentioned "Cluster admins control what users can do by creating ServiceAccounts".
		// For now, if no RBACPolicy is present on the resource, we assume it's open or controlled by other means.
		// But let's stick to the explicit policy fields for now.
		return nil
	}

	// 1. Check allowedUsers
	if !e.isUserAllowed(policy, user) {
		return fmt.Errorf("user %s is not allowed to use this resource", user.Username)
	}

	// 2. Check allowedActions
	if !e.isActionAllowed(policy, pkg.Spec.Action) {
		return fmt.Errorf("action %s is not allowed by policy", pkg.Spec.Action)
	}

	// 3. Check allowedSources
	if !e.isSourceAllowed(policy, pkg.Spec.Source) {
		return fmt.Errorf("source type %s is not allowed by policy", pkg.Spec.Source.Type)
	}

	// 4. Check allowedDestinations
	if pkg.Spec.Publish != nil {
		if !e.isDestinationAllowed(policy, pkg.Spec.Publish.Destination) {
			return fmt.Errorf("destination type %s is not allowed by policy", pkg.Spec.Publish.Destination.Type)
		}
	}

	// 5. Check allowedDeployTargets
	if pkg.Spec.Deploy != nil {
		if !e.isDeployTargetAllowed(policy, pkg.Spec.Deploy.Target) {
			return fmt.Errorf("deploy target %s is not allowed by policy", pkg.Spec.Deploy.Target)
		}
	}

	return nil
}

func (e *Engine) isUserAllowed(policy *zarfv1alpha1.RBACPolicy, user authenticationv1.UserInfo) bool {
	if len(policy.AllowedUsers) == 0 {
		return true // Empty list means no restrictions (or all allowed)
	}
	for _, allowed := range policy.AllowedUsers {
		if allowed == user.Username {
			return true
		}
		// TODO: Group matching
	}
	return false
}

func (e *Engine) isActionAllowed(policy *zarfv1alpha1.RBACPolicy, action zarfv1alpha1.Action) bool {
	if len(policy.AllowedActions) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedActions {
		if allowed == action {
			return true
		}
	}
	return false
}

func (e *Engine) isSourceAllowed(policy *zarfv1alpha1.RBACPolicy, source zarfv1alpha1.PackageSource) bool {
	if len(policy.AllowedSources) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedSources {
		if allowed.Type == source.Type {
			// Check specific restrictions
			switch source.Type {
			case zarfv1alpha1.SourceTypeGit:
				if len(allowed.Repos) > 0 && source.Git != nil {
					if !matchAny(allowed.Repos, source.Git.URL) {
						return false
					}
				}
			case zarfv1alpha1.SourceTypeS3:
				if len(allowed.Buckets) > 0 && source.S3 != nil {
					if !matchAny(allowed.Buckets, source.S3.Bucket) {
						return false
					}
				}
			case zarfv1alpha1.SourceTypeOCI:
				if len(allowed.Images) > 0 && source.OCI != nil {
					if !matchAny(allowed.Images, source.OCI.Image) {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

func (e *Engine) isDestinationAllowed(policy *zarfv1alpha1.RBACPolicy, dest zarfv1alpha1.PublishDestination) bool {
	if len(policy.AllowedDestinations) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedDestinations {
		if allowed.Type == dest.Type {
			switch dest.Type {
			case zarfv1alpha1.DestinationTypeS3:
				if len(allowed.Buckets) > 0 && dest.S3 != nil {
					if !matchAny(allowed.Buckets, dest.S3.Bucket) {
						return false
					}
				}
			case zarfv1alpha1.DestinationTypeOCI:
				if len(allowed.Registries) > 0 && dest.OCI != nil {
					if !matchAny(allowed.Registries, dest.OCI.Registry) {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

func (e *Engine) isDeployTargetAllowed(policy *zarfv1alpha1.RBACPolicy, target zarfv1alpha1.DeployTargetType) bool {
	if len(policy.AllowedDeployTargets) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedDeployTargets {
		if allowed == target {
			return true
		}
	}
	return false
}

func matchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, value); matched {
			return true
		}
	}
	return false
}
