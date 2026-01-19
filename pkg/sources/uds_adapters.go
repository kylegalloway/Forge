package sources

import (
	"fmt"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// GetUDSInitContainer returns an init container for the given UDS bundle source
// This adapts UDS bundle sources to use the shared source handler logic
func GetUDSInitContainer(bundle *udsv1alpha3.UDSBundleJob) (*corev1.Container, error) {
	switch bundle.Spec.Source.Type {
	case udsv1alpha3.SourceTypeGit:
		gitSource := bundle.Spec.Source.Git
		if gitSource == nil {
			return nil, fmt.Errorf("git source configuration is missing")
		}

		// Convert UDS GitSource to common GitSourceConfig
		var secretName string
		if gitSource.CredentialRef != nil { // pragma: allowlist secret
			secretName = gitSource.CredentialRef.Name // pragma: allowlist secret
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

	case udsv1alpha3.SourceTypeS3:
		s3Source := bundle.Spec.Source.S3
		if s3Source == nil {
			return nil, fmt.Errorf("s3 source configuration is missing")
		}

		// Convert UDS S3Source to common S3SourceConfig
		var secretName string
		if s3Source.CredentialRef != nil { // pragma: allowlist secret
			secretName = s3Source.CredentialRef.Name // pragma: allowlist secret
		}

		config := &S3SourceConfig{
			Bucket:                s3Source.Bucket,
			Key:                   s3Source.Key,
			Region:                s3Source.Region,
			Endpoint:              s3Source.Endpoint,
			CredentialsSecretName: secretName,
		}

		// Use common builder with UDS UID
		return BuildS3InitContainer(config, int64(constants.DefaultUDSUID))

	case udsv1alpha3.SourceTypeOCI:
		ociSource := bundle.Spec.Source.OCI
		if ociSource == nil {
			return nil, fmt.Errorf("oci source configuration is missing")
		}

		// Convert UDS OCISource to common OCISourceConfig
		var secretName string
		if ociSource.CredentialRef != nil { // pragma: allowlist secret
			secretName = ociSource.CredentialRef.Name // pragma: allowlist secret
		}

		config := &OCISourceConfig{
			Reference:             ociSource.Reference,
			CredentialsSecretName: secretName,
		}

		// Use common builder with UDS UID
		return BuildOCIInitContainer(config, int64(constants.DefaultUDSUID))

	case udsv1alpha3.SourceTypeLocal:
		// Local sources don't need an init container - the volume is mounted directly
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported source type: %s", bundle.Spec.Source.Type)
	}
}
