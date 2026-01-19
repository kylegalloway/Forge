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
