// Package common provides shared interfaces and types for generic action handlers.
//
// This package defines the contracts that enable unified Zarf and UDS handler implementations.
//
//nolint:revive // Package name "common" is intentionally generic for cross-cutting concerns
package common

import (
	"context"

	"github.com/kylegalloway/forge/pkg/actions"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
)

// ExecuteOptions contains optional parameters for action execution.
// This extensible struct allows handlers to accept different configurations
// without breaking signatures when new options are added.
type ExecuteOptions struct {
	// ArtifactPath specifies the path to an existing artifact (for Publish/Deploy)
	// When set, handlers use this artifact instead of building/creating a new one
	ArtifactPath string

	// ArtifactPVCName specifies the PVC containing artifacts for multi-action workflows
	// Enables efficient action chaining (Build→Publish→Deploy) without rebuilding
	ArtifactPVCName string
}

// ActionHandler defines the contract for executing package operations.
// Generic handlers implement this interface with type parameters, enabling
// a single implementation to work with both ZarfPackageJob and UDSBundleJob.
type ActionHandler[T apiscommon.PackageResource] interface {
	// Execute runs the action for the given resource with optional parameters
	Execute(ctx context.Context, resource T, opts ExecuteOptions) (*actions.ActionResult, error)
}
