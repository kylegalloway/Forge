// Package logging provides structured logging utilities for Forge
package logging

import (
	"context"
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

// ContextKey type for context keys to avoid collisions
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation IDs
	CorrelationIDKey ContextKey = "correlationID"
	// JobNameKey is the context key for job names
	JobNameKey ContextKey = "jobName"
	// NamespaceKey is the context key for namespace
	NamespaceKey ContextKey = "namespace"
	// ActionKey is the context key for action type
	ActionKey ContextKey = "action"
)

// Logger provides structured logging with correlation IDs and sensitive data redaction
type Logger struct {
	component string
}

// NewLogger creates a new logger for a component
func NewLogger(component string) *Logger {
	return &Logger{
		component: component,
	}
}

// WithContext extracts contextual information and returns key-value pairs
func (l *Logger) WithContext(ctx context.Context) []interface{} {
	kvs := []interface{}{
		"component", l.component,
	}

	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		kvs = append(kvs, "correlationID", correlationID)
	}

	if jobName, ok := ctx.Value(JobNameKey).(string); ok && jobName != "" {
		kvs = append(kvs, "job", jobName)
	}

	if namespace, ok := ctx.Value(NamespaceKey).(string); ok && namespace != "" {
		kvs = append(kvs, "namespace", namespace)
	}

	if action, ok := ctx.Value(ActionKey).(string); ok && action != "" {
		kvs = append(kvs, "action", action)
	}

	return kvs
}

// Info logs an info message with context
func (l *Logger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	kvs := l.WithContext(ctx)
	kvs = append(kvs, keysAndValues...)
	kvs = redactSensitiveData(kvs)
	klog.InfoS(msg, kvs...)
}

// Error logs an error message with context
func (l *Logger) Error(ctx context.Context, err error, msg string, keysAndValues ...interface{}) {
	kvs := l.WithContext(ctx)
	kvs = append(kvs, keysAndValues...)
	kvs = redactSensitiveData(kvs)
	klog.ErrorS(err, msg, kvs...)
}

// Warning logs a warning message with context
func (l *Logger) Warning(ctx context.Context, msg string, keysAndValues ...interface{}) {
	kvs := l.WithContext(ctx)
	kvs = append(kvs, keysAndValues...)
	kvs = redactSensitiveData(kvs)
	// klog v2 doesn't have WarningS, use V(2).InfoS for warnings
	klog.V(2).InfoS(msg, kvs...)
}

// Debug logs a debug message with context (V(4))
func (l *Logger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if klog.V(4).Enabled() {
		kvs := l.WithContext(ctx)
		kvs = append(kvs, keysAndValues...)
		kvs = redactSensitiveData(kvs)
		klog.V(4).InfoS(msg, kvs...)
	}
}

// Trace logs a trace message with context (V(5))
func (l *Logger) Trace(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if klog.V(5).Enabled() {
		kvs := l.WithContext(ctx)
		kvs = append(kvs, keysAndValues...)
		kvs = redactSensitiveData(kvs)
		klog.V(5).InfoS(msg, kvs...)
	}
}

// Sensitive data patterns to redact
var sensitivePatterns = []*regexp.Regexp{
	// AWS credentials
	regexp.MustCompile(`(?i)(aws[_-]?access[_-]?key[_-]?id|aws[_-]?secret[_-]?access[_-]?key)[\s:="']+([A-Za-z0-9+/=]{20,})`),
	// Generic secrets/passwords
	regexp.MustCompile(`(?i)(password|secret|token|api[_-]?key)[\s:="']+([^\s"']+)`),
	// Git tokens
	regexp.MustCompile(`(?i)(gh[ps]_[A-Za-z0-9]{36,})`),
	// Docker config auth
	regexp.MustCompile(`(?i)"auth"[\s:]*"([A-Za-z0-9+/=]+)"`),
}

// Sensitive key names to redact values
var sensitiveKeys = map[string]bool{
	"password":         true,
	"secret":           true,
	"token":            true,
	"apikey":           true,
	"api_key":          true,
	"access_key":       true,
	"secret_key":       true,
	"credentials":      true,
	"auth":             true,
	"authorization":    true,
	"bearer":           true,
	"ssh_key":          true,
	"private_key":      true,
	"certificate":      true,
	"kubeconfig":       true,
	"dockerconfigjson": true,
}

// redactSensitiveData redacts sensitive information from log key-value pairs
func redactSensitiveData(keysAndValues []interface{}) []interface{} {
	redacted := make([]interface{}, len(keysAndValues))
	copy(redacted, keysAndValues)

	for i := 0; i < len(redacted); i += 2 {
		if i+1 >= len(redacted) {
			break
		}

		key, ok := redacted[i].(string)
		if !ok {
			continue
		}

		// Check if key is sensitive
		normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
		if sensitiveKeys[normalizedKey] {
			redacted[i+1] = "***REDACTED***"
			continue
		}

		// Check if value contains sensitive patterns
		value, ok := redacted[i+1].(string)
		if !ok {
			continue
		}

		for _, pattern := range sensitivePatterns {
			if pattern.MatchString(value) {
				redacted[i+1] = "***REDACTED***"
				break
			}
		}
	}

	return redacted
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// WithJobName adds a job name to the context
func WithJobName(ctx context.Context, jobName string) context.Context {
	return context.WithValue(ctx, JobNameKey, jobName)
}

// WithNamespace adds a namespace to the context
func WithNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, NamespaceKey, namespace)
}

// WithAction adds an action to the context
func WithAction(ctx context.Context, action string) context.Context {
	return context.WithValue(ctx, ActionKey, action)
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// GenerateCorrelationID generates a correlation ID from job metadata
func GenerateCorrelationID(namespace, name, action string) string {
	return namespace + "/" + name + "/" + action
}
