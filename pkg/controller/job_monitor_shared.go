package controller

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/constants"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobStatus represents the current status of a Job.
type JobStatus struct {
	// Phase is the current phase (Running, Completed, Failed)
	Phase string

	// Message provides details about the status
	Message string

	// Completed indicates if the Job has finished
	Completed bool

	// CompletionTime when the Job completed
	CompletionTime *metav1.Time

	// StartTime when the Job started
	StartTime *metav1.Time
}

// GetJobStatus checks a Job and returns its current status.
// This function is shared by both Zarf and UDS controllers.
func GetJobStatus(job *batchv1.Job, actionName string) (*JobStatus, error) {
	if job == nil {
		return nil, fmt.Errorf("job is nil")
	}

	status := &JobStatus{
		Phase:     constants.PhaseRunning,
		Message:   fmt.Sprintf("%s job %s is running", actionName, job.Name),
		Completed: false,
		StartTime: job.Status.StartTime,
	}

	// Check Job conditions
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			status.Phase = constants.PhaseCompleted
			status.Message = fmt.Sprintf("%s job %s completed successfully", actionName, job.Name)
			status.Completed = true
			status.CompletionTime = condition.LastTransitionTime.DeepCopy()
			return status, nil
		}

		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			status.Phase = constants.PhaseFailed
			status.Message = fmt.Sprintf("%s job %s failed: %s", actionName, job.Name, condition.Message)
			status.Completed = true
			status.CompletionTime = condition.LastTransitionTime.DeepCopy()
			return status, nil
		}
	}

	// Job is still running
	return status, nil
}

// CalculateDuration calculates the duration of a Job in seconds.
// Returns 0 if startTime or completionTime is nil.
func CalculateDuration(startTime, completionTime *metav1.Time) float64 {
	if startTime == nil || completionTime == nil {
		return 0
	}
	return completionTime.Sub(startTime.Time).Seconds()
}

// IsJobCompleted checks if a Job has completed (either success or failure).
func IsJobCompleted(job *batchv1.Job) bool {
	if job == nil {
		return false
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// IsJobSuccessful checks if a Job completed successfully.
func IsJobSuccessful(job *batchv1.Job) bool {
	if job == nil {
		return false
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// IsJobFailed checks if a Job has failed.
func IsJobFailed(job *batchv1.Job) bool {
	if job == nil {
		return false
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
