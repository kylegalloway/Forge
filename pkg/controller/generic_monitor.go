package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/logging"
	"github.com/kylegalloway/forge/pkg/resources"
	"github.com/kylegalloway/forge/pkg/retry"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	logger        *logging.Logger

	// Action handlers for chaining
	primaryHandler common.ActionHandler[T]
	publishHandler common.ActionHandler[T]
	deployHandler  common.ActionHandler[T]

	// Status updater function - injected from controller
	statusUpdater func(ctx context.Context, obj *unstructured.Unstructured, phase, message string, opStatus map[string]interface{}) error

	// jobStore provides cached job data from a shared informer (optional, falls back to API calls)
	jobStore cache.Store

	// requeueFunc is called when a job completes/fails and frees capacity,
	// triggering re-reconciliation of queued resources
	requeueFunc func(namespace string)
}

// MonitorOption is a functional option for configuring the GenericJobMonitor.
type MonitorOption[T apiscommon.PackageResource] func(*GenericJobMonitor[T])

// WithJobStore sets the job cache store for the monitor, replacing API List calls.
func WithJobStore[T apiscommon.PackageResource](store cache.Store) MonitorOption[T] {
	return func(m *GenericJobMonitor[T]) {
		m.jobStore = store
	}
}

// WithRequeueFunc sets the requeue function called when a job completes/fails.
func WithRequeueFunc[T apiscommon.PackageResource](fn func(namespace string)) MonitorOption[T] {
	return func(m *GenericJobMonitor[T]) {
		m.requeueFunc = fn
	}
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
	opts ...MonitorOption[T],
) *GenericJobMonitor[T] {
	m := &GenericJobMonitor[T]{
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
		namespace:      namespace,
		resourceGVR:    resourceGVR,
		metrics:        metrics,
		config:         config,
		logger:         logging.NewLogger("monitor"),
		primaryHandler: primaryHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
		statusUpdater:  statusUpdater,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
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

// checkJobStatuses checks all jobs with the configured label selector.
// When a jobStore is configured, it reads from the informer cache instead of making API calls.
func (m *GenericJobMonitor[T]) checkJobStatuses(ctx context.Context) error {
	startTime := time.Now()

	m.logger.Debug(ctx, "Starting job status check",
		"labelSelector", m.config.LabelSelector,
		"namespace", m.namespace,
		"usingCache", m.jobStore != nil,
	)

	var jobsToProcess []*batchv1.Job

	if m.jobStore != nil {
		// Read from informer cache - no API call needed
		items := m.jobStore.List()
		for _, item := range items {
			job, ok := item.(*batchv1.Job)
			if !ok {
				continue
			}
			// Filter by namespace if watching a specific namespace
			if m.namespace != "" && job.Namespace != m.namespace {
				continue
			}
			jobsToProcess = append(jobsToProcess, job)
		}
	} else {
		// Fall back to API List call
		jobs, err := m.kubeClient.BatchV1().Jobs(m.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: m.config.LabelSelector,
		})
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}
		for i := range jobs.Items {
			jobsToProcess = append(jobsToProcess, &jobs.Items[i])
		}
	}

	m.logger.Debug(ctx, "Found jobs to check",
		"jobCount", len(jobsToProcess),
	)

	for _, job := range jobsToProcess {
		if err := m.processJobStatus(ctx, job); err != nil {
			klog.ErrorS(err, "Failed to process job status", "job", job.Name, "namespace", job.Namespace)
			// Continue processing other jobs
		}
	}

	m.logger.Debug(ctx, "Completed job status check",
		"jobCount", len(jobsToProcess),
		"duration", time.Since(startTime),
	)

	return nil
}

// processJobStatus processes a single job and updates the corresponding resource
func (m *GenericJobMonitor[T]) processJobStatus(ctx context.Context, job *batchv1.Job) error {
	startTime := time.Now()

	// Get resource name and action from job labels
	resourceName, ok := job.Labels[constants.LabelPackage]
	if !ok {
		m.logger.Debug(ctx, "Job missing package label, skipping",
			"job", job.Name,
			"namespace", job.Namespace,
		)
		return nil
	}

	action, ok := job.Labels[constants.LabelAction]
	if !ok {
		m.logger.Debug(ctx, "Job missing action label, skipping",
			"job", job.Name,
			"namespace", job.Namespace,
			"resource", resourceName,
		)
		return nil
	}

	// Set up logging context with correlation ID
	ctx = logging.WithCorrelationID(ctx, logging.GenerateCorrelationID(job.Namespace, resourceName, action))
	ctx = logging.WithJobName(ctx, job.Name)
	ctx = logging.WithNamespace(ctx, job.Namespace)
	ctx = logging.WithAction(ctx, action)

	m.logger.Debug(ctx, "Processing job status",
		"resource", resourceName,
		"active", job.Status.Active,
		"succeeded", job.Status.Succeeded,
		"failed", job.Status.Failed,
		"conditions", len(job.Status.Conditions),
	)

	// Check if job is complete or failed
	var phase, message string
	var completed bool
	var completionTime *metav1.Time

	for _, condition := range job.Status.Conditions {
		m.logger.Debug(ctx, "Evaluating job condition",
			"conditionType", condition.Type,
			"conditionStatus", condition.Status,
			"reason", condition.Reason,
			"message", condition.Message,
		)

		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			phase = constants.PhaseCompleted
			message = fmt.Sprintf("%s job %s completed successfully", action, job.Name)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()

			m.logger.Debug(ctx, "Job completed successfully",
				"resource", resourceName,
				"completionTime", completionTime,
			)

			// Record metrics
			m.recordCompletionMetrics(ctx, job.Namespace, resourceName, action, job.Status.StartTime, completionTime)
			break
		}

		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			m.logger.Debug(ctx, "Job failed, checking retry policy",
				"resource", resourceName,
				"failureMessage", condition.Message,
			)
			// Get the resource to check retry policy
			unstrObj, err := m.dynamicClient.Resource(m.resourceGVR).Namespace(job.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
			if err != nil {
				m.logger.Debug(ctx, "Failed to get resource, may be deleted",
					"resource", resourceName,
					"error", err,
				)
				return nil
			}

			// Get current retry count from operation status
			statusField := m.getStatusFieldName(action)
			currentRetryCount := m.extractRetryCount(unstrObj, statusField)

			m.logger.Debug(ctx, "Checking retry policy",
				"resource", resourceName,
				"statusField", statusField,
				"currentRetryCount", currentRetryCount,
			)

			// Check if we should retry
			retryPolicy, err := m.getRetryPolicy(unstrObj, action)
			if err != nil {
				m.logger.Error(ctx, err, "Failed to get retry policy",
					"resource", resourceName,
				)
			}

			if retryPolicy != nil {
				m.logger.Debug(ctx, "Retry policy found",
					"resource", resourceName,
					"maxRetries", retryPolicy.MaxRetries,
					"initialBackoff", retryPolicy.InitialBackoff,
				)

				tracker := retry.NewTracker(retryPolicy)
				decision, err := tracker.RecordFailure(ctx, currentRetryCount, condition.Message)
				if err != nil {
					m.logger.Error(ctx, err, "Failed to record failure",
						"resource", resourceName,
					)
				}

				if decision != nil && decision.ShouldRetry {
					m.logger.Debug(ctx, "Scheduling retry",
						"resource", resourceName,
						"retryAt", decision.RetryAt,
						"attempt", currentRetryCount+1,
						"reason", decision.Reason,
					)

					// Update status to "Retrying"
					opStatus := map[string]interface{}{}
					opStatus[statusField] = map[string]interface{}{
						constants.StatusKeyState:             constants.PhaseRetrying,
						constants.StatusKeyRetryCount:        currentRetryCount + 1,
						constants.StatusKeyNextRetryTime:     metav1.NewTime(decision.RetryAt).Format(time.RFC3339),
						constants.StatusKeyLastFailureReason: condition.Message,
						constants.StatusKeyMessage:           decision.Reason,
					}

					// Delete failed job so it can be recreated
					if err := m.deleteFailedJob(ctx, job); err != nil {
						m.logger.Error(ctx, err, "Failed to delete failed job for retry")
					}

					// Update resource status
					return m.statusUpdater(ctx, unstrObj, constants.PhaseRetrying, decision.Reason, opStatus)
				}

				m.logger.Debug(ctx, "Retry not possible",
					"resource", resourceName,
					"shouldRetry", decision != nil && decision.ShouldRetry,
					"currentRetryCount", currentRetryCount,
				)
			} else {
				m.logger.Debug(ctx, "No retry policy configured",
					"resource", resourceName,
				)
			}

			// Not retryable or max retries exhausted - mark as failed
			phase = constants.PhaseFailed
			message = fmt.Sprintf("%s job %s failed: %s", action, job.Name, condition.Message)
			completed = true
			completionTime = condition.LastTransitionTime.DeepCopy()

			m.logger.Debug(ctx, "Job marked as failed",
				"resource", resourceName,
				"failureMessage", condition.Message,
			)

			// Record metrics
			m.recordFailureMetrics(ctx, job.Namespace, resourceName, action, job.Status.StartTime, completionTime)
			break
		}
	}

	// If job is still running, nothing to update
	if !completed {
		m.logger.Debug(ctx, "Job still running, no status update needed",
			"resource", resourceName,
		)
		return nil
	}

	m.logger.Debug(ctx, "Job status changed",
		"resource", resourceName,
		"phase", phase,
		"duration", time.Since(startTime),
	)

	// Get the resource
	unstrObj, err := m.dynamicClient.Resource(m.resourceGVR).Namespace(job.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		m.logger.Debug(ctx, "Failed to get resource, may be deleted",
			"resource", resourceName,
			"error", err,
		)
		return nil
	}

	// Extract spec for later use
	spec, ok := unstrObj.Object["spec"].(map[string]interface{})
	if !ok {
		m.logger.Error(ctx, nil, "Failed to extract spec from resource",
			"resource", resourceName,
		)
		return fmt.Errorf("failed to extract spec from resource %s", resourceName)
	}

	// Build operation status
	opStatus := map[string]interface{}{}
	statusField := m.getStatusFieldName(action)
	artifactLocation := constants.DefaultArtifactPath // Default artifact location

	m.logger.Debug(ctx, "Updating operation status",
		"resource", resourceName,
		"statusField", statusField,
		"phase", phase,
		"artifactLocation", artifactLocation,
	)

	opStatus[statusField] = map[string]interface{}{
		constants.StatusKeyState:            phase,
		constants.StatusKeyMessage:          message,
		constants.StatusKeyCompletionTime:   completionTime.Format(time.RFC3339),
		constants.StatusKeyArtifactLocation: artifactLocation,
	}

	// Update resource status
	if err := m.statusUpdater(ctx, unstrObj, phase, message, opStatus); err != nil {
		m.logger.Error(ctx, err, "Failed to update resource status",
			"resource", resourceName,
		)
		return err
	}

	// Handle resource adoption if job succeeded and action is deploy
	if phase == constants.PhaseCompleted && action == constants.ActionDeploy {
		m.logger.Debug(ctx, "Checking for resource adoption after deploy",
			"resource", resourceName,
		)
		if err := m.adoptDeployedResources(ctx, unstrObj); err != nil {
			// Don't fail the deployment, just log warning
			m.logger.Warning(ctx, "Failed to adopt resources",
				"resource", resourceName,
				"error", err,
			)
		}
	}

	// Handle action chaining if job succeeded
	if phase == constants.PhaseCompleted {
		m.logger.Debug(ctx, "Checking for action chaining",
			"resource", resourceName,
			"completedAction", action,
		)
		err := m.handleActionChaining(ctx, unstrObj, action, artifactLocation)
		// If there's no next action or chaining failed, clean up PVC if needed
		currentAction, ok := spec["action"].(string)
		if !ok {
			m.logger.Debug(ctx, "Could not determine current action for chaining check",
				"resource", resourceName,
			)
			currentAction = ""
		}
		nextAction := m.determineNextAction(currentAction, action)
		if err != nil || nextAction == "" {
			m.logger.Debug(ctx, "No next action or chaining failed, cleaning up PVC",
				"resource", resourceName,
				"nextAction", nextAction,
				"chainingError", err,
			)
			m.cleanupArtifactPVCIfNeeded(ctx, unstrObj, resourceName)
		}
		return err
	}

	// If job failed, clean up PVC if needed
	if phase == constants.PhaseFailed {
		m.logger.Debug(ctx, "Job failed, cleaning up PVC",
			"resource", resourceName,
		)
		m.cleanupArtifactPVCIfNeeded(ctx, unstrObj, resourceName)
	}

	// Notify controller that capacity may have been freed
	if completed && m.requeueFunc != nil {
		m.requeueFunc(job.Namespace)
	}

	return nil
}

// getStatusFieldName returns the status field name for an action
func (m *GenericJobMonitor[T]) getStatusFieldName(action string) string {
	switch action {
	case m.config.PrimaryAction:
		return m.config.PrimaryStatusField
	case constants.ActionPublish:
		return constants.StatusFieldPublish
	case constants.ActionDeploy:
		return constants.StatusFieldDeploy
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
		m.logger.Debug(ctx, "No spec found, skipping action chaining")
		return nil
	}

	action, ok := spec["action"].(string)
	if !ok {
		m.logger.Debug(ctx, "No action in spec, skipping action chaining")
		return nil
	}

	// Convert to typed resource
	var resource T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, &resource); err != nil {
		m.logger.Error(ctx, err, "Failed to convert unstructured to typed resource")
		return fmt.Errorf("failed to convert to %s: %w", m.config.ResourceType, err)
	}

	m.logger.Debug(ctx, "Evaluating action chaining",
		"resource", resource.GetName(),
		"specAction", action,
		"completedAction", completedAction,
		"isMultiAction", m.isMultiActionJob(action),
	)

	// Determine next action based on compound action pattern
	nextAction := m.determineNextAction(action, completedAction)
	if nextAction == "" {
		m.logger.Debug(ctx, "No next action in chain",
			"resource", resource.GetName(),
			"specAction", action,
			"completedAction", completedAction,
		)
		return nil
	}

	m.logger.Debug(ctx, "Triggering next action in chain",
		"resource", resource.GetName(),
		"nextAction", nextAction,
		"artifactPath", artifactPath,
	)

	// Prepare options for next action
	opts := common.ExecuteOptions{
		ArtifactPath: artifactPath,
	}

	// Determine artifact PVC name if needed
	if m.config.SupportsPVC && m.isMultiActionJob(action) {
		opts.ArtifactPVCName = fmt.Sprintf("%s-artifacts", resource.GetName())
		opts.ArtifactPath = "/artifacts/*.tar.zst"
		m.logger.Debug(ctx, "Using shared artifact PVC for chained action",
			"resource", resource.GetName(),
			"pvc", opts.ArtifactPVCName,
			"artifactPath", opts.ArtifactPath,
		)
	}

	// Execute the next action
	m.logger.Debug(ctx, "Executing chained action",
		"resource", resource.GetName(),
		"nextAction", nextAction,
	)

	var result *actions.ActionResult
	var err error

	switch nextAction {
	case constants.ActionPublish:
		result, err = m.publishHandler.Execute(ctx, resource, opts)
	case constants.ActionDeploy:
		result, err = m.deployHandler.Execute(ctx, resource, opts)
	default:
		m.logger.Error(ctx, nil, "Unknown next action in chain",
			"nextAction", nextAction,
		)
		return fmt.Errorf("unknown next action: %s", nextAction)
	}

	if err != nil {
		m.logger.Error(ctx, err, "Failed to execute chained action",
			"resource", resource.GetName(),
			"nextAction", nextAction,
		)
		return m.statusUpdater(ctx, unstrObj, constants.PhaseFailed, fmt.Sprintf("Failed to execute %s: %v", nextAction, err), nil)
	}

	if result != nil {
		m.logger.Debug(ctx, "Chained action started successfully",
			"resource", resource.GetName(),
			"nextAction", nextAction,
			"jobName", result.JobName,
			"phase", result.Phase,
		)

		// Update status for the next action
		opStatus := map[string]interface{}{}
		statusField := m.getStatusFieldName(nextAction)
		opStatus[statusField] = map[string]interface{}{
			constants.StatusKeyState:   result.Phase,
			constants.StatusKeyMessage: result.Message,
			"startTime":                result.StartTime.Format(time.RFC3339),
		}
		return m.statusUpdater(ctx, unstrObj, result.Phase, result.Message, opStatus)
	}

	return nil
}

// determineNextAction determines the next action in a compound workflow.
// mainAction is the spec.action value (PascalCase, e.g., "BuildPublish").
// completedAction is the job label action (lowercase, e.g., "build").
func (m *GenericJobMonitor[T]) determineNextAction(mainAction, completedAction string) string {
	// Map of compound actions to their sequences.
	// mainAction uses PascalCase constants (SpecAction*) matching the CRD enum.
	// completedAction uses lowercase constants (Action*) matching job labels.
	primaryAction := m.config.PrimaryAction // lowercase, e.g., "build" or "create"

	switch mainAction {
	case constants.SpecActionBuildPublish, constants.SpecActionCreatePublish:
		if completedAction == primaryAction || completedAction == constants.ActionCreate {
			return constants.ActionPublish
		}
	case constants.SpecActionBuildDeploy, constants.SpecActionCreateDeploy:
		if completedAction == primaryAction || completedAction == constants.ActionCreate {
			return constants.ActionDeploy
		}
	case constants.SpecActionPublishDeploy:
		if completedAction == constants.ActionPublish {
			return constants.ActionDeploy
		}
	case constants.SpecActionBuildPublishDeploy, constants.SpecActionCreatePublishDeploy:
		switch completedAction {
		case primaryAction, constants.ActionCreate:
			return constants.ActionPublish
		case constants.ActionPublish:
			return constants.ActionDeploy
		}
	}

	return ""
}

// isMultiActionJob checks if an action is a compound action that needs PVC support.
// action is the spec.action value (PascalCase, e.g., "BuildPublish").
func (m *GenericJobMonitor[T]) isMultiActionJob(action string) bool {
	multiActions := []string{
		constants.SpecActionBuildPublish,
		constants.SpecActionBuildDeploy,
		constants.SpecActionBuildPublishDeploy,
		constants.SpecActionCreatePublish,
		constants.SpecActionCreateDeploy,
		constants.SpecActionCreatePublishDeploy,
		constants.SpecActionPublishDeploy,
	}

	for _, ma := range multiActions {
		if action == ma {
			return true
		}
	}

	return false
}

// getRetryPolicy extracts the retry policy for a given action from the resource spec
func (m *GenericJobMonitor[T]) getRetryPolicy(obj *unstructured.Unstructured, action string) (*retry.Policy, error) {
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	// Determine which config to check based on action
	var configName string
	switch action {
	case constants.ActionBuild:
		configName = constants.ActionBuild
	case constants.ActionCreate:
		// Create doesn't have a config section in UDS, return nil
		return nil, nil
	case constants.ActionPublish:
		configName = constants.ActionPublish
	case constants.ActionDeploy:
		configName = constants.ActionDeploy
	default:
		return nil, nil
	}

	config, ok := spec[configName].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	retryMap, ok := config["retry"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	// Determine resource type and parse accordingly
	resourceType := m.config.ResourceType
	if resourceType == constants.ResourceTypeZarfPackageJob {
		// Convert map to Zarf RetryPolicy
		apiPolicy := &zarfv1alpha3.RetryPolicy{}
		if maxRetries, ok := retryMap["maxRetries"].(int64); ok {
			val := int32(maxRetries) //nolint:gosec // G115: MaxRetries is validated by API schema
			apiPolicy.MaxRetries = &val
		}
		if initialBackoff, ok := retryMap["initialBackoff"].(string); ok {
			apiPolicy.InitialBackoff = initialBackoff
		}
		if maxBackoff, ok := retryMap["maxBackoff"].(string); ok {
			apiPolicy.MaxBackoff = maxBackoff
		}
		if backoffMultiplier, ok := retryMap["backoffMultiplier"].(int64); ok {
			val := int32(backoffMultiplier) //nolint:gosec // G115: BackoffMultiplier is validated by API schema
			apiPolicy.BackoffMultiplier = &val
		}
		if retryableErrors, ok := retryMap["retryableErrors"].([]interface{}); ok {
			for _, err := range retryableErrors {
				if errStr, ok := err.(string); ok {
					apiPolicy.RetryableErrors = append(apiPolicy.RetryableErrors, errStr)
				}
			}
		}
		return retry.ParseZarfPolicy(apiPolicy)
	}

	// Convert map to UDS RetryPolicy
	apiPolicy := &udsv1alpha3.RetryPolicy{}
	if maxRetries, ok := retryMap["maxRetries"].(int64); ok {
		val := int32(maxRetries) //nolint:gosec // G115: MaxRetries is validated by API schema
		apiPolicy.MaxRetries = &val
	}
	if initialBackoff, ok := retryMap["initialBackoff"].(string); ok {
		apiPolicy.InitialBackoff = initialBackoff
	}
	if maxBackoff, ok := retryMap["maxBackoff"].(string); ok {
		apiPolicy.MaxBackoff = maxBackoff
	}
	if backoffMultiplier, ok := retryMap["backoffMultiplier"].(int64); ok {
		val := int32(backoffMultiplier) //nolint:gosec // G115: BackoffMultiplier is validated by API schema
		apiPolicy.BackoffMultiplier = &val
	}
	if retryableErrors, ok := retryMap["retryableErrors"].([]interface{}); ok {
		for _, err := range retryableErrors {
			if errStr, ok := err.(string); ok {
				apiPolicy.RetryableErrors = append(apiPolicy.RetryableErrors, errStr)
			}
		}
	}
	return retry.ParseUDSPolicy(apiPolicy)
}

// extractRetryCount safely extracts retry count from operation status
func (m *GenericJobMonitor[T]) extractRetryCount(obj *unstructured.Unstructured, statusField string) int32 {
	status, ok := obj.Object["status"].(map[string]interface{})
	if !ok {
		return 0
	}

	opStatus, ok := status[statusField].(map[string]interface{})
	if !ok {
		return 0
	}

	retryCount, ok := opStatus["retryCount"].(int64)
	if !ok {
		// Try float64 (JSON unmarshaling default for numbers)
		retryCountFloat, ok := opStatus["retryCount"].(float64)
		if ok {
			return int32(retryCountFloat) //nolint:gosec // G115: retryCount is small, bounded by MaxRetries
		}
		return 0
	}

	return int32(retryCount) //nolint:gosec // G115: retryCount is small, bounded by MaxRetries
}

// deleteFailedJob deletes a failed job so it can be recreated during retry
func (m *GenericJobMonitor[T]) deleteFailedJob(ctx context.Context, job *batchv1.Job) error {
	m.logger.Debug(ctx, "Deleting failed job for retry",
		"job", job.Name,
		"namespace", job.Namespace,
		"propagationPolicy", "Background",
	)

	deletePolicy := metav1.DeletePropagationBackground
	err := m.kubeClient.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})

	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}

// cleanupArtifactPVCIfNeeded deletes the artifact PVC if retainArtifactPVC is false
func (m *GenericJobMonitor[T]) cleanupArtifactPVCIfNeeded(ctx context.Context, obj *unstructured.Unstructured, resourceName string) {
	// Only cleanup if PVC support is enabled
	if !m.config.SupportsPVC {
		m.logger.Debug(ctx, "PVC support not enabled, skipping cleanup",
			"resource", resourceName,
		)
		return
	}

	// Check spec for PVC settings
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		m.logger.Debug(ctx, "No spec found, skipping PVC cleanup",
			"resource", resourceName,
		)
		return
	}

	// Check if PVC was even created (useArtifactPVC)
	usePVC, usePVCOK := spec["useArtifactPVC"].(bool)
	if usePVCOK && !usePVC {
		m.logger.Debug(ctx, "useArtifactPVC is false, no PVC to cleanup",
			"resource", resourceName,
		)
		return
	}

	// Check if retainArtifactPVC is set to false
	retainPVC, ok := spec["retainArtifactPVC"].(bool)
	if !ok {
		m.logger.Debug(ctx, "retainArtifactPVC not specified, defaulting to retain",
			"resource", resourceName,
		)
		return
	}

	if retainPVC {
		m.logger.Debug(ctx, "retainArtifactPVC is true, keeping PVC",
			"resource", resourceName,
		)
		return
	}

	// Delete the artifact PVC
	pvcName := fmt.Sprintf("%s-artifacts", resourceName)
	m.logger.Debug(ctx, "Deleting artifact PVC as requested by retainArtifactPVC=false",
		"resource", resourceName,
		"pvc", pvcName,
		"namespace", obj.GetNamespace(),
	)

	err := m.kubeClient.CoreV1().PersistentVolumeClaims(obj.GetNamespace()).Delete(
		ctx, pvcName, metav1.DeleteOptions{})

	if err != nil {
		m.logger.Error(ctx, err, "Failed to delete artifact PVC",
			"pvc", pvcName,
			"namespace", obj.GetNamespace(),
		)
		return
	}

	m.logger.Debug(ctx, "Successfully deleted artifact PVC",
		"pvc", pvcName,
		"namespace", obj.GetNamespace(),
	)
}

// adoptDeployedResources handles resource adoption after successful deployment
func (m *GenericJobMonitor[T]) adoptDeployedResources(ctx context.Context, obj *unstructured.Unstructured) error {
	// Extract deploy config from spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		m.logger.Debug(ctx, "No spec found, skipping adoption",
			"resource", obj.GetName(),
		)
		return nil
	}

	deploy, ok := spec["deploy"].(map[string]interface{})
	if !ok {
		m.logger.Debug(ctx, "No deploy config found, skipping adoption",
			"resource", obj.GetName(),
		)
		return nil
	}

	// Check if adoption policy is set
	adoptionPolicyStr, ok := deploy["adoptionPolicy"].(string)
	if !ok || adoptionPolicyStr == "" {
		m.logger.Debug(ctx, "No adoption policy set, skipping adoption",
			"resource", obj.GetName(),
		)
		return nil
	}

	// Only proceed if policy is "Adopt"
	if adoptionPolicyStr != "Adopt" {
		m.logger.Debug(ctx, "Adoption policy is not 'Adopt', skipping",
			"resource", obj.GetName(),
			"policy", adoptionPolicyStr,
		)
		return nil
	}

	// Get resource selector
	selectorMap, ok := deploy["resourceSelector"].(map[string]interface{})
	if !ok {
		m.logger.Error(ctx, nil, "resourceSelector is required when adoptionPolicy is 'Adopt'",
			"resource", obj.GetName(),
		)
		return fmt.Errorf("resourceSelector is required when adoptionPolicy is 'Adopt'")
	}

	// Extract namespaces for discovery
	var namespaces []string
	if nsArray, ok := selectorMap["namespaces"].([]interface{}); ok {
		for _, ns := range nsArray {
			if nsStr, ok := ns.(string); ok {
				namespaces = append(namespaces, nsStr)
			}
		}
	}

	// Default to deploy namespace if not specified
	if len(namespaces) == 0 {
		deployNS, ok := deploy["namespace"].(string)
		if ok && deployNS != "" {
			namespaces = []string{deployNS}
		} else {
			namespaces = []string{obj.GetNamespace()}
		}
	}

	m.logger.Debug(ctx, "Starting resource adoption",
		"resource", obj.GetName(),
		"namespaces", namespaces,
		"policy", adoptionPolicyStr,
	)

	// Determine resource type and parse selector accordingly
	resourceType := m.config.ResourceType
	var discovered []resources.ResourceReference
	var err error

	discoverer := resources.NewDiscoverer(m.dynamicClient)

	m.logger.Debug(ctx, "Discovering resources for adoption",
		"resource", obj.GetName(),
		"resourceType", resourceType,
	)

	if resourceType == constants.ResourceTypeZarfPackageJob {
		// Convert map to Zarf ResourceSelector
		selector := m.parseZarfResourceSelector(selectorMap)
		discovered, err = discoverer.DiscoverZarfResources(ctx, selector, namespaces)
	} else {
		// Convert map to UDS ResourceSelector
		selector := m.parseUDSResourceSelector(selectorMap)
		discovered, err = discoverer.DiscoverUDSResources(ctx, selector, namespaces)
	}

	if err != nil {
		m.logger.Error(ctx, err, "Failed to discover resources for adoption",
			"resource", obj.GetName(),
		)
		return fmt.Errorf("failed to discover resources: %w", err)
	}

	m.logger.Debug(ctx, "Discovered resources for adoption",
		"resource", obj.GetName(),
		"discoveredCount", len(discovered),
	)

	if len(discovered) == 0 {
		m.logger.Debug(ctx, "No resources found to adopt",
			"resource", obj.GetName(),
		)
		return nil
	}

	// Validate ownership
	validateOwnership := true
	if validateOwnershipBool, ok := selectorMap["validateOwnership"].(bool); ok {
		validateOwnership = validateOwnershipBool
	}

	m.logger.Debug(ctx, "Validating ownership before adoption",
		"resource", obj.GetName(),
		"validateOwnership", validateOwnership,
	)

	adopter := resources.NewAdopter(m.dynamicClient)

	if validateOwnership {
		if err := adopter.ValidateNoConflictingOwners(discovered); err != nil {
			m.logger.Error(ctx, err, "Ownership validation failed",
				"resource", obj.GetName(),
			)
			return fmt.Errorf("ownership validation failed: %w", err)
		}
	}

	// Adopt resources
	ownerGVK := m.resourceGVR.GroupVersion().WithKind(m.config.ResourceType)
	m.logger.Debug(ctx, "Adopting resources",
		"resource", obj.GetName(),
		"ownerGVK", ownerGVK.String(),
		"count", len(discovered),
	)

	if err := adopter.AdoptResources(ctx, obj, ownerGVK, discovered, validateOwnership); err != nil {
		m.logger.Error(ctx, err, "Failed to adopt resources",
			"resource", obj.GetName(),
		)
		return fmt.Errorf("failed to adopt resources: %w", err)
	}

	m.logger.Debug(ctx, "Successfully adopted resources",
		"resource", obj.GetName(),
		"count", len(discovered),
	)
	return nil
}

// parseZarfResourceSelector converts a map to ZarfResourceSelector
func (m *GenericJobMonitor[T]) parseZarfResourceSelector(selectorMap map[string]interface{}) *zarfv1alpha3.ResourceSelector {
	selector := &zarfv1alpha3.ResourceSelector{}

	if matchLabels, ok := selectorMap["matchLabels"].(map[string]interface{}); ok {
		selector.MatchLabels = make(map[string]string)
		for k, v := range matchLabels {
			if vStr, ok := v.(string); ok {
				selector.MatchLabels[k] = vStr
			}
		}
	}

	if matchNames, ok := selectorMap["matchNames"].([]interface{}); ok {
		for _, name := range matchNames {
			if nameStr, ok := name.(string); ok {
				selector.MatchNames = append(selector.MatchNames, nameStr)
			}
		}
	}

	if namespaces, ok := selectorMap["namespaces"].([]interface{}); ok {
		for _, ns := range namespaces {
			if nsStr, ok := ns.(string); ok {
				selector.Namespaces = append(selector.Namespaces, nsStr)
			}
		}
	}

	if validateOwnership, ok := selectorMap["validateOwnership"].(bool); ok {
		selector.ValidateOwnership = &validateOwnership
	}

	return selector
}

// parseUDSResourceSelector converts a map to UDSResourceSelector
func (m *GenericJobMonitor[T]) parseUDSResourceSelector(selectorMap map[string]interface{}) *udsv1alpha3.ResourceSelector {
	selector := &udsv1alpha3.ResourceSelector{}

	if matchLabels, ok := selectorMap["matchLabels"].(map[string]interface{}); ok {
		selector.MatchLabels = make(map[string]string)
		for k, v := range matchLabels {
			if vStr, ok := v.(string); ok {
				selector.MatchLabels[k] = vStr
			}
		}
	}

	if matchNames, ok := selectorMap["matchNames"].([]interface{}); ok {
		for _, name := range matchNames {
			if nameStr, ok := name.(string); ok {
				selector.MatchNames = append(selector.MatchNames, nameStr)
			}
		}
	}

	if namespaces, ok := selectorMap["namespaces"].([]interface{}); ok {
		for _, ns := range namespaces {
			if nsStr, ok := ns.(string); ok {
				selector.Namespaces = append(selector.Namespaces, nsStr)
			}
		}
	}

	if validateOwnership, ok := selectorMap["validateOwnership"].(bool); ok {
		selector.ValidateOwnership = &validateOwnership
	}

	return selector
}
