// Package common contains shared types used by both UDS and Zarf APIs.
//
//nolint:revive // package name "common" is intentional for shared API types
package common

// SecretReference contains information to locate a secret
type SecretReference struct {
	// Name is the name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the secret. Defaults to the namespace of the referencing resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AWSCredentialType specifies how AWS credentials are provided
// +kubebuilder:validation:Enum=EnvVar;File;Node
type AWSCredentialType string

const (
	// AWSCredentialTypeEnvVar loads credentials from secret keys as environment variables
	// Secret must contain 'access-key-id' and 'secret-access-key' keys
	AWSCredentialTypeEnvVar AWSCredentialType = "EnvVar"

	// AWSCredentialTypeFile mounts a credentials file from the secret
	// Secret must contain a 'credentials' key with AWS credentials file format
	AWSCredentialTypeFile AWSCredentialType = "File"

	// AWSCredentialTypeNode relies on node-level credentials (IRSA, instance profile, etc.)
	// No secret is needed - AWS SDK handles credential resolution
	AWSCredentialTypeNode AWSCredentialType = "Node"
)

// AWSCredentialRef references AWS credentials with configurable loading method
type AWSCredentialRef struct {
	// Type specifies how credentials are loaded
	// - EnvVar: Load from secret keys as environment variables (default)
	// - File: Mount secret as AWS credentials file
	// - Node: Use node-level credentials (IRSA, instance profile) - no secret needed
	// +optional
	// +kubebuilder:default="EnvVar"
	Type AWSCredentialType `json:"type,omitempty"`

	// Name is the name of the secret containing credentials
	// Required for EnvVar and File types, ignored for Node type
	// +optional
	Name string `json:"name,omitempty"`

	// Namespace of the secret. Defaults to the namespace of the referencing resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key is the key within the secret containing the credentials file
	// Only used when Type is File. Defaults to "credentials"
	// +optional
	// +kubebuilder:default="credentials"
	Key string `json:"key,omitempty"`
}

// ExternalClusterConfig defines configuration for deploying to a remote cluster
type ExternalClusterConfig struct {
	// SecretRef references a secret containing the kubeconfig
	// +kubebuilder:validation:Required
	SecretRef SecretReference `json:"secretRef"`

	// Key is the key within the secret containing the kubeconfig data
	// +optional
	// +kubebuilder:default="kubeconfig"
	Key string `json:"key,omitempty"`

	// Context specifies which context to use from the kubeconfig
	// +optional
	Context string `json:"context,omitempty"`
}
