package retry

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tracker manages retry state for operations
type Tracker struct {
	policy *Policy
}

// RetryDecision indicates whether to retry and when
type RetryDecision struct {
	ShouldRetry bool
	RetryAt     time.Time
	Reason      string
}

// NewTracker creates a new Tracker with the given policy
func NewTracker(policy *Policy) *Tracker {
	return &Tracker{
		policy: policy,
	}
}

// RecordFailure updates retry state after a failure and decides whether to retry
func (t *Tracker) RecordFailure(_ context.Context, currentRetryCount int32, errorMessage string) (*RetryDecision, error) {
	if t.policy == nil {
		return &RetryDecision{
			ShouldRetry: false,
			Reason:      "no retry policy configured",
		}, nil
	}

	// Check if we should retry based on policy
	shouldRetry := t.policy.ShouldRetry(errorMessage, currentRetryCount)

	if !shouldRetry {
		// Determine reason for not retrying
		reason := "max retries exhausted"
		if currentRetryCount < t.policy.MaxRetries && len(t.policy.RetryableErrors) > 0 {
			reason = "error not retryable (does not match retryable patterns)"
		}

		return &RetryDecision{
			ShouldRetry: false,
			Reason:      reason,
		}, nil
	}

	// Calculate backoff for next retry
	backoff := t.policy.CalculateBackoff(currentRetryCount)
	retryAt := time.Now().Add(backoff)

	return &RetryDecision{
		ShouldRetry: true,
		RetryAt:     retryAt,
		Reason: fmt.Sprintf("retry attempt %d/%d scheduled in %s",
			currentRetryCount+1, t.policy.MaxRetries, backoff),
	}, nil
}

// BuildRetryStatus creates the status map for a retry decision
func BuildRetryStatus(decision *RetryDecision, currentRetryCount int32, errorMessage string) map[string]interface{} {
	status := map[string]interface{}{
		"state":             "Retrying",
		"retryCount":        currentRetryCount + 1,
		"lastFailureReason": errorMessage,
	}

	if decision.ShouldRetry {
		retryTime := metav1.NewTime(decision.RetryAt)
		status["nextRetryTime"] = retryTime.Format(time.RFC3339)
	}

	return status
}

// ShouldRetryNow checks if the retry time has been reached
func ShouldRetryNow(nextRetryTime *metav1.Time) bool {
	if nextRetryTime == nil {
		return true // No specific time set, retry immediately
	}

	return time.Now().After(nextRetryTime.Time)
}

// ExtractRetryCount safely extracts retry count from operation status
func ExtractRetryCount(opStatus map[string]interface{}, statusField string) int32 {
	if opStatus == nil {
		return 0
	}

	statusMap, ok := opStatus[statusField].(map[string]interface{})
	if !ok {
		return 0
	}

	retryCount, ok := statusMap["retryCount"].(int32)
	if !ok {
		// Try float64 (JSON unmarshaling default for numbers)
		retryCountFloat, ok := statusMap["retryCount"].(float64)
		if ok {
			return int32(retryCountFloat)
		}
		return 0
	}

	return retryCount
}
