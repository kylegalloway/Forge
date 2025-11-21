// Package v1alpha1 defines the v1alpha1 API for Zarf package operations.
//
// This package contains type definitions for ZarfPackage resources that enable
// building, publishing, and deploying Zarf packages in a Kubernetes-native way.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GroupName is the API group name
	GroupName = "zarf.dev"
	// Version is the API version
	Version = "v1alpha1"
)

// Action represents an operation to perform on a Zarf package
// +kubebuilder:validation:Enum=Build;Publish;Deploy;BuildPublish;BuildDeploy;PublishDeploy;BuildPublishDeploy
type Action string

const (
	// ActionBuild builds a Zarf package from source
	ActionBuild Action = "Build"
	// ActionPublish publishes a built package to a destination
	ActionPublish Action = "Publish"
	// ActionDeploy deploys a package to a target cluster
	ActionDeploy Action = "Deploy"
	// ActionBuildPublish builds and publishes in sequence
	ActionBuildPublish Action = "BuildPublish"
	// ActionBuildDeploy builds and deploys in sequence
	ActionBuildDeploy Action = "BuildDeploy"
	// ActionPublishDeploy publishes and deploys in sequence
	ActionPublishDeploy Action = "PublishDeploy"
	// ActionBuildPublishDeploy performs all three actions in sequence
	ActionBuildPublishDeploy Action = "BuildPublishDeploy"
)

// SourceType represents where the package source comes from
// +kubebuilder:validation:Enum=Git;S3;OCI;Local
type SourceType string

const (
	// SourceTypeGit pulls package sources from a Git repository
	SourceTypeGit SourceType = "Git"
	// SourceTypeS3 pulls package sources from an S3 bucket
	SourceTypeS3 SourceType = "S3"
	// SourceTypeOCI pulls package sources from an OCI registry
	SourceTypeOCI SourceType = "OCI"
	// SourceTypeLocal uses package sources from a local directory
	SourceTypeLocal SourceType = "Local"
)

// DestinationType represents where artifacts are published
// +kubebuilder:validation:Enum=S3;OCI;Local
type DestinationType string

const (
	// DestinationTypeS3 publishes artifacts to an S3 bucket
	DestinationTypeS3 DestinationType = "S3"
	// DestinationTypeOCI publishes artifacts to an OCI registry
	DestinationTypeOCI DestinationType = "OCI"
	// DestinationTypeLocal stores artifacts locally on ephemeral storage
	DestinationTypeLocal DestinationType = "Local"
)

// DeployTargetType represents where packages are deployed
// +kubebuilder:validation:Enum=InCluster;ExternalCluster
type DeployTargetType string

const (
	// DeployTargetInCluster deploys to the same cluster where Forge is running
	DeployTargetInCluster DeployTargetType = "InCluster"
	// DeployTargetExternalCluster deploys to a different cluster using provided kubeconfig
	DeployTargetExternalCluster DeployTargetType = "ExternalCluster"
)

// ZarfPackageSpec defines the desired state of a ZarfPackage
type ZarfPackageSpec struct {
	// ServiceAccountName references the ServiceAccount that defines permissions for this package
	// The ServiceAccount must have forge.zarf.dev/* annotations defining allowed actions
	// Cluster admins control what users can do by creating ServiceAccounts with appropriate annotations
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// Action specifies what operation(s) to perform
	// +kubebuilder:validation:Required
	Action Action `json:"action"`

	// Source defines where the package definition or artifact comes from
	// +kubebuilder:validation:Required
	Source PackageSource `json:"source"`

	// Publish defines where to publish the built package (required if action includes Publish)
	// +optional
	Publish *PublishConfig `json:"publish,omitempty"`

	// Deploy defines how to deploy the package (required if action includes Deploy)
	// +optional
	Deploy *DeployConfig `json:"deploy,omitempty"`

	// RBACPolicy defines policy restrictions for this resource
	// +optional
	RBACPolicy *RBACPolicy `json:"rbacPolicy,omitempty"`
}

// RBACPolicy defines policy restrictions
type RBACPolicy struct {
	// AllowedUsers specifies which users can use this resource
	// +optional
	AllowedUsers []string `json:"allowedUsers,omitempty"`

	// AllowedActions specifies which actions are permitted
	// +optional
	AllowedActions []Action `json:"allowedActions,omitempty"`

	// AllowedSources specifies which source types/patterns are permitted
	// +optional
	AllowedSources []AllowedSource `json:"allowedSources,omitempty"`

	// AllowedDestinations specifies which destination types/patterns are permitted
	// +optional
	AllowedDestinations []AllowedDestination `json:"allowedDestinations,omitempty"`

	// AllowedDeployTargets specifies which deploy targets are permitted
	// +optional
	AllowedDeployTargets []DeployTargetType `json:"allowedDeployTargets,omitempty"`
}

// AllowedSource defines a permitted source pattern
type AllowedSource struct {
	// Type of source allowed
	// +kubebuilder:validation:Required
	Type SourceType `json:"type"`

	// Repos allowed (glob pattern) for Git
	// +optional
	Repos []string `json:"repos,omitempty"`

	// Buckets allowed (glob pattern) for S3
	// +optional
	Buckets []string `json:"buckets,omitempty"`

	// Images allowed (glob pattern) for OCI
	// +optional
	Images []string `json:"images,omitempty"`
}

// AllowedDestination defines a permitted destination pattern
type AllowedDestination struct {
	// Type of destination allowed
	// +kubebuilder:validation:Required
	Type DestinationType `json:"type"`

	// Buckets allowed (glob pattern) for S3
	// +optional
	Buckets []string `json:"buckets,omitempty"`

	// Registries allowed (glob pattern) for OCI
	// +optional
	Registries []string `json:"registries,omitempty"`
}

// PackageSource defines where to get the package from
type PackageSource struct {
	// Type specifies the source type
	// +kubebuilder:validation:Required
	Type SourceType `json:"type"`

	// Git source configuration (required if type=Git)
	// +optional
	Git *GitSource `json:"git,omitempty"`

	// S3 source configuration (required if type=S3)
	// +optional
	S3 *S3Source `json:"s3,omitempty"`

	// OCI source configuration (required if type=OCI)
	// +optional
	OCI *OCISource `json:"oci,omitempty"`

	// Local source configuration (required if type=Local, dev/testing only)
	// +optional
	Local *LocalSource `json:"local,omitempty"`
}

// GitSource defines a Git repository source
type GitSource struct {
	// URL of the Git repository
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	URL string `json:"url"`

	// Ref is the branch, tag, or commit to checkout
	// +kubebuilder:validation:Required
	Ref string `json:"ref"`

	// Path to the zarf.yaml within the repository
	// +optional
	// +kubebuilder:default="."
	Path string `json:"path,omitempty"`

	// CredentialsSecretRef references a Secret with Git credentials
	// Secret should contain 'ssh-key' or 'token' key
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`
}

// S3Source defines an S3 bucket source
type S3Source struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// Key is the object key (path) in the bucket
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Region is the AWS region
	// +kubebuilder:validation:Required
	Region string `json:"region"`

	// CredentialsSecretRef references a Secret with AWS credentials
	// Secret should contain 'access-key-id' and 'secret-access-key'
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`
}

// OCISource defines an OCI registry source
type OCISource struct {
	// Image is the full OCI image reference (registry/repo:tag or @digest)
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// CredentialsSecretRef references a Secret with registry credentials
	// Secret should be type kubernetes.io/dockerconfigjson
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`
}

// LocalSource defines a local filesystem source (dev/testing only)
type LocalSource struct {
	// Path to the local directory or file
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// DevMode must be explicitly set to true for local sources
	// +kubebuilder:validation:Required
	DevMode bool `json:"devMode"`
}

// PublishConfig defines where and how to publish packages
type PublishConfig struct {
	// Destination specifies where to publish
	// +kubebuilder:validation:Required
	Destination PublishDestination `json:"destination"`
}

// PublishDestination defines the publish target
type PublishDestination struct {
	// Type specifies the destination type
	// +kubebuilder:validation:Required
	Type DestinationType `json:"type"`

	// S3 destination configuration (required if type=S3)
	// +optional
	S3 *S3Destination `json:"s3,omitempty"`

	// OCI destination configuration (required if type=OCI)
	// +optional
	OCI *OCIDestination `json:"oci,omitempty"`

	// Local destination configuration (required if type=Local, dev/testing only)
	// +optional
	Local *LocalDestination `json:"local,omitempty"`
}

// S3Destination defines S3 publish configuration
type S3Destination struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// KeyPrefix for the uploaded artifact
	// +optional
	KeyPrefix string `json:"keyPrefix,omitempty"`

	// Region is the AWS region
	// +kubebuilder:validation:Required
	Region string `json:"region"`

	// CredentialsSecretRef references a Secret with AWS credentials
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`
}

// OCIDestination defines OCI registry publish configuration
type OCIDestination struct {
	// Registry is the OCI registry URL
	// +kubebuilder:validation:Required
	Registry string `json:"registry"`

	// Repository within the registry
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// Tag for the published artifact (supports templating)
	// +kubebuilder:validation:Required
	Tag string `json:"tag"`

	// CredentialsSecretRef references a Secret with registry credentials
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`
}

// LocalDestination defines local filesystem publish (dev/testing only)
type LocalDestination struct {
	// Path to write the artifact
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// DevMode must be explicitly set to true
	// +kubebuilder:validation:Required
	DevMode bool `json:"devMode"`
}

// DeployConfig defines how to deploy the package
type DeployConfig struct {
	// Target specifies where to deploy
	// +kubebuilder:validation:Required
	Target DeployTargetType `json:"target"`

	// Namespace to deploy into
	// +optional
	// +kubebuilder:default="default"
	Namespace string `json:"namespace,omitempty"`

	// Timeout for the deployment
	// +optional
	// +kubebuilder:default="30m"
	Timeout string `json:"timeout,omitempty"`

	// Components to deploy (if not specified, deploys all)
	// +optional
	Components []string `json:"components,omitempty"`

	// SetVariables are Zarf variables to set during deployment
	// +optional
	SetVariables map[string]string `json:"setVariables,omitempty"`

	// ExternalCluster configuration (required if target=ExternalCluster)
	// +optional
	ExternalCluster *ExternalClusterConfig `json:"externalCluster,omitempty"`
}

// ExternalClusterConfig defines connection to an external cluster
type ExternalClusterConfig struct {
	// KubeconfigSecretRef references a Secret containing the kubeconfig
	// +kubebuilder:validation:Required
	KubeconfigSecretRef SecretReference `json:"kubeconfigSecretRef"`

	// Context is the kubeconfig context to use
	// +optional
	Context string `json:"context,omitempty"`
}

// SecretReference references a Kubernetes Secret
type SecretReference struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the secret (defaults to resource namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ZarfPackageStatus defines the observed state of ZarfPackage
type ZarfPackageStatus struct {
	// Phase represents the current phase of the operation
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// BuildStatus contains build operation status
	// +optional
	BuildStatus *OperationStatus `json:"buildStatus,omitempty"`

	// PublishStatus contains publish operation status
	// +optional
	PublishStatus *OperationStatus `json:"publishStatus,omitempty"`

	// DeployStatus contains deploy operation status
	// +optional
	DeployStatus *OperationStatus `json:"deployStatus,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// OperationStatus represents the status of a single operation
type OperationStatus struct {
	// State is the current state (Pending, Running, Completed, Failed)
	State string `json:"state"`

	// StartTime when the operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime when the operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message provides details about the operation
	// +optional
	Message string `json:"message,omitempty"`

	// ArtifactLocation where the artifact was stored (for build/publish)
	// +optional
	ArtifactLocation string `json:"artifactLocation,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=zp
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ZarfPackage is the Schema for the zarf packages API
type ZarfPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZarfPackageSpec   `json:"spec,omitempty"`
	Status ZarfPackageStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ZarfPackageList contains a list of ZarfPackage
type ZarfPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZarfPackage `json:"items"`
}
