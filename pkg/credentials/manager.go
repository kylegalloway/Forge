// Package credentials provides utilities for managing Kubernetes credentials (git, registry, S3) for ZarfPackageJob operations.
package credentials

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Type defines the type of credential
type Type string

const (
	// TypeGit represents git credentials (ssh-key or token)
	TypeGit Type = "git"
	// TypeS3 represents S3 credentials (access-key-id, secret-access-key)
	TypeS3 Type = "s3"
	// TypeOCI represents OCI credentials (.dockerconfigjson)
	TypeOCI Type = "oci"
)

// Credential holds the extracted credential data
type Credential struct {
	Type Type
	Data map[string][]byte
}

// Manager handles credential retrieval
type Manager interface {
	GetSecret(ctx context.Context, namespace, name string, credType Type) (*Credential, error)
}

// KubeManager implements Manager using Kubernetes secrets
type KubeManager struct {
	client kubernetes.Interface
}

// NewManager creates a new credential manager
func NewManager(client kubernetes.Interface) *KubeManager {
	return &KubeManager{
		client: client,
	}
}

// GetSecret retrieves and validates a secret for the given type
func (m *KubeManager) GetSecret(ctx context.Context, namespace, name string, credType Type) (*Credential, error) {
	secret, err := m.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	cred := &Credential{
		Type: credType,
		Data: secret.Data,
	}

	if err := m.validate(cred); err != nil {
		return nil, fmt.Errorf("invalid secret %s/%s: %w", namespace, name, err)
	}

	return cred, nil
}

func (m *KubeManager) validate(cred *Credential) error {
	switch cred.Type {
	case TypeGit:
		if _, hasKey := cred.Data["ssh-key"]; hasKey {
			return nil
		}
		if _, hasToken := cred.Data["token"]; hasToken {
			return nil
		}
		return fmt.Errorf("git secret must contain 'ssh-key' or 'token'")

	case TypeS3:
		if _, ok := cred.Data["access-key-id"]; !ok {
			return fmt.Errorf("s3 secret missing 'access-key-id'")
		}
		if _, ok := cred.Data["secret-access-key"]; !ok {
			return fmt.Errorf("s3 secret missing 'secret-access-key'")
		}

	case TypeOCI:
		if _, ok := cred.Data[".dockerconfigjson"]; !ok {
			return fmt.Errorf("oci secret missing '.dockerconfigjson'")
		}
	}

	return nil
}
