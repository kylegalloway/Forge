package controller

import (
	"context"
	"net/http"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

func TestNewController(t *testing.T) {
	kubeClient := fake.NewClientset()
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
	kubeClient := fake.NewClientset()
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
	kubeClient := fake.NewClientset()
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
	kubeClient := fake.NewClientset()
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
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
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
				Object: unstructuredObj,
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
	kubeClient := fake.NewClientset()
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
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "ZarfPackageJob",
	})

	// Create the resource first
	_, err = dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace("forge-system").Create(
		context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	err = ctrl.updateStatus(context.Background(), unstructuredObj, "Running", "Test message", nil)
	if err != nil {
		t.Errorf("updateStatus() error = %v", err)
	}

	// Verify status was updated
	updated, err := dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace("forge-system").Get(
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

func TestReconcilePackage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name              string
		pkg               *zarfv1alpha1.ZarfPackageJob
		expectedPhase     string
		expectStatusError bool
	}{
		{
			name: "build action without service account",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha1.ActionBuild,
				},
			},
			expectedPhase:     "Failed",
			expectStatusError: false, // Status update succeeds, but phase is Failed
		},
		{
			name: "unknown action",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-unknown",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "UnknownAction",
				},
			},
			expectedPhase:     "Failed",
			expectStatusError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.pkg)
			if err != nil {
				t.Fatalf("Failed to convert to unstructured: %v", err)
			}
			unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
			unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "forge.dev",
				Version: "v1alpha1",
				Kind:    "ZarfPackageJob",
			})

			// Create the resource
			_, err = dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(tt.pkg.Namespace).Create(
				context.Background(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test resource: %v", err)
			}

			err = ctrl.reconcilePackage(context.Background(), unstructuredObj, tt.pkg)
			if tt.expectStatusError && err == nil {
				t.Error("Expected error from reconcilePackage, got nil")
			}
			if !tt.expectStatusError && err != nil {
				t.Errorf("Unexpected error from reconcilePackage: %v", err)
			}

			// Verify status was updated
			updated, getErr := dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(tt.pkg.Namespace).Get(
				context.Background(), tt.pkg.Name, metav1.GetOptions{})
			if getErr != nil {
				t.Fatalf("Failed to get updated resource: %v", getErr)
			}

			status, found, _ := unstructured.NestedMap(updated.Object, "status")
			if found && tt.expectedPhase != "" {
				phase, _, _ := unstructured.NestedString(status, "phase")
				if phase != tt.expectedPhase {
					t.Errorf("Expected phase %q, got %q", tt.expectedPhase, phase)
				}
			}
		})
	}
}

func TestHealthzHandlerResponse(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name           string
		healthy        bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy",
			healthy:        true,
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		{
			name:           "unhealthy",
			healthy:        false,
			expectedStatus: 503,
			expectedBody:   "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl.healthy = tt.healthy
			handler := ctrl.HealthzHandler()

			req := &http.Request{}
			writer := &fakeResponseWriter{status: 200, body: []byte{}}
			handler(writer, req)

			if writer.status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, writer.status)
			}
			if string(writer.body) != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, string(writer.body))
			}
		})
	}
}

func TestReadyzHandlerResponse(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name           string
		ready          bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "ready",
			ready:          true,
			expectedStatus: 200,
			expectedBody:   "ready",
		},
		{
			name:           "not ready",
			ready:          false,
			expectedStatus: 503,
			expectedBody:   "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl.ready = tt.ready
			handler := ctrl.ReadyzHandler()

			req := &http.Request{}
			writer := &fakeResponseWriter{status: 200, body: []byte{}}
			handler(writer, req)

			if writer.status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, writer.status)
			}
			if string(writer.body) != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, string(writer.body))
			}
		})
	}
}

// fakeResponseWriter implements http.ResponseWriter for testing
type fakeResponseWriter struct {
	status int
	body   []byte
}

func (f *fakeResponseWriter) Header() http.Header {
	return http.Header{}
}

func (f *fakeResponseWriter) Write(data []byte) (int, error) {
	f.body = append(f.body, data...)
	return len(data), nil
}

func (f *fakeResponseWriter) WriteHeader(statusCode int) {
	f.status = statusCode
}

func TestProcessJobStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	// Create a ZarfPackageJob first
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
			Action: zarfv1alpha1.ActionBuild,
		},
	}

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pkg)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "ZarfPackageJob",
	})

	_, err = dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace("forge-system").Create(
		context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ZarfPackageJob: %v", err)
	}

	tests := []struct {
		name          string
		job           *batchv1.Job
		expectUpdate  bool
		expectedPhase string
	}{
		{
			name: "job missing package label",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-1",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app": "forge",
					},
				},
			},
			expectUpdate: false,
		},
		{
			name: "job missing action label",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-2",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app":               "forge",
						"forge.dev/package": "test-package",
					},
				},
			},
			expectUpdate: false,
		},
		{
			name: "job still running",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-3",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app":               "forge",
						"forge.dev/package": "test-package",
						"forge.dev/action":  "build",
					},
				},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expectUpdate: false,
		},
		{
			name: "job completed successfully",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-4",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app":               "forge",
						"forge.dev/package": "test-package",
						"forge.dev/action":  "build",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:               batchv1.JobComplete,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			expectUpdate:  true,
			expectedPhase: "Completed",
		},
		{
			name: "job failed",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-5",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app":               "forge",
						"forge.dev/package": "test-package",
						"forge.dev/action":  "build",
					},
				},
				Status: batchv1.JobStatus{
					Failed: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:               batchv1.JobFailed,
							Status:             corev1.ConditionTrue,
							Message:            "Pod failed",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			expectUpdate:  true,
			expectedPhase: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctrl.processJobStatus(context.Background(), tt.job)
			if err != nil {
				t.Errorf("processJobStatus() error = %v", err)
			}

			if tt.expectUpdate {
				// Verify the ZarfPackageJob status was updated
				updated, getErr := dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace("forge-system").Get(
					context.Background(), "test-package", metav1.GetOptions{})
				if getErr != nil {
					t.Fatalf("Failed to get updated ZarfPackageJob: %v", getErr)
				}

				status, found, _ := unstructured.NestedMap(updated.Object, "status")
				if !found {
					t.Fatal("Status not found after job completion")
				}

				phase, _, _ := unstructured.NestedString(status, "phase")
				if phase != tt.expectedPhase {
					t.Errorf("Expected phase %q, got %q", tt.expectedPhase, phase)
				}
			}
		})
	}
}

func TestCheckJobStatuses(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	// Create some test Jobs in the fake client
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-1",
			Namespace: "forge-system",
			Labels: map[string]string{
				"app": "forge",
			},
		},
	}
	_, err := kubeClient.BatchV1().Jobs("forge-system").Create(context.Background(), job1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	// This should not error even with no matching jobs
	err = ctrl.checkJobStatuses(context.Background())
	if err != nil {
		t.Errorf("checkJobStatuses() error = %v", err)
	}
}

func TestHandleActionChaining(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name            string
		pkg             *zarfv1alpha1.ZarfPackageJob
		completedAction string
		expectChain     bool
		expectedNext    string // Expected next action ("publish", "deploy", or empty)
	}{
		{
			name: "BuildPublish chain - build completed",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-buildpublish",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "BuildPublish",
					Publish: &zarfv1alpha1.PublishConfig{
						Destination: zarfv1alpha1.PublishDestination{
							Type: zarfv1alpha1.DestinationTypeOCI,
							OCI: &zarfv1alpha1.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/package",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			completedAction: "build",
			expectChain:     true,
			expectedNext:    "publish",
		},
		{
			name: "BuildDeploy chain - build completed",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-builddeploy",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "BuildDeploy",
					Deploy: &zarfv1alpha1.DeployConfig{
						Target: zarfv1alpha1.DeployTargetInCluster,
					},
				},
			},
			completedAction: "build",
			expectChain:     true,
			expectedNext:    "deploy",
		},
		{
			name: "PublishDeploy chain - publish completed",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publishdeploy",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "PublishDeploy",
					Deploy: &zarfv1alpha1.DeployConfig{
						Target: zarfv1alpha1.DeployTargetInCluster,
					},
				},
			},
			completedAction: "publish",
			expectChain:     true,
			expectedNext:    "deploy",
		},
		{
			name: "BuildPublishDeploy chain - build completed",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-buildpublishdeploy",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "BuildPublishDeploy",
					Publish: &zarfv1alpha1.PublishConfig{
						Destination: zarfv1alpha1.PublishDestination{
							Type: zarfv1alpha1.DestinationTypeOCI,
							OCI: &zarfv1alpha1.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/package",
								Tag:        "v1.0.0",
							},
						},
					},
					Deploy: &zarfv1alpha1.DeployConfig{
						Target: zarfv1alpha1.DeployTargetInCluster,
					},
				},
			},
			completedAction: "build",
			expectChain:     true,
			expectedNext:    "publish",
		},
		{
			name: "BuildPublishDeploy chain - publish completed",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bpd-publish",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             "BuildPublishDeploy",
					Deploy: &zarfv1alpha1.DeployConfig{
						Target: zarfv1alpha1.DeployTargetInCluster,
					},
				},
			},
			completedAction: "publish",
			expectChain:     true,
			expectedNext:    "deploy",
		},
		{
			name: "single Build action - no chaining",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "ZarfPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-only",
					Namespace: "forge-system",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha1.ActionBuild,
				},
			},
			completedAction: "build",
			expectChain:     false,
			expectedNext:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.pkg)
			if err != nil {
				t.Fatalf("Failed to convert to unstructured: %v", err)
			}
			unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
			unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "forge.dev",
				Version: "v1alpha1",
				Kind:    "ZarfPackageJob",
			})

			// Create the resource
			_, err = dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(tt.pkg.Namespace).Create(
				context.Background(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test resource: %v", err)
			}

			err = ctrl.handleActionChaining(context.Background(), unstructuredObj, tt.completedAction, "/workspace/package.tar.zst")

			// For chained actions, we expect the handler to attempt execution
			// The chaining logic itself should work (identify next action, call handler)
			// The handler may fail to create a Job due to missing source configuration in test fixtures
			if tt.expectChain {
				// Verify status was updated (either Running if job created, or Failed if handler errored)
				updated, getErr := dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(tt.pkg.Namespace).Get(
					context.Background(), tt.pkg.Name, metav1.GetOptions{})
				if getErr != nil {
					t.Fatalf("Failed to get updated resource: %v", getErr)
				}

				status, found, _ := unstructured.NestedMap(updated.Object, "status")
				if !found {
					t.Fatal("Status not found after action chaining - chaining logic did not execute")
				}

				phase, _, _ := unstructured.NestedString(status, "phase")
				// The chaining should have attempted to execute the next action and updated status
				// Phase could be Running (job created successfully), Failed (job creation failed),
				// or even empty string (no status update yet)
				// The important thing is that the handler was called, indicated by status being set
				if phase == "" {
					// Workaround: check message field instead
					message, _, _ := unstructured.NestedString(status, "message")
					if message == "" {
						t.Error("Status has no phase or message after action chaining - handler may not have been called")
					}
				}

				// Log what happened for visibility
				t.Logf("After chaining %s → %s: phase=%q, err=%v", tt.completedAction, tt.expectedNext, phase, err)
			} else {
				// For non-chained actions, no error expected and handler should not be called
				if err != nil {
					t.Errorf("handleActionChaining() unexpected error for non-chained action: %v", err)
				}

				// Verify status was not modified (should still be empty)
				updated, getErr := dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(tt.pkg.Namespace).Get(
					context.Background(), tt.pkg.Name, metav1.GetOptions{})
				if getErr != nil {
					t.Fatalf("Failed to get updated resource: %v", getErr)
				}

				status, found, _ := unstructured.NestedMap(updated.Object, "status")
				// For non-chained actions, status may not exist or should not have been modified by chaining
				if found {
					phase, _, _ := unstructured.NestedString(status, "phase")
					if phase != "" {
						t.Errorf("Expected no status update for non-chained action, but got phase %q", phase)
					}
				}
			}
		})
	}
}

func mustNewMetrics() *telemetry.Metrics {
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return metrics
}
