package telemetry

import (
	"context"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	if metrics == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if metrics.zarfPackageJobsCreated == nil {
		t.Error("zarfPackageJobsCreated not initialized")
	}
	if metrics.zarfPackageJobsActive == nil {
		t.Error("zarfPackageJobsActive not initialized")
	}
	if metrics.jobsCreated == nil {
		t.Error("jobsCreated not initialized")
	}
	if metrics.actionDuration == nil {
		t.Error("actionDuration not initialized")
	}
}

func TestRecordZarfPackageJobCreated(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.RecordZarfPackageJobCreated(ctx, "default")
}

func TestRecordZarfPackageJobDeleted(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.RecordZarfPackageJobDeleted(ctx, "default")
}

func TestRecordJobCreated(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordJobCreated(ctx, "default", "test-pkg", "Build")
}

func TestRecordJobCompleted(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordJobCompleted(ctx, "default", "test-pkg", "Build")
}

func TestRecordJobFailed(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordJobFailed(ctx, "default", "test-pkg", "Build")
}

func TestRecordBuildActions(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordBuildStarted(ctx, "default", "test-pkg")
	metrics.RecordBuildCompleted(ctx, "default", "test-pkg")
	metrics.RecordBuildFailed(ctx, "default", "test-pkg")
}

func TestRecordPublishActions(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordPublishStarted(ctx, "default", "test-pkg")
	metrics.RecordPublishCompleted(ctx, "default", "test-pkg")
	metrics.RecordPublishFailed(ctx, "default", "test-pkg")
}

func TestRecordDeployActions(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordDeployStarted(ctx, "default", "test-pkg")
	metrics.RecordDeployCompleted(ctx, "default", "test-pkg")
	metrics.RecordDeployFailed(ctx, "default", "test-pkg")
}

func TestRecordActionDuration(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	tests := []struct {
		name     string
		duration float64
		status   string
	}{
		{"short build", 10.5, "success"},
		{"long deploy", 300.0, "success"},
		{"failed publish", 5.2, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			metrics.RecordActionDuration(ctx, "default", "test-pkg", "Build", tt.duration, tt.status)
		})
	}
}

func TestRecordReconcileError(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordReconcileError(ctx, "policy_violation")
	metrics.RecordReconcileError(ctx, "job_creation_failed")
}

func TestRecordReconcileDuration(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	metrics.RecordReconcileDuration(ctx, 0.123)
}

func TestRecordWebhookValidation(t *testing.T) {
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	tests := []struct {
		name    string
		allowed bool
		reason  string
	}{
		{"allowed request", true, "policy_passed"},
		{"denied request", false, "unauthorized_repo"},
		{"allowed with warning", true, "allowed_with_override"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			metrics.RecordWebhookValidation(ctx, tt.allowed, tt.reason)
		})
	}
}
