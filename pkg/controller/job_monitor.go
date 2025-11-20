// Package controller implements Job monitoring for tracking operation completion.
package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// startJobMonitoring starts a goroutine to monitor Job completion
func (c *Controller) startJobMonitoring(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	klog.Info("Starting Job monitoring loop")

	for {
		select {
		case <-ctx.Done():
			klog.Info("Job monitoring stopped")
			return
		case <-ticker.C:
			if err := c.checkJobStatuses(ctx); err != nil {
				klog.ErrorS(err, "Error checking job statuses")
			}
		}
	}
}

// checkJobStatuses checks all Jobs labeled with forge labels and updates ZarfPackage status
func (c *Controller) checkJobStatuses(ctx context.Context) error {
	// List all Jobs with forge labels
	jobs, err := c.kubeClient.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=forge",
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, job := range jobs.Items {
		if err := c.processJobStatus(ctx, &job); err != nil {
			klog.ErrorS(err, "Failed to process job status", "job", job.Name, "namespace", job.Namespace)
			// Continue processing other jobs
		}
	}

	return nil
}

// processJobStatus checks a single Job and updates the corresponding ZarfPackage status
func (c *Controller) processJobStatus(ctx context.Context, job *batchv1.Job) error {
	// Get the ZarfPackage name from job labels
	packageName, ok := job.Labels["forge.zarf.dev/package"]
	if !ok {
		klog.V(4).InfoS("Job missing package label, skipping", "job", job.Name)
		return nil
	}

	action, ok := job.Labels["forge.zarf.dev/action"]
	if !ok {
		klog.V(4).InfoS("Job missing action label, skipping", "job", job.Name)
		return nil
	}

	// Check if job is complete or failed
	var phase, message string
	var completed bool
	var completionTime *metav1.Time

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			phase = "Completed"
			message = fmt.Sprintf("%s job %s completed successfully", action, job.Name)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()
			break
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			phase = "Failed"
			message = fmt.Sprintf("%s job %s failed: %s", action, job.Name, condition.Message)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()
			break
		}
	}

	// If job is still running, nothing to update
	if !completed {
		return nil
	}

	klog.InfoS("Job status changed", "job", job.Name, "package", packageName, "phase", phase)

	// Get the ZarfPackage resource
	u, err := c.dynamicClient.Resource(ZarfPackageGVR).Namespace(job.Namespace).Get(ctx, packageName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ZarfPackage: %w", err)
	}

	// Build operation status
	opStatus := map[string]interface{}{}

	// Determine which status field to update based on action
	statusField := ""
	switch action {
	case "build":
		statusField = "buildStatus"
	case "publish":
		statusField = "publishStatus"
	case "deploy":
		statusField = "deployStatus"
	default:
		klog.V(4).InfoS("Unknown action type", "action", action, "job", job.Name)
		return nil
	}

	opStatus[statusField] = map[string]interface{}{
		"state":          phase,
		"message":        message,
		"completionTime": completionTime.Format(time.RFC3339),
	}

	// Update ZarfPackage status
	return c.updateStatus(ctx, u, phase, message, opStatus)
}

// getJobLogs retrieves logs from a completed job (useful for debugging)
func (c *Controller) getJobLogs(ctx context.Context, job *batchv1.Job) (string, error) {
	// List pods for this job
	pods, err := c.kubeClient.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", job.Name)
	}

	// Get logs from first pod (there should only be one for our jobs with backoffLimit=0)
	pod := pods.Items[0]

	// Try to get logs from main container
	for _, container := range pod.Spec.Containers {
		req := c.kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container.Name,
			TailLines: ptr(int64(100)), // Last 100 lines
		})

		logs, err := req.DoRaw(ctx)
		if err != nil {
			klog.V(4).InfoS("Failed to get logs for container", "pod", pod.Name, "container", container.Name, "error", err)
			continue
		}

		return string(logs), nil
	}

	return "", fmt.Errorf("failed to retrieve logs from any container in pod %s", pod.Name)
}

// ptr returns a pointer to the given value
func ptr[T any](v T) *T {
	return &v
}
