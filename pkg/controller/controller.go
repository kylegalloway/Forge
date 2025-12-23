// Package controller implements the Forge controller for ZarfPackageJob resources.
//
// The controller watches for ZarfPackageJob resources and executes the specified
// actions (Build, Publish, Deploy) using dedicated handler packages.
package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/policy"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// Controller watches ZarfPackageJob resources and executes actions
type Controller struct {
	kubeClient     kubernetes.Interface
	dynamicClient  dynamic.Interface
	namespace      string
	metrics        *telemetry.Metrics
	tracer         *telemetry.Tracer
	policyEngine   *policy.Engine
	buildHandler   *actions.BuildHandler
	publishHandler *actions.PublishHandler
	deployHandler  *actions.DeployHandler
	healthy        bool
	ready          bool
}

// NewController creates a new Forge controller
func NewController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
) *Controller {
	// Initialize policy engine
	policyEngine := policy.NewEngine(kubeClient)

	// Initialize action handlers
	buildHandler := actions.NewBuildHandler(kubeClient, metrics, tracer)
	publishHandler := actions.NewPublishHandler(kubeClient, metrics, tracer)
	deployHandler := actions.NewDeployHandler(kubeClient, metrics, tracer)

	return &Controller{
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
		namespace:      namespace,
		metrics:        metrics,
		tracer:         tracer,
		policyEngine:   policyEngine,
		buildHandler:   buildHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
		healthy:        true,
		ready:          false,
	}
}

// Run starts the controller's main reconciliation loop
func (controller *Controller) Run(ctx context.Context) error {
	klog.Info("Starting Forge controller")

	// Start Job monitoring in background
	go controller.startJobMonitoring(ctx)

	// Watch ZarfPackageJob resources
	watcher, err := controller.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(controller.namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Stop()

	controller.ready = true
	klog.Info("Forge controller is ready")

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			klog.Info("Context canceled, stopping controller")
			return nil

		case event, ok := <-watcher.ResultChan():
			if !ok {
				klog.Warning("Watch channel closed, restarting watcher")
				// Recreate watcher
				var watchErr error
				watcher, watchErr = controller.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(controller.namespace).Watch(ctx, metav1.ListOptions{})
				if watchErr != nil {
					return fmt.Errorf("failed to recreate watcher: %w", watchErr)
				}
				continue
			}

			if handleErr := controller.handleEvent(ctx, event); handleErr != nil {
				klog.ErrorS(handleErr, "Error handling event", "type", event.Type)
				// Don't return error, continue processing
			}
		}
	}
}

// handleEvent processes a single watch event
func (controller *Controller) handleEvent(ctx context.Context, event watch.Event) error {
	switch event.Type {
	case watch.Added:
		return controller.handleObject(ctx, event.Object)
	case watch.Modified:
		return controller.handleObject(ctx, event.Object)
	case watch.Deleted:
		// Cleanup handled by owner references
		obj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type in deleted event")
		}
		// Record deletion in metrics (decrement active counter)
		controller.metrics.RecordZarfPackageJobDeleted(ctx, obj.GetNamespace())
		klog.InfoS("ZarfPackageJob deleted", "name", obj.GetName(), "namespace", obj.GetNamespace())
		return nil
	case watch.Error:
		return fmt.Errorf("watch error: %v", event.Object)
	default:
		klog.V(4).InfoS("Ignoring event type", "type", event.Type)
		return nil
	}
}

// handleObject dispatches to the appropriate handler based on kind
func (controller *Controller) handleObject(ctx context.Context, obj interface{}) error {
	unstrObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}

	gvk := unstrObj.GroupVersionKind()
	if gvk.Group == constants.APIGroup && gvk.Kind == "ZarfPackageJob" {
		return controller.handleZarfPackageJob(ctx, unstrObj)
	}

	return fmt.Errorf("unsupported kind: %s", gvk.Kind)
}

// handleZarfPackage reconciles a ZarfPackageJob resource
func (controller *Controller) handleZarfPackageJob(ctx context.Context, unstrObj *unstructured.Unstructured) error {
	name := unstrObj.GetName()
	namespace := unstrObj.GetNamespace()

	klog.InfoS("Reconciling ZarfPackageJob", "name", name, "namespace", namespace)

	// Convert unstructured to typed ZarfPackageJob
	pkg := &zarfv1alpha1.ZarfPackageJob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, pkg); err != nil {
		klog.ErrorS(err, "Failed to convert unstructured to ZarfPackageJob", "name", name, "namespace", namespace)
		controller.metrics.RecordReconcileError(ctx, "conversion_error")
		return controller.updateStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Invalid ZarfPackageJob: %v", err), nil)
	}

	// Record ZarfPackageJob creation if this is the first reconciliation
	if pkg.Status.Phase == "" {
		controller.metrics.RecordZarfPackageJobCreated(ctx, namespace)
	}

	return controller.reconcilePackage(ctx, unstrObj, pkg)
}

// reconcilePackage performs the actual reconciliation logic
func (controller *Controller) reconcilePackage(ctx context.Context, unstrObj *unstructured.Unstructured, pkg *zarfv1alpha1.ZarfPackageJob) error {
	startTime := time.Now()
	name := pkg.Name
	namespace := pkg.Namespace

	klog.InfoS("Processing ZarfPackageJob action", "name", name, "namespace", namespace, "action", pkg.Spec.Action)

	// Validate policy
	if err := controller.policyEngine.Validate(ctx, pkg); err != nil {
		klog.ErrorS(err, "Policy validation failed", "name", name, "namespace", namespace)
		controller.metrics.RecordReconcileError(ctx, "policy_violation")
		return controller.updateStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Policy violation: %v", err), nil)
	}

	// Create shared artifact PVC if this is a multi-action job
	var artifactPVCName string
	if isMultiActionZarfJob(pkg.Spec.Action) {
		ownerRef := *metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackageJob"))
		pvc, err := ensureArtifactPVC(ctx, controller.kubeClient, pkg.Name, pkg.Namespace, ownerRef)
		if err != nil {
			klog.ErrorS(err, "Failed to create artifact PVC", "name", name, "namespace", namespace)
			controller.metrics.RecordReconcileError(ctx, "pvc_creation_failed")
			return controller.updateStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Failed to create artifact storage: %v", err), nil)
		}
		artifactPVCName = pvc.Name
		klog.InfoS("Using shared artifact PVC", "name", name, "pvc", artifactPVCName)
	}

	// Dispatch to appropriate action handler
	var result *actions.ActionResult
	var err error

	switch pkg.Spec.Action {
	case zarfv1alpha1.ActionBuild:
		result, err = controller.buildHandler.Execute(ctx, pkg, artifactPVCName)

	case zarfv1alpha1.ActionPublish:
		// Standalone publish: artifact must be pre-staged at /workspace/package.tar.zst
		result, err = controller.publishHandler.Execute(ctx, pkg, "/workspace/package.tar.zst", artifactPVCName)

	case zarfv1alpha1.ActionDeploy:
		// Standalone deploy: artifact must be pre-staged at /workspace/package.tar.zst
		result, err = controller.deployHandler.Execute(ctx, pkg, "/workspace/package.tar.zst", artifactPVCName)

	case zarfv1alpha1.ActionBuildPublish:
		// Execute build first, job monitor will trigger publish when build completes
		result, err = controller.buildHandler.Execute(ctx, pkg, artifactPVCName)

	case zarfv1alpha1.ActionBuildDeploy:
		// Execute build first, job monitor will trigger deploy when build completes
		result, err = controller.buildHandler.Execute(ctx, pkg, artifactPVCName)

	case zarfv1alpha1.ActionPublishDeploy:
		// Execute publish first, job monitor will trigger deploy when publish completes
		result, err = controller.publishHandler.Execute(ctx, pkg, "/workspace/package.tar.zst", artifactPVCName)

	case zarfv1alpha1.ActionBuildPublishDeploy:
		// Execute build first, job monitor will chain publish → deploy
		result, err = controller.buildHandler.Execute(ctx, pkg, artifactPVCName)

	default:
		err = fmt.Errorf("action %s not yet implemented", pkg.Spec.Action)
	}

	// Update status based on result
	if err != nil {
		klog.ErrorS(err, "Action failed", "name", name, "namespace", namespace, "action", pkg.Spec.Action)
		controller.metrics.RecordReconcileError(ctx, "action_failed")
		// Record reconcile duration even on failure
		duration := time.Since(startTime)
		controller.metrics.RecordReconcileDuration(ctx, duration.Seconds())
		return controller.updateStatus(ctx, unstrObj, "Failed", err.Error(), nil)
	}

	if result != nil {
		opStatus := map[string]interface{}{
			"buildStatus": map[string]interface{}{
				"state":     result.Phase,
				"message":   result.Message,
				"startTime": result.StartTime.Format(time.RFC3339),
			},
		}
		if err := controller.updateStatus(ctx, unstrObj, result.Phase, result.Message, opStatus); err != nil {
			klog.ErrorS(err, "Failed to update status", "name", name, "namespace", namespace)
			controller.metrics.RecordReconcileError(ctx, "status_update_failed")
		}
	}

	duration := time.Since(startTime)
	controller.metrics.RecordReconcileDuration(ctx, duration.Seconds())
	klog.InfoS("Reconciliation complete", "name", name, "namespace", namespace, "duration", duration)

	return nil
}

// updateStatus updates the status of a ZarfPackageJob resource
func (controller *Controller) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string, operationStatus map[string]interface{}) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	// Build status object
	status := map[string]interface{}{
		"phase":              phase,
		"message":            message,
		"lastUpdateTime":     metav1.Now().Format(time.RFC3339),
		"observedGeneration": obj.GetGeneration(),
	}

	// Add operation-specific status if provided
	for key, value := range operationStatus {
		status[key] = value
	}

	// Update status subresource
	obj.Object["status"] = status

	_, err := controller.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).InfoS("ZarfPackageJob not found during status update", "name", name, "namespace", namespace)
			return nil
		}
		return fmt.Errorf("failed to update status: %w", err)
	}

	klog.V(4).InfoS("Status updated", "name", name, "namespace", namespace, "phase", phase)
	return nil
}

// Healthy returns the controller's health status
func (controller *Controller) Healthy() bool {
	return controller.healthy
}

// Ready returns whether the controller is ready to serve traffic
func (controller *Controller) Ready() bool {
	return controller.ready
}

// HealthzHandler returns an HTTP handler for health checks
func (controller *Controller) HealthzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if controller.healthy {
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
func (controller *Controller) ReadyzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if controller.ready {
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
