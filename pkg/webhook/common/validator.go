// Package common provides shared validation utilities for webhook validators.
//
// This package consolidates duplicated validation logic that was previously
// in both zarfpackage_validator.go and udsbundle_validator.go.
package common

import (
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// GetAnnotation safely retrieves an annotation value from a ServiceAccount.
func GetAnnotation(sa *corev1.ServiceAccount, key string) string {
	if sa.Annotations == nil {
		return ""
	}
	return sa.Annotations[key]
}

// MatchesGlob checks if a string matches a glob pattern.
// Supports both filepath glob patterns and simple prefix matching with *.
func MatchesGlob(s, pattern string) bool {
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

// ValidateActionAllowed checks if an action is allowed by ServiceAccount annotations.
func ValidateActionAllowed(sa *corev1.ServiceAccount, action string, annotationKey string) error {
	allowedActions := GetAnnotation(sa, annotationKey)
	if allowedActions == "" {
		return fmt.Errorf("ServiceAccount %s has no %s annotation", sa.Name, annotationKey)
	}

	actions := strings.Split(allowedActions, ",")
	for _, allowed := range actions {
		if strings.TrimSpace(allowed) == action {
			klog.V(4).InfoS("Action allowed", "action", action, "serviceAccount", sa.Name)
			return nil
		}
	}

	return fmt.Errorf("action %s is not allowed by ServiceAccount %s (allowed: %s)", action, sa.Name, allowedActions)
}

// ValidateGlobPattern validates that a value matches one of the allowed glob patterns.
// Returns nil if validation passes, error otherwise.
func ValidateGlobPattern(sa *corev1.ServiceAccount, value string, annotationKey string, resourceType string) error {
	allowedPatterns := GetAnnotation(sa, annotationKey)
	if allowedPatterns == "" {
		return fmt.Errorf("ServiceAccount %s has no %s annotation", sa.Name, annotationKey)
	}

	patterns := strings.Split(allowedPatterns, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || MatchesGlob(value, pattern) {
			klog.V(4).InfoS("Resource allowed", "type", resourceType, "value", value, "pattern", pattern)
			return nil
		}
	}

	return fmt.Errorf("%s %s is not allowed by ServiceAccount %s (allowed patterns: %s)",
		resourceType, value, sa.Name, allowedPatterns)
}

// ValidateGitSource validates Git source URL against allowed patterns.
func ValidateGitSource(sa *corev1.ServiceAccount, gitURL string, annotationKey string) error {
	return ValidateGlobPattern(sa, gitURL, annotationKey, "git repository")
}

// ValidateS3Source validates S3 bucket against allowed patterns.
func ValidateS3Source(sa *corev1.ServiceAccount, bucket string, annotationKey string) error {
	return ValidateGlobPattern(sa, bucket, annotationKey, "S3 bucket")
}

// ValidateOCISource validates OCI image against allowed patterns.
func ValidateOCISource(sa *corev1.ServiceAccount, image string, annotationKey string) error {
	return ValidateGlobPattern(sa, image, annotationKey, "OCI image")
}

// ValidateS3Publish validates S3 publish bucket against allowed patterns.
func ValidateS3Publish(sa *corev1.ServiceAccount, bucket string, annotationKey string) error {
	return ValidateGlobPattern(sa, bucket, annotationKey, "S3 publish bucket")
}

// ValidateOCIPublish validates OCI publish registry/repository against allowed patterns.
func ValidateOCIPublish(sa *corev1.ServiceAccount, registry, repository string, annotationKey string) error {
	ociRef := fmt.Sprintf("%s/%s", registry, repository)
	return ValidateGlobPattern(sa, ociRef, annotationKey, "OCI registry")
}

// ValidateDeployTarget validates deploy target against allowed targets.
func ValidateDeployTarget(sa *corev1.ServiceAccount, target string, annotationKey string) error {
	allowedTargets := GetAnnotation(sa, annotationKey)
	if allowedTargets == "" {
		return fmt.Errorf("ServiceAccount %s has no %s annotation", sa.Name, annotationKey)
	}

	targets := strings.Split(allowedTargets, ",")
	for _, allowed := range targets {
		if strings.TrimSpace(allowed) == target {
			klog.V(4).InfoS("Deploy target allowed", "target", target, "serviceAccount", sa.Name)
			return nil
		}
	}

	return fmt.Errorf("deploy target %s is not allowed by ServiceAccount %s (allowed: %s)",
		target, sa.Name, allowedTargets)
}
