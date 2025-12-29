package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
)

// TestProcessJobStatus_MissingLabels tests that jobs without required labels are skipped
func TestProcessJobStatus_MissingLabels(t *testing.T) {
	monitor := createTestMonitor(t)

	tests := []struct {
		name   string
		job    *batchv1.Job
		labels map[string]string
	}{
		{
			name: "missing package label",
			labels: map[string]string{
				constants.LabelAction: "Build",
			},
		},
		{
			name: "missing action label",
			labels: map[string]string{
				constants.LabelPackage: "test-pkg",
			},
		},
		{
			name:   "missing both labels",
			labels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
					Labels:    tt.labels,
				},
			}

			err := monitor.processJobStatus(context.Background(), job)
			if err != nil {
				t.Errorf("Expected no error for job with missing labels, got: %v", err)
			}
		})
	}
}

// TestProcessJobStatus_RunningJob tests that running jobs don't trigger updates
func TestProcessJobStatus_RunningJob(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create resource
	resource := createTestUnstructuredZarfJob("test-pkg", "default")
	_, err := dynamicClient.Resource(zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")).
		Namespace("default").
		Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	updateCalled := false
	statusUpdater := func(_ context.Context, _ *unstructured.Unstructured, _, _ string, _ map[string]interface{}) error {
		updateCalled = true
		return nil
	}

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		statusUpdater,
	)

	// Create a running job (no completion conditions)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}

	err = monitor.processJobStatus(context.Background(), job)
	if err != nil {
		t.Errorf("Expected no error for running job, got: %v", err)
	}

	if updateCalled {
		t.Error("Expected status update not to be called for running job")
	}
}

// TestProcessJobStatus_CompletedJob tests successful job completion handling
func TestProcessJobStatus_CompletedJob(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create resource
	resource := createTestUnstructuredZarfJob("test-pkg", "default")
	_, err := dynamicClient.Resource(zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")).
		Namespace("default").
		Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	updateCalled := false
	var capturedPhase string
	statusUpdater := func(_ context.Context, _ *unstructured.Unstructured, phase, _ string, _ map[string]interface{}) error {
		updateCalled = true
		capturedPhase = phase
		return nil
	}

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		statusUpdater,
	)

	// Create completed job
	now := metav1.Now()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
			StartTime: &now,
		},
	}

	err = monitor.processJobStatus(context.Background(), job)
	if err != nil {
		t.Errorf("Expected no error for completed job, got: %v", err)
	}

	if !updateCalled {
		t.Error("Expected status update to be called for completed job")
	}

	if capturedPhase != "Completed" {
		t.Errorf("Expected phase 'Completed', got: %s", capturedPhase)
	}
}

// TestProcessJobStatus_FailedJob tests failed job handling
func TestProcessJobStatus_FailedJob(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create resource
	resource := createTestUnstructuredZarfJob("test-pkg", "default")
	_, err := dynamicClient.Resource(zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")).
		Namespace("default").
		Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	updateCalled := false
	var capturedPhase string
	statusUpdater := func(_ context.Context, _ *unstructured.Unstructured, phase, _ string, _ map[string]interface{}) error {
		updateCalled = true
		capturedPhase = phase
		return nil
	}

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		statusUpdater,
	)

	// Create failed job
	now := metav1.Now()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Failed: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobFailed,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
					Message:            "Pod failed",
				},
			},
			StartTime: &now,
		},
	}

	err = monitor.processJobStatus(context.Background(), job)
	if err != nil {
		t.Errorf("Expected no error for failed job, got: %v", err)
	}

	if !updateCalled {
		t.Error("Expected status update to be called for failed job")
	}

	if capturedPhase != "Failed" {
		t.Errorf("Expected phase 'Failed', got: %s", capturedPhase)
	}
}

// TestProcessJobStatus_ResourceNotFound tests handling when resource is deleted
func TestProcessJobStatus_ResourceNotFound(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	// Don't create resource - it will be "not found"
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		func(_ context.Context, _ *unstructured.Unstructured, _, _ string, _ map[string]interface{}) error {
			return nil
		},
	)

	// Create completed job for non-existent resource
	now := metav1.Now()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "nonexistent-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
		},
	}

	// Should not error - just log and continue
	err := monitor.processJobStatus(context.Background(), job)
	if err != nil {
		t.Errorf("Expected no error when resource not found, got: %v", err)
	}
}

// TestProcessJobStatus_StatusUpdateError tests handling when status update fails
func TestProcessJobStatus_StatusUpdateError(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create resource
	resource := createTestUnstructuredZarfJob("test-pkg", "default")
	_, err := dynamicClient.Resource(zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")).
		Namespace("default").
		Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	// Status updater that always fails
	statusUpdater := func(_ context.Context, _ *unstructured.Unstructured, _, _ string, _ map[string]interface{}) error {
		return errors.New("status update failed: conflict")
	}

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		statusUpdater,
	)

	// Create completed job
	now := metav1.Now()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
		},
	}

	err = monitor.processJobStatus(context.Background(), job)
	if err == nil {
		t.Error("Expected error when status update fails")
	}
}

// TestProcessJobStatus_MultipleCompletedJobs tests the edge case of multiple completed jobs
func TestProcessJobStatus_MultipleCompletedJobs(t *testing.T) {
	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create resource
	resource := createTestUnstructuredZarfJob("test-pkg", "default")
	_, err := dynamicClient.Resource(zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs")).
		Namespace("default").
		Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	updateCount := 0
	statusUpdater := func(_ context.Context, _ *unstructured.Unstructured, _, _ string, _ map[string]interface{}) error {
		updateCount++
		return nil
	}

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	monitor := NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		statusUpdater,
	)

	now := metav1.Now()

	// Process first completed job
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-1",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
		},
	}

	err = monitor.processJobStatus(context.Background(), job1)
	if err != nil {
		t.Errorf("Expected no error for first job, got: %v", err)
	}

	// Process second completed job for same resource (edge case)
	job2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-2",
			Namespace: "default",
			Labels: map[string]string{
				constants.LabelPackage: "test-pkg",
				constants.LabelAction:  "Build",
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(now.Add(time.Minute)),
				},
			},
		},
	}

	err = monitor.processJobStatus(context.Background(), job2)
	if err != nil {
		t.Errorf("Expected no error for second job, got: %v", err)
	}

	if updateCount != 2 {
		t.Errorf("Expected 2 status updates (one per job), got: %d", updateCount)
	}
}

// Helper function to create test monitor
func createTestMonitor(t *testing.T) *GenericJobMonitor[*zarfv1alpha1.ZarfPackageJob] {
	t.Helper()

	kubeClient := fake.NewClientset()
	scheme := runtime.NewScheme()
	_ = zarfv1alpha1.AddToScheme(scheme)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	config := MonitorConfig{
		ResourceType:       "ZarfPackageJob",
		LabelSelector:      "app=forge",
		PrimaryAction:      "Build",
		PrimaryStatusField: "buildStatus",
		SupportsPVC:        true,
	}

	return NewGenericJobMonitor(
		kubeClient,
		dynamicClient,
		"default",
		zarfv1alpha1.SchemeGroupVersion.WithResource("zarfpackagejobs"),
		&mockMetricsRecorder{},
		config,
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		&mockSuccessHandler{},
		func(_ context.Context, _ *unstructured.Unstructured, _, _ string, _ map[string]interface{}) error {
			return nil
		},
	)
}
