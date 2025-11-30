package destinations

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// OCIDestination handles OCI registry destinations
type OCIDestination struct{}

// GetPublishCommand returns the zarf package publish command
func (destination *OCIDestination) GetPublishCommand(pkg *zarfv1alpha1.ZarfPackageJob, artifactPath string) (string, error) {
	ociConfig := pkg.Spec.Publish.Destination.OCI
	if ociConfig == nil {
		return "", fmt.Errorf("oci destination configuration is missing")
	}

	ociRef := fmt.Sprintf("oci://%s/%s:%s",
		ociConfig.Registry,
		ociConfig.Repository,
		ociConfig.Tag,
	)
	return fmt.Sprintf("zarf package publish %s %s --confirm", artifactPath, ociRef), nil
}

// GetJobConfiguration returns the docker config volume mount
func (destination *OCIDestination) GetJobConfiguration(pkg *zarfv1alpha1.ZarfPackageJob) (*JobConfig, error) {
	ociConfig := pkg.Spec.Publish.Destination.OCI
	if ociConfig == nil {
		return nil, fmt.Errorf("oci destination configuration is missing")
	}

	config := &JobConfig{}

	if ociConfig.CredentialsSecretRef != nil {
		secretName := ociConfig.CredentialsSecretRef.Name
		config.Volumes = append(config.Volumes, corev1.Volume{
			Name: "registry-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})

		config.VolumeMounts = append(config.VolumeMounts, corev1.VolumeMount{
			Name:      "registry-creds",
			MountPath: "/home/zarf/.docker",
			ReadOnly:  true,
		})
	}

	return config, nil
}
