package telemetry

import (
	"context"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if m.zarfPackageJobsCreated == nil {
		t.Error("zarfPackageJobsCreated not initialized")
	}
	if m.zarfPackageJobsActive == nil {
		t.Error("zarfPackageJobsActive not initialized")
	}
	if m.jobsCreated == nil {
		t.Error("jobsCreated not initialized")
	}
	if m.actionDuration == nil {
		t.Error("actionDuration not initialized")
	}
}

func TestRecordZarfPackageJobCreated(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	// Should not panic
	m.RecordZarfPackageJobCreated(ctx, "default")
}

func TestRecordZarfPackageJobDeleted(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	// Should not panic
	m.RecordZarfPackageJobDeleted(ctx, "default")
}

func TestRecordJobCreated(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordJobCreated(ctx, "default", "test-pkg", "Build")
}

func TestRecordJobCompleted(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordJobCompleted(ctx, "default", "test-pkg", "Build")
}

func TestRecordJobFailed(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordJobFailed(ctx, "default", "test-pkg", "Build")
}

func TestRecordBuildActions(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordBuildStarted(ctx, "default", "test-pkg")
	m.RecordBuildCompleted(ctx, "default", "test-pkg")
	m.RecordBuildFailed(ctx, "default", "test-pkg")
}

func TestRecordPublishActions(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordPublishStarted(ctx, "default", "test-pkg")
	m.RecordPublishCompleted(ctx, "default", "test-pkg")
	m.RecordPublishFailed(ctx, "default", "test-pkg")
}

func TestRecordDeployActions(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordDeployStarted(ctx, "default", "test-pkg")
	m.RecordDeployCompleted(ctx, "default", "test-pkg")
	m.RecordDeployFailed(ctx, "default", "test-pkg")
}

func TestRecordActionDuration(t *testing.T) {
	m, err := NewMetrics()
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
			m.RecordActionDuration(ctx, "default", "test-pkg", "Build", tt.duration, tt.status)
		})
	}
}

func TestRecordReconcileError(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordReconcileError(ctx, "policy_violation")
	m.RecordReconcileError(ctx, "job_creation_failed")
}

func TestRecordReconcileDuration(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	ctx := context.Background()
	m.RecordReconcileDuration(ctx, 0.123)
}

func TestRecordWebhookValidation(t *testing.T) {
	m, err := NewMetrics()
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
			m.RecordWebhookValidation(ctx, tt.allowed, tt.reason)
		})
	}
}
