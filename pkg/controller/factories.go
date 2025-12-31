// Package controller provides factory functions for creating generic controllers
package controller

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/kylegalloway/forge/pkg/actions/uds"
	"github.com/kylegalloway/forge/pkg/actions/zarf"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// NewGenericZarfController creates a new GenericController for ZarfPackageJob resources
func NewGenericZarfController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
) *GenericController[*zarfv1alpha1.ZarfPackageJob] {
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

	// Configure controller
	config := ControllerConfig{
		ResourceType:    "ZarfPackageJob",
		ResourceGVR:     constants.ZarfPackageJobGVR,
		PrimaryAction:   constants.ActionBuild,
		LabelSelector:   "app=forge",
		SupportsPVC:     true,
		StatusFieldName: "buildStatus",
	}

	return NewGenericController[*zarfv1alpha1.ZarfPackageJob](
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
	)
}

// NewGenericUDSController creates a new GenericController for UDSBundleJob resources
func NewGenericUDSController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer,
) *GenericController[*udsv1alpha2.UDSBundleJob] {
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

	// Configure controller
	config := ControllerConfig{
		ResourceType:    "UDSBundleJob",
		ResourceGVR:     constants.UDSBundleJobGVR,
		PrimaryAction:   constants.ActionCreate,
		LabelSelector:   "app=forge-uds",
		SupportsPVC:     true, // Phase 4: PVC support enabled for multi-action workflows
		StatusFieldName: "createStatus",
	}

	return NewGenericController[*udsv1alpha2.UDSBundleJob](
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
	)
}
