package destinations

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// S3Destination handles S3 bucket destinations
type S3Destination struct{}

// GetPublishCommand returns the aws s3 cp command
func (d *S3Destination) GetPublishCommand(pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (string, error) {
	dest := pkg.Spec.Publish.Destination.S3
	if dest == nil {
		return "", fmt.Errorf("s3 destination configuration is missing")
	}

	s3Path := fmt.Sprintf("s3://%s/%s", dest.Bucket, dest.KeyPrefix)
	return fmt.Sprintf("aws s3 cp %s %s --region %s", artifactPath, s3Path, dest.Region), nil
}

// GetJobConfiguration returns the AWS credentials env vars
func (d *S3Destination) GetJobConfiguration(pkg *zarfv1alpha1.ZarfPackageJob) (*JobConfig, error) {
	dest := pkg.Spec.Publish.Destination.S3
	if dest == nil {
		return nil, fmt.Errorf("s3 destination configuration is missing")
	}

	config := &JobConfig{}

	if dest.CredentialsSecretRef != nil {
		config.EnvFrom = append(config.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: dest.CredentialsSecretRef.Name,
				},
			},
		})
	}

	return config, nil
}
