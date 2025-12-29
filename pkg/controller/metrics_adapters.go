package controller

import (
	"context"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// ZarfMetricsRecorder adapts telemetry.Metrics to MetricsRecorder[*ZarfPackageJob]
type ZarfMetricsRecorder struct {
	metrics *telemetry.Metrics
}

// NewZarfMetricsRecorder creates a new Zarf metrics recorder adapter
func NewZarfMetricsRecorder(metrics *telemetry.Metrics) *ZarfMetricsRecorder {
	return &ZarfMetricsRecorder{metrics: metrics}
}

// RecordPrimaryActionStarted records when a build starts
func (r *ZarfMetricsRecorder) RecordPrimaryActionStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageBuildStarted(ctx, namespace, name)
}

// RecordPrimaryActionCompleted records when a build completes
func (r *ZarfMetricsRecorder) RecordPrimaryActionCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageBuildCompleted(ctx, namespace, name)
}

// RecordPrimaryActionFailed records when a build fails
func (r *ZarfMetricsRecorder) RecordPrimaryActionFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageBuildFailed(ctx, namespace, name)
}

// RecordPublishStarted records when a publish starts
func (r *ZarfMetricsRecorder) RecordPublishStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackagePublishStarted(ctx, namespace, name)
}

// RecordPublishCompleted records when a publish completes
func (r *ZarfMetricsRecorder) RecordPublishCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackagePublishCompleted(ctx, namespace, name)
}

// RecordPublishFailed records when a publish fails
func (r *ZarfMetricsRecorder) RecordPublishFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackagePublishFailed(ctx, namespace, name)
}

// RecordDeployStarted records when a deploy starts
func (r *ZarfMetricsRecorder) RecordDeployStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageDeployStarted(ctx, namespace, name)
}

// RecordDeployCompleted records when a deploy completes
func (r *ZarfMetricsRecorder) RecordDeployCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageDeployCompleted(ctx, namespace, name)
}

// RecordDeployFailed records when a deploy fails
func (r *ZarfMetricsRecorder) RecordDeployFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordPackageDeployFailed(ctx, namespace, name)
}

// RecordJobCreated records when a job is created
func (r *ZarfMetricsRecorder) RecordJobCreated(ctx context.Context, namespace, name, action string) {
	r.metrics.RecordJobCreated(ctx, namespace, name, action)
}

// RecordJobCompleted records when a job completes
func (r *ZarfMetricsRecorder) RecordJobCompleted(ctx context.Context, namespace, name, action string) {
	r.metrics.RecordJobCompleted(ctx, namespace, name, action)
}

// RecordJobFailed records when a job fails
func (r *ZarfMetricsRecorder) RecordJobFailed(ctx context.Context, namespace, name, action string) {
	r.metrics.RecordJobFailed(ctx, namespace, name, action)
}

// RecordActionDuration records the duration of an action
func (r *ZarfMetricsRecorder) RecordActionDuration(ctx context.Context, namespace, name, action string, duration float64, status string) {
	r.metrics.RecordActionDuration(ctx, namespace, name, action, duration, status)
}

// Compile-time assertion that ZarfMetricsRecorder implements MetricsRecorder
var _ MetricsRecorder[*zarfv1alpha1.ZarfPackageJob] = (*ZarfMetricsRecorder)(nil)

// UDSMetricsRecorder adapts telemetry.Metrics to MetricsRecorder[*UDSBundleJob]
type UDSMetricsRecorder struct {
	metrics *telemetry.Metrics
}

// NewUDSMetricsRecorder creates a new UDS metrics recorder adapter
func NewUDSMetricsRecorder(metrics *telemetry.Metrics) *UDSMetricsRecorder {
	return &UDSMetricsRecorder{metrics: metrics}
}

// RecordPrimaryActionStarted records when a create starts
func (r *UDSMetricsRecorder) RecordPrimaryActionStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleCreateStarted(ctx, namespace, name)
}

// RecordPrimaryActionCompleted records when a create completes
func (r *UDSMetricsRecorder) RecordPrimaryActionCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleCreateCompleted(ctx, namespace, name)
}

// RecordPrimaryActionFailed records when a create fails
func (r *UDSMetricsRecorder) RecordPrimaryActionFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleCreateFailed(ctx, namespace, name)
}

// RecordPublishStarted records when a publish starts
func (r *UDSMetricsRecorder) RecordPublishStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundlePublishStarted(ctx, namespace, name)
}

// RecordPublishCompleted records when a publish completes
func (r *UDSMetricsRecorder) RecordPublishCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundlePublishCompleted(ctx, namespace, name)
}

// RecordPublishFailed records when a publish fails
func (r *UDSMetricsRecorder) RecordPublishFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundlePublishFailed(ctx, namespace, name)
}

// RecordDeployStarted records when a deploy starts
func (r *UDSMetricsRecorder) RecordDeployStarted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleDeployStarted(ctx, namespace, name)
}

// RecordDeployCompleted records when a deploy completes
func (r *UDSMetricsRecorder) RecordDeployCompleted(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleDeployCompleted(ctx, namespace, name)
}

// RecordDeployFailed records when a deploy fails
func (r *UDSMetricsRecorder) RecordDeployFailed(ctx context.Context, namespace, name string) {
	r.metrics.RecordBundleDeployFailed(ctx, namespace, name)
}

// RecordJobCreated records when a job is created
func (r *UDSMetricsRecorder) RecordJobCreated(ctx context.Context, namespace, name, action string) {
	r.metrics.RecordBundleJobCreated(ctx, namespace, name, action)
}

// RecordJobCompleted records when a job completes
// TODO: Add RecordBundleJobCompleted to telemetry.Metrics
func (r *UDSMetricsRecorder) RecordJobCompleted(_ context.Context, _, _, _ string) {
	// UDS metrics don't currently have a generic job completion metric
	// Action-specific metrics are recorded via RecordPrimaryActionCompleted, etc.
}

// RecordJobFailed records when a job fails
// TODO: Add RecordBundleJobFailed to telemetry.Metrics
func (r *UDSMetricsRecorder) RecordJobFailed(_ context.Context, _, _, _ string) {
	// UDS metrics don't currently have a generic job failure metric
	// Action-specific metrics are recorded via RecordPrimaryActionFailed, etc.
}

// RecordActionDuration records the duration of an action
// TODO: Add RecordBundleActionDuration to telemetry.Metrics
func (r *UDSMetricsRecorder) RecordActionDuration(_ context.Context, _, _, _ string, _ float64, _ string) {
	// UDS metrics don't currently have action duration tracking
	// Will be added in a future iteration
}

// Compile-time assertion that UDSMetricsRecorder implements MetricsRecorder
var _ MetricsRecorder[*udsv1alpha2.UDSBundleJob] = (*UDSMetricsRecorder)(nil)
