package logging

import (
	"context"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test-component")
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	if logger.component != "test-component" {
		t.Errorf("Expected component 'test-component', got '%s'", logger.component)
	}
}

func TestWithContext(t *testing.T) {
	logger := NewLogger("test")
	ctx := context.Background()

	tests := []struct {
		name     string
		ctx      context.Context
		expected map[string]string
	}{
		{
			name: "empty context",
			ctx:  ctx,
			expected: map[string]string{
				"component": "test",
			},
		},
		{
			name: "with correlation ID",
			ctx:  WithCorrelationID(ctx, "correlation-123"),
			expected: map[string]string{
				"component":     "test",
				"correlationID": "correlation-123",
			},
		},
		{
			name: "with job name",
			ctx:  WithJobName(ctx, "my-job"),
			expected: map[string]string{
				"component": "test",
				"job":       "my-job",
			},
		},
		{
			name: "with namespace",
			ctx:  WithNamespace(ctx, "default"),
			expected: map[string]string{
				"component": "test",
				"namespace": "default",
			},
		},
		{
			name: "with action",
			ctx:  WithAction(ctx, "Build"),
			expected: map[string]string{
				"component": "test",
				"action":    "Build",
			},
		},
		{
			name: "with all context",
			ctx: WithAction(
				WithNamespace(
					WithJobName(
						WithCorrelationID(ctx, "corr-123"),
						"test-job",
					),
					"prod",
				),
				"Deploy",
			),
			expected: map[string]string{
				"component":     "test",
				"correlationID": "corr-123",
				"job":           "test-job",
				"namespace":     "prod",
				"action":        "Deploy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kvs := logger.WithContext(tt.ctx)

			// Convert to map for easier comparison
			result := make(map[string]string)
			for i := 0; i < len(kvs); i += 2 {
				if i+1 < len(kvs) {
					key := kvs[i].(string)
					value := kvs[i+1].(string)
					result[key] = value
				}
			}

			// Verify all expected keys are present
			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("Expected key '%s' not found in result", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("For key '%s': expected '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify no unexpected keys
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d keys, got %d: %v", len(tt.expected), len(result), result)
			}
		})
	}
}

func TestRedactSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []interface{}
	}{
		{
			name:     "no sensitive data",
			input:    []interface{}{"key", "value", "number", 42},
			expected: []interface{}{"key", "value", "number", 42},
		},
		{
			name:     "password key",
			input:    []interface{}{"password", "secret123", "user", "admin"},
			expected: []interface{}{"password", "***REDACTED***", "user", "admin"},
		},
		{
			name:     "token key",
			input:    []interface{}{"token", "Bearer abc123", "id", "123"},
			expected: []interface{}{"token", "***REDACTED***", "id", "123"},
		},
		{
			name:     "api_key key",
			input:    []interface{}{"api_key", "key-123456"},
			expected: []interface{}{"api_key", "***REDACTED***"},
		},
		{
			name:     "api-key normalized",
			input:    []interface{}{"api-key", "key-123456"},
			expected: []interface{}{"api-key", "***REDACTED***"},
		},
		{
			name:     "apikey normalized",
			input:    []interface{}{"apikey", "key-123456"},
			expected: []interface{}{"apikey", "***REDACTED***"},
		},
		{
			name:     "secret key",
			input:    []interface{}{"secret", "my-secret"},
			expected: []interface{}{"secret", "***REDACTED***"},
		},
		{
			name:     "authorization header",
			input:    []interface{}{"authorization", "Bearer token123"},
			expected: []interface{}{"authorization", "***REDACTED***"},
		},
		{
			name:     "kubeconfig",
			input:    []interface{}{"kubeconfig", "apiVersion: v1..."},
			expected: []interface{}{"kubeconfig", "***REDACTED***"},
		},
		{
			name:     "dockerconfigjson",
			input:    []interface{}{"dockerconfigjson", `{"auths":{}}`},
			expected: []interface{}{"dockerconfigjson", "***REDACTED***"},
		},
		{
			name:     "password pattern in value",
			input:    []interface{}{"config", "password: secret123"},
			expected: []interface{}{"config", "***REDACTED***"},
		},
		{
			name:     "AWS credentials pattern",
			input:    []interface{}{"env", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"}, // pragma: allowlist secret
			expected: []interface{}{"env", "***REDACTED***"},
		},
		{
			name:     "GitHub token pattern",
			input:    []interface{}{"token_value", "ghp_1234567890123456789012345678901234567890"}, // pragma: allowlist secret
			expected: []interface{}{"token_value", "***REDACTED***"},
		},
		{
			name:     "mixed sensitive and non-sensitive",
			input:    []interface{}{"name", "test", "password", "secret", "count", 5, "token", "abc"},
			expected: []interface{}{"name", "test", "password", "***REDACTED***", "count", 5, "token", "***REDACTED***"},
		},
		{
			name:     "odd number of elements",
			input:    []interface{}{"key1", "value1", "key2"},
			expected: []interface{}{"key1", "value1", "key2"},
		},
		{
			name:     "non-string values",
			input:    []interface{}{"count", 42, "enabled", true},
			expected: []interface{}{"count", 42, "enabled", true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveData(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d elements, got %d", len(tt.expected), len(result))
			}

			for i := 0; i < len(result); i += 2 {
				if result[i] != tt.expected[i] {
					t.Errorf("At index %d: expected key '%v', got '%v'", i, tt.expected[i], result[i])
				}
				if i+1 < len(result) {
					if result[i+1] != tt.expected[i+1] {
						t.Errorf("At index %d: expected value '%v', got '%v'", i+1, tt.expected[i+1], result[i+1])
					}
				}
			}
		})
	}
}

func TestCorrelationIDHelpers(t *testing.T) {
	ctx := context.Background()

	// Test adding correlation ID
	correlationID := "test-correlation-123"
	ctx = WithCorrelationID(ctx, correlationID)

	// Test retrieving correlation ID
	retrieved := GetCorrelationID(ctx)
	if retrieved != correlationID {
		t.Errorf("Expected correlation ID '%s', got '%s'", correlationID, retrieved)
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyID := GetCorrelationID(emptyCtx)
	if emptyID != "" {
		t.Errorf("Expected empty correlation ID, got '%s'", emptyID)
	}
}

func TestGenerateCorrelationID(t *testing.T) {
	tests := []struct {
		namespace string
		name      string
		action    string
		expected  string
	}{
		{
			namespace: "default",
			name:      "my-job",
			action:    "Build",
			expected:  "default/my-job/Build",
		},
		{
			namespace: "prod",
			name:      "deployment-1",
			action:    "Deploy",
			expected:  "prod/deployment-1/Deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GenerateCorrelationID(tt.namespace, tt.name, tt.action)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test WithJobName
	ctx = WithJobName(ctx, "test-job")
	jobName, ok := ctx.Value(JobNameKey).(string)
	if !ok || jobName != "test-job" {
		t.Errorf("Expected job name 'test-job', got '%v'", jobName)
	}

	// Test WithNamespace
	ctx = WithNamespace(ctx, "test-namespace")
	namespace, ok := ctx.Value(NamespaceKey).(string)
	if !ok || namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%v'", namespace)
	}

	// Test WithAction
	ctx = WithAction(ctx, "Build")
	action, ok := ctx.Value(ActionKey).(string)
	if !ok || action != "Build" {
		t.Errorf("Expected action 'Build', got '%v'", action)
	}
}

func TestSensitivePatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "AWS access key",
			input:       "aws_access_key_id=AKIAIOSFODNN7EXAMPLE", // pragma: allowlist secret
			shouldMatch: true,
		},
		{
			name:        "AWS secret key",
			input:       "AWS_SECRET_ACCESS_KEY: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", // pragma: allowlist secret
			shouldMatch: true,
		},
		{
			name:        "password field",
			input:       "password: mySecretPassword123",
			shouldMatch: true,
		},
		{
			name:        "token field",
			input:       "token=\"abc123def456\"",
			shouldMatch: true,
		},
		{
			name:        "api_key field",
			input:       "api_key: sk-1234567890abcdef",
			shouldMatch: true,
		},
		{
			name:        "GitHub personal access token",
			input:       "ghp_1234567890abcdefghijklmnopqrstuvwxyz1234567890", // pragma: allowlist secret
			shouldMatch: true,
		},
		{
			name:        "Docker auth",
			input:       `"auth":"dXNlcjpwYXNzd29yZA=="`, // pragma: allowlist secret
			shouldMatch: true,
		},
		{
			name:        "normal text",
			input:       "This is just normal log text",
			shouldMatch: false,
		},
		{
			name:        "URL without credentials",
			input:       "https://example.com/api/v1/users",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := false
			for _, pattern := range sensitivePatterns {
				if pattern.MatchString(tt.input) {
					matched = true
					break
				}
			}

			if matched != tt.shouldMatch {
				if tt.shouldMatch {
					t.Errorf("Expected pattern to match '%s', but it didn't", tt.input)
				} else {
					t.Errorf("Expected pattern not to match '%s', but it did", tt.input)
				}
			}
		})
	}
}

func TestSensitiveKeys(t *testing.T) {
	tests := []struct {
		key         string
		shouldMatch bool
	}{
		{"password", true},
		{"secret", true},
		{"token", true},
		{"apikey", true},
		{"api_key", true},
		{"api-key", true}, // Should match after normalization
		{"access_key", true},
		{"access-key", true}, // Should match after normalization
		{"secret_key", true},
		{"secret-key", true}, // Should match after normalization
		{"credentials", true},
		{"auth", true},
		{"authorization", true},
		{"bearer", true},
		{"ssh_key", true},
		{"ssh-key", true}, // Should match after normalization
		{"private_key", true},
		{"private-key", true}, // Should match after normalization
		{"certificate", true},
		{"kubeconfig", true},
		{"dockerconfigjson", true},
		{"username", false},
		{"name", false},
		{"namespace", false},
		{"action", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			// Normalize the key the same way the redaction function does
			normalizedKey := strings.ToLower(strings.ReplaceAll(tt.key, "-", "_"))
			matched := sensitiveKeys[normalizedKey]

			if matched != tt.shouldMatch {
				if tt.shouldMatch {
					t.Errorf("Expected key '%s' to be sensitive, but it wasn't", tt.key)
				} else {
					t.Errorf("Expected key '%s' not to be sensitive, but it was", tt.key)
				}
			}
		})
	}
}
