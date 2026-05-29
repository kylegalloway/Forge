package retry

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kylegalloway/forge/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -------------------------
// Tracker — state transitions
// -------------------------

// TestTracker_NilPolicy verifies that a Tracker without a policy always
// decides not to retry.
func TestTracker_NilPolicy(t *testing.T) {
	tracker := NewTracker(nil)
	decision, err := tracker.RecordFailure(context.Background(), 0, "some error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.ShouldRetry {
		t.Error("Tracker with nil policy should never retry")
	}
	if !strings.Contains(decision.Reason, "no retry policy") {
		t.Errorf("expected 'no retry policy' in reason, got: %q", decision.Reason)
	}
}

// TestTracker_FreshTracker_ShouldRetry verifies that the first failure on a
// fresh tracker with headroom returns ShouldRetry=true and a future RetryAt.
func TestTracker_FreshTracker_ShouldRetry(t *testing.T) {
	policy := &Policy{
		MaxRetries:        3,
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
	tracker := NewTracker(policy)

	before := time.Now()
	decision, err := tracker.RecordFailure(context.Background(), 0, "transient error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decision.ShouldRetry {
		t.Errorf("expected ShouldRetry=true on first failure, reason: %s", decision.Reason)
	}
	if !decision.RetryAt.After(before) {
		t.Error("RetryAt should be in the future")
	}
}

// TestTracker_MaxRetriesReached verifies that once retries are exhausted the
// decision is ShouldRetry=false with an appropriate reason.
func TestTracker_MaxRetriesReached(t *testing.T) {
	policy := &Policy{
		MaxRetries:        3,
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
	tracker := NewTracker(policy)

	decision, err := tracker.RecordFailure(context.Background(), 3, "some error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.ShouldRetry {
		t.Error("expected ShouldRetry=false when max retries reached")
	}
	if !strings.Contains(decision.Reason, "max retries exhausted") {
		t.Errorf("expected 'max retries exhausted' reason, got: %q", decision.Reason)
	}
}

// TestTracker_NonRetryableError verifies that errors not matching any
// retryable pattern produce ShouldRetry=false with a clear reason.
func TestTracker_NonRetryableError(t *testing.T) {
	policy := &Policy{
		MaxRetries:      5,
		RetryableErrors: []*regexp.Regexp{mustCompileGlob("*timeout*")},
	}
	tracker := NewTracker(policy)

	decision, err := tracker.RecordFailure(context.Background(), 0, "permission denied")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.ShouldRetry {
		t.Error("expected ShouldRetry=false for non-matching error")
	}
	if !strings.Contains(decision.Reason, "not retryable") {
		t.Errorf("expected 'not retryable' in reason, got: %q", decision.Reason)
	}
}

// TestTracker_ReasonContainsAttemptInfo verifies that the retry reason string
// includes attempt count and max retries.
func TestTracker_ReasonContainsAttemptInfo(t *testing.T) {
	policy := &Policy{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
	tracker := NewTracker(policy)

	decision, err := tracker.RecordFailure(context.Background(), 1, "transient error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.ShouldRetry {
		t.Fatalf("expected retry, got: %s", decision.Reason)
	}
	// Reason should reference attempt number and max
	if !strings.Contains(decision.Reason, "2/3") {
		t.Errorf("expected '2/3' in reason, got: %q", decision.Reason)
	}
}

// -------------------------
// BuildRetryStatus
// -------------------------

// TestBuildRetryStatus_ShouldRetry verifies that BuildRetryStatus includes
// the next retry time when ShouldRetry is true.
func TestBuildRetryStatus_ShouldRetry(t *testing.T) {
	retryAt := time.Now().Add(30 * time.Second)
	decision := &RetryDecision{
		ShouldRetry: true,
		RetryAt:     retryAt,
		Reason:      "retry attempt 1/3 scheduled in 30s",
	}

	status := BuildRetryStatus(decision, 0, "transient error")

	if status[constants.StatusKeyState] != constants.PhaseRetrying {
		t.Errorf("expected state=%q, got %v", constants.PhaseRetrying, status[constants.StatusKeyState])
	}
	if status[constants.StatusKeyRetryCount] != int32(1) {
		t.Errorf("expected retryCount=1, got %v", status[constants.StatusKeyRetryCount])
	}
	if status[constants.StatusKeyLastFailureReason] != "transient error" {
		t.Errorf("expected lastFailureReason='transient error', got %v", status[constants.StatusKeyLastFailureReason])
	}
	if _, ok := status[constants.StatusKeyNextRetryTime]; !ok {
		t.Error("expected nextRetryTime to be set when ShouldRetry=true")
	}
}

// TestBuildRetryStatus_NoRetry verifies that BuildRetryStatus omits
// nextRetryTime when ShouldRetry is false.
func TestBuildRetryStatus_NoRetry(t *testing.T) {
	decision := &RetryDecision{
		ShouldRetry: false,
		Reason:      "max retries exhausted",
	}

	status := BuildRetryStatus(decision, 3, "fatal error")

	if _, ok := status[constants.StatusKeyNextRetryTime]; ok {
		t.Error("expected nextRetryTime to be absent when ShouldRetry=false")
	}
	if status[constants.StatusKeyRetryCount] != int32(4) {
		t.Errorf("expected retryCount=4 (3+1), got %v", status[constants.StatusKeyRetryCount])
	}
}

// -------------------------
// ShouldRetryNow
// -------------------------

// TestShouldRetryNow_Nil verifies that a nil nextRetryTime means retry
// immediately.
func TestShouldRetryNow_Nil(t *testing.T) {
	if !ShouldRetryNow(nil) {
		t.Error("expected ShouldRetryNow(nil) to return true")
	}
}

// TestShouldRetryNow_Past verifies that a time in the past means retry now.
func TestShouldRetryNow_Past(t *testing.T) {
	past := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	if !ShouldRetryNow(&past) {
		t.Error("expected ShouldRetryNow to return true for past time")
	}
}

// TestShouldRetryNow_Future verifies that a time in the future means do not
// retry yet.
func TestShouldRetryNow_Future(t *testing.T) {
	future := metav1.NewTime(time.Now().Add(1 * time.Hour))
	if ShouldRetryNow(&future) {
		t.Error("expected ShouldRetryNow to return false for future time")
	}
}

// -------------------------
// ExtractRetryCount
// -------------------------

// TestExtractRetryCount_Nil verifies that a nil status map returns 0.
func TestExtractRetryCount_Nil(t *testing.T) {
	if got := ExtractRetryCount(nil, "someField"); got != 0 {
		t.Errorf("expected 0 for nil map, got %d", got)
	}
}

// TestExtractRetryCount_MissingField verifies that a missing status field
// returns 0.
func TestExtractRetryCount_MissingField(t *testing.T) {
	status := map[string]interface{}{
		"otherField": map[string]interface{}{"retryCount": int32(5)},
	}
	if got := ExtractRetryCount(status, "missingField"); got != 0 {
		t.Errorf("expected 0 for missing field, got %d", got)
	}
}

// TestExtractRetryCount_Int32 verifies extraction when retryCount is int32.
func TestExtractRetryCount_Int32(t *testing.T) {
	status := map[string]interface{}{
		"buildStatus": map[string]interface{}{"retryCount": int32(3)},
	}
	got := ExtractRetryCount(status, "buildStatus")
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

// TestExtractRetryCount_Float64 verifies extraction when retryCount is
// float64 (the default type from JSON unmarshalling).
func TestExtractRetryCount_Float64(t *testing.T) {
	status := map[string]interface{}{
		"buildStatus": map[string]interface{}{"retryCount": float64(7)},
	}
	got := ExtractRetryCount(status, "buildStatus")
	if got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
}

// TestExtractRetryCount_MissingRetryCount verifies that a status map without
// a retryCount key returns 0.
func TestExtractRetryCount_MissingRetryCount(t *testing.T) {
	status := map[string]interface{}{
		"buildStatus": map[string]interface{}{"phase": "Running"},
	}
	got := ExtractRetryCount(status, "buildStatus")
	if got != 0 {
		t.Errorf("expected 0 for missing retryCount key, got %d", got)
	}
}
