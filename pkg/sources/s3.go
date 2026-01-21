package sources

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// S3SourceConfig contains configuration for S3 source downloads
type S3SourceConfig struct {
	Bucket        string
	Key           string
	Region        string
	Endpoint      string
	CredentialRef *common.AWSCredentialRef // pragma: allowlist secret
}

// S3Source handles S3 bucket sources
type S3Source struct{}

// GetInitContainer returns an init container to download the artifact from S3
func (source *S3Source) GetInitContainer(pkg *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error) {
	s3Source := pkg.Spec.Source.S3
	if s3Source == nil {
		return nil, fmt.Errorf("s3 source configuration is missing")
	}

	config := &S3SourceConfig{
		Bucket:        s3Source.Bucket,
		Key:           s3Source.Key,
		Region:        s3Source.Region,
		Endpoint:      s3Source.Endpoint,
		CredentialRef: s3Source.CredentialRef, // pragma: allowlist secret
	}

	// Use common builder with Zarf UID
	return BuildS3InitContainer(config, int64(constants.DefaultZarfUID))
}

// BuildS3InitContainer creates an init container for S3 downloads
// This is shared between Zarf and UDS sources, with configurable runAsUser
func BuildS3InitContainer(config *S3SourceConfig, runAsUser int64) (*corev1.Container, error) {
	if config == nil {
		return nil, fmt.Errorf("s3 source configuration is missing")
	}

	s3Path := fmt.Sprintf("s3://%s/%s", config.Bucket, config.Key)

	// Build download command
	var downloadCmd string
	if config.Endpoint != "" {
		// S3-compatible storage (MinIO, etc.)
		downloadCmd = fmt.Sprintf("aws s3 cp %s %s/package.tar.zst --endpoint-url %s --region %s",
			s3Path, constants.VolumeMountPathWorkspace, config.Endpoint, config.Region)
	} else {
		// Standard AWS S3
		downloadCmd = fmt.Sprintf("aws s3 cp %s %s/package.tar.zst --region %s",
			s3Path, constants.VolumeMountPathWorkspace, config.Region)
	}

	env := []corev1.EnvVar{
		{
			Name:  "HOME",
			Value: constants.HomePathTmp,
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      constants.VolumeNameWorkspace,
			MountPath: constants.VolumeMountPathWorkspace,
		},
	}

	// Configure credentials based on type
	if config.CredentialRef != nil { // pragma: allowlist secret
		credType := config.CredentialRef.Type // pragma: allowlist secret
		if credType == "" {
			// Default to EnvVar for backward compatibility
			credType = common.AWSCredentialTypeEnvVar
		}

		switch credType {
		case common.AWSCredentialTypeEnvVar:
			// Load credentials from secret as environment variables
			if config.CredentialRef.Name != "" { // pragma: allowlist secret
				env = append(env,
					corev1.EnvVar{
						Name: "AWS_ACCESS_KEY_ID",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: config.CredentialRef.Name, // pragma: allowlist secret
								},
								Key: "access-key-id",
							},
						},
					},
					corev1.EnvVar{
						Name: "AWS_SECRET_ACCESS_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: config.CredentialRef.Name, // pragma: allowlist secret
								},
								Key: "secret-access-key",
							},
						},
					},
				)
			}

		case common.AWSCredentialTypeFile:
			// Mount credentials file from secret
			if config.CredentialRef.Name != "" { // pragma: allowlist secret
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      constants.VolumeNameAWSCredentials,
					MountPath: constants.HomePathTmp + "/.aws",
					ReadOnly:  true,
				})
				// Note: The volume must be added by the job builder using
				// WithAWSCredentialsVolume(secretName, key)
			}

		case common.AWSCredentialTypeNode:
			// No credentials needed - AWS SDK will use node-level credentials
			// (IRSA, instance profile, etc.)
		}
	}

	return &corev1.Container{
		Name:         "s3-download",
		Image:        "amazon/aws-cli:latest",
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{downloadCmd},
		Env:          env,
		VolumeMounts: volumeMounts,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             actions.Ptr(true),
			RunAsUser:                actions.Ptr(runAsUser),
			AllowPrivilegeEscalation: actions.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}, nil
}

// GetS3CredentialVolume returns the volume for S3 credential file mounting
// Returns nil if no file-based credentials are needed
func GetS3CredentialVolume(credRef *common.AWSCredentialRef) *corev1.Volume { // pragma: allowlist secret
	if credRef == nil || credRef.Type != common.AWSCredentialTypeFile || credRef.Name == "" {
		return nil
	}

	key := credRef.Key
	if key == "" {
		key = "credentials"
	}

	return &corev1.Volume{
		Name: constants.VolumeNameAWSCredentials,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: credRef.Name,
				Items: []corev1.KeyToPath{
					{
						Key:  key,
						Path: "credentials",
					},
				},
			},
		},
	}
}
