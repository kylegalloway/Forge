// Package testing provides test fixtures and helpers for controller testing
package testing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// ControllerFixture provides a test fixture for controller testing
// with pre-configured clients and utilities.
type ControllerFixture struct {
	KubeClient    *fake.Clientset
	DynamicClient *dynamicfake.FakeDynamicClient
	Metrics       *telemetry.Metrics
	Tracer        *telemetry.Tracer
	Namespace     string
	Scheme        *runtime.Scheme
}

// NewControllerFixture creates a new test fixture with default configuration
func NewControllerFixture(namespace string) *ControllerFixture {
	scheme := runtime.NewScheme()

	return &ControllerFixture{
		KubeClient:    fake.NewClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme),
		Metrics:       MustNewMetrics(),
		Tracer:        telemetry.NewTracer(),
		Namespace:     namespace,
		Scheme:        scheme,
	}
}

// NewControllerFixtureWithScheme creates a test fixture with a custom scheme
func NewControllerFixtureWithScheme(namespace string, scheme *runtime.Scheme) *ControllerFixture {
	return &ControllerFixture{
		KubeClient:    fake.NewClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme),
		Metrics:       MustNewMetrics(),
		Tracer:        telemetry.NewTracer(),
		Namespace:     namespace,
		Scheme:        scheme,
	}
}

// ToUnstructured converts a PackageResource to an unstructured object
func (f *ControllerFixture) ToUnstructured(resource apiscommon.PackageResource) (*unstructured.Unstructured, error) {
	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstrObj}, nil
}

// MustToUnstructured converts a PackageResource to unstructured or panics
func (f *ControllerFixture) MustToUnstructured(resource apiscommon.PackageResource) *unstructured.Unstructured {
	obj, err := f.ToUnstructured(resource)
	if err != nil {
		panic(err)
	}
	return obj
}
