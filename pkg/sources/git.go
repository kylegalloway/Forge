// Package sources provides implementations for different source types (git, OCI, Helm, local) used by ZarfPackageJob operations.
package sources

import (
	"fmt"

	"github.com/kylegalloway/forge/pkg/actions/common"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

// GitSourceConfig contains the common configuration for Git sources
type GitSourceConfig struct {
	URL                     string
	Ref                     string
	Path                    string
	CredentialsSecretName   string
	DisableCloneCredentials bool
}

// GitSource handles git repository sources
type GitSource struct{}

// GetInitContainer returns an init container to clone the git repository
func (source *GitSource) GetInitContainer(pkg *zarfv1alpha1.ZarfPackageJob) (*corev1.Container, error) {
	gitSource := pkg.Spec.Source.Git
	if gitSource == nil {
		return nil, fmt.Errorf("git source configuration is missing")
	}

	// Convert to common GitSourceConfig
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

	// Use common builder with Zarf UID
	return BuildGitInitContainer(config, int64(constants.DefaultZarfUID))
}

// BuildGitInitContainer creates an init container for Git cloning with the given configuration and UID
func BuildGitInitContainer(config *GitSourceConfig, runAsUser int64) (*corev1.Container, error) {
	if config == nil {
		return nil, fmt.Errorf("git source configuration is missing")
	}

	// Construct git clone command (with or without credentials)
	var cloneCmd string
	if config.DisableCloneCredentials {
		cloneCmd = fmt.Sprintf("GIT_ASKPASS='' git clone --depth 1 --branch %s %s /workspace", config.Ref, config.URL)
	} else {
		cloneCmd = fmt.Sprintf("git clone --depth 1 --branch %s %s /workspace", config.Ref, config.URL)
	}

	if config.Path != "" && config.Path != "." {
		// Move subdirectory contents to workspace root using tar to handle all files correctly
		cloneCmd = fmt.Sprintf("%s && cd /workspace/%s && tar cf - . | (cd /workspace && tar xf -) && cd /workspace && rm -rf %s", cloneCmd, config.Path, config.Path)
	}

	container := &corev1.Container{
		Name:    "git-clone",
		Image:   "alpine/git:latest",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{cloneCmd},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             common.Ptr(true),
			RunAsUser:                common.Ptr(runAsUser),
			AllowPrivilegeEscalation: common.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}

	// Handle credentials if provided  # pragma: allowlist secret
	if config.CredentialsSecretName != "" && !config.DisableCloneCredentials { // pragma: allowlist secret
		// Mount secret to /etc/git-secret  # pragma: allowlist secret
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "git-creds",
			MountPath: "/etc/git-secret",
			ReadOnly:  true,
		})

		// Setup command to configure credentials
		// #nosec G101 - This is a shell script template, not a hardcoded credential
		setupCmd := `
if [ -f /etc/git-secret/ssh-key ]; then
  mkdir -p ~/.ssh
  cp /etc/git-secret/ssh-key ~/.ssh/id_rsa
  chmod 600 ~/.ssh/id_rsa
  echo "StrictHostKeyChecking no" >> ~/.ssh/config
elif [ -f /etc/git-secret/token ]; then
  git config --global credential.helper store
  token=$(cat /etc/git-secret/token)
  echo "https://oauth2:${token}@github.com" > ~/.git-credentials
  echo "https://oauth2:${token}@gitlab.com" >> ~/.git-credentials
fi
`
		// Prepend setup to clone command
		cloneCmd = fmt.Sprintf("%s && %s", setupCmd, cloneCmd)
		container.Args = []string{cloneCmd}
	}

	return container, nil
}
