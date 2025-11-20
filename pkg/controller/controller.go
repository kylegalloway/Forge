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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

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
	return &Controller{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		metrics:       metrics,
		tracer:        tracer,
		healthy:       true,
		ready:         false,
	}
}

// Run starts the controller's main reconciliation loop
func (c *Controller) Run(ctx context.Context) error {
	klog.Info("Starting Forge controller")

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

	// Extract spec
	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("failed to get spec: %w", err)
	}

	// Get action
	action, found, err := unstructured.NestedString(spec, "action")
	if err != nil || !found {
		return c.updateStatus(ctx, u, "Failed", "No action specified", nil)
	}

	klog.InfoS("Processing ZarfPackage action", "name", name, "namespace", namespace, "action", action)

	// TODO: Implement action dispatching to handlers
	// For now, just update status to show controller is running
	err = c.updateStatus(ctx, u, "Pending", fmt.Sprintf("Action %s not yet implemented", action), nil)

	duration := time.Since(startTime)
	klog.InfoS("Reconciliation complete", "name", name, "namespace", namespace, "duration", duration)

	return err
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
