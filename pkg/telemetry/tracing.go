package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "forge.dev/controller"

// Tracer wraps OpenTelemetry tracer with convenience methods
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new tracer wrapper
func NewTracer() *Tracer {
	return &Tracer{
		tracer: otel.Tracer(tracerName),
	}
}

// StartReconcileSpan starts a new span for reconciliation
func (t *Tracer) StartReconcileSpan(ctx context.Context, namespace, name string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "reconcile_zarfpackagejob",
		trace.WithAttributes(
			attribute.String("ZarfPackageJob.namespace", namespace),
			attribute.String("ZarfPackageJob.name", name),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartJobCreationSpan starts a new span for Job creation
func (t *Tracer) StartJobCreationSpan(ctx context.Context, namespace, zarfPackageJob, jobName string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "create_job",
		trace.WithAttributes(
			attribute.String("ZarfPackageJob.namespace", namespace),
			attribute.String("ZarfPackageJob.name", zarfPackageJob),
			attribute.String("job.name", jobName),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartWebhookValidationSpan starts a new span for webhook validation
func (t *Tracer) StartWebhookValidationSpan(ctx context.Context, namespace, name string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "validate_zarfpackagejob",
		trace.WithAttributes(
			attribute.String("ZarfPackageJob.namespace", namespace),
			attribute.String("ZarfPackageJob.name", name),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
}

// StartWebhookMutationSpan starts a new span for webhook mutation
func (t *Tracer) StartWebhookMutationSpan(ctx context.Context, namespace, name string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "mutate_zarfpackagejob",
		trace.WithAttributes(
			attribute.String("ZarfPackageJob.namespace", namespace),
			attribute.String("ZarfPackageJob.name", name),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
}

// RecordError records an error on the span
func RecordError(span trace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSuccess marks the span as successful
func SetSuccess(span trace.Span) {
	if span != nil {
		span.SetStatus(codes.Ok, "")
	}
}

// AddEvent adds an event to the span
func AddEvent(span trace.Span, name string, attributes ...attribute.KeyValue) {
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attributes...))
	}
}
