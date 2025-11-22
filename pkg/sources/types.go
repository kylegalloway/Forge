package sources

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Source defines the interface for package sources
type Source interface {
	// GetInitContainer returns an init container to fetch the source
	GetInitContainer(pkg *zarfv1alpha1.ZarfPackageJob) (*corev1.Container, error)
}

// New returns a new Source based on the package configuration
func New(pkg *zarfv1alpha1.ZarfPackageJob) (Source, error) {
	switch pkg.Spec.Source.Type {
	case zarfv1alpha1.SourceTypeGit:
		return &GitSource{}, nil
	case zarfv1alpha1.SourceTypeS3:
		return &S3Source{}, nil
	case zarfv1alpha1.SourceTypeOCI:
		return &OCISource{}, nil
	case zarfv1alpha1.SourceTypeLocal:
		return &LocalSource{}, nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", pkg.Spec.Source.Type)
	}
}

func ptr[T any](v T) *T {
	return &v
}
