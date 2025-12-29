package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	"github.com/kylegalloway/forge/pkg/constants"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// MonitorConfig holds configuration for the generic monitor
type MonitorConfig struct {
	// ResourceType is the kind name (e.g., "ZarfPackageJob" or "UDSBundleJob")
	ResourceType string

	// LabelSelector for finding jobs (e.g., "app=forge" or "app=forge-uds")
	LabelSelector string

	// PrimaryAction is the name of the primary action ("build" for Zarf, "create" for UDS)
	PrimaryAction string

	// PrimaryStatusField is the status field name ("buildStatus" for Zarf, "createStatus" for UDS)
	PrimaryStatusField string

	// SupportsPVC indicates if this resource type supports artifact PVCs
	SupportsPVC bool
}

// GenericJobMonitor monitors Kubernetes Jobs and updates resource status
type GenericJobMonitor[T apiscommon.PackageResource] struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	resourceGVR   schema.GroupVersionResource
	metrics       MetricsRecorder[T]
	config        MonitorConfig

	// Action handlers for chaining
	primaryHandler common.ActionHandler[T]
	publishHandler common.ActionHandler[T]
	deployHandler  common.ActionHandler[T]

	// Status updater function - injected from controller
	statusUpdater func(ctx context.Context, obj *unstructured.Unstructured, phase, message string, opStatus map[string]interface{}) error
}

// NewGenericJobMonitor creates a new generic job monitor
func NewGenericJobMonitor[T apiscommon.PackageResource](
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	resourceGVR schema.GroupVersionResource,
	metrics MetricsRecorder[T],
	config MonitorConfig,
	primaryHandler common.ActionHandler[T],
	publishHandler common.ActionHandler[T],
	deployHandler common.ActionHandler[T],
	statusUpdater func(ctx context.Context, obj *unstructured.Unstructured, phase, message string, opStatus map[string]interface{}) error,
) *GenericJobMonitor[T] {
	return &GenericJobMonitor[T]{
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
		namespace:      namespace,
		resourceGVR:    resourceGVR,
		metrics:        metrics,
		config:         config,
		primaryHandler: primaryHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
		statusUpdater:  statusUpdater,
	}
}

// Start begins monitoring jobs
func (m *GenericJobMonitor[T]) Start(ctx context.Context) {
	ticker := time.NewTicker(constants.JobMonitorInterval)
	defer ticker.Stop()

	klog.InfoS("Starting job monitoring loop", "resourceType", m.config.ResourceType, "labelSelector", m.config.LabelSelector)

	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Job monitoring stopped", "resourceType", m.config.ResourceType)
			return
		case <-ticker.C:
			if err := m.checkJobStatuses(ctx); err != nil {
				klog.ErrorS(err, "Error checking job statuses", "resourceType", m.config.ResourceType)
			}
		}
	}
}

// checkJobStatuses checks all jobs with the configured label selector
func (m *GenericJobMonitor[T]) checkJobStatuses(ctx context.Context) error {
	jobs, err := m.kubeClient.BatchV1().Jobs(m.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: m.config.LabelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, job := range jobs.Items {
		if err := m.processJobStatus(ctx, &job); err != nil {
			klog.ErrorS(err, "Failed to process job status", "job", job.Name, "namespace", job.Namespace)
			// Continue processing other jobs
		}
	}

	return nil
}

// processJobStatus processes a single job and updates the corresponding resource
func (m *GenericJobMonitor[T]) processJobStatus(ctx context.Context, job *batchv1.Job) error {
	// Get resource name and action from job labels
	resourceName, ok := job.Labels[constants.LabelPackage]
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

			// Record metrics
			m.recordCompletionMetrics(ctx, job.Namespace, resourceName, action, job.Status.StartTime, completionTime)
			break
		}

		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			phase = "Failed"
			message = fmt.Sprintf("%s job %s failed: %s", action, job.Name, condition.Message)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()

			// Record metrics
			m.recordFailureMetrics(ctx, job.Namespace, resourceName, action, job.Status.StartTime, completionTime)
			break
		}
	}

	// If job is still running, nothing to update
	if !completed {
		return nil
	}

	klog.InfoS("Job status changed", "job", job.Name, "resource", resourceName, "phase", phase)

	// Get the resource
	unstrObj, err := m.dynamicClient.Resource(m.resourceGVR).Namespace(job.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		klog.V(4).InfoS("Failed to get resource, may be deleted", "resource", resourceName, "error", err)
		return nil
	}

	// Build operation status
	opStatus := map[string]interface{}{}
	statusField := m.getStatusFieldName(action)
	artifactLocation := "/workspace/package.tar.zst" // Default artifact location

	opStatus[statusField] = map[string]interface{}{
		"state":            phase,
		"message":          message,
		"completionTime":   completionTime.Format(time.RFC3339),
		"artifactLocation": artifactLocation,
	}

	// Update resource status
	if err := m.statusUpdater(ctx, unstrObj, phase, message, opStatus); err != nil {
		return err
	}

	// Handle action chaining if job succeeded
	if phase == "Completed" {
		return m.handleActionChaining(ctx, unstrObj, action, artifactLocation)
	}

	return nil
}

// getStatusFieldName returns the status field name for an action
func (m *GenericJobMonitor[T]) getStatusFieldName(action string) string {
	switch action {
	case m.config.PrimaryAction:
		return m.config.PrimaryStatusField
	case constants.ActionPublish:
		return "publishStatus"
	case constants.ActionDeploy:
		return "deployStatus"
	default:
		return action + "Status"
	}
}

// recordCompletionMetrics records metrics for successful job completion
func (m *GenericJobMonitor[T]) recordCompletionMetrics(ctx context.Context, namespace, name, action string, startTime, completionTime *metav1.Time) {
	// Record job completion
	m.metrics.RecordJobCompleted(ctx, namespace, name, action)

	// Record action-specific completion
	switch action {
	case m.config.PrimaryAction:
		m.metrics.RecordPrimaryActionCompleted(ctx, namespace, name)
	case constants.ActionPublish:
		m.metrics.RecordPublishCompleted(ctx, namespace, name)
	case constants.ActionDeploy:
		m.metrics.RecordDeployCompleted(ctx, namespace, name)
	}

	// Record duration if available
	if startTime != nil && completionTime != nil {
		duration := completionTime.Sub(startTime.Time).Seconds()
		m.metrics.RecordActionDuration(ctx, namespace, name, action, duration, "success")
	}
}

// recordFailureMetrics records metrics for failed job completion
func (m *GenericJobMonitor[T]) recordFailureMetrics(ctx context.Context, namespace, name, action string, startTime, completionTime *metav1.Time) {
	// Record job failure
	m.metrics.RecordJobFailed(ctx, namespace, name, action)

	// Record action-specific failure
	switch action {
	case m.config.PrimaryAction:
		m.metrics.RecordPrimaryActionFailed(ctx, namespace, name)
	case constants.ActionPublish:
		m.metrics.RecordPublishFailed(ctx, namespace, name)
	case constants.ActionDeploy:
		m.metrics.RecordDeployFailed(ctx, namespace, name)
	}

	// Record duration if available
	if startTime != nil && completionTime != nil {
		duration := completionTime.Sub(startTime.Time).Seconds()
		m.metrics.RecordActionDuration(ctx, namespace, name, action, duration, "failure")
	}
}

// handleActionChaining triggers the next action in a chained workflow
func (m *GenericJobMonitor[T]) handleActionChaining(ctx context.Context, unstrObj *unstructured.Unstructured, completedAction, artifactPath string) error {
	// Get the action from spec to determine if this is a chained workflow
	spec, ok := unstrObj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	action, ok := spec["action"].(string)
	if !ok {
		return nil
	}

	// Convert to typed resource
	var resource T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, &resource); err != nil {
		return fmt.Errorf("failed to convert to %s: %w", m.config.ResourceType, err)
	}

	klog.InfoS("Checking for action chaining", "resource", resource.GetName(), "action", action, "completedAction", completedAction)

	// Determine next action based on compound action pattern
	nextAction := m.determineNextAction(action, completedAction)
	if nextAction == "" {
		return nil
	}

	klog.InfoS("Triggering next action in chain", "resource", resource.GetName(), "nextAction", nextAction, "artifactPath", artifactPath)

	// Prepare options for next action
	opts := common.ExecuteOptions{
		ArtifactPath: artifactPath,
	}

	// Determine artifact PVC name if needed
	if m.config.SupportsPVC && m.isMultiActionJob(action) {
		opts.ArtifactPVCName = fmt.Sprintf("%s-artifacts", resource.GetName())
		opts.ArtifactPath = "/artifacts/*.tar.zst"
		klog.InfoS("Using shared artifact PVC for chained action", "resource", resource.GetName(), "pvc", opts.ArtifactPVCName)
	}

	// Execute the next action
	var result *actions.ActionResult
	var err error

	switch nextAction {
	case constants.ActionPublish:
		result, err = m.publishHandler.Execute(ctx, resource, opts)
	case constants.ActionDeploy:
		result, err = m.deployHandler.Execute(ctx, resource, opts)
	default:
		return fmt.Errorf("unknown next action: %s", nextAction)
	}

	if err != nil {
		klog.ErrorS(err, "Failed to execute next action", "resource", resource.GetName(), "action", nextAction)
		return m.statusUpdater(ctx, unstrObj, "Failed", fmt.Sprintf("Failed to execute %s: %v", nextAction, err), nil)
	}

	if result != nil {
		// Update status for the next action
		opStatus := map[string]interface{}{}
		statusField := m.getStatusFieldName(nextAction)
		opStatus[statusField] = map[string]interface{}{
			"state":     result.Phase,
			"message":   result.Message,
			"startTime": result.StartTime.Format(time.RFC3339),
		}
		return m.statusUpdater(ctx, unstrObj, result.Phase, result.Message, opStatus)
	}

	return nil
}

// determineNextAction determines the next action in a compound workflow
func (m *GenericJobMonitor[T]) determineNextAction(mainAction, completedAction string) string {
	// Map of compound actions to their sequences
	// Uses generic pattern: {Primary}Publish, {Primary}Deploy, etc.
	primaryAction := m.config.PrimaryAction

	switch mainAction {
	case primaryAction + "Publish", "CreatePublish": // BuildPublish or CreatePublish
		if completedAction == primaryAction || completedAction == "create" {
			return constants.ActionPublish
		}
	case primaryAction + "Deploy", "CreateDeploy": // BuildDeploy or CreateDeploy
		if completedAction == primaryAction || completedAction == "create" {
			return constants.ActionDeploy
		}
	case "PublishDeploy":
		if completedAction == constants.ActionPublish {
			return constants.ActionDeploy
		}
	case primaryAction + "PublishDeploy", "CreatePublishDeploy": // BuildPublishDeploy or CreatePublishDeploy
		switch completedAction {
		case primaryAction, "create":
			return constants.ActionPublish
		case constants.ActionPublish:
			return constants.ActionDeploy
		}
	}

	return ""
}

// isMultiActionJob checks if an action is a compound action that needs PVC support
func (m *GenericJobMonitor[T]) isMultiActionJob(action string) bool {
	primaryAction := m.config.PrimaryAction

	multiActions := []string{
		primaryAction + "Publish",
		primaryAction + "Deploy",
		"PublishDeploy",
		primaryAction + "PublishDeploy",
		"CreatePublish",
		"CreateDeploy",
		"CreatePublishDeploy",
	}

	for _, ma := range multiActions {
		if action == ma {
			return true
		}
	}

	return false
}
