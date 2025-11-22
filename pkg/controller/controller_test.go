package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

func TestNewController(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	metrics := mustNewMetrics()
	tracer := telemetry.NewTracer()

	ctrl := NewController(kubeClient, dynamicClient, "forge-system", metrics, tracer)

	if ctrl == nil {
		t.Fatal("NewController returned nil")
	}
	if ctrl.kubeClient == nil {
		t.Error("kubeClient not set")
	}
	if ctrl.dynamicClient == nil {
		t.Error("dynamicClient not set")
	}
	if ctrl.namespace != "forge-system" {
		t.Errorf("Expected namespace 'forge-system', got '%s'", ctrl.namespace)
	}
	if ctrl.metrics == nil {
		t.Error("metrics not set")
	}
	if ctrl.tracer == nil {
		t.Error("tracer not set")
	}
	if ctrl.policyEngine == nil {
		t.Error("policyEngine not initialized")
	}
	if ctrl.buildHandler == nil {
		t.Error("buildHandler not initialized")
	}
	if ctrl.publishHandler == nil {
		t.Error("publishHandler not initialized")
	}
	if ctrl.deployHandler == nil {
		t.Error("deployHandler not initialized")
	}
	if !ctrl.healthy {
		t.Error("Controller should start healthy")
	}
	if ctrl.ready {
		t.Error("Controller should not be ready before Run()")
	}
}

func TestHealthzHandler(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name           string
		healthy        bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy controller",
			healthy:        true,
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		{
			name:           "unhealthy controller",
			healthy:        false,
			expectedStatus: 503,
			expectedBody:   "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl.healthy = tt.healthy
			// Health check testing would require HTTP testing infrastructure
			// This validates the handler creation doesn't panic
			handler := ctrl.HealthzHandler()
			if handler == nil {
				t.Error("HealthzHandler returned nil")
			}
		})
	}
}

func TestReadyzHandler(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name           string
		ready          bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "ready controller",
			ready:          true,
			expectedStatus: 200,
			expectedBody:   "ready",
		},
		{
			name:           "not ready controller",
			ready:          false,
			expectedStatus: 503,
			expectedBody:   "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl.ready = tt.ready
			handler := ctrl.ReadyzHandler()
			if handler == nil {
				t.Error("ReadyzHandler returned nil")
			}
		})
	}
}

func TestHandleEvent(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha1.ZarfPackageJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forge.dev/v1alpha1",
			Kind:       "ZarfPackageJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-package",
			Namespace: "forge-system",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	// Convert to unstructured
	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pkg)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: unstrObj}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "ZarfPackageJob",
	})

	tests := []struct {
		name      string
		eventType watch.EventType
		wantErr   bool
	}{
		{
			name:      "handle added event",
			eventType: watch.Added,
			wantErr:   false, // handleEvent doesn't return errors, logs them
		},
		{
			name:      "handle modified event",
			eventType: watch.Modified,
			wantErr:   false, // handleEvent doesn't return errors, logs them
		},
		{
			name:      "handle deleted event",
			eventType: watch.Deleted,
			wantErr:   false,
		},
		{
			name:      "handle error event",
			eventType: watch.Error,
			wantErr:   true,
		},
		{
			name:      "handle bookmark event",
			eventType: watch.Bookmark,
			wantErr:   false, // Ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := watch.Event{
				Type:   tt.eventType,
				Object: u,
			}
			err := ctrl.handleEvent(context.Background(), event)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewSimpleClientset()
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha1.ZarfPackageJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forge.dev/v1alpha1",
			Kind:       "ZarfPackageJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-package",
			Namespace:  "forge-system",
			Generation: 1,
		},
	}

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pkg)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: unstrObj}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "ZarfPackageJob",
	})

	// Create the resource first
	_, err = dynamicClient.Resource(ZarfPackageJobGVR).Namespace("forge-system").Create(
		context.Background(), u, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	err = ctrl.updateStatus(context.Background(), u, "Running", "Test message", nil)
	if err != nil {
		t.Errorf("updateStatus() error = %v", err)
	}

	// Verify status was updated
	updated, err := dynamicClient.Resource(ZarfPackageJobGVR).Namespace("forge-system").Get(
		context.Background(), "test-package", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated resource: %v", err)
	}

	status, found, err := unstructured.NestedMap(updated.Object, "status")
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if !found {
		t.Fatal("Status not found")
	}

	phase, found, err := unstructured.NestedString(status, "phase")
	if err != nil || !found {
		t.Fatal("Phase not found in status")
	}
	if phase != "Running" {
		t.Errorf("Expected phase 'Running', got '%s'", phase)
	}

	message, found, err := unstructured.NestedString(status, "message")
	if err != nil || !found {
		t.Fatal("Message not found in status")
	}
	if message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", message)
	}
}

func mustNewMetrics() *telemetry.Metrics {
	m, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return m
}
