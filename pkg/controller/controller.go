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

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/actions"
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
)

// Controller watches ZarfPackage resources and executes actions
type Controller struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	metrics       *telemetry.Metrics
	tracer        *telemetry.Tracer
	buildHandler  *actions.BuildHandler
	healthy       bool
	ready         bool
}

// NewController creates a new Forge controller
func NewController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
) *Controller {
	// Initialize action handlers
	buildHandler := actions.NewBuildHandler(kubeClient, metrics, tracer)

	return &Controller{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		metrics:       metrics,
		tracer:        tracer,
		buildHandler:  buildHandler,
		healthy:       true,
		ready:         false,
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

	c.ready = true
	klog.Info("Forge controller is ready")

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			klog.Info("Context cancelled, stopping controller")
			return nil

		case event, ok := <-watcher.ResultChan():
			if !ok {
				klog.Warning("Watch channel closed, restarting watcher")
				// Recreate watcher
				watcher, err = c.dynamicClient.Resource(ZarfPackageGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to recreate watcher: %w", err)
				}
				continue
			}

			if err := c.handleEvent(ctx, event); err != nil {
				klog.ErrorS(err, "Error handling event", "type", event.Type)
				// Don't return error, continue processing
			}
		}
	}
}

// handleEvent processes a single watch event
func (c *Controller) handleEvent(ctx context.Context, event watch.Event) error {
	switch event.Type {
	case watch.Added:
		return c.handleZarfPackage(ctx, event.Object)
	case watch.Modified:
		return c.handleZarfPackage(ctx, event.Object)
	case watch.Deleted:
		// Cleanup handled by owner references
		obj := event.Object.(*unstructured.Unstructured)
		klog.InfoS("ZarfPackage deleted", "name", obj.GetName(), "namespace", obj.GetNamespace())
		return nil
	case watch.Error:
		return fmt.Errorf("watch error: %v", event.Object)
	default:
		klog.V(4).InfoS("Ignoring event type", "type", event.Type)
		return nil
	}
}

// handleZarfPackage reconciles a ZarfPackage resource
func (c *Controller) handleZarfPackage(ctx context.Context, obj interface{}) error {
	startTime := time.Now()

	// Convert to unstructured
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}

	name := u.GetName()
	namespace := u.GetNamespace()

	klog.InfoS("Reconciling ZarfPackage", "name", name, "namespace", namespace)

	// Convert unstructured to typed ZarfPackage
	pkg := &zarfv1alpha1.ZarfPackage{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pkg); err != nil {
		klog.ErrorS(err, "Failed to convert unstructured to ZarfPackage", "name", name, "namespace", namespace)
		return c.updateStatus(ctx, u, "Failed", fmt.Sprintf("Invalid ZarfPackage: %v", err), nil)
	}

	klog.InfoS("Processing ZarfPackage action", "name", name, "namespace", namespace, "action", pkg.Spec.Action)

	// Dispatch to appropriate action handler
	var result *actions.ActionResult
	var err error

	switch pkg.Spec.Action {
	case zarfv1alpha1.ActionBuild:
		result, err = c.buildHandler.Execute(ctx, pkg)
	case zarfv1alpha1.ActionBuildPublish, zarfv1alpha1.ActionBuildDeploy, zarfv1alpha1.ActionBuildPublishDeploy:
		// For now, just execute the build part
		// TODO: Implement publish and deploy handlers
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
		"phase":             phase,
		"message":           message,
		"lastUpdateTime":    metav1.Now().Format(time.RFC3339),
		"observedGeneration": obj.GetGeneration(),
	}

	// Add operation-specific status if provided
	if operationStatus != nil {
		for key, value := range operationStatus {
			status[key] = value
		}
	}

	// Update status subresource
	obj.Object["status"] = status

	_, err := c.dynamicClient.Resource(ZarfPackageGVR).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
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
	return func(w http.ResponseWriter, r *http.Request) {
		if c.healthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy"))
		}
	}
}

// ReadyzHandler returns an HTTP handler for readiness checks
func (c *Controller) ReadyzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c.ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	}
}
