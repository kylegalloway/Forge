// Package sources provides implementations for different source types (git, OCI, Helm, local) used by ZarfPackageJob operations.
package sources

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
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
func (source *GitSource) GetInitContainer(pkg *zarfv1alpha3.ZarfPackageJob) (*corev1.Container, error) {
	gitSource := pkg.Spec.Source.Git
	if gitSource == nil {
		return nil, fmt.Errorf("git source configuration is missing")
	}

	// Convert to common GitSourceConfig
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
		Env: []corev1.EnvVar{
			{
				Name:  "HOME",
				Value: constants.HomePathTmp,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
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
	}

	// Handle credentials if provided  # pragma: allowlist secret
	if config.CredentialsSecretName != "" && !config.DisableCloneCredentials { // pragma: allowlist secret
		// Mount secret to /etc/git-secret  # pragma: allowlist secret
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      constants.VolumeNameGitCredentials,
			MountPath: constants.VolumeMountPathGitCredentials,
			ReadOnly:  true,
		})

		// Setup command to configure credentials
		// Extract host and scheme from URL to support any git server (GitHub, GitLab, Bitbucket, private instances, etc.)
		// Supports multiple auth modes:
		// 1. SSH key: uses ssh-key file for SSH URLs
		// 2. Token/password auth: uses username+token/password or oauth2+token for HTTPS URLs
		//
		// The script handles:
		// - URL encoding of credentials (special chars like @, :, /, %, etc.)
		// - Both 'token' and 'password' secret keys
		// - Optional username (defaults to 'oauth2' for OAuth-style tokens)
		// - Non-standard ports (included in host matching)
		//
		// #nosec G101 - This is a shell script template, not a hardcoded credential
		scheme := extractGitScheme(config.URL)
		host := extractGitHost(config.URL)

		// If host extraction failed, we can't set up credentials properly
		// The clone will proceed without stored credentials (may prompt or fail)
		if host == "" {
			host = "UNKNOWN_HOST"
		}

		// URL encoding function for shell - encodes characters that break git credential URLs
		// Must encode: % (first!), then @, :, /, space, and other special chars
		// Using awk for reliable encoding across different shell environments
		urlEncodeFunc := `urlencode() { printf '%s' "$1" | awk '
BEGIN { for(i=0;i<256;i++) ord[sprintf("%c",i)]=i }
{ n=split($0,c,""); for(i=1;i<=n;i++) {
    ch=c[i]
    if(ch ~ /[A-Za-z0-9._~-]/) printf "%s",ch
    else printf "%%%02X",ord[ch]
  }
}'
}`

		setupCmd := fmt.Sprintf(`%s
if [ -f /etc/git-secret/ssh-key ]; then
  mkdir -p ~/.ssh
  cp /etc/git-secret/ssh-key ~/.ssh/id_rsa
  chmod 600 ~/.ssh/id_rsa
  echo "StrictHostKeyChecking no" >> ~/.ssh/config
  echo "UserKnownHostsFile /dev/null" >> ~/.ssh/config
elif [ -f /etc/git-secret/token ] || [ -f /etc/git-secret/password ]; then
  git config --global credential.helper store
  # Support both 'token' and 'password' secret keys
  if [ -f /etc/git-secret/token ]; then
    cred=$(cat /etc/git-secret/token)
  else
    cred=$(cat /etc/git-secret/password)
  fi
  encoded_cred=$(urlencode "$cred")
  if [ -f /etc/git-secret/username ]; then
    raw_user=$(cat /etc/git-secret/username)
    encoded_user=$(urlencode "$raw_user")
    echo "%s://${encoded_user}:${encoded_cred}@%s" > ~/.git-credentials
  else
    echo "%s://oauth2:${encoded_cred}@%s" > ~/.git-credentials
  fi
fi`, urlEncodeFunc, scheme, host, scheme, host)
		// Prepend setup to clone command
		cloneCmd = fmt.Sprintf("%s && %s", setupCmd, cloneCmd)
		container.Args = []string{cloneCmd}
	}

	return container, nil
}

// GetGitCredentialVolume returns the volume for Git credential mounting.
// Returns nil if no credentials are needed (credRef is nil or DisableCloneCredentials is true).
func GetGitCredentialVolume(credRef *common.SecretReference, disableCloneCredentials bool) *corev1.Volume { // pragma: allowlist secret
	if credRef == nil || credRef.Name == "" || disableCloneCredentials {
		return nil
	}

	return &corev1.Volume{
		Name: constants.VolumeNameGitCredentials,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: credRef.Name,
			},
		},
	}
}

// extractGitScheme extracts the URL scheme (http or https) from a git URL.
// Returns "https" as fallback if parsing fails or for SSH URLs.
func extractGitScheme(gitURL string) string {
	// Handle SSH URLs - they don't have a scheme, default to https
	if strings.HasPrefix(gitURL, "git@") || strings.HasPrefix(gitURL, "ssh://") {
		return "https"
	}

	// Handle HTTP/HTTPS URLs
	if parsed, err := url.Parse(gitURL); err == nil && parsed.Scheme != "" {
		return parsed.Scheme
	}

	// Fallback to https
	return "https"
}

// extractGitHost extracts the hostname (including port if present) from a git URL.
// Supports multiple URL formats:
//   - HTTPS: https://github.com/user/repo, https://git.example.com:8443/repo
//   - SSH: git@github.com:user/repo, ssh://git@host:22/repo
//   - HTTP: http://gitea.local:3000/user/repo
//
// Returns empty string if parsing fails (caller should handle this case).
func extractGitHost(gitURL string) string {
	// Handle ssh:// URLs: ssh://git@host:port/path
	if strings.HasPrefix(gitURL, "ssh://") {
		if parsed, err := url.Parse(gitURL); err == nil && parsed.Host != "" {
			// parsed.Host includes port if present, but we need just the hostname
			// for SSH, the port is for SSH connection, not for credential matching
			return parsed.Hostname()
		}
	}

	// Handle SCP-style SSH URLs: git@github.com:user/repo.git
	if hostPart, found := strings.CutPrefix(gitURL, "git@"); found {
		// Extract host between @ and : (the : separates host from path in SCP syntax)
		if idx := strings.Index(hostPart, ":"); idx > 0 {
			return hostPart[:idx]
		}
	}

	// Handle HTTP/HTTPS URLs: https://github.com/user/repo.git
	// Note: parsed.Host includes port if present (e.g., "gitea.local:3000")
	// This is correct for credential matching - git matches on host:port
	if parsed, err := url.Parse(gitURL); err == nil && parsed.Host != "" {
		return parsed.Host
	}

	// Return empty string on parse failure - caller must handle this
	return ""
}
