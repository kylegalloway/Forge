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

	"github.com/kylegalloway/forge/pkg/actions/common"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
)

// startJobMonitoring starts a goroutine to monitor Job completion
func (ctrl *Controller) startJobMonitoring(ctx context.Context) {
	ticker := time.NewTicker(constants.JobMonitorInterval)
	defer ticker.Stop()

	klog.Info("Starting Job monitoring loop")

	for {
		select {
		case <-ctx.Done():
			klog.Info("Job monitoring stopped")
			return
		case <-ticker.C:
			if err := ctrl.checkJobStatuses(ctx); err != nil {
				klog.ErrorS(err, "Error checking job statuses")
			}
		}
	}
}

// checkJobStatuses checks all Jobs labeled with forge labels and updates ZarfPackageJob status
func (ctrl *Controller) checkJobStatuses(ctx context.Context) error {
	// List all Jobs with forge labels
	jobs, err := ctrl.kubeClient.BatchV1().Jobs(ctrl.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=forge",
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, job := range jobs.Items {
		if err := ctrl.processJobStatus(ctx, &job); err != nil {
			klog.ErrorS(err, "Failed to process job status", "job", job.Name, "namespace", job.Namespace)
			// Continue processing other jobs
		}
	}

	return nil
}

// processJobStatus checks a single Job and updates the corresponding ZarfPackageJob status
func (ctrl *Controller) processJobStatus(ctx context.Context, job *batchv1.Job) error {
	// Get the ZarfPackageJob name from job labels
	packageName, ok := job.Labels[constants.LabelPackage]
	if !ok {
		klog.V(4).InfoS("Job missing package label, skipping", "job", job.Name)
		return nil
	}

	action, ok := job.Labels[constants.LabelAction]
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

			// Record job completion metrics
			ctrl.metrics.RecordJobCompleted(ctx, job.Namespace, packageName, action)

			// Record action-specific completion metrics
			switch action {
			case constants.ActionBuild:
				ctrl.metrics.RecordPackageBuildCompleted(ctx, job.Namespace, packageName)
			case constants.ActionPublish:
				ctrl.metrics.RecordPackagePublishCompleted(ctx, job.Namespace, packageName)
			case constants.ActionDeploy:
				ctrl.metrics.RecordPackageDeployCompleted(ctx, job.Namespace, packageName)
			}

			// Calculate and record action duration if start time is available
			if job.Status.StartTime != nil {
				duration := completionTime.Sub(job.Status.StartTime.Time).Seconds()
				ctrl.metrics.RecordActionDuration(ctx, job.Namespace, packageName, action, duration, "success")
			}

			break
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			phase = "Failed"
			message = fmt.Sprintf("%s job %s failed: %s", action, job.Name, condition.Message)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()

			// Record job failure metrics
			ctrl.metrics.RecordJobFailed(ctx, job.Namespace, packageName, action)

			// Record action-specific failure metrics
			switch action {
			case constants.ActionBuild:
				ctrl.metrics.RecordPackageBuildFailed(ctx, job.Namespace, packageName)
			case constants.ActionPublish:
				ctrl.metrics.RecordPackagePublishFailed(ctx, job.Namespace, packageName)
			case constants.ActionDeploy:
				ctrl.metrics.RecordPackageDeployFailed(ctx, job.Namespace, packageName)
			}

			// Calculate and record action duration if start time is available
			if job.Status.StartTime != nil {
				duration := completionTime.Sub(job.Status.StartTime.Time).Seconds()
				ctrl.metrics.RecordActionDuration(ctx, job.Namespace, packageName, action, duration, "failure")
			}

			break
		}
	}

	// If job is still running, nothing to update
	if !completed {
		return nil
	}

	klog.InfoS("Job status changed", "job", job.Name, "package", packageName, "phase", phase)

	// Get the ZarfPackageJob resource
	unstrObj, err := ctrl.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(job.Namespace).Get(ctx, packageName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ZarfPackageJob: %w", err)
	}

	// Build operation status
	opStatus := map[string]interface{}{}

	// Determine which status field to update based on action
	statusField := ""
	artifactLocation := "/workspace/package.tar.zst" // Default artifact location

	switch action {
	case constants.ActionBuild:
		statusField = "buildStatus"
	case constants.ActionPublish:
		statusField = "publishStatus"
	case constants.ActionDeploy:
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

	// Update ZarfPackageJob status
	if err := ctrl.updateStatus(ctx, unstrObj, phase, message, opStatus); err != nil {
		return err
	}

	// Handle action chaining: if this job succeeded and is part of a chained workflow,
	// trigger the next action
	if phase == "Completed" {
		return ctrl.handleActionChaining(ctx, unstrObj, action, artifactLocation)
	}

	return nil
}

// handleActionChaining triggers the next action in a chained workflow
func (ctrl *Controller) handleActionChaining(ctx context.Context, unstrObj *unstructured.Unstructured, completedAction, artifactPath string) error {
	// Get the action from spec to determine if this is a chained workflow
	spec, ok := unstrObj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	action, ok := spec["action"].(string)
	if !ok {
		return nil
	}

	// Convert to typed ZarfPackageJob for easier handling
	pkg := &zarfv1alpha1.ZarfPackageJob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, pkg); err != nil {
		return fmt.Errorf("failed to convert to ZarfPackageJob: %w", err)
	}

	klog.InfoS("Checking for action chaining", "package", pkg.Name, "action", action, "completedAction", completedAction)

	// Determine if we need to trigger the next action
	var nextAction string

	switch action {
	case "BuildPublish":
		if completedAction == constants.ActionBuild {
			nextAction = constants.ActionPublish
		}
	case "BuildDeploy":
		if completedAction == constants.ActionBuild {
			nextAction = constants.ActionDeploy
		}
	case "PublishDeploy":
		if completedAction == constants.ActionPublish {
			nextAction = constants.ActionDeploy
		}
	case "BuildPublishDeploy":
		switch completedAction {
		case constants.ActionBuild:
			nextAction = constants.ActionPublish
		case constants.ActionPublish:
			nextAction = constants.ActionDeploy
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

	// Determine artifact PVC name for multi-action jobs
	var artifactPVCName string
	if isMultiActionZarfJob(pkg.Spec.Action) {
		artifactPVCName = fmt.Sprintf("%s-artifacts", pkg.Name)
		// Override artifactPath to use glob pattern for PVC location
		artifactPath = "/artifacts/*.tar.zst"
		klog.InfoS("Using shared artifact PVC for chained action", "package", pkg.Name, "pvc", artifactPVCName)
	}

	// Execute the next action handler
	var result *common.ActionResult
	var err error

	switch nextAction {
	case "publish":
		result, err = ctrl.publishHandler.Execute(ctx, pkg, artifactPath, artifactPVCName)
	case "deploy":
		result, err = ctrl.deployHandler.Execute(ctx, pkg, artifactPath, artifactPVCName)
	default:
		return fmt.Errorf("unknown next action: %s", nextAction)
	}

	if err != nil {
		klog.ErrorS(err, "Failed to execute next action", "package", pkg.Name, "action", nextAction)
		// Update status to Failed
		return ctrl.updateStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Failed to execute %s: %v", nextAction, err), nil)
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
		return ctrl.updateStatus(ctx, unstrObj, result.Phase, result.Message, opStatus)
	}

	return nil
}
