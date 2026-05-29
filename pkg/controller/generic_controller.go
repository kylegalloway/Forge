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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions/common"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/logging"
	"github.com/kylegalloway/forge/pkg/policy"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// DefaultArtifactStorageSize is the default size for artifact PVCs
	DefaultArtifactStorageSize = "10Gi"

	// numWorkers is the number of worker goroutines processing the work queue
	numWorkers = 2
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

	// Concurrency holds concurrency limit configuration
	Concurrency ConcurrencyConfig
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
	logger        *logging.Logger

	// Action handlers
	primaryHandler common.ActionHandler[T]
	publishHandler common.ActionHandler[T]
	deployHandler  common.ActionHandler[T]

	// Job monitor
	monitor *GenericJobMonitor[T]

	// Informer and work queue
	informer           cache.SharedIndexInformer
	jobInformer        cache.SharedIndexInformer
	queue              workqueue.TypedRateLimitingInterface[string]
	concurrencyLimiter *ConcurrencyLimiter

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
	opts ...ControllerOption[T],
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
		logger:         logging.NewLogger(config.ResourceType + "-controller"),
		primaryHandler: primaryHandler,
		publishHandler: publishHandler,
		deployHandler:  deployHandler,
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[string](),
		),
	}

	// Apply options
	for _, opt := range opts {
		opt(ctrl)
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

	// Build monitor options
	var monitorOpts []MonitorOption[T]
	// jobStore and requeueFunc will be wired in factories.go via options

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
		monitorOpts...,
	)

	return ctrl
}

// ControllerOption is a functional option for configuring the GenericController.
type ControllerOption[T apiscommon.PackageResource] func(*GenericController[T])

// WithInformer sets the shared informer for the CRD resources.
func WithInformer[T apiscommon.PackageResource](informer cache.SharedIndexInformer) ControllerOption[T] {
	return func(ctrl *GenericController[T]) {
		ctrl.informer = informer
	}
}

// WithConcurrencyLimiter sets the concurrency limiter for job creation.
func WithConcurrencyLimiter[T apiscommon.PackageResource](limiter *ConcurrencyLimiter) ControllerOption[T] {
	return func(ctrl *GenericController[T]) {
		ctrl.concurrencyLimiter = limiter
	}
}

// WithMonitorJobStore sets the job cache store on the monitor.
func WithMonitorJobStore[T apiscommon.PackageResource](store cache.Store) ControllerOption[T] {
	return func(ctrl *GenericController[T]) {
		ctrl.monitor.jobStore = store
	}
}

// WithMonitorRequeueFunc sets the requeue function on the monitor.
func WithMonitorRequeueFunc[T apiscommon.PackageResource](fn func(namespace string)) ControllerOption[T] {
	return func(ctrl *GenericController[T]) {
		ctrl.monitor.requeueFunc = fn
	}
}

// WithJobInformer sets the job informer that will be started alongside the CRD informer.
func WithJobInformer[T apiscommon.PackageResource](informer cache.SharedIndexInformer) ControllerOption[T] {
	return func(ctrl *GenericController[T]) {
		ctrl.jobInformer = informer
	}
}

// Run starts the controller's main reconciliation loop
func (ctrl *GenericController[T]) Run(ctx context.Context) error {
	klog.InfoS("Starting controller", "resourceType", ctrl.config.ResourceType)

	if ctrl.informer != nil {
		return ctrl.runWithInformer(ctx)
	}
	return ctrl.runWithWatch(ctx)
}

// runWithInformer runs the controller using a shared informer and work queue.
func (ctrl *GenericController[T]) runWithInformer(ctx context.Context) error {
	// Register event handlers that enqueue keys
	if _, err := ctrl.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				ctrl.queue.Add(key)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				ctrl.queue.Add(key)
			}
		},
	}); err != nil {
		return fmt.Errorf("adding event handler: %w", err)
	}

	// Start informers
	go ctrl.informer.Run(ctx.Done())
	syncFuncs := []cache.InformerSynced{ctrl.informer.HasSynced}
	if ctrl.jobInformer != nil {
		go ctrl.jobInformer.Run(ctx.Done())
		syncFuncs = append(syncFuncs, ctrl.jobInformer.HasSynced)
	}

	// Wait for cache sync
	klog.InfoS("Waiting for informer cache sync", "resourceType", ctrl.config.ResourceType)
	if !cache.WaitForCacheSync(ctx.Done(), syncFuncs...) {
		return fmt.Errorf("failed to sync informer cache for %s", ctrl.config.ResourceType)
	}
	klog.InfoS("Informer cache synced", "resourceType", ctrl.config.ResourceType)

	ctrl.ready.Store(true)
	klog.InfoS("Controller is ready", "resourceType", ctrl.config.ResourceType)

	// Start job monitoring in background
	go ctrl.monitor.Start(ctx)

	// Start worker goroutines
	workers := numWorkers
	if ctrl.config.Concurrency.NumWorkers > 0 {
		workers = ctrl.config.Concurrency.NumWorkers
	}
	for i := 0; i < workers; i++ {
		go ctrl.runWorker(ctx)
	}

	<-ctx.Done()
	klog.InfoS("Context canceled, stopping controller", "resourceType", ctrl.config.ResourceType)
	ctrl.queue.ShutDown()
	return nil
}

// runWithWatch runs the controller using the legacy watch pattern (when no informer is provided).
func (ctrl *GenericController[T]) runWithWatch(ctx context.Context) error {
	// Start job monitoring in background
	go ctrl.monitor.Start(ctx)

	// Watch resources with retry loop
	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Context canceled, stopping controller", "resourceType", ctrl.config.ResourceType)
			ctrl.queue.ShutDown()
			return nil
		default:
			if err := ctrl.watchResources(ctx); err != nil {
				klog.ErrorS(err, "Watch error, restarting in 5 seconds", "resourceType", ctrl.config.ResourceType)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// watchResources establishes a watch on resources (legacy path used when no informer is configured)
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

// handleEvent processes a single watch event (legacy path)
func (ctrl *GenericController[T]) handleEvent(ctx context.Context, event watch.Event) error {
	switch event.Type {
	case watch.Added:
		return ctrl.handleObject(ctx, event.Object)
	case watch.Modified:
		return ctrl.handleObject(ctx, event.Object)
	case watch.Deleted:
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

// runWorker runs the work queue processing loop.
func (ctrl *GenericController[T]) runWorker(ctx context.Context) {
	for ctrl.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem dequeues a key and reconciles the resource.
func (ctrl *GenericController[T]) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := ctrl.queue.Get()
	if shutdown {
		return false
	}
	defer ctrl.queue.Done(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Invalid key", "key", key)
		ctrl.queue.Forget(key)
		return true
	}

	if err := ctrl.reconcile(ctx, namespace, name); err != nil {
		klog.ErrorS(err, "Reconcile error, requeuing", "key", key)
		ctrl.queue.AddRateLimited(key)
		return true
	}

	ctrl.queue.Forget(key)
	return true
}

// reconcile processes a single resource by namespace/name.
func (ctrl *GenericController[T]) reconcile(ctx context.Context, namespace, name string) error {
	startTime := time.Now()

	// Fetch object from informer cache if available, otherwise from API
	var unstrObj *unstructured.Unstructured
	if ctrl.informer != nil {
		key := keyFunc(namespace, name)
		item, exists, err := ctrl.informer.GetStore().GetByKey(key)
		if err != nil {
			return fmt.Errorf("failed to get object from cache: %w", err)
		}
		if !exists {
			klog.V(4).InfoS("Resource not found in cache, skipping", "name", name, "namespace", namespace)
			return nil
		}
		var ok bool
		unstrObj, ok = item.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type in cache: %T", item)
		}
	} else {
		// Legacy: fetch directly from API
		var resourceInterface dynamic.ResourceInterface
		if namespace == "" {
			resourceInterface = ctrl.dynamicClient.Resource(ctrl.resourceGVR)
		} else {
			resourceInterface = ctrl.dynamicClient.Resource(ctrl.resourceGVR).Namespace(namespace)
		}
		var err error
		unstrObj, err = resourceInterface.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				klog.V(4).InfoS("Resource not found, skipping", "name", name, "namespace", namespace)
				return nil
			}
			return fmt.Errorf("failed to get resource: %w", err)
		}
	}

	// Convert unstructured to typed resource first to get action
	var resource T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, &resource); err != nil {
		klog.ErrorS(err, "Failed to convert to typed resource", "name", name, "namespace", namespace)
		return err
	}

	// Set up logging context with correlation ID
	correlationID := logging.GenerateCorrelationID(namespace, name, resource.GetAction())
	ctx = logging.WithCorrelationID(ctx, correlationID)
	ctx = logging.WithJobName(ctx, name)
	ctx = logging.WithNamespace(ctx, namespace)
	ctx = logging.WithAction(ctx, resource.GetAction())

	ctrl.logger.Debug(ctx, "Handling object event",
		"eventType", "reconcile",
		"generation", unstrObj.GetGeneration(),
		"resourceVersion", unstrObj.GetResourceVersion(),
		"debugMode", resource.GetDebugMode(),
	)

	klog.InfoS("Reconciling resource", "name", name, "namespace", namespace, "resourceType", ctrl.config.ResourceType)

	// Check if resource is in retry state
	status, hasStatus := unstrObj.Object["status"].(map[string]interface{})
	if hasStatus {
		phase, hasPhase := status["phase"].(string)
		if !hasPhase {
			return nil
		}

		// Handle retry scheduling
		switch phase {
		case constants.PhaseRetrying:
			if ctrl.shouldRetryNow(status) {
				ctrl.logger.Debug(ctx, "Retry time reached, dispatching action", "phase", phase)
				klog.InfoS("Retry time reached, dispatching action", "name", name)
				// Fall through to dispatch action
			} else {
				ctrl.logger.Debug(ctx, "Retry scheduled but not due yet", "phase", phase)
				klog.V(4).InfoS("Retry scheduled but not due yet", "name", name)
				return nil
			}
		case constants.PhaseCompleted, constants.PhaseFailed:
			// Resource in terminal state, skip
			ctrl.logger.Debug(ctx, "Skipping terminal resource", "status", phase, "reason", "already_completed")
			klog.V(4).InfoS("Resource already in terminal state, skipping", "name", name, "phase", phase)
			return nil
		case constants.PhaseQueued:
			// Re-check concurrency on queued resources (they were requeued)
			ctrl.logger.Debug(ctx, "Re-evaluating queued resource", "phase", phase)
		}
	}

	// Policy validation
	ctrl.logger.Debug(ctx, "Starting policy validation")
	if err := ctrl.validatePolicy(ctx, resource); err != nil {
		ctrl.logger.Debug(ctx, "Policy validation failed", "error", err.Error())
		klog.ErrorS(err, "Policy validation failed", "name", name)
		return ctrl.updateResourceStatus(ctx, unstrObj, constants.PhaseFailed, fmt.Sprintf("Policy validation failed: %v", err))
	}
	ctrl.logger.Debug(ctx, "Policy validation passed")

	// Concurrency check before dispatch
	if ctrl.concurrencyLimiter != nil {
		allowed, reason := ctrl.concurrencyLimiter.CanCreateJob(namespace)
		if !allowed {
			ctrl.logger.Debug(ctx, "Backpressure: queuing resource", "reason", reason)
			klog.InfoS("Backpressure: queuing resource", "name", name, "namespace", namespace, "reason", reason)
			if ctrl.metrics != nil {
				ctrl.metrics.RecordBackpressureEvent(ctx, namespace)
			}
			// Update status to Queued
			if err := ctrl.updateResourceStatus(ctx, unstrObj, constants.PhaseQueued,
				fmt.Sprintf("Waiting for capacity: %s", reason)); err != nil {
				return err
			}
			// Requeue with delay
			key := keyFunc(namespace, name)
			ctrl.queue.AddAfter(key, 5*time.Second)
			return nil
		}
	}

	// Action dispatch
	ctrl.logger.Debug(ctx, "Dispatching to handler",
		"action", resource.GetAction(),
		"isPrimaryAction", ctrl.isPrimaryAction(resource.GetAction()),
	)
	if err := ctrl.dispatchAction(ctx, resource); err != nil {
		ctrl.logger.Debug(ctx, "Handler failed", "error", err.Error(), "duration", time.Since(startTime).String())
		klog.ErrorS(err, "Action dispatch failed", "name", name, "action", resource.GetAction())
		return ctrl.updateResourceStatus(ctx, unstrObj, constants.PhaseFailed, fmt.Sprintf("Action failed: %v", err))
	}

	ctrl.logger.Debug(ctx, "Handler completed", "duration", time.Since(startTime).String())
	return nil
}

// handleObject processes a resource object (used by legacy watch path)
func (ctrl *GenericController[T]) handleObject(ctx context.Context, obj interface{}) error {
	unstrObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}
	return ctrl.reconcile(ctx, unstrObj.GetNamespace(), unstrObj.GetName())
}

// RequeueQueuedResources re-enqueues all resources in PhaseQueued state from the given namespace.
// This is called by the monitor's requeueFunc when a job completes and frees capacity.
func (ctrl *GenericController[T]) RequeueQueuedResources(namespace string) {
	if ctrl.informer == nil {
		return
	}

	for _, item := range ctrl.informer.GetStore().List() {
		obj, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if namespace != "" && obj.GetNamespace() != namespace {
			continue
		}
		status, ok := obj.Object["status"].(map[string]interface{})
		if !ok {
			continue
		}
		phase, ok := status["phase"].(string)
		if !ok {
			continue
		}
		if phase == constants.PhaseQueued {
			key := keyFunc(obj.GetNamespace(), obj.GetName())
			ctrl.queue.Add(key)
		}
	}
}

// validatePolicy validates the resource against ServiceAccount policies
func (ctrl *GenericController[T]) validatePolicy(ctx context.Context, resource T) error {
	_ = ctx
	_ = resource
	return nil
}

// dispatchAction routes to the appropriate action handler
func (ctrl *GenericController[T]) dispatchAction(ctx context.Context, resource T) error {
	action := resource.GetAction()
	opts := common.ExecuteOptions{}

	// Create PVC for all primary actions (Build/Create) unless explicitly disabled
	if ctrl.config.SupportsPVC && ctrl.isPrimaryAction(action) && resource.GetUseArtifactPVC() {
		pvcName := fmt.Sprintf("%s-artifacts", resource.GetName())
		ctrl.logger.Debug(ctx, "Ensuring artifact PVC", "pvcName", pvcName)
		if err := ctrl.ensureArtifactPVC(ctx, resource.GetNamespace(), pvcName, resource); err != nil {
			ctrl.logger.Debug(ctx, "Failed to ensure artifact PVC", "error", err.Error())
			return fmt.Errorf("failed to ensure artifact PVC: %w", err)
		}
		opts.ArtifactPVCName = pvcName
		ctrl.logger.Debug(ctx, "Artifact PVC ready", "pvcName", pvcName)
	}

	// Execute primary action based on the action type
	if ctrl.isPrimaryAction(action) {
		ctrl.logger.Debug(ctx, "Executing primary handler", "handler", ctrl.config.PrimaryAction)
		_, err := ctrl.primaryHandler.Execute(ctx, resource, opts)
		return err
	}

	// Single publish or deploy action (standalone)
	switch action {
	case constants.SpecActionPublish:
		ctrl.logger.Debug(ctx, "Executing publish handler")
		_, err := ctrl.publishHandler.Execute(ctx, resource, opts)
		return err
	case constants.SpecActionDeploy:
		ctrl.logger.Debug(ctx, "Executing deploy handler")
		_, err := ctrl.deployHandler.Execute(ctx, resource, opts)
		return err
	default:
		ctrl.logger.Debug(ctx, "Unknown action", "action", action)
		return fmt.Errorf("unknown action: %s", action)
	}
}

// isPrimaryAction checks if this is the primary action (Build or Create).
func (ctrl *GenericController[T]) isPrimaryAction(action string) bool {
	primaryActions := []string{
		constants.SpecActionBuild,
		constants.SpecActionCreate,
		constants.SpecActionBuildPublish,
		constants.SpecActionBuildDeploy,
		constants.SpecActionBuildPublishDeploy,
		constants.SpecActionCreatePublish,
		constants.SpecActionCreateDeploy,
		constants.SpecActionCreatePublishDeploy,
	}

	for _, pa := range primaryActions {
		if action == pa {
			return true
		}
	}

	return false
}

// ensureArtifactPVC creates or verifies the artifact PVC exists
func (ctrl *GenericController[T]) ensureArtifactPVC(ctx context.Context, namespace, pvcName string, resource T) error {
	return EnsureArtifactPVC(ctx, ctrl.kubeClient, namespace, pvcName, resource)
}

// updateResourceStatus updates resource status (helper for unstructured)
func (ctrl *GenericController[T]) updateResourceStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string) error {
	return ctrl.updateStatus(ctx, obj, phase, message, nil)
}

// updateStatus updates the resource status.
// It merges into the existing status (preserving prior operation fields across
// chained actions) and derives standard conditions from the resulting phase.
func (ctrl *GenericController[T]) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string, operationStatus map[string]interface{}) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	// Start from existing status so chained actions don't lose prior operation fields.
	existing, _, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return fmt.Errorf("failed to read existing status: %w", err)
	}
	if existing == nil {
		existing = map[string]interface{}{}
	}

	existingConditions := conditionsFromUnstructured(existing["conditions"])

	existing["phase"] = phase
	existing["message"] = message
	existing["lastUpdateTime"] = metav1.Now().Format(time.RFC3339)
	existing["observedGeneration"] = obj.GetGeneration()

	for key, value := range operationStatus {
		existing[key] = value
	}

	conditions := DeriveConditions(phase, existing, existingConditions, obj.GetGeneration())
	existing["conditions"] = conditionsToUnstructured(conditions)

	obj.Object["status"] = existing

	_, err = ctrl.dynamicClient.Resource(ctrl.resourceGVR).Namespace(namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
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
func EnsureArtifactPVC(ctx context.Context, kubeClient kubernetes.Interface, namespace, pvcName string, res apiscommon.PackageResource) error {
	_, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Artifact PVC already exists", "pvc", pvcName, "namespace", namespace)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing PVC: %w", err)
	}

	storageSize := resource.MustParse(DefaultArtifactStorageSize)
	if vs := res.GetVolumeSizes(); vs != nil && vs.ArtifactStorage != nil {
		storageSize = *vs.ArtifactStorage
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.LabelApp:           constants.LabelAppValueZarf,
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
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	_, err = kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create artifact PVC: %w", err)
	}

	klog.InfoS("Created artifact PVC", "pvc", pvcName, "namespace", namespace, "size", storageSize.String())
	return nil
}

// shouldRetryNow checks if the retry time has been reached for any operation in retry state
func (ctrl *GenericController[T]) shouldRetryNow(status map[string]interface{}) bool {
	statusFields := []string{constants.StatusFieldBuild, constants.StatusFieldCreate, constants.StatusFieldPublish, constants.StatusFieldDeploy}

	for _, field := range statusFields {
		opStatus, ok := status[field].(map[string]interface{})
		if !ok {
			continue
		}

		state, ok := opStatus[constants.StatusKeyState].(string)
		if !ok || state != constants.PhaseRetrying {
			continue
		}

		nextRetryTimeStr, ok := opStatus[constants.StatusKeyNextRetryTime].(string)
		if !ok {
			return true
		}

		nextRetryTime, err := time.Parse(time.RFC3339, nextRetryTimeStr)
		if err != nil {
			klog.V(4).InfoS("Failed to parse nextRetryTime, retrying immediately", "field", field, "time", nextRetryTimeStr)
			return true
		}

		if time.Now().After(nextRetryTime) {
			return true
		}
	}

	return false
}
