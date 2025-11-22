package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestNewTracer(t *testing.T) {
	tracer := NewTracer()
	if tracer == nil {
		t.Fatal("NewTracer() returned nil")
	}
	if tracer.tracer == nil {
		t.Error("tracer.tracer not initialized")
	}
}

func TestStartReconcileSpan(t *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	newCtx, span := tracer.StartReconcileSpan(ctx, "default", "test-package")
	if newCtx == nil {
		t.Error("StartReconcileSpan returned nil context")
	}
	if span == nil {
		t.Error("StartReconcileSpan returned nil span")
	}
	defer span.End()
}

func TestStartJobCreationSpan(t *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	newCtx, span := tracer.StartJobCreationSpan(ctx, "default", "test-package", "test-job")
	if newCtx == nil {
		t.Error("StartJobCreationSpan returned nil context")
	}
	if span == nil {
		t.Error("StartJobCreationSpan returned nil span")
	}
	defer span.End()
}

func TestStartWebhookValidationSpan(t *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	newCtx, span := tracer.StartWebhookValidationSpan(ctx, "default", "test-package")
	if newCtx == nil {
		t.Error("StartWebhookValidationSpan returned nil context")
	}
	if span == nil {
		t.Error("StartWebhookValidationSpan returned nil span")
	}
	defer span.End()
}

func TestStartWebhookMutationSpan(t *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	newCtx, span := tracer.StartWebhookMutationSpan(ctx, "default", "test-package")
	if newCtx == nil {
		t.Error("StartWebhookMutationSpan returned nil context")
	}
	if span == nil {
		t.Error("StartWebhookMutationSpan returned nil span")
	}
	defer span.End()
}

func TestRecordError(_ *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	_, span := tracer.StartReconcileSpan(ctx, "default", "test-package")
	defer span.End()

	testErr := errors.New("test error")
	RecordError(span, testErr)
	// Should not panic

	// Test with nil span
	RecordError(nil, testErr)
	// Should not panic

	// Test with nil error
	RecordError(span, nil)
	// Should not panic
}

func TestSetSuccess(_ *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	_, span := tracer.StartReconcileSpan(ctx, "default", "test-package")
	defer span.End()

	SetSuccess(span)
	// Should not panic

	// Test with nil span
	SetSuccess(nil)
	// Should not panic
}

func TestAddEvent(_ *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	_, span := tracer.StartReconcileSpan(ctx, "default", "test-package")
	defer span.End()

	AddEvent(span, "test.event", attribute.String("key", "value"))
	AddEvent(span, "another.event")
	// Should not panic

	// Test with nil span
	AddEvent(nil, "test.event")
	// Should not panic
}

func TestSpanEndToEnd(_ *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	// Simulate full reconciliation lifecycle
	ctx, reconcileSpan := tracer.StartReconcileSpan(ctx, "default", "test-package")
	defer reconcileSpan.End()

	AddEvent(reconcileSpan, "policy.validation.started")

	// Simulate job creation
	_, jobSpan := tracer.StartJobCreationSpan(ctx, "default", "test-package", "test-job")
	AddEvent(jobSpan, "job.created")
	SetSuccess(jobSpan)
	jobSpan.End()

	// Mark reconciliation as successful
	SetSuccess(reconcileSpan)
}

func TestSpanWithError(_ *testing.T) {
	tracer := NewTracer()
	ctx := context.Background()

	_, span := tracer.StartWebhookValidationSpan(ctx, "default", "test-package")
	defer span.End()

	AddEvent(span, "validation.started")

	// Simulate error
	validationErr := errors.New("invalid configuration")
	RecordError(span, validationErr)
	AddEvent(span, "validation.failed", attribute.String("reason", validationErr.Error()))
}
