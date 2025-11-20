// Package controller implements Job monitoring for tracking operation completion.
package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
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
	artifactLocation := "/workspace/package.tar.zst" // Default artifact location

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
		"state":            phase,
		"message":          message,
		"completionTime":   completionTime.Format(time.RFC3339),
		"artifactLocation": artifactLocation,
	}

	// Update ZarfPackage status
	if err := c.updateStatus(ctx, u, phase, message, opStatus); err != nil {
		return err
	}

	// Handle action chaining: if this job succeeded and is part of a chained workflow,
	// trigger the next action
	if phase == "Completed" {
		return c.handleActionChaining(ctx, u, action, artifactLocation)
	}

	return nil
}

// handleActionChaining triggers the next action in a chained workflow
func (c *Controller) handleActionChaining(ctx context.Context, u *unstructured.Unstructured, completedAction, artifactPath string) error {
	// Get the action from spec to determine if this is a chained workflow
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	action, ok := spec["action"].(string)
	if !ok {
		return nil
	}

	// Convert to typed ZarfPackage for easier handling
	pkg := &zarfv1alpha1.ZarfPackage{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pkg); err != nil {
		return fmt.Errorf("failed to convert to ZarfPackage: %w", err)
	}

	klog.InfoS("Checking for action chaining", "package", pkg.Name, "action", action, "completedAction", completedAction)

	// Determine if we need to trigger the next action
	var nextAction string

	switch action {
	case "BuildPublish":
		if completedAction == "build" {
			nextAction = "publish"
		}
	case "BuildDeploy":
		if completedAction == "build" {
			nextAction = "deploy"
		}
	case "PublishDeploy":
		if completedAction == "publish" {
			nextAction = "deploy"
		}
	case "BuildPublishDeploy":
		if completedAction == "build" {
			nextAction = "publish"
		} else if completedAction == "publish" {
			nextAction = "deploy"
		}
	default:
		// Not a chained action, nothing to do
		return nil
	}

	// If there's no next action, we're done
	if nextAction == "" {
		return nil
	}

	klog.InfoS("Triggering next action in chain", "package", pkg.Name, "nextAction", nextAction, "artifactPath", artifactPath)

	// Execute the next action handler
	var result *actions.ActionResult
	var err error

	switch nextAction {
	case "publish":
		result, err = c.publishHandler.Execute(ctx, pkg, artifactPath)
	case "deploy":
		result, err = c.deployHandler.Execute(ctx, pkg, artifactPath)
	default:
		return fmt.Errorf("unknown next action: %s", nextAction)
	}

	if err != nil {
		klog.ErrorS(err, "Failed to execute next action", "package", pkg.Name, "action", nextAction)
		// Update status to Failed
		return c.updateStatus(ctx, u, "Failed", fmt.Sprintf("Failed to execute %s: %v", nextAction, err), nil)
	}

	if result != nil {
		// Update status for the next action
		opStatus := map[string]interface{}{}
		statusField := fmt.Sprintf("%sStatus", nextAction)
		opStatus[statusField] = map[string]interface{}{
			"state":     result.Phase,
			"message":   result.Message,
			"startTime": result.StartTime.Format(time.RFC3339),
		}
		return c.updateStatus(ctx, u, result.Phase, result.Message, opStatus)
	}

	return nil
}
