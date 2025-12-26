package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "forge.dev/controller"

// Metrics holds all OpenTelemetry metrics for the Forge controller
type Metrics struct {
	// Counter metrics
	zarfPackageJobsCreated metric.Int64Counter
	jobsCreated            metric.Int64Counter
	jobsCompleted          metric.Int64Counter
	jobsFailed             metric.Int64Counter
	reconcileErrors        metric.Int64Counter
	webhookValidations     metric.Int64Counter

	// Action-specific metrics (Zarf packages)
	buildsStarted      metric.Int64Counter
	buildsCompleted    metric.Int64Counter
	buildsFailed       metric.Int64Counter
	publishesStarted   metric.Int64Counter
	publishesCompleted metric.Int64Counter
	publishesFailed    metric.Int64Counter
	deploysStarted     metric.Int64Counter
	deploysCompleted   metric.Int64Counter
	deploysFailed      metric.Int64Counter

	// UDS Bundle-specific metrics
	bundleCreatesStarted     metric.Int64Counter
	bundleCreatesCompleted   metric.Int64Counter
	bundleCreatesFailed      metric.Int64Counter
	bundlePublishesStarted   metric.Int64Counter
	bundlePublishesCompleted metric.Int64Counter
	bundlePublishesFailed    metric.Int64Counter
	bundleDeploysStarted     metric.Int64Counter
	bundleDeploysCompleted   metric.Int64Counter
	bundleDeploysFailed      metric.Int64Counter
	bundleJobsCreated        metric.Int64Counter

	// Gauge metrics (using UpDownCounter for current state)
	zarfPackageJobsActive metric.Int64UpDownCounter
	udsPackageJobsActive  metric.Int64UpDownCounter

	// Histogram metrics
	actionDuration    metric.Float64Histogram
	reconcileDuration metric.Float64Histogram
}

// NewMetrics creates and registers all OpenTelemetry metrics
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	// Create counters for ZarfPackageJob resources
	zarfPackageJobsCreated, err := meter.Int64Counter(
		"forge.zarf_package_jobs.created",
		metric.WithDescription("Total number of ZarfPackageJob resources created"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	zarfPackageJobsActive, err := meter.Int64UpDownCounter(
		"forge.zarf_package_jobs.active",
		metric.WithDescription("Current number of active ZarfPackageJob resources"),
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

	// UDS Bundle metrics
	bundleCreatesStarted, err := meter.Int64Counter(
		"forge.bundle_creates.started",
		metric.WithDescription("Total number of UDS bundle create actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleCreatesCompleted, err := meter.Int64Counter(
		"forge.bundle_creates.completed",
		metric.WithDescription("Total number of UDS bundle create actions completed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleCreatesFailed, err := meter.Int64Counter(
		"forge.bundle_creates.failed",
		metric.WithDescription("Total number of UDS bundle create actions failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundlePublishesStarted, err := meter.Int64Counter(
		"forge.bundle_publishes.started",
		metric.WithDescription("Total number of UDS bundle publish actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundlePublishesCompleted, err := meter.Int64Counter(
		"forge.bundle_publishes.completed",
		metric.WithDescription("Total number of UDS bundle publish actions completed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundlePublishesFailed, err := meter.Int64Counter(
		"forge.bundle_publishes.failed",
		metric.WithDescription("Total number of UDS bundle publish actions failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleDeploysStarted, err := meter.Int64Counter(
		"forge.bundle_deploys.started",
		metric.WithDescription("Total number of UDS bundle deploy actions started"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleDeploysCompleted, err := meter.Int64Counter(
		"forge.bundle_deploys.completed",
		metric.WithDescription("Total number of UDS bundle deploy actions completed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleDeploysFailed, err := meter.Int64Counter(
		"forge.bundle_deploys.failed",
		metric.WithDescription("Total number of UDS bundle deploy actions failed"),
		metric.WithUnit("{action}"),
	)
	if err != nil {
		return nil, err
	}

	bundleJobsCreated, err := meter.Int64Counter(
		"forge.bundle_jobs.created",
		metric.WithDescription("Total number of Jobs created for UDS bundles"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, err
	}

	udsPackageJobsActive, err := meter.Int64UpDownCounter(
		"forge.uds_bundle_jobs.active",
		metric.WithDescription("Current number of active UDSBundleJob resources"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		zarfPackageJobsCreated:   zarfPackageJobsCreated,
		zarfPackageJobsActive:    zarfPackageJobsActive,
		jobsCreated:              jobsCreated,
		jobsCompleted:            jobsCompleted,
		jobsFailed:               jobsFailed,
		buildsStarted:            buildsStarted,
		buildsCompleted:          buildsCompleted,
		buildsFailed:             buildsFailed,
		publishesStarted:         publishesStarted,
		publishesCompleted:       publishesCompleted,
		publishesFailed:          publishesFailed,
		deploysStarted:           deploysStarted,
		deploysCompleted:         deploysCompleted,
		deploysFailed:            deploysFailed,
		bundleCreatesStarted:     bundleCreatesStarted,
		bundleCreatesCompleted:   bundleCreatesCompleted,
		bundleCreatesFailed:      bundleCreatesFailed,
		bundlePublishesStarted:   bundlePublishesStarted,
		bundlePublishesCompleted: bundlePublishesCompleted,
		bundlePublishesFailed:    bundlePublishesFailed,
		bundleDeploysStarted:     bundleDeploysStarted,
		bundleDeploysCompleted:   bundleDeploysCompleted,
		bundleDeploysFailed:      bundleDeploysFailed,
		bundleJobsCreated:        bundleJobsCreated,
		udsPackageJobsActive:     udsPackageJobsActive,
		reconcileErrors:          reconcileErrors,
		webhookValidations:       webhookValidations,
		actionDuration:           actionDuration,
		reconcileDuration:        reconcileDuration,
	}, nil
}

// RecordZarfPackageJobCreated increments the ZarfPackageJob created counter.
func (metrics *Metrics) RecordZarfPackageJobCreated(ctx context.Context, namespace string) {
	metrics.zarfPackageJobsCreated.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
	metrics.zarfPackageJobsActive.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordZarfPackageJobDeleted decrements the active ZarfPackageJob counter.
func (metrics *Metrics) RecordZarfPackageJobDeleted(ctx context.Context, namespace string) {
	metrics.zarfPackageJobsActive.Add(ctx, -1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordJobCreated increments the Job created counter
func (metrics *Metrics) RecordJobCreated(ctx context.Context, namespace, packageName, action string) {
	metrics.jobsCreated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordJobCompleted increments the Job completed counter
func (metrics *Metrics) RecordJobCompleted(ctx context.Context, namespace, packageName, action string) {
	metrics.jobsCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordJobFailed increments the Job failed counter
func (metrics *Metrics) RecordJobFailed(ctx context.Context, namespace, packageName, action string) {
	metrics.jobsFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
		))
}

// RecordPackageBuildStarted increments the package build started counter
func (metrics *Metrics) RecordPackageBuildStarted(ctx context.Context, namespace, packageName string) {
	metrics.buildsStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackageBuildCompleted increments the package build completed counter
func (metrics *Metrics) RecordPackageBuildCompleted(ctx context.Context, namespace, packageName string) {
	metrics.buildsCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackageBuildFailed increments the package build failed counter
func (metrics *Metrics) RecordPackageBuildFailed(ctx context.Context, namespace, packageName string) {
	metrics.buildsFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackagePublishStarted increments the package publish started counter
func (metrics *Metrics) RecordPackagePublishStarted(ctx context.Context, namespace, packageName string) {
	metrics.publishesStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackagePublishCompleted increments the package publish completed counter
func (metrics *Metrics) RecordPackagePublishCompleted(ctx context.Context, namespace, packageName string) {
	metrics.publishesCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackagePublishFailed increments the package publish failed counter
func (metrics *Metrics) RecordPackagePublishFailed(ctx context.Context, namespace, packageName string) {
	metrics.publishesFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackageDeployStarted increments the package deploy started counter
func (metrics *Metrics) RecordPackageDeployStarted(ctx context.Context, namespace, packageName string) {
	metrics.deploysStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackageDeployCompleted increments the package deploy completed counter
func (metrics *Metrics) RecordPackageDeployCompleted(ctx context.Context, namespace, packageName string) {
	metrics.deploysCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordPackageDeployFailed increments the package deploy failed counter
func (metrics *Metrics) RecordPackageDeployFailed(ctx context.Context, namespace, packageName string) {
	metrics.deploysFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
		))
}

// RecordActionDuration records the duration of an action (build/publish/deploy)
func (metrics *Metrics) RecordActionDuration(ctx context.Context, namespace, packageName, action string, durationSeconds float64, status string) {
	metrics.actionDuration.Record(ctx, durationSeconds,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("package", packageName),
			attribute.String("action", action),
			attribute.String("status", status),
		))
}

// RecordReconcileError increments the reconcile error counter
func (metrics *Metrics) RecordReconcileError(ctx context.Context, errorType string) {
	metrics.reconcileErrors.Add(ctx, 1,
		metric.WithAttributes(attribute.String("error_type", errorType)))
}

// RecordReconcileDuration records the duration of a reconciliation loop
func (metrics *Metrics) RecordReconcileDuration(ctx context.Context, durationSeconds float64) {
	metrics.reconcileDuration.Record(ctx, durationSeconds)
}

// RecordWebhookValidation increments the webhook validation counter
func (metrics *Metrics) RecordWebhookValidation(ctx context.Context, allowed bool, reason string) {
	status := "allowed"
	if !allowed {
		status = "denied"
	}
	metrics.webhookValidations.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("reason", reason),
		))
}

// UDS Bundle metric recording methods

// RecordBundleCreateStarted increments the bundle create started counter
func (metrics *Metrics) RecordBundleCreateStarted(ctx context.Context, namespace, bundleName string) {
	metrics.bundleCreatesStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleCreateCompleted increments the bundle create completed counter
func (metrics *Metrics) RecordBundleCreateCompleted(ctx context.Context, namespace, bundleName string) {
	metrics.bundleCreatesCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleCreateFailed increments the bundle create failed counter
func (metrics *Metrics) RecordBundleCreateFailed(ctx context.Context, namespace, bundleName string) {
	metrics.bundleCreatesFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundlePublishStarted increments the bundle publish started counter
func (metrics *Metrics) RecordBundlePublishStarted(ctx context.Context, namespace, bundleName string) {
	metrics.bundlePublishesStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundlePublishCompleted increments the bundle publish completed counter
func (metrics *Metrics) RecordBundlePublishCompleted(ctx context.Context, namespace, bundleName string) {
	metrics.bundlePublishesCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundlePublishFailed increments the bundle publish failed counter
func (metrics *Metrics) RecordBundlePublishFailed(ctx context.Context, namespace, bundleName string) {
	metrics.bundlePublishesFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleDeployStarted increments the bundle deploy started counter
func (metrics *Metrics) RecordBundleDeployStarted(ctx context.Context, namespace, bundleName string) {
	metrics.bundleDeploysStarted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleDeployCompleted increments the bundle deploy completed counter
func (metrics *Metrics) RecordBundleDeployCompleted(ctx context.Context, namespace, bundleName string) {
	metrics.bundleDeploysCompleted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleDeployFailed increments the bundle deploy failed counter
func (metrics *Metrics) RecordBundleDeployFailed(ctx context.Context, namespace, bundleName string) {
	metrics.bundleDeploysFailed.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
		))
}

// RecordBundleJobCreated increments the bundle job created counter
func (metrics *Metrics) RecordBundleJobCreated(ctx context.Context, namespace, bundleName, action string) {
	metrics.bundleJobsCreated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("namespace", namespace),
			attribute.String("bundle", bundleName),
			attribute.String("action", action),
		))
}

// RecordUDSPackageJobCreated increments the UDSPackageJob created and active counters
func (metrics *Metrics) RecordUDSPackageJobCreated(ctx context.Context, namespace string) {
	metrics.udsPackageJobsActive.Add(ctx, 1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}

// RecordUDSPackageJobDeleted decrements the active UDSPackageJob counter
func (metrics *Metrics) RecordUDSPackageJobDeleted(ctx context.Context, namespace string) {
	metrics.udsPackageJobsActive.Add(ctx, -1,
		metric.WithAttributes(attribute.String("namespace", namespace)))
}
