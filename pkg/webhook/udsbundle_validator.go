// Package webhook implements admission webhook validation for UDSPackageJob resources.
//
// The webhook validates UDSPackageJob resources against ServiceAccount permissions to ensure:
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
	"strings"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// UDSPackageJobValidator validates UDSPackageJob resources against ServiceAccount permissions
type UDSPackageJobValidator struct {
	kubeClient kubernetes.Interface
}

// NewUDSPackageJobValidator creates a new UDSPackageJob validator
func NewUDSPackageJobValidator(kubeClient kubernetes.Interface) *UDSPackageJobValidator {
	return &UDSPackageJobValidator{
		kubeClient: kubeClient,
	}
}

// ValidateUDSPackageJob validates a UDSPackageJob resource against ServiceAccount permissions
func (validator *UDSPackageJobValidator) ValidateUDSPackageJob(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob) error {
	klog.InfoS("Validating UDSPackageJob", "name", bundle.Name, "namespace", bundle.Namespace)

	// Get the ServiceAccount
	sa, err := validator.kubeClient.CoreV1().ServiceAccounts(bundle.Namespace).Get(ctx, bundle.Spec.ServiceAccountName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ServiceAccount %s: %w", bundle.Spec.ServiceAccountName, err)
	}

	// Validate action is allowed
	if err := validator.validateAction(sa, bundle.Spec.Action); err != nil {
		return err
	}

	// Validate source permissions
	if err := validator.validateSource(sa, &bundle.Spec.Source); err != nil {
		return err
	}

	// Validate publish permissions if publish config is specified
	if bundle.Spec.Publish != nil {
		if err := validator.validatePublish(sa, bundle.Spec.Publish); err != nil {
			return err
		}
	}

	// Validate deploy permissions if deploy config is specified
	if bundle.Spec.Deploy != nil {
		if err := validator.validateDeploy(sa, bundle.Spec.Deploy); err != nil {
			return err
		}
	}

	klog.InfoS("UDSPackageJob validation passed", "name", bundle.Name, "namespace", bundle.Namespace)
	return nil
}

// validateAction checks if the action is allowed by the ServiceAccount
func (validator *UDSPackageJobValidator) validateAction(sa *corev1.ServiceAccount, action udsv1alpha2.Action) error {
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
func (validator *UDSPackageJobValidator) validateSource(sa *corev1.ServiceAccount, source *udsv1alpha2.PackageSource) error {
	switch source.Type {
	case udsv1alpha2.SourceTypeGit:
		if source.Git == nil {
			return fmt.Errorf("git source configuration is required")
		}
		return validator.validateGitSource(sa, source.Git)

	case udsv1alpha2.SourceTypeS3:
		if source.S3 == nil {
			return fmt.Errorf("s3 source configuration is required")
		}
		return validator.validateS3Source(sa, source.S3)

	case udsv1alpha2.SourceTypeOCI:
		if source.OCI == nil {
			return fmt.Errorf("oci source configuration is required")
		}
		return validator.validateOCISource(sa, source.OCI)

	case udsv1alpha2.SourceTypeLocal:
		// Local source is dev/testing only - could add annotation to control this
		klog.V(4).InfoS("Local source allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}
}

// validateGitSource validates Git source permissions
func (validator *UDSPackageJobValidator) validateGitSource(sa *corev1.ServiceAccount, git *udsv1alpha2.GitSource) error {
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
func (validator *UDSPackageJobValidator) validateS3Source(sa *corev1.ServiceAccount, s3 *udsv1alpha2.S3Source) error {
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
func (validator *UDSPackageJobValidator) validateOCISource(sa *corev1.ServiceAccount, oci *udsv1alpha2.OCISource) error {
	allowedRegistries := getAnnotation(sa, constants.AnnotationAllowedSourceRegistries)
	if allowedRegistries == "" {
		return fmt.Errorf("ServiceAccount %s has no allowed-source-registries annotation", sa.Name)
	}

	patterns := strings.Split(allowedRegistries, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || matchesGlob(oci.Reference, pattern) {
			klog.V(4).InfoS("OCI source allowed", "reference", oci.Reference, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("OCI reference %s is not allowed by ServiceAccount %s (allowed patterns: %s)", oci.Reference, sa.Name, allowedRegistries)
}

// validatePublish validates publish destination permissions
func (validator *UDSPackageJobValidator) validatePublish(sa *corev1.ServiceAccount, publish *udsv1alpha2.PublishConfig) error {
	switch publish.Destination.Type {
	case udsv1alpha2.DestinationTypeS3:
		if publish.Destination.S3 == nil {
			return fmt.Errorf("s3 publish destination is required")
		}
		return validator.validateS3Publish(sa, publish.Destination.S3)

	case udsv1alpha2.DestinationTypeOCI:
		if publish.Destination.OCI == nil {
			return fmt.Errorf("oci publish destination is required")
		}
		return validator.validateOCIPublish(sa, publish.Destination.OCI)

	case udsv1alpha2.DestinationTypeLocal:
		// Local publish is dev/testing only
		klog.V(4).InfoS("Local publish allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown publish destination type: %s", publish.Destination.Type)
	}
}

// validateS3Publish validates S3 publish permissions
func (validator *UDSPackageJobValidator) validateS3Publish(sa *corev1.ServiceAccount, s3 *udsv1alpha2.S3Destination) error {
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
func (validator *UDSPackageJobValidator) validateOCIPublish(sa *corev1.ServiceAccount, oci *udsv1alpha2.OCIDestination) error {
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
func (validator *UDSPackageJobValidator) validateDeploy(sa *corev1.ServiceAccount, deploy *udsv1alpha2.DeployConfig) error {
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
