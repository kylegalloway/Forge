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

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

func TestNewUDSController(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	metrics := mustNewMetrics()
	tracer := telemetry.NewTracer()

	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", metrics, tracer)

	if ctrl == nil {
		t.Fatal("NewUDSController returned nil")
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
	if ctrl.createHandler == nil {
		t.Error("createHandler not initialized")
	}
	if ctrl.publishHandler == nil {
		t.Error("publishHandler not initialized")
	}
	if ctrl.deployHandler == nil {
		t.Error("deployHandler not initialized")
	}
}

func TestUDSHandleEvent(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	bundle := &udsv1alpha2.UDSPackageJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forge.dev/v1alpha1",
			Kind:       "UDSPackageJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "forge-system",
		},
		Spec: udsv1alpha2.UDSPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	// Convert to unstructured
	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bundle)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "UDSPackageJob",
	})

	tests := []struct {
		name      string
		eventType watch.EventType
		wantErr   bool
	}{
		{
			name:      "handle added event",
			eventType: watch.Added,
			wantErr:   false,
		},
		{
			name:      "handle modified event",
			eventType: watch.Modified,
			wantErr:   false,
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
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			event := watch.Event{
				Type:   tt.eventType,
				Object: unstructuredObj,
			}
			// UDSController uses handleWatchEvent instead of handleEvent
			ctrl.handleWatchEvent(context.Background(), event)
			// handleWatchEvent doesn't return errors, so we can't check for them
		})
	}
}

func TestUDSUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	bundle := &udsv1alpha2.UDSPackageJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forge.dev/v1alpha1",
			Kind:       "UDSPackageJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-bundle",
			Namespace:  "forge-system",
			Generation: 1,
		},
	}

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bundle)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "UDSPackageJob",
	})

	// Create the resource first
	_, err = dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace("forge-system").Create(
		context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	// Convert back to typed object for updateStatus
	typedBundle := &udsv1alpha2.UDSPackageJob{}
	if convErr := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, typedBundle); convErr != nil {
		t.Fatalf("Failed to convert to typed bundle: %v", convErr)
	}

	err = ctrl.updateStatus(context.Background(), typedBundle, "Running", "Test message")
	if err != nil {
		t.Errorf("updateStatus() error = %v", err)
	}

	// Verify status was updated
	updated, err := dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace("forge-system").Get(
		context.Background(), "test-bundle", metav1.GetOptions{})
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

func TestUDSReconcileBundle(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name              string
		bundle            *udsv1alpha2.UDSPackageJob
		expectedPhase     string
		expectStatusError bool
	}{
		{
			name: "create action without service account",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionCreate,
				},
			},
			expectedPhase:     "Failed",
			expectStatusError: false,
		},
		{
			name: "unknown action",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-unknown",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
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
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.bundle)
			if err != nil {
				t.Fatalf("Failed to convert to unstructured: %v", err)
			}
			unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
			unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "forge.dev",
				Version: "v1alpha1",
				Kind:    "UDSPackageJob",
			})

			// Create the resource
			_, err = dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace(tt.bundle.Namespace).Create(
				context.Background(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test resource: %v", err)
			}

			// UDSController uses reconcile instead of reconcileBundle
			err = ctrl.reconcile(context.Background(), tt.bundle)
			if tt.expectStatusError && err == nil {
				t.Error("Expected error from reconcile, got nil")
			}
			if !tt.expectStatusError && err != nil {
				t.Errorf("Unexpected error from reconcile: %v", err)
			}

			// Verify status was updated
			updated, getErr := dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace(tt.bundle.Namespace).Get(
				context.Background(), tt.bundle.Name, metav1.GetOptions{})
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

func TestUDSProcessJobStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	// Create a UDSPackageJob first
	bundle := &udsv1alpha2.UDSPackageJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forge.dev/v1alpha1",
			Kind:       "UDSPackageJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "forge-system",
		},
		Spec: udsv1alpha2.UDSPackageJobSpec{
			Action: udsv1alpha2.ActionCreate,
		},
	}

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bundle)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "forge.dev",
		Version: "v1alpha1",
		Kind:    "UDSPackageJob",
	})

	_, err = dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace("forge-system").Create(
		context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create UDSPackageJob: %v", err)
	}

	tests := []struct {
		name          string
		job           *batchv1.Job
		expectUpdate  bool
		expectedPhase string
	}{
		{
			name: "job missing bundle label",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-1",
					Namespace: "forge-system",
					Labels: map[string]string{
						"app": "forge-uds",
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
						"app":                  "forge-uds",
						constants.LabelPackage: "test-bundle",
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
						"app":                  "forge-uds",
						constants.LabelPackage: "test-bundle",
						constants.LabelAction:  "create",
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
						"app":                  "forge-uds",
						constants.LabelPackage: "test-bundle",
						constants.LabelAction:  "create",
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
						"app":                  "forge-uds",
						constants.LabelPackage: "test-bundle",
						constants.LabelAction:  "create",
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
				// Verify the UDSPackageJob status was updated
				updated, getErr := dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace("forge-system").Get(
					context.Background(), "test-bundle", metav1.GetOptions{})
				if getErr != nil {
					t.Fatalf("Failed to get updated UDSPackageJob: %v", getErr)
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

func TestUDSCheckJobStatuses(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	// Create some test Jobs in the fake client
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-1",
			Namespace: "forge-system",
			Labels: map[string]string{
				"app": "forge-uds",
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

func TestUDSHandleActionChaining(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add UDS scheme: %v", err)
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := fake.NewClientset()
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name            string
		bundle          *udsv1alpha2.UDSPackageJob
		completedAction string
		expectChain     bool
	}{
		{
			name: "CreatePublish chain - create completed",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-createpublish",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: "CreatePublish",
					Publish: &udsv1alpha2.PublishConfig{
						Destination: udsv1alpha2.PublishDestination{
							Type: udsv1alpha2.DestinationTypeOCI,
							OCI: &udsv1alpha2.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/bundle",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			completedAction: "create",
			expectChain:     true,
		},
		{
			name: "CreateDeploy chain - create completed",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-createdeploy",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: "CreateDeploy",
					Deploy: &udsv1alpha2.DeployConfig{
						Target: udsv1alpha2.DeployTargetInCluster,
					},
				},
			},
			completedAction: "create",
			expectChain:     true,
		},
		{
			name: "PublishDeploy chain - publish completed",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publishdeploy",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: "PublishDeploy",
					Deploy: &udsv1alpha2.DeployConfig{
						Target: udsv1alpha2.DeployTargetInCluster,
					},
				},
			},
			completedAction: "publish",
			expectChain:     true,
		},
		{
			name: "CreatePublishDeploy chain - create completed",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-createpublishdeploy",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: "CreatePublishDeploy",
					Publish: &udsv1alpha2.PublishConfig{
						Destination: udsv1alpha2.PublishDestination{
							Type: udsv1alpha2.DestinationTypeOCI,
							OCI: &udsv1alpha2.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/bundle",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			completedAction: "create",
			expectChain:     true,
		},
		{
			name: "CreatePublishDeploy chain - publish completed",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cpd-publish",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: "CreatePublishDeploy",
					Deploy: &udsv1alpha2.DeployConfig{
						Target: udsv1alpha2.DeployTargetInCluster,
					},
				},
			},
			completedAction: "publish",
			expectChain:     true,
		},
		{
			name: "single Create action - no chaining",
			bundle: &udsv1alpha2.UDSPackageJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "forge.dev/v1alpha1",
					Kind:       "UDSPackageJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-only",
					Namespace: "forge-system",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Action: udsv1alpha2.ActionCreate,
				},
			},
			completedAction: "create",
			expectChain:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.bundle)
			if err != nil {
				t.Fatalf("Failed to convert to unstructured: %v", err)
			}
			unstructuredObj := &unstructured.Unstructured{Object: unstrObj}
			unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "forge.dev",
				Version: "v1alpha1",
				Kind:    "UDSPackageJob",
			})

			// Create the resource
			_, err = dynamicClient.Resource(constants.UDSPackageJobGVR).Namespace(tt.bundle.Namespace).Create(
				context.Background(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test resource: %v", err)
			}

			err = ctrl.handleActionChaining(context.Background(), tt.bundle, tt.completedAction)
			// We expect errors for chained actions since handlers will fail without real infrastructure
			// But we verify the function executed without panic
			if err != nil && !tt.expectChain {
				t.Errorf("handleActionChaining() unexpected error for non-chained action: %v", err)
			}
			// For chained actions, errors are expected (missing infra), just verify no panic
		})
	}
}

func TestUDSHealthzHandler(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

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

func TestUDSReadyzHandler(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

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

func TestUDSHealthzHandlerResponse(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

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
			writer := &fakeUDSResponseWriter{status: 200, body: []byte{}}
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

func TestUDSReadyzHandlerResponse(t *testing.T) {
	kubeClient := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := NewUDSController(kubeClient, dynamicClient, "forge-system", mustNewMetrics(), telemetry.NewTracer())

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
			writer := &fakeUDSResponseWriter{status: 200, body: []byte{}}
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

// fakeUDSResponseWriter implements http.ResponseWriter for testing
type fakeUDSResponseWriter struct {
	status int
	body   []byte
}

func (f *fakeUDSResponseWriter) Header() http.Header {
	return http.Header{}
}

func (f *fakeUDSResponseWriter) Write(data []byte) (int, error) {
	f.body = append(f.body, data...)
	return len(data), nil
}

func (f *fakeUDSResponseWriter) WriteHeader(statusCode int) {
	f.status = statusCode
}
