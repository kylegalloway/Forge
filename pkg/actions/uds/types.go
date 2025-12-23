// Package uds provides handlers for UDSBundleJob actions (Create, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a UDS bundle using the UDS CLI.
package uds

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActionResult represents the result of executing a UDS bundle action
type ActionResult struct {
	// JobName is the name of the Kubernetes Job created for this action
	JobName string

	// Phase is the current phase of the action (Pending, Running, Completed, Failed)
	Phase string

	// Message provides details about the action execution
	Message string

	// StartTime when the action started
	StartTime metav1.Time

	// CompletionTime when the action completed
	CompletionTime *metav1.Time

	// Completed indicates if the action has finished
	Completed bool

	// ArtifactLocation where the bundle artifact was stored (for create/publish)
	ArtifactLocation string

	// Error if the action failed
	Error error
}
