package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "scriptrunner.io/controller"

// Metrics holds all OpenTelemetry metrics for the ScriptRunner controller
type Metrics struct {
	// Counter metrics
	scriptRunnersCreated metric.Int64Counter
	jobsCreated          metric.Int64Counter
	jobsCompleted        metric.Int64Counter
	jobsFailed           metric.Int64Counter
	reconcileErrors      metric.Int64Counter
	webhookValidations   metric.Int64Counter
	webhookMutations     metric.Int64Counter

	// Gauge metrics (using UpDownCounter for current state)
	scriptRunnersActive metric.Int64UpDownCounter

	// Histogram metrics
	jobDuration       metric.Float64Histogram
	reconcileDuration metric.Float64Histogram
}

// NewMetrics creates and registers all OpenTelemetry metrics
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	// Create counters
	scriptRunnersCreated, err := meter.Int64Counter(
		"scriptrunner.resources.created",
		metric.WithDescription("Total number of ScriptRunner resources created"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	jobsCreated, err := meter.Int64Counter(
		"scriptrunner.jobs.created",
		metric.WithDescription("Total number of Jobs created by controller"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	jobsCompleted, err := meter.Int64Counter(
		"scriptrunner.jobs.completed",
		metric.WithDescription("Total number of Jobs completed successfully"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	jobsFailed, err := meter.Int64Counter(
		"scriptrunner.jobs.failed",
		metric.WithDescription("Total number of Jobs that failed"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	reconcileErrors, err := meter.Int64Counter(
		"scriptrunner.reconcile.errors",
		metric.WithDescription("Total number of reconciliation errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	webhookValidations, err := meter.Int64Counter(
		"scriptrunner.webhook.validations",
		metric.WithDescription("Total number of webhook validation requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	webhookMutations, err := meter.Int64Counter(
		"scriptrunner.webhook.mutations",
		metric.WithDescription("Total number of webhook mutations applied"),
		metric.WithUnit("{mutation}"),
	)
	if err != nil {
		return nil, err
	}

	// Create gauge (UpDownCounter for current state)
	scriptRunnersActive, err := meter.Int64UpDownCounter(
		"scriptrunner.resources.active",
		metric.WithDescription("Current number of active ScriptRunner resources"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	// Create histograms
	jobDuration, err := meter.Float64Histogram(
		"scriptrunner.job.duration",
		metric.WithDescription("Duration of Job execution"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024),
	)
	if err != nil {
		return nil, err
	}

	reconcileDuration, err := meter.Float64Histogram(
		"scriptrunner.reconcile.duration",
		metric.WithDescription("Duration of reconciliation loop"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		scriptRunnersCreated: scriptRunnersCreated,
		jobsCreated:          jobsCreated,
		jobsCompleted:        jobsCompleted,
		jobsFailed:           jobsFailed,
		reconcileErrors:      reconcileErrors,
		webhookValidations:   webhookValidations,
		webhookMutations:     webhookMutations,
		scriptRunnersActive:  scriptRunnersActive,
		jobDuration:          jobDuration,
		reconcileDuration:    reconcileDuration,
	}, nil
}

// RecordScriptRunnerCreated increments the ScriptRunner created counter
func (m *Metrics) RecordScriptRunnerCreated(ctx context.Context, namespace string) {
	m.scriptRunnersCreated.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
	m.scriptRunnersActive.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordScriptRunnerDeleted decrements the active ScriptRunner counter
func (m *Metrics) RecordScriptRunnerDeleted(ctx context.Context, namespace string) {
	m.scriptRunnersActive.Add(ctx, -1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordJobCreated increments the Job created counter
func (m *Metrics) RecordJobCreated(ctx context.Context, namespace, scriptRunner string) {
	m.jobsCreated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("scriptrunner", scriptRunner),
		))
}

// RecordJobCompleted increments the Job completed counter
func (m *Metrics) RecordJobCompleted(ctx context.Context, namespace, scriptRunner string) {
	m.jobsCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("scriptrunner", scriptRunner),
		))
}

// RecordJobFailed increments the Job failed counter
func (m *Metrics) RecordJobFailed(ctx context.Context, namespace, scriptRunner string) {
	m.jobsFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("scriptrunner", scriptRunner),
		))
}

// RecordJobDuration records the duration of a Job
func (m *Metrics) RecordJobDuration(ctx context.Context, namespace, scriptRunner string, durationSeconds float64, status string) {
	m.jobDuration.Record(ctx, durationSeconds,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("scriptrunner", scriptRunner),
			attribute.String("status", status),
		))
}

// RecordReconcileError increments the reconcile error counter
func (m *Metrics) RecordReconcileError(ctx context.Context, errorType string) {
	m.reconcileErrors.Add(ctx, 1,
		metric.WithAttributes(attribute.String("error_type", errorType)))
}

// RecordReconcileDuration records the duration of a reconciliation loop
func (m *Metrics) RecordReconcileDuration(ctx context.Context, durationSeconds float64) {
	m.reconcileDuration.Record(ctx, durationSeconds)
}

// RecordWebhookValidation increments the webhook validation counter
func (m *Metrics) RecordWebhookValidation(ctx context.Context, allowed bool, reason string) {
	status := "allowed"
	if !allowed {
		status = "denied"
	}
	m.webhookValidations.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("reason", reason),
		))
}

// RecordWebhookMutation increments the webhook mutation counter
func (m *Metrics) RecordWebhookMutation(ctx context.Context, mutationType string) {
	m.webhookMutations.Add(ctx, 1,
		metric.WithAttributes(attribute.String("mutation_type", mutationType)))
}
