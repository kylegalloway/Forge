package destinations

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// DestinationParams is the normalised, CRD-agnostic description of a publish destination.
// Both ZarfPackageJob and UDSBundleJob collapse into this struct before any command
// or job-config is built; the Get* helpers carry no knowledge of either CRD.
type DestinationParams struct {
	Type  string
	OCI   *OCIDestParams
	S3    *S3DestParams
	Local *LocalDestParams
}

// OCIDestParams holds OCI-specific destination config plus the per-CLI knobs that differ
// between the Zarf and UDS publish workflows.
type OCIDestParams struct {
	Registry   string
	Repository string
	Tag        string
	CredSecret string // secret name; empty = no credentials

	// CLI-specific
	PublishVerb        string             // e.g. "zarf package publish" or "uds publish"
	VolumeName         string             // e.g. "registry-creds" or "docker-config"
	MountPath          string             // e.g. "/home/zarf/.docker" or "/.docker"
	SecretItems        []corev1.KeyToPath // nil for Zarf; [{".dockerconfigjson","config.json"}] for UDS
	SetDockerConfigEnv bool               // true for UDS (sets DOCKER_CONFIG env var)
}

// S3DestParams holds S3-specific destination config plus per-CLI knobs.
type S3DestParams struct {
	Bucket        string
	KeyPrefix     string
	Region        string
	Endpoint      string
	CredentialRef *common.AWSCredentialRef // pragma: allowlist secret

	// CLI-specific
	HomeDir        string // used as the mount base for file-based AWS creds
	SetRegionEnv   bool   // true for UDS: injects AWS_REGION into job env
	SetEndpointEnv bool   // true for UDS: injects AWS_ENDPOINT_URL into job env
}

// LocalDestParams holds local-destination config.
type LocalDestParams struct {
	Path    string
	DevMode bool
	// Command is the full publish command template (%s = artifactPath).
	// Zarf uses "cp %s <path>"; UDS uses "echo 'Bundle artifact stored locally at %s'".
	CommandFn func(artifactPath string) string
}

// GetPublishCommand is the generic publish-command factory used by all action handlers.
func GetPublishCommand(p DestinationParams, artifactPath string) (string, error) {
	switch p.Type {
	case string(zarfv1alpha3.DestinationTypeOCI):
		if p.OCI == nil {
			return "", fmt.Errorf("OCI destination configuration is required")
		}
		ociRef := fmt.Sprintf("oci://%s/%s:%s", p.OCI.Registry, p.OCI.Repository, p.OCI.Tag)
		return fmt.Sprintf("%s %s %s --confirm", p.OCI.PublishVerb, artifactPath, ociRef), nil

	case string(zarfv1alpha3.DestinationTypeS3):
		if p.S3 == nil {
			return "", fmt.Errorf("S3 destination configuration is required")
		}
		s3Path := fmt.Sprintf("s3://%s/%s", p.S3.Bucket, p.S3.KeyPrefix)
		cmd := fmt.Sprintf("aws s3 cp %s %s", artifactPath, s3Path)
		if p.S3.Endpoint != "" {
			cmd += fmt.Sprintf(" --endpoint-url %s", p.S3.Endpoint)
		}
		if p.S3.Region != "" {
			cmd += fmt.Sprintf(" --region %s", p.S3.Region)
		}
		return cmd, nil

	case string(zarfv1alpha3.DestinationTypeLocal):
		if p.Local == nil {
			return "", fmt.Errorf("local destination configuration is required")
		}
		return p.Local.CommandFn(artifactPath), nil

	default:
		return "", fmt.Errorf("unsupported destination type: %s", p.Type)
	}
}

// GetJobConfiguration is the generic job-config factory used by all action handlers.
func GetJobConfiguration(p DestinationParams) (*JobConfig, error) {
	config := &JobConfig{}

	switch p.Type {
	case string(zarfv1alpha3.DestinationTypeOCI):
		if p.OCI == nil || p.OCI.CredSecret == "" {
			return config, nil
		}
		vol := corev1.Volume{
			Name: p.OCI.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
					SecretName: p.OCI.CredSecret,
				},
			},
		}
		if len(p.OCI.SecretItems) > 0 {
			vol.Secret.Items = p.OCI.SecretItems
		}
		config.Volumes = append(config.Volumes, vol)
		config.VolumeMounts = append(config.VolumeMounts, corev1.VolumeMount{
			Name:      p.OCI.VolumeName,
			MountPath: p.OCI.MountPath,
			ReadOnly:  true,
		})
		if p.OCI.SetDockerConfigEnv {
			config.Env = append(config.Env, corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: p.OCI.MountPath,
			})
		}

	case string(zarfv1alpha3.DestinationTypeS3):
		if p.S3 == nil {
			return config, nil
		}
		if p.S3.SetRegionEnv && p.S3.Region != "" {
			config.Env = append(config.Env, corev1.EnvVar{
				Name:  "AWS_REGION",
				Value: p.S3.Region,
			})
		}
		if p.S3.SetEndpointEnv && p.S3.Endpoint != "" {
			config.Env = append(config.Env, corev1.EnvVar{
				Name:  "AWS_ENDPOINT_URL",
				Value: p.S3.Endpoint,
			})
		}
		if p.S3.CredentialRef != nil { // pragma: allowlist secret
			config = applyS3Credentials(config, p.S3.CredentialRef, p.S3.HomeDir) // pragma: allowlist secret
		}

	case string(zarfv1alpha3.DestinationTypeLocal):
		// No special job configuration needed for local destinations.

	default:
		return nil, fmt.Errorf("unsupported destination type: %s", p.Type)
	}

	return config, nil
}

// applyS3Credentials appends AWS credential env vars or a mounted credential file
// to the provided JobConfig, depending on the credential type.
func applyS3Credentials(config *JobConfig, credRef *common.AWSCredentialRef, homeDir string) *JobConfig { // pragma: allowlist secret
	credType := credRef.Type // pragma: allowlist secret
	if credType == "" {
		credType = common.AWSCredentialTypeEnvVar
	}

	switch credType {
	case common.AWSCredentialTypeEnvVar:
		if credRef.Name != "" { // pragma: allowlist secret
			config.Env = append(config.Env,
				corev1.EnvVar{
					Name: "AWS_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{ // pragma: allowlist secret
							LocalObjectReference: corev1.LocalObjectReference{
								Name: credRef.Name, // pragma: allowlist secret
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
								Name: credRef.Name, // pragma: allowlist secret
							},
							Key: "secret-access-key", // pragma: allowlist secret
						},
					},
				},
			)
		}

	case common.AWSCredentialTypeFile:
		if credRef.Name != "" { // pragma: allowlist secret
			key := credRef.Key
			if key == "" {
				key = "credentials"
			}
			config.Volumes = append(config.Volumes, corev1.Volume{
				Name: constants.VolumeNameAWSCredentials,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: credRef.Name, // pragma: allowlist secret
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
				MountPath: homeDir + "/.aws",
				ReadOnly:  true,
			})
		}

	case common.AWSCredentialTypeNode:
		// No credentials needed - AWS SDK will use node-level credentials.
	}

	return config
}

// DestinationParamsFromZarf translates a ZarfPackageJob destination spec into DestinationParams.
func DestinationParamsFromZarf(pkg *zarfv1alpha3.ZarfPackageJob) (DestinationParams, error) {
	if pkg.Spec.Publish == nil {
		return DestinationParams{}, fmt.Errorf("publish configuration is missing")
	}
	dest := pkg.Spec.Publish.Destination
	p := DestinationParams{Type: string(dest.Type)}

	switch dest.Type {
	case zarfv1alpha3.DestinationTypeOCI:
		if dest.OCI == nil {
			return p, fmt.Errorf("OCI destination configuration is required")
		}
		credSecret := ""
		if dest.OCI.CredentialRef != nil {
			credSecret = dest.OCI.CredentialRef.Name // pragma: allowlist secret
		}
		p.OCI = &OCIDestParams{
			Registry:    dest.OCI.Registry,
			Repository:  dest.OCI.Repository,
			Tag:         dest.OCI.Tag,
			CredSecret:  credSecret,
			PublishVerb: "zarf package publish",
			VolumeName:  "registry-creds",
			MountPath:   constants.VolumeMountPathDockerConfig,
			// Zarf: mount raw secret, no SecretItems mapping, no DOCKER_CONFIG env
		}

	case zarfv1alpha3.DestinationTypeS3:
		if dest.S3 == nil {
			return p, fmt.Errorf("S3 destination configuration is required")
		}
		p.S3 = &S3DestParams{
			Bucket:        dest.S3.Bucket,
			KeyPrefix:     dest.S3.KeyPrefix,
			Region:        dest.S3.Region,
			Endpoint:      dest.S3.Endpoint,
			CredentialRef: dest.S3.CredentialRef, // pragma: allowlist secret
			HomeDir:       constants.HomePathZarf,
			// Zarf: region/endpoint go into the command, not env vars
		}

	case zarfv1alpha3.DestinationTypeLocal:
		if dest.Local == nil {
			return p, fmt.Errorf("local destination configuration is required")
		}
		localPath := dest.Local.Path
		devMode := dest.Local.DevMode
		p.Local = &LocalDestParams{
			Path:    localPath,
			DevMode: devMode,
			CommandFn: func(artifactPath string) string {
				if !devMode {
					return ""
				}
				return fmt.Sprintf("cp %s %s", artifactPath, localPath)
			},
		}

	default:
		return p, fmt.Errorf("unsupported destination type: %s", dest.Type)
	}

	return p, nil
}

// DestinationParamsFromUDS translates a UDSBundleJob destination spec into DestinationParams.
func DestinationParamsFromUDS(bundle *udsv1alpha3.UDSBundleJob) (DestinationParams, error) {
	if bundle.Spec.Publish == nil {
		return DestinationParams{}, fmt.Errorf("publish configuration is missing")
	}
	dest := bundle.Spec.Publish.Destination
	p := DestinationParams{Type: string(dest.Type)}

	switch dest.Type {
	case udsv1alpha3.DestinationTypeOCI:
		if dest.OCI == nil {
			return p, fmt.Errorf("OCI destination configuration is required")
		}
		credSecret := ""
		if dest.OCI.CredentialRef != nil {
			credSecret = dest.OCI.CredentialRef.Name // pragma: allowlist secret
		}
		p.OCI = &OCIDestParams{
			Registry:    dest.OCI.Registry,
			Repository:  dest.OCI.Repository,
			Tag:         dest.OCI.Tag,
			CredSecret:  credSecret,
			PublishVerb: "uds publish",
			VolumeName:  "docker-config",
			MountPath:   "/.docker",
			SecretItems: []corev1.KeyToPath{
				{Key: ".dockerconfigjson", Path: "config.json"},
			},
			SetDockerConfigEnv: true,
		}

	case udsv1alpha3.DestinationTypeS3:
		if dest.S3 == nil {
			return p, fmt.Errorf("S3 destination configuration is required")
		}
		p.S3 = &S3DestParams{
			Bucket:         dest.S3.Bucket,
			KeyPrefix:      dest.S3.KeyPrefix,
			Region:         dest.S3.Region,
			Endpoint:       dest.S3.Endpoint,
			CredentialRef:  dest.S3.CredentialRef, // pragma: allowlist secret
			HomeDir:        constants.HomePathUDS,
			SetRegionEnv:   true,
			SetEndpointEnv: true,
		}

	case udsv1alpha3.DestinationTypeLocal:
		if dest.Local == nil {
			return p, fmt.Errorf("local destination configuration is required")
		}
		p.Local = &LocalDestParams{
			Path:    dest.Local.Path,
			DevMode: dest.Local.DevMode,
			CommandFn: func(artifactPath string) string {
				return fmt.Sprintf("echo 'Bundle artifact stored locally at %s'", artifactPath)
			},
		}

	default:
		return p, fmt.Errorf("unsupported destination type: %s", dest.Type)
	}

	return p, nil
}
