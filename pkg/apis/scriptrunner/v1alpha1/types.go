package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScriptRunner is a specification for a ScriptRunner resource
type ScriptRunner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScriptRunnerSpec   `json:"spec"`
	Status ScriptRunnerStatus `json:"status,omitempty"`
}

// ScriptRunnerSpec is the spec for a ScriptRunner resource
type ScriptRunnerSpec struct {
	// Inputs are key-value pairs passed to the script
	Inputs map[string]string `json:"inputs,omitempty"`

	// Image is the container image to use for the job (optional, defaults to hardcoded value)
	Image string `json:"image,omitempty"`

	// Script is the shell script to run (optional, defaults to hardcoded value)
	Script string `json:"script,omitempty"`
}

// ScriptRunnerStatus is the status for a ScriptRunner resource
type ScriptRunnerStatus struct {
	// Phase represents the current phase of the ScriptRunner
	Phase string `json:"phase,omitempty"`

	// JobName is the name of the created Job
	JobName string `json:"jobName,omitempty"`

	// Message contains additional information about the current state
	Message string `json:"message,omitempty"`

	// LastUpdateTime is the last time the status was updated
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScriptRunnerList is a list of ScriptRunner resources
type ScriptRunnerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ScriptRunner `json:"items"`
}
