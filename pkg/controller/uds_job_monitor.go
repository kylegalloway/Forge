// Package controller implements Job monitoring for UDS bundle operations.
package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
)

// startJobMonitoring starts a goroutine to monitor UDS bundle Job completion
func (ctrl *UDSController) startJobMonitoring(ctx context.Context) {
	ticker := time.NewTicker(constants.JobMonitorInterval)
	defer ticker.Stop()

	klog.Info("Starting UDS bundle Job monitoring loop")

	for {
		select {
		case <-ctx.Done():
			klog.Info("UDS bundle Job monitoring stopped")
			return
		case <-ticker.C:
			if err := ctrl.checkJobStatuses(ctx); err != nil {
				klog.ErrorS(err, "Error checking UDS bundle job statuses")
			}
		}
	}
}

// checkJobStatuses checks all Jobs labeled with forge-uds labels and updates UDSPackageJob status
func (ctrl *UDSController) checkJobStatuses(ctx context.Context) error {
	// List all Jobs with forge-uds labels
	jobs, err := ctrl.kubeClient.BatchV1().Jobs(ctrl.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=forge-uds",
	})
	if err != nil {
		return fmt.Errorf("failed to list UDS bundle jobs: %w", err)
	}

	for _, job := range jobs.Items {
		if err := ctrl.processJobStatus(ctx, &job); err != nil {
			klog.ErrorS(err, "Failed to process UDS bundle job status", "job", job.Name, "namespace", job.Namespace)
			// Continue processing other jobs
		}
	}

	return nil
}

// processJobStatus checks a single Job and updates the corresponding UDSPackageJob status
func (ctrl *UDSController) processJobStatus(ctx context.Context, job *batchv1.Job) error {
	// Get the UDSPackageJob name from job labels
	bundleName, ok := job.Labels[constants.LabelPackage]
	if !ok {
		klog.V(4).InfoS("Job missing bundle label, skipping", "job", job.Name)
		return nil
	}

	action, ok := job.Labels[constants.LabelAction]
	if !ok {
		klog.V(4).InfoS("Job missing action label, skipping", "job", job.Name)
		return nil
	}

	// Get the UDSPackageJob resource
	unstructuredBundle, err := ctrl.dynamicClient.Resource(constants.UDSPackageJobGVR).
		Namespace(job.Namespace).
		Get(ctx, bundleName, metav1.GetOptions{})
	if err != nil {
		klog.V(4).InfoS("Failed to get UDSPackageJob, may be deleted", "bundle", bundleName, "error", err)
		return nil
	}

	bundle := &udsv1alpha2.UDSPackageJob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredBundle.Object, bundle); err != nil {
		return fmt.Errorf("failed to convert to UDSPackageJob: %w", err)
	}

	// Check if job is complete or failed
	jobComplete := false
	jobFailed := false

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			jobComplete = true
			break
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			jobFailed = true
			break
		}
	}

	if !jobComplete && !jobFailed {
		// Job still running
		return nil
	}

	// Update status based on job completion
	now := metav1.Now()

	if jobComplete {
		klog.InfoS("UDS bundle Job completed successfully", "job", job.Name, "action", action, "bundle", bundleName)

		// Update operation status
		switch action {
		case constants.ActionCreate:
			if bundle.Status.CreateStatus != nil {
				bundle.Status.CreateStatus.State = "Completed"
				bundle.Status.CreateStatus.CompletionTime = &now
			}
			ctrl.metrics.RecordBundleCreateCompleted(ctx, bundle.Namespace, bundle.Name)

		case constants.ActionPublish:
			if bundle.Status.PublishStatus != nil {
				bundle.Status.PublishStatus.State = "Completed"
				bundle.Status.PublishStatus.CompletionTime = &now
			}
			ctrl.metrics.RecordBundlePublishCompleted(ctx, bundle.Namespace, bundle.Name)

		case constants.ActionDeploy:
			if bundle.Status.DeployStatus != nil {
				bundle.Status.DeployStatus.State = "Completed"
				bundle.Status.DeployStatus.CompletionTime = &now
			}
			ctrl.metrics.RecordBundleDeployCompleted(ctx, bundle.Namespace, bundle.Name)
		}

		// Check if we need to chain to next action
		if err := ctrl.handleActionChaining(ctx, bundle, action); err != nil {
			klog.ErrorS(err, "Failed to chain next action", "bundle", bundleName, "action", action)
			return ctrl.markBundleFailed(ctx, bundle, fmt.Sprintf("Action chaining failed: %v", err))
		}

	} else if jobFailed {
		klog.ErrorS(nil, "UDS bundle Job failed", "job", job.Name, "action", action, "bundle", bundleName)

		// Update operation status
		failureMsg := "Job failed"
		switch action {
		case constants.ActionCreate:
			if bundle.Status.CreateStatus != nil {
				bundle.Status.CreateStatus.State = "Failed"
				bundle.Status.CreateStatus.CompletionTime = &now
				bundle.Status.CreateStatus.Message = failureMsg
			}
			ctrl.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)

		case constants.ActionPublish:
			if bundle.Status.PublishStatus != nil {
				bundle.Status.PublishStatus.State = "Failed"
				bundle.Status.PublishStatus.CompletionTime = &now
				bundle.Status.PublishStatus.Message = failureMsg
			}
			ctrl.metrics.RecordBundlePublishFailed(ctx, bundle.Namespace, bundle.Name)

		case constants.ActionDeploy:
			if bundle.Status.DeployStatus != nil {
				bundle.Status.DeployStatus.State = "Failed"
				bundle.Status.DeployStatus.CompletionTime = &now
				bundle.Status.DeployStatus.Message = failureMsg
			}
			ctrl.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		}

		return ctrl.markBundleFailed(ctx, bundle, fmt.Sprintf("%s action failed", action))
	}

	return nil
}

// handleActionChaining triggers the next action in compound action workflows
func (ctrl *UDSController) handleActionChaining(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob, completedAction string) error {
	action := bundle.Spec.Action

	klog.V(2).InfoS("Checking action chaining", "bundle", bundle.Name, "mainAction", action, "completedAction", completedAction)

	switch action {
	case udsv1alpha2.ActionCreatePublish:
		if completedAction == constants.ActionCreate {
			klog.InfoS("Chaining to Publish after Create", "bundle", bundle.Name)
			return ctrl.executePublish(ctx, bundle)
		} else if completedAction == constants.ActionPublish {
			return ctrl.markBundleCompleted(ctx, bundle)
		}

	case udsv1alpha2.ActionCreateDeploy:
		if completedAction == constants.ActionCreate {
			klog.InfoS("Chaining to Deploy after Create", "bundle", bundle.Name)
			return ctrl.executeDeploy(ctx, bundle)
		} else if completedAction == constants.ActionDeploy {
			return ctrl.markBundleCompleted(ctx, bundle)
		}

	case udsv1alpha2.ActionPublishDeploy:
		if completedAction == constants.ActionPublish {
			klog.InfoS("Chaining to Deploy after Publish", "bundle", bundle.Name)
			return ctrl.executeDeploy(ctx, bundle)
		} else if completedAction == constants.ActionDeploy {
			return ctrl.markBundleCompleted(ctx, bundle)
		}

	case udsv1alpha2.ActionCreatePublishDeploy:
		switch completedAction {
		case constants.ActionCreate:
			klog.InfoS("Chaining to Publish after Create (full pipeline)", "bundle", bundle.Name)
			return ctrl.executePublish(ctx, bundle)
		case constants.ActionPublish:
			klog.InfoS("Chaining to Deploy after Publish (full pipeline)", "bundle", bundle.Name)
			return ctrl.executeDeploy(ctx, bundle)
		case constants.ActionDeploy:
			return ctrl.markBundleCompleted(ctx, bundle)
		}

	default:
		// Single action - mark as completed
		return ctrl.markBundleCompleted(ctx, bundle)
	}

	return nil
}

// markBundleCompleted marks the UDSPackageJob as completed
func (ctrl *UDSController) markBundleCompleted(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob) error {
	klog.InfoS("Marking UDSPackageJob as completed", "bundle", bundle.Name, "namespace", bundle.Namespace)
	return ctrl.updateStatus(ctx, bundle, "Completed", "All actions completed successfully")
}

// markBundleFailed marks the UDSPackageJob as failed
func (ctrl *UDSController) markBundleFailed(ctx context.Context, bundle *udsv1alpha2.UDSPackageJob, message string) error {
	klog.InfoS("Marking UDSPackageJob as failed", "bundle", bundle.Name, "namespace", bundle.Namespace, "message", message)
	return ctrl.updateStatus(ctx, bundle, "Failed", message)
}
