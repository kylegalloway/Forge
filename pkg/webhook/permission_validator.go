// Package webhook implements admission webhook validation shared across CRD types.
package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/kylegalloway/forge/pkg/audit"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// SourceFacade provides a unified view of a source configuration regardless of CRD type.
type SourceFacade struct {
	Type string

	GitURL string

	S3Bucket string

	OCIReference string
}

// PublishFacade provides a unified view of a publish configuration regardless of CRD type.
// Nil indicates no publish config is present.
type PublishFacade struct {
	DestinationType string

	S3Bucket string

	OCIRegistry   string
	OCIRepository string
}

// DeployFacade provides a unified view of a deploy configuration regardless of CRD type.
// Nil indicates no deploy config is present.
type DeployFacade struct {
	Target string
}

// SpecFacade is implemented by thin adapters wrapping each CRD-specific spec.
// It supplies the parts of the spec that the shared permission-validation chain needs.
type SpecFacade interface {
	// ResourceKind returns the CRD kind string used in audit trail events (e.g. "ZarfPackageJob").
	ResourceKind() string

	// Action returns the requested action string.
	Action() string

	// Source returns a SourceFacade describing the source configuration.
	Source() SourceFacade

	// Publish returns a non-nil *PublishFacade when a publish config is present.
	Publish() *PublishFacade

	// Deploy returns a non-nil *DeployFacade when a deploy config is present.
	Deploy() *DeployFacade

	// ValidateExtraArgs performs CRD-specific extra-args / preTasks validation.
	// The shared chain calls this once as part of the standard chain; returning nil
	// means the spec-level args are clean.
	ValidateExtraArgs() error
}

// PermissionValidator owns the shared validation chain used by both
// ZarfPackageJobValidator and UDSBundleJobValidator.  Each CRD-specific
// validator calls Validate with an appropriate SpecFacade adapter.
type PermissionValidator struct {
	kubeClient kubernetes.Interface
	auditTrail audit.Trail
	logger     *logging.Logger
}

// newPermissionValidator constructs a PermissionValidator. It is called by the
// CRD-specific validator constructors rather than directly by callers.
func newPermissionValidator(kubeClient kubernetes.Interface, auditTrail audit.Trail, loggerName string) *PermissionValidator {
	return &PermissionValidator{
		kubeClient: kubeClient,
		auditTrail: auditTrail,
		logger:     logging.NewLogger(loggerName),
	}
}

// Validate runs the full permission-validation chain for a given resource.
// name and namespace are the Kubernetes object metadata values.
// serviceAccountName is the SA referenced by the spec.
func (v *PermissionValidator) Validate(
	ctx context.Context,
	name, namespace, serviceAccountName string,
	spec SpecFacade,
) error {
	startTime := time.Now()

	correlationID := logging.GenerateCorrelationID(namespace, name, spec.Action())
	ctx = logging.WithCorrelationID(ctx, correlationID)
	ctx = logging.WithJobName(ctx, name)
	ctx = logging.WithNamespace(ctx, namespace)
	ctx = logging.WithAction(ctx, spec.Action())

	kind := spec.ResourceKind()

	v.logger.Debug(ctx, "Starting validation",
		"resource", name,
		"serviceAccount", serviceAccountName,
	)
	klog.InfoS("Validating "+kind, "name", name, "namespace", namespace)

	// ── ServiceAccount lookup ───────────────────────────────────────────────
	v.logger.Debug(ctx, "Fetching ServiceAccount", "serviceAccount", serviceAccountName)

	sa, err := v.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
	if err != nil {
		reason := fmt.Sprintf("failed to get ServiceAccount %s: %v", serviceAccountName, err)
		v.logger.Debug(ctx, "ServiceAccount lookup failed",
			"serviceAccount", serviceAccountName, "error", err.Error(), "decision", "DENY")
		if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, reason); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return fmt.Errorf("failed to get ServiceAccount %s: %w", serviceAccountName, err)
	}

	v.logger.Debug(ctx, "ServiceAccount annotations parsed",
		"allowedActions", GetAnnotation(sa, constants.AnnotationAllowedActions),
		"allowedSourceRepos", GetAnnotation(sa, constants.AnnotationAllowedSourceRepos),
		"allowedSourceRegistries", GetAnnotation(sa, constants.AnnotationAllowedSourceRegistries),
		"allowedSourceBuckets", GetAnnotation(sa, constants.AnnotationAllowedSourceBuckets),
		"allowedPublishRegistries", GetAnnotation(sa, constants.AnnotationAllowedPublishRegistries),
		"allowedPublishBuckets", GetAnnotation(sa, constants.AnnotationAllowedPublishBuckets),
		"allowedDeployTargets", GetAnnotation(sa, constants.AnnotationAllowedDeployTargets),
	)

	// ── Action ─────────────────────────────────────────────────────────────
	v.logger.Debug(ctx, "Checking allowed actions",
		"requestedAction", spec.Action(),
		"allowedActions", GetAnnotation(sa, constants.AnnotationAllowedActions),
	)
	if err := ValidateActionAllowed(sa, spec.Action(), constants.AnnotationAllowedActions); err != nil {
		v.logger.Debug(ctx, "Action validation failed",
			"requestedAction", spec.Action(), "decision", "DENY", "reason", err.Error())
		if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	v.logger.Debug(ctx, "Action validation passed", "action", spec.Action(), "decision", "ALLOW")

	// ── Source ─────────────────────────────────────────────────────────────
	src := spec.Source()
	v.logger.Debug(ctx, "Checking source policy", "sourceType", src.Type)
	if err := validateSourceFacade(sa, src); err != nil {
		v.logger.Debug(ctx, "Source validation failed",
			"sourceType", src.Type, "decision", "DENY", "reason", err.Error())
		if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	v.logger.Debug(ctx, "Source validation passed", "sourceType", src.Type, "decision", "ALLOW")

	// ── ExtraArgs (including preTasks for UDS) ─────────────────────────────
	v.logger.Debug(ctx, "Checking extraArgs for command injection")
	if err := spec.ValidateExtraArgs(); err != nil {
		v.logger.Debug(ctx, "ExtraArgs validation failed",
			"decision", "DENY", "reason", err.Error())
		if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, err.Error()); auditErr != nil {
			klog.V(4).ErrorS(auditErr, "Failed to record audit event")
		}
		return err
	}
	v.logger.Debug(ctx, "ExtraArgs validation passed", "decision", "ALLOW")

	// ── Publish ────────────────────────────────────────────────────────────
	if pub := spec.Publish(); pub != nil {
		v.logger.Debug(ctx, "Checking publish policy", "destinationType", pub.DestinationType)
		if err := validatePublishFacade(sa, *pub); err != nil {
			v.logger.Debug(ctx, "Publish validation failed",
				"destinationType", pub.DestinationType, "decision", "DENY", "reason", err.Error())
			if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, err.Error()); auditErr != nil {
				klog.V(4).ErrorS(auditErr, "Failed to record audit event")
			}
			return err
		}
		v.logger.Debug(ctx, "Publish validation passed", "destinationType", pub.DestinationType, "decision", "ALLOW")
	}

	// ── Deploy ─────────────────────────────────────────────────────────────
	if dep := spec.Deploy(); dep != nil {
		v.logger.Debug(ctx, "Checking deploy policy", "target", dep.Target)
		if err := validateDeployFacade(sa, *dep); err != nil {
			v.logger.Debug(ctx, "Deploy validation failed",
				"target", dep.Target, "decision", "DENY", "reason", err.Error())
			if auditErr := v.auditTrail.RecordJobValidationFailed(ctx, kind, namespace, name, serviceAccountName, err.Error()); auditErr != nil {
				klog.V(4).ErrorS(auditErr, "Failed to record audit event")
			}
			return err
		}
		v.logger.Debug(ctx, "Deploy validation passed", "target", dep.Target, "decision", "ALLOW")
	}

	// ── Success ─────────────────────────────────────────────────────────────
	details := map[string]string{
		"serviceAccount": serviceAccountName,
		"action":         spec.Action(),
	}
	if auditErr := v.auditTrail.RecordJobValidated(ctx, kind, namespace, name, serviceAccountName, details); auditErr != nil {
		klog.V(4).ErrorS(auditErr, "Failed to record audit event")
	}

	v.logger.Debug(ctx, "Validation complete",
		"resource", name, "allowed", true,
		"duration", time.Since(startTime).String(),
	)
	klog.InfoS(kind+" validation passed", "name", name, "namespace", namespace)
	return nil
}

// ── Facade-level validators ─────────────────────────────────────────────────

// validateSourceFacade dispatches to the correct source validator based on SourceFacade.Type.
func validateSourceFacade(sa *corev1.ServiceAccount, src SourceFacade) error {
	switch src.Type {
	case "Git":
		if src.GitURL == "" {
			return fmt.Errorf("git source configuration is required")
		}
		return ValidateGitSource(sa, src.GitURL, constants.AnnotationAllowedSourceRepos)

	case "S3":
		if src.S3Bucket == "" {
			return fmt.Errorf("s3 source configuration is required")
		}
		return ValidateS3Source(sa, src.S3Bucket, constants.AnnotationAllowedSourceBuckets)

	case "OCI":
		if src.OCIReference == "" {
			return fmt.Errorf("oci source configuration is required")
		}
		return ValidateOCISource(sa, src.OCIReference, constants.AnnotationAllowedSourceRegistries)

	case "Local":
		klog.V(4).InfoS("Local source allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown source type: %s", src.Type)
	}
}

// validatePublishFacade dispatches to the correct publish validator based on PublishFacade.DestinationType.
func validatePublishFacade(sa *corev1.ServiceAccount, pub PublishFacade) error {
	switch pub.DestinationType {
	case "S3":
		if pub.S3Bucket == "" {
			return fmt.Errorf("s3 publish destination is required")
		}
		return ValidateS3Publish(sa, pub.S3Bucket, constants.AnnotationAllowedPublishBuckets)

	case "OCI":
		if pub.OCIRegistry == "" || pub.OCIRepository == "" {
			return fmt.Errorf("oci publish destination is required")
		}
		return ValidateOCIPublish(sa, pub.OCIRegistry, pub.OCIRepository, constants.AnnotationAllowedPublishRegistries)

	case "Local":
		klog.V(4).InfoS("Local publish allowed", "serviceAccount", sa.Name)
		return nil

	default:
		return fmt.Errorf("unknown publish destination type: %s", pub.DestinationType)
	}
}

// validateDeployFacade checks the deploy target against the allowed-deploy-targets annotation.
func validateDeployFacade(sa *corev1.ServiceAccount, dep DeployFacade) error {
	return ValidateDeployTarget(sa, dep.Target, constants.AnnotationAllowedDeployTargets)
}
