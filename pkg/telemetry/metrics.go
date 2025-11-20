package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "forge.zarf.dev/controller"

// Metrics holds all OpenTelemetry metrics for the Forge controller
type Metrics struct {
	// Counter metrics
	zarfPackagesCreated metric.Int64Counter
	jobsCreated         metric.Int64Counter
	jobsCompleted       metric.Int64Counter
	jobsFailed          metric.Int64Counter
	reconcileErrors     metric.Int64Counter
	webhookValidations  metric.Int64Counter

	// Action-specific metrics
	buildsStarted    metric.Int64Counter
	buildsCompleted  metric.Int64Counter
	buildsFailed     metric.Int64Counter
	publishesStarted metric.Int64Counter
	publishesCompleted metric.Int64Counter
	publishesFailed  metric.Int64Counter
	deploysStarted   metric.Int64Counter
	deploysCompleted metric.Int64Counter
	deploysFailed    metric.Int64Counter

	// Gauge metrics (using UpDownCounter for current state)
	zarfPackagesActive metric.Int64UpDownCounter

	// Histogram metrics
	actionDuration    metric.Float64Histogram
	reconcileDuration metric.Float64Histogram
}

// NewMetrics creates and registers all OpenTelemetry metrics
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	// Create counters for ZarfPackage resources
	zarfPackagesCreated, err := meter.Int64Counter(
		"forge.zarf_packages.created",
		metric.WithDescription("Total number of ZarfPackage resources created"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	zarfPackagesActive, err := meter.Int64UpDownCounter(
		"forge.zarf_packages.active",
		metric.WithDescription("Current number of active ZarfPackage resources"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	// Job metrics
	jobsCreated, err := meter.Int64Counter(
		"forge.jobs.created",
		metric.WithDescription("Total number of Jobs created by controller"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	jobsCompleted, err := meter.Int64Counter(
		"forge.jobs.completed",
		metric.WithDescription("Total number of Jobs completed successfully"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	jobsFailed, err := meter.Int64Counter(
		"forge.jobs.failed",
		metric.WithDescription("Total number of Jobs that failed"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	// Build action metrics
	buildsStarted, err := meter.Int64Counter(
		"forge.builds.started",
		metric.WithDescription("Total number of build actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	buildsCompleted, err := meter.Int64Counter(
		"forge.builds.completed",
		metric.WithDescription("Total number of build actions completed successfully"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	buildsFailed, err := meter.Int64Counter(
		"forge.builds.failed",
		metric.WithDescription("Total number of build actions that failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	// Publish action metrics
	publishesStarted, err := meter.Int64Counter(
		"forge.publishes.started",
		metric.WithDescription("Total number of publish actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	publishesCompleted, err := meter.Int64Counter(
		"forge.publishes.completed",
		metric.WithDescription("Total number of publish actions completed successfully"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	publishesFailed, err := meter.Int64Counter(
		"forge.publishes.failed",
		metric.WithDescription("Total number of publish actions that failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	// Deploy action metrics
	deploysStarted, err := meter.Int64Counter(
		"forge.deploys.started",
		metric.WithDescription("Total number of deploy actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	deploysCompleted, err := meter.Int64Counter(
		"forge.deploys.completed",
		metric.WithDescription("Total number of deploy actions completed successfully"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	deploysFailed, err := meter.Int64Counter(
		"forge.deploys.failed",
		metric.WithDescription("Total number of deploy actions that failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	// Reconcile errors
	reconcileErrors, err := meter.Int64Counter(
		"forge.reconcile.errors",
		metric.WithDescription("Total number of reconciliation errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	// Webhook validations
	webhookValidations, err := meter.Int64Counter(
		"forge.webhook.validations",
		metric.WithDescription("Total number of webhook validation requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	// Histograms
	actionDuration, err := meter.Float64Histogram(
		"forge.action.duration",
		metric.WithDescription("Duration of action execution (build/publish/deploy)"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048),
	)
	if err != nil {
		return nil, err
	}

	reconcileDuration, err := meter.Float64Histogram(
		"forge.reconcile.duration",
		metric.WithDescription("Duration of reconciliation loop"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		zarfPackagesCreated: zarfPackagesCreated,
		zarfPackagesActive:  zarfPackagesActive,
		jobsCreated:         jobsCreated,
		jobsCompleted:       jobsCompleted,
		jobsFailed:          jobsFailed,
		buildsStarted:       buildsStarted,
		buildsCompleted:     buildsCompleted,
		buildsFailed:        buildsFailed,
		publishesStarted:    publishesStarted,
		publishesCompleted:  publishesCompleted,
		publishesFailed:     publishesFailed,
		deploysStarted:      deploysStarted,
		deploysCompleted:    deploysCompleted,
		deploysFailed:       deploysFailed,
		reconcileErrors:     reconcileErrors,
		webhookValidations:  webhookValidations,
		actionDuration:      actionDuration,
		reconcileDuration:   reconcileDuration,
	}, nil
}

// RecordZarfPackageCreated increments the ZarfPackage created counter
func (m *Metrics) RecordZarfPackageCreated(ctx context.Context, namespace string) {
	m.zarfPackagesCreated.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
	m.zarfPackagesActive.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordZarfPackageDeleted decrements the active ZarfPackage counter
func (m *Metrics) RecordZarfPackageDeleted(ctx context.Context, namespace string) {
	m.zarfPackagesActive.Add(ctx, -1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordJobCreated increments the Job created counter
func (m *Metrics) RecordJobCreated(ctx context.Context, namespace, packageName, action string) {
	m.jobsCreated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordJobCompleted increments the Job completed counter
func (m *Metrics) RecordJobCompleted(ctx context.Context, namespace, packageName, action string) {
	m.jobsCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordJobFailed increments the Job failed counter
func (m *Metrics) RecordJobFailed(ctx context.Context, namespace, packageName, action string) {
	m.jobsFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordBuildStarted increments the build started counter
func (m *Metrics) RecordBuildStarted(ctx context.Context, namespace, packageName string) {
	m.buildsStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordBuildCompleted increments the build completed counter
func (m *Metrics) RecordBuildCompleted(ctx context.Context, namespace, packageName string) {
	m.buildsCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordBuildFailed increments the build failed counter
func (m *Metrics) RecordBuildFailed(ctx context.Context, namespace, packageName string) {
	m.buildsFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPublishStarted increments the publish started counter
func (m *Metrics) RecordPublishStarted(ctx context.Context, namespace, packageName string) {
	m.publishesStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPublishCompleted increments the publish completed counter
func (m *Metrics) RecordPublishCompleted(ctx context.Context, namespace, packageName string) {
	m.publishesCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPublishFailed increments the publish failed counter
func (m *Metrics) RecordPublishFailed(ctx context.Context, namespace, packageName string) {
	m.publishesFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordDeployStarted increments the deploy started counter
func (m *Metrics) RecordDeployStarted(ctx context.Context, namespace, packageName string) {
	m.deploysStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordDeployCompleted increments the deploy completed counter
func (m *Metrics) RecordDeployCompleted(ctx context.Context, namespace, packageName string) {
	m.deploysCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordDeployFailed increments the deploy failed counter
func (m *Metrics) RecordDeployFailed(ctx context.Context, namespace, packageName string) {
	m.deploysFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordActionDuration records the duration of an action (build/publish/deploy)
func (m *Metrics) RecordActionDuration(ctx context.Context, namespace, packageName, action string, durationSeconds float64, status string) {
	m.actionDuration.Record(ctx, durationSeconds,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
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
