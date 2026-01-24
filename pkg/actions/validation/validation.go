// Package validation provides extraArgs validation for CLI command building.
package validation

import (
	"fmt"
	"strings"
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
