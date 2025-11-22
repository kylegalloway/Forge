// Package destinations provides implementations for different destination types (S3, OCI, local) used for publishing Zarf packages.
package destinations

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
)

// LocalDestination handles local filesystem destinations
type LocalDestination struct{}

// GetPublishCommand returns the cp command
func (d *LocalDestination) GetPublishCommand(pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (string, error) {
	dest := pkg.Spec.Publish.Destination.Local
	if dest == nil {
		return "", fmt.Errorf("local destination configuration is missing")
	}

	if !dest.DevMode {
		return "", fmt.Errorf("local destination requires devMode=true")
	}

	return fmt.Sprintf("cp %s %s", artifactPath, dest.Path), nil
}

// GetJobConfiguration returns nil for local destinations
func (d *LocalDestination) GetJobConfiguration(_ *zarfv1alpha1.ZarfPackageJob) (*JobConfig, error) {
	return &JobConfig{}, nil
}
