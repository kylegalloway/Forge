// Package controller implements the Forge controller for UDSBundleJob resources.
package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	udsactions "github.com/kylegalloway/forge/pkg/actions/uds"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/policy"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// UDSController watches UDSBundleJob resources and executes actions
type UDSController struct {
	kubeClient     kubernetes.Interface
	dynamicClient  dynamic.Interface
	namespace      string
	metrics        *telemetry.Metrics
	tracer         *telemetry.Tracer
	policyEngine   *policy.Engine
	createHandler  *udsactions.CreateHandler
	publishHandler *udsactions.PublishHandler
	deployHandler  *udsactions.DeployHandler
	healthy        bool
	ready          bool
}

// NewUDSController creates a new UDS bundle controller
func NewUDSController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
) *UDSController {
	// Initialize policy engine
	policyEngine := policy.NewEngine(kubeClient)

	// Initialize UDS action handlers
	createHandler := udsactions.NewCreateHandler(kubeClient, metrics, tracer)
	publishHandler := udsactions.NewPublishHandler(kubeClient, metrics, tracer)
	deployHandler := udsactions.NewDeployHandler(kubeClient, metrics, tracer)

	return &UDSController{
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
		namespace:      namespace,
		metrics:        metrics,
		tracer:         tracer,
		policyEngine:   policyEngine,
		createHandler:  createHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
		healthy:        true,
		ready:          false,
	}
}

// Run starts the UDS controller's main reconciliation loop
func (ctrl *UDSController) Run(ctx context.Context) error {
	klog.Info("Starting Forge UDS Bundle controller")

	// Start Job monitoring in background
	go ctrl.startJobMonitoring(ctx)

	// Watch UDSBundleJob resources
	for {
		select {
		case <-ctx.Done():
			klog.Info("Stopping UDS controller: context canceled")
			return ctx.Err()
		default:
			if err := ctrl.watchUDSBundleJobs(ctx); err != nil {
				klog.ErrorS(err, "Watch error, restarting in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// watchUDSBundleJobs establishes a watch on UDSBundleJob resources
func (ctrl *UDSController) watchUDSBundleJobs(ctx context.Context) error {
	klog.V(2).Info("Starting watch on UDSBundleJob resources")

	// Set up resource interface
	var resourceInterface dynamic.ResourceInterface
	if ctrl.namespace == "" {
		resourceInterface = ctrl.dynamicClient.Resource(constants.UDSBundleJobGVR)
	} else {
		resourceInterface = ctrl.dynamicClient.Resource(constants.UDSBundleJobGVR).Namespace(ctrl.namespace)
	}

	// Start watch
	watcher, err := resourceInterface.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}
	defer watcher.Stop()

	ctrl.ready = true
	klog.Info("UDS controller is ready")

	// Process watch events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				klog.V(2).Info("Watch channel closed, restarting")
				return nil
			}
			ctrl.handleWatchEvent(ctx, event)
		}
	}
}

// handleWatchEvent processes a watch event
func (ctrl *UDSController) handleWatchEvent(ctx context.Context, event watch.Event) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		ctrl.metrics.RecordReconcileDuration(ctx, duration)
	}()

	switch event.Type {
	case watch.Added, watch.Modified:
		unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			klog.Error("Failed to cast event object to Unstructured")
			return
		}

		// Convert unstructured to UDSBundleJob
		bundle := &udsv1alpha2.UDSBundleJob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, bundle); err != nil {
			klog.ErrorS(err, "Failed to convert to UDSBundleJob")
			return
		}

		klog.V(2).InfoS("Processing UDSBundleJob event", "type", event.Type, "name", bundle.Name, "namespace", bundle.Namespace)

		if event.Type == watch.Added {
			ctrl.metrics.RecordUDSBundleJobCreated(ctx, bundle.Namespace)
		}

		// Reconcile the bundle
		if err := ctrl.reconcile(ctx, bundle); err != nil {
			klog.ErrorS(err, "Reconciliation failed", "name", bundle.Name, "namespace", bundle.Namespace)
			ctrl.metrics.RecordReconcileError(ctx, bundle.Namespace)
		}

	case watch.Deleted:
		unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			klog.Error("Failed to cast event object to Unstructured")
			return
		}

		bundle := &udsv1alpha2.UDSBundleJob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, bundle); err != nil {
			klog.ErrorS(err, "Failed to convert to UDSBundleJob")
			return
		}

		klog.V(2).InfoS("UDSBundleJob deleted", "name", bundle.Name, "namespace", bundle.Namespace)
		ctrl.metrics.RecordUDSBundleJobDeleted(ctx, bundle.Namespace)

	case watch.Error:
		klog.ErrorS(nil, "Watch error event", "object", event.Object)
	}
}

// reconcile handles the reconciliation logic for a UDSBundleJob.
//
// Control Flow:
// 1. Early exit: Skip if job already reached terminal state (Completed/Failed)
// 2. Policy validation: Checks ServiceAccount annotations against requested operations
// 3. Action dispatch: Routes to appropriate handler based on action type
// 4. Status update: Records operation state in status subresource
//
// Compound Actions (CreatePublish, CreateDeploy, etc.):
// - Reconciliation only starts the FIRST action in the chain
// - Job monitor (see uds_job_monitor.go) watches for Job completion
// - Monitor updates UDSBundleJob status with next action in chain
// - Status update triggers re-reconciliation, which executes next action
// - Chain continues until all actions complete or one fails
//
// State Transitions:
// - "" (empty) → "Running" (first action started)
// - "Running" → "Running" (subsequent actions in chain, managed by job monitor)
// - "Running" → "Completed" (final action succeeds)
// - any → "Failed" (any action fails or policy violation)
//
// Idempotency:
// - executeCreate/Publish/Deploy check if action already started via status fields
// - Re-reconciliation of same action is safe - handler skips if already running
func (ctrl *UDSController) reconcile(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	klog.V(2).InfoS("Reconciling UDSBundleJob", "name", bundle.Name, "namespace", bundle.Namespace, "action", bundle.Spec.Action)

	// Early Exit: Skip terminal states to avoid re-processing completed/failed jobs
	// Terminal states set by: action handlers (on completion) or status updates (on failure)
	if bundle.Status.Phase == "Completed" || bundle.Status.Phase == "Failed" {
		klog.V(2).InfoS("Skipping completed/failed bundle", "name", bundle.Name, "phase", bundle.Status.Phase)
		return nil
	}

	// Policy Validation: Verify ServiceAccount has required permissions via annotations
	// Example annotations: forge.dev/allowed-actions, forge.dev/allowed-source-repos
	// Validation happens BEFORE any Job creation to fail fast on permission issues
	if err := ctrl.validatePolicy(ctx, bundle); err != nil {
		klog.ErrorS(err, "Policy validation failed", "name", bundle.Name)
		return ctrl.updateStatus(ctx, bundle, "Failed", fmt.Sprintf("Policy validation failed: %v", err))
	}

	// Action Dispatch: Route to appropriate handler and start first action in chain
	// Handlers create Kubernetes Jobs that execute UDS CLI commands
	// Job monitor handles chaining for compound actions (see dispatchAction for details)
	if err := ctrl.dispatchAction(ctx, bundle); err != nil {
		klog.ErrorS(err, "Action dispatch failed", "name", bundle.Name, "action", bundle.Spec.Action)
		return ctrl.updateStatus(ctx, bundle, "Failed", fmt.Sprintf("Action failed: %v", err))
	}

	return nil
}

// validatePolicy validates the bundle against ServiceAccount policies
func (ctrl *UDSController) validatePolicy(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	return ctrl.policyEngine.ValidateUDSBundle(ctx, bundle)
}

// dispatchAction routes the bundle to the appropriate action handler.
//
// Single Actions (Create, Publish, Deploy):
// - Execute the requested action and return
// - No follow-up actions, job completes when action finishes
// - Status update marks job as Completed or Failed
//
// Compound Actions (CreatePublish, CreateDeploy, PublishDeploy, CreatePublishDeploy):
// - Reconciliation executes ONLY the first action
// - Job monitor (uds_job_monitor.go) detects Job completion
// - Monitor updates UDSBundleJob status fields (createStatus, publishStatus, deployStatus)
// - Status update triggers re-reconciliation via Kubernetes watch
// - Next action in chain starts based on updated status
// - Chain continues until all actions complete or one fails
//
// Example CreatePublishDeploy flow:
// 1. dispatchAction → executeCreate (creates uds create Job)
// 2. Job monitor detects Job completion → updates status.createStatus
// 3. Watch event triggers reconcile again
// 4. dispatchAction → executePublish (creates uds publish Job, reads bundle from shared storage)
// 5. Job monitor detects Job completion → updates status.publishStatus
// 6. Watch event triggers reconcile again
// 7. dispatchAction → executeDeploy (creates uds deploy Job, reads bundle from shared storage)
// 8. Job monitor detects Job completion → marks phase="Completed"
func (ctrl *UDSController) dispatchAction(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	action := bundle.Spec.Action

	switch action {
	case udsv1alpha2.ActionCreate:
		// Single Create: Build UDS bundle from uds-bundle.yaml definition
		// Output: Bundle tarball written to shared workspace
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha2.ActionPublish:
		// Single Publish: Upload bundle artifact to OCI registry or S3
		// Requires: Bundle pre-staged at /workspace/ (from source.type: OCI, S3, or Local)
		return ctrl.executePublish(ctx, bundle)

	case udsv1alpha2.ActionDeploy:
		// Single Deploy: Install UDS bundle into target Kubernetes cluster
		// Requires: Bundle pre-staged at /workspace/ (from source.type: OCI, S3, or Local)
		return ctrl.executeDeploy(ctx, bundle)

	case udsv1alpha2.ActionCreatePublish:
		// Compound: Create → Publish
		// Reconcile: Starts Create (writes bundle to workspace)
		// Monitor: Detects Create complete → updates createStatus → triggers Publish
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha2.ActionCreateDeploy:
		// Compound: Create → Deploy
		// Reconcile: Starts Create (writes bundle to workspace)
		// Monitor: Detects Create complete → updates createStatus → triggers Deploy
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha2.ActionPublishDeploy:
		// Compound: Publish → Deploy
		// Reconcile: Starts Publish (bundle from source → registry/S3)
		// Monitor: Detects Publish complete → updates publishStatus → triggers Deploy
		return ctrl.executePublish(ctx, bundle)

	case udsv1alpha2.ActionCreatePublishDeploy:
		// Compound: Create → Publish → Deploy
		// Reconcile: Starts Create (builds bundle)
		// Monitor chain: Create done → Publish (upload) → Deploy (install to cluster)
		return ctrl.executeCreate(ctx, bundle)

	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

// executeCreate executes the Create action
func (ctrl *UDSController) executeCreate(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	// Skip if create already started
	if bundle.Status.CreateStatus != nil && bundle.Status.CreateStatus.State != "" {
		klog.V(2).InfoS("Create already started", "name", bundle.Name, "state", bundle.Status.CreateStatus.State)
		return nil
	}

	klog.InfoS("Executing Create action", "name", bundle.Name, "namespace", bundle.Namespace)

	result, err := ctrl.createHandler.Execute(ctx, bundle)
	if err != nil {
		return err
	}

	// Update status with create operation details
	bundle.Status.CreateStatus = &udsv1alpha2.OperationStatus{
		State:     result.Phase,
		Message:   result.Message,
		StartTime: &result.StartTime,
		JobName:   result.JobName,
	}
	bundle.Status.Phase = "Running"
	bundle.Status.Message = "Create action started"

	return ctrl.updateStatus(ctx, bundle, bundle.Status.Phase, bundle.Status.Message)
}

// executePublish executes the Publish action
func (ctrl *UDSController) executePublish(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	// Skip if publish already started
	if bundle.Status.PublishStatus != nil && bundle.Status.PublishStatus.State != "" {
		klog.V(2).InfoS("Publish already started", "name", bundle.Name, "state", bundle.Status.PublishStatus.State)
		return nil
	}

	klog.InfoS("Executing Publish action", "name", bundle.Name, "namespace", bundle.Namespace)

	result, err := ctrl.publishHandler.Execute(ctx, bundle)
	if err != nil {
		return err
	}

	// Update status with publish operation details
	bundle.Status.PublishStatus = &udsv1alpha2.OperationStatus{
		State:     result.Phase,
		Message:   result.Message,
		StartTime: &result.StartTime,
		JobName:   result.JobName,
	}
	bundle.Status.Phase = "Running"
	bundle.Status.Message = "Publish action started"

	return ctrl.updateStatus(ctx, bundle, bundle.Status.Phase, bundle.Status.Message)
}

// executeDeploy executes the Deploy action
func (ctrl *UDSController) executeDeploy(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	// Skip if deploy already started
	if bundle.Status.DeployStatus != nil && bundle.Status.DeployStatus.State != "" {
		klog.V(2).InfoS("Deploy already started", "name", bundle.Name, "state", bundle.Status.DeployStatus.State)
		return nil
	}

	klog.InfoS("Executing Deploy action", "name", bundle.Name, "namespace", bundle.Namespace)

	result, err := ctrl.deployHandler.Execute(ctx, bundle)
	if err != nil {
		return err
	}

	// Update status with deploy operation details
	bundle.Status.DeployStatus = &udsv1alpha2.OperationStatus{
		State:     result.Phase,
		Message:   result.Message,
		StartTime: &result.StartTime,
		JobName:   result.JobName,
	}
	bundle.Status.Phase = "Running"
	bundle.Status.Message = "Deploy action started"

	return ctrl.updateStatus(ctx, bundle, bundle.Status.Phase, bundle.Status.Message)
}

// updateStatus updates the UDSBundleJob status
func (ctrl *UDSController) updateStatus(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, phase, message string) error {
	bundle.Status.Phase = phase
	bundle.Status.Message = message
	bundle.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}
	bundle.Status.ObservedGeneration = bundle.Generation

	// Convert to unstructured for update
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bundle)
	if err != nil {
		return fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	unstructuredBundle := &unstructured.Unstructured{Object: unstructuredObj}

	// Update status subresource
	_, err = ctrl.dynamicClient.Resource(constants.UDSBundleJobGVR).
		Namespace(bundle.Namespace).
		UpdateStatus(ctx, unstructuredBundle, metav1.UpdateOptions{})

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	klog.V(2).InfoS("Status updated", "name", bundle.Name, "phase", phase, "message", message)
	return nil
}

// Healthy returns the controller's health status
func (ctrl *UDSController) Healthy() bool {
	return ctrl.healthy
}

// Ready returns whether the controller is ready to serve traffic
func (ctrl *UDSController) Ready() bool {
	return ctrl.ready
}

// HealthzHandler returns an HTTP handler for health checks
func (ctrl *UDSController) HealthzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if ctrl.healthy {
			responseWriter.WriteHeader(http.StatusOK)
			if _, err := responseWriter.Write([]byte("ok")); err != nil {
				klog.ErrorS(err, "Failed to write health response")
			}
		} else {
			responseWriter.WriteHeader(http.StatusServiceUnavailable)
			if _, err := responseWriter.Write([]byte("unhealthy")); err != nil {
				klog.ErrorS(err, "Failed to write unhealthy response")
			}
		}
	}
}

// ReadyzHandler returns an HTTP handler for readiness checks
func (ctrl *UDSController) ReadyzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if ctrl.ready {
			responseWriter.WriteHeader(http.StatusOK)
			if _, err := responseWriter.Write([]byte("ready")); err != nil {
				klog.ErrorS(err, "Failed to write ready response")
			}
		} else {
			responseWriter.WriteHeader(http.StatusServiceUnavailable)
			if _, err := responseWriter.Write([]byte("not ready")); err != nil {
				klog.ErrorS(err, "Failed to write not ready response")
			}
		}
	}
}
