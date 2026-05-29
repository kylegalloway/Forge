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

	"github.com/kylegalloway/forge/pkg/actions/validation"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/audit"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ZarfPackageJobValidator validates ZarfPackageJob resources against ServiceAccount permissions.
type ZarfPackageJobValidator struct {
	pv *PermissionValidator
}

// NewZarfPackageJobValidator creates a new ZarfPackageJob validator.
// The caller supplies the audit trail so that tests can pass a noop without
// requiring a live Kubernetes API server.
func NewZarfPackageJobValidator(kubeClient kubernetes.Interface, auditTrail audit.Trail) *ZarfPackageJobValidator {
	return &ZarfPackageJobValidator{
		pv: newPermissionValidator(kubeClient, auditTrail, "zarf-validator"),
	}
}

// ValidateZarfPackageJob validates a ZarfPackageJob resource against ServiceAccount permissions.
func (validator *ZarfPackageJobValidator) ValidateZarfPackageJob(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob) error {
	return validator.pv.Validate(
		ctx,
		pkg.Name,
		pkg.Namespace,
		pkg.Spec.ServiceAccountName,
		&zarfSpecFacade{spec: &pkg.Spec},
	)
}

// ── Private methods kept for test compatibility ──────────────────────────────
// Tests construct &ZarfPackageJobValidator{} and call these directly;
// they are thin delegators to the shared facade-level helpers.

func (validator *ZarfPackageJobValidator) validateAction(sa *corev1.ServiceAccount, action zarfv1alpha3.Action) error {
	return ValidateActionAllowed(sa, string(action), constants.AnnotationAllowedActions)
}

func (validator *ZarfPackageJobValidator) validateSource(sa *corev1.ServiceAccount, source *zarfv1alpha3.PackageSource) error {
	return validateSourceFacade(sa, zarfSourceFacade(source))
}

func (validator *ZarfPackageJobValidator) validatePublish(sa *corev1.ServiceAccount, publish *zarfv1alpha3.PublishConfig) error {
	if publish == nil {
		return nil
	}
	pub := zarfPublishFacade(publish)
	return validatePublishFacade(sa, pub)
}

func (validator *ZarfPackageJobValidator) validateDeploy(sa *corev1.ServiceAccount, deploy *zarfv1alpha3.DeployConfig) error {
	if deploy == nil {
		return nil
	}
	return validateDeployFacade(sa, DeployFacade{Target: string(deploy.Target)})
}

func (validator *ZarfPackageJobValidator) validateExtraArgs(spec *zarfv1alpha3.ZarfPackageJobSpec) error {
	return (&zarfSpecFacade{spec: spec}).ValidateExtraArgs()
}

// ── zarfSpecFacade ────────────────────────────────────────────────────────────

// zarfSpecFacade adapts ZarfPackageJobSpec to SpecFacade.
type zarfSpecFacade struct {
	spec *zarfv1alpha3.ZarfPackageJobSpec
}

// ResourceKind returns "ZarfPackageJob" for audit trail events.
func (f *zarfSpecFacade) ResourceKind() string { return "ZarfPackageJob" }

// Action returns the requested action string.
func (f *zarfSpecFacade) Action() string { return string(f.spec.Action) }

// Source returns a SourceFacade for the spec's source configuration.
func (f *zarfSpecFacade) Source() SourceFacade {
	return zarfSourceFacade(&f.spec.Source)
}

// Publish returns a PublishFacade when a publish config is present, nil otherwise.
func (f *zarfSpecFacade) Publish() *PublishFacade {
	if f.spec.Publish == nil {
		return nil
	}
	pub := zarfPublishFacade(f.spec.Publish)
	return &pub
}

// Deploy returns a DeployFacade when a deploy config is present, nil otherwise.
func (f *zarfSpecFacade) Deploy() *DeployFacade {
	if f.spec.Deploy == nil {
		return nil
	}
	return &DeployFacade{Target: string(f.spec.Deploy.Target)}
}

// ValidateExtraArgs validates build/deploy/publish extraArgs for command injection.
func (f *zarfSpecFacade) ValidateExtraArgs() error {
	if f.spec.Build != nil && len(f.spec.Build.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Build.ExtraArgs); err != nil {
			return fmt.Errorf("build.extraArgs: %w", err)
		}
	}
	if f.spec.Deploy != nil && len(f.spec.Deploy.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Deploy.ExtraArgs); err != nil {
			return fmt.Errorf("deploy.extraArgs: %w", err)
		}
	}
	if f.spec.Publish != nil && len(f.spec.Publish.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Publish.ExtraArgs); err != nil {
			return fmt.Errorf("publish.extraArgs: %w", err)
		}
	}
	return nil
}

// ── Zarf facade helpers ───────────────────────────────────────────────────────

func zarfSourceFacade(source *zarfv1alpha3.PackageSource) SourceFacade {
	f := SourceFacade{Type: string(source.Type)}
	switch source.Type {
	case zarfv1alpha3.SourceTypeGit:
		if source.Git != nil {
			f.GitURL = source.Git.URL
		}
	case zarfv1alpha3.SourceTypeS3:
		if source.S3 != nil {
			f.S3Bucket = source.S3.Bucket
		}
	case zarfv1alpha3.SourceTypeOCI:
		if source.OCI != nil {
			f.OCIReference = source.OCI.Reference
		}
	}
	return f
}

func zarfPublishFacade(publish *zarfv1alpha3.PublishConfig) PublishFacade {
	pub := PublishFacade{DestinationType: string(publish.Destination.Type)}
	switch publish.Destination.Type {
	case zarfv1alpha3.DestinationTypeS3:
		if publish.Destination.S3 != nil {
			pub.S3Bucket = publish.Destination.S3.Bucket
		}
	case zarfv1alpha3.DestinationTypeOCI:
		if publish.Destination.OCI != nil {
			pub.OCIRegistry = publish.Destination.OCI.Registry
			pub.OCIRepository = publish.Destination.OCI.Repository
		}
	}
	return pub
}
