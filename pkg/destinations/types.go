package destinations

import (
	"fmt"

	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

// JobConfig holds configuration for the Kubernetes Job
type JobConfig struct {
	Volumes      []corev1.Volume
	VolumeMounts []corev1.VolumeMount
	Env          []corev1.EnvVar
	EnvFrom      []corev1.EnvFromSource
}

// Destination defines the interface for package destinations
type Destination interface {
	// GetPublishCommand returns the command to publish the artifact
	GetPublishCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string) (string, error)
	// GetJobConfiguration returns the job configuration (volumes, envs) needed for publishing
	GetJobConfiguration(pkg *zarfv1alpha3.ZarfPackageJob) (*JobConfig, error)
}

// New returns a new Destination based on the package configuration
func New(pkg *zarfv1alpha3.ZarfPackageJob) (Destination, error) {
	if pkg.Spec.Publish == nil {
		return nil, fmt.Errorf("publish configuration is missing")
	}

	switch pkg.Spec.Publish.Destination.Type {
	case zarfv1alpha3.DestinationTypeS3:
		return &S3Destination{}, nil
	case zarfv1alpha3.DestinationTypeOCI:
		return &OCIDestination{}, nil
	case zarfv1alpha3.DestinationTypeLocal:
		return &LocalDestination{}, nil
	default:
		return nil, fmt.Errorf("unsupported destination type: %s", pkg.Spec.Publish.Destination.Type)
	}
}
