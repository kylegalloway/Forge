package constants

const (
	// ConditionTypeReady indicates the overall job completed successfully.
	// True=succeeded, False=failed terminally, Unknown=in progress.
	ConditionTypeReady = "Ready"

	// ConditionTypeReconciling indicates the controller is actively working on the job.
	ConditionTypeReconciling = "Reconciling"

	// ConditionTypeBuildSucceeded reflects the build operation outcome.
	ConditionTypeBuildSucceeded = "BuildSucceeded"

	// ConditionTypeCreateSucceeded reflects the UDS create operation outcome.
	ConditionTypeCreateSucceeded = "CreateSucceeded"

	// ConditionTypePublishSucceeded reflects the publish operation outcome.
	ConditionTypePublishSucceeded = "PublishSucceeded"

	// ConditionTypeDeploySucceeded reflects the deploy operation outcome.
	ConditionTypeDeploySucceeded = "DeploySucceeded"
)

const (
	// ConditionReasonSucceeded is used when an operation or job completed successfully.
	ConditionReasonSucceeded = "Succeeded"
	// ConditionReasonFailed is used when an operation or job terminated with an error.
	ConditionReasonFailed = "Failed"
	// ConditionReasonProgressing is used while an operation or job is in flight.
	ConditionReasonProgressing = "Progressing"
	// ConditionReasonSuspended is used when reconciliation is paused via spec.suspend.
	ConditionReasonSuspended = "Suspended"
)

// OperationConditionTypes maps OperationStatus field names to their condition type.
var OperationConditionTypes = map[string]string{
	"buildStatus":   ConditionTypeBuildSucceeded,
	"createStatus":  ConditionTypeCreateSucceeded,
	"publishStatus": ConditionTypePublishSucceeded,
	"deployStatus":  ConditionTypeDeploySucceeded,
}
