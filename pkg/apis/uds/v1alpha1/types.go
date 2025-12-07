// Package v1alpha1 defines the v1alpha1 API for UDS bundle job specifications.
//
// This package contains type definitions for UDSBundleJob resources that enable
// creating, publishing, and deploying UDS bundles in a Kubernetes-native way.
//
// NOTE: UDSBundleJob is a Forge job specification, NOT a UDS bundle definition.
// It describes what operations Forge should perform on UDS bundles.
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GroupName is the API group name
	GroupName = "forge.dev"
	// Version is the API version
	Version = "v1alpha1"
)

// BundleAction represents an operation to perform on a UDS bundle
// +kubebuilder:validation:Enum=Create;Publish;Deploy;CreatePublish;CreateDeploy;PublishDeploy;CreatePublishDeploy
type BundleAction string

const (
	// BundleActionCreate creates a UDS bundle from a uds-bundle.yaml definition
	BundleActionCreate BundleAction = "Create"
	// BundleActionPublish publishes a built bundle to a destination
	BundleActionPublish BundleAction = "Publish"
	// BundleActionDeploy deploys a bundle to a target cluster
	BundleActionDeploy BundleAction = "Deploy"
	// BundleActionCreatePublish creates and publishes in sequence
	BundleActionCreatePublish BundleAction = "CreatePublish"
	// BundleActionCreateDeploy creates and deploys in sequence
	BundleActionCreateDeploy BundleAction = "CreateDeploy"
	// BundleActionPublishDeploy publishes and deploys in sequence
	BundleActionPublishDeploy BundleAction = "PublishDeploy"
	// BundleActionCreatePublishDeploy performs all three actions in sequence
	BundleActionCreatePublishDeploy BundleAction = "CreatePublishDeploy"
)

// BundleSourceType represents where the bundle source or artifact comes from
// +kubebuilder:validation:Enum=Git;S3;OCI;Local
type BundleSourceType string

const (
	// BundleSourceTypeGit pulls bundle definition (uds-bundle.yaml) from a Git repository
	BundleSourceTypeGit BundleSourceType = "Git"
	// BundleSourceTypeS3 pulls bundle tarball from an S3 bucket
	BundleSourceTypeS3 BundleSourceType = "S3"
	// BundleSourceTypeOCI pulls bundle from an OCI registry
	BundleSourceTypeOCI BundleSourceType = "OCI"
	// BundleSourceTypeLocal uses bundle from a local directory
	BundleSourceTypeLocal BundleSourceType = "Local"
)

// BundleDestinationType represents where bundle artifacts are published
// +kubebuilder:validation:Enum=S3;OCI;Local
type BundleDestinationType string

const (
	// BundleDestinationTypeS3 publishes bundle artifacts to an S3 bucket
	BundleDestinationTypeS3 BundleDestinationType = "S3"
	// BundleDestinationTypeOCI publishes bundle artifacts to an OCI registry
	BundleDestinationTypeOCI BundleDestinationType = "OCI"
	// BundleDestinationTypeLocal stores bundle artifacts locally on ephemeral storage
	BundleDestinationTypeLocal BundleDestinationType = "Local"
)

// BundleDeployTargetType represents where bundles are deployed
// +kubebuilder:validation:Enum=InCluster;ExternalCluster
type BundleDeployTargetType string

const (
	// BundleDeployTargetInCluster deploys to the same cluster where Forge is running
	BundleDeployTargetInCluster BundleDeployTargetType = "InCluster"
	// BundleDeployTargetExternalCluster deploys to a different cluster using provided kubeconfig
	BundleDeployTargetExternalCluster BundleDeployTargetType = "ExternalCluster"
)

// UDSBundleJobSpec defines the desired state of a UDSBundleJob
//
// This is a job specification that tells Forge what operations to perform on UDS bundles.
// It is NOT a UDS bundle definition (which uses uds-bundle.yaml format).
type UDSBundleJobSpec struct {
	// ServiceAccountName references the ServiceAccount that defines permissions for this job
	// The ServiceAccount must have forge.forge.dev/* annotations defining allowed actions
	// Cluster admins control what users can do by creating ServiceAccounts with appropriate annotations
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// Action specifies what operation(s) to perform
	// +kubebuilder:validation:Required
	Action BundleAction `json:"action"`

	// Source defines where the bundle definition or artifact comes from
	// +kubebuilder:validation:Required
	Source BundleSource `json:"source"`

	// Publish defines where to publish the bundle artifact (optional)
	// +optional
	Publish *BundlePublishConfig `json:"publish,omitempty"`

	// Deploy defines deployment configuration (optional)
	// +optional
	Deploy *BundleDeployConfig `json:"deploy,omitempty"`

	// Resources defines resource requirements for the Job pods
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// BundleSource defines where to get the bundle definition or artifact
type BundleSource struct {
	// Type specifies the source type
	// +kubebuilder:validation:Required
	Type BundleSourceType `json:"type"`

	// Git source for uds-bundle.yaml
	// +optional
	Git *GitSource `json:"git,omitempty"`

	// S3 source for pre-built bundle tarball
	// +optional
	S3 *S3Source `json:"s3,omitempty"`

	// OCI source for bundle from OCI registry
	// +optional
	OCI *OCISource `json:"oci,omitempty"`

	// Local source for development only
	// +optional
	Local *LocalSource `json:"local,omitempty"`
}

// GitSource defines a Git repository source
type GitSource struct {
	// URL of the Git repository
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Ref specifies the branch, tag, or commit to checkout
	// +kubebuilder:default="main"
	Ref string `json:"ref,omitempty"`

	// Path to the uds-bundle.yaml file within the repository
	// +kubebuilder:default="."
	Path string `json:"path,omitempty"`

	// CredentialsSecretRef references a Secret containing Git credentials
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// S3Source defines an S3 bucket source
type S3Source struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// Key is the object key/path within the bucket
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Region is the AWS region
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint for S3-compatible storage (e.g., MinIO)
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecretRef references a Secret containing AWS credentials
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// OCISource defines an OCI registry source
type OCISource struct {
	// Reference is the full OCI reference (registry/repository:tag or @digest)
	// +kubebuilder:validation:Required
	Reference string `json:"reference"`

	// CredentialsSecretRef references a Secret containing registry credentials
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// LocalSource defines a local filesystem source (development only)
type LocalSource struct {
	// Path to the local directory containing uds-bundle.yaml
	// +kubebuilder:validation:Required
	Path string `json:"path"`
}

// BundlePublishConfig defines where and how to publish bundle artifacts
type BundlePublishConfig struct {
	// Destination defines where to publish
	// +kubebuilder:validation:Required
	Destination BundleDestination `json:"destination"`
}

// BundleDestination defines the publish destination
type BundleDestination struct {
	// Type specifies the destination type
	// +kubebuilder:validation:Required
	Type BundleDestinationType `json:"type"`

	// S3 destination configuration
	// +optional
	S3 *S3Destination `json:"s3,omitempty"`

	// OCI destination configuration
	// +optional
	OCI *OCIDestination `json:"oci,omitempty"`
}

// S3Destination defines an S3 bucket destination
type S3Destination struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// Key is the object key/path within the bucket
	// +optional
	Key string `json:"key,omitempty"`

	// Region is the AWS region
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint for S3-compatible storage
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecretRef references a Secret containing AWS credentials
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// OCIDestination defines an OCI registry destination
type OCIDestination struct {
	// Registry hostname
	// +kubebuilder:validation:Required
	Registry string `json:"registry"`

	// Repository path
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// Tag for the bundle artifact
	// +kubebuilder:validation:Required
	Tag string `json:"tag"`

	// CredentialsSecretRef references a Secret containing registry credentials
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// BundleDeployConfig defines deployment configuration
type BundleDeployConfig struct {
	// Target specifies where to deploy
	// +kubebuilder:validation:Required
	Target BundleDeployTargetType `json:"target"`

	// Namespace to deploy into (if applicable)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Timeout for deployment operations
	// +kubebuilder:default="60m"
	Timeout string `json:"timeout,omitempty"`

	// Kubeconfig for external cluster deployment
	// +optional
	Kubeconfig *KubeconfigReference `json:"kubeconfig,omitempty"`

	// Variables to pass to the bundle deployment
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// Components to deploy (if empty, deploys all)
	// +optional
	Components []string `json:"components,omitempty"`
}

// KubeconfigReference references a kubeconfig for external cluster deployment
type KubeconfigReference struct {
	// SecretRef references a Secret containing kubeconfig data
	// +kubebuilder:validation:Required
	SecretRef corev1.SecretReference `json:"secretRef"`

	// Key within the Secret containing the kubeconfig
	// +kubebuilder:default="kubeconfig"
	Key string `json:"key,omitempty"`
}

// UDSBundleJobStatus defines the observed state of a UDSBundleJob
type UDSBundleJobStatus struct {
	// Phase represents the current phase of the job
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// CreateStatus tracks the create operation status
	// +optional
	CreateStatus *OperationStatus `json:"createStatus,omitempty"`

	// PublishStatus tracks the publish operation status
	// +optional
	PublishStatus *OperationStatus `json:"publishStatus,omitempty"`

	// DeployStatus tracks the deploy operation status
	// +optional
	DeployStatus *OperationStatus `json:"deployStatus,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// OperationStatus tracks the status of a specific operation
type OperationStatus struct {
	// State represents the operation state
	// +optional
	State string `json:"state,omitempty"`

	// Message provides operation-specific details
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime is when the operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// JobName is the Kubernetes Job name for this operation
	// +optional
	JobName string `json:"jobName,omitempty"`
}

// UDSBundleJob represents a job to create, publish, or deploy a UDS bundle
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ubj
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type UDSBundleJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UDSBundleJobSpec   `json:"spec,omitempty"`
	Status UDSBundleJobStatus `json:"status,omitempty"`
}

// UDSBundleJobList contains a list of UDSBundleJob
// +kubebuilder:object:root=true
type UDSBundleJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UDSBundleJob `json:"items"`
}
