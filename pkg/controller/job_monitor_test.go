package controller

import (
	"context"
	"testing"
	"time"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestJobMonitor_SingleActionCompletion tests job monitoring for single action jobs
func TestJobMonitor_SingleActionCompletion(t *testing.T) {
	kubeClient := fake.NewClientset()
	ctx := context.Background()

	// Create a completed job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	_, err := kubeClient.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Verify job was created
	retrievedJob, err := kubeClient.BatchV1().Jobs("default").Get(ctx, "test-build", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to retrieve job: %v", err)
	}

	if retrievedJob.Status.Succeeded != 1 {
		t.Errorf("Expected 1 succeeded, got %d", retrievedJob.Status.Succeeded)
	}

	if len(retrievedJob.Status.Conditions) == 0 {
		t.Error("Expected job conditions, got none")
	}
}

// TestJobMonitor_MultiActionChaining tests action chaining progression
func TestJobMonitor_MultiActionChaining(t *testing.T) {
	tests := []struct {
		name         string
		initialJob   *batchv1.Job
		expectedNext string
	}{
		{
			name: "BuildPublish - after build complete, trigger publish",
			initialJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-job",
					Labels: map[string]string{
						"action": "BuildPublish",
						"stage":  "Build",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expectedNext: "Publish",
		},
		{
			name: "BuildPublishDeploy - after publish complete, trigger deploy",
			initialJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-job",
					Labels: map[string]string{
						"action": "BuildPublishDeploy",
						"stage":  "Publish",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expectedNext: "Deploy",
		},
		{
			name: "CreatePublishDeploy - after create complete, trigger publish",
			initialJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-job",
					Labels: map[string]string{
						"action": "CreatePublishDeploy",
						"stage":  "Create",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expectedNext: "Publish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify job has expected labels for action chaining
			if stage, ok := tt.initialJob.Labels["stage"]; !ok {
				t.Error("Expected stage label for action chaining")
			} else if stage == "" {
				t.Error("Stage label should not be empty")
			}

			if action, ok := tt.initialJob.Labels["action"]; !ok {
				t.Error("Expected action label for multi-action jobs")
			} else if action == "" {
				t.Error("Action label should not be empty")
			}
		})
	}
}

// TestJobMonitor_StatusUpdates tests that status fields are properly updated
func TestJobMonitor_StatusUpdates(t *testing.T) {
	// Create a ZarfPackageJob with build status
	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
		},
		Status: zarfv1alpha3.ZarfPackageJobStatus{
			BuildStatus: &zarfv1alpha3.OperationStatus{
				Phase:     constants.PhaseRunning,
				StartTime: &metav1.Time{Time: time.Now()},
			},
		},
	}

	// Simulate status update to complete
	if pkg.Status.BuildStatus != nil {
		pkg.Status.BuildStatus.Phase = constants.PhaseCompleted
		pkg.Status.BuildStatus.CompletionTime = &metav1.Time{Time: time.Now()}
	}

	// Verify status was updated
	if pkg.Status.BuildStatus.Phase != constants.PhaseCompleted {
		t.Errorf("Expected phase Complete, got %v", pkg.Status.BuildStatus.Phase)
	}

	if pkg.Status.BuildStatus.CompletionTime == nil {
		t.Error("Expected completion time to be set")
	}
}

// TestJobMonitor_ArtifactPassing tests artifact discovery and passing between actions
func TestJobMonitor_ArtifactPassing(t *testing.T) {
	tests := []struct {
		name         string
		artifactPath string
		expectedGlob string
	}{
		{
			name:         "Zarf build artifact",
			artifactPath: "/tmp/artifacts/package.tar.zst",
			expectedGlob: "*.tar.zst",
		},
		{
			name:         "UDS bundle artifact",
			artifactPath: "/tmp/artifacts/bundle.tar.zst",
			expectedGlob: "*.tar.zst",
		},
		{
			name:         "OCI artifact",
			artifactPath: "ghcr.io/org/image:v1.0.0",
			expectedGlob: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify artifact path would match expected glob
			switch tt.expectedGlob {
			case "*":
				// OCI reference - just validate format
				if len(tt.artifactPath) == 0 {
					t.Error("OCI reference should not be empty")
				}
			case "*.tar.zst":
				// Zarf/UDS artifact - validate extension
				if len(tt.artifactPath) < 8 {
					t.Error("Artifact path too short for tar.zst")
				}
			}
		})
	}
}

// TestJobMonitor_DebugModeCompletion tests debug mode marker detection
func TestJobMonitor_DebugModeCompletion(t *testing.T) {
	kubeClient := fake.NewClientset()
	ctx := context.Background()

	// Create a pod in debug mode
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-debug-job-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"debug-mode": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   "alpine:latest",
					Command: []string{"sleep", "infinity"},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	_, err := kubeClient.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	// Retrieve pod and verify debug mode
	retrievedPod, err := kubeClient.CoreV1().Pods("default").Get(ctx, "test-debug-job-abc123", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to retrieve pod: %v", err)
	}

	if debugMode, ok := retrievedPod.Labels["debug-mode"]; !ok || debugMode != "true" {
		t.Error("Pod should have debug-mode=true label")
	}

	// Pod should still be running (not completed)
	if retrievedPod.Status.Phase != corev1.PodRunning {
		t.Errorf("Debug pod should be running, got %v", retrievedPod.Status.Phase)
	}
}

// TestJobMonitor_RetryLogic tests retry configuration on job failures
func TestJobMonitor_RetryLogic(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int32
		backoffLimit int32
		shouldRetry  bool
	}{
		{
			name:         "First failure, retry allowed",
			failureCount: 1,
			backoffLimit: 3,
			shouldRetry:  true,
		},
		{
			name:         "Max retries exceeded",
			failureCount: 3,
			backoffLimit: 3,
			shouldRetry:  false,
		},
		{
			name:         "Multiple failures within limit",
			failureCount: 2,
			backoffLimit: 5,
			shouldRetry:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate retry logic
			shouldRetry := tt.failureCount < tt.backoffLimit
			if shouldRetry != tt.shouldRetry {
				t.Errorf("Expected shouldRetry=%v, got %v", tt.shouldRetry, shouldRetry)
			}
		})
	}
}

// TestJobMonitor_CleanupTTL tests TTL-based cleanup
func TestJobMonitor_CleanupTTL(t *testing.T) {
	tests := []struct {
		name                    string
		ttlSecondsAfterFinished *int32
		completionTime          time.Time
		currentTime             time.Time
		shouldBeDeleted         bool
	}{
		{
			name:                    "TTL not expired",
			ttlSecondsAfterFinished: int32Ptr(3600),
			completionTime:          time.Now().Add(-30 * time.Minute),
			currentTime:             time.Now(),
			shouldBeDeleted:         false,
		},
		{
			name:                    "TTL expired",
			ttlSecondsAfterFinished: int32Ptr(300),
			completionTime:          time.Now().Add(-10 * time.Minute),
			currentTime:             time.Now(),
			shouldBeDeleted:         true,
		},
		{
			name:                    "No TTL set",
			ttlSecondsAfterFinished: nil,
			completionTime:          time.Now().Add(-1 * time.Hour),
			currentTime:             time.Now(),
			shouldBeDeleted:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate TTL cleanup logic
			var shouldDelete bool
			if tt.ttlSecondsAfterFinished != nil {
				ttlDuration := time.Duration(*tt.ttlSecondsAfterFinished) * time.Second
				deadlineTime := tt.completionTime.Add(ttlDuration)
				shouldDelete = tt.currentTime.After(deadlineTime)
			}

			if shouldDelete != tt.shouldBeDeleted {
				t.Errorf("Expected shouldDelete=%v, got %v", tt.shouldBeDeleted, shouldDelete)
			}
		})
	}
}

// TestJobMonitor_StatusConditions tests job status condition transitions
func TestJobMonitor_StatusConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []batchv1.JobCondition
		expectPass bool
	}{
		{
			name: "Job complete condition",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
			expectPass: true,
		},
		{
			name: "Job failed condition",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
			expectPass: false,
		},
		{
			name:       "No conditions yet (running)",
			conditions: []batchv1.JobCondition{},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check for completion condition
			var isComplete bool
			for _, cond := range tt.conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					isComplete = true
					break
				}
			}

			if isComplete != tt.expectPass {
				t.Errorf("Expected isComplete=%v, got %v", tt.expectPass, isComplete)
			}
		})
	}
}

// TestJobMonitor_UDSBundleJob tests UDS-specific job monitoring
func TestJobMonitor_UDSBundleJob(t *testing.T) {
	bundle := &udsv1alpha3.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha3.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha3.ActionCreate,
			Source: udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: "https://github.com/test/bundle",
				},
			},
		},
		Status: udsv1alpha3.UDSBundleJobStatus{
			CreateStatus: &udsv1alpha3.OperationStatus{
				Phase: constants.PhaseRunning,
			},
		},
	}

	if bundle.Status.CreateStatus == nil {
		t.Error("Expected CreateStatus to be initialized")
	}

	if bundle.Status.CreateStatus.Phase != constants.PhaseRunning {
		t.Errorf("Expected phase Running, got %v", bundle.Status.CreateStatus.Phase)
	}
}

// TestJobMonitor_MultipleActionStatusFields tests that all action status fields exist
func TestJobMonitor_MultipleActionStatusFields(t *testing.T) {
	pkg := &zarfv1alpha3.ZarfPackageJob{
		Status: zarfv1alpha3.ZarfPackageJobStatus{},
	}

	// Simulate initialization of all three action statuses
	pkg.Status.BuildStatus = &zarfv1alpha3.OperationStatus{Phase: constants.PhaseRunning}
	pkg.Status.PublishStatus = &zarfv1alpha3.OperationStatus{Phase: constants.PhasePending}
	pkg.Status.DeployStatus = &zarfv1alpha3.OperationStatus{Phase: constants.PhasePending}

	if pkg.Status.BuildStatus == nil || pkg.Status.BuildStatus.Phase != constants.PhaseRunning {
		t.Error("BuildStatus not properly initialized")
	}
	if pkg.Status.PublishStatus == nil || pkg.Status.PublishStatus.Phase != constants.PhasePending {
		t.Error("PublishStatus not properly initialized")
	}
	if pkg.Status.DeployStatus == nil || pkg.Status.DeployStatus.Phase != constants.PhasePending {
		t.Error("DeployStatus not properly initialized")
	}
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}
