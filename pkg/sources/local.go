package sources

import (
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// LocalSource handles local filesystem sources
type LocalSource struct{}

// GetInitContainer returns nil for local sources as they are mounted directly
func (s *LocalSource) GetInitContainer(pkg *zarfv1alpha1.ZarfPackage) (*corev1.Container, error) {
	// Local sources don't need an init container, they are expected to be mounted
	// directly into the workspace volume by the controller/job spec.
	return nil, nil
}
