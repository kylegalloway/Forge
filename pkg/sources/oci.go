package sources

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// OCISourceConfig contains configuration for OCI source pulls
type OCISourceConfig struct {
	Reference             string
	CredentialsSecretName string
}

// OCISource handles OCI registry sources
type OCISource struct{}

// GetInitContainer returns an init container to pull the artifact from OCI
func (source *OCISource) GetInitContainer(pkg *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error) {
	ociSource := pkg.Spec.Source.OCI
	if ociSource == nil {
		return nil, fmt.Errorf("oci source configuration is missing")
	}

	// Convert to common config
	var secretName string
	if ociSource.CredentialRef != nil { // pragma: allowlist secret
		secretName = ociSource.CredentialRef.Name // pragma: allowlist secret
	}

	config := &OCISourceConfig{
		Reference:             ociSource.Reference,
		CredentialsSecretName: secretName,
	}

	// Use common builder with Zarf UID (note: OCI originally used 65532, keeping that)
	return BuildOCIInitContainer(config, int64(constants.DefaultUDSUID))
}

// BuildOCIInitContainer creates an init container for OCI pulls
// This is shared between Zarf and UDS sources, with configurable runAsUser
func BuildOCIInitContainer(config *OCISourceConfig, runAsUser int64) (*corev1.Container, error) {
	if config == nil {
		return nil, fmt.Errorf("oci source configuration is missing")
	}

	pullCmd := fmt.Sprintf("crane export %s - | tar -xz -C %s", config.Reference, constants.VolumeMountPathWorkspace)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      constants.VolumeNameWorkspace,
			MountPath: constants.VolumeMountPathWorkspace,
		},
	}

	if config.CredentialsSecretName != "" { // pragma: allowlist secret
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "source-docker-config",
			MountPath: "/home/nonroot/.docker",
			ReadOnly:  true,
		})
	}

	return &corev1.Container{
		Name:         "oci-pull",
		Image:        "gcr.io/go-containerregistry/crane:latest",
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{pullCmd},
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
