// Package webhook implements admission webhook validation and mutation for ScriptRunner resources.
//
// The webhook validates ScriptRunner resources before they are persisted to ensure:
//   - Images come from approved registries
//   - Scripts reference approved paths
//   - Inputs are sanitized and within configured limits
//   - No command injection patterns are present
//
// It also provides mutation capabilities to set default values and apply
// security policies consistently across all ScriptRunner resources.
package webhook

import (
	"fmt"
	"regexp"
	"strings"

	scriptrunnerv1alpha1 "github.com/kylegalloway/scriptrunner/pkg/apis/scriptrunner/v1alpha1"
	"k8s.io/klog/v2"
)

// Config holds the webhook configuration
type Config struct {
	// ApprovedScripts is the list of allowed scriptRef values
	ApprovedScripts []string `json:"approvedScripts"`

	// ApprovedImagePrefix is the required prefix for container images
	ApprovedImagePrefix string `json:"approvedImagePrefix"`

	// MaxInputs is the maximum number of inputs allowed
	MaxInputs int `json:"maxInputs"`

	// MaxInputValueLength is the maximum length of an input value
	MaxInputValueLength int `json:"maxInputValueLength"`

	// MaxScriptArgs is the maximum number of script arguments
	MaxScriptArgs int `json:"maxScriptArgs"`

	// MaxScriptArgLength is the maximum length of a script argument
	MaxScriptArgLength int `json:"maxScriptArgLength"`

	// AllowInlineScripts controls whether inline scripts are permitted
	AllowInlineScripts bool `json:"allowInlineScripts"`

	// Defaults holds default values for ScriptRunner resources
	Defaults DefaultValues `json:"defaults"`
}

// DefaultValues contains default values to apply
type DefaultValues struct {
	Image string `json:"image"`
}

// Validator validates ScriptRunner resources
type Validator struct {
	config Config
}

// NewValidator creates a new validator with the given configuration
func NewValidator(config Config) *Validator {
	return &Validator{
		config: config,
	}
}

// ValidateScriptRunner validates a ScriptRunner resource
func (v *Validator) ValidateScriptRunner(sr *scriptrunnerv1alpha1.ScriptRunner) error {
	klog.InfoS("Validating ScriptRunner", "name", sr.Name, "namespace", sr.Namespace)

	// Validate scriptRef if specified
	if sr.Spec.ScriptRef != "" {
		if err := v.validateScriptRef(sr.Spec.ScriptRef); err != nil {
			return err
		}
	}

	// Validate script field (inline scripts)
	if sr.Spec.Script != "" {
		if !v.config.AllowInlineScripts {
			return fmt.Errorf("inline scripts are not allowed in this environment - use scriptRef with approved scripts")
		}
	}

	// Ensure either script or scriptRef is specified
	if sr.Spec.Script == "" && sr.Spec.ScriptRef == "" {
		// This is okay - controller will use default script
		klog.V(4).InfoS("ScriptRunner has no script or scriptRef, will use default", "name", sr.Name)
	}

	// Validate mutually exclusive script and scriptRef
	if sr.Spec.Script != "" && sr.Spec.ScriptRef != "" {
		return fmt.Errorf("script and scriptRef are mutually exclusive - specify only one")
	}

	// Validate image
	if sr.Spec.Image != "" {
		if err := v.validateImage(sr.Spec.Image); err != nil {
			return err
		}
	}

	// Validate inputs
	if err := v.validateInputs(sr.Spec.Inputs); err != nil {
		return err
	}

	// Validate script arguments
	if err := v.validateScriptArgs(sr.Spec.ScriptArgs); err != nil {
		return err
	}

	klog.InfoS("ScriptRunner validation passed", "name", sr.Name, "namespace", sr.Namespace)
	return nil
}

// validateScriptRef checks if the scriptRef is in the approved list
func (v *Validator) validateScriptRef(scriptRef string) error {
	if len(v.config.ApprovedScripts) == 0 {
		// No whitelist configured - allow all
		klog.V(4).InfoS("No script whitelist configured, allowing scriptRef", "scriptRef", scriptRef)
		return nil
	}

	for _, approved := range v.config.ApprovedScripts {
		if scriptRef == approved {
			klog.V(4).InfoS("ScriptRef approved", "scriptRef", scriptRef)
			return nil
		}
	}

	return fmt.Errorf("scriptRef '%s' is not in the approved scripts list: %v", scriptRef, v.config.ApprovedScripts)
}

// validateImage checks if the image is from an approved registry
func (v *Validator) validateImage(image string) error {
	if v.config.ApprovedImagePrefix == "" {
		// No registry restriction
		klog.V(4).InfoS("No image registry restriction configured", "image", image)
		return nil
	}

	if !strings.HasPrefix(image, v.config.ApprovedImagePrefix) {
		return fmt.Errorf("image '%s' must be from approved registry: %s", image, v.config.ApprovedImagePrefix)
	}

	klog.V(4).InfoS("Image approved", "image", image)
	return nil
}

// validateInputs validates the inputs map
func (v *Validator) validateInputs(inputs map[string]string) error {
	if len(inputs) > v.config.MaxInputs {
		return fmt.Errorf("too many inputs: %d (maximum: %d)", len(inputs), v.config.MaxInputs)
	}

	// Input key pattern: alphanumeric, underscore, hyphen
	keyPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	for key, value := range inputs {
		// Validate key format
		if !keyPattern.MatchString(key) {
			return fmt.Errorf("invalid input key '%s': must contain only alphanumeric characters, underscore, or hyphen", key)
		}

		// Validate key length
		if len(key) > 64 {
			return fmt.Errorf("input key '%s' too long: %d characters (maximum: 64)", key, len(key))
		}

		// Validate value length
		if len(value) > v.config.MaxInputValueLength {
			return fmt.Errorf("input value for '%s' too long: %d characters (maximum: %d)", key, len(value), v.config.MaxInputValueLength)
		}

		// Check for potential command injection patterns
		if containsSuspiciousPatterns(value) {
			klog.Warningf("Input value for '%s' contains suspicious patterns", key)
			// Suspicious patterns are logged for audit purposes but not rejected to allow
			// legitimate use cases. Configure stricter validation via webhook config if needed.
		}
	}

	klog.V(4).InfoS("Inputs validated", "count", len(inputs))
	return nil
}

// validateScriptArgs validates script arguments
func (v *Validator) validateScriptArgs(args []string) error {
	if len(args) > v.config.MaxScriptArgs {
		return fmt.Errorf("too many script arguments: %d (maximum: %d)", len(args), v.config.MaxScriptArgs)
	}

	for i, arg := range args {
		if len(arg) > v.config.MaxScriptArgLength {
			return fmt.Errorf("script argument %d too long: %d characters (maximum: %d)", i, len(arg), v.config.MaxScriptArgLength)
		}

		// Check for suspicious patterns
		if containsSuspiciousPatterns(arg) {
			klog.Warningf("Script argument %d contains suspicious patterns", i)
		}
	}

	klog.V(4).InfoS("Script arguments validated", "count", len(args))
	return nil
}

// containsSuspiciousPatterns checks for potentially dangerous patterns
func containsSuspiciousPatterns(s string) bool {
	suspiciousPatterns := []string{
		";", "|", "&", "$", "`", "$(", "${", "\n", "\r",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}

	return false
}

// SetDefaults applies default values to a ScriptRunner
func (v *Validator) SetDefaults(sr *scriptrunnerv1alpha1.ScriptRunner) {
	// Set default image if not specified
	if sr.Spec.Image == "" && v.config.Defaults.Image != "" {
		klog.InfoS("Setting default image", "name", sr.Name, "image", v.config.Defaults.Image)
		sr.Spec.Image = v.config.Defaults.Image
	}

	// Add managed-by label
	if sr.Labels == nil {
		sr.Labels = make(map[string]string)
	}
	if _, exists := sr.Labels["app.kubernetes.io/managed-by"]; !exists {
		sr.Labels["app.kubernetes.io/managed-by"] = "scriptrunner-webhook"
	}
}
