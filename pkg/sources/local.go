package sources

import (
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

// LocalSource handles local filesystem sources
type LocalSource struct{}

// GetInitContainer returns nil for local sources as they are mounted directly
func (source *LocalSource) GetInitContainer(_ *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error) {
	// Local sources don't need an init container, they are expected to be mounted
	// directly into the workspace volume by the controller/job spec.
	return nil, nil
}
