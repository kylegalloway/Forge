package controller

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	testhelpers "github.com/kylegalloway/forge/pkg/controller/testing"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// TestHandleObject_ConversionError tests error handling when converting unstructured to typed resource
func TestHandleObject_ConversionError(t *testing.T) {
	ctrl := createTestController(t)

	// Create invalid unstructured object that can't be converted
	invalidObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "forge.dev/v1alpha1",
			"kind":       "ZarfPackageJob",
			"metadata": map[string]interface{}{
				"name":      "test-pkg",
				"namespace": "default",
			},
			"spec": "invalid-spec-should-be-object", // Invalid spec type
		},
	}

	err := ctrl.handleObject(context.Background(), invalidObj)
	if err == nil {
		t.Error("Expected error when converting invalid unstructured object")
	}
}

// TestHandleObject_SkipsTerminalState tests that completed/failed resources are skipped
func TestHandleObject_SkipsTerminalState(t *testing.T) {
	ctrl := createTestController(t)

	tests := []struct {
		name  string
		phase string
	}{
		{"completed resource", "Completed"},
		{"failed resource", "Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := createTestUnstructuredZarfJob("test-pkg", "default")
			obj.Object["status"] = map[string]interface{}{
				"phase": tt.phase,
			}

			err := ctrl.handleObject(context.Background(), obj)
			if err != nil {
				t.Errorf("Expected no error for %s resource, got: %v", tt.phase, err)
			}
		})
	}
}

// TestHandleObject_ActionDispatchError tests error handling when action dispatch fails
func TestHandleObject_ActionDispatchError(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create handler that always fails
	failingHandler := &mockFailingHandler{
		err: errors.New("job creation failed: API server unreachable"),
	}

	gvr := zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")

	config := ControllerConfig{
		ResourceType:    "ZarfPackageJob",
		ResourceGVR:     gvr,
		PrimaryAction:   "Build",
		LabelSelector:   "app=forge",
		SupportsPVC:     true,
		StatusFieldName: "buildStatus",
	}

	ctrl := NewGenericController(
		kubeClient,
		dynamicClient,
		"default",
		testhelpers.MustNewMetrics(),
		telemetry.NewTracer(),
		config,
		failingHandler,
		failingHandler,
		failingHandler,
		&mockMetricsRecorder{},
	)

	obj := createTestUnstructuredZarfJob("test-pkg", "default")

	// Create the resource first so status update can succeed
	_, err := dynamicClient.Resource(gvr).Namespace("default").Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	err = ctrl.handleObject(context.Background(), obj)
	// When action dispatch fails, the controller updates status to "Failed"
	// With the resource created, status update should succeed, so handleObject returns nil
	if err != nil {
		t.Errorf("Expected no error after successful status update, got: %v", err)
	}

	// Verify the resource was marked as Failed
	updated, err := dynamicClient.Resource(gvr).Namespace("default").Get(context.Background(), "test-pkg", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated resource: %v", err)
	}

	status, ok := updated.Object["status"].(map[string]interface{})
	if !ok {
		t.Fatal("Resource status not set")
	}

	phase, ok := status["phase"].(string)
	if !ok || phase != "Failed" {
		t.Errorf("Expected phase 'Failed', got: %s", phase)
	}
}

// TestHandleEvent_UnexpectedType tests error handling for unexpected event types
func TestHandleEvent_UnexpectedType(t *testing.T) {
	ctrl := createTestController(t)

	tests := []struct {
		name      string
		eventType watch.EventType
		wantErr   bool
	}{
		{"watch error event", watch.Error, true},
		{"deleted event with wrong type", watch.Deleted, true},
		{"unknown event type", watch.EventType("Unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event watch.Event
			if tt.eventType == watch.Deleted {
				// Send wrong object type for deleted event
				event = watch.Event{
					Type:   tt.eventType,
					Object: &runtime.Unknown{}, // Wrong type
				}
			} else {
				event = watch.Event{
					Type:   tt.eventType,
					Object: createTestUnstructuredZarfJob("test", "default"),
				}
			}

			err := ctrl.handleEvent(context.Background(), event)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestEnsureArtifactPVC_CreateError tests PVC creation error handling
func TestEnsureArtifactPVC_CreateError(t *testing.T) {
	// This test would require a fake client that can simulate creation failures
	// For now, we test the happy path is covered by integration tests
	// TODO: Add test with client that fails on Create()
	t.Skip("Requires fake client with controlled failures")
}

// TestUpdateStatus_NotFoundError tests that NotFound errors during status update are handled gracefully
func TestUpdateStatus_NotFoundError(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	gvr := zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")

	config := ControllerConfig{
		ResourceType:    "ZarfPackageJob",
		ResourceGVR:     gvr,
		PrimaryAction:   "Build",
		LabelSelector:   "app=forge",
		SupportsPVC:     true,
		StatusFieldName: "buildStatus",
	}

	ctrl := NewGenericController(
		kubeClient,
		dynamicClient,
		"default",
		testhelpers.MustNewMetrics(),
		telemetry.NewTracer(),
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockMetricsRecorder{},
	)

	// Try to update status for non-existent object
	obj := createTestUnstructuredZarfJob("nonexistent", "default")

	err := ctrl.updateStatus(context.Background(), obj, "Failed", "test error", nil)
	// NotFound errors should be handled gracefully (logged but not returned)
	if err != nil {
		t.Errorf("Expected NotFound error to be handled gracefully, got: %v", err)
	}
}

// Helper functions

func createTestController(t *testing.T) *GenericController[*zarfv1alpha1.ZarfPackageJob] {
	t.Helper()

	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	gvr := zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")

	config := ControllerConfig{
		ResourceType:    "ZarfPackageJob",
		ResourceGVR:     gvr,
		PrimaryAction:   "Build",
		LabelSelector:   "app=forge",
		SupportsPVC:     true,
		StatusFieldName: "buildStatus",
	}

	return NewGenericController(
		kubeClient,
		dynamicClient,
		"default",
		testhelpers.MustNewMetrics(),
		telemetry.NewTracer(),
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockMetricsRecorder{},
	)
}

func createTestUnstructuredZarfJob(name, namespace string) *unstructured.Unstructured { //nolint:unparam // namespace parameter kept for API consistency
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "forge.dev/v1alpha1",
			"kind":       "ZarfPackageJob",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"serviceAccountName": "test-sa",
				"action":             "Build",
				"source": map[string]interface{}{
					"type": "Git",
					"git": map[string]interface{}{
						"url": "https://github.com/test/repo",
						"ref": "main",
					},
				},
			},
		},
	}
}

// Mock handlers for testing

type mockSuccessHandler struct{}

func (m *mockSuccessHandler) Execute(_ context.Context, resource *zarfv1alpha1.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return &actions.ActionResult{
		JobName: "test-job",
		Phase:   "Running",
	}, nil
}

type mockFailingHandler struct {
	err error
}

func (m *mockFailingHandler) Execute(_ context.Context, resource *zarfv1alpha1.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return nil, m.err
}

type mockMetricsRecorder struct{}

func (m *mockMetricsRecorder) RecordPrimaryActionStarted(_ context.Context, namespace, name string) {
}
func (m *mockMetricsRecorder) RecordPrimaryActionCompleted(_ context.Context, namespace, name string) {
}
func (m *mockMetricsRecorder) RecordPrimaryActionFailed(_ context.Context, namespace, name string) {
}
func (m *mockMetricsRecorder) RecordPublishStarted(_ context.Context, namespace, name string)     {}
func (m *mockMetricsRecorder) RecordPublishCompleted(_ context.Context, namespace, name string)   {}
func (m *mockMetricsRecorder) RecordPublishFailed(_ context.Context, namespace, name string)      {}
func (m *mockMetricsRecorder) RecordDeployStarted(_ context.Context, namespace, name string)      {}
func (m *mockMetricsRecorder) RecordDeployCompleted(_ context.Context, namespace, name string)    {}
func (m *mockMetricsRecorder) RecordDeployFailed(_ context.Context, namespace, name string)       {}
func (m *mockMetricsRecorder) RecordJobCreated(_ context.Context, namespace, name, action string) {}
func (m *mockMetricsRecorder) RecordJobCompleted(_ context.Context, namespace, name, action string) {
}
func (m *mockMetricsRecorder) RecordJobFailed(_ context.Context, namespace, name, action string) {}
func (m *mockMetricsRecorder) RecordActionDuration(_ context.Context, namespace, name, action string, duration float64, status string) {
}

// Compile-time check that mockMetricsRecorder implements MetricsRecorder
var _ MetricsRecorder[*zarfv1alpha1.ZarfPackageJob] = (*mockMetricsRecorder)(nil)
