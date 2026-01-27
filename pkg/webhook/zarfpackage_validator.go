// Package webhook implements admission webhook validation for ZarfPackageJob resources.
//
// The webhook validates ZarfPackageJob resources against ServiceAccount permissions to ensure:
//   - Users can only perform actions allowed by their ServiceAccount annotations
//   - Source repositories/registries/buckets match allowed patterns
//   - Publish destinations match allowed patterns
//   - Deploy targets are permitted
//
// ServiceAccount annotations define permissions using glob patterns:
//   - forge.dev/allowed-actions: Comma-separated list of allowed actions
//   - forge.dev/allowed-source-repos: Glob patterns for Git sources
//   - forge.dev/allowed-source-registries: Glob patterns for OCI sources
//   - forge.dev/allowed-source-buckets: Glob patterns for S3 sources
//   - forge.dev/allowed-publish-registries: Glob patterns for OCI publish
//   - forge.dev/allowed-publish-buckets: Glob patterns for S3 publish
//   - forge.dev/allowed-deploy-targets: Comma-separated list (InCluster, ExternalCluster)
package webhook

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kylegalloway/forge/pkg/actions/validation"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/audit"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ZarfPackageJobValidator validates ZarfPackageJob resources against ServiceAccount permissions
type ZarfPackageJobValidator struct {
	kubeClient kubernetes.Interface
	auditTrail *audit.AuditTrail
	logger     *logging.Logger
}

// NewZarfPackageJobValidator creates a new ZarfPackageJob validator
func NewZarfPackageJobValidator(kubeClient kubernetes.Interface) *ZarfPackageJobValidator {
	return &ZarfPackageJobValidator{
		kubeClient: kubeClient,
		auditTrail: audit.NewAuditTrail(kubeClient, audit.DefaultConfig()),
		logger:     logging.NewLogger("zarf-validator"),
	}
}

// ValidateZarfPackageJob validates a ZarfPackageJob resource against ServiceAccount permissions
func (validator *ZarfPackageJobValidator) ValidateZarfPackageJob(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob) error {
	startTime := time.Now()

	// Set up logging context with correlation ID
	correlationID := logging.GenerateCorrelationID(pkg.Namespace, pkg.Name, string(pkg.Spec.Action))
	ctx = logging.WithCorrelationID(ctx, correlationID)
	ctx = logging.WithJobName(ctx, pkg.Name)
	ctx = logging.WithNamespace(ctx, pkg.Namespace)
	ctx = logging.WithAction(ctx, string(pkg.Spec.Action))

	validator.logger.Debug(ctx, "Starting validation",
		"resource", pkg.Name,
		"serviceAccount", pkg.Spec.ServiceAccountName,
		"debugMode", pkg.Spec.DebugMode,
	)

	klog.InfoS("Validating ZarfPackageJob", "name", pkg.Name, "namespace", pkg.Namespace)

	// Get the ServiceAccount
	validator.logger.Debug(ctx, "Fetching ServiceAccount",
		"serviceAccount", pkg.Spec.ServiceAccountName,
	)
	sa, err := validator.kubeClient.CoreV1().ServiceAccounts(pkg.Namespace).Get(ctx, pkg.Spec.ServiceAccountName, metav1.GetOptions{})
	if err != nil {
		reason := fmt.Sprintf("failed to get ServiceAccount %s: %v", pkg.Spec.ServiceAccountName, err)
		validator.logger.Debug(ctx, "ServiceAccount lookup failed",
			"serviceAccount", pkg.Spec.ServiceAccountName,
			"error", err.Error(),
			"decision", "DENY",
		)
		if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, reason); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return fmt.Errorf("failed to get ServiceAccount %s: %w", pkg.Spec.ServiceAccountName, err)
	}

	// Log ServiceAccount annotations for debugging
	validator.logger.Debug(ctx, "ServiceAccount annotations parsed",
		"allowedActions", getAnnotation(sa, constants.AnnotationAllowedActions),
		"allowedSourceRepos", getAnnotation(sa, constants.AnnotationAllowedSourceRepos),
		"allowedSourceRegistries", getAnnotation(sa, constants.AnnotationAllowedSourceRegistries),
		"allowedSourceBuckets", getAnnotation(sa, constants.AnnotationAllowedSourceBuckets),
		"allowedPublishRegistries", getAnnotation(sa, constants.AnnotationAllowedPublishRegistries),
		"allowedPublishBuckets", getAnnotation(sa, constants.AnnotationAllowedPublishBuckets),
		"allowedDeployTargets", getAnnotation(sa, constants.AnnotationAllowedDeployTargets),
	)

	// Validate action is allowed
	validator.logger.Debug(ctx, "Checking allowed actions",
		"requestedAction", pkg.Spec.Action,
		"allowedActions", getAnnotation(sa, constants.AnnotationAllowedActions),
	)
	if err := validator.validateAction(sa, pkg.Spec.Action); err != nil {
		validator.logger.Debug(ctx, "Action validation failed",
			"requestedAction", pkg.Spec.Action,
			"decision", "DENY",
			"reason", err.Error(),
		)
		if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	validator.logger.Debug(ctx, "Action validation passed", "action", pkg.Spec.Action, "decision", "ALLOW")

	// Validate source permissions
	validator.logger.Debug(ctx, "Checking source policy",
		"sourceType", pkg.Spec.Source.Type,
	)
	if err := validator.validateSource(sa, &pkg.Spec.Source); err != nil {
		validator.logger.Debug(ctx, "Source validation failed",
			"sourceType", pkg.Spec.Source.Type,
			"decision", "DENY",
			"reason", err.Error(),
		)
		if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	validator.logger.Debug(ctx, "Source validation passed", "sourceType", pkg.Spec.Source.Type, "decision", "ALLOW")

	// Validate extraArgs for command injection
	validator.logger.Debug(ctx, "Checking extraArgs for command injection")
	if err := validator.validateExtraArgs(&pkg.Spec); err != nil {
		validator.logger.Debug(ctx, "ExtraArgs validation failed",
			"decision", "DENY",
			"reason", err.Error(),
		)
		if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	validator.logger.Debug(ctx, "ExtraArgs validation passed", "decision", "ALLOW")

	// Validate publish permissions if publish config is specified
	if pkg.Spec.Publish != nil {
		validator.logger.Debug(ctx, "Checking publish policy",
			"destinationType", pkg.Spec.Publish.Destination.Type,
		)
		if err := validator.validatePublish(sa, pkg.Spec.Publish); err != nil {
			validator.logger.Debug(ctx, "Publish validation failed",
				"destinationType", pkg.Spec.Publish.Destination.Type,
				"decision", "DENY",
				"reason", err.Error(),
			)
			if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, err.Error()); auditErr != nil {
				klog.V(4).ErrorS(auditErr, "Failed to record audit event")
			}
			return err
		}
		validator.logger.Debug(ctx, "Publish validation passed", "destinationType", pkg.Spec.Publish.Destination.Type, "decision", "ALLOW")
	}

	// Validate deploy permissions if deploy config is specified
	if pkg.Spec.Deploy != nil {
		validator.logger.Debug(ctx, "Checking deploy policy",
			"target", pkg.Spec.Deploy.Target,
		)
		if err := validator.validateDeploy(sa, pkg.Spec.Deploy); err != nil {
			validator.logger.Debug(ctx, "Deploy validation failed",
				"target", pkg.Spec.Deploy.Target,
				"decision", "DENY",
				"reason", err.Error(),
			)
			if auditErr := validator.auditTrail.RecordJobValidationFailed(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, err.Error()); auditErr != nil {
				klog.V(4).ErrorS(auditErr, "Failed to record audit event")
			}
			return err
		}
		validator.logger.Debug(ctx, "Deploy validation passed", "target", pkg.Spec.Deploy.Target, "decision", "ALLOW")
	}

	// Record successful validation
	details := map[string]string{
		"serviceAccount": pkg.Spec.ServiceAccountName,
		"action":         string(pkg.Spec.Action),
	}
	if auditErr := validator.auditTrail.RecordJobValidated(ctx, "ZarfPackageJob", pkg.Namespace, pkg.Name, pkg.Spec.ServiceAccountName, details); auditErr != nil {
		klog.V(4).ErrorS(auditErr, "Failed to record audit event")
	}

	validator.logger.Debug(ctx, "Validation complete",
		"resource", pkg.Name,
		"allowed", true,
		"duration", time.Since(startTime).String(),
	)

	klog.InfoS("ZarfPackageJob validation passed", "name", pkg.Name, "namespace", pkg.Namespace)
	return nil
}

// validateAction checks if the action is allowed by the ServiceAccount
func (validator *ZarfPackageJobValidator) validateAction(sa *corev1.ServiceAccount, action zarfv1alpha3.Action) error {
	allowedActions := getAnnotation(sa, constants.AnnotationAllowedActions)
	if allowedActions == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-actions annotation", sa.Name)
	}

	actions := strings.Split(allowedActions, ",")
	for _, allowed := range actions {
		if strings.TrimSpace(allowed) == string(action) {
			klog.V(4).InfoS("Action allowed", "action", action, "serviceAccount", sa.Name)
			return nil
		}
	}

	return fmt.Errorf("action %s is not allowed by ServiceAccount %s (allowed: %s)", action, sa.Name, allowedActions)
}

// validateSource validates the source configuration
func (validator *ZarfPackageJobValidator) validateSource(sa *corev1.ServiceAccount, source *zarfv1alpha3.PackageSource) error {
	switch source.Type {
	case zarfv1alpha3.SourceTypeGit:
		if source.Git == nil {
			return fmt.Errorf("git source configuration is required")
		}
		return validator.validateGitSource(sa, source.Git)

	case zarfv1alpha3.SourceTypeS3:
		if source.S3 == nil {
			return fmt.Errorf("s3 source configuration is required")
		}
		return validator.validateS3Source(sa, source.S3)

	case zarfv1alpha3.SourceTypeOCI:
		if source.OCI == nil {
			return fmt.Errorf("oci source configuration is required")
		}
		return validator.validateOCISource(sa, source.OCI)

	case zarfv1alpha3.SourceTypeLocal:
		// Local source is dev/testing only - could add annotation to control this
		klog.V(4).InfoS("Local source allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}
}

// validateGitSource validates Git source permissions
func (validator *ZarfPackageJobValidator) validateGitSource(sa *corev1.ServiceAccount, git *zarfv1alpha3.GitSource) error {
	allowedRepos := getAnnotation(sa, constants.AnnotationAllowedSourceRepos)
	if allowedRepos == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-source-repos annotation", sa.Name)
	}

	patterns := strings.Split(allowedRepos, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(git.URL, pattern) {
			klog.V(4).InfoS("Git source allowed", "url", git.URL, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("git repository %s is not allowed by ServiceAccount %s (allowed patterns: %s)", git.URL, sa.Name, allowedRepos)
}

// validateS3Source validates S3 source permissions
func (validator *ZarfPackageJobValidator) validateS3Source(sa *corev1.ServiceAccount, s3 *zarfv1alpha3.S3Source) error {
	allowedBuckets := getAnnotation(sa, constants.AnnotationAllowedSourceBuckets)
	if allowedBuckets == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-source-buckets annotation", sa.Name)
	}

	patterns := strings.Split(allowedBuckets, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(s3.Bucket, pattern) {
			klog.V(4).InfoS("S3 source allowed", "bucket", s3.Bucket, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("S3 bucket %s is not allowed by ServiceAccount %s (allowed patterns: %s)", s3.Bucket, sa.Name, allowedBuckets)
}

// validateOCISource validates OCI source permissions
func (validator *ZarfPackageJobValidator) validateOCISource(sa *corev1.ServiceAccount, oci *zarfv1alpha3.OCISource) error {
	allowedRegistries := getAnnotation(sa, constants.AnnotationAllowedSourceRegistries)
	if allowedRegistries == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-source-registries annotation", sa.Name)
	}

	patterns := strings.Split(allowedRegistries, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(oci.Reference, pattern) {
			klog.V(4).InfoS("OCI source allowed", "image", oci.Reference, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("OCI image %s is not allowed by ServiceAccount %s (allowed patterns: %s)", oci.Reference, sa.Name, allowedRegistries)
}

// validatePublish validates publish destination permissions
func (validator *ZarfPackageJobValidator) validatePublish(sa *corev1.ServiceAccount, publish *zarfv1alpha3.PublishConfig) error {
	switch publish.Destination.Type {
	case zarfv1alpha3.DestinationTypeS3:
		if publish.Destination.S3 == nil {
			return fmt.Errorf("s3 publish destination is required")
		}
		return validator.validateS3Publish(sa, publish.Destination.S3)

	case zarfv1alpha3.DestinationTypeOCI:
		if publish.Destination.OCI == nil {
			return fmt.Errorf("oci publish destination is required")
		}
		return validator.validateOCIPublish(sa, publish.Destination.OCI)

	case zarfv1alpha3.DestinationTypeLocal:
		// Local publish is dev/testing only
		klog.V(4).InfoS("Local publish allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown publish destination type: %s", publish.Destination.Type)
	}
}

// validateS3Publish validates S3 publish permissions
func (validator *ZarfPackageJobValidator) validateS3Publish(sa *corev1.ServiceAccount, s3 *zarfv1alpha3.S3Destination) error {
	allowedBuckets := getAnnotation(sa, constants.AnnotationAllowedPublishBuckets)
	if allowedBuckets == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-publish-buckets annotation", sa.Name)
	}

	patterns := strings.Split(allowedBuckets, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(s3.Bucket, pattern) {
			klog.V(4).InfoS("S3 publish allowed", "bucket", s3.Bucket, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("S3 bucket %s is not allowed for publishing by ServiceAccount %s (allowed patterns: %s)", s3.Bucket, sa.Name, allowedBuckets)
}

// validateOCIPublish validates OCI publish permissions
func (validator *ZarfPackageJobValidator) validateOCIPublish(sa *corev1.ServiceAccount, oci *zarfv1alpha3.OCIDestination) error {
	allowedRegistries := getAnnotation(sa, constants.AnnotationAllowedPublishRegistries)
	if allowedRegistries == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-publish-registries annotation", sa.Name)
	}

	// Construct full OCI reference for matching
	ociRef := fmt.Sprintf("%s/%s", oci.Registry, oci.Repository)

	patterns := strings.Split(allowedRegistries, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(ociRef, pattern) {
			klog.V(4).InfoS("OCI publish allowed", "registry", ociRef, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("OCI registry %s is not allowed for publishing by ServiceAccount %s (allowed patterns: %s)", ociRef, sa.Name, allowedRegistries)
}

// validateDeploy validates deploy target permissions
func (validator *ZarfPackageJobValidator) validateDeploy(sa *corev1.ServiceAccount, deploy *zarfv1alpha3.DeployConfig) error {
	allowedTargets := getAnnotation(sa, constants.AnnotationAllowedDeployTargets)
	if allowedTargets == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-deploy-targets annotation", sa.Name)
	}

	targets := strings.Split(allowedTargets, ",")
	for _, allowed := range targets {
		if strings.TrimSpace(allowed) == string(deploy.Target) {
			klog.V(4).InfoS("Deploy target allowed", "target", deploy.Target, "serviceAccount", sa.Name)
			return nil
		}
	}

	return fmt.Errorf("deploy target %s is not allowed by ServiceAccount %s (allowed: %s)", deploy.Target, sa.Name, allowedTargets)
}

// getAnnotation safely retrieves an annotation value
func getAnnotation(sa *corev1.ServiceAccount, key string) string {
	if sa.Annotations == nil {
		return ""
	}
	return sa.Annotations[key]
}

// validateExtraArgs validates all extraArgs fields for command injection
func (validator *ZarfPackageJobValidator) validateExtraArgs(spec *zarfv1alpha3.ZarfPackageJobSpec) error {
	// Validate build.extraArgs
	if spec.Build != nil && len(spec.Build.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(spec.Build.ExtraArgs); err != nil {
			return fmt.Errorf("build.extraArgs: %w", err)
		}
	}

	// Validate deploy.extraArgs
	if spec.Deploy != nil && len(spec.Deploy.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(spec.Deploy.ExtraArgs); err != nil {
			return fmt.Errorf("deploy.extraArgs: %w", err)
		}
	}

	// Validate publish.extraArgs
	if spec.Publish != nil && len(spec.Publish.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(spec.Publish.ExtraArgs); err != nil {
			return fmt.Errorf("publish.extraArgs: %w", err)
		}
	}

	return nil
}

// matchesGlob checks if a string matches a glob pattern
func matchesGlob(s, pattern string) bool {
	// Try filepath.Match first for standard glob patterns
	matched, err := filepath.Match(pattern, s)
	if err != nil {
		klog.V(4).InfoS("Invalid glob pattern", "pattern", pattern, "error", err)
		// Continue to prefix matching even if pattern is invalid
	}
	if matched {
		return true
	}

	// Fallback: if pattern ends with *, do simple prefix matching
	// This handles cases like "https://github.com/*" matching "https://github.com/org/repo"
	// where filepath.Match would fail because * doesn't cross path separators
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}
