// Package v1alpha3 defines the v1alpha3 API for Zarf package job specifications.
//
// This package contains type definitions for ZarfPackageJob resources that enable
// building, publishing, and deploying Zarf packages in a Kubernetes-native way.
//
// v1alpha3 API Changes:
// - Aligned field names with UDS API (Reference, Variables, ExternalCluster)
// - Uses common.SecretReference and common.ExternalClusterConfig
// - Feature parity with UDS API (S3Source, GitSource, LocalSource, LocalDestination)
//
// NOTE: ZarfPackageJob is a Forge job specification, NOT a Zarf package definition.
// It describes what operations Forge should perform on Zarf packages.
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

// ZarfPackageJobSpec defines the desired state of a ZarfPackageJob
//
// This is a job specification that tells Forge what operations to perform on Zarf packages.
// It is NOT a Zarf package definition (which uses zarf.yaml format).
type ZarfPackageJobSpec struct {
	// ServiceAccountName references the ServiceAccount that defines permissions for this job
	// The ServiceAccount must have forge.dev/* annotations defining allowed actions
	// Cluster admins control what users can do by creating ServiceAccounts with appropriate annotations
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// Action specifies what operation(s) to perform
	// +kubebuilder:validation:Required
	Action Action `json:"action"`

	// Source defines where the package definition or artifact comes from
	// +kubebuilder:validation:Required
	Source PackageSource `json:"source"`

	// Build defines configuration for building the package (optional)
	// +optional
	Build *BuildConfig `json:"build,omitempty"`

	// Publish defines where to publish the built package (required if action includes Publish)
	// +optional
	Publish *PublishConfig `json:"publish,omitempty"`

	// Deploy defines how to deploy the package (required if action includes Deploy)
	// +optional
	Deploy *DeployConfig `json:"deploy,omitempty"`

	// Resources defines resource requirements for job pods
	// If not specified, defaults to 200m CPU / 512Mi memory requests, 1 CPU / 2Gi limits
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
	// Defaults to true (PVC created for all build jobs)
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

	// DebugMode enables debugging capabilities for this job.
	// When enabled:
	// - Job pods run in debug mode instead of actual commands
	// - Automatic pod/job cleanup is skipped (TTL set to 1 hour)
	// - Enhanced debug logging is emitted for this job's operations
	// This allows operators to exec into pods and inspect the environment.
	// Per-job debugMode takes precedence over global FORGE_DEBUG_MODE environment variable.
	// For chained actions (e.g., BuildPublish), use debugActions to debug specific steps.
	// +optional
	DebugMode bool `json:"debugMode,omitempty"`

	// DebugActions specifies which actions to run in debug mode for chained workflows.
	// If set, only the listed actions will run with debug mode enabled, allowing
	// other actions to proceed normally. Valid values: "build", "publish", "deploy".
	// Example: For a BuildPublish job, set debugActions: ["build"] to debug only the
	// build step while publish runs normally after you signal completion.
	// To signal debug completion and continue to the next action:
	//   kubectl exec -it <pod> -- touch /tmp/debug-complete
	// If debugActions is empty and debugMode is true, all actions run in debug mode.
	// +optional
	DebugActions []string `json:"debugActions,omitempty"`
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
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// Ref is the branch, tag, or commit to checkout
	// +optional
	// +kubebuilder:default="main"
	Ref string `json:"ref,omitempty"`

	// Path to the zarf.yaml within the repository
	// +optional
	// +kubebuilder:default="."
	Path string `json:"path,omitempty"`

	// CredentialRef references a Secret with Git credentials
	// Secret should contain 'ssh-key' or 'token' key
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

	// Key is the object key (path) in the bucket
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
	// Reference is the full OCI reference (registry/repo:tag or @digest)
	// +kubebuilder:validation:Required
	Reference string `json:"reference"`

	// CredentialRef references a Secret with registry credentials
	// Secret should be type kubernetes.io/dockerconfigjson
	// +optional
	CredentialRef *common.SecretReference `json:"credentialRef,omitempty"`
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

// BuildConfig defines configuration for package building
type BuildConfig struct {
	// Timeout for the build operation
	// +optional
	// +kubebuilder:default="1h"
	Timeout string `json:"timeout,omitempty"`

	// Retry policy for build failures
	// +optional
	Retry *RetryPolicy `json:"retry,omitempty"`

	// RegistryCredentialRef references a Secret with registry credentials for pulling
	// images during the build. This is needed when your zarf.yaml references images
	// from private registries (e.g., Docker Hub, private registries).
	// Secret should be type kubernetes.io/dockerconfigjson
	// +optional
	RegistryCredentialRef *common.SecretReference `json:"registryCredentialRef,omitempty"`

	// Variables are Zarf variables to set during package creation
	// These are passed as --set KEY=VALUE flags to 'zarf package create'
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// Flavor specifies which package flavor to build
	// +optional
	Flavor string `json:"flavor,omitempty"`

	// Architecture specifies target architecture (e.g., "arm64", "amd64")
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// SkipSBOM disables SBOM generation for faster builds
	// +optional
	SkipSBOM bool `json:"skipSBOM,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'zarf package create'
	// Use for flags not explicitly supported in the API
	// Example: ["--max-package-size", "100"]
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// PublishConfig defines where and how to publish packages
type PublishConfig struct {
	// Destination specifies where to publish
	// +kubebuilder:validation:Required
	Destination PublishDestination `json:"destination"`

	// Timeout for the publish operation
	// +optional
	// +kubebuilder:default="30m"
	Timeout string `json:"timeout,omitempty"`

	// Retry policy for publish failures
	// +optional
	Retry *RetryPolicy `json:"retry,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'zarf package publish'
	// Use for flags not explicitly supported in the API
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
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

	// CredentialRef references a Secret with registry credentials
	// +optional
	CredentialRef *common.SecretReference `json:"credentialRef,omitempty"`
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

	// Variables are Zarf variables to set during deployment
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// ExternalCluster configuration for deploying to a remote cluster
	// +optional
	ExternalCluster *common.ExternalClusterConfig `json:"externalCluster,omitempty"`

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

	// AdoptExistingResources enables adoption of pre-existing resources
	// This passes --adopt-existing-resources to zarf deploy
	// +optional
	AdoptExistingResources bool `json:"adoptExistingResources,omitempty"`

	// SkipWebhooks disables webhook validation during deploy
	// This passes --skip-webhooks to zarf deploy
	// +optional
	SkipWebhooks bool `json:"skipWebhooks,omitempty"`

	// Retries specifies the number of retry attempts for failed deployments
	// This passes --retries to zarf deploy
	// +optional
	Retries *int `json:"retries,omitempty"`

	// ExtraArgs are additional CLI arguments passed to 'zarf package deploy'
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

// ZarfPackageJobStatus defines the observed state of ZarfPackageJob
type ZarfPackageJobStatus struct {
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
	// Phase is the current phase (Pending, Running, Completed, Failed, Retrying)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides operation-specific details
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime when the operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime when the operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// ArtifactLocation where the artifact was stored (for build/publish)
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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=zpj
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ZarfPackageJob is a Forge job specification for Zarf package operations.
// This is NOT a Zarf package definition - it's a job spec telling Forge what to do.
type ZarfPackageJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZarfPackageJobSpec   `json:"spec,omitempty"`
	Status ZarfPackageJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ZarfPackageJobList contains a list of ZarfPackageJob resources
type ZarfPackageJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZarfPackageJob `json:"items"`
}

// GetName implements the PackageResource interface
func (z *ZarfPackageJob) GetName() string {
	return z.Name
}

// GetNamespace implements the PackageResource interface
func (z *ZarfPackageJob) GetNamespace() string {
	return z.Namespace
}

// GetServiceAccountName implements the PackageResource interface
func (z *ZarfPackageJob) GetServiceAccountName() string {
	return z.Spec.ServiceAccountName
}

// GetAction implements the PackageResource interface
func (z *ZarfPackageJob) GetAction() string {
	return string(z.Spec.Action)
}

// GetUseArtifactPVC implements the PackageResource interface
// Returns true by default if not specified
func (z *ZarfPackageJob) GetUseArtifactPVC() bool {
	if z.Spec.UseArtifactPVC == nil {
		return true
	}
	return *z.Spec.UseArtifactPVC
}

// GetRetainArtifactPVC implements the PackageResource interface
// Returns true by default if not specified
func (z *ZarfPackageJob) GetRetainArtifactPVC() bool {
	if z.Spec.RetainArtifactPVC == nil {
		return true
	}
	return *z.Spec.RetainArtifactPVC
}

// GetDebugMode implements the PackageResource interface
// Returns true if debug mode is enabled for this job
func (z *ZarfPackageJob) GetDebugMode() bool {
	return z.Spec.DebugMode
}

// GetDebugActions implements the PackageResource interface
// Returns the list of actions to run in debug mode
func (z *ZarfPackageJob) GetDebugActions() []string {
	return z.Spec.DebugActions
}
