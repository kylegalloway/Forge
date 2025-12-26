// Package actions provides shared types and utilities for Zarf and UDS action handlers.
//
// This package consolidates common code used by both pkg/actions/zarf and
// pkg/actions/uds subpackages.
package actions

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActionResult represents the result of executing an action (Build, Create, Publish, Deploy).
// This type is shared by both Zarf and UDS handlers.
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

	// ArtifactLocation where the artifact was stored (for build/create/publish)
	ArtifactLocation string

	// Error if the action failed
	Error error
}

// Helper functions

// Ptr returns a pointer to the given value.
// This is a generic helper used throughout action handlers.
func Ptr[T any](v T) *T {
	return &v
}

// MustParseQuantity parses a resource quantity string or panics.
// Used for defining default resource requirements in Job specs.
func MustParseQuantity(quantityStr string) resource.Quantity {
	return resource.MustParse(quantityStr)
}
