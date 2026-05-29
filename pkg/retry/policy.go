// Package retry provides job retry logic with configurable exponential backoff and error pattern matching.
package retry

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

// Policy encapsulates retry configuration
type Policy struct {
	MaxRetries        int32
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []*regexp.Regexp
}

// PolicySpec holds the raw field values shared by all CRD RetryPolicy types.
// Callers convert their CRD-specific type into a PolicySpec and call ParsePolicy.
type PolicySpec struct {
	// MaxRetries is the maximum number of retry attempts (nil means use default of 3).
	MaxRetries *int32
	// InitialBackoff is the delay before the first retry (e.g., "30s", "1m"); empty means default.
	InitialBackoff string
	// MaxBackoff is the maximum delay between retries; empty means default.
	MaxBackoff string
	// BackoffMultiplier is the scaled multiplier (value * 100); nil means use default of 2.0.
	BackoffMultiplier *int32
	// RetryableErrors is the list of glob patterns for retryable error messages.
	RetryableErrors []string
}

// ParsePolicy converts a PolicySpec into an internal Policy.
// Returns nil, nil when spec is nil.
func ParsePolicy(spec *PolicySpec) (*Policy, error) {
	if spec == nil {
		return nil, nil
	}

	policy := &Policy{
		MaxRetries:        3, // default
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	if spec.MaxRetries != nil {
		policy.MaxRetries = *spec.MaxRetries
	}

	if spec.InitialBackoff != "" {
		duration, err := time.ParseDuration(spec.InitialBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid initialBackoff: %w", err)
		}
		policy.InitialBackoff = duration
	}

	if spec.MaxBackoff != "" {
		duration, err := time.ParseDuration(spec.MaxBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid maxBackoff: %w", err)
		}
		policy.MaxBackoff = duration
	}

	if spec.BackoffMultiplier != nil {
		policy.BackoffMultiplier = float64(*spec.BackoffMultiplier) / 100.0
	}

	// Compile retryable error patterns
	for _, pattern := range spec.RetryableErrors {
		re, err := compileGlobPattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid retryable error pattern %q: %w", pattern, err)
		}
		policy.RetryableErrors = append(policy.RetryableErrors, re)
	}

	return policy, nil
}

// ShouldRetry determines if error is retryable based on policy
func (p *Policy) ShouldRetry(errorMessage string, currentRetries int32) bool {
	// Check if max retries exhausted
	if currentRetries >= p.MaxRetries {
		return false
	}

	// If no error patterns specified, all errors are retryable
	if len(p.RetryableErrors) == 0 {
		return true
	}

	// Check if error matches any retryable pattern
	for _, pattern := range p.RetryableErrors {
		if pattern.MatchString(errorMessage) {
			return true
		}
	}

	return false
}

// CalculateBackoff computes next retry delay using exponential backoff
func (p *Policy) CalculateBackoff(attemptNumber int32) time.Duration {
	if attemptNumber <= 0 {
		return p.InitialBackoff
	}

	// Calculate exponential backoff: initial * (multiplier ^ attempt)
	backoff := float64(p.InitialBackoff)
	for i := int32(0); i < attemptNumber; i++ {
		backoff *= p.BackoffMultiplier
	}

	duration := time.Duration(backoff)

	// Cap at max backoff
	if duration > p.MaxBackoff {
		return p.MaxBackoff
	}

	return duration
}

// compileGlobPattern converts a glob pattern to a regexp
// Supports * (zero or more characters) and ? (exactly one character)
func compileGlobPattern(pattern string) (*regexp.Regexp, error) {
	// Escape special regex characters except * and ?
	escaped := regexp.QuoteMeta(pattern)

	// Replace escaped glob wildcards with regex equivalents
	// \* was created by QuoteMeta from *, convert to .*
	escaped = regexp.MustCompile(`\\\*`).ReplaceAllString(escaped, ".*")
	// \? was created by QuoteMeta from ?, convert to .
	escaped = regexp.MustCompile(`\\\?`).ReplaceAllString(escaped, ".")

	// Compile with case-insensitive matching
	return regexp.Compile("(?i)" + escaped)
}

// MatchesGlobPattern checks if a string matches a glob pattern
func MatchesGlobPattern(s, pattern string) bool {
	match, err := filepath.Match(pattern, s)
	if err != nil {
		return false
	}
	return match
}
