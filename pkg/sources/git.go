package sources

import (
	"fmt"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// GitSource handles git repository sources
type GitSource struct{}

// GetInitContainer returns an init container to clone the git repository
func (s *GitSource) GetInitContainer(pkg *zarfv1alpha1.ZarfPackage) (*corev1.Container, error) {
	gitSource := pkg.Spec.Source.Git
	if gitSource == nil {
		return nil, fmt.Errorf("git source configuration is missing")
	}

	cloneCmd := fmt.Sprintf("git clone --depth 1 --branch %s %s /workspace", gitSource.Ref, gitSource.URL)
	if gitSource.Path != "" && gitSource.Path != "." {
		cloneCmd = fmt.Sprintf("%s && cd /workspace && mv %s/* . && rm -rf %s", cloneCmd, gitSource.Path, gitSource.Path)
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
			RunAsNonRoot:             ptr(true),
			RunAsUser:                ptr(int64(1000)),
			AllowPrivilegeEscalation: ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}

	// Handle credentials if provided
	if gitSource.CredentialsSecretRef != nil {
		// TODO: Implement git credential handling (SSH key or token)
		// This might require mounting the secret and configuring git to use it
	}

	return container, nil
}
