package controller

import (
	"context"

	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
)

// MetricsRecorder defines the interface for recording metrics for package operations.
// This abstraction allows the generic monitor to work with both Zarf and UDS resources
// without hardcoding specific metric method names.
type MetricsRecorder[T apiscommon.PackageResource] interface {
	// RecordPrimaryActionStarted records when the primary action (Build/Create) starts
	RecordPrimaryActionStarted(ctx context.Context, namespace, name string)

	// RecordPrimaryActionCompleted records when the primary action completes successfully
	RecordPrimaryActionCompleted(ctx context.Context, namespace, name string)

	// RecordPrimaryActionFailed records when the primary action fails
	RecordPrimaryActionFailed(ctx context.Context, namespace, name string)

	// RecordPublishStarted records when a publish action starts
	RecordPublishStarted(ctx context.Context, namespace, name string)

	// RecordPublishCompleted records when a publish action completes successfully
	RecordPublishCompleted(ctx context.Context, namespace, name string)

	// RecordPublishFailed records when a publish action fails
	RecordPublishFailed(ctx context.Context, namespace, name string)

	// RecordDeployStarted records when a deploy action starts
	RecordDeployStarted(ctx context.Context, namespace, name string)

	// RecordDeployCompleted records when a deploy action completes successfully
	RecordDeployCompleted(ctx context.Context, namespace, name string)

	// RecordDeployFailed records when a deploy action fails
	RecordDeployFailed(ctx context.Context, namespace, name string)

	// RecordJobCreated records when a job is created
	RecordJobCreated(ctx context.Context, namespace, name, action string)

	// RecordJobCompleted records when a job completes
	RecordJobCompleted(ctx context.Context, namespace, name, action string)

	// RecordJobFailed records when a job fails
	RecordJobFailed(ctx context.Context, namespace, name, action string)

	// RecordActionDuration records the duration of an action
	RecordActionDuration(ctx context.Context, namespace, name, action string, duration float64, status string)
}
