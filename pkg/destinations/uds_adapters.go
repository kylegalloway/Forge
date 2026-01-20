package destinations

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// GetUDSPublishCommand returns the command to publish a UDS bundle
// This adapts UDS bundle destinations to use consistent command generation
func GetUDSPublishCommand(bundle *udsv1alpha3.UDSBundleJob, artifactPath string) (string, error) {
	if bundle.Spec.Publish == nil {
		return "", fmt.Errorf("publish configuration is missing")
	}

	dest := bundle.Spec.Publish.Destination

	switch dest.Type {
	case udsv1alpha3.DestinationTypeOCI:
		if dest.OCI == nil {
			return "", fmt.Errorf("OCI destination configuration is required")
		}
		ociRef := fmt.Sprintf("oci://%s/%s:%s", dest.OCI.Registry, dest.OCI.Repository, dest.OCI.Tag)
		return fmt.Sprintf("uds publish %s %s --confirm", artifactPath, ociRef), nil

	case udsv1alpha3.DestinationTypeS3:
		if dest.S3 == nil {
			return "", fmt.Errorf("S3 destination configuration is required")
		}
		// For S3, we use AWS CLI to upload the bundle
		s3Path := fmt.Sprintf("s3://%s/%s", dest.S3.Bucket, dest.S3.KeyPrefix)
		return fmt.Sprintf("aws s3 cp %s %s", artifactPath, s3Path), nil

	case udsv1alpha3.DestinationTypeLocal:
		// Local destination - just echo success for dev/testing
		return fmt.Sprintf("echo 'Bundle artifact stored locally at %s'", artifactPath), nil

	default:
		return "", fmt.Errorf("unsupported destination type: %s", dest.Type)
	}
}

// GetUDSJobConfiguration returns the job configuration (volumes, env vars) for UDS publish
// This adapts UDS bundle destinations to use consistent job configuration
func GetUDSJobConfiguration(bundle *udsv1alpha3.UDSBundleJob) (*JobConfig, error) {
	if bundle.Spec.Publish == nil {
		return nil, fmt.Errorf("publish configuration is missing")
	}

	dest := bundle.Spec.Publish.Destination
	config := &JobConfig{}

	switch dest.Type {
	case udsv1alpha3.DestinationTypeOCI:
		if dest.OCI != nil && dest.OCI.CredentialRef != nil {
			secretName := dest.OCI.CredentialRef.Name // pragma: allowlist secret
			config.Volumes = append(config.Volumes, corev1.Volume{
				Name: "docker-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
						SecretName: secretName,
						Items: []corev1.KeyToPath{
							{
								Key:  ".dockerconfigjson",
								Path: "config.json",
							},
						},
					},
				},
			})

			config.VolumeMounts = append(config.VolumeMounts, corev1.VolumeMount{
				Name:      "docker-config",
				MountPath: "/.docker",
				ReadOnly:  true,
			})

			config.Env = append(config.Env, corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: "/.docker",
			})
		}

	case udsv1alpha3.DestinationTypeS3:
		if dest.S3 != nil {
			if dest.S3.Region != "" {
				config.Env = append(config.Env, corev1.EnvVar{
					Name:  "AWS_REGION",
					Value: dest.S3.Region,
				})
			}
			if dest.S3.Endpoint != "" {
				config.Env = append(config.Env, corev1.EnvVar{
					Name:  "AWS_ENDPOINT_URL",
					Value: dest.S3.Endpoint,
				})
			}

			// Add AWS credentials based on type
			if dest.S3.CredentialRef != nil { // pragma: allowlist secret
				credType := dest.S3.CredentialRef.Type // pragma: allowlist secret
				if credType == "" {
					// Default to EnvVar for backward compatibility
					credType = common.AWSCredentialTypeEnvVar
				}

				switch credType {
				case common.AWSCredentialTypeEnvVar:
					// Load credentials from secret as environment variables
					if dest.S3.CredentialRef.Name != "" { // pragma: allowlist secret
						config.Env = append(config.Env,
							corev1.EnvVar{
								Name: "AWS_ACCESS_KEY_ID",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
										LocalObjectReference: corev1.LocalObjectReference{
											Name: dest.S3.CredentialRef.Name, // pragma: allowlist secret
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
											Name: dest.S3.CredentialRef.Name, // pragma: allowlist secret
										},
										Key: "secret-access-key", // pragma: allowlist secret
									},
								},
							},
						)
					}

				case common.AWSCredentialTypeFile:
					// Mount credentials file from secret
					if dest.S3.CredentialRef.Name != "" { // pragma: allowlist secret
						key := dest.S3.CredentialRef.Key
						if key == "" {
							key = "credentials" // Default key
						}
						config.Volumes = append(config.Volumes, corev1.Volume{
							Name: constants.VolumeNameAWSCredentials,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: dest.S3.CredentialRef.Name, // pragma: allowlist secret
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
							MountPath: "/home/nonroot/.aws",
							ReadOnly:  true,
						})
					}

				case common.AWSCredentialTypeNode:
					// No credentials needed - AWS SDK will use node-level credentials
					// (IRSA, instance profile, etc.)
				}
			}
		}

	case udsv1alpha3.DestinationTypeLocal:
		// No special configuration needed for local destinations
	}

	return config, nil
}
