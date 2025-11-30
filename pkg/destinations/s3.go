package destinations

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// S3Destination handles S3 bucket destinations
type S3Destination struct{}

// GetPublishCommand returns the aws s3 cp command
func (destination *S3Destination) GetPublishCommand(pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (string, error) {
	s3Config := pkg.Spec.Publish.Destination.S3
	if s3Config == nil {
		return "", fmt.Errorf("s3 destination configuration is missing")
	}

	s3Path := fmt.Sprintf("s3://%s/%s", s3Config.Bucket, s3Config.KeyPrefix)
	return fmt.Sprintf("aws s3 cp %s %s --region %s", artifactPath, s3Path, s3Config.Region), nil
}

// GetJobConfiguration returns the AWS credentials env vars
func (destination *S3Destination) GetJobConfiguration(pkg *zarfv1alpha1.ZarfPackageJob) (*JobConfig, error) {
	s3Config := pkg.Spec.Publish.Destination.S3
	if s3Config == nil {
		return nil, fmt.Errorf("s3 destination configuration is missing")
	}

	config := &JobConfig{}

	if s3Config.CredentialsSecretRef != nil {
		config.EnvFrom = append(config.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: s3Config.CredentialsSecretRef.Name,
				},
			},
		})
	}

	return config, nil
}
