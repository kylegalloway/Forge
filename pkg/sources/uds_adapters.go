package sources

import (
	"fmt"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// GetUDSInitContainer returns an init container for the given UDS bundle source
// This adapts UDS bundle sources to use the shared source handler logic
//
//nolint:staticcheck // SA1019: UDSPackageJob v1alpha1 must be supported until v0.10.0
func GetUDSInitContainer(bundle *udsv1alpha2.UDSPackageJob) (*corev1.Container, error) {
	switch bundle.Spec.Source.Type {
	case udsv1alpha2.SourceTypeGit:
		gitSource := bundle.Spec.Source.Git
		if gitSource == nil {
			return nil, fmt.Errorf("git source configuration is missing")
		}

		// Convert UDS GitSource to common GitSourceConfig
		var secretName string
		if gitSource.CredentialsSecretRef != nil { // pragma: allowlist secret
			secretName = gitSource.CredentialsSecretRef.Name // pragma: allowlist secret
		}

		config := &GitSourceConfig{
			URL:                     gitSource.URL,
			Ref:                     gitSource.Ref,
			Path:                    gitSource.Path,
			CredentialsSecretName:   secretName,
			DisableCloneCredentials: gitSource.DisableCloneCredentials,
		}

		// Use common builder with UDS UID
		return BuildGitInitContainer(config, int64(constants.DefaultUDSUID))

	case udsv1alpha2.SourceTypeS3:
		// TODO: Implement S3 source adapter when needed
		return nil, fmt.Errorf("S3 source type is not yet implemented for UDS bundles")

	case udsv1alpha2.SourceTypeOCI:
		// TODO: Implement OCI source adapter when needed
		return nil, fmt.Errorf("OCI source type is not yet implemented for UDS bundles")

	case udsv1alpha2.SourceTypeLocal:
		// Local sources don't need an init container - the volume is mounted directly
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported source type: %s", bundle.Spec.Source.Type)
	}
}
