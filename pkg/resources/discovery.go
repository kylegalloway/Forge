package resources

import (
	"context"
	"path/filepath"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"

	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// Discoverer finds existing resources matching selector criteria
type Discoverer struct {
	dynamicClient dynamic.Interface
}

// NewDiscoverer creates a new resource discoverer
func NewDiscoverer(dynamicClient dynamic.Interface) *Discoverer {
	return &Discoverer{
		dynamicClient: dynamicClient,
	}
}

// DiscoverZarfResources finds resources matching Zarf ResourceSelector in specified namespaces
func (d *Discoverer) DiscoverZarfResources(_ context.Context, selector *zarfv1alpha3.ResourceSelector, namespaces []string) ([]ResourceReference, error) {
	if selector == nil {
		return nil, nil
	}

	klog.InfoS("Discovering resources", "selector", selector, "namespaces", namespaces)

	var discovered []ResourceReference

	// TODO: For now, we'll return empty - actual resource discovery would require
	// knowledge of which resource types to look for (Deployments, Services, etc.)
	// This is a placeholder that shows the structure

	klog.InfoS("Resource discovery completed", "count", len(discovered))
	return discovered, nil
}

// DiscoverUDSResources finds resources matching UDS ResourceSelector in specified namespaces
func (d *Discoverer) DiscoverUDSResources(_ context.Context, selector *udsv1alpha3.ResourceSelector, namespaces []string) ([]ResourceReference, error) {
	if selector == nil {
		return nil, nil
	}

	klog.InfoS("Discovering resources", "selector", selector, "namespaces", namespaces)

	var discovered []ResourceReference

	// TODO: For now, we'll return empty - actual resource discovery would require
	// knowledge of which resource types to look for
	// This is a placeholder that shows the structure

	klog.InfoS("Resource discovery completed", "count", len(discovered))
	return discovered, nil
}

// MatchesGlobPattern checks if a name matches a glob pattern
// Supports * (zero or more characters) and ? (exactly one character)
func MatchesGlobPattern(name, pattern string) bool {
	match, err := filepath.Match(pattern, name)
	if err != nil {
		klog.V(4).InfoS("Invalid glob pattern", "pattern", pattern, "error", err)
		return false
	}
	return match
}
