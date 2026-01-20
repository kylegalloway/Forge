package destinations

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// S3Destination handles S3 bucket destinations
type S3Destination struct{}

// GetPublishCommand returns the aws s3 cp command
func (destination *S3Destination) GetPublishCommand(pkg *zarfv1alpha3.ZarfPackageJob, artifactPath string) (string, error) {
	s3Config := pkg.Spec.Publish.Destination.S3
	if s3Config == nil {
		return "", fmt.Errorf("s3 destination configuration is missing")
	}

	s3Path := fmt.Sprintf("s3://%s/%s", s3Config.Bucket, s3Config.KeyPrefix)
	return fmt.Sprintf("aws s3 cp %s %s --region %s", artifactPath, s3Path, s3Config.Region), nil
}

// GetJobConfiguration returns the AWS credentials configuration for S3 destinations
// Supports three credential types:
// - EnvVar (default): Load from secret keys as environment variables
// - File: Mount secret as AWS credentials file
// - Node: Use node-level credentials (IRSA, instance profile)
func (destination *S3Destination) GetJobConfiguration(pkg *zarfv1alpha3.ZarfPackageJob) (*JobConfig, error) {
	s3Config := pkg.Spec.Publish.Destination.S3
	if s3Config == nil {
		return nil, fmt.Errorf("s3 destination configuration is missing")
	}

	config := &JobConfig{}

	if s3Config.CredentialRef != nil { // pragma: allowlist secret
		credType := s3Config.CredentialRef.Type // pragma: allowlist secret
		if credType == "" {
			// Default to EnvVar for backward compatibility
			credType = common.AWSCredentialTypeEnvVar
		}

		switch credType {
		case common.AWSCredentialTypeEnvVar:
			// Load credentials from secret as environment variables
			if s3Config.CredentialRef.Name != "" { // pragma: allowlist secret
				secretName := s3Config.CredentialRef.Name // pragma: allowlist secret
				config.Env = append(config.Env,
					corev1.EnvVar{
						Name: "AWS_ACCESS_KEY_ID",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: "access-key-id",
							},
						},
					},
					corev1.EnvVar{
						Name: "AWS_SECRET_ACCESS_KEY", // pragma: allowlist secret
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: "secret-access-key", // pragma: allowlist secret
							},
						},
					},
				)
			}

		case common.AWSCredentialTypeFile:
			// Mount credentials file from secret
			if s3Config.CredentialRef.Name != "" { // pragma: allowlist secret
				key := s3Config.CredentialRef.Key
				if key == "" {
					key = "credentials" // Default key
				}
				config.Volumes = append(config.Volumes, corev1.Volume{
					Name: constants.VolumeNameAWSCredentials,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: s3Config.CredentialRef.Name, // pragma: allowlist secret
							Items: []corev1.KeyToPath{
								{
									Key:  key,
									Path: "credentials",
								},
							},
						},
					},
				})
				config.VolumeMounts = append(config.VolumeMounts, corev1.VolumeMount{
					Name:      constants.VolumeNameAWSCredentials,
					MountPath: "/home/zarf/.aws",
					ReadOnly:  true,
				})
			}

		case common.AWSCredentialTypeNode:
			// No credentials needed - AWS SDK will use node-level credentials
			// (IRSA, instance profile, etc.)
		}
	}

	return config, nil
}
