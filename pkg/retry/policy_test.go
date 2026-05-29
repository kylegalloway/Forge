package retry

import (
	"regexp"
	"testing"
	"time"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
)

// ptr32 returns a pointer to an int32 value.
func ptr32(v int32) *int32 { return &v }

// mustCompileGlob compiles a glob pattern for use in tests, panicking on error.
func mustCompileGlob(pattern string) *regexp.Regexp {
	re, err := compileGlobPattern(pattern)
	if err != nil {
		panic("mustCompileGlob: " + err.Error())
	}
	return re
}

// -------------------------
// ShouldRetry — count gating
// -------------------------

// TestShouldRetry_CountGating verifies that retries are refused once the
// attempt counter reaches MaxRetries.
func TestShouldRetry_CountGating(t *testing.T) {
	p := &Policy{MaxRetries: 3}

	cases := []struct {
		retries int32
		want    bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, false}, // exactly at max → no retry
		{4, false}, // past max → no retry
	}

	for _, tc := range cases {
		got := p.ShouldRetry("any error", tc.retries)
		if got != tc.want {
			t.Errorf("ShouldRetry(%d retries): got %v, want %v", tc.retries, got, tc.want)
		}
	}
}

// TestShouldRetry_ZeroMaxRetries verifies that a policy with MaxRetries=0
// never retries, even on the first failure.
func TestShouldRetry_ZeroMaxRetries(t *testing.T) {
	p := &Policy{MaxRetries: 0}
	if p.ShouldRetry("any error", 0) {
		t.Error("expected ShouldRetry to be false when MaxRetries=0")
	}
}

// TestShouldRetry_NoPatterns_AllRetryable verifies that when no error patterns
// are specified every error is retryable up to max retries.
func TestShouldRetry_NoPatterns_AllRetryable(t *testing.T) {
	p := &Policy{MaxRetries: 5}

	errors := []string{
		"connection refused",
		"timeout",
		"unknown error",
		"",
	}

	for _, e := range errors {
		if !p.ShouldRetry(e, 0) {
			t.Errorf("expected %q to be retryable when no patterns are set", e)
		}
	}
}

// TestShouldRetry_MatchingPattern verifies that errors matching a compiled
// retryable pattern allow a retry.
func TestShouldRetry_MatchingPattern(t *testing.T) {
	p := &Policy{
		MaxRetries:      3,
		RetryableErrors: []*regexp.Regexp{mustCompileGlob("*timeout*")},
	}

	matching := []string{
		"operation timeout exceeded",
		"TIMEOUT", // case-insensitive
		"Timeout: context deadline exceeded",
	}

	for _, e := range matching {
		if !p.ShouldRetry(e, 0) {
			t.Errorf("expected %q to match *timeout* pattern", e)
		}
	}
}

// TestShouldRetry_NonMatchingPattern verifies that errors that do not match
// any retryable pattern are refused even with retries remaining.
func TestShouldRetry_NonMatchingPattern(t *testing.T) {
	p := &Policy{
		MaxRetries:      5,
		RetryableErrors: []*regexp.Regexp{mustCompileGlob("*timeout*")},
	}

	nonMatching := []string{
		"permission denied",
		"image not found",
		"syntax error",
	}

	for _, e := range nonMatching {
		if p.ShouldRetry(e, 0) {
			t.Errorf("expected %q NOT to be retryable against *timeout* pattern", e)
		}
	}
}

// TestShouldRetry_MultiplePatterns verifies that a match against any one
// pattern is sufficient to allow a retry.
func TestShouldRetry_MultiplePatterns(t *testing.T) {
	p := &Policy{
		MaxRetries: 3,
		RetryableErrors: []*regexp.Regexp{
			mustCompileGlob("*timeout*"),
			mustCompileGlob("*throttl*"),
		},
	}

	if !p.ShouldRetry("rate throttled by server", 0) {
		t.Error("expected throttle error to be retryable")
	}
	if p.ShouldRetry("permission denied", 0) {
		t.Error("expected permission denied NOT to be retryable")
	}
}

// -------------------------
// CalculateBackoff
// -------------------------

// TestCalculateBackoff_InitialValue verifies that attempt 0 returns
// InitialBackoff unchanged.
func TestCalculateBackoff_InitialValue(t *testing.T) {
	p := &Policy{
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	got := p.CalculateBackoff(0)
	if got != 30*time.Second {
		t.Errorf("CalculateBackoff(0): got %v, want 30s", got)
	}
}

// TestCalculateBackoff_MultiplierProgression verifies exponential growth:
// attempt 1 → 30s * 2 = 60s, attempt 2 → 30s * 4 = 120s, attempt 3 → 240s.
func TestCalculateBackoff_MultiplierProgression(t *testing.T) {
	p := &Policy{
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        10 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	cases := []struct {
		attempt int32
		want    time.Duration
	}{
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
	}

	for _, tc := range cases {
		got := p.CalculateBackoff(tc.attempt)
		if got != tc.want {
			t.Errorf("CalculateBackoff(%d): got %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// TestCalculateBackoff_MaxCap verifies that backoff never exceeds MaxBackoff.
// With initial=30s, multiplier=2, attempt 3 would be 240s but max is 2m (120s).
func TestCalculateBackoff_MaxCap(t *testing.T) {
	p := &Policy{
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        2 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	got := p.CalculateBackoff(3)
	if got != 2*time.Minute {
		t.Errorf("CalculateBackoff(3) with MaxBackoff=2m: got %v, want 2m", got)
	}
}

// TestCalculateBackoff_NegativeAttempt verifies that a negative attempt
// number is treated the same as attempt 0 and returns InitialBackoff.
func TestCalculateBackoff_NegativeAttempt(t *testing.T) {
	p := &Policy{
		InitialBackoff:    30 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	got := p.CalculateBackoff(-1)
	if got != 30*time.Second {
		t.Errorf("CalculateBackoff(-1): got %v, want 30s", got)
	}
}

// -------------------------
// Glob-to-regex conversion
// -------------------------

// TestCompileGlobPattern_Star verifies that * matches zero or more characters.
func TestCompileGlobPattern_Star(t *testing.T) {
	re := mustCompileGlob("*error*")

	matching := []string{"error", "some error occurred", "ERROR", "an-error-here"}
	for _, s := range matching {
		if !re.MatchString(s) {
			t.Errorf("*error* should match %q", s)
		}
	}

	notMatching := []string{"warning", "info"}
	for _, s := range notMatching {
		if re.MatchString(s) {
			t.Errorf("*error* should NOT match %q", s)
		}
	}
}

// TestCompileGlobPattern_Question verifies that ? matches exactly one character.
func TestCompileGlobPattern_Question(t *testing.T) {
	re := mustCompileGlob("er?or")

	if !re.MatchString("error") {
		t.Error("er?or should match 'error'")
	}
	if !re.MatchString("er1or") {
		t.Error("er?or should match 'er1or'")
	}
	// Zero characters in the ? slot should not match
	if re.MatchString("eror") {
		t.Error("er?or should NOT match 'eror' (zero chars in ? slot)")
	}
}

// TestCompileGlobPattern_SpecialRegexChars verifies that regex metacharacters
// in the pattern are escaped and treated as literals.
func TestCompileGlobPattern_SpecialRegexChars(t *testing.T) {
	re := mustCompileGlob("error (code: 42)")

	if !re.MatchString("error (code: 42)") {
		t.Error("literal parentheses should match as-is")
	}
	// Without proper escaping the ( and ) would be regex grouping operators
	if re.MatchString("error code 42") {
		t.Error("parentheses should be literals, not regex grouping")
	}
}

// TestCompileGlobPattern_DotLiteral verifies that . in a pattern is treated
// as a literal dot rather than the regex any-character wildcard.
func TestCompileGlobPattern_DotLiteral(t *testing.T) {
	re := mustCompileGlob("v1.2.3")

	if !re.MatchString("v1.2.3") {
		t.Error("literal dot should match 'v1.2.3'")
	}
	// "v1X2Y3" must NOT match because . is not a wildcard
	if re.MatchString("v1X2Y3") {
		t.Error("dot should be a literal, not a wildcard — 'v1X2Y3' should not match 'v1.2.3'")
	}
}

// TestCompileGlobPattern_CaseInsensitive verifies that compiled patterns
// match regardless of case.
func TestCompileGlobPattern_CaseInsensitive(t *testing.T) {
	re := mustCompileGlob("*Timeout*")

	cases := []string{"timeout", "TIMEOUT", "Timeout", "a Timeout in progress"}
	for _, s := range cases {
		if !re.MatchString(s) {
			t.Errorf("*Timeout* (case-insensitive) should match %q", s)
		}
	}
}

// TestCompileGlobPattern_ValidPattern verifies that well-formed patterns
// compile without error.
func TestCompileGlobPattern_ValidPattern(t *testing.T) {
	_, err := compileGlobPattern("valid*pattern?here")
	if err != nil {
		t.Errorf("expected no error for valid pattern, got: %v", err)
	}
}

// -------------------------
// ParseZarfPolicy
// -------------------------

// TestParseZarfPolicy_Nil verifies that a nil API policy returns nil, nil.
func TestParseZarfPolicy_Nil(t *testing.T) {
	p, err := ParseZarfPolicy(nil)
	if err != nil || p != nil {
		t.Errorf("ParseZarfPolicy(nil): got (%v, %v), want (nil, nil)", p, err)
	}
}

// TestParseZarfPolicy_Defaults verifies that an empty API policy populates
// the expected default values.
func TestParseZarfPolicy_Defaults(t *testing.T) {
	apiPolicy := &zarfv1alpha3.RetryPolicy{}
	p, err := ParseZarfPolicy(apiPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", p.MaxRetries)
	}
	if p.InitialBackoff != 30*time.Second {
		t.Errorf("InitialBackoff: got %v, want 30s", p.InitialBackoff)
	}
	if p.MaxBackoff != 5*time.Minute {
		t.Errorf("MaxBackoff: got %v, want 5m", p.MaxBackoff)
	}
	if p.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier: got %v, want 2.0", p.BackoffMultiplier)
	}
	if len(p.RetryableErrors) != 0 {
		t.Errorf("RetryableErrors: got %d patterns, want 0", len(p.RetryableErrors))
	}
}

// TestParseZarfPolicy_CustomValues verifies that explicit API fields override
// the defaults.
func TestParseZarfPolicy_CustomValues(t *testing.T) {
	multiplier := int32(300) // 3.0x (value / 100)
	apiPolicy := &zarfv1alpha3.RetryPolicy{
		MaxRetries:        ptr32(5),
		InitialBackoff:    "10s",
		MaxBackoff:        "2m",
		BackoffMultiplier: &multiplier,
		RetryableErrors:   []string{"*timeout*", "*throttl*"},
	}

	p, err := ParseZarfPolicy(apiPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d, want 5", p.MaxRetries)
	}
	if p.InitialBackoff != 10*time.Second {
		t.Errorf("InitialBackoff: got %v, want 10s", p.InitialBackoff)
	}
	if p.MaxBackoff != 2*time.Minute {
		t.Errorf("MaxBackoff: got %v, want 2m", p.MaxBackoff)
	}
	if p.BackoffMultiplier != 3.0 {
		t.Errorf("BackoffMultiplier: got %v, want 3.0", p.BackoffMultiplier)
	}
	if len(p.RetryableErrors) != 2 {
		t.Errorf("RetryableErrors length: got %d, want 2", len(p.RetryableErrors))
	}
}

// TestParseZarfPolicy_InvalidInitialBackoff verifies that a malformed
// initialBackoff duration string surfaces as an error.
func TestParseZarfPolicy_InvalidInitialBackoff(t *testing.T) {
	apiPolicy := &zarfv1alpha3.RetryPolicy{InitialBackoff: "not-a-duration"}
	_, err := ParseZarfPolicy(apiPolicy)
	if err == nil {
		t.Error("expected error for invalid initialBackoff duration")
	}
}

// TestParseZarfPolicy_InvalidMaxBackoff verifies that a malformed maxBackoff
// duration string surfaces as an error.
func TestParseZarfPolicy_InvalidMaxBackoff(t *testing.T) {
	apiPolicy := &zarfv1alpha3.RetryPolicy{MaxBackoff: "???"}
	_, err := ParseZarfPolicy(apiPolicy)
	if err == nil {
		t.Error("expected error for invalid maxBackoff duration")
	}
}

// TestParseZarfPolicy_RetryableErrors_Compiled verifies that compiled error
// patterns actually work end-to-end: a timeout error is retryable but a
// permission error is not.
func TestParseZarfPolicy_RetryableErrors_Compiled(t *testing.T) {
	apiPolicy := &zarfv1alpha3.RetryPolicy{
		RetryableErrors: []string{"*timeout*"},
	}
	p, err := ParseZarfPolicy(apiPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !p.ShouldRetry("connection timeout", 0) {
		t.Error("expected timeout to be retryable")
	}
	if p.ShouldRetry("permission denied", 0) {
		t.Error("expected permission denied NOT to be retryable")
	}
}

// -------------------------
// ParseUDSPolicy
// -------------------------

// TestParseUDSPolicy_Nil verifies that a nil UDS policy returns nil, nil.
func TestParseUDSPolicy_Nil(t *testing.T) {
	p, err := ParseUDSPolicy(nil)
	if err != nil || p != nil {
		t.Errorf("ParseUDSPolicy(nil): got (%v, %v), want (nil, nil)", p, err)
	}
}

// TestParseUDSPolicy_Defaults mirrors the Zarf defaults test for the UDS path.
func TestParseUDSPolicy_Defaults(t *testing.T) {
	apiPolicy := &udsv1alpha3.RetryPolicy{}
	p, err := ParseUDSPolicy(apiPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", p.MaxRetries)
	}
	if p.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier: got %v, want 2.0", p.BackoffMultiplier)
	}
}

// TestParseUDSPolicy_CustomValues verifies that UDS-specific policy fields
// are parsed with the same logic as the Zarf path.
func TestParseUDSPolicy_CustomValues(t *testing.T) {
	multiplier := int32(150) // 1.5x
	apiPolicy := &udsv1alpha3.RetryPolicy{
		MaxRetries:        ptr32(2),
		InitialBackoff:    "5s",
		MaxBackoff:        "1m",
		BackoffMultiplier: &multiplier,
		RetryableErrors:   []string{"*timeout*"},
	}

	p, err := ParseUDSPolicy(apiPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.MaxRetries != 2 {
		t.Errorf("MaxRetries: got %d, want 2", p.MaxRetries)
	}
	if p.InitialBackoff != 5*time.Second {
		t.Errorf("InitialBackoff: got %v, want 5s", p.InitialBackoff)
	}
	if p.BackoffMultiplier != 1.5 {
		t.Errorf("BackoffMultiplier: got %v, want 1.5", p.BackoffMultiplier)
	}
}

// TestParseUDSPolicy_InvalidBackoff verifies that malformed durations in the
// UDS path surface as errors.
func TestParseUDSPolicy_InvalidBackoff(t *testing.T) {
	apiPolicy := &udsv1alpha3.RetryPolicy{InitialBackoff: "not-a-duration"}
	_, err := ParseUDSPolicy(apiPolicy)
	if err == nil {
		t.Error("expected error for invalid initialBackoff duration")
	}
}
