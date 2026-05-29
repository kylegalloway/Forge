package sources

import (
	"fmt"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// SourceParams is the normalised, CRD-agnostic description of a package source.
// Both ZarfPackageJob and UDSBundleJob collapse into this struct before any
// init-container is built; the Build* helpers carry no knowledge of either CRD.
type SourceParams struct {
	Type    string
	Git     *GitSourceConfig
	S3      *S3SourceConfig
	OCI     *OCISourceConfig
	UID     int64
	S3Image string
}

// GetInitContainer is the generic factory entry point used by all action handlers.
// It replaces the old CRD-specific paths (GetUDSInitContainer, sources.New + GetInitContainer)
// with a single dispatch that both Zarf and UDS go through.
func GetInitContainer(p SourceParams) (*corev1.Container, error) {
	switch p.Type {
	case string(zarfv1alpha3.SourceTypeGit):
		return BuildGitInitContainer(p.Git, p.UID)
	case string(zarfv1alpha3.SourceTypeS3):
		return BuildS3InitContainer(p.S3, p.UID, p.S3Image)
	case string(zarfv1alpha3.SourceTypeOCI):
		return BuildOCIInitContainer(p.OCI, p.UID)
	case string(zarfv1alpha3.SourceTypeLocal):
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", p.Type)
	}
}

// SourceParamsFromZarf translates a ZarfPackageJob source spec into SourceParams.
func SourceParamsFromZarf(pkg *zarfv1alpha3.ZarfPackageJob) (SourceParams, error) {
	p := SourceParams{
		Type:    string(pkg.Spec.Source.Type),
		UID:     int64(constants.DefaultZarfUID),
		S3Image: constants.ZarfCLIImage,
	}

	switch pkg.Spec.Source.Type {
	case zarfv1alpha3.SourceTypeGit:
		git := pkg.Spec.Source.Git
		if git == nil {
			return p, fmt.Errorf("git source configuration is missing")
		}
		var secretName string
		if git.CredentialRef != nil { // pragma: allowlist secret
			secretName = git.CredentialRef.Name // pragma: allowlist secret
		}
		p.Git = &GitSourceConfig{
			URL:                     git.URL,
			Ref:                     git.Ref,
			Path:                    git.Path,
			CredentialsSecretName:   secretName,
			DisableCloneCredentials: git.DisableCloneCredentials,
		}

	case zarfv1alpha3.SourceTypeS3:
		s3 := pkg.Spec.Source.S3
		if s3 == nil {
			return p, fmt.Errorf("s3 source configuration is missing")
		}
		p.S3 = &S3SourceConfig{
			Bucket:        s3.Bucket,
			Key:           s3.Key,
			Region:        s3.Region,
			Endpoint:      s3.Endpoint,
			CredentialRef: s3.CredentialRef, // pragma: allowlist secret
		}

	case zarfv1alpha3.SourceTypeOCI:
		oci := pkg.Spec.Source.OCI
		if oci == nil {
			return p, fmt.Errorf("oci source configuration is missing")
		}
		var secretName string
		if oci.CredentialRef != nil { // pragma: allowlist secret
			secretName = oci.CredentialRef.Name // pragma: allowlist secret
		}
		p.OCI = &OCISourceConfig{
			Reference:             oci.Reference,
			CredentialsSecretName: secretName,
		}

	case zarfv1alpha3.SourceTypeLocal:
		// No sub-config needed; GetInitContainer returns nil for local.

	default:
		return p, fmt.Errorf("unsupported source type: %s", pkg.Spec.Source.Type)
	}

	return p, nil
}

// SourceParamsFromUDS translates a UDSBundleJob source spec into SourceParams.
func SourceParamsFromUDS(bundle *udsv1alpha3.UDSBundleJob) (SourceParams, error) {
	p := SourceParams{
		Type:    string(bundle.Spec.Source.Type),
		UID:     int64(constants.DefaultUDSUID),
		S3Image: constants.UDSCLIImage,
	}

	switch bundle.Spec.Source.Type {
	case udsv1alpha3.SourceTypeGit:
		git := bundle.Spec.Source.Git
		if git == nil {
			return p, fmt.Errorf("git source configuration is missing")
		}
		var secretName string
		if git.CredentialRef != nil { // pragma: allowlist secret
			secretName = git.CredentialRef.Name // pragma: allowlist secret
		}
		p.Git = &GitSourceConfig{
			URL:                     git.URL,
			Ref:                     git.Ref,
			Path:                    git.Path,
			CredentialsSecretName:   secretName,
			DisableCloneCredentials: git.DisableCloneCredentials,
		}

	case udsv1alpha3.SourceTypeS3:
		s3 := bundle.Spec.Source.S3
		if s3 == nil {
			return p, fmt.Errorf("s3 source configuration is missing")
		}
		p.S3 = &S3SourceConfig{
			Bucket:        s3.Bucket,
			Key:           s3.Key,
			Region:        s3.Region,
			Endpoint:      s3.Endpoint,
			CredentialRef: s3.CredentialRef, // pragma: allowlist secret
		}

	case udsv1alpha3.SourceTypeOCI:
		oci := bundle.Spec.Source.OCI
		if oci == nil {
			return p, fmt.Errorf("oci source configuration is missing")
		}
		var secretName string
		if oci.CredentialRef != nil { // pragma: allowlist secret
			secretName = oci.CredentialRef.Name // pragma: allowlist secret
		}
		p.OCI = &OCISourceConfig{
			Reference:             oci.Reference,
			CredentialsSecretName: secretName,
		}

	case udsv1alpha3.SourceTypeLocal:
		// No sub-config needed; GetInitContainer returns nil for local.

	default:
		return p, fmt.Errorf("unsupported source type: %s", bundle.Spec.Source.Type)
	}

	return p, nil
}
