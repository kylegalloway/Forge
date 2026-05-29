// Package webhook implements admission webhook validation for UDSBundleJob resources.
//
// The webhook validates UDSBundleJob resources against ServiceAccount permissions to ensure:
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
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/audit"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// UDSBundleJobValidator validates UDSBundleJob resources against ServiceAccount permissions.
type UDSBundleJobValidator struct {
	pv *PermissionValidator
}

// NewUDSBundleJobValidator creates a new UDSBundleJob validator.
// The caller supplies the audit trail so that tests can pass a noop without
// requiring a live Kubernetes API server.
func NewUDSBundleJobValidator(kubeClient kubernetes.Interface, auditTrail audit.Trail) *UDSBundleJobValidator {
	return &UDSBundleJobValidator{
		pv: newPermissionValidator(kubeClient, auditTrail, "uds-validator"),
	}
}

// ValidateUDSBundleJob validates a UDSBundleJob resource against ServiceAccount permissions.
func (validator *UDSBundleJobValidator) ValidateUDSBundleJob(ctx context.Context, bundle *udsv1alpha3.UDSBundleJob) error {
	return validator.pv.Validate(
		ctx,
		bundle.Name,
		bundle.Namespace,
		bundle.Spec.ServiceAccountName,
		&udsSpecFacade{spec: &bundle.Spec},
	)
}

// ── Private methods kept for test compatibility ──────────────────────────────
// Tests construct &UDSBundleJobValidator{} and call these directly;
// they are thin delegators to the shared facade-level helpers.

func (validator *UDSBundleJobValidator) validateAction(sa *corev1.ServiceAccount, action udsv1alpha3.Action) error {
	return ValidateActionAllowed(sa, string(action), constants.AnnotationAllowedActions)
}

func (validator *UDSBundleJobValidator) validateSource(sa *corev1.ServiceAccount, source *udsv1alpha3.PackageSource) error {
	return validateSourceFacade(sa, udsSourceFacade(source))
}

func (validator *UDSBundleJobValidator) validatePublish(sa *corev1.ServiceAccount, publish *udsv1alpha3.PublishConfig) error {
	if publish == nil {
		return nil
	}
	pub := udsPublishFacade(publish)
	return validatePublishFacade(sa, pub)
}

func (validator *UDSBundleJobValidator) validateDeploy(sa *corev1.ServiceAccount, deploy *udsv1alpha3.DeployConfig) error {
	if deploy == nil {
		return nil
	}
	return validateDeployFacade(sa, DeployFacade{Target: string(deploy.Target)})
}

func (validator *UDSBundleJobValidator) validateExtraArgs(spec *udsv1alpha3.UDSBundleJobSpec) error {
	return (&udsSpecFacade{spec: spec}).ValidateExtraArgs()
}

// ── udsSpecFacade ─────────────────────────────────────────────────────────────

// udsSpecFacade adapts UDSBundleJobSpec to SpecFacade.
type udsSpecFacade struct {
	spec *udsv1alpha3.UDSBundleJobSpec
}

// ResourceKind returns "UDSBundleJob" for audit trail events.
func (f *udsSpecFacade) ResourceKind() string { return "UDSBundleJob" }

// Action returns the requested action string.
func (f *udsSpecFacade) Action() string { return string(f.spec.Action) }

// Source returns a SourceFacade for the spec's source configuration.
func (f *udsSpecFacade) Source() SourceFacade {
	return udsSourceFacade(&f.spec.Source)
}

// Publish returns a PublishFacade when a publish config is present, nil otherwise.
func (f *udsSpecFacade) Publish() *PublishFacade {
	if f.spec.Publish == nil {
		return nil
	}
	pub := udsPublishFacade(f.spec.Publish)
	return &pub
}

// Deploy returns a DeployFacade when a deploy config is present, nil otherwise.
func (f *udsSpecFacade) Deploy() *DeployFacade {
	if f.spec.Deploy == nil {
		return nil
	}
	return &DeployFacade{Target: string(f.spec.Deploy.Target)}
}

// ValidateExtraArgs validates extraArgs fields and — UDS-specific — preTasks fields.
func (f *udsSpecFacade) ValidateExtraArgs() error {
	if f.spec.Create != nil && len(f.spec.Create.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Create.ExtraArgs); err != nil {
			return fmt.Errorf("create.extraArgs: %w", err)
		}
	}
	if f.spec.Create != nil && len(f.spec.Create.PreTasks) > 0 {
		if err := validation.ValidatePreTasks(f.spec.Create.PreTasks); err != nil {
			return fmt.Errorf("create.%w", err)
		}
	}
	if f.spec.Deploy != nil && len(f.spec.Deploy.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Deploy.ExtraArgs); err != nil {
			return fmt.Errorf("deploy.extraArgs: %w", err)
		}
	}
	if f.spec.Deploy != nil && len(f.spec.Deploy.PreTasks) > 0 {
		if err := validation.ValidatePreTasks(f.spec.Deploy.PreTasks); err != nil {
			return fmt.Errorf("deploy.%w", err)
		}
	}
	if f.spec.Publish != nil && len(f.spec.Publish.ExtraArgs) > 0 {
		if err := validation.ValidateExtraArgs(f.spec.Publish.ExtraArgs); err != nil {
			return fmt.Errorf("publish.extraArgs: %w", err)
		}
	}
	return nil
}

// ── UDS facade helpers ────────────────────────────────────────────────────────

func udsSourceFacade(source *udsv1alpha3.PackageSource) SourceFacade {
	f := SourceFacade{Type: string(source.Type)}
	switch source.Type {
	case udsv1alpha3.SourceTypeGit:
		if source.Git != nil {
			f.GitURL = source.Git.URL
		}
	case udsv1alpha3.SourceTypeS3:
		if source.S3 != nil {
			f.S3Bucket = source.S3.Bucket
		}
	case udsv1alpha3.SourceTypeOCI:
		if source.OCI != nil {
			f.OCIReference = source.OCI.Reference
		}
	}
	return f
}

func udsPublishFacade(publish *udsv1alpha3.PublishConfig) PublishFacade {
	pub := PublishFacade{DestinationType: string(publish.Destination.Type)}
	switch publish.Destination.Type {
	case udsv1alpha3.DestinationTypeS3:
		if publish.Destination.S3 != nil {
			pub.S3Bucket = publish.Destination.S3.Bucket
		}
	case udsv1alpha3.DestinationTypeOCI:
		if publish.Destination.OCI != nil {
			pub.OCIRegistry = publish.Destination.OCI.Registry
			pub.OCIRepository = publish.Destination.OCI.Repository
		}
	}
	return pub
}
