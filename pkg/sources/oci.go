package sources

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions/common"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// OCISource handles OCI registry sources
type OCISource struct{}

// GetInitContainer returns an init container to pull the artifact from OCI
func (source *OCISource) GetInitContainer(pkg *zarfv1alpha1.ZarfPackageJob) (*corev1.Container, error) {
	ociSource := pkg.Spec.Source.OCI
	if ociSource == nil {
		return nil, fmt.Errorf("oci source configuration is missing")
	}

	pullCmd := fmt.Sprintf("crane export %s - | tar -xz -C /workspace", ociSource.Image)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "workspace",
			MountPath: "/workspace",
		},
	}

	if ociSource.CredentialsSecretRef != nil { // pragma: allowlist secret
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
			RunAsNonRoot:             common.Ptr(true),
			RunAsUser:                common.Ptr(int64(65532)),
			AllowPrivilegeEscalation: common.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}, nil
}
