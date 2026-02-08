// Package controller provides factory functions for creating generic controllers
package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kylegalloway/forge/pkg/actions/uds"
	"github.com/kylegalloway/forge/pkg/actions/zarf"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// informerResyncPeriod is the resync period for informers
	informerResyncPeriod = 30 * time.Second
)

// NewGenericZarfController creates a new GenericController for ZarfPackageJob resources
func NewGenericZarfController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
	concurrencyConfig ConcurrencyConfig,
) *GenericController[*zarfv1alpha3.ZarfPackageJob] {
	// Create action handlers
	buildHandler := zarf.NewBuildHandler(kubeClient, metrics, tracer)
	publishHandler := zarf.NewPublishHandler(kubeClient, metrics, tracer)
	deployHandler := zarf.NewDeployHandler(kubeClient, dynamicClient, metrics, tracer)

	// Create handler adapters
	primaryAdapter := NewZarfBuildHandlerAdapter(buildHandler)
	publishAdapter := NewZarfPublishHandlerAdapter(publishHandler)
	deployAdapter := NewZarfDeployHandlerAdapter(deployHandler)

	// Create metrics recorder
	metricsRecorder := NewZarfMetricsRecorder(metrics)

	labelSelector := constants.LabelApp + "=" + constants.LabelAppValueZarf

	// Configure controller
	config := ControllerConfig{
		ResourceType:    constants.ResourceTypeZarfPackageJob,
		ResourceGVR:     constants.ZarfPackageJobGVR,
		PrimaryAction:   constants.ActionBuild,
		LabelSelector:   labelSelector,
		SupportsPVC:     true,
		StatusFieldName: constants.StatusFieldBuild,
		Concurrency:     concurrencyConfig,
	}

	// Create informers
	crdInformer := newDynamicInformer(dynamicClient, constants.ZarfPackageJobGVR, namespace)
	jobInformer := newJobInformer(kubeClient, namespace, labelSelector)

	// Build controller options
	opts := []ControllerOption[*zarfv1alpha3.ZarfPackageJob]{
		WithInformer[*zarfv1alpha3.ZarfPackageJob](crdInformer),
		WithJobInformer[*zarfv1alpha3.ZarfPackageJob](jobInformer),
	}

	if concurrencyConfig.MaxConcurrentJobsPerNamespace > 0 || concurrencyConfig.MaxConcurrentJobsGlobal > 0 {
		limiter := NewConcurrencyLimiter(concurrencyConfig, jobInformer.GetStore())
		opts = append(opts, WithConcurrencyLimiter[*zarfv1alpha3.ZarfPackageJob](limiter))
	}

	ctrl := NewGenericController(
		kubeClient,
		dynamicClient,
		namespace,
		metrics,
		tracer,
		config,
		primaryAdapter,
		publishAdapter,
		deployAdapter,
		metricsRecorder,
		opts...,
	)

	// Wire the requeueFunc and jobStore after construction (needs ctrl reference)
	ctrl.monitor.jobStore = jobInformer.GetStore()
	ctrl.monitor.requeueFunc = ctrl.RequeueQueuedResources

	return ctrl
}

// NewGenericUDSController creates a new GenericController for UDSBundleJob resources
func NewGenericUDSController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
	concurrencyConfig ConcurrencyConfig,
) *GenericController[*udsv1alpha3.UDSBundleJob] {
	// Create action handlers
	createHandler := uds.NewCreateHandler(kubeClient, metrics, tracer)
	publishHandler := uds.NewPublishHandler(kubeClient, metrics, tracer)
	deployHandler := uds.NewDeployHandler(kubeClient, dynamicClient, metrics, tracer)

	// Create handler adapters
	primaryAdapter := NewUDSCreateHandlerAdapter(createHandler)
	publishAdapter := NewUDSPublishHandlerAdapter(publishHandler)
	deployAdapter := NewUDSDeployHandlerAdapter(deployHandler)

	// Create metrics recorder
	metricsRecorder := NewUDSMetricsRecorder(metrics)

	labelSelector := constants.LabelApp + "=" + constants.LabelAppValueUDS

	// Configure controller
	config := ControllerConfig{
		ResourceType:    constants.ResourceTypeUDSBundleJob,
		ResourceGVR:     constants.UDSBundleJobGVR,
		PrimaryAction:   constants.ActionCreate,
		LabelSelector:   labelSelector,
		SupportsPVC:     true,
		StatusFieldName: constants.StatusFieldCreate,
		Concurrency:     concurrencyConfig,
	}

	// Create informers
	crdInformer := newDynamicInformer(dynamicClient, constants.UDSBundleJobGVR, namespace)
	jobInformer := newJobInformer(kubeClient, namespace, labelSelector)

	// Build controller options
	opts := []ControllerOption[*udsv1alpha3.UDSBundleJob]{
		WithInformer[*udsv1alpha3.UDSBundleJob](crdInformer),
		WithJobInformer[*udsv1alpha3.UDSBundleJob](jobInformer),
	}

	if concurrencyConfig.MaxConcurrentJobsPerNamespace > 0 || concurrencyConfig.MaxConcurrentJobsGlobal > 0 {
		limiter := NewConcurrencyLimiter(concurrencyConfig, jobInformer.GetStore())
		opts = append(opts, WithConcurrencyLimiter[*udsv1alpha3.UDSBundleJob](limiter))
	}

	ctrl := NewGenericController(
		kubeClient,
		dynamicClient,
		namespace,
		metrics,
		tracer,
		config,
		primaryAdapter,
		publishAdapter,
		deployAdapter,
		metricsRecorder,
		opts...,
	)

	// Wire the requeueFunc and jobStore after construction (needs ctrl reference)
	ctrl.monitor.jobStore = jobInformer.GetStore()
	ctrl.monitor.requeueFunc = ctrl.RequeueQueuedResources

	return ctrl
}

// newDynamicInformer creates a shared informer for a dynamic CRD resource.
func newDynamicInformer(client dynamic.Interface, gvr schema.GroupVersionResource, namespace string) cache.SharedIndexInformer {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		client,
		informerResyncPeriod,
		namespace,
		nil,
	)
	return factory.ForResource(gvr).Informer()
}

// newJobInformer creates a shared informer for batch/v1 Jobs filtered by label selector.
func newJobInformer(client kubernetes.Interface, namespace, labelSelector string) cache.SharedIndexInformer {
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		informerResyncPeriod,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)
	return factory.Batch().V1().Jobs().Informer()
}
