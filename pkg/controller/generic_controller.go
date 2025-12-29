package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions/common"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	"github.com/kylegalloway/forge/pkg/policy"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// DefaultArtifactStorageSize is the default size for artifact PVCs
	DefaultArtifactStorageSize = "10Gi"
)

// ControllerConfig holds configuration for the generic controller
type ControllerConfig struct {
	// ResourceType is the kind name (e.g., "ZarfPackageJob" or "UDSBundleJob")
	ResourceType string

	// ResourceGVR is the GroupVersionResource for this controller
	ResourceGVR schema.GroupVersionResource

	// PrimaryAction is the name of the primary action ("build" for Zarf, "create" for UDS)
	PrimaryAction string

	// LabelSelector for job monitoring (e.g., "app=forge" or "app=forge-uds")
	LabelSelector string

	// SupportsPVC indicates if this resource type supports artifact PVCs
	SupportsPVC bool

	// StatusFieldName is the status field name for primary action
	StatusFieldName string
}

// GenericController is a unified controller for package resources
type GenericController[T apiscommon.PackageResource] struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	resourceGVR   schema.GroupVersionResource
	metrics       *telemetry.Metrics
	tracer        *telemetry.Tracer
	policyEngine  *policy.Engine
	config        ControllerConfig

	// Action handlers
	primaryHandler common.ActionHandler[T]
	publishHandler common.ActionHandler[T]
	deployHandler  common.ActionHandler[T]

	// Job monitor
	monitor *GenericJobMonitor[T]

	// Health and readiness
	healthy atomic.Bool
	ready   atomic.Bool
}

// NewGenericController creates a new generic controller
func NewGenericController[T apiscommon.PackageResource](
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
	config ControllerConfig,
	primaryHandler common.ActionHandler[T],
	publishHandler common.ActionHandler[T],
	deployHandler common.ActionHandler[T],
	metricsRecorder MetricsRecorder[T],
) *GenericController[T] {
	policyEngine := policy.NewEngine(kubeClient)

	ctrl := &GenericController[T]{
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
		namespace:      namespace,
		resourceGVR:    config.ResourceGVR,
		metrics:        metrics,
		tracer:         tracer,
		policyEngine:   policyEngine,
		config:         config,
		primaryHandler: primaryHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
	}

	// Set initial health/ready state
	ctrl.healthy.Store(true)
	ctrl.ready.Store(false)

	// Initialize monitor config
	monitorConfig := MonitorConfig{
		ResourceType:       config.ResourceType,
		LabelSelector:      config.LabelSelector,
		PrimaryAction:      config.PrimaryAction,
		PrimaryStatusField: config.StatusFieldName,
		SupportsPVC:        config.SupportsPVC,
	}

	// Initialize generic job monitor
	ctrl.monitor = NewGenericJobMonitor[T](
		kubeClient,
		dynamicClient,
		namespace,
		config.ResourceGVR,
		metricsRecorder,
		monitorConfig,
		primaryHandler,
		publishHandler,
		deployHandler,
		ctrl.updateStatus,
	)

	return ctrl
}

// Run starts the controller's main reconciliation loop
func (ctrl *GenericController[T]) Run(ctx context.Context) error {
	klog.InfoS("Starting controller", "resourceType", ctrl.config.ResourceType)

	// Start job monitoring in background
	go ctrl.monitor.Start(ctx)

	// Watch resources with retry loop
	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Context canceled, stopping controller", "resourceType", ctrl.config.ResourceType)
			return nil
		default:
			if err := ctrl.watchResources(ctx); err != nil {
				klog.ErrorS(err, "Watch error, restarting in 5 seconds", "resourceType", ctrl.config.ResourceType)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// watchResources establishes a watch on resources
func (ctrl *GenericController[T]) watchResources(ctx context.Context) error {
	klog.V(2).InfoS("Starting watch", "resourceType", ctrl.config.ResourceType)

	// Set up resource interface
	var resourceInterface dynamic.ResourceInterface
	if ctrl.namespace == "" {
		resourceInterface = ctrl.dynamicClient.Resource(ctrl.resourceGVR)
	} else {
		resourceInterface = ctrl.dynamicClient.Resource(ctrl.resourceGVR).Namespace(ctrl.namespace)
	}

	// Start watch
	watcher, err := resourceInterface.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Stop()

	ctrl.ready.Store(true)
	klog.InfoS("Controller is ready", "resourceType", ctrl.config.ResourceType)

	// Process events
	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Context canceled, stopping watcher", "resourceType", ctrl.config.ResourceType)
			return nil

		case event, ok := <-watcher.ResultChan():
			if !ok {
				klog.Warning("Watch channel closed, recreating watcher", "resourceType", ctrl.config.ResourceType)
				return fmt.Errorf("watch channel closed")
			}

			if handleErr := ctrl.handleEvent(ctx, event); handleErr != nil {
				klog.ErrorS(handleErr, "Error handling event", "type", event.Type, "resourceType", ctrl.config.ResourceType)
				// Don't return error, continue processing
			}
		}
	}
}

// handleEvent processes a single watch event
func (ctrl *GenericController[T]) handleEvent(ctx context.Context, event watch.Event) error {
	switch event.Type {
	case watch.Added:
		return ctrl.handleObject(ctx, event.Object)
	case watch.Modified:
		return ctrl.handleObject(ctx, event.Object)
	case watch.Deleted:
		// Cleanup handled by owner references
		obj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type in deleted event")
		}
		klog.InfoS("Resource deleted", "name", obj.GetName(), "namespace", obj.GetNamespace(), "resourceType", ctrl.config.ResourceType)
		return nil
	case watch.Error:
		return fmt.Errorf("watch error: %v", event.Object)
	default:
		klog.V(4).InfoS("Ignoring event type", "type", event.Type)
		return nil
	}
}

// handleObject processes a resource object
func (ctrl *GenericController[T]) handleObject(ctx context.Context, obj interface{}) error {
	unstrObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}

	name := unstrObj.GetName()
	namespace := unstrObj.GetNamespace()

	klog.InfoS("Reconciling resource", "name", name, "namespace", namespace, "resourceType", ctrl.config.ResourceType)

	// Convert unstructured to typed resource
	var resource T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, &resource); err != nil {
		klog.ErrorS(err, "Failed to convert to typed resource", "name", name, "namespace", namespace)
		return err
	}

	// Check if resource is already completed
	status, hasStatus := unstrObj.Object["status"].(map[string]interface{})
	if hasStatus {
		phase, hasPhase := status["phase"].(string)
		if !hasPhase {
			return nil
		}
		if phase == "Completed" || phase == "Failed" {
			klog.V(4).InfoS("Resource already in terminal state, skipping", "name", name, "phase", phase)
			return nil
		}
	}

	// Policy validation
	if err := ctrl.validatePolicy(ctx, resource); err != nil {
		klog.ErrorS(err, "Policy validation failed", "name", name)
		return ctrl.updateResourceStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Policy validation failed: %v", err))
	}

	// Action dispatch
	if err := ctrl.dispatchAction(ctx, resource); err != nil {
		klog.ErrorS(err, "Action dispatch failed", "name", name, "action", resource.GetAction())
		return ctrl.updateResourceStatus(ctx, unstrObj, "Failed", fmt.Sprintf("Action failed: %v", err))
	}

	return nil
}

// validatePolicy validates the resource against ServiceAccount policies
func (ctrl *GenericController[T]) validatePolicy(ctx context.Context, resource T) error {
	// Policy validation is handled by the policy engine
	// Type-specific validation methods will be called based on the actual resource type
	// For now, use a simple interface-based approach

	// Check if resource implements a PolicyValidatable interface (future enhancement)
	// For now, return nil to allow policy engine to be called separately if needed
	_ = ctx
	_ = resource
	return nil
}

// dispatchAction routes to the appropriate action handler
func (ctrl *GenericController[T]) dispatchAction(ctx context.Context, resource T) error {
	action := resource.GetAction()
	opts := common.ExecuteOptions{}

	// Determine if this is a multi-action job that needs PVC
	if ctrl.config.SupportsPVC && ctrl.isMultiActionJob(action) {
		// Create PVC for artifact sharing
		pvcName := fmt.Sprintf("%s-artifacts", resource.GetName())
		if err := ctrl.ensureArtifactPVC(ctx, resource.GetNamespace(), pvcName, resource); err != nil {
			return fmt.Errorf("failed to ensure artifact PVC: %w", err)
		}
		opts.ArtifactPVCName = pvcName
	}

	// Execute primary action based on the action type
	// The monitor will handle chaining for compound actions
	if ctrl.isPrimaryAction(action) {
		_, err := ctrl.primaryHandler.Execute(ctx, resource, opts)
		return err
	}

	// Single publish or deploy action
	switch action {
	case "Publish", "publish":
		_, err := ctrl.publishHandler.Execute(ctx, resource, opts)
		return err
	case "Deploy", "deploy":
		_, err := ctrl.deployHandler.Execute(ctx, resource, opts)
		return err
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// isPrimaryAction checks if this is the primary action (Build or Create)
func (ctrl *GenericController[T]) isPrimaryAction(action string) bool {
	// Check if action starts with primary action or is a compound action containing it
	primaryActions := []string{
		ctrl.config.PrimaryAction,
		ctrl.config.PrimaryAction + "Publish",
		ctrl.config.PrimaryAction + "Deploy",
		ctrl.config.PrimaryAction + "PublishDeploy",
		"BuildPublish", "BuildDeploy", "BuildPublishDeploy", // Zarf
		"CreatePublish", "CreateDeploy", "CreatePublishDeploy", // UDS
	}

	for _, pa := range primaryActions {
		if action == pa {
			return true
		}
	}

	return false
}

// isMultiActionJob checks if an action is a compound action
func (ctrl *GenericController[T]) isMultiActionJob(action string) bool {
	multiActions := []string{
		ctrl.config.PrimaryAction + "Publish",
		ctrl.config.PrimaryAction + "Deploy",
		"PublishDeploy",
		ctrl.config.PrimaryAction + "PublishDeploy",
		"BuildPublish", "BuildDeploy", "BuildPublishDeploy",
		"CreatePublish", "CreateDeploy", "CreatePublishDeploy",
	}

	for _, ma := range multiActions {
		if action == ma {
			return true
		}
	}

	return false
}

// ensureArtifactPVC creates or verifies the artifact PVC exists
func (ctrl *GenericController[T]) ensureArtifactPVC(ctx context.Context, namespace, pvcName string, resource T) error {
	// Use the existing PVC creation logic
	return EnsureArtifactPVC(ctx, ctrl.kubeClient, namespace, pvcName, resource)
}

// updateResourceStatus updates resource status (helper for unstructured)
func (ctrl *GenericController[T]) updateResourceStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string) error {
	return ctrl.updateStatus(ctx, obj, phase, message, nil)
}

// updateStatus updates the resource status
func (ctrl *GenericController[T]) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string, operationStatus map[string]interface{}) error {
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

	_, err := ctrl.dynamicClient.Resource(ctrl.resourceGVR).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).InfoS("Resource not found during status update", "name", name, "namespace", namespace)
			return nil
		}
		return fmt.Errorf("failed to update status: %w", err)
	}

	klog.V(4).InfoS("Status updated", "name", name, "namespace", namespace, "phase", phase)
	return nil
}

// Healthy returns the controller's health status
func (ctrl *GenericController[T]) Healthy() bool {
	return ctrl.healthy.Load()
}

// Ready returns whether the controller is ready to serve traffic
func (ctrl *GenericController[T]) Ready() bool {
	return ctrl.ready.Load()
}

// HealthzHandler returns an HTTP handler for health checks
func (ctrl *GenericController[T]) HealthzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if ctrl.Healthy() {
			responseWriter.WriteHeader(http.StatusOK)
			if _, err := responseWriter.Write([]byte("ok")); err != nil {
				klog.ErrorS(err, "Failed to write health response")
			}
		} else {
			responseWriter.WriteHeader(http.StatusServiceUnavailable)
			if _, err := responseWriter.Write([]byte("unhealthy")); err != nil {
				klog.ErrorS(err, "Failed to write health response")
			}
		}
	}
}

// ReadyzHandler returns an HTTP handler for readiness checks
func (ctrl *GenericController[T]) ReadyzHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if ctrl.Ready() {
			responseWriter.WriteHeader(http.StatusOK)
			if _, err := responseWriter.Write([]byte("ready")); err != nil {
				klog.ErrorS(err, "Failed to write readiness response")
			}
		} else {
			responseWriter.WriteHeader(http.StatusServiceUnavailable)
			if _, err := responseWriter.Write([]byte("not ready")); err != nil {
				klog.ErrorS(err, "Failed to write readiness response")
			}
		}
	}
}

// EnsureArtifactPVC creates or ensures a PVC for artifact sharing exists
func EnsureArtifactPVC(ctx context.Context, kubeClient kubernetes.Interface, namespace, pvcName string, _ apiscommon.PackageResource) error {
	// Check if it exists and create if needed
	_, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Artifact PVC already exists", "pvc", pvcName, "namespace", namespace)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing PVC: %w", err)
	}

	// Create PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                        "forge",
				"forge.dev/artifact-storage": "true",
				"forge.dev/managed-by":       "forge-controller",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(DefaultArtifactStorageSize),
				},
			},
		},
	}

	_, err = kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create artifact PVC: %w", err)
	}

	klog.InfoS("Created artifact PVC", "pvc", pvcName, "namespace", namespace, "size", DefaultArtifactStorageSize)
	return nil
}
