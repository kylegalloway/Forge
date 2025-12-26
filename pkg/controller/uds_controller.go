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
	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
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
		bundle := &udsv1alpha1.UDSBundleJob{}
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

		bundle := &udsv1alpha1.UDSBundleJob{}
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

// reconcile handles the reconciliation logic for a UDSBundleJob
func (ctrl *UDSController) reconcile(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
	// Deprecation warning for v1alpha1 API
	klog.InfoS("DEPRECATION WARNING: v1alpha1 UDSBundleJob API is deprecated and will be removed in Forge v0.10.0. Please migrate to v1alpha2 UDSPackageJob. See docs/operations/V1ALPHA2_MIGRATION.md for details.", "name", bundle.Name, "namespace", bundle.Namespace)

	klog.V(2).InfoS("Reconciling UDSBundleJob", "name", bundle.Name, "namespace", bundle.Namespace, "action", bundle.Spec.Action)

	// Skip if already completed
	if bundle.Status.Phase == "Completed" || bundle.Status.Phase == "Failed" {
		klog.V(2).InfoS("Skipping completed/failed bundle", "name", bundle.Name, "phase", bundle.Status.Phase)
		return nil
	}

	// Validate policy (ServiceAccount annotations)
	if err := ctrl.validatePolicy(ctx, bundle); err != nil {
		klog.ErrorS(err, "Policy validation failed", "name", bundle.Name)
		return ctrl.updateStatus(ctx, bundle, "Failed", fmt.Sprintf("Policy validation failed: %v", err))
	}

	// Dispatch to appropriate handler based on action
	if err := ctrl.dispatchAction(ctx, bundle); err != nil {
		klog.ErrorS(err, "Action dispatch failed", "name", bundle.Name, "action", bundle.Spec.Action)
		return ctrl.updateStatus(ctx, bundle, "Failed", fmt.Sprintf("Action failed: %v", err))
	}

	return nil
}

// validatePolicy validates the bundle against ServiceAccount policies
//
//nolint:staticcheck // SA1019: UDSBundleJob v1alpha1 must be supported until v0.10.0
func (ctrl *UDSController) validatePolicy(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
	return ctrl.policyEngine.ValidateUDSBundle(ctx, bundle)
}

// dispatchAction routes the bundle to the appropriate action handler
func (ctrl *UDSController) dispatchAction(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
	action := bundle.Spec.Action

	// Handle compound actions by dispatching the first action
	// Job monitoring will handle chaining
	switch action {
	case udsv1alpha1.BundleActionCreate:
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha1.BundleActionPublish:
		return ctrl.executePublish(ctx, bundle)

	case udsv1alpha1.BundleActionDeploy:
		return ctrl.executeDeploy(ctx, bundle)

	case udsv1alpha1.BundleActionCreatePublish:
		// Start with Create, monitoring will trigger Publish
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha1.BundleActionCreateDeploy:
		// Start with Create, monitoring will trigger Deploy
		return ctrl.executeCreate(ctx, bundle)

	case udsv1alpha1.BundleActionPublishDeploy:
		// Start with Publish, monitoring will trigger Deploy
		return ctrl.executePublish(ctx, bundle)

	case udsv1alpha1.BundleActionCreatePublishDeploy:
		// Start with Create, monitoring will chain through all
		return ctrl.executeCreate(ctx, bundle)

	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

// executeCreate executes the Create action
func (ctrl *UDSController) executeCreate(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
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
	bundle.Status.CreateStatus = &udsv1alpha1.OperationStatus{
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
func (ctrl *UDSController) executePublish(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
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
	bundle.Status.PublishStatus = &udsv1alpha1.OperationStatus{
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
func (ctrl *UDSController) executeDeploy(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) error {
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
	bundle.Status.DeployStatus = &udsv1alpha1.OperationStatus{
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
func (ctrl *UDSController) updateStatus(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob, phase, message string) error {
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
