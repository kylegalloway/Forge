// Package v1alpha3 defines the v1alpha3 API for UDS bundle job specifications.
//
// This package contains type definitions for UDSBundleJob resources that enable
// creating, publishing, and deploying UDS bundles in a Kubernetes-native way.
//
// v1alpha3 API Changes:
// - Aligned field names with Zarf API (Reference, Variables, ExternalCluster)
// - Uses common.SecretReference and common.ExternalClusterConfig
// - Feature parity with Zarf API (S3Source, GitSource, LocalSource, LocalDestination)
//
// NOTE: UDSBundleJob is a Forge job specification, NOT a UDS bundle definition.
// It describes what operations Forge should perform on UDS bundles.
package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kylegalloway/forge/pkg/apis/common"
)

const (
	// GroupName is the API group name
	GroupName = "forge.dev"
	// Version is the API version
	Version = "v1alpha3"
)

// Action represents an operation to perform on a UDS bundle
// +kubebuilder:validation:Enum=Create;Publish;Deploy;CreatePublish;CreateDeploy;PublishDeploy;CreatePublishDeploy
type Action string

const (
	// ActionCreate creates a UDS bundle from a uds-bundle.yaml definition
	ActionCreate Action = "Create"
	// ActionPublish publishes a built bundle to a destination
	ActionPublish Action = "Publish"
	// ActionDeploy deploys a bundle to a target cluster
	ActionDeploy Action = "Deploy"
	// ActionCreatePublish creates and publishes in sequence
	ActionCreatePublish Action = "CreatePublish"
	// ActionCreateDeploy creates and deploys in sequence
	ActionCreateDeploy Action = "CreateDeploy"
	// ActionPublishDeploy publishes and deploys in sequence
	ActionPublishDeploy Action = "PublishDeploy"
	// ActionCreatePublishDeploy performs all three actions in sequence
	ActionCreatePublishDeploy Action = "CreatePublishDeploy"
)

// SourceType represents where the bundle source or artifact comes from
// +kubebuilder:validation:Enum=Git;S3;OCI;Local
type SourceType string

const (
	// SourceTypeGit pulls bundle definition (uds-bundle.yaml) from a Git repository
	SourceTypeGit SourceType = "Git"
	// SourceTypeS3 pulls bundle tarball from an S3 bucket
	SourceTypeS3 SourceType = "S3"
	// SourceTypeOCI pulls bundle from an OCI registry
	SourceTypeOCI SourceType = "OCI"
	// SourceTypeLocal uses bundle from a local directory
	SourceTypeLocal SourceType = "Local"
)

// DestinationType represents where bundle artifacts are published
// +kubebuilder:validation:Enum=S3;OCI;Local
type DestinationType string

const (
	// DestinationTypeS3 publishes bundle artifacts to an S3 bucket
	DestinationTypeS3 DestinationType = "S3"
	// DestinationTypeOCI publishes bundle artifacts to an OCI registry
	DestinationTypeOCI DestinationType = "OCI"
	// DestinationTypeLocal stores bundle artifacts locally on ephemeral storage
	DestinationTypeLocal DestinationType = "Local"
)

// DeployTargetType represents where bundles are deployed
// +kubebuilder:validation:Enum=InCluster;ExternalCluster
type DeployTargetType string

const (
	// DeployTargetInCluster deploys to the same cluster where Forge is running
	DeployTargetInCluster DeployTargetType = "InCluster"
	// DeployTargetExternalCluster deploys to a different cluster using provided kubeconfig
	DeployTargetExternalCluster DeployTargetType = "ExternalCluster"
)

// UDSBundleJobSpec defines the desired state of a UDSBundleJob
//
// This is a job specification that tells Forge what operations to perform on UDS bundles.
// It is NOT a UDS bundle definition (which uses uds-bundle.yaml format).
type UDSBundleJobSpec struct {
	// ServiceAccountName references the ServiceAccount that defines permissions for this job
	// The ServiceAccount must have forge.dev/* annotations defining allowed actions
	// Cluster admins control what users can do by creating ServiceAccounts with appropriate annotations
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// Action specifies what operation(s) to perform
	// +kubebuilder:validation:Required
	Action Action `json:"action"`

	// Source defines where the bundle definition or artifact comes from
	// +kubebuilder:validation:Required
	Source PackageSource `json:"source"`

	// Create defines configuration for bundle creation (optional)
	// +optional
	Create *CreateConfig `json:"create,omitempty"`

	// Publish defines where to publish the bundle artifact (optional)
	// +optional
	Publish *PublishConfig `json:"publish,omitempty"`

	// Deploy defines deployment configuration (optional)
	// +optional
	Deploy *DeployConfig `json:"deploy,omitempty"`

	// Resources defines resource requirements for the Job pods
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector defines node selection constraints for job pods
	// Allows targeting specific nodes based on labels (e.g., disktype=ssd, region=us-west)
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity defines pod scheduling affinity/anti-affinity rules
	// Supports node affinity, pod affinity, and pod anti-affinity for advanced scheduling
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations define tolerations for job pods
	// Allows pods to be scheduled on nodes with matching taints
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// UseArtifactPVC determines whether to create and attach a PVC for storing build artifacts
	// Set to false to disable PVC creation for simple jobs that don't need artifact persistence
	// Defaults to true (PVC created for all create jobs)
	// +optional
	// +kubebuilder:default=true
	UseArtifactPVC *bool `json:"useArtifactPVC,omitempty"`

	// RetainArtifactPVC determines whether to keep the artifact PVC after job completion
	// Set to false to automatically delete the PVC when the job finishes (success or failure)
	// Defaults to true (keep PVC) for safety
	// Only applies when UseArtifactPVC is true
	// +optional
	// +kubebuilder:default=true
	RetainArtifactPVC *bool `json:"retainArtifactPVC,omitempty"`

	// ExtraMounts specifies additional ConfigMaps or Secrets to mount into all action pods.
	// +optional
	ExtraMounts []common.ExtraMount `json:"extraMounts,omitempty"`

	// DebugMode enables debugging capabilities for this job.
	// When enabled:
	// - Job pods run in debug mode instead of actual commands
	// - Automatic pod/job cleanup is skipped (TTL set to 1 hour)
	// - Enhanced debug logging is emitted for this job's operations
	// This allows operators to exec into pods and inspect the environment.
	// Per-job debugMode takes precedence over global FORGE_DEBUG_MODE environment variable.
	// For chained actions (e.g., CreatePublish), use debugActions to debug specific steps.
	// +optional
	DebugMode bool `json:"debugMode,omitempty"`

	// DebugActions specifies which actions to run in debug mode for chained workflows.
	// If set, only the listed actions will run with debug mode enabled, allowing
	// other actions to proceed normally. Valid values: "create", "publish", "deploy".
	// Example: For a CreatePublish job, set debugActions: ["create"] to debug only the
	// create step while publish runs normally after you signal completion.
	// To signal debug completion and continue to the next action:
	//   kubectl exec -it <pod> -- touch /tmp/debug-complete
	// If debugActions is empty and debugMode is true, all actions run in debug mode.
	// +optional
	DebugActions []string `json:"debugActions,omitempty"`
}

// PackageSource defines where to get the bundle definition or artifact
type PackageSource struct {
	// Type specifies the source type
	// +kubebuilder:validation:Required
	Type SourceType `json:"type"`

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
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// Ref specifies the branch, tag, or commit to checkout
	// +optional
	// +kubebuilder:default="main"
	Ref string `json:"ref,omitempty"`

	// Path to the uds-bundle.yaml file within the repository
	// +optional
	// +kubebuilder:default="."
	Path string `json:"path,omitempty"`

	// CredentialRef references a Secret containing Git credentials
	// +optional
	CredentialRef *common.SecretReference `json:"credentialRef,omitempty"`

	// DisableCloneCredentials prevents using the credentials for clone operations
	// +optional
	// +kubebuilder:default=false
	DisableCloneCredentials bool `json:"disableCloneCredentials,omitempty"`
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

	// CredentialRef references AWS credentials for S3 access
	// Supports three modes:
	// - EnvVar (default): Secret with 'access-key-id' and 'secret-access-key' keys
	// - File: Secret with 'credentials' key containing AWS credentials file
	// - Node: Use node-level credentials (IRSA, instance profile) - no secret needed
	// +optional
	CredentialRef *common.AWSCredentialRef `json:"credentialRef,omitempty"`
}

// OCISource defines an OCI registry source
type OCISource struct {
	// Reference is the full OCI reference (registry/repository:tag or @digest)
	// +kubebuilder:validation:Required
	Reference string `json:"reference"`

	// CredentialRef references a Secret containing registry credentials
	// +optional
	CredentialRef *common.SecretReference `json:"credentialRef,omitempty"`
}

// LocalSource defines a local filesystem source (development only)
type LocalSource struct {
	// Path to the local directory containing uds-bundle.yaml
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// DevMode must be true to use local sources
	// +kubebuilder:validation:Required
	DevMode bool `json:"devMode"`
}

// RetryPolicy defines retry behavior for transient failures
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	// +optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	MaxRetries *int32 `json:"maxRetries,omitempty"`

	// InitialBackoff is the delay before first retry (e.g., "30s", "1m")
	// +optional
	// +kubebuilder:default="30s"
	InitialBackoff string `json:"initialBackoff,omitempty"`

	// MaxBackoff is the maximum delay between retries
	// +optional
	// +kubebuilder:default="5m"
	MaxBackoff string `json:"maxBackoff,omitempty"`

	// BackoffMultiplier for exponential backoff as a percentage (200 = 2.0x, 150 = 1.5x)
	// +optional
	// +kubebuilder:default=200
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=1000
	BackoffMultiplier *int32 `json:"backoffMultiplier,omitempty"`

	// RetryableErrors defines which error patterns should trigger retries
	// Supports glob patterns (e.g., "*timeout*", "*throttle*")
	// If empty, all failures are retryable
	// +optional
	RetryableErrors []string `json:"retryableErrors,omitempty"`
}

// CreateConfig defines configuration for bundle creation
type CreateConfig struct {
	// ExtraMounts specifies additional ConfigMaps or Secrets to mount into the create pod.
	// These are merged with spec-level extraMounts.
	// +optional
	ExtraMounts []common.ExtraMount `json:"extraMounts,omitempty"`

	// Timeout for the create operation
	// +optional
	// +kubebuilder:default="1h"
	Timeout string `json:"timeout,omitempty"`

	// Retry policy for create failures
	// +optional
	Retry *RetryPolicy `json:"retry,omitempty"`

	// RegistryCredentialRef references a Secret with registry credentials for pulling
	// Zarf packages during bundle creation. This is needed when your uds-bundle.yaml
	// references packages from private OCI registries.
	// Secret should be type kubernetes.io/dockerconfigjson
	// +optional
	RegistryCredentialRef *common.SecretReference `json:"registryCredentialRef,omitempty"`

	// Variables are UDS variables to set during bundle creation
	// These are passed as --set KEY=VALUE flags to 'uds create'
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// Flavor specifies which bundle flavor to create
	// +optional
	Flavor string `json:"flavor,omitempty"`

	// Architecture specifies target architecture (e.g., "arm64", "amd64")
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// SkipSBOM disables SBOM generation for faster builds
	// +optional
	SkipSBOM bool `json:"skipSBOM,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'uds create'
	// Use for flags not explicitly supported in the API
	// Example: ["--no-progress"]
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// PublishConfig defines where and how to publish bundle artifacts
type PublishConfig struct {
	// ExtraMounts specifies additional ConfigMaps or Secrets to mount into the publish pod.
	// These are merged with spec-level extraMounts.
	// +optional
	ExtraMounts []common.ExtraMount `json:"extraMounts,omitempty"`

	// Destination defines where to publish
	// +kubebuilder:validation:Required
	Destination PublishDestination `json:"destination"`

	// Timeout for the publish operation
	// +optional
	// +kubebuilder:default="30m"
	Timeout string `json:"timeout,omitempty"`

	// Retry policy for publish failures
	// +optional
	Retry *RetryPolicy `json:"retry,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'uds publish'
	// Use for flags not explicitly supported in the API
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// PublishDestination defines the publish destination
type PublishDestination struct {
	// Type specifies the destination type
	// +kubebuilder:validation:Required
	Type DestinationType `json:"type"`

	// S3 destination configuration
	// +optional
	S3 *S3Destination `json:"s3,omitempty"`

	// OCI destination configuration
	// +optional
	OCI *OCIDestination `json:"oci,omitempty"`

	// Local destination configuration (dev/testing only)
	// +optional
	Local *LocalDestination `json:"local,omitempty"`
}

// S3Destination defines an S3 bucket destination
type S3Destination struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// KeyPrefix is the S3 key prefix (artifact name is appended)
	// +kubebuilder:validation:Required
	KeyPrefix string `json:"keyPrefix"`

	// Region is the AWS region
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint for S3-compatible storage
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialRef references AWS credentials for S3 access
	// Supports three modes:
	// - EnvVar (default): Secret with 'access-key-id' and 'secret-access-key' keys
	// - File: Secret with 'credentials' key containing AWS credentials file
	// - Node: Use node-level credentials (IRSA, instance profile) - no secret needed
	// +optional
	CredentialRef *common.AWSCredentialRef `json:"credentialRef,omitempty"`
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

	// CredentialRef references a Secret containing registry credentials
	// +optional
	CredentialRef *common.SecretReference `json:"credentialRef,omitempty"`
}

// LocalDestination defines local filesystem destination (dev/testing only)
type LocalDestination struct {
	// Path is the local filesystem path for the artifact
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// DevMode must be true to use local destinations
	// +kubebuilder:validation:Required
	DevMode bool `json:"devMode"`
}

// DeployConfig defines deployment configuration
type DeployConfig struct {
	// ExtraMounts specifies additional ConfigMaps or Secrets to mount into the deploy pod.
	// These are merged with spec-level extraMounts.
	// +optional
	ExtraMounts []common.ExtraMount `json:"extraMounts,omitempty"`

	// Target specifies where to deploy
	// +kubebuilder:validation:Required
	Target DeployTargetType `json:"target"`

	// Namespace to deploy into (if applicable)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Timeout for deployment operations
	// +optional
	// +kubebuilder:default="30m"
	Timeout string `json:"timeout,omitempty"`

	// Variables to pass to the bundle deployment
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// ExternalCluster configuration for deploying to a remote cluster
	// +optional
	ExternalCluster *common.ExternalClusterConfig `json:"externalCluster,omitempty"`

	// Components to deploy (if empty, deploys all)
	// +optional
	Components []string `json:"components,omitempty"`

	// Retry policy for deploy failures
	// +optional
	Retry *RetryPolicy `json:"retry,omitempty"`

	// AdoptionPolicy defines how to handle existing resources
	// +optional
	// +kubebuilder:default="Error"
	// +kubebuilder:validation:Enum=Adopt;Skip;Error
	AdoptionPolicy *AdoptionPolicy `json:"adoptionPolicy,omitempty"`

	// ResourceSelector specifies how to discover existing resources to adopt
	// Only used when AdoptionPolicy is "Adopt"
	// +optional
	ResourceSelector *ResourceSelector `json:"resourceSelector,omitempty"`

	// Insecure skips TLS verification during deploy
	// This passes --insecure to uds deploy
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// Retries specifies the number of retry attempts for failed deployments
	// This passes --retries to uds deploy
	// +optional
	Retries *int `json:"retries,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'uds deploy'
	// Use for flags not explicitly supported in the API
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// AdoptionPolicy defines how deploy actions handle existing resources
// +kubebuilder:validation:Enum=Adopt;Skip;Error
type AdoptionPolicy string

const (
	// AdoptionPolicyAdopt takes ownership of existing resources by adding OwnerReferences
	AdoptionPolicyAdopt AdoptionPolicy = "Adopt"

	// AdoptionPolicySkip ignores existing resources and only creates missing ones
	AdoptionPolicySkip AdoptionPolicy = "Skip"

	// AdoptionPolicyError fails deployment if any expected resources already exist (default)
	AdoptionPolicyError AdoptionPolicy = "Error"
)

// ResourceSelector defines how to discover existing resources
type ResourceSelector struct {
	// MatchLabels selects resources with all specified labels
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchNames selects resources by exact name match
	// Supports glob patterns (e.g., "app-*", "service-?-prod")
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`

	// Namespaces to search in (defaults to deploy.namespace)
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// ValidateOwnership ensures resources don't already have other owners
	// +optional
	// +kubebuilder:default=true
	ValidateOwnership *bool `json:"validateOwnership,omitempty"`
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
	// Phase is the current phase (Pending, Running, Completed, Failed, Retrying)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides operation-specific details
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime is when the operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// ArtifactLocation where the artifact was stored
	// +optional
	ArtifactLocation string `json:"artifactLocation,omitempty"`

	// JobName is the Kubernetes Job name for this operation
	// +optional
	JobName string `json:"jobName,omitempty"`

	// RetryCount tracks number of retry attempts
	// +optional
	RetryCount int32 `json:"retryCount,omitempty"`

	// NextRetryTime when next retry will be attempted
	// +optional
	NextRetryTime *metav1.Time `json:"nextRetryTime,omitempty"`

	// LastFailureReason from most recent failure
	// +optional
	LastFailureReason string `json:"lastFailureReason,omitempty"`
}

// UDSBundleJob represents a job to create, publish, or deploy a UDS bundle
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

// GetName implements the PackageResource interface
func (u *UDSBundleJob) GetName() string {
	return u.Name
}

// GetNamespace implements the PackageResource interface
func (u *UDSBundleJob) GetNamespace() string {
	return u.Namespace
}

// GetServiceAccountName implements the PackageResource interface
func (u *UDSBundleJob) GetServiceAccountName() string {
	return u.Spec.ServiceAccountName
}

// GetAction implements the PackageResource interface
func (u *UDSBundleJob) GetAction() string {
	return string(u.Spec.Action)
}

// GetUseArtifactPVC implements the PackageResource interface
// Returns true by default if not specified
func (u *UDSBundleJob) GetUseArtifactPVC() bool {
	if u.Spec.UseArtifactPVC == nil {
		return true
	}
	return *u.Spec.UseArtifactPVC
}

// GetRetainArtifactPVC implements the PackageResource interface
// Returns true by default if not specified
func (u *UDSBundleJob) GetRetainArtifactPVC() bool {
	if u.Spec.RetainArtifactPVC == nil {
		return true
	}
	return *u.Spec.RetainArtifactPVC
}

// GetDebugMode implements the PackageResource interface
// Returns true if debug mode is enabled for this job
func (u *UDSBundleJob) GetDebugMode() bool {
	return u.Spec.DebugMode
}

// GetDebugActions implements the PackageResource interface
// Returns the list of actions to run in debug mode
func (u *UDSBundleJob) GetDebugActions() []string {
	return u.Spec.DebugActions
}
