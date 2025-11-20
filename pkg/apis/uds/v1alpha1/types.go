package v1alpha1

import (
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GroupName is the API group name
	GroupName = "uds.io"
	// Version is the API version
	Version = "v1alpha1"
)

// UDSBundleSpec defines the desired state of a UDSBundle
type UDSBundleSpec struct {
	// ServiceAccountName references the ServiceAccount that defines permissions for this bundle
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// Action specifies what operation(s) to perform
	// +kubebuilder:validation:Required
	Action zarfv1alpha1.Action `json:"action"`

	// Source defines where the bundle definition or artifact comes from
	// +kubebuilder:validation:Required
	Source zarfv1alpha1.PackageSource `json:"source"`

	// Publish defines where to publish the built bundle
	// +optional
	Publish *zarfv1alpha1.PublishConfig `json:"publish,omitempty"`

	// Deploy defines how to deploy the bundle
	// +optional
	Deploy *zarfv1alpha1.DeployConfig `json:"deploy,omitempty"`

	// RBACPolicy defines policy restrictions for this resource
	// +optional
	RBACPolicy *zarfv1alpha1.RBACPolicy `json:"rbacPolicy,omitempty"`
}

// UDSBundleStatus defines the observed state of UDSBundle
type UDSBundleStatus struct {
	// Phase represents the current phase of the operation
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// BuildStatus contains build operation status
	// +optional
	BuildStatus *zarfv1alpha1.OperationStatus `json:"buildStatus,omitempty"`

	// PublishStatus contains publish operation status
	// +optional
	PublishStatus *zarfv1alpha1.OperationStatus `json:"publishStatus,omitempty"`

	// DeployStatus contains deploy operation status
	// +optional
	DeployStatus *zarfv1alpha1.OperationStatus `json:"deployStatus,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ub
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// UDSBundle is the Schema for the UDS bundles API
type UDSBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UDSBundleSpec   `json:"spec,omitempty"`
	Status UDSBundleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// UDSBundleList contains a list of UDSBundle
type UDSBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UDSBundle `json:"items"`
}
