package sources

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// S3SourceConfig contains configuration for S3 source downloads
type S3SourceConfig struct {
	Bucket                string
	Key                   string
	Region                string
	Endpoint              string
	CredentialsSecretName string
}

// S3Source handles S3 bucket sources
type S3Source struct{}

// GetInitContainer returns an init container to download the artifact from S3
func (source *S3Source) GetInitContainer(pkg *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error) {
	s3Source := pkg.Spec.Source.S3
	if s3Source == nil {
		return nil, fmt.Errorf("s3 source configuration is missing")
	}

	// Convert to common config
	var secretName string
	if s3Source.CredentialRef != nil { // pragma: allowlist secret
		secretName = s3Source.CredentialRef.Name // pragma: allowlist secret
	}

	config := &S3SourceConfig{
		Bucket:                s3Source.Bucket,
		Key:                   s3Source.Key,
		Region:                s3Source.Region,
		Endpoint:              "", // Zarf doesn't support custom endpoints
		CredentialsSecretName: secretName,
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

	env := []corev1.EnvVar{}
	if config.CredentialsSecretName != "" { // pragma: allowlist secret
		env = append(env,
			corev1.EnvVar{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: config.CredentialsSecretName,
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
							Name: config.CredentialsSecretName,
						},
						Key: "secret-access-key",
					},
				},
			},
		)
	}

	return &corev1.Container{
		Name:    "s3-download",
		Image:   "amazon/aws-cli:latest",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{downloadCmd},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      constants.VolumeNameWorkspace,
				MountPath: constants.VolumeMountPathWorkspace,
			},
		},
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
