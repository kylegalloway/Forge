// Package destinations provides implementations for different destination types (S3, OCI, local) used for publishing Zarf packages.
package destinations

import (
	"fmt"

	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
)

// LocalDestination handles local filesystem destinations
type LocalDestination struct{}

// GetPublishCommand returns the cp command
func (destination *LocalDestination) GetPublishCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string) (string, error) {
	localConfig := pkg.Spec.Publish.Destination.Local
	if localConfig == nil {
		return "", fmt.Errorf("local destination configuration is missing")
	}

	if !localConfig.DevMode {
		return "", fmt.Errorf("local destination requires devMode=true")
	}

	return fmt.Sprintf("cp %s %s", artifactPath, localConfig.Path), nil
}

// GetJobConfiguration returns nil for local destinations
func (destination *LocalDestination) GetJobConfiguration(_ *zarfv1alpha3.ZarfPackageJob) (*JobConfig, error) {
	return &JobConfig{}, nil
}
