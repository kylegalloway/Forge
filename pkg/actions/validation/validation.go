// Package validation provides extraArgs validation for CLI command building.
package validation

import (
	"fmt"
	"strings"

	"github.com/kylegalloway/forge/pkg/apis/common"
)

// ValidateExtraArgs ensures extra args don't contain shell metacharacters
// that could be used for command injection.
func ValidateExtraArgs(args []string) error {
	// Forbidden characters that could enable command injection
	forbidden := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\n", "\r"}

	for _, arg := range args {
		for _, char := range forbidden {
			if strings.Contains(arg, char) {
				return fmt.Errorf("extraArgs contains forbidden character %q: %s", char, arg)
			}
		}
	}
	return nil
}

// ShellEscape escapes a string for safe use in shell commands.
// This provides an additional layer of safety when appending extraArgs.
func ShellEscape(s string) string {
	// If the string contains no special characters, return as-is
	if !strings.ContainsAny(s, " \t\n\r'\"\\") {
		return s
	}
	// Otherwise, wrap in single quotes and escape any single quotes within
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// AppendExtraArgs safely appends extra arguments to a command string.
// Each argument is validated and shell-escaped before appending.
func AppendExtraArgs(cmd string, extraArgs []string) (string, error) {
	if err := ValidateExtraArgs(extraArgs); err != nil {
		return "", err
	}

	for _, arg := range extraArgs {
		cmd = fmt.Sprintf("%s %s", cmd, ShellEscape(arg))
	}
	return cmd, nil
}

// reservedMountPaths contains mount paths that cannot be used by extraMounts.
var reservedMountPaths = map[string]bool{
	"/workspace":         true,
	"/output":            true,
	"/artifacts":         true,
	"/etc/kubeconfig":    true,
	"/etc/git-secret":    true,
	"/home/zarf":         true,
	"/home/uds":          true,
	"/home/zarf/.docker": true,
	"/home/uds/.docker":  true,
	"/tmp":               true,
	"/var/run/secrets/kubernetes.io/serviceaccount": true,
}

// ValidateExtraMounts validates a merged list of extra mounts.
func ValidateExtraMounts(mounts []common.ExtraMount) error {
	seenPaths := make(map[string]bool)
	for i, mount := range mounts {
		// Exactly one ref must be set
		if mount.ConfigMapRef == nil && mount.SecretRef == nil { // pragma: allowlist secret
			return fmt.Errorf("extraMounts[%d]: exactly one of configMapRef or secretRef must be set", i)
		}
		if mount.ConfigMapRef != nil && mount.SecretRef != nil { // pragma: allowlist secret
			return fmt.Errorf("extraMounts[%d]: only one of configMapRef or secretRef may be set", i)
		}

		// MountPath must be absolute
		if !strings.HasPrefix(mount.MountPath, "/") {
			return fmt.Errorf("extraMounts[%d]: mountPath must be absolute, got %q", i, mount.MountPath)
		}

		// MountPath must not be reserved
		if reservedMountPaths[mount.MountPath] {
			return fmt.Errorf("extraMounts[%d]: mountPath %q is reserved by the system", i, mount.MountPath)
		}

		// No duplicate mount paths
		if seenPaths[mount.MountPath] {
			return fmt.Errorf("extraMounts[%d]: duplicate mountPath %q", i, mount.MountPath)
		}
		seenPaths[mount.MountPath] = true
	}
	return nil
}

// MergeExtraMounts combines top-level and per-action extra mounts.
// Returns an error if there are duplicate mount paths.
func MergeExtraMounts(topLevel, perAction []common.ExtraMount) ([]common.ExtraMount, error) {
	merged := make([]common.ExtraMount, 0, len(topLevel)+len(perAction))
	merged = append(merged, topLevel...)
	merged = append(merged, perAction...)
	if err := ValidateExtraMounts(merged); err != nil {
		return nil, err
	}
	return merged, nil
}
