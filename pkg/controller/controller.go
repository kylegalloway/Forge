// Package controller implements the Forge controller for ZarfPackage resources.
//
// The controller watches for ZarfPackage resources and executes the specified
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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/policy"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// ZarfPackageGroup is the API group for ZarfPackage resources
	ZarfPackageGroup = "zarf.dev"
	// ZarfPackageVersion is the API version
	ZarfPackageVersion = "v1alpha1"
	// ZarfPackageResource is the resource name
	ZarfPackageResource = "zarfpackages"
)

var (
	// ZarfPackageGVR is the GroupVersionResource for ZarfPackage
	ZarfPackageGVR = schema.GroupVersionResource{
		Group:    ZarfPackageGroup,
		Version:  ZarfPackageVersion,
		Resource: ZarfPackageResource,
	}

	// UDSBundleGVR is the GroupVersionResource for UDSBundle
	UDSBundleGVR = schema.GroupVersionResource{
		Group:    udsv1alpha1.GroupName,
		Version:  udsv1alpha1.Version,
		Resource: "udsbundles",
	}
)

// Controller watches ZarfPackage resources and executes actions
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
func (c *Controller) Run(ctx context.Context) error {
	klog.Info("Starting Forge controller")

	// Start Job monitoring in background
	go c.startJobMonitoring(ctx)

	// Watch ZarfPackage resources
	watcher, err := c.dynamicClient.Resource(ZarfPackageGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Stop()

	// Watch UDSBundle resources
	udsWatcher, err := c.dynamicClient.Resource(UDSBundleGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create uds watcher: %w", err)
	}
	defer udsWatcher.Stop()

	c.ready = true
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
				watcher, watchErr = c.dynamicClient.Resource(ZarfPackageGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
				if watchErr != nil {
					return fmt.Errorf("failed to recreate watcher: %w", watchErr)
				}
				continue
			}

			if handleErr := c.handleEvent(ctx, event); handleErr != nil {
				klog.ErrorS(handleErr, "Error handling event", "type", event.Type)
				// Don't return error, continue processing
			}

		case event, ok := <-udsWatcher.ResultChan():
			if !ok {
				klog.Warning("UDS Watch channel closed, restarting watcher")
				// Recreate watcher
				udsWatcher, err = c.dynamicClient.Resource(UDSBundleGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to recreate uds watcher: %w", err)
				}
				continue
			}

			if err := c.handleEvent(ctx, event); err != nil {
				klog.ErrorS(err, "Error handling UDS event", "type", event.Type)
			}
		}
	}
}

// handleEvent processes a single watch event
func (c *Controller) handleEvent(ctx context.Context, event watch.Event) error {
	switch event.Type {
	case watch.Added:
		return c.handleObject(ctx, event.Object)
	case watch.Modified:
		return c.handleObject(ctx, event.Object)
	case watch.Deleted:
		// Cleanup handled by owner references
		obj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type in deleted event")
		}
		klog.InfoS("ZarfPackage deleted", "name", obj.GetName(), "namespace", obj.GetNamespace())
		return nil
	case watch.Error:
		return fmt.Errorf("watch error: %v", event.Object)
	default:
		klog.V(4).InfoS("Ignoring event type", "type", event.Type)
		return nil
	}
}

// handleObject dispatches to the appropriate handler based on kind
func (c *Controller) handleObject(ctx context.Context, obj interface{}) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}

	gvk := u.GroupVersionKind()
	if gvk.Group == ZarfPackageGroup && gvk.Kind == "ZarfPackage" {
		return c.handleZarfPackage(ctx, u)
	} else if gvk.Group == udsv1alpha1.GroupName && gvk.Kind == "UDSBundle" {
		return c.handleUDSBundle(ctx, u)
	}

	return fmt.Errorf("unsupported kind: %s", gvk.Kind)
}

// handleZarfPackage reconciles a ZarfPackage resource
func (c *Controller) handleZarfPackage(ctx context.Context, u *unstructured.Unstructured) error {
	name := u.GetName()
	namespace := u.GetNamespace()

	klog.InfoS("Reconciling ZarfPackage", "name", name, "namespace", namespace)

	// Convert unstructured to typed ZarfPackage
	pkg := &zarfv1alpha1.ZarfPackage{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pkg); err != nil {
		klog.ErrorS(err, "Failed to convert unstructured to ZarfPackage", "name", name, "namespace", namespace)
		return c.updateStatus(ctx, u, "Failed", fmt.Sprintf("Invalid ZarfPackage: %v", err), nil)
	}

	return c.reconcilePackage(ctx, u, pkg)
}

// handleUDSBundle reconciles a UDSBundle resource
func (c *Controller) handleUDSBundle(ctx context.Context, u *unstructured.Unstructured) error {
	name := u.GetName()
	namespace := u.GetNamespace()

	klog.InfoS("Reconciling UDSBundle", "name", name, "namespace", namespace)

	// Convert unstructured to typed UDSBundle
	bundle := &udsv1alpha1.UDSBundle{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, bundle); err != nil {
		klog.ErrorS(err, "Failed to convert unstructured to UDSBundle", "name", name, "namespace", namespace)
		return c.updateStatus(ctx, u, "Failed", fmt.Sprintf("Invalid UDSBundle: %v", err), nil)
	}

	// Convert to ZarfPackage for processing
	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: bundle.ObjectMeta,
		Spec: zarfv1alpha1.ZarfPackageSpec{
			ServiceAccountName: bundle.Spec.ServiceAccountName,
			Action:             bundle.Spec.Action,
			Source:             bundle.Spec.Source,
			Publish:            bundle.Spec.Publish,
			Deploy:             bundle.Spec.Deploy,
			RBACPolicy:         bundle.Spec.RBACPolicy,
		},
	}

	return c.reconcilePackage(ctx, u, pkg)
}

// reconcilePackage performs the actual reconciliation logic
func (c *Controller) reconcilePackage(ctx context.Context, u *unstructured.Unstructured, pkg *zarfv1alpha1.ZarfPackage) error {
	startTime := time.Now()
	name := pkg.Name
	namespace := pkg.Namespace

	klog.InfoS("Processing ZarfPackage action", "name", name, "namespace", namespace, "action", pkg.Spec.Action)

	// Validate policy
	if err := c.policyEngine.Validate(ctx, pkg); err != nil {
		klog.ErrorS(err, "Policy validation failed", "name", name, "namespace", namespace)
		return c.updateStatus(ctx, u, "Failed", fmt.Sprintf("Policy violation: %v", err), nil)
	}

	// Dispatch to appropriate action handler
	var result *actions.ActionResult
	var err error

	switch pkg.Spec.Action {
	case zarfv1alpha1.ActionBuild:
		result, err = c.buildHandler.Execute(ctx, pkg)

	case zarfv1alpha1.ActionPublish:
		// For standalone publish, assume artifact is already available
		// TODO: Implement artifact fetching from source
		result, err = c.publishHandler.Execute(ctx, pkg, "/workspace/package.tar.zst")

	case zarfv1alpha1.ActionDeploy:
		// For standalone deploy, assume artifact is already available
		// TODO: Implement artifact fetching from source
		result, err = c.deployHandler.Execute(ctx, pkg, "/workspace/package.tar.zst")

	case zarfv1alpha1.ActionBuildPublish:
		// Execute build first, job monitor will trigger publish when build completes
		result, err = c.buildHandler.Execute(ctx, pkg)

	case zarfv1alpha1.ActionBuildDeploy:
		// Execute build first, job monitor will trigger deploy when build completes
		result, err = c.buildHandler.Execute(ctx, pkg)

	case zarfv1alpha1.ActionPublishDeploy:
		// Execute publish first, job monitor will trigger deploy when publish completes
		result, err = c.publishHandler.Execute(ctx, pkg, "/workspace/package.tar.zst")

	case zarfv1alpha1.ActionBuildPublishDeploy:
		// Execute build first, job monitor will chain publish → deploy
		result, err = c.buildHandler.Execute(ctx, pkg)

	default:
		err = fmt.Errorf("action %s not yet implemented", pkg.Spec.Action)
	}

	// Update status based on result
	if err != nil {
		klog.ErrorS(err, "Action failed", "name", name, "namespace", namespace, "action", pkg.Spec.Action)
		return c.updateStatus(ctx, u, "Failed", err.Error(), nil)
	}

	if result != nil {
		opStatus := map[string]interface{}{
			"buildStatus": map[string]interface{}{
				"state":     result.Phase,
				"message":   result.Message,
				"startTime": result.StartTime.Format(time.RFC3339),
			},
		}
		if err := c.updateStatus(ctx, u, result.Phase, result.Message, opStatus); err != nil {
			klog.ErrorS(err, "Failed to update status", "name", name, "namespace", namespace)
		}
	}

	duration := time.Since(startTime)
	klog.InfoS("Reconciliation complete", "name", name, "namespace", namespace, "duration", duration)

	return nil
}

// updateStatus updates the status of a ZarfPackage resource
func (c *Controller) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string, operationStatus map[string]interface{}) error {
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

	gvr := ZarfPackageGVR
	if obj.GroupVersionKind().Group == udsv1alpha1.GroupName {
		gvr = UDSBundleGVR
	}

	_, err := c.dynamicClient.Resource(gvr).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).InfoS("ZarfPackage not found during status update", "name", name, "namespace", namespace)
			return nil
		}
		return fmt.Errorf("failed to update status: %w", err)
	}

	klog.V(4).InfoS("Status updated", "name", name, "namespace", namespace, "phase", phase)
	return nil
}

// HealthzHandler returns an HTTP handler for health checks
func (c *Controller) HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if c.healthy {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ok")); err != nil {
				klog.ErrorS(err, "Failed to write health response")
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("unhealthy")); err != nil {
				klog.ErrorS(err, "Failed to write unhealthy response")
			}
		}
	}
}

// ReadyzHandler returns an HTTP handler for readiness checks
func (c *Controller) ReadyzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if c.ready {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ready")); err != nil {
				klog.ErrorS(err, "Failed to write ready response")
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("not ready")); err != nil {
				klog.ErrorS(err, "Failed to write not ready response")
			}
		}
	}
}
