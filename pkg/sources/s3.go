package sources

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// S3Source handles S3 bucket sources
type S3Source struct{}

// GetInitContainer returns an init container to download the artifact from S3
func (s *S3Source) GetInitContainer(pkg *zarfv1alpha1.ZarfPackage) (*corev1.Container, error) {
	s3Source := pkg.Spec.Source.S3
	if s3Source == nil {
		return nil, fmt.Errorf("s3 source configuration is missing")
	}

	s3Path := fmt.Sprintf("s3://%s/%s", s3Source.Bucket, s3Source.Key)
	downloadCmd := fmt.Sprintf("aws s3 cp %s /workspace/package.tar.zst --region %s", s3Path, s3Source.Region)

	env := []corev1.EnvVar{}
	if s3Source.CredentialsSecretRef != nil {
		env = append(env,
			corev1.EnvVar{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: s3Source.CredentialsSecretRef.Name,
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
							Name: s3Source.CredentialsSecretRef.Name,
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
				Name:      "workspace",
				MountPath: "/workspace",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr(true),
			RunAsUser:                ptr(int64(1000)),
			AllowPrivilegeEscalation: ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}, nil
}
