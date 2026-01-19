package sources

import (
	"fmt"

	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

// Source defines the interface for package sources
type Source interface {
	// GetInitContainer returns an init container to fetch the source
	GetInitContainer(pkg *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error)
}

// New returns a new Source based on the package configuration
func New(pkg *zarfv1alpha3.ZarfPackageJob) (Source, error) {
	switch pkg.Spec.Source.Type {
	case zarfv1alpha3.SourceTypeGit:
		return &GitSource{}, nil
	case zarfv1alpha3.SourceTypeS3:
		return &S3Source{}, nil
	case zarfv1alpha3.SourceTypeOCI:
		return &OCISource{}, nil
	case zarfv1alpha3.SourceTypeLocal:
		return &LocalSource{}, nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", pkg.Spec.Source.Type)
	}
}
